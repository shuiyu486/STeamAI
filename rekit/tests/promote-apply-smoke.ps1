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

function Assert-WriteAction {
  param(
    [Parameter(Mandatory=$true)]$Result,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Action
  )
  $writes = @($Result.writes | Where-Object { [string]$_.path -eq $Path -and [string]$_.action -eq $Action })
  if ($writes.Count -ne 1) { throw "expected exactly one write for $Path/$Action, got $($writes.Count): $($Result | ConvertTo-Json -Depth 10)" }
  if ([string]::IsNullOrWhiteSpace([string]$writes[0].targetPath)) { throw "write for $Path/$Action missing targetPath" }
  return $writes[0]
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
$caseRoot = Join-Path $WorkRoot "pas-$suffix"
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
  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"promote-apply-$suffix",'-Apply') | Out-Null

  $caseReadme = Join-Path $caseRoot 'references\template\README.md'
  $caseWorkflow = Join-Path $caseRoot 'references\template\workflow-template.md'
  $packReadme = Join-Path $packReferencesRoot 'README.md'
  $packWorkflow = Join-Path $packReferencesRoot 'workflow-template.md'
  $originalReadme = [System.IO.File]::ReadAllText($packReadme, [System.Text.Encoding]::UTF8)
  $originalWorkflow = [System.IO.File]::ReadAllText($packWorkflow, [System.Text.Encoding]::UTF8)
  $safeReadme = "# Template README`r`n`r`nReusable Go apply baseline from smoke.`r`n"
  [System.IO.File]::WriteAllText($caseReadme, $safeReadme, [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText($caseWorkflow, "# Blocked workflow`r`n`r`nDo not promote C:\case\artifact\sample-trace.csv from this case.`r`n", [System.Text.UTF8Encoding]::new($false))

  $goWhatIf = Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf') | ConvertFrom-Json
  if ([bool]$goWhatIf.isMutation -or [bool]$goWhatIf.applied) { throw "unexpected Go what-if mutation: $($goWhatIf | ConvertTo-Json -Depth 10)" }
  if ([int]$goWhatIf.changed -lt 1 -or [int]$goWhatIf.blocked -lt 1) { throw "unexpected Go what-if counts: $($goWhatIf | ConvertTo-Json -Depth 10)" }
  $readmePreview = Assert-WriteAction -Result $goWhatIf -Path 'references/template/README.md' -Action 'would-promote'
  Assert-WriteAction -Result $goWhatIf -Path 'references/template/workflow-template.md' -Action 'blocked-deny-pattern' | Out-Null
  if (-not [string]::IsNullOrWhiteSpace([string]$goWhatIf.backupRoot)) { throw "Go promote apply what-if returned backupRoot: $($goWhatIf.backupRoot)" }
  if (Test-Path -LiteralPath ([string]$readmePreview.backupPath)) { throw "Go promote apply what-if created backup: $($readmePreview.backupPath)" }
  if ([System.IO.File]::ReadAllText($packReadme, [System.Text.Encoding]::UTF8) -ne $originalReadme) { throw 'Go promote -Apply -WhatIf changed pack README' }
  if ($beforePromoteTree -ne (Get-TreeSnapshot -Path $promoteCandidateRoot)) { throw 'Go promote -Apply -WhatIf changed promote-candidates tree' }
  if ($beforeToolingTree -ne (Get-TreeSnapshot -Path $toolingCandidateRoot)) { throw 'Go promote -Apply -WhatIf changed tooling candidates tree' }

  $facadeWhatIf = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-WhatIf') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }
  Assert-ContainsText -Text $facadeWhatIf -Expected 'would promote candidate: references/template/README.md' -Label 'facade promote apply fallback'
  Assert-NotContainsText -Text $facadeWhatIf -Unexpected 'validationRows' -Label 'facade promote apply fallback'

  $goApply = Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-Apply') | ConvertFrom-Json
  if (-not [bool]$goApply.isMutation -or -not [bool]$goApply.applied) { throw "unexpected Go apply result flags: $($goApply | ConvertTo-Json -Depth 10)" }
  if ([int]$goApply.changed -lt 1 -or [int]$goApply.blocked -lt 1 -or -not [bool]$goApply.requiresCleanup) { throw "unexpected Go apply counts: $($goApply | ConvertTo-Json -Depth 10)" }
  if (@($goApply.validationRows).Count -lt 1) { throw "Go apply did not return validation rows: $($goApply | ConvertTo-Json -Depth 10)" }
  Assert-InsideRoot -Root (Join-Path $promoteCandidateRoot '.backup') -Path ([string]$goApply.backupRoot) -Label 'promote apply backup root'
  $readmeWrite = Assert-WriteAction -Result $goApply -Path 'references/template/README.md' -Action 'promote'
  $workflowWrite = Assert-WriteAction -Result $goApply -Path 'references/template/workflow-template.md' -Action 'blocked-deny-pattern'
  Assert-InsideRoot -Root $promoteCandidateRoot -Path ([string]$readmeWrite.backupPath) -Label 'promote apply backup'
  Assert-InsideRoot -Root $packRoot -Path ([string]$readmeWrite.targetPath) -Label 'promote apply target'
  if (-not (Test-Path -LiteralPath ([string]$readmeWrite.backupPath))) { throw "missing Go promote backup: $($readmeWrite.backupPath)" }
  $backupText = [System.IO.File]::ReadAllText([string]$readmeWrite.backupPath, [System.Text.Encoding]::UTF8)
  if ($backupText -ne $originalReadme) { throw 'Go promote backup content did not match original pack README' }
  if ([System.IO.File]::ReadAllText($packReadme, [System.Text.Encoding]::UTF8) -ne $safeReadme) { throw 'Go promote -Apply did not update pack README to safe case content' }
  if ([System.IO.File]::ReadAllText($packWorkflow, [System.Text.Encoding]::UTF8) -ne $originalWorkflow) { throw 'blocked workflow was written to pack source' }
  if (-not [string]::IsNullOrWhiteSpace([string]$workflowWrite.backupPath)) { throw "blocked workflow unexpectedly has backup path: $($workflowWrite.backupPath)" }
  if ($beforeToolingTree -ne (Get-TreeSnapshot -Path $toolingCandidateRoot)) { throw 'Go promote -Apply changed tooling candidates tree' }

  'promote apply smoke ok'
} finally {
  Restore-TreeSnapshot -Root $packReferencesRoot -BeforeSnapshot $beforePackRefs -BeforeDirectories $beforePackRefsDirs -RootExisted $true
  Restore-TreeSnapshot -Root $promoteCandidateRoot -BeforeSnapshot $beforePromote -BeforeDirectories $beforePromoteDirs -RootExisted $promoteRootExisted
  Restore-TreeSnapshot -Root $toolingCandidateRoot -BeforeSnapshot $beforeTooling -BeforeDirectories $beforeToolingDirs -RootExisted $toolingRootExisted
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
