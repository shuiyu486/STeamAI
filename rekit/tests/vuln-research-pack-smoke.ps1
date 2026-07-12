param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

. (Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'pack-smoke-lib.ps1')

Invoke-RekitPackSmoke `
  -WorkRoot $WorkRoot `
  -Pack 'vuln-research' `
  -CasePrefix 'vrp' `
  -PlanTaskType 'crash-triage' `
  -PlanItems 'crash-a,patch-b' `
  -ExpectedPlanRoute 'vuln-research:vuln-analysis' `
  -ExpectedOutputContractText 'target_ref' `
  -FacadeTaskType 'finding-review' `
  -FacadeItems 'finding-a,repro-b'
