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
    [hashtable]$Env = @{}
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
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $Rekit @Arguments 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
  } finally {
    $ErrorActionPreference = $oldEap
    foreach ($key in $Env.Keys) { [Environment]::SetEnvironmentVariable($key, $oldValues[$key], 'Process') }
  }
  if ($AllowedExitCodes -notcontains $exitCode) {
    throw "unexpected exit code $exitCode; output:`n$output"
  }
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
  } elseif ([string]::Equals($Pack, 'vmp-re', [System.StringComparison]::OrdinalIgnoreCase)) {
    $gateLane = 'feature-handler-0x40a010'
  }

  # Low-risk read-only commands, overview/note reads, note append/what-if, gate what-if/apply request paths, bounded case lifecycle writes, sync review/apply, promote review/candidate/apply writes, promote JSON previews, start/handoff JSON preview/apply paths, and continue JSON preview default to Go.
  $out = Invoke-RekitSmoke -Arguments @('-Command','status')
  Assert-ContainsText -Text $out -Expected 'rekit go backend:' -Label 'default go status'

  $packsOut = Invoke-RekitSmoke -Arguments @('-Command','packs')
  Assert-ContainsText -Text $packsOut -Expected "pack`t" -Label 'default go packs'
  Assert-ContainsText -Text $packsOut -Expected "vmp-re`t" -Label 'default go packs'

  $caseDoctor = Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$CaseRoot,'-Pack',$Pack)
  Assert-ContainsText -Text $caseDoctor -Expected 'instance validation ok' -Label 'default go case doctor'

  Assert-FakeDefaultDelegation -Arguments @('-Command','status') -CommandName 'status' -Label 'default status fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','packs') -CommandName 'packs' -Label 'default packs fake delegation'
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
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'default-start-apply','-Pack',$Pack,'-Apply') -CommandName 'start' -Label 'default start apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'handoff' -Label 'default handoff JSON preview fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'-Pack',$Pack,'-Apply') -CommandName 'handoff' -Label 'default handoff apply fake delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'continue' -Label 'default continue JSON preview fake delegation'

  $gateOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane',$gateLane,'-Subject','facade smoke default gate')
  Assert-ContainsText -Text $gateOut -Expected '"isMutation": false' -Label 'default go gate dry-run'
  Assert-ContainsText -Text $gateOut -Expected '"status": "pending-gate"' -Label 'default go gate dry-run'

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
  Assert-FakeFallback -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Expected 'would attach case' -Label 'sync apply text preview fallback'

  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf','-Format','json') -CommandName 'promote' -Label 'default promote candidate JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates') -CommandName 'promote' -Label 'default promote create-candidates delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -CommandName 'promote' -Label 'default promote apply JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply') -CommandName 'promote' -Label 'default promote apply delegation'
  Assert-FakeFallback -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf') -Expected 'promote summary:' -Label 'promote create-candidates text preview fallback'
  Assert-FakeFallback -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Expected 'promote summary:' -Label 'promote apply text preview fallback'
  Assert-FakeFallback -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-CreateCandidates') -Expected 'promote -Apply cannot be combined with -CreateCandidates' -Label 'promote actual write combination fallback' -AllowedExitCodes @(1)

  Assert-FakeDefaultDelegation -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack) -CommandName 'overview' -Label 'default overview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack,'-Format','json') -CommandName 'overview' -Label 'default overview JSON delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List') -CommandName 'note' -Label 'default note list text delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List','-Format','json') -CommandName 'note' -Label 'default note list JSON delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-Kind','verification','-Lane','main','-Subject','matrix note append','-Summary','fake default note append') -CommandName 'note' -Label 'default note append delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-Kind','verification','-Lane','main','-Subject','matrix note what-if','-Summary','fake default note what-if','-WhatIf') -CommandName 'note' -Label 'default note what-if delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane',$gateLane) -CommandName 'gate' -Label 'default gate what-if delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-Action','debug','-Lane',$gateLane,'-Actor','facade-smoke') -CommandName 'gate' -Label 'default gate apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-lane','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'start' -Label 'default start JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-apply','-Pack',$Pack,'-Apply') -CommandName 'start' -Label 'default start apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-json-apply','-Pack',$Pack,'-Apply','-Format','json') -CommandName 'start' -Label 'default start JSON apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'handoff' -Label 'default handoff JSON preview delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-Apply') -CommandName 'handoff' -Label 'default handoff apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-Apply','-Format','json') -CommandName 'handoff' -Label 'default handoff JSON apply delegation'
  Assert-FakeDefaultDelegation -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'continue' -Label 'default continue JSON preview delegation'
  Assert-FakeFallback -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-lane','-Pack',$Pack,'-WhatIf') -Expected 'would create or enter feature workstream:' -Label 'start text preview fallback'
  Assert-FakeFallback -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-lane','-Pack',$Pack) -Expected 'feature-' -Label 'start bare text fallback'
  Assert-FakeFallback -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf') -Expected 'would write workstream handoff:' -Label 'handoff text preview fallback'
  Assert-FakeFallback -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack) -Expected 'main-latest.md' -Label 'handoff bare text fallback'

  Assert-FakeFallback -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf') -Expected 'prompts/RESUME.md' -Label 'continue text preview fallback'

  # Disable wins over default and explicit enable.
  $disabledOut = Invoke-RekitSmoke -Arguments @('-Command','status') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '1' }
  Assert-ContainsText -Text $disabledOut -Expected 'rekit runtime:' -Label 'go disabled status fallback'
  $disabledDefaultOut = Invoke-RekitSmoke -Arguments @('-Command','packs') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledDefaultOut -Unexpected 'delegatedByFake' -Label 'go disabled default packs fallback'
  Assert-ContainsText -Text $disabledDefaultOut -Expected "pack`t" -Label 'go disabled default packs fallback'
  $disabledAttachOut = Invoke-RekitSmoke -Arguments @('-Command','attach','-Target',(Join-Path $matrixRoot 'disabled-attach'),'-Pack',$Pack,'-WhatIf') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledAttachOut -Unexpected 'delegatedByFake' -Label 'go disabled attach fallback'
  Assert-ContainsText -Text $disabledAttachOut -Expected 'would attach case' -Label 'go disabled attach fallback'
  $disabledOverviewOut = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack,'-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledOverviewOut -Unexpected 'delegatedByFake' -Label 'go disabled overview JSON fallback'
  Assert-ContainsText -Text $disabledOverviewOut -Expected '"command"' -Label 'go disabled overview JSON fallback'
  Assert-ContainsText -Text $disabledOverviewOut -Expected 'overview' -Label 'go disabled overview JSON fallback'
  $disabledNoteTextOut = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledNoteTextOut -Unexpected 'delegatedByFake' -Label 'go disabled note text fallback'
  Assert-ContainsText -Text $disabledNoteTextOut -Expected '[observation]' -Label 'go disabled note text fallback'
  $disabledNoteOut = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledNoteOut -Unexpected 'delegatedByFake' -Label 'go disabled note JSON fallback'
  Assert-ContainsText -Text $disabledNoteOut -Expected '"command"' -Label 'go disabled note JSON fallback'
  Assert-ContainsText -Text $disabledNoteOut -Expected 'note' -Label 'go disabled note JSON fallback'
  $disabledNoteAppendOut = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-Kind','observation','-Lane','main','-Subject','disabled note append','-Summary','fallback note append','-Actor','facade-smoke') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledNoteAppendOut -Unexpected 'delegatedByFake' -Label 'go disabled note append fallback'
  Assert-ContainsText -Text $disabledNoteAppendOut -Expected 'observation' -Label 'go disabled note append fallback'
  $disabledStartApplyOut = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$CaseRoot,'disabled-start','-Pack',$Pack,'-Apply') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledStartApplyOut -Unexpected 'delegatedByFake' -Label 'go disabled start apply fallback'
  Assert-ContainsText -Text $disabledStartApplyOut -Expected 'feature-' -Label 'go disabled start apply fallback'
  $disabledHandoffApplyOut = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-Apply') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledHandoffApplyOut -Unexpected 'delegatedByFake' -Label 'go disabled handoff apply fallback'
  Assert-ContainsText -Text $disabledHandoffApplyOut -Expected 'main-latest.md' -Label 'go disabled handoff apply fallback'

  'facade smoke ok'
} finally {
  if (Test-Path -LiteralPath $matrixRoot) { Remove-Item -LiteralPath $matrixRoot -Recurse -Force -Confirm:$false }
}
