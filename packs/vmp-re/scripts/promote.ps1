param(
  [Parameter(Mandatory=$true)]
  [string]$Target,
  [switch]$WhatIf,
  [switch]$Apply
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackRoot = Split-Path -Parent $ScriptDir
$PacksDir = Split-Path -Parent $PackRoot
$RepoRoot = Split-Path -Parent $PacksDir
$PackName = Split-Path -Leaf $PackRoot

& (Join-Path $RepoRoot 'rekit\rekit.ps1') -Command promote -Target $Target -Pack $PackName -WhatIf:$WhatIf -Apply:$Apply
