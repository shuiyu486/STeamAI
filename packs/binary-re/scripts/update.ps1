param(
  [Parameter(Mandatory=$true)]
  [string]$Target,
  [switch]$NoBackup,
  [switch]$Apply
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackRoot = Split-Path -Parent $ScriptDir
$PacksDir = Split-Path -Parent $PackRoot
$RepoRoot = Split-Path -Parent $PacksDir
$PackName = Split-Path -Leaf $PackRoot

if ($NoBackup) {
  Write-Warning 'NoBackup is kept for compatibility but rekit sync currently always creates backups when overwriting managed files.'
}

& (Join-Path $RepoRoot 'rekit\rekit.ps1') -Command sync -Target $Target -Pack $PackName -Apply:$Apply
