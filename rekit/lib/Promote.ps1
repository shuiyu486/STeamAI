function Test-RekitPromoteContent {
  param(
    [Parameter(Mandatory=$true)][string]$Text,
    [Parameter(Mandatory=$true)][string[]]$Patterns
  )
  $violations = @()
  foreach ($pattern in $Patterns) {
    if ([string]::IsNullOrWhiteSpace($pattern)) { continue }
    try {
      if ($Text -match $pattern) { $violations += $pattern }
    } catch {
      if ($Text.Contains($pattern)) { $violations += $pattern }
    }
  }
  return $violations
}

function Backup-RekitPackFile {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)]$Manifest
  )
  if (-not (Test-Path -LiteralPath $Path)) { return '' }
  $rel = $Path.Substring($Manifest.PackRoot.Length).TrimStart('\')
  $dest = Join-Path $Manifest.PackRoot ("promote-candidates\.backup\" + (Get-Date -Format 'yyyyMMdd-HHmmss') + "\" + $rel)
  Ensure-RekitDirectory (Split-Path -Parent $dest)
  Copy-Item -LiteralPath $Path -Destination $dest -Force
  return $dest
}

function Promote-RekitChanges {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [switch]$WhatIf,
    [switch]$Apply
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $candidateRoot = Join-Path $manifest.PackRoot 'promote-candidates'
  if (-not $WhatIf) { Ensure-RekitDirectory $candidateRoot }

  $changed = 0
  $blocked = 0
  $candidateIndex = @()
  foreach ($rel in $manifest.PromoteFiles) {
    if ($manifest.ManagedFiles -notcontains $rel) {
      Write-Host "skip non-managed promote file: $rel"
      continue
    }
    $caseFile = Join-RekitPath -Root $caseRoot -RelativePath $rel
    $packFile = Get-RekitSourcePath -Manifest $manifest -RelativePath $rel
    if (-not (Test-Path -LiteralPath $caseFile)) {
      Write-Host "skip missing case file: $caseFile"
      continue
    }
    if (-not (Test-Path -LiteralPath $packFile)) { throw "missing pack file: $packFile" }
    $caseText = [System.IO.File]::ReadAllText($caseFile, [System.Text.Encoding]::UTF8)
    $packText = [System.IO.File]::ReadAllText($packFile, [System.Text.Encoding]::UTF8)
    if ($caseText -eq $packText) {
      Write-Host "unchanged: $rel"
      continue
    }

    $violations = @(Test-RekitPromoteContent -Text $caseText -Patterns $manifest.PromoteDenyPatterns)
    if ($violations.Count -gt 0) {
      $blocked++
      Write-Host "blocked promote: $rel"
      foreach ($v in $violations) { Write-Host "  deny pattern: $v" }
      continue
    }

    $changed++
    if ($WhatIf) {
      Write-Host "would promote candidate: $rel"
      continue
    }

    if ($Apply) {
      $backup = Backup-RekitPackFile -Path $packFile -Manifest $manifest
      if (-not [string]::IsNullOrWhiteSpace($backup)) { Write-Host "backup pack file: $backup" }
      [System.IO.File]::WriteAllText($packFile, $caseText, [System.Text.UTF8Encoding]::new($false))
      Write-Host "promoted: $caseFile -> $packFile"
    } else {
      $safeName = ($rel -replace '[\\/:*?"<>|]', '_')
      $candidate = Join-Path $candidateRoot ((Get-Date -Format 'yyyyMMdd-HHmmss') + '_' + $safeName + '.candidate.md')
      [System.IO.File]::WriteAllText($candidate, $caseText, [System.Text.UTF8Encoding]::new($false))
      $candidateIndex += [pscustomobject]@{ path = $rel; candidate = $candidate }
      Write-Host "candidate written: $candidate"
    }
  }

  if (-not $WhatIf -and -not $Apply -and $candidateIndex.Count -gt 0) {
    $indexPath = Join-Path $candidateRoot 'index.json'
    [System.IO.File]::WriteAllText($indexPath, ($candidateIndex | ConvertTo-Json -Depth 4), [System.Text.UTF8Encoding]::new($false))
    Write-Host "candidate index: $indexPath"
  }

  if ($Apply -and -not $WhatIf) {
    Test-RekitPack -RepoRoot $RepoRoot -Pack $Pack | ForEach-Object {
      Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
    }
  }
  Write-Host "promote summary: changed=$changed blocked=$blocked apply=$Apply whatIf=$WhatIf"
}
