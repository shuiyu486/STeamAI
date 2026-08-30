package sessionhost

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

const (
	dailyAdoptionInitialize = "initialize-in-place"
	dailyAdoptionConfirm    = "confirm-exact-plan"
	dailyAdoptionCancel     = "cancel"
)

type DailyDirectoryAdoption struct {
	State  string                  `json:"state"`
	Choice string                  `json:"choice,omitempty"`
	Plan   *syncreview.InitPlan    `json:"plan,omitempty"`
	Apply  *syncreview.ApplyResult `json:"apply,omitempty"`
}

func runDailyDirectoryAdoption(opt DailyOptions, result *DailyResult) error {
	action := strings.ToLower(strings.TrimSpace(opt.DirectoryAdoptionAction))
	expected := strings.TrimSpace(opt.ExpectedInitPlanSHA256)
	if strings.TrimSpace(opt.Correction) != "" {
		return fmt.Errorf("daily directory adoption does not accept a correction before onboarding")
	}
	if strings.TrimSpace(opt.SelectedLane) != "" {
		return fmt.Errorf("daily directory adoption does not accept a lane before onboarding")
	}
	if strings.TrimSpace(opt.DirectoryAdoptionPack) != "" && action != dailyAdoptionInitialize && action != dailyAdoptionConfirm {
		return fmt.Errorf("daily directory adoption pack requires initialize-in-place or confirm-exact-plan")
	}
	result.DirectoryAdoption = &DailyDirectoryAdoption{
		State:  DailyActionDirectoryAdoptionRequired,
		Choice: action,
	}

	switch action {
	case "":
		if expected != "" {
			return fmt.Errorf("daily directory adoption plan hash requires an exact adoption action")
		}
		result.FinalState = DailyActionDirectoryAdoptionRequired
		result.Blocked = true
		result.Action = dailyAction(DailyActionDirectoryAdoptionRequired)
		return nil
	case dailyAdoptionCancel:
		if expected != "" {
			return fmt.Errorf("daily directory adoption cancel does not accept a plan hash")
		}
		result.FinalState = "directory-adoption-cancelled"
		result.DirectoryAdoption.State = result.FinalState
		result.Action = &DailyUserAction{
			Code:    DailyActionCompleted,
			Message: "已取消当前目录接入；目录保持不变，也没有启动 Claude。",
			Now:     "当前目录接入已取消。",
			Reason:  "取消操作不写入项目，也不启动 Claude。",
			Next:    "需要时可重新请求安全接入预览。",
		}
		return nil
	case dailyAdoptionInitialize:
		if expected != "" {
			return fmt.Errorf("daily directory adoption preview does not accept a plan hash")
		}
		pack, err := dailyDirectoryAdoptionPack(opt)
		if err != nil {
			return err
		}
		plan, err := dailyDirectoryAdoptionPreview(opt, result.CaseRoot, pack)
		if err != nil {
			return err
		}
		result.DirectoryAdoption.Plan = &plan
		if !plan.AdoptionReady {
			result.FinalState = "directory-adoption-blocked"
			result.Blocked = true
			result.DirectoryAdoption.State = result.FinalState
			result.Action = dailyAction(DailyActionBlocked)
			result.Action.Reason = "安全接入计划会碰到不能自动保留的现有项目内容。"
			result.Action.Next = "检查接入计划中的 adoptionBlockers，解决冲突后重新预览。"
			return nil
		}
		result.FinalState = DailyActionConfirmationRequired
		result.Blocked = true
		result.DirectoryAdoption.State = result.FinalState
		result.Action = dailyAction(DailyActionConfirmationRequired)
		return nil
	case dailyAdoptionConfirm:
		if expected == "" {
			return fmt.Errorf("daily directory adoption confirmation requires the exact init plan SHA-256")
		}
		pack, err := dailyDirectoryAdoptionPack(opt)
		if err != nil {
			return err
		}
		repoRoot := strings.TrimSpace(opt.InitializationRepoRoot)
		if repoRoot == "" {
			return fmt.Errorf("daily directory adoption requires the external source repository owner")
		}
		if strings.TrimSpace(opt.InitializationSourceExecutable) == "" {
			return fmt.Errorf("daily directory adoption requires a verified unified runtime executable source")
		}
		applyOpt := dailyDirectoryAdoptionOptions(opt.InitializationSourceExecutable)
		applyOpt.ExpectedPlanSHA256 = expected
		applied, err := syncreview.Apply(
			repoRoot,
			result.CaseRoot,
			pack,
			applyOpt,
		)
		if err != nil {
			return fmt.Errorf("apply exact daily directory adoption plan: %w", err)
		}
		classified, err := classifyDailyTarget(result.CaseRoot)
		if err != nil {
			return fmt.Errorf("verify adopted daily target: %w", err)
		}
		if classified.Kind != dailyTargetAttached {
			return fmt.Errorf("daily directory adoption did not produce an attached project: %s", classified.Kind)
		}
		result.FinalState = DailyActionReadyToContinue
		result.DirectoryAdoption.State = result.FinalState
		result.DirectoryAdoption.Apply = &applied
		result.Action = &DailyUserAction{
			Code:    DailyActionReadyToContinue,
			Message: "当前目录已按精确计划安全接入；本次没有启动 Claude。",
			Now:     "当前目录已接入 STeamAI。",
			Reason:  "canonical init Apply 已重新验证计划哈希并完成有界写入。",
			Next:    "重新发起原目标，让 fresh daily 状态进入项目 onboarding。",
		}
		return nil
	default:
		return fmt.Errorf("unknown daily directory adoption action %q", opt.DirectoryAdoptionAction)
	}
}

func dailyDirectoryAdoptionPreview(opt DailyOptions, caseRoot, pack string) (syncreview.InitPlan, error) {
	repoRoot := strings.TrimSpace(opt.InitializationRepoRoot)
	if repoRoot == "" {
		return syncreview.InitPlan{}, fmt.Errorf("daily directory adoption requires the external source repository owner")
	}
	if strings.TrimSpace(opt.InitializationSourceExecutable) == "" {
		return syncreview.InitPlan{}, fmt.Errorf("daily directory adoption requires a verified unified runtime executable source")
	}
	plan, err := syncreview.InitPreview(
		repoRoot,
		caseRoot,
		pack,
		dailyDirectoryAdoptionOptions(opt.InitializationSourceExecutable),
	)
	if err != nil {
		return syncreview.InitPlan{}, fmt.Errorf("preview daily directory adoption: %w", err)
	}
	if plan.TargetClass != string(dailyTargetOrdinary) {
		return syncreview.InitPlan{}, fmt.Errorf("daily directory adoption preview changed target class to %s", plan.TargetClass)
	}
	return plan, nil
}

func dailyDirectoryAdoptionPack(opt DailyOptions) (string, error) {
	pack := strings.TrimSpace(opt.DirectoryAdoptionPack)
	if pack == "" {
		pack = defaults.DefaultPack
	}
	repoRoot := strings.TrimSpace(opt.InitializationRepoRoot)
	if repoRoot == "" {
		if strings.TrimSpace(opt.DirectoryAdoptionAction) == "" {
			return pack, nil
		}
		return "", fmt.Errorf("daily directory adoption requires the external source repository owner")
	}
	loaded, err := manifest.Load(repoRoot, pack)
	if err != nil {
		return "", fmt.Errorf("load daily directory adoption pack %s: %w", pack, err)
	}
	if err := loaded.ValidateSchema(); err != nil {
		return "", fmt.Errorf("daily directory adoption pack %s is invalid: %w", pack, err)
	}
	if !strings.EqualFold(strings.TrimSpace(loaded.Maturity), "mature") {
		return "", fmt.Errorf("daily directory adoption pack %s is not mature and cannot be selected", pack)
	}
	return loaded.Pack, nil
}

func dailyDirectoryAdoptionOptions(sourceExecutable string) syncreview.ApplyOptions {
	return syncreview.ApplyOptions{
		CreateLocalFiles: true,
		Command:          "init",
		SourceExecutable: strings.TrimSpace(sourceExecutable),
	}
}
