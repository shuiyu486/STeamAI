package workstream

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

type LaneTakeoverPackage struct {
	Ready                         bool                                    `json:"ready"`
	State                         string                                  `json:"state,omitempty"`
	Lane                          string                                  `json:"lane,omitempty"`
	Label                         string                                  `json:"label,omitempty"`
	Status                        string                                  `json:"status,omitempty"`
	Workspace                     string                                  `json:"workspace,omitempty"`
	CurrentExecutor               string                                  `json:"currentExecutor,omitempty"`
	ExecutorGeneration            int                                     `json:"executorGeneration,omitempty"`
	LastTakeoverAt                string                                  `json:"lastTakeoverAt,omitempty"`
	LastTakeoverBy                string                                  `json:"lastTakeoverBy,omitempty"`
	LastTakeoverReason            string                                  `json:"lastTakeoverReason,omitempty"`
	ApplyRequired                 bool                                    `json:"applyRequired,omitempty"`
	Blocked                       bool                                    `json:"blocked,omitempty"`
	ContinueReady                 bool                                    `json:"continueReady,omitempty"`
	ResumePath                    string                                  `json:"resumePath,omitempty"`
	CheckpointPath                string                                  `json:"checkpointPath,omitempty"`
	HandoffPath                   string                                  `json:"handoffPath,omitempty"`
	ContinueCommand               string                                  `json:"continueCommand,omitempty"`
	HandoffCommand                string                                  `json:"handoffCommand,omitempty"`
	CurrentCommand                string                                  `json:"currentCommand,omitempty"`
	MissionCommanderAction        mission.MissionCommanderAction          `json:"missionCommanderAction"`
	MissionCommanderCurrentAction *mission.MissionCommanderNextActionItem `json:"missionCommanderCurrentAction,omitempty"`
	MissionCommanderActionQueue   mission.MissionCommanderActionQueue     `json:"missionCommanderActionQueue"`
	RunbookSteps                  []string                                `json:"runbookSteps,omitempty"`
	Boundary                      []string                                `json:"boundary,omitempty"`
}

func laneTakeoverPackageFor(caseRoot string, lane Lane, action laneExecutorAction, queue mission.MissionCommanderActionQueue, applyRequired bool) *LaneTakeoverPackage {
	label := workstreamLabel(lane)
	resumeRel := relJoin(lane.LaneRoot, "prompts", "RESUME.md")
	checkpointRel := relJoin(lane.LaneRoot, "checkpoints", "latest.json")
	if laneRoot, err := laneRootPath(caseRoot, lane); err == nil {
		resumeRel = relativePath(caseRoot, relJoin(laneRoot, "prompts", "RESUME.md"))
		checkpointRel = relativePath(caseRoot, relJoin(laneRoot, "checkpoints", "latest.json"))
	}
	handoffRel := relJoin(".rekit", "handovers", lane.ID+"-latest.md")
	currentCommand := strings.TrimSpace(action.MissionCommanderAction.PrimaryCommand)
	if queue.CurrentAction != nil && strings.TrimSpace(queue.CurrentAction.Command) != "" {
		currentCommand = strings.TrimSpace(queue.CurrentAction.Command)
	}
	currentAction := cloneMissionCommanderCurrentAction(queue.CurrentAction)
	state := strings.TrimSpace(action.MissionCommanderAction.State)
	if currentAction != nil && strings.TrimSpace(currentAction.State) != "" {
		state = strings.TrimSpace(currentAction.State)
	}
	pkg := &LaneTakeoverPackage{
		Ready:                         true,
		State:                         state,
		Lane:                          lane.ID,
		Label:                         label,
		Status:                        lane.Status,
		Workspace:                     lane.Workspace,
		CurrentExecutor:               lane.CurrentExecutor,
		ExecutorGeneration:            lane.ExecutorGeneration,
		LastTakeoverAt:                lane.LastTakeoverAt,
		LastTakeoverBy:                lane.LastTakeoverBy,
		LastTakeoverReason:            lane.LastTakeoverReason,
		ApplyRequired:                 applyRequired,
		Blocked:                       action.Blocked,
		ContinueReady:                 action.Ready && !action.Blocked,
		ResumePath:                    resumeRel,
		CheckpointPath:                checkpointRel,
		HandoffPath:                   handoffRel,
		ContinueCommand:               action.ResumeCommand,
		HandoffCommand:                action.HandoffCommand,
		CurrentCommand:                currentCommand,
		MissionCommanderAction:        action.MissionCommanderAction,
		MissionCommanderCurrentAction: currentAction,
		MissionCommanderActionQueue:   queue,
	}
	pkg.RunbookSteps = laneTakeoverRunbookSteps(pkg)
	pkg.Boundary = laneTakeoverBoundary(pkg)
	return pkg
}

func cloneMissionCommanderCurrentAction(item *mission.MissionCommanderNextActionItem) *mission.MissionCommanderNextActionItem {
	if item == nil {
		return nil
	}
	clone := *item
	clone.Reasons = append([]string{}, item.Reasons...)
	clone.Boundary = append([]string{}, item.Boundary...)
	return &clone
}

func laneTakeoverRunbookSteps(pkg *LaneTakeoverPackage) []string {
	if pkg == nil {
		return nil
	}
	steps := []string{
		fmt.Sprintf("read %s before continuing lane %s", pkg.ResumePath, firstText(pkg.Label, pkg.Lane)),
	}
	if pkg.ApplyRequired {
		steps = append(steps, "review the current preview and run its explicit -Apply command before treating this package as durable")
	}
	if pkg.Blocked {
		steps = append(steps, "resolve the Mission Commander current action before running the continue command")
	} else if strings.TrimSpace(pkg.ContinueCommand) != "" {
		steps = append(steps, "run owner-bound continue command: "+strings.TrimSpace(pkg.ContinueCommand))
	}
	if strings.TrimSpace(pkg.HandoffCommand) != "" {
		steps = append(steps, "refresh durable lane handoff with "+strings.TrimSpace(pkg.HandoffCommand)+" after state changes")
	}
	return mission.UniqueStrings(steps)
}

func appendLaneTakeoverPackage(lines []string, pkg *LaneTakeoverPackage) []string {
	if pkg == nil || !pkg.Ready {
		return lines
	}
	lines = append(lines,
		"",
		"## Lane takeover package",
		"",
		fmt.Sprintf("- ready: `%t`", pkg.Ready),
		"- lane: `"+pkg.Lane+"` label=`"+pkg.Label+"` status=`"+pkg.Status+"`",
		"- workspace: `"+pkg.Workspace+"`",
		"- current executor: `"+firstText(pkg.CurrentExecutor, "unassigned")+"`",
		fmt.Sprintf("- executor generation: `%d`", pkg.ExecutorGeneration),
		"- last takeover: `"+firstText(pkg.LastTakeoverAt, "none")+"` by `"+firstText(pkg.LastTakeoverBy, "none")+"` reason=`"+firstText(pkg.LastTakeoverReason, "none")+"`",
		fmt.Sprintf("- apply required: `%t`", pkg.ApplyRequired),
		fmt.Sprintf("- blocked: `%t` continue ready: `%t`", pkg.Blocked, pkg.ContinueReady),
		"- resume: `"+pkg.ResumePath+"`",
		"- checkpoint: `"+pkg.CheckpointPath+"`",
		"- handoff: `"+pkg.HandoffPath+"`",
		"- continue command: `"+pkg.ContinueCommand+"`",
		"- handoff command: `"+pkg.HandoffCommand+"`",
		"- current command: `"+pkg.CurrentCommand+"`",
		"- commander state: `"+pkg.MissionCommanderAction.State+"`",
		"- commander primary command: `"+pkg.MissionCommanderAction.PrimaryCommand+"`",
		"- action queue: "+pkg.MissionCommanderActionQueue.Summary,
	)
	if pkg.MissionCommanderCurrentAction == nil {
		lines = append(lines, "- action queue current: none")
	} else {
		lines = append(lines, "- action queue current: "+MissionCommanderNextActionMarkdownLine(*pkg.MissionCommanderCurrentAction))
	}
	for idx, step := range pkg.RunbookSteps {
		lines = append(lines, fmt.Sprintf("- runbook step %d: %s", idx+1, step))
	}
	for _, boundary := range pkg.Boundary {
		lines = append(lines, "- boundary: "+boundary)
	}
	return lines
}

func laneTakeoverBoundary(pkg *LaneTakeoverPackage) []string {
	boundary := []string{
		"lane takeover package is read-only guidance; it does not claim a new executor",
		"continue command must keep -Executor and -ExpectedExecutorGeneration owner guard",
		"continue -Apply remains explicit; handoff/start/preview do not auto-continue",
		"no authority/confirmed writes or heavy-tool execution",
	}
	if pkg != nil && pkg.Blocked {
		boundary = append(boundary, "do not run continue while lane blockers remain open")
	}
	return mission.UniqueStrings(boundary)
}
