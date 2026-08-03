[CmdletBinding(PositionalBinding=$false)]
param(
  [Parameter(Position=0)]
  [ValidateSet('status','packs','release-check','release-run','run-current-loop','run-current-step','run-driver-step','run-reviewer-step','run-reviewer-wave','next-batch','attach','repair','init','bootstrap','sync','update','promote','validate','doctor','plan-subagents','overview','complete','continue','reconcile','start','handoff','note','gate')]
  [string]$Command = 'status',
  [string]$Target = '',
  [string]$Pack = 'vmp-re',
  [string]$ProjectName = '',
  [switch]$WhatIf,
  [switch]$Apply,
  [switch]$CreateCandidates,
  [switch]$Force,
  [switch]$Review,
  [string]$ReviewOutputDir = '',
  [string]$PacketPath = '',
  [string]$ReviewerResultPath = '',
  [string]$ReviewerResultInputSourcePath = '',
  [string]$ReviewerHarness = '',
  [string]$ReviewerSession = '',
  [string]$ReviewerOutcome = '',
  [string]$ReviewerExitStatus = '',
  [string]$DiffPath = '',
  [string]$Action = '',
  [string]$Lane = '',
  [string]$Subject = '',
  [string]$Summary = '',
  [string]$Actor = '',
  [string]$Executor = '',
  [string]$Reason = '',
  [string]$Risk = '',
  [string]$TargetRef = '',
  [string]$BatchId = '',
  [string]$Scope = '',
  [string]$Budget = '',
  [int]$RuntimeSeconds = 0,
  [int]$DiskMB = 0,
  [int]$Requests = 0,
  [string]$OutputPaths = '',
  [string]$TriedLightSteps = '',
  [string]$StopConditions = '',
  [string]$GateEventId = '',
  [string]$ExecutionStatus = '',
  [int]$ActualRuntimeSeconds = 0,
  [int]$ActualDiskMB = 0,
  [int]$ActualRequests = 0,
  [string]$OutputRefs = '',
  [string]$EvidenceRefs = '',
  [string]$ExecutionEvidenceRefs = '',
  [string]$BoundaryHits = '',
  [string]$Escalation = '',
  [string]$ExecutionReportPath = '',
  [switch]$ExecutionReportContract,
  [switch]$ValidateExecutionReport,
  [string]$Route = '',
  [string]$TaskType = '',
  [string]$Items = '',
  [string]$ItemsFile = '',
  [int]$ItemsPerAgent = 0,
  [int]$MaxParallel = 0,
  [int]$MaxSteps = 0,
  [string]$Format = '',
  [string]$Domain = '',
  [string]$Closure = '',
  [string]$ExpectedNextBatchPlanSha256 = '',
  [string]$ExpectedCompletePlanSha256 = '',
  [string]$ExpectedHandoffPlanSha256 = '',
  [string]$HandoffPublicationStamp = '',
  [string]$ExpectedCurrentLoopPlanSha256 = '',
  [string]$ExpectedCurrentLoopCheckpointSha256 = '',
  [string]$ExpectedCurrentLoopReviewerAttemptSha256 = '',
  [switch]$ResumeCurrentLoop,
  [string]$ExpectedCurrentStepPlanSha256 = '',
  [string]$ExpectedDriverStepPlanSha256 = '',
  [string]$ExpectedReviewerStepPlanSha256 = '',
  [string]$ReviewerWaveObservationsPath = '',
  [string]$ExpectedReviewerWavePlanSha256 = '',
  [string]$CreatedAt = '',
  [string]$ExpectedNoteEventSha256 = '',
  [Parameter(ValueFromRemainingArguments=$true)]
  [string[]]$RemainingArgs = @()
)

$ErrorActionPreference = 'Stop'
$RuntimeRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = [System.IO.Path]::GetFullPath((Join-Path $RuntimeRoot '..'))
$CallerWorkingDirectory = [System.IO.Path]::GetFullPath((Get-Location).Path)
function Resolve-RekitCallerPath {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) { return '' }
  if ([System.IO.Path]::IsPathRooted($Value)) { return [System.IO.Path]::GetFullPath($Value) }
  return [System.IO.Path]::GetFullPath((Join-Path $CallerWorkingDirectory $Value))
}
function Resolve-RekitTarget {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) { return [System.IO.Path]::GetFullPath((Get-Location).Path) }
  return [System.IO.Path]::GetFullPath($Value)
}

function Test-RekitLooksLikeCase {
  param([string]$Path)
  return (Test-Path -LiteralPath (Join-Path $Path '.rekit\instance.yml')) -or (Test-Path -LiteralPath (Join-Path $Path '.re-template.yml'))
}

function Resolve-RekitCaseRoot {
  param([string]$Path)
  if ([string]::IsNullOrWhiteSpace($Path)) { return '' }
  try {
    $dir = [System.IO.Path]::GetFullPath($Path)
  } catch {
    return ''
  }
  if (Test-Path -LiteralPath $dir -PathType Leaf) { $dir = Split-Path -Parent $dir }
  while (-not [string]::IsNullOrWhiteSpace($dir)) {
    if (Test-RekitLooksLikeCase $dir) { return $dir }
    $parent = Split-Path -Parent $dir
    if ([string]::IsNullOrWhiteSpace($parent) -or [string]::Equals($parent, $dir, [System.StringComparison]::OrdinalIgnoreCase)) { return '' }
    $dir = $parent
  }
  return ''
}

function Resolve-RekitActionTargetAndArgs {
  param(
    [string]$Value,
    [string[]]$Remaining = @()
  )
  $actionTarget = $Value
  $actionArgs = @($Remaining)
  if (-not [string]::IsNullOrWhiteSpace($Value)) {
    $maybeTarget = Resolve-RekitTarget $Value
    if (-not (Test-RekitLooksLikeCase $maybeTarget)) {
      $actionTarget = ''
      $actionArgs = @($Value) + $actionArgs
    }
  }
  return [pscustomobject]@{ Target = (Resolve-RekitTarget $actionTarget); Args = $actionArgs }
}

function Get-RekitRemainingArgMap {
  param([string[]]$Tokens = @())
  $map = @{}
  for ($i = 0; $i -lt $Tokens.Count; $i++) {
    $token = [string]$Tokens[$i]
    if ($token -like '-*=*') {
      $eq = $token.IndexOf('=')
      $name = $token.Substring(1, $eq - 1)
      $map[$name] = $token.Substring($eq + 1)
    } elseif ($token.StartsWith('-')) {
      $name = $token.Substring(1)
      if (($i + 1) -lt $Tokens.Count -and -not ([string]$Tokens[$i+1]).StartsWith('-')) {
        $map[$name] = [string]$Tokens[$i+1]
        $i++
      } else {
        $map[$name] = $true
      }
    }
  }
  return $map
}

function Test-RekitRemainingSwitch {
  param([hashtable]$Map, [string]$Name)
  return ($Map.ContainsKey($Name) -and [bool]$Map[$Name])
}

function Test-RekitEnvTruthy {
  param([string]$Name)
  $value = [Environment]::GetEnvironmentVariable($Name)
  if ([string]::IsNullOrWhiteSpace($value)) { return $false }
  $normalized = $value.Trim().ToLowerInvariant()
  return @('1','true','yes','on') -contains $normalized
}

function Test-RekitGoDefaultDelegationCommand {
  param([string]$Name)
  return (@('status','packs','release-check','release-run','run-current-loop','run-current-step','run-driver-step','run-reviewer-step','run-reviewer-wave','next-batch','doctor','validate','attach','repair','init','bootstrap','sync','update','promote','overview','note','gate','start','handoff','complete','continue','reconcile','plan-subagents') -contains $Name)
}

function Test-RekitNoPowerShellFallbackCommand {
  param([string]$Name)
  return (@('release-check','release-run','run-current-loop','run-current-step','run-driver-step','run-reviewer-step','run-reviewer-wave','next-batch','status','packs','doctor','validate','attach','repair','init','bootstrap','sync','update','promote','overview','note','gate','start','handoff','complete','continue','reconcile','plan-subagents') -contains $Name)
}

function Test-RekitGoDelegationEnabled {
  if (Test-RekitEnvTruthy 'REKIT_GO_DISABLE') { return $false }
  if (Test-RekitEnvTruthy 'REKIT_GO_ENABLE') { return $true }
  return (Test-RekitGoDefaultDelegationCommand -Name $Command)
}

function Test-RekitGoDelegationSafe {
  if ($Command -ne 'plan-subagents' -and -not [string]::IsNullOrWhiteSpace($ReviewerResultPath)) { return $false }
  switch ($Command) {
    { $_ -in @('status','packs','release-check','release-run','doctor','validate') } {
      if ($Apply -or $CreateCandidates -or $Review -or $WhatIf) { return $false }
      if ($Command -in @('release-check','release-run') -and -not [string]::IsNullOrWhiteSpace($Target)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      return $true
    }
    'run-current-loop' {
      foreach ($key in $script:PSBoundParameters.Keys) {
        if (@('Command','Target','Pack','MaxSteps','WhatIf','Apply','Format','ExpectedCurrentLoopPlanSha256','ExpectedCurrentLoopCheckpointSha256','ExpectedCurrentLoopReviewerAttemptSha256','ResumeCurrentLoop','Actor','ReviewerResultInputSourcePath','ReviewerHarness','ReviewerSession','ReviewerOutcome','ReviewerExitStatus') -notcontains [string]$key) { return $false }
      }
      if ([string]::IsNullOrWhiteSpace($Target)) { return $false }
      if ((-not $ResumeCurrentLoop) -and ($MaxSteps -lt 1 -or $MaxSteps -gt 20)) { return $false }
      if ($ResumeCurrentLoop -and $script:PSBoundParameters.ContainsKey('MaxSteps')) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if ($WhatIf -and -not [string]::IsNullOrWhiteSpace($ExpectedCurrentLoopPlanSha256)) { return $false }
      if ($Apply -and [string]::IsNullOrWhiteSpace($ExpectedCurrentLoopPlanSha256)) { return $false }
      if ($ResumeCurrentLoop -and $Apply -and [string]::IsNullOrWhiteSpace($ExpectedCurrentLoopCheckpointSha256)) { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      return (([string]$Format).Trim().ToLowerInvariant() -eq 'json')
    }
    'run-current-step' {
      foreach ($key in $script:PSBoundParameters.Keys) {
        if (@('Command','Target','Pack','WhatIf','Apply','Format','ExpectedCurrentStepPlanSha256','Actor','ReviewerResultInputSourcePath','ReviewerHarness','ReviewerSession','ReviewerOutcome','ReviewerExitStatus') -notcontains [string]$key) { return $false }
      }
      if ([string]::IsNullOrWhiteSpace($Target)) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if ($WhatIf -and -not [string]::IsNullOrWhiteSpace($ExpectedCurrentStepPlanSha256)) { return $false }
      if ($Apply -and [string]::IsNullOrWhiteSpace($ExpectedCurrentStepPlanSha256)) { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      return (([string]$Format).Trim().ToLowerInvariant() -eq 'json')
    }
    'run-driver-step' {
      foreach ($key in $script:PSBoundParameters.Keys) {
        if (@('Command','Target','Pack','WhatIf','Apply','Format','ExpectedDriverStepPlanSha256') -notcontains [string]$key) { return $false }
      }
      if ([string]::IsNullOrWhiteSpace($Target)) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if ($WhatIf -and -not [string]::IsNullOrWhiteSpace($ExpectedDriverStepPlanSha256)) { return $false }
      if ($Apply -and [string]::IsNullOrWhiteSpace($ExpectedDriverStepPlanSha256)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      return (([string]$Format).Trim().ToLowerInvariant() -eq 'json')
    }
    'run-reviewer-step' {
      foreach ($key in $script:PSBoundParameters.Keys) {
        if (@('Command','Target','Pack','WhatIf','Apply','Format','ExpectedReviewerStepPlanSha256','Actor','ReviewerResultInputSourcePath','ReviewerHarness','ReviewerSession','ReviewerOutcome','ReviewerExitStatus') -notcontains [string]$key) { return $false }
      }
      if ([string]::IsNullOrWhiteSpace($Target)) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if ($WhatIf -and -not [string]::IsNullOrWhiteSpace($ExpectedReviewerStepPlanSha256)) { return $false }
      if ($Apply -and [string]::IsNullOrWhiteSpace($ExpectedReviewerStepPlanSha256)) { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      return (([string]$Format).Trim().ToLowerInvariant() -eq 'json')
    }
    'run-reviewer-wave' {
      foreach ($key in $script:PSBoundParameters.Keys) {
        if (@('Command','Target','Pack','PacketPath','Lane','Actor','ReviewerWaveObservationsPath','WhatIf','Apply','Format','ExpectedReviewerWavePlanSha256') -notcontains [string]$key) { return $false }
      }
      if ([string]::IsNullOrWhiteSpace($Target) -or [string]::IsNullOrWhiteSpace($PacketPath) -or [string]::IsNullOrWhiteSpace($Lane) -or [string]::IsNullOrWhiteSpace($Actor) -or [string]::IsNullOrWhiteSpace($ReviewerWaveObservationsPath)) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if ($WhatIf -and -not [string]::IsNullOrWhiteSpace($ExpectedReviewerWavePlanSha256)) { return $false }
      if ($Apply -and [string]::IsNullOrWhiteSpace($ExpectedReviewerWavePlanSha256)) { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      return (([string]$Format).Trim().ToLowerInvariant() -eq 'json')
    }
    'next-batch' {
      if (-not [string]::IsNullOrWhiteSpace($Target)) { return $false }
      if ($CreateCandidates -or $Review -or $Force) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if ([string]::IsNullOrWhiteSpace($Domain) -or [string]::IsNullOrWhiteSpace($Closure)) { return $false }
      if ($WhatIf -and -not [string]::IsNullOrWhiteSpace($ExpectedNextBatchPlanSha256)) { return $false }
      if ($Apply -and [string]::IsNullOrWhiteSpace($ExpectedNextBatchPlanSha256)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      return ([string]::IsNullOrWhiteSpace($formatValue) -or @('json','text','table','tsv') -contains $formatValue)
    }
    'attach' {
      if ([string]::IsNullOrWhiteSpace($Target) -or $CreateCandidates -or $Review) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      return $true
    }
    'repair' {
      if ([string]::IsNullOrWhiteSpace($Target) -or $CreateCandidates -or $Review) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      return (Test-RekitLooksLikeCase $caseRoot)
    }
    { $_ -in @('init','bootstrap') } {
      if ([string]::IsNullOrWhiteSpace($Target) -or $CreateCandidates -or $Review) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      return $true
    }
    'overview' {
      if ($Apply -or $CreateCandidates -or $WhatIf -or $Review) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      if ((-not [string]::IsNullOrWhiteSpace($formatValue)) -and $formatValue -ne 'json') { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      return (Test-RekitLooksLikeCase $caseRoot)
    }
    'note' {
      $noteArgs = Get-RekitRemainingArgMap -Tokens $RemainingArgs
      $listRequested = $List -or (Test-RekitRemainingSwitch -Map $noteArgs -Name 'List')
      if ($Apply -or $CreateCandidates -or $Review -or $Force) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      if ([string]::IsNullOrWhiteSpace($formatValue) -and $noteArgs.ContainsKey('Format')) { $formatValue = ([string]$noteArgs['Format']).Trim().ToLowerInvariant() }
      $caseRoot = Resolve-RekitTarget $Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      if ($listRequested) {
        if ($WhatIf) { return $false }
        return ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -in @('table','text','tsv','json'))
      }
      return ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -eq 'json')
    }
    { $_ -in @('sync','update') } {
      $caseRoot = Resolve-RekitTarget $Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      if ($Apply) {
        if ($CreateCandidates -or $Review) { return $false }
        if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
        $formatValue = ([string]$Format).Trim().ToLowerInvariant()
        if ($WhatIf) { return ($formatValue -eq 'json') }
        return ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -eq 'json')
      }
      if ($WhatIf -or $CreateCandidates -or $Force) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      return ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -eq 'json')
    }
    'promote' {
      $caseRoot = Resolve-RekitTarget $Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      if ($Force) { return $false }
      if ($Apply) {
        if ($CreateCandidates -or $Review) { return $false }
        if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
        $formatValue = ([string]$Format).Trim().ToLowerInvariant()
        if ($WhatIf) { return ($formatValue -eq 'json') }
        return ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -eq 'json')
      }
      if ($CreateCandidates) {
        if ($Review) { return $false }
        if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
        $formatValue = ([string]$Format).Trim().ToLowerInvariant()
        if ($WhatIf) { return ($formatValue -eq 'json') }
        return ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -eq 'json')
      }
      if ($WhatIf) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      return ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -eq 'json')
    }
    'gate' {
      if ($CreateCandidates -or $Review -or $Force) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ($ExecutionReportContract -and $ValidateExecutionReport) { return $false }
      if ($ExecutionReportContract -or $ValidateExecutionReport) {
        if ($WhatIf -or $Apply) { return $false }
      } elseif ((-not $WhatIf) -and (-not $Apply)) {
        return $false
      }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      if ((-not [string]::IsNullOrWhiteSpace($formatValue)) -and $formatValue -ne 'json') { return $false }
      if ([string]::IsNullOrWhiteSpace($Target)) {
        $caseRoot = Resolve-RekitCaseRoot $CallerWorkingDirectory
        return (-not [string]::IsNullOrWhiteSpace($caseRoot))
      }
      $caseRoot = Resolve-RekitTarget $Target
      return (Test-RekitLooksLikeCase $caseRoot)
    }
    'start' {
      if ($CreateCandidates -or $Review) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
      if (-not (Test-RekitLooksLikeCase ([string]$resolved.Target))) { return $false }
      $selector = ((@($resolved.Args) | ForEach-Object { [string]$_ }) -join '-').Trim('-')
      if ([string]::IsNullOrWhiteSpace($selector)) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      return ([string]::IsNullOrWhiteSpace($formatValue) -or @('json','text','table','tsv') -contains $formatValue)
    }
    'handoff' {
      if ($CreateCandidates -or $Review -or $Force) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
      $caseRoot = [string]$resolved.Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      if (-not (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json'))) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      return ([string]::IsNullOrWhiteSpace($formatValue) -or @('json','text','table','tsv') -contains $formatValue)
    }
    'complete' {
      if ($CreateCandidates -or $Review -or $Force) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if ($WhatIf -and -not [string]::IsNullOrWhiteSpace($ExpectedCompletePlanSha256)) { return $false }
      if ($Apply -and [string]::IsNullOrWhiteSpace($ExpectedCompletePlanSha256)) { return $false }
      if ([string]::IsNullOrWhiteSpace($Actor) -or [string]::IsNullOrWhiteSpace($Reason) -or [string]::IsNullOrWhiteSpace($EvidenceRefs)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
      $caseRoot = [string]$resolved.Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      if (-not (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json'))) { return $false }
      $selector = ((@($resolved.Args) | ForEach-Object { [string]$_ }) -join '-').Trim('-')
      if ([string]::IsNullOrWhiteSpace($selector) -and [string]::IsNullOrWhiteSpace($Lane)) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      return ([string]::IsNullOrWhiteSpace($formatValue) -or @('json','text','table','tsv') -contains $formatValue)
    }
    'continue' {
      if ($CreateCandidates -or $Review -or $Force) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
      $caseRoot = [string]$resolved.Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      if (-not (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json'))) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      return ([string]::IsNullOrWhiteSpace($formatValue) -or @('json','text','table','tsv') -contains $formatValue)
    }
    'reconcile' {
      if ($CreateCandidates -or $Review -or $Force) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
      $caseRoot = [string]$resolved.Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      if (-not (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json'))) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      return ([string]::IsNullOrWhiteSpace($formatValue) -or @('json','text','table','tsv') -contains $formatValue)
    }
    'plan-subagents' {
      if ($CreateCandidates -or $Review -or $Force) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      if ([string]::IsNullOrWhiteSpace($Target)) { return $false }
      $planRoot = Resolve-RekitTarget $Target
      if (-not [string]::IsNullOrWhiteSpace($ReviewerResultPath)) {
        if ($WhatIf -and $Apply) { return $false }
        if ((-not $WhatIf) -and (-not $Apply)) { return $false }
        if ([string]::IsNullOrWhiteSpace($PacketPath) -or [string]::IsNullOrWhiteSpace($Lane) -or [string]::IsNullOrWhiteSpace($Actor)) { return $false }
        if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
        if ((-not [string]::IsNullOrWhiteSpace($formatValue)) -and $formatValue -ne 'json') { return $false }
        return (Test-RekitLooksLikeCase $planRoot)
      }
      if ($Apply -or $WhatIf) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($formatValue)) { return $false }
      if (Test-RekitLooksLikeCase $planRoot) { return $true }
      if ([string]::IsNullOrWhiteSpace($ReviewOutputDir)) { return $false }
      return (Test-Path -LiteralPath $planRoot)
    }
    default { return $false }
  }
}

function Get-RekitGoInvocation {
  $envExe = [Environment]::GetEnvironmentVariable('REKIT_GO_EXE')
  if (-not [string]::IsNullOrWhiteSpace($envExe)) {
    return [pscustomobject]@{ Command = $envExe; Prefix = @(); WorkingDirectory = $RepoRoot }
  }
  $bin = Join-Path $RepoRoot 'rekit\bin\rekit-go.exe'
  if (Test-Path -LiteralPath $bin) {
    return [pscustomobject]@{ Command = $bin; Prefix = @(); WorkingDirectory = $RepoRoot }
  }
  $go = Get-Command go -ErrorAction SilentlyContinue
  if ($null -ne $go) {
    return [pscustomobject]@{ Command = $go.Source; Prefix = @('run','./cmd/rekit','--'); WorkingDirectory = $RepoRoot }
  }
  return $null
}

function Add-RekitGoArg {
  param([ref]$List, [string]$Name, [string]$Value)
  if (-not [string]::IsNullOrWhiteSpace($Value)) {
    $List.Value = @($List.Value) + @($Name, $Value)
  }
}

function Add-RekitGoSwitch {
  param([ref]$List, [string]$Name, [bool]$Enabled)
  if ($Enabled) {
    $List.Value = @($List.Value) + @($Name)
  }
}

function Get-RekitGoTarget {
  switch ($Command) {
    { $_ -in @('status','packs','release-check','release-run','run-current-loop','run-current-step','run-driver-step','run-reviewer-step','run-reviewer-wave','next-batch') } { return (Resolve-RekitTarget $Target) }
    'gate' {
      if ([string]::IsNullOrWhiteSpace($Target)) { return '' }
      return (Resolve-RekitTarget $Target)
    }
    { $_ -in @('attach','repair','init','bootstrap','overview','note','sync','update','promote','plan-subagents') } { return (Resolve-RekitTarget $Target) }
    { $_ -in @('doctor','validate') } {
      if (-not [string]::IsNullOrWhiteSpace($Target)) { return (Resolve-RekitTarget $Target) }
      $cwd = Resolve-RekitTarget ''
      if ((Test-RekitLooksLikeCase $cwd) -and (-not [string]::Equals($cwd, $RepoRoot, [System.StringComparison]::OrdinalIgnoreCase))) { return $cwd }
      return ''
    }
    default { return '' }
  }
}

function Get-RekitGoArgs {
  $goArgs = @('-Command', $Command, '-Pack', $Pack)
  $goTarget = Get-RekitGoTarget
  if ($Command -notin @('start','handoff','complete','continue','reconcile','release-check','release-run','next-batch')) { Add-RekitGoArg ([ref]$goArgs) '-Target' $goTarget }
  $goReview = $Review.IsPresent
  if ($Command -in @('sync','update') -and (-not $Apply) -and (-not $WhatIf)) { $goReview = $true }
  if ($Command -eq 'promote' -and (-not $Apply) -and (-not $CreateCandidates) -and (-not $WhatIf)) { $goReview = $true }
  Add-RekitGoSwitch ([ref]$goArgs) '-Review' $goReview
  Add-RekitGoSwitch ([ref]$goArgs) '-Apply' $Apply.IsPresent
  Add-RekitGoSwitch ([ref]$goArgs) '-CreateCandidates' $CreateCandidates.IsPresent
  Add-RekitGoSwitch ([ref]$goArgs) '-WhatIf' $WhatIf.IsPresent
  Add-RekitGoArg ([ref]$goArgs) '-ReviewOutputDir' (Resolve-RekitCallerPath $ReviewOutputDir)
  Add-RekitGoArg ([ref]$goArgs) '-PacketPath' (Resolve-RekitCallerPath $PacketPath)
  Add-RekitGoArg ([ref]$goArgs) '-ReviewerResultPath' (Resolve-RekitCallerPath $ReviewerResultPath)
  Add-RekitGoArg ([ref]$goArgs) '-DiffPath' (Resolve-RekitCallerPath $DiffPath)
  $goFormat = $Format
  if ($Command -in @('start','handoff','complete','continue','reconcile') -and (-not $Apply.IsPresent) -and [string]::IsNullOrWhiteSpace([string]$goFormat)) { $goFormat = 'text' }
  if ($Command -in @('status','packs','release-check','release-run','run-current-loop','run-current-step','run-driver-step','run-reviewer-step','run-reviewer-wave','next-batch','doctor','validate','attach','repair','init','bootstrap','sync','update','promote','overview','note','gate','start','handoff','complete','continue','reconcile')) { Add-RekitGoArg ([ref]$goArgs) '-Format' $goFormat }
  if ($Command -eq 'run-current-loop') {
    if (-not $ResumeCurrentLoop) { Add-RekitGoArg ([ref]$goArgs) '-MaxSteps' ([string]$MaxSteps) }
    Add-RekitGoArg ([ref]$goArgs) '-ExpectedCurrentLoopPlanSha256' $ExpectedCurrentLoopPlanSha256
    Add-RekitGoArg ([ref]$goArgs) '-ExpectedCurrentLoopCheckpointSha256' $ExpectedCurrentLoopCheckpointSha256
    Add-RekitGoArg ([ref]$goArgs) '-ExpectedCurrentLoopReviewerAttemptSha256' $ExpectedCurrentLoopReviewerAttemptSha256
    Add-RekitGoSwitch ([ref]$goArgs) '-ResumeCurrentLoop' $ResumeCurrentLoop.IsPresent
    Add-RekitGoArg ([ref]$goArgs) '-Actor' $Actor
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerResultInputSourcePath' (Resolve-RekitCallerPath $ReviewerResultInputSourcePath)
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerHarness' $ReviewerHarness
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerSession' $ReviewerSession
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerOutcome' $ReviewerOutcome
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerExitStatus' $ReviewerExitStatus
  }
  if ($Command -eq 'run-current-step') {
    Add-RekitGoArg ([ref]$goArgs) '-ExpectedCurrentStepPlanSha256' $ExpectedCurrentStepPlanSha256
    Add-RekitGoArg ([ref]$goArgs) '-Actor' $Actor
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerResultInputSourcePath' (Resolve-RekitCallerPath $ReviewerResultInputSourcePath)
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerHarness' $ReviewerHarness
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerSession' $ReviewerSession
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerOutcome' $ReviewerOutcome
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerExitStatus' $ReviewerExitStatus
  }
  if ($Command -eq 'run-driver-step') {
    Add-RekitGoArg ([ref]$goArgs) '-ExpectedDriverStepPlanSha256' $ExpectedDriverStepPlanSha256
  }
  if ($Command -eq 'run-reviewer-step') {
    Add-RekitGoArg ([ref]$goArgs) '-ExpectedReviewerStepPlanSha256' $ExpectedReviewerStepPlanSha256
    Add-RekitGoArg ([ref]$goArgs) '-Actor' $Actor
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerResultInputSourcePath' (Resolve-RekitCallerPath $ReviewerResultInputSourcePath)
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerHarness' $ReviewerHarness
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerSession' $ReviewerSession
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerOutcome' $ReviewerOutcome
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerExitStatus' $ReviewerExitStatus
  }
  if ($Command -eq 'run-reviewer-wave') {
    Add-RekitGoArg ([ref]$goArgs) '-Lane' $Lane
    Add-RekitGoArg ([ref]$goArgs) '-Actor' $Actor
    Add-RekitGoArg ([ref]$goArgs) '-ReviewerWaveObservationsPath' (Resolve-RekitCallerPath $ReviewerWaveObservationsPath)
    Add-RekitGoArg ([ref]$goArgs) '-ExpectedReviewerWavePlanSha256' $ExpectedReviewerWavePlanSha256
  }
  if ($Command -eq 'next-batch') {
    Add-RekitGoArg ([ref]$goArgs) '-Domain' $Domain
    Add-RekitGoArg ([ref]$goArgs) '-Closure' $Closure
    Add-RekitGoArg ([ref]$goArgs) '-ExpectedNextBatchPlanSha256' $ExpectedNextBatchPlanSha256
  }
  if ($Command -in @('attach','repair','init','bootstrap','sync','update')) { Add-RekitGoArg ([ref]$goArgs) '-ProjectName' $ProjectName }
  if ($Command -in @('init','bootstrap','sync','update')) { Add-RekitGoSwitch ([ref]$goArgs) '-Force' $Force.IsPresent }
  if ($Command -eq 'note') {
    $noteArgs = Get-RekitRemainingArgMap -Tokens $RemainingArgs
    $noteList = $List.IsPresent -or (Test-RekitRemainingSwitch -Map $noteArgs -Name 'List')
    Add-RekitGoSwitch ([ref]$goArgs) '-List' $noteList
    if ([string]::IsNullOrWhiteSpace([string]$Format) -and $noteArgs.ContainsKey('Format')) { Add-RekitGoArg ([ref]$goArgs) '-Format' ([string]$noteArgs['Format']) }
    $noteValues = [ordered]@{
      Kind = ''
      Lane = $Lane
      Subject = $Subject
      Summary = $Summary
      Actor = $Actor
      Risk = $Risk
      Related = ''
      Confidence = ''
      Decision = ''
      Reason = $Reason
      Status = ''
      BatchId = $BatchId
      TargetRef = $TargetRef
      Verifier = ''
      Verdict = ''
      Action = $Action
      ApprovedBy = ''
      Scope = $Scope
      Expires = ''
      EvidenceRefs = ''
      EventId = ''
      CreatedAt = $CreatedAt
      ExpectedNoteEventSha256 = $ExpectedNoteEventSha256
    }
    foreach ($name in @($noteValues.Keys)) {
      if ([string]::IsNullOrWhiteSpace([string]$noteValues[$name]) -and $noteArgs.ContainsKey($name)) { $noteValues[$name] = [string]$noteArgs[$name] }
    }
    foreach ($name in @($noteValues.Keys)) {
      Add-RekitGoArg ([ref]$goArgs) ('-' + $name) ([string]$noteValues[$name])
    }
  }
  if ($Command -in @('start','handoff','complete','continue','reconcile')) {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Add-RekitGoArg ([ref]$goArgs) '-Target' ([string]$resolved.Target)
    foreach ($arg in @($resolved.Args)) {
      if (-not [string]::IsNullOrWhiteSpace([string]$arg)) {
        $goArgs = @($goArgs) + @([string]$arg)
      }
    }
    if ($Command -eq 'start') {
      Add-RekitGoSwitch ([ref]$goArgs) '-Force' $Force.IsPresent
      Add-RekitGoArg ([ref]$goArgs) '-Actor' $Actor
      Add-RekitGoArg ([ref]$goArgs) '-Executor' $Executor
      Add-RekitGoArg ([ref]$goArgs) '-Reason' $Reason
    }
    if ($Command -eq 'handoff') {
      Add-RekitGoArg ([ref]$goArgs) '-ExpectedHandoffPlanSha256' $ExpectedHandoffPlanSha256
      Add-RekitGoArg ([ref]$goArgs) '-HandoffPublicationStamp' $HandoffPublicationStamp
    }
    if ($Command -eq 'complete') {
      Add-RekitGoArg ([ref]$goArgs) '-Lane' $Lane
      Add-RekitGoArg ([ref]$goArgs) '-Actor' $Actor
      Add-RekitGoArg ([ref]$goArgs) '-Reason' $Reason
      Add-RekitGoArg ([ref]$goArgs) '-EvidenceRefs' $EvidenceRefs
      Add-RekitGoArg ([ref]$goArgs) '-ExpectedCompletePlanSha256' $ExpectedCompletePlanSha256
    }
  }
  if ($Command -eq 'reconcile') {
    $reconcileArgs = Get-RekitRemainingArgMap -Tokens $RemainingArgs
    $reconcileValues = [ordered]@{
      Lane = $Lane
      InterventionId = ''
      EventId = ''
      Executor = $Executor
      Actor = $Actor
      Summary = $Summary
      Reason = $Reason
    }
    foreach ($name in @($reconcileValues.Keys)) {
      if ([string]::IsNullOrWhiteSpace([string]$reconcileValues[$name]) -and $reconcileArgs.ContainsKey($name)) { $reconcileValues[$name] = [string]$reconcileArgs[$name] }
    }
    foreach ($name in @($reconcileValues.Keys)) {
      Add-RekitGoArg ([ref]$goArgs) ('-' + $name) ([string]$reconcileValues[$name])
    }
  }
  if ($Command -eq 'gate') {
    Add-RekitGoArg ([ref]$goArgs) '-Action' $Action
    Add-RekitGoArg ([ref]$goArgs) '-Lane' $Lane
    Add-RekitGoArg ([ref]$goArgs) '-Subject' $Subject
    Add-RekitGoArg ([ref]$goArgs) '-Summary' $Summary
    Add-RekitGoArg ([ref]$goArgs) '-Actor' $Actor
    Add-RekitGoArg ([ref]$goArgs) '-Risk' $Risk
    Add-RekitGoArg ([ref]$goArgs) '-TargetRef' $TargetRef
    Add-RekitGoArg ([ref]$goArgs) '-BatchId' $BatchId
    Add-RekitGoArg ([ref]$goArgs) '-Scope' $Scope
    Add-RekitGoArg ([ref]$goArgs) '-Budget' $Budget
    if ($RuntimeSeconds -gt 0) { Add-RekitGoArg ([ref]$goArgs) '-RuntimeSeconds' ([string]$RuntimeSeconds) }
    if ($DiskMB -gt 0) { Add-RekitGoArg ([ref]$goArgs) '-DiskMB' ([string]$DiskMB) }
    if ($Requests -gt 0) { Add-RekitGoArg ([ref]$goArgs) '-Requests' ([string]$Requests) }
    Add-RekitGoArg ([ref]$goArgs) '-OutputPaths' $OutputPaths
    Add-RekitGoArg ([ref]$goArgs) '-TriedLightSteps' $TriedLightSteps
    Add-RekitGoArg ([ref]$goArgs) '-StopConditions' $StopConditions
    Add-RekitGoArg ([ref]$goArgs) '-GateEventId' $GateEventId
    Add-RekitGoArg ([ref]$goArgs) '-ExecutionStatus' $ExecutionStatus
    if ($ActualRuntimeSeconds -ne 0) { Add-RekitGoArg ([ref]$goArgs) '-ActualRuntimeSeconds' ([string]$ActualRuntimeSeconds) }
    if ($ActualDiskMB -ne 0) { Add-RekitGoArg ([ref]$goArgs) '-ActualDiskMB' ([string]$ActualDiskMB) }
    if ($ActualRequests -ne 0) { Add-RekitGoArg ([ref]$goArgs) '-ActualRequests' ([string]$ActualRequests) }
    Add-RekitGoArg ([ref]$goArgs) '-OutputRefs' $OutputRefs
    Add-RekitGoArg ([ref]$goArgs) '-ExecutionEvidenceRefs' $ExecutionEvidenceRefs
    Add-RekitGoArg ([ref]$goArgs) '-BoundaryHits' $BoundaryHits
    Add-RekitGoArg ([ref]$goArgs) '-Escalation' $Escalation
    Add-RekitGoArg ([ref]$goArgs) '-ExecutionReportPath' $ExecutionReportPath
    Add-RekitGoSwitch ([ref]$goArgs) '-ExecutionReportContract' $ExecutionReportContract.IsPresent
    Add-RekitGoSwitch ([ref]$goArgs) '-ValidateExecutionReport' $ValidateExecutionReport.IsPresent
  }
  if ($Command -eq 'plan-subagents') {
    Add-RekitGoArg ([ref]$goArgs) '-Route' $Route
    Add-RekitGoArg ([ref]$goArgs) '-TaskType' $TaskType
    Add-RekitGoArg ([ref]$goArgs) '-Items' $Items
    Add-RekitGoArg ([ref]$goArgs) '-ItemsFile' (Resolve-RekitCallerPath $ItemsFile)
    if ($ItemsPerAgent -gt 0) { Add-RekitGoArg ([ref]$goArgs) '-ItemsPerAgent' ([string]$ItemsPerAgent) }
    if ($MaxParallel -gt 0) { Add-RekitGoArg ([ref]$goArgs) '-MaxParallel' ([string]$MaxParallel) }
    Add-RekitGoArg ([ref]$goArgs) '-Lane' $Lane
    Add-RekitGoArg ([ref]$goArgs) '-Actor' $Actor
    Add-RekitGoArg ([ref]$goArgs) '-Format' $Format
  }
  return $goArgs
}

function Invoke-RekitGoBackend {
  param([Parameter(Mandatory=$true)]$Invocation)
  $goArgs = Get-RekitGoArgs
  $oldCallerCwd = [Environment]::GetEnvironmentVariable('REKIT_CALLER_CWD', 'Process')
  $callerCwd = ''
  if ($Command -eq 'gate' -and [string]::IsNullOrWhiteSpace($Target)) { $callerCwd = $CallerWorkingDirectory }
  if (-not [string]::IsNullOrWhiteSpace($callerCwd)) { [Environment]::SetEnvironmentVariable('REKIT_CALLER_CWD', $callerCwd, 'Process') }
  Push-Location $Invocation.WorkingDirectory
  try {
    $argv = @($Invocation.Prefix) + @($goArgs)
    & $Invocation.Command @argv
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  } finally {
    Pop-Location
    [Environment]::SetEnvironmentVariable('REKIT_CALLER_CWD', $oldCallerCwd, 'Process')
  }
}

if (Test-RekitGoDelegationEnabled) {
  if (Test-RekitGoDelegationSafe) {
    $goInvocation = Get-RekitGoInvocation
    if ($null -ne $goInvocation) {
      Invoke-RekitGoBackend -Invocation $goInvocation
      return
    }
  }
}

if (Test-RekitNoPowerShellFallbackCommand -Name $Command) {
  if ($Command -ne 'plan-subagents' -and -not [string]::IsNullOrWhiteSpace($ReviewerResultPath)) {
    throw "-ReviewerResultPath is supported only by plan-subagents reviewer intake."
  }
  throw "$Command is implemented by the Go backend only; PowerShell fallback has been retired for this command. Remove REKIT_GO_DISABLE or use go run ./cmd/rekit -- -Command $Command."
}

throw "$Command is not available through the retired PowerShell fallback dispatcher. Use go run ./cmd/rekit -- -Command $Command."
