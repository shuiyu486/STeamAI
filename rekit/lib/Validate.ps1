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

function Test-RekitPolicyRegistry {
  param([Parameter(Mandatory=$true)]$Manifest)
  $rows = @()
  $commonManifest = Join-RekitPath -Root $Manifest.RepoRoot -RelativePath 'common/policies/manifest.yml'
  $rows += Assert-RekitTextFile -Path $commonManifest -LimitBytes 16384
  $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $Manifest.RepoRoot -RelativePath 'common/policies/README.md') -LimitBytes 16384
  foreach ($rel in (Get-RekitPolicyManifestPaths -ManifestPath $commonManifest -ListKey 'policies' -PathKey 'path')) {
    $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root (Split-Path -Parent $commonManifest) -RelativePath $rel) -LimitBytes 16384
  }

  $overlayManifest = Join-RekitPath -Root $Manifest.PackRoot -RelativePath 'policies/manifest.yml'
  $rows += Assert-RekitTextFile -Path $overlayManifest -LimitBytes 16384
  $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root $Manifest.PackRoot -RelativePath 'policies/README.md') -LimitBytes 16384
  foreach ($rel in (Get-RekitPolicyManifestPaths -ManifestPath $overlayManifest -ListKey 'overlays' -PathKey 'path')) {
    $rows += Assert-RekitTextFile -Path (Join-RekitPath -Root (Split-Path -Parent $overlayManifest) -RelativePath $rel) -LimitBytes 16384
  }
  foreach ($rel in $Manifest.PolicyOverlays) {
    [void](Join-RekitPath -Root $Manifest.PackRoot -RelativePath $rel)
  }
  return $rows
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

  foreach ($rel in $manifest.ManagedFiles) {
    $rows += Assert-RekitTextFile -Path (Get-RekitSourcePath -Manifest $manifest -RelativePath $rel) -LimitBytes (Get-RekitBudgetLimit -Manifest $manifest -RelativePath $rel)
  }
  foreach ($rel in $manifest.TemplateFiles) {
    $rows += Assert-RekitTextFile -Path (Get-RekitSourcePath -Manifest $manifest -RelativePath $rel) -LimitBytes (Get-RekitBudgetLimit -Manifest $manifest -RelativePath ($rel -replace '\.template\.md$', '.md'))
  }
  foreach ($rel in $manifest.ToolingFiles) {
    $rows += Assert-RekitTextFile -Path (Get-RekitSourcePath -Manifest $manifest -RelativePath $rel) -LimitBytes (Get-RekitBudgetLimit -Manifest $manifest -RelativePath $rel)
  }
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
  return $rows
}
