function Get-RekitBudgetLimit {
  param(
    [Parameter(Mandatory=$true)]$Manifest,
    [Parameter(Mandatory=$true)][string]$RelativePath
  )
  if ($Manifest.Budgets.ContainsKey($RelativePath)) { return [int]$Manifest.Budgets[$RelativePath] }
  return [int]$Manifest.Budgets['defaultMarkdown']
}

function Assert-RekitTextFile {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [int]$LimitBytes = 16384
  )
  if (-not (Test-Path -LiteralPath $Path)) { throw "missing file: $Path" }
  $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
  if ([string]::IsNullOrWhiteSpace($text)) { throw "empty file: $Path" }
  $size = (Get-Item -LiteralPath $Path).Length
  if ($size -gt $LimitBytes) { throw "file too large: $Path $size > $LimitBytes" }
  return [pscustomobject]@{ File = $Path; Bytes = $size; Limit = $LimitBytes }
}

function Test-RekitManagedBlock {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$BlockId
  )
  if (-not (Test-Path -LiteralPath $Path)) { throw "missing managed block host: $Path" }
  $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
  if ($text -notmatch ('<!-- BEGIN ' + [regex]::Escape($BlockId))) { throw "missing managed block begin marker in ${Path}: $BlockId" }
  if ($text -notmatch ('<!-- END ' + [regex]::Escape($BlockId) + ' -->')) { throw "missing managed block end marker in ${Path}: $BlockId" }
}

function Assert-RekitManifestMapValue {
  param(
    [Parameter(Mandatory=$true)][hashtable]$Map,
    [Parameter(Mandatory=$true)][string]$Key,
    [Parameter(Mandatory=$true)][string]$Context
  )
  if (-not $Map.ContainsKey($Key) -or [string]::IsNullOrWhiteSpace([string]$Map[$Key])) {
    throw "$Context is missing required key: $Key"
  }
}

function Assert-RekitSupportedSyncPolicy {
  param(
    [Parameter(Mandatory=$true)][hashtable]$Policy,
    [Parameter(Mandatory=$true)][string]$Key,
    [Parameter(Mandatory=$true)][string[]]$Allowed
  )
  if (-not $Policy.ContainsKey($Key)) { throw "syncPolicy is missing required key: $Key" }
  $value = [string]$Policy[$Key]
  if ($Allowed -notcontains $value) { throw "syncPolicy.$Key has unsupported value: $value" }
}

function Test-RekitManifestSchema {
  param([Parameter(Mandatory=$true)]$Manifest)
  $lines = [System.IO.File]::ReadAllLines($Manifest.ManifestPath, [System.Text.Encoding]::UTF8)
  $explicitManagedBlock = Get-RekitYamlMap -Lines $lines -Key 'managedBlock'
  $explicitToolingCandidateSources = @(Get-RekitYamlList -Lines $lines -Key 'toolingCandidateSources')

  foreach ($rel in $Manifest.PromoteFiles) {
    if ($Manifest.ManagedFiles -notcontains $rel) { throw "promoteFiles entry is not managed: $rel" }
  }
  foreach ($rel in $Manifest.LocalFiles) {
    if ($Manifest.ManagedFiles -contains $rel) { throw "localNeverOverwrite entry also appears in managedFiles: $rel" }
    if ($Manifest.TemplateFiles -contains $rel) { throw "localNeverOverwrite entry also appears in templateFiles: $rel" }
  }
  $managedTargets = @($Manifest.ManagedFiles) + @($Manifest.TemplateFiles | ForEach-Object { ([string]$_) -replace '\.template\.md$', '.md' }) + @([string]$Manifest.ManagedBlock['file']) + @($Manifest.LocalFiles)

  Assert-RekitManifestMapValue -Map $explicitManagedBlock -Key 'file' -Context 'managedBlock'
  Assert-RekitManifestMapValue -Map $explicitManagedBlock -Key 'blockId' -Context 'managedBlock'
  Assert-RekitManifestMapValue -Map $explicitManagedBlock -Key 'source' -Context 'managedBlock'
  [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath $Manifest.ManagedBlock['file'])
  [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath $Manifest.ManagedBlock['source'])

  if ($explicitToolingCandidateSources.Count -eq 0) { throw 'manifest must explicitly declare toolingCandidateSources; implicit vmp-re fallback is not allowed' }
  foreach ($rel in $Manifest.ToolingCandidateSources) { [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath $rel) }

  Assert-RekitManifestMapValue -Map $Manifest.WorkstreamDefaults -Key 'defaultAuthorityLane' -Context 'workstreamDefaults'
  Assert-RekitManifestMapValue -Map $Manifest.WorkstreamDefaults -Key 'defaultStartLaneType' -Context 'workstreamDefaults'
  Assert-RekitManifestMapValue -Map $Manifest.WorkstreamDefaults -Key 'backupRoot' -Context 'workstreamDefaults'
  Assert-RekitManifestMapValue -Map $Manifest.WorkstreamDefaults -Key 'requestDefaultTargetLane' -Context 'workstreamDefaults'
  [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath ([string]$Manifest.WorkstreamDefaults['backupRoot']))
  if ($Manifest.WorkstreamDefaults.ContainsKey('handoffPath') -and -not [string]::IsNullOrWhiteSpace([string]$Manifest.WorkstreamDefaults['handoffPath'])) {
    [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath ([string]$Manifest.WorkstreamDefaults['handoffPath']))
    if ($managedTargets -notcontains ([string]$Manifest.WorkstreamDefaults['handoffPath'])) { throw "workstreamDefaults.handoffPath is not a managed, template, managed-block, or local file: $($Manifest.WorkstreamDefaults['handoffPath'])" }
  }
  if ($Manifest.AuthorityFiles.Count -eq 0) { throw 'manifest must explicitly declare authorityFiles, even if the list is intentionally minimal' }
  $laneTypeIds = @(@(Get-RekitLaneTypes -Manifest $Manifest) | ForEach-Object { [string]$_.Id })
  $authorityLane = Get-RekitLaneType -Manifest $Manifest -Type ([string]$Manifest.WorkstreamDefaults['defaultAuthorityLane'])
  if (-not $authorityLane.Authority) { throw "workstreamDefaults.defaultAuthorityLane must reference an authority lane: $($Manifest.WorkstreamDefaults['defaultAuthorityLane'])" }
  [void](Get-RekitLaneType -Manifest $Manifest -Type ([string]$Manifest.WorkstreamDefaults['defaultStartLaneType']))
  [void](Get-RekitLaneType -Manifest $Manifest -Type ([string]$Manifest.WorkstreamDefaults['requestDefaultTargetLane']))
  foreach ($rel in $Manifest.AuthorityFiles) {
    [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath $rel)
    if (@($authorityLane.CanWrite) -notcontains ([string]$rel)) { throw "authorityFiles entry is not writable by default authority lane $($authorityLane.Id): $rel" }
  }

  Assert-RekitSupportedSyncPolicy -Policy $Manifest.SyncPolicy -Key 'managedFiles' -Allowed @('overwrite-with-backup')
  Assert-RekitSupportedSyncPolicy -Policy $Manifest.SyncPolicy -Key 'templateFiles' -Allowed @('create-if-missing')
  Assert-RekitSupportedSyncPolicy -Policy $Manifest.SyncPolicy -Key 'localFiles' -Allowed @('never-overwrite')

  foreach ($pattern in $Manifest.PromoteDenyPatterns) {
    if ([string]::IsNullOrWhiteSpace($pattern)) { throw 'promoteDenyPatterns contains an empty pattern' }
    try { [void][regex]::new([string]$pattern) } catch { throw "invalid promoteDenyPatterns regex '$pattern': $_" }
  }

  if (-not [string]::Equals([string]$Manifest.Pack, 'vmp-re', [System.StringComparison]::OrdinalIgnoreCase)) {
    $routePaths = @()
    foreach ($route in @($Manifest.SubagentRoutes)) {
      if (-not [string]::IsNullOrWhiteSpace([string]$route.reference)) { $routePaths += [string]$route.reference }
      if (-not [string]::IsNullOrWhiteSpace([string]$route.policyOverlay)) { $routePaths += [string]$route.policyOverlay }
    }
    $allDeclaredPaths = @($Manifest.ManagedFiles) + @($Manifest.TemplateFiles) + @($Manifest.LocalFiles) + @($Manifest.PromoteFiles) + @($Manifest.ToolingFiles) + @($Manifest.ToolingCandidateSources) + @($Manifest.PromptFiles) + @($Manifest.PolicyOverlays) + @($routePaths) + @([string]$Manifest.ManagedBlock['file'], [string]$Manifest.ManagedBlock['source'])
    foreach ($rel in $allDeclaredPaths) {
      if (([string]$rel) -match '(^|[\\/])vmp-re([\\/]|$)') { throw "non-vmp pack declares vmp-re path: $rel" }
    }
    foreach ($rel in @($Manifest.AuthorityFiles)) {
      if (([string]$rel) -match '(^|[\\/])vmp-re([\\/]|$)') { throw "non-vmp pack declares vmp-re authority path: $rel" }
    }
    foreach ($key in @('handoffPath','backupRoot')) {
      if (([string]$Manifest.WorkstreamDefaults[$key]) -match '(^|[\\/])vmp-re([\\/]|$)') { throw "non-vmp pack declares vmp-re workstream default: $key=$($Manifest.WorkstreamDefaults[$key])" }
    }
  }
  foreach ($lane in @(Get-RekitLaneTypes -Manifest $Manifest)) {
    foreach ($rel in @($lane.CanWrite) + @($lane.ReadOnly)) {
      if ([string]$rel -eq 'own-workspace') { continue }
      if (-not [string]::IsNullOrWhiteSpace([string]$rel)) { Assert-RekitCaseRelativePattern -Value ([string]$rel) -Context "laneTypes.$($lane.Id) boundary" }
    }
  }
  return @()
}

function Test-RekitCaseShimMatchesTemplate {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$TemplateRoot
  )
  $shim = Join-Path $CaseRoot '.claude\skills\rekit\SKILL.md'
  $template = Join-Path $TemplateRoot 'rekit\templates\case-shim\SKILL.md'
  $shimText = [System.IO.File]::ReadAllText($shim, [System.Text.Encoding]::UTF8)
  $templateText = [System.IO.File]::ReadAllText($template, [System.Text.Encoding]::UTF8)
  if ($shimText -ne $templateText) { throw "case-local /rekit shim differs from canonical thin shim template: $shim" }
}

function Assert-RekitJsonFileValid {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return }
  try {
    $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
    if (-not [string]::IsNullOrWhiteSpace($text)) { [void]($text | ConvertFrom-Json) }
  } catch {
    throw "malformed json file: $Path :: $_"
  }
}

function Assert-RekitJsonLinesValid {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return }
  $lineNo = 0
  foreach ($line in [System.IO.File]::ReadLines($Path, [System.Text.Encoding]::UTF8)) {
    $lineNo++
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    try { [void]($line | ConvertFrom-Json) } catch { throw "malformed jsonl line in ${Path}:$lineNo :: $_" }
  }
}

function Test-RekitWorkstreamState {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  foreach ($file in @($paths.Observations,$paths.Candidates,$paths.Requests,$paths.Publications,$paths.Decisions)) { Assert-RekitJsonLinesValid -Path $file }
  if (Test-Path -LiteralPath $paths.Board) {
    Assert-RekitJsonFileValid -Path $paths.Board
    $board = Read-RekitJsonFile -Path $paths.Board
    if ($null -ne $board) {
      if ($board.PSObject.Properties['caseRoot']) {
        $actual = [System.IO.Path]::GetFullPath($CaseRoot).TrimEnd('\')
        $recorded = [System.IO.Path]::GetFullPath([string]$board.caseRoot).TrimEnd('\')
        if (-not [string]::Equals($actual, $recorded, [System.StringComparison]::OrdinalIgnoreCase)) { throw "board caseRoot mismatch: $($board.caseRoot)" }
      }
    }
  }
  foreach ($dir in (Get-RekitLaneDirectories -CaseRoot $CaseRoot)) {
    $laneFile = Join-Path $dir.FullName 'lane.json'
    Assert-RekitJsonFileValid -Path $laneFile
    $lane = Read-RekitJsonFile -Path $laneFile
    if ($null -eq $lane) { throw "empty lane file: $laneFile" }
    if ([string]::IsNullOrWhiteSpace([string]$lane.id)) { throw "lane is missing id: $laneFile" }
    if (-not [string]::Equals([string]$lane.id, $dir.Name, [System.StringComparison]::OrdinalIgnoreCase)) { throw "lane id does not match directory: $laneFile" }
    if ([string]::IsNullOrWhiteSpace([string]$lane.workspace)) { throw "lane is missing workspace: $laneFile" }
    [void](Join-RekitPath -Root $CaseRoot -RelativePath ([string]$lane.workspace))
    if ($lane.PSObject.Properties['laneRoot']) { [void](Join-RekitPath -Root $CaseRoot -RelativePath ([string]$lane.laneRoot)) }
    foreach ($file in @('events.jsonl','tasks.jsonl','inbox.jsonl','outbox.jsonl')) { Assert-RekitJsonLinesValid -Path (Join-Path $dir.FullName $file) }
    $workspace = Join-RekitPath -Root $CaseRoot -RelativePath ([string]$lane.workspace)
    foreach ($file in @('observations.jsonl','requests.jsonl','candidates.jsonl','publications.jsonl')) { Assert-RekitJsonLinesValid -Path (Join-Path $workspace $file) }
  }
  return @()
}

function Get-RekitPolicyManifestPaths {
  param(
    [Parameter(Mandatory=$true)][string]$ManifestPath,
    [Parameter(Mandatory=$true)][string]$ListKey,
    [Parameter(Mandatory=$true)][string]$PathKey
  )
  $lines = [System.IO.File]::ReadAllLines($ManifestPath, [System.Text.Encoding]::UTF8)
  $paths = @()
  $inside = $false
  foreach ($line in $lines) {
    if (-not $inside) {
      if ($line -match ('^' + [regex]::Escape($ListKey) + '\s*:\s*$')) { $inside = $true }
      continue
    }
    if ($line -match '^\S') { break }
    if ($line -match ('^\s{4}' + [regex]::Escape($PathKey) + '\s*:\s*(.+?)\s*$')) {
      $paths += (Convert-RekitYamlValue $Matches[1])
    }
  }
  return $paths
}

function Get-RekitPolicyManifestEntries {
  param(
    [Parameter(Mandatory=$true)][string]$ManifestPath,
    [Parameter(Mandatory=$true)][string]$ListKey
  )
  $lines = [System.IO.File]::ReadAllLines($ManifestPath, [System.Text.Encoding]::UTF8)
  return @(Get-RekitYamlObjectList -Lines $lines -Key $ListKey)
}

function Test-RekitPolicyRegistry {
  param([Parameter(Mandatory=$true)]$Manifest)
  $rows = @()
  $commonManifest = Join-RekitPath -Root $Manifest.RepoRoot -RelativePath 'common/policies/manifest.yml'
  $rows += Assert-RekitTextFile -Path $commonManifest -LimitBytes 16384
  $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $Manifest.RepoRoot -RelativePath 'common/policies/README.md') -LimitBytes 16384
  $commonEntries = @(Get-RekitPolicyManifestEntries -ManifestPath $commonManifest -ListKey 'policies')
  $commonIds = @($commonEntries | ForEach-Object { [string]$_.id })
  foreach ($rel in (Get-RekitPolicyManifestPaths -ManifestPath $commonManifest -ListKey 'policies' -PathKey 'path')) {
    $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root (Split-Path -Parent $commonManifest) -RelativePath $rel) -LimitBytes 16384
  }
  foreach ($id in $Manifest.CommonPolicies) {
    if ($commonIds -notcontains $id) { throw "manifest common policy is not registered: $id" }
  }

  $overlayManifest = Join-RekitPath -Root $Manifest.PackRoot -RelativePath 'policies/manifest.yml'
  $rows += Assert-RekitTextFile -Path $overlayManifest -LimitBytes 16384
  $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $Manifest.PackRoot -RelativePath 'policies/README.md') -LimitBytes 16384
  $overlayEntries = @(Get-RekitPolicyManifestEntries -ManifestPath $overlayManifest -ListKey 'overlays')
  $overlayPaths = @($overlayEntries | ForEach-Object { [string]$_.path })
  foreach ($rel in (Get-RekitPolicyManifestPaths -ManifestPath $overlayManifest -ListKey 'overlays' -PathKey 'path')) {
    $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root (Split-Path -Parent $overlayManifest) -RelativePath $rel) -LimitBytes 16384
  }
  foreach ($entry in $overlayEntries) {
    if (-not [string]::IsNullOrWhiteSpace([string]$entry.extends) -and $commonIds -notcontains ([string]$entry.extends)) {
      throw "policy overlay extends unknown common policy: $($entry.id) -> $($entry.extends)"
    }
  }
  foreach ($rel in $Manifest.PolicyOverlays) {
    $normalized = ([string]$rel) -replace '^policies[\/]', ''
    if ($overlayPaths -notcontains $normalized) { throw "manifest policy overlay is not registered: $rel" }
    $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $Manifest.PackRoot -RelativePath $rel) -LimitBytes 16384
  }
  return $rows
}

function Test-RekitSubagentRoutesPack {
  param([Parameter(Mandatory=$true)]$Manifest)
  $rows = @()
  $routes = @($Manifest.SubagentRoutes)
  if ($routes.Count -eq 0) {
    Write-Warning "manifest has no subagentRoutes: $($Manifest.ManifestPath)"
    return $rows
  }
  foreach ($route in $routes) {
    if ([string]::IsNullOrWhiteSpace([string]$route.id)) { throw "subagent route is missing id in $($Manifest.ManifestPath)" }
    if ([string]::IsNullOrWhiteSpace([string]$route.taskTypes)) { Write-Warning "subagent route $($route.id) has no taskTypes" }
    if ([string]::IsNullOrWhiteSpace([string]$route.outputContract)) { Write-Warning "subagent route $($route.id) has no outputContract" }
    if (-not [string]::IsNullOrWhiteSpace([string]$route.reference)) {
      $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $Manifest.PackRoot -RelativePath ([string]$route.reference)) -LimitBytes 16384
    } else {
      Write-Warning "subagent route $($route.id) has no reference document"
    }
    if (-not [string]::IsNullOrWhiteSpace([string]$route.policyOverlay)) {
      $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $Manifest.PackRoot -RelativePath ([string]$route.policyOverlay)) -LimitBytes 16384
    }
  }
  return $rows
}

function Test-RekitSubagentRoutesInstance {
  param(
    [Parameter(Mandatory=$true)]$Manifest,
    [Parameter(Mandatory=$true)][string]$CaseRoot
  )
  foreach ($route in @($Manifest.SubagentRoutes)) {
    if (-not [string]::IsNullOrWhiteSpace([string]$route.reference)) {
      $target = Join-RekitPath -Root $CaseRoot -RelativePath ([string]$route.reference)
      if (Test-Path -LiteralPath $target) {
        $text = [System.IO.File]::ReadAllText($target, [System.Text.Encoding]::UTF8)
        if ($text -notmatch 'agent|分片|bounded') { Write-Warning "route reference lacks visible subagent guidance: $target" }
      } else {
        Write-Warning "route reference is not present in case: $target"
      }
    }
  }
}

function Split-RekitManifestScalarList {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) { return @() }
  return @($Value -split '[,;]' | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
}

function Assert-RekitCaseRelativePattern {
  param(
    [Parameter(Mandatory=$true)][string]$Value,
    [Parameter(Mandatory=$true)][string]$Context
  )
  if ([string]::IsNullOrWhiteSpace($Value)) { throw "$Context is empty" }
  if ([string]$Value -eq 'own-workspace') { return }
  if ([System.IO.Path]::IsPathRooted($Value)) { throw "$Context must be relative: $Value" }
  $parts = @($Value -split '[\\/]')
  if ($parts -contains '..') { throw "$Context must not escape case root: $Value" }
}

function Test-RekitLaneTypesManifest {
  param([Parameter(Mandatory=$true)]$Manifest)
  $seen = @{}
  $hasAuthority = $false
  foreach ($lane in @($Manifest.LaneTypes)) {
    $id = [string]$lane.id
    if ([string]::IsNullOrWhiteSpace($id)) { throw "manifest laneTypes entry is missing id: $($Manifest.ManifestPath)" }
    if ($seen.ContainsKey($id)) { throw "duplicate laneTypes id: $id" }
    $seen[$id] = $true
    if ($id -notmatch '^[a-z0-9][a-z0-9._-]*$') { throw "laneTypes id must be lowercase slug: $id" }
    if ([string]::IsNullOrWhiteSpace([string]$lane.title)) { throw "laneTypes.$id title is empty" }
    $workspaceRoot = [string]$lane.workspaceRoot
    Assert-RekitCaseRelativePattern -Value $workspaceRoot -Context "laneTypes.$id.workspaceRoot"
    if (Convert-RekitYamlBool $lane.authority) { $hasAuthority = $true }
    foreach ($path in (Split-RekitManifestScalarList ([string]$lane.canWrite))) { Assert-RekitCaseRelativePattern -Value $path -Context "laneTypes.$id.canWrite" }
    foreach ($path in (Split-RekitManifestScalarList ([string]$lane.readOnly))) { Assert-RekitCaseRelativePattern -Value $path -Context "laneTypes.$id.readOnly" }
    $outputs = @(Split-RekitManifestScalarList ([string]$lane.outputs))
    if ($outputs.Count -eq 0) { throw "laneTypes.$id outputs is empty" }
  }
  if ($Manifest.LaneTypes.Count -gt 0 -and -not $hasAuthority) { throw "manifest laneTypes must include at least one authority workstream" }
  return @()
}

function Test-RekitPack {
  param(
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re'
  )
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $rows = @()
  $rows += Assert-RekitTextFile -Path $manifest.ManifestPath -LimitBytes 16384
  $canonicalSkill = Join-Path $RepoRoot '.claude\skills\rekit\SKILL.md'
  $rows += Assert-RekitTextFile -Path $canonicalSkill -LimitBytes 32768
  $caseShimTemplate = Join-Path $RepoRoot 'rekit\templates\case-shim\SKILL.md'
  $rows += Assert-RekitTextFile -Path $caseShimTemplate -LimitBytes 16384
  $rows += Test-RekitManifestSchema -Manifest $manifest
  $rows += Test-RekitPolicyRegistry -Manifest $manifest
  $rows += Test-RekitSubagentRoutesPack -Manifest $manifest

  foreach ($rel in $manifest.ManagedFiles) {
    $rows += Assert-RekitTextFile -Path (Get-RekitSourcePath -Manifest $manifest -RelativePath $rel) -LimitBytes (Get-RekitBudgetLimit -Manifest $manifest -RelativePath $rel)
  }
  foreach ($rel in $manifest.TemplateFiles) {
    $rows += Assert-RekitTextFile -Path (Get-RekitSourcePath -Manifest $manifest -RelativePath $rel) -LimitBytes (Get-RekitBudgetLimit -Manifest $manifest -RelativePath ($rel -replace '\.template\.md$', '.md'))
  }
  foreach ($rel in $manifest.ToolingFiles) {
    $rows += Assert-RekitTextFile -Path (Get-RekitSourcePath -Manifest $manifest -RelativePath $rel) -LimitBytes (Get-RekitBudgetLimit -Manifest $manifest -RelativePath $rel)
  }
  foreach ($rel in $manifest.PromptFiles) {
    $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $manifest.RepoRoot -RelativePath $rel) -LimitBytes 16384
  }
  $rows += Test-RekitLaneTypesManifest -Manifest $manifest
  foreach ($rel in $manifest.PromoteFiles) {
    [void](Join-RekitPath -Root $manifest.PackRoot -RelativePath $rel)
  }
  foreach ($rel in $manifest.LocalFiles) {
    [void](Join-RekitPath -Root $manifest.PackRoot -RelativePath $rel)
  }
  foreach ($rel in $manifest.ToolingCandidateSources) {
    [void](Join-RekitPath -Root $manifest.PackRoot -RelativePath $rel)
  }
  [void](Join-RekitPath -Root $manifest.PackRoot -RelativePath $manifest.ManagedBlock['file'])
  $blockSource = Join-RekitPath -Root $manifest.PackRoot -RelativePath $manifest.ManagedBlock['source']
  $rows += Assert-RekitTextFile -Path $blockSource -LimitBytes 8192
  return $rows
}

function Test-RekitInstance {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [string]$RepoRoot = '',
    [string]$Pack = ''
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $instance = Get-RekitInstance -Target $caseRoot
  if ([string]::IsNullOrWhiteSpace($instance.TemplateRoot)) { throw "missing templateRoot in .rekit/instance.yml or .re-template.yml: $caseRoot" }
  $templateRoot = [System.IO.Path]::GetFullPath($instance.TemplateRoot)
  if (-not [string]::IsNullOrWhiteSpace($RepoRoot)) { $templateRoot = [System.IO.Path]::GetFullPath($RepoRoot) }
  $packName = if ([string]::IsNullOrWhiteSpace($Pack)) { $instance.TemplatePack } else { $Pack }
  $manifest = Get-RekitPackManifest -RepoRoot $templateRoot -Pack $packName

  $rows = @()
  $instanceFile = Join-Path $caseRoot '.rekit\instance.yml'
  if (Test-Path -LiteralPath $instanceFile) { $rows += Assert-RekitTextFile -Path $instanceFile -LimitBytes 8192 }
  $legacyFile = Join-Path $caseRoot '.re-template.yml'
  if (Test-Path -LiteralPath $legacyFile) { $rows += Assert-RekitTextFile -Path $legacyFile -LimitBytes 8192 }

  $shim = Join-Path $caseRoot '.claude\skills\rekit\SKILL.md'
  $rows += Assert-RekitTextFile -Path $shim -LimitBytes 16384
  $canonicalSkill = Join-Path $templateRoot '.claude\skills\rekit\SKILL.md'
  $rows += Assert-RekitTextFile -Path $canonicalSkill -LimitBytes 32768
  Test-RekitCaseShimMatchesTemplate -CaseRoot $caseRoot -TemplateRoot $templateRoot

  foreach ($rel in $manifest.ManagedFiles) {
    $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $caseRoot -RelativePath $rel) -LimitBytes (Get-RekitBudgetLimit -Manifest $manifest -RelativePath $rel)
  }
  foreach ($rel in $manifest.TemplateFiles) {
    $targetRel = $rel -replace '\.template\.md$', '.md'
    $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $caseRoot -RelativePath $targetRel) -LimitBytes (Get-RekitBudgetLimit -Manifest $manifest -RelativePath $targetRel)
  }

  $blockHost = Join-RekitPath -Root $caseRoot -RelativePath $manifest.ManagedBlock['file']
  Test-RekitManagedBlock -Path $blockHost -BlockId $manifest.ManagedBlock['blockId']
  Test-RekitSubagentRoutesInstance -Manifest $manifest -CaseRoot $caseRoot
  $rows += Test-RekitWorkstreamState -CaseRoot $caseRoot
  return $rows
}
