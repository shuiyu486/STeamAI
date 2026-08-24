param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

$Pack = 'binary-re'
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
  if (-not $Text.Contains($Expected)) { throw "$Label missing expected text '$Expected'. Output:`n$Text" }
}

function Assert-NotContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Unexpected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text.Contains($Unexpected)) { throw "$Label contained unexpected text '$Unexpected'. Output:`n$Text" }
}

function Write-Utf8File {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text
  )
  $parent = Split-Path -Parent $Path
  if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
  [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Read-JsonFile {
  param([Parameter(Mandatory=$true)][string]$Path)
  Add-Type -AssemblyName System.Web.Extensions
  $serializer = [System.Web.Script.Serialization.JavaScriptSerializer]::new()
  $serializer.MaxJsonLength = [int]::MaxValue
  $serializer.RecursionLimit = 512
  return $serializer.DeserializeObject([System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8))
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "binary-re-agent-team-dryrun-$suffix"
$reviewRoot = Join-Path $WorkRoot "binary-re-agent-team-review-$suffix"

try {
  $initPreview = Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"binary-re-dryrun-$suffix",'-WhatIf','-Format','json') | ConvertFrom-Json
  $initApplyArgs = @($initPreview.applyArgs | ForEach-Object { [string]$_ })
  if ([string]::IsNullOrWhiteSpace([string]$initPreview.expectedPlanSha256) -or $initApplyArgs.Count -eq 0) { throw 'binary-re dry-run init preview omitted exact Apply action' }
  Invoke-RekitSmoke -Arguments $initApplyArgs | Out-Null

  $start = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','sample','-Apply','-Format','json') | ConvertFrom-Json
  if ([string]$start.command -ne 'start' -or -not [bool]$start.isMutation -or -not [bool]$start.applied -or [string]$start.lane.id -ne 'binary-analysis-sample' -or [string]$start.lane.type -ne 'binary-analysis' -or [string]$start.lane.workspace -ne 'captures/binary_analysis/binary-analysis-sample') {
    throw "unexpected binary-re start result: $($start | ConvertTo-Json -Depth 20)"
  }

  $packetRel = 'captures\binary_analysis\binary-analysis-sample\packet.md'
  Write-Utf8File -Path (Join-Path $caseRoot $packetRel) -Text "# packet`r`n`r`ngeneric binary analysis packet for dry-run only; no sample execution, debug, trace, dump, or patch`r`n"

  $planOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','binary-analysis','-Items','function-init,string-login','-ItemsPerAgent','1','-MaxParallel','2','-ReviewOutputDir',$reviewRoot)
  Assert-ContainsText -Text $planOut -Expected '"command": "plan-subagents"' -Label 'binary-re plan-subagents default Go output'
  $packetPath = Join-Path $reviewRoot 'packet.json'
  if (-not (Test-Path -LiteralPath $packetPath)) { throw "missing binary-re review packet: $packetPath" }
  $packet = Read-JsonFile -Path $packetPath
  if ([string]$packet.route.id -ne 'binary-re:binary-analysis' -or [string]$packet.observability.routeDebug.selectedBy -ne 'taskType') {
    throw "unexpected binary-re route packet: $($packet | ConvertTo-Json -Depth 20)"
  }
  if ([string]$packet.observability.dispatchMode -ne 'manual-main-agent' -or @($packet.observability.shardStatuses).Count -ne 2 -or [string]@($packet.observability.shardStatuses)[0].status -ne 'planned' -or [string]$packet.reviewLoop.spawnOwner -ne 'main-agent') {
    throw "unexpected binary-re dispatch observability: $($packet | ConvertTo-Json -Depth 20)"
  }
  foreach ($expected in @('runtime does not spawn subagents','canonical-write')) {
    Assert-ContainsText -Text (($packet | ConvertTo-Json -Depth 20)) -Expected $expected -Label 'binary-re dispatch contract'
  }

  $candidate = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','candidate','-Lane','binary-analysis-sample','-Subject','binary behavior candidate','-Summary','candidate awaiting bounded generic binary review','-Actor','binary-agent','-Confidence','high','-Status','open','-Risk','medium','-TargetRef','function-init','-BatchId','batch-generic-real-dryrun','-EvidenceRefs','captures/binary_analysis/binary-analysis-sample/packet.md') | ConvertFrom-Json
  if ([string]$candidate.command -ne 'note' -or -not [bool]$candidate.applied -or [string]$candidate.path -ne '.steamai/facts/candidates.jsonl' -or [string]$candidate.event.kind -ne 'candidate' -or [string]$candidate.event.lane -ne 'binary-analysis-sample') {
    throw "unexpected binary-re candidate append: $($candidate | ConvertTo-Json -Depth 20)"
  }

  $requestsPath = Join-Path $caseRoot '.steamai\facts\requests.jsonl'
  $beforeRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  $gatePreview = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane','binary-analysis-sample','-Actor','runtime-test','-Subject','generic binary debug gate','-Summary','needs user confirmation before interactive binary debugging','-TargetRef','function-init','-BatchId','batch-generic-real-dryrun','-Scope','function behavior only','-Budget','30s','-TriedLightSteps','plan-subagents,static triage','-StopConditions','timeout') | ConvertFrom-Json
  if ([string]$gatePreview.command -ne 'gate' -or [bool]$gatePreview.isMutation -or -not [bool]$gatePreview.requiresConfirmation -or [string]$gatePreview.eventPreview.status -ne 'pending-gate' -or [string]$gatePreview.eventPreview.gate.action -ne 'debug') {
    throw "unexpected binary-re gate preview: $($gatePreview | ConvertTo-Json -Depth 20)"
  }
  $afterPreviewRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  if ($beforeRequests -ne $afterPreviewRequests) { throw 'binary-re gate what-if changed requests ledger' }

  $gateApply = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Action','debug','-Lane','binary-analysis-sample','-Actor','runtime-test','-Risk','medium','-Subject','generic binary debug gate','-Summary','needs user confirmation before interactive binary debugging','-TargetRef','function-init','-BatchId','batch-generic-real-dryrun','-Scope','function behavior only','-Budget','30s','-TriedLightSteps','plan-subagents,static triage','-StopConditions','timeout') | ConvertFrom-Json
  if ([string]$gateApply.command -ne 'gate' -or -not [bool]$gateApply.isMutation -or -not [bool]$gateApply.applied -or [string]$gateApply.event.status -ne 'pending-gate' -or [string]$gateApply.event.gate.action -ne 'debug' -or [string]$gateApply.event.gate.scope -ne 'function behavior only') {
    throw "unexpected binary-re gate apply: $($gateApply | ConvertTo-Json -Depth 20)"
  }

  $verification = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','verification','-Lane','binary-analysis-sample','-Subject','bounded binary review','-Summary','bounded reviewer accepted generic binary evidence','-Actor','reviewer-smoke','-Verifier','tool-review','-Verdict','accepted','-TargetRef','function-init','-BatchId','batch-generic-real-dryrun','-EvidenceRefs','captures/binary_analysis/binary-analysis-sample/packet.md') | ConvertFrom-Json
  if ([string]$verification.command -ne 'note' -or -not [bool]$verification.applied -or [string]$verification.event.verifier -ne 'tool-review' -or [string]$verification.event.verdict -ne 'accepted') {
    throw "unexpected binary-re verification append: $($verification | ConvertTo-Json -Depth 20)"
  }

  $decision = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','decision','-Lane','binary-analysis-sample','-Subject','generic binary merge decision','-Summary','main accepted reviewed generic binary candidate','-Actor','main-smoke','-Decision','accept','-Status','accepted','-Reason','reviewer accepted generic binary evidence','-TargetRef','function-init','-BatchId','batch-generic-real-dryrun') | ConvertFrom-Json
  if ([string]$decision.command -ne 'note' -or -not [bool]$decision.applied -or [string]$decision.event.decision -ne 'accept') {
    throw "unexpected binary-re decision append: $($decision | ConvertTo-Json -Depth 20)"
  }

  $requestList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','request','-Lane','binary-analysis-sample')
  foreach ($expected in @('generic binary debug gate','status=pending-gate','by=runtime-test','risk=medium','action=debug','scope=function behavior only','budget=30s','batch=batch-generic-real-dryrun')) {
    Assert-ContainsText -Text $requestList -Expected $expected -Label 'binary-re request note list'
  }
  $verificationList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','verification','-Lane','binary-analysis-sample')
  foreach ($expected in @('bounded binary review','verifier=tool-review','verdict=accepted','target=function-init','batch=batch-generic-real-dryrun')) {
    Assert-ContainsText -Text $verificationList -Expected $expected -Label 'binary-re verification note list'
  }

  $overviewJson = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') | ConvertFrom-Json
  if ([string]$overviewJson.command -ne 'overview' -or [bool]$overviewJson.isMutation -or [string]$overviewJson.pack -ne $Pack -or [int]$overviewJson.counts.candidates -ne 1 -or [int]$overviewJson.counts.requests -ne 1 -or [int]$overviewJson.sections.pendingGates.total -ne 1 -or [int]$overviewJson.sections.verifications.total -ne 1 -or [int]$overviewJson.sections.decisions.total -ne 1 -or [int]$overviewJson.sections.batches.total -ne 1) {
    throw "unexpected binary-re overview JSON: $($overviewJson | ConvertTo-Json -Depth 20)"
  }

  $overview = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack)
  foreach ($expected in @('binary-analysis-sample','binary behavior candidate','generic binary debug gate','action=debug','scope=function behavior only','verifier=tool-review','verdict=accepted','decision=accept','reviewer accepted generic binary evidence','batch-generic-real-dryrun')) {
    Assert-ContainsText -Text $overview -Expected $expected -Label 'binary-re overview text'
  }

  $handoffPreview = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','-Format','json','sample') | ConvertFrom-Json
  $handoffApplyArgs = @($handoffPreview.applyArgs | ForEach-Object { [string]$_ })
  if ([string]::IsNullOrWhiteSpace([string]$handoffPreview.publicationPlanSha256) -or [string]::IsNullOrWhiteSpace([string]$handoffPreview.publicationStamp) -or $handoffApplyArgs.Count -eq 0) { throw 'binary-re handoff preview omitted exact Apply action' }
  $handoff = Invoke-RekitSmoke -Arguments $handoffApplyArgs | ConvertFrom-Json
  if ([string]$handoff.command -ne 'handoff' -or -not [bool]$handoff.isMutation -or -not [bool]$handoff.applied -or [bool]$handoff.project -or [string]$handoff.lane.id -ne 'binary-analysis-sample' -or [string]$handoff.lane.workspace -ne 'captures/binary_analysis/binary-analysis-sample') {
    throw "unexpected binary-re handoff result: $($handoff | ConvertTo-Json -Depth 20)"
  }
  if (-not ([string]$handoff.missionCommanderActionQueue.currentAction.command).StartsWith('/steamai ')) {
    throw "current binary-re handoff JSON did not project its typed command to /steamai: $($handoff.missionCommanderActionQueue.currentAction | ConvertTo-Json -Depth 10)"
  }
  $handoffPath = Join-Path $caseRoot '.steamai\handovers\binary-analysis-sample-latest.md'
  if (-not (Test-Path -LiteralPath $handoffPath)) { throw "missing binary-re handoff: $handoffPath" }
  $handoffText = [System.IO.File]::ReadAllText($handoffPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('binary-analysis-sample','`/rekit continue sample`','captures/binary_analysis/binary-analysis-sample/packet.md','## verification','verifier=tool-review','## decision','decision=accept','## pending-gate','action=debug','scope=function behavior only')) {
    Assert-ContainsText -Text $handoffText -Expected $expected -Label 'binary-re handoff text'
  }
  foreach ($unexpected in @('web-security','workspace/features','feature-authz','endpoint-login','action=network','references/template','vmp-re','generic-binary-re')) {
    Assert-NotContainsText -Text $handoffText -Unexpected $unexpected -Label 'binary-re handoff leakage guard'
  }

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  'binary-re agent-team dry-run smoke ok'
} finally {
  foreach ($path in @($caseRoot,$reviewRoot)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
