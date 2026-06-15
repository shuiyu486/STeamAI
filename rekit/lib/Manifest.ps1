function Convert-RekitYamlValue {
  param([string]$Value)
  if ($null -eq $Value) { return '' }
  $v = $Value.Trim()
  if (($v.StartsWith('"') -and $v.EndsWith('"')) -or ($v.StartsWith("'") -and $v.EndsWith("'"))) {
    return $v.Substring(1, $v.Length - 2)
  }
  return $v
}

function Get-RekitYamlScalar {
  param(
    [string[]]$Lines,
    [string]$Key,
    [string]$Default = ''
  )
  foreach ($line in $Lines) {
    if ($line -match ('^' + [regex]::Escape($Key) + '\s*:\s*(.*)$')) {
      return Convert-RekitYamlValue $Matches[1]
    }
  }
  return $Default
}

function Get-RekitYamlList {
  param(
    [string[]]$Lines,
    [string]$Key
  )
  $items = @()
  $inside = $false
  foreach ($line in $Lines) {
    if (-not $inside) {
      if ($line -match ('^' + [regex]::Escape($Key) + '\s*:\s*$')) { $inside = $true }
      continue
    }
    if ($line -match '^\S') { break }
    if ($line -match '^\s{2,}-\s*(.+?)\s*$') {
      $items += (Convert-RekitYamlValue $Matches[1])
    }
  }
  return $items
}

function Get-RekitYamlMap {
  param(
    [string[]]$Lines,
    [string]$Key
  )
  $map = @{}
  $inside = $false
  foreach ($line in $Lines) {
    if (-not $inside) {
      if ($line -match ('^' + [regex]::Escape($Key) + '\s*:\s*$')) { $inside = $true }
      continue
    }
    if ($line -match '^\S') { break }
    if ($line -match '^\s{2,}([^:#]+?)\s*:\s*(.*?)\s*$') {
      $map[$Matches[1].Trim()] = Convert-RekitYamlValue $Matches[2]
    }
  }
  return $map
}

function Get-RekitRepoRoot {
  param([string]$RuntimeRoot)
  return [System.IO.Path]::GetFullPath((Join-Path $RuntimeRoot '..'))
}

function Get-RekitPackManifest {
  param(
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re'
  )
  $repo = [System.IO.Path]::GetFullPath($RepoRoot)
  $packRoot = Join-Path $repo ("packs\" + $Pack)
  $manifestPath = Join-Path $packRoot 'manifest.yml'
  if (-not (Test-Path -LiteralPath $manifestPath)) { throw "missing pack manifest: $manifestPath" }

  $lines = [System.IO.File]::ReadAllLines($manifestPath, [System.Text.Encoding]::UTF8)
  $name = Get-RekitYamlScalar -Lines $lines -Key 'name' -Default $Pack
  $version = Get-RekitYamlScalar -Lines $lines -Key 'version' -Default '0.0.0'
  $description = Get-RekitYamlScalar -Lines $lines -Key 'description' -Default ''
  $managedFiles = @(Get-RekitYamlList -Lines $lines -Key 'managedFiles')
  $templateFiles = @(Get-RekitYamlList -Lines $lines -Key 'templateFiles')
  $localFiles = @(Get-RekitYamlList -Lines $lines -Key 'localNeverOverwrite')
  $promoteFiles = @(Get-RekitYamlList -Lines $lines -Key 'promoteFiles')
  $toolingFiles = @(Get-RekitYamlList -Lines $lines -Key 'toolingFiles')
  $toolingCandidateSources = @(Get-RekitYamlList -Lines $lines -Key 'toolingCandidateSources')
  $promoteDenyPatterns = @(Get-RekitYamlList -Lines $lines -Key 'promoteDenyPatterns')
  $budgets = Get-RekitYamlMap -Lines $lines -Key 'budgets'
  $managedBlock = Get-RekitYamlMap -Lines $lines -Key 'managedBlock'
  $syncPolicy = Get-RekitYamlMap -Lines $lines -Key 'syncPolicy'

  if ($managedFiles.Count -eq 0) { throw "manifest managedFiles is empty: $manifestPath" }
  if ($promoteFiles.Count -eq 0) { $promoteFiles = $managedFiles }
  if ($toolingCandidateSources.Count -eq 0) { $toolingCandidateSources = @('references/vmp-re/toolchain-router.md') }
  if (-not $managedBlock.ContainsKey('file')) { $managedBlock['file'] = 'CLAUDE.local.md' }
  if (-not $managedBlock.ContainsKey('blockId')) { $managedBlock['blockId'] = 'vmp-re-template:router' }
  if (-not $managedBlock.ContainsKey('source')) { $managedBlock['source'] = 'CLAUDE.local.snippet.md' }
  if (-not $budgets.ContainsKey('defaultMarkdown')) { $budgets['defaultMarkdown'] = '16384' }
  if ($promoteDenyPatterns.Count -eq 0) {
    $promoteDenyPatterns = @('C:\\', 'artifacts[\\/]', 'captures[\\/]', '[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\.(csv|jsonl|log|txt|bin)', '[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\.(dmp|bin|raw|exe|dll)', '\.dmp\b', '0x[0-9A-Fa-f]{6,}', 'ctx[0-9]+', 'round[0-9]+', 'Task #[0-9]+')
  }

  return [pscustomobject]@{
    RepoRoot = $repo
    Pack = $Pack
    PackRoot = $packRoot
    ManifestPath = $manifestPath
    Name = $name
    Version = $version
    Description = $description
    ManagedFiles = $managedFiles
    TemplateFiles = $templateFiles
    LocalFiles = $localFiles
    PromoteFiles = $promoteFiles
    ToolingFiles = $toolingFiles
    ToolingCandidateSources = $toolingCandidateSources
    PromoteDenyPatterns = $promoteDenyPatterns
    Budgets = $budgets
    ManagedBlock = $managedBlock
    SyncPolicy = $syncPolicy
  }
}

function Join-RekitPath {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][string]$RelativePath
  )
  if ([string]::IsNullOrWhiteSpace($RelativePath)) { throw 'relative path is empty' }
  if ([System.IO.Path]::IsPathRooted($RelativePath)) { throw "path must be relative: $RelativePath" }

  $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\')
  $pathFull = [System.IO.Path]::GetFullPath((Join-Path $rootFull $RelativePath)).TrimEnd('\')
  $isRoot = [string]::Equals($pathFull, $rootFull, [System.StringComparison]::OrdinalIgnoreCase)
  $isChild = $pathFull.StartsWith($rootFull + '\', [System.StringComparison]::OrdinalIgnoreCase)
  if (-not ($isRoot -or $isChild)) { throw "path escapes root: $RelativePath" }
  return $pathFull
}

function Get-RekitSourcePath {
  param(
    [Parameter(Mandatory=$true)]$Manifest,
    [Parameter(Mandatory=$true)][string]$RelativePath
  )
  return Join-RekitPath -Root $Manifest.PackRoot -RelativePath $RelativePath
}
