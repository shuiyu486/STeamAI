function Get-RekitDefaultPolicyText {
  return @'
schemaVersion: 1
automationMode: assisted-autopilot
autoCollect: true
autoVerify: true
autoRouteRequests: true
autoSyncLanes: true
autoPublishSharedFacts: true
autoAcceptLowRiskCandidates: true
authorityAutoAppend: conditional
authorityAutoOverwrite: never
authorityAutoDelete: never
requireEvidence: true
requireVerifier: true
minConfidence: 0.90
requireNoConflict: true
requireSchemaValid: true
requireBackup: true
requireDiff: true
maxAuthorityRowsPerRun: 10
askUserWhen: conflict,overwriteAuthority,deleteAuthority,confidenceBelowThreshold,schemaChange,changesProjectBaseline,externalSideEffect,destructiveAction
'@
}

function Ensure-RekitPolicyFile {
  param([Parameter(Mandatory=$true)][string]$CaseRoot)
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  if (-not (Test-Path -LiteralPath $paths.Policy)) {
    Ensure-RekitDirectory (Split-Path -Parent $paths.Policy)
    [System.IO.File]::WriteAllText($paths.Policy, (Get-RekitDefaultPolicyText), [System.Text.UTF8Encoding]::new($false))
  }
  return $paths.Policy
}

function Get-RekitPolicy {
  param(
    [Parameter(Mandatory=$true)][string]$CaseRoot,
    [switch]$NoCreate
  )
  $paths = Get-RekitBoardPaths -CaseRoot $CaseRoot
  if ((Test-Path -LiteralPath $paths.Policy) -or -not $NoCreate) {
    $path = Ensure-RekitPolicyFile -CaseRoot $CaseRoot
    $lines = [System.IO.File]::ReadAllLines($path, [System.Text.Encoding]::UTF8)
  } else {
    $lines = (Get-RekitDefaultPolicyText) -split "`r?`n"
  }
  $policy = [ordered]@{}
  foreach ($line in $lines) {
    $trim = $line.Trim()
    if ($trim -eq '' -or $trim.StartsWith('#') -or -not $trim.Contains(':')) { continue }
    $parts = $trim.Split(':', 2)
    $key = $parts[0].Trim()
    $value = Convert-RekitYamlValue $parts[1]
    $policy[$key] = $value
  }
  foreach ($pair in @{
    automationMode = 'assisted-autopilot'; autoCollect = 'true'; autoVerify = 'true'; autoRouteRequests = 'true'; autoSyncLanes = 'true'; autoPublishSharedFacts = 'true'; autoAcceptLowRiskCandidates = 'true'; authorityAutoAppend = 'conditional'; authorityAutoOverwrite = 'never'; authorityAutoDelete = 'never'; requireEvidence = 'true'; requireVerifier = 'true'; minConfidence = '0.90'; requireNoConflict = 'true'; requireSchemaValid = 'true'; requireBackup = 'true'; requireDiff = 'true'; maxAuthorityRowsPerRun = '10'
  }.GetEnumerator()) {
    if (-not $policy.Contains($pair.Key)) { $policy[$pair.Key] = $pair.Value }
  }
  return [pscustomobject]$policy
}

function Test-RekitPolicyBool {
  param(
    [Parameter(Mandatory=$true)]$Policy,
    [Parameter(Mandatory=$true)][string]$Name,
    [bool]$Default = $false
  )
  $prop = $Policy.PSObject.Properties[$Name]
  if ($null -eq $prop) { return $Default }
  return Convert-RekitYamlBool $prop.Value $Default
}

function Get-RekitPolicyNumber {
  param(
    [Parameter(Mandatory=$true)]$Policy,
    [Parameter(Mandatory=$true)][string]$Name,
    [double]$Default = 0
  )
  $prop = $Policy.PSObject.Properties[$Name]
  if ($null -eq $prop) { return $Default }
  $out = 0.0
  if ([double]::TryParse([string]$prop.Value, [ref]$out)) { return $out }
  return $Default
}
