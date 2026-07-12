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

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "agent-team-d5-dryrun-$suffix"

try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"d5-dryrun-$suffix") | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,"d5-$suffix") | Out-Null

  $board = Get-Content -LiteralPath (Join-Path $caseRoot '.rekit\board.json') -Raw | ConvertFrom-Json
  $laneObj = @($board.lanes | Where-Object { -not [bool]$_.authority } | Select-Object -Last 1)
  $lane = [string]$laneObj.id
  if ([string]::IsNullOrWhiteSpace($lane)) { throw 'start did not create a feature lane for D5 dry-run smoke' }

  $candidate = "mock-candidate-$suffix"
  $evidence = "ev-d5-$suffix"
  $batch = "batch-d5-$suffix"
  $intervention = "mock-intervention-$suffix"
  $rollback = "mock-rollback-$suffix"

  Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','candidate','-Lane',$lane,'-Subject',$candidate,'-Summary','mock candidate from non-sensitive dry-run','-Actor','feature-d5-smoke','-Confidence','medium','-Status','open','-Risk','low','-BatchId',$batch,'-EvidenceRefs',$evidence) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','verification','-Lane',$lane,'-Subject',$candidate,'-Summary','reviewer accepted mock candidate','-Actor','reviewer-d5-smoke','-TargetRef',$candidate,'-Verifier','manual-review','-Verdict','accepted','-BatchId',$batch,'-EvidenceRefs',$evidence) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','decision','-Lane',$lane,'-Subject',$candidate,'-Summary','main accepted mock reviewer verdict','-Actor','main-d5-smoke','-TargetRef',$candidate,'-Decision','accept','-Status','accepted','-Reason','accepted mock reviewer verdict','-BatchId',$batch,'-EvidenceRefs',$evidence) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','intervention','-Lane',$lane,'-Subject',$intervention,'-Summary','record mock intervention for dry-run closure','-Actor','main-d5-smoke','-TargetRef',$batch,'-Action','rollback','-ApprovedBy','d5-smoke','-Scope','mock case only','-Status','open','-Reason','exercise intervention display without side effects','-BatchId',$batch,'-EvidenceRefs',$evidence) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','rollback','-Lane',$lane,'-Subject',$rollback,'-Summary','record mock rollback note','-Actor','main-d5-smoke','-TargetRef',$batch,'-Status','resolved','-Reason','mock rollback verified no template pollution','-BatchId',$batch,'-EvidenceRefs',$evidence) | Out-Null

  $candidateList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','candidate','-Lane',$lane)
  foreach ($expected in @($candidate,'confidence=medium','status=open','risk=low',"batch=$batch")) {
    Assert-ContainsText -Text $candidateList -Expected $expected -Label 'note candidate list'
  }

  $verificationList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','verification','-Lane',$lane)
  foreach ($expected in @($candidate,'verifier=manual-review','verdict=accepted',"target=$candidate","batch=$batch")) {
    Assert-ContainsText -Text $verificationList -Expected $expected -Label 'note verification list'
  }

  $decisionList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','decision','-Lane',$lane)
  foreach ($expected in @($candidate,'decision=accept','by=main-d5-smoke',"batch=$batch")) {
    Assert-ContainsText -Text $decisionList -Expected $expected -Label 'note decision list'
  }

  $interventionList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','intervention','-Lane',$lane)
  foreach ($expected in @($intervention,'action=rollback',"target=$batch",'approvedBy=d5-smoke','scope=mock case only','status=open','reason=exercise intervention display without side effects',"batch=$batch")) {
    Assert-ContainsText -Text $interventionList -Expected $expected -Label 'note intervention list'
  }

  $rollbackList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','rollback','-Lane',$lane)
  foreach ($expected in @($rollback,"target=$batch",'status=resolved','reason=mock rollback verified no template pollution',"batch=$batch")) {
    Assert-ContainsText -Text $rollbackList -Expected $expected -Label 'note rollback list'
  }

  $allJson = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Lane',$lane,'-Format','json') | ConvertFrom-Json
  if ([string]$allJson.command -ne 'note' -or [bool]$allJson.isMutation -or [int]$allJson.eventCount -ne 5) { throw "unexpected note list JSON: $($allJson | ConvertTo-Json -Depth 20)" }
  $listedKinds = @($allJson.groups | ForEach-Object { [string]$_.kind })
  foreach ($kind in @('candidate','verification','decision','intervention','rollback')) {
    if ($listedKinds -notcontains $kind) { throw "note list JSON missing kind ${kind}: $($allJson | ConvertTo-Json -Depth 20)" }
  }

  $overview = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack)
  foreach ($expected in @($candidate,'confidence=medium','verifier=manual-review','verdict=accepted',"target=$candidate",'decision=accept','by=main-d5-smoke',"batch=$batch",'candidate=1','verification=1','decision=1','intervention=1','rollback=1',$intervention,'action=rollback',"target=$batch",'status=open',$rollback,'status=resolved')) {
    Assert-ContainsText -Text $overview -Expected $expected -Label 'overview D5 dry-run display'
  }

  Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,$lane) | Out-Null
  $handoffPath = Join-Path $caseRoot ('.rekit\handovers\' + $lane + '-latest.md')
  if (-not (Test-Path -LiteralPath $handoffPath)) { throw "missing lane handoff: $handoffPath" }
  $handoff = [System.IO.File]::ReadAllText($handoffPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('## verification',$candidate,'verifier=manual-review','verdict=accepted',"target=$candidate",'## decision','decision=accept','by=main-d5-smoke','## intervention',$intervention,'action=rollback',"target=$batch",'approvedBy=d5-smoke','scope=mock case only','status=open','## rollback',$rollback,'status=resolved','reason=mock rollback verified no template pollution')) {
    Assert-ContainsText -Text $handoff -Expected $expected -Label 'handoff D5 dry-run display'
  }

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  'agent-team D5 dry-run smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
