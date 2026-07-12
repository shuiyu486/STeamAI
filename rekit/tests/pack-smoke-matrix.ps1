param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string[]]$Packs = @('web-security','malware-analysis','vuln-research','ctf','unpack-pe','ollvm','android-native','generic-binary-re'),
  [switch]$FailFast,
  [ValidateSet('text','json')][string]$Format = 'text'
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackSmokeScripts = [ordered]@{
  'web-security' = 'web-security-pack-smoke.ps1'
  'malware-analysis' = 'malware-analysis-pack-smoke.ps1'
  'vuln-research' = 'vuln-research-pack-smoke.ps1'
  'ctf' = 'ctf-pack-smoke.ps1'
  'unpack-pe' = 'unpack-pe-pack-smoke.ps1'
  'ollvm' = 'ollvm-pack-smoke.ps1'
  'android-native' = 'android-native-pack-smoke.ps1'
  'generic-binary-re' = 'generic-binary-re-pack-smoke.ps1'
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
