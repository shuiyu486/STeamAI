function Get-RekitBoardPaths {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $case = [System.IO.Path]::GetFullPath($CaseRoot)
  $root = Join-Path $case '.rekit'
  return [pscustomobject]@{
    CaseRoot = $case
    Root = $root
    Board = Join-Path $root 'board.json'
    Policy = Join-Path $root 'policy.yml'
    Lanes = Join-Path $root 'lanes'
    Facts = Join-Path $root 'facts'
    Runs = Join-Path $root 'runs'
    Reviews = Join-Path $root 'reviews'
    Backups = Join-Path $root 'backups'
    Observations = Join-Path $root 'facts\observations.jsonl'
    Candidates = Join-Path $root 'facts\candidates.jsonl'
    Requests = Join-Path $root 'facts\requests.jsonl'
    Publications = Join-Path $root 'facts\publications.jsonl'
    Decisions = Join-Path $root 'facts\decisions.jsonl'
  }
}

function Save-RekitBoard {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [Parameter(Mandatory=$true)]$Manifest
  )
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  $lanes = @()
  foreach ($dir in (Get-RekitLaneDirectories -CaseRoot $CaseRoot)) {
    $lane = Read-RekitJsonFile -Path (Join-Path $dir.FullName 'lane.json')
    if ($null -ne $lane) {
      $lanes += [ordered]@{ id = $lane.id; type = $lane.type; title = $lane.title; status = $lane.status; authority = $lane.authority; workspace = $lane.workspace; updatedAt = $lane.updatedAt }
    }
  }
  $board = [ordered]@{
    schemaVersion = 1
    caseRoot = [System.IO.Path]::GetFullPath($CaseRoot)
    repoRoot = [System.IO.Path]::GetFullPath($RepoRoot)
    pack = $Manifest.Pack
    automationMode = (Get-RekitPolicy -CaseRoot $CaseRoot).automationMode
    defaultAuthorityLane = 'devirt-main'
    lanes = $lanes
    factsRoot = '.rekit/facts'
    updatedAt = New-RekitIsoTime
  }
  Write-RekitJsonFile -Path $paths.Board -Object $board
  return $board
}

function Ensure-RekitBoard {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [Parameter(Mandatory=$true)][string]$RepoRoot,
    [string]$Pack = 'vmp-re',
    [switch]$CreateDefaultLane
  )
  $case = [System.IO.Path]::GetFullPath($CaseRoot)
  [void](Assert-RekitAttachedCase -Target $case -RepoRoot $RepoRoot -Pack $Pack)
  $manifest = Get-RekitPackManifest -RepoRoot $RepoRoot -Pack $Pack
  $paths = Get-RekitBoardPaths -CaseRoot $case
  foreach ($dir in @($paths.Root,$paths.Lanes,$paths.Facts,$paths.Runs,$paths.Reviews,$paths.Backups)) { Ensure-RekitDirectory $dir }
  foreach ($file in @($paths.Observations,$paths.Candidates,$paths.Requests,$paths.Publications,$paths.Decisions)) {
    if (-not (Test-Path -LiteralPath $file)) { [System.IO.File]::WriteAllText($file, '', [System.Text.UTF8Encoding]::new($false)) }
  }
  [void](Ensure-RekitPolicyFile -CaseRoot $case)
  if ($CreateDefaultLane -and -not (Test-Path -LiteralPath (Join-Path (Get-RekitLanePath -CaseRoot $case -LaneId 'devirt-main') 'lane.json'))) {
    New-RekitLane -CaseRoot $case -RepoRoot $RepoRoot -Manifest $manifest -Type 'devirt-main' -Name '' | Out-Null
  }
  return Save-RekitBoard -CaseRoot $case -RepoRoot $RepoRoot -Manifest $manifest
}
