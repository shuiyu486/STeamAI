package workstream

import (
	"encoding/json"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestLaneCheckpointJSONContract(t *testing.T) {
	checkpoint := laneCheckpoint{
		SchemaVersion:      1,
		Lane:               "main",
		Status:             "open",
		Workspace:          "workspace/main/main",
		CurrentExecutor:    "session-main",
		ExecutorGeneration: 2,
		LastTakeoverAt:     "2026-01-01T00:00:00Z",
		LastTakeoverBy:     "main-agent",
		LastTakeoverReason: "replace stale session",
		AutonomyProfile: autonomy.Summary{
			Mode:  autonomy.ModeManualGate,
			Ready: true,
		},
		MissionBrief: mission.Brief{
			Summary:          "openLanes=1 ready=0 blocked=1 pendingGates=0 authorizedGates=1 openDecisions=1 interventions=0",
			BlockedLanes:     []string{"main (open-decision)"},
			AuthorizedGates:  []string{"authorized debug | auth=preauthorized"},
			OpenDecisions:    []string{"candidate: review candidate"},
			NextAgentActions: []string{"review open candidate/decision item(s) with evidence and authority boundary"},
		},
		ExecutorAction: laneExecutorAction{
			Blocked:              true,
			BlockerReasons:       []string{"open-decision"},
			OpenDecisions:        1,
			OpenDecisionRequired: true,
			ResumeCommand:        "/rekit continue main",
			HandoffCommand:       "/rekit handoff main",
		},
		PendingGates:    []string{},
		AuthorizedGates: []string{"authorized debug | auth=preauthorized"},
		ExecutionEvidenceReview: []ExecutionEvidenceReviewItem{{
			GateEventID:    "evt-authorized",
			Subject:        "execution evidence for authorized debug",
			Status:         "succeeded",
			Action:         "debug",
			OutputRefs:     []string{"workspace/main/debug/result.json"},
			EvidenceRefs:   []string{"workspace/main/debug/result.json"},
			ReviewCommand:  "review outputRefs/evidenceRefs for gateEventId evt-authorized",
			HandoffCommand: "/rekit handoff main",
			Boundary:       []string{"observation evidence is already recorded; do not replay heavy tool", "review outputRefs/evidenceRefs before any authority/confirmed outcome"},
		}},
		OpenInterventions: []InterventionSummary{},
		Inbox:             2,
		Tasks:             3,
		UpdatedAt:         "2026-01-01T00:00:00Z",
		Resume:            ".rekit/lanes/main/prompts/RESUME.md",
	}
	encoded, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		SchemaVersion      int    `json:"schemaVersion"`
		CurrentExecutor    string `json:"currentExecutor"`
		ExecutorGeneration int    `json:"executorGeneration"`
		LastTakeoverAt     string `json:"lastTakeoverAt"`
		LastTakeoverBy     string `json:"lastTakeoverBy"`
		LastTakeoverReason string `json:"lastTakeoverReason"`
		MissionBrief       struct {
			AuthorizedGates []string `json:"authorizedGates"`
			OpenDecisions   []string `json:"openDecisions"`
		} `json:"missionBrief"`
		ExecutorAction struct {
			Blocked              bool     `json:"blocked"`
			BlockerReasons       []string `json:"blockerReasons"`
			PendingGates         int      `json:"pendingGates"`
			OpenInterventions    int      `json:"openInterventions"`
			OpenDecisions        int      `json:"openDecisions"`
			OpenDecisionRequired bool     `json:"openDecisionRequired"`
			ResumeCommand        string   `json:"resumeCommand"`
		} `json:"executorAction"`
		AuthorizedGates         []string                      `json:"authorizedGates"`
		ExecutionEvidenceReview []ExecutionEvidenceReviewItem `json:"executionEvidenceReview"`
		Resume                  string                        `json:"resume"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("lane checkpoint json did not decode: %v\n%s", err, string(encoded))
	}
	if decoded.SchemaVersion != 1 || decoded.CurrentExecutor != "session-main" || decoded.ExecutorGeneration != 2 || decoded.LastTakeoverAt == "" || decoded.LastTakeoverBy != "main-agent" || decoded.LastTakeoverReason != "replace stale session" || len(decoded.MissionBrief.AuthorizedGates) != 1 || len(decoded.MissionBrief.OpenDecisions) != 1 {
		t.Fatalf("checkpoint mission brief contract drifted: %+v", decoded)
	}
	if !decoded.ExecutorAction.Blocked || !decoded.ExecutorAction.OpenDecisionRequired || decoded.ExecutorAction.OpenDecisions != 1 || decoded.ExecutorAction.PendingGates != 0 || decoded.ExecutorAction.OpenInterventions != 0 || decoded.ExecutorAction.ResumeCommand != "/rekit continue main" || len(decoded.ExecutorAction.BlockerReasons) != 1 {
		t.Fatalf("checkpoint executor action contract drifted: %+v", decoded.ExecutorAction)
	}
	if len(decoded.AuthorizedGates) != 1 || len(decoded.ExecutionEvidenceReview) != 1 || decoded.ExecutionEvidenceReview[0].GateEventID != "evt-authorized" || decoded.ExecutionEvidenceReview[0].HandoffCommand != "/rekit handoff main" || decoded.Resume != ".rekit/lanes/main/prompts/RESUME.md" {
		t.Fatalf("checkpoint shortcut fields drifted: %+v", decoded)
	}
}
