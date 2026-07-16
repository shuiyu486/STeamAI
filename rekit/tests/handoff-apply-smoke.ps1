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
    [Parameter(Mandatory=$true)][hashtable]$BeforeDirectories
  )
  $afterSnapshot = Save-TreeSnapshot -Path $Root
  $afterDirectories = Save-TreeDirectories -Path $Root
  foreach ($rel in $BeforeSnapshot.Keys) {
    if (-not $afterSnapshot.ContainsKey($rel)) { throw "handoff preview removed file: $rel" }
    $beforeBytes = [byte[]]$BeforeSnapshot[$rel]
    $afterBytes = [byte[]]$afterSnapshot[$rel]
    if ($beforeBytes.Length -ne $afterBytes.Length) { throw "handoff preview changed file length: $rel" }
    for ($i = 0; $i -lt $beforeBytes.Length; $i++) {
      if ($beforeBytes[$i] -ne $afterBytes[$i]) { throw "handoff preview changed file content: $rel" }
    }
  }
  foreach ($rel in $afterSnapshot.Keys) {
    if (-not $BeforeSnapshot.ContainsKey($rel)) { throw "handoff preview created file: $rel" }
  }
  foreach ($rel in $BeforeDirectories.Keys) {
    if (-not $afterDirectories.ContainsKey($rel)) { throw "handoff preview removed directory: $rel" }
  }
  foreach ($rel in $afterDirectories.Keys) {
    if (-not $BeforeDirectories.ContainsKey($rel)) { throw "handoff preview created directory: $rel" }
  }
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

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "handoff-apply-$suffix"
$facadeRoot = Join-Path $WorkRoot "handoff-facade-$suffix"
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"handoff-$suffix",'-Apply') | Out-Null
  Invoke-GoRekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','login','-Apply') | Out-Null

  $laneRoot = Join-Path $caseRoot '.rekit\lanes\feature-login'
  $workspace = Join-Path $caseRoot 'workspace\features\feature-login'
  $factsRoot = Join-Path $caseRoot '.rekit\facts'
  Write-Utf8File -Path (Join-Path $laneRoot 'inbox.jsonl') -Text ('{"eventId":"in-1","summary":"review queued"}' + "`r`n")
  Write-Utf8File -Path (Join-Path $laneRoot 'tasks.jsonl') -Text ('{"taskId":"task-1","summary":"inspect candidate","status":"open"}' + "`r`n")
  Write-Utf8File -Path (Join-Path $workspace 'packet.md') -Text "# packet`r`n"
  Write-Utf8File -Path (Join-Path $factsRoot 'verifications.jsonl') -Text ('{"kind":"verification","lane":"feature-login","subject":"review target","actor":"reviewer-smoke","target":"candidate-alpha","verifier":"manual-review","verdict":"accepted","batchId":"batch-handoff"}' + "`r`n")
  Write-Utf8File -Path (Join-Path $factsRoot 'decisions.jsonl') -Text ('{"kind":"decision","lane":"feature-login","subject":"decision subject","decision":"defer","actor":"runtime-test","reason":"needs review","batchId":"batch-handoff"}' + "`r`n")
  Write-Utf8File -Path (Join-Path $factsRoot 'requests.jsonl') -Text ('{"kind":"request","lane":"feature-login","subject":"debug gate","summary":"needs confirmation","status":"pending-gate","actor":"runtime-test","risk":"high","target":"batch-handoff","batchId":"batch-handoff","gate":{"action":"debug","scope":"handler only","budget":"30s","triedLightSteps":["overview","static review"],"stopConditions":["timeout"]}}' + "`r`n")
  Write-Utf8File -Path (Join-Path $factsRoot 'interventions.jsonl') -Text ('{"kind":"intervention","lane":"feature-login","subject":"manual override","action":"override","target":"batch-handoff","approvedBy":"lead","scope":"metadata","status":"open","batchId":"batch-handoff"}' + "`r`n")
  Write-Utf8File -Path (Join-Path $factsRoot 'rollbacks.jsonl') -Text ('{"kind":"rollback","lane":"feature-login","subject":"rollback item","target":"batch-handoff","status":"resolved","reason":"cleanup","batchId":"batch-handoff"}' + "`r`n")

  $rekitRoot = Join-Path $caseRoot '.rekit'
  $beforeFiles = Save-TreeSnapshot -Path $rekitRoot
  $beforeDirs = Save-TreeDirectories -Path $rekitRoot
  $preview = Invoke-GoRekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf') | ConvertFrom-Json
  if ([bool]$preview.isMutation -or [bool]$preview.applied -or -not [bool]$preview.requiresConfirmation -or -not [bool]$preview.project) { throw "unexpected handoff preview flags: $($preview | ConvertTo-Json -Depth 10)" }
  Assert-WriteAction -Result $preview -Path '.rekit/handovers/latest.md' -Action 'would-write-latest-project-handoff' | Out-Null
  Assert-TreeUnchanged -Root $rekitRoot -BeforeSnapshot $beforeFiles -BeforeDirectories $beforeDirs

  $facadePreviewJson = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-WhatIf','login','-Format','json') | ConvertFrom-Json
  if ([string]$facadePreviewJson.command -ne 'handoff' -or [bool]$facadePreviewJson.isMutation -or [bool]$facadePreviewJson.applied -or [bool]$facadePreviewJson.project -or [string]$facadePreviewJson.lane.id -ne 'feature-login') { throw "unexpected facade handoff JSON preview: $($facadePreviewJson | ConvertTo-Json -Depth 20)" }
  Assert-ContainsText -Text ([string]::Join("`n", @($facadePreviewJson.nextSteps))) -Expected 'JSON preview/apply is Go-owned by default' -Label 'facade handoff JSON delegated to Go by default'
  Assert-WriteAction -Result $facadePreviewJson -Path '.rekit/handovers/feature-login-latest.md' -Action 'would-write-latest-lane-handoff' | Out-Null
  Assert-TreeUnchanged -Root $rekitRoot -BeforeSnapshot $beforeFiles -BeforeDirectories $beforeDirs

  $project = Invoke-GoRekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-Apply') | ConvertFrom-Json
  if (-not [bool]$project.isMutation -or -not [bool]$project.applied -or -not [bool]$project.project) { throw "unexpected project handoff result: $($project | ConvertTo-Json -Depth 10)" }
  Assert-WriteAction -Result $project -Path '.rekit/handovers/latest.md' -Action 'write-latest-project-handoff' | Out-Null
  $projectText = [System.IO.File]::ReadAllText((Join-Path $caseRoot '.rekit\handovers\latest.md'), [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $projectText -Expected '# rekit ' -Label 'project handoff title'
  Assert-ContainsText -Text $projectText -Expected '/rekit continue main' -Label 'project handoff main selector'
  Assert-ContainsText -Text $projectText -Expected '/rekit handoff login' -Label 'project handoff feature selector'

  $lane = Invoke-GoRekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-Apply','login') | ConvertFrom-Json
  if (-not [bool]$lane.isMutation -or -not [bool]$lane.applied -or [bool]$lane.project -or [string]$lane.lane.id -ne 'feature-login') { throw "unexpected lane handoff result: $($lane | ConvertTo-Json -Depth 10)" }
  Assert-WriteAction -Result $lane -Path '.rekit/handovers/feature-login-latest.md' -Action 'write-latest-lane-handoff' | Out-Null
  $laneText = [System.IO.File]::ReadAllText((Join-Path $caseRoot '.rekit\handovers\feature-login-latest.md'), [System.Text.Encoding]::UTF8)
  foreach ($expected in @('# rekit ','feature-login','workspace/features/feature-login/packet.md','## verification','verifier=manual-review','verdict=accepted','target=candidate-alpha','by=reviewer-smoke','## decision','by=runtime-test','## pending-gate','action=debug','## intervention','## rollback')) {
    Assert-ContainsText -Text $laneText -Expected $expected -Label 'lane handoff content'
  }
  $resume = [System.IO.File]::ReadAllText((Join-Path $laneRoot 'prompts\RESUME.md'), [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $resume -Expected 'review queued' -Label 'handoff refreshed resume inbox'
  Assert-ContainsText -Text $resume -Expected 'inspect candidate' -Label 'handoff refreshed resume task'

  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null

  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$facadeRoot,'-Pack',$Pack,'-ProjectName',"handoff-facade-$suffix",'-Apply') | Out-Null
  Invoke-GoRekitSmoke -Arguments @('-Command','start','-Target',$facadeRoot,'-Pack',$Pack,'-Name','facade','-Apply') | Out-Null
  $facadeOut = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$facadeRoot,'-Pack',$Pack,'-WhatIf','facade') -Env @{ REKIT_GO_DISABLE = '1' }
  Assert-ContainsText -Text $facadeOut -Expected 'would write workstream handoff: facade' -Label 'facade handoff fallback'
  Assert-NotContainsText -Text $facadeOut -Unexpected 'schemaVersion' -Label 'facade handoff fallback'
  $facadeRekitRoot = Join-Path $facadeRoot '.rekit'
  $disabledBeforeFiles = Save-TreeSnapshot -Path $facadeRekitRoot
  $disabledBeforeDirs = Save-TreeDirectories -Path $facadeRekitRoot
  $disabledPreviewOut = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$facadeRoot,'-Pack',$Pack,'-WhatIf','facade','-Format','json') -AllowedExitCodes @(1) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '1' }
  Assert-ContainsText -Text $disabledPreviewOut -Expected 'PowerShell fallback has been retired' -Label 'go disabled handoff JSON no fallback'
  Assert-TreeUnchanged -Root $facadeRekitRoot -BeforeSnapshot $disabledBeforeFiles -BeforeDirectories $disabledBeforeDirs
  $facadeApplyJson = Invoke-RekitSmoke -Arguments @('-Command','handoff','-Target',$caseRoot,'-Pack',$Pack,'-Apply','-Format','json','login') | ConvertFrom-Json
  if ([string]$facadeApplyJson.command -ne 'handoff' -or -not [bool]$facadeApplyJson.isMutation -or -not [bool]$facadeApplyJson.applied -or [bool]$facadeApplyJson.project -or [string]$facadeApplyJson.lane.id -ne 'feature-login') { throw "unexpected facade handoff JSON apply: $($facadeApplyJson | ConvertTo-Json -Depth 20)" }
  Assert-WriteAction -Result $facadeApplyJson -Path '.rekit/handovers/feature-login-latest.md' -Action 'write-latest-lane-handoff' | Out-Null

  'handoff apply smoke ok'
} finally {
  foreach ($path in @($caseRoot,$facadeRoot)) {
    if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
