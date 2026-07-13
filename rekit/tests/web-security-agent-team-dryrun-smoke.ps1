param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

$Pack = 'web-security'
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
$caseRoot = Join-Path $WorkRoot "web-security-agent-team-dryrun-$suffix"
$reviewRoot = Join-Path $WorkRoot "web-security-agent-team-review-$suffix"

try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"web-security-dryrun-$suffix",'-Apply') | Out-Null

  $start = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','authz','-Apply','-Format','json') | ConvertFrom-Json
  if ([string]$start.command -ne 'start' -or -not [bool]$start.isMutation -or -not [bool]$start.applied -or [string]$start.lane.id -ne 'feature-authz' -or [string]$start.lane.type -ne 'feature' -or [string]$start.lane.workspace -ne 'workspace/features/feature-authz') {
    throw "unexpected web-security start result: $($start | ConvertTo-Json -Depth 20)"
  }

  $packetRel = 'workspace\features\feature-authz\packet.md'
  Write-Utf8File -Path (Join-Path $caseRoot $packetRel) -Text "# packet`r`n`r`nweb endpoint authorization packet for dry-run only`r`n"

  $planOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','endpoint-analysis','-Items','endpoint-login,api-flow-authz','-ItemsPerAgent','1','-MaxParallel','2','-ReviewOutputDir',$reviewRoot)
  Assert-ContainsText -Text $planOut -Expected 'review packet:' -Label 'web-security plan-subagents output'
  $packetPath = Join-Path $reviewRoot 'packet.json'
  if (-not (Test-Path -LiteralPath $packetPath)) { throw "missing web-security review packet: $packetPath" }
  $packet = Read-JsonFile -Path $packetPath
  if ([string]$packet.route.id -ne 'web-security:feature-analysis' -or [string]$packet.observability.routeDebug.selectedBy -ne 'taskType') {
    throw "unexpected web-security route packet: $($packet | ConvertTo-Json -Depth 20)"
  }
  if ([string]$packet.observability.dispatchMode -ne 'manual-main-agent' -or @($packet.observability.shardStatuses).Count -ne 2 -or [string]@($packet.observability.shardStatuses)[0].status -ne 'planned' -or [string]$packet.reviewLoop.spawnOwner -ne 'main-agent') {
    throw "unexpected web-security dispatch observability: $($packet | ConvertTo-Json -Depth 20)"
  }
  foreach ($expected in @('runtime does not spawn subagents','canonical-write')) {
    Assert-ContainsText -Text (($packet | ConvertTo-Json -Depth 20)) -Expected $expected -Label 'web-security dispatch contract'
  }

  $candidate = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','candidate','-Lane','feature-authz','-Subject','endpoint authz candidate','-Summary','candidate awaiting bounded web review','-Actor','web-agent','-Confidence','high','-Status','open','-Risk','high','-TargetRef','endpoint-login','-BatchId','batch-web-real-dryrun','-EvidenceRefs','workspace/features/feature-authz/packet.md') | ConvertFrom-Json
  if ([string]$candidate.command -ne 'note' -or -not [bool]$candidate.applied -or [string]$candidate.path -ne '.rekit/facts/candidates.jsonl' -or [string]$candidate.event.kind -ne 'candidate' -or [string]$candidate.event.lane -ne 'feature-authz') {
    throw "unexpected web-security candidate append: $($candidate | ConvertTo-Json -Depth 20)"
  }

  $requestsPath = Join-Path $caseRoot '.rekit\facts\requests.jsonl'
  $beforeRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  $gatePreview = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','-Action','network','-Lane','feature-authz','-Actor','runtime-test','-Subject','web request replay gate','-Summary','needs user confirmation before request replay','-TargetRef','endpoint-login','-BatchId','batch-web-real-dryrun','-Scope','single endpoint replay','-Budget','30s','-TriedLightSteps','plan-subagents,passive triage','-StopConditions','timeout') | ConvertFrom-Json
  if ([string]$gatePreview.command -ne 'gate' -or [bool]$gatePreview.isMutation -or -not [bool]$gatePreview.requiresConfirmation -or [string]$gatePreview.eventPreview.status -ne 'pending-gate' -or [string]$gatePreview.eventPreview.gate.action -ne 'network') {
    throw "unexpected web-security gate preview: $($gatePreview | ConvertTo-Json -Depth 20)"
  }
  $afterPreviewRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  if ($beforeRequests -ne $afterPreviewRequests) { throw 'web-security gate what-if changed requests ledger' }

  $gateApply = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Action','network','-Lane','feature-authz','-Actor','runtime-test','-Risk','high','-Subject','web request replay gate','-Summary','needs user confirmation before request replay','-TargetRef','endpoint-login','-BatchId','batch-web-real-dryrun','-Scope','single endpoint replay','-Budget','30s','-TriedLightSteps','plan-subagents,passive triage','-StopConditions','timeout') | ConvertFrom-Json
  if ([string]$gateApply.command -ne 'gate' -or -not [bool]$gateApply.isMutation -or -not [bool]$gateApply.applied -or [string]$gateApply.event.status -ne 'pending-gate' -or [string]$gateApply.event.gate.action -ne 'network' -or [string]$gateApply.event.gate.scope -ne 'single endpoint replay') {
    throw "unexpected web-security gate apply: $($gateApply | ConvertTo-Json -Depth 20)"
  }

  $verification = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','verification','-Lane','feature-authz','-Subject','bounded endpoint review','-Summary','bounded reviewer accepted web endpoint evidence','-Actor','reviewer-smoke','-Verifier','manual-review','-Verdict','accepted','-TargetRef','endpoint-login','-BatchId','batch-web-real-dryrun','-EvidenceRefs','workspace/features/feature-authz/packet.md') | ConvertFrom-Json
  if ([string]$verification.command -ne 'note' -or -not [bool]$verification.applied -or [string]$verification.event.verifier -ne 'manual-review' -or [string]$verification.event.verdict -ne 'accepted') {
    throw "unexpected web-security verification append: $($verification | ConvertTo-Json -Depth 20)"
  }

  $decision = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','decision','-Lane','feature-authz','-Subject','web merge decision','-Summary','main accepted reviewed web endpoint candidate','-Actor','main-smoke','-Decision','accept','-Status','accepted','-Reason','reviewer accepted web endpoint evidence','-TargetRef','endpoint-login','-BatchId','batch-web-real-dryrun') | ConvertFrom-Json
  if ([string]$decision.command -ne 'note' -or -not [bool]$decision.applied -or [string]$decision.event.decision -ne 'accept') {
    throw "unexpected web-security decision append: $($decision | ConvertTo-Json -Depth 20)"
  }

  $requestList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','request','-Lane','feature-authz')
  foreach ($expected in @('web request replay gate','status=pending-gate','by=runtime-test','risk=high','action=network','scope=single endpoint replay','budget=30s','batch=batch-web-real-dryrun')) {
    Assert-ContainsText -Text $requestList -Expected $expected -Label 'web-security request note list'
  }
  $verificationList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','verification','-Lane','feature-authz')
  foreach ($expected in @('bounded endpoint review','verifier=manual-review','verdict=accepted','target=endpoint-login','batch=batch-web-real-dryrun')) {
    Assert-ContainsText -Text $verificationList -Expected $expected -Label 'web-security verification note list'
  }

  $overviewJson = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') | ConvertFrom-Json
  if ([string]$overviewJson.command -ne 'overview' -or [bool]$overviewJson.isMutation -or [string]$overviewJson.pack -ne $Pack -or [int]$overviewJson.counts.candidates -ne 1 -or [int]$overviewJson.counts.requests -ne 1 -or [int]$overviewJson.sections.pendingGates.total -ne 1 -or [int]$overviewJson.sections.verifications.total -ne 1 -or [int]$overviewJson.sections.decisions.total -ne 1 -or [int]$overviewJson.sections.batches.total -ne 1) {
    throw "unexpected web-security overview JSON: $($overviewJson | ConvertTo-Json -Depth 20)"
  }

  $overview = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack)
  foreach ($expected in @('feature-authz','endpoint authz candidate','web request replay gate','action=network','scope=single endpoint replay','verifier=manual-review','verdict=accepted','decision=accept','reviewer accepted web endpoint evidence','batch-web-real-dryrun')) {
    Assert-ContainsText -Text $overview -Expected $expected -Label 'web-security overview text'
  }

  $handoff = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Format','json','authz') | ConvertFrom-Json
  if ([string]$handoff.command -ne 'handoff' -or -not [bool]$handoff.isMutation -or -not [bool]$handoff.applied -or [bool]$handoff.project -or [string]$handoff.lane.id -ne 'feature-authz' -or [string]$handoff.lane.workspace -ne 'workspace/features/feature-authz') {
    throw "unexpected web-security handoff result: $($handoff | ConvertTo-Json -Depth 20)"
  }
  $handoffPath = Join-Path $caseRoot '.rekit\handovers\feature-authz-latest.md'
  if (-not (Test-Path -LiteralPath $handoffPath)) { throw "missing web-security handoff: $handoffPath" }
  $handoffText = [System.IO.File]::ReadAllText($handoffPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('feature-authz','/rekit continue authz','workspace/features/feature-authz/packet.md','## verification','verifier=manual-review','## decision','decision=accept','## pending-gate','action=network','scope=single endpoint replay')) {
    Assert-ContainsText -Text $handoffText -Expected $expected -Label 'web-security handoff text'
  }
  foreach ($unexpected in @('generic-binary-re','workspace/binary','binary-analysis-sample','references/template','vmp-re')) {
    Assert-NotContainsText -Text $handoffText -Unexpected $unexpected -Label 'web-security handoff leakage guard'
  }

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  'web-security agent-team dry-run smoke ok'
} finally {
  foreach ($path in @($caseRoot,$reviewRoot)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
