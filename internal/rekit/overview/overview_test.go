package overview

import (
	"bytes"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestOverviewMissionCommanderActionQueuePrintsIdentity(t *testing.T) {
	current := mission.MissionCommanderNextActionItem{
		Lane:           "main",
		Label:          "Main",
		GateEventID:    "evt-adapter",
		ActionID:       "evt-adapter:adapter-report-record",
		State:          "ready-to-record-evidence",
		Command:        "/rekit gate -Apply -GateEventId evt-adapter",
		Source:         "adapterReportValidation.missionCommanderAction",
		RequiresReview: true,
	}
	var out strings.Builder
	var buf bytes.Buffer
	writeMissionCommanderActionQueue(&buf, mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{current}))
	out.WriteString(buf.String())
	text := out.String()
	for _, want := range []string{"- current: Main state=ready-to-record-evidence source=adapterReportValidation.missionCommanderAction", "command=`/rekit gate -Apply -GateEventId evt-adapter` lane=main label=Main", "gateEventId=evt-adapter", "actionId=evt-adapter:adapter-report-record", "current run loop：currentRunLoopStep=preview-current steps=3", "run loop step：order=2 step=preview-current actor=main-agent state=ready-to-record-evidence source=adapterReportValidation.missionCommanderAction", "run loop boundary：step=preview-current boundary=review preview output and returned expected hashes before any apply follow-up"} {
		if !strings.Contains(text, want) {
			t.Fatalf("overview action queue missing %q:\n%s", want, text)
		}
	}
}

func TestOverviewNextStepsFollowActionQueueCurrentAction(t *testing.T) {
	current := mission.MissionCommanderNextActionItem{
		State:          "waiting-for-reviewer-result",
		Command:        "dispatch read-only reviewer for shard-01",
		Source:         "reviewerDispatchIntakeHandoffs",
		Blocked:        true,
		RequiresReview: true,
	}
	steps := overviewNextSteps(MissionBrief{NextAgentActions: []string{"/rekit continue main", "/rekit handoff main"}}, nil, mission.MissionCommanderActionQueue{
		Counts:        mission.MissionCommanderActionQueueCounts{Total: 1, Blocked: 1, RequiresReview: 1},
		CurrentAction: &current,
	})
	if !overviewTestContains(steps, "follow Mission Commander current action: dispatch read-only reviewer for shard-01") || overviewTestContains(steps, "/rekit continue main") || !overviewTestContains(steps, "/rekit handoff main") {
		t.Fatalf("overview next steps did not follow current action or suppress generic continue: %+v", steps)
	}
}

func TestOverviewNextStepsKeepReadyContinue(t *testing.T) {
	current := mission.MissionCommanderNextActionItem{
		State:   "ready-to-continue",
		Command: "/rekit continue main",
		Source:  "missionCommanderActions",
	}
	steps := overviewNextSteps(MissionBrief{NextAgentActions: []string{"/rekit continue main"}}, nil, mission.MissionCommanderActionQueue{
		Counts:        mission.MissionCommanderActionQueueCounts{Total: 1, Unblocked: 1},
		CurrentAction: &current,
	})
	if !overviewTestContains(steps, "follow Mission Commander current action: /rekit continue main") || !overviewTestContains(steps, "/rekit continue main") {
		t.Fatalf("overview next steps should keep ordinary continue when queue is ready: %+v", steps)
	}
}

func overviewTestContains(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}
