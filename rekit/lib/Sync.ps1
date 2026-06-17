function Backup-RekitFile {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$BackupRoot
  )
  if (-not (Test-Path -LiteralPath $Path)) { return '' }
  $rel = $Path.Substring($CaseRoot.Length).TrimStart('\')
  $dest = Join-Path $BackupRoot $rel
  Ensure-RekitDirectory (Split-Path -Parent $dest)
  Copy-Item -LiteralPath $Path -Destination $dest -Force
  return $dest
}

function Copy-RekitTextFile {
  param(
    [Parameter(Mandatory=$true)][string]$Source,
    [Parameter(Mandatory=$true)][string]$Destination,
    [string]$CaseRoot = '',
    [string]$BackupRoot = '',
    [switch]$WhatIf
  )
  if (-not (Test-Path -LiteralPath $Source)) { throw "missing source file: $Source" }
  if ($WhatIf) {
    Write-Host "would copy: $Source -> $Destination"
    return
  }
  Ensure-RekitDirectory (Split-Path -Parent $Destination)
  $sourceText = [System.IO.File]::ReadAllText($Source, [System.Text.Encoding]::UTF8)
  if (Test-Path -LiteralPath $Destination) {
    $destText = [System.IO.File]::ReadAllText($Destination, [System.Text.Encoding]::UTF8)
    if ($destText -ne $sourceText -and -not [string]::IsNullOrWhiteSpace($BackupRoot) -and -not [string]::IsNullOrWhiteSpace($CaseRoot)) {
      $backup = Backup-RekitFile -Path $Destination -CaseRoot $CaseRoot -BackupRoot $BackupRoot
      Write-Host "backup: $backup"
    }
  }
  [System.IO.File]::WriteAllText($Destination, $sourceText, [System.Text.UTF8Encoding]::new($false))
  Write-Host "synced: $Destination"
}

function Set-RekitManagedBlock {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$BlockId,
    [Parameter(Mandatory=$true)][string]$BlockText,
    [string]$CaseRoot = '',
    [string]$BackupRoot = '',
    [switch]$WhatIf
  )
  $block = $BlockText.Trim()
  if ($WhatIf) {
    Write-Host "would update managed block: $Path#$BlockId"
    return
  }
  Ensure-RekitDirectory (Split-Path -Parent $Path)
  if (Test-Path -LiteralPath $Path) {
    if (-not [string]::IsNullOrWhiteSpace($BackupRoot) -and -not [string]::IsNullOrWhiteSpace($CaseRoot)) {
      $backup = Backup-RekitFile -Path $Path -CaseRoot $CaseRoot -BackupRoot $BackupRoot
      Write-Host "backup: $backup"
    }
    $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
    $pattern = '(?s)<!-- BEGIN ' + [regex]::Escape($BlockId) + '.*?<!-- END ' + [regex]::Escape($BlockId) + ' -->'
    if ([regex]::IsMatch($text, $pattern)) {
      $text = [regex]::Replace($text, $pattern, [System.Text.RegularExpressions.MatchEvaluator]{ param($m) $block })
    } else {
      $text = $text.TrimEnd() + "`r`n`r`n" + $block + "`r`n"
    }
  } else {
    $text = "# Project Context`r`n`r`n" + $block + "`r`n"
  }
  [System.IO.File]::WriteAllText($Path, $text, [System.Text.UTF8Encoding]::new($false))
  Write-Host "updated managed block: $Path#$BlockId"
}

function Invoke-RekitAttach {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string]$ProjectName = '',
    [switch]$WhatIf
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  if ($WhatIf) {
    Write-Host "would attach case: $caseRoot -> $RepoRoot ($Pack)"
    return
  }
  Ensure-RekitDirectory $caseRoot
  $instance = Write-RekitInstance -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName
  $shim = Write-RekitCaseShim -CaseRoot $caseRoot -RepoRoot $RepoRoot
  Write-Host "wrote instance: $instance"
  Write-Host "wrote case shim: $shim"
}

function Sync-RekitPack {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string]$ProjectName = '',
    [switch]$WhatIf,
    [switch]$CreateLocalFiles,
    [switch]$Apply,
    [switch]$Review,
    [string]$ReviewOutputDir = '',
    [string]$PacketPath = '',
    [string]$DiffPath = ''
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $inst = Get-RekitInstance -Target $caseRoot
  if ($inst.Source -eq 'missing' -and -not $CreateLocalFiles) {
    throw "target is not an attached rekit case. Use 'rekit attach -Target `"$caseRoot`"' or 'rekit init -Target `"$caseRoot`"' first."
  }
  if (Test-RekitInstanceMoved -Instance $inst) {
    throw "case metadata points to a different directory. Run 'rekit repair -Target `"$caseRoot`" -Apply' after confirming the move."
  }
  if ([string]::IsNullOrWhiteSpace($ProjectName) -and $inst.Source -ne 'missing') { $ProjectName = $inst.ProjectName }
  if ($Review) {
    Write-RekitSyncReview -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -CreateLocalFiles:$CreateLocalFiles -ReviewOutputDir $ReviewOutputDir -PacketPath $PacketPath -DiffPath $DiffPath
    return
  }
  if (-not $Apply -and -not $WhatIf) { throw 'sync writes managed files and state; run sync without -Apply for review, or re-run with -Apply after user confirmation.' }
  Invoke-RekitAttach -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -WhatIf:$WhatIf

  $backupRoot = Join-Path $caseRoot ("references\vmp-re\.backup\" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
  foreach ($rel in $manifest.ManagedFiles) {
    $source = Get-RekitSourcePath -Manifest $manifest -RelativePath $rel
    $dest = Join-RekitPath -Root $caseRoot -RelativePath $rel
    Copy-RekitTextFile -Source $source -Destination $dest -CaseRoot $caseRoot -BackupRoot $backupRoot -WhatIf:$WhatIf
  }

  foreach ($rel in $manifest.TemplateFiles) {
    $source = Get-RekitSourcePath -Manifest $manifest -RelativePath $rel
    $targetRel = $rel -replace '\.template\.md$', '.md'
    $dest = Join-RekitPath -Root $caseRoot -RelativePath $targetRel
    if ((Test-Path -LiteralPath $dest) -and -not $CreateLocalFiles) {
      Write-Host "skip existing local file: $dest"
      continue
    }
    if ($WhatIf) {
      Write-Host "would create local template file: $dest"
      continue
    }
    Ensure-RekitDirectory (Split-Path -Parent $dest)
    $text = [System.IO.File]::ReadAllText($source, [System.Text.Encoding]::UTF8)
    $project = if ([string]::IsNullOrWhiteSpace($ProjectName)) { Get-RekitProjectName $caseRoot } else { $ProjectName }
    $text = $text.Replace('<PROJECT_NAME>', $project).Replace('<PROJECT_ROOT>', $caseRoot)
    [System.IO.File]::WriteAllText($dest, $text, [System.Text.UTF8Encoding]::new($false))
    Write-Host "created local template file: $dest"
  }

  $blockSource = Join-RekitPath -Root $manifest.PackRoot -RelativePath $manifest.ManagedBlock['source']
  $blockHost = Join-RekitPath -Root $caseRoot -RelativePath $manifest.ManagedBlock['file']
  $blockText = [System.IO.File]::ReadAllText($blockSource, [System.Text.Encoding]::UTF8)
  Set-RekitManagedBlock -Path $blockHost -BlockId $manifest.ManagedBlock['blockId'] -BlockText $blockText -CaseRoot $caseRoot -BackupRoot $backupRoot -WhatIf:$WhatIf

  $gitignoreSource = Join-RekitPath -Root $manifest.PackRoot -RelativePath 'examples/gitignore.example'
  $gitignoreTarget = Join-Path $caseRoot '.gitignore'
  if ((Test-Path -LiteralPath $gitignoreSource) -and -not (Test-Path -LiteralPath $gitignoreTarget)) {
    Copy-RekitTextFile -Source $gitignoreSource -Destination $gitignoreTarget -WhatIf:$WhatIf
  }

  if (-not $WhatIf) {
    Update-RekitStateAfterSync -CaseRoot $caseRoot -Manifest $manifest
    Test-RekitInstance -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack | ForEach-Object {
      Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
    }
    Write-Host 'sync ok'
  }
}
