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

# Disable wins over enable.
$disabledOut = Invoke-RekitSmoke -Arguments @('-Command','status') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '1' }
Assert-ContainsText -Text $disabledOut -Expected 'rekit runtime:' -Label 'go disabled status fallback'

'facade smoke ok'
