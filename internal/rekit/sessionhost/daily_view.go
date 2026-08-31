package sessionhost

import (
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

const (
	DailyActionCompleted                 = "completed"
	DailyActionReadyToContinue           = "ready-to-continue"
	DailyActionWaitingForCorrection      = "waiting-for-correction"
	DailyActionInputRequired             = "input-required"
	DailyActionDirectoryAdoptionRequired = "directory-adoption-required"
	DailyActionConfirmationRequired      = "confirmation-required"
	DailyActionReadyForEvidenceReview    = "ready-for-evidence-review"
	DailyActionPaused                    = "paused"
	DailyActionResumed                   = "resumed"
	DailyActionStopped                   = "stopped"
	DailyActionBlocked                   = "blocked"
	DailyActionFailed                    = "failed"
)

const (
	DailyRecoveryAutoRecovered        = "auto-recovered"
	DailyRecoveryRetryable            = "retryable"
	DailyRecoveryUserDecisionRequired = "user-decision-required"
)

type DailyUserAction struct {
	Code          string        `json:"code"`
	Message       string        `json:"message"`
	RequiresInput bool          `json:"requiresInput,omitempty"`
	Choices       []DailyChoice `json:"choices,omitempty"`
	Recovery      string        `json:"recovery,omitempty"`
	Now           string        `json:"now,omitempty"`
	Reason        string        `json:"reason,omitempty"`
	Next          string        `json:"next,omitempty"`
	RecoveryState string        `json:"recoveryState,omitempty"`
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
		Now:           "有多个任务都可能继续，系统已安全停下。",
		Reason:        "当前结果不能唯一确定要继续哪个任务。",
		Next:          "请选择一个任务；选择前不会启动 Claude 或写入 case。",
		RecoveryState: DailyRecoveryUserDecisionRequired,
	}
}

func dailyControlLaneSelectionAction(choices []DailyChoice) *DailyUserAction {
	return &DailyUserAction{
		Code:          DailyActionBlocked,
		Message:       "当前有多个任务可以控制，请先选择一个；选择前不会写入控制状态或启动 Claude。",
		RequiresInput: true,
		Choices:       append([]DailyChoice{}, choices...),
		Now:           "有多个任务可执行暂停、恢复或停止，系统已安全停下。",
		Reason:        "durable control 必须绑定唯一任务，系统不能替你选择。",
		Next:          "请选择一个任务，再复核该任务的精确控制计划。",
		RecoveryState: DailyRecoveryUserDecisionRequired,
	}
}

func dailySelectedLaneBlockedAction(choice DailyChoice) *DailyUserAction {
	return &DailyUserAction{
		Code:          DailyActionBlocked,
		Message:       "所选任务当前不可安全继续；已停在该任务，没有切换到其它任务。",
		Choices:       []DailyChoice{choice},
		Now:           "所选任务当前不可安全继续，系统已停在该任务。",
		Reason:        "最新状态不允许系统替你切换任务或继续执行。",
		Next:          "请检查这个任务的最新要求，再决定如何处理。",
		RecoveryState: DailyRecoveryUserDecisionRequired,
	}
}

func dailyHeldClaudeResultAction(disposition string) *DailyUserAction {
	action := &DailyUserAction{
		Code:          DailyActionBlocked,
		RequiresInput: false,
		RecoveryState: DailyRecoveryUserDecisionRequired,
	}
	switch strings.ToLower(strings.TrimSpace(disposition)) {
	case executioncontrol.ResultDispositionHeldWhilePaused:
		action.Message = "任务暂停期间返回的结果已安全隔离，不会自动发布或推进。"
		action.Now = "任务仍处于暂停状态；旧结果已单独保存。"
		action.Reason = "暂停后到达的结果不能进入正式输出、复核或完成流程。"
		action.Next = "如需继续，先恢复任务，再从最新状态启动新的有界执行；旧结果不会自动接收。"
	case executioncontrol.ResultDispositionLateAfterStop:
		action.Message = "任务停止后返回的结果已安全隔离，不会自动发布或推进。"
		action.Now = "任务已停止；迟到结果已单独保存。"
		action.Reason = "durable stop 已先提交，后到结果不能改变停止状态。"
		action.Next = "如需重新开展工作，请创建新的明确执行，不要复用这份迟到结果。"
	default:
		action.Message = "旧执行上下文返回的结果已安全隔离，不会自动发布或推进。"
		action.Now = "结果与当前任务控制或执行者身份不一致，系统已停下。"
		action.Reason = "旧 control generation 或旧 executor 的结果不能进入当前正式流程。"
		action.Next = "从最新状态重新开始有界执行；不要手动把隔离结果写回正式输出。"
	}
	return action
}

func dailyCurrentSyncRecoveryAction(recovery syncreview.CurrentSyncRecovery) *DailyUserAction {
	recoveryState := DailyRecoveryRetryable
	if !recovery.Recoverable {
		recoveryState = DailyRecoveryUserDecisionRequired
	}
	return &DailyUserAction{
		Code:          DailyActionBlocked,
		Message:       recovery.Now,
		Recovery:      recovery.Next,
		Now:           recovery.Now,
		Reason:        recovery.Reason,
		Next:          recovery.Next,
		RecoveryState: recoveryState,
	}
}

func dailyUserAction(result DailyResult) *DailyUserAction {
	if result.Failure != nil {
		return dailyFailureAction(*result.Failure)
	}

	state := strings.ToLower(strings.TrimSpace(result.FinalState))
	var action *DailyUserAction
	switch state {
	case "mission-complete":
		action = dailyAction(DailyActionCompleted)
	case "lane-closed":
		action = dailyAction(DailyActionReadyToContinue)
		action.Message = "当前工作线已完成并保存；系统没有把它提升为整个任务完成。"
		action.Now = "当前工作线已完成，整个任务尚未被宣布完成。"
		action.Reason = "工作线完成只表示本次有界工作已关闭，不代表整个任务已完成。"
		if result.CurrentDriverRequest != nil {
			action.Next = "按最新状态继续剩余工作线或后续审核步骤。"
		} else {
			action.Next = "刷新当前任务状态，查看其它工作线或后续审核要求。"
		}
	case "member-intake-ready", "reviewer-intake-complete":
		action = dailyAction(DailyActionReadyToContinue)
	case "reviewer-rejected-awaiting-correction":
		action = dailyAction(DailyActionWaitingForCorrection)
	case DailyActionInputRequired:
		if result.Action != nil {
			action = result.Action
		} else {
			action = dailyInputRequiredAction("")
		}
	case "attention-required":
		action = dailyAction(DailyActionBlocked)
	case DailyActionReadyForEvidenceReview:
		action = dailyAction(DailyActionReadyForEvidenceReview)
	case DailyActionPaused, DailyActionResumed, DailyActionStopped:
		action = dailyAction(state)
	case "reviewer-step", "attempt-input", "attempt-publication-recovery", "dispatch-claim", "dispatch-claim-input", "launch-accepted", "launch-failed", "launch-receipt-input", "result-turn", "running-handoff", "no-external-session":
		action = dailyAction(DailyActionReadyToContinue)
	case "not-started", "":
		if result.Blocked {
			action = dailyAction(DailyActionBlocked)
		}
	default:
		action = dailyAction(DailyActionBlocked)
	}
	if action != nil && dailyActionCanReportAutoRecovery(action) && dailyResultAutoRecovered(result) {
		action.RecoveryState = DailyRecoveryAutoRecovered
		action.Now = "系统已完成确定性恢复，当前结果可继续使用。"
		action.Reason = "恢复沿用了原有有界步骤，没有替你扩大范围或作出新的授权决定。"
	}
	return action
}

type dailyFailureGuidance struct {
	reason       string
	fallbackNext string
}

func dailyFailureAction(failure FailureDiagnosis) *DailyUserAction {
	action := dailyAction(DailyActionFailed)
	guidance := dailyFailureGuidanceFor(failure.Code)
	action.Reason = guidance.reason
	action.Next = guidance.fallbackNext
	action.Recovery = guidance.fallbackNext
	if dailyFailureIsRetryable(failure) {
		action.Message = "当前操作没有完成；修复已识别的问题后，可以从最新状态继续。"
		action.Now = "当前操作没有完成，但已保留可继续的确定状态。"
		action.RecoveryState = DailyRecoveryRetryable
		return action
	}
	action.RecoveryState = DailyRecoveryUserDecisionRequired
	return action
}

func dailyFailureGuidanceFor(code string) dailyFailureGuidance {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "claude-executable-unavailable":
		return dailyFailureGuidance{reason: "系统找不到当前项目可调用的 Claude Code。", fallbackNext: "确认本机 Claude Code 可从当前会话调用后，重试同一操作。"}
	case "claude-authentication-failed":
		return dailyFailureGuidance{reason: "Claude Code 的登录状态不可用。", fallbackNext: "恢复 Claude Code 登录后，重试同一操作。"}
	case "claude-quota-unavailable":
		return dailyFailureGuidance{reason: "Claude 服务当前受配额或计费限制。", fallbackNext: "解决配额或计费限制后，重试同一操作。"}
	case "claude-model-unavailable":
		return dailyFailureGuidance{reason: "所选 Claude 模型当前不可用。", fallbackNext: "选择可用模型后，重试同一操作。"}
	case "claude-spawn-failed":
		return dailyFailureGuidance{reason: "Claude Code 未能启动。", fallbackNext: "修复程序启动条件后，重试同一操作。"}
	case "claude-host-operation-failed":
		return dailyFailureGuidance{reason: "日常执行在主机运行阶段中断。", fallbackNext: "修复已报告的运行条件后，从最新状态重试同一操作。"}
	case "claude-timeout":
		return dailyFailureGuidance{reason: "Claude Code 在本次等待时间内没有完成。", fallbackNext: "缩小目标或增加等待时间后，重试同一操作。"}
	case "claude-nonzero-exit":
		return dailyFailureGuidance{reason: "Claude Code 异常退出。", fallbackNext: "修复退出原因后，重试同一操作。"}
	case "claude-invalid-envelope", "claude-result-envelope-failed":
		return dailyFailureGuidance{reason: "Claude Code 返回的结果格式不完整，系统没有接收它。", fallbackNext: "从最新状态重试同一操作，让系统生成一个新的有效结果。"}
	case "claude-session-id-mismatch":
		return dailyFailureGuidance{reason: "返回结果不属于当前会话，系统已拒绝接收。", fallbackNext: "从最新状态重试同一操作，让系统生成匹配当前会话的结果。"}
	case "claude-invalid-structured-output":
		return dailyFailureGuidance{reason: "返回内容不符合当前任务的结果约定，系统已拒绝接收。", fallbackNext: "从最新状态重试同一操作，让系统生成符合约定的结果。"}
	case "claude-supervision-fenced":
		return dailyFailureGuidance{reason: "上一次执行失去可靠监督，已被安全隔离。", fallbackNext: "从最新状态重试同一操作，让系统使用下一个有界尝试。"}
	case "claude-submission-failed":
		return dailyFailureGuidance{reason: "结果在保存阶段中断，部分结果可能已经保存。", fallbackNext: "刷新状态并恢复当前精确保存步骤；不要重新发起一项新任务。"}
	case "claude-intake-failed":
		return dailyFailureGuidance{reason: "已保存结果在接收阶段中断。", fallbackNext: "刷新状态并从已保存结果继续当前接收步骤。"}
	case "claude-attempt-limit-invalid":
		return dailyFailureGuidance{reason: "允许的尝试次数设置无效。", fallbackNext: "改用有效的尝试次数后，重试同一操作。"}
	case "current-driver-request-required", "current-driver-request-stale":
		return dailyFailureGuidance{reason: "本次请求已不是最新状态，系统没有继续执行。", fallbackNext: "刷新状态后，重试同一操作。"}
	case "claude-permission-denied":
		return dailyFailureGuidance{reason: "任务需要的只读访问被拒绝。", fallbackNext: "检查被拒绝的访问和任务输入；不要绕过权限。"}
	case "claude-attempt-limit-reached":
		return dailyFailureGuidance{reason: "本次操作已经用完允许的尝试次数。", fallbackNext: "查看最后一次失败原因，再决定是否开始新的有界尝试。"}
	case "claude-reported-failure":
		return dailyFailureGuidance{reason: "Claude 已明确报告当前任务无法完成。", fallbackNext: "查看报告原因，再决定如何调整目标或纠偏。"}
	case "current-driver-request-lane-required":
		return dailyFailureGuidance{reason: "有多个任务可能继续，当前请求不能唯一选定一个任务。", fallbackNext: "先选择一个任务，再从它的最新状态继续。"}
	case "current-driver-request-binding-invalid", "current-driver-request-unavailable", "current-driver-request-invalid":
		return dailyFailureGuidance{reason: "当前执行请求不完整或不可安全使用。", fallbackNext: "刷新状态并处理最新阻塞要求后，再决定是否继续。"}
	default:
		return dailyFailureGuidance{reason: "系统报告了尚未转换成普通说明的失败。", fallbackNext: "刷新状态并查看失败详情后，再决定如何处理。"}
	}
}

func dailyFailureIsRetryable(failure FailureDiagnosis) bool {
	if !dailyFailureCodeRetryable(failure.Code) || failure.Terminal || strings.EqualFold(strings.TrimSpace(failure.State), failureStateTerminal) {
		return false
	}
	if failure.AttemptsLimit > 0 && failure.AttemptsUsed >= failure.AttemptsLimit {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(failure.State))
	if state != "" && state != failureStateRecoverable && state != failureStateReplaceable {
		return false
	}
	classCount := 0
	for _, set := range []bool{failure.Terminal, failure.Recoverable, failure.Replaceable} {
		if set {
			classCount++
		}
	}
	if classCount > 1 || (!failure.Recoverable && !failure.Replaceable && state == "") {
		return false
	}
	return dailyFailureRetryBoundaryAllowed(failure)
}

func dailyFailureCodeRetryable(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "claude-executable-unavailable", "claude-authentication-failed", "claude-quota-unavailable", "claude-model-unavailable", "claude-spawn-failed", "claude-timeout", "claude-nonzero-exit", "claude-invalid-envelope", "claude-result-envelope-failed", "claude-session-id-mismatch", "claude-invalid-structured-output", "claude-supervision-fenced", "claude-submission-failed", "claude-intake-failed", "claude-attempt-limit-invalid", "current-driver-request-required", "current-driver-request-stale":
		return true
	default:
		return false
	}
}

func dailyFailureRetryBoundaryAllowed(failure FailureDiagnosis) bool {
	code := strings.ToLower(strings.TrimSpace(failure.Code))
	boundary := strings.ToLower(strings.TrimSpace(failure.MutationBoundary))
	if boundary == "" {
		return false
	}
	if boundary == "none" {
		return !failure.MutationApplied
	}
	switch boundary {
	case "session-launch-recorded":
		switch code {
		case "claude-authentication-failed", "claude-quota-unavailable", "claude-model-unavailable", "claude-spawn-failed", "claude-timeout", "claude-nonzero-exit", "claude-invalid-envelope", "claude-result-envelope-failed", "claude-session-id-mismatch", "claude-invalid-structured-output", "claude-supervision-fenced":
			return true
		}
	case "durable-launch-failure-recorded":
		return code == "claude-spawn-failed"
	case "result-artifact-publication-may-have-committed":
		return code == "claude-submission-failed" && failure.MutationApplied
	case "durable-runtime-step-may-have-committed":
		return code == "claude-intake-failed" && failure.MutationApplied
	}
	return false
}

func dailyActionCanReportAutoRecovery(action *DailyUserAction) bool {
	if action == nil || action.RequiresInput || action.RecoveryState == DailyRecoveryUserDecisionRequired {
		return false
	}
	return action.Code == DailyActionCompleted || action.Code == DailyActionReadyToContinue
}

func dailyResultAutoRecovered(result DailyResult) bool {
	if result.Failure != nil {
		return false
	}
	for _, hostRun := range result.HostRuns {
		for _, session := range hostRun.Sessions {
			if session.Failure == nil && session.Recovered && strings.EqualFold(strings.TrimSpace(session.Outcome), "returned-recovered") {
				return true
			}
		}
	}
	return false
}

func dailyAction(code string) *DailyUserAction {
	switch code {
	case DailyActionCompleted:
		return &DailyUserAction{
			Code: code, Message: "当前任务已完成，结果和完成状态都已保存。",
			Now: "当前任务已完成。", Reason: "结果和完成状态都已保存。", Next: "无需操作；有新目标时再开始。",
		}
	case DailyActionReadyToContinue:
		return &DailyUserAction{
			Code: code, Message: "当前结果已保存，可以从最新状态继续下一阶段。",
			Now: "当前阶段已完成，可以继续。", Reason: "最新结果已经保存。", Next: "从最新状态继续下一阶段。",
		}
	case DailyActionInputRequired:
		return dailyInputRequiredAction("")
	case DailyActionWaitingForCorrection:
		return &DailyUserAction{
			Code:          code,
			Message:       "复核没有通过，需要你补充一句具体纠偏意见后再继续。",
			RequiresInput: true,
			Choices: []DailyChoice{
				{ID: "provide-correction", Label: "补充纠偏意见"},
				{ID: "stop", Label: "先停在这里"},
			},
			Now:           "复核没有通过，任务已停在纠偏入口。",
			Reason:        "Reviewer 要求补充具体纠偏意见。",
			Next:          "补充一句纠偏意见，或选择先停在这里。",
			RecoveryState: DailyRecoveryUserDecisionRequired,
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
			Now:           "当前目录尚未接入 STeamAI，系统已保持零写入。",
			Reason:        "接入可能新增受管文件，不能替你决定或覆盖现有内容。",
			Next:          "选择安全接入预览，或取消。",
			RecoveryState: DailyRecoveryUserDecisionRequired,
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
			Now:           "精确计划已准备好，正在等待确认。",
			Reason:        "下一步会产生有界写入，系统不能替你确认。",
			Next:          "确认精确计划，或取消。",
			RecoveryState: DailyRecoveryUserDecisionRequired,
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
			Now:           "新的执行证据正在等待复核。",
			Reason:        "证据本身不能授予 authority 或 confirmed。",
			Next:          "复核证据，或选择稍后处理。",
			RecoveryState: DailyRecoveryUserDecisionRequired,
		}
	case DailyActionPaused:
		return &DailyUserAction{
			Code: code, Message: "当前任务已暂停；系统不会启动新会话、发布新结果或推进任务。",
			Now: "当前任务已持久化暂停。", Reason: "暂停控制已成为 durable head。", Next: "需要继续时，先复核一份独立的恢复计划。",
		}
	case DailyActionResumed:
		return &DailyUserAction{
			Code: code, Message: "当前任务已恢复为可运行状态，但系统没有自动继续或接收暂停期间的结果。",
			Now: "当前任务已持久化恢复。", Reason: "恢复只改变后续工作的控制状态。", Next: "刷新状态后，再明确决定是否继续下一步。",
		}
	case DailyActionStopped:
		return &DailyUserAction{
			Code: code, Message: "当前任务已持久化停止；进程处理结果不会改变停止状态。",
			Now: "当前任务已停止。", Reason: "停止控制已先于可选进程处置成为 durable head。", Next: "保留现有诊断记录；停止状态不能隐式恢复。",
		}
	case DailyActionBlocked:
		return &DailyUserAction{
			Code: code, Message: "当前操作被阻塞；请先处理最新状态中的要求。",
			Now: "当前操作被阻塞，系统已安全停下。", Reason: "最新状态没有唯一可安全执行的恢复。", Next: "查看最新要求并决定如何处理。", RecoveryState: DailyRecoveryUserDecisionRequired,
		}
	case DailyActionFailed:
		return &DailyUserAction{
			Code: code, Message: "当前操作失败；请先处理机器结果中的 typed failure，再重试同一操作。",
			Now: "当前操作没有完成。", Reason: "系统报告了明确的失败。", Next: "查看失败原因后决定如何处理。", RecoveryState: DailyRecoveryUserDecisionRequired,
		}
	default:
		return &DailyUserAction{
			Code: DailyActionBlocked, Message: "当前状态没有可安全自动执行的下一步；请先刷新状态并检查 typed blocker。",
			Now: "当前状态无法安全识别。", Reason: "系统没有匹配到已知动作。", Next: "刷新状态并检查最新阻塞要求。", RecoveryState: DailyRecoveryUserDecisionRequired,
		}
	}
}
