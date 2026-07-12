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

function Assert-ContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Expected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text -notlike "*$Expected*") { throw "$Label missing expected text '$Expected'. Output:`n$Text" }
}

function Assert-FileEquals {
  param(
    [Parameter(Mandatory=$true)][string]$Left,
    [Parameter(Mandatory=$true)][string]$Right,
    [Parameter(Mandatory=$true)][string]$Label
  )
  $leftHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Left).Hash
  $rightHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $Right).Hash
  if ($leftHash -ne $rightHash) { throw "$Label hash mismatch: $Left != $Right" }
}

function Get-TreeSnapshot {
  param([Parameter(Mandatory=$true)][string]$Root)
  $snapshot = @{}
  if (-not (Test-Path -LiteralPath $Root)) { return $snapshot }
  $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)
  Get-ChildItem -LiteralPath $Root -Recurse -File | ForEach-Object {
    $full = [System.IO.Path]::GetFullPath($_.FullName)
    $rel = $full.Substring($rootFull.Length).TrimStart([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)
    $snapshot[$rel] = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash
  }
  return $snapshot
}

function Assert-SnapshotEquals {
  param(
    [Parameter(Mandatory=$true)][hashtable]$Before,
    [Parameter(Mandatory=$true)][hashtable]$After,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Before.Count -ne $After.Count) { throw "$Label file count changed: before=$($Before.Count) after=$($After.Count)" }
  foreach ($key in $Before.Keys) {
    if (-not $After.ContainsKey($key)) { throw "$Label missing file after preview: $key" }
    if ([string]$Before[$key] -ne [string]$After[$key]) { throw "$Label changed file after preview: $key" }
  }
}

function Get-WriteItem {
  param(
    [Parameter(Mandatory=$true)]$Result,
    [Parameter(Mandatory=$true)][string]$Path
  )
  $items = @($Result.writes | Where-Object { [string]$_.path -eq $Path })
  if ($items.Count -ne 1) { throw "expected exactly one write item for $Path, got $($items.Count)" }
  return $items[0]
}

function Assert-WriteAction {
  param(
    [Parameter(Mandatory=$true)]$Result,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Action,
    [switch]$RequireBackup
  )
  $item = Get-WriteItem -Result $Result -Path $Path
  if ([string]$item.action -ne $Action) { throw "write action for $Path was '$($item.action)', want '$Action'" }
  if ($RequireBackup) {
    if ([string]::IsNullOrWhiteSpace([string]$item.backupPath)) { throw "write item for $Path did not report a backup" }
    if (-not (Test-Path -LiteralPath ([string]$item.backupPath))) { throw "backup path for $Path does not exist: $($item.backupPath)" }
  }
  return $item
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "sync-apply-smoke-$suffix"
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','attach','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"sync-apply-$suffix",'-Apply') | Out-Null

  $readme = Join-Path $caseRoot 'references\template\README.md'
  $handoff = Join-Path $caseRoot 'references\template\task-handoff.md'
  $blockHost = Join-Path $caseRoot 'CLAUDE.local.md'
  New-Item -ItemType Directory -Path (Split-Path -Parent $readme) -Force | Out-Null
  [System.IO.File]::WriteAllText($readme, "# Local drift`r`n`r`nchanged before Go sync apply`r`n", [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText($handoff, "# Local handoff`r`n`r`nkeep this file on first apply`r`n", [System.Text.UTF8Encoding]::new($false))
  $oldBlock = "prefix`r`n`r`n<!-- BEGIN template-pack:router -->`r`nold managed block`r`n<!-- END template-pack:router -->`r`n`r`nsuffix`r`n"
  [System.IO.File]::WriteAllText($blockHost, $oldBlock, [System.Text.UTF8Encoding]::new($false))

  $beforePreview = Get-TreeSnapshot -Root $caseRoot
  $preview = Invoke-GoRekitSmoke -Arguments @('-Command','sync','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-ProjectName',"sync-preview-$suffix") | ConvertFrom-Json
  if ([bool]$preview.applied -or [bool]$preview.isMutation) { throw "unexpected sync apply preview result: $($preview | ConvertTo-Json -Depth 8)" }
  Assert-WriteAction -Result $preview -Path 'references/template/README.md' -Action 'overwrite-with-backup' | Out-Null
  Assert-WriteAction -Result $preview -Path 'references/template/workflow-template.md' -Action 'create-managed-file' | Out-Null
  Assert-WriteAction -Result $preview -Path 'references/template/task-handoff.md' -Action 'skip-existing-local-file' | Out-Null
  Assert-WriteAction -Result $preview -Path 'CLAUDE.local.md' -Action 'replace-managed-block' | Out-Null
  if (Test-Path -LiteralPath ([string]$preview.backupRoot)) { throw "sync apply preview created backup root: $($preview.backupRoot)" }
  Assert-SnapshotEquals -Before $beforePreview -After (Get-TreeSnapshot -Root $caseRoot) -Label 'Go sync apply preview'

  $fakeGo = Join-Path $caseRoot 'fake-rekit-go.cmd'
  [System.IO.File]::WriteAllText($fakeGo, ('@echo off' + "`r`n" + 'echo {"schemaVersion":1,"command":"sync","delegatedByFake":true,"isMutation":false,"applied":false}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $beforeFacadePreview = Get-TreeSnapshot -Root $caseRoot
  $facadePreview = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadePreview.delegatedByFake) { throw "facade sync apply JSON preview did not use default REKIT_GO_EXE delegation: $($facadePreview | ConvertTo-Json -Depth 8)" }
  Assert-SnapshotEquals -Before $beforeFacadePreview -After (Get-TreeSnapshot -Root $caseRoot) -Label 'facade sync apply JSON preview'

  $facadeApply = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeApply.delegatedByFake) { throw "facade sync apply did not use default REKIT_GO_EXE delegation: $($facadeApply | ConvertTo-Json -Depth 8)" }

  $facadeTextPreview = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $facadeTextPreview -Expected 'would attach case' -Label 'facade sync apply text fallback'

  $disabledFacadeApply = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $disabledFacadeApply -Expected 'would attach case' -Label 'facade sync apply disabled fallback'

  $apply = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-ProjectName',"sync-apply-$suffix") | ConvertFrom-Json
  if (-not [bool]$apply.applied -or [bool]$apply.isMutation -ne $true) { throw "unexpected sync apply result: $($apply | ConvertTo-Json -Depth 8)" }
  Assert-WriteAction -Result $apply -Path 'references/template/README.md' -Action 'overwrite-with-backup' -RequireBackup | Out-Null
  Assert-WriteAction -Result $apply -Path 'references/template/workflow-template.md' -Action 'create-managed-file' | Out-Null
  Assert-WriteAction -Result $apply -Path 'references/template/task-handoff.md' -Action 'skip-existing-local-file' | Out-Null
  Assert-WriteAction -Result $apply -Path 'CLAUDE.local.md' -Action 'replace-managed-block' -RequireBackup | Out-Null

  Assert-FileEquals -Left $readme -Right (Join-Path $RepoRoot 'packs\_template\references\template\README.md') -Label 'managed README'
  if (-not (Test-Path -LiteralPath (Join-Path $caseRoot 'references\template\workflow-template.md'))) { throw 'missing managed workflow-template after sync apply' }
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText($handoff, [System.Text.Encoding]::UTF8)) -Expected 'keep this file on first apply' -Label 'template skip'
  $blockText = [System.IO.File]::ReadAllText($blockHost, [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $blockText -Expected 'Template pack router' -Label 'managed block replacement'
  Assert-ContainsText -Text $blockText -Expected 'prefix' -Label 'managed block surrounding content'
  Assert-ContainsText -Text $blockText -Expected 'suffix' -Label 'managed block surrounding content'
  $state = [System.IO.File]::ReadAllText((Join-Path $caseRoot '.rekit\state.json'), [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $state -Expected 'targetHashAtSync' -Label 'sync state'

  $force = Invoke-GoRekitSmoke -Arguments @('-Command','sync','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Force','-ProjectName',"sync-force-$suffix") | ConvertFrom-Json
  Assert-WriteAction -Result $force -Path 'references/template/task-handoff.md' -Action 'overwrite-local-template-file-with-force' -RequireBackup | Out-Null
  $forcedText = [System.IO.File]::ReadAllText($handoff, [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $forcedText -Expected "sync-force-$suffix" -Label 'forced template placeholder'
  Assert-ContainsText -Text $forcedText -Expected $caseRoot -Label 'forced template placeholder'

  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null

  'sync apply smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
