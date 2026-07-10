# Handoff subsystem: project-level and lane-level handoff document generation.
# Mechanically moved from B3.Commands.ps1 (C7) to keep B3.Commands.ps1 focused on
# user-facing command entrypoints. Behavior unchanged.

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
    [Parameter(Mandatory=$true)]$Manifest,
    [Parameter(Mandatory=$true)][string]$Pack,
    [Parameter(Mandatory=$true)][string]$Handovers,
    [Parameter(Mandatory=$true)][string]$Stamp
  )
  $handoffPath = Join-Path $Handovers ($Stamp + '.md')
  $latestPath = Join-Path $Handovers 'latest.md'
  $handoffRel = Get-RekitHandoffPath -Manifest $Manifest
  $taskHandoff = if ([string]::IsNullOrWhiteSpace($handoffRel)) { '' } else { Join-RekitPath -Root $CaseRoot -RelativePath $handoffRel }
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
  if (-not [string]::IsNullOrWhiteSpace($taskHandoff) -and (Test-Path -LiteralPath $taskHandoff)) { $lines.Add('- `' + (Join-RekitRelativePath -Root $CaseRoot -Path $taskHandoff) + '`：case 长期主线 handoff。') }
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
  if (-not [string]::IsNullOrWhiteSpace($handoffRel)) { $lines.Add('- 本索引不会覆盖 `' + $handoffRel + '`，只引用它。') }
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
    [Parameter(Mandatory=$true)]$Manifest,
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
    $handoffRel = Get-RekitHandoffPath -Manifest $Manifest
    if (-not [string]::IsNullOrWhiteSpace($handoffRel)) {
      $taskHandoff = Join-RekitPath -Root $CaseRoot -RelativePath $handoffRel
      if (Test-Path -LiteralPath $taskHandoff) { $lines.Add('- `' + (Join-RekitRelativePath -Root $CaseRoot -Path $taskHandoff) + '`：case 长期主线 handoff。') }
    }
  }
  if (-not [string]::IsNullOrWhiteSpace($latestDigest)) { $lines.Add('- `' + (Join-RekitRelativePath -Root $CaseRoot -Path $latestDigest) + '`：最近一次自动整理摘要。') }
  $lines.Add('')
  $workspaceRel = [string]$Lane.workspace
  if (-not [string]::IsNullOrWhiteSpace($workspaceRel)) {
    $workspaceAbs = Join-RekitPath -Root $CaseRoot -RelativePath $workspaceRel
    if (Test-Path -LiteralPath $workspaceAbs) {
      $packets = @(Get-ChildItem -LiteralPath $workspaceAbs -Filter *.md -File -ErrorAction SilentlyContinue | Sort-Object Name)
      if ($packets.Count -gt 0) {
        $lines.Add('## workspace packet')
        $lines.Add('')
        foreach ($p in $packets) {
          $lines.Add('- `' + (Join-RekitRelativePath -Root $CaseRoot -Path $p.FullName) + '`：workspace 产物，含 evidence/candidate/decision packet。')
        }
        $lines.Add('')
      }
    }
  }
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  $decisions = @(Read-RekitJsonLines -Path $paths.Decisions | Where-Object { [string]$_.lane -eq $laneId } | Select-Object -Last 5)
  if ($decisions.Count -gt 0) {
    $lines.Add('## decision')
    $lines.Add('')
    foreach ($d in $decisions) {
      $subj = [string]$d.subject; if ([string]::IsNullOrWhiteSpace($subj)) { $subj = [string]$d.kind }
      $dec = [string]$d.decision; if ([string]::IsNullOrWhiteSpace($dec)) { $dec = [string]$d.action }
      $rson = [string]$d.reason
      $by = [string]$d.confirmedBy; if ([string]::IsNullOrWhiteSpace($by)) { $by = [string]$d.actor }
      $byTag = if (-not [string]::IsNullOrWhiteSpace($by)) { " | by=$by" } else { '' }
      $lines.Add(("- {0} | decision={1}{2} | reason={3}" -f $subj, $dec, $byTag, $rson))
    }
    $lines.Add('')
  }
  $pendingGates = @(Read-RekitJsonLines -Path $paths.Requests | Where-Object { [string]$_.status -eq 'pending-gate' -and [string]$_.lane -eq $laneId } | Select-Object -Last 5)
  if ($pendingGates.Count -gt 0) {
    $lines.Add('## pending-gate')
    $lines.Add('')
    foreach ($g in $pendingGates) {
      $detail = Format-RekitGateRequestDetail -Event $g -OmitStatus
      $lines.Add(("- {0} | {1}{2}" -f [string]$g.subject, [string]$g.summary, $detail))
    }
    $lines.Add('')
  }
  $interventions = @(Read-RekitJsonLines -Path $paths.Interventions | Where-Object { [string]$_.lane -eq $laneId } | Select-Object -Last 5)
  if ($interventions.Count -gt 0) {
    $lines.Add('## intervention')
    $lines.Add('')
    foreach ($i in $interventions) {
      $subj = [string]$i.subject; if ([string]::IsNullOrWhiteSpace($subj)) { $subj = [string]$i.action }
      $batch = [string]$i.batchId
      $batchTag = if (-not [string]::IsNullOrWhiteSpace($batch)) { " | batch=$batch" } else { '' }
      $lines.Add(("- {0} | action={1} | target={2} | approvedBy={3} | scope={4} | status={5}{6}" -f $subj, [string]$i.action, [string]$i.target, [string]$i.approvedBy, [string]$i.scope, [string]$i.status, $batchTag))
    }
    $lines.Add('')
  }
  $rollbacks = @(Read-RekitJsonLines -Path $paths.Rollbacks | Where-Object { [string]$_.lane -eq $laneId } | Select-Object -Last 5)
  if ($rollbacks.Count -gt 0) {
    $lines.Add('## rollback')
    $lines.Add('')
    foreach ($r in $rollbacks) {
      $subj = [string]$r.subject; if ([string]::IsNullOrWhiteSpace($subj)) { $subj = [string]$r.kind }
      $batch = [string]$r.batchId
      $batchTag = if (-not [string]::IsNullOrWhiteSpace($batch)) { " | batch=$batch" } else { '' }
      $lines.Add(("- {0} | target={1} | status={2}{3} | reason={4}" -f $subj, [string]$r.target, [string]$r.status, $batchTag, [string]$r.reason))
    }
    $lines.Add('')
  }
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
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $paths = Get-RekitBoardPaths -CaseRoot $caseRoot
  $handovers = Join-Path $paths.Root 'handovers'
  Ensure-RekitDirectory $handovers
  $stamp = New-RekitBoardTimestamp
  if ([string]::IsNullOrWhiteSpace($selector)) {
    $path = Write-RekitProjectHandoff -CaseRoot $caseRoot -Board $board -Manifest $manifest -Pack $Pack -Handovers $handovers -Stamp $stamp
    Write-Host "项目级接手索引：$(Join-RekitRelativePath -Root $caseRoot -Path $path)"
    return
  }
  $lane = Resolve-RekitWorkstreamSelector -CaseRoot $caseRoot -Board $board -Selector $selector
  if ($null -eq $lane) {
    Write-RekitWorkstreamChoices -Board $board -Prefix "找不到工作线：$selector"
    throw "unknown workstream selector: $selector"
  }
  $path = Write-RekitLaneHandoff -CaseRoot $caseRoot -Lane $lane -Manifest $manifest -Pack $Pack -Handovers $handovers -Stamp $stamp
  Write-Host "工作线接手文档：$(Join-RekitRelativePath -Root $caseRoot -Path $path)"
}
