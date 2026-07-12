param(
  [string]$CaseRoot = 'C:\AI\m_projects\RE\_dryrun_cases\agent-team-dryrun',
  [string]$Pack = 'vmp-re'
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
$matrixRoot = Join-Path $env:TEMP "rekit-facade-matrix-$suffix"
New-Item -ItemType Directory -Path $matrixRoot -Force | Out-Null
$fakeGo = Join-Path $matrixRoot 'fake-rekit-go.cmd'

# Default path stays PowerShell and does not expose gate through the facade.
$out = Invoke-RekitSmoke -Arguments @('-Command','status')
Assert-ContainsText -Text $out -Expected 'rekit runtime:' -Label 'default status'

$gateOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane','feature-handler-0x40a010') -AllowedExitCodes @(1)
Assert-ContainsText -Text $gateOut -Expected 'gate is implemented by the Go backend only' -Label 'default gate guard'

# Explicit enable delegates only the safe set.
$goEnv = @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }
$out = Invoke-RekitSmoke -Arguments @('-Command','status') -Env $goEnv
Assert-ContainsText -Text $out -Expected 'rekit go backend:' -Label 'go status'

$caseDoctor = Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$CaseRoot,'-Pack',$Pack) -Env $goEnv
Assert-ContainsText -Text $caseDoctor -Expected 'instance validation ok' -Label 'go case doctor'

$reviewRoot = Join-Path $env:TEMP 'rekit-facade-smoke-sync'
if (Test-Path -LiteralPath $reviewRoot) { Remove-Item -LiteralPath $reviewRoot -Recurse -Force -Confirm:$false }
$syncOut = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-ReviewOutputDir',$reviewRoot) -Env $goEnv
Assert-ContainsText -Text $syncOut -Expected '"writesArtifacts": true' -Label 'go sync review'
if (-not (Test-Path -LiteralPath (Join-Path $reviewRoot 'packet.json'))) { throw 'sync review packet was not written' }
if (-not (Test-Path -LiteralPath (Join-Path $reviewRoot 'diffs\combined.diff'))) { throw 'sync review combined diff was not written' }

$gateOut = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$CaseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane','feature-handler-0x40a010','-Subject','facade smoke gate') -Env $goEnv
Assert-ContainsText -Text $gateOut -Expected '"isMutation": false' -Label 'go gate dry-run'
Assert-ContainsText -Text $gateOut -Expected '"status": "pending-gate"' -Label 'go gate dry-run'

# Only JSON sync apply preview is delegated; text preview still emits PowerShell would-* text.
$syncApplyJson = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -Env $goEnv
Assert-ContainsText -Text $syncApplyJson -Expected '"isMutation": false' -Label 'go sync apply JSON preview'
Assert-ContainsText -Text $syncApplyJson -Expected '"applied": false' -Label 'go sync apply JSON preview'

$syncApplyOut = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Env $goEnv
Assert-ContainsText -Text $syncApplyOut -Expected 'would attach case' -Label 'sync write fallback'

# Fake backend matrix proves the JSON preview/read-only safe set is delegated and write/text paths are not.
$attachRoot = Join-Path $matrixRoot 'attach-preview-case'
Assert-FakeDelegation -Arguments @('-Command','attach','-Target',$attachRoot,'-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'attach' -Label 'attach JSON preview delegation'
Assert-FakeFallback -Arguments @('-Command','attach','-Target',$attachRoot,'-Pack',$Pack,'-WhatIf') -Expected 'would attach case' -Label 'attach text preview fallback'

Assert-FakeDelegation -Arguments @('-Command','repair','-Target',$CaseRoot,'-Pack',$Pack,'-Format','json') -CommandName 'repair' -Label 'repair JSON preview delegation'
Assert-FakeFallback -Arguments @('-Command','repair','-Target',$CaseRoot,'-Pack',$Pack) -Expected 'repair target:' -Label 'repair text preview fallback'

$initRoot = Join-Path $matrixRoot 'init-preview-case'
$bootstrapRoot = Join-Path $matrixRoot 'bootstrap-preview-case'
Assert-FakeDelegation -Arguments @('-Command','init','-Target',$initRoot,'-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'init' -Label 'init JSON preview delegation'
Assert-FakeDelegation -Arguments @('-Command','bootstrap','-Target',$bootstrapRoot,'-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'bootstrap' -Label 'bootstrap JSON preview delegation'
Assert-FakeFallback -Arguments @('-Command','init','-Target',$initRoot,'-Pack',$Pack,'-WhatIf') -Expected 'would attach case' -Label 'init text preview fallback'

Assert-FakeDelegation -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -CommandName 'sync' -Label 'sync apply JSON preview delegation'
Assert-FakeFallback -Arguments @('-Command','sync','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Expected 'would attach case' -Label 'sync apply text preview fallback'

Assert-FakeDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf','-Format','json') -CommandName 'promote' -Label 'promote candidate JSON preview delegation'
Assert-FakeDelegation -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -CommandName 'promote' -Label 'promote apply JSON preview delegation'
Assert-FakeFallback -Arguments @('-Command','promote','-Target',$CaseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Expected 'promote summary:' -Label 'promote apply text preview fallback'

Assert-FakeDelegation -Arguments @('-Command','overview','-Target',$CaseRoot,'-Pack',$Pack,'-Format','json') -CommandName 'overview' -Label 'overview JSON delegation'
Assert-FakeDelegation -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List','-Format','json') -CommandName 'note' -Label 'note list JSON delegation'
Assert-FakeFallback -Arguments @('-Command','note','-Target',$CaseRoot,'-Pack',$Pack,'-List') -Expected '[observation]' -Label 'note text list fallback'

Assert-FakeDelegation -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-lane','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'start' -Label 'start JSON preview delegation'
Assert-FakeDelegation -Arguments @('-Command','handoff','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'handoff' -Label 'handoff JSON preview delegation'
Assert-FakeDelegation -Arguments @('-Command','continue','-Target',$CaseRoot,'main','-Pack',$Pack,'-WhatIf','-Format','json') -CommandName 'continue' -Label 'continue JSON preview delegation'
Assert-FakeFallback -Arguments @('-Command','start','-Target',$CaseRoot,'matrix-lane','-Pack',$Pack,'-WhatIf') -Expected 'would create or enter feature workstream:' -Label 'start text preview fallback'

# Disable wins over enable.
$disabledOut = Invoke-RekitSmoke -Arguments @('-Command','status') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '1' }
Assert-ContainsText -Text $disabledOut -Expected 'rekit runtime:' -Label 'go disabled status fallback'

if (Test-Path -LiteralPath $matrixRoot) { Remove-Item -LiteralPath $matrixRoot -Recurse -Force -Confirm:$false }

'facade smoke ok'
