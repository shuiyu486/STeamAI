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
  Write-Host ''
  Write-Host '工作线：'
  foreach ($lane in @($board.lanes)) {
    $kind = if ($lane.authority) { '主线' } else { '功能/工具支线' }
    Write-Host ("- {0}：{1}，状态={2}，工作区={3}" -f $kind, $lane.id, $lane.status, $lane.workspace)
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
  Write-Host '- 继续推进当前项目：/rekit continue'
  Write-Host '- 开新功能支线：/rekit start <name>'
  Write-Host '- 生成新会话接手包：/rekit handoff'
}

function Invoke-RekitContinue {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string[]]$ActionArgs = @(),
    [switch]$WhatIf
  )
  Invoke-RekitAuto -Target $Target -RepoRoot $RepoRoot -Pack $Pack -ActionArgs $ActionArgs -WhatIf:$WhatIf
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
  $name = (($ActionArgs | ForEach-Object { [string]$_ }) -join '-').Trim('-')
  if ([string]::IsNullOrWhiteSpace($name)) { throw 'start requires a non-empty feature name' }
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  if ($WhatIf) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    [void](Get-RekitLaneType -Manifest $manifest -Type 'feature-analysis')
    $id = ConvertTo-RekitLaneId -Type 'feature-analysis' -Name $name
    Write-Host "would create feature workstream: $id"
    return
  }
  [void](Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane)
  $lane = New-RekitLane -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest -Type 'feature-analysis' -Name $name -Force:$Force
  [void](Save-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Manifest $manifest)
  Write-Host "功能支线已准备：$($lane.id)"
  Write-Host "工作区：$($lane.workspace)"
  Write-Host "接续提示：$($lane.laneRoot)/prompts/RESUME.md"
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

function Write-RekitHandoff {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [switch]$WhatIf
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  if ($WhatIf) {
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    Write-Host "would write handoff package: .rekit/handovers/latest.md"
    return
  }
  $board = Ensure-RekitBoard -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -CreateDefaultLane
  $paths = Get-RekitBoardPaths -CaseRoot $caseRoot
  $handovers = Join-Path $paths.Root 'handovers'
  Ensure-RekitDirectory $handovers
  $stamp = New-RekitBoardTimestamp
  $handoffPath = Join-Path $handovers ($stamp + '.md')
  $latestPath = Join-Path $handovers 'latest.md'
  $taskHandoff = Join-Path $caseRoot 'references/vmp-re/task-handoff.md'
  $latestDigest = Get-RekitLatestRunDigestPath -CaseRoot $caseRoot
  $lines = New-Object System.Collections.Generic.List[string]
  $lines.Add('# rekit 新会话接手包')
  $lines.Add('')
  $lines.Add("生成时间：$(New-RekitIsoTime)")
  $lines.Add("case：$caseRoot")
  $lines.Add("pack：$Pack")
  $lines.Add('')
  $lines.Add('## 新会话开场')
  $lines.Add('')
  $lines.Add('直接说：按 `.rekit/handovers/latest.md` 接手继续。')
  $lines.Add('')
  $lines.Add('## 推荐读取')
  $lines.Add('')
  if (Test-Path -LiteralPath $taskHandoff) { $lines.Add('- `' + (Join-RekitRelativePath -Root $caseRoot -Path $taskHandoff) + '`：case 长期主线 handoff。') }
  if (-not [string]::IsNullOrWhiteSpace($latestDigest)) { $lines.Add('- `' + (Join-RekitRelativePath -Root $caseRoot -Path $latestDigest) + '`：最近一次自动整理摘要。') }
  $lines.Add('')
  $lines.Add('## 工作线')
  $lines.Add('')
  foreach ($lane in @($board.lanes)) {
    $laneFull = Read-RekitLane -CaseRoot $caseRoot -LaneId ([string]$lane.id)
    if ($null -eq $laneFull) { continue }
    $resume = Write-RekitLaneResume -CaseRoot $caseRoot -LaneId ([string]$laneFull.id)
    $kind = if ($laneFull.authority) { '主线' } else { '支线' }
    $lines.Add(('- {0} `{1}`：status={2}，workspace=`{3}`' -f $kind, $laneFull.id, $laneFull.status, $laneFull.workspace))
    $relResume = Join-RekitRelativePath -Root $caseRoot -Path $resume
    $lines.Add(('  - 接续提示：`{0}`' -f $relResume))
  }
  $lines.Add('')
  $lines.Add('## 注意边界')
  $lines.Add('')
  $lines.Add('- 主线负责最终结论写入；功能支线只写自己的工作区、证据、候选和请求。')
  $lines.Add('- 本接手包不会覆盖 `references/vmp-re/task-handoff.md`，只引用它。')
  $lines.Add('- 继续推进时使用 `/rekit continue`；开新功能支线用 `/rekit start <name>`。')
  $text = ($lines -join "`r`n") + "`r`n"
  [System.IO.File]::WriteAllText($handoffPath, $text, [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText($latestPath, $text, [System.Text.UTF8Encoding]::new($false))
  Write-Host "新会话接手包：$(Join-RekitRelativePath -Root $caseRoot -Path $latestPath)"
}
