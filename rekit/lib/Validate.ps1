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

function Test-RekitParallelManifest {
  param([Parameter(Mandatory=$true)]$Manifest)
  $rows = @()
  foreach ($key in @('defaultKind','sessionRoot','reviewRoot','workspaceRoot','workspaceDateLayout','initialStatus','defaultLifecycleMode')) {
    if (-not $Manifest.ParallelDefaults.ContainsKey($key) -or [string]::IsNullOrWhiteSpace([string]$Manifest.ParallelDefaults[$key])) {
      throw "manifest parallelDefaults.$key is empty: $($Manifest.ManifestPath)"
    }
  }
  foreach ($key in @('sessionRoot','reviewRoot','workspaceRoot')) {
    [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath ([string]$Manifest.ParallelDefaults[$key]))
  }

  $seen = @{}
  foreach ($rel in @($Manifest.ParallelTemplateFiles)) {
    if ([string]::IsNullOrWhiteSpace([string]$rel)) { throw "manifest parallelTemplateFiles contains an empty path: $($Manifest.ManifestPath)" }
    $seen[[string]$rel] = $true
    $rows += Assert-RekitTextFile -Path (Get-RekitSourcePath -Manifest $Manifest -RelativePath ([string]$rel)) -LimitBytes (Get-RekitBudgetLimit -Manifest $Manifest -RelativePath ([string]$rel))
  }

  foreach ($file in @($Manifest.ParallelFiles)) {
    if ([string]::IsNullOrWhiteSpace([string]$file.Path)) { throw "manifest parallelFiles entry is missing path: $($Manifest.ManifestPath)" }
    [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath ([string]$file.Path))
    if (-not [string]::IsNullOrWhiteSpace([string]$file.Template)) {
      $template = [string]$file.Template
      [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath $template)
      if (-not $seen.ContainsKey($template)) { throw "parallelFiles template is not listed in parallelTemplateFiles: $template" }
    }
    if (-not [string]::IsNullOrWhiteSpace([string]$file.StatusColumn) -and [string]::IsNullOrWhiteSpace([string]$file.CounterName)) {
      throw "parallelFiles entry with statusColumn must declare counterName: $($file.Path)"
    }
    if (-not [string]::IsNullOrWhiteSpace([string]$file.CounterName)) {
      $counter = ([string]$file.CounterName).ToLowerInvariant()
      if (@('loweringrequests','vmblockers','candidaterows') -notcontains $counter) { throw "unsupported parallelFiles counterName '$($file.CounterName)' for $($file.Path)" }
    }
  }

  foreach ($rel in @($Manifest.ParallelReadOnlyFiles)) {
    if ([string]::IsNullOrWhiteSpace([string]$rel)) { throw "manifest parallelReadOnlyFiles contains an empty path: $($Manifest.ManifestPath)" }
    [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath ([string]$rel))
  }
  return $rows
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
  if ($Manifest.LaneTypes.Count -gt 0 -and -not $hasAuthority) { throw "manifest laneTypes must include at least one authority lane" }
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
  $rows += Test-RekitParallelManifest -Manifest $manifest
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
  return $rows
}
