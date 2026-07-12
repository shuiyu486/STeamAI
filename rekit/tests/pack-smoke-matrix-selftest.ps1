param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Matrix = Join-Path $ScriptDir 'pack-smoke-matrix.ps1'

function Invoke-MatrixSelftest {
  param(
    [Parameter(Mandatory=$true)][string[]]$Arguments,
    [int[]]$AllowedExitCodes = @(0)
  )

  $oldEap = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $Matrix @Arguments 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
  } finally {
    $ErrorActionPreference = $oldEap
  }

  if ($AllowedExitCodes -notcontains $exitCode) { throw "unexpected matrix exit code $exitCode; output:`n$output" }
  [pscustomobject]@{
    exitCode = $exitCode
    output = $output
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

$discoveryText = Invoke-MatrixSelftest -Arguments @('-DiscoveryOnly')
Assert-ContainsText -Text $discoveryText.output -Expected 'pack smoke discovery ok (8 skeleton packs)' -Label 'discovery text'
Assert-ContainsText -Text $discoveryText.output -Expected 'excluded from skeleton smoke matrix:' -Label 'discovery excluded text'

$discoveryJson = (Invoke-MatrixSelftest -Arguments @('-DiscoveryOnly','-Format','json')).output | ConvertFrom-Json
if ([string]$discoveryJson.command -ne 'pack-smoke-discovery' -or [bool]$discoveryJson.isMutation -or -not [bool]$discoveryJson.ok -or [int]$discoveryJson.expectedSkeletonPackCount -ne 8 -or [int]$discoveryJson.matrixPackCount -ne 8) {
  throw "unexpected discovery JSON: $($discoveryJson | ConvertTo-Json -Depth 20)"
}
foreach ($field in @('missingSmokePacks','extraMatrixPacks','orphanWrapperPacks','missingScriptPacks')) {
  if (@($discoveryJson.$field).Count -ne 0) { throw "unexpected discovery $field rows: $($discoveryJson | ConvertTo-Json -Depth 20)" }
}

$matrixJson = (Invoke-MatrixSelftest -Arguments @('-WorkRoot',$WorkRoot,'-Packs','web-security,generic-binary-re','-Format','json')).output | ConvertFrom-Json
if ([string]$matrixJson.command -ne 'pack-smoke-matrix' -or [bool]$matrixJson.isMutation -or -not [bool]$matrixJson.ok -or [int]$matrixJson.packCount -ne 2 -or [int]$matrixJson.failedCount -ne 0 -or @($matrixJson.results).Count -ne 2) {
  throw "unexpected matrix JSON: $($matrixJson | ConvertTo-Json -Depth 20)"
}
foreach ($row in @($matrixJson.results)) {
  if (-not [bool]$row.success -or [int]$row.exitCode -ne 0 -or [string]::IsNullOrWhiteSpace([string]$row.output)) {
    throw "unexpected matrix JSON row: $($matrixJson | ConvertTo-Json -Depth 20)"
  }
}

$dedupJson = (Invoke-MatrixSelftest -Arguments @('-WorkRoot',$WorkRoot,'-Packs','web-security,web-security','-Format','json')).output | ConvertFrom-Json
if (-not [bool]$dedupJson.ok -or [int]$dedupJson.packCount -ne 1 -or @($dedupJson.results).Count -ne 1 -or [string]$dedupJson.packs[0] -ne 'web-security') {
  throw "unexpected dedup matrix JSON: $($dedupJson | ConvertTo-Json -Depth 20)"
}

$text = Invoke-MatrixSelftest -Arguments @('-WorkRoot',$WorkRoot,'-Packs','web-security,generic-binary-re')
Assert-ContainsText -Text $text.output -Expected 'pack smoke matrix running: web-security' -Label 'matrix text running'
Assert-ContainsText -Text $text.output -Expected 'generic-binary-re pack smoke ok' -Label 'matrix text smoke output'
Assert-ContainsText -Text $text.output -Expected 'pack smoke matrix ok (2 packs)' -Label 'matrix text summary'

$unknown = Invoke-MatrixSelftest -Arguments @('-Packs','does-not-exist') -AllowedExitCodes @(1)
Assert-ContainsText -Text $unknown.output -Expected "unknown pack smoke 'does-not-exist'" -Label 'unknown pack guard'

$global:LASTEXITCODE = 0
'pack smoke matrix selftest ok'
