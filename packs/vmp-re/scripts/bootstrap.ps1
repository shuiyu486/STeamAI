param(
  [Parameter(Mandatory=$true)]
  [string]$Target,
  [string]$ProjectName = '',
  [switch]$Force
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackRoot = Split-Path -Parent $ScriptDir
$PacksDir = Split-Path -Parent $PackRoot
$RepoRoot = Split-Path -Parent $PacksDir
$PackName = Split-Path -Leaf $PackRoot
$Version = '0.1.0'

if ([string]::IsNullOrWhiteSpace($ProjectName)) {
  $ProjectName = Split-Path -Leaf (Resolve-Path -LiteralPath (Split-Path -Parent $Target) -ErrorAction SilentlyContinue)
  if ([string]::IsNullOrWhiteSpace($ProjectName)) { $ProjectName = Split-Path -Leaf $Target }
}

function Copy-TemplateFile {
  param(
    [string]$Source,
    [string]$Destination,
    [switch]$NeverOverwrite
  )
  $parent = Split-Path -Parent $Destination
  if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
  if ((Test-Path -LiteralPath $Destination) -and $NeverOverwrite -and -not $Force) {
    Write-Host "skip existing local file: $Destination"
    return
  }
  Copy-Item -LiteralPath $Source -Destination $Destination -Force
  Write-Host "wrote: $Destination"
}

function Set-ManagedBlock {
  param(
    [string]$Path,
    [string]$Block
  )
  $parent = Split-Path -Parent $Path
  if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
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
if (-not (Test-Path -LiteralPath $Target)) { New-Item -ItemType Directory -Path $Target -Force | Out-Null }

$TargetRefs = Join-Path $Target 'references\vmp-re'
New-Item -ItemType Directory -Path $TargetRefs -Force | Out-Null

$ManagedDocs = @(
  'README.md',
  'workflow-template.md',
  'progressive-disclosure.md',
  'toolchain-router.md',
  'singleton-handler-review.md'
)
foreach ($doc in $ManagedDocs) {
  Copy-TemplateFile -Source (Join-Path $PackRoot "references\vmp-re\$doc") -Destination (Join-Path $TargetRefs $doc)
}

$taskTemplate = Join-Path $PackRoot 'references\vmp-re\task-handoff.template.md'
$taskOut = Join-Path $TargetRefs 'task-handoff.md'
if ((-not (Test-Path -LiteralPath $taskOut)) -or $Force) {
  $taskText = [System.IO.File]::ReadAllText($taskTemplate, [System.Text.Encoding]::UTF8)
  $taskText = $taskText.Replace('<PROJECT_NAME>', $ProjectName).Replace('<PROJECT_ROOT>', $Target)
  [System.IO.File]::WriteAllText($taskOut, $taskText, [System.Text.UTF8Encoding]::new($false))
  Write-Host "wrote local task handoff: $taskOut"
} else {
  Write-Host "skip existing local task handoff: $taskOut"
}

$snippet = [System.IO.File]::ReadAllText((Join-Path $PackRoot 'CLAUDE.local.snippet.md'), [System.Text.Encoding]::UTF8).Trim()
Set-ManagedBlock -Path (Join-Path $Target 'CLAUDE.local.md') -Block $snippet

$gitignoreSource = Join-Path $PackRoot 'examples\gitignore.example'
$gitignoreTarget = Join-Path $Target '.gitignore'
if (-not (Test-Path -LiteralPath $gitignoreTarget)) {
  Copy-TemplateFile -Source $gitignoreSource -Destination $gitignoreTarget -NeverOverwrite
}

$meta = @"
templateRepo: git@github.com:shuiyu486/re-context-kits.git
templatePack: $PackName
templateVersion: $Version
installedAt: $(Get-Date -Format 'yyyy-MM-dd')

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
[System.IO.File]::WriteAllText((Join-Path $Target '.re-template.yml'), $meta, [System.Text.UTF8Encoding]::new($false))
Write-Host "wrote metadata: $(Join-Path $Target '.re-template.yml')"

& (Join-Path $ScriptDir 'validate.ps1') -Target $Target
