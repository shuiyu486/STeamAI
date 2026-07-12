param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

. (Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'pack-smoke-lib.ps1')

Invoke-RekitPackSmoke `
  -WorkRoot $WorkRoot `
  -Pack 'generic-binary-re' `
  -CasePrefix 'gbr' `
  -PlanTaskType 'function-analysis' `
  -PlanItems 'binary-a,function-b' `
  -ExpectedPlanRoute 'generic-binary-re:binary-analysis' `
  -ExpectedOutputContractText 'function_ref' `
  -FacadeTaskType 'function-review' `
  -FacadeItems 'finding-a,candidate-b'
