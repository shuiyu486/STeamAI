package workstream

import (
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestContinuePlanBindingUsesCanonicalLaneIdentity(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	alias, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{
		Selector:                   "main",
		Executor:                   "executor-one",
		ExpectedExecutorGeneration: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	exact, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{
		Selector:                   "devirt-main",
		ExactSelector:              true,
		Executor:                   "executor-one",
		ExpectedExecutorGeneration: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if alias.ContinuePlanSHA256 == "" || alias.ContinuePlanSHA256 != exact.ContinuePlanSHA256 {
		t.Fatalf("equivalent lane selectors produced different continue plans: alias=%s exact=%s", alias.ContinuePlanSHA256, exact.ContinuePlanSHA256)
	}
	if alias.MissionCommanderActionQueue.CurrentDriverRequest == nil || exact.MissionCommanderActionQueue.CurrentDriverRequest == nil {
		t.Fatalf("continue preview omitted Apply request: alias=%+v exact=%+v", alias.MissionCommanderActionQueue.CurrentDriverRequest, exact.MissionCommanderActionQueue.CurrentDriverRequest)
	}
	aliasPlan, aliasPresent, aliasValid := alias.MissionCommanderActionQueue.CurrentDriverRequest.Invocation.FlagValue("-ExpectedContinuePlanSha256", "--expected-continue-plan-sha256")
	exactPlan, exactPresent, exactValid := exact.MissionCommanderActionQueue.CurrentDriverRequest.Invocation.FlagValue("-ExpectedContinuePlanSha256", "--expected-continue-plan-sha256")
	if !aliasPresent || !aliasValid || !exactPresent || !exactValid || aliasPlan != exactPlan || aliasPlan != alias.ContinuePlanSHA256 {
		t.Fatalf("equivalent lane projections did not bind one canonical plan: alias=%+v exact=%+v", alias.MissionCommanderActionQueue.CurrentDriverRequest, exact.MissionCommanderActionQueue.CurrentDriverRequest)
	}
}
