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
    [string]$Pack = 'vmp-re'
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $board = Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane
  $paths = Get-RekitBoardPaths -CaseRoot $caseRoot
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
    [switch]$WhatIf
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
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
    [switch]$WhatIf,
    [switch]$Force
  )
  if ($ActionArgs.Count -lt 1) { throw 'start requires a feature name, e.g. /rekit start login' }
  $name = Get-RekitActionSelector -ActionArgs $ActionArgs
  if ([string]::IsNullOrWhiteSpace($name)) { throw 'start requires a non-empty feature name' }
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $startLaneType = Get-RekitDefaultStartLaneTypeId -Manifest $manifest
  if ($WhatIf) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    [void](Get-RekitLaneType -Manifest $manifest -Type $startLaneType)
    $id = ConvertTo-RekitLaneId -Type $startLaneType -Name $name
    Write-Host "would create or enter feature workstream: $id"
    return
  }
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
    [switch]$List,
    [switch]$WhatIf
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
  $validKinds = @('observation','candidate','request','publication','decision','hypothesis','verification','intervention','rollback')
  if ($List) {
    $paths = Get-RekitBoardPaths -CaseRoot $caseRoot
    $kindsToList = if ([string]::IsNullOrWhiteSpace($Kind)) { $validKinds } else { @($Kind) }
    $hasFilter = $false
    foreach ($k in $kindsToList) { if ($validKinds -notcontains $k) { throw "invalid note kind: $k" } }
    $maxRows = 20
    foreach ($k in $kindsToList) {
      $file = Get-RekitFactFilePath -Paths $paths -Kind $k
      $items = @(Read-RekitJsonLines -Path $file)
      if (-not [string]::IsNullOrWhiteSpace($Lane)) { $items = @($items | Where-Object { [string]$_.lane -eq $Lane }); $hasFilter = $true }
      if ($items.Count -eq 0) { continue }
      Write-Host "[$k] ($($items.Count) 条)"
      $shown = @($items | Select-Object -Last $maxRows)
      foreach ($it in $shown) {
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
      $rest = $items.Count - $shown.Count
      if ($rest -gt 0) { Write-Host "- 另有 $rest 条 $k" }
      Write-Host ''
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
