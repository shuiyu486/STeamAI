function Get-RekitActionSelector {
  param([string[]]$ActionArgs = @())
  $selector = (($ActionArgs | ForEach-Object { [string]$_ }) -join '-').Trim('-')
  return $selector
}

function Get-RekitWorkstreamLabel {
  param([Parameter(Mandatory=$true)]$Lane)
  if ([bool]$Lane.authority) { return 'main' }
  $id = [string]$Lane.id
  if ($id.StartsWith('feature-')) { return $id.Substring(8) }
  return $id
}

function Get-RekitOpenBoardLanes {
  param([Parameter(Mandatory=$true)]$Board)
  return @($Board.lanes | Where-Object { @('archived','paused','closed') -notcontains ([string]$_.status).ToLowerInvariant() })
}

function Resolve-RekitWorkstreamSelector {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Board,
    [Parameter(Mandatory=$true)][string]$Selector
  )
  $raw = $Selector.Trim()
  if ([string]::IsNullOrWhiteSpace($raw)) { return $null }
  $normalized = $raw.ToLowerInvariant()
  $candidates = New-Object System.Collections.Generic.List[string]
  if ($normalized -eq 'main') {
    $candidates.Add([string]$Board.defaultAuthorityLane)
  } else {
    $candidates.Add($raw)
    if (-not $normalized.StartsWith('feature-')) { $candidates.Add('feature-' + $raw) }
  }
  foreach ($candidate in $candidates) {
    foreach ($lane in @($Board.lanes)) {
      if ([string]::Equals([string]$lane.id, [string]$candidate, [System.StringComparison]::OrdinalIgnoreCase)) {
        return Read-RekitLane -CaseRoot $CaseRoot -LaneId ([string]$lane.id)
      }
    }
  }
  foreach ($lane in @($Board.lanes)) {
    $laneFull = Read-RekitLane -CaseRoot $CaseRoot -LaneId ([string]$lane.id)
    if ($null -eq $laneFull) { continue }
    if ($laneFull.PSObject.Properties['name'] -and [string]::Equals([string]$laneFull.name, $raw, [System.StringComparison]::OrdinalIgnoreCase)) { return $laneFull }
  }
  return $null
}

function Write-RekitWorkstreamChoices {
  param(
    [Parameter(Mandatory=$true)]$Board,
    [string]$Prefix = '请选择要接手的工作线：'
  )
  Write-Host $Prefix
  foreach ($lane in (Get-RekitOpenBoardLanes -Board $Board)) {
    $kind = if ($lane.authority) { '主线' } else { '功能支线' }
    $label = Get-RekitWorkstreamLabel -Lane $lane
    Write-Host ("- {0} {1}：/rekit continue {2}" -f $kind, $lane.id, $label)
  }
}

function Show-RekitOverview {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string]$Format = ''
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $formatValue = ([string]$Format).Trim().ToLowerInvariant()
  if ([string]::IsNullOrWhiteSpace($formatValue)) { $formatValue = 'table' }
  if (@('table','text','tsv','json') -notcontains $formatValue) { throw "unsupported overview format: $Format" }
  $paths = Get-RekitBoardPaths -CaseRoot $caseRoot
  $wasMissingBoard = -not (Test-Path -LiteralPath $paths.Board)
  if ($formatValue -eq 'json' -and -not $wasMissingBoard) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    $board = Read-RekitJsonFile -Path $paths.Board
  } else {
    $board = Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane
  }
  if ($formatValue -eq 'json') {
    $maxRows = 10
    function New-RekitOverviewEventSection {
      param([object[]]$Items, [int]$MaxRows)
      $itemsArray = @($Items)
      $shown = @($itemsArray | Select-Object -Last $MaxRows)
      return [ordered]@{ total = [int]$itemsArray.Count; shown = [int]$shown.Count; events = @($shown) }
    }
    $terminalStatus = @('confirmed','accepted','rejected','superseded','resolved')
    $observations = @(Read-RekitJsonLines -Path $paths.Observations)
    $requests = @(Read-RekitJsonLines -Path $paths.Requests)
    $candidates = @(Read-RekitJsonLines -Path $paths.Candidates)
    $publications = @(Read-RekitJsonLines -Path $paths.Publications)
    $decisions = @(Read-RekitJsonLines -Path $paths.Decisions)
    $hypotheses = @(Read-RekitJsonLines -Path $paths.Hypotheses)
    $verifications = @(Read-RekitJsonLines -Path $paths.Verifications)
    $interventions = @(Read-RekitJsonLines -Path $paths.Interventions)
    $rollbacks = @(Read-RekitJsonLines -Path $paths.Rollbacks)
    $pendingDecisionTerminalStatus = @('confirmed','accepted','rejected','resolved','deferred','superseded')
    $pending = @($decisions | Where-Object {
      $d = [string]$_.decision
      $s = [string]$_.status
      $a = [string]$_.action
      $a -eq 'pending-user' -or ([string]::IsNullOrWhiteSpace($s) -and $d -eq 'defer') -or ($s -ne '' -and $pendingDecisionTerminalStatus -notcontains $s)
    }).Count
    $openCands = @($candidates | Where-Object { $s = [string]$_.status; $s -eq '' -or $terminalStatus -notcontains $s })
    $pendingGates = @($requests | Where-Object { [string]$_.status -eq 'pending-gate' })
    $openInterventions = @($interventions | Where-Object { $s = [string]$_.status; $s -eq '' -or $terminalStatus -notcontains $s })
    $allBatchEvents = @($observations + $hypotheses + $candidates + $verifications + $decisions + $interventions + $rollbacks + $publications + $requests | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_.batchId) })
    $batchGroups = @($allBatchEvents | Group-Object -Property batchId | Sort-Object {
      $last = @($_.Group | Select-Object -Last 1)[0]
      $lastTime = [string]$last.time
      if ([string]::IsNullOrWhiteSpace($lastTime)) { $lastTime = [string]$last.createdAt }
      $lastTime
    })
    $shownBatchGroups = @($batchGroups | Select-Object -Last $maxRows)
    $batchRows = @($shownBatchGroups | ForEach-Object {
      $kindCounts = [ordered]@{}
      foreach ($kg in @($_.Group | Group-Object -Property kind | Sort-Object Name)) {
        $name = [string]$kg.Name
        if ([string]::IsNullOrWhiteSpace($name)) { $name = 'unknown' }
        $kindCounts[$name] = [int]$kg.Count
      }
      $last = @($_.Group | Select-Object -Last 1)[0]
      $lastTime = [string]$last.time
      if ([string]::IsNullOrWhiteSpace($lastTime)) { $lastTime = [string]$last.createdAt }
      [ordered]@{ id = [string]$_.Name; events = [int]$_.Count; kinds = $kindCounts; last = $lastTime }
    })
    $openLanes = @(Get-RekitOpenBoardLanes -Board $board)
    $nextSteps = @()
    if ($openLanes.Count -eq 1) {
      $nextSteps += ('/rekit continue {0}' -f (Get-RekitWorkstreamLabel -Lane $openLanes[0]))
    } else {
      foreach ($lane in $openLanes) { $nextSteps += ('/rekit continue {0}' -f (Get-RekitWorkstreamLabel -Lane $lane)) }
    }
    $nextSteps += @('/rekit start <name>','/rekit handoff','/rekit handoff main 或 /rekit handoff <name>')
    [ordered]@{
      schemaVersion = 1
      command = 'overview'
      caseRoot = $caseRoot
      repoRoot = $RepoRoot
      pack = $Pack
      isMutation = [bool]$wasMissingBoard
      automationMode = [string]$board.automationMode
      lanes = @($board.lanes | ForEach-Object { [ordered]@{ id = [string]$_.id; label = (Get-RekitWorkstreamLabel -Lane $_); kind = $(if ([bool]$_.authority) { 'main' } else { 'feature' }); status = [string]$_.status; workspace = [string]$_.workspace; authority = [bool]$_.authority } })
      counts = [ordered]@{ observations = [int]$observations.Count; requests = [int]$requests.Count; candidates = [int]$candidates.Count; publications = [int]$publications.Count; pendingDecisions = [int]$pending }
      sections = [ordered]@{
        openCandidates = (New-RekitOverviewEventSection -Items $openCands -MaxRows $maxRows)
        pendingGates = (New-RekitOverviewEventSection -Items $pendingGates -MaxRows $maxRows)
        verifications = (New-RekitOverviewEventSection -Items $verifications -MaxRows $maxRows)
        decisions = (New-RekitOverviewEventSection -Items $decisions -MaxRows $maxRows)
        batches = [ordered]@{ total = [int]$batchGroups.Count; shown = [int]$batchRows.Count; batches = @($batchRows) }
        openInterventions = (New-RekitOverviewEventSection -Items $openInterventions -MaxRows $maxRows)
        interventions = (New-RekitOverviewEventSection -Items $interventions -MaxRows $maxRows)
        rollbacks = (New-RekitOverviewEventSection -Items $rollbacks -MaxRows $maxRows)
      }
      nextSteps = @($nextSteps)
    } | ConvertTo-Json -Depth 20
    return
  }
  Write-Host "项目概览：$caseRoot"
  Write-Host "自动化模式：$($board.automationMode)"
  Write-Host '当前是项目总览，还没有为本会话选择具体工作线。'
  Write-Host ''
  Write-Host '工作线：'
  foreach ($lane in @($board.lanes)) {
    $kind = if ($lane.authority) { '主线' } else { '功能/工具支线' }
    $label = Get-RekitWorkstreamLabel -Lane $lane
    Write-Host ("- {0}：{1}，选择名={2}，状态={3}，工作区={4}" -f $kind, $lane.id, $label, $lane.status, $lane.workspace)
  }
  $obs = @(Read-RekitJsonLines -Path $paths.Observations).Count
  $req = @(Read-RekitJsonLines -Path $paths.Requests).Count
  $cand = @(Read-RekitJsonLines -Path $paths.Candidates).Count
  $pub = @(Read-RekitJsonLines -Path $paths.Publications).Count
  $pendingDecisionTerminalStatus = @('confirmed','accepted','rejected','resolved','deferred','superseded')
  $pending = @((Read-RekitJsonLines -Path $paths.Decisions) | Where-Object {
    $d = [string]$_.decision
    $s = [string]$_.status
    $a = [string]$_.action
    $a -eq 'pending-user' -or ([string]::IsNullOrWhiteSpace($s) -and $d -eq 'defer') -or ($s -ne '' -and $pendingDecisionTerminalStatus -notcontains $s)
  }).Count
  Write-Host ''
  Write-Host '共享事实：'
  Write-Host "- observation: $obs"
  Write-Host "- request: $req"
  Write-Host "- candidate: $cand"
  Write-Host "- publication: $pub"
  Write-Host "- 需要确认: $pending"
  Write-Host ''

  $terminalStatus = @('confirmed','accepted','rejected','superseded','resolved')
  $candidates = @(Read-RekitJsonLines -Path $paths.Candidates)
  $requests = @(Read-RekitJsonLines -Path $paths.Requests)
  $openCands = @($candidates | Where-Object {
    $s = [string]$_.status
    $s -eq '' -or $terminalStatus -notcontains $s
  })
  $pendingGates = @($requests | Where-Object { [string]$_.status -eq 'pending-gate' })
  $conflicts = @()
  if ($candidates.Count -gt 0) {
    $groups = $candidates | Where-Object {
      $s = [string]$_.status
      $s -eq '' -or $terminalStatus -notcontains $s
    } | Group-Object -Property subject | Where-Object { $_.Count -gt 1 }
    $conflicts = @($groups | ForEach-Object { $_.Name })
  }
  $maxRows = 10
  if ($openCands.Count -gt 0) {
    Write-Host '未决 candidate：'
    $shown = @($openCands | Select-Object -Last $maxRows)
    foreach ($c in $shown) {
      $subj = [string]$c.subject; $summ = [string]$c.summary; $conf = [string]$c.confidence
      $mark = if ($conflicts -contains $subj) { ' [冲突]' } else { '' }
      Write-Host ("- {0} | {1} | confidence={2}{3}" -f $subj, $summ, $conf, $mark)
    }
    $rest = $openCands.Count - $shown.Count
    if ($rest -gt 0) { Write-Host "- 另有 $rest 条未决 candidate" }
    Write-Host ''
  }
  if ($pendingGates.Count -gt 0) {
    Write-Host 'pending-gate（heavy-tool 待确认）：'
    $shown = @($pendingGates | Select-Object -Last $maxRows)
    foreach ($g in $shown) {
      $detail = Format-RekitGateRequestDetail -Event $g -OmitStatus
      Write-Host ("- {0} | {1}{2}" -f [string]$g.subject, [string]$g.summary, $detail)
    }
    $rest = $pendingGates.Count - $shown.Count
    if ($rest -gt 0) { Write-Host "- 另有 $rest 条 pending-gate" }
    Write-Host ''
  }
  $allVerifications = @(Read-RekitJsonLines -Path $paths.Verifications)
  if ($allVerifications.Count -gt 0) {
    Write-Host '最近 verification：'
    $shownV = @($allVerifications | Select-Object -Last $maxRows)
    foreach ($v in $shownV) {
      $subj = [string]$v.subject; if ([string]::IsNullOrWhiteSpace($subj)) { $subj = [string]$v.kind }
      $actor = [string]$v.actor
      $byTag = if (-not [string]::IsNullOrWhiteSpace($actor)) { " | by=$actor" } else { '' }
      $batch = [string]$v.batchId
      $batchTag = if (-not [string]::IsNullOrWhiteSpace($batch)) { " | batch=$batch" } else { '' }
      Write-Host ("- {0} | lane={1} | verifier={2} | verdict={3} | target={4}{5}{6}" -f $subj, [string]$v.lane, [string]$v.verifier, [string]$v.verdict, [string]$v.target, $byTag, $batchTag)
    }
    $rest = $allVerifications.Count - $shownV.Count
    if ($rest -gt 0) { Write-Host "- 另有 $rest 条 verification" }
    Write-Host ''
  }

  $allDecisions = @(Read-RekitJsonLines -Path $paths.Decisions)
  if ($allDecisions.Count -gt 0) {
    Write-Host '最近 decision：'
    $shownD = @($allDecisions | Select-Object -Last $maxRows)
    foreach ($d in $shownD) {
      $subj = [string]$d.subject; if ([string]::IsNullOrWhiteSpace($subj)) { $subj = [string]$d.kind }
      $dec = [string]$d.decision; if ([string]::IsNullOrWhiteSpace($dec)) { $dec = [string]$d.action }
      $ln = [string]$d.lane
      $actor = [string]$d.actor; if ([string]::IsNullOrWhiteSpace($actor)) { $actor = [string]$d.confirmedBy }
      $extra = ''
      if (-not [string]::IsNullOrWhiteSpace($actor)) { $extra = " | by=$actor" }
      Write-Host ("- {0} | lane={1} | decision={2}{3} | reason={4}" -f $subj, $ln, $dec, $extra, [string]$d.reason)
    }
    $rest = $allDecisions.Count - $shownD.Count
    if ($rest -gt 0) { Write-Host "- 另有 $rest 条 decision" }
    Write-Host ''
  }

  $allBatchEvents = @()
  foreach ($batchFile in @($paths.Observations,$paths.Hypotheses,$paths.Candidates,$paths.Verifications,$paths.Decisions,$paths.Interventions,$paths.Rollbacks,$paths.Publications,$paths.Requests)) {
    $allBatchEvents += @(Read-RekitJsonLines -Path $batchFile | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_.batchId) })
  }
  if ($allBatchEvents.Count -gt 0) {
    Write-Host '最近 batch：'
    $batchGroups = @($allBatchEvents | Group-Object -Property batchId | Sort-Object { @($_.Group | Select-Object -Last 1)[0].time } | Select-Object -Last $maxRows)
    foreach ($g in $batchGroups) {
      $kindSummary = @($g.Group | Group-Object -Property kind | ForEach-Object { "$($_.Name)=$($_.Count)" }) -join ','
      $last = @($g.Group | Select-Object -Last 1)[0]
      $lastTime = [string]$last.time; if ([string]::IsNullOrWhiteSpace($lastTime)) { $lastTime = [string]$last.createdAt }
      Write-Host ("- {0} | events={1} | kinds={2} | last={3}" -f $g.Name, $g.Count, $kindSummary, $lastTime)
    }
    $restB = (@($allBatchEvents | Group-Object -Property batchId).Count) - $batchGroups.Count
    if ($restB -gt 0) { Write-Host "- 另有 $restB 个 batch" }
    Write-Host ''
  }

  $interventions = @(Read-RekitJsonLines -Path $paths.Interventions)
  $rollbacks = @(Read-RekitJsonLines -Path $paths.Rollbacks)
  $openInterventions = @($interventions | Where-Object {
    $s = [string]$_.status
    $s -eq '' -or $terminalStatus -notcontains $s
  })
  if ($openInterventions.Count -gt 0) {
    Write-Host '未解决 intervention：'
    $shownI = @($openInterventions | Select-Object -Last $maxRows)
    foreach ($i in $shownI) {
      $subj = [string]$i.subject; if ([string]::IsNullOrWhiteSpace($subj)) { $subj = [string]$i.action }
      $status = [string]$i.status; if ([string]::IsNullOrWhiteSpace($status)) { $status = 'open' }
      $batch = [string]$i.batchId
      $batchTag = if (-not [string]::IsNullOrWhiteSpace($batch)) { " | batch=$batch" } else { '' }
      Write-Host ("- {0} | action={1} | target={2} | status={3}{4} | summary={5}" -f $subj, [string]$i.action, [string]$i.target, $status, $batchTag, [string]$i.summary)
    }
    $restI = $openInterventions.Count - $shownI.Count
    if ($restI -gt 0) { Write-Host "- 另有 $restI 条未解决 intervention" }
    Write-Host ''
  }
  if ($interventions.Count -gt 0) {
    Write-Host '最近 intervention：'
    $shownI2 = @($interventions | Select-Object -Last $maxRows)
    foreach ($i in $shownI2) {
      $subj = [string]$i.subject; if ([string]::IsNullOrWhiteSpace($subj)) { $subj = [string]$i.action }
      $batch = [string]$i.batchId
      $batchTag = if (-not [string]::IsNullOrWhiteSpace($batch)) { " | batch=$batch" } else { '' }
      Write-Host ("- {0} | action={1} | target={2} | approvedBy={3} | scope={4}{5}" -f $subj, [string]$i.action, [string]$i.target, [string]$i.approvedBy, [string]$i.scope, $batchTag)
    }
    $restI2 = $interventions.Count - $shownI2.Count
    if ($restI2 -gt 0) { Write-Host "- 另有 $restI2 条 intervention" }
    Write-Host ''
  }
  if ($rollbacks.Count -gt 0) {
    Write-Host '最近 rollback：'
    $shownR = @($rollbacks | Select-Object -Last $maxRows)
    foreach ($r in $shownR) {
      $subj = [string]$r.subject; if ([string]::IsNullOrWhiteSpace($subj)) { $subj = [string]$r.kind }
      $batch = [string]$r.batchId
      $batchTag = if (-not [string]::IsNullOrWhiteSpace($batch)) { " | batch=$batch" } else { '' }
      Write-Host ("- {0} | target={1} | status={2}{3} | reason={4}" -f $subj, [string]$r.target, [string]$r.status, $batchTag, [string]$r.reason)
    }
    $restR = $rollbacks.Count - $shownR.Count
    if ($restR -gt 0) { Write-Host "- 另有 $restR 条 rollback" }
    Write-Host ''
  }
  Write-Host '建议下一步：'
  $open = @(Get-RekitOpenBoardLanes -Board $board)
  if ($open.Count -eq 1) {
    $label = Get-RekitWorkstreamLabel -Lane $open[0]
    Write-Host ("- 接手当前工作线：/rekit continue {0}" -f $label)
  } else {
    foreach ($lane in $open) {
      $kind = if ($lane.authority) { '主线' } else { '功能支线' }
      $label = Get-RekitWorkstreamLabel -Lane $lane
      Write-Host ("- 接手{0} {1}：/rekit continue {2}" -f $kind, $lane.id, $label)
    }
  }
  Write-Host '- 创建或进入功能支线：/rekit start <name>'
  Write-Host '- 生成项目级接手索引：/rekit handoff'
  Write-Host '- 生成指定工作线接手文档：/rekit handoff main 或 /rekit handoff <name>'
}

function Invoke-RekitContinue {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string[]]$ActionArgs = @(),
    [string]$Format = '',
    [switch]$WhatIf
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $formatValue = ([string]$Format).Trim().ToLowerInvariant()
  if ([string]::IsNullOrWhiteSpace($formatValue)) { $formatValue = 'table' }
  if (@('table','text','tsv','json') -notcontains $formatValue) { throw "unsupported continue format: $Format" }
  if (-not $WhatIf -and $formatValue -eq 'json') { throw 'continue -Format json currently supports -WhatIf preview only; omit -Format for apply' }
  if ($WhatIf) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    $boardPath = (Get-RekitBoardPaths -CaseRoot $caseRoot).Board
    if (-not (Test-Path -LiteralPath $boardPath)) { throw 'continue -WhatIf requires existing .rekit state; run /rekit overview first or run continue without -WhatIf after confirmation.' }
    $board = Read-RekitJsonFile -Path $boardPath
  } else {
    $board = Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane
  }
  $selector = Get-RekitActionSelector -ActionArgs $ActionArgs
  if ([string]::IsNullOrWhiteSpace($selector)) {
    $open = @(Get-RekitOpenBoardLanes -Board $board)
    if ($open.Count -eq 1) {
      $selector = Get-RekitWorkstreamLabel -Lane $open[0]
    } else {
      if ($formatValue -eq 'json') { throw 'continue -WhatIf -Format json requires exactly one open workstream or an explicit selector' }
      Write-RekitWorkstreamChoices -Board $board -Prefix '检测到多条 open 工作线；/rekit continue 不会猜测当前会话身份。'
      Write-Host '请使用 /rekit continue main 或 /rekit continue <name>。'
      return
    }
  }
  $lane = Resolve-RekitWorkstreamSelector -CaseRoot $caseRoot -Board $board -Selector $selector
  if ($null -eq $lane) {
    Write-RekitWorkstreamChoices -Board $board -Prefix "找不到工作线：$selector"
    throw "unknown workstream selector: $selector"
  }
  if ($WhatIf -and $formatValue -eq 'json') {
    $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
    New-RekitContinuePreview -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -Manifest $manifest -Selector $selector -Lane $lane | ConvertTo-Json -Depth 20
    return
  }
  Invoke-RekitAuto -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ActionArgs @($selector) -FocusLaneId ([string]$lane.id) -WhatIf:$WhatIf
  $resume = if ($WhatIf) { Join-RekitPath -Root (Join-RekitPath -Root $caseRoot -RelativePath ([string]$lane.laneRoot)) -RelativePath 'prompts/RESUME.md' } else { Write-RekitLaneResume -CaseRoot $caseRoot -LaneId ([string]$lane.id) }
  Write-Host "已选择工作线：$($lane.id)"
  Write-Host "工作区：$($lane.workspace)"
  Write-Host "接续提示：$(Join-RekitRelativePath -Root $caseRoot -Path $resume)"
  if ([bool]$lane.authority) {
    $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
    $handoffRel = Get-RekitHandoffPath -Manifest $manifest
    if (-not [string]::IsNullOrWhiteSpace($handoffRel)) {
      $taskHandoff = Join-RekitPath -Root $caseRoot -RelativePath $handoffRel
      if (Test-Path -LiteralPath $taskHandoff) { Write-Host "主线长期 handoff：$(Join-RekitRelativePath -Root $caseRoot -Path $taskHandoff)" }
    }
  }
}

function Invoke-RekitStart {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string[]]$ActionArgs = @(),
    [string]$Format = '',
    [switch]$WhatIf,
    [switch]$Force
  )
  if ($ActionArgs.Count -lt 1) { throw 'start requires a feature name, e.g. /rekit start login' }
  $name = Get-RekitActionSelector -ActionArgs $ActionArgs
  if ([string]::IsNullOrWhiteSpace($name)) { throw 'start requires a non-empty feature name' }
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $startLaneType = Get-RekitDefaultStartLaneTypeId -Manifest $manifest
  $formatValue = ([string]$Format).Trim().ToLowerInvariant()
  if ([string]::IsNullOrWhiteSpace($formatValue)) { $formatValue = 'table' }
  if (@('table','text','tsv','json') -notcontains $formatValue) { throw "unsupported start format: $Format" }
  if ($WhatIf) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    $laneType = Get-RekitLaneType -Manifest $manifest -Type $startLaneType
    $id = ConvertTo-RekitLaneId -Type $laneType.Id -Name $name
    if ([string]::IsNullOrWhiteSpace($id)) { throw "start produced an empty lane id for '$name'" }
    if ($formatValue -eq 'json') {
      $laneRoot = Get-RekitLanePath -CaseRoot $caseRoot -LaneId $id
      $laneFile = Join-Path $laneRoot 'lane.json'
      $workspace = Join-RekitPath -Root $caseRoot -RelativePath (Join-Path $laneType.WorkspaceRoot $id)
      $action = 'would-create-lane'
      if (Test-Path -LiteralPath $laneFile) {
        $action = if ($Force) { 'would-refresh-lane-with-force' } else { 'would-enter-existing-lane' }
      }
      [ordered]@{
        schemaVersion = 1
        command = 'start'
        caseRoot = $caseRoot
        repoRoot = $RepoRoot
        pack = $Pack
        isMutation = $false
        applied = $false
        requiresConfirmation = $true
        lane = [ordered]@{
          schemaVersion = 1
          id = $id
          type = [string]$laneType.Id
          name = $name
          title = if ([string]::IsNullOrWhiteSpace($name)) { [string]$laneType.Title } else { [string]$laneType.Title + ': ' + $name }
          status = 'open'
          authority = [bool]$laneType.Authority
          workspace = (Join-RekitRelativePath -Root $caseRoot -Path $workspace)
          laneRoot = (Join-RekitRelativePath -Root $caseRoot -Path $laneRoot)
          canWrite = @($laneType.CanWrite)
          readOnly = @($laneType.ReadOnly)
          outputs = @($laneType.Outputs)
          counters = [ordered]@{ observations = 0; requests = 0; candidates = 0; publications = 0; pendingUser = 0 }
          createdAt = ''
          updatedAt = ''
        }
        writes = @([ordered]@{ path = ".rekit/lanes/$id/lane.json"; kind = 'lane'; action = $action; targetPath = $laneFile })
        blockedActions = @('authority/confirmed writes','heavy-tool execution','handoff writes','continue auto-apply')
        nextSteps = @(
          'review this plan, then re-run start without -WhatIf to create or enter the workstream',
          'PowerShell /rekit remains the public entrypoint; start preview stays read-only'
        )
      } | ConvertTo-Json -Depth 12
      return
    }
    Write-Host "would create or enter feature workstream: $id"
    return
  }
  if ($formatValue -eq 'json') { throw 'start -Format json currently supports -WhatIf preview only; omit -Format for apply' }
  [void](Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane)
  $lane = New-RekitLane -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest -Type $startLaneType -Name $name -Force:$Force
  [void](Save-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest)
  Write-Host "功能支线已准备：$($lane.id)"
  Write-Host "工作区：$($lane.workspace)"
  Write-Host "接续提示：$($lane.laneRoot)/prompts/RESUME.md"
  Write-Host "继续此支线：/rekit continue $(Get-RekitWorkstreamLabel -Lane $lane)"
}


function Invoke-RekitNote {
  <#
    Internal command: append a single fact event to .rekit/facts/*.jsonl. Lets the main agent persist
    observation/candidate/request/publication/decision without the continue auto flow. Append-only,
    dedups by eventId, does not write authority files or kit templates.
    Args: -Kind <observation|candidate|request|publication|decision> -Lane <id> [-Subject ...] [-Summary ...]
          [-Confidence low|medium|high] [-Decision confirm|reject|defer|supersede] [-Reason ...] [-Status ...]
          [-EvidenceRefs id1,id2]
  #>
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string]$Kind = '',
    [string]$Lane = '',
    [string]$Subject = '',
    [string]$Summary = '',
    [string]$Actor = '',
    [string]$Risk = '',
    [string]$Related = '',
    [string]$Confidence = '',
    [string]$Decision = '',
    [string]$Reason = '',
    [string]$Status = '',
    [string]$BatchId = '',
    [string]$TargetRef = '',
    [string]$Verifier = '',
    [string]$Verdict = '',
    [string]$Action = '',
    [string]$ApprovedBy = '',
    [string]$Scope = '',
    [string]$Expires = '',
    [string]$EvidenceRefs = '',
    [string]$Format = '',
    [switch]$List,
    [switch]$WhatIf
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
  $validKinds = @('observation','candidate','request','publication','decision','hypothesis','verification','intervention','rollback')
  if ($List) {
    $formatValue = ([string]$Format).Trim().ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($formatValue)) { $formatValue = 'table' }
    $paths = Get-RekitBoardPaths -CaseRoot $caseRoot
    $kindsToList = if ([string]::IsNullOrWhiteSpace($Kind)) { $validKinds } else { @($Kind) }
    foreach ($k in $kindsToList) { if ($validKinds -notcontains $k) { throw "invalid note kind: $k" } }
    $maxRows = 20
    $groups = @()
    $eventCount = 0
    foreach ($k in $kindsToList) {
      $file = Get-RekitFactFilePath -Paths $paths -Kind $k
      $items = @(Read-RekitJsonLines -Path $file)
      if (-not [string]::IsNullOrWhiteSpace($Lane)) { $items = @($items | Where-Object { [string]$_.lane -eq $Lane }) }
      if ($items.Count -eq 0) { continue }
      $shown = @($items | Select-Object -Last $maxRows)
      $eventCount += $items.Count
      $groups += [ordered]@{ kind = $k; total = [int]$items.Count; shown = [int]$shown.Count; events = @($shown) }
    }
    switch ($formatValue) {
      { $_ -in @('table','text','tsv') } {
        foreach ($group in $groups) {
          $k = [string]$group.kind
          Write-Host "[$k] ($($group.total) 条)"
          foreach ($it in @($group.events)) {
            $subj = [string]$it.subject; if ([string]::IsNullOrWhiteSpace($subj)) { $subj = [string]$it.kind }
            $ln = [string]$it.lane
            $extra = ''
            if ($k -eq 'candidate') { $extra = " | confidence=$([string]$it.confidence) | status=$([string]$it.status) | risk=$([string]$it.risk)" }
            if ($k -eq 'decision') { $dec = [string]$it.decision; if ([string]::IsNullOrWhiteSpace($dec)) { $dec = [string]$it.action }; $by = [string]$it.confirmedBy; if ([string]::IsNullOrWhiteSpace($by)) { $by = [string]$it.actor }; $extra = " | decision=$dec | by=$by" }
            if ($k -eq 'request') { $extra = Format-RekitGateRequestDetail -Event $it -OmitBatch }
            if ($k -eq 'verification') { $extra = " | verifier=$([string]$it.verifier) | verdict=$([string]$it.verdict) | target=$([string]$it.target)" }
            if ($k -eq 'intervention') { $extra = " | action=$([string]$it.action) | target=$([string]$it.target) | approvedBy=$([string]$it.approvedBy) | scope=$([string]$it.scope) | status=$([string]$it.status) | reason=$([string]$it.reason)" }
            if ($k -eq 'rollback') { $extra = " | target=$([string]$it.target) | status=$([string]$it.status) | reason=$([string]$it.reason)" }
            $batch = [string]$it.batchId
            if (-not [string]::IsNullOrWhiteSpace($batch)) { $extra += " | batch=$batch" }
            Write-Host ("- {0} | lane={1}{2}" -f $subj, $ln, $extra)
          }
          $rest = [int]$group.total - [int]$group.shown
          if ($rest -gt 0) { Write-Host "- 另有 $rest 条 $k" }
          Write-Host ''
        }
      }
      'json' {
        [ordered]@{
          schemaVersion = 1
          command = 'note'
          caseRoot = $caseRoot
          repoRoot = $RepoRoot
          pack = $Pack
          isMutation = $false
          kind = ([string]$Kind).Trim().ToLowerInvariant()
          lane = ([string]$Lane).Trim()
          eventCount = [int]$eventCount
          groups = $groups
        } | ConvertTo-Json -Depth 20
      }
      default { throw "unsupported note list format: $Format" }
    }
    return
  }
  if ([string]::IsNullOrWhiteSpace($Kind)) { throw 'note requires -Kind observation|candidate|request|publication|decision|hypothesis|verification|intervention|rollback' }
  if ([string]::IsNullOrWhiteSpace($Lane)) { throw 'note requires -Lane <lane id>' }
  if ($validKinds -notcontains $Kind) { throw "invalid note kind: $Kind" }
  $refs = @(Split-RekitScalarList $EvidenceRefs)
  if ($WhatIf) {
    Write-Host "would append $Kind event to lane=$Lane subject=$Subject"
    return
  }
  $board = Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack
  $result = Add-RekitFactEvent -CaseRoot $caseRoot -Kind $Kind -Lane $Lane -Subject $Subject -Summary $Summary -Actor $Actor -Risk $Risk -Related $Related -Confidence $Confidence -Decision $Decision -Reason $Reason -Status $Status -BatchId $BatchId -Target $TargetRef -Verifier $Verifier -Verdict $Verdict -Action $Action -ApprovedBy $ApprovedBy -Scope $Scope -Expires $Expires -EvidenceRefs $refs -Board $board
  if ($result.Applied) {
    Write-Host "已记录 $Kind 事件：$($result.EventId)"
    Write-Host "账本：$($result.Path)"
  } else {
    Write-Host "跳过 $Kind 事件：$($result.EventId)（$($result.Reason)）"
  }
}
