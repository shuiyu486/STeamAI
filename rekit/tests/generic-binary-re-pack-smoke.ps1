param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
$RepoRoot = Split-Path -Parent $RekitRoot
$Rekit = Join-Path $RekitRoot 'rekit.ps1'
$Pack = 'generic-binary-re'

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
$caseRoot = Join-Path $WorkRoot "gbr-$suffix"
$reviewRoot = Join-Path $WorkRoot "gbr-review-$suffix"
$facadeReviewRoot = Join-Path $WorkRoot "gbr-facade-review-$suffix"
$promoteReviewRoot = Join-Path $WorkRoot "gbr-promote-review-$suffix"
try {
  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Pack',$Pack) | Out-Null

  Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',"generic-binary-re-$suffix",'-Apply') | Out-Null
  foreach ($rel in @('references\generic-binary-re\README.md','references\generic-binary-re\agent-team.md','references\generic-binary-re\workflow-template.md','references\generic-binary-re\toolchain-router.md','references\generic-binary-re\task-handoff.md','CLAUDE.local.md')) {
    if (-not (Test-Path -LiteralPath (Join-Path $caseRoot $rel))) { throw "missing generic-binary-re case file: $rel" }
  }
  $local = [System.IO.File]::ReadAllText((Join-Path $caseRoot 'CLAUDE.local.md'), [System.Text.Encoding]::UTF8)
  Assert-ContainsText -Text $local -Expected 'BEGIN generic-binary-re:router' -Label 'generic-binary-re managed block'

  Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
  Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null

  $goPlan = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','function-analysis','-Items','binary-a,function-b','-ReviewOutputDir',$reviewRoot) | ConvertFrom-Json
  if ([string]$goPlan.command -ne 'plan-subagents' -or [bool]$goPlan.isMutation -or -not [bool]$goPlan.writesReviewArtifacts -or [int]$goPlan.itemCount -ne 2) { throw "unexpected Go generic-binary-re plan: $($goPlan | ConvertTo-Json -Depth 20)" }
  $packet = Get-Content -LiteralPath ([string]$goPlan.packetPath) -Raw | ConvertFrom-Json
  if ([string]$packet.route.id -ne 'generic-binary-re:binary-analysis' -or [string]$packet.observability.routeDebug.selectedBy -ne 'taskType') { throw "unexpected Go generic-binary-re packet: $($packet | ConvertTo-Json -Depth 20)" }
  Assert-ContainsText -Text ([string]$packet.outputContract) -Expected 'function_ref' -Label 'generic-binary-re output contract'

  $facadePlan = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType','function-review','-Items','finding-a,candidate-b','-ReviewOutputDir',$facadeReviewRoot) -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }
  Assert-ContainsText -Text $facadePlan -Expected 'review packet:' -Label 'generic-binary-re facade plan fallback'
  $facadePacket = Get-Content -LiteralPath (Join-Path $facadeReviewRoot 'packet.json') -Raw | ConvertFrom-Json
  if ([string]$facadePacket.route.id -ne 'generic-binary-re:bounded-review') { throw "unexpected facade generic-binary-re route: $($facadePacket | ConvertTo-Json -Depth 20)" }

  $workflowPath = Join-Path $caseRoot 'references\generic-binary-re\workflow-template.md'
  Add-Content -LiteralPath $workflowPath -Value "`r`nReusable safe generic-binary-re pack note from smoke."
  $promoteReview = Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-ReviewOutputDir',$promoteReviewRoot) | ConvertFrom-Json
  if ([string]$promoteReview.command -ne 'promote' -or [bool]$promoteReview.isMutation) { throw "unexpected generic-binary-re promote review result: $($promoteReview | ConvertTo-Json -Depth 20)" }
  $promotePacket = Get-Content -LiteralPath ([string]$promoteReview.packetPath) -Raw | ConvertFrom-Json
  $workflowItems = @($promotePacket.items | Where-Object { [string]$_.path -eq 'references/generic-binary-re/workflow-template.md' -and [string]$_.kind -eq 'managed-doc' })
  if ($workflowItems.Count -ne 1 -or [string]$workflowItems[0].action -ne 'candidate-after-llm-review') { throw "generic-binary-re workflow promote should not be blocked by deny pattern: $($promotePacket | ConvertTo-Json -Depth 20)" }

  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json')) { throw 'generic-binary-re plan-subagents created board.json' }
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\facts')) { throw 'generic-binary-re plan-subagents created facts' }
  if (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\lanes')) { throw 'generic-binary-re plan-subagents created lanes' }

  'generic-binary-re pack smoke ok'
} finally {
  foreach ($path in @($caseRoot,$reviewRoot,$facadeReviewRoot,$promoteReviewRoot)) {
    if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
  }
}
