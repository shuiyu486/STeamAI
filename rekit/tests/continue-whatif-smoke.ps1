param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string]$Pack = 'vmp-re'
)

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
  param(
    [Parameter(Mandatory=$true)][string[]]$Arguments,
    [int[]]$AllowedExitCodes = @(0)
  )
  Push-Location $RepoRoot
  $oldEap = $ErrorActionPreference
  try {
    $ErrorActionPreference = 'Continue'
    $global:LASTEXITCODE = 0
    $output = & go run ./cmd/rekit -- @Arguments 2>&1 | Out-String
    $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
    if ($AllowedExitCodes -notcontains $exitCode) { throw "go rekit unexpected exit code $exitCode; output:`n$output" }
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

function Write-Utf8File {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text
  )
  $parent = Split-Path -Parent $Path
  if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
  [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Save-TreeSnapshot {
  param([Parameter(Mandatory=$true)][string]$Path)
  $snapshot = @{}
  if (-not (Test-Path -LiteralPath $Path)) { return $snapshot }
  $root = [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
  Get-ChildItem -LiteralPath $Path -Recurse -File | ForEach-Object {
    $rel = $_.FullName.Substring($root.Length).TrimStart('\')
    $snapshot[$rel] = [System.IO.File]::ReadAllBytes($_.FullName)
  }
  return $snapshot
}

function Save-TreeDirectories {
  param([Parameter(Mandatory=$true)][string]$Path)
  $snapshot = @{}
  if (-not (Test-Path -LiteralPath $Path)) { return $snapshot }
  $root = [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
  Get-ChildItem -LiteralPath $Path -Recurse -Directory | ForEach-Object {
    $rel = $_.FullName.Substring($root.Length).TrimStart('\')
    if (-not [string]::IsNullOrWhiteSpace($rel)) { $snapshot[$rel] = $true }
  }
  return $snapshot
}

function Assert-TreeUnchanged {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][hashtable]$BeforeSnapshot,
    [Parameter(Mandatory=$true)][hashtable]$BeforeDirectories,
    [string]$Label = 'continue what-if'
  )
  $afterSnapshot = Save-TreeSnapshot -Path $Root
  $afterDirectories = Save-TreeDirectories -Path $Root
  foreach ($rel in $BeforeSnapshot.Keys) {
    if (-not $afterSnapshot.ContainsKey($rel)) { throw "$Label removed file: $rel" }
    $beforeBytes = [byte[]]$BeforeSnapshot[$rel]
    $afterBytes = [byte[]]$afterSnapshot[$rel]
    if ($beforeBytes.Length -ne $afterBytes.Length) { throw "$Label changed file length: $rel" }
    for ($i = 0; $i -lt $beforeBytes.Length; $i++) {
      if ($beforeBytes[$i] -ne $afterBytes[$i]) { throw "$Label changed file content: $rel" }
    }
  }
  foreach ($rel in $afterSnapshot.Keys) {
    if (-not $BeforeSnapshot.ContainsKey($rel)) { throw "$Label created file: $rel" }
  }
  foreach ($rel in $BeforeDirectories.Keys) {
    if (-not $afterDirectories.ContainsKey($rel)) { throw "$Label removed directory: $rel" }
  }
  foreach ($rel in $afterDirectories.Keys) {
    if (-not $BeforeDirectories.ContainsKey($rel)) { throw "$Label created directory: $rel" }
  }
}

function Assert-WriteAction {
  param(
    [Parameter(Mandatory=$true)]$Result,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Action
  )
  $writes = @($Result.wouldWrites | Where-Object { [string]$_.path -eq $Path -and [string]$_.action -eq $Action })
  if ($writes.Count -lt 1) { throw "expected write for $Path/$Action, got: $($Result | ConvertTo-Json -Depth 20)" }
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "continue-whatif-$suffix"
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"continue-whatif-$suffix",'-Apply') | Out-Null
  Invoke-GoRekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','login','-Apply') | Out-Null

  $authorityPath = Join-Path $caseRoot 'captures\vm_opcode_semantics_confirmed.csv'
  Write-Utf8File -Path $authorityPath -Text "opcode,semantics,status`r`nOP_EXISTING,known,confirmed`r`n"
  Write-Utf8File -Path (Join-Path $caseRoot 'workspace\features\feature-login\packet.md') -Text "# packet`r`n"
  $outbox = @(
    '{"eventId":"evt-go-continue-observation","kind":"observation","subject":"continue observation","summary":"preview observation","evidence":"evidence-observation-token"}',
    '{"eventId":"evt-go-continue-request","kind":"request","subject":"route request","summary":"route to authority lane","requestId":"req-go-continue","targetLane":"devirt-main","evidence":"evidence-request-token"}',
    '{"eventId":"evt-go-continue-authority","kind":"candidate","subject":"authority candidate","summary":"append opcode row","authorityFile":"captures/vm_opcode_semantics_confirmed.csv","confidence":"0.95","evidence":"evidence-authority-token","row":{"opcode":"OP_GO_CONTINUE","semantics":"continue-preview","status":"confirmed"}}'
  ) -join "`r`n"
  Write-Utf8File -Path (Join-Path $caseRoot '.rekit\lanes\feature-login\outbox.jsonl') -Text ($outbox + "`r`n")

  $beforeFiles = Save-TreeSnapshot -Path $caseRoot
  $beforeDirs = Save-TreeDirectories -Path $caseRoot
  $preview = Invoke-GoRekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','login') | ConvertFrom-Json
  if ([string]$preview.command -ne 'continue' -or [bool]$preview.isMutation -or [bool]$preview.applied -or -not [bool]$preview.requiresConfirmation) {
    throw "unexpected continue preview flags: $($preview | ConvertTo-Json -Depth 20)"
  }
  if ([string]$preview.lane.id -ne 'feature-login') { throw "unexpected continue lane: $($preview | ConvertTo-Json -Depth 20)" }
  if ([int]$preview.summary.collected -ne 3 -or [int]$preview.summary.observations -ne 1 -or [int]$preview.summary.requests -ne 1 -or [int]$preview.summary.routed -ne 1 -or [int]$preview.summary.candidates -ne 1 -or [int]$preview.summary.authorityApplied -ne 0 -or [int]$preview.summary.authorityWouldAppend -ne 1 -or [int]$preview.summary.pendingUser -ne 0) {
    throw "unexpected continue summary: $($preview | ConvertTo-Json -Depth 20)"
  }
  Assert-WriteAction -Result $preview -Path 'captures/vm_opcode_semantics_confirmed.csv' -Action 'would-append'
  Assert-WriteAction -Result $preview -Path '.rekit/lanes/devirt-main/tasks.jsonl' -Action 'would-append'
  Assert-TreeUnchanged -Root $caseRoot -BeforeSnapshot $beforeFiles -BeforeDirectories $beforeDirs -Label 'go continue what-if'

  $applyGuard = Invoke-GoRekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,'-Apply','login') -AllowedExitCodes @(1)
  Assert-ContainsText -Text $applyGuard -Expected 'supports -WhatIf preview only' -Label 'go continue apply guard'

  $facadeBeforeFiles = Save-TreeSnapshot -Path $caseRoot
  $facadeBeforeDirs = Save-TreeDirectories -Path $caseRoot
  $facadeOut = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }
  Assert-NotContainsText -Text $facadeOut -Unexpected '"command": "continue"' -Label 'facade continue fallback'
  Assert-TreeUnchanged -Root $caseRoot -BeforeSnapshot $facadeBeforeFiles -BeforeDirectories $facadeBeforeDirs -Label 'facade continue what-if fallback'

  'continue what-if smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
