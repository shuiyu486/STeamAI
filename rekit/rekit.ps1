[CmdletBinding(PositionalBinding=$false)]
param(
  [Parameter(Position=0)]
  [ValidateSet('status','attach','repair','init','bootstrap','sync','update','promote','validate','doctor','plan-subagents','board','lane','auto','policy')]
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
  [string]$Route = '',
  [string]$TaskType = '',
  [string]$Items = '',
  [string]$ItemsFile = '',
  [int]$ItemsPerAgent = 0,
  [int]$MaxParallel = 0,
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
. (Join-Path $RuntimeRoot 'lib\Board.ps1')

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

switch ($Command) {
  'board' {
    $caseRoot = Resolve-RekitTarget $Target
    Show-RekitBoard -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack
  }
  'lane' {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Invoke-RekitLaneCommand -Target $resolved.Target -RepoRoot $RepoRoot -Pack $Pack -ActionArgs $resolved.Args -WhatIf:$WhatIf -Force:$Force
  }
  'auto' {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Invoke-RekitAuto -Target $resolved.Target -RepoRoot $RepoRoot -Pack $Pack -ActionArgs $resolved.Args -WhatIf:$WhatIf
  }
  'policy' {
    $resolved = Resolve-RekitActionTargetAndArgs -Value $Target -Remaining $RemainingArgs
    Invoke-RekitPolicyCommand -Target $resolved.Target -ActionArgs $resolved.Args
  }
  'status' {
    $cwd = Resolve-RekitTarget $Target
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
    if ([string]::IsNullOrWhiteSpace($Target)) {
      $cwd = Resolve-RekitTarget ''
      if (Test-RekitLooksLikeCase $cwd -and (-not [string]::Equals($cwd, $RepoRoot, [System.StringComparison]::OrdinalIgnoreCase))) {
        [void](Assert-RekitAttachedCase -Target $cwd -RepoRoot $RepoRoot -Pack $Pack)
        Test-RekitInstance -Target $cwd -RepoRoot $RepoRoot -Pack $Pack | ForEach-Object {
          Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
        }
        Write-Host 'instance validation ok'
      } else {
        Test-RekitPack -RepoRoot $RepoRoot -Pack $Pack | ForEach-Object {
          Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
        }
        Write-Host 'pack validation ok'
      }
    } else {
      $resolvedTarget = Resolve-RekitTarget $Target
      if ([string]::Equals($resolvedTarget, $RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        Test-RekitPack -RepoRoot $RepoRoot -Pack $Pack | ForEach-Object {
          Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
        }
        Write-Host 'pack validation ok'
      } elseif (Test-RekitLooksLikeCase $resolvedTarget) {
        [void](Assert-RekitAttachedCase -Target $resolvedTarget -RepoRoot $RepoRoot -Pack $Pack)
        Test-RekitInstance -Target $resolvedTarget -RepoRoot $RepoRoot -Pack $Pack | ForEach-Object {
          Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
        }
        Write-Host 'instance validation ok'
      } else {
        throw "target is neither this kit root nor an attached rekit case: $resolvedTarget"
      }
    }
  }
}
