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

function Assert-WriteAction {
  param(
    [Parameter(Mandatory=$true)]$Result,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Action
  )
  $writes = @($Result.writes | Where-Object { [string]$_.path -eq $Path -and [string]$_.action -eq $Action })
  if ($writes.Count -ne 1) { throw "expected exactly one write for $Path/$Action, got $($writes.Count)" }
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

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "pca-$suffix"
$packRoot = Join-Path $RepoRoot "packs\$Pack"
$promoteCandidateRoot = Join-Path $packRoot 'promote-candidates'
$toolingCandidateRoot = Join-Path $packRoot 'tooling\candidates'
$promoteRootExisted = Test-Path -LiteralPath $promoteCandidateRoot
$toolingRootExisted = Test-Path -LiteralPath $toolingCandidateRoot
$beforePromote = Save-TreeSnapshot -Path $promoteCandidateRoot
$beforeTooling = Save-TreeSnapshot -Path $toolingCandidateRoot
$beforePromoteDirs = Save-TreeDirectories -Path $promoteCandidateRoot
$beforeToolingDirs = Save-TreeDirectories -Path $toolingCandidateRoot
$beforePromoteTree = Get-TreeSnapshot -Path $promoteCandidateRoot
$beforeToolingTree = Get-TreeSnapshot -Path $toolingCandidateRoot
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"promote-apply-$suffix",'-Apply') | Out-Null

  $readme = Join-Path $caseRoot 'references\template\README.md'
  $workflow = Join-Path $caseRoot 'references\template\workflow-template.md'
  $tooling = Join-Path $caseRoot 'references\template\toolchain-router.md'
  [System.IO.File]::WriteAllText($readme, "# Template README`r`n`r`nReusable Go candidate from smoke.`r`n", [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText($workflow, "# Blocked workflow`r`n`r`nDo not promote C:\case\artifact\sample-trace.csv from this case.`r`n", [System.Text.UTF8Encoding]::new($false))
  $toolingText = @"
# Tooling source

Case root: $caseRoot
Absolute path: C:\cases\promote-apply\sample.exe
Artifacts path: artifacts/run1/demo-trace.csv
Captures path: captures/run1/demo-dump.bin
Address: 0x401000
Context: ctx123 round7 Task #99
"@
  [System.IO.File]::WriteAllText($tooling, $toolingText, [System.Text.UTF8Encoding]::new($false))

  $preview = Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf') | ConvertFrom-Json
  if ([bool]$preview.isMutation -or [bool]$preview.applied) { throw "unexpected what-if mutation: $($preview | ConvertTo-Json -Depth 10)" }
  Assert-WriteAction -Result $preview -Path 'references/template/README.md' -Action 'would-create-candidate' | Out-Null
  Assert-WriteAction -Result $preview -Path 'references/template/workflow-template.md' -Action 'blocked-deny-pattern' | Out-Null
  Assert-WriteAction -Result $preview -Path 'references/template/toolchain-router.md' -Action 'would-create-candidate' | Out-Null
  if ($beforePromoteTree -ne (Get-TreeSnapshot -Path $promoteCandidateRoot)) { throw 'Go promote what-if changed promote-candidates tree' }
  if ($beforeToolingTree -ne (Get-TreeSnapshot -Path $toolingCandidateRoot)) { throw 'Go promote what-if changed tooling candidates tree' }

  $facadeWhatIf = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '' }
  Assert-ContainsText -Text $facadeWhatIf -Expected 'PowerShell fallback has been retired' -Label 'facade promote text candidate no fallback'

  $result = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-CreateCandidates') | ConvertFrom-Json
  if (-not [bool]$result.applied -or [bool]$result.isMutation -ne $true) { throw "unexpected façade apply result: $($result | ConvertTo-Json -Depth 10)" }
  if ([int]$result.created -ne 2 -or [int]$result.blocked -lt 1 -or -not [bool]$result.requiresCleanup) { throw "unexpected counts: $($result | ConvertTo-Json -Depth 10)" }
  Assert-InsideRoot -Root $promoteCandidateRoot -Path ([string]$result.indexPath) -Label 'candidate index'
  if (-not (Test-Path -LiteralPath ([string]$result.indexPath))) { throw "missing candidate index: $($result.indexPath)" }

  $readmeWrite = Assert-WriteAction -Result $result -Path 'references/template/README.md' -Action 'create-candidate'
  $workflowWrite = Assert-WriteAction -Result $result -Path 'references/template/workflow-template.md' -Action 'blocked-deny-pattern'
  $toolingWrite = Assert-WriteAction -Result $result -Path 'references/template/toolchain-router.md' -Action 'create-candidate'
  Assert-InsideRoot -Root $promoteCandidateRoot -Path ([string]$readmeWrite.targetPath) -Label 'managed candidate'
  Assert-InsideRoot -Root $toolingCandidateRoot -Path ([string]$toolingWrite.targetPath) -Label 'tooling candidate'
  if (-not (Test-Path -LiteralPath ([string]$readmeWrite.targetPath))) { throw "missing managed candidate: $($readmeWrite.targetPath)" }
  if (-not (Test-Path -LiteralPath ([string]$toolingWrite.targetPath))) { throw "missing tooling candidate: $($toolingWrite.targetPath)" }
  if (Test-Path -LiteralPath ([string]$workflowWrite.targetPath)) {
    if ([string]$workflowWrite.targetPath -like "*.candidate.md") { throw 'blocked workflow unexpectedly wrote a candidate file' }
  }

  $index = Get-Content -LiteralPath ([string]$result.indexPath) -Raw | ConvertFrom-Json
  $readmeIndex = @($index | Where-Object { [string]$_.path -eq 'references/template/README.md' })
  if ($readmeIndex.Count -ne 1) { throw "candidate index missing README entry: $($index | ConvertTo-Json -Depth 8)" }
  if ([string]$readmeIndex[0].candidate -ne [string]$readmeWrite.targetPath) { throw 'candidate index does not point to README candidate' }

  $readmeCandidate = [System.IO.File]::ReadAllText([string]$readmeWrite.targetPath, [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $readmeCandidate -Expected 'Reusable Go candidate from smoke' -Label 'managed candidate content'
  $toolingCandidate = [System.IO.File]::ReadAllText([string]$toolingWrite.targetPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('<caseRoot>','<absolutePath>','<artifactsPath>','<capturesPath>','<address>','<ctxNNN>','<roundN>','Task #<n>')) {
    Assert-ContainsText -Text $toolingCandidate -Expected $expected -Label 'tooling candidate placeholders'
  }
  foreach ($unexpected in @($caseRoot,'C:\cases','demo-trace.csv','demo-dump.bin','0x401000','ctx123','round7','Task #99')) {
    Assert-NotContainsText -Text $toolingCandidate -Unexpected $unexpected -Label 'tooling candidate redaction'
  }

  'promote candidates apply smoke ok'
} finally {
  Restore-TreeSnapshot -Root $promoteCandidateRoot -BeforeSnapshot $beforePromote -BeforeDirectories $beforePromoteDirs -RootExisted $promoteRootExisted
  Restore-TreeSnapshot -Root $toolingCandidateRoot -BeforeSnapshot $beforeTooling -BeforeDirectories $beforeToolingDirs -RootExisted $toolingRootExisted
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
