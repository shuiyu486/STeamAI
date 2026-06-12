function Ensure-RekitDirectory {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Path $Path -Force | Out-Null
  }
}

function Get-RekitFileHash {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return '' }
  return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Get-RekitProjectName {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  return Split-Path -Leaf ([System.IO.Path]::GetFullPath($CaseRoot).TrimEnd('\'))
}

function Get-RekitInstance {
  param([Parameter(Mandatory=$true)][string]$Target)
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $instancePath = Join-Path $caseRoot '.rekit\instance.yml'
  $legacyPath = Join-Path $caseRoot '.re-template.yml'

  if (Test-Path -LiteralPath $instancePath) {
    $lines = [System.IO.File]::ReadAllLines($instancePath, [System.Text.Encoding]::UTF8)
    return [pscustomobject]@{
      CaseRoot = $caseRoot
      InstancePath = $instancePath
      Source = 'instance'
      TemplateRoot = Get-RekitYamlScalar -Lines $lines -Key 'templateRoot'
      TemplatePack = Get-RekitYamlScalar -Lines $lines -Key 'templatePack' -Default 'vmp-re'
      ProjectName = Get-RekitYamlScalar -Lines $lines -Key 'projectName' -Default (Get-RekitProjectName $caseRoot)
      ProjectRoot = Get-RekitYamlScalar -Lines $lines -Key 'projectRoot' -Default $caseRoot
    }
  }

  if (Test-Path -LiteralPath $legacyPath) {
    $lines = [System.IO.File]::ReadAllLines($legacyPath, [System.Text.Encoding]::UTF8)
    return [pscustomobject]@{
      CaseRoot = $caseRoot
      InstancePath = $legacyPath
      Source = 'legacy'
      TemplateRoot = Get-RekitYamlScalar -Lines $lines -Key 'templateRoot'
      TemplatePack = Get-RekitYamlScalar -Lines $lines -Key 'templatePack' -Default 'vmp-re'
      ProjectName = Get-RekitProjectName $caseRoot
      ProjectRoot = Get-RekitYamlScalar -Lines $lines -Key 'currentProjectPath' -Default $caseRoot
    }
  }

  return [pscustomobject]@{
    CaseRoot = $caseRoot
    InstancePath = $instancePath
    Source = 'missing'
    TemplateRoot = ''
    TemplatePack = 'vmp-re'
    ProjectName = Get-RekitProjectName $caseRoot
    ProjectRoot = $caseRoot
  }
}

function Set-RekitYamlScalar {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Key,
    [Parameter(Mandatory=$true)][string]$Value
  )
  if (Test-Path -LiteralPath $Path) {
    $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
    $pattern = '(?m)^' + [regex]::Escape($Key) + '\s*:.*$'
    $line = "${Key}: $Value"
    if ([regex]::IsMatch($text, $pattern)) {
      $text = [regex]::Replace($text, $pattern, [System.Text.RegularExpressions.MatchEvaluator]{ param($m) $line })
    } else {
      $text = $text.TrimEnd() + "`r`n$line`r`n"
    }
  } else {
    $text = "${Key}: $Value`r`n"
  }
  [System.IO.File]::WriteAllText($Path, $text, [System.Text.UTF8Encoding]::new($false))
}

function Write-RekitInstance {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string]$ProjectName = ''
  )
  $case = [System.IO.Path]::GetFullPath($CaseRoot)
  $repo = [System.IO.Path]::GetFullPath($RepoRoot)
  if ([string]::IsNullOrWhiteSpace($ProjectName)) { $ProjectName = Get-RekitProjectName $case }
  $metaDir = Join-Path $case '.rekit'
  Ensure-RekitDirectory $metaDir
  $instancePath = Join-Path $metaDir 'instance.yml'
  $text = @"
schemaVersion: 1
templateRoot: $repo
templatePack: $Pack
projectName: $ProjectName
projectRoot: $case
mode: case-local-shim
"@
  [System.IO.File]::WriteAllText($instancePath, $text, [System.Text.UTF8Encoding]::new($false))

  $legacyPath = Join-Path $case '.re-template.yml'
  $legacyExisted = Test-Path -LiteralPath $legacyPath
  Set-RekitYamlScalar -Path $legacyPath -Key 'templateRoot' -Value $repo
  Set-RekitYamlScalar -Path $legacyPath -Key 'rekitMode' -Value 'case-local-shim'
  if (-not $legacyExisted) {
    Set-RekitYamlScalar -Path $legacyPath -Key 'templatePack' -Value $Pack
    Set-RekitYamlScalar -Path $legacyPath -Key 'templateVersion' -Value '0.0.0'
  }

  $statePath = Join-Path $metaDir 'state.json'
  if (-not (Test-Path -LiteralPath $statePath)) {
    $state = [ordered]@{
      schemaVersion = 1
      templateRoot = $repo
      templatePack = $Pack
      managed = [ordered]@{}
      promote = [ordered]@{ candidates = @() }
    }
    [System.IO.File]::WriteAllText($statePath, ($state | ConvertTo-Json -Depth 6), [System.Text.UTF8Encoding]::new($false))
  }
  return $instancePath
}

function Write-RekitCaseShim {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$RepoRoot
  )
  $templatePath = Join-Path $RepoRoot 'rekit\templates\case-shim\SKILL.md'
  if (-not (Test-Path -LiteralPath $templatePath)) { throw "missing case shim template: $templatePath" }
  $destDir = Join-Path $CaseRoot '.claude\skills\rekit'
  Ensure-RekitDirectory $destDir
  $dest = Join-Path $destDir 'SKILL.md'
  $text = [System.IO.File]::ReadAllText($templatePath, [System.Text.Encoding]::UTF8)
  [System.IO.File]::WriteAllText($dest, $text, [System.Text.UTF8Encoding]::new($false))
  return $dest
}

function Update-RekitStateAfterSync {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)]$Manifest
  )
  $statePath = Join-Path $CaseRoot '.rekit\state.json'
  $managed = [ordered]@{}
  foreach ($rel in $Manifest.ManagedFiles) {
    $source = Get-RekitSourcePath -Manifest $Manifest -RelativePath $rel
    $target = Join-RekitPath -Root $CaseRoot -RelativePath $rel
    $managed[$rel] = [ordered]@{
      sourceHash = Get-RekitFileHash $source
      targetHashAtSync = Get-RekitFileHash $target
      lastAction = 'sync'
    }
  }
  $state = [ordered]@{
    schemaVersion = 1
    templateRoot = $Manifest.RepoRoot
    templatePack = $Manifest.Pack
    lastSyncAt = (Get-Date -Format 'yyyy-MM-ddTHH:mm:ssK')
    managed = $managed
  }
  Ensure-RekitDirectory (Split-Path -Parent $statePath)
  [System.IO.File]::WriteAllText($statePath, ($state | ConvertTo-Json -Depth 8), [System.Text.UTF8Encoding]::new($false))
}
