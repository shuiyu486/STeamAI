param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string]$Pack = 'vmp-re'
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
$RepoRoot = Split-Path -Parent $RekitRoot
$Rekit = Join-Path $RekitRoot 'rekit.ps1'

function Invoke-RekitSmoke {
  param(
    [Parameter(Mandatory=$true)][string[]]$Arguments,
    [int[]]$AllowedExitCodes = @(0)
  )
  $oldEap = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $Rekit @Arguments 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
  } finally {
    $ErrorActionPreference = $oldEap
  }
  if ($AllowedExitCodes -notcontains $exitCode) {
    throw "unexpected exit code $exitCode; output:`n$output"
  }
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

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "gate-parity-smoke-$suffix"
try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"gate-parity-$suffix") | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  $board = Get-Content -LiteralPath (Join-Path $caseRoot '.rekit\board.json') -Raw | ConvertFrom-Json
  $lane = [string]@($board.lanes)[0].id
  if ([string]::IsNullOrWhiteSpace($lane)) { throw 'init did not create a lane for gate parity smoke' }

  $subject = "gate parity smoke $suffix"
  $batch = "batch-gate-parity-$suffix"
  Invoke-GoRekitSmoke -Arguments @(
    '-Command','gate',
    '-Target',$caseRoot,
    '-Pack',$Pack,
    '-Apply',
    '-Action','debug',
    '-Lane',$lane,
    '-Actor','gate-parity-smoke',
    '-Risk','high',
    '-Subject',$subject,
    '-Summary','verify gate request display parity',
    '-TargetRef',$batch,
    '-BatchId',$batch,
    '-Scope','handler only',
    '-Budget','30s',
    '-TriedLightSteps','overview,static review',
    '-StopConditions','timeout,unexpected side effect'
  ) | Out-Null

  $overview = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack)
  foreach ($expected in @($subject,'by=gate-parity-smoke','risk=high',"target=$batch","batch=$batch",'action=debug','scope=handler only','budget=30s','tried=overview,static review','stop=timeout,unexpected side effect')) {
    Assert-ContainsText -Text $overview -Expected $expected -Label 'overview pending-gate'
  }

  $list = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','request','-Lane',$lane)
  foreach ($expected in @($subject,'status=pending-gate','by=gate-parity-smoke','risk=high',"target=$batch","batch=$batch",'action=debug','scope=handler only','budget=30s')) {
    Assert-ContainsText -Text $list -Expected $expected -Label 'note -List request'
  }

  Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,$lane) | Out-Null
  $handoffPath = Join-Path $caseRoot ('.rekit\handovers\' + $lane + '-latest.md')
  if (-not (Test-Path -LiteralPath $handoffPath)) { throw "handoff file was not written: $handoffPath" }
  $handoff = [System.IO.File]::ReadAllText($handoffPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @($subject,'by=gate-parity-smoke','risk=high',"target=$batch","batch=$batch",'action=debug','scope=handler only','budget=30s')) {
    Assert-ContainsText -Text $handoff -Expected $expected -Label 'handoff pending-gate'
  }

  'gate parity smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
