param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases',
  [string]$Pack = 'binary-re'
)

$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RekitRoot = Split-Path -Parent $ScriptDir
$RepoRoot = Split-Path -Parent $RekitRoot

function Invoke-GoTest {
  param([Parameter(Mandatory=$true)][string[]]$Arguments)
  Push-Location $RepoRoot
  try {
    & go test @Arguments
    if ($LASTEXITCODE -ne 0) { throw "go test failed: go test $($Arguments -join ' ')" }
  } finally {
    Pop-Location
  }
}

# WorkRoot and Pack are accepted for compatibility with older smoke invocations.
# The current preflight regression is Go-owned and uses package-level temp dirs.
Invoke-GoTest -Arguments @('./internal/rekit/workstream', '-run', 'TestContinueAuthorityAppendReasonPolicyMatrix')
Invoke-GoTest -Arguments @('./internal/rekit/cli', '-run', 'TestRunContinue(WhatIfDoesNotWrite|ApplyWritesDigestAndFacts)')

'continue preflight smoke ok'
