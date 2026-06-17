function New-RekitReviewTimestamp {
  return (Get-Date -Format 'yyyyMMdd-HHmmssfff')
}

function ConvertTo-RekitSafeFileName {
  param([Parameter(Mandatory=$true)][string]$Value)
  $safe = $Value -replace '[\\/:*?"<>|]', '_'
  if ([string]::IsNullOrWhiteSpace($safe)) { return 'item' }
  return $safe
}

function Get-RekitReviewPaths {
  param(
    [Parameter(Mandatory=$true)][string]$Command,
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [string]$ReviewOutputDir = '',
    [string]$PacketPath = '',
    [string]$DiffPath = ''
  )
  $case = [System.IO.Path]::GetFullPath($CaseRoot)
  $root = if ([string]::IsNullOrWhiteSpace($ReviewOutputDir)) {
    Join-Path $case ('.rekit\reviews\' + (New-RekitReviewTimestamp) + '-' + $Command)
  } else {
    [System.IO.Path]::GetFullPath($ReviewOutputDir)
  }
  Ensure-RekitDirectory $root
  $diffRoot = Join-Path $root 'diffs'
  Ensure-RekitDirectory $diffRoot
  $previewRoot = Join-Path $root 'previews'
  Ensure-RekitDirectory $previewRoot

  $packet = if ([string]::IsNullOrWhiteSpace($PacketPath)) { Join-Path $root 'packet.json' } else { [System.IO.Path]::GetFullPath($PacketPath) }
  $summary = Join-Path $root 'summary.md'
  $combinedDiff = if ([string]::IsNullOrWhiteSpace($DiffPath)) { Join-Path $diffRoot 'combined.diff' } else { [System.IO.Path]::GetFullPath($DiffPath) }
  Ensure-RekitDirectory (Split-Path -Parent $packet)
  Ensure-RekitDirectory (Split-Path -Parent $combinedDiff)

  return [pscustomobject]@{
    Root = $root
    DiffRoot = $diffRoot
    PreviewRoot = $previewRoot
    PacketPath = $packet
    SummaryPath = $summary
    CombinedDiffPath = $combinedDiff
  }
}

function Get-RekitStateManagedEntry {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$RelativePath
  )
  $statePath = Join-Path $CaseRoot '.rekit\state.json'
  if (-not (Test-Path -LiteralPath $statePath)) { return $null }
  try {
    $state = [System.IO.File]::ReadAllText($statePath, [System.Text.Encoding]::UTF8) | ConvertFrom-Json
  } catch {
    return $null
  }
  if ($null -eq $state.managed) { return $null }
  $prop = $state.managed.PSObject.Properties[$RelativePath]
  if ($null -eq $prop) { return $null }
  return $prop.Value
}

function Read-RekitTextIfExists {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return '' }
  return [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
}

function Get-RekitFileBytesIfExists {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return 0 }
  return (Get-Item -LiteralPath $Path).Length
}

function New-RekitBoundedDiffText {
  param(
    [Parameter(Mandatory=$true)][string]$OldLabel,
    [Parameter(Mandatory=$true)][string]$OldText,
    [Parameter(Mandatory=$true)][string]$NewLabel,
    [Parameter(Mandatory=$true)][string]$NewText,
    [int]$MaxChanges = 120,
    [int]$MaxLineChars = 240
  )
  if ($OldText -eq $NewText) { return '' }
  $oldLines = $OldText -split "`r?`n", -1
  $newLines = $NewText -split "`r?`n", -1
  $max = [Math]::Max($oldLines.Count, $newLines.Count)
  $out = New-Object System.Collections.Generic.List[string]
  $out.Add("--- $OldLabel")
  $out.Add("+++ $NewLabel")
  $changes = 0
  for ($i = 0; $i -lt $max; $i++) {
    $old = if ($i -lt $oldLines.Count) { $oldLines[$i] } else { $null }
    $new = if ($i -lt $newLines.Count) { $newLines[$i] } else { $null }
    if ($old -ne $new) {
      $lineNo = $i + 1
      $out.Add("@@ line $lineNo @@")
      if ($null -ne $old) {
        if ($old.Length -gt $MaxLineChars) { $old = $old.Substring(0, $MaxLineChars) + ' ...<truncated line>' }
        $out.Add('- ' + $old)
      }
      if ($null -ne $new) {
        if ($new.Length -gt $MaxLineChars) { $new = $new.Substring(0, $MaxLineChars) + ' ...<truncated line>' }
        $out.Add('+ ' + $new)
      }
      $changes++
      if ($changes -ge $MaxChanges) {
        $out.Add('...<diff truncated>')
        break
      }
    }
  }
  return (($out -join "`r`n") + "`r`n")
}

function Write-RekitDiffFile {
  param(
    [Parameter(Mandatory=$true)][string]$DiffRoot,
    [Parameter(Mandatory=$true)][string]$RelativePath,
    [Parameter(Mandatory=$true)][string]$OldLabel,
    [Parameter(Mandatory=$true)][string]$OldText,
    [Parameter(Mandatory=$true)][string]$NewLabel,
    [Parameter(Mandatory=$true)][string]$NewText,
    [Parameter(Mandatory=$true)][string]$CombinedDiffPath
  )
  $diff = New-RekitBoundedDiffText -OldLabel $OldLabel -OldText $OldText -NewLabel $NewLabel -NewText $NewText
  if ([string]::IsNullOrWhiteSpace($diff)) { return '' }
  $file = Join-Path $DiffRoot ((ConvertTo-RekitSafeFileName $RelativePath) + '.diff')
  [System.IO.File]::WriteAllText($file, $diff, [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::AppendAllText($CombinedDiffPath, $diff + "`r`n", [System.Text.UTF8Encoding]::new($false))
  return $file
}

function Get-RekitManagedBlockAppliedText {
  param(
    [Parameter(Mandatory=$true)][string]$HostText,
    [Parameter(Mandatory=$true)][string]$BlockId,
    [Parameter(Mandatory=$true)][string]$BlockText
  )
  $block = $BlockText.Trim()
  if ([string]::IsNullOrWhiteSpace($HostText)) { return "# Project Context`r`n`r`n$block`r`n" }
  $pattern = '(?s)<!-- BEGIN ' + [regex]::Escape($BlockId) + '.*?<!-- END ' + [regex]::Escape($BlockId) + ' -->'
  if ([regex]::IsMatch($HostText, $pattern)) {
    return [regex]::Replace($HostText, $pattern, [System.Text.RegularExpressions.MatchEvaluator]{ param($m) $block })
  }
  return $HostText.TrimEnd() + "`r`n`r`n" + $block + "`r`n"
}

function Invoke-RekitRegexReplaceWithCount {
  param(
    [Parameter(Mandatory=$true)][string]$Text,
    [Parameter(Mandatory=$true)][string]$Pattern,
    [Parameter(Mandatory=$true)][string]$Replacement,
    [string]$Key = '',
    [hashtable]$Counts = @{},
    [System.Text.RegularExpressions.RegexOptions]$Options = [System.Text.RegularExpressions.RegexOptions]::IgnoreCase
  )
  $count = 0
  $result = [regex]::Replace($Text, $Pattern, [System.Text.RegularExpressions.MatchEvaluator]{ param($m) $script:__rekitReplaceCount++; return $Replacement }, $Options)
  $count = $script:__rekitReplaceCount
  $script:__rekitReplaceCount = 0
  if (-not [string]::IsNullOrWhiteSpace($Key)) { $Counts[$Key] = [int]$Counts[$Key] + $count }
  return $result
}

function Convert-RekitToolingCandidateTextWithStats {
  param(
    [Parameter(Mandatory=$true)][string]$Text,
    [Parameter(Mandatory=$true)][string]$CaseRoot
  )
  $counts = @{
    caseRoot = 0
    absolutePath = 0
    targetExe = 0
    casePath = 0
    caseTerm = 0
    toolsRoot = 0
    artifactsPath = 0
    capturesPath = 0
    traceFile = 0
    dumpFile = 0
    address = 0
    ctx = 0
    round = 0
    task = 0
  }
  $out = $Text
  $case = [regex]::Escape(([System.IO.Path]::GetFullPath($CaseRoot)).TrimEnd('\'))
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern $case -Replacement '<caseRoot>' -Key 'caseRoot' -Counts $counts
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern '[A-Za-z]:\\[^`\r\n|，。；;\)\] ]+' -Replacement '<absolutePath>' -Key 'absolutePath' -Counts $counts
  foreach ($pattern in (Get-RekitCaseSpecificPatterns -CaseRoot $CaseRoot)) {
    $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern ($pattern + '[A-Za-z0-9_.-]*\.exe') -Replacement '<target.exe>' -Key 'targetExe' -Counts $counts
    $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern ($pattern + '[\/]') -Replacement '<case>/' -Key 'casePath' -Counts $counts
    $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern $pattern -Replacement '<caseTerm>' -Key 'caseTerm' -Counts $counts
  }
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern '\.\.[\\/]tools[\\/]' -Replacement '<toolsRoot>/' -Key 'toolsRoot' -Counts $counts
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern 'artifacts[\\/][^`\r\n|，。；;\)\] ]+' -Replacement '<artifactsPath>' -Key 'artifactsPath' -Counts $counts
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern 'captures[\\/][^`\r\n|，。；;\)\] ]+' -Replacement '<capturesPath>' -Key 'capturesPath' -Counts $counts
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern '[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\.(csv|jsonl|log|txt|bin)' -Replacement '<traceFile>' -Key 'traceFile' -Counts $counts
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern '[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\.(dmp|bin|raw|exe|dll)' -Replacement '<dumpFile>' -Key 'dumpFile' -Counts $counts
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern '0x[0-9A-Fa-f]{6,}' -Replacement '<address>' -Key 'address' -Counts $counts -Options ([System.Text.RegularExpressions.RegexOptions]::None)
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern 'ctx\d+' -Replacement '<ctxNNN>' -Key 'ctx' -Counts $counts
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern 'round\d+' -Replacement '<roundN>' -Key 'round' -Counts $counts
  $out = Invoke-RekitRegexReplaceWithCount -Text $out -Pattern 'Task #\d+' -Replacement 'Task #<n>' -Key 'task' -Counts $counts
  return [pscustomobject]@{ Text = $out; ReplacementCounts = $counts }
}

function New-RekitToolingCandidateHeader {
  param([Parameter(Mandatory=$true)][string]$RelativePath)
  $lines = @(
    '# Tooling candidate from case',
    '',
    ('Source: `' + $RelativePath + '`'),
    'Generated by: `rekit promote`',
    '',
    '> Review before merging into `tooling/catalog.yml` or `tooling/recipes/*`.',
    '',
    '---',
    ''
  )
  return (($lines -join "`r`n") + "`r`n")
}

function Write-RekitReviewPacket {
  param(
    [Parameter(Mandatory=$true)]$Paths,
    [Parameter(Mandatory=$true)]$Packet,
    [string[]]$SummaryLines
  )
  [System.IO.File]::WriteAllText($Paths.PacketPath, ($Packet | ConvertTo-Json -Depth 12), [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText($Paths.SummaryPath, (($SummaryLines -join "`r`n") + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  Write-Host "review packet: $($Paths.PacketPath)"
  Write-Host "review summary: $($Paths.SummaryPath)"
  Write-Host "review diff: $($Paths.CombinedDiffPath)"
}

function Split-RekitPlanItems {
  param(
    [string]$Items = '',
    [string]$ItemsFile = ''
  )
  $text = $Items
  if (-not [string]::IsNullOrWhiteSpace($ItemsFile)) {
    $itemPath = [System.IO.Path]::GetFullPath($ItemsFile)
    if (-not (Test-Path -LiteralPath $itemPath)) { throw "missing plan items file: $itemPath" }
    $text = [System.IO.File]::ReadAllText($itemPath, [System.Text.Encoding]::UTF8)
  }
  if ([string]::IsNullOrWhiteSpace($text)) { return @() }
  $parts = $text -split '[,;\r\n\t ]+'
  return @($parts | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | ForEach-Object { $_.Trim() })
}

function Get-RekitSubagentRoute {
  param(
    [Parameter(Mandatory=$true)]$Manifest,
    [string]$Route = '',
    [string]$TaskType = ''
  )
  $routes = @($Manifest.SubagentRoutes)
  if ($routes.Count -eq 0) { throw "manifest has no subagentRoutes: $($Manifest.ManifestPath)" }
  if (-not [string]::IsNullOrWhiteSpace($Route)) {
    foreach ($item in $routes) {
      if ([string]::Equals([string]$item.id, $Route, [System.StringComparison]::OrdinalIgnoreCase)) { return $item }
    }
    throw "subagent route not found: $Route"
  }
  if (-not [string]::IsNullOrWhiteSpace($TaskType)) {
    foreach ($item in $routes) {
      $taskTypes = ([string]$item.taskTypes) -split '[,;]' | ForEach-Object { $_.Trim() }
      foreach ($task in $taskTypes) {
        if ([string]::Equals($task, $TaskType, [System.StringComparison]::OrdinalIgnoreCase)) { return $item }
      }
    }
  }
  return $routes[0]
}

function New-RekitPlanShards {
  param(
    [string[]]$Items,
    [int]$TargetItemsPerAgent = 4
  )
  if ($TargetItemsPerAgent -lt 1) { $TargetItemsPerAgent = 4 }
  $shards = @()
  for ($i = 0; $i -lt $Items.Count; $i += $TargetItemsPerAgent) {
    $end = [Math]::Min($i + $TargetItemsPerAgent - 1, $Items.Count - 1)
    $slice = @($Items[$i..$end])
    $shards += [pscustomobject]@{
      id = ('shard-{0:D2}' -f ($shards.Count + 1))
      items = $slice
      prompt = ('Review only these items: ' + ($slice -join ', ') + '. Return the route output contract only; do not write files or paste long logs.')
    }
  }
  return $shards
}

function Write-RekitSubagentPlan {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string]$Route = '',
    [string]$TaskType = '',
    [string]$Items = '',
    [string]$ItemsFile = '',
    [int]$ItemsPerAgent = 0,
    [int]$MaxParallel = 0,
    [string]$ReviewOutputDir = '',
    [string]$PacketPath = '',
    [string]$DiffPath = ''
  )
  $planRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $routeItem = Get-RekitSubagentRoute -Manifest $manifest -Route $Route -TaskType $TaskType
  $routeItemsPerAgent = 4
  if (-not [string]::IsNullOrWhiteSpace([string]$routeItem.targetItemsPerAgent)) { $routeItemsPerAgent = [int]$routeItem.targetItemsPerAgent }
  if ($ItemsPerAgent -gt 0) { $routeItemsPerAgent = $ItemsPerAgent }
  $routeMaxParallel = 3
  if (-not [string]::IsNullOrWhiteSpace([string]$routeItem.maxParallel)) { $routeMaxParallel = [int]$routeItem.maxParallel }
  if ($MaxParallel -gt 0) { $routeMaxParallel = $MaxParallel }

  $planItems = @(Split-RekitPlanItems -Items $Items -ItemsFile $ItemsFile)
  $shards = @(New-RekitPlanShards -Items $planItems -TargetItemsPerAgent $routeItemsPerAgent)
  $paths = Get-RekitReviewPaths -Command 'plan-subagents' -CaseRoot $planRoot -ReviewOutputDir $ReviewOutputDir -PacketPath $PacketPath -DiffPath $DiffPath
  if (Test-Path -LiteralPath $paths.CombinedDiffPath) { Remove-Item -LiteralPath $paths.CombinedDiffPath -Force }

  $packet = [ordered]@{
    schemaVersion = 1
    command = 'plan-subagents'
    isMutation = $false
    writesReviewArtifacts = $true
    repoRoot = [System.IO.Path]::GetFullPath($RepoRoot)
    pack = $Pack
    manifestPath = $manifest.ManifestPath
    route = $routeItem
    input = [ordered]@{ taskType = $TaskType; itemCount = $planItems.Count; itemsFile = $ItemsFile }
    shardPolicy = [ordered]@{ basis = [string]$routeItem.shardBasis; targetItemsPerAgent = $routeItemsPerAgent; maxParallel = $routeMaxParallel }
    shards = $shards
    mainAgentResponsibilities = ([string]$routeItem.mainAgentOwns)
    subagentPermissions = ([string]$routeItem.subagentPermissions)
    outputContract = ([string]$routeItem.outputContract)
    reviewRequired = $true
  }

  $summary = @(
    '# rekit subagent plan',
    '',
    ('- route: `' + [string]$routeItem.id + '`'),
    ('- task type: `' + $TaskType + '`'),
    ('- items: `' + $planItems.Count + '`'),
    ('- shards: `' + $shards.Count + '`'),
    ('- target items per agent: `' + $routeItemsPerAgent + '`'),
    ('- max parallel: `' + $routeMaxParallel + '`'),
    '- writes review artifacts: `true`',
    '',
    'Use the generated packet to launch read-only subagents. The command only writes review artifacts; the main agent owns project writes, validation, and handoff updates.'
  )
  Write-RekitReviewPacket -Paths $paths -Packet $packet -SummaryLines $summary
}

function Write-RekitSyncReview {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string]$ProjectName = '',
    [switch]$CreateLocalFiles,
    [string]$ReviewOutputDir = '',
    [string]$PacketPath = '',
    [string]$DiffPath = ''
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $inst = Get-RekitInstance -Target $caseRoot
  if ([string]::IsNullOrWhiteSpace($ProjectName) -and $inst.Source -ne 'missing') { $ProjectName = $inst.ProjectName }
  if ([string]::IsNullOrWhiteSpace($ProjectName)) { $ProjectName = Get-RekitProjectName $caseRoot }
  $paths = Get-RekitReviewPaths -Command 'sync' -CaseRoot $caseRoot -ReviewOutputDir $ReviewOutputDir -PacketPath $PacketPath -DiffPath $DiffPath
  if (Test-Path -LiteralPath $paths.CombinedDiffPath) { Remove-Item -LiteralPath $paths.CombinedDiffPath -Force }

  $items = @()
  $backupPreview = Join-Path $caseRoot 'references\vmp-re\.backup\<timestamp>'
  $items += [ordered]@{
    path = '.rekit/instance.yml + .claude/skills/rekit/SKILL.md'
    kind = 'case-metadata'
    direction = 'kit-to-case'
    action = 'refresh-metadata-and-shim'
    riskLevel = 'low'
    sourceHash = ''
    targetHash = ''
    reason = 'sync refreshes attached-case metadata before managed file updates'
  }

  foreach ($rel in $manifest.ManagedFiles) {
    $source = Get-RekitSourcePath -Manifest $manifest -RelativePath $rel
    $dest = Join-RekitPath -Root $caseRoot -RelativePath $rel
    $sourceText = Read-RekitTextIfExists $source
    $destText = Read-RekitTextIfExists $dest
    $sourceHash = Get-RekitFileHash $source
    $targetHash = Get-RekitFileHash $dest
    $stateEntry = Get-RekitStateManagedEntry -CaseRoot $caseRoot -RelativePath $rel
    $lastSyncHash = if ($null -ne $stateEntry) { $stateEntry.targetHashAtSync } else { '' }
    $changedSinceLastSync = if ([string]::IsNullOrWhiteSpace($lastSyncHash)) { $null } else { -not [string]::Equals($targetHash, $lastSyncHash, [System.StringComparison]::OrdinalIgnoreCase) }
    $action = if (-not (Test-Path -LiteralPath $dest)) { 'create-managed-file' } elseif ($sourceHash -eq $targetHash) { 'unchanged' } else { 'overwrite-with-backup' }
    $risk = if ($action -eq 'unchanged') { 'none' } elseif ($changedSinceLastSync -eq $true) { 'high' } else { 'medium' }
    $diffFile = ''
    if ($action -ne 'unchanged') {
      $diffFile = Write-RekitDiffFile -DiffRoot $paths.DiffRoot -RelativePath $rel -OldLabel ('case:' + $rel) -OldText $destText -NewLabel ('pack:' + $rel) -NewText $sourceText -CombinedDiffPath $paths.CombinedDiffPath
    }
    $items += [ordered]@{
      path = $rel
      kind = 'managed-file'
      direction = 'kit-to-case'
      sourcePath = $source
      targetPath = $dest
      sourceExists = (Test-Path -LiteralPath $source)
      targetExists = (Test-Path -LiteralPath $dest)
      sourceHash = $sourceHash
      targetHash = $targetHash
      lastSyncHash = $lastSyncHash
      changedSinceLastSync = $changedSinceLastSync
      sourceBytes = Get-RekitFileBytesIfExists $source
      targetBytes = Get-RekitFileBytesIfExists $dest
      action = $action
      riskLevel = $risk
      backupPreview = if ($action -eq 'overwrite-with-backup') { Join-Path $backupPreview $rel } else { '' }
      diffPath = $diffFile
    }
  }

  foreach ($rel in $manifest.TemplateFiles) {
    $source = Get-RekitSourcePath -Manifest $manifest -RelativePath $rel
    $targetRel = $rel -replace '\.template\.md$', '.md'
    $dest = Join-RekitPath -Root $caseRoot -RelativePath $targetRel
    $sourceText = (Read-RekitTextIfExists $source).Replace('<PROJECT_NAME>', $ProjectName).Replace('<PROJECT_ROOT>', $caseRoot)
    $destText = Read-RekitTextIfExists $dest
    $action = if ((Test-Path -LiteralPath $dest) -and -not $CreateLocalFiles) { 'skip-existing-local-file' } elseif ($sourceText -eq $destText) { 'unchanged' } else { 'create-local-template-file' }
    $diffFile = ''
    if ($action -eq 'create-local-template-file') {
      $diffFile = Write-RekitDiffFile -DiffRoot $paths.DiffRoot -RelativePath $targetRel -OldLabel ('case:' + $targetRel) -OldText $destText -NewLabel ('template:' + $rel) -NewText $sourceText -CombinedDiffPath $paths.CombinedDiffPath
    }
    $items += [ordered]@{
      path = $targetRel
      kind = 'template-file'
      direction = 'kit-to-case'
      sourcePath = $source
      targetPath = $dest
      sourceHash = Get-RekitFileHash $source
      targetHash = Get-RekitFileHash $dest
      action = $action
      riskLevel = if ($action -eq 'skip-existing-local-file') { 'none' } else { 'low' }
      diffPath = $diffFile
    }
  }

  $blockSource = Join-RekitPath -Root $manifest.PackRoot -RelativePath $manifest.ManagedBlock['source']
  $blockHost = Join-RekitPath -Root $caseRoot -RelativePath $manifest.ManagedBlock['file']
  $hostText = Read-RekitTextIfExists $blockHost
  $blockText = Read-RekitTextIfExists $blockSource
  $nextHostText = Get-RekitManagedBlockAppliedText -HostText $hostText -BlockId $manifest.ManagedBlock['blockId'] -BlockText $blockText
  $blockAction = if ($hostText -eq $nextHostText) { 'unchanged' } elseif ([string]::IsNullOrWhiteSpace($hostText)) { 'create-managed-block-host' } elseif ($hostText -match ('<!-- BEGIN ' + [regex]::Escape($manifest.ManagedBlock['blockId']))) { 'replace-managed-block' } else { 'append-managed-block' }
  $blockDiff = ''
  if ($blockAction -ne 'unchanged') {
    $blockDiff = Write-RekitDiffFile -DiffRoot $paths.DiffRoot -RelativePath ($manifest.ManagedBlock['file'] + '#managed-block') -OldLabel ('case:' + $manifest.ManagedBlock['file']) -OldText $hostText -NewLabel ('pack-block:' + $manifest.ManagedBlock['source']) -NewText $nextHostText -CombinedDiffPath $paths.CombinedDiffPath
  }
  $items += [ordered]@{
    path = $manifest.ManagedBlock['file']
    kind = 'managed-block'
    blockId = $manifest.ManagedBlock['blockId']
    direction = 'kit-to-case'
    sourcePath = $blockSource
    targetPath = $blockHost
    sourceHash = Get-RekitFileHash $blockSource
    targetHash = Get-RekitFileHash $blockHost
    action = $blockAction
    riskLevel = if ($blockAction -eq 'unchanged') { 'none' } else { 'medium' }
    backupPreview = if ($blockAction -ne 'unchanged') { Join-Path $backupPreview $manifest.ManagedBlock['file'] } else { '' }
    diffPath = $blockDiff
  }

  $gitignoreSource = Join-RekitPath -Root $manifest.PackRoot -RelativePath 'examples/gitignore.example'
  $gitignoreTarget = Join-Path $caseRoot '.gitignore'
  if (Test-Path -LiteralPath $gitignoreSource) {
    $items += [ordered]@{
      path = '.gitignore'
      kind = 'support-file'
      direction = 'kit-to-case'
      sourcePath = $gitignoreSource
      targetPath = $gitignoreTarget
      sourceHash = Get-RekitFileHash $gitignoreSource
      targetHash = Get-RekitFileHash $gitignoreTarget
      action = if (Test-Path -LiteralPath $gitignoreTarget) { 'skip-existing-support-file' } else { 'create-support-file' }
      riskLevel = 'low'
      diffPath = ''
    }
  }

  $changedCount = @($items | Where-Object { $_['action'] -notin @('unchanged','skip-existing-local-file','skip-existing-support-file') }).Count
  $packet = [ordered]@{
    schemaVersion = 1
    command = 'sync'
    direction = 'kit-to-case'
    caseRoot = $caseRoot
    repoRoot = [System.IO.Path]::GetFullPath($RepoRoot)
    pack = $Pack
    manifestPath = $manifest.ManifestPath
    manifestVersion = $manifest.Version
    reviewRoot = $paths.Root
    createdAt = (Get-Date -Format 'yyyy-MM-ddTHH:mm:ssK')
    isMutation = $false
    summary = [ordered]@{ total = $items.Count; changed = $changedCount; reviewRequired = $true }
    items = $items
  }
  $summary = @(
    '# rekit sync review',
    '',
    "- direction: kit -> case",
    "- case: $caseRoot",
    "- pack: $Pack $($manifest.Version)",
    "- changed/planned items: $changedCount / $($items.Count)",
    '',
    'Claude should compare the packet and diffs, explain benefits/conflicts, then ask the user before running a write action.'
  )
  Write-RekitReviewPacket -Paths $paths -Packet $packet -SummaryLines $summary
}

function Write-RekitPromoteReview {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string]$ReviewOutputDir = '',
    [string]$PacketPath = '',
    [string]$DiffPath = ''
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $paths = Get-RekitReviewPaths -Command 'promote' -CaseRoot $caseRoot -ReviewOutputDir $ReviewOutputDir -PacketPath $PacketPath -DiffPath $DiffPath
  if (Test-Path -LiteralPath $paths.CombinedDiffPath) { Remove-Item -LiteralPath $paths.CombinedDiffPath -Force }

  $denyPatterns = @($manifest.PromoteDenyPatterns) + @(Get-RekitCaseSpecificPatterns -CaseRoot $caseRoot)
  $items = @()
  foreach ($rel in $manifest.PromoteFiles) {
    if ($manifest.ManagedFiles -notcontains $rel) {
      $items += [ordered]@{ path = $rel; kind = 'managed-doc'; direction = 'case-to-kit'; action = 'skip-non-managed-promote-file'; riskLevel = 'none' }
      continue
    }
    $caseFile = Join-RekitPath -Root $caseRoot -RelativePath $rel
    $packFile = Get-RekitSourcePath -Manifest $manifest -RelativePath $rel
    $caseExists = Test-Path -LiteralPath $caseFile
    $packExists = Test-Path -LiteralPath $packFile
    $caseText = Read-RekitTextIfExists $caseFile
    $packText = Read-RekitTextIfExists $packFile
    $violations = if ($caseExists) { @(Test-RekitPromoteContent -Text $caseText -Patterns $denyPatterns) } else { @() }
    $changed = ($caseText -ne $packText)
    $action = if (-not $caseExists) { 'skip-missing-case-file' } elseif (-not $packExists) { 'blocked-missing-pack-file' } elseif (-not $changed) { 'unchanged' } elseif ($violations.Count -gt 0) { 'blocked-deny-pattern' } else { 'candidate-after-llm-review' }
    $risk = if ($action -eq 'unchanged') { 'none' } elseif ($violations.Count -gt 0) { 'high' } elseif ($changed) { 'medium' } else { 'low' }
    $diffFile = ''
    if ($changed -and $violations.Count -eq 0 -and $caseExists -and $packExists) {
      $diffFile = Write-RekitDiffFile -DiffRoot $paths.DiffRoot -RelativePath $rel -OldLabel ('pack:' + $rel) -OldText $packText -NewLabel ('case:' + $rel) -NewText $caseText -CombinedDiffPath $paths.CombinedDiffPath
    }
    $items += [ordered]@{
      path = $rel
      kind = 'managed-doc'
      direction = 'case-to-kit'
      casePath = $caseFile
      packPath = $packFile
      caseExists = $caseExists
      packExists = $packExists
      caseHash = Get-RekitFileHash $caseFile
      packHash = Get-RekitFileHash $packFile
      caseBytes = Get-RekitFileBytesIfExists $caseFile
      packBytes = Get-RekitFileBytesIfExists $packFile
      changed = $changed
      denyViolations = $violations
      action = $action
      riskLevel = $risk
      mechanicalRecommendation = if ($violations.Count -gt 0) { 'do-not-apply-directly' } elseif ($changed) { 'llm-review-before-merge' } else { 'skip' }
      reason = if ($violations.Count -gt 0) { 'deny-pattern-hit; raw diff intentionally omitted' } else { '' }
      diffPath = $diffFile
    }
  }

  $tooling = @()
  foreach ($rel in $manifest.ToolingCandidateSources) {
    $source = Join-RekitPath -Root $caseRoot -RelativePath $rel
    if (-not (Test-Path -LiteralPath $source)) {
      $tooling += [ordered]@{ path = $rel; kind = 'tooling-candidate-source'; action = 'skip-missing-source'; riskLevel = 'none' }
      continue
    }
    $raw = [System.IO.File]::ReadAllText($source, [System.Text.Encoding]::UTF8)
    $converted = Convert-RekitToolingCandidateTextWithStats -Text $raw -CaseRoot $caseRoot
    $remaining = @(Test-RekitPromoteContent -Text $converted.Text -Patterns $denyPatterns)
    $previewPath = ''
    if ($remaining.Count -eq 0) {
      $previewName = (ConvertTo-RekitSafeFileName $rel) + '.sanitized-preview.md'
      $previewPath = Join-Path $paths.PreviewRoot $previewName
      [System.IO.File]::WriteAllText($previewPath, (New-RekitToolingCandidateHeader -RelativePath $rel) + $converted.Text, [System.Text.UTF8Encoding]::new($false))
    }
    $tooling += [ordered]@{
      path = $rel
      kind = 'tooling-candidate-source'
      direction = 'case-to-kit'
      sourcePath = $source
      sourceHash = Get-RekitFileHash $source
      sourceBytes = Get-RekitFileBytesIfExists $source
      sanitizedPreviewPath = $previewPath
      replacementCounts = $converted.ReplacementCounts
      remainingDenyViolations = $remaining
      action = if ($remaining.Count -gt 0) { 'blocked-after-sanitization' } else { 'sanitized-preview-for-llm-review' }
      riskLevel = if ($remaining.Count -gt 0) { 'high' } else { 'medium' }
      mechanicalRecommendation = if ($remaining.Count -gt 0) { 'do-not-create-candidate' } else { 'llm-review-before-merge' }
    }
  }

  $blocked = @($items | Where-Object { $_['action'] -like 'blocked*' }).Count
  $changedCount = @($items | Where-Object { $_['changed'] -eq $true }).Count
  $packet = [ordered]@{
    schemaVersion = 1
    command = 'promote'
    direction = 'case-to-kit'
    caseRoot = $caseRoot
    repoRoot = [System.IO.Path]::GetFullPath($RepoRoot)
    pack = $Pack
    manifestPath = $manifest.ManifestPath
    manifestVersion = $manifest.Version
    reviewRoot = $paths.Root
    createdAt = (Get-Date -Format 'yyyy-MM-ddTHH:mm:ssK')
    isMutation = $false
    summary = [ordered]@{ managedDocs = $items.Count; changedManagedDocs = $changedCount; blockedManagedDocs = $blocked; toolingSources = $tooling.Count; reviewRequired = $true }
    items = $items
    toolingCandidateSources = $tooling
  }
  $summary = @(
    '# rekit promote review',
    '',
    "- direction: case -> kit",
    "- case: $caseRoot",
    "- pack: $Pack $($manifest.Version)",
    "- changed managed docs: $changedCount",
    "- blocked managed docs: $blocked",
    "- tooling sources: $($tooling.Count)",
    '',
    'Claude should judge conflicts and relative value, then ask the user before creating candidates or editing pack files.'
  )
  Write-RekitReviewPacket -Paths $paths -Packet $packet -SummaryLines $summary
}
