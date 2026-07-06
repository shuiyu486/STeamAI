function Get-RekitBoardPaths {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $case = [System.IO.Path]::GetFullPath($CaseRoot)
  $root = Join-Path $case '.rekit'
  return [pscustomobject]@{
    CaseRoot = $case
    Root = $root
    Board = Join-Path $root 'board.json'
    Policy = Join-Path $root 'policy.yml'
    Lanes = Join-Path $root 'lanes'
    Facts = Join-Path $root 'facts'
    Runs = Join-Path $root 'runs'
    Reviews = Join-Path $root 'reviews'
    Backups = Join-Path $root 'backups'
    Observations = Join-Path $root 'facts\observations.jsonl'
    Candidates = Join-Path $root 'facts\candidates.jsonl'
    Requests = Join-Path $root 'facts\requests.jsonl'
    Publications = Join-Path $root 'facts\publications.jsonl'
    Decisions = Join-Path $root 'facts\decisions.jsonl'
    Hypotheses = Join-Path $root 'facts\hypotheses.jsonl'
    Verifications = Join-Path $root 'facts\verifications.jsonl'
    Interventions = Join-Path $root 'facts\interventions.jsonl'
    Rollbacks = Join-Path $root 'facts\rollbacks.jsonl'
  }
}

function Save-RekitBoard {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [Parameter(Mandatory=$true)]$Manifest
  )
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  $lanes = @()
  foreach ($dir in (Get-RekitLaneDirectories -CaseRoot $CaseRoot)) {
    $lane = Read-RekitJsonFile -Path (Join-Path $dir.FullName 'lane.json')
    if ($null -ne $lane) {
      $lanes += [ordered]@{ id = $lane.id; type = $lane.type; title = $lane.title; status = $lane.status; authority = $lane.authority; workspace = $lane.workspace; updatedAt = $lane.updatedAt }
    }
  }
  $authorityLane = Get-RekitAuthorityLaneType -Manifest $Manifest
  $board = [ordered]@{
    schemaVersion = 1
    caseRoot = [System.IO.Path]::GetFullPath($CaseRoot)
    repoRoot = [System.IO.Path]::GetFullPath($RepoRoot)
    pack = $Manifest.Pack
    automationMode = (Get-RekitPolicy -CaseRoot $CaseRoot).automationMode
    defaultAuthorityLane = $authorityLane.Id
    lanes = $lanes
    factsRoot = '.rekit/facts'
    updatedAt = New-RekitIsoTime
  }
  Write-RekitJsonFile -Path $paths.Board -Object $board
  return $board
}

function Ensure-RekitBoard {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [switch]$CreateDefaultLane
  )
  $case = [System.IO.Path]::GetFullPath($CaseRoot)
  [void](Assert-RekitAttachedCase -Target $case -RepoRoot $RepoRoot -Pack $Pack)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $paths = Get-RekitBoardPaths -CaseRoot $case
  foreach ($dir in @($paths.Root,$paths.Lanes,$paths.Facts,$paths.Runs,$paths.Reviews,$paths.Backups)) { Ensure-RekitDirectory $dir }
  foreach ($file in @($paths.Observations,$paths.Candidates,$paths.Requests,$paths.Publications,$paths.Decisions,$paths.Hypotheses,$paths.Verifications,$paths.Interventions,$paths.Rollbacks)) {
    if (-not (Test-Path -LiteralPath $file)) { [System.IO.File]::WriteAllText($file, '', [System.Text.UTF8Encoding]::new($false)) }
  }
  [void](Ensure-RekitPolicyFile -CaseRoot $case)
  $authorityLane = Get-RekitAuthorityLaneType -Manifest $manifest
  $authorityLaneId = ConvertTo-RekitLaneId -Type $authorityLane.Id -Name ''
  if ($CreateDefaultLane -and -not (Test-Path -LiteralPath (Join-Path (Get-RekitLanePath -CaseRoot $case -LaneId $authorityLaneId) 'lane.json'))) {
    New-RekitLane -CaseRoot $case -RepoRoot $RepoRoot -Manifest $manifest -Type $authorityLane.Id -Name '' | Out-Null
  }
  return Save-RekitBoard -CaseRoot $case -RepoRoot $RepoRoot -Manifest $manifest
}

function Get-RekitFactFilePath {
  param(
    [Parameter(Mandatory=$true)]$Paths,
    [Parameter(Mandatory=$true)][ValidateSet('observation','candidate','request','publication','decision','hypothesis','verification','intervention','rollback')][string]$Kind
  )
  switch ($Kind) {
    'observation' { return $Paths.Observations }
    'candidate' { return $Paths.Candidates }
    'request' { return $Paths.Requests }
    'publication' { return $Paths.Publications }
    'decision' { return $Paths.Decisions }
    'hypothesis' { return $Paths.Hypotheses }
    'verification' { return $Paths.Verifications }
    'intervention' { return $Paths.Interventions }
    'rollback' { return $Paths.Rollbacks }
  }
}

function Add-RekitFactEvent {
  <#
    Append-only fact ledger event. Lets a main agent persist an observation/candidate/request/publication/decision
    without going through the continue auto flow. Validates enums + lane existence (when Board provided), dedups by
    eventId, routes to the matching .rekit/facts/*.jsonl. Does not write authority files or kit templates.
    Field set aligns with docs/evidence-ledger.md draft: schemaVersion/eventId/kind/time/actor/lane/subject/summary/
    evidenceRefs/related/status/risk/confidence + per-kind extensions via -Extra.
  #>
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][ValidateSet('observation','candidate','request','publication','decision','hypothesis','verification','intervention','rollback')][string]$Kind,
    [Parameter(Mandatory=$true)][string]$Lane,
    [string]$Subject = '',
    [string]$Summary = '',
    [string]$Actor = '',
    [string]$Risk = '',
    [string]$Related = '',
    [string]$Confidence = '',
    [string]$Decision = '',
    [string]$Reason = '',
    [string]$Status = '',
    [string]$Target = '',
    [string]$Verifier = '',
    [string]$Verdict = '',
    [string]$Action = '',
    [string]$ApprovedBy = '',
    [string]$Scope = '',
    [string]$Expires = '',
    [string[]]$EvidenceRefs = @(),
    [string]$EventId = '',
    [hashtable]$Extra = $null,
    $Board = $null
  )
  $validConfidence = @('low','medium','high')
  $validDecision = @('accept','reject','defer','supersede')
  $validStatus = @('open','accepted','rejected','superseded','resolved','deferred','pending-gate','confirmed','needs_more_evidence')
  $validVerifier = @('manual-review','schema-check','focused-trace','parity','cross-run','tool-review')
  $validVerdict = @('accepted','rejected','inconclusive','needs-more-evidence')
  $validInterventionAction = @('override','rollback','heavy-tool-approval','schema-migration','external-side-effect')
  if (-not [string]::IsNullOrWhiteSpace($Confidence) -and $validConfidence -notcontains $Confidence) {
    throw "invalid Confidence '$Confidence'; allowed: $($validConfidence -join ',')"
  }
  if ($Kind -eq 'decision' -and -not [string]::IsNullOrWhiteSpace($Decision) -and $validDecision -notcontains $Decision) {
    throw "invalid Decision '$Decision'; allowed: $($validDecision -join ',')"
  }
  if ($Kind -eq 'verification' -and -not [string]::IsNullOrWhiteSpace($Verdict) -and $validVerdict -notcontains $Verdict) {
    throw "invalid Verdict '$Verdict'; allowed: $($validVerdict -join ',')"
  }
  if ($Kind -eq 'verification' -and -not [string]::IsNullOrWhiteSpace($Verifier) -and $validVerifier -notcontains $Verifier) {
    throw "invalid Verifier '$Verifier'; allowed: $($validVerifier -join ',')"
  }
  if ($Kind -eq 'intervention' -and -not [string]::IsNullOrWhiteSpace($Action) -and $validInterventionAction -notcontains $Action) {
    throw "invalid Action '$Action'; allowed: $($validInterventionAction -join ',')"
  }
  if (-not [string]::IsNullOrWhiteSpace($Status) -and $validStatus -notcontains $Status) {
    throw "invalid Status '$Status'; allowed: $($validStatus -join ',')"
  }
  if ($EvidenceRefs.Count -gt 0) {
    foreach ($r in $EvidenceRefs) { if ([string]::IsNullOrWhiteSpace([string]$r)) { throw 'EvidenceRefs contains empty element' } }
  }
  if ($null -ne $Board) {
    $laneIds = @()
    $lanes = $null
    if ($Board -is [System.Collections.IDictionary]) {
      if ($Board.Contains('lanes')) { $lanes = $Board['lanes'] }
    } elseif ($Board.PSObject.Properties.Name -contains 'lanes') {
      $lanes = $Board.lanes
    }
    if ($null -ne $lanes) { $laneIds = @($lanes | ForEach-Object { [string]$_.id }) }
    if ($laneIds.Count -gt 0 -and $laneIds -notcontains $Lane) {
      throw "unknown lane '$Lane'; known: $($laneIds -join ',')"
    }
  }

  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  $file = Get-RekitFactFilePath -Paths $paths -Kind $Kind
  if (-not (Test-Path -LiteralPath $file)) { [System.IO.File]::WriteAllText($file, '', [System.Text.UTF8Encoding]::new($false)) }

  $event = [ordered]@{
    schemaVersion = 1
    kind = $Kind
    lane = $Lane
    subject = $Subject
    summary = $Summary
    createdAt = New-RekitIsoTime
  }
  if (-not [string]::IsNullOrWhiteSpace($Actor)) { $event['actor'] = $Actor }
  if (-not [string]::IsNullOrWhiteSpace($Risk)) { $event['risk'] = $Risk }
  if (-not [string]::IsNullOrWhiteSpace($Related)) { $event['related'] = @(Split-RekitScalarList $Related) }
  if (-not [string]::IsNullOrWhiteSpace($Confidence)) { $event['confidence'] = $Confidence }
  if (-not [string]::IsNullOrWhiteSpace($Decision)) { $event['decision'] = $Decision }
  if (-not [string]::IsNullOrWhiteSpace($Reason)) { $event['reason'] = $Reason }
  if (-not [string]::IsNullOrWhiteSpace($Status)) { $event['status'] = $Status }
  if ($EvidenceRefs.Count -gt 0) { $event['evidenceRefs'] = @($EvidenceRefs) }
  if (-not [string]::IsNullOrWhiteSpace($Target)) { $event['target'] = $Target }
  if ($Kind -eq 'verification') {
    if (-not [string]::IsNullOrWhiteSpace($Verifier)) { $event['verifier'] = $Verifier }
    if (-not [string]::IsNullOrWhiteSpace($Verdict)) { $event['verdict'] = $Verdict }
  }
  if ($Kind -eq 'intervention') {
    if (-not [string]::IsNullOrWhiteSpace($Action)) { $event['action'] = $Action }
    if (-not [string]::IsNullOrWhiteSpace($ApprovedBy)) { $event['approvedBy'] = $ApprovedBy }
    if (-not [string]::IsNullOrWhiteSpace($Scope)) { $event['scope'] = $Scope }
    if (-not [string]::IsNullOrWhiteSpace($Expires)) { $event['expires'] = $Expires }
  }
  if ($null -ne $Extra) { foreach ($k in $Extra.Keys) { $event[$k] = $Extra[$k] } }

  $known = Get-RekitKnownEventIds -CaseRoot $CaseRoot
  if ([string]::IsNullOrWhiteSpace($EventId)) {
    $json = ConvertTo-RekitJsonLine $event
    $EventId = 'evt-' + (Get-RekitTextHash ($Lane + '|' + $json)).Substring(0,16)
  }
  $event['eventId'] = $EventId
  if ($known.ContainsKey($EventId)) {
    return [pscustomobject]@{ Applied = $false; EventId = $EventId; Kind = $Kind; Reason = 'duplicate eventId' }
  }

  $line = ConvertTo-RekitJsonLine $event
  [System.IO.File]::AppendAllText($file, ($line + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  return [pscustomobject]@{ Applied = $true; EventId = $EventId; Kind = $Kind; Path = (Join-RekitRelativePath -Root $CaseRoot -Path $file) }
}
