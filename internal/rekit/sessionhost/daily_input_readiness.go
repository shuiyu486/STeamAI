package sessionhost

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func dailyInputRequested(input DailyInputRequest) bool {
	return strings.TrimSpace(input.Mode) != "" || strings.TrimSpace(input.ArtifactPath) != "" || strings.TrimSpace(input.Scope) != ""
}

func currentDailyRequestRequiresMemberInput(status publicStatus, lane string) bool {
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.Scope != "case" {
		return false
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	return request != nil && !request.Blocked &&
		strings.TrimSpace(request.Lane) == strings.TrimSpace(lane) &&
		mission.MissionCommanderDriverRequestOwnsMemberContinuation(*request)
}

func prepareDailyInputReadiness(caseRoot, pack, lane string, input DailyInputRequest, result *DailyResult) (bool, error) {
	mode := strings.TrimSpace(input.Mode)
	artifactPath := strings.TrimSpace(input.ArtifactPath)
	scope := strings.TrimSpace(input.Scope)
	if pack != defaults.DefaultPack {
		if mode != "" || artifactPath != "" || scope != "" {
			return false, fmt.Errorf("typed daily input readiness is supported only by binary-re")
		}
		return true, nil
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		return false, err
	}
	owner, found := mission.LookupBoardLane(board.Lanes, lane, false)
	if !found || strings.TrimSpace(owner.CurrentExecutor) == "" || owner.ExecutorGeneration < 1 {
		if mode == "" {
			result.FinalState = "input-required"
			result.Blocked = true
			result.InputReadiness = &DailyInputReadiness{State: "input-required"}
			result.Action = dailyInputRequiredAction("")
			return false, nil
		}
		return false, fmt.Errorf("typed daily input requires a current durable executor generation")
	}
	current, _, _, err := memberexecution.ReadTaskBindingForOwner(
		caseRoot,
		lane,
		owner.ExecutorGeneration,
	)
	if err != nil {
		return false, err
	}
	if current != nil {
		if mode == "" {
			if memberexecution.IsTaskInputBinding(*current) {
				if err := memberexecution.ValidateTaskInputBinding(caseRoot, *current); err != nil {
					return false, err
				}
				readiness := &DailyInputReadiness{State: "ready", Mode: current.Kind}
				if current.Kind == DailyInputArtifactAnalysis {
					readiness.ArtifactPath = current.Values["artifact-path"]
				} else {
					readiness.Scope = current.Values["workspace-scope"]
				}
				result.InputReadiness = readiness
			}
			return true, nil
		}
		if !memberexecution.IsTaskInputBinding(*current) {
			return false, fmt.Errorf("typed daily input cannot replace the current specialized member binding")
		}
	}
	if mode == "" {
		result.FinalState = "input-required"
		result.Blocked = true
		result.InputReadiness = &DailyInputReadiness{State: "input-required"}
		result.Action = dailyInputRequiredAction("")
		return false, nil
	}
	if artifactPath != "" && mode != DailyInputArtifactAnalysis {
		return false, fmt.Errorf("daily artifact path requires artifact-analysis input mode")
	}
	if scope != "" && mode != DailyInputWorkspaceInventory {
		return false, fmt.Errorf("daily workspace scope requires workspace-inventory input mode")
	}
	var binding memberexecution.TaskBinding
	switch mode {
	case DailyInputArtifactAnalysis:
		if artifactPath == "" {
			result.FinalState = "input-required"
			result.Blocked = true
			result.InputReadiness = &DailyInputReadiness{State: "input-required", Mode: mode}
			result.Action = dailyInputRequiredAction(mode)
			return false, nil
		}
		binding, err = memberexecution.ArtifactAnalysisTaskBinding(caseRoot, artifactPath)
		if err == nil {
			result.InputReadiness = &DailyInputReadiness{State: "ready", Mode: mode, ArtifactPath: artifactPath}
		}
	case DailyInputWorkspaceInventory:
		if scope == "" {
			scope = "."
		}
		binding, err = memberexecution.WorkspaceInventoryTaskBinding(caseRoot, scope)
		if err == nil {
			result.InputReadiness = &DailyInputReadiness{State: "ready", Mode: mode, Scope: scope}
		}
	default:
		return false, fmt.Errorf("daily input mode must be artifact-analysis or workspace-inventory")
	}
	if err != nil {
		return false, fmt.Errorf("prepare typed daily input: %w", err)
	}
	if current == nil {
		if _, _, err := memberexecution.WriteTaskBindingForOwner(
			caseRoot,
			lane,
			strings.TrimSpace(owner.CurrentExecutor),
			owner.ExecutorGeneration,
			binding,
		); err != nil {
			return false, fmt.Errorf("bind typed daily input: %w", err)
		}
	} else if current.Kind != binding.Kind || !equalStringMap(current.Values, binding.Values) {
		return false, fmt.Errorf("typed daily input differs from the current owner-generation binding")
	}
	return true, nil
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func dailyInputRequiredAction(mode string) *DailyUserAction {
	action := &DailyUserAction{
		Code:          "input-required",
		RequiresInput: true,
		Now:           "任务尚未启动；系统正在等待明确的 typed 输入。",
		Reason:        "binary-re 不能从自然语言或目录扫描猜测要分析的 artifact。",
		RecoveryState: DailyRecoveryUserDecisionRequired,
	}
	if mode == DailyInputArtifactAnalysis {
		action.Message = "分析具体 artifact 前，需要提供一个 case-local artifact、alias 或已绑定 sidecar。"
		action.Next = "提供 case-local artifact 路径；如果只想检查目录内容，请改选 workspace-inventory。"
		action.Choices = []DailyChoice{{ID: DailyInputWorkspaceInventory, Label: "检查工作区"}, {ID: "provide-artifact", Label: "提供 artifact"}}
		return action
	}
	action.Message = "请选择分析具体 artifact，或对明确工作区做 bounded inventory；选择前不会启动 member 或 Reviewer。"
	action.Next = "选择 artifact-analysis 并提供目标，或选择 workspace-inventory。"
	action.Choices = []DailyChoice{{ID: DailyInputArtifactAnalysis, Label: "分析 artifact"}, {ID: DailyInputWorkspaceInventory, Label: "检查工作区"}}
	return action
}
