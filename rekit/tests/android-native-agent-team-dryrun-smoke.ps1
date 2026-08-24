param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

$Pack = 'android-native'
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
$caseRoot = Join-Path $WorkRoot "android-native-agent-team-dryrun-$suffix"
$reviewRoot = Join-Path $WorkRoot "android-native-agent-team-review-$suffix"

try {
  $initPreview = Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"android-native-dryrun-$suffix",'-WhatIf','-Format','json') | ConvertFrom-Json
  $initApplyArgs = @($initPreview.applyArgs | ForEach-Object { [string]$_ })
  if ($initApplyArgs.Count -eq 0) { throw 'android-native dry-run init preview omitted applyArgs' }
  Invoke-RekitSmoke -Arguments $initApplyArgs | Out-Null

  $start = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','jni','-Apply','-Format','json') | ConvertFrom-Json
  if ([string]$start.command -ne 'start' -or -not [bool]$start.isMutation -or -not [bool]$start.applied -or [string]$start.lane.id -ne 'native-analysis-jni' -or [string]$start.lane.type -ne 'native-analysis' -or [string]$start.lane.workspace -ne 'workspace/native/native-analysis-jni') {
    throw "unexpected android-native start result: $($start | ConvertTo-Json -Depth 20)"
  }

  $packetRel = 'workspace\native\native-analysis-jni\packet.md'
  Write-Utf8File -Path (Join-Path $caseRoot $packetRel) -Text "# packet`r`n`r`nAndroid native JNI triage packet for dry-run only; no device connection, Frida attach, hook execution, injection, dump, patch, or network call`r`n"

  $planOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','jni-triage','-Items','libnative-alpha,jni-symbol-init','-ItemsPerAgent','1','-MaxParallel','2','-ReviewOutputDir',$reviewRoot)
  Assert-ContainsText -Text $planOut -Expected '"command": "plan-subagents"' -Label 'android-native plan-subagents default Go output'
  $packetPath = Join-Path $reviewRoot 'packet.json'
  if (-not (Test-Path -LiteralPath $packetPath)) { throw "missing android-native review packet: $packetPath" }
  $packet = Read-JsonFile -Path $packetPath
  if ([string]$packet.route.id -ne 'android-native:native-analysis' -or [string]$packet.observability.routeDebug.selectedBy -ne 'taskType') {
    throw "unexpected android-native route packet: $($packet | ConvertTo-Json -Depth 20)"
  }
  if ([string]$packet.observability.dispatchMode -ne 'manual-main-agent' -or @($packet.observability.shardStatuses).Count -ne 2 -or [string]@($packet.observability.shardStatuses)[0].status -ne 'planned' -or [string]$packet.reviewLoop.spawnOwner -ne 'main-agent') {
    throw "unexpected android-native dispatch observability: $($packet | ConvertTo-Json -Depth 20)"
  }
  foreach ($expected in @('runtime does not spawn subagents','canonical-write')) {
    Assert-ContainsText -Text (($packet | ConvertTo-Json -Depth 20)) -Expected $expected -Label 'android-native dispatch contract'
  }

  $candidate = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','candidate','-Lane','native-analysis-jni','-Subject','JNI bridge candidate','-Summary','candidate awaiting bounded Android native review','-Actor','android-agent','-Confidence','high','-Status','open','-Risk','medium','-TargetRef','libnative-alpha','-BatchId','batch-android-real-dryrun','-EvidenceRefs','workspace/native/native-analysis-jni/packet.md') | ConvertFrom-Json
  if ([string]$candidate.command -ne 'note' -or -not [bool]$candidate.applied -or [string]$candidate.path -ne '.steamai/facts/candidates.jsonl' -or [string]$candidate.event.kind -ne 'candidate' -or [string]$candidate.event.lane -ne 'native-analysis-jni') {
    throw "unexpected android-native candidate append: $($candidate | ConvertTo-Json -Depth 20)"
  }

  $requestsPath = Join-Path $caseRoot '.steamai\facts\requests.jsonl'
  $beforeRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  $gatePreview = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','-Action','inject','-Lane','native-analysis-jni','-Actor','runtime-test','-Subject','Frida hook gate','-Summary','needs user confirmation before Frida hook or injection sidecar','-TargetRef','libnative-alpha','-BatchId','batch-android-real-dryrun','-Scope','single JNI symbol sidecar only','-Budget','30s','-TriedLightSteps','plan-subagents,static JNI triage','-StopConditions','timeout,no-device-connection,no-hook-execution') | ConvertFrom-Json
  if ([string]$gatePreview.command -ne 'gate' -or [bool]$gatePreview.isMutation -or -not [bool]$gatePreview.requiresConfirmation -or [string]$gatePreview.eventPreview.status -ne 'pending-gate' -or [string]$gatePreview.eventPreview.gate.action -ne 'inject') {
    throw "unexpected android-native gate preview: $($gatePreview | ConvertTo-Json -Depth 20)"
  }
  $afterPreviewRequests = if (Test-Path -LiteralPath $requestsPath) { [System.IO.File]::ReadAllText($requestsPath, [System.Text.Encoding]::UTF8) } else { $null }
  if ($beforeRequests -ne $afterPreviewRequests) { throw 'android-native gate what-if changed requests ledger' }

  $gateApply = Invoke-RekitSmoke -Arguments @('-Command','gate','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Action','inject','-Lane','native-analysis-jni','-Actor','runtime-test','-Risk','medium','-Subject','Frida hook gate','-Summary','needs user confirmation before Frida hook or injection sidecar','-TargetRef','libnative-alpha','-BatchId','batch-android-real-dryrun','-Scope','single JNI symbol sidecar only','-Budget','30s','-TriedLightSteps','plan-subagents,static JNI triage','-StopConditions','timeout,no-device-connection,no-hook-execution') | ConvertFrom-Json
  if ([string]$gateApply.command -ne 'gate' -or -not [bool]$gateApply.isMutation -or -not [bool]$gateApply.applied -or [string]$gateApply.event.status -ne 'pending-gate' -or [string]$gateApply.event.gate.action -ne 'inject' -or [string]$gateApply.event.gate.scope -ne 'single JNI symbol sidecar only') {
    throw "unexpected android-native gate apply: $($gateApply | ConvertTo-Json -Depth 20)"
  }

  $verification = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','verification','-Lane','native-analysis-jni','-Subject','bounded Android native review','-Summary','bounded reviewer accepted JNI triage evidence','-Actor','reviewer-smoke','-Verifier','tool-review','-Verdict','accepted','-TargetRef','libnative-alpha','-BatchId','batch-android-real-dryrun','-EvidenceRefs','workspace/native/native-analysis-jni/packet.md') | ConvertFrom-Json
  if ([string]$verification.command -ne 'note' -or -not [bool]$verification.applied -or [string]$verification.event.verifier -ne 'tool-review' -or [string]$verification.event.verdict -ne 'accepted') {
    throw "unexpected android-native verification append: $($verification | ConvertTo-Json -Depth 20)"
  }

  $decision = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-Kind','decision','-Lane','native-analysis-jni','-Subject','Android native merge decision','-Summary','main accepted reviewed JNI bridge candidate','-Actor','main-smoke','-Decision','accept','-Status','accepted','-Reason','reviewer accepted JNI triage evidence','-TargetRef','libnative-alpha','-BatchId','batch-android-real-dryrun') | ConvertFrom-Json
  if ([string]$decision.command -ne 'note' -or -not [bool]$decision.applied -or [string]$decision.event.decision -ne 'accept') {
    throw "unexpected android-native decision append: $($decision | ConvertTo-Json -Depth 20)"
  }

  $requestList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','request','-Lane','native-analysis-jni')
  foreach ($expected in @('Frida hook gate','status=pending-gate','by=runtime-test','risk=medium','action=inject','scope=single JNI symbol sidecar only','budget=30s','batch=batch-android-real-dryrun')) {
    Assert-ContainsText -Text $requestList -Expected $expected -Label 'android-native request note list'
  }
  $verificationList = Invoke-RekitSmoke -Arguments @('-Command','note','-Target',$caseRoot,'-Pack',$Pack,'-List','-Kind','verification','-Lane','native-analysis-jni')
  foreach ($expected in @('bounded Android native review','verifier=tool-review','verdict=accepted','target=libnative-alpha','batch=batch-android-real-dryrun')) {
    Assert-ContainsText -Text $verificationList -Expected $expected -Label 'android-native verification note list'
  }

  $overviewJson = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') | ConvertFrom-Json
  if ([string]$overviewJson.command -ne 'overview' -or [bool]$overviewJson.isMutation -or [string]$overviewJson.pack -ne $Pack -or [int]$overviewJson.counts.candidates -ne 1 -or [int]$overviewJson.counts.requests -ne 1 -or [int]$overviewJson.sections.pendingGates.total -ne 1 -or [int]$overviewJson.sections.verifications.total -ne 1 -or [int]$overviewJson.sections.decisions.total -ne 1 -or [int]$overviewJson.sections.batches.total -ne 1) {
    throw "unexpected android-native overview JSON: $($overviewJson | ConvertTo-Json -Depth 20)"
  }

  $overview = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack)
  foreach ($expected in @('native-analysis-jni','JNI bridge candidate','Frida hook gate','action=inject','scope=single JNI symbol sidecar only','verifier=tool-review','verdict=accepted','decision=accept','reviewer accepted JNI triage evidence','batch-android-real-dryrun')) {
    Assert-ContainsText -Text $overview -Expected $expected -Label 'android-native overview text'
  }

  $handoffPreview = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','-Format','json','jni') | ConvertFrom-Json
  $handoffApplyArgs = @($handoffPreview.applyArgs | ForEach-Object { [string]$_ })
  if ([string]::IsNullOrWhiteSpace([string]$handoffPreview.publicationPlanSha256) -or [string]::IsNullOrWhiteSpace([string]$handoffPreview.publicationStamp) -or $handoffApplyArgs.Count -eq 0) { throw 'android-native handoff preview omitted exact Apply action' }
  $handoff = Invoke-RekitSmoke -Arguments $handoffApplyArgs | ConvertFrom-Json
  if ([string]$handoff.command -ne 'handoff' -or -not [bool]$handoff.isMutation -or -not [bool]$handoff.applied -or [bool]$handoff.project -or [string]$handoff.lane.id -ne 'native-analysis-jni' -or [string]$handoff.lane.workspace -ne 'workspace/native/native-analysis-jni') {
    throw "unexpected android-native handoff result: $($handoff | ConvertTo-Json -Depth 20)"
  }
  $handoffPath = Join-Path $caseRoot '.steamai\handovers\native-analysis-jni-latest.md'
  if (-not (Test-Path -LiteralPath $handoffPath)) { throw "missing android-native handoff: $handoffPath" }
  $handoffText = [System.IO.File]::ReadAllText($handoffPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('native-analysis-jni','/rekit continue -Lane native-analysis-jni','workspace/native/native-analysis-jni/packet.md','## verification','verifier=tool-review','## decision','decision=accept','## pending-gate','action=inject','scope=single JNI symbol sidecar only')) {
    Assert-ContainsText -Text $handoffText -Expected $expected -Label 'android-native handoff text'
  }
  foreach ($unexpected in @('web-security','workspace/features','endpoint-login','generic-binary-re','workspace/binary','binary-analysis-sample','malware-analysis','workspace/samples','sample-alpha','vuln-research','workspace/vulns','ctf','workspace/challenges','challenge-alpha','unpack-pe','workspace/unpack','packed-sample','ollvm','workspace/obfuscation','function-alpha','references/template','vmp-re')) {
    Assert-NotContainsText -Text $handoffText -Unexpected $unexpected -Label 'android-native handoff leakage guard'
  }

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  'android-native agent-team dry-run smoke ok'
} finally {
  foreach ($path in @($caseRoot,$reviewRoot)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
