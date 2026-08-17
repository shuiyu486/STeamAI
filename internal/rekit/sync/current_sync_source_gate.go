package sync

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func rebuildCurrentSyncPlanForStaging(
	repoRoot,
	caseRoot,
	pack,
	expectedPlanSHA256 string,
	opt CurrentSyncOptions,
	intent currentSyncIntent,
) (CurrentSyncPlan, error) {
	if err := validateCurrentSyncIntentProject(
		intent,
		caseRoot,
		caseRootStateRoot(caseRoot),
	); err != nil {
		return CurrentSyncPlan{}, err
	}
	if err := validateCurrentSyncApplyIdentity(
		intent,
		repoRoot,
		caseRoot,
		pack,
		expectedPlanSHA256,
		opt,
	); err != nil {
		return CurrentSyncPlan{}, err
	}
	if err := validateCurrentSyncIntentSource(intent); err != nil {
		return CurrentSyncPlan{}, err
	}
	fresh, err := buildCurrentSyncPlan(repoRoot, caseRoot, pack, opt)
	if err != nil {
		return CurrentSyncPlan{}, err
	}
	if fresh.prepared == nil || fresh.AlreadyCurrent ||
		!strings.EqualFold(
			fresh.ExpectedPlanSHA256,
			expectedPlanSHA256,
		) ||
		!currentSyncCanonicalEqual(
			currentSyncIdentity(fresh),
			currentSyncIdentity(intent.Plan),
		) {
		return CurrentSyncPlan{}, fmt.Errorf(
			"current sync plan changed after preview; rerun sync preview",
		)
	}
	return fresh, nil
}

func validateCurrentSyncApplyIdentity(
	intent currentSyncIntent,
	repoRoot,
	caseRoot,
	pack,
	expectedPlanSHA256 string,
	opt CurrentSyncOptions,
) error {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return err
	}
	plan := intent.Plan
	if !validCurrentSyncSHA(expectedPlanSHA256) ||
		!strings.EqualFold(expectedPlanSHA256, intent.PlanSHA256) ||
		!sameCurrentSyncPath(repoRoot, plan.SourceRepoRoot) ||
		!sameCurrentSyncPath(caseRoot, plan.CaseRoot) ||
		strings.TrimSpace(pack) != plan.Pack ||
		strings.TrimSpace(opt.Command) != plan.Command ||
		strings.TrimSpace(opt.ProjectName) != plan.ProjectName ||
		opt.ForceLocalTemplates ||
		!sameCurrentSyncPath(opt.SourceExecutable, plan.SourceExecutable) {
		return fmt.Errorf(
			"current sync Apply invocation differs from the durable reviewed plan",
		)
	}
	return nil
}

func caseRootStateRoot(caseRoot string) string {
	return filepath.Join(caseRoot, projectstate.CurrentDir)
}
