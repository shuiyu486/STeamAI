param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string]$Pack = '_template'
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
$RepoRoot = Split-Path -Parent $RekitRoot
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
  $global:LASTEXITCODE = 0
  return $output
}

function Invoke-GoRekitSmoke {
  param([Parameter(Mandatory=$true)][string[]]$Arguments)
  Push-Location $RepoRoot
  try {
    $output = & go run ./cmd/rekit -- @Arguments | Out-String
    if ($LASTEXITCODE -ne 0) { throw "go rekit failed; output:`n$output" }
    return $output
  } finally {
    Pop-Location
  }
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

function Assert-FileEquals {
  param(
    [Parameter(Mandatory=$true)][string]$Left,
    [Parameter(Mandatory=$true)][string]$Right,
    [Parameter(Mandatory=$true)][string]$Label
  )
  $leftHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Left).Hash
  $rightHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Right).Hash
  if ($leftHash -ne $rightHash) { throw "$Label hash mismatch: $Left != $Right" }
}

function Get-WriteItem {
  param(
    [Parameter(Mandatory=$true)]$Result,
    [Parameter(Mandatory=$true)][string]$Path
  )
  $items = @($Result.writes | Where-Object { [string]$_.path -eq $Path })
  if ($items.Count -ne 1) { throw "expected exactly one write item for $Path, got $($items.Count)" }
  return $items[0]
}

function Assert-WriteAction {
  param(
    [Parameter(Mandatory=$true)]$Result,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Action,
    [switch]$RequireBackup
  )
  $item = Get-WriteItem -Result $Result -Path $Path
  if ([string]$item.action -ne $Action) { throw "write action for $Path was '$($item.action)', want '$Action'" }
  if ($RequireBackup) {
    if ([string]::IsNullOrWhiteSpace([string]$item.backupPath)) { throw "write item for $Path did not report a backup" }
    if (-not (Test-Path -LiteralPath ([string]$item.backupPath))) { throw "backup path for $Path does not exist: $($item.backupPath)" }
  }
  return $item
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "init-bootstrap-smoke-$suffix"
$bootstrapRoot = Join-Path $WorkRoot "bootstrap-smoke-$suffix"
$facadeAttachRoot = Join-Path $WorkRoot "facade-attach-$suffix"
$facadeInitRoot = Join-Path $WorkRoot "facade-init-$suffix"
$facadeBootstrapRoot = Join-Path $WorkRoot "facade-bootstrap-$suffix"
$fakeGo = Join-Path $WorkRoot "fake-rekit-go-$suffix.cmd"
try {
  $preview = Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"init-preview-$suffix",'-WhatIf') | ConvertFrom-Json
  if ([bool]$preview.isMutation -or -not [bool]$preview.requiresConfirmation) { throw "unexpected init preview result: $($preview | ConvertTo-Json -Depth 8)" }
  if (Test-Path -LiteralPath $caseRoot) { throw 'init -WhatIf created the target directory' }
  Assert-WriteAction -Result $preview -Path 'references/template/README.md' -Action 'create-managed-file' | Out-Null
  Assert-WriteAction -Result $preview -Path 'references/template/task-handoff.md' -Action 'create-local-template-file' | Out-Null

  $apply = Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"init-apply-$suffix",'-Apply') | ConvertFrom-Json
  if (-not [bool]$apply.applied -or [bool]$apply.isMutation -ne $true -or [string]$apply.command -ne 'init') { throw "unexpected init apply result: $($apply | ConvertTo-Json -Depth 8)" }
  Assert-WriteAction -Result $apply -Path 'references/template/README.md' -Action 'create-managed-file' | Out-Null
  Assert-WriteAction -Result $apply -Path 'references/template/task-handoff.md' -Action 'create-local-template-file' | Out-Null
  Assert-WriteAction -Result $apply -Path 'CLAUDE.local.md' -Action 'create-managed-block-host' | Out-Null
  Assert-WriteAction -Result $apply -Path '.rekit/state.json' -Action 'refresh' | Out-Null

  $readme = Join-Path $caseRoot 'references\template\README.md'
  $handoff = Join-Path $caseRoot 'references\template\task-handoff.md'
  $blockHost = Join-Path $caseRoot 'CLAUDE.local.md'
  Assert-FileEquals -Left $readme -Right (Join-Path $RepoRoot 'packs\_template\references\template\README.md') -Label 'managed README'
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText($handoff, [System.Text.Encoding]::UTF8)) -Expected "init-apply-$suffix" -Label 'template placeholder project name'
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText($handoff, [System.Text.Encoding]::UTF8)) -Expected $caseRoot -Label 'template placeholder project root'
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText($blockHost, [System.Text.Encoding]::UTF8)) -Expected 'Template pack router' -Label 'managed block'
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText((Join-Path $caseRoot '.rekit\state.json'), [System.Text.Encoding]::UTF8)) -Expected 'targetHashAtSync' -Label 'sync state'

  [System.IO.File]::WriteAllText($handoff, "# Local handoff`r`n`r`nkeep on init refresh`r`n", [System.Text.UTF8Encoding]::new($false))
  $refresh = Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"init-refresh-$suffix",'-Apply') | ConvertFrom-Json
  Assert-WriteAction -Result $refresh -Path 'references/template/task-handoff.md' -Action 'skip-existing-local-file' | Out-Null
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText($handoff, [System.Text.Encoding]::UTF8)) -Expected 'keep on init refresh' -Label 'template skip on init refresh'

  $force = Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"init-force-$suffix",'-Apply','-Force') | ConvertFrom-Json
  Assert-WriteAction -Result $force -Path 'references/template/task-handoff.md' -Action 'overwrite-local-template-file-with-force' -RequireBackup | Out-Null
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText($handoff, [System.Text.Encoding]::UTF8)) -Expected "init-force-$suffix" -Label 'forced template placeholder'

  $bootstrap = Invoke-GoRekitSmoke -Arguments @('-Command','bootstrap','-Target',$bootstrapRoot,'-Pack',$Pack,'-ProjectName',"bootstrap-$suffix",'-Apply') | ConvertFrom-Json
  if (-not [bool]$bootstrap.applied -or [string]$bootstrap.command -ne 'bootstrap') { throw "unexpected bootstrap apply result: $($bootstrap | ConvertTo-Json -Depth 8)" }

  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$bootstrapRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$bootstrapRoot,'-Pack',$Pack) | Out-Null

  [System.IO.File]::WriteAllText($fakeGo, ('@echo off' + "`r`n" + 'echo {"schemaVersion":1,"command":"attach","delegatedByFake":true}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $facadeAttachJson = Invoke-RekitSmoke -Arguments @('-Command','attach','-Target',$facadeAttachRoot,'-Pack',$Pack,'-ProjectName',"facade-attach-$suffix",'-WhatIf','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeAttachJson.delegatedByFake) { throw "facade attach JSON preview did not use default REKIT_GO_EXE delegation: $($facadeAttachJson | ConvertTo-Json -Depth 8)" }
  if (Test-Path -LiteralPath $facadeAttachRoot) { throw 'facade attach JSON preview created the target directory' }

  $facadeAttachText = Invoke-RekitSmoke -Arguments @('-Command','attach','-Target',$facadeAttachRoot,'-Pack',$Pack,'-ProjectName',"facade-attach-$suffix",'-WhatIf') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeAttachText.delegatedByFake) { throw "facade attach text preview did not use default REKIT_GO_EXE delegation: $($facadeAttachText | ConvertTo-Json -Depth 8)" }
  if (Test-Path -LiteralPath $facadeAttachRoot) { throw 'facade attach text preview created the target directory' }

  [System.IO.File]::WriteAllText($fakeGo, ('@echo off' + "`r`n" + 'echo {"schemaVersion":1,"command":"repair","delegatedByFake":true}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $facadeRepairJson = Invoke-RekitSmoke -Arguments @('-Command','repair','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeRepairJson.delegatedByFake) { throw "facade repair JSON preview did not use default REKIT_GO_EXE delegation: $($facadeRepairJson | ConvertTo-Json -Depth 8)" }
  $facadeRepairText = Invoke-RekitSmoke -Arguments @('-Command','repair','-Target',$caseRoot,'-Pack',$Pack) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeRepairText.delegatedByFake) { throw "facade repair text preview did not use default REKIT_GO_EXE delegation: $($facadeRepairText | ConvertTo-Json -Depth 8)" }

  [System.IO.File]::WriteAllText($fakeGo, ('@echo off' + "`r`n" + 'echo {"schemaVersion":1,"command":"init","delegatedByFake":true}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $facadeInitJson = Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$facadeInitRoot,'-Pack',$Pack,'-ProjectName',"facade-init-$suffix",'-WhatIf','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeInitJson.delegatedByFake) { throw "facade init JSON preview did not use default REKIT_GO_EXE delegation: $($facadeInitJson | ConvertTo-Json -Depth 8)" }
  if (Test-Path -LiteralPath $facadeInitRoot) { throw 'facade init JSON preview created the target directory' }

  [System.IO.File]::WriteAllText($fakeGo, ('@echo off' + "`r`n" + 'echo {"schemaVersion":1,"command":"bootstrap","delegatedByFake":true}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $facadeBootstrapJson = Invoke-RekitSmoke -Arguments @('-Command','bootstrap','-Target',$facadeBootstrapRoot,'-Pack',$Pack,'-ProjectName',"facade-bootstrap-$suffix",'-WhatIf','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeBootstrapJson.delegatedByFake) { throw "facade bootstrap JSON preview did not use default REKIT_GO_EXE delegation: $($facadeBootstrapJson | ConvertTo-Json -Depth 8)" }
  if (Test-Path -LiteralPath $facadeBootstrapRoot) { throw 'facade bootstrap JSON preview created the target directory' }

  $facadeInitApply = Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$facadeInitRoot,'-Pack',$Pack,'-ProjectName',"facade-init-$suffix",'-Apply') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeInitApply.delegatedByFake) { throw "facade init apply did not use default REKIT_GO_EXE delegation: $($facadeInitApply | ConvertTo-Json -Depth 8)" }
  if (Test-Path -LiteralPath $facadeInitRoot) { throw 'facade init apply fake delegation created the target directory' }

  $facadeOut = Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$facadeInitRoot,'-Pack',$Pack,'-ProjectName',"facade-init-$suffix",'-WhatIf') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeOut.delegatedByFake) { throw "facade init text preview did not use default REKIT_GO_EXE delegation: $($facadeOut | ConvertTo-Json -Depth 8)" }

  $disabledAttachOut = Invoke-RekitSmoke -Arguments @('-Command','attach','-Target',(Join-Path $WorkRoot "disabled-attach-$suffix"),'-Pack',$Pack,'-ProjectName',"disabled-attach-$suffix",'-WhatIf') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledAttachOut -Unexpected 'delegatedByFake' -Label 'disabled attach no fallback'
  Assert-ContainsText -Text $disabledAttachOut -Expected 'PowerShell fallback has been retired' -Label 'disabled attach no fallback'

  $disabledRepairOut = Invoke-RekitSmoke -Arguments @('-Command','repair','-Target',$caseRoot,'-Pack',$Pack) -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledRepairOut -Unexpected 'delegatedByFake' -Label 'disabled repair no fallback'
  Assert-ContainsText -Text $disabledRepairOut -Expected 'PowerShell fallback has been retired' -Label 'disabled repair no fallback'

  $disabledInitOut = Invoke-RekitSmoke -Arguments @('-Command','init','-Target',(Join-Path $WorkRoot "disabled-init-$suffix"),'-Pack',$Pack,'-ProjectName',"disabled-init-$suffix",'-WhatIf') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledInitOut -Unexpected 'delegatedByFake' -Label 'disabled init no fallback'
  Assert-ContainsText -Text $disabledInitOut -Expected 'PowerShell fallback has been retired' -Label 'disabled init no fallback'

  $disabledBootstrapOut = Invoke-RekitSmoke -Arguments @('-Command','bootstrap','-Target',(Join-Path $WorkRoot "disabled-bootstrap-$suffix"),'-Pack',$Pack,'-ProjectName',"disabled-bootstrap-$suffix",'-Apply') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledBootstrapOut -Unexpected 'delegatedByFake' -Label 'disabled bootstrap no fallback'
  Assert-ContainsText -Text $disabledBootstrapOut -Expected 'PowerShell fallback has been retired' -Label 'disabled bootstrap no fallback'

  'init/bootstrap smoke ok'
} finally {
  foreach ($path in @($caseRoot,$bootstrapRoot,$facadeAttachRoot,$facadeInitRoot,$facadeBootstrapRoot,$fakeGo)) {
    if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
