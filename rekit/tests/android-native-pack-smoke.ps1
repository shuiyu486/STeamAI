param(
  [string]$WorkRoot = 'C:\AI\m_projects\RE\_dryrun_cases'
)

$ErrorActionPreference = 'Stop'

. (Join-Path (Split-Path -Parent $MyInvocation.MyCommand.Path) 'pack-smoke-lib.ps1')

Invoke-RekitPackSmoke `
  -WorkRoot $WorkRoot `
  -Pack 'android-native' `
  -CasePrefix 'an' `
  -PlanTaskType 'jni-triage' `
  -PlanItems 'app-a,library-b' `
  -ExpectedPlanRoute 'android-native:native-analysis' `
  -ExpectedOutputContractText 'jni_symbol_ref' `
  -FacadeTaskType 'jni-review' `
  -FacadeItems 'finding-a,candidate-b'
