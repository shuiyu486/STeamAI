param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

. (Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'pack-smoke-lib.ps1')

Invoke-RekitPackSmoke `
  -WorkRoot $WorkRoot `
  -Pack 'web-security' `
  -CasePrefix 'wsp' `
  -PlanTaskType 'endpoint-analysis' `
  -PlanItems '/login,/api/orders' `
  -ExpectedPlanRoute 'web-security:feature-analysis' `
  -ExpectedOutputContractText 'endpoint' `
  -FacadeTaskType 'finding-review' `
  -FacadeItems 'finding-a,finding-b'
