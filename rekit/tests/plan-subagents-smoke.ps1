param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string]$Pack = 'binary-re'
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
    $global:LASTEXITCODE = 0
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
  if ([string]$Result.observability.dispatchMode -ne 'manual-main-agent' -or [string]$packet.observability.dispatchMode -ne 'manual-main-agent') { throw "missing plan observability: $($packet | ConvertTo-Json -Depth 20)" }
  if ([string]$packet.reviewLoop.spawnOwner -ne 'main-agent' -or [string]$packet.reviewLoop.mergeOwner -ne 'main-agent') { throw "missing review loop ownership: $($packet | ConvertTo-Json -Depth 20)" }
  if (@($packet.observability.shardStatuses).Count -ne $Shards) { throw "unexpected shard status count: $($packet | ConvertTo-Json -Depth 20)" }
  foreach ($status in @($packet.observability.shardStatuses)) {
    if ([string]$status.status -ne 'planned') { throw "unexpected shard status: $($packet | ConvertTo-Json -Depth 20)" }
  }
  return $packet
}

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "plan-subagents-$suffix"
$outRoot = Join-Path $WorkRoot "plan-subagents-out-$suffix"
$facadeRoot = Join-Path $WorkRoot "plan-subagents-facade-$suffix"
$templateRoot = Join-Path $WorkRoot "plan-subagents-template-$suffix"
$templateFacadeRoot = Join-Path $WorkRoot "plan-subagents-template-facade-$suffix"
$itemsFile = Join-Path $WorkRoot "plan-subagents-items-$suffix.txt"
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"plan-subagents-$suffix",'-Apply') | Out-Null

  $go = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','feature-analysis','-Items','alpha,beta gamma','-ItemsPerAgent','2','-MaxParallel','7') | ConvertFrom-Json
  $packet = Assert-PlanPacket -Result $go -Route 'binary-re:lane-feature-analysis' -Items 3 -Shards 2
  if ([int]$packet.shardPolicy.targetItemsPerAgent -ne 2 -or [int]$packet.shardPolicy.maxParallel -ne 7) { throw "unexpected shard policy: $($packet | ConvertTo-Json -Depth 10)" }
  if ((@($packet.shards)[0].items -join ',') -ne 'alpha,beta' -or (@($packet.shards)[1].items -join ',') -ne 'gamma') { throw "unexpected shards: $($packet | ConvertTo-Json -Depth 10)" }
  if ([string]$packet.ownerBinding.targetLane -ne 'devirt-main' -or [string]$packet.ownerBinding.bindingMode -eq '') { throw "missing owner binding: $($packet | ConvertTo-Json -Depth 20)" }
  if ((@(@($packet.shardHandoffs)[0].reviewerResultContract.requiredFields) -notcontains 'reviewerSession') -or [string]@($packet.shardHandoffs)[0].ownerBinding.targetLane -ne 'devirt-main') { throw "missing reviewer provenance contract: $($packet | ConvertTo-Json -Depth 20)" }
  $goSummary = [System.IO.File]::ReadAllText([string]$go.summaryPath, [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $goSummary -Expected '# rekit subagent plan' -Label 'go plan summary'
  Assert-ContainsText -Text $goSummary -Expected 'bounded dispatch observability' -Label 'go plan summary observability'
  Assert-ContainsText -Text $goSummary -Expected 'owner binding target lane' -Label 'go plan summary owner binding'
  Assert-ContainsText -Text $goSummary -Expected 'runtime does not spawn subagents' -Label 'go plan summary blocked actions'
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json')) { throw 'plan-subagents created board.json' }
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\facts')) { throw 'plan-subagents created facts' }
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\lanes')) { throw 'plan-subagents created lanes' }

  Write-Utf8File -Path $itemsFile -Text "one`ntwo;three"
  $guard = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$WorkRoot,'-Pack',$Pack,'-ItemsFile',$itemsFile) -AllowedExitCodes @(1)
  Assert-ContainsText -Text $guard -Expected 'unless -ReviewOutputDir' -Label 'go plan out-of-case guard'
  $outCase = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$WorkRoot,'-Pack',$Pack,'-Route','binary-re:bounded-review','-ItemsFile',$itemsFile,'-ReviewOutputDir',$outRoot) | ConvertFrom-Json
  $outPacket = Assert-PlanPacket -Result $outCase -Route 'binary-re:bounded-review' -Items 3 -Shards 1
  if ([string]$outPacket.input.itemsFile -ne $itemsFile) { throw "itemsFile was not preserved: $($outPacket | ConvertTo-Json -Depth 10)" }
  Assert-ContainsText -Text ([string](@($outPacket.shardHandoffs)[0].reviewerIntakeCommands.previewCommand)) -Expected 'reviewer intake requires an attached rekit case' -Label 'out-of-case reviewer intake preview disabled'
  Assert-ContainsText -Text ((@($outPacket.shardHandoffs)[0].reviewerIntakeCommands.blockedOutputs -join ';')) -Expected 'out-of-case plan packets must not be presented as immediately runnable reviewer intake commands' -Label 'out-of-case reviewer intake blocked output'

  $templatePlan = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$WorkRoot,'-Pack','_template','-TaskType','feature-analysis','-Items','one,two','-ReviewOutputDir',$templateRoot) | ConvertFrom-Json
  $templatePacket = Assert-PlanPacket -Result $templatePlan -Route '_template:lane-feature-analysis' -Items 2 -Shards 2
  if ([string]$templatePacket.observability.routeDebug.selectedBy -ne 'taskType') { throw "template route was not selected by taskType: $($templatePacket | ConvertTo-Json -Depth 20)" }
  $templateSummary = [System.IO.File]::ReadAllText([string]$templatePlan.summaryPath, [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $templateSummary -Expected 'bounded dispatch observability' -Label 'template plan summary observability'

  $templateFacadeOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$WorkRoot,'-Pack','_template','-TaskType','feature-analysis','-Items','one,two','-ReviewOutputDir',$templateFacadeRoot)
  $templateFacadeResult = $templateFacadeOut | ConvertFrom-Json
  $templateFacadePacket = Get-Content -LiteralPath ([string]$templateFacadeResult.packetPath) -Raw | ConvertFrom-Json
  if ([string]$templateFacadePacket.route.id -ne '_template:lane-feature-analysis' -or [string]$templateFacadePacket.observability.routeDebug.selectedBy -ne 'taskType') { throw "template facade route mismatch: $($templateFacadePacket | ConvertTo-Json -Depth 20)" }

  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null

  $facadeOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-Items','alpha,beta','-ReviewOutputDir',$facadeRoot)
  $facadeResult = $facadeOut | ConvertFrom-Json
  if (-not [bool]$facadeResult.writesReviewArtifacts -or [bool]$facadeResult.isMutation) { throw "facade plan-subagents did not return Go result: $facadeOut" }
  $facadePacketPath = [string]$facadeResult.packetPath
  if (-not (Test-Path -LiteralPath $facadePacketPath)) { throw 'facade plan-subagents packet was not written' }
  $facadePacket = Get-Content -LiteralPath $facadePacketPath -Raw | ConvertFrom-Json
  if ([string]$facadePacket.observability.dispatchMode -ne 'manual-main-agent' -or [string]$facadePacket.reviewLoop.spawnOwner -ne 'main-agent' -or [string]$facadePacket.reviewLoop.mergeOwner -ne 'main-agent') { throw "facade packet missing observability: $($facadePacket | ConvertTo-Json -Depth 20)" }
  foreach ($expected in @('runtime does not spawn subagents','subagents must not write files','authority, and confirmed writes')) {
    Assert-ContainsText -Text (@($facadePacket.observability.blockedActions) -join ';') -Expected $expected -Label 'facade blocked actions'
  }
  Assert-ContainsText -Text ([string]$facadePacket.reviewLoop.verdictWriteback) -Expected 'plan-subagents -ReviewerResultPath' -Label 'facade verdict writeback'
  Assert-ContainsText -Text ([string](@($facadePacket.shardHandoffs)[0].reviewerIntakeCommands.previewCommand)) -Expected '-Lane "devirt-main"' -Label 'facade reviewer intake target lane'
  if (@($facadePacket.reviewLoop.completionCriteria).Count -lt 3 -or @($facadePacket.observability.shardStatuses).Count -ne 1 -or [string]@($facadePacket.observability.shardStatuses)[0].status -ne 'planned') { throw "facade packet missing review loop details: $($facadePacket | ConvertTo-Json -Depth 20)" }
  $facadeSummary = [System.IO.File]::ReadAllText([string]$facadeResult.summaryPath, [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $facadeSummary -Expected 'bounded dispatch observability' -Label 'facade plan summary observability'
  Assert-ContainsText -Text $facadeSummary -Expected 'runtime does not spawn subagents' -Label 'facade plan summary blocked action'
  Assert-ContainsText -Text $facadeSummary -Expected 'completion criteria' -Label 'facade plan summary completion criteria'
  foreach ($unexpectedPath in @('.rekit\board.json','.rekit\facts','.rekit\lanes','.rekit\handovers','captures\vm_opcode_semantics_confirmed.csv')) {
    if (Test-Path -LiteralPath (Join-Path $caseRoot $unexpectedPath)) { throw "facade plan-subagents created unexpected case state: $unexpectedPath" }
  }

  $fallbackRoot = Join-Path $WorkRoot "plan-subagents-fallback-$suffix"
  $fallbackOut = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-Items','fallback-alpha,fallback-beta','-ReviewOutputDir',$fallbackRoot) -AllowedExitCodes @(1) -Env @{ REKIT_GO_DISABLE = '1' }
  Assert-ContainsText -Text $fallbackOut -Expected 'PowerShell fallback has been retired' -Label 'facade plan-subagents disabled no fallback'
  if (Test-Path -LiteralPath (Join-Path $fallbackRoot 'packet.json')) { throw 'retired fallback plan-subagents packet was written' }

  'plan-subagents smoke ok'
} finally {
  foreach ($path in @($caseRoot,$outRoot,$facadeRoot,$templateRoot,$templateFacadeRoot,$fallbackRoot,$itemsFile)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
