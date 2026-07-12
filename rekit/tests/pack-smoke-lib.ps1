$RekitPackSmokeScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitPackSmokeRoot = Split-Path -Parent $RekitPackSmokeScriptDir
$RekitPackSmokeRepoRoot = Split-Path -Parent $RekitPackSmokeRoot
$RekitPackSmokeRekit = Join-Path $RekitPackSmokeRoot 'rekit.ps1'

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
    $output = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $RekitPackSmokeRekit @Arguments 2>&1 | Out-String
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
  Push-Location $RekitPackSmokeRepoRoot
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

function Assert-PackSmokeContainsText {
  param(
    [Parameter(Mandatory=$true)][AllowEmptyString()][string]$Text,
    [Parameter(Mandatory=$true)][string]$Expected,
    [Parameter(Mandatory=$true)][string]$Label
  )
  if ($Text -notlike "*$Expected*") { throw "$Label missing expected text '$Expected'. Output:`n$Text" }
}

function Invoke-RekitPackSmoke {
  param(
    [Parameter(Mandatory=$true)][string]$WorkRoot,
    [Parameter(Mandatory=$true)][string]$Pack,
    [Parameter(Mandatory=$true)][string]$CasePrefix,
    [string]$ProjectNamePrefix,
    [Parameter(Mandatory=$true)][string]$PlanTaskType,
    [Parameter(Mandatory=$true)][string]$PlanItems,
    [Parameter(Mandatory=$true)][string]$ExpectedPlanRoute,
    [Parameter(Mandatory=$true)][string]$ExpectedOutputContractText,
    [Parameter(Mandatory=$true)][string]$FacadeTaskType,
    [Parameter(Mandatory=$true)][string]$FacadeItems,
    [string]$ExpectedFacadeRoute,
    [string[]]$ExpectedCaseFiles = @(),
    [string]$ManagedBlockText,
    [string]$PromoteManagedDocPath,
    [string]$PromoteNote,
    [string]$SuccessMessage,
    [int]$ExpectedItemCount = 0
  )

  if ([string]::IsNullOrWhiteSpace($ProjectNamePrefix)) { $ProjectNamePrefix = $Pack }
  if ([string]::IsNullOrWhiteSpace($ExpectedFacadeRoute)) { $ExpectedFacadeRoute = ('{0}:bounded-review' -f $Pack) }
  if ([string]::IsNullOrWhiteSpace($ManagedBlockText)) { $ManagedBlockText = ('BEGIN {0}:router' -f $Pack) }
  if ([string]::IsNullOrWhiteSpace($PromoteManagedDocPath)) { $PromoteManagedDocPath = ('references/{0}/workflow-template.md' -f $Pack) }
  if ([string]::IsNullOrWhiteSpace($PromoteNote)) { $PromoteNote = ('Reusable safe {0} pack note from smoke.' -f $Pack) }
  if ([string]::IsNullOrWhiteSpace($SuccessMessage)) { $SuccessMessage = ('{0} pack smoke ok' -f $Pack) }
  if ($ExpectedItemCount -le 0) {
    $ExpectedItemCount = @($PlanItems -split ',' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }).Count
  }
  if ($ExpectedCaseFiles.Count -eq 0) {
    $ExpectedCaseFiles = @(
      ('references\{0}\README.md' -f $Pack),
      ('references\{0}\agent-team.md' -f $Pack),
      ('references\{0}\workflow-template.md' -f $Pack),
      ('references\{0}\toolchain-router.md' -f $Pack),
      ('references\{0}\task-handoff.md' -f $Pack),
      'CLAUDE.local.md'
    )
  }

  Get-ChildItem -LiteralPath $WorkRoot | Select-Object -First 1 | Out-Null
  $suffix = [Guid]::NewGuid().ToString('N').Substring(0,8)
  $caseRoot = Join-Path $WorkRoot ("$CasePrefix-$suffix")
  $reviewRoot = Join-Path $WorkRoot ("$CasePrefix-review-$suffix")
  $facadeReviewRoot = Join-Path $WorkRoot ("$CasePrefix-facade-review-$suffix")
  $promoteReviewRoot = Join-Path $WorkRoot ("$CasePrefix-promote-review-$suffix")
  try {
    Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Pack',$Pack) | Out-Null
    Invoke-RekitSmoke -Arguments @('-Command','doctor','-Pack',$Pack) | Out-Null

    Invoke-GoRekitSmoke -Arguments @('-Command','init','-Target',$caseRoot,'-Pack',$Pack,'-ProjectName',("$ProjectNamePrefix-$suffix"),'-Apply') | Out-Null
    foreach ($rel in $ExpectedCaseFiles) {
      if (-not (Test-Path -LiteralPath (Join-Path $caseRoot $rel))) { throw "missing $Pack case file: $rel" }
    }
    $local = [System.IO.File]::ReadAllText((Join-Path $caseRoot 'CLAUDE.local.md'), [System.Text.Encoding]::UTF8)
    Assert-PackSmokeContainsText -Text $local -Expected $ManagedBlockText -Label "$Pack managed block"

    Invoke-GoRekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null
    Invoke-RekitSmoke -Arguments @('-Command','doctor','-Target',$caseRoot,'-Pack',$Pack) | Out-Null

    $goPlan = Invoke-GoRekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType',$PlanTaskType,'-Items',$PlanItems,'-ReviewOutputDir',$reviewRoot) | ConvertFrom-Json
    if ([string]$goPlan.command -ne 'plan-subagents' -or [bool]$goPlan.isMutation -or -not [bool]$goPlan.writesReviewArtifacts -or [int]$goPlan.itemCount -ne $ExpectedItemCount) { throw "unexpected Go $Pack plan: $($goPlan | ConvertTo-Json -Depth 20)" }
    $packet = Get-Content -LiteralPath ([string]$goPlan.packetPath) -Raw | ConvertFrom-Json
    if ([string]$packet.route.id -ne $ExpectedPlanRoute -or [string]$packet.observability.routeDebug.selectedBy -ne 'taskType') { throw "unexpected Go $Pack packet: $($packet | ConvertTo-Json -Depth 20)" }
    Assert-PackSmokeContainsText -Text ([string]$packet.outputContract) -Expected $ExpectedOutputContractText -Label "$Pack output contract"

    $facadePlan = Invoke-RekitSmoke -Arguments @('-Command','plan-subagents','-Target',$caseRoot,'-Pack',$Pack,'-TaskType',$FacadeTaskType,'-Items',$FacadeItems,'-ReviewOutputDir',$facadeReviewRoot) -Env @{ REKIT_GO_ENABLE = '1'; REKIT_GO_DISABLE = '' }
    Assert-PackSmokeContainsText -Text $facadePlan -Expected 'review packet:' -Label "$Pack facade plan fallback"
    $facadePacket = Get-Content -LiteralPath (Join-Path $facadeReviewRoot 'packet.json') -Raw | ConvertFrom-Json
    if ([string]$facadePacket.route.id -ne $ExpectedFacadeRoute) { throw "unexpected facade $Pack route: $($facadePacket | ConvertTo-Json -Depth 20)" }

    $workflowRel = ($PromoteManagedDocPath -split '/') -join [System.IO.Path]::DirectorySeparatorChar
    $workflowPath = Join-Path $caseRoot $workflowRel
    Add-Content -LiteralPath $workflowPath -Value ("`r`n{0}" -f $PromoteNote)
    $promoteReview = Invoke-GoRekitSmoke -Arguments @('-Command','promote','-Target',$caseRoot,'-Pack',$Pack,'-ReviewOutputDir',$promoteReviewRoot) | ConvertFrom-Json
    if ([string]$promoteReview.command -ne 'promote' -or [bool]$promoteReview.isMutation) { throw "unexpected $Pack promote review result: $($promoteReview | ConvertTo-Json -Depth 20)" }
    $promotePacket = Get-Content -LiteralPath ([string]$promoteReview.packetPath) -Raw | ConvertFrom-Json
    $workflowItems = @($promotePacket.items | Where-Object { [string]$_.path -eq $PromoteManagedDocPath -and [string]$_.kind -eq 'managed-doc' })
    if ($workflowItems.Count -ne 1 -or [string]$workflowItems[0].action -ne 'candidate-after-llm-review') { throw "$Pack workflow promote should not be blocked by deny pattern: $($promotePacket | ConvertTo-Json -Depth 20)" }

    foreach ($unexpected in @('.rekit\board.json','.rekit\facts','.rekit\lanes')) {
      if (Test-Path -LiteralPath (Join-Path $caseRoot $unexpected)) { throw "$Pack plan-subagents created $unexpected" }
    }

    $SuccessMessage
  } finally {
    foreach ($path in @($caseRoot,$reviewRoot,$facadeReviewRoot,$promoteReviewRoot)) {
      if (-not [string]::IsNullOrWhiteSpace($path) -and (Test-Path -LiteralPath $path)) { Remove-Item -LiteralPath $path -Recurse -Force -Confirm:$false }
    }
  }
}
