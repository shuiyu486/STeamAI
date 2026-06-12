param(
  [Parameter(Mandatory=$true)]
  [string]$Target,
  [switch]$NoBackup
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackRoot = Split-Path -Parent $ScriptDir
$PackName = Split-Path -Leaf $PackRoot
$Version = '0.1.0'

function Backup-IfNeeded {
  param(
    [string]$Path,
    [string]$BackupRoot
  )
  if ($NoBackup -or -not (Test-Path -LiteralPath $Path)) { return }
  $rel = $Path.Substring($Target.Length).TrimStart('\')
  $dest = Join-Path $BackupRoot $rel
  $parent = Split-Path -Parent $dest
  if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
  Copy-Item -LiteralPath $Path -Destination $dest -Force
}

function Copy-ManagedFile {
  param(
    [string]$Source,
    [string]$Destination,
    [string]$BackupRoot
  )
  Backup-IfNeeded -Path $Destination -BackupRoot $BackupRoot
  $parent = Split-Path -Parent $Destination
  if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
  Copy-Item -LiteralPath $Source -Destination $Destination -Force
  Write-Host "updated managed file: $Destination"
}

function Set-ManagedBlock {
  param(
    [string]$Path,
    [string]$Block,
    [string]$BackupRoot
  )
  Backup-IfNeeded -Path $Path -BackupRoot $BackupRoot
  if (Test-Path -LiteralPath $Path) {
    $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
    $pattern = '(?s)<!-- BEGIN vmp-re-template:router.*?<!-- END vmp-re-template:router -->'
    if ([regex]::IsMatch($text, $pattern)) {
      $text = [regex]::Replace($text, $pattern, [System.Text.RegularExpressions.MatchEvaluator]{ param($m) $Block })
    } else {
      $text = $text.TrimEnd() + "`r`n`r`n" + $Block + "`r`n"
    }
  } else {
    $text = "# Project Context`r`n`r`n" + $Block + "`r`n"
  }
  [System.IO.File]::WriteAllText($Path, $text, [System.Text.UTF8Encoding]::new($false))
  Write-Host "updated managed CLAUDE block: $Path"
}

$Target = [System.IO.Path]::GetFullPath($Target)
if (-not (Test-Path -LiteralPath $Target)) { throw "target not found: $Target" }

$backupRoot = Join-Path $Target ("references\vmp-re\.backup\" + (Get-Date -Format 'yyyyMMdd-HHmmss'))
if (-not $NoBackup) { New-Item -ItemType Directory -Path $backupRoot -Force | Out-Null }

$TargetRefs = Join-Path $Target 'references\vmp-re'
$ManagedDocs = @(
  'README.md',
  'workflow-template.md',
  'progressive-disclosure.md',
  'toolchain-router.md',
  'singleton-handler-review.md'
)
foreach ($doc in $ManagedDocs) {
  Copy-ManagedFile -Source (Join-Path $PackRoot "references\vmp-re\$doc") -Destination (Join-Path $TargetRefs $doc) -BackupRoot $backupRoot
}

$snippet = [System.IO.File]::ReadAllText((Join-Path $PackRoot 'CLAUDE.local.snippet.md'), [System.Text.Encoding]::UTF8).Trim()
Set-ManagedBlock -Path (Join-Path $Target 'CLAUDE.local.md') -Block $snippet -BackupRoot $backupRoot

$metaPath = Join-Path $Target '.re-template.yml'
Backup-IfNeeded -Path $metaPath -BackupRoot $backupRoot
$meta = @"
templateRepo: git@github.com:shuiyu486/re-context-kits.git
templatePack: $PackName
templateVersion: $Version
updatedAt: $(Get-Date -Format 'yyyy-MM-dd')

managedFiles:
  - references/vmp-re/README.md
  - references/vmp-re/workflow-template.md
  - references/vmp-re/progressive-disclosure.md
  - references/vmp-re/toolchain-router.md
  - references/vmp-re/singleton-handler-review.md

localFiles:
  - CLAUDE.local.md
  - references/vmp-re/task-handoff.md
  - tools.local.yml

updatePolicy:
  managedFiles: overwrite-with-backup
  localFiles: never-overwrite
"@
[System.IO.File]::WriteAllText($metaPath, $meta, [System.Text.UTF8Encoding]::new($false))
Write-Host "updated metadata: $metaPath"
if (-not $NoBackup) { Write-Host "backup root: $backupRoot" }

& (Join-Path $ScriptDir 'validate.ps1') -Target $Target
