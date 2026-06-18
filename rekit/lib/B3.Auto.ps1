function Get-RekitKnownEventIds {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  $ids = @{}
  foreach ($file in @($paths.Observations,$paths.Candidates,$paths.Requests,$paths.Publications,$paths.Decisions)) {
    foreach ($item in (Read-RekitJsonLines -Path $file)) {
      if ($item.PSObject.Properties['eventId'] -and -not [string]::IsNullOrWhiteSpace([string]$item.eventId)) { $ids[[string]$item.eventId] = $true }
    }
  }
  return $ids
}

function Ensure-RekitEventId {
  param(
    [Parameter(Mandatory=$true)]$Event,
    [Parameter(Mandatory=$true)][string]$LaneId
  )
  if ($Event.PSObject.Properties['eventId'] -and -not [string]::IsNullOrWhiteSpace([string]$Event.eventId)) { return $Event }
  $json = ConvertTo-RekitJsonLine $Event
  $id = 'evt-' + (Get-RekitTextHash ($LaneId + '|' + $json)).Substring(0,16)
  $Event | Add-Member -NotePropertyName eventId -NotePropertyValue $id -Force
  return $Event
}

function Get-RekitEventConfidence {
  param($Event)
  if (-not $Event.PSObject.Properties['confidence']) { return 0.0 }
  $text = ([string]$Event.confidence).Trim().ToLowerInvariant()
  $num = 0.0
  if ([double]::TryParse($text, [ref]$num)) { return $num }
  switch ($text) {
    'high' { return 0.95 }
    'medium_high' { return 0.82 }
    'medium-high' { return 0.82 }
    'medium' { return 0.65 }
    'medium_low' { return 0.45 }
    'medium-low' { return 0.45 }
    'low' { return 0.25 }
    default { return 0.0 }
  }
}

function Get-RekitEventEvidenceItems {
  param($Event)
  if (-not $Event.PSObject.Properties['evidence']) { return @() }
  $e = $Event.evidence
  if ($null -eq $e) { return @() }
  if ($e -is [array]) { return @($e | ForEach-Object { [string]$_ } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }) }
  return @(Split-RekitScalarList ([string]$e))
}

function Test-RekitEventEvidence {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Event
  )
  $items = @(Get-RekitEventEvidenceItems $Event)
  if ($items.Count -eq 0) { return $false }
  foreach ($item in $items) {
    if ($item -match '^[A-Za-z]:\\' -or $item.StartsWith('/') -or $item.Contains('\') -or $item.Contains('/')) {
      try {
        $path = if ([System.IO.Path]::IsPathRooted($item)) { [System.IO.Path]::GetFullPath($item) } else { Join-RekitPath -Root $CaseRoot -RelativePath $item }
        if (Test-Path -LiteralPath $path) { return $true }
      } catch {}
    } else {
      if ($item.Length -ge 8) { return $true }
    }
  }
  return $false
}

function Test-RekitCandidateCsvSchema {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Event,
    [Parameter(Mandatory=$true)][string]$AuthorityFile
  )
  if ([string]::IsNullOrWhiteSpace($AuthorityFile) -or -not $AuthorityFile.ToLowerInvariant().EndsWith('.csv')) { return $true }
  try {
    $path = Join-RekitPath -Root $CaseRoot -RelativePath $AuthorityFile
  } catch {
    return $false
  }
  if (-not (Test-Path -LiteralPath $path)) { return $false }
  $header = @((Get-Content -LiteralPath $path -TotalCount 1) -split ',' | ForEach-Object { $_.Trim().Trim('"') })
  if ($header.Count -eq 0 -or [string]::IsNullOrWhiteSpace([string]$header[0])) { return $false }
  $rows = @(Get-RekitCandidateRows $Event)
  if ($rows.Count -eq 0) { return $false }
  foreach ($row in $rows) {
    if ($row -is [string]) {
      if ([string]::IsNullOrWhiteSpace($row) -or $row.Contains("`r") -or $row.Contains("`n")) { return $false }
      try {
        $parsed = @(((($header -join ',') + "`r`n" + $row) | ConvertFrom-Csv))
        if ($parsed.Count -ne 1) { return $false }
      } catch { return $false }
      continue
    }
    foreach ($column in $header) {
      if (-not $row.PSObject.Properties[$column]) { return $false }
    }
  }
  return $true
}

function Test-RekitEventVerified {
  param(
    [Parameter(Mandatory=$true)]$Policy,
    [Parameter(Mandatory=$true)]$Event
  )
  if (-not (Test-RekitPolicyBool -Policy $Policy -Name 'requireVerifier' -Default $true)) { return $true }
  $verifier = if ($Event.PSObject.Properties['verifier']) { [string]$Event.verifier } else { '' }
  $verdict = if ($Event.PSObject.Properties['verifierVerdict']) { [string]$Event.verifierVerdict } else { '' }
  return ((-not [string]::IsNullOrWhiteSpace($verifier)) -and [string]::Equals($verdict, 'accepted', [System.StringComparison]::OrdinalIgnoreCase))
}

function Get-RekitAuthorityFiles {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Manifest
  )
  $files = @('captures/vm_opcode_semantics_confirmed.csv','captures/vm_handler_roles_confirmed.csv','references/vmp-re/task-handoff.md')
  $devirt = Get-RekitLaneType -Manifest $Manifest -Type 'devirt-main'
  $files += @($devirt.CanWrite | Where-Object { $_ -ne 'own-workspace' })
  return @($files | Select-Object -Unique)
}

function Test-RekitEventConflict {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Event,
    [string]$AuthorityFile = ''
  )
  if (-not [string]::IsNullOrWhiteSpace($AuthorityFile) -and ($AuthorityFile.ToLowerInvariant().EndsWith('.csv'))) {
    try {
      $path = Join-RekitPath -Root $CaseRoot -RelativePath $AuthorityFile
    } catch {
      return $true
    }
    if (-not (Test-Path -LiteralPath $path)) { return $false }
    $rowKey = Get-RekitCandidateRowKey -Event $Event -CsvPath $path
    if ([string]::IsNullOrWhiteSpace($rowKey)) { return $false }
      $rows = @(Import-Csv -LiteralPath $path)
    if ($rows.Count -eq 0) { return $false }
    $first = @($rows[0].PSObject.Properties.Name)[0]
    foreach ($row in $rows) {
      if ([string]$row.$first -eq $rowKey) { return $true }
    }
  }
  return $false
}

function Get-RekitCandidateAuthorityFile {
  param($Event)
  foreach ($name in @('authorityFile','authorityCsv','targetFile','file')) {
    if ($Event.PSObject.Properties[$name] -and -not [string]::IsNullOrWhiteSpace([string]$Event.$name)) { return ([string]$Event.$name).Replace('\','/') }
  }
  return ''
}

function Get-RekitCandidateRows {
  param($Event)
  foreach ($name in @('rows','row','csvRow')) {
    if ($Event.PSObject.Properties[$name] -and $null -ne $Event.$name) {
      $value = $Event.$name
      if ($value -is [array]) { return @($value) }
      return @($value)
    }
  }
  return @()
}

function Get-RekitCandidateRowKey {
  param(
    [Parameter(Mandatory=$true)]$Event,
    [Parameter(Mandatory=$true)][string]$CsvPath
  )
  $rows = @(Get-RekitCandidateRows $Event)
  if ($rows.Count -eq 0) { return '' }
  $candidate = $rows[0]
  $header = @((Get-Content -LiteralPath $CsvPath -TotalCount 1) -split ',')
  if ($header.Count -eq 0) { return '' }
  $first = $header[0]
  if ($candidate -is [string]) { return (($candidate -split ',', 2)[0]).Trim('"') }
  if ($candidate.PSObject.Properties[$first]) { return [string]$candidate.$first }
  return ''
}

function Convert-RekitCandidateRowToCsvLine {
  param(
    [Parameter(Mandatory=$true)]$Row,
    [Parameter(Mandatory=$true)][string[]]$Header
  )
  if ($Row -is [string]) { return $Row.TrimEnd("`r","`n") }
  $ordered = [ordered]@{}
  foreach ($h in $Header) {
    $value = ''
    if ($Row.PSObject.Properties[$h]) { $value = [string]$Row.$h }
    $ordered[$h] = $value
  }
  return (($ordered | ForEach-Object { [pscustomobject]$_ }) | ConvertTo-Csv -NoTypeInformation | Select-Object -Skip 1 -First 1)
}

function Invoke-RekitAuthorityAppend {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Manifest,
    [Parameter(Mandatory=$true)]$Policy,
    [Parameter(Mandatory=$true)]$Event,
    [Parameter(Mandatory=$true)][string]$RunRoot
  )
  $authorityFile = Get-RekitCandidateAuthorityFile $Event
  if ([string]::IsNullOrWhiteSpace($authorityFile)) { return [pscustomobject]@{ Applied = $false; Reason = 'no authority file' } }
  $allowed = @(Get-RekitAuthorityFiles -CaseRoot $CaseRoot -Manifest $Manifest)
  if ($allowed -notcontains $authorityFile) { return [pscustomobject]@{ Applied = $false; Reason = "authority file is not allowed: $authorityFile" } }
  if ([string]$Policy.authorityAutoAppend -eq 'never') { return [pscustomobject]@{ Applied = $false; Reason = 'authority auto append disabled' } }
  $confidence = Get-RekitEventConfidence $Event
  $minConfidence = Get-RekitPolicyNumber -Policy $Policy -Name 'minConfidence' -Default 0.90
  if ($confidence -lt $minConfidence) { return [pscustomobject]@{ Applied = $false; Reason = "confidence below threshold: $confidence < $minConfidence" } }
  if ((Test-RekitPolicyBool -Policy $Policy -Name 'requireEvidence' -Default $true) -and -not (Test-RekitEventEvidence -CaseRoot $CaseRoot -Event $Event)) { return [pscustomobject]@{ Applied = $false; Reason = 'missing evidence' } }
  if (-not (Test-RekitEventVerified -Policy $Policy -Event $Event)) { return [pscustomobject]@{ Applied = $false; Reason = 'missing accepted verifier verdict' } }
  $target = Join-RekitPath -Root $CaseRoot -RelativePath $authorityFile
  if (-not (Test-Path -LiteralPath $target)) { return [pscustomobject]@{ Applied = $false; Reason = "missing authority file: $authorityFile" } }
  if (-not $authorityFile.ToLowerInvariant().EndsWith('.csv')) { return [pscustomobject]@{ Applied = $false; Reason = 'only csv authority append is automated' } }
  if ((Test-RekitPolicyBool -Policy $Policy -Name 'requireSchemaValid' -Default $true) -and -not (Test-RekitCandidateCsvSchema -CaseRoot $CaseRoot -Event $Event -AuthorityFile $authorityFile)) { return [pscustomobject]@{ Applied = $false; Reason = 'candidate row does not match authority csv schema' } }
  if ((Test-RekitPolicyBool -Policy $Policy -Name 'requireNoConflict' -Default $true) -and (Test-RekitEventConflict -CaseRoot $CaseRoot -Event $Event -AuthorityFile $authorityFile)) { return [pscustomobject]@{ Applied = $false; Reason = 'authority key conflict' } }
  $rows = @(Get-RekitCandidateRows $Event)
  if ($rows.Count -eq 0) { return [pscustomobject]@{ Applied = $false; Reason = 'no candidate rows' } }
  foreach ($row in $rows) {
    if ($row -is [string] -and ($row.Contains("`r") -or $row.Contains("`n"))) { return [pscustomobject]@{ Applied = $false; Reason = 'candidate row contains newline' } }
  }
  $maxRows = [int](Get-RekitPolicyNumber -Policy $Policy -Name 'maxAuthorityRowsPerRun' -Default 10)
  if ($rows.Count -gt $maxRows) { return [pscustomobject]@{ Applied = $false; Reason = "too many rows: $($rows.Count) > $maxRows" } }
  $old = [System.IO.File]::ReadAllText($target, [System.Text.Encoding]::UTF8)
  $header = @(($old -split "`r?`n", 2)[0] -split ',')
  $lines = @()
  foreach ($row in $rows) { $lines += Convert-RekitCandidateRowToCsvLine -Row $row -Header $header }
  $new = $old.TrimEnd("`r","`n") + "`r`n" + (($lines -join "`r`n") + "`r`n")
  $backup = Join-Path (Join-Path $RunRoot 'backups') $authorityFile
  Ensure-RekitDirectory (Split-Path -Parent $backup)
  Copy-Item -LiteralPath $target -Destination $backup -Force
  if ((Test-RekitPolicyBool -Policy $Policy -Name 'requireBackup' -Default $true) -and -not (Test-Path -LiteralPath $backup)) { return [pscustomobject]@{ Applied = $false; Reason = 'backup was not created' } }
  $diffRoot = Join-Path $RunRoot 'diffs'
  Ensure-RekitDirectory $diffRoot
  $diff = if (Get-Command New-RekitBoundedDiffText -ErrorAction SilentlyContinue) { New-RekitBoundedDiffText -OldLabel ("before/" + $authorityFile) -OldText $old -NewLabel ("after/" + $authorityFile) -NewText $new } else { "appended $($lines.Count) row(s) to $authorityFile`r`n" }
  $diffPath = Join-Path $diffRoot (($authorityFile -replace '[\/:*?"<>|]', '_') + '.diff')
  [System.IO.File]::WriteAllText($diffPath, $diff, [System.Text.UTF8Encoding]::new($false))
  if ((Test-RekitPolicyBool -Policy $Policy -Name 'requireDiff' -Default $true) -and -not (Test-Path -LiteralPath $diffPath)) { return [pscustomobject]@{ Applied = $false; Reason = 'diff was not created' } }
  [System.IO.File]::WriteAllText($target, $new, [System.Text.UTF8Encoding]::new($false))
  try { Import-Csv -LiteralPath $target | Out-Null } catch {
    Copy-Item -LiteralPath $backup -Destination $target -Force
    throw "authority csv validation failed after append; restored backup: $_"
  }
  return [pscustomobject]@{ Applied = $true; AuthorityFile = $authorityFile; Rows = $lines.Count; Backup = (Join-RekitRelativePath -Root $CaseRoot -Path $backup); Diff = (Join-RekitRelativePath -Root $CaseRoot -Path $diffPath) }
}

function Invoke-RekitRuleVerifier {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Manifest,
    [Parameter(Mandatory=$true)]$Policy,
    [Parameter(Mandatory=$true)]$Event
  )
  $kind = ([string]$Event.kind).ToLowerInvariant()
  $confidence = Get-RekitEventConfidence $Event
  $hasEvidence = Test-RekitEventEvidence -CaseRoot $CaseRoot -Event $Event
  $authorityFile = Get-RekitCandidateAuthorityFile $Event
  $conflict = $false
  $schemaValid = $true
  if (-not [string]::IsNullOrWhiteSpace($authorityFile)) {
    $conflict = Test-RekitEventConflict -CaseRoot $CaseRoot -Event $Event -AuthorityFile $authorityFile
    $schemaValid = Test-RekitCandidateCsvSchema -CaseRoot $CaseRoot -Event $Event -AuthorityFile $authorityFile
  }
  $minConfidence = Get-RekitPolicyNumber -Policy $Policy -Name 'minConfidence' -Default 0.90
  $reasons = New-Object System.Collections.Generic.List[string]
  if ($kind -eq 'candidate' -and (Test-RekitPolicyBool -Policy $Policy -Name 'requireEvidence' -Default $true) -and -not $hasEvidence) { $reasons.Add('missing evidence') }
  if ($kind -eq 'candidate' -and $confidence -lt $minConfidence) { $reasons.Add("confidence below threshold: $confidence < $minConfidence") }
  if ($kind -eq 'candidate' -and (Test-RekitPolicyBool -Policy $Policy -Name 'requireSchemaValid' -Default $true) -and -not $schemaValid) { $reasons.Add('schema invalid') }
  if ($conflict) { $reasons.Add('authority conflict') }
  $verdict = if ($reasons.Count -eq 0) { 'accepted' } else { 'rejected' }
  return [pscustomobject]@{
    verifier = 'rule-verifier'
    verdict = $verdict
    confidence = $confidence
    hasEvidence = $hasEvidence
    schemaValid = $schemaValid
    conflict = $conflict
    reasons = @($reasons)
    time = New-RekitIsoTime
  }
}

function Set-RekitEventVerification {
  param(
    [Parameter(Mandatory=$true)]$Event,
    [Parameter(Mandatory=$true)]$Verification
  )
  $Event | Add-Member -NotePropertyName verifier -NotePropertyValue $Verification.verifier -Force
  $Event | Add-Member -NotePropertyName verifierVerdict -NotePropertyValue $Verification.verdict -Force
  $Event | Add-Member -NotePropertyName verifierConfidence -NotePropertyValue $Verification.confidence -Force
  $Event | Add-Member -NotePropertyName verification -NotePropertyValue $Verification -Force
  return $Event
}

function New-RekitDecision {
  param(
    [Parameter(Mandatory=$true)]$Event,
    [Parameter(Mandatory=$true)][string]$Action,
    [Parameter(Mandatory=$true)][string]$Reason,
    [string]$RunId = '',
    $Extra = $null
  )
  $decision = [ordered]@{
    eventId = [string]$Event.eventId
    kind = [string]$Event.kind
    lane = [string]$Event.lane
    action = $Action
    reason = $Reason
    runId = $RunId
    time = New-RekitIsoTime
  }
  if ($null -ne $Extra) { $decision['extra'] = $Extra }
  return $decision
}

function Get-RekitLaneOutputEvents {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Lane
  )
  $laneRoot = Get-RekitLanePath -CaseRoot $CaseRoot -LaneId $Lane.id
  $workspace = Get-RekitLaneWorkspacePath -CaseRoot $CaseRoot -Lane $Lane
  $events = @()
  foreach ($file in @((Join-Path $laneRoot 'outbox.jsonl'), (Join-Path $workspace 'observations.jsonl'), (Join-Path $workspace 'requests.jsonl'), (Join-Path $workspace 'candidates.jsonl'), (Join-Path $workspace 'publications.jsonl'))) {
    foreach ($event in (Read-RekitJsonLines -Path $file)) { $events += (Ensure-RekitEventId -Event $event -LaneId $Lane.id) }
  }
  $lowering = Join-Path $workspace 'lowering_requests.csv'
  if (Test-Path -LiteralPath $lowering) {
    foreach ($row in (Import-Csv -LiteralPath $lowering)) {
      $status = ([string]$row.status).Trim().ToLowerInvariant()
      if (@('resolved','done','closed','accepted','rejected') -contains $status) { continue }
      $summary = if (-not [string]::IsNullOrWhiteSpace([string]$row.reason)) { [string]$row.reason } else { 'lowering request' }
      $requestId = [string]$row.request_id
      $event = [pscustomobject]@{ kind = 'request'; lane = $Lane.id; targetLane = 'devirt-main'; requestId = $requestId; summary = $summary; evidence = [string]$row.evidence; priority = [string]$row.priority; status = 'open'; source = (Join-RekitRelativePath -Root $CaseRoot -Path $lowering) }
      if (-not [string]::IsNullOrWhiteSpace($requestId)) { $event | Add-Member -NotePropertyName eventId -NotePropertyValue ('evt-' + (Get-RekitTextHash ($Lane.id + '|request|' + $requestId)).Substring(0,16)) -Force }
      $events += (Ensure-RekitEventId -Event $event -LaneId $Lane.id)
    }
  }
  $candidatesDir = Join-Path $workspace 'candidates'
  if (Test-Path -LiteralPath $candidatesDir) {
    foreach ($csv in (Get-ChildItem -LiteralPath $candidatesDir -Filter '*.csv' -File)) {
      foreach ($row in (Import-Csv -LiteralPath $csv.FullName)) {
        $status = ([string]$row.status).Trim().ToLowerInvariant()
        if (@('resolved','done','closed','accepted','rejected') -contains $status) { continue }
        $event = [pscustomobject]@{ kind = 'candidate'; lane = $Lane.id; target = $csv.BaseName; summary = ('candidate from ' + $csv.Name); evidence = [string]$row.evidence; confidence = [string]$row.confidence; status = 'open'; source = (Join-RekitRelativePath -Root $CaseRoot -Path $csv.FullName); row = $row }
        $events += (Ensure-RekitEventId -Event $event -LaneId $Lane.id)
      }
    }
  }
  return $events
}

function Add-RekitTaskIfMissing {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$LaneId,
    [Parameter(Mandatory=$true)]$Event
  )
  $laneRoot = Get-RekitLanePath -CaseRoot $CaseRoot -LaneId $LaneId
  $laneFile = Join-Path $laneRoot 'lane.json'
  if (-not (Test-Path -LiteralPath $laneFile)) { throw "target lane does not exist: $LaneId" }
  $lane = Read-RekitJsonFile -Path $laneFile
  if ($null -eq $lane -or $lane.status -eq 'archived' -or $lane.status -eq 'paused') { throw "target lane is not open: $LaneId" }
  $tasksPath = Join-Path $laneRoot 'tasks.jsonl'
  $tasks = @(Read-RekitJsonLines -Path $tasksPath)
  $sourceLane = [string]$Event.lane
  $requestId = if ($Event.PSObject.Properties['requestId']) { [string]$Event.requestId } else { '' }
  if (-not [string]::IsNullOrWhiteSpace($requestId)) {
    if (@($tasks | Where-Object { $_.requestId -eq $requestId -and $_.sourceLane -eq $sourceLane }).Count -gt 0) { return $false }
  } elseif (@($tasks | Where-Object { $_.eventId -eq $Event.eventId }).Count -gt 0) { return $false }
  $task = [ordered]@{ taskId = 'task-' + ([string]$Event.eventId).Replace('evt-',''); eventId = [string]$Event.eventId; requestId = $requestId; kind = [string]$Event.kind; sourceLane = $sourceLane; summary = [string]$Event.summary; status = 'open'; createdAt = New-RekitIsoTime }
  Add-RekitJsonLine -Path $tasksPath -Object $task
  Add-RekitJsonLine -Path (Join-Path $laneRoot 'inbox.jsonl') -Object ([ordered]@{ eventId = [string]$Event.eventId; requestId = $requestId; kind = 'routed-request'; sourceLane = $sourceLane; summary = [string]$Event.summary; time = New-RekitIsoTime })
  return $true
}

function Invoke-RekitAuto {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string[]]$ActionArgs = @(),
    [switch]$WhatIf
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  if ($WhatIf) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
  } else {
    [void](Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane)
  }
  $policy = Get-RekitPolicy -CaseRoot $caseRoot -NoCreate:$WhatIf
  $paths = Get-RekitBoardPaths -CaseRoot $caseRoot
  $runId = 'run-' + (New-RekitBoardTimestamp)
  $runRoot = Join-Path $paths.Runs $runId
  if (-not $WhatIf) { Ensure-RekitDirectory $runRoot }
  $known = Get-RekitKnownEventIds -CaseRoot $caseRoot
  $summary = [ordered]@{ collected = 0; observations = 0; requests = 0; routed = 0; candidates = 0; acceptedCandidates = 0; publications = 0; authorityApplied = 0; pendingUser = 0; skipped = 0 }
  $digest = New-Object System.Collections.Generic.List[string]
  $digest.Add("# rekit continue digest：$runId")
  $digest.Add('')
  foreach ($dir in (Get-RekitLaneDirectories -CaseRoot $caseRoot)) {
    $lane = Read-RekitJsonFile -Path (Join-Path $dir.FullName 'lane.json')
    if ($null -eq $lane -or $lane.status -eq 'archived' -or $lane.status -eq 'paused') { continue }
    foreach ($event in (Get-RekitLaneOutputEvents -CaseRoot $caseRoot -Lane $lane)) {
      if ($known.ContainsKey([string]$event.eventId)) { continue }
      $summary.collected++
      $event | Add-Member -NotePropertyName lane -NotePropertyValue $lane.id -Force
      if (-not $event.PSObject.Properties['time']) { $event | Add-Member -NotePropertyName time -NotePropertyValue (New-RekitIsoTime) -Force }
      $kind = ([string]$event.kind).ToLowerInvariant()
      if ($WhatIf) {
        $digest.Add("- would collect `$kind` from `$($lane.id)`: $($event.summary)")
        continue
      }
      switch ($kind) {
        'observation' {
          if (Test-RekitPolicyBool -Policy $policy -Name 'autoPublishSharedFacts' -Default $true) {
            Add-RekitJsonLine -Path $paths.Observations -Object $event
            Add-RekitJsonLine -Path $paths.Decisions -Object (New-RekitDecision -Event $event -Action 'auto-publish' -Reason 'shared observation' -RunId $runId)
            $summary.observations++
          }
        }
        'request' {
          Add-RekitJsonLine -Path $paths.Requests -Object $event
          $summary.requests++
          $targetLane = if ($event.PSObject.Properties['targetLane'] -and -not [string]::IsNullOrWhiteSpace([string]$event.targetLane)) { [string]$event.targetLane } else { 'devirt-main' }
          if (Test-RekitPolicyBool -Policy $policy -Name 'autoRouteRequests' -Default $true) {
            try {
              if (Add-RekitTaskIfMissing -CaseRoot $caseRoot -LaneId $targetLane -Event $event) { $summary.routed++ }
              Add-RekitJsonLine -Path $paths.Decisions -Object (New-RekitDecision -Event $event -Action 'auto-route' -Reason "routed to $targetLane" -RunId $runId)
            } catch {
              $event | Add-Member -NotePropertyName decision -NotePropertyValue 'route-failed' -Force
              $event | Add-Member -NotePropertyName decisionReason -NotePropertyValue ([string]$_) -Force
              Add-RekitJsonLine -Path $paths.Decisions -Object (New-RekitDecision -Event $event -Action 'pending-user' -Reason ([string]$_) -RunId $runId)
              $summary.pendingUser++
            }
          }
        }
        'candidate' {
          $summary.candidates++
          if (Test-RekitPolicyBool -Policy $policy -Name 'autoVerify' -Default $true) {
            $verification = Invoke-RekitRuleVerifier -CaseRoot $caseRoot -Manifest $manifest -Policy $policy -Event $event
          } else {
            $verification = [pscustomobject]@{ verifier = 'policy-disabled'; verdict = 'skipped'; confidence = (Get-RekitEventConfidence $event); hasEvidence = (Test-RekitEventEvidence -CaseRoot $caseRoot -Event $event); schemaValid = $true; conflict = $false; reasons = @('autoVerify disabled'); time = New-RekitIsoTime }
          }
          $event = Set-RekitEventVerification -Event $event -Verification $verification
          $hasEvidence = [bool]$verification.hasEvidence
          $verified = Test-RekitEventVerified -Policy $policy -Event $event
          $authority = Get-RekitCandidateAuthorityFile $event
          if (-not [string]::IsNullOrWhiteSpace($authority)) {
            $result = Invoke-RekitAuthorityAppend -CaseRoot $caseRoot -Manifest $manifest -Policy $policy -Event $event -RunRoot $runRoot
            if ($result.Applied) {
              Add-RekitJsonLine -Path $paths.Publications -Object ([ordered]@{ eventId = [string]$event.eventId; kind = 'publication'; sourceLane = [string]$event.lane; summary = "authority append: $($result.AuthorityFile)"; authorityFile = $result.AuthorityFile; rows = $result.Rows; backup = $result.Backup; diff = $result.Diff; time = New-RekitIsoTime; runId = $runId })
              Add-RekitJsonLine -Path $paths.Decisions -Object (New-RekitDecision -Event $event -Action 'auto-apply-authority' -Reason 'passed authority append policy' -RunId $runId -Extra $result)
              $summary.authorityApplied += [int]$result.Rows
            } else {
              $event | Add-Member -NotePropertyName decision -NotePropertyValue 'pending-user' -Force
              $event | Add-Member -NotePropertyName decisionReason -NotePropertyValue $result.Reason -Force
              Add-RekitJsonLine -Path $paths.Candidates -Object $event
              Add-RekitJsonLine -Path $paths.Decisions -Object (New-RekitDecision -Event $event -Action 'pending-user' -Reason $result.Reason -RunId $runId)
              $summary.pendingUser++
            }
          } elseif ((Test-RekitPolicyBool -Policy $policy -Name 'autoAcceptLowRiskCandidates' -Default $true) -and $hasEvidence -and $verified) {
            $event | Add-Member -NotePropertyName decision -NotePropertyValue 'accepted-shared' -Force
            Add-RekitJsonLine -Path $paths.Candidates -Object $event
            Add-RekitJsonLine -Path $paths.Decisions -Object (New-RekitDecision -Event $event -Action 'auto-accept-shared' -Reason 'candidate has evidence, verifier accepted, and does not touch authority' -RunId $runId)
            $summary.acceptedCandidates++
          } else {
            $event | Add-Member -NotePropertyName decision -NotePropertyValue 'needs-evidence' -Force
            Add-RekitJsonLine -Path $paths.Candidates -Object $event
            Add-RekitJsonLine -Path $paths.Decisions -Object (New-RekitDecision -Event $event -Action 'defer' -Reason 'candidate lacks evidence or policy disabled' -RunId $runId)
            $summary.pendingUser++
          }
        }
        'publication' {
          Add-RekitJsonLine -Path $paths.Publications -Object $event
          Add-RekitJsonLine -Path $paths.Decisions -Object (New-RekitDecision -Event $event -Action 'auto-publish' -Reason 'publication event' -RunId $runId)
          $summary.publications++
        }
        default {
          Add-RekitJsonLine -Path $paths.Observations -Object $event
          Add-RekitJsonLine -Path $paths.Decisions -Object (New-RekitDecision -Event $event -Action 'auto-publish' -Reason "unknown kind treated as observation: $kind" -RunId $runId)
          $summary.observations++
        }
      }
      $known[[string]$event.eventId] = $true
    }
    if (-not $WhatIf) { Write-RekitLaneResume -CaseRoot $caseRoot -LaneId $lane.id | Out-Null }
  }
  if (-not $WhatIf) { [void](Save-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest) }
  $digest.Add('## 自动处理')
  $digest.Add('')
  foreach ($key in $summary.Keys) { $digest.Add("- ${key}: $($summary[$key])") }
  $digest.Add('')
  if ($summary.pendingUser -gt 0) {
    $digest.Add('## 需要关注')
    $digest.Add('')
    $digest.Add("- 有 $($summary.pendingUser) 个事件未自动确认，已写入 `.rekit/facts/decisions.jsonl`。")
  } else {
    $digest.Add('## 需要关注')
    $digest.Add('')
    $digest.Add('- 无。')
  }
  if (-not $WhatIf) {
    Write-RekitJsonFile -Path (Join-Path $runRoot 'status.json') -Object ([ordered]@{ schemaVersion = 1; runId = $runId; summary = $summary; time = New-RekitIsoTime })
    [System.IO.File]::WriteAllText((Join-Path $runRoot 'digest.md'), (($digest -join "`r`n") + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  }
  Write-Host "继续推进完成：$runId"
  foreach ($key in $summary.Keys) { Write-Host ("{0}: {1}" -f $key, $summary[$key]) }
  if (-not $WhatIf) { Write-Host "本轮摘要：$((Join-RekitRelativePath -Root $caseRoot -Path (Join-Path $runRoot 'digest.md')))" }
}
