param()

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent (Split-Path -Parent $ScriptDir)
$CatalogPath = Join-Path $ScriptDir 'catalog.json'
$MatrixPath = Join-Path $ScriptDir 'pack-smoke-matrix.ps1'

function Assert-True {
  param(
    [Parameter(Mandatory=$true)][bool]$Condition,
    [Parameter(Mandatory=$true)][string]$Message
  )
  if (-not $Condition) { throw $Message }
}

function Assert-NonEmptyString {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Value,
    [Parameter(Mandatory=$true)][string]$Label
  )
  Assert-True -Condition (-not [string]::IsNullOrWhiteSpace($Value)) -Message "$Label must be non-empty"
}

function Get-ScriptLeafFromCatalogCommand {
  param([Parameter(Mandatory=$true)][string]$Command)
  return @($Command -split '\s+')[0]
}

function Invoke-DiscoveryJson {
  $oldEap = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $MatrixPath -DiscoveryOnly -Format json 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
  } finally {
    $ErrorActionPreference = $oldEap
  }
  if ($exitCode -ne 0) { throw "pack smoke discovery failed with exit code $exitCode; output:`n$output" }
  return ($output | ConvertFrom-Json)
}

Assert-True -Condition (Test-Path -LiteralPath $CatalogPath -PathType Leaf) -Message 'catalog.json is missing'
$catalog = Get-Content -LiteralPath $CatalogPath -Raw | ConvertFrom-Json

Assert-True -Condition ([int]$catalog.schemaVersion -eq 1) -Message 'catalog schemaVersion must be 1'
Assert-NonEmptyString -Value ([string]$catalog.description) -Label 'description'
Assert-NonEmptyString -Value ([string]$catalog.defaultWorkRoot) -Label 'defaultWorkRoot'
Assert-True -Condition (@($catalog.globalBoundaries).Count -ge 1) -Message 'globalBoundaries must be non-empty'
Assert-True -Condition (@($catalog.recommendedMinimum).Count -ge 1) -Message 'recommendedMinimum must be non-empty'

$tests = @($catalog.tests)
Assert-True -Condition ($tests.Count -ge 1) -Message 'tests must be non-empty'

$validCategories = @(
  'facade',
  'inventory',
  'catalog',
  'pack-matrix',
  'pack-helper',
  'pack-smoke',
  'case-scaffold',
  'subagents',
  'sync-promote',
  'agent-team',
  'gate-ledger',
  'workstream'
)
$categorySet = @{}
foreach ($category in $validCategories) { $categorySet[$category] = $true }

$scriptFiles = Get-ChildItem -LiteralPath $ScriptDir -Filter '*.ps1' -File | ForEach-Object { $_.Name }
$scriptFileSet = @{}
foreach ($scriptFile in @($scriptFiles)) { $scriptFileSet[[string]$scriptFile] = $true }

$ids = @{}
$catalogScriptSet = @{}
foreach ($test in $tests) {
  $id = [string]$test.id
  Assert-NonEmptyString -Value $id -Label 'test.id'
  Assert-True -Condition ($id -match '^[a-z0-9][a-z0-9-]*$') -Message "invalid test id: $id"
  Assert-True -Condition (-not $ids.ContainsKey($id)) -Message "duplicate test id: $id"
  $ids[$id] = $true

  $script = [string]$test.script
  $category = [string]$test.category
  Assert-NonEmptyString -Value $script -Label "$id script"
  Assert-NonEmptyString -Value $category -Label "$id category"
  Assert-True -Condition $categorySet.ContainsKey($category) -Message "unexpected category for ${id}: $category"
  Assert-NonEmptyString -Value ([string]$test.purpose) -Label "$id purpose"
  Assert-NonEmptyString -Value ([string]$test.riskBoundary) -Label "$id riskBoundary"
  Assert-True -Condition (@($test.recommendedFor).Count -ge 1) -Message "$id recommendedFor must be non-empty"
  Assert-True -Condition ($null -ne $test.supportsWorkRoot) -Message "$id supportsWorkRoot is missing"
  Assert-True -Condition ($null -ne $test.supportsCaseRoot) -Message "$id supportsCaseRoot is missing"
  Assert-True -Condition (@($test.relatedDocs).Count -ge 1) -Message "$id relatedDocs must be non-empty"

  $scriptLeaf = Get-ScriptLeafFromCatalogCommand -Command $script
  if ($scriptLeaf -like '*.ps1') {
    Assert-True -Condition (Test-Path -LiteralPath (Join-Path $ScriptDir $scriptLeaf) -PathType Leaf) -Message "$id script does not exist: $scriptLeaf"
    $catalogScriptSet[$scriptLeaf] = $true
  }

  foreach ($doc in @($test.relatedDocs)) {
    $docPath = Join-Path $RepoRoot ([string]$doc)
    Assert-True -Condition (Test-Path -LiteralPath $docPath -PathType Leaf) -Message "$id related doc does not exist: $doc"
  }

  if ($category -eq 'pack-smoke') {
    Assert-NonEmptyString -Value ([string]$test.pack) -Label "$id pack"
    Assert-True -Condition ($script -eq ("$($test.pack)-pack-smoke.ps1")) -Message "$id script must match pack name"
    Assert-True -Condition ([bool]$test.supportsWorkRoot) -Message "$id must support WorkRoot"
    Assert-True -Condition (-not [bool]$test.supportsCaseRoot) -Message "$id must not support CaseRoot"
  }
}

foreach ($requiredId in @('facade-smoke','pack-inventory-smoke','catalog-smoke','pack-smoke-matrix-selftest','pack-smoke-matrix-discovery','pack-smoke-matrix','pack-smoke-lib')) {
  Assert-True -Condition $ids.ContainsKey($requiredId) -Message "catalog missing required test id: $requiredId"
}

foreach ($scriptFile in $scriptFileSet.Keys) {
  Assert-True -Condition $catalogScriptSet.ContainsKey($scriptFile) -Message "catalog missing script entry: $scriptFile"
}
foreach ($scriptLeaf in $catalogScriptSet.Keys) {
  Assert-True -Condition $scriptFileSet.ContainsKey($scriptLeaf) -Message "catalog references unknown script: $scriptLeaf"
}

$discovery = Invoke-DiscoveryJson
Assert-True -Condition ([bool]$discovery.ok) -Message "pack smoke discovery is not ok: $($discovery | ConvertTo-Json -Depth 20)"
$expectedPackSet = @{}
foreach ($pack in @($discovery.expectedSmokePacks)) { $expectedPackSet[[string]$pack] = $true }
$catalogPackSet = @{}
foreach ($entry in @($tests | Where-Object { [string]$_.category -eq 'pack-smoke' })) { $catalogPackSet[[string]$entry.pack] = $true }

foreach ($pack in $expectedPackSet.Keys) {
  Assert-True -Condition $catalogPackSet.ContainsKey($pack) -Message "catalog missing pack smoke entry for expected pack: $pack"
}
foreach ($pack in $catalogPackSet.Keys) {
  Assert-True -Condition $expectedPackSet.ContainsKey($pack) -Message "catalog has pack smoke entry not in expected discovery: $pack"
}

'catalog smoke ok'
