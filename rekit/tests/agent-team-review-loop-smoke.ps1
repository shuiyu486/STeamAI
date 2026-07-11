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

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "agent-team-review-loop-$suffix"
$reviewRoot = Join-Path $WorkRoot "agent-team-review-loop-review-$suffix"
try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"review-loop-$suffix") | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,"review-$suffix") | Out-Null

  $board = Get-Content -LiteralPath (Join-Path $caseRoot '.rekit\board.json') -Raw | ConvertFrom-Json
  $laneObj = @($board.lanes | Where-Object { -not [bool]$_.authority } | Select-Object -Last 1)
  $lane = [string]$laneObj.id
  if ([string]::IsNullOrWhiteSpace($lane)) { throw 'start did not create a feature lane' }

  $planOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','feature-analysis','-Items','candidate-alpha','-ReviewOutputDir',$reviewRoot)
  Assert-ContainsText -Text $planOut -Expected 'review packet:' -Label 'plan-subagents output'
  $packetPath = Join-Path $reviewRoot 'packet.json'
  if (-not (Test-Path -LiteralPath $packetPath)) { throw "missing review packet: $packetPath" }
  $packet = Get-Content -LiteralPath $packetPath -Raw | ConvertFrom-Json
  if ([string]$packet.route.id -ne 'vmp-re:lane-feature-analysis') { throw "unexpected route: $($packet | ConvertTo-Json -Depth 10)" }
  foreach ($field in @('tier_used','tool_scope')) {
    Assert-ContainsText -Text ([string]$packet.outputContract) -Expected $field -Label 'route outputContract'
  }
  if (@($packet.shards).Count -ne 1 -or [string]@($packet.shards)[0].items[0] -ne 'candidate-alpha') { throw "unexpected plan shards: $($packet | ConvertTo-Json -Depth 10)" }

  $candidate = [string]@($packet.shards)[0].items[0]
  $evidence = "ev-review-loop-$suffix"
  Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','verification','-Lane',$lane,'-Subject',$candidate,'-Summary','reviewer accepted packet shard','-Actor','reviewer-smoke','-TargetRef',$candidate,'-Verifier','manual-review','-Verdict','accepted','-EvidenceRefs',$evidence) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','decision','-Lane',$lane,'-Subject',$candidate,'-Summary','main accepted reviewer verdict','-Actor','main-smoke','-TargetRef',$candidate,'-Decision','accept','-Status','accepted','-Reason','reviewer verdict accepted','-EvidenceRefs',$evidence) | Out-Null

  $verificationList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','verification','-Lane',$lane)
  foreach ($expected in @($candidate,'verifier=manual-review','verdict=accepted',"target=$candidate")) {
    Assert-ContainsText -Text $verificationList -Expected $expected -Label 'note verification list'
  }

  $decisionList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','decision','-Lane',$lane)
  foreach ($expected in @($candidate,'decision=accept','by=main-smoke')) {
    Assert-ContainsText -Text $decisionList -Expected $expected -Label 'note decision list'
  }

  $overview = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack)
  $sectionVerification = TextFromCodes @(26368,36817,32,118,101,114,105,102,105,99,97,116,105,111,110,65306)
  foreach ($expected in @($sectionVerification,$candidate,'verifier=manual-review','verdict=accepted',"target=$candidate",'decision=accept','by=main-smoke')) {
    Assert-ContainsText -Text $overview -Expected $expected -Label 'overview review loop display'
  }

  Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,$lane) | Out-Null
  $handoffPath = Join-Path $caseRoot ('.rekit\handovers\' + $lane + '-latest.md')
  if (-not (Test-Path -LiteralPath $handoffPath)) { throw "missing handoff: $handoffPath" }
  $handoff = [System.IO.File]::ReadAllText($handoffPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('## verification',$candidate,'verifier=manual-review','verdict=accepted',"target=$candidate",'decision=accept','by=main-smoke')) {
    Assert-ContainsText -Text $handoff -Expected $expected -Label 'handoff review loop display'
  }

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  'agent-team review loop smoke ok'
} finally {
  foreach ($path in @($caseRoot,$reviewRoot)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
