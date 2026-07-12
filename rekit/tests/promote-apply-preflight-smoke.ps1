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

  $whatIf = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf')
  Assert-ContainsText -Text $whatIf -Expected 'would promote candidate: references/template/README.md' -Label 'PowerShell promote apply what-if'
  Assert-ContainsText -Text $whatIf -Expected 'blocked promote: references/template/workflow-template.md' -Label 'PowerShell promote apply blocked what-if'
  Assert-ContainsText -Text $whatIf -Expected 'would write tooling candidate:' -Label 'PowerShell promote apply what-if tooling preview'
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

  $facadeWhatIf = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $facadeWhatIf -Expected 'would promote candidate: references/template/README.md' -Label 'facade promote apply text fallback'
  Assert-NotContainsText -Text $facadeWhatIf -Unexpected 'go backend' -Label 'facade promote apply text fallback'

  $disabledJsonPreview = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledJsonPreview -Unexpected 'delegatedByFake' -Label 'facade promote apply JSON preview disabled fallback'
  Assert-ContainsText -Text $disabledJsonPreview -Expected 'would promote candidate: references/template/README.md' -Label 'facade promote apply JSON preview disabled fallback'

  $goWhatIf = Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf') | ConvertFrom-Json
  if ([bool]$goWhatIf.isMutation -or [bool]$goWhatIf.applied) { throw "unexpected Go apply what-if mutation: $($goWhatIf | ConvertTo-Json -Depth 10)" }
  if ([int]$goWhatIf.changed -lt 1 -or [int]$goWhatIf.blocked -lt 1) { throw "unexpected Go apply what-if counts: $($goWhatIf | ConvertTo-Json -Depth 10)" }
  if (-not [string]::IsNullOrWhiteSpace([string]$goWhatIf.backupRoot)) { throw "Go promote apply what-if returned backupRoot: $($goWhatIf.backupRoot)" }

  $apply = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1' }
  Assert-ContainsText -Text $apply -Expected 'backup pack file:' -Label 'PowerShell disabled promote apply backup'
  Assert-ContainsText -Text $apply -Expected 'promoted:' -Label 'PowerShell disabled promote apply write'
  Assert-ContainsText -Text $apply -Expected 'promote summary:' -Label 'PowerShell disabled promote apply summary'
  $packReadmeAfterApply = [System.IO.File]::ReadAllText($packReadme, [System.Text.Encoding]::UTF8)
  if ($packReadmeAfterApply -ne $safeReadme) { throw 'promote -Apply did not update pack README to safe case content' }
  $packWorkflowAfterApply = [System.IO.File]::ReadAllText($packWorkflow, [System.Text.Encoding]::UTF8)
  if ($packWorkflowAfterApply -ne $originalWorkflow) { throw 'blocked workflow was written to pack source' }

  $backupLine = @($apply -split "`r?`n" | Where-Object { $_ -like 'backup pack file:*' } | Select-Object -First 1)
  if ($backupLine.Count -ne 1) { throw "missing backup line in promote apply output:`n$apply" }
  $backupPath = ([string]$backupLine[0]).Substring('backup pack file:'.Length).Trim()
  Assert-InsideRoot -Root (Join-Path $promoteCandidateRoot '.backup') -Path $backupPath -Label 'promote apply backup'
  if (-not (Test-Path -LiteralPath $backupPath)) { throw "missing promote backup: $backupPath" }
  $backupText = [System.IO.File]::ReadAllText($backupPath, [System.Text.Encoding]::UTF8)
  if ($backupText -ne $originalReadme) { throw 'promote backup content did not match original pack README' }

  'promote apply preflight smoke ok'
} finally {
  Restore-TreeSnapshot -Root $packReferencesRoot -BeforeSnapshot $beforePackRefs -BeforeDirectories $beforePackRefsDirs -RootExisted $true
  Restore-TreeSnapshot -Root $promoteCandidateRoot -BeforeSnapshot $beforePromote -BeforeDirectories $beforePromoteDirs -RootExisted $promoteRootExisted
  Restore-TreeSnapshot -Root $toolingCandidateRoot -BeforeSnapshot $beforeTooling -BeforeDirectories $beforeToolingDirs -RootExisted $toolingRootExisted
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
