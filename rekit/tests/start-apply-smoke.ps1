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
    if (-not $afterSnapshot.ContainsKey($rel)) { throw "start preview removed file: $rel" }
    $beforeBytes = [byte[]]$BeforeSnapshot[$rel]
    $afterBytes = [byte[]]$afterSnapshot[$rel]
    if ($beforeBytes.Length -ne $afterBytes.Length) { throw "start preview changed file length: $rel" }
    for ($i = 0; $i -lt $beforeBytes.Length; $i++) {
      if ($beforeBytes[$i] -ne $afterBytes[$i]) { throw "start preview changed file content: $rel" }
    }
  }
  foreach ($rel in $afterSnapshot.Keys) {
    if (-not $BeforeSnapshot.ContainsKey($rel)) { throw "start preview created file: $rel" }
  }
  foreach ($rel in $BeforeDirectories.Keys) {
    if (-not $afterDirectories.ContainsKey($rel)) { throw "start preview removed directory: $rel" }
  }
  foreach ($rel in $afterDirectories.Keys) {
    if (-not $BeforeDirectories.ContainsKey($rel)) { throw "start preview created directory: $rel" }
  }
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "start-apply-$suffix"
$facadeRoot = Join-Path $WorkRoot "start-facade-$suffix"
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"start-$suffix",'-Apply') | Out-Null
  $rekitRoot = Join-Path $caseRoot '.rekit'
  $beforeFiles = Save-TreeSnapshot -Path $rekitRoot
  $beforeDirs = Save-TreeDirectories -Path $rekitRoot

  $preview = Invoke-GoRekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','login','-WhatIf') | ConvertFrom-Json
  if ([bool]$preview.isMutation -or [bool]$preview.applied -or -not [bool]$preview.requiresConfirmation) { throw "unexpected start preview flags: $($preview | ConvertTo-Json -Depth 10)" }
  if ([string]$preview.lane.id -ne 'feature-login') { throw "unexpected preview lane id: $($preview.lane.id)" }
  Assert-WriteAction -Result $preview -Path '.rekit/lanes/feature-login/lane.json' -Action 'would-create-lane' | Out-Null
  Assert-TreeUnchanged -Root $rekitRoot -BeforeSnapshot $beforeFiles -BeforeDirectories $beforeDirs

  $apply = Invoke-GoRekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','login','-Apply') | ConvertFrom-Json
  if (-not [bool]$apply.isMutation -or -not [bool]$apply.applied -or [string]$apply.lane.id -ne 'feature-login') { throw "unexpected start apply result: $($apply | ConvertTo-Json -Depth 10)" }
  Assert-WriteAction -Result $apply -Path '.rekit/policy.yml' -Action 'create-policy' | Out-Null
  Assert-WriteAction -Result $apply -Path '.rekit/lanes/main/lane.json' -Action 'create-lane' | Out-Null
  Assert-WriteAction -Result $apply -Path '.rekit/lanes/feature-login/lane.json' -Action 'create-lane' | Out-Null
  Assert-WriteAction -Result $apply -Path '.rekit/board.json' -Action 'refresh' | Out-Null
  foreach ($rel in @('.rekit\board.json','.rekit\facts\observations.jsonl','.rekit\lanes\main\lane.json','.rekit\lanes\feature-login\lane.json','.rekit\lanes\feature-login\prompts\RESUME.md','workspace\features\feature-login\summary.md')) {
    $path = Join-Path $caseRoot $rel
    if (-not (Test-Path -LiteralPath $path)) { throw "missing start artifact: $rel" }
  }
  $board = Get-Content -LiteralPath (Join-Path $caseRoot '.rekit\board.json') -Raw | ConvertFrom-Json
  if (@($board.lanes | Where-Object { [string]$_.id -eq 'feature-login' }).Count -ne 1) { throw 'board did not include feature-login lane' }
  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null

  $again = Invoke-GoRekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','login','-Apply') | ConvertFrom-Json
  Assert-WriteAction -Result $again -Path '.rekit/lanes/feature-login/lane.json' -Action 'enter-existing-lane' | Out-Null

  $laneRoot = Join-Path $caseRoot '.rekit\lanes\feature-login'
  [System.IO.File]::WriteAllText((Join-Path $laneRoot 'inbox.jsonl'), ('{"eventId":"in-1","summary":"review queued"}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  [System.IO.File]::WriteAllText((Join-Path $laneRoot 'tasks.jsonl'), ('{"taskId":"task-1","summary":"inspect candidate","status":"open"}' + "`r`n" + '{"taskId":"task-2","summary":"closed task","status":"closed"}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $forced = Invoke-GoRekitSmoke -Arguments @('-Command','start','-Target',$caseRoot,'-Pack',$Pack,'-Name','login','-Apply','-Force') | ConvertFrom-Json
  Assert-WriteAction -Result $forced -Path '.rekit/lanes/feature-login/lane.json' -Action 'refresh-lane-with-force' | Out-Null
  Assert-WriteAction -Result $forced -Path '.rekit/lanes/feature-login/events.jsonl' -Action 'append-lane-refreshed' | Out-Null
  $resume = [System.IO.File]::ReadAllText((Join-Path $laneRoot 'prompts\RESUME.md'), [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $resume -Expected 'review queued' -Label 'force refresh resume inbox'
  Assert-ContainsText -Text $resume -Expected 'inspect candidate' -Label 'force refresh resume task'
  Assert-NotContainsText -Text $resume -Unexpected 'closed task' -Label 'force refresh resume closed task'

  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$facadeRoot,'-Pack',$Pack,'-ProjectName',"start-facade-$suffix",'-Apply') | Out-Null
  $facadeOut = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$facadeRoot,'-Pack',$Pack,'-WhatIf','facade') -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }
  Assert-ContainsText -Text $facadeOut -Expected 'would create or enter feature workstream' -Label 'facade start fallback'
  Assert-NotContainsText -Text $facadeOut -Unexpected 'schemaVersion' -Label 'facade start fallback'

  $facadeRekitRoot = Join-Path $facadeRoot '.rekit'
  $beforeFacadeFiles = Save-TreeSnapshot -Path $facadeRekitRoot
  $beforeFacadeDirs = Save-TreeDirectories -Path $facadeRekitRoot
  $facadePreviewJson = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$facadeRoot,'-Pack',$Pack,'-WhatIf','facade','-Format','json') | ConvertFrom-Json
  if ([string]$facadePreviewJson.command -ne 'start' -or [bool]$facadePreviewJson.isMutation -or [bool]$facadePreviewJson.applied -or -not [bool]$facadePreviewJson.requiresConfirmation -or [string]$facadePreviewJson.lane.id -ne 'feature-facade') { throw "unexpected facade start JSON preview: $($facadePreviewJson | ConvertTo-Json -Depth 20)" }
  Assert-ContainsText -Text ([string]::Join("`n", @($facadePreviewJson.nextSteps))) -Expected 'JSON preview/apply is Go-owned by default' -Label 'facade start JSON delegated to Go by default'
  Assert-WriteAction -Result $facadePreviewJson -Path '.rekit/lanes/feature-facade/lane.json' -Action 'would-create-lane' | Out-Null
  Assert-TreeUnchanged -Root $facadeRekitRoot -BeforeSnapshot $beforeFacadeFiles -BeforeDirectories $beforeFacadeDirs

  $facadeApplyJson = Invoke-RekitSmoke -Arguments @('-Command','start','-Target',$facadeRoot,'-Pack',$Pack,'-Apply','-Format','json','facade') | ConvertFrom-Json
  if ([string]$facadeApplyJson.command -ne 'start' -or -not [bool]$facadeApplyJson.isMutation -or -not [bool]$facadeApplyJson.applied -or [string]$facadeApplyJson.lane.id -ne 'feature-facade') { throw "unexpected facade start JSON apply: $($facadeApplyJson | ConvertTo-Json -Depth 20)" }
  Assert-WriteAction -Result $facadeApplyJson -Path '.rekit/lanes/feature-facade/lane.json' -Action 'create-lane' | Out-Null

  'start apply smoke ok'
} finally {
  foreach ($path in @($caseRoot,$facadeRoot)) {
    if (Test-Path -LiteralPath $path) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
