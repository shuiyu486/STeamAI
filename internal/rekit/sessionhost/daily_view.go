package sessionhost

import "strings"

const (
	DailyActionCompleted                 = "completed"
	DailyActionReadyToContinue           = "ready-to-continue"
	DailyActionWaitingForCorrection      = "waiting-for-correction"
	DailyActionDirectoryAdoptionRequired = "directory-adoption-required"
	DailyActionConfirmationRequired      = "confirmation-required"
	DailyActionReadyForEvidenceReview    = "ready-for-evidence-review"
	DailyActionBlocked                   = "blocked"
	DailyActionFailed                    = "failed"
)

type DailyUserAction struct {
	Code          string        `json:"code"`
	Message       string        `json:"message"`
	RequiresInput bool          `json:"requiresInput,omitempty"`
	Choices       []DailyChoice `json:"choices,omitempty"`
}

type DailyChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func dailyLaneSelectionAction(choices []DailyChoice) *DailyUserAction {
	return &DailyUserAction{
		Code:          DailyActionBlocked,
		Message:       "当前有多个可继续任务，请先选择一个；选择前不会启动 Claude 或写入 case。",
		RequiresInput: true,
		Choices:       append([]DailyChoice{}, choices...),
	}
}

func dailySelectedLaneBlockedAction(choice DailyChoice) *DailyUserAction {
	return &DailyUserAction{
		Code:    DailyActionBlocked,
		Message: "所选任务当前不可安全继续；已停在该任务，没有切换到其它任务。",
		Choices: []DailyChoice{choice},
	}
}

func dailyUserAction(result DailyResult) *DailyUserAction {
	if result.Failure != nil {
		action := dailyAction(DailyActionFailed)
		if result.Failure.Recoverable || result.Failure.Replaceable {
			action.Message = "当前操作没有完成；修复已识别的问题后，可以从最新状态继续。"
		}
		return action
	}

	state := strings.ToLower(strings.TrimSpace(result.FinalState))
	switch state {
	case "mission-complete", "lane-closed":
		return dailyAction(DailyActionCompleted)
	case "member-intake-ready", "reviewer-intake-complete":
		return dailyAction(DailyActionReadyToContinue)
	case "reviewer-rejected-awaiting-correction":
		return dailyAction(DailyActionWaitingForCorrection)
	case "attention-required":
		return dailyAction(DailyActionBlocked)
	case DailyActionReadyForEvidenceReview:
		return dailyAction(DailyActionReadyForEvidenceReview)
	case "reviewer-step", "attempt-input", "attempt-publication-recovery", "dispatch-claim", "dispatch-claim-input", "launch-accepted", "launch-failed", "launch-receipt-input", "result-turn", "running-handoff", "no-external-session":
		return dailyAction(DailyActionReadyToContinue)
	case "not-started", "":
		if result.Blocked {
			return dailyAction(DailyActionBlocked)
		}
		return nil
	default:
		return dailyAction(DailyActionBlocked)
	}
}

func dailyAction(code string) *DailyUserAction {
	switch code {
	case DailyActionCompleted:
		return &DailyUserAction{Code: code, Message: "当前任务已完成，结果和完成状态都已保存。"}
	case DailyActionReadyToContinue:
		return &DailyUserAction{Code: code, Message: "当前结果已保存，可以从最新状态继续下一阶段。"}
	case DailyActionWaitingForCorrection:
		return &DailyUserAction{
			Code:          code,
			Message:       "复核没有通过，需要你补充一句具体纠偏意见后再继续。",
			RequiresInput: true,
			Choices: []DailyChoice{
				{ID: "provide-correction", Label: "补充纠偏意见"},
				{ID: "stop", Label: "先停在这里"},
			},
		}
	case DailyActionDirectoryAdoptionRequired:
		return &DailyUserAction{
			Code:          code,
			Message:       "这个普通目录需要先做安全接入预览；确认前不会写入或覆盖原有内容。",
			RequiresInput: true,
			Choices: []DailyChoice{
				{ID: "initialize-in-place", Label: "在当前目录安全接入"},
				{ID: "cancel", Label: "取消"},
			},
		}
	case DailyActionConfirmationRequired:
		return &DailyUserAction{
			Code:          code,
			Message:       "下一步会产生写入，需要先确认精确计划和范围。",
			RequiresInput: true,
			Choices: []DailyChoice{
				{ID: "confirm-exact-plan", Label: "确认精确计划"},
				{ID: "cancel", Label: "取消"},
			},
		}
	case DailyActionReadyForEvidenceReview:
		return &DailyUserAction{
			Code:          code,
			Message:       "有新的执行证据可供复核；复核前不会推断 authority 或 confirmed。",
			RequiresInput: true,
			Choices: []DailyChoice{
				{ID: "review-evidence", Label: "复核证据"},
				{ID: "defer", Label: "稍后处理"},
			},
		}
	case DailyActionBlocked:
		return &DailyUserAction{Code: code, Message: "当前操作被阻塞；请先处理最新状态中的要求。"}
	case DailyActionFailed:
		return &DailyUserAction{Code: code, Message: "当前操作失败；请先处理机器结果中的 typed failure，再重试同一操作。"}
	default:
		return &DailyUserAction{Code: DailyActionBlocked, Message: "当前状态没有可安全自动执行的下一步；请先刷新状态并检查 typed blocker。"}
	}
}
