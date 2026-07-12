[CmdletBinding(PositionalBinding=$false)]
param(
  [Parameter(Position=0)]
  [ValidateSet('status','packs','attach','repair','init','bootstrap','sync','update','promote','validate','doctor','plan-subagents','overview','continue','start','handoff','note','gate')]
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

function Test-RekitEnvTruthy {
  param([string]$Name)
  $value = [Environment]::GetEnvironmentVariable($Name)
  if ([string]::IsNullOrWhiteSpace($value)) { return $false }
  $normalized = $value.Trim().ToLowerInvariant()
  return @('1','true','yes','on') -contains $normalized
}

function Test-RekitGoDelegationEnabled {
  if (Test-RekitEnvTruthy 'REKIT_GO_DISABLE') { return $false }
  return (Test-RekitEnvTruthy 'REKIT_GO_ENABLE')
}

function Test-RekitGoDelegationSafe {
  switch ($Command) {
    { $_ -in @('status','packs') } { return $true }
    { $_ -in @('doctor','validate') } { return $true }
    { $_ -in @('sync','update') } {
      if ($Apply -or $WhatIf) { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      return (Test-RekitLooksLikeCase $caseRoot)
    }
    'promote' {
      if ($Apply -or $CreateCandidates -or $WhatIf) { return $false }
      $caseRoot = Resolve-RekitTarget $Target
      return (Test-RekitLooksLikeCase $caseRoot)
    }
    'gate' {
      return ($WhatIf -and -not $Apply)
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
    { $_ -in @('status','packs') } { return (Resolve-RekitTarget $Target) }
    { $_ -in @('sync','update','promote','gate') } { return (Resolve-RekitTarget $Target) }
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
  Add-RekitGoArg ([ref]$goArgs) '-Target' $goTarget
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
  if ($Command -in @('status','packs','doctor','validate')) { Add-RekitGoArg ([ref]$goArgs) '-Format' $Format }
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

if ($Command -eq 'gate') {
  throw 'gate is implemented by the Go backend only; set REKIT_GO_ENABLE=1 and use -WhatIf for facade delegation, or run go run ./cmd/rekit -- -Command gate manually.'
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
  'overview' {
    $caseRoot = Resolve-RekitTarget $Target
    Show-RekitOverview -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -Format $Format
  }
  'continue' {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Invoke-RekitContinue -Target $resolved.Target -RepoRoot $RepoRoot -Pack $Pack -ActionArgs $resolved.Args -WhatIf:$WhatIf
  }
  'start' {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Invoke-RekitStart -Target $resolved.Target -RepoRoot $RepoRoot -Pack $Pack -ActionArgs $resolved.Args -WhatIf:$WhatIf -Force:$Force
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
    Sync-RekitPack -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -WhatIf:$WhatIf -Apply:$Apply -Review:$syncReview -ReviewOutputDir $ReviewOutputDir -PacketPath $PacketPath -DiffPath $DiffPath
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
