param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string[]]$Packs = @('web-security','malware-analysis','vuln-research','ctf','unpack-pe','ollvm','android-native'),
  [switch]$FailFast,
  [ValidateSet('text','json')][string]$Format = 'text',
  [switch]$DiscoveryOnly
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
$Rekit = Join-Path $RekitRoot 'rekit.ps1'
$PackSmokeScripts = [ordered]@{
  'web-security' = 'web-security-pack-smoke.ps1'
  'malware-analysis' = 'malware-analysis-pack-smoke.ps1'
  'vuln-research' = 'vuln-research-pack-smoke.ps1'
  'ctf' = 'ctf-pack-smoke.ps1'
  'unpack-pe' = 'unpack-pe-pack-smoke.ps1'
  'ollvm' = 'ollvm-pack-smoke.ps1'
  'android-native' = 'android-native-pack-smoke.ps1'
}

function Expand-PackSmokeSelection {
  param([string[]]$Selection)

  $expanded = New-Object System.Collections.Generic.List[string]
  foreach ($entry in $Selection) {
    foreach ($name in ([string]$entry -split ',')) {
      $trimmed = $name.Trim()
      if ([string]::IsNullOrWhiteSpace($trimmed)) { continue }
      if ($trimmed -eq 'all') {
        foreach ($known in $PackSmokeScripts.Keys) { $expanded.Add([string]$known) }
        continue
      }
      $expanded.Add($trimmed)
    }
  }

  if ($expanded.Count -eq 0) { throw 'no pack smoke selected' }

  $seen = @{}
  $deduped = New-Object System.Collections.Generic.List[string]
  foreach ($pack in $expanded) {
    if (-not $PackSmokeScripts.Contains($pack)) {
      throw "unknown pack smoke '$pack'; known packs: $($PackSmokeScripts.Keys -join ', ')"
    }
    if ($seen.ContainsKey($pack)) { continue }
    $seen[$pack] = $true
    $deduped.Add($pack)
  }
  return @($deduped)
}

function Invoke-PackInventoryJson {
  $oldEap = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $Rekit -Command packs -Format json 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
  } finally {
    $ErrorActionPreference = $oldEap
  }
  if ($exitCode -ne 0) { throw "packs inventory unexpected exit code $exitCode; output:`n$output" }
  return ($output | ConvertFrom-Json)
}

function Invoke-PackSmokeScript {
  param(
    [Parameter(Mandatory=$true)][string]$Pack,
    [Parameter(Mandatory=$true)][string]$ScriptPath
  )

  $oldEap = $ErrorActionPreference
  $watch = [System.Diagnostics.Stopwatch]::StartNew()
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $ScriptPath -WorkRoot $WorkRoot 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
  } finally {
    $watch.Stop()
    $ErrorActionPreference = $oldEap
  }

  [pscustomobject]@{
    pack = $Pack
    script = $ScriptPath
    success = ($exitCode -eq 0)
    exitCode = $exitCode
    elapsedMs = [int]$watch.ElapsedMilliseconds
    output = $output.Trim()
  }
}

function New-PackSmokeMatrixEnvelope {
  param()

  $selected = @($selectedPacks)
  $resultItems = @($results | ForEach-Object { $_ })
  $failureItems = @($failures | ForEach-Object { $_ })

  [pscustomobject]@{
    schemaVersion = 1
    command = 'pack-smoke-matrix'
    isMutation = $false
    workRoot = $WorkRoot
    failFast = [bool]$FailFast
    packCount = [int]$selected.Count
    failedCount = [int]$failureItems.Count
    ok = ([int]$failureItems.Count -eq 0)
    packs = @($selected)
    results = @($resultItems)
  }
}

function New-PackSmokeDiscoveryEnvelope {
  $inventory = Invoke-PackInventoryJson
  $inventoryRows = @($inventory.packs)
  $matrixPacks = @($PackSmokeScripts.Keys | ForEach-Object { [string]$_ })
  $wrapperPacks = @(Get-ChildItem -LiteralPath $ScriptDir -Filter '*-pack-smoke.ps1' -File | ForEach-Object { $_.BaseName -replace '-pack-smoke$','' } | Sort-Object)

  $expectedSkeletonPacks = @($inventoryRows | Where-Object { [bool]$_.schemaValid -and [string]$_.maturity -eq 'skeleton' } | ForEach-Object { [string]$_.id } | Sort-Object)
  $productionSmokePacks = @('web-security')
  $invalidProductionSmokePacks = @()
  foreach ($productionPack in $productionSmokePacks) {
    $productionRows = @($inventoryRows | Where-Object { [string]$_.id -eq [string]$productionPack })
    if ($productionRows.Count -ne 1 -or -not [bool]$productionRows[0].schemaValid -or [string]$productionRows[0].maturity -ne 'mature') {
      $invalidProductionSmokePacks += [string]$productionPack
    }
  }
  $expectedSmokeSet = @{}
  foreach ($pack in $expectedSkeletonPacks + $productionSmokePacks) { $expectedSmokeSet[[string]$pack] = $true }
  $expectedSmokePacks = @($expectedSmokeSet.Keys | Sort-Object)
  $excludedPacks = @($inventoryRows | Where-Object { -not $expectedSmokeSet.ContainsKey([string]$_.id) } | ForEach-Object {
    $reason = 'not-retained-production-smoke'
    if (-not [bool]$_.schemaValid) { $reason = 'schema-invalid' }
    [pscustomobject]@{
      pack = [string]$_.id
      maturity = [string]$_.maturity
      schemaValid = [bool]$_.schemaValid
      reason = $reason
    }
  } | Sort-Object pack)

  $matrixSet = @{}
  foreach ($pack in $matrixPacks) { $matrixSet[$pack] = $true }

  $missingSmokePacks = @($expectedSmokePacks | Where-Object { -not $matrixSet.ContainsKey($_) } | Sort-Object)
  $extraMatrixPacks = @($matrixPacks | Where-Object { -not $expectedSmokeSet.ContainsKey($_) } | Sort-Object)
  $orphanWrapperPacks = @($wrapperPacks | Where-Object { -not $matrixSet.ContainsKey($_) } | Sort-Object)
  $missingScriptPacks = @($matrixPacks | Where-Object { -not (Test-Path -LiteralPath (Join-Path $ScriptDir ([string]$PackSmokeScripts[$_])) -PathType Leaf) } | Sort-Object)

  $ok = ($invalidProductionSmokePacks.Count -eq 0 -and $missingSmokePacks.Count -eq 0 -and $extraMatrixPacks.Count -eq 0 -and $orphanWrapperPacks.Count -eq 0 -and $missingScriptPacks.Count -eq 0)

  [pscustomobject]@{
    schemaVersion = 1
    command = 'pack-smoke-discovery'
    isMutation = $false
    ok = $ok
    inventoryPackCount = [int]$inventoryRows.Count
    expectedSkeletonPackCount = [int]$expectedSkeletonPacks.Count
    expectedProductionPackCount = [int]$productionSmokePacks.Count
    expectedSmokePackCount = [int]$expectedSmokePacks.Count
    matrixPackCount = [int]$matrixPacks.Count
    wrapperPackCount = [int]$wrapperPacks.Count
    expectedSkeletonPacks = @($expectedSkeletonPacks)
    productionSmokePacks = @($productionSmokePacks)
    expectedSmokePacks = @($expectedSmokePacks)
    invalidProductionSmokePacks = @($invalidProductionSmokePacks)
    matrixPacks = @($matrixPacks)
    wrapperPacks = @($wrapperPacks)
    excludedPacks = @($excludedPacks)
    missingSmokePacks = @($missingSmokePacks)
    extraMatrixPacks = @($extraMatrixPacks)
    orphanWrapperPacks = @($orphanWrapperPacks)
    missingScriptPacks = @($missingScriptPacks)
  }
}

function Write-PackSmokeDiscoveryText {
  param([Parameter(Mandatory=$true)]$Discovery)

  if ([bool]$Discovery.ok) {
    "pack smoke discovery ok ($($Discovery.expectedSkeletonPackCount) skeleton + $($Discovery.expectedProductionPackCount) production packs; $($Discovery.expectedSmokePackCount) total)"
  } else {
    "pack smoke discovery failed"
  }

  if (@($Discovery.invalidProductionSmokePacks).Count -gt 0) { "invalid production pack smoke: $(@($Discovery.invalidProductionSmokePacks) -join ', ')" }
  if (@($Discovery.missingSmokePacks).Count -gt 0) { "missing pack smoke: $(@($Discovery.missingSmokePacks) -join ', ')" }
  if (@($Discovery.extraMatrixPacks).Count -gt 0) { "extra matrix pack smoke: $(@($Discovery.extraMatrixPacks) -join ', ')" }
  if (@($Discovery.orphanWrapperPacks).Count -gt 0) { "orphan pack smoke wrapper: $(@($Discovery.orphanWrapperPacks) -join ', ')" }
  if (@($Discovery.missingScriptPacks).Count -gt 0) { "missing matrix script: $(@($Discovery.missingScriptPacks) -join ', ')" }

  $excluded = @($Discovery.excludedPacks | ForEach-Object { "$($_.pack):$($_.maturity):$($_.reason)" })
  if ($excluded.Count -gt 0) { "excluded from pack smoke matrix: $($excluded -join ', ')" }
}

if ($DiscoveryOnly) {
  $discovery = New-PackSmokeDiscoveryEnvelope
  if ($Format -eq 'json') {
    $discovery | ConvertTo-Json -Depth 20
    if (-not [bool]$discovery.ok) { exit 1 }
    exit 0
  }

  Write-PackSmokeDiscoveryText -Discovery $discovery
  if (-not [bool]$discovery.ok) { throw 'pack smoke discovery failed' }
  exit 0
}

if (-not (Test-Path -LiteralPath $WorkRoot -PathType Container)) { throw "missing WorkRoot: $WorkRoot" }
$selectedPacks = Expand-PackSmokeSelection -Selection $Packs

$results = New-Object System.Collections.Generic.List[object]
$failures = New-Object System.Collections.Generic.List[object]

foreach ($pack in $selectedPacks) {
  $scriptPath = Join-Path $ScriptDir ([string]$PackSmokeScripts[$pack])
  if (-not (Test-Path -LiteralPath $scriptPath -PathType Leaf)) { throw "missing pack smoke script for ${pack}: $scriptPath" }

  if ($Format -eq 'text') { "pack smoke matrix running: $pack" }
  $result = Invoke-PackSmokeScript -Pack $pack -ScriptPath $scriptPath
  $results.Add($result)

  if ($result.exitCode -ne 0) {
    $failures.Add($result)
    if ($Format -eq 'text') {
      "pack smoke matrix failed: $pack exit=$($result.exitCode) elapsedMs=$($result.elapsedMs)"
      $result.output
    }
    if ($FailFast) { break }
    continue
  }

  if ($Format -eq 'text') {
    "pack smoke matrix passed: $pack elapsedMs=$($result.elapsedMs)"
    if (-not [string]::IsNullOrWhiteSpace($result.output)) { $result.output }
  }
}

if ($Format -eq 'json') {
  New-PackSmokeMatrixEnvelope | ConvertTo-Json -Depth 20
  if ($failures.Count -gt 0) { exit 1 }
  exit 0
}

if ($failures.Count -gt 0) {
  $summary = ($failures | ForEach-Object { "$($_.pack) exit=$($_.exitCode)" }) -join '; '
  throw "pack smoke matrix failed for $($failures.Count) pack(s): $summary"
}

"pack smoke matrix ok ($($results.Count) packs)"
