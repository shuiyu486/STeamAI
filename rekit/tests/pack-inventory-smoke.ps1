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

function Assert-PackJson {
  param(
    [Parameter(Mandatory=$true)]$Inventory,
    [Parameter(Mandatory=$true)][string]$Pack,
    [Parameter(Mandatory=$true)][string]$Maturity,
    [Parameter(Mandatory=$true)][string]$Authority,
    [Parameter(Mandatory=$true)][int]$Managed,
    [Parameter(Mandatory=$true)][int]$Tooling
  )
  if ([string]$Inventory.command -ne 'packs' -or [int]$Inventory.schemaVersion -ne 1 -or [bool]$Inventory.isMutation -or [int]$Inventory.packCount -ne @($Inventory.packs).Count) {
    throw "unexpected packs JSON envelope: $($Inventory | ConvertTo-Json -Depth 20)"
  }
  $rows = @($Inventory.packs | Where-Object { [string]$_.id -eq $Pack })
  if ($rows.Count -ne 1) { throw "expected one JSON row for ${Pack}: $($Inventory | ConvertTo-Json -Depth 20)" }
  $row = $rows[0]
  if ([string]$row.maturity -ne $Maturity -or -not [bool]$row.schemaValid -or [string]$row.defaultAuthorityLane -ne $Authority -or [int]$row.managedFiles -ne $Managed -or [int]$row.toolingFiles -ne $Tooling) {
    throw "unexpected JSON row for ${Pack}: $($row | ConvertTo-Json -Depth 20)"
  }
}

function Assert-StatusJson {
  param(
    [Parameter(Mandatory=$true)]$Status,
    [Parameter(Mandatory=$true)][string]$Mode
  )
  if ([string]$Status.command -ne 'status' -or [int]$Status.schemaVersion -ne 1 -or [bool]$Status.isMutation -or [string]$Status.mode -ne $Mode) {
    throw "unexpected status JSON envelope: $($Status | ConvertTo-Json -Depth 20)"
  }
  if ([string]::IsNullOrWhiteSpace([string]$Status.runtimeRoot) -or [string]::IsNullOrWhiteSpace([string]$Status.templateRoot) -or [string]::IsNullOrWhiteSpace([string]$Status.target)) {
    throw "status JSON roots are incomplete: $($Status | ConvertTo-Json -Depth 20)"
  }
  if ($Mode -eq 'kit') {
    if ($null -ne $Status.case -or $null -eq $Status.manifest -or [string]$Status.pack -ne 'vmp-re' -or [int]$Status.manifest.managedFiles -ne 7 -or [int]$Status.manifest.promoteFiles -ne 7 -or [int]$Status.manifest.toolingFiles -ne 11) {
      throw "unexpected kit status JSON: $($Status | ConvertTo-Json -Depth 20)"
    }
  }
}

function Assert-DoctorJson {
  param(
    [Parameter(Mandatory=$true)]$Doctor,
    [Parameter(Mandatory=$true)][string]$Command
  )
  if ([string]$Doctor.command -ne $Command -or [int]$Doctor.schemaVersion -ne 1 -or [bool]$Doctor.isMutation -or [string]$Doctor.mode -ne 'pack' -or -not [bool]$Doctor.valid -or [string]$Doctor.summary -ne 'pack validation ok') {
    throw "unexpected $Command JSON envelope: $($Doctor | ConvertTo-Json -Depth 20)"
  }
  $rows = @($Doctor.rows)
  if ($rows.Count -lt 1 -or [string]::IsNullOrWhiteSpace([string]$rows[0].file) -or [int64]$rows[0].bytes -le 0 -or [int64]$rows[0].limit -le 0) {
    throw "unexpected $Command JSON rows: $($Doctor | ConvertTo-Json -Depth 20)"
  }
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

$goStatusJson = Invoke-GoRekitSmoke -Arguments @('-Command','status','-Format','json') | ConvertFrom-Json
$psStatusJson = Invoke-RekitSmoke -Arguments @('-Command','status','-Format','json') | ConvertFrom-Json
$facadeStatusJson = Invoke-RekitSmoke -Arguments @('-Command','status','-Format','json') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' } | ConvertFrom-Json
foreach ($json in @($goStatusJson,$psStatusJson,$facadeStatusJson)) {
  Assert-StatusJson -Status $json -Mode 'kit'
}

$goDoctorJson = Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Format','json') | ConvertFrom-Json
$psDoctorJson = Invoke-RekitSmoke -Arguments @('-Command','doctor','-Format','json') | ConvertFrom-Json
$facadeDoctorJson = Invoke-RekitSmoke -Arguments @('-Command','doctor','-Format','json') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' } | ConvertFrom-Json
foreach ($json in @($goDoctorJson,$psDoctorJson,$facadeDoctorJson)) {
  Assert-DoctorJson -Doctor $json -Command 'doctor'
}

$goValidateJson = Invoke-GoRekitSmoke -Arguments @('-Command','validate','-Format','json') | ConvertFrom-Json
$psValidateJson = Invoke-RekitSmoke -Arguments @('-Command','validate','-Format','json') | ConvertFrom-Json
$facadeValidateJson = Invoke-RekitSmoke -Arguments @('-Command','validate','-Format','json') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' } | ConvertFrom-Json
foreach ($json in @($goValidateJson,$psValidateJson,$facadeValidateJson)) {
  Assert-DoctorJson -Doctor $json -Command 'validate'
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

$goJson = Invoke-GoRekitSmoke -Arguments @('-Command','packs','-Format','json') | ConvertFrom-Json
$psJson = Invoke-RekitSmoke -Arguments @('-Command','packs','-Format','json') | ConvertFrom-Json
$facadeJson = Invoke-RekitSmoke -Arguments @('-Command','packs','-Format','json') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' } | ConvertFrom-Json
foreach ($json in @($goJson,$psJson,$facadeJson)) {
  Assert-PackJson -Inventory $json -Pack '_template' -Maturity 'template' -Authority 'main' -Managed 4 -Tooling 2
  Assert-PackJson -Inventory $json -Pack 'vmp-re' -Maturity 'mature' -Authority 'devirt-main' -Managed 7 -Tooling 11
  Assert-PackJson -Inventory $json -Pack 'web-security' -Maturity 'skeleton' -Authority 'main' -Managed 4 -Tooling 4
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
  $goMaturityJson = Invoke-GoRekitSmoke -Arguments @('-Command','packs','-Format','json') | ConvertFrom-Json
  $psMaturityJson = Invoke-RekitSmoke -Arguments @('-Command','packs','-Format','json') | ConvertFrom-Json
  foreach ($json in @($goMaturityJson,$psMaturityJson)) {
    $missingRow = @($json.packs | Where-Object { [string]$_.id -eq $missingPack })[0]
    $invalidRow = @($json.packs | Where-Object { [string]$_.id -eq $invalidPack })[0]
    if ([string]$missingRow.maturity -ne 'missing' -or [bool]$missingRow.schemaValid -or [string]$missingRow.error -notlike '*maturity is missing*') { throw "unexpected missing maturity JSON row: $($missingRow | ConvertTo-Json -Depth 20)" }
    if ([string]$invalidRow.maturity -ne 'preview' -or [bool]$invalidRow.schemaValid -or [string]$invalidRow.error -notlike '*maturity has unsupported value*') { throw "unexpected invalid maturity JSON row: $($invalidRow | ConvertTo-Json -Depth 20)" }
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

  $sentinelJsonOut = Invoke-RekitSmoke -Arguments @('-Command','packs','-Format','json') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $sentinel }
  Assert-ContainsText -Text $sentinelJsonOut -Expected '-Format json' -Label 'facade packs format delegation args'

  $sentinelStatusJsonOut = Invoke-RekitSmoke -Arguments @('-Command','status','-Format','json') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $sentinel }
  Assert-ContainsText -Text $sentinelStatusJsonOut -Expected '-Format json' -Label 'facade status format delegation args'

  $sentinelDoctorJsonOut = Invoke-RekitSmoke -Arguments @('-Command','doctor','-Format','json') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $sentinel }
  Assert-ContainsText -Text $sentinelDoctorJsonOut -Expected '-Format json' -Label 'facade doctor format delegation args'

  $sentinelValidateJsonOut = Invoke-RekitSmoke -Arguments @('-Command','validate','-Format','json') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $sentinel }
  Assert-ContainsText -Text $sentinelValidateJsonOut -Expected '-Format json' -Label 'facade validate format delegation args'

  $disabledOut = Invoke-RekitSmoke -Arguments @('-Command','packs') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $sentinel }
  Assert-NotContainsText -Text $disabledOut -Unexpected 'sentinel-go-packs' -Label 'facade packs disable fallback'
  Assert-PackRow -Text $disabledOut -Pack 'web-security' -Maturity 'skeleton' -Authority 'main' -Managed '4' -Tooling '4'
} finally {
  if (Test-Path -LiteralPath $sentinel) { Remove-Item -LiteralPath $sentinel -Force -Confirm:$false }
}

'pack inventory smoke ok'
