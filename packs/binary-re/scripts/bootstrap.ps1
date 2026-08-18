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

$params = @{
  Command = 'init'
  Target = $Target
  Pack = $PackName
}
if (-not [string]::IsNullOrWhiteSpace($ProjectName)) { $params.ProjectName = $ProjectName }
if ($Force) { $params.Force = $true }

& (Join-Path $RepoRoot 'rekit\rekit.ps1') @params
