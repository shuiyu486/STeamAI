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

function Get-RekitCaseSpecificPatterns {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $caseName = Split-Path -Leaf ([System.IO.Path]::GetFullPath($CaseRoot).TrimEnd('\'))
  $terms = @($caseName -split '[-_\.\s]+' | Where-Object { $_.Length -ge 4 })
  return @($terms | ForEach-Object { [regex]::Escape($_) })
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

function Convert-RekitToolingCandidateText {
  param(
    [Parameter(Mandatory=$true)][string]$Text,
    [Parameter(Mandatory=$true)][string]$CaseRoot
  )
  $case = [regex]::Escape(([System.IO.Path]::GetFullPath($CaseRoot)).TrimEnd('\'))
  $out = $Text
  $out = [regex]::Replace($out, $case, '<caseRoot>', 'IgnoreCase')
  $out = [regex]::Replace($out, '[A-Za-z]:\\[^`\r\n|，。；;\)\] ]+', '<absolutePath>', 'IgnoreCase')
  foreach ($pattern in (Get-RekitCaseSpecificPatterns -CaseRoot $CaseRoot)) {
    $out = [regex]::Replace($out, $pattern + '[A-Za-z0-9_.-]*\.exe', '<target.exe>', 'IgnoreCase')
    $out = [regex]::Replace($out, $pattern + '[\\/]', '<case>/', 'IgnoreCase')
    $out = [regex]::Replace($out, $pattern, '<caseTerm>', 'IgnoreCase')
  }
  $out = [regex]::Replace($out, '\.\.[\\/]tools[\\/]', '<toolsRoot>/', 'IgnoreCase')
  $out = [regex]::Replace($out, 'artifacts[\\/][^`\r\n|，。；;\)\] ]+', '<artifactsPath>', 'IgnoreCase')
  $out = [regex]::Replace($out, 'captures[\\/][^`\r\n|，。；;\)\] ]+', '<capturesPath>', 'IgnoreCase')
  $out = [regex]::Replace($out, '[A-Za-z0-9_.-]*trace[A-Za-z0-9_.-]*\.(csv|jsonl|log|txt|bin)', '<traceFile>', 'IgnoreCase')
  $out = [regex]::Replace($out, '[A-Za-z0-9_.-]*dump[A-Za-z0-9_.-]*\.(dmp|bin|raw|exe|dll)', '<dumpFile>', 'IgnoreCase')
  $out = [regex]::Replace($out, '0x[0-9A-Fa-f]{6,}', '<address>')
  $out = [regex]::Replace($out, 'ctx\d+', '<ctxNNN>', 'IgnoreCase')
  $out = [regex]::Replace($out, 'round\d+', '<roundN>', 'IgnoreCase')
  $out = [regex]::Replace($out, 'Task #\d+', 'Task #<n>', 'IgnoreCase')
  return $out
}

function Write-RekitToolingCandidates {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)]$Manifest,
    [switch]$WhatIf
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $candidateRoot = Join-Path $Manifest.PackRoot 'tooling\candidates'
  $denyPatterns = @($Manifest.PromoteDenyPatterns) + @(Get-RekitCaseSpecificPatterns -CaseRoot $caseRoot)
  $written = @()
  foreach ($rel in $Manifest.ToolingCandidateSources) {
    $source = Join-RekitPath -Root $caseRoot -RelativePath $rel
    if (-not (Test-Path -LiteralPath $source)) {
      Write-Host "skip missing tooling source: $source"
      continue
    }
    $raw = [System.IO.File]::ReadAllText($source, [System.Text.Encoding]::UTF8)
    $sanitized = Convert-RekitToolingCandidateText -Text $raw -CaseRoot $caseRoot
    $violations = @(Test-RekitPromoteContent -Text $sanitized -Patterns $denyPatterns)
    if ($violations.Count -gt 0) {
      Write-Host "blocked tooling candidate after sanitization: $rel"
      foreach ($v in $violations) { Write-Host "  remaining pattern: $v" }
      continue
    }
    $safeName = ($rel -replace '[\\/:*?"<>|]', '_')
    $dest = Join-Path $candidateRoot ((Get-Date -Format 'yyyyMMdd-HHmmss') + '_tooling_' + $safeName + '.candidate.md')
    if ($WhatIf) {
      $written += $dest
      Write-Host "would write tooling candidate: $dest"
      continue
    }
    Ensure-RekitDirectory $candidateRoot
    $header = New-RekitToolingCandidateHeader -RelativePath $rel
    [System.IO.File]::WriteAllText($dest, $header + $sanitized, [System.Text.UTF8Encoding]::new($false))
    $written += $dest
    Write-Host "tooling candidate written: $dest"
  }
  return $written
}

function Promote-RekitChanges {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [switch]$WhatIf,
    [switch]$Apply,
    [switch]$CreateCandidates,
    [switch]$Review,
    [string]$ReviewOutputDir = '',
    [string]$PacketPath = '',
    [string]$DiffPath = ''
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  if ($Review) {
    Write-RekitPromoteReview -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ReviewOutputDir $ReviewOutputDir -PacketPath $PacketPath -DiffPath $DiffPath
    return
  }
  if (-not ($Apply -or $CreateCandidates -or $WhatIf)) { throw 'promote writes candidates or pack files; run promote without write flags for review, or re-run with -CreateCandidates / -Apply after user confirmation.' }
  $candidateRoot = Join-Path $manifest.PackRoot 'promote-candidates'
  if ($CreateCandidates -and -not $WhatIf) { Ensure-RekitDirectory $candidateRoot }

  $denyPatterns = @($manifest.PromoteDenyPatterns) + @(Get-RekitCaseSpecificPatterns -CaseRoot $caseRoot)
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

    $violations = @(Test-RekitPromoteContent -Text $caseText -Patterns $denyPatterns)
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
    } elseif ($CreateCandidates) {
      $safeName = ($rel -replace '[\\/:*?"<>|]', '_')
      $candidate = Join-Path $candidateRoot ((Get-Date -Format 'yyyyMMdd-HHmmss') + '_' + $safeName + '.candidate.md')
      [System.IO.File]::WriteAllText($candidate, $caseText, [System.Text.UTF8Encoding]::new($false))
      $candidateIndex += [pscustomobject]@{ path = $rel; candidate = $candidate }
      Write-Host "candidate written: $candidate"
    }
  }

  if (-not $WhatIf -and $CreateCandidates -and $candidateIndex.Count -gt 0) {
    $indexPath = Join-Path $candidateRoot 'index.json'
    [System.IO.File]::WriteAllText($indexPath, ($candidateIndex | ConvertTo-Json -Depth 4), [System.Text.UTF8Encoding]::new($false))
    Write-Host "candidate index: $indexPath"
  }

  $toolingCandidates = @()
  if ($CreateCandidates -or $WhatIf) {
    $toolingCandidates = @(Write-RekitToolingCandidates -Target $caseRoot -Manifest $manifest -WhatIf:$WhatIf)
  }

  if ($Apply -and -not $WhatIf) {
    Test-RekitPack -RepoRoot $RepoRoot -Pack $Pack | ForEach-Object {
      Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
    }
  }
  Write-Host "promote summary: changed=$changed blocked=$blocked toolingCandidates=$($toolingCandidates.Count) apply=$Apply createCandidates=$CreateCandidates whatIf=$WhatIf"
}
