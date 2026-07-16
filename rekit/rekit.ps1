[CmdletBinding(PositionalBinding=$false)]
param(
  [Parameter(Position=0)]
  [ValidateSet('status','packs','release-check','attach','repair','init','bootstrap','sync','update','promote','validate','doctor','plan-subagents','overview','continue','start','handoff','note','gate')]
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
  [string]$DiffPath = '',
  [string]$Action = '',
  [string]$Lane = '',
  [string]$Subject = '',
  [string]$Summary = '',
  [string]$Actor = '',
  [string]$Risk = '',
  [string]$TargetRef = '',
  [string]$BatchId = '',
  [string]$Scope = '',
  [string]$Budget = '',
  [string]$TriedLightSteps = '',
  [string]$StopConditions = '',
  [string]$Route = '',
  [string]$TaskType = '',
  [string]$Items = '',
  [string]$ItemsFile = '',
  [int]$ItemsPerAgent = 0,
  [int]$MaxParallel = 0,
  [string]$Format = '',
  [Parameter(ValueFromRemainingArguments=$true)]
  [string[]]$RemainingArgs = @()
)

$ErrorActionPreference = 'Stop'
$RuntimeRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = [System.IO.Path]::GetFullPath((Join-Path $RuntimeRoot '..'))

. (Join-Path $RuntimeRoot 'lib\Manifest.ps1')
. (Join-Path $RuntimeRoot 'lib\Instance.ps1')
. (Join-Path $RuntimeRoot 'lib\Validate.ps1')
. (Join-Path $RuntimeRoot 'lib\Promote.ps1')
. (Join-Path $RuntimeRoot 'lib\Review.ps1')
. (Join-Path $RuntimeRoot 'lib\Sync.ps1')
. (Join-Path $RuntimeRoot 'lib\B3.Core.ps1')
. (Join-Path $RuntimeRoot 'lib\B3.State.ps1')
. (Join-Path $RuntimeRoot 'lib\B3.Policy.ps1')
. (Join-Path $RuntimeRoot 'lib\B3.Lane.ps1')
. (Join-Path $RuntimeRoot 'lib\B3.Auto.ps1')
. (Join-Path $RuntimeRoot 'lib\B3.Handoff.ps1')
. (Join-Path $RuntimeRoot 'lib\B3.Commands.ps1')

function Resolve-RekitTarget {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) { return [System.IO.Path]::GetFullPath((Get-Location).Path) }
  return [System.IO.Path]::GetFullPath($Value)
}

function Test-RekitLooksLikeCase {
  param([string]$Path)
  return (Test-Path -LiteralPath (Join-Path $Path '.rekit\instance.yml')) -or (Test-Path -LiteralPath (Join-Path $Path '.re-template.yml'))
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
  return (@('status','packs','release-check','doctor','validate','attach','repair','init','bootstrap','sync','update','promote','overview','note','gate','start','handoff','continue','plan-subagents') -contains $Name)
}

function Test-RekitNoPowerShellFallbackCommand {
  param([string]$Name)
  return (@('release-check','status','packs','doctor','validate','attach','repair','init','bootstrap','sync','update','promote','overview','note','gate','plan-subagents') -contains $Name)
}

function Test-RekitGoDelegationEnabled {
  if (Test-RekitEnvTruthy 'REKIT_GO_DISABLE') { return $false }
  if (Test-RekitEnvTruthy 'REKIT_GO_ENABLE') { return $true }
  return (Test-RekitGoDefaultDelegationCommand -Name $Command)
}

function Test-RekitGoDelegationSafe {
  switch ($Command) {
    { $_ -in @('status','packs','release-check','doctor','validate') } {
      if ($Apply -or $CreateCandidates -or $Review -or $WhatIf) { return $false }
      if ($Command -eq 'release-check' -and -not [string]::IsNullOrWhiteSpace($Target)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      return $true
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
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      if ((-not [string]::IsNullOrWhiteSpace($formatValue)) -and $formatValue -ne 'json') { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      return (Test-RekitLooksLikeCase $caseRoot)
    }
    'start' {
      if ($CreateCandidates -or $Review) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
      if (-not (Test-RekitLooksLikeCase ([string]$resolved.Target))) { return $false }
      $selector = ((@($resolved.Args) | ForEach-Object { [string]$_ }) -join '-').Trim('-')
      if ([string]::IsNullOrWhiteSpace($selector)) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      if ($WhatIf) { return ($formatValue -eq 'json') }
      return ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -eq 'json')
    }
    'handoff' {
      if ($CreateCandidates -or $Review -or $Force) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
      $caseRoot = [string]$resolved.Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      if (-not (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json'))) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      if ($WhatIf) { return ($formatValue -eq 'json') }
      return ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -eq 'json')
    }
    'continue' {
      if ($CreateCandidates -or $Review -or $Force) { return $false }
      if ($WhatIf -and $Apply) { return $false }
      if ((-not $WhatIf) -and (-not $Apply)) { return $false }
      if (-not [string]::IsNullOrWhiteSpace($ReviewOutputDir) -or -not [string]::IsNullOrWhiteSpace($PacketPath) -or -not [string]::IsNullOrWhiteSpace($DiffPath)) { return $false }
      $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
      $caseRoot = [string]$resolved.Target
      if (-not (Test-RekitLooksLikeCase $caseRoot)) { return $false }
      if (-not (Test-Path -LiteralPath (Join-Path $caseRoot '.rekit\board.json'))) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      if ($WhatIf) { return ($formatValue -eq 'json') }
      return ($Apply -and ([string]::IsNullOrWhiteSpace($formatValue) -or $formatValue -eq 'json'))
    }
    'plan-subagents' {
      if ($Apply -or $WhatIf -or $CreateCandidates -or $Review -or $Force) { return $false }
      $formatValue = ([string]$Format).Trim().ToLowerInvariant()
      if (-not [string]::IsNullOrWhiteSpace($formatValue)) { return $false }
      if ([string]::IsNullOrWhiteSpace($Target)) { return $false }
      $planRoot = Resolve-RekitTarget $Target
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
    { $_ -in @('status','packs','release-check') } { return (Resolve-RekitTarget $Target) }
    { $_ -in @('attach','repair','init','bootstrap','overview','note','sync','update','promote','gate','plan-subagents') } { return (Resolve-RekitTarget $Target) }
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
  if ($Command -notin @('start','handoff','continue','release-check')) { Add-RekitGoArg ([ref]$goArgs) '-Target' $goTarget }
  $goReview = $Review.IsPresent
  if ($Command -in @('sync','update') -and (-not $Apply) -and (-not $WhatIf)) { $goReview = $true }
  if ($Command -eq 'promote' -and (-not $Apply) -and (-not $CreateCandidates) -and (-not $WhatIf)) { $goReview = $true }
  Add-RekitGoSwitch ([ref]$goArgs) '-Review' $goReview
  Add-RekitGoSwitch ([ref]$goArgs) '-Apply' $Apply.IsPresent
  Add-RekitGoSwitch ([ref]$goArgs) '-CreateCandidates' $CreateCandidates.IsPresent
  Add-RekitGoSwitch ([ref]$goArgs) '-WhatIf' $WhatIf.IsPresent
  Add-RekitGoArg ([ref]$goArgs) '-ReviewOutputDir' $ReviewOutputDir
  Add-RekitGoArg ([ref]$goArgs) '-PacketPath' $PacketPath
  Add-RekitGoArg ([ref]$goArgs) '-DiffPath' $DiffPath
  if ($Command -in @('status','packs','release-check','doctor','validate','attach','repair','init','bootstrap','sync','update','promote','overview','note','start','handoff','continue')) { Add-RekitGoArg ([ref]$goArgs) '-Format' $Format }
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
      Reason = ''
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
    }
    foreach ($name in @($noteValues.Keys)) {
      if ([string]::IsNullOrWhiteSpace([string]$noteValues[$name]) -and $noteArgs.ContainsKey($name)) { $noteValues[$name] = [string]$noteArgs[$name] }
    }
    foreach ($name in @($noteValues.Keys)) {
      Add-RekitGoArg ([ref]$goArgs) ('-' + $name) ([string]$noteValues[$name])
    }
  }
  if ($Command -in @('start','handoff','continue')) {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Add-RekitGoArg ([ref]$goArgs) '-Target' ([string]$resolved.Target)
    foreach ($arg in @($resolved.Args)) {
      if (-not [string]::IsNullOrWhiteSpace([string]$arg)) {
        $goArgs = @($goArgs) + @([string]$arg)
      }
    }
    if ($Command -eq 'start') { Add-RekitGoSwitch ([ref]$goArgs) '-Force' $Force.IsPresent }
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
    Add-RekitGoArg ([ref]$goArgs) '-TriedLightSteps' $TriedLightSteps
    Add-RekitGoArg ([ref]$goArgs) '-StopConditions' $StopConditions
  }
  if ($Command -eq 'plan-subagents') {
    Add-RekitGoArg ([ref]$goArgs) '-Route' $Route
    Add-RekitGoArg ([ref]$goArgs) '-TaskType' $TaskType
    Add-RekitGoArg ([ref]$goArgs) '-Items' $Items
    Add-RekitGoArg ([ref]$goArgs) '-ItemsFile' $ItemsFile
    if ($ItemsPerAgent -gt 0) { Add-RekitGoArg ([ref]$goArgs) '-ItemsPerAgent' ([string]$ItemsPerAgent) }
    if ($MaxParallel -gt 0) { Add-RekitGoArg ([ref]$goArgs) '-MaxParallel' ([string]$MaxParallel) }
  }
  return $goArgs
}

function Invoke-RekitGoBackend {
  param([Parameter(Mandatory=$true)]$Invocation)
  $goArgs = Get-RekitGoArgs
  Push-Location $Invocation.WorkingDirectory
  try {
    $argv = @($Invocation.Prefix) + @($goArgs)
    & $Invocation.Command @argv
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
  } finally {
    Pop-Location
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
  throw "$Command is implemented by the Go backend only; PowerShell fallback has been retired for this command. Remove REKIT_GO_DISABLE or use go run ./cmd/rekit -- -Command $Command."
}

switch ($Command) {
  'packs' {
    $packs = @(Get-RekitPackInventory -RepoRoot $RepoRoot)
    $formatValue = ([string]$Format).Trim().ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($formatValue)) { $formatValue = 'table' }
    switch ($formatValue) {
      { $_ -in @('table','tsv') } {
        Write-Host "pack`tmaturity`tschema`troutes`tmanaged`ttooling`tauthority`tversion`tdescription"
        foreach ($packItem in $packs) {
          $schema = if ([bool]$packItem.SchemaValid) { 'ok' } else { 'error' }
          Write-Host ("{0}`t{1}`t{2}`t{3}`t{4}`t{5}`t{6}`t{7}`t{8}" -f $packItem.ID,$packItem.Maturity,$schema,$packItem.SubagentRoutes,$packItem.ManagedFiles,$packItem.ToolingFiles,$packItem.DefaultAuthorityLane,$packItem.Version,$packItem.Description)
          if (-not [bool]$packItem.SchemaValid -and -not [string]::IsNullOrWhiteSpace([string]$packItem.Error)) { Write-Host ("  error: {0}" -f $packItem.Error) }
        }
      }
      'json' {
        $packRows = @($packs | ForEach-Object {
          [ordered]@{
            id = [string]$_.ID
            name = [string]$_.Name
            version = [string]$_.Version
            maturity = [string]$_.Maturity
            description = [string]$_.Description
            manifestPath = [string]$_.ManifestPath
            schemaValid = [bool]$_.SchemaValid
            error = [string]$_.Error
            managedFiles = [int]$_.ManagedFiles
            templateFiles = [int]$_.TemplateFiles
            localFiles = [int]$_.LocalFiles
            promoteFiles = [int]$_.PromoteFiles
            toolingFiles = [int]$_.ToolingFiles
            promptFiles = [int]$_.PromptFiles
            subagentRoutes = [int]$_.SubagentRoutes
            heavyToolGates = [int]$_.HeavyToolGates
            heavyToolGateActions = @($_.HeavyToolGateActions)
            laneTypes = [int]$_.LaneTypes
            authorityFiles = [int]$_.AuthorityFiles
            defaultAuthorityLane = [string]$_.DefaultAuthorityLane
          }
        })
        [ordered]@{
          command = 'packs'
          schemaVersion = 1
          isMutation = $false
          packCount = [int]$packRows.Count
          packs = $packRows
        } | ConvertTo-Json -Depth 8
      }
      default { throw "unsupported packs format: $Format" }
    }
  }
  'release-check' {
    throw 'release-check is implemented by the Go backend only; use /rekit release-check or go run ./cmd/rekit -- -Command release-check.'
  }
  'overview' {
    $caseRoot = Resolve-RekitTarget $Target
    Show-RekitOverview -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -Format $Format
  }
  'continue' {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Invoke-RekitContinue -Target $resolved.Target -RepoRoot $RepoRoot -Pack $Pack -ActionArgs $resolved.Args -Format $Format -WhatIf:$WhatIf
  }
  'start' {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Invoke-RekitStart -Target $resolved.Target -RepoRoot $RepoRoot -Pack $Pack -ActionArgs $resolved.Args -Format $Format -WhatIf:$WhatIf -Force:$Force
  }
  'handoff' {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Write-RekitHandoff -Target $resolved.Target -RepoRoot $RepoRoot -Pack $Pack -ActionArgs $resolved.Args -Format $Format -WhatIf:$WhatIf
  }
  'note' {
    $caseRoot = Resolve-RekitTarget $Target
    $noteParams = @{
      Target = $caseRoot
      RepoRoot = $RepoRoot
      Pack = $Pack
      WhatIf = $WhatIf
      Lane = $Lane
      Subject = $Subject
      Summary = $Summary
      Actor = $Actor
      Risk = $Risk
      TargetRef = $TargetRef
      BatchId = $BatchId
      Action = $Action
      Scope = $Scope
      Format = $Format
    }
    $noteArgs = @($RemainingArgs)
    for ($i = 0; $i -lt $noteArgs.Count; $i++) {
      $token = [string]$noteArgs[$i]
      if ($token -like '-*=*') {
        $eq = $token.IndexOf('=')
        $name = $token.Substring(1, $eq - 1)
        $value = $token.Substring($eq + 1)
        $noteParams[$name] = $value
      } elseif ($token.StartsWith('-')) {
        $name = $token.Substring(1)
        if (($i + 1) -lt $noteArgs.Count -and -not ([string]$noteArgs[$i+1]).StartsWith('-')) {
          $noteParams[$name] = [string]$noteArgs[$i+1]; $i++
        } else {
          $noteParams[$name] = $true
        }
      }
    }
    Invoke-RekitNote @noteParams
  }
  'status' {
    $cwd = Resolve-RekitTarget $Target
    $formatValue = ([string]$Format).Trim().ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($formatValue)) { $formatValue = 'table' }
    switch ($formatValue) {
      { $_ -in @('table','text','tsv') } {
        Write-Host "rekit runtime: $RuntimeRoot"
        Write-Host "template root: $RepoRoot"
        Write-Host "pack: $Pack"
        if (Test-RekitLooksLikeCase $cwd) {
          $inst = Get-RekitInstance -Target $cwd
          Write-Host "case: $($inst.CaseRoot)"
          Write-Host "case metadata: $($inst.Source) $($inst.InstancePath)"
          Write-Host "case templateRoot: $($inst.TemplateRoot)"
          Write-Host "case templatePack: $($inst.TemplatePack)"
          if (Test-RekitInstanceMoved -Instance $inst) { Write-RekitMoveWarning -Instance $inst }
        } else {
          $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
          Write-Host "manifest: $($manifest.ManifestPath)"
          Write-Host "managed files: $($manifest.ManagedFiles.Count)"
          Write-Host "promote files: $($manifest.PromoteFiles.Count)"
          Write-Host "tooling files: $($manifest.ToolingFiles.Count)"
        }
      }
      'json' {
        $caseInfo = $null
        $manifestInfo = $null
        $mode = 'kit'
        if (Test-RekitLooksLikeCase $cwd) {
          $inst = Get-RekitInstance -Target $cwd
          $mode = 'case'
          $caseInfo = [ordered]@{
            caseRoot = [string]$inst.CaseRoot
            metadataSource = [string]$inst.Source
            instancePath = [string]$inst.InstancePath
            templateRoot = [string]$inst.TemplateRoot
            templatePack = [string]$inst.TemplatePack
            projectName = [string]$inst.ProjectName
            projectRoot = [string]$inst.ProjectRoot
            moved = [bool](Test-RekitInstanceMoved -Instance $inst)
          }
        } else {
          $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
          $manifestInfo = [ordered]@{
            manifestPath = [string]$manifest.ManifestPath
            managedFiles = [int]$manifest.ManagedFiles.Count
            promoteFiles = [int]$manifest.PromoteFiles.Count
            toolingFiles = [int]$manifest.ToolingFiles.Count
          }
        }
        [ordered]@{
          command = 'status'
          schemaVersion = 1
          isMutation = $false
          runtimeRoot = $RuntimeRoot
          templateRoot = $RepoRoot
          pack = $Pack
          target = $cwd
          targetProvided = -not [string]::IsNullOrWhiteSpace($Target)
          mode = $mode
          case = $caseInfo
          manifest = $manifestInfo
        } | ConvertTo-Json -Depth 8
      }
      default { throw "unsupported status format: $Format" }
    }
  }
  'attach' {
    $caseRoot = Resolve-RekitTarget $Target
    Invoke-RekitAttach -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -WhatIf:$WhatIf
  }
  'repair' {
    $caseRoot = Resolve-RekitTarget $Target
    Repair-RekitInstance -CaseRoot $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -Apply:$Apply
  }
  { $_ -in @('init','bootstrap') } {
    $caseRoot = Resolve-RekitTarget $Target
    Sync-RekitPack -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -WhatIf:$WhatIf -CreateLocalFiles -Apply -ForceLocalTemplates:$Force
  }
  { $_ -in @('sync','update') } {
    $caseRoot = Resolve-RekitTarget $Target
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    if ($Review -and $Apply) { throw 'sync -Review cannot be combined with -Apply. Review first, then run sync -Apply after user confirmation.' }
    $syncReview = (-not $Apply -and -not $WhatIf)
    if ($Review) { $syncReview = $true }
    if ($Review -and $WhatIf) { throw 'sync -Review cannot be combined with -WhatIf. Choose review artifacts or a PowerShell dry run.' }
    Sync-RekitPack -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -WhatIf:$WhatIf -Apply:$Apply -Review:$syncReview -ReviewOutputDir $ReviewOutputDir -PacketPath $PacketPath -DiffPath $DiffPath -ForceLocalTemplates:$Force
  }
  'promote' {
    $caseRoot = Resolve-RekitTarget $Target
    [void](Assert-RekitAttachedCase -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack)
    if ($Review -and ($Apply -or $CreateCandidates)) { throw 'promote -Review cannot be combined with -Apply or -CreateCandidates. Review first, then run an explicit write action after user confirmation.' }
    if ($Apply -and $CreateCandidates) { throw 'promote -Apply cannot be combined with -CreateCandidates. Choose one write action.' }
    $promoteReview = (-not $Apply -and -not $CreateCandidates -and -not $WhatIf)
    if ($Review) { $promoteReview = $true }
    if ($Review -and $WhatIf) { throw 'promote -Review cannot be combined with -WhatIf. Choose review artifacts or a PowerShell dry run.' }
    Promote-RekitChanges -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -WhatIf:$WhatIf -Apply:$Apply -CreateCandidates:$CreateCandidates -Review:$promoteReview -ReviewOutputDir $ReviewOutputDir -PacketPath $PacketPath -DiffPath $DiffPath
  }
  'plan-subagents' {
    $planRoot = Resolve-RekitTarget $Target
    if (Test-RekitLooksLikeCase $planRoot) {
      [void](Assert-RekitAttachedCase -Target $planRoot -RepoRoot $RepoRoot -Pack $Pack)
    } elseif ([string]::IsNullOrWhiteSpace($ReviewOutputDir)) {
      throw 'plan-subagents target must be an attached rekit case unless -ReviewOutputDir is provided for an explicit out-of-case review artifact path.'
    } elseif (-not (Test-Path -LiteralPath $planRoot)) {
      throw "plan-subagents target directory does not exist: $planRoot"
    }
    Write-RekitSubagentPlan -Target $planRoot -RepoRoot $RepoRoot -Pack $Pack -Route $Route -TaskType $TaskType -Items $Items -ItemsFile $ItemsFile -ItemsPerAgent $ItemsPerAgent -MaxParallel $MaxParallel -ReviewOutputDir $ReviewOutputDir -PacketPath $PacketPath -DiffPath $DiffPath
  }
  { $_ -in @('validate','doctor') } {
    $mode = 'pack'
    $doctorTarget = $RepoRoot
    $rows = @()
    $summary = 'pack validation ok'
    if ([string]::IsNullOrWhiteSpace($Target)) {
      $cwd = Resolve-RekitTarget ''
      if (Test-RekitLooksLikeCase $cwd -and (-not [string]::Equals($cwd, $RepoRoot, [System.StringComparison]::OrdinalIgnoreCase))) {
        [void](Assert-RekitAttachedCase -Target $cwd -RepoRoot $RepoRoot -Pack $Pack)
        $mode = 'case'
        $doctorTarget = $cwd
        $rows = @(Test-RekitInstance -Target $cwd -RepoRoot $RepoRoot -Pack $Pack)
        $summary = 'instance validation ok'
      } else {
        $rows = @(Test-RekitPack -RepoRoot $RepoRoot -Pack $Pack)
      }
    } else {
      $resolvedTarget = Resolve-RekitTarget $Target
      $doctorTarget = $resolvedTarget
      if ([string]::Equals($resolvedTarget, $RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        $rows = @(Test-RekitPack -RepoRoot $RepoRoot -Pack $Pack)
      } elseif (Test-RekitLooksLikeCase $resolvedTarget) {
        [void](Assert-RekitAttachedCase -Target $resolvedTarget -RepoRoot $RepoRoot -Pack $Pack)
        $mode = 'case'
        $rows = @(Test-RekitInstance -Target $resolvedTarget -RepoRoot $RepoRoot -Pack $Pack)
        $summary = 'instance validation ok'
      } else {
        throw "target is neither this kit root nor an attached rekit case: $resolvedTarget"
      }
    }
    $formatValue = ([string]$Format).Trim().ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($formatValue)) { $formatValue = 'table' }
    switch ($formatValue) {
      { $_ -in @('table','text','tsv') } {
        $rows | ForEach-Object {
          Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
        }
        Write-Host $summary
      }
      'json' {
        $jsonRows = @($rows | ForEach-Object {
          [ordered]@{
            file = [string]$_.File
            bytes = [int64]$_.Bytes
            limit = [int64]$_.Limit
          }
        })
        [ordered]@{
          command = $Command
          schemaVersion = 1
          isMutation = $false
          pack = $Pack
          target = $doctorTarget
          mode = $mode
          valid = $true
          summary = $summary
          rows = $jsonRows
        } | ConvertTo-Json -Depth 8
      }
      default { throw "unsupported $Command format: $Format" }
    }
  }
}
