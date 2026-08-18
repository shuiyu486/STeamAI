param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string]$Pack = 'binary-re'
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

function Assert-ApplyWriteAction {
  param(
    [Parameter(Mandatory=$true)]$Result,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Action
  )
  $writes = @($Result.writes | Where-Object { [string]$_.path -eq $Path -and [string]$_.action -eq $Action })
  if ($writes.Count -lt 1) { throw "expected apply write for $Path/$Action, got: $($Result | ConvertTo-Json -Depth 20)" }
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

  $apply = Invoke-GoRekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,'-Apply','login') | ConvertFrom-Json
  if ([string]$apply.command -ne 'continue' -or -not [bool]$apply.isMutation -or -not [bool]$apply.applied -or [bool]$apply.requiresConfirmation -or [string]$apply.lane.id -ne 'feature-login') {
    throw "unexpected continue apply flags: $($apply | ConvertTo-Json -Depth 20)"
  }
  if ([int]$apply.summary.collected -ne 3 -or [int]$apply.summary.observations -ne 1 -or [int]$apply.summary.requests -ne 1 -or [int]$apply.summary.routed -ne 1 -or [int]$apply.summary.candidates -ne 1 -or [int]$apply.summary.authorityApplied -ne 0 -or [int]$apply.summary.authorityWouldAppend -ne 0 -or [int]$apply.summary.pendingUser -ne 1) {
    throw "unexpected continue apply summary: $($apply | ConvertTo-Json -Depth 20)"
  }
  Assert-ApplyWriteAction -Result $apply -Path '.rekit/facts/observations.jsonl' -Action 'append'
  Assert-ApplyWriteAction -Result $apply -Path '.rekit/facts/requests.jsonl' -Action 'append'
  Assert-ApplyWriteAction -Result $apply -Path '.rekit/facts/candidates.jsonl' -Action 'append'
  Assert-ApplyWriteAction -Result $apply -Path '.rekit/facts/decisions.jsonl' -Action 'append'
  Assert-ApplyWriteAction -Result $apply -Path '.rekit/lanes/devirt-main/tasks.jsonl' -Action 'append'
  Assert-ApplyWriteAction -Result $apply -Path '.rekit/board.json' -Action 'refresh'
  $authorityAfterApply = [System.IO.File]::ReadAllText($authorityPath, [System.Text.Encoding]::UTF8)
  Assert-NotContainsText -Text $authorityAfterApply -Unexpected 'OP_GO_CONTINUE' -Label 'go continue apply authority guard'
  $decisionText = [System.IO.File]::ReadAllText((Join-Path $caseRoot '.rekit\facts\decisions.jsonl'), [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $decisionText -Expected 'authority append requires explicit user confirmation' -Label 'go continue apply authority decision'

  $facadeBeforeFiles = Save-TreeSnapshot -Path $caseRoot
  $facadeBeforeDirs = Save-TreeDirectories -Path $caseRoot
  $facadeOut = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','login')
  Assert-ContainsText -Text $facadeOut -Expected 'feature-login' -Label 'facade continue text delegated to Go'
  Assert-NotContainsText -Text $facadeOut -Unexpected '"command": "continue"' -Label 'facade continue text delegated to Go'
  Assert-TreeUnchanged -Root $caseRoot -BeforeSnapshot $facadeBeforeFiles -BeforeDirectories $facadeBeforeDirs -Label 'facade continue what-if text delegated to Go'

  $facadeJsonBeforeFiles = Save-TreeSnapshot -Path $caseRoot
  $facadeJsonBeforeDirs = Save-TreeDirectories -Path $caseRoot
  $facadePreviewJson = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','login','-Format','json') | ConvertFrom-Json
  if ([string]$facadePreviewJson.command -ne 'continue' -or [bool]$facadePreviewJson.isMutation -or [bool]$facadePreviewJson.applied -or -not [bool]$facadePreviewJson.requiresConfirmation -or [string]$facadePreviewJson.lane.id -ne 'feature-login') {
    throw "unexpected facade continue JSON preview: $($facadePreviewJson | ConvertTo-Json -Depth 20)"
  }
  Assert-ContainsText -Text ([string]::Join("`n", @($facadePreviewJson.nextSteps))) -Expected 'JSON preview and explicit apply are Go-owned by default' -Label 'facade continue JSON delegated to Go'
  if ([int]$facadePreviewJson.summary.collected -ne 0 -or [int]$facadePreviewJson.summary.skipped -ne 3) {
    throw "unexpected facade continue JSON summary after apply: $($facadePreviewJson | ConvertTo-Json -Depth 20)"
  }
  Assert-TreeUnchanged -Root $caseRoot -BeforeSnapshot $facadeJsonBeforeFiles -BeforeDirectories $facadeJsonBeforeDirs -Label 'facade continue json preview'

  $facadeApplyJson = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,'-Apply','login','-Format','json') | ConvertFrom-Json
  if ([string]$facadeApplyJson.command -ne 'continue' -or -not [bool]$facadeApplyJson.isMutation -or -not [bool]$facadeApplyJson.applied -or [string]$facadeApplyJson.lane.id -ne 'feature-login') {
    throw "unexpected facade continue JSON apply: $($facadeApplyJson | ConvertTo-Json -Depth 20)"
  }
  if ([int]$facadeApplyJson.summary.collected -ne 0 -or [int]$facadeApplyJson.summary.skipped -ne 3) {
    throw "unexpected facade continue JSON apply duplicate summary: $($facadeApplyJson | ConvertTo-Json -Depth 20)"
  }
  $disabledBeforeFiles = Save-TreeSnapshot -Path $caseRoot
  $disabledBeforeDirs = Save-TreeDirectories -Path $caseRoot
  $disabledPreviewOut = Invoke-RekitSmoke -Arguments @('-Command','continue','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','login','-Format','json') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1' }
  Assert-ContainsText -Text $disabledPreviewOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled continue JSON no fallback'
  Assert-NotContainsText -Text $disabledPreviewOut -Unexpected 'PowerShell workflow after review' -Label 'go disabled continue JSON no fallback'
  Assert-TreeUnchanged -Root $caseRoot -BeforeSnapshot $disabledBeforeFiles -BeforeDirectories $disabledBeforeDirs -Label 'go disabled continue json no fallback'
  $global:LASTEXITCODE = 0

  'continue what-if smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
