param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string]$Pack = '_template'
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
$RepoRoot = Split-Path -Parent $RekitRoot
$Rekit = Join-Path $RekitRoot 'rekit.ps1'

function Invoke-RekitSmoke {
  param(
    [Parameter(Mandatory=$true)][string[]]$Arguments,
    [int[]]$AllowedExitCodes = @(0)
  )
  $oldEap = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $Rekit @Arguments 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
  } finally {
    $ErrorActionPreference = $oldEap
  }
  if ($AllowedExitCodes -notcontains $exitCode) {
    throw "unexpected exit code $exitCode; output:`n$output"
  }
  return $output
}

function Invoke-GoRekitSmoke {
  param([Parameter(Mandatory=$true)][string[]]$Arguments)
  Push-Location $RepoRoot
  try {
    $output = & go run ./cmd/rekit -- @Arguments | Out-String
    if ($LASTEXITCODE -ne 0) { throw "go rekit failed; output:`n$output" }
    return $output
  } finally {
    Pop-Location
  }
}

function Assert-EqualText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Actual,
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Expected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Actual -ne $Expected) { throw "$Label mismatch. Expected '$Expected', got '$Actual'" }
}

function Assert-ContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Expected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text -notlike "*$Expected*") { throw "$Label missing expected text '$Expected'. Output:`n$Text" }
}

function Get-ReviewItem {
  param(
    [Parameter(Mandatory=$true)]$Packet,
    [Parameter(Mandatory=$true)][string]$Path,
    [string]$Kind = ''
  )
  $items = @($Packet.items | Where-Object { [string]$_.path -eq $Path })
  if (-not [string]::IsNullOrWhiteSpace($Kind)) { $items = @($items | Where-Object { [string]$_.kind -eq $Kind }) }
  if ($items.Count -ne 1) { throw "expected exactly one review item for $Path/$Kind, got $($items.Count)" }
  return $items[0]
}

function Assert-ReviewAction {
  param(
    [Parameter(Mandatory=$true)]$Packet,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Kind,
    [Parameter(Mandatory=$true)][string]$Action,
    [Parameter(Mandatory=$true)][string]$Label
  )
  $item = Get-ReviewItem -Packet $Packet -Path $Path -Kind $Kind
  Assert-EqualText -Actual ([string]$item.action) -Expected $Action -Label $Label
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "sync-review-parity-$suffix"
try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"sync-parity-$suffix") | Out-Null

  $managedDrift = Join-Path $caseRoot 'references\template\README.md'
  $managedMissing = Join-Path $caseRoot 'references\template\workflow-template.md'
  $templateLocal = Join-Path $caseRoot 'references\template\task-handoff.md'
  $blockHost = Join-Path $caseRoot 'CLAUDE.local.md'

  [System.IO.File]::WriteAllText($managedDrift, "# Local drift`r`n`r`nchanged after initial sync`r`n", [System.Text.UTF8Encoding]::new($false))
  Remove-Item -LiteralPath $managedMissing -Force -Confirm:$false
  [System.IO.File]::WriteAllText($templateLocal, "# Local handoff`r`n`r`nkeep this local file`r`n", [System.Text.UTF8Encoding]::new($false))
  $block = "Case preface`r`n`r`n<!-- BEGIN template-pack:router -->`r`nold managed block`r`n<!-- END template-pack:router -->`r`n`r`nCase suffix`r`n"
  [System.IO.File]::WriteAllText($blockHost, $block, [System.Text.UTF8Encoding]::new($false))

  $psReview = Join-Path $caseRoot '.rekit\reviews\ps-sync-review'
  $goReview = Join-Path $caseRoot '.rekit\reviews\go-sync-review'

  Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$caseRoot,'-Pack',$Pack,'-ReviewOutputDir',$psReview) | Out-Null
  Invoke-GoRekitSmoke -Arguments @('-Command','sync','-Target',$caseRoot,'-Pack',$Pack,'-ReviewOutputDir',$goReview) | Out-Null

  $psPacketPath = Join-Path $psReview 'packet.json'
  $goPacketPath = Join-Path $goReview 'packet.json'
  if (-not (Test-Path -LiteralPath $psPacketPath)) { throw "missing PowerShell review packet: $psPacketPath" }
  if (-not (Test-Path -LiteralPath $goPacketPath)) { throw "missing Go review packet: $goPacketPath" }
  $psPacket = Get-Content -LiteralPath $psPacketPath -Raw | ConvertFrom-Json
  $goPacket = Get-Content -LiteralPath $goPacketPath -Raw | ConvertFrom-Json

  foreach ($packetInfo in @(@{ label = 'PowerShell'; packet = $psPacket }, @{ label = 'Go'; packet = $goPacket })) {
    $label = [string]$packetInfo.label
    $packet = $packetInfo.packet
    Assert-ReviewAction -Packet $packet -Path 'references/template/README.md' -Kind 'managed-file' -Action 'overwrite-with-backup' -Label "$label managed drift action"
    Assert-ReviewAction -Packet $packet -Path 'references/template/workflow-template.md' -Kind 'managed-file' -Action 'create-managed-file' -Label "$label missing managed action"
    Assert-ReviewAction -Packet $packet -Path 'references/template/task-handoff.md' -Kind 'template-file' -Action 'skip-existing-local-file' -Label "$label local template action"
    Assert-ReviewAction -Packet $packet -Path 'CLAUDE.local.md' -Kind 'managed-block' -Action 'replace-managed-block' -Label "$label managed block action"
  }

  $psDiff = [System.IO.File]::ReadAllText((Join-Path $psReview 'diffs\combined.diff'), [System.Text.Encoding]::UTF8)
  $goDiff = [System.IO.File]::ReadAllText((Join-Path $goReview 'diffs\combined.diff'), [System.Text.Encoding]::UTF8)
  foreach ($expected in @('changed after initial sync','old managed block','references/template/workflow-template.md')) {
    Assert-ContainsText -Text $psDiff -Expected $expected -Label 'PowerShell combined diff'
    Assert-ContainsText -Text $goDiff -Expected $expected -Label 'Go combined diff'
  }

  Assert-ContainsText -Text ([System.IO.File]::ReadAllText($managedDrift, [System.Text.Encoding]::UTF8)) -Expected 'changed after initial sync' -Label 'review-only managed drift preservation'
  if (Test-Path -LiteralPath $managedMissing) { throw 'review-only sync recreated a missing managed file' }
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText($templateLocal, [System.Text.Encoding]::UTF8)) -Expected 'keep this local file' -Label 'review-only local template preservation'
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText($blockHost, [System.Text.Encoding]::UTF8)) -Expected 'old managed block' -Label 'review-only managed block preservation'

  'sync review parity smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
