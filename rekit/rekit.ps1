param(
  [Parameter(Position=0)]
  [ValidateSet('status','attach','init','bootstrap','sync','update','promote','validate')]
  [string]$Command = 'status',
  [string]$Target = '',
  [string]$Pack = 'vmp-re',
  [string]$ProjectName = '',
  [switch]$WhatIf,
  [switch]$Apply,
  [switch]$Force
)

$ErrorActionPreference = 'Stop'
$RuntimeRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = [System.IO.Path]::GetFullPath((Join-Path $RuntimeRoot '..'))

. (Join-Path $RuntimeRoot 'lib\Manifest.ps1')
. (Join-Path $RuntimeRoot 'lib\Instance.ps1')
. (Join-Path $RuntimeRoot 'lib\Validate.ps1')
. (Join-Path $RuntimeRoot 'lib\Sync.ps1')
. (Join-Path $RuntimeRoot 'lib\Promote.ps1')

function Resolve-RekitTarget {
  param([string]$Value)
  if ([string]::IsNullOrWhiteSpace($Value)) { return [System.IO.Path]::GetFullPath((Get-Location).Path) }
  return [System.IO.Path]::GetFullPath($Value)
}

function Test-RekitLooksLikeCase {
  param([string]$Path)
  return (Test-Path -LiteralPath (Join-Path $Path '.rekit\instance.yml')) -or (Test-Path -LiteralPath (Join-Path $Path '.re-template.yml'))
}

switch ($Command) {
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
    } else {
      $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
      Write-Host "manifest: $($manifest.ManifestPath)"
      Write-Host "managed files: $($manifest.ManagedFiles.Count)"
      Write-Host "promote files: $($manifest.PromoteFiles.Count)"
    }
  }
  'attach' {
    $caseRoot = Resolve-RekitTarget $Target
    Invoke-RekitAttach -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -WhatIf:$WhatIf
  }
  { $_ -in @('init','bootstrap') } {
    $caseRoot = Resolve-RekitTarget $Target
    Sync-RekitPack -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -WhatIf:$WhatIf -CreateLocalFiles
  }
  { $_ -in @('sync','update') } {
    $caseRoot = Resolve-RekitTarget $Target
    Sync-RekitPack -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -ProjectName $ProjectName -WhatIf:$WhatIf
  }
  'promote' {
    $caseRoot = Resolve-RekitTarget $Target
    Promote-RekitChanges -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack -WhatIf:$WhatIf -Apply:$Apply
  }
  'validate' {
    if ([string]::IsNullOrWhiteSpace($Target)) {
      $cwd = Resolve-RekitTarget ''
      if (Test-RekitLooksLikeCase $cwd -and ($cwd -ne $RepoRoot)) {
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
      $caseRoot = Resolve-RekitTarget $Target
      if (($caseRoot -eq $RepoRoot) -or (-not (Test-RekitLooksLikeCase $caseRoot))) {
        Test-RekitPack -RepoRoot $RepoRoot -Pack $Pack | ForEach-Object {
          Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
        }
        Write-Host 'pack validation ok'
      } else {
        Test-RekitInstance -Target $caseRoot -RepoRoot $RepoRoot -Pack $Pack | ForEach-Object {
          Write-Host ("{0}`t{1}/{2}" -f $_.File, $_.Bytes, $_.Limit)
        }
        Write-Host 'instance validation ok'
      }
    }
  }
}
