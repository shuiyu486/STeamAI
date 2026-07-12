param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

. (Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'pack-smoke-lib.ps1')

Invoke-RekitPackSmoke `
  -WorkRoot $WorkRoot `
  -Pack 'unpack-pe' `
  -CasePrefix 'upe' `
  -PlanTaskType 'loader-triage' `
  -PlanItems 'sample-a,loader-b' `
  -ExpectedPlanRoute 'unpack-pe:unpack-analysis' `
  -ExpectedOutputContractText 'sample_ref' `
  -FacadeTaskType 'unpack-review' `
  -FacadeItems 'finding-a,candidate-b'
