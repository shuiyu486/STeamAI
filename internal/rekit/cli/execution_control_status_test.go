package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestStatusExecutionControlBlocksLiveLaneRoutesWithoutWrites(t *testing.T) {
	for _, root := range []struct {
		name       string
		stateDir   string
		entrypoint string
	}{
		{name: "current", stateDir: ".steamai", entrypoint: "/steamai"},
		{name: "legacy", stateDir: ".rekit", entrypoint: "/rekit"},
	} {
		for _, state := range []string{executioncontrol.StatePaused, "pending"} {
			t.Run(root.name+"-"+state, func(t *testing.T) {
				caseRoot, status := statusExecutionControlFixture(t, root.stateDir, root.entrypoint)
				preview, err := executioncontrol.Preview(caseRoot, executioncontrol.Options{
					Lane: "binary-analysis-main", Action: executioncontrol.ActionPause,
					Actor: "main-agent", Reason: "operator requested a bounded pause",
					PublicationStamp: "2026-08-18T12:00:00Z",
				})
				if err != nil {
					t.Fatal(err)
				}
				if state == executioncontrol.StatePaused {
					if _, err := executioncontrol.Apply(caseRoot, executioncontrol.Options{
						Lane: preview.Lane, Action: preview.Action, Actor: preview.Actor, Reason: preview.Reason,
						PublicationStamp: preview.PublicationStamp, ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
					}); err != nil {
						t.Fatal(err)
					}
				} else {
					writeStatusExecutionControlIntent(t, caseRoot, preview)
				}

				before := snapshotFiles(t, filepath.Join(caseRoot, root.stateDir))
				if err := bindStatusExecutionControls(&status); err != nil {
					t.Fatal(err)
				}
				if err := projectStatusPublicEntrypoint(&status); err != nil {
					t.Fatal(err)
				}
				assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, root.stateDir)))

				if len(status.ExecutionControls) != 1 {
					t.Fatalf("status omitted selected execution control: %+v", status.ExecutionControls)
				}
				control := status.ExecutionControls[0]
				if control.Lane != "binary-analysis-main" || !control.Blocked {
					t.Fatalf("status control identity drifted: %+v", control)
				}
				if state == executioncontrol.StatePaused {
					if control.State != executioncontrol.StatePaused || control.Pending || control.CurrentGeneration != 1 || len(control.CurrentReceiptSHA256) != 64 || control.RecoveryCommand != "" {
						t.Fatalf("paused control head is incomplete: %+v", control)
					}
				} else if control.State != executioncontrol.StateRunning || !control.Pending || control.PendingGeneration != 1 || control.PendingAction != executioncontrol.ActionPause || !strings.HasPrefix(control.RecoveryCommand, root.entrypoint+" control ") {
					t.Fatalf("pending control recovery is incomplete: %+v", control)
				}

				if status.CaseMission == nil || status.CaseMission.FirstScreenLaneTakeoverPackage != nil || status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage != nil {
					t.Fatalf("controlled lane leaked takeover or Reviewer operator package: %+v", status.CaseMission)
				}
				request := status.MissionControlRunbook.CurrentDriverRequest
				if request == nil || request.CommandExecutable || !request.Blocked || request.Invocation != nil || request.Command != "" || strings.TrimSpace(request.Guidance) == "" {
					t.Fatalf("controlled lane leaked an executable request: %+v", request)
				}
				if status.MissionControlRunbook.CurrentLoopOperator != nil || status.MissionControlRunbook.ReplacementExecutorTakeover != nil || status.MissionControlRunbook.CurrentDriverRequestSHA256 == "" {
					t.Fatalf("controlled lane leaked nested execution package: %+v", status.MissionControlRunbook)
				}
				if state == "pending" && !strings.HasPrefix(request.Guidance, root.entrypoint+" control ") {
					t.Fatalf("pending request lost exact project entrypoint: %q", request.Guidance)
				}
				compact, err := buildStatusCompactInventory(status)
				if err != nil || len(compact.ExecutionControls) != 1 || compact.ExecutionControls[0].State != control.State || compact.ExecutionControls[0].Pending != control.Pending {
					t.Fatalf("compact status lost control head: compact=%+v err=%v", compact.ExecutionControls, err)
				}
			})
		}
	}
}

func statusExecutionControlFixture(t *testing.T, stateDir, entrypoint string) (string, statusInventory) {
	t.Helper()
	const lane = "binary-analysis-main"
	caseRoot := filepath.Join(t.TempDir(), "case")
	writeCaseFile(t, caseRoot, stateDir+"/instance.yml", "schemaVersion: 1\n")
	writeCaseFile(t, caseRoot, stateDir+"/board.json", `{"schemaVersion":1,"lanes":[{"id":"binary-analysis-main","status":"open","currentExecutor":"member-main","executorGeneration":3}]}`+"\n")
	writeCaseFile(t, caseRoot, stateDir+"/lanes/"+lane+"/lane.json", `{"schemaVersion":1,"id":"binary-analysis-main","status":"open","currentExecutor":"member-main","executorGeneration":3}`+"\n")
	command := entrypoint + " continue -Lane " + lane + " -WhatIf -Format json"
	action := mission.MissionCommanderNextActionItem{
		Lane: lane, Label: lane, State: "ready-to-continue", Command: command,
		Source: "missionCommanderActions",
	}
	queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{action})
	caseMission := &statusCaseMission{
		Ready: true, LaneCount: 1, ReadyLaneCount: 1, ReadyLanes: []string{lane},
		LaneExecutorActions: []mission.LaneExecutorActionSnapshot{{
			Lane: lane, Label: lane, Status: "open", CurrentExecutor: "member-main", ExecutorGeneration: 3,
			ExecutorAction: mission.ExecutorAction{Ready: true, ResumeCommand: command, MissionCommanderAction: mission.MissionCommanderAction{
				State: "ready-to-continue", PrimaryCommand: command,
			}},
		}},
		MissionCommanderNextActions: []mission.MissionCommanderNextActionItem{action},
		MissionCommanderActionQueue: queue,
	}
	status := statusInventory{
		Command: "status", SchemaVersion: 1, Target: caseRoot, TargetProvided: true,
		Mode: "case", Pack: "binary-re", CaseMission: caseMission, selectedCurrentLane: lane,
	}
	status.MissionControlRunbook = buildStatusMissionControlRunbookWithConsumption(caseRoot, caseMission, nil, nil)
	return caseRoot, status
}

func writeStatusExecutionControlIntent(t *testing.T, caseRoot string, plan executioncontrol.Plan) {
	t.Helper()
	intent := executioncontrol.Intent{
		SchemaVersion: executioncontrol.SchemaVersion, Kind: "lane-execution-control-intent",
		Lane: plan.Lane, Action: plan.Action, PreviousState: plan.PreviousState, State: plan.State,
		ControlGeneration: plan.ControlGeneration, PreviousReceiptSHA: plan.PreviousReceiptSHA,
		Owner: plan.Owner, Actor: plan.Actor, Reason: plan.Reason, PublicationStamp: plan.PublicationStamp,
		IntentPath: plan.IntentPath, ReceiptPath: plan.ReceiptPath, PlanSHA256: plan.ExpectedPlanSHA256,
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
	}
	data, err := json.MarshalIndent(intent, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, plan.IntentPath, string(append(data, '\n')))
}
