param(
  [string]$Target = ''
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackRoot = Split-Path -Parent $ScriptDir
$PacksDir = Split-Path -Parent $PackRoot
$RepoRoot = Split-Path -Parent $PacksDir
$PackName = Split-Path -Leaf $PackRoot

if ([string]::IsNullOrWhiteSpace($Target)) {
  & (Join-Path $RepoRoot 'rekit\rekit.ps1') -Command validate -Target $RepoRoot -Pack $PackName
} else {
  & (Join-Path $RepoRoot 'rekit\rekit.ps1') -Command validate -Target $Target -Pack $PackName
}
