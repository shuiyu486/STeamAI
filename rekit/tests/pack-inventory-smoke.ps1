param()

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
$RepoRoot = Split-Path -Parent $RekitRoot
$Rekit = Join-Path $RekitRoot 'rekit.ps1'

function Invoke-RekitSmoke {
  param(
    [Parameter(Mandatory=$true)][string[]]$Arguments,
    [int[]]$AllowedExitCodes = @(0),
    [hashtable]$Env = @{}
  )
  $oldValues = @{}
  foreach ($key in $Env.Keys) {
    $oldValues[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
    [Environment]::SetEnvironmentVariable($key, [string]$Env[$key], 'Process')
  }
  $oldEap = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $Rekit @Arguments 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
  } finally {
    $ErrorActionPreference = $oldEap
    foreach ($key in $Env.Keys) { [Environment]::SetEnvironmentVariable($key, $oldValues[$key], 'Process') }
  }
  if ($AllowedExitCodes -notcontains $exitCode) { throw "unexpected exit code $exitCode; output:`n$output" }
  return $output
}

function Invoke-GoRekitSmoke {
  param([Parameter(Mandatory=$true)][string[]]$Arguments)
  Push-Location $RepoRoot
  $oldEap = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & go run ./cmd/rekit -- @Arguments 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
    if ($exitCode -ne 0) { throw "go rekit unexpected exit code $exitCode; output:`n$output" }
    return $output
  } finally {
    $ErrorActionPreference = $oldEap
    Pop-Location
  }
}

function Assert-ContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Expected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text -notlike "*$Expected*") { throw "$Label missing expected text '$Expected'. Output:`n$Text" }
}

function Assert-NotContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Unexpected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text -like "*$Unexpected*") { throw "$Label contained unexpected text '$Unexpected'. Output:`n$Text" }
}

function Assert-PackRow {
  param(
    [Parameter(Mandatory=$true)][string]$Text,
    [Parameter(Mandatory=$true)][string]$Pack,
    [Parameter(Mandatory=$true)][string]$Maturity,
    [Parameter(Mandatory=$true)][string]$Authority,
    [Parameter(Mandatory=$true)][string]$Managed,
    [Parameter(Mandatory=$true)][string]$Tooling
  )
  $line = @($Text -split "`r?`n" | Where-Object { $_ -like "$Pack`t*" })
  if ($line.Count -ne 1) { throw "expected one row for $Pack; output:`n$Text" }
  $cols = @([string]$line[0] -split "`t")
  if ($cols.Count -lt 9) { throw "pack row has too few columns for ${Pack}: $($line[0])" }
  if ($cols[1] -ne $Maturity -or $cols[2] -ne 'ok' -or $cols[3] -ne '2' -or $cols[4] -ne $Managed -or $cols[5] -ne $Tooling -or $cols[6] -ne $Authority) {
    throw "unexpected row for ${Pack}: $($line[0])"
  }
}

function Assert-PackSchemaRow {
  param(
    [Parameter(Mandatory=$true)][string]$Text,
    [Parameter(Mandatory=$true)][string]$Pack,
    [Parameter(Mandatory=$true)][string]$Maturity,
    [Parameter(Mandatory=$true)][string]$Schema,
    [Parameter(Mandatory=$true)][string]$ErrorText
  )
  $line = @($Text -split "`r?`n" | Where-Object { $_ -like "$Pack`t*" })
  if ($line.Count -ne 1) { throw "expected one row for $Pack; output:`n$Text" }
  $cols = @([string]$line[0] -split "`t")
  if ($cols.Count -lt 9) { throw "pack row has too few columns for ${Pack}: $($line[0])" }
  if ($cols[1] -ne $Maturity -or $cols[2] -ne $Schema) { throw "unexpected schema row for ${Pack}: $($line[0])" }
  Assert-ContainsText -Text $Text -Expected $ErrorText -Label "schema error for $Pack"
}

function New-TransientPackManifest {
  param(
    [Parameter(Mandatory=$true)][string]$Pack,
    [string]$MaturityLine = ''
  )
  $packRoot = Join-Path $RepoRoot ("packs\" + $Pack)
  [System.IO.Directory]::CreateDirectory($packRoot) | Out-Null
  $lines = @(
    'schemaVersion: 1',
    "name: $Pack",
    'version: 0.1.0',
    'description: maturity inventory smoke pack'
  )
  if (-not [string]::IsNullOrWhiteSpace($MaturityLine)) { $lines += $MaturityLine }
  $lines += @(
    '',
    'managedFiles:',
    '  - references/test/README.md'
  )
  [System.IO.File]::WriteAllText((Join-Path $packRoot 'manifest.yml'), (($lines -join "`n") + "`n"), [System.Text.Encoding]::UTF8)
  return $packRoot
}

$goOut = Invoke-GoRekitSmoke -Arguments @('-Command','packs')
$psOut = Invoke-RekitSmoke -Arguments @('-Command','packs')
$facadeOut = Invoke-RekitSmoke -Arguments @('-Command','packs') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }

foreach ($out in @($goOut,$psOut,$facadeOut)) {
  if ($out -notlike "pack`t*") { throw "packs output missing header:`n$out" }
  Assert-PackRow -Text $out -Pack '_template' -Maturity 'template' -Authority 'main' -Managed '4' -Tooling '2'
  Assert-PackRow -Text $out -Pack 'vmp-re' -Maturity 'mature' -Authority 'devirt-main' -Managed '7' -Tooling '11'
  Assert-PackRow -Text $out -Pack 'web-security' -Maturity 'skeleton' -Authority 'main' -Managed '4' -Tooling '4'
}

$transientPacks = @()
try {
  $suffix = [Guid]::NewGuid().ToString('N')
  $missingPack = "_maturity_missing_$suffix"
  $invalidPack = "_maturity_invalid_$suffix"
  $transientPacks += (New-TransientPackManifest -Pack $missingPack)
  $transientPacks += (New-TransientPackManifest -Pack $invalidPack -MaturityLine 'maturity: preview')

  $goMaturityOut = Invoke-GoRekitSmoke -Arguments @('-Command','packs')
  $psMaturityOut = Invoke-RekitSmoke -Arguments @('-Command','packs')
  foreach ($out in @($goMaturityOut,$psMaturityOut)) {
    Assert-PackSchemaRow -Text $out -Pack $missingPack -Maturity 'missing' -Schema 'error' -ErrorText 'maturity is missing'
    Assert-PackSchemaRow -Text $out -Pack $invalidPack -Maturity 'preview' -Schema 'error' -ErrorText 'maturity has unsupported value'
  }
} finally {
  foreach ($packRoot in $transientPacks) {
    if (Test-Path -LiteralPath $packRoot) { Remove-Item -LiteralPath $packRoot -Recurse -Force -Confirm:$false }
  }
}

$sentinel = Join-Path ([System.IO.Path]::GetTempPath()) ("rekit-pack-inventory-sentinel-$([Guid]::NewGuid().ToString('N')).cmd")
try {
  [System.IO.File]::WriteAllText($sentinel, "@echo off`r`necho sentinel-go-packs %*`r`n", [System.Text.Encoding]::ASCII)
  $sentinelOut = Invoke-RekitSmoke -Arguments @('-Command','packs') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $sentinel }
  Assert-ContainsText -Text $sentinelOut -Expected 'sentinel-go-packs' -Label 'facade packs go delegation sentinel'
  Assert-ContainsText -Text $sentinelOut -Expected '-Command packs' -Label 'facade packs delegated command args'

  $disabledOut = Invoke-RekitSmoke -Arguments @('-Command','packs') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $sentinel }
  Assert-NotContainsText -Text $disabledOut -Unexpected 'sentinel-go-packs' -Label 'facade packs disable fallback'
  Assert-PackRow -Text $disabledOut -Pack 'web-security' -Maturity 'skeleton' -Authority 'main' -Managed '4' -Tooling '4'
} finally {
  if (Test-Path -LiteralPath $sentinel) { Remove-Item -LiteralPath $sentinel -Force -Confirm:$false }
}

'pack inventory smoke ok'
