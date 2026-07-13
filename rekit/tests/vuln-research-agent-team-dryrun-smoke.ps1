param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

$Pack = 'vuln-research'
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
$caseRoot = Join-Path $WorkRoot "vuln-research-agent-team-dryrun-$suffix"
$reviewRoot = Join-Path $WorkRoot "vuln-research-agent-team-review-$suffix"

try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"vuln-research-dryrun-$suffix",'-Apply') | Out-Null

  $start = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','crash','-Apply','-Format','json') | ConvertFrom-Json
  if ([string]$start.command -ne 'start' -or -not [bool]$start.isMutation -or -not [bool]$start.applied -or [string]$start.lane.id -ne 'vuln-analysis-crash' -or [string]$start.lane.type -ne 'vuln-analysis' -or [string]$start.lane.workspace -ne 'workspace/vulns/vuln-analysis-crash') {
    throw "unexpected vuln-research start result: $($start | ConvertTo-Json -Depth 20)"
  }

  $packetRel = 'workspace\vulns\vuln-analysis-crash\packet.md'
  Write-Utf8File -Path (Join-Path $caseRoot $packetRel) -Text "# packet`r`n`r`nvulnerability crash triage packet for dry-run only; no live target, active scan, fuzzing, exploit replay, debug, dump, patch, or network call`r`n"

  $planOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','crash-triage','-Items','crash-hypothesis-a,patch-diff-b','-ItemsPerAgent','1','-MaxParallel','2','-ReviewOutputDir',$reviewRoot)
  Assert-ContainsText -Text $planOut -Expected 'review packet:' -Label 'vuln-research plan-subagents output'
  $packetPath = Join-Path $reviewRoot 'packet.json'
  if (-not (Test-Path -LiteralPath $packetPath)) { throw "missing vuln-research review packet: $packetPath" }
  $packet = Read-JsonFile -Path $packetPath
  if ([string]$packet.route.id -ne 'vuln-research:vuln-analysis' -or [string]$packet.observability.routeDebug.selectedBy -ne 'taskType') {
    throw "unexpected vuln-research route packet: $($packet | ConvertTo-Json -Depth 20)"
  }
  if ([string]$packet.observability.dispatchMode -ne 'manual-main-agent' -or @($packet.observability.shardStatuses).Count -ne 2 -or [string]@($packet.observability.shardStatuses)[0].status -ne 'planned' -or [string]$packet.reviewLoop.spawnOwner -ne 'main-agent') {
    throw "unexpected vuln-research dispatch observability: $($packet | ConvertTo-Json -Depth 20)"
  }
  foreach ($expected in @('runtime does not spawn subagents','canonical-write')) {
    Assert-ContainsText -Text (($packet | ConvertTo-Json -Depth 20)) -Expected $expected -Label 'vuln-research dispatch contract'
  }

  $candidate = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','candidate','-Lane','vuln-analysis-crash','-Subject','root cause candidate','-Summary','candidate awaiting bounded vulnerability review','-Actor','vuln-agent','-Confidence','high','-Status','open','-Risk','high','-TargetRef','crash-hypothesis-a','-BatchId','batch-vuln-real-dryrun','-EvidenceRefs','workspace/vulns/vuln-analysis-crash/packet.md') | ConvertFrom-Json
  if ([string]$candidate.command -ne 'note' -or -not [bool]$candidate.applied -or [string]$candidate.path -ne '.rekit/facts/candidates.jsonl' -or [string]$candidate.event.kind -ne 'candidate' -or [string]$candidate.event.lane -ne 'vuln-analysis-crash') {
    throw "unexpected vuln-research candidate append: $($candidate | ConvertTo-Json -Depth 20)"
  }

  $requestsPath = Join-Path $caseRoot '.rekit\facts\requests.jsonl'
  $beforeRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  $gatePreview = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','-Action','debug','-Lane','vuln-analysis-crash','-Actor','runtime-test','-Subject','crash debugger gate','-Summary','needs user confirmation before debugger-assisted crash reproduction','-TargetRef','crash-hypothesis-a','-BatchId','batch-vuln-real-dryrun','-Scope','single crash sidecar only','-Budget','30s','-TriedLightSteps','plan-subagents,static crash triage','-StopConditions','timeout,no live target') | ConvertFrom-Json
  if ([string]$gatePreview.command -ne 'gate' -or [bool]$gatePreview.isMutation -or -not [bool]$gatePreview.requiresConfirmation -or [string]$gatePreview.eventPreview.status -ne 'pending-gate' -or [string]$gatePreview.eventPreview.gate.action -ne 'debug') {
    throw "unexpected vuln-research gate preview: $($gatePreview | ConvertTo-Json -Depth 20)"
  }
  $afterPreviewRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  if ($beforeRequests -ne $afterPreviewRequests) { throw 'vuln-research gate what-if changed requests ledger' }

  $gateApply = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Action','debug','-Lane','vuln-analysis-crash','-Actor','runtime-test','-Risk','high','-Subject','crash debugger gate','-Summary','needs user confirmation before debugger-assisted crash reproduction','-TargetRef','crash-hypothesis-a','-BatchId','batch-vuln-real-dryrun','-Scope','single crash sidecar only','-Budget','30s','-TriedLightSteps','plan-subagents,static crash triage','-StopConditions','timeout,no live target') | ConvertFrom-Json
  if ([string]$gateApply.command -ne 'gate' -or -not [bool]$gateApply.isMutation -or -not [bool]$gateApply.applied -or [string]$gateApply.event.status -ne 'pending-gate' -or [string]$gateApply.event.gate.action -ne 'debug' -or [string]$gateApply.event.gate.scope -ne 'single crash sidecar only') {
    throw "unexpected vuln-research gate apply: $($gateApply | ConvertTo-Json -Depth 20)"
  }

  $verification = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','verification','-Lane','vuln-analysis-crash','-Subject','bounded vulnerability review','-Summary','bounded reviewer accepted root cause evidence','-Actor','reviewer-smoke','-Verifier','tool-review','-Verdict','accepted','-TargetRef','crash-hypothesis-a','-BatchId','batch-vuln-real-dryrun','-EvidenceRefs','workspace/vulns/vuln-analysis-crash/packet.md') | ConvertFrom-Json
  if ([string]$verification.command -ne 'note' -or -not [bool]$verification.applied -or [string]$verification.event.verifier -ne 'tool-review' -or [string]$verification.event.verdict -ne 'accepted') {
    throw "unexpected vuln-research verification append: $($verification | ConvertTo-Json -Depth 20)"
  }

  $decision = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','decision','-Lane','vuln-analysis-crash','-Subject','vulnerability merge decision','-Summary','main accepted reviewed vulnerability candidate','-Actor','main-smoke','-Decision','accept','-Status','accepted','-Reason','reviewer accepted root cause evidence','-TargetRef','crash-hypothesis-a','-BatchId','batch-vuln-real-dryrun') | ConvertFrom-Json
  if ([string]$decision.command -ne 'note' -or -not [bool]$decision.applied -or [string]$decision.event.decision -ne 'accept') {
    throw "unexpected vuln-research decision append: $($decision | ConvertTo-Json -Depth 20)"
  }

  $requestList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','request','-Lane','vuln-analysis-crash')
  foreach ($expected in @('crash debugger gate','status=pending-gate','by=runtime-test','risk=high','action=debug','scope=single crash sidecar only','budget=30s','batch=batch-vuln-real-dryrun')) {
    Assert-ContainsText -Text $requestList -Expected $expected -Label 'vuln-research request note list'
  }
  $verificationList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','verification','-Lane','vuln-analysis-crash')
  foreach ($expected in @('bounded vulnerability review','verifier=tool-review','verdict=accepted','target=crash-hypothesis-a','batch=batch-vuln-real-dryrun')) {
    Assert-ContainsText -Text $verificationList -Expected $expected -Label 'vuln-research verification note list'
  }

  $overviewJson = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') | ConvertFrom-Json
  if ([string]$overviewJson.command -ne 'overview' -or [bool]$overviewJson.isMutation -or [string]$overviewJson.pack -ne $Pack -or [int]$overviewJson.counts.candidates -ne 1 -or [int]$overviewJson.counts.requests -ne 1 -or [int]$overviewJson.sections.pendingGates.total -ne 1 -or [int]$overviewJson.sections.verifications.total -ne 1 -or [int]$overviewJson.sections.decisions.total -ne 1 -or [int]$overviewJson.sections.batches.total -ne 1) {
    throw "unexpected vuln-research overview JSON: $($overviewJson | ConvertTo-Json -Depth 20)"
  }

  $overview = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack)
  foreach ($expected in @('vuln-analysis-crash','root cause candidate','crash debugger gate','action=debug','scope=single crash sidecar only','verifier=tool-review','verdict=accepted','decision=accept','reviewer accepted root cause evidence','batch-vuln-real-dryrun')) {
    Assert-ContainsText -Text $overview -Expected $expected -Label 'vuln-research overview text'
  }

  $handoff = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Format','json','crash') | ConvertFrom-Json
  if ([string]$handoff.command -ne 'handoff' -or -not [bool]$handoff.isMutation -or -not [bool]$handoff.applied -or [bool]$handoff.project -or [string]$handoff.lane.id -ne 'vuln-analysis-crash' -or [string]$handoff.lane.workspace -ne 'workspace/vulns/vuln-analysis-crash') {
    throw "unexpected vuln-research handoff result: $($handoff | ConvertTo-Json -Depth 20)"
  }
  $handoffPath = Join-Path $caseRoot '.rekit\handovers\vuln-analysis-crash-latest.md'
  if (-not (Test-Path -LiteralPath $handoffPath)) { throw "missing vuln-research handoff: $handoffPath" }
  $handoffText = [System.IO.File]::ReadAllText($handoffPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('vuln-analysis-crash','/rekit continue vuln-analysis-crash','workspace/vulns/vuln-analysis-crash/packet.md','## verification','verifier=tool-review','## decision','decision=accept','## pending-gate','action=debug','scope=single crash sidecar only')) {
    Assert-ContainsText -Text $handoffText -Expected $expected -Label 'vuln-research handoff text'
  }
  foreach ($unexpected in @('web-security','workspace/features','endpoint-login','generic-binary-re','workspace/binary','binary-analysis-sample','malware-analysis','workspace/samples','sample-alpha','sandbox','references/template','vmp-re')) {
    Assert-NotContainsText -Text $handoffText -Unexpected $unexpected -Label 'vuln-research handoff leakage guard'
  }

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  'vuln-research agent-team dry-run smoke ok'
} finally {
  foreach ($path in @($caseRoot,$reviewRoot)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
