param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string]$Pack = 'vmp-re'
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

function Write-Utf8File {
  param(
    [Parameter(Mandatory=$true)][string]$Path,
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text
  )
  $parent = Split-Path -Parent $Path
  if (-not (Test-Path -LiteralPath $parent)) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
  [System.IO.File]::WriteAllText($Path, $Text, [System.Text.UTF8Encoding]::new($false))
}

function Assert-PlanPacket {
  param(
    [Parameter(Mandatory=$true)]$Result,
    [Parameter(Mandatory=$true)][string]$Route,
    [Parameter(Mandatory=$true)][int]$Items,
    [Parameter(Mandatory=$true)][int]$Shards
  )
  if ([string]$Result.command -ne 'plan-subagents' -or [bool]$Result.isMutation -or -not [bool]$Result.writesReviewArtifacts -or -not [bool]$Result.reviewRequired) {
    throw "unexpected plan-subagents result flags: $($Result | ConvertTo-Json -Depth 10)"
  }
  if ([int]$Result.itemCount -ne $Items -or [int]$Result.shardCount -ne $Shards) {
    throw "unexpected plan-subagents counts: $($Result | ConvertTo-Json -Depth 10)"
  }
  foreach ($path in @([string]$Result.packetPath,[string]$Result.summaryPath)) {
    if (-not (Test-Path -LiteralPath $path)) { throw "missing plan-subagents artifact: $path" }
  }
  $packet = Get-Content -LiteralPath ([string]$Result.packetPath) -Raw | ConvertFrom-Json
  if ([string]$packet.route.id -ne $Route) { throw "unexpected route: $($packet | ConvertTo-Json -Depth 10)" }
  return $packet
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "plan-subagents-$suffix"
$outRoot = Join-Path $WorkRoot "plan-subagents-out-$suffix"
$facadeRoot = Join-Path $WorkRoot "plan-subagents-facade-$suffix"
$templateRoot = Join-Path $WorkRoot "plan-subagents-template-$suffix"
$itemsFile = Join-Path $WorkRoot "plan-subagents-items-$suffix.txt"
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"plan-subagents-$suffix",'-Apply') | Out-Null

  $go = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','feature-analysis','-Items','alpha,beta gamma','-ItemsPerAgent','2','-MaxParallel','7') | ConvertFrom-Json
  $packet = Assert-PlanPacket -Result $go -Route 'vmp-re:lane-feature-analysis' -Items 3 -Shards 2
  if ([int]$packet.shardPolicy.targetItemsPerAgent -ne 2 -or [int]$packet.shardPolicy.maxParallel -ne 7) { throw "unexpected shard policy: $($packet | ConvertTo-Json -Depth 10)" }
  if ((@($packet.shards)[0].items -join ',') -ne 'alpha,beta' -or (@($packet.shards)[1].items -join ',') -ne 'gamma') { throw "unexpected shards: $($packet | ConvertTo-Json -Depth 10)" }
  Assert-ContainsText -Text ([System.IO.File]::ReadAllText([string]$go.summaryPath, [System.Text.Encoding]::UTF8)) -Expected '# rekit subagent plan' -Label 'go plan summary'
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json')) { throw 'plan-subagents created board.json' }
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\facts')) { throw 'plan-subagents created facts' }
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\lanes')) { throw 'plan-subagents created lanes' }

  Write-Utf8File -Path $itemsFile -Text "one`ntwo;three"
  $guard = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$WorkRoot,'-Pack',$Pack,'-ItemsFile',$itemsFile) -AllowedExitCodes @(1)
  Assert-ContainsText -Text $guard -Expected 'unless -ReviewOutputDir' -Label 'go plan out-of-case guard'
  $outCase = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$WorkRoot,'-Pack',$Pack,'-Route','vmp-re:bounded-review','-ItemsFile',$itemsFile,'-ReviewOutputDir',$outRoot) | ConvertFrom-Json
  $outPacket = Assert-PlanPacket -Result $outCase -Route 'vmp-re:bounded-review' -Items 3 -Shards 1
  if ([string]$outPacket.input.itemsFile -ne $itemsFile) { throw "itemsFile was not preserved: $($outPacket | ConvertTo-Json -Depth 10)" }

  $missingRoutes = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$WorkRoot,'-Pack','_template','-ReviewOutputDir',$templateRoot) -AllowedExitCodes @(1)
  Assert-ContainsText -Text $missingRoutes -Expected 'no subagentRoutes' -Label 'go plan missing route guard'

  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null

  $facadeOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-Items','alpha,beta','-ReviewOutputDir',$facadeRoot) -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }
  Assert-ContainsText -Text $facadeOut -Expected 'review packet:' -Label 'facade plan-subagents fallback'
  Assert-NotContainsText -Text $facadeOut -Unexpected 'schemaVersion' -Label 'facade plan-subagents fallback'
  if (-not (Test-Path -LiteralPath (Join-Path $facadeRoot 'packet.json'))) { throw 'facade plan-subagents packet was not written' }

  'plan-subagents smoke ok'
} finally {
  foreach ($path in @($caseRoot,$outRoot,$facadeRoot,$templateRoot,$itemsFile)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
