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

function Write-Utf8File {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Text
  )
  $parent = Split-Path -Parent $Path
  if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
  [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Normalize-Text {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$CaseRoot
  )
  $normalized = $Text.TrimStart([char]0xFEFF)
  $normalized = $normalized -replace "`r`n", "`n"
  $normalized = $normalized -replace "`r", "`n"
  $normalized = $normalized.Replace($CaseRoot, '<CASE_ROOT>')
  $normalized = $normalized.Replace(($CaseRoot -replace '\\','/'), '<CASE_ROOT>')
  return $normalized.TrimEnd()
}

function Read-NormalizedFile {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][string]$Rel
  )
  $path = Join-Path $Root $Rel
  if (-not (Test-Path -LiteralPath $path)) { throw "missing file: $path" }
  return Normalize-Text -Text ([System.IO.File]::ReadAllText($path, [System.Text.Encoding]::UTF8)) -CaseRoot $Root
}

function Assert-NormalizedFileEquals {
  param(
    [Parameter(Mandatory=$true)][string]$LeftRoot,
    [Parameter(Mandatory=$true)][string]$RightRoot,
    [Parameter(Mandatory=$true)][string]$Rel
  )
  $left = Read-NormalizedFile -Root $LeftRoot -Rel $Rel
  $right = Read-NormalizedFile -Root $RightRoot -Rel $Rel
  if ($left -ne $right) { throw "normalized file mismatch for $Rel`n--- left ---`n$left`n--- right ---`n$right" }
}

function Assert-SyncStateParity {
  param(
    [Parameter(Mandatory=$true)][string]$LeftRoot,
    [Parameter(Mandatory=$true)][string]$RightRoot
  )
  $left = [System.IO.File]::ReadAllText((Join-Path $LeftRoot '.rekit\state.json'), [System.Text.Encoding]::UTF8) | ConvertFrom-Json
  $right = [System.IO.File]::ReadAllText((Join-Path $RightRoot '.rekit\state.json'), [System.Text.Encoding]::UTF8) | ConvertFrom-Json
  if ([string]$left.templateRoot -ne [string]$right.templateRoot -or [string]$left.templatePack -ne [string]$right.templatePack) { throw 'sync state template binding mismatch' }
  $leftProps = @($left.managed.PSObject.Properties.Name | Sort-Object)
  $rightProps = @($right.managed.PSObject.Properties.Name | Sort-Object)
  if (($leftProps -join '|') -ne ($rightProps -join '|')) { throw "sync state managed keys mismatch: $($leftProps -join ',') vs $($rightProps -join ',')" }
  foreach ($rel in $leftProps) {
    $l = $left.managed.$rel
    $r = $right.managed.$rel
    if ([string]$l.sourceHash -ne [string]$r.sourceHash -or [string]$l.targetHashAtSync -ne [string]$r.targetHashAtSync -or [string]$l.lastAction -ne [string]$r.lastAction) {
      throw "sync state entry mismatch for $rel"
    }
  }
}

function Assert-BackupExists {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][string]$Leaf,
    [Parameter(Mandatory=$true)][string]$Label
  )
  $backupRoot = Join-Path $Root '.rekit\backups'
  if (-not (Test-Path -LiteralPath $backupRoot)) { throw "$Label missing backup root: $backupRoot" }
  $matches = @(Get-ChildItem -LiteralPath $backupRoot -Recurse -File | Where-Object { $_.Name -eq $Leaf })
  if ($matches.Count -lt 1) { throw "$Label missing backup leaf $Leaf under $backupRoot" }
}

function Seed-CaseDrift {
  param([Parameter(Mandatory=$true)][string]$Root)
  Write-Utf8File -Path (Join-Path $Root 'references\template\README.md') -Text "# Local drift`r`n`r`nchanged before sync apply parity`r`n"
  Write-Utf8File -Path (Join-Path $Root 'references\template\task-handoff.md') -Text "# Local handoff`r`n`r`nkeep this file before force`r`n"
  $oldBlock = "prefix`r`n`r`n<!-- BEGIN template-pack:router -->`r`nold managed block`r`n<!-- END template-pack:router -->`r`n`r`nsuffix`r`n"
  Write-Utf8File -Path (Join-Path $Root 'CLAUDE.local.md') -Text $oldBlock
}

function Assert-CaseParity {
  param(
    [Parameter(Mandatory=$true)][string]$PowerShellRoot,
    [Parameter(Mandatory=$true)][string]$GoRoot
  )
  foreach ($rel in @(
    'references\template\README.md',
    'references\template\agent-team.md',
    'references\template\workflow-template.md',
    'references\template\toolchain-router.md',
    'references\template\task-handoff.md',
    'CLAUDE.local.md',
    '.claude\skills\rekit\SKILL.md',
    '.rekit\instance.yml',
    '.re-template.yml'
  )) {
    Assert-NormalizedFileEquals -LeftRoot $PowerShellRoot -RightRoot $GoRoot -Rel $rel
  }
  Assert-SyncStateParity -LeftRoot $PowerShellRoot -RightRoot $GoRoot
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$psCase = Join-Path $WorkRoot "sync-parity-ps-$suffix"
$goCase = Join-Path $WorkRoot "sync-parity-go-$suffix"
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','attach','-Target',$psCase,'-Pack',$Pack,'-ProjectName',"sync-parity-$suffix",'-Apply') | Out-Null
  Invoke-GoRekitSmoke -Arguments @('-Command','attach','-Target',$goCase,'-Pack',$Pack,'-ProjectName',"sync-parity-$suffix",'-Apply') | Out-Null
  Seed-CaseDrift -Root $psCase
  Seed-CaseDrift -Root $goCase

  Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$psCase,'-Pack',$Pack,'-Apply','-ProjectName',"sync-parity-$suffix") -Env @{ REKIT_GO_DISABLE = '1' } | Out-Null
  $goApply = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$goCase,'-Pack',$Pack,'-Apply','-ProjectName',"sync-parity-$suffix") | ConvertFrom-Json
  if (-not [bool]$goApply.applied -or [bool]$goApply.isMutation -ne $true) { throw "unexpected Go apply result: $($goApply | ConvertTo-Json -Depth 8)" }

  Assert-CaseParity -PowerShellRoot $psCase -GoRoot $goCase
  Assert-BackupExists -Root $psCase -Leaf 'README.md' -Label 'PowerShell sync apply'
  Assert-BackupExists -Root $goCase -Leaf 'README.md' -Label 'Go sync apply'
  Assert-BackupExists -Root $psCase -Leaf 'CLAUDE.local.md' -Label 'PowerShell sync apply'
  Assert-BackupExists -Root $goCase -Leaf 'CLAUDE.local.md' -Label 'Go sync apply'

  Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$psCase,'-Pack',$Pack,'-Apply','-Force','-ProjectName',"sync-force-$suffix") -Env @{ REKIT_GO_DISABLE = '1' } | Out-Null
  $goForce = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$goCase,'-Pack',$Pack,'-Apply','-Force','-ProjectName',"sync-force-$suffix") | ConvertFrom-Json
  if (-not [bool]$goForce.applied -or [bool]$goForce.isMutation -ne $true) { throw "unexpected Go force apply result: $($goForce | ConvertTo-Json -Depth 8)" }

  Assert-CaseParity -PowerShellRoot $psCase -GoRoot $goCase
  Assert-ContainsText -Text (Read-NormalizedFile -Root $psCase -Rel 'references\template\task-handoff.md') -Expected "sync-force-$suffix" -Label 'PowerShell forced template'
  Assert-ContainsText -Text (Read-NormalizedFile -Root $goCase -Rel 'references\template\task-handoff.md') -Expected "sync-force-$suffix" -Label 'Go forced template'
  Assert-BackupExists -Root $psCase -Leaf 'task-handoff.md' -Label 'PowerShell force sync apply'
  Assert-BackupExists -Root $goCase -Leaf 'task-handoff.md' -Label 'Go force sync apply'

  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$psCase,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$goCase,'-Pack',$Pack) | Out-Null
  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$psCase,'-Pack',$Pack) | Out-Null
  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$goCase,'-Pack',$Pack) | Out-Null

  $fakeGo = Join-Path $goCase 'fake-rekit-go.cmd'
  [System.IO.File]::WriteAllText($fakeGo, ('@echo off' + "`r`n" + 'echo {"schemaVersion":1,"command":"sync","delegatedByFake":true,"isMutation":true,"applied":true}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $facadeDefaultApply = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$goCase,'-Pack',$Pack,'-Apply','-Format','json') -Env @{ REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeDefaultApply.delegatedByFake) { throw "facade sync apply did not use default delegation: $($facadeDefaultApply | ConvertTo-Json -Depth 8)" }
  $facadeDisabledApply = Invoke-RekitSmoke -Arguments @('-Command','sync','-Target',$goCase,'-Pack',$Pack,'-Apply') -Env @{ REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  if ($facadeDisabledApply -like '*delegatedByFake*') { throw "disabled sync apply unexpectedly used fake Go delegation: $facadeDisabledApply" }
  Assert-ContainsText -Text $facadeDisabledApply -Expected 'sync ok' -Label 'disabled sync apply fallback'

  'sync apply parity smoke ok'
} finally {
  if (Test-Path -LiteralPath $psCase) { Remove-Item -LiteralPath $psCase -Recurse -Force -Confirm:$false }
  if (Test-Path -LiteralPath $goCase) { Remove-Item -LiteralPath $goCase -Recurse -Force -Confirm:$false }
}
