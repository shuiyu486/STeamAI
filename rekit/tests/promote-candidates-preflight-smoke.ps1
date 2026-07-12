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

function Get-PromoteItem {
  param(
    [Parameter(Mandatory=$true)]$Packet,
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][string]$Kind
  )
  $items = @($Packet.items | Where-Object { [string]$_.path -eq $Path -and [string]$_.kind -eq $Kind })
  if ($items.Count -ne 1) { throw "expected exactly one promote item for $Path/$Kind, got $($items.Count)" }
  return $items[0]
}

function Get-ToolingItem {
  param(
    [Parameter(Mandatory=$true)]$Packet,
    [Parameter(Mandatory=$true)][string]$Path
  )
  $items = @($Packet.toolingCandidateSources | Where-Object { [string]$_.path -eq $Path })
  if ($items.Count -ne 1) { throw "expected exactly one tooling item for $Path, got $($items.Count)" }
  return $items[0]
}

function Get-TreeSnapshot {
  param([Parameter(Mandatory=$true)][string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return '' }
  $root = [System.IO.Path]::GetFullPath($Path).TrimEnd('\')
  $files = @(Get-ChildItem -LiteralPath $Path -Recurse -File | ForEach-Object { $_.FullName.Substring($root.Length).TrimStart('\') } | Sort-Object)
  return ($files -join "`n")
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "pcp-$suffix"
$reviewRoot = Join-Path $caseRoot '.rekit\reviews\go-promote-review'
$packRoot = Join-Path $RepoRoot "packs\$Pack"
$promoteCandidateRoot = Join-Path $packRoot 'promote-candidates'
$toolingCandidateRoot = Join-Path $packRoot 'tooling\candidates'
$beforePromote = Get-TreeSnapshot -Path $promoteCandidateRoot
$beforeTooling = Get-TreeSnapshot -Path $toolingCandidateRoot
try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"promote-preflight-$suffix") | Out-Null

  $readme = Join-Path $caseRoot 'references\template\README.md'
  $workflow = Join-Path $caseRoot 'references\template\workflow-template.md'
  $tooling = Join-Path $caseRoot 'references\template\toolchain-router.md'
  [System.IO.File]::WriteAllText($readme, "# Template README`r`n`r`nReusable safe candidate from smoke.`r`n", [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText($workflow, "# Blocked workflow`r`n`r`nDo not promote C:\\case\\artifact\\sample-trace.csv from this case.`r`n", [System.Text.UTF8Encoding]::new($false))
  $toolingText = @"
# Tooling source

Case root: $caseRoot
Absolute path: C:\cases\promote-preflight\sample.exe
Artifacts path: artifacts/run1/demo-trace.csv
Captures path: captures/run1/demo-dump.bin
Address: 0x401000
Context: ctx123 round7 Task #99
"@
  [System.IO.File]::WriteAllText($tooling, $toolingText, [System.Text.UTF8Encoding]::new($false))

  $psWhatIf = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf')
  Assert-ContainsText -Text $psWhatIf -Expected 'would promote candidate: references/template/README.md' -Label 'PowerShell managed candidate what-if'
  Assert-ContainsText -Text $psWhatIf -Expected 'blocked promote: references/template/workflow-template.md' -Label 'PowerShell blocked candidate what-if'
  Assert-ContainsText -Text $psWhatIf -Expected 'would write tooling candidate:' -Label 'PowerShell tooling candidate what-if'

  $fakeGo = Join-Path $caseRoot 'fake-rekit-go.cmd'
  [System.IO.File]::WriteAllText($fakeGo, ('@echo off' + "`r`n" + 'echo {"schemaVersion":1,"command":"promote","delegatedByFake":true}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $facadeJsonPreview = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeJsonPreview.delegatedByFake) { throw "facade promote create-candidates JSON preview did not use default REKIT_GO_EXE delegation: $($facadeJsonPreview | ConvertTo-Json -Depth 8)" }

  $facadeCreateCandidates = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-CreateCandidates') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeCreateCandidates.delegatedByFake) { throw "facade promote create-candidates did not use default REKIT_GO_EXE delegation: $($facadeCreateCandidates | ConvertTo-Json -Depth 8)" }

  $facadeReview = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-ReviewOutputDir',$reviewRoot) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$facadeReview.delegatedByFake) { throw "facade promote review did not use default REKIT_GO_EXE delegation: $($facadeReview | ConvertTo-Json -Depth 8)" }

  $facadeWhatIf = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo }
  Assert-ContainsText -Text $facadeWhatIf -Expected 'would promote candidate: references/template/README.md' -Label 'facade promote text candidate fallback'
  Assert-ContainsText -Text $facadeWhatIf -Expected 'would write tooling candidate:' -Label 'facade tooling text candidate fallback'

  $disabledJsonPreview = Invoke-RekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf','-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1'; REKIT_GO_EXE = $fakeGo }
  Assert-NotContainsText -Text $disabledJsonPreview -Unexpected 'delegatedByFake' -Label 'facade promote candidate JSON preview disabled fallback'
  Assert-ContainsText -Text $disabledJsonPreview -Expected 'would promote candidate: references/template/README.md' -Label 'facade promote candidate JSON preview disabled fallback'

  $goWhatIf = Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-CreateCandidates','-WhatIf') | ConvertFrom-Json
  if ([bool]$goWhatIf.isMutation -or [bool]$goWhatIf.applied) { throw "Go promote create-candidates what-if reported mutation: $($goWhatIf | ConvertTo-Json -Depth 8)" }
  if ([int]$goWhatIf.created -lt 2 -or [int]$goWhatIf.blocked -lt 1) { throw "Go promote create-candidates what-if had unexpected counts: $($goWhatIf | ConvertTo-Json -Depth 8)" }

  Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-ReviewOutputDir',$reviewRoot) | Out-Null
  $packetPath = Join-Path $reviewRoot 'packet.json'
  if (-not (Test-Path -LiteralPath $packetPath)) { throw "missing Go promote packet: $packetPath" }
  $packet = Get-Content -LiteralPath $packetPath -Raw | ConvertFrom-Json
  if ([bool]$packet.isMutation) { throw 'Go promote review reported mutation' }

  $readmeItem = Get-PromoteItem -Packet $packet -Path 'references/template/README.md' -Kind 'managed-doc'
  if ([string]$readmeItem.action -ne 'candidate-after-llm-review') { throw "README action was '$($readmeItem.action)'" }
  $workflowItem = Get-PromoteItem -Packet $packet -Path 'references/template/workflow-template.md' -Kind 'managed-doc'
  if ([string]$workflowItem.action -ne 'blocked-deny-pattern') { throw "workflow action was '$($workflowItem.action)'" }

  $toolingItem = Get-ToolingItem -Packet $packet -Path 'references/template/toolchain-router.md'
  if ([string]$toolingItem.action -ne 'sanitized-preview-for-llm-review') { throw "tooling action was '$($toolingItem.action)'" }
  $previewPath = [string]$toolingItem.sanitizedPreviewPath
  if ([string]::IsNullOrWhiteSpace($previewPath) -or -not (Test-Path -LiteralPath $previewPath)) { throw "missing sanitized preview: $previewPath" }
  $preview = [System.IO.File]::ReadAllText($previewPath, [System.Text.Encoding]::UTF8)
  foreach ($expected in @('<caseRoot>','<absolutePath>','<artifactsPath>','<capturesPath>','<address>','<ctxNNN>','<roundN>','Task #<n>')) {
    Assert-ContainsText -Text $preview -Expected $expected -Label 'sanitized preview placeholders'
  }
  foreach ($unexpected in @($caseRoot,'C:\cases','demo-trace.csv','demo-dump.bin','0x401000','ctx123','round7','Task #99')) {
    Assert-NotContainsText -Text $preview -Unexpected $unexpected -Label 'sanitized preview redaction'
  }

  $combined = [System.IO.File]::ReadAllText((Join-Path $reviewRoot 'diffs\combined.diff'), [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $combined -Expected 'Reusable safe candidate from smoke' -Label 'Go promote bounded diff'

  $afterPromote = Get-TreeSnapshot -Path $promoteCandidateRoot
  $afterTooling = Get-TreeSnapshot -Path $toolingCandidateRoot
  if ($beforePromote -ne $afterPromote) { throw 'PowerShell what-if or Go review changed promote-candidates tree' }
  if ($beforeTooling -ne $afterTooling) { throw 'PowerShell what-if or Go review changed tooling candidates tree' }

  'promote candidates preflight smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
