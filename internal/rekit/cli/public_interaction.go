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
	State                 string                       `json:"state"`
	Reason                string                       `json:"reason"`
	DetailsRequired       bool                         `json:"detailsRequired"`
	Summary               string                       `json:"summary"`
	CaseMission           *publicStatusMissionInput    `json:"caseMission"`
	Onboarding            *publicStatusOnboardingInput `json:"onboarding"`
	MissionControlRunbook *publicStatusRunbookInput    `json:"missionControlRunbook"`
}

type publicStatusMissionInput struct {
	Summary           string                        `json:"summary"`
	MissionCompletion *publicMissionCompletionInput `json:"missionCompletion"`
}

type publicMissionCompletionInput struct {
	State                 string `json:"state"`
	OperationallyComplete bool   `json:"operationallyComplete"`
}

type publicStatusOnboardingInput struct {
	State        string                        `json:"state"`
	SelectedPack string                        `json:"selectedPack"`
	PackChoices  []publicStatusPackChoiceInput `json:"packChoices"`
}

type publicStatusPackChoiceInput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Maturity    string `json:"maturity"`
	Recommended bool   `json:"recommended"`
	Selectable  bool   `json:"selectable"`
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

	if input.DetailsRequired || strings.EqualFold(strings.TrimSpace(input.State), "details-required") {
		return publicInteraction{
			Now:    "完整情况暂时无法在简要状态中安全显示",
			Reason: "当前下一步无法在简要信息中完整验证",
			Next:   "请主 Agent 查看同一项目的完整状态后给出可选步骤；此前不要猜测或执行下一步",
		}
	}
	current := (*publicStatusDriverInput)(nil)
	if input.MissionControlRunbook != nil {
		current = input.MissionControlRunbook.CurrentDriverRequest
	}
	onboardingState := ""
	if input.Onboarding != nil {
		onboardingState = strings.TrimSpace(input.Onboarding.State)
	}
	mode := strings.TrimSpace(input.Mode)
	if current == nil && mode == "case-onboarding-conflict" {
		return publicInteraction{
			Now:    summary,
			Reason: "当前任务信息缺失，但项目中已经有任务面板",
			Next:   "请主 Agent 查看完整状态并核对项目；此前不要选择 pack、初始化任务面板或继续执行",
		}
	}
	if current == nil && (mode == "case-onboarding-required" || onboardingState == "absent") {
		selectedPack := ""
		if input.Onboarding != nil {
			selectedPack = strings.TrimSpace(input.Onboarding.SelectedPack)
		}
		next := "告诉主 Agent 你的目标并选择一个 pack；status 不会自动写入项目"
		reason := "项目尚未完成首次接入"
		if choices := publicOnboardingPackChoices(input.Onboarding); choices != "" {
			next = "告诉主 Agent 你的目标并选择一个 pack（" + choices + "）；status 不会自动写入项目"
		} else if selectedPack != "" {
			reason = "项目已接入，但还缺少当前任务目标"
			next = "告诉主 Agent 这个任务的目标；项目已固定使用 " + selectedPack + " pack，status 不会自动写入项目"
		}
		return publicInteraction{
			Now:    summary,
			Reason: reason,
			Next:   next,
		}
	}
	if publicMissionCompleted(input.CaseMission) {
		return publicInteraction{
			Now:    summary,
			Reason: "当前任务已完成",
			Next:   "如需补充或纠正，请告诉主 Agent 具体内容；新的独立目标目前不能从这个已完成任务直接开始",
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

func publicMissionCompleted(mission *publicStatusMissionInput) bool {
	if mission == nil || mission.MissionCompletion == nil {
		return false
	}
	return mission.MissionCompletion.OperationallyComplete &&
		strings.EqualFold(strings.TrimSpace(mission.MissionCompletion.State), "mission-complete")
}

func publicOnboardingPackChoices(onboarding *publicStatusOnboardingInput) string {
	if onboarding == nil {
		return ""
	}
	choices := []string{}
	for _, choice := range onboarding.PackChoices {
		name := strings.TrimSpace(choice.Name)
		if name == "" {
			name = strings.TrimSpace(choice.ID)
		}
		if name == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(choice.Maturity)) {
		case "mature":
			name += "，可直接使用"
		case "skeleton":
			name += "，功能骨架，不可直接选择"
		default:
			if !choice.Selectable {
				name += "，不可直接选择"
			}
		}
		if choice.Recommended {
			name += "，推荐"
		}
		choices = append(choices, name)
	}
	return strings.Join(choices, "；")
}

func reducePublicContinueInteraction(input publicContinueInteractionInput) publicInteraction {
	if input.Blocked {
		return publicInteraction{
			Now:    "已完成继续前的检查，但当前工作线暂时不能继续",
			Reason: "需要先处理当前阻塞、纠偏或确认",
			Next:   "先按主 Agent 展示的安全选择处理；本次没有更改项目或启动执行",
		}
	}
	return publicInteraction{
		Now:    "已完成继续前的检查",
		Reason: "检查结果绑定当前工作线与本次计划",
		Next:   "先查看计划；确认后仅按主 Agent 刚刚展示的步骤继续（本命令不会自动更改项目）",
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
		return "当前没有可以直接执行的下一步", "告诉主 Agent 你的目标，或请主 Agent 查看完整状态后给出可选步骤"
	}
	state = strings.ToLower(strings.TrimSpace(state))
	if blocked || strings.Contains(state, "blocked") || strings.Contains(state, "waiting") || strings.Contains(state, "confirmation") {
		return "需要先处理当前阻塞、纠偏或确认", "按主 Agent 展示的选择处理；不要自行拼接内部参数"
	}
	if strings.Contains(state, "completed") || strings.Contains(state, "mission-complete") {
		return "当前阶段已完成", "如需补充或纠正，请告诉主 Agent 具体内容"
	}
	if state == "refresh-required" || strings.Contains(state, "refresh-required") || strings.Contains(state, "stale") {
		return "当前步骤需要先刷新项目状态", "请主 Agent 重新读取状态；不要执行之前的步骤"
	}
	if state == "case-board-missing" && executable {
		return "当前项目需要先建立任务面板", "让主 Agent 使用本次状态中唯一的初始化步骤；查看状态本身不会写入项目"
	}
	if !executable || state == "" {
		return "当前步骤还不能执行", "请主 Agent 查看完整情况并处理当前审核、阻塞或确认"
	}
	if strings.HasPrefix(state, "ready-") || strings.HasPrefix(state, "needs-") || strings.HasSuffix(state, "-ready") || strings.Contains(state, "preview-available") || strings.Contains(state, "review-required") || state == "case-mission-onboarding" {
		return "可以继续推进", "让主 Agent 按本次状态中给出的步骤推进；需要更多信息时由主 Agent查看完整状态"
	}
	return "当前步骤无法安全归类", "请主 Agent 重新读取完整状态；不要执行之前的步骤"
}
