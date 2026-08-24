package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/currentloop"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/overview"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packmemoryconsumption"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	deepProjectionLegacyCommand = "/rekit status -Format json"
	deepProjectionProse         = "review /rekit status inside prose; do not rewrite"
	deepProjectionHostCommand   = "rekit-host -daily -target <target> -correction <human-correction>"
)

func TestProjectStatusPublicEntrypointProjectsDeepCaseSurface(t *testing.T) {
	for _, test := range []struct {
		name       string
		stateDir   string
		entrypoint string
	}{
		{name: "current", stateDir: projectstate.CurrentDir, entrypoint: commands.CurrentPublicEntrypoint},
		{name: "legacy", stateDir: projectstate.LegacyDir, entrypoint: commands.LegacyPublicEntrypoint},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, test.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			status := deepProjectionStatusFixture(t, caseRoot)
			waveSHA := status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage.Wave.SnapshotSHA256
			artifactSHA := status.MissionControlRunbook.CurrentLoopSegment.ArtifactSHA256

			if err := projectStatusPublicEntrypoint(&status); err != nil {
				t.Fatal(err)
			}
			if got := status.CaseShim.Entrypoint.CaseLocalFirstScreenCommand; got != test.entrypoint {
				t.Fatalf("case-local first-screen selector = %q, want %q", got, test.entrypoint)
			}
			if test.entrypoint == commands.CurrentPublicEntrypoint && status.ProjectHandoff != nil && (len(status.ProjectHandoff.ValidationCommands) != 0 || status.ProjectHandoff.NextBatchSelectionPackage != nil || status.ProjectHandoff.MissionCommanderActionQueue.Counts.Total != 0) {
				t.Fatalf("current public status retained central project handoff state: %+v", status.ProjectHandoff)
			}
			if got := status.MemberExecution.CorrectionCommand; got != deepProjectionHostCommand {
				t.Fatalf("non-slash host command changed: %q", got)
			}
			if got := status.CaseMission.ExecutionEvidenceReview[0].ReviewCommand; got != deepProjectionProse {
				t.Fatalf("execution review prose changed: %q", got)
			}
			if got := status.CaseMission.ExecutionEvidenceReview[0].Acknowledgement.RecordCommand; got != deepProjectionProse {
				t.Fatalf("acknowledgement prose changed: %q", got)
			}
			if got := status.CaseMission.ReviewerDispatchIntakeHandoffs[0].RunbookSteps[0]; got != deepProjectionProse {
				t.Fatalf("reviewer runbook prose changed: %q", got)
			}
			if got := status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage.Wave.SnapshotSHA256; got != waveSHA {
				t.Fatalf("source reviewer wave SHA changed: got %q want %q", got, waveSHA)
			}
			if got := status.MissionControlRunbook.CurrentLoopSegment.ArtifactSHA256; got != artifactSHA {
				t.Fatalf("durable current-loop artifact SHA changed: got %q want %q", got, artifactSHA)
			}

			encoded, err := json.Marshal(status)
			if err != nil {
				t.Fatal(err)
			}
			var public any
			if err := json.Unmarshal(encoded, &public); err != nil {
				t.Fatal(err)
			}
			assertDeepProjectionPublicCommands(t, "status", public, test.entrypoint)
			requestCount := assertDeepProjectionDriverRequests(t, "status", public, test.entrypoint)
			if requestCount < 10 {
				t.Fatalf("validated %d typed requests, want broad deep-surface coverage", requestCount)
			}

			request := status.MissionControlRunbook.CurrentDriverRequest
			actualSHA, err := mission.MissionCommanderDriverRequestSHA256(*request)
			if err != nil {
				t.Fatal(err)
			}
			if actualSHA != status.MissionControlRunbook.CurrentDriverRequestSHA256 {
				t.Fatalf("top-level request SHA = %q, want %q", status.MissionControlRunbook.CurrentDriverRequestSHA256, actualSHA)
			}
			evidence := status.CaseMission.ExecutionEvidenceReview[0]
			summary := status.CaseMission.ExecutionEvidenceReviewSummary
			if summary.LatestReviewCommand != evidence.ReviewCommand ||
				summary.LatestHandoffCommand != evidence.HandoffCommand ||
				summary.LatestCommanderPrimary != evidence.MissionCommanderAction.PrimaryCommand ||
				summary.CurrentAction != status.CaseMission.MissionCommanderActionQueue.CurrentAction.Command {
				t.Fatalf("execution evidence summary mirrors are stale: %+v", summary)
			}
		})
	}
}

func TestProjectStatusPublicEntrypointClearsGuidanceCommandAliases(t *testing.T) {
	for _, test := range []struct {
		name       string
		stateDir   string
		entrypoint string
	}{
		{name: "current", stateDir: projectstate.CurrentDir, entrypoint: commands.CurrentPublicEntrypoint},
		{name: "legacy", stateDir: projectstate.LegacyDir, entrypoint: commands.LegacyPublicEntrypoint},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, test.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			queue := mission.MissionCommanderActionQueueFor([]mission.MissionCommanderNextActionItem{{
				State:   "ready-for-next-batch-selection",
				Command: "select the next Windows-verifiable product-path closure",
				Source:  "releaseHandoffNextBatch",
			}})
			runbook := buildStatusMissionControlRunbook("", nil, &statusProjectHandoff{
				MissionCommanderActionQueue: queue,
			})
			if runbook.CurrentDriverRequest == nil || runbook.CurrentDriverRequest.CommandExecutable || runbook.CurrentDriverRequest.Guidance == "" {
				t.Fatalf("guidance fixture drifted: %+v", runbook)
			}
			status := statusInventory{Target: caseRoot, MissionControlRunbook: runbook}
			if err := projectStatusPublicEntrypoint(&status); err != nil {
				t.Fatal(err)
			}
			request := status.MissionControlRunbook.CurrentDriverRequest
			if status.MissionControlRunbook.CurrentCommand != "" || status.MissionControlRunbook.CurrentRunLoopStepID != request.RunLoopStepID || status.MissionControlRunbook.RefreshStatusCommand != request.ExpectedReceipt.RefreshStatusCommand {
				t.Fatalf("public runbook aliases differ from guidance request: %+v", status.MissionControlRunbook)
			}
			if status.MissionControlRunbook.Quickstart == nil || status.MissionControlRunbook.Quickstart.Command != "" || status.MissionControlRunbook.Quickstart.Guidance == "" {
				t.Fatalf("quickstart guidance alias drifted: %+v", status.MissionControlRunbook.Quickstart)
			}
			actualSHA, err := mission.MissionCommanderDriverRequestSHA256(*request)
			if err != nil || actualSHA != status.MissionControlRunbook.CurrentDriverRequestSHA256 {
				t.Fatalf("projected guidance request SHA drifted: actual=%q stored=%q err=%v", actualSHA, status.MissionControlRunbook.CurrentDriverRequestSHA256, err)
			}
		})
	}
}

func TestProjectStatusPublicEntrypointPreservesPausedReviewerWaveFence(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	status := statusInventory{
		Mode:   "case",
		Target: caseRoot,
		CaseMission: &statusCaseMission{
			ReviewerDispatchIntakeSummary: workstream.ReviewerDispatchIntakeSummary{
				OperatorPackage: &workstream.ReviewerDispatchOperatorPackage{
					Ready:       false,
					Paused:      true,
					PauseReason: "open intervention",
					Current:     &workstream.ReviewerDispatchOperatorPackageItem{},
					Wave: &workstream.ReviewerDispatchWavePackage{
						Ready:          false,
						Paused:         true,
						AvailableSlots: 0,
						SpawnWave:      nil,
						Shards:         []workstream.ReviewerDispatchWavePackageItem{{}},
					},
				},
			},
		},
	}
	if err := projectStatusPublicEntrypoint(&status); err != nil {
		t.Fatal(err)
	}
	pkg := status.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage
	if pkg.Ready || !pkg.Paused || pkg.CurrentDriverRequest != nil || pkg.RefreshStatusCommand != "" || len(pkg.RunLoop) != 0 {
		t.Fatalf("paused reviewer operator fence changed: %+v", pkg)
	}
	if pkg.Current == nil || pkg.Current.DispatchCommand != "" || pkg.Current.NextAction != "" {
		t.Fatalf("paused reviewer current item regained commands: %+v", pkg.Current)
	}
	wave := pkg.Wave
	if wave == nil || wave.Ready || !wave.Paused || wave.SnapshotSHA256 != "" || wave.AvailableSlots != 0 || len(wave.SpawnWave) != 0 {
		t.Fatalf("paused reviewer wave fence changed: %+v", wave)
	}
	if len(wave.Shards) != 1 || wave.Shards[0].RecordDispatchCommand != "" || wave.Shards[0].CurrentDriverRequest != nil {
		t.Fatalf("paused reviewer shard regained executable state: %+v", wave.Shards)
	}
}

func TestProjectOverviewPublicEntrypointProjectsCommandsAndPreservesProse(t *testing.T) {
	const mixedProse = "/rekit handoff main 或 /rekit handoff <name>"
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, fixture.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			inventory := overview.Inventory{
				Command: "overview",
				MissionBrief: mission.Brief{NextAgentActions: []string{
					deepProjectionLegacyCommand,
					deepProjectionProse,
				}},
				NextSteps: []string{
					deepProjectionLegacyCommand,
					deepProjectionProse,
					mixedProse,
				},
			}

			if err := projectOverviewInventoryPublicEntrypoint(&inventory, caseRoot); err != nil {
				t.Fatal(err)
			}
			wantCommand := fixture.entrypoint + " status -Format json"
			if inventory.Command != "overview" {
				t.Fatalf("top-level command enum changed: %q", inventory.Command)
			}
			if got := inventory.MissionBrief.NextAgentActions[0]; got != wantCommand {
				t.Fatalf("mission brief command = %q, want %q", got, wantCommand)
			}
			if got := inventory.NextSteps[0]; got != wantCommand {
				t.Fatalf("next-step command = %q, want %q", got, wantCommand)
			}
			if got := inventory.MissionBrief.NextAgentActions[1]; got != deepProjectionProse {
				t.Fatalf("mission brief prose changed: %q", got)
			}
			if got := inventory.NextSteps[1]; got != deepProjectionProse {
				t.Fatalf("next-step prose changed: %q", got)
			}
			if got := inventory.NextSteps[2]; got != mixedProse {
				t.Fatalf("slash-leading mixed prose changed: %q", got)
			}
		})
	}
}

func TestProjectPublicCommandForEntrypointParsesAndCanonicalizesStrictFields(t *testing.T) {
	for _, test := range []struct {
		name    string
		command string
		want    string
		wantErr string
	}{
		{name: "tab separator", command: "/rekit\tstatus -Format json", want: "/steamai status -Format json"},
		{name: "already current canonicalized", command: `/steamai status   -Format "json"`, want: "/steamai status -Format json"},
		{name: "unknown slash entrypoint", command: "/unknown status", wantErr: "must begin"},
		{name: "unterminated quote", command: `/rekit status -Target "case`, wantErr: "unterminated quote"},
		{name: "ordinary prose", command: deepProjectionProse, want: deepProjectionProse},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := projectPublicCommandForEntrypoint(test.command, commands.CurrentPublicEntrypoint)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("projection = %q err=%v, want %q", got, err, test.want)
			}
		})
	}
}

func TestProjectOverviewPublicEntrypointSeparatesCommandsFromProse(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	const prose = "/rekit status is documentation, not a command"
	inventory := overview.Inventory{NextSteps: []string{
		"/rekit status -Target /rekit",
		prose,
	}}
	if err := projectOverviewInventoryPublicEntrypoint(&inventory, caseRoot); err != nil {
		t.Fatal(err)
	}
	if got := inventory.NextSteps[0]; got != "/steamai status -Target /rekit" {
		t.Fatalf("command with slash argument = %q", got)
	}
	if got := inventory.NextSteps[1]; got != prose {
		t.Fatalf("slash-leading prose changed: %q", got)
	}
}

func TestProjectOverviewPublicEntrypointRejectsMalformedTypedCommand(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	inventory := overview.Inventory{NextSteps: []string{"/rekit unknown -Format json"}}
	if err := projectOverviewInventoryPublicEntrypoint(&inventory, caseRoot); err == nil ||
		!strings.Contains(err.Error(), "nextSteps[0]") {
		t.Fatalf("malformed overview command did not fail closed: %v", err)
	}
}

func TestProjectStatusPublicEntrypointRejectsMalformedDeepCommand(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	status := deepProjectionStatusFixture(t, caseRoot)
	status.CaseMission.ReviewerDispatchIntakeSummary.NextActionCollectionApplyCommand = "/rekit unknown -Format json"
	if err := projectStatusPublicEntrypoint(&status); err == nil ||
		!strings.Contains(err.Error(), "nextActionCollectionApplyCommand") {
		t.Fatalf("malformed deep command did not fail closed: %v", err)
	}
}

func TestBuildStatusDiagnosticsDTOProjectsWithoutMutatingSource(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	source := deepProjectionStatusFixture(t, caseRoot)
	source.selectedCurrentLane = "feature-mission"
	source.CaseMission.Sections = &overview.OverviewSections{
		PendingGates: overview.EventSection{Events: []map[string]any{{
			"generation": int64(9007199254740993),
		}}},
	}
	before := mustJSONBytes(t, source)

	diagnostics, err := buildStatusDiagnosticsDTO(source)
	if err != nil {
		t.Fatal(err)
	}
	after := mustJSONBytes(t, source)
	if string(after) != string(before) {
		t.Fatalf("status diagnostics projection mutated source:\nbefore=%s\nafter=%s", before, after)
	}
	projected := statusInventory(diagnostics)
	if projected.selectedCurrentLane != source.selectedCurrentLane {
		t.Fatalf("status diagnostics lost internal selector binding: got %q want %q", projected.selectedCurrentLane, source.selectedCurrentLane)
	}
	if got := projected.MissionControlRunbook.CurrentDriverRequest.Command; !strings.HasPrefix(got, commands.CurrentPublicEntrypoint+" ") {
		t.Fatalf("status diagnostics did not project current entrypoint: %q", got)
	}
	if got := source.MissionControlRunbook.CurrentDriverRequest.Command; got != deepProjectionLegacyCommand {
		t.Fatalf("status diagnostics projection aliased source request: %q", got)
	}
	encoded := string(mustJSONBytes(t, diagnostics))
	if !strings.Contains(encoded, `"generation":9007199254740993`) {
		t.Fatalf("status diagnostics changed a large integer token: %s", encoded)
	}
}

func TestBuildStatusDiagnosticsDTOFailureDoesNotPartiallyMutateSource(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.CurrentDir), 0o755); err != nil {
		t.Fatal(err)
	}
	source := deepProjectionStatusFixture(t, caseRoot)
	source.CaseMission.ReviewerDispatchIntakeSummary.NextActionCollectionApplyCommand = "/rekit unknown -Format json"
	before := mustJSONBytes(t, source)
	_, err := buildStatusDiagnosticsDTO(source)
	if err == nil || !strings.Contains(err.Error(), "nextActionCollectionApplyCommand") {
		t.Fatalf("invalid status diagnostics command error=%v", err)
	}
	after := mustJSONBytes(t, source)
	if string(after) != string(before) {
		t.Fatalf("failed status diagnostics projection partially mutated source:\nbefore=%s\nafter=%s", before, after)
	}
	if source.CaseShim.Entrypoint.CaseLocalFirstScreenCommand != commands.LegacyPublicEntrypoint {
		t.Fatalf("failed status projection leaked clone mutation: %+v", source.CaseShim.Entrypoint)
	}
}

func deepProjectionStatusFixture(t *testing.T, caseRoot string) statusInventory {
	t.Helper()
	command := deepProjectionLegacyCommand
	request := func() *mission.MissionCommanderDriverRequest {
		return deepProjectionDriverRequest(t, command)
	}
	action := func() mission.MissionCommanderNextActionItem {
		invocation, err := commands.ParsePublicInvocation(command)
		if err != nil {
			t.Fatal(err)
		}
		return mission.MissionCommanderNextActionItem{
			State: "review-required", Source: "deep-projection-fixture", Command: command,
			Invocation: &invocation, RequiresReview: true,
		}
	}
	queue := func() mission.MissionCommanderActionQueue {
		current := action()
		return mission.MissionCommanderActionQueue{
			Summary:               "deep projection queue",
			CurrentAction:         &current,
			CurrentActionRunLoop:  []mission.MissionCommanderRunLoopStep{{Command: command}},
			CurrentDriverRequest:  request(),
			UnblockedActions:      []mission.MissionCommanderNextActionItem{action()},
			ReviewRequiredActions: []mission.MissionCommanderNextActionItem{action()},
		}
	}
	operatorItem := func() *workstream.ReviewerDispatchOperatorPackageItem {
		return &workstream.ReviewerDispatchOperatorPackageItem{
			DispatchPromptRepairCommand:               command,
			ReviewerDispatchRecordCommand:             command,
			ReviewerCompletionRecordCommand:           command,
			ReviewerResultInputSavePreviewCommand:     command,
			ReviewerResultInputSaveApplyCommand:       command,
			ReviewerResultSourceCapturePreviewCommand: command,
			ReviewerResultSourceCaptureApplyCommand:   command,
			ReviewerResultStagingPreviewCommand:       command,
			ReviewerResultCollectionPreviewCommand:    command,
			ReviewerResultCollectionApplyCommand:      command,
			ReviewerResultIntakePreviewCommand:        command,
			ReviewerResultIntakeApplyCommand:          command,
			ReviewerResultBatchIntakePreviewCommand:   command,
			ReviewerResultBatchIntakeApplyCommand:     command,
			DispatchCommand:                           command,
			NextAction:                                command,
		}
	}
	waveItem := func() workstream.ReviewerDispatchWavePackageItem {
		return workstream.ReviewerDispatchWavePackageItem{
			RecordDispatchCommand:   command,
			RecordCompletionCommand: command,
			CurrentDriverRequest:    request(),
			OperatorItem:            operatorItem(),
		}
	}
	operator := &workstream.ReviewerDispatchOperatorPackage{
		Ready:                true,
		Current:              operatorItem(),
		CurrentDriverRequest: request(),
		RefreshStatusCommand: command,
		RunLoop:              []workstream.ReviewerDispatchRunLoopStep{{Command: command, PreviewCommand: command, ApplyCommand: command}},
		Wave: &workstream.ReviewerDispatchWavePackage{
			Ready:          true,
			SnapshotSHA256: strings.Repeat("a", 64),
			SpawnWave:      []workstream.ReviewerDispatchWavePackageItem{waveItem()},
			Active:         []workstream.ReviewerDispatchWavePackageItem{waveItem()},
			Returned:       []workstream.ReviewerDispatchWavePackageItem{waveItem()},
			Failed:         []workstream.ReviewerDispatchWavePackageItem{waveItem()},
			Blocked:        []workstream.ReviewerDispatchWavePackageItem{waveItem()},
			Complete:       []workstream.ReviewerDispatchWavePackageItem{waveItem()},
			Shards:         []workstream.ReviewerDispatchWavePackageItem{waveItem()},
		},
	}
	reviewerSummary := workstream.ReviewerDispatchIntakeSummary{
		LatestReviewerResultSourceCaptureCommand:          command,
		LatestReviewerResultSourceCaptureApplyCommand:     command,
		LatestReviewerResultStagingCommand:                command,
		LatestCollectionPreviewCommand:                    command,
		LatestCollectionApplyCommand:                      command,
		LatestPreviewCommand:                              command,
		LatestApplyCommand:                                command,
		LatestBatchPreviewCommand:                         command,
		LatestBatchApplyCommand:                           command,
		NextActionDispatchPromptRepairCommand:             command,
		NextActionReviewerResultSourceCaptureCommand:      command,
		NextActionReviewerResultSourceCaptureApplyCommand: command,
		NextActionReviewerResultStagingCommand:            command,
		NextActionCollectionPreviewCommand:                command,
		NextActionCollectionApplyCommand:                  command,
		NextActionPreviewCommand:                          command,
		NextActionApplyCommand:                            command,
		NextActionBatchPreviewCommand:                     command,
		NextActionBatchApplyCommand:                       command,
		NextActionPacketRetirementPreviewCommand:          command,
		NextAction:                                        command,
		OperatorPackage:                                   operator,
	}
	reviewerHandoff := workstream.ReviewerDispatchIntakeHandoff{
		DispatchPromptRepairCommand:              command,
		ReviewerDispatchRecordCommand:            command,
		ReviewerCompletionRecordCommand:          command,
		ReviewerResultInputSaveCommand:           command,
		ReviewerResultInputSaveApplyCommand:      command,
		ReviewerResultSourceCaptureCommand:       command,
		ReviewerResultSourceCaptureApplyCommand:  command,
		ReviewerResultStagingCommand:             command,
		ReviewerResultCollectionCommands:         &workstream.ReviewerResultCollectionCommands{PreviewCommand: command, ApplyCommand: command},
		ReviewerResultRecoveryCommand:            command,
		ReviewerResultRecoveryApplyCommand:       command,
		ReviewerResultRecoveryDispositionCommand: command,
		DispatchCommand:                          command,
		PreviewCommand:                           command,
		ApplyCommand:                             command,
		BatchPreviewCommand:                      command,
		BatchApplyCommand:                        command,
		RefreshStatusCommand:                     command,
		OwnerAdoptionPreviewCommand:              command,
		PacketRetirementPreviewCommand:           command,
		ManagedDispatch: &workstream.ReviewerManagedDispatchHandoff{
			InputSavePreviewCommand:     command,
			InputSaveApplyCommand:       command,
			SourceCapturePreviewCommand: command,
			SourceCaptureApplyCommand:   command,
			StagingPreviewCommand:       command,
			CollectionPreviewCommand:    command,
			CollectionApplyCommand:      command,
			IntakePreviewCommand:        command,
			IntakeApplyCommand:          command,
			DispatchCommand:             command,
			NextAction:                  command,
		},
		RunbookSteps: []string{deepProjectionProse},
	}
	evidence := workstream.ExecutionEvidenceReviewItem{
		EventID:            "evidence-1",
		ReviewCommand:      deepProjectionProse,
		HandoffCommand:     command,
		ReviewRunbookSteps: []string{deepProjectionProse},
		Acknowledgement: &mission.ExecutionEvidenceReviewAcknowledgement{
			AcknowledgementReviewCommand: command,
			AcceptedPreviewCommand:       command,
			RejectedPreviewCommand:       command,
			RecordCommand:                deepProjectionProse,
		},
		MissionCommanderAction: mission.MissionCommanderAction{
			PrimaryCommand:   command,
			FollowUpCommands: []string{command, deepProjectionProse},
		},
		FollowThrough: mission.ExecutionEvidenceFollowThrough{
			Outcomes: []mission.ExecutionEvidenceOutcome{{
				Command:              command,
				VerificationCommands: []string{command, deepProjectionProse},
			}},
			ActionQueue: queue(),
		},
	}
	caseQueue := queue()
	caseMission := &statusCaseMission{
		PendingGateHandoffs: []statusPendingGateHandoff{{ReviewCommand: command, WhatIfCommand: command, ApplyCommand: command}},
		AuthorizedGateHandoffs: []statusAuthorizedGateHandoff{{
			ReportContract:            command,
			HandoffCommand:            command,
			ReportSummary:             &gate.AdapterReportHandoffSummary{CurrentAction: command},
			LiveValidationRepairHints: []gate.AdapterReportRepairHint{{RepairAction: command}},
			LiveValidation: &statusAuthorizedGateLiveValidationHandoff{
				DispatchCommand:                  command,
				RunLoop:                          []mission.MissionCommanderRunLoopStep{{Command: command}},
				ValidateCommand:                  command,
				RecordCommand:                    command,
				ScaffoldCommand:                  command,
				ScaffoldApplyCommand:             command,
				DraftCommand:                     command,
				DraftApplyCommand:                command,
				ReceiptPreviewCommand:            command,
				CaseRelativeValidateCommand:      command,
				CaseRelativeRecordCommand:        command,
				CaseRelativeScaffoldCommand:      command,
				CaseRelativeScaffoldApplyCommand: command,
				CaseRelativeDraftCommand:         command,
				CaseRelativeDraftApplyCommand:    command,
			},
		}},
		OpenDecisionHandoffs: []statusOpenDecisionHandoff{{
			SourceCommand: command, ReviewCommand: command, WhatIfCommand: command, RecordCommand: command,
		}},
		InterventionHandoffs: []statusInterventionHandoff{{
			ReviewCommand: command, WhatIfCommand: command, ApplyCommand: command,
		}},
		ReviewerDispatchIntakeHandoffs:    []workstream.ReviewerDispatchIntakeHandoff{reviewerHandoff},
		ReviewerDispatchIntakeSummary:     reviewerSummary,
		ReviewerPacketRetirementHandoffs:  []workstream.ReviewerPacketRetirementHandoff{{NextAction: command, RunbookSteps: []string{deepProjectionProse}}},
		ReviewerPacketRetirementSummary:   workstream.ReviewerPacketRetirementSummary{NextAction: command, RunbookSteps: []string{deepProjectionProse}},
		ReviewerDispatchIntakeActionQueue: queue(),
		ExecutionEvidenceReview:           []workstream.ExecutionEvidenceReviewItem{evidence},
		ExecutionEvidenceReviewSummary:    workstream.ExecutionEvidenceReviewSummary{LatestHandoffCommand: "stale"},
		MissionCommanderActionQueue:       caseQueue,
		MissionCommanderNextActions:       []mission.MissionCommanderNextActionItem{action()},
		DailyMissionControlRunbook: &workstream.DailyMissionControlRunbook{
			CurrentCommand:              command,
			CurrentDriverRequest:        request(),
			RefreshStatusCommand:        command,
			HandoffPreviewCommand:       command,
			HandoffApplyCommand:         command,
			HandoffPreviewDriverRequest: request(),
			HandoffApplyDriverRequest:   request(),
			RunLoop:                     []workstream.DailyMissionControlRunbookStep{{Command: command}},
		},
		HandoffPreviewCommand: command,
		HandoffApplyCommand:   command,
	}
	currentLoopSegment := &currentloop.Inspection{
		ArtifactSHA256:                strings.Repeat("b", 64),
		ResumeDriverRequest:           request(),
		RefreshedCurrentDriverRequest: request(),
		LegacyUnboundWhatIfCommand:    command,
		Continuation: &currentloop.Continuation{
			WhatIfCommand: command,
			ObservationContract: &currentloop.ObservationContract{
				Alternatives: []currentloop.ObservationAlternative{{PreviewCommandTemplate: command}},
			},
			ExternalMemberHandoff: &mission.CurrentLoopExternalMemberHandoff{
				ObservationContract: mission.CurrentLoopObservationContract{
					Alternatives: []mission.CurrentLoopObservationAlternative{{
						PreviewCommandTemplate: command,
						ObservationPathCommand: command,
					}},
				},
			},
		},
	}
	runbookRequest := request()
	runbookSHA, err := mission.MissionCommanderDriverRequestSHA256(*runbookRequest)
	if err != nil {
		t.Fatal(err)
	}
	packAction := action()
	packQueue := queue()
	return statusInventory{
		Command: "status",
		Mode:    "case",
		Target:  caseRoot,
		CaseShim: statusCaseShim{Entrypoint: &statusCaseShimEntrypoint{
			CaseLocalFirstScreenCommand: commands.LegacyPublicEntrypoint,
			ExplicitFirstScreenCommand:  command,
		}},
		PackMemoryConsumption: &packMemoryConsumptionStatus{
			Discovery: packmemoryconsumption.Discovery{
				Available: []packmemoryconsumption.ChangeStatus{{PreviewCommand: command}},
				Consumed:  []packmemoryconsumption.ChangeStatus{{PreviewCommand: command}},
				Conflicts: []packmemoryconsumption.ChangeStatus{{PreviewCommand: command}},
			},
			MissionCommanderNextActions: []mission.MissionCommanderNextActionItem{packAction},
			MissionCommanderActionQueue: packQueue,
		},
		CaseMission: caseMission,
		MissionControlRunbook: &statusMissionControlRunbook{
			Scope:                      "case",
			CurrentCommand:             command,
			CurrentDriverRequest:       runbookRequest,
			CurrentDriverRequestSHA256: runbookSHA,
			CurrentDriverReceipt: &workstream.MissionCommanderDriverReceipt{
				Command:                       command,
				RefreshedCurrentDriverRequest: request(),
			},
			Quickstart: &statusMissionControlQuickstart{
				Command:              command,
				CurrentDriverRequest: request(),
				CurrentDriverReceipt: &workstream.MissionCommanderDriverReceipt{
					Command:                       command,
					RefreshedCurrentDriverRequest: request(),
				},
				RefreshStatusCommand: command,
				CurrentLoopOperator:  &mission.CurrentLoopOperatorPackage{SourceCurrentDriverRequest: request()},
			},
			CurrentLoopSegment: currentLoopSegment,
			CurrentLoopOperator: &mission.CurrentLoopOperatorPackage{
				SourceCurrentDriverRequest: request(),
				SelectedDriverRequest:      request(),
				StartDriverRequest:         request(),
				ResumeDriverRequest:        request(),
				ObservationInbox:           &mission.CurrentLoopObservationInbox{SelectedDriverRequest: request()},
				ExternalMemberHandoff: &mission.CurrentLoopExternalMemberHandoff{
					ObservationContract: mission.CurrentLoopObservationContract{
						Alternatives: []mission.CurrentLoopObservationAlternative{{
							PreviewCommandTemplate: command,
							ObservationPathCommand: command,
						}},
					},
				},
			},
			RefreshStatusCommand:        command,
			HandoffPreviewCommand:       command,
			HandoffApplyCommand:         command,
			HandoffPreviewDriverRequest: request(),
			HandoffApplyDriverRequest:   request(),
			Queues:                      []statusMissionControlRunbookQueue{{CurrentCommand: command}},
			RunLoop:                     []statusMissionControlRunbookStep{{Command: command}},
		},
		MemberExecution: &memberExecutionStatus{
			PreviewCommand:      command,
			ObservationCommand:  command,
			ReviewerPlanCommand: command,
			CorrectionCommand:   deepProjectionHostCommand,
		},
	}
}

func deepProjectionDriverRequest(t *testing.T, command string) *mission.MissionCommanderDriverRequest {
	t.Helper()
	request, err := mission.MissionCommanderDriverRequestWithTypedCommand(
		mission.MissionCommanderDriverRequest{
			Kind:              "preview-command",
			RunLoopStepID:     "deep-projection-fixture",
			State:             "review-required",
			Source:            "deep-projection-fixture",
			Command:           command,
			CommandExecutable: true,
			RequiresReview:    true,
			ExpectedReceipt: mission.MissionCommanderDriverReceiptExpectation{
				State:                "preview-ready",
				RefreshStatusCommand: command,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return &request
}

func assertDeepProjectionPublicCommands(t *testing.T, path string, value any, entrypoint string) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			assertDeepProjectionPublicCommands(t, path+"."+key, child, entrypoint)
		}
	case []any:
		for index, child := range typed {
			assertDeepProjectionPublicCommands(t, fmt.Sprintf("%s[%d]", path, index), child, entrypoint)
		}
	case string:
		text := strings.TrimSpace(typed)
		if (strings.HasPrefix(text, commands.LegacyPublicEntrypoint+" ") ||
			strings.HasPrefix(text, commands.CurrentPublicEntrypoint+" ")) &&
			!strings.HasPrefix(text, entrypoint+" ") {
			t.Errorf("%s uses mixed public entrypoint: %q; want %s", path, text, entrypoint)
		}
	}
}

func assertDeepProjectionDriverRequests(t *testing.T, path string, value any, entrypoint string) int {
	t.Helper()
	count := 0
	switch typed := value.(type) {
	case map[string]any:
		if _, hasExecutable := typed["commandExecutable"]; hasExecutable && typed["expectedReceipt"] != nil {
			encoded, err := json.Marshal(typed)
			if err != nil {
				t.Fatal(err)
			}
			var request mission.MissionCommanderDriverRequest
			if err := json.Unmarshal(encoded, &request); err != nil {
				t.Fatalf("decode typed request at %s: %v", path, err)
			}
			if err := mission.ValidateMissionCommanderDriverRequest(request); err != nil {
				t.Errorf("invalid driver request at %s: %v", path, err)
			} else if request.Invocation != nil {
				projected, parseErr := commands.ParsePublicInvocation(request.Command)
				if parseErr != nil || !request.Invocation.Equivalent(projected) {
					t.Errorf("typed request at %s command=%q does not match invocation=%+v err=%v", path, request.Command, request.Invocation, parseErr)
				}
				if !strings.HasPrefix(strings.TrimSpace(request.Command), entrypoint+" ") {
					t.Errorf("typed request at %s command=%q uses the wrong entrypoint; want %s", path, request.Command, entrypoint)
				}
				count++
			} else if request.CommandExecutable || strings.TrimSpace(request.Command) != "" {
				t.Errorf("command request at %s omitted invocation: %+v", path, request)
			}
		}
		for key, child := range typed {
			count += assertDeepProjectionDriverRequests(t, path+"."+key, child, entrypoint)
		}
	case []any:
		for index, child := range typed {
			count += assertDeepProjectionDriverRequests(t, fmt.Sprintf("%s[%d]", path, index), child, entrypoint)
		}
	}
	return count
}
