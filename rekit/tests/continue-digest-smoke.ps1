param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string]$Pack = '_template'
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
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

function Assert-ContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Expected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text -notlike "*$Expected*") { throw "$Label missing expected text '$Expected'. Output:`n$Text" }
}

function TextFromCodes {
  param([Parameter(Mandatory=$true)][int[]]$Codes)
  return (-join ($Codes | ForEach-Object { [char]$_ }))
}

function Write-Utf8File {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [AllowEmptyString()][string]$Text = ''
  )
  $parent = Split-Path -Parent $Path
  if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
  [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "continue-digest-$suffix"
try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"continue-digest-$suffix") | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,"digest-$suffix") | Out-Null

  $board = Get-Content -LiteralPath (Join-Path $caseRoot '.rekit\board.json') -Raw | ConvertFrom-Json
  $lane = @($board.lanes | Where-Object { -not [bool]$_.authority } | Select-Object -Last 1)
  $laneId = [string]$lane.id
  if ([string]::IsNullOrWhiteSpace($laneId)) { throw 'start did not create a feature lane' }

  $laneRoot = Join-Path $caseRoot ('.rekit\lanes\' + $laneId)
  $workspace = Join-Path $caseRoot ([string]$lane.workspace)
  $packetRel = ([string]$lane.workspace).Replace('\\','/') + '/review-packet.md'
  Write-Utf8File -Path (Join-Path $workspace 'review-packet.md') -Text "# Review packet`r`n"
  Write-Utf8File -Path (Join-Path $laneRoot 'outbox.jsonl') -Text (@(
    '{"kind":"observation","subject":"digest observation","summary":"shared fact","evidence":"evidence-digest"}',
    '{"kind":"candidate","subject":"digest candidate","summary":"needs review","confidence":"medium"}'
  ) -join "`r`n" + "`r`n")

  $continueOut = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,"digest-$suffix")
  Assert-ContainsText -Text $continueOut -Expected '.rekit/runs/' -Label 'continue output digest path'
  $match = [regex]::Match($continueOut, '\.rekit/runs/[^\s]+/digest\.md')
  if (-not $match.Success) { throw "missing digest path in continue output:`n$continueOut" }
  $digestRel = $match.Value
  $digestPath = Join-Path $caseRoot ($digestRel -replace '/', '\')
  if (-not (Test-Path -LiteralPath $digestPath)) { throw "missing digest: $digestPath" }
  $digest = [System.IO.File]::ReadAllText($digestPath, [System.Text.Encoding]::UTF8)

  $sectionInput = '## ' + (TextFromCodes @(36755,20837))
  $sectionAuto = '## ' + (TextFromCodes @(33258,21160,22788,29702))
  $sectionAttention = '## ' + (TextFromCodes @(38656,35201,20851,27880))
  foreach ($expected in @($sectionInput,'case:','pack:','runId:','batchId:','focus lane:',"$laneId",'## route','focus lane','## packet refs',$packetRel,'## inputs','outbox.jsonl','## outputs','collected: 2','observations: 1','candidates: 1','pendingUser: 1','## decisions','digest observation','decision=accept','digest candidate','decision=defer','## open risks','candidate lacks evidence or policy disabled',$sectionAuto,$sectionAttention)) {
    Assert-ContainsText -Text $digest -Expected $expected -Label 'continue digest'
  }

  $statusPath = Join-Path (Split-Path -Parent $digestPath) 'status.json'
  if (-not (Test-Path -LiteralPath $statusPath)) { throw "missing run status: $statusPath" }
  $status = Get-Content -LiteralPath $statusPath -Raw | ConvertFrom-Json
  if ([string]::IsNullOrWhiteSpace([string]$status.batchId)) { throw "status.json missing batchId: $($status | ConvertTo-Json -Depth 10)" }
  if (@($status.inputs).Count -lt 1 -or @($status.packetRefs).Count -lt 1 -or @($status.openRisks).Count -lt 1) { throw "status.json missing digest fields: $($status | ConvertTo-Json -Depth 10)" }

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  'continue digest smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
