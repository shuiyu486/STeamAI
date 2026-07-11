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

function Assert-NotContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Unexpected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text -like "*$Unexpected*") { throw "$Label contained unexpected text '$Unexpected'. Output:`n$Text" }
}

function Assert-Equals {
  param(
    [Parameter(Mandatory=$true)]$Expected,
    [Parameter(Mandatory=$true)]$Actual,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Expected -ne $Actual) { throw "$Label expected '$Expected', got '$Actual'" }
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

function Write-JsonLines {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][object[]]$Objects
  )
  $lines = @($Objects | ForEach-Object { $_ | ConvertTo-Json -Depth 32 -Compress })
  Write-Utf8File -Path $Path -Text (($lines -join "`r`n") + "`r`n")
}

function Read-JsonLines {
  param([Parameter(Mandatory=$true)][string]$Path)
  $items = @()
  if (-not (Test-Path -LiteralPath $Path)) { return $items }
  foreach ($line in [System.IO.File]::ReadLines($Path, [System.Text.Encoding]::UTF8)) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    $items += ($line | ConvertFrom-Json)
  }
  return $items
}

function Save-TreeSnapshot {
  param([Parameter(Mandatory=$true)][string]$Path)
  $snapshot = @{}
  if (-not (Test-Path -LiteralPath $Path)) { return $snapshot }
  $root = [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
  Get-ChildItem -LiteralPath $Path -Recurse -File | ForEach-Object {
    $rel = $_.FullName.Substring($root.Length).TrimStart('\')
    $snapshot[$rel] = [System.IO.File]::ReadAllBytes($_.FullName)
  }
  return $snapshot
}

function Save-TreeDirectories {
  param([Parameter(Mandatory=$true)][string]$Path)
  $snapshot = @{}
  if (-not (Test-Path -LiteralPath $Path)) { return $snapshot }
  $root = [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
  Get-ChildItem -LiteralPath $Path -Recurse -Directory | ForEach-Object {
    $rel = $_.FullName.Substring($root.Length).TrimStart('\')
    if (-not [string]::IsNullOrWhiteSpace($rel)) { $snapshot[$rel] = $true }
  }
  return $snapshot
}

function Assert-TreeUnchanged {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][hashtable]$BeforeSnapshot,
    [Parameter(Mandatory=$true)][hashtable]$BeforeDirectories,
    [Parameter(Mandatory=$true)][string]$Label
  )
  $afterSnapshot = Save-TreeSnapshot -Path $Root
  $afterDirectories = Save-TreeDirectories -Path $Root
  foreach ($rel in $BeforeSnapshot.Keys) {
    if (-not $afterSnapshot.ContainsKey($rel)) { throw "$Label removed file: $rel" }
    $beforeBytes = [byte[]]$BeforeSnapshot[$rel]
    $afterBytes = [byte[]]$afterSnapshot[$rel]
    if ($beforeBytes.Length -ne $afterBytes.Length) { throw "$Label changed file length: $rel" }
    for ($i = 0; $i -lt $beforeBytes.Length; $i++) {
      if ($beforeBytes[$i] -ne $afterBytes[$i]) { throw "$Label changed file content: $rel" }
    }
  }
  foreach ($rel in $afterSnapshot.Keys) {
    if (-not $BeforeSnapshot.ContainsKey($rel)) { throw "$Label created file: $rel" }
  }
  foreach ($rel in $BeforeDirectories.Keys) {
    if (-not $afterDirectories.ContainsKey($rel)) { throw "$Label removed directory: $rel" }
  }
  foreach ($rel in $afterDirectories.Keys) {
    if (-not $BeforeDirectories.ContainsKey($rel)) { throw "$Label created directory: $rel" }
  }
}

function Get-EventById {
  param(
    [Parameter(Mandatory=$true)][object[]]$Items,
    [Parameter(Mandatory=$true)][string]$EventId,
    [Parameter(Mandatory=$true)][string]$Label
  )
  $found = @($Items | Where-Object { [string]$_.eventId -eq $EventId })
  if ($found.Count -ne 1) { throw "$Label expected one event $EventId, got $($found.Count)" }
  return $found[0]
}

function Assert-EventReason {
  param(
    [Parameter(Mandatory=$true)]$Event,
    [Parameter(Mandatory=$true)][string]$Expected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  $reasonText = (@($Event.decisionReason) + @($Event.verification.reasons) | ForEach-Object { [string]$_ }) -join '|'
  if ($reasonText -notlike "*$Expected*") { throw "$Label missing reason '$Expected': $($Event | ConvertTo-Json -Depth 16)" }
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "continue-preflight-$suffix"
try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"continue-preflight-$suffix") | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,"preflight-$suffix") | Out-Null

  $board = Get-Content -LiteralPath (Join-Path $caseRoot '.rekit\board.json') -Raw | ConvertFrom-Json
  $lane = @($board.lanes | Where-Object { -not [bool]$_.authority } | Select-Object -Last 1)
  $laneId = [string]$lane.id
  if ([string]::IsNullOrWhiteSpace($laneId)) { throw 'start did not create a feature lane' }

  $laneRoot = Join-Path $caseRoot ('.rekit\lanes\' + $laneId)
  $workspace = Join-Path $caseRoot ([string]$lane.workspace)
  $packetRel = ([string]$lane.workspace).Replace('\','/') + '/review-packet.md'
  Write-Utf8File -Path (Join-Path $workspace 'review-packet.md') -Text "# Continue preflight packet`r`n"

  $authorityRel = 'captures/vm_opcode_semantics_confirmed.csv'
  $authorityPath = Join-Path $caseRoot ($authorityRel -replace '/', '\')
  Write-Utf8File -Path $authorityPath -Text "opcode,semantics,status`r`nOP_EXISTING,known,confirmed`r`n"

  $restoreRel = 'captures/vm_handler_roles_confirmed.csv'
  $restorePath = Join-Path $caseRoot ($restoreRel -replace '/', '\')
  Write-Utf8File -Path $restorePath -Text "handler,role,status`r`nH_EXISTING,known,confirmed`r`n"

  $tooManyRows = @()
  for ($i = 0; $i -lt 11; $i++) {
    $tooManyRows += [ordered]@{ opcode = "OP_MANY_$i"; semantics = "many-$i"; status = 'confirmed' }
  }

  $events = @(
    [ordered]@{ eventId = 'evt-preflight-observation'; kind = 'observation'; subject = 'preflight observation'; summary = 'shared preflight observation'; evidence = 'evidence-observation-token' },
    [ordered]@{ eventId = 'evt-preflight-route-a'; kind = 'request'; subject = 'route request'; summary = 'route request once'; requestId = 'req-preflight'; targetLane = 'devirt-main'; evidence = 'evidence-route-token' },
    [ordered]@{ eventId = 'evt-preflight-route-b'; kind = 'request'; subject = 'route request duplicate'; summary = 'route request duplicate'; requestId = 'req-preflight'; targetLane = 'devirt-main'; evidence = 'evidence-route-token' },
    [ordered]@{ eventId = 'evt-preflight-authority-ok'; kind = 'candidate'; subject = 'authority ok'; summary = 'append opcode authority row'; authorityFile = $authorityRel; confidence = '0.95'; evidence = 'evidence-authority-ok'; row = [ordered]@{ opcode = 'OP_OK'; semantics = 'semantics-ok'; status = 'confirmed' } },
    [ordered]@{ eventId = 'evt-preflight-missing-evidence'; kind = 'candidate'; subject = 'missing evidence'; summary = 'reject missing evidence'; authorityFile = $authorityRel; confidence = '0.95'; row = [ordered]@{ opcode = 'OP_NO_EVID'; semantics = 'no-evidence'; status = 'confirmed' } },
    [ordered]@{ eventId = 'evt-preflight-low-confidence'; kind = 'candidate'; subject = 'low confidence'; summary = 'reject low confidence'; authorityFile = $authorityRel; confidence = '0.65'; evidence = 'evidence-low-confidence'; row = [ordered]@{ opcode = 'OP_LOW'; semantics = 'low-confidence'; status = 'confirmed' } },
    [ordered]@{ eventId = 'evt-preflight-schema-invalid'; kind = 'candidate'; subject = 'schema invalid'; summary = 'reject invalid schema'; authorityFile = $authorityRel; confidence = '0.95'; evidence = 'evidence-schema-invalid'; row = [ordered]@{ opcode = 'OP_SCHEMA'; semantics = 'missing-status' } },
    [ordered]@{ eventId = 'evt-preflight-conflict'; kind = 'candidate'; subject = 'conflict'; summary = 'reject conflicting key'; authorityFile = $authorityRel; confidence = '0.95'; evidence = 'evidence-conflict'; row = [ordered]@{ opcode = 'OP_EXISTING'; semantics = 'conflict'; status = 'confirmed' } },
    [ordered]@{ eventId = 'evt-preflight-not-allowed'; kind = 'candidate'; subject = 'not allowed'; summary = 'reject unlisted authority file'; authorityFile = 'captures/not_allowed.csv'; confidence = '0.95'; evidence = 'evidence-not-allowed'; row = [ordered]@{ opcode = 'OP_DENY'; semantics = 'deny'; status = 'confirmed' } },
    [ordered]@{ eventId = 'evt-preflight-too-many'; kind = 'candidate'; subject = 'too many rows'; summary = 'reject too many authority rows'; authorityFile = $authorityRel; confidence = '0.95'; evidence = 'evidence-too-many'; rows = $tooManyRows }
  )
  Write-JsonLines -Path (Join-Path $laneRoot 'outbox.jsonl') -Objects $events

  $beforeFiles = Save-TreeSnapshot -Path $caseRoot
  $beforeDirs = Save-TreeDirectories -Path $caseRoot
  $whatIfOut = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf',"preflight-$suffix")
  Assert-NotContainsText -Text $whatIfOut -Unexpected '.rekit/runs/' -Label 'continue what-if output'
  Assert-TreeUnchanged -Root $caseRoot -BeforeSnapshot $beforeFiles -BeforeDirectories $beforeDirs -Label 'continue what-if'

  $continueOut = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,"preflight-$suffix")
  Assert-ContainsText -Text $continueOut -Expected '.rekit/runs/' -Label 'continue output digest path'
  $match = [regex]::Match($continueOut, '\.rekit/runs/[^\s]+/digest\.md')
  if (-not $match.Success) { throw "missing digest path in continue output:`n$continueOut" }
  $digestRel = $match.Value
  $digestPath = Join-Path $caseRoot ($digestRel -replace '/', '\')
  if (-not (Test-Path -LiteralPath $digestPath)) { throw "missing digest: $digestPath" }
  $runRoot = Split-Path -Parent $digestPath
  $digest = [System.IO.File]::ReadAllText($digestPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('## route','focus lane', $laneId, '## packet refs', $packetRel, '## inputs', 'outbox.jsonl', '## outputs', 'collected: 10', 'observations: 1', 'requests: 2', 'routed: 1', 'candidates: 7', 'authorityApplied: 1', 'pendingUser: 6', '## decisions', 'authority ok', 'decision=accept', 'missing evidence', 'decision=defer', '## open risks', 'missing evidence', 'too many rows')) {
    Assert-ContainsText -Text $digest -Expected $expected -Label 'continue digest'
  }

  $statusPath = Join-Path $runRoot 'status.json'
  if (-not (Test-Path -LiteralPath $statusPath)) { throw "missing run status: $statusPath" }
  $status = Get-Content -LiteralPath $statusPath -Raw | ConvertFrom-Json
  Assert-Equals 10 ([int]$status.summary.collected) 'status collected'
  Assert-Equals 1 ([int]$status.summary.observations) 'status observations'
  Assert-Equals 2 ([int]$status.summary.requests) 'status requests'
  Assert-Equals 1 ([int]$status.summary.routed) 'status routed'
  Assert-Equals 7 ([int]$status.summary.candidates) 'status candidates'
  Assert-Equals 1 ([int]$status.summary.authorityApplied) 'status authorityApplied'
  Assert-Equals 6 ([int]$status.summary.pendingUser) 'status pendingUser'
  if (@($status.inputs).Count -lt 1 -or @($status.packetRefs).Count -lt 1 -or @($status.openRisks).Count -lt 1) { throw "status.json missing digest fields: $($status | ConvertTo-Json -Depth 10)" }

  $authorityText = [System.IO.File]::ReadAllText($authorityPath, [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $authorityText -Expected 'OP_OK' -Label 'authority csv append'
  foreach ($unexpected in @('OP_NO_EVID','OP_LOW','OP_SCHEMA','OP_DENY','OP_MANY_0')) {
    Assert-NotContainsText -Text $authorityText -Unexpected $unexpected -Label 'authority csv rejected row'
  }

  $factsRoot = Join-Path $caseRoot '.rekit\facts'
  $candidates = @(Read-JsonLines -Path (Join-Path $factsRoot 'candidates.jsonl'))
  $publications = @(Read-JsonLines -Path (Join-Path $factsRoot 'publications.jsonl'))
  $decisions = @(Read-JsonLines -Path (Join-Path $factsRoot 'decisions.jsonl'))

  $publication = Get-EventById -Items $publications -EventId 'evt-preflight-authority-ok' -Label 'authority publication'
  Assert-Equals $authorityRel ([string]$publication.authorityFile) 'authority publication file'
  $authorityDecision = Get-EventById -Items $decisions -EventId 'evt-preflight-authority-ok' -Label 'authority decision'
  Assert-Equals 'accept' ([string]$authorityDecision.decision) 'authority decision value'
  if (@($authorityDecision.writes).Count -ne 1) { throw "authority decision missing writes: $($authorityDecision | ConvertTo-Json -Depth 16)" }
  $write = @($authorityDecision.writes)[0]
  foreach ($rel in @([string]$write.backup, [string]$write.diff)) {
    $path = Join-Path $caseRoot ($rel -replace '/', '\')
    if (-not (Test-Path -LiteralPath $path)) { throw "missing authority write artifact: $rel" }
  }

  Assert-EventReason -Event (Get-EventById -Items $candidates -EventId 'evt-preflight-missing-evidence' -Label 'missing evidence candidate') -Expected 'missing evidence' -Label 'missing evidence gate'
  Assert-EventReason -Event (Get-EventById -Items $candidates -EventId 'evt-preflight-low-confidence' -Label 'low confidence candidate') -Expected 'confidence below threshold' -Label 'confidence gate'
  Assert-EventReason -Event (Get-EventById -Items $candidates -EventId 'evt-preflight-schema-invalid' -Label 'schema invalid candidate') -Expected 'schema invalid' -Label 'schema gate'
  Assert-EventReason -Event (Get-EventById -Items $candidates -EventId 'evt-preflight-conflict' -Label 'conflict candidate') -Expected 'authority conflict' -Label 'conflict gate'
  Assert-EventReason -Event (Get-EventById -Items $candidates -EventId 'evt-preflight-not-allowed' -Label 'allowlist candidate') -Expected 'authority file is not allowed' -Label 'allowlist gate'
  $tooMany = Get-EventById -Items $candidates -EventId 'evt-preflight-too-many' -Label 'too many candidate'
  Assert-Equals 'accepted' ([string]$tooMany.verifierVerdict) 'accepted verifier verdict before max rows gate'
  Assert-EventReason -Event $tooMany -Expected 'too many rows' -Label 'max rows gate'

  $tasks = @(Read-JsonLines -Path (Join-Path $caseRoot '.rekit\lanes\devirt-main\tasks.jsonl') | Where-Object { [string]$_.requestId -eq 'req-preflight' -and [string]$_.sourceLane -eq $laneId })
  $inbox = @(Read-JsonLines -Path (Join-Path $caseRoot '.rekit\lanes\devirt-main\inbox.jsonl') | Where-Object { [string]$_.requestId -eq 'req-preflight' -and [string]$_.sourceLane -eq $laneId })
  Assert-Equals 1 $tasks.Count 'routed task idempotency'
  Assert-Equals 1 $inbox.Count 'routed inbox idempotency'

  . (Join-Path $RekitRoot 'lib\Manifest.ps1')
  . (Join-Path $RekitRoot 'lib\Instance.ps1')
  . (Join-Path $RekitRoot 'lib\Review.ps1')
  . (Join-Path $RekitRoot 'lib\B3.Core.ps1')
  . (Join-Path $RekitRoot 'lib\B3.State.ps1')
  . (Join-Path $RekitRoot 'lib\B3.Policy.ps1')
  . (Join-Path $RekitRoot 'lib\B3.Lane.ps1')
  . (Join-Path $RekitRoot 'lib\B3.Auto.ps1')

  $restoreBefore = [System.IO.File]::ReadAllText($restorePath, [System.Text.Encoding]::UTF8)
  $restoreRunRoot = Join-Path $caseRoot '.rekit\runs\restore-injected'
  Ensure-RekitDirectory $restoreRunRoot
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $policy = Get-RekitPolicy -CaseRoot $caseRoot

  $missingVerifierEvent = [pscustomobject]@{
    eventId = 'evt-preflight-missing-verifier'
    kind = 'candidate'
    lane = $laneId
    subject = 'missing verifier'
    summary = 'reject missing accepted verifier verdict'
    authorityFile = $restoreRel
    confidence = '0.95'
    evidence = 'evidence-missing-verifier'
    row = [pscustomobject]@{ handler = 'H_NO_VERIFIER'; role = 'no-verifier'; status = 'confirmed' }
  }
  $missingVerifier = Invoke-RekitAuthorityAppend -CaseRoot $caseRoot -Manifest $manifest -Policy $policy -Event $missingVerifierEvent -RunRoot (Join-Path $caseRoot '.rekit\runs\missing-verifier')
  if ([bool]$missingVerifier.Applied) { throw "missing verifier candidate unexpectedly applied: $($missingVerifier | ConvertTo-Json -Depth 10)" }
  Assert-Equals 'missing accepted verifier verdict' ([string]$missingVerifier.Reason) 'accepted verifier gate'
  Assert-NotContainsText -Text ([System.IO.File]::ReadAllText($restorePath, [System.Text.Encoding]::UTF8)) -Unexpected 'H_NO_VERIFIER' -Label 'accepted verifier rejected row'

  $restoreEvent = [pscustomobject]@{
    eventId = 'evt-preflight-restore'
    kind = 'candidate'
    lane = $laneId
    subject = 'restore'
    summary = 'restore backup after csv failure'
    authorityFile = $restoreRel
    confidence = '0.95'
    evidence = 'evidence-restore'
    verifier = 'manual-review'
    verifierVerdict = 'accepted'
    row = [pscustomobject]@{ handler = 'H_RESTORE'; role = 'restore'; status = 'confirmed' }
  }
  function Import-Csv {
    param(
      [string[]]$LiteralPath,
      [string[]]$Path
    )
    $target = if ($LiteralPath -and $LiteralPath.Count -gt 0) { [string]$LiteralPath[0] } elseif ($Path -and $Path.Count -gt 0) { [string]$Path[0] } else { '' }
    if (-not [string]::IsNullOrWhiteSpace($target) -and (Test-Path -LiteralPath $target)) {
      $text = [System.IO.File]::ReadAllText($target, [System.Text.Encoding]::UTF8)
      if ($text -like '*H_RESTORE*') { throw 'injected csv validation failure' }
    }
    if ($LiteralPath -and $LiteralPath.Count -gt 0) { return Microsoft.PowerShell.Utility\Import-Csv -LiteralPath $LiteralPath }
    return Microsoft.PowerShell.Utility\Import-Csv -Path $Path
  }
  try {
    Invoke-RekitAuthorityAppend -CaseRoot $caseRoot -Manifest $manifest -Policy $policy -Event $restoreEvent -RunRoot $restoreRunRoot | Out-Null
    throw 'expected injected csv validation failure'
  } catch {
    Assert-ContainsText -Text ([string]$_) -Expected 'restored backup' -Label 'csv restore failure'
  } finally {
    if (Test-Path function:\Import-Csv) { Remove-Item function:\Import-Csv -Force }
  }
  $restoreAfter = [System.IO.File]::ReadAllText($restorePath, [System.Text.Encoding]::UTF8)
  Assert-Equals $restoreBefore $restoreAfter 'authority csv restored after validation failure'
  $restoreBackup = Join-Path $restoreRunRoot 'backups\captures\vm_handler_roles_confirmed.csv'
  if (-not (Test-Path -LiteralPath $restoreBackup)) { throw "missing restore backup: $restoreBackup" }

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  'continue preflight smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
