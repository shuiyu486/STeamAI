param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
$RepoRoot = Split-Path -Parent $RekitRoot
$Rekit = Join-Path $RekitRoot 'rekit.ps1'
$Pack = 'ollvm'

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
  if ($AllowedExitCodes -notcontains $exitCode) { throw "unexpected exit code $exitCode; output:`n$output" }
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

Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
$suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
$caseRoot = Join-Path $WorkRoot "ol-$suffix"
$reviewRoot = Join-Path $WorkRoot "ol-review-$suffix"
$facadeReviewRoot = Join-Path $WorkRoot "ol-facade-review-$suffix"
$promoteReviewRoot = Join-Path $WorkRoot "ol-promote-review-$suffix"
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Pack',$Pack) | Out-Null

  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"ollvm-$suffix",'-Apply') | Out-Null
  foreach ($rel in @('references\ollvm\README.md','references\ollvm\agent-team.md','references\ollvm\workflow-template.md','references\ollvm\toolchain-router.md','references\ollvm\task-handoff.md','CLAUDE.local.md')) {
    if (-not (Test-Path -LiteralPath (Join-Path $caseRoot $rel))) { throw "missing ollvm case file: $rel" }
  }
  $local = [System.IO.File]::ReadAllText((Join-Path $caseRoot 'CLAUDE.local.md'), [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $local -Expected 'BEGIN ollvm:router' -Label 'ollvm managed block'

  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null

  $goPlan = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','control-flow-triage','-Items','function-a,region-b','-ReviewOutputDir',$reviewRoot) | ConvertFrom-Json
  if ([string]$goPlan.command -ne 'plan-subagents' -or [bool]$goPlan.isMutation -or -not [bool]$goPlan.writesReviewArtifacts -or [int]$goPlan.itemCount -ne 2) { throw "unexpected Go ollvm plan: $($goPlan | ConvertTo-Json -Depth 20)" }
  $packet = Get-Content -LiteralPath ([string]$goPlan.packetPath) -Raw | ConvertFrom-Json
  if ([string]$packet.route.id -ne 'ollvm:obfuscation-analysis' -or [string]$packet.observability.routeDebug.selectedBy -ne 'taskType') { throw "unexpected Go ollvm packet: $($packet | ConvertTo-Json -Depth 20)" }
  Assert-ContainsText -Text ([string]$packet.outputContract) -Expected 'function_ref' -Label 'ollvm output contract'

  $facadePlan = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','cfg-review','-Items','finding-a,candidate-b','-ReviewOutputDir',$facadeReviewRoot) -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }
  Assert-ContainsText -Text $facadePlan -Expected 'review packet:' -Label 'ollvm facade plan fallback'
  $facadePacket = Get-Content -LiteralPath (Join-Path $facadeReviewRoot 'packet.json') -Raw | ConvertFrom-Json
  if ([string]$facadePacket.route.id -ne 'ollvm:bounded-review') { throw "unexpected facade ollvm route: $($facadePacket | ConvertTo-Json -Depth 20)" }

  $workflowPath = Join-Path $caseRoot 'references\ollvm\workflow-template.md'
  Add-Content -LiteralPath $workflowPath -Value "`r`nReusable safe ollvm pack note from smoke."
  $promoteReview = Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-ReviewOutputDir',$promoteReviewRoot) | ConvertFrom-Json
  if ([string]$promoteReview.command -ne 'promote' -or [bool]$promoteReview.isMutation) { throw "unexpected ollvm promote review result: $($promoteReview | ConvertTo-Json -Depth 20)" }
  $promotePacket = Get-Content -LiteralPath ([string]$promoteReview.packetPath) -Raw | ConvertFrom-Json
  $workflowItems = @($promotePacket.items | Where-Object { [string]$_.path -eq 'references/ollvm/workflow-template.md' -and [string]$_.kind -eq 'managed-doc' })
  if ($workflowItems.Count -ne 1 -or [string]$workflowItems[0].action -ne 'candidate-after-llm-review') { throw "ollvm workflow promote should not be blocked by deny pattern: $($promotePacket | ConvertTo-Json -Depth 20)" }

  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json')) { throw 'ollvm plan-subagents created board.json' }
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\facts')) { throw 'ollvm plan-subagents created facts' }
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\lanes')) { throw 'ollvm plan-subagents created lanes' }

  'ollvm pack smoke ok'
} finally {
  foreach ($path in @($caseRoot,$reviewRoot,$facadeReviewRoot,$promoteReviewRoot)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
