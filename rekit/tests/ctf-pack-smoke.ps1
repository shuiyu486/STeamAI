param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

. (Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'pack-smoke-lib.ps1')

Invoke-RekitPackSmoke `
  -WorkRoot $WorkRoot `
  -Pack 'ctf' `
  -CasePrefix 'ctf' `
  -PlanTaskType 'pwn-analysis' `
  -PlanItems 'challenge-a,artifact-b' `
  -ExpectedPlanRoute 'ctf:challenge-analysis' `
  -ExpectedOutputContractText 'challenge_ref' `
  -FacadeTaskType 'solution-review' `
  -FacadeItems 'solution-a,writeup-b'
