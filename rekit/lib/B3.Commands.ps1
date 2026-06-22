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
  $pending = @((Read-RekitJsonLines -Path $paths.Decisions) | Where-Object { $_.action -eq 'pending-user' }).Count
  Write-Host ''
  Write-Host '共享事实：'
  Write-Host "- observation: $obs"
  Write-Host "- request: $req"
  Write-Host "- candidate: $cand"
  Write-Host "- publication: $pub"
  Write-Host "- 需要确认: $pending"
  Write-Host ''
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
    $taskHandoff = Join-Path $caseRoot 'references/vmp-re/task-handoff.md'
    if (Test-Path -LiteralPath $taskHandoff) { Write-Host "主线长期 handoff：$(Join-RekitRelativePath -Root $caseRoot -Path $taskHandoff)" }
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
  if ($WhatIf) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    [void](Get-RekitLaneType -Manifest $manifest -Type 'feature-analysis')
    $id = ConvertTo-RekitLaneId -Type 'feature-analysis' -Name $name
    Write-Host "would create or enter feature workstream: $id"
    return
  }
  [void](Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane)
  $lane = New-RekitLane -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest -Type 'feature-analysis' -Name $name -Force:$Force
  [void](Save-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest)
  Write-Host "功能支线已准备：$($lane.id)"
  Write-Host "工作区：$($lane.workspace)"
  Write-Host "接续提示：$($lane.laneRoot)/prompts/RESUME.md"
  Write-Host "继续此支线：/rekit continue $(Get-RekitWorkstreamLabel -Lane $lane)"
}

function Get-RekitLatestRunDigestPath {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  if (-not (Test-Path -LiteralPath $paths.Runs)) { return '' }
  $latest = Get-ChildItem -LiteralPath $paths.Runs -Directory | Sort-Object Name -Descending | Select-Object -First 1
  if ($null -eq $latest) { return '' }
  $digest = Join-Path $latest.FullName 'digest.md'
  if (Test-Path -LiteralPath $digest) { return $digest }
  return ''
}

function Write-RekitProjectHandoff {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Board,
    [Parameter(Mandatory=$true)][string]$Pack,
    [Parameter(Mandatory=$true)][string]$Handovers,
    [Parameter(Mandatory=$true)][string]$Stamp
  )
  $handoffPath = Join-Path $Handovers ($Stamp + '.md')
  $latestPath = Join-Path $Handovers 'latest.md'
  $taskHandoff = Join-Path $CaseRoot 'references/vmp-re/task-handoff.md'
  $latestDigest = Get-RekitLatestRunDigestPath -CaseRoot $CaseRoot
  $lines = New-Object System.Collections.Generic.List[string]
  $lines.Add('# rekit 项目接手索引')
  $lines.Add('')
  $lines.Add("生成时间：$(New-RekitIsoTime)")
  $lines.Add("case：$CaseRoot")
  $lines.Add("pack：$Pack")
  $lines.Add('')
  $lines.Add('## 说明')
  $lines.Add('')
  $lines.Add('这是项目级索引，不代表某个会话已经选择主线或支线。新会话应先选择要接手的工作线。')
  $lines.Add('')
  $lines.Add('## 推荐读取')
  $lines.Add('')
  if (Test-Path -LiteralPath $taskHandoff) { $lines.Add('- `' + (Join-RekitRelativePath -Root $CaseRoot -Path $taskHandoff) + '`：case 长期主线 handoff。') }
  if (-not [string]::IsNullOrWhiteSpace($latestDigest)) { $lines.Add('- `' + (Join-RekitRelativePath -Root $CaseRoot -Path $latestDigest) + '`：最近一次自动整理摘要。') }
  $lines.Add('')
  $lines.Add('## 工作线')
  $lines.Add('')
  foreach ($lane in @($Board.lanes)) {
    $laneFull = Read-RekitLane -CaseRoot $CaseRoot -LaneId ([string]$lane.id)
    if ($null -eq $laneFull) { continue }
    $resume = Write-RekitLaneResume -CaseRoot $CaseRoot -LaneId ([string]$laneFull.id)
    $kind = if ($laneFull.authority) { '主线' } else { '功能支线' }
    $label = Get-RekitWorkstreamLabel -Lane $laneFull
    $lines.Add(('- {0} `{1}`：status={2}，workspace=`{3}`' -f $kind, $laneFull.id, $laneFull.status, $laneFull.workspace))
    $lines.Add(('  - 接手：`/rekit continue {0}`' -f $label))
    $lines.Add(('  - 指定交接：`/rekit handoff {0}`' -f $label))
    $lines.Add(('  - 接续提示：`{0}`' -f (Join-RekitRelativePath -Root $CaseRoot -Path $resume)))
  }
  $lines.Add('')
  $lines.Add('## 注意边界')
  $lines.Add('')
  $lines.Add('- 主线负责最终结论写入；功能支线只写自己的工作区、证据、候选和请求。')
  $lines.Add('- 本索引不会覆盖 `references/vmp-re/task-handoff.md`，只引用它。')
  $lines.Add('- 多工作线时不要使用无参数 `/rekit continue` 盲目继续，应使用 `/rekit continue main` 或 `/rekit continue <name>`。')
  $text = ($lines -join "`r`n") + "`r`n"
  [System.IO.File]::WriteAllText($handoffPath, $text, [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText($latestPath, $text, [System.Text.UTF8Encoding]::new($false))
  return $latestPath
}

function Write-RekitLaneHandoff {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Lane,
    [Parameter(Mandatory=$true)][string]$Pack,
    [Parameter(Mandatory=$true)][string]$Handovers,
    [Parameter(Mandatory=$true)][string]$Stamp
  )
  $laneId = [string]$Lane.id
  $handoffPath = Join-Path $Handovers ($laneId + '-' + $Stamp + '.md')
  $latestPath = Join-Path $Handovers ($laneId + '-latest.md')
  $resume = Write-RekitLaneResume -CaseRoot $CaseRoot -LaneId $laneId
  $latestDigest = Get-RekitLatestRunDigestPath -CaseRoot $CaseRoot
  $lines = New-Object System.Collections.Generic.List[string]
  $kind = if ($Lane.authority) { '主线' } else { '功能支线' }
  $label = Get-RekitWorkstreamLabel -Lane $Lane
  $lines.Add("# rekit 工作线接手：$laneId")
  $lines.Add('')
  $lines.Add("生成时间：$(New-RekitIsoTime)")
  $lines.Add("case：$CaseRoot")
  $lines.Add("pack：$Pack")
  $lines.Add("类型：$kind")
  $lines.Add("状态：$($Lane.status)")
  $lines.Add("工作区：$($Lane.workspace)")
  $lines.Add('')
  $lines.Add('## 新会话开场')
  $lines.Add('')
  $lines.Add(("直接说：按 `{0}` 接手，然后执行 `/rekit continue {1}`。" -f (Join-RekitRelativePath -Root $CaseRoot -Path $latestPath), $label))
  $lines.Add('')
  $lines.Add('## 推荐读取')
  $lines.Add('')
  $lines.Add('- `' + (Join-RekitRelativePath -Root $CaseRoot -Path $resume) + '`：本工作线接续提示。')
  if ([bool]$Lane.authority) {
    $taskHandoff = Join-Path $CaseRoot 'references/vmp-re/task-handoff.md'
    if (Test-Path -LiteralPath $taskHandoff) { $lines.Add('- `' + (Join-RekitRelativePath -Root $CaseRoot -Path $taskHandoff) + '`：case 长期主线 handoff。') }
  }
  if (-not [string]::IsNullOrWhiteSpace($latestDigest)) { $lines.Add('- `' + (Join-RekitRelativePath -Root $CaseRoot -Path $latestDigest) + '`：最近一次自动整理摘要。') }
  $lines.Add('')
  $lines.Add('## 边界')
  $lines.Add('')
  if ([bool]$Lane.authority) {
    $lines.Add('- 这是主线；可以维护最终结论、验证和长期 handoff。')
  } else {
    $lines.Add('- 这是功能支线；只写自己的 workspace、证据、候选和 request。')
  }
  $lines.Add('- 不并发写 IDB 注释/rename/type；不把完整 trace、disasm、decompile、dump 内容复制进 Markdown。')
  $text = ($lines -join "`r`n") + "`r`n"
  [System.IO.File]::WriteAllText($handoffPath, $text, [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText($latestPath, $text, [System.Text.UTF8Encoding]::new($false))
  return $latestPath
}

function Write-RekitHandoff {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string[]]$ActionArgs = @(),
    [switch]$WhatIf
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $selector = Get-RekitActionSelector -ActionArgs $ActionArgs
  if ($WhatIf) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    if ([string]::IsNullOrWhiteSpace($selector)) { Write-Host 'would write project handoff index: .rekit/handovers/latest.md' } else { Write-Host "would write workstream handoff: $selector" }
    return
  }
  $board = Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane
  $paths = Get-RekitBoardPaths -CaseRoot $caseRoot
  $handovers = Join-Path $paths.Root 'handovers'
  Ensure-RekitDirectory $handovers
  $stamp = New-RekitBoardTimestamp
  if ([string]::IsNullOrWhiteSpace($selector)) {
    $path = Write-RekitProjectHandoff -CaseRoot $caseRoot -Board $board -Pack $Pack -Handovers $handovers -Stamp $stamp
    Write-Host "项目级接手索引：$(Join-RekitRelativePath -Root $caseRoot -Path $path)"
    return
  }
  $lane = Resolve-RekitWorkstreamSelector -CaseRoot $caseRoot -Board $board -Selector $selector
  if ($null -eq $lane) {
    Write-RekitWorkstreamChoices -Board $board -Prefix "找不到工作线：$selector"
    throw "unknown workstream selector: $selector"
  }
  $path = Write-RekitLaneHandoff -CaseRoot $caseRoot -Lane $lane -Pack $Pack -Handovers $handovers -Stamp $stamp
  Write-Host "工作线接手文档：$(Join-RekitRelativePath -Root $caseRoot -Path $path)"
}
