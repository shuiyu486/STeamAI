function Get-RekitLaneTypes {
  param([Parameter(Mandatory=$true)]$Manifest)
  $types = @()
  if ($Manifest.PSObject.Properties['LaneTypes']) {
    foreach ($row in @($Manifest.LaneTypes)) {
      if ([string]::IsNullOrWhiteSpace([string]$row.id)) { continue }
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

function Get-RekitAuthorityLaneType {
  param([Parameter(Mandatory=$true)]$Manifest)
  $configured = if ($Manifest.PSObject.Properties['WorkstreamDefaults']) { [string]$Manifest.WorkstreamDefaults['defaultAuthorityLane'] } else { '' }
  if (-not [string]::IsNullOrWhiteSpace($configured)) { return Get-RekitLaneType -Manifest $Manifest -Type $configured }
  $authority = @(Get-RekitLaneTypes -Manifest $Manifest | Where-Object { $_.Authority } | Select-Object -First 1)
  if ($authority.Count -eq 0) { throw "manifest laneTypes must include an authority lane: $($Manifest.ManifestPath)" }
  return $authority[0]
}

function Get-RekitDefaultStartLaneTypeId {
  param([Parameter(Mandatory=$true)]$Manifest)
  $configured = if ($Manifest.PSObject.Properties['WorkstreamDefaults']) { [string]$Manifest.WorkstreamDefaults['defaultStartLaneType'] } else { '' }
  if (-not [string]::IsNullOrWhiteSpace($configured)) { return $configured }
  $nonAuthority = @(Get-RekitLaneTypes -Manifest $Manifest | Where-Object { -not $_.Authority } | Select-Object -First 1)
  if ($nonAuthority.Count -gt 0) { return [string]$nonAuthority[0].Id }
  return [string](Get-RekitAuthorityLaneType -Manifest $Manifest).Id
}

function Get-RekitHandoffPath {
  param([Parameter(Mandatory=$true)]$Manifest)
  $configured = if ($Manifest.PSObject.Properties['WorkstreamDefaults']) { [string]$Manifest.WorkstreamDefaults['handoffPath'] } else { '' }
  return $configured
}

function Get-RekitBackupRootRelativePath {
  param([Parameter(Mandatory=$true)]$Manifest)
  $configured = if ($Manifest.PSObject.Properties['WorkstreamDefaults']) { [string]$Manifest.WorkstreamDefaults['backupRoot'] } else { '' }
  if (-not [string]::IsNullOrWhiteSpace($configured)) { return $configured }
  return '.rekit/backups/sync'
}

function Get-RekitRequestDefaultTargetLane {
  param([Parameter(Mandatory=$true)]$Manifest)
  $configured = if ($Manifest.PSObject.Properties['WorkstreamDefaults']) { [string]$Manifest.WorkstreamDefaults['requestDefaultTargetLane'] } else { '' }
  if (-not [string]::IsNullOrWhiteSpace($configured)) { return $configured }
  return [string](Get-RekitAuthorityLaneType -Manifest $Manifest).Id
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
  $raw = if ([string]::IsNullOrWhiteSpace($Name)) { $Type } elseif ($Type -match 'feature') { 'feature-' + $Name } else { $Type + '-' + $Name }
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
  if (-not $laneType.Authority) {
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
    '- 只写本工作线 workspace，除非 lane.json 明确列入 canWrite。',
    '- authority 文件只由主线或 policy gate 写入。',
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
