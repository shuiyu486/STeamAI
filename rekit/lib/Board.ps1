function New-RekitBoardTimestamp {
  return (Get-Date -Format 'yyyyMMdd-HHmmssfff')
}

function New-RekitIsoTime {
  return (Get-Date).ToUniversalTime().ToString('o')
}

function Get-RekitTextHash {
  param([AllowEmptyString()][string]$Text)
  $sha = [System.Security.Cryptography.SHA256]::Create()
  try {
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($Text)
    return ([System.BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
  } finally {
    $sha.Dispose()
  }
}

function ConvertTo-RekitJsonLine {
  param([Parameter(Mandatory=$true)]$Object)
  return ($Object | ConvertTo-Json -Depth 16 -Compress)
}

function Write-RekitJsonFile {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)]$Object
  )
  Ensure-RekitDirectory (Split-Path -Parent $Path)
  [System.IO.File]::WriteAllText($Path, ($Object | ConvertTo-Json -Depth 16), [System.Text.UTF8Encoding]::new($false))
}

function Read-RekitJsonFile {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return $null }
  $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
  if ([string]::IsNullOrWhiteSpace($text)) { return $null }
  return ($text | ConvertFrom-Json)
}

function Add-RekitJsonLine {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)]$Object
  )
  Ensure-RekitDirectory (Split-Path -Parent $Path)
  [System.IO.File]::AppendAllText($Path, (ConvertTo-RekitJsonLine $Object) + "`r`n", [System.Text.UTF8Encoding]::new($false))
}

function Read-RekitJsonLines {
  param([Parameter(Mandatory=$true)][string]$Path)
  $items = @()
  if (-not (Test-Path -LiteralPath $Path)) { return $items }
  foreach ($line in [System.IO.File]::ReadLines($Path, [System.Text.Encoding]::UTF8)) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    try {
      $items += ($line | ConvertFrom-Json)
    } catch {
      Write-Warning "skip malformed jsonl line in $Path"
    }
  }
  return $items
}

function Split-RekitScalarList {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) { return @() }
  return @($Value -split '[,;]' | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
}

function Select-RekitFirstText {
  param([object[]]$Values)
  foreach ($value in $Values) {
    if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) { return [string]$value }
  }
  return ''
}

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
  }
}

function Get-RekitDefaultPolicyText {
  return @'
schemaVersion: 1
automationMode: assisted-autopilot
autoCollect: true
autoVerify: true
autoRouteRequests: true
autoSyncLanes: true
autoPublishSharedFacts: true
autoAcceptLowRiskCandidates: true
authorityAutoAppend: conditional
authorityAutoOverwrite: never
authorityAutoDelete: never
requireEvidence: true
requireVerifier: true
minConfidence: 0.90
requireNoConflict: true
requireSchemaValid: true
requireBackup: true
requireDiff: true
maxAuthorityRowsPerRun: 10
askUserWhen: conflict,overwriteAuthority,deleteAuthority,confidenceBelowThreshold,schemaChange,changesProjectBaseline,externalSideEffect,destructiveAction
'@
}

function Ensure-RekitPolicyFile {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  if (-not (Test-Path -LiteralPath $paths.Policy)) {
    Ensure-RekitDirectory (Split-Path -Parent $paths.Policy)
    [System.IO.File]::WriteAllText($paths.Policy, (Get-RekitDefaultPolicyText), [System.Text.UTF8Encoding]::new($false))
  }
  return $paths.Policy
}

function Get-RekitPolicy {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [switch]$NoCreate
  )
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  if ((Test-Path -LiteralPath $paths.Policy) -or -not $NoCreate) {
    $path = Ensure-RekitPolicyFile -CaseRoot $CaseRoot
    $lines = [System.IO.File]::ReadAllLines($path, [System.Text.Encoding]::UTF8)
  } else {
    $lines = (Get-RekitDefaultPolicyText) -split "`r?`n"
  }
  $policy = [ordered]@{}
  foreach ($line in $lines) {
    $trim = $line.Trim()
    if ($trim -eq '' -or $trim.StartsWith('#') -or -not $trim.Contains(':')) { continue }
    $parts = $trim.Split(':', 2)
    $key = $parts[0].Trim()
    $value = Convert-RekitYamlValue $parts[1]
    $policy[$key] = $value
  }
  foreach ($pair in @{
    automationMode = 'assisted-autopilot'; autoCollect = 'true'; autoVerify = 'true'; autoRouteRequests = 'true'; autoSyncLanes = 'true'; autoPublishSharedFacts = 'true'; autoAcceptLowRiskCandidates = 'true'; authorityAutoAppend = 'conditional'; authorityAutoOverwrite = 'never'; authorityAutoDelete = 'never'; requireEvidence = 'true'; requireVerifier = 'true'; minConfidence = '0.90'; requireNoConflict = 'true'; requireSchemaValid = 'true'; requireBackup = 'true'; requireDiff = 'true'; maxAuthorityRowsPerRun = '10'
  }.GetEnumerator()) {
    if (-not $policy.Contains($pair.Key)) { $policy[$pair.Key] = $pair.Value }
  }
  return [pscustomobject]$policy
}

function Test-RekitPolicyBool {
  param(
    [Parameter(Mandatory=$true)]$Policy,
    [Parameter(Mandatory=$true)][string]$Name,
    [bool]$Default = $false
  )
  $prop = $Policy.PSObject.Properties[$Name]
  if ($null -eq $prop) { return $Default }
  return Convert-RekitYamlBool $prop.Value $Default
}

function Get-RekitPolicyNumber {
  param(
    [Parameter(Mandatory=$true)]$Policy,
    [Parameter(Mandatory=$true)][string]$Name,
    [double]$Default = 0
  )
  $prop = $Policy.PSObject.Properties[$Name]
  if ($null -eq $prop) { return $Default }
  $out = 0.0
  if ([double]::TryParse([string]$prop.Value, [ref]$out)) { return $out }
  return $Default
}

function Get-RekitDefaultLaneTypes {
  return @(
    [pscustomobject]@{
      Id = 'devirt-main'
      Title = 'VMProtect 脱壳主线'
      Authority = $true
      WorkspaceRoot = 'captures/devirt_main'
      CanWrite = @('captures/vm_opcode_semantics_confirmed.csv','captures/vm_handler_roles_confirmed.csv','references/vmp-re/task-handoff.md','captures/routine_ir.events.csv','captures/routine_ir.summary.csv','captures/routine_ir.md')
      ReadOnly = @('.rekit/facts/**')
      Outputs = @('publication','authority-update','resolved-request','observation')
    },
    [pscustomobject]@{
      Id = 'feature-analysis'
      Title = '功能分析'
      Authority = $false
      WorkspaceRoot = 'captures/feature_analysis'
      CanWrite = @('own-workspace')
      ReadOnly = @('captures/vm_opcode_semantics_confirmed.csv','captures/vm_handler_roles_confirmed.csv','captures/routine_ir.events.csv','captures/routine_ir.summary.csv','captures/routine_ir.md','references/vmp-re/task-handoff.md','.rekit/facts/**')
      Outputs = @('observation','request','candidate','summary')
    },
    [pscustomobject]@{
      Id = 'tooling'
      Title = '工具链开发'
      Authority = $false
      WorkspaceRoot = 'captures/tooling_lanes'
      CanWrite = @('own-workspace')
      ReadOnly = @('.rekit/facts/**','references/vmp-re/**')
      Outputs = @('observation','request','candidate','tooling-change')
    }
  )
}

function Get-RekitLaneTypes {
  param([Parameter(Mandatory=$true)]$Manifest)
  $types = @(Get-RekitDefaultLaneTypes)
  if ($Manifest.PSObject.Properties['LaneTypes']) {
    foreach ($row in @($Manifest.LaneTypes)) {
      if ([string]::IsNullOrWhiteSpace([string]$row.id)) { continue }
      $types = @($types | Where-Object { $_.Id -ne [string]$row.id })
      $types += [pscustomobject]@{
        Id = [string]$row.id
        Title = if ([string]::IsNullOrWhiteSpace([string]$row.title)) { [string]$row.id } else { [string]$row.title }
        Authority = Convert-RekitYamlBool $row.authority
        WorkspaceRoot = if ([string]::IsNullOrWhiteSpace([string]$row.workspaceRoot)) { 'captures/lanes' } else { [string]$row.workspaceRoot }
        CanWrite = @(Split-RekitScalarList ([string]$row.canWrite))
        ReadOnly = @(Split-RekitScalarList ([string]$row.readOnly))
        Outputs = @(Split-RekitScalarList ([string]$row.outputs))
      }
    }
  }
  return $types
}

function Get-RekitLaneType {
  param(
    [Parameter(Mandatory=$true)]$Manifest,
    [Parameter(Mandatory=$true)][string]$Type
  )
  foreach ($laneType in (Get-RekitLaneTypes -Manifest $Manifest)) {
    if ([string]::Equals($laneType.Id, $Type, [System.StringComparison]::OrdinalIgnoreCase)) { return $laneType }
  }
  throw "unknown lane type: $Type"
}

function ConvertTo-RekitLaneId {
  param(
    [Parameter(Mandatory=$true)][string]$Type,
    [string]$Name = ''
  )
  $raw = if ([string]::IsNullOrWhiteSpace($Name)) { $Type } elseif ($Type -eq 'feature-analysis') { 'feature-' + $Name } else { $Type + '-' + $Name }
  $safe = ($raw.Trim().ToLowerInvariant() -replace '[^a-z0-9._-]+', '-')
  return $safe.Trim('-_.')
}

function Get-RekitLanePath {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$LaneId
  )
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  return Join-RekitPath -Root $paths.Lanes -RelativePath $LaneId
}

function Read-RekitLane {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$LaneId
  )
  $lanePath = Join-Path (Get-RekitLanePath -CaseRoot $CaseRoot -LaneId $LaneId) 'lane.json'
  return Read-RekitJsonFile -Path $lanePath
}

function Get-RekitLaneDirectories {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  if (-not (Test-Path -LiteralPath $paths.Lanes)) { return @() }
  return @(Get-ChildItem -LiteralPath $paths.Lanes -Directory | Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'lane.json') })
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
  $board = [ordered]@{
    schemaVersion = 1
    caseRoot = [System.IO.Path]::GetFullPath($CaseRoot)
    repoRoot = [System.IO.Path]::GetFullPath($RepoRoot)
    pack = $Manifest.Pack
    automationMode = (Get-RekitPolicy -CaseRoot $CaseRoot).automationMode
    defaultAuthorityLane = 'devirt-main'
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
  foreach ($file in @($paths.Observations,$paths.Candidates,$paths.Requests,$paths.Publications,$paths.Decisions)) {
    if (-not (Test-Path -LiteralPath $file)) { [System.IO.File]::WriteAllText($file, '', [System.Text.UTF8Encoding]::new($false)) }
  }
  [void](Ensure-RekitPolicyFile -CaseRoot $case)
  if ($CreateDefaultLane -and -not (Test-Path -LiteralPath (Join-Path (Get-RekitLanePath -CaseRoot $case -LaneId 'devirt-main') 'lane.json'))) {
    New-RekitLane -CaseRoot $case -RepoRoot $RepoRoot -Manifest $manifest -Type 'devirt-main' -Name '' | Out-Null
  }
  return Save-RekitBoard -CaseRoot $case -RepoRoot $RepoRoot -Manifest $manifest
}

function New-RekitLane {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [Parameter(Mandatory=$true)]$Manifest,
    [Parameter(Mandatory=$true)][string]$Type,
    [string]$Name = '',
    [switch]$Force
  )
  $laneType = Get-RekitLaneType -Manifest $Manifest -Type $Type
  $laneId = ConvertTo-RekitLaneId -Type $laneType.Id -Name $Name
  $laneRoot = Get-RekitLanePath -CaseRoot $CaseRoot -LaneId $laneId
  $laneFile = Join-Path $laneRoot 'lane.json'
  if ((Test-Path -LiteralPath $laneFile) -and -not $Force) { return Read-RekitJsonFile -Path $laneFile }
  $workspaceName = if ([string]::IsNullOrWhiteSpace($Name)) { $laneId } else { $laneId }
  $workspace = Join-RekitPath -Root $CaseRoot -RelativePath (Join-Path $laneType.WorkspaceRoot $workspaceName)
  foreach ($dir in @($laneRoot, (Join-Path $laneRoot 'checkpoints'), (Join-Path $laneRoot 'prompts'), (Join-Path $laneRoot 'reviews'), $workspace)) { Ensure-RekitDirectory $dir }
  foreach ($file in @('events.jsonl','tasks.jsonl','inbox.jsonl','outbox.jsonl')) {
    $path = Join-Path $laneRoot $file
    if (-not (Test-Path -LiteralPath $path)) { [System.IO.File]::WriteAllText($path, '', [System.Text.UTF8Encoding]::new($false)) }
  }
  foreach ($file in @('observations.jsonl','requests.jsonl','candidates.jsonl','publications.jsonl')) {
    $path = Join-Path $workspace $file
    if (-not (Test-Path -LiteralPath $path)) { [System.IO.File]::WriteAllText($path, '', [System.Text.UTF8Encoding]::new($false)) }
  }
  if ($laneType.Id -eq 'feature-analysis') {
    foreach ($nameFile in @('summary.md','evidence.md','notes.md')) {
      $path = Join-Path $workspace $nameFile
      if (-not (Test-Path -LiteralPath $path)) {
        [System.IO.File]::WriteAllText($path, "# $laneId`r`n`r`n待填写。`r`n", [System.Text.UTF8Encoding]::new($false))
      }
    }
  }
  $now = New-RekitIsoTime
  $lane = [ordered]@{
    schemaVersion = 1
    id = $laneId
    type = $laneType.Id
    name = $Name
    title = if ([string]::IsNullOrWhiteSpace($Name)) { $laneType.Title } else { $laneType.Title + ': ' + $Name }
    status = 'open'
    authority = [bool]$laneType.Authority
    workspace = (Join-RekitRelativePath -Root $CaseRoot -Path $workspace)
    laneRoot = (Join-RekitRelativePath -Root $CaseRoot -Path $laneRoot)
    canWrite = @($laneType.CanWrite)
    readOnly = @($laneType.ReadOnly)
    outputs = @($laneType.Outputs)
    counters = [ordered]@{ observations = 0; requests = 0; candidates = 0; publications = 0; pendingUser = 0 }
    createdAt = $now
    updatedAt = $now
  }
  Write-RekitJsonFile -Path $laneFile -Object $lane
  Add-RekitJsonLine -Path (Join-Path $laneRoot 'events.jsonl') -Object ([ordered]@{ eventId = 'evt-' + (Get-RekitTextHash "$laneId|created|$now").Substring(0,16); kind = 'lane-created'; lane = $laneId; time = $now; summary = "lane created: $laneId" })
  Write-RekitLaneResume -CaseRoot $CaseRoot -LaneId $laneId | Out-Null
  return Read-RekitJsonFile -Path $laneFile
}

function Join-RekitRelativePath {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][string]$Path
  )
  $rel = [System.IO.Path]::GetRelativePath([System.IO.Path]::GetFullPath($Root), [System.IO.Path]::GetFullPath($Path))
  return ($rel -replace '\\','/')
}

function Get-RekitLaneWorkspacePath {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Lane
  )
  return Join-RekitPath -Root $CaseRoot -RelativePath ([string]$Lane.workspace)
}

function Write-RekitLaneResume {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$LaneId
  )
  $laneRoot = Get-RekitLanePath -CaseRoot $CaseRoot -LaneId $LaneId
  $lane = Read-RekitJsonFile -Path (Join-Path $laneRoot 'lane.json')
  if ($null -eq $lane) { throw "missing lane: $LaneId" }
  $inbox = @(Read-RekitJsonLines -Path (Join-Path $laneRoot 'inbox.jsonl'))
  $tasks = @(Read-RekitJsonLines -Path (Join-Path $laneRoot 'tasks.jsonl'))
  $resume = @(
    ('# RESUME：' + $lane.id),
    '',
    ('lane type: `' + $lane.type + '`'),
    ('status: `' + $lane.status + '`'),
    ('workspace: `' + $lane.workspace + '`'),
    '',
    '## 边界',
    '',
    '- 只写本 lane workspace，除非 lane.json 明确列入 canWrite。',
    '- authority 文件只由 authority lane 或 policy gate 写入。',
    '- 新发现写入 outbox.jsonl / workspace observations.jsonl / requests.jsonl / candidates.jsonl。',
    '',
    '## 最近 inbox',
    ''
  )
  foreach ($msg in ($inbox | Select-Object -Last 8)) { $resume += ('- ' + (Select-RekitFirstText @($msg.summary, $msg.kind, $msg.eventId))) }
  if ($inbox.Count -eq 0) { $resume += '- 无。' }
  $resume += @('', '## 未关闭任务', '')
  foreach ($task in ($tasks | Where-Object { $_.status -ne 'closed' -and $_.status -ne 'resolved' } | Select-Object -Last 12)) { $resume += ('- ' + (Select-RekitFirstText @($task.summary, $task.taskId, $task.eventId))) }
  if (($tasks | Where-Object { $_.status -ne 'closed' -and $_.status -ne 'resolved' }).Count -eq 0) { $resume += '- 无。' }
  $path = Join-Path $laneRoot 'prompts\RESUME.md'
  Ensure-RekitDirectory (Split-Path -Parent $path)
  [System.IO.File]::WriteAllText($path, (($resume -join "`r`n") + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $checkpoint = [ordered]@{ schemaVersion = 1; lane = $lane.id; status = $lane.status; workspace = $lane.workspace; inbox = $inbox.Count; tasks = $tasks.Count; updatedAt = New-RekitIsoTime; resume = (Join-RekitRelativePath -Root $CaseRoot -Path $path) }
  Write-RekitJsonFile -Path (Join-Path $laneRoot 'checkpoints\latest.json') -Object $checkpoint
  return $path
}

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
  $digest.Add("# rekit auto digest：$runId")
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
  Write-Host "rekit auto: $runId"
  foreach ($key in $summary.Keys) { Write-Host ("{0}: {1}" -f $key, $summary[$key]) }
  if (-not $WhatIf) { Write-Host "digest: $((Join-RekitRelativePath -Root $caseRoot -Path (Join-Path $runRoot 'digest.md')))" }
}

function Show-RekitBoard {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re'
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $board = Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane
  $paths = Get-RekitBoardPaths -CaseRoot $caseRoot
  Write-Host "rekit board: $caseRoot"
  Write-Host "automation: $($board.automationMode)"
  Write-Host "lanes:"
  foreach ($lane in @($board.lanes)) {
    Write-Host ("- {0} [{1}] status={2} authority={3} workspace={4}" -f $lane.id, $lane.type, $lane.status, $lane.authority, $lane.workspace)
  }
  $obs = @(Read-RekitJsonLines -Path $paths.Observations).Count
  $req = @(Read-RekitJsonLines -Path $paths.Requests).Count
  $cand = @(Read-RekitJsonLines -Path $paths.Candidates).Count
  $pub = @(Read-RekitJsonLines -Path $paths.Publications).Count
  $pending = @((Read-RekitJsonLines -Path $paths.Decisions) | Where-Object { $_.action -eq 'pending-user' }).Count
  Write-Host "facts: observations=$obs requests=$req candidates=$cand publications=$pub pendingUser=$pending"
  Write-Host "next: /rekit auto"
}

function Invoke-RekitLaneCommand {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string[]]$ActionArgs = @(),
    [switch]$WhatIf,
    [switch]$Force
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $action = if ($ActionArgs.Count -gt 0) { ([string]$ActionArgs[0]).ToLowerInvariant() } else { 'list' }
  if ($WhatIf) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
  } else {
    [void](Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane)
  }
  switch ($action) {
    'list' {
      if ($WhatIf) { Write-Host "would show board: $caseRoot"; return }
      Show-RekitBoard -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack
    }
    'start' {
      if ($ActionArgs.Count -lt 2) { throw 'lane start requires lane type, e.g. /rekit lane start feature-analysis login' }
      $type = [string]$ActionArgs[1]
      $name = if ($ActionArgs.Count -gt 2) { [string]$ActionArgs[2] } else { '' }
      if ($WhatIf) {
        [void](Get-RekitLaneType -Manifest $manifest -Type $type)
        $id = ConvertTo-RekitLaneId -Type $type -Name $name
        Write-Host "would create lane: $id"
        return
      }
      $lane = New-RekitLane -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest -Type $type -Name $name -Force:$Force
      [void](Save-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest)
      Write-Host "lane ready: $($lane.id)"
      Write-Host "workspace: $($lane.workspace)"
    }
    'resume' {
      if ($ActionArgs.Count -lt 2) { throw 'lane resume requires lane id' }
      $laneId = [string]$ActionArgs[1]
      if ($WhatIf) {
        if ($null -eq (Read-RekitLane -CaseRoot $caseRoot -LaneId $laneId)) { throw "missing lane: $laneId" }
        Write-Host "would refresh resume prompt: $laneId"
        return
      }
      $resume = Write-RekitLaneResume -CaseRoot $caseRoot -LaneId $laneId
      Write-Host "resume prompt: $(Join-RekitRelativePath -Root $caseRoot -Path $resume)"
    }
    'pause' {
      if ($ActionArgs.Count -lt 2) { throw 'lane pause requires lane id' }
      if ($WhatIf) {
        if ($null -eq (Read-RekitLane -CaseRoot $caseRoot -LaneId ([string]$ActionArgs[1]))) { throw "missing lane: $($ActionArgs[1])" }
        Write-Host "would set lane $($ActionArgs[1]) status: paused"
        return
      }
      Set-RekitLaneStatus -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest -LaneId ([string]$ActionArgs[1]) -Status 'paused'
    }
    'archive' {
      if ($ActionArgs.Count -lt 2) { throw 'lane archive requires lane id' }
      if ($WhatIf) {
        if ($null -eq (Read-RekitLane -CaseRoot $caseRoot -LaneId ([string]$ActionArgs[1]))) { throw "missing lane: $($ActionArgs[1])" }
        Write-Host "would set lane $($ActionArgs[1]) status: archived"
        return
      }
      Set-RekitLaneStatus -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest -LaneId ([string]$ActionArgs[1]) -Status 'archived'
    }
    'open' {
      if ($ActionArgs.Count -lt 2) { throw 'lane open requires lane id' }
      if ($WhatIf) {
        if ($null -eq (Read-RekitLane -CaseRoot $caseRoot -LaneId ([string]$ActionArgs[1]))) { throw "missing lane: $($ActionArgs[1])" }
        Write-Host "would set lane $($ActionArgs[1]) status: open"
        return
      }
      Set-RekitLaneStatus -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest -LaneId ([string]$ActionArgs[1]) -Status 'open'
    }
    default { throw "unknown lane action: $action" }
  }
}

function Set-RekitLaneStatus {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [Parameter(Mandatory=$true)]$Manifest,
    [Parameter(Mandatory=$true)][string]$LaneId,
    [Parameter(Mandatory=$true)][string]$Status
  )
  $laneRoot = Get-RekitLanePath -CaseRoot $CaseRoot -LaneId $LaneId
  $laneFile = Join-Path $laneRoot 'lane.json'
  $lane = Read-RekitJsonFile -Path $laneFile
  if ($null -eq $lane) { throw "missing lane: $LaneId" }
  $lane.status = $Status
  $lane.updatedAt = New-RekitIsoTime
  Write-RekitJsonFile -Path $laneFile -Object $lane
  $now = New-RekitIsoTime
  $eventId = 'evt-' + (Get-RekitTextHash ($LaneId + '|status|' + $Status + '|' + $now)).Substring(0,16)
  Add-RekitJsonLine -Path (Join-Path $laneRoot 'events.jsonl') -Object ([ordered]@{ eventId = $eventId; kind = 'lane-status'; lane = $LaneId; status = $Status; time = $now; summary = ('lane status: ' + $Status) })
  [void](Save-RekitBoard -CaseRoot $CaseRoot -RepoRoot $RepoRoot -Manifest $Manifest)
  Write-Host "lane $LaneId status: $Status"
}

function Invoke-RekitPolicyCommand {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [string[]]$ActionArgs = @()
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $path = Ensure-RekitPolicyFile -CaseRoot $caseRoot
  if ($ActionArgs.Count -ge 3 -and ([string]$ActionArgs[0]).ToLowerInvariant() -eq 'set') {
    $key = [string]$ActionArgs[1]
    $value = [string]$ActionArgs[2]
    $text = [System.IO.File]::ReadAllText($path, [System.Text.Encoding]::UTF8)
    $pattern = '(?m)^' + [regex]::Escape($key) + '\s*:.*$'
    $line = $key + ': ' + $value
    if ([regex]::IsMatch($text, $pattern)) { $text = [regex]::Replace($text, $pattern, [System.Text.RegularExpressions.MatchEvaluator]{ param($m) $line }) } else { $text = $text.TrimEnd() + "`r`n" + $line + "`r`n" }
    [System.IO.File]::WriteAllText($path, $text, [System.Text.UTF8Encoding]::new($false))
    Write-Host "policy updated: $key=$value"
    return
  }
  Write-Host "policy: $path"
  Get-Content -LiteralPath $path
}
