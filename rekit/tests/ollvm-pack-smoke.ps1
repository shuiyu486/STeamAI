param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

. (Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'pack-smoke-lib.ps1')

Invoke-RekitPackSmoke `
  -WorkRoot $WorkRoot `
  -Pack 'ollvm' `
  -CasePrefix 'ol' `
  -PlanTaskType 'control-flow-triage' `
  -PlanItems 'function-a,region-b' `
  -ExpectedPlanRoute 'ollvm:obfuscation-analysis' `
  -ExpectedOutputContractText 'function_ref' `
  -FacadeTaskType 'cfg-review' `
  -FacadeItems 'finding-a,candidate-b'
