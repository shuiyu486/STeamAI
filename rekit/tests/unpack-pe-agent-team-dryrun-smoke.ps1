param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

$Pack = 'unpack-pe'
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
  return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "unpack-pe-agent-team-dryrun-$suffix"
$reviewRoot = Join-Path $WorkRoot "unpack-pe-agent-team-review-$suffix"

try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"unpack-pe-dryrun-$suffix",'-Apply') | Out-Null

  $start = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','loader','-Apply','-Format','json') | ConvertFrom-Json
  if ([string]$start.command -ne 'start' -or -not [bool]$start.isMutation -or -not [bool]$start.applied -or [string]$start.lane.id -ne 'unpack-analysis-loader' -or [string]$start.lane.type -ne 'unpack-analysis' -or [string]$start.lane.workspace -ne 'workspace/unpack/unpack-analysis-loader') {
    throw "unexpected unpack-pe start result: $($start | ConvertTo-Json -Depth 20)"
  }

  $packetRel = 'workspace\unpack\unpack-analysis-loader\packet.md'
  Write-Utf8File -Path (Join-Path $caseRoot $packetRel) -Text "# packet`r`n`r`nPE loader triage packet for dry-run only; no sample execution, debug, dump, patch, memory extraction, unpacked binary writeback, or network call`r`n"

  $planOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','pe-triage','-Items','packed-sample,loader-stage-one','-ItemsPerAgent','1','-MaxParallel','2','-ReviewOutputDir',$reviewRoot)
  Assert-ContainsText -Text $planOut -Expected 'review packet:' -Label 'unpack-pe plan-subagents output'
  $packetPath = Join-Path $reviewRoot 'packet.json'
  if (-not (Test-Path -LiteralPath $packetPath)) { throw "missing unpack-pe review packet: $packetPath" }
  $packet = Read-JsonFile -Path $packetPath
  if ([string]$packet.route.id -ne 'unpack-pe:unpack-analysis' -or [string]$packet.observability.routeDebug.selectedBy -ne 'taskType') {
    throw "unexpected unpack-pe route packet: $($packet | ConvertTo-Json -Depth 20)"
  }
  if ([string]$packet.observability.dispatchMode -ne 'manual-main-agent' -or @($packet.observability.shardStatuses).Count -ne 2 -or [string]@($packet.observability.shardStatuses)[0].status -ne 'planned' -or [string]$packet.reviewLoop.spawnOwner -ne 'main-agent') {
    throw "unexpected unpack-pe dispatch observability: $($packet | ConvertTo-Json -Depth 20)"
  }
  foreach ($expected in @('runtime does not spawn subagents','canonical-write')) {
    Assert-ContainsText -Text (($packet | ConvertTo-Json -Depth 20)) -Expected $expected -Label 'unpack-pe dispatch contract'
  }

  $candidate = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','candidate','-Lane','unpack-analysis-loader','-Subject','loader unpack candidate','-Summary','candidate awaiting bounded PE unpacking review','-Actor','unpack-agent','-Confidence','high','-Status','open','-Risk','high','-TargetRef','packed-sample','-BatchId','batch-unpack-pe-real-dryrun','-EvidenceRefs','workspace/unpack/unpack-analysis-loader/packet.md') | ConvertFrom-Json
  if ([string]$candidate.command -ne 'note' -or -not [bool]$candidate.applied -or [string]$candidate.path -ne '.rekit/facts/candidates.jsonl' -or [string]$candidate.event.kind -ne 'candidate' -or [string]$candidate.event.lane -ne 'unpack-analysis-loader') {
    throw "unexpected unpack-pe candidate append: $($candidate | ConvertTo-Json -Depth 20)"
  }

  $requestsPath = Join-Path $caseRoot '.rekit\facts\requests.jsonl'
  $beforeRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  $gatePreview = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','-Action','dump','-Lane','unpack-analysis-loader','-Actor','runtime-test','-Subject','loader dump gate','-Summary','needs user confirmation before memory dump or unpacked binary extraction','-TargetRef','packed-sample','-BatchId','batch-unpack-pe-real-dryrun','-Scope','single loader stage only','-Budget','30s','-TriedLightSteps','plan-subagents,static PE triage','-StopConditions','timeout,no unpacked binary writeback') | ConvertFrom-Json
  if ([string]$gatePreview.command -ne 'gate' -or [bool]$gatePreview.isMutation -or -not [bool]$gatePreview.requiresConfirmation -or [string]$gatePreview.eventPreview.status -ne 'pending-gate' -or [string]$gatePreview.eventPreview.gate.action -ne 'dump') {
    throw "unexpected unpack-pe gate preview: $($gatePreview | ConvertTo-Json -Depth 20)"
  }
  $afterPreviewRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  if ($beforeRequests -ne $afterPreviewRequests) { throw 'unpack-pe gate what-if changed requests ledger' }

  $gateApply = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Action','dump','-Lane','unpack-analysis-loader','-Actor','runtime-test','-Risk','high','-Subject','loader dump gate','-Summary','needs user confirmation before memory dump or unpacked binary extraction','-TargetRef','packed-sample','-BatchId','batch-unpack-pe-real-dryrun','-Scope','single loader stage only','-Budget','30s','-TriedLightSteps','plan-subagents,static PE triage','-StopConditions','timeout,no unpacked binary writeback') | ConvertFrom-Json
  if ([string]$gateApply.command -ne 'gate' -or -not [bool]$gateApply.isMutation -or -not [bool]$gateApply.applied -or [string]$gateApply.event.status -ne 'pending-gate' -or [string]$gateApply.event.gate.action -ne 'dump' -or [string]$gateApply.event.gate.scope -ne 'single loader stage only') {
    throw "unexpected unpack-pe gate apply: $($gateApply | ConvertTo-Json -Depth 20)"
  }

  $verification = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','verification','-Lane','unpack-analysis-loader','-Subject','bounded PE unpacking review','-Summary','bounded reviewer accepted loader triage evidence','-Actor','reviewer-smoke','-Verifier','tool-review','-Verdict','accepted','-TargetRef','packed-sample','-BatchId','batch-unpack-pe-real-dryrun','-EvidenceRefs','workspace/unpack/unpack-analysis-loader/packet.md') | ConvertFrom-Json
  if ([string]$verification.command -ne 'note' -or -not [bool]$verification.applied -or [string]$verification.event.verifier -ne 'tool-review' -or [string]$verification.event.verdict -ne 'accepted') {
    throw "unexpected unpack-pe verification append: $($verification | ConvertTo-Json -Depth 20)"
  }

  $decision = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','decision','-Lane','unpack-analysis-loader','-Subject','PE unpacking merge decision','-Summary','main accepted reviewed loader unpack candidate','-Actor','main-smoke','-Decision','accept','-Status','accepted','-Reason','reviewer accepted loader triage evidence','-TargetRef','packed-sample','-BatchId','batch-unpack-pe-real-dryrun') | ConvertFrom-Json
  if ([string]$decision.command -ne 'note' -or -not [bool]$decision.applied -or [string]$decision.event.decision -ne 'accept') {
    throw "unexpected unpack-pe decision append: $($decision | ConvertTo-Json -Depth 20)"
  }

  $requestList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','request','-Lane','unpack-analysis-loader')
  foreach ($expected in @('loader dump gate','status=pending-gate','by=runtime-test','risk=high','action=dump','scope=single loader stage only','budget=30s','batch=batch-unpack-pe-real-dryrun')) {
    Assert-ContainsText -Text $requestList -Expected $expected -Label 'unpack-pe request note list'
  }
  $verificationList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','verification','-Lane','unpack-analysis-loader')
  foreach ($expected in @('bounded PE unpacking review','verifier=tool-review','verdict=accepted','target=packed-sample','batch=batch-unpack-pe-real-dryrun')) {
    Assert-ContainsText -Text $verificationList -Expected $expected -Label 'unpack-pe verification note list'
  }

  $overviewJson = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') | ConvertFrom-Json
  if ([string]$overviewJson.command -ne 'overview' -or [bool]$overviewJson.isMutation -or [string]$overviewJson.pack -ne $Pack -or [int]$overviewJson.counts.candidates -ne 1 -or [int]$overviewJson.counts.requests -ne 1 -or [int]$overviewJson.sections.pendingGates.total -ne 1 -or [int]$overviewJson.sections.verifications.total -ne 1 -or [int]$overviewJson.sections.decisions.total -ne 1 -or [int]$overviewJson.sections.batches.total -ne 1) {
    throw "unexpected unpack-pe overview JSON: $($overviewJson | ConvertTo-Json -Depth 20)"
  }

  $overview = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack)
  foreach ($expected in @('unpack-analysis-loader','loader unpack candidate','loader dump gate','action=dump','scope=single loader stage only','verifier=tool-review','verdict=accepted','decision=accept','reviewer accepted loader triage evidence','batch-unpack-pe-real-dryrun')) {
    Assert-ContainsText -Text $overview -Expected $expected -Label 'unpack-pe overview text'
  }

  $handoff = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Format','json','loader') | ConvertFrom-Json
  if ([string]$handoff.command -ne 'handoff' -or -not [bool]$handoff.isMutation -or -not [bool]$handoff.applied -or [bool]$handoff.project -or [string]$handoff.lane.id -ne 'unpack-analysis-loader' -or [string]$handoff.lane.workspace -ne 'workspace/unpack/unpack-analysis-loader') {
    throw "unexpected unpack-pe handoff result: $($handoff | ConvertTo-Json -Depth 20)"
  }
  $handoffPath = Join-Path $caseRoot '.rekit\handovers\unpack-analysis-loader-latest.md'
  if (-not (Test-Path -LiteralPath $handoffPath)) { throw "missing unpack-pe handoff: $handoffPath" }
  $handoffText = [System.IO.File]::ReadAllText($handoffPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('unpack-analysis-loader','/rekit continue unpack-analysis-loader','workspace/unpack/unpack-analysis-loader/packet.md','## verification','verifier=tool-review','## decision','decision=accept','## pending-gate','action=dump','scope=single loader stage only')) {
    Assert-ContainsText -Text $handoffText -Expected $expected -Label 'unpack-pe handoff text'
  }
  foreach ($unexpected in @('web-security','workspace/features','endpoint-login','generic-binary-re','workspace/binary','binary-analysis-sample','malware-analysis','workspace/samples','sample-alpha','vuln-research','workspace/vulns','ctf','workspace/challenges','challenge-alpha','references/template','vmp-re')) {
    Assert-NotContainsText -Text $handoffText -Unexpected $unexpected -Label 'unpack-pe handoff leakage guard'
  }

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  'unpack-pe agent-team dry-run smoke ok'
} finally {
  foreach ($path in @($caseRoot,$reviewRoot)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
