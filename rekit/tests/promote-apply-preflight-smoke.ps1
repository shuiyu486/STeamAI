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
  $global:LASTEXITCODE = 0
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

function Get-TreeSnapshot {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return '' }
  $root = [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
  $files = @(Get-ChildItem -LiteralPath $Path -Recurse -File | ForEach-Object { $_.FullName.Substring($root.Length).TrimStart('\') } | Sort-Object)
  return ($files -join "`n")
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

function Restore-TreeSnapshot {
  param(
    [string]$Root,
    [hashtable]$BeforeSnapshot,
    [hashtable]$BeforeDirectories,
    [bool]$RootExisted
  )
  if (-not (Test-Path -LiteralPath $Root)) { return }
  $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\')
  Get-ChildItem -LiteralPath $Root -Recurse -File | ForEach-Object {
    $rel = $_.FullName.Substring($rootFull.Length).TrimStart('\')
    if (-not $BeforeSnapshot.ContainsKey($rel)) { Remove-Item -LiteralPath $_.FullName -Force -Confirm:$false }
  }
  foreach ($rel in $BeforeSnapshot.Keys) {
    $path = Join-Path $rootFull ([string]$rel)
    $parent = Split-Path -Parent $path
    if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
    [System.IO.File]::WriteAllBytes($path, [byte[]]$BeforeSnapshot[$rel])
  }
  if (-not $RootExisted -and (Test-Path -LiteralPath $Root)) {
    Remove-Item -LiteralPath $Root -Recurse -Force -Confirm:$false
    return
  }
  $dirs = @(Get-ChildItem -LiteralPath $Root -Recurse -Directory | Sort-Object { $_.FullName.Length } -Descending)
  foreach ($dir in $dirs) {
    $rel = $dir.FullName.Substring($rootFull.Length).TrimStart('\')
    if ($BeforeDirectories.ContainsKey($rel)) { continue }
    if (@(Get-ChildItem -LiteralPath $dir.FullName -Force).Count -eq 0) {
      Remove-Item -LiteralPath $dir.FullName -Force -Confirm:$false
    }
  }
}

function Assert-InsideRoot {
  param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Label
  )
  $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\') + '\'
  $pathFull = [System.IO.Path]::GetFullPath($Path)
  if (-not $pathFull.StartsWith($rootFull, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "$Label escaped root: $Path not under $Root"
  }
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "pap-$suffix"
$packRoot = Join-Path $RepoRoot "packs\$Pack"
$packReferencesRoot = Join-Path $packRoot 'references\template'
$promoteCandidateRoot = Join-Path $packRoot 'promote-candidates'
$toolingCandidateRoot = Join-Path $packRoot 'tooling\candidates'
$promoteRootExisted = Test-Path -LiteralPath $promoteCandidateRoot
$toolingRootExisted = Test-Path -LiteralPath $toolingCandidateRoot
$beforePackRefs = Save-TreeSnapshot -Path $packReferencesRoot
$beforePackRefsDirs = Save-TreeDirectories -Path $packReferencesRoot
$beforePromote = Save-TreeSnapshot -Path $promoteCandidateRoot
$beforeTooling = Save-TreeSnapshot -Path $toolingCandidateRoot
$beforePromoteDirs = Save-TreeDirectories -Path $promoteCandidateRoot
$beforeToolingDirs = Save-TreeDirectories -Path $toolingCandidateRoot
$beforePromoteTree = Get-TreeSnapshot -Path $promoteCandidateRoot
$beforeToolingTree = Get-TreeSnapshot -Path $toolingCandidateRoot
try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"promote-apply-$suffix") | Out-Null

  $caseReadme = Join-Path $caseRoot 'references\template\README.md'
  $caseWorkflow = Join-Path $caseRoot 'references\template\workflow-template.md'
  $packReadme = Join-Path $packReferencesRoot 'README.md'
  $packWorkflow = Join-Path $packReferencesRoot 'workflow-template.md'
  $originalReadme = [System.IO.File]::ReadAllText($packReadme, [System.Text.Encoding]::UTF8)
  $originalWorkflow = [System.IO.File]::ReadAllText($packWorkflow, [System.Text.Encoding]::UTF8)
  $safeReadme = "# Template README`r`n`r`nReusable apply baseline from smoke.`r`n"
  [System.IO.File]::WriteAllText($caseReadme, $safeReadme, [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText($caseWorkflow, "# Blocked workflow`r`n`r`nDo not promote C:\case\artifact\sample-trace.csv from this case.`r`n", [System.Text.UTF8Encoding]::new($false))

  $whatIf = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -AllowedExitCodes @(1)
  Assert-ContainsText -Text $whatIf -Expected 'PowerShell fallback has been retired' -Label 'promote apply text no fallback'
  Assert-NotContainsText -Text $whatIf -Unexpected 'would promote candidate:' -Label 'promote apply text no fallback'
  $readmeAfterWhatIf = [System.IO.File]::ReadAllText($packReadme, [System.Text.Encoding]::UTF8)
  if ($readmeAfterWhatIf -ne $originalReadme) { throw 'promote -Apply -WhatIf changed pack README' }
  if ($beforePromoteTree -ne (Get-TreeSnapshot -Path $promoteCandidateRoot)) { throw 'promote -Apply -WhatIf changed promote-candidates tree' }
  if ($beforeToolingTree -ne (Get-TreeSnapshot -Path $toolingCandidateRoot)) { throw 'promote -Apply -WhatIf changed tooling candidates tree' }

  $fakeGo = Join-Path $caseRoot 'fake-rekit-go.cmd'
  [System.IO.File]::WriteAllText($fakeGo, ('@echo off' + "`r`n" + 'echo {"schemaVersion":1,"command":"promote","delegatedByFake":true}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $facadeJsonPreview = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeJsonPreview.delegatedByFake) { throw "facade promote apply JSON preview did not use default REKIT_GO_EXE delegation: $($facadeJsonPreview | ConvertTo-Json -Depth 8)" }
  $facadeActualApply = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeActualApply.delegatedByFake) { throw "facade promote apply did not use default REKIT_GO_EXE delegation: $($facadeActualApply | ConvertTo-Json -Depth 8)" }

  $facadeWhatIf = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $facadeWhatIf -Unexpected 'delegatedByFake' -Label 'facade promote apply text no fallback'
  Assert-ContainsText -Text $facadeWhatIf -Expected 'PowerShell fallback has been retired' -Label 'facade promote apply text no fallback'

  $disabledJsonPreview = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledJsonPreview -Unexpected 'delegatedByFake' -Label 'facade promote apply JSON preview disabled no fallback'
  Assert-ContainsText -Text $disabledJsonPreview -Expected 'PowerShell fallback has been retired' -Label 'facade promote apply JSON preview disabled no fallback'

  $goWhatIf = Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf') | ConvertFrom-Json
  if ([bool]$goWhatIf.isMutation -or [bool]$goWhatIf.applied) { throw "unexpected Go apply what-if mutation: $($goWhatIf | ConvertTo-Json -Depth 10)" }
  if ([int]$goWhatIf.changed -lt 1 -or [int]$goWhatIf.blocked -lt 1) { throw "unexpected Go apply what-if counts: $($goWhatIf | ConvertTo-Json -Depth 10)" }
  if (-not [string]::IsNullOrWhiteSpace([string]$goWhatIf.backupRoot)) { throw "Go promote apply what-if returned backupRoot: $($goWhatIf.backupRoot)" }

  $apply = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1' }
  Assert-ContainsText -Text $apply -Expected 'PowerShell fallback has been retired' -Label 'disabled promote apply no fallback'
  Assert-NotContainsText -Text $apply -Unexpected 'promoted:' -Label 'disabled promote apply no fallback'
  $packReadmeAfterApply = [System.IO.File]::ReadAllText($packReadme, [System.Text.Encoding]::UTF8)
  if ($packReadmeAfterApply -ne $originalReadme) { throw 'disabled promote -Apply no-fallback changed pack README' }
  $packWorkflowAfterApply = [System.IO.File]::ReadAllText($packWorkflow, [System.Text.Encoding]::UTF8)
  if ($packWorkflowAfterApply -ne $originalWorkflow) { throw 'disabled promote -Apply no-fallback changed blocked workflow' }

  'promote apply preflight smoke ok'
} finally {
  Restore-TreeSnapshot -Root $packReferencesRoot -BeforeSnapshot $beforePackRefs -BeforeDirectories $beforePackRefsDirs -RootExisted $true
  Restore-TreeSnapshot -Root $promoteCandidateRoot -BeforeSnapshot $beforePromote -BeforeDirectories $beforePromoteDirs -RootExisted $promoteRootExisted
  Restore-TreeSnapshot -Root $toolingCandidateRoot -BeforeSnapshot $beforeTooling -BeforeDirectories $beforeToolingDirs -RootExisted $toolingRootExisted
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
