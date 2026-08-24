package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// publicInteraction is the complete default user-facing publication. It does
// not carry durable identities, commands, paths, hashes, or receipts.
type publicInteraction struct {
	Now    string
	Reason string
	Next   string
}

type publicStatusInteractionInput struct {
	Mode                  string                       `json:"mode"`
	Summary               string                       `json:"summary"`
	CaseMission           *publicStatusMissionInput    `json:"caseMission"`
	Onboarding            *publicStatusOnboardingInput `json:"onboarding"`
	MissionControlRunbook *publicStatusRunbookInput    `json:"missionControlRunbook"`
}

type publicStatusMissionInput struct {
	Summary string `json:"summary"`
}

type publicStatusOnboardingInput struct {
	State string `json:"state"`
}

type publicStatusRunbookInput struct {
	CurrentDriverRequest *publicStatusDriverInput `json:"currentDriverRequest"`
}

type publicStatusDriverInput struct {
	State             string `json:"state"`
	CommandExecutable bool   `json:"commandExecutable"`
	Blocked           bool   `json:"blocked"`
}

type publicContinueInteractionInput struct {
	Blocked bool `json:"blocked"`
}

func writePublicInteraction(out io.Writer, command string, raw json.RawMessage) error {
	var interaction publicInteraction
	switch command {
	case "status":
		var input publicStatusInteractionInput
		if err := json.Unmarshal(raw, &input); err != nil {
			return fmt.Errorf("decode public status interaction: %w", err)
		}
		interaction = reducePublicStatusInteraction(input)
	case "continue":
		var input publicContinueInteractionInput
		if err := json.Unmarshal(raw, &input); err != nil {
			return fmt.Errorf("decode public continue interaction: %w", err)
		}
		interaction = reducePublicContinueInteraction(input)
	default:
		return fmt.Errorf("unsupported public command: %s", command)
	}
	return publishPublicInteraction(out, interaction)
}

func reducePublicStatusInteraction(input publicStatusInteractionInput) publicInteraction {
	summary := ""
	if input.CaseMission != nil {
		summary = strings.TrimSpace(input.CaseMission.Summary)
	}
	if summary == "" {
		summary = strings.TrimSpace(input.Summary)
	}
	if summary == "" {
		summary = "状态已读取"
	}

	current := (*publicStatusDriverInput)(nil)
	if input.MissionControlRunbook != nil {
		current = input.MissionControlRunbook.CurrentDriverRequest
	}
	onboardingState := ""
	if input.Onboarding != nil {
		onboardingState = strings.TrimSpace(input.Onboarding.State)
	}
	if current == nil && (strings.TrimSpace(input.Mode) == "case-onboarding-required" || onboardingState == "absent") {
		return publicInteraction{
			Now:    summary,
			Reason: "项目尚未完成首次接入",
			Next:   "告诉主 Agent 你的目标并选择一个 pack；status 不会自动写入项目",
		}
	}

	state := ""
	executable := false
	blocked := false
	if current != nil {
		state = current.State
		executable = current.CommandExecutable
		blocked = current.Blocked
	}
	reason, next := publicStatusGuidance(state, current != nil, executable, blocked)
	return publicInteraction{Now: summary, Reason: reason, Next: next}
}

func reducePublicContinueInteraction(input publicContinueInteractionInput) publicInteraction {
	if input.Blocked {
		return publicInteraction{
			Now:    "继续预览已生成，但当前工作线暂时不能继续",
			Reason: "当前 action 需要先处理 blocker、纠偏或确认",
			Next:   "先按主 Agent 展示的安全选择处理；本次没有写入或启动任何执行",
		}
	}
	return publicInteraction{
		Now:    "已生成 fresh continue 预览",
		Reason: "预览绑定当前工作线与完整 mutation snapshot",
		Next:   "先 review 预览；确认后只执行返回的 exact Apply action（本命令不会自动 Apply）",
	}
}

func publishPublicInteraction(out io.Writer, interaction publicInteraction) error {
	_, err := fmt.Fprintf(
		out,
		"现在：%s\n原因：%s\n下一步：%s\n",
		interaction.Now,
		interaction.Reason,
		interaction.Next,
	)
	return err
}

func publicStatusGuidance(state string, hasCurrent, executable, blocked bool) (string, string) {
	if !hasCurrent {
		return "当前没有可执行的 typed action", "告诉主 Agent 你的目标，或使用 --diagnostics 查看 typed choices"
	}
	state = strings.ToLower(strings.TrimSpace(state))
	if blocked || strings.Contains(state, "blocked") || strings.Contains(state, "waiting") || strings.Contains(state, "confirmation") {
		return "当前 action 需要先处理 blocker、纠偏或确认", "按主 Agent 展示的选择处理；不要手工拼内部参数"
	}
	if strings.Contains(state, "completed") || strings.Contains(state, "mission-complete") {
		return "当前阶段已完成", "告诉主 Agent 下一个目标，或查看 typed diagnostics"
	}
	if state == "refresh-required" || strings.Contains(state, "refresh-required") || strings.Contains(state, "stale") {
		return "当前 action 需要先刷新 durable 状态", "让主 Agent 刷新 status；不要执行旧 action"
	}
	if !executable || state == "" {
		return "当前 typed action 尚未达到可执行状态", "让主 Agent 查看 typed diagnostics 并处理当前 review 或 blocker"
	}
	if strings.HasPrefix(state, "ready-") || strings.HasPrefix(state, "needs-") || strings.HasSuffix(state, "-ready") || strings.Contains(state, "preview-available") || strings.Contains(state, "review-required") || state == "case-mission-onboarding" {
		return "fresh typed action 已就绪", "让主 Agent 按该 exact action 推进；如需机器字段请加 --diagnostics"
	}
	return "当前 action 状态无法安全归类", "让主 Agent 刷新 status 并查看 typed diagnostics；不要执行旧 action"
}
