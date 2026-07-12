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

function TextFromCodes {
  param([Parameter(Mandatory=$true)][int[]]$Codes)
  return (-join ($Codes | ForEach-Object { [char]$_ }))
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
    if (-not $afterSnapshot.ContainsKey($rel)) { throw "readonly overview removed file: $rel" }
    $beforeBytes = [byte[]]$BeforeSnapshot[$rel]
    $afterBytes = [byte[]]$afterSnapshot[$rel]
    if ($beforeBytes.Length -ne $afterBytes.Length) { throw "readonly overview changed file length: $rel" }
    for ($i = 0; $i -lt $beforeBytes.Length; $i++) {
      if ($beforeBytes[$i] -ne $afterBytes[$i]) { throw "readonly overview changed file content: $rel" }
    }
  }
  foreach ($rel in $afterSnapshot.Keys) {
    if (-not $BeforeSnapshot.ContainsKey($rel)) { throw "readonly overview created file: $rel" }
  }
  foreach ($rel in $BeforeDirectories.Keys) {
    if (-not $afterDirectories.ContainsKey($rel)) { throw "readonly overview removed directory: $rel" }
  }
  foreach ($rel in $afterDirectories.Keys) {
    if (-not $BeforeDirectories.ContainsKey($rel)) { throw "readonly overview created directory: $rel" }
  }
}

function Write-FactFile {
  param(
    [Parameter(Mandatory=$true)][string]$FactsRoot,
    [Parameter(Mandatory=$true)][string]$Name,
    [object[]]$Events = @()
  )
  $lines = @($Events | ForEach-Object { $_ | ConvertTo-Json -Compress -Depth 10 })
  $text = if ($lines.Count -gt 0) { ($lines -join "`n") + "`n" } else { '' }
  [System.IO.File]::WriteAllText((Join-Path $FactsRoot $Name), $text, [System.Text.UTF8Encoding]::new($false))
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "overview-readonly-$suffix"
try {
  Invoke-RekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"overview-readonly-$suffix") | Out-Null

  $goInitJson = Invoke-GoRekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') | ConvertFrom-Json
  if ([string]$goInitJson.command -ne 'overview' -or -not [bool]$goInitJson.isMutation -or @($goInitJson.lanes).Count -lt 1) { throw "unexpected Go overview initialization JSON: $($goInitJson | ConvertTo-Json -Depth 20)" }
  Remove-Item -LiteralPath (Join-Path $caseRoot '.rekit\board.json') -Force -Confirm:$false
  $facadeInit = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack) -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '' }
  $sectionProject = TextFromCodes @(39033,30446,27010,35272,65306)
  Assert-ContainsText -Text $facadeInit -Expected $sectionProject -Label 'facade overview default initialization'
  if (-not (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json'))) { throw 'facade overview default delegation did not initialize board.json' }

  $board = Get-Content -LiteralPath (Join-Path $caseRoot '.rekit\board.json') -Raw | ConvertFrom-Json
  $lane = [string]@($board.lanes)[0].id
  if ([string]::IsNullOrWhiteSpace($lane)) { throw 'overview initialization did not create a lane' }

  $factsRoot = Join-Path $caseRoot '.rekit\facts'
  $batch = "batch-overview-$suffix"
  Write-FactFile -FactsRoot $factsRoot -Name 'observations.jsonl' -Events @([ordered]@{ kind='observation'; lane=$lane; subject='obs'; summary='seen' })
  Write-FactFile -FactsRoot $factsRoot -Name 'candidates.jsonl' -Events @(
    [ordered]@{ kind='candidate'; lane=$lane; subject='smoke-handler'; summary='candidate one'; confidence='high'; status='open' },
    [ordered]@{ kind='candidate'; lane=$lane; subject='smoke-handler'; summary='candidate two'; confidence='medium'; status='open' }
  )
  Write-FactFile -FactsRoot $factsRoot -Name 'requests.jsonl' -Events @([ordered]@{ kind='request'; lane=$lane; subject='debug gate'; summary='needs confirmation'; status='pending-gate'; actor='overview-smoke'; risk='high'; target=$batch; batchId=$batch; gate=[ordered]@{ action='debug'; scope='handler only'; budget='30s'; triedLightSteps=@('overview','static review'); stopConditions=@('timeout') } })
  Write-FactFile -FactsRoot $factsRoot -Name 'publications.jsonl' -Events @([ordered]@{ kind='publication'; lane=$lane; subject='pub'; summary='published' })
  Write-FactFile -FactsRoot $factsRoot -Name 'decisions.jsonl' -Events @([ordered]@{ kind='decision'; lane=$lane; subject='decision subject'; decision='defer'; actor='overview-smoke'; reason='needs review'; batchId=$batch })
  Write-FactFile -FactsRoot $factsRoot -Name 'hypotheses.jsonl' -Events @()
  Write-FactFile -FactsRoot $factsRoot -Name 'verifications.jsonl' -Events @([ordered]@{ kind='verification'; lane=$lane; subject='review target'; actor='reviewer-smoke'; target='candidate-alpha'; verifier='manual-review'; verdict='accepted'; batchId=$batch })
  Write-FactFile -FactsRoot $factsRoot -Name 'interventions.jsonl' -Events @([ordered]@{ kind='intervention'; lane=$lane; subject='manual override'; summary='needs human'; action='override'; target=$batch; approvedBy='lead'; scope='metadata'; status='open'; batchId=$batch })
  Write-FactFile -FactsRoot $factsRoot -Name 'rollbacks.jsonl' -Events @([ordered]@{ kind='rollback'; lane=$lane; subject='rollback item'; target=$batch; status='resolved'; reason='cleanup'; batchId=$batch })

  $rekitRoot = Join-Path $caseRoot '.rekit'
  $beforeFiles = Save-TreeSnapshot -Path $rekitRoot
  $beforeDirs = Save-TreeDirectories -Path $rekitRoot
  $overview = Invoke-GoRekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack)
  $sectionLanes = TextFromCodes @(24037,20316,32447,65306)
  $sectionFacts = TextFromCodes @(20849,20139,20107,23454,65306)
  $sectionCandidates = TextFromCodes @(26410,20915,32,99,97,110,100,105,100,97,116,101,65306)
  $conflictMark = TextFromCodes @(91,20914,31361,93)
  $sectionVerification = TextFromCodes @(26368,36817,32,118,101,114,105,102,105,99,97,116,105,111,110,65306)
  $sectionDecisions = TextFromCodes @(26368,36817,32,100,101,99,105,115,105,111,110,65306)
  $sectionBatches = TextFromCodes @(26368,36817,32,98,97,116,99,104,65306)
  $sectionOpenInterventions = TextFromCodes @(26410,35299,20915,32,105,110,116,101,114,118,101,110,116,105,111,110,65306)
  $sectionInterventions = TextFromCodes @(26368,36817,32,105,110,116,101,114,118,101,110,116,105,111,110,65306)
  $sectionRollbacks = TextFromCodes @(26368,36817,32,114,111,108,108,98,97,99,107,65306)
  foreach ($expected in @($sectionProject,$sectionLanes,$sectionFacts,$sectionCandidates,'smoke-handler',$conflictMark,'pending-gate','by=overview-smoke','risk=high',"target=$batch",'action=debug','scope=handler only','budget=30s','tried=overview,static review','stop=timeout',$sectionVerification,'verifier=manual-review','verdict=accepted','target=candidate-alpha','by=reviewer-smoke',$sectionDecisions,'decision=defer',$sectionBatches,$batch,$sectionOpenInterventions,$sectionInterventions,$sectionRollbacks,'/rekit continue main')) {
    Assert-ContainsText -Text $overview -Expected $expected -Label 'go overview readonly summary'
  }
  Assert-TreeUnchanged -Root $rekitRoot -BeforeSnapshot $beforeFiles -BeforeDirectories $beforeDirs

  $overviewJson = Invoke-GoRekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') | ConvertFrom-Json
  if ([string]$overviewJson.command -ne 'overview' -or [bool]$overviewJson.isMutation -or [string]$overviewJson.pack -ne $Pack -or [int]$overviewJson.counts.candidates -ne 2 -or [int]$overviewJson.sections.pendingGates.total -ne 1) { throw "unexpected Go overview JSON: $($overviewJson | ConvertTo-Json -Depth 20)" }
  if (@($overviewJson.lanes).Count -lt 1 -or [string]@($overviewJson.lanes)[0].label -ne 'main' -or @($overviewJson.nextSteps).Count -lt 1) { throw "unexpected Go overview JSON lanes/nextSteps: $($overviewJson | ConvertTo-Json -Depth 20)" }
  Assert-TreeUnchanged -Root $rekitRoot -BeforeSnapshot $beforeFiles -BeforeDirectories $beforeDirs

  $fakeGo = Join-Path $caseRoot 'fake-rekit-go.cmd'
  [System.IO.File]::WriteAllText($fakeGo, ('@echo off' + "`r`n" + 'echo {"schemaVersion":1,"command":"overview","delegatedByFake":true}' + "`r`n"), [System.Text.UTF8Encoding]::new($false))
  $delegatedJson = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = ''; REKIT_GO_EXE = $fakeGo } | ConvertFrom-Json
  if (-not [bool]$delegatedJson.delegatedByFake) { throw "facade overview JSON did not use default REKIT_GO_EXE delegation: $($delegatedJson | ConvertTo-Json -Depth 20)" }

  $facadeJson = Invoke-RekitSmoke -Arguments @('-Command','overview','-Target',$caseRoot,'-Pack',$Pack,'-Format','json') -Env @{ REKIT_GO_ENABLE = ''; REKIT_GO_DISABLE = '' } | ConvertFrom-Json
  if ([string]$facadeJson.command -ne 'overview' -or [bool]$facadeJson.isMutation -or [string]$facadeJson.pack -ne $Pack -or [int]$facadeJson.counts.candidates -ne 2 -or [int]$facadeJson.sections.verifications.total -ne 1) { throw "unexpected facade overview JSON: $($facadeJson | ConvertTo-Json -Depth 20)" }
  Assert-TreeUnchanged -Root $rekitRoot -BeforeSnapshot $beforeFiles -BeforeDirectories $beforeDirs

  'overview readonly smoke ok'
} finally {
  if (Test-Path -LiteralPath $caseRoot) { Remove-Item -LiteralPath $caseRoot -Recurse -Force -Confirm:$false }
}
