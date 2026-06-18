function Test-RekitGoRuntime {
  param([Parameter(Mandatory=$true)][string]$RepoRoot)
  $repo = [System.IO.Path]::GetFullPath($RepoRoot)
  $goRoot = Join-Path $repo 'rekit-go'
  $goMod = Join-Path $goRoot 'go.mod'
  $goMain = Join-Path $goRoot 'cmd\rekit\main.go'
  if (-not (Test-Path -LiteralPath $goMod)) { throw "missing Go rekit runtime module: $goMod" }
  if (-not (Test-Path -LiteralPath $goMain)) { throw "missing Go rekit runtime entrypoint: $goMain" }
  $go = Get-Command go -ErrorAction SilentlyContinue
  if ($null -eq $go) { throw 'missing Go toolchain: `go` was not found in PATH. Install Go 1.22+ or use a rekit build that provides a compiled runtime.' }
  return [pscustomobject]@{ Root = $goRoot; Go = $go.Source; Module = $goMod; Entrypoint = $goMain }
}

function Invoke-RekitGoParallel {
  param(
    [Parameter(Mandatory=$true)][string]$Target,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [string[]]$RemainingArgs = @(),
    [switch]$Force,
    [switch]$WhatIf,
    [string]$ReviewOutputDir = '',
    [string]$PacketPath = '',
    [string]$DiffPath = ''
  )
  $caseRoot = [System.IO.Path]::GetFullPath($Target)
  [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
  $reserved = @('--target','-target','--repo-root','-reporoot','--pack','-pack')
  foreach ($arg in @($RemainingArgs)) {
    $lower = ([string]$arg).ToLowerInvariant()
    foreach ($flag in $reserved) {
      if ($lower -eq $flag -or $lower.StartsWith($flag + '=')) { throw "parallel argument is reserved for the trusted wrapper: $arg" }
    }
  }
  $runtime = Test-RekitGoRuntime -RepoRoot $RepoRoot
  $parallelArgs = @('run', './cmd/rekit', 'parallel', '--target', $caseRoot, '--repo-root', $RepoRoot, '--pack', $Pack)
  if ($Force) { $parallelArgs += '--force' }
  if ($WhatIf) { $parallelArgs += '--what-if' }
  if (-not [string]::IsNullOrWhiteSpace($DiffPath)) { throw 'parallel does not support -DiffPath; use review packet.json and summary.md outputs instead.' }
  if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir)) { $parallelArgs += @('--review-output-dir', $ReviewOutputDir) }
  if (-not [string]::IsNullOrWhiteSpace($PacketPath)) { $parallelArgs += @('--packet-path', $PacketPath) }
  if ($RemainingArgs.Count -gt 0) { $parallelArgs += $RemainingArgs }
  Push-Location $runtime.Root
  try {
    & go @parallelArgs
    if ($LASTEXITCODE -ne 0) { throw "rekit parallel failed with exit code $LASTEXITCODE" }
  } finally {
    Pop-Location
  }
}
