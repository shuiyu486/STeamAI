param(
  [string]$CaseRoot = '',
  [string]$Pack = '_template',
  [string]$WorkRoot = ''
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
$Rekit = Join-Path $RekitRoot 'rekit.ps1'

function Invoke-RekitSmoke {
  param(
    [Parameter(Mandatory=$true)][string[]]$Arguments,
    [int[]]$AllowedExitCodes = @(0),
    [hashtable]$Env = @{},
    [string]$ScriptPath = $Rekit
  )
  $oldValues = @{}
  foreach ($key in $Env.Keys) {
    $oldValues[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
    [Environment]::SetEnvironmentVariable($key, [string]$Env[$key], 'Process')
  }
  $oldEap = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $ScriptPath @Arguments 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
  } finally {
    $ErrorActionPreference = $oldEap
    foreach ($key in $Env.Keys) { [Environment]::SetEnvironmentVariable($key, $oldValues[$key], 'Process') }
  }
  if ($AllowedExitCodes -notcontains $exitCode) {
    throw "unexpected exit code $exitCode; output:`n$output"
  }
  $global:LASTEXITCODE = 0
  return $output
}

function Assert-ContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Expected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text -notlike "*$Expected*") { throw "$Label missing expected text '$Expected'. Output:`n$Text" }
}

function Assert-NotContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Unexpected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text -like "*$Unexpected*") { throw "$Label contained unexpected text '$Unexpected'. Output:`n$Text" }
}

function Write-FakeGoBackend {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$CommandName
  )
  $json = '{"schemaVersion":1,"command":"' + $CommandName + '","delegatedByFake":true,"isMutation":false,"applied":false}'
  [System.IO.File]::WriteAllText($Path, ('@echo off' + "`r`n" + 'echo ' + $json + "`r`n"), [System.Text.UTF8Encoding]::new($false))
}

function Write-CapturingFakeGoBackend {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$CapturePath
  )
  $json = '{"schemaVersion":1,"command":"plan-subagents","delegatedByFake":true,"isMutation":false,"applied":false}'
  $script = '@echo off' + "`r`n" + 'echo %* > "' + $CapturePath + '"' + "`r`n" + 'echo ' + $json + "`r`n"
  [System.IO.File]::WriteAllText($Path, $script, [System.Text.UTF8Encoding]::new($false))
}

function Write-PreauthorizedGateProfile {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $laneRoot = Join-Path $CaseRoot '.rekit\lanes\main'
  New-Item -ItemType Directory -Path $laneRoot -Force | Out-Null
  $profileLines = @(
    '{',
    '  "schemaVersion": 1,',
    '  "profileId": "facade-smoke-main-debug",',
    '  "lane": "main",',
    '  "mode": "preauthorized",',
    '  "allowedActions": ["debug"],',
    '  "deniedActions": ["symex"],',
    '  "targetScope": [{"match":"exact","value":"target-alpha"}],',
    '  "budget": {"runtimeSeconds": 60, "diskMB": 128, "requests": 2},',
    '  "stopConditions": ["timeout"],',
    '  "outputPaths": ["workspace/main/debug"],',
    '  "recordRequired": true,',
    '  "notifyMainOn": ["boundary-hit", "new-risk"],',
    '  "grantedBy": "facade-smoke",',
    '  "grantedAt": "2026-01-01T00:00:00Z",',
    '  "expiresAt": "2999-01-01T00:00:00Z"',
    '}'
  )
  [System.IO.File]::WriteAllText((Join-Path $laneRoot 'autonomy.json'), ([string]::Join("`n", $profileLines) + "`n"), [System.Text.UTF8Encoding]::new($false))
}

function Assert-FakeDelegation {
  param(
    [Parameter(Mandatory=$true)][string[]]$Arguments,
    [Parameter(Mandatory=$true)][string]$CommandName,
    [Parameter(Mandatory=$true)][string]$Label
  )
  Write-FakeGoBackend -Path $fakeGo -CommandName $CommandName
  $out = Invoke-RekitSmoke -Arguments $Arguments -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $out -Expected '"delegatedByFake":true' -Label $Label
}

function Assert-FakeDefaultDelegation {
  param(
    [Parameter(Mandatory=$true)][string[]]$Arguments,
    [Parameter(Mandatory=$true)][string]$CommandName,
    [Parameter(Mandatory=$true)][string]$Label
  )
  Write-FakeGoBackend -Path $fakeGo -CommandName $CommandName
  $out = Invoke-RekitSmoke -Arguments $Arguments -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $out -Expected '"delegatedByFake":true' -Label $Label
}

function Assert-FakeFallback {
  param(
    [Parameter(Mandatory=$true)][string[]]$Arguments,
    [Parameter(Mandatory=$true)][string]$Expected,
    [Parameter(Mandatory=$true)][string]$Label,
    [int[]]$AllowedExitCodes = @(0)
  )
  Write-FakeGoBackend -Path $fakeGo -CommandName 'must-not-run'
  $out = Invoke-RekitSmoke -Arguments $Arguments -AllowedExitCodes $AllowedExitCodes -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $out -Unexpected 'delegatedByFake' -Label $Label
  Assert-ContainsText -Text $out -Expected $Expected -Label $Label
}

function Assert-FacadeRuntimeNoLegacyDependency {
  $facadeText = [System.IO.File]::ReadAllText($Rekit, [System.Text.Encoding]::UTF8)
  foreach ($unexpected in @("Join-Path `$RuntimeRoot 'lib\", 'Get-RekitPackInventory -RepoRoot', 'Invoke-RekitStart -Target', 'Sync-RekitPack -Target', 'Promote-RekitChanges -Target', 'Write-RekitSubagentPlan -Target')) {
    Assert-NotContainsText -Text $facadeText -Unexpected $unexpected -Label 'facade source has no legacy runtime dependency'
  }
  foreach ($expected in @('Invoke-RekitGoBackend -Invocation $goInvocation', 'if (Test-RekitNoPowerShellFallbackCommand -Name $Command)')) {
    Assert-ContainsText -Text $facadeText -Expected $expected -Label 'facade source keeps Go delegation and no-fallback guard'
  }

  Write-FakeGoBackend -Path $fakeGo -CommandName 'status'
  $delegatedOut = Invoke-RekitSmoke -Arguments @('-Command','status') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $delegatedOut -Expected '"delegatedByFake":true' -Label 'default delegation remains Go-owned'

  $disabledOut = Invoke-RekitSmoke -Arguments @('-Command','status') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $disabledOut -Expected 'PowerShell fallback has been retired' -Label 'disabled no-fallback remains retired'
}

$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
if ([string]::IsNullOrWhiteSpace($WorkRoot)) { $WorkRoot = $env:TEMP }
$matrixRoot = Join-Path $WorkRoot "rekit-facade-matrix-$suffix"
New-Item -ItemType Directory -Path $matrixRoot -Force | Out-Null
$fakeGo = Join-Path $matrixRoot 'fake-rekit-go.cmd'
$usingSelfContainedCase = [string]::IsNullOrWhiteSpace($CaseRoot)
$gateLane = 'main'

try {
  if ($usingSelfContainedCase) {
    $CaseRoot = Join-Path $matrixRoot 'case'
    Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$CaseRoot,'-Pack',$Pack,'-ProjectName',"facade-smoke-$suffix",'-Apply') | Out-Null
    Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack) | Out-Null
    Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-Kind','observation','-Lane','main','-Subject',"facade-smoke-$suffix",'-Summary','seed observation for facade smoke','-Actor','facade-smoke') | Out-Null
    Write-PreauthorizedGateProfile -CaseRoot $CaseRoot
  } elseif ([string]::Equals($Pack, 'vmp-re', [System.StringComparison]::OrdinalIgnoreCase)) {
    $gateLane = 'feature-handler-0x40a010'
  }

  Assert-FacadeRuntimeNoLegacyDependency

  # Low-risk read-only commands, overview/note reads, note append/what-if, gate what-if/apply request paths, bounded case lifecycle writes, sync review/apply, promote review/candidate/apply writes, promote JSON previews, start/handoff JSON preview/apply paths, continue JSON preview/apply, and plan-subagents review artifacts default to Go; retired groups fail instead of using PowerShell fallback.
  $out = Invoke-RekitSmoke -Arguments @('-Command','status')
  Assert-ContainsText -Text $out -Expected 'rekit go backend:' -Label 'default go status'

  $packsOut = Invoke-RekitSmoke -Arguments @('-Command','packs')
  Assert-ContainsText -Text $packsOut -Expected "pack`t" -Label 'default go packs'
  Assert-ContainsText -Text $packsOut -Expected "vmp-re`t" -Label 'default go packs'

  $caseDoctor = Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$CaseRoot,'-Pack',$Pack)
  Assert-ContainsText -Text $caseDoctor -Expected 'instance validation ok' -Label 'default go case doctor'

  Assert-FakeDefaultDelegation -Arguments @('-Command','status') -CommandName 'status' -Label 'default status fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','packs') -CommandName 'packs' -Label 'default packs fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','release-check','-Format','json') -CommandName 'release-check' -Label 'default release-check fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','doctor','-Target',$CaseRoot,'-Pack',$Pack) -CommandName 'doctor' -Label 'default doctor fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','validate','-Target',$CaseRoot,'-Pack',$Pack) -CommandName 'validate' -Label 'default validate fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','attach','-Target',(Join-Path $matrixRoot 'default-attach'),'-Pack',$Pack,'-Apply') -CommandName 'attach' -Label 'default attach apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','repair','-Target',$CaseRoot,'-Pack',$Pack,'-Apply') -CommandName 'repair' -Label 'default repair apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','init','-Target',(Join-Path $matrixRoot 'default-init'),'-Pack',$Pack,'-ProjectName',"default-init-$suffix",'-Apply') -CommandName 'init' -Label 'default init apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','bootstrap','-Target',(Join-Path $matrixRoot 'default-bootstrap'),'-Pack',$Pack,'-ProjectName',"default-bootstrap-$suffix",'-Apply') -CommandName 'bootstrap' -Label 'default bootstrap apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack) -CommandName 'sync' -Label 'default sync review fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply') -CommandName 'sync' -Label 'default sync apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','update','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-Format','json') -CommandName 'update' -Label 'default update apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack) -CommandName 'promote' -Label 'default promote review fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-ReviewOutputDir',(Join-Path $matrixRoot 'promote-review-default')) -CommandName 'promote' -Label 'default promote review artifacts fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf','-Format','json') -CommandName 'promote' -Label 'default promote candidate JSON preview fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates') -CommandName 'promote' -Label 'default promote create-candidates fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -CommandName 'promote' -Label 'default promote apply JSON preview fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply') -CommandName 'promote' -Label 'default promote apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack) -CommandName 'overview' -Label 'default overview fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack,'-Format','json') -CommandName 'overview' -Label 'default overview JSON fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List') -CommandName 'note' -Label 'default note list text fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List','-Format','json') -CommandName 'note' -Label 'default note list JSON fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-Kind','observation','-Lane','main','-Subject','default note append','-Summary','fake default note append') -CommandName 'note' -Label 'default note append fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-Kind','observation','-Lane','main','-Subject','default note what-if','-Summary','fake default note what-if','-WhatIf') -CommandName 'note' -Label 'default note what-if fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane',$gateLane) -CommandName 'gate' -Label 'default gate what-if fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-Action','debug','-Lane',$gateLane,'-Actor','facade-smoke') -CommandName 'gate' -Label 'default gate apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'default-start-preview','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'start' -Label 'default start JSON preview fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'default-start-apply','-Pack',$Pack,'-Apply','-Executor','facade-session','-Actor','facade-smoke','-Reason','facade explicit takeover') -CommandName 'start' -Label 'default start apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'handoff' -Label 'default handoff JSON preview fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'-Pack',$Pack,'-Apply') -CommandName 'handoff' -Label 'default handoff apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'continue' -Label 'default continue JSON preview fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-Apply') -CommandName 'continue' -Label 'default continue apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','plan-subagents','-Target',$CaseRoot,'-Pack',$Pack,'-Items','alpha,beta','-ReviewOutputDir',(Join-Path $matrixRoot 'plan-default')) -CommandName 'plan-subagents' -Label 'default plan-subagents fake delegation'

  $gateOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane',$gateLane,'-Subject','facade smoke default gate')
  Assert-ContainsText -Text $gateOut -Expected '"isMutation": false' -Label 'default go gate dry-run'
  Assert-ContainsText -Text $gateOut -Expected '"status": "pending-gate"' -Label 'default go gate dry-run'

  if ($usingSelfContainedCase) {
    $workspaceRoot = Join-Path $CaseRoot 'workspace\main\debug\session-1'
    New-Item -ItemType Directory -Path $workspaceRoot -Force | Out-Null
    $gateApplyOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-Action','debug','-Lane','main','-Actor','facade-smoke','-Subject','facade smoke authorized gate','-TargetRef','target-alpha','-BatchId','facade-smoke-nested-output','-Scope','handler only','-RuntimeSeconds','30','-DiskMB','64','-Requests','1','-OutputPaths','workspace/main/debug/session-1','-StopConditions','timeout','-Format','json')
    $gateApply = $gateApplyOut | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace([string]$gateApply.eventId)) { throw "facade nested product path gate apply did not return eventId. Output:`n$gateApplyOut" }
    $adapterReport = '{"schemaVersion":1,"kind":"adapter-execution-report","adapterId":"facade-smoke-adapter","action":"debug","status":"succeeded","gateEventId":"' + [string]$gateApply.eventId + '","actualBudget":{"runtimeSeconds":20,"diskMB":32,"requests":1},"outputRefs":["workspace/main/debug/session-1/result.json"],"summary":"facade smoke adapter report"}'
    [System.IO.File]::WriteAllText((Join-Path $workspaceRoot 'adapter-report.json'), $adapterReport, [System.Text.UTF8Encoding]::new($false))
    [System.IO.File]::WriteAllText((Join-Path $workspaceRoot 'result.json'), '{"ok":true}', [System.Text.UTF8Encoding]::new($false))
    Push-Location $workspaceRoot
    try {
      $nestedContractOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Pack',$Pack,'-GateEventId',([string]$gateApply.eventId),'-ExecutionReportContract','-Format','json')
      $nestedValidationOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Pack',$Pack,'-GateEventId',([string]$gateApply.eventId),'-ValidateExecutionReport','-ExecutionReportPath','adapter-report.json','-Format','json')
      $nestedEvidenceOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Pack',$Pack,'-Apply','-GateEventId',([string]$gateApply.eventId),'-ExecutionReportPath','adapter-report.json','-Actor','facade-smoke-adapter','-Format','json')
    } finally {
      Pop-Location
    }
    Assert-ContainsText -Text $nestedContractOut -Expected '"kind": "adapter-execution-report-contract"' -Label 'facade nested workspace contract product path'
    Assert-ContainsText -Text $nestedContractOut -Expected '"isMutation": false' -Label 'facade nested workspace contract product path'
    Assert-ContainsText -Text $nestedValidationOut -Expected '"kind": "adapter-execution-report-validation"' -Label 'facade nested workspace validation product path'
    Assert-ContainsText -Text $nestedValidationOut -Expected '"valid": true' -Label 'facade nested workspace validation product path'
    Assert-ContainsText -Text $nestedValidationOut -Expected '"applied": false' -Label 'facade nested workspace validation product path'
    Assert-ContainsText -Text $nestedValidationOut -Expected '"reportPath": "workspace/main/debug/session-1/adapter-report.json"' -Label 'facade nested workspace validation product path'
    Assert-ContainsText -Text $nestedEvidenceOut -Expected '"applied": true' -Label 'facade nested workspace evidence product path'
    Assert-ContainsText -Text $nestedEvidenceOut -Expected '"path": ".rekit/facts/observations.jsonl"' -Label 'facade nested workspace evidence product path'
    Assert-ContainsText -Text $nestedEvidenceOut -Expected '"executionReportPath": "workspace/main/debug/session-1/adapter-report.json"' -Label 'facade nested workspace evidence product path'
    Assert-ContainsText -Text $nestedEvidenceOut -Expected '"adapterId": "facade-smoke-adapter"' -Label 'facade nested workspace evidence product path'
    $observationsText = [System.IO.File]::ReadAllText((Join-Path $CaseRoot '.rekit\facts\observations.jsonl'), [System.Text.Encoding]::UTF8)
    Assert-ContainsText -Text $observationsText -Expected '"executionReportPath":"workspace/main/debug/session-1/adapter-report.json"' -Label 'facade nested workspace evidence ledger'
    Assert-ContainsText -Text $observationsText -Expected '"adapterId":"facade-smoke-adapter"' -Label 'facade nested workspace evidence ledger'
    if (Test-Path -LiteralPath (Join-Path $CaseRoot '.rekit\facts\authority.jsonl')) { throw 'facade nested workspace evidence wrote authority ledger' }
    if (Test-Path -LiteralPath (Join-Path $CaseRoot '.rekit\facts\confirmed.jsonl')) { throw 'facade nested workspace evidence wrote confirmed ledger' }
  }

  # Explicit enable delegates the remaining expanded preview/review safe set.
  $goEnv = @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }
  $out = Invoke-RekitSmoke -Arguments @('-Command','status') -Env $goEnv
  Assert-ContainsText -Text $out -Expected 'rekit go backend:' -Label 'explicit go status'

  $caseDoctor = Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$CaseRoot,'-Pack',$Pack) -Env $goEnv
  Assert-ContainsText -Text $caseDoctor -Expected 'instance validation ok' -Label 'explicit go case doctor'

  $reviewRoot = Join-Path $matrixRoot 'sync-review'
  $syncOut = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-ReviewOutputDir',$reviewRoot) -Env $goEnv
  Assert-ContainsText -Text $syncOut -Expected '"writesArtifacts": true' -Label 'go sync review'
  if (-not (Test-Path -LiteralPath (Join-Path $reviewRoot 'packet.json'))) { throw 'sync review packet was not written' }
  if (-not (Test-Path -LiteralPath (Join-Path $reviewRoot 'diffs\combined.diff'))) { throw 'sync review combined diff was not written' }

  $gateApplyMissingActorOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-Action','debug','-Lane',$gateLane) -AllowedExitCodes @(1) -Env $goEnv
  Assert-ContainsText -Text $gateApplyMissingActorOut -Expected 'gate -Apply requires -Actor' -Label 'gate apply actor guard'

  # JSON sync apply preview remains delegated; default actual sync apply delegation is covered by the fake backend matrix below.
  $syncApplyJson = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -Env $goEnv
  Assert-ContainsText -Text $syncApplyJson -Expected '"isMutation": false' -Label 'go sync apply JSON preview'
  Assert-ContainsText -Text $syncApplyJson -Expected '"applied": false' -Label 'go sync apply JSON preview'

  # Fake backend matrix proves the default sync/case lifecycle/promote/readonly workstream set is delegated and unrelated preview/text paths are not.
  $attachRoot = Join-Path $matrixRoot 'attach-preview-case'
  Assert-FakeDefaultDelegation -Arguments @('-Command','attach','-Target',$attachRoot,'-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'attach' -Label 'default attach JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','attach','-Target',$attachRoot,'-Pack',$Pack,'-WhatIf') -CommandName 'attach' -Label 'default attach text preview delegation'

  Assert-FakeDefaultDelegation -Arguments @('-Command','repair','-Target',$CaseRoot,'-Pack',$Pack,'-Format','json') -CommandName 'repair' -Label 'default repair JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','repair','-Target',$CaseRoot,'-Pack',$Pack) -CommandName 'repair' -Label 'default repair text preview delegation'

  $initRoot = Join-Path $matrixRoot 'init-preview-case'
  $bootstrapRoot = Join-Path $matrixRoot 'bootstrap-preview-case'
  Assert-FakeDefaultDelegation -Arguments @('-Command','init','-Target',$initRoot,'-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'init' -Label 'default init JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','bootstrap','-Target',$bootstrapRoot,'-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'bootstrap' -Label 'default bootstrap JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','init','-Target',$initRoot,'-Pack',$Pack,'-WhatIf') -CommandName 'init' -Label 'default init text preview delegation'

  Assert-FakeDefaultDelegation -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -CommandName 'sync' -Label 'default sync apply JSON preview delegation'
  Assert-FakeFallback -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Expected 'PowerShell fallback has been retired' -Label 'sync apply text preview no fallback' -AllowedExitCodes @(1)

  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf','-Format','json') -CommandName 'promote' -Label 'default promote candidate JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates') -CommandName 'promote' -Label 'default promote create-candidates delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -CommandName 'promote' -Label 'default promote apply JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply') -CommandName 'promote' -Label 'default promote apply delegation'
  Assert-FakeFallback -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf') -Expected 'PowerShell fallback has been retired' -Label 'promote create-candidates text preview no fallback' -AllowedExitCodes @(1)
  Assert-FakeFallback -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Expected 'PowerShell fallback has been retired' -Label 'promote apply text preview no fallback' -AllowedExitCodes @(1)
  Assert-FakeFallback -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-CreateCandidates') -Expected 'PowerShell fallback has been retired' -Label 'promote actual write combination no fallback' -AllowedExitCodes @(1)

  Assert-FakeDefaultDelegation -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack) -CommandName 'overview' -Label 'default overview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack,'-Format','json') -CommandName 'overview' -Label 'default overview JSON delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List') -CommandName 'note' -Label 'default note list text delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List','-Format','json') -CommandName 'note' -Label 'default note list JSON delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-Kind','verification','-Lane','main','-Subject','matrix note append','-Summary','fake default note append') -CommandName 'note' -Label 'default note append delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-Kind','verification','-Lane','main','-Subject','matrix note what-if','-Summary','fake default note what-if','-WhatIf') -CommandName 'note' -Label 'default note what-if delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane',$gateLane) -CommandName 'gate' -Label 'default gate what-if delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-Action','debug','-Lane',$gateLane,'-Actor','facade-smoke') -CommandName 'gate' -Label 'default gate apply delegation'
  $gateExecutionCapturePath = Join-Path $matrixRoot 'gate-execution-args.txt'
  Write-CapturingFakeGoBackend -Path $fakeGo -CapturePath $gateExecutionCapturePath
  $gateExecutionOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-GateEventId','evt-authorized-gate','-ExecutionStatus','succeeded','-Actor','facade-smoke','-ActualRuntimeSeconds','3','-ActualDiskMB','4','-ActualRequests','1','-OutputRefs','workspace/main/debug/out.json','-ExecutionEvidenceRefs','workspace/main/debug/evidence.json','-BoundaryHits','timeout','-Escalation','manual review','-ExecutionReportPath','workspace/main/debug/adapter-report.json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $gateExecutionOut -Expected '"delegatedByFake":true' -Label 'default gate execution evidence delegation'
  $capturedGateExecutionArgs = [System.IO.File]::ReadAllText($gateExecutionCapturePath, [System.Text.Encoding]::Default)
  foreach ($expectedGateArg in @('-GateEventId evt-authorized-gate','-ExecutionStatus succeeded','-ActualRuntimeSeconds 3','-ActualDiskMB 4','-ActualRequests 1','-OutputRefs workspace/main/debug/out.json','-ExecutionEvidenceRefs workspace/main/debug/evidence.json','-BoundaryHits timeout','-Escalation "manual review"','-ExecutionReportPath workspace/main/debug/adapter-report.json')) {
    Assert-ContainsText -Text $capturedGateExecutionArgs -Expected $expectedGateArg -Label 'gate execution evidence facade args'
  }
  $gateContractCapturePath = Join-Path $matrixRoot 'gate-execution-contract-args.txt'
  Write-CapturingFakeGoBackend -Path $fakeGo -CapturePath $gateContractCapturePath
  $gateContractOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-GateEventId','evt-authorized-gate','-ExecutionReportContract','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $gateContractOut -Expected '"delegatedByFake":true' -Label 'default gate execution report contract delegation'
  $capturedGateContractArgs = [System.IO.File]::ReadAllText($gateContractCapturePath, [System.Text.Encoding]::Default)
  foreach ($expectedGateArg in @('-GateEventId evt-authorized-gate','-ExecutionReportContract','-Format json')) {
    Assert-ContainsText -Text $capturedGateContractArgs -Expected $expectedGateArg -Label 'gate execution report contract facade args'
  }
  $workspaceRoot = Join-Path $CaseRoot 'workspace\main\debug\session-1'
  New-Item -ItemType Directory -Path $workspaceRoot -Force | Out-Null
  $gateNestedContractCapturePath = Join-Path $matrixRoot 'gate-nested-contract-args.txt'
  Write-CapturingFakeGoBackend -Path $fakeGo -CapturePath $gateNestedContractCapturePath
  Push-Location $workspaceRoot
  try {
    $gateNestedContractOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Pack',$Pack,'-GateEventId','evt-authorized-gate','-ExecutionReportContract','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  } finally {
    Pop-Location
  }
  Assert-ContainsText -Text $gateNestedContractOut -Expected '"delegatedByFake":true' -Label 'nested gate execution report contract delegation'
  $capturedGateNestedContractArgs = [System.IO.File]::ReadAllText($gateNestedContractCapturePath, [System.Text.Encoding]::Default)
  foreach ($expectedGateArg in @('-GateEventId evt-authorized-gate','-ExecutionReportContract','-Format json')) {
    Assert-ContainsText -Text $capturedGateNestedContractArgs -Expected $expectedGateArg -Label 'nested gate execution report contract facade args'
  }
  Assert-NotContainsText -Text $capturedGateNestedContractArgs -Unexpected '-Target ' -Label 'nested gate execution report contract omits facade target'
  $gateValidationCapturePath = Join-Path $matrixRoot 'gate-execution-validation-args.txt'
  Write-CapturingFakeGoBackend -Path $fakeGo -CapturePath $gateValidationCapturePath
  $gateValidationOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-GateEventId','evt-authorized-gate','-ValidateExecutionReport','-ExecutionReportPath','workspace/main/debug/adapter-report.json','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $gateValidationOut -Expected '"delegatedByFake":true' -Label 'default gate execution report validation delegation'
  $capturedGateValidationArgs = [System.IO.File]::ReadAllText($gateValidationCapturePath, [System.Text.Encoding]::Default)
  foreach ($expectedGateArg in @('-GateEventId evt-authorized-gate','-ValidateExecutionReport','-ExecutionReportPath workspace/main/debug/adapter-report.json','-Format json')) {
    Assert-ContainsText -Text $capturedGateValidationArgs -Expected $expectedGateArg -Label 'gate execution report validation facade args'
  }
  $gateNestedValidationCapturePath = Join-Path $matrixRoot 'gate-nested-validation-args.txt'
  Write-CapturingFakeGoBackend -Path $fakeGo -CapturePath $gateNestedValidationCapturePath
  Push-Location $workspaceRoot
  try {
    $gateNestedValidationOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Pack',$Pack,'-GateEventId','evt-authorized-gate','-ValidateExecutionReport','-ExecutionReportPath','adapter-report.json','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  } finally {
    Pop-Location
  }
  Assert-ContainsText -Text $gateNestedValidationOut -Expected '"delegatedByFake":true' -Label 'nested gate execution report validation delegation'
  $capturedGateNestedValidationArgs = [System.IO.File]::ReadAllText($gateNestedValidationCapturePath, [System.Text.Encoding]::Default)
  foreach ($expectedGateArg in @('-GateEventId evt-authorized-gate','-ValidateExecutionReport','-ExecutionReportPath adapter-report.json','-Format json')) {
    Assert-ContainsText -Text $capturedGateNestedValidationArgs -Expected $expectedGateArg -Label 'nested gate execution report validation facade args'
  }
  Assert-NotContainsText -Text $capturedGateNestedValidationArgs -Unexpected '-Target ' -Label 'nested gate execution report validation omits facade target'
  $gateNestedEvidenceCapturePath = Join-Path $matrixRoot 'gate-nested-evidence-args.txt'
  Write-CapturingFakeGoBackend -Path $fakeGo -CapturePath $gateNestedEvidenceCapturePath
  Push-Location $workspaceRoot
  try {
    $gateNestedEvidenceOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Pack',$Pack,'-Apply','-GateEventId','evt-authorized-gate','-ExecutionReportPath','adapter-report.json','-Actor','facade-smoke','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  } finally {
    Pop-Location
  }
  Assert-ContainsText -Text $gateNestedEvidenceOut -Expected '"delegatedByFake":true' -Label 'nested gate execution report evidence delegation'
  $capturedGateNestedEvidenceArgs = [System.IO.File]::ReadAllText($gateNestedEvidenceCapturePath, [System.Text.Encoding]::Default)
  foreach ($expectedGateArg in @('-Apply','-GateEventId evt-authorized-gate','-ExecutionReportPath adapter-report.json','-Actor facade-smoke','-Format json')) {
    Assert-ContainsText -Text $capturedGateNestedEvidenceArgs -Expected $expectedGateArg -Label 'nested gate execution report evidence facade args'
  }
  Assert-NotContainsText -Text $capturedGateNestedEvidenceArgs -Unexpected '-Target ' -Label 'nested gate execution report evidence omits facade target'
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-lane','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'start' -Label 'default start JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-apply','-Pack',$Pack,'-Apply','-Executor','matrix-session','-Actor','facade-smoke','-Reason','matrix explicit takeover') -CommandName 'start' -Label 'default start apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-json-apply','-Pack',$Pack,'-Apply','-Format','json') -CommandName 'start' -Label 'default start JSON apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'handoff' -Label 'default handoff JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-Apply') -CommandName 'handoff' -Label 'default handoff apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-Apply','-Format','json') -CommandName 'handoff' -Label 'default handoff JSON apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'continue' -Label 'default continue JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-Apply') -CommandName 'continue' -Label 'default continue apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','plan-subagents','-Target',$CaseRoot,'-Pack',$Pack,'-Items','matrix-alpha,matrix-beta','-ReviewOutputDir',(Join-Path $matrixRoot 'plan-matrix')) -CommandName 'plan-subagents' -Label 'default plan-subagents delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','plan-subagents','-Target',$CaseRoot,'-Pack',$Pack,'-PacketPath',(Join-Path $matrixRoot 'packet.json'),'-ReviewerResultPath',(Join-Path $matrixRoot 'reviewer-result.json'),'-Lane','main','-Actor','facade-smoke','-WhatIf','-Format','json') -CommandName 'plan-subagents' -Label 'default plan-subagents reviewer intake preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','plan-subagents','-Target',$CaseRoot,'-Pack',$Pack,'-PacketPath',(Join-Path $matrixRoot 'packet.json'),'-ReviewerResultPath',(Join-Path $matrixRoot 'reviewer-result.json'),'-Lane','main','-Actor','facade-smoke','-Apply','-Format','json') -CommandName 'plan-subagents' -Label 'default plan-subagents reviewer intake apply delegation'

  $capturePath = Join-Path $matrixRoot 'relative-reviewer-intake-args.txt'
  Write-CapturingFakeGoBackend -Path $fakeGo -CapturePath $capturePath
  Push-Location $matrixRoot
  try {
    $relativeReviewerOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$CaseRoot,'-Pack',$Pack,'-PacketPath','relative\packet.json','-ReviewerResultPath','relative\reviewer-result.json','-Lane','main','-Actor','facade-smoke','-WhatIf','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  } finally {
    Pop-Location
  }
  Assert-ContainsText -Text $relativeReviewerOut -Expected '"delegatedByFake":true' -Label 'relative reviewer intake delegation'
  $capturedArgs = [System.IO.File]::ReadAllText($capturePath, [System.Text.Encoding]::Default)
  Assert-ContainsText -Text $capturedArgs -Expected (Join-Path $matrixRoot 'relative\packet.json') -Label 'relative packet path uses caller cwd'
  Assert-ContainsText -Text $capturedArgs -Expected (Join-Path $matrixRoot 'relative\reviewer-result.json') -Label 'relative reviewer result path uses caller cwd'

  $relativePlanCapturePath = Join-Path $matrixRoot 'relative-plan-args.txt'
  Write-CapturingFakeGoBackend -Path $fakeGo -CapturePath $relativePlanCapturePath
  $relativeReviewOut = Join-Path $matrixRoot 'relative\review-out'
  Push-Location $matrixRoot
  try {
    $relativePlanOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$CaseRoot,'-Pack',$Pack,'-ItemsFile','relative\items.txt','-ReviewOutputDir','relative\review-out','-DiffPath','relative\combined.diff') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  } finally {
    Pop-Location
  }
  Assert-ContainsText -Text $relativePlanOut -Expected '"delegatedByFake":true' -Label 'relative plan artifact delegation'
  $capturedPlanArgs = [System.IO.File]::ReadAllText($relativePlanCapturePath, [System.Text.Encoding]::Default)
  Assert-ContainsText -Text $capturedPlanArgs -Expected (Join-Path $matrixRoot 'relative\items.txt') -Label 'relative items file uses caller cwd'
  Assert-ContainsText -Text $capturedPlanArgs -Expected $relativeReviewOut -Label 'relative review output dir uses caller cwd'
  Assert-ContainsText -Text $capturedPlanArgs -Expected (Join-Path $matrixRoot 'relative\combined.diff') -Label 'relative diff path uses caller cwd'

  Assert-FakeFallback -Arguments @('-Command','status','-ReviewerResultPath',(Join-Path $matrixRoot 'reviewer-result.json')) -Expected 'ReviewerResultPath is supported only by plan-subagents reviewer intake' -Label 'reviewer result path rejected outside plan-subagents' -AllowedExitCodes @(1)
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-lane','-Pack',$Pack,'-WhatIf') -CommandName 'start' -Label 'default start text preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-lane','-Pack',$Pack) -CommandName 'start' -Label 'default start bare text delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf') -CommandName 'handoff' -Label 'default handoff text preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack) -CommandName 'handoff' -Label 'default handoff bare text delegation'

  Assert-FakeDefaultDelegation -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf') -CommandName 'continue' -Label 'default continue text preview delegation'

  # Disable wins over default and explicit enable for legacy fallback candidates; low-risk read-only fallback has been retired.
  $disabledOut = Invoke-RekitSmoke -Arguments @('-Command','status') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '1' }
  Assert-ContainsText -Text $disabledOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled status no fallback'
  $disabledDefaultOut = Invoke-RekitSmoke -Arguments @('-Command','packs') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledDefaultOut -Unexpected 'delegatedByFake' -Label 'go disabled default packs no fallback'
  Assert-ContainsText -Text $disabledDefaultOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled default packs no fallback'
  $disabledGateOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane',$gateLane) -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledGateOut -Unexpected 'delegatedByFake' -Label 'go disabled gate no fallback'
  Assert-ContainsText -Text $disabledGateOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled gate no fallback'
  $disabledSyncPreviewOut = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledSyncPreviewOut -Unexpected 'delegatedByFake' -Label 'go disabled sync JSON preview no fallback'
  Assert-ContainsText -Text $disabledSyncPreviewOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled sync JSON preview no fallback'
  $disabledSyncApplyOut = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledSyncApplyOut -Unexpected 'delegatedByFake' -Label 'go disabled sync apply no fallback'
  Assert-ContainsText -Text $disabledSyncApplyOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled sync apply no fallback'
  $disabledPromotePreviewOut = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf','-Format','json') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledPromotePreviewOut -Unexpected 'delegatedByFake' -Label 'go disabled promote JSON preview no fallback'
  Assert-ContainsText -Text $disabledPromotePreviewOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled promote JSON preview no fallback'
  $disabledPromoteApplyOut = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledPromoteApplyOut -Unexpected 'delegatedByFake' -Label 'go disabled promote apply no fallback'
  Assert-ContainsText -Text $disabledPromoteApplyOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled promote apply no fallback'
  $disabledUpdateOut = Invoke-RekitSmoke -Arguments @('-Command','update','-Target',$CaseRoot,'-Pack',$Pack) -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledUpdateOut -Unexpected 'delegatedByFake' -Label 'go disabled update review no fallback'
  Assert-ContainsText -Text $disabledUpdateOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled update review no fallback'
  $disabledAttachOut = Invoke-RekitSmoke -Arguments @('-Command','attach','-Target',(Join-Path $matrixRoot 'disabled-attach'),'-Pack',$Pack,'-WhatIf') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledAttachOut -Unexpected 'delegatedByFake' -Label 'go disabled attach no fallback'
  Assert-ContainsText -Text $disabledAttachOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled attach no fallback'
  $disabledRepairOut = Invoke-RekitSmoke -Arguments @('-Command','repair','-Target',$CaseRoot,'-Pack',$Pack) -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledRepairOut -Unexpected 'delegatedByFake' -Label 'go disabled repair no fallback'
  Assert-ContainsText -Text $disabledRepairOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled repair no fallback'
  $disabledInitOut = Invoke-RekitSmoke -Arguments @('-Command','init','-Target',(Join-Path $matrixRoot 'disabled-init'),'-Pack',$Pack,'-ProjectName',"disabled-init-$suffix",'-WhatIf') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledInitOut -Unexpected 'delegatedByFake' -Label 'go disabled init no fallback'
  Assert-ContainsText -Text $disabledInitOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled init no fallback'
  $disabledBootstrapOut = Invoke-RekitSmoke -Arguments @('-Command','bootstrap','-Target',(Join-Path $matrixRoot 'disabled-bootstrap'),'-Pack',$Pack,'-ProjectName',"disabled-bootstrap-$suffix",'-Apply') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledBootstrapOut -Unexpected 'delegatedByFake' -Label 'go disabled bootstrap no fallback'
  Assert-ContainsText -Text $disabledBootstrapOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled bootstrap no fallback'
  $disabledOverviewOut = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack,'-Format','json') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledOverviewOut -Unexpected 'delegatedByFake' -Label 'go disabled overview JSON no fallback'
  Assert-ContainsText -Text $disabledOverviewOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled overview JSON no fallback'
  $disabledNoteTextOut = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledNoteTextOut -Unexpected 'delegatedByFake' -Label 'go disabled note text no fallback'
  Assert-ContainsText -Text $disabledNoteTextOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled note text no fallback'
  $disabledNoteOut = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List','-Format','json') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledNoteOut -Unexpected 'delegatedByFake' -Label 'go disabled note JSON no fallback'
  Assert-ContainsText -Text $disabledNoteOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled note JSON no fallback'
  $disabledNoteAppendOut = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-Kind','observation','-Lane','main','-Subject','disabled note append','-Summary','fallback note append','-Actor','facade-smoke') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledNoteAppendOut -Unexpected 'delegatedByFake' -Label 'go disabled note append no fallback'
  Assert-ContainsText -Text $disabledNoteAppendOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled note append no fallback'
  $disabledStartTextOut = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$CaseRoot,'disabled-start-text','-Pack',$Pack,'-WhatIf') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledStartTextOut -Unexpected 'delegatedByFake' -Label 'go disabled start text no fallback'
  Assert-ContainsText -Text $disabledStartTextOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled start text no fallback'
  $disabledStartApplyOut = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$CaseRoot,'disabled-start','-Pack',$Pack,'-Apply') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledStartApplyOut -Unexpected 'delegatedByFake' -Label 'go disabled start apply no fallback'
  Assert-ContainsText -Text $disabledStartApplyOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled start apply no fallback'
  $disabledHandoffTextOut = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledHandoffTextOut -Unexpected 'delegatedByFake' -Label 'go disabled handoff text no fallback'
  Assert-ContainsText -Text $disabledHandoffTextOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled handoff text no fallback'
  $disabledHandoffApplyOut = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-Apply') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledHandoffApplyOut -Unexpected 'delegatedByFake' -Label 'go disabled handoff apply no fallback'
  Assert-ContainsText -Text $disabledHandoffApplyOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled handoff apply no fallback'
  $disabledContinueTextOut = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledContinueTextOut -Unexpected 'delegatedByFake' -Label 'go disabled continue text no fallback'
  Assert-ContainsText -Text $disabledContinueTextOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled continue text no fallback'
  $disabledContinuePreviewOut = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf','-Format','json') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledContinuePreviewOut -Unexpected 'delegatedByFake' -Label 'go disabled continue JSON preview no fallback'
  Assert-ContainsText -Text $disabledContinuePreviewOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled continue JSON preview no fallback'
  $disabledContinueApplyOut = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-Apply') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledContinueApplyOut -Unexpected 'delegatedByFake' -Label 'go disabled continue apply no fallback'
  Assert-ContainsText -Text $disabledContinueApplyOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled continue apply no fallback'
  $disabledPlanOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$CaseRoot,'-Pack',$Pack,'-Items','disabled-alpha,disabled-beta','-ReviewOutputDir',(Join-Path $matrixRoot 'plan-disabled')) -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledPlanOut -Unexpected 'delegatedByFake' -Label 'go disabled plan-subagents no fallback'
  Assert-ContainsText -Text $disabledPlanOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled plan-subagents no fallback'
  $disabledReviewerIntakeOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$CaseRoot,'-Pack',$Pack,'-PacketPath',(Join-Path $matrixRoot 'packet.json'),'-ReviewerResultPath',(Join-Path $matrixRoot 'reviewer-result.json'),'-Lane','main','-Actor','facade-smoke','-Apply','-Format','json') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledReviewerIntakeOut -Unexpected 'delegatedByFake' -Label 'go disabled reviewer intake no fallback'
  Assert-ContainsText -Text $disabledReviewerIntakeOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled reviewer intake no fallback'

  'facade smoke ok'
} finally {
  if (Test-Path -LiteralPath $matrixRoot) { Remove-Item -LiteralPath $matrixRoot -Recurse -Force -Confirm:$false }
}
