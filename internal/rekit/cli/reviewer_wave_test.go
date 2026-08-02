package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReviewerWaveRecordsParallelAcceptances(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	status := reviewerWaveStatus(t, caseRoot)
	wave := reviewerWaveFromStatus(status)
	if wave == nil || len(wave.SpawnWave) != 2 {
		t.Fatalf("initial reviewer wave = %+v", wave)
	}
	observationPath := filepath.Join(caseRoot, "workspace", "wave-accepted.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-session-01"},
		{ShardID: wave.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-session-02"},
	}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	if preview.ObservationCount != 2 || len(preview.Previews) != 2 || preview.ExpectedReviewerWavePlanSHA256 == "" || preview.Previews[0].ExpectedBindingSHA256 == "" || preview.Previews[1].ExpectedBindingSHA256 == "" {
		t.Fatalf("reviewer wave acceptance preview = %+v", preview)
	}
	out.Reset()
	applyArgs := append(append([]string{}, args...), "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply")
	if err := Run(applyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var applied reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.AppliedCount != 2 || applied.RefreshedWave == nil || applied.RefreshedWave.ActiveSlots != 2 || len(applied.RefreshedWave.Active) != 2 || len(applied.RefreshedWave.SpawnWave) != 0 {
		t.Fatalf("reviewer wave acceptance apply = %+v", applied)
	}
	for _, item := range applied.RefreshedWave.Active {
		if item.AgentToolRequest != nil || item.ReviewerSession == "" {
			t.Fatalf("active reviewer shard still exposed spawn request: %+v", item)
		}
	}
	out.Reset()
	if err := Run(applyArgs, &out); err == nil || (!strings.Contains(err.Error(), "expected plan sha256 mismatch") && !strings.Contains(err.Error(), "current spawnWave")) {
		t.Fatalf("stale reviewer wave bundle was not rejected: %v", err)
	}
}

func TestRunReviewerWaveRecordsReturnedAndFailedObservations(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	acceptPath := filepath.Join(caseRoot, "workspace", "wave-accept-for-terminal.json")
	writeReviewerWaveObservations(t, acceptPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-return-session"},
		{ShardID: wave.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-fail-session"},
	}})
	acceptArgs := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", acceptPath, "-Format", "json"}
	acceptPreview := reviewerWavePreview(t, append(acceptArgs, "-WhatIf")...)
	out.Reset()
	if err := Run(append(acceptArgs, "-ExpectedReviewerWavePlanSha256", acceptPreview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	activeWave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if len(activeWave.Active) != 2 {
		t.Fatalf("accepted reviewer wave = %+v", activeWave)
	}
	packet := decodePlanSubagentsPacket(t, activeWave.PacketPath)
	first := packet.ShardHandoffs[0]
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := filepath.Join(caseRoot, "workspace", "wave-return-result.json")
	if err := os.WriteFile(resultSource, reviewerResultForCLIPlan(t, packet, first, "accept", "accepted", "wave-return-session"), 0o644); err != nil {
		t.Fatal(err)
	}
	terminalPath := filepath.Join(caseRoot, "workspace", "wave-terminal.json")
	writeReviewerWaveObservations(t, terminalPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: activeWave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: activeWave.Active[0].ShardID, Kind: "returned", ReviewerExitStatus: "completed", ReviewerResultInputSourcePath: resultSource},
		{ShardID: activeWave.Active[1].ShardID, Kind: "failed", ReviewerDispatchID: activeWave.Active[1].ReviewerDispatchID, ReviewerExitStatus: "reviewer-error"},
	}})
	terminalArgs := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", activeWave.PacketPath, "-Lane", activeWave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", terminalPath, "-Format", "json"}
	terminalPreview := reviewerWavePreview(t, append(terminalArgs, "-WhatIf")...)
	if terminalPreview.Previews[0].ExpectedInputSaveSHA256 == "" || terminalPreview.Previews[0].ExpectedDispatchSHA256 == "" || terminalPreview.Previews[1].ExpectedDispatchSHA256 == "" {
		t.Fatalf("terminal reviewer wave preview = %+v", terminalPreview)
	}
	out.Reset()
	if err := Run(append(terminalArgs, "-ExpectedReviewerWavePlanSha256", terminalPreview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	var applied reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.AppliedCount != 2 || applied.RefreshedWave == nil || applied.RefreshedWave.ActiveSlots != 0 || len(applied.RefreshedWave.Returned) != 1 || len(applied.RefreshedWave.Failed) != 1 || len(applied.RefreshedWave.SpawnWave) != 1 || applied.RefreshedWave.SpawnWave[0].ShardID != activeWave.Active[1].ShardID {
		t.Fatalf("terminal reviewer wave apply = %+v", applied)
	}
}

func TestRunReviewerWaveRejectsInvalidObservationContracts(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	observationPath := filepath.Join(caseRoot, "workspace", "wave-invalid.json")
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-WhatIf", "-Format", "json"}
	valid := reviewerWaveObservation{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-contract-session"}

	tests := []struct {
		name       string
		file       reviewerWaveObservationFile
		packetPath string
		lane       string
		want       string
	}{
		{name: "duplicate shard", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{valid, valid}}, want: "repeats shardId"},
		{name: "packet mismatch", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: "other-packet", Observations: []reviewerWaveObservation{valid}}, want: "does not match current packet"},
		{name: "lane mismatch", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{valid}}, lane: "other-lane", want: "packet or lane does not match"},
		{name: "accepted terminal field", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: valid.ShardID, Kind: "accepted", ReviewerHarness: valid.ReviewerHarness, ReviewerSession: valid.ReviewerSession, ReviewerExitStatus: "completed"}}}, want: "does not accept terminal"},
		{name: "returned without result", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: valid.ShardID, Kind: "returned"}}}, want: "requires reviewerResultInputSourcePath"},
		{name: "failed without dispatch", file: reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: valid.ShardID, Kind: "failed", ReviewerExitStatus: "reviewer-error"}}}, want: "requires reviewerDispatchId"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writeReviewerWaveObservations(t, observationPath, test.file)
			callArgs := append([]string{}, args...)
			if test.lane != "" {
				for idx := range callArgs {
					if callArgs[idx] == "-Lane" {
						callArgs[idx+1] = test.lane
					}
				}
			}
			var result bytes.Buffer
			err := Run(callArgs, &result)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid reviewer wave contract error = %v, want %q", err, test.want)
			}
		})
	}

	if err := os.WriteFile(observationPath, []byte(`{"schemaVersion":1,"packetId":"`+wave.PacketID+`","observations":[{"shardId":"`+valid.ShardID+`","kind":"accepted","reviewerHarness":"go-test-harness","reviewerSession":"wave-contract-session","unknown":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(args, &out); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown reviewer wave field error = %v", err)
	}
}

func TestRunReviewerWaveRejectsAcceptanceBeyondSpawnWave(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if wave.MaxParallel != 1 || len(wave.SpawnWave) != 1 || len(wave.Shards) != 2 {
		t.Fatalf("bounded reviewer wave = %+v", wave)
	}
	observationPath := filepath.Join(caseRoot, "workspace", "wave-over-capacity.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: wave.Shards[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-capacity-01"},
		{ShardID: wave.Shards[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-capacity-02"},
	}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-WhatIf", "-Format", "json"}
	out.Reset()
	if err := Run(args, &out); err == nil || !strings.Contains(err.Error(), "current spawnWave") {
		t.Fatalf("over-capacity acceptance error = %v", err)
	}
	refreshed := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if refreshed.ActiveSlots != 0 {
		t.Fatalf("over-capacity acceptance wrote dispatch state: %+v", refreshed)
	}
}

func TestRunReviewerWaveBindsObservationFileBytes(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	observationPath := filepath.Join(caseRoot, "workspace", "wave-drift.json")
	observation := reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-before-drift"}}}
	writeReviewerWaveObservations(t, observationPath, observation)
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	observation.Observations[0].ReviewerSession = "wave-after-drift"
	writeReviewerWaveObservations(t, observationPath, observation)
	out.Reset()
	err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out)
	if err == nil || !strings.Contains(err.Error(), "expected plan sha256 mismatch") {
		t.Fatalf("observation file drift error = %v", err)
	}
}

func TestReadReviewerWaveObservationsRejectsUnsafeFiles(t *testing.T) {
	caseRoot := t.TempDir()
	workspace := filepath.Join(caseRoot, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readReviewerWaveObservations(caseRoot, outside); err == nil || !strings.Contains(err.Error(), "case-local") {
		t.Fatalf("outside observation error = %v", err)
	}
	oversized := filepath.Join(workspace, "oversized.json")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte{'x'}, 256*1024+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := readReviewerWaveObservations(caseRoot, oversized); err == nil || !strings.Contains(err.Error(), "bounded") {
		t.Fatalf("oversized observation error = %v", err)
	}
	linkRoot := filepath.Join(caseRoot, "linked-workspace")
	if err := os.Symlink(filepath.Dir(outside), linkRoot); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	linked := filepath.Join(linkRoot, filepath.Base(outside))
	if _, _, _, err := readReviewerWaveObservations(caseRoot, linked); err == nil || !strings.Contains(err.Error(), "symlink-free") {
		t.Fatalf("symlink ancestor observation error = %v", err)
	}
	stablePath := filepath.Join(workspace, "stable.json")
	if err := os.WriteFile(stablePath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewerWaveBeforeObservationOpenHook = func() error {
		if err := os.Remove(stablePath); err != nil {
			return err
		}
		return os.Symlink(outside, stablePath)
	}
	defer func() { reviewerWaveBeforeObservationOpenHook = nil }()
	if _, _, _, err := readReviewerWaveObservations(caseRoot, stablePath); err == nil || !strings.Contains(err.Error(), "changed while opening") {
		if err != nil && strings.Contains(err.Error(), "privilege") {
			t.Skipf("file symlink unavailable: %v", err)
		}
		t.Fatalf("observation replacement error = %v", err)
	}
}

func TestRunReviewerWaveRejectsReturnedResultFromPriorDispatch(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	applyWave := func(path string, observations []reviewerWaveObservation) reviewerWavePlan {
		writeReviewerWaveObservations(t, path, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: observations})
		args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", path, "-Format", "json"}
		preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
		out.Reset()
		if err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
			t.Fatal(err)
		}
		var applied reviewerWavePlan
		if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
			t.Fatal(err)
		}
		wave = applied.RefreshedWave
		return applied
	}
	applyWave(filepath.Join(caseRoot, "workspace", "wave-prior-accept.json"), []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-prior-session"}})
	priorDispatchID := wave.Active[0].ReviewerDispatchID
	applyWave(filepath.Join(caseRoot, "workspace", "wave-prior-failed.json"), []reviewerWaveObservation{{ShardID: wave.Active[0].ShardID, Kind: "failed", ReviewerDispatchID: priorDispatchID, ReviewerExitStatus: "reviewer-error"}})
	applyWave(filepath.Join(caseRoot, "workspace", "wave-current-accept.json"), []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-current-session"}})
	if wave.Active[0].ReviewerDispatchID == priorDispatchID {
		t.Fatalf("reviewer shard did not receive a new dispatch: %+v", wave.Active[0])
	}
	packet := decodePlanSubagentsPacket(t, wave.PacketPath)
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := filepath.Join(caseRoot, "workspace", "wave-prior-result.json")
	if err := os.WriteFile(resultSource, reviewerResultForCLIPlan(t, packet, packet.ShardHandoffs[0], "accept", "accepted", "wave-prior-session"), 0o644); err != nil {
		t.Fatal(err)
	}
	observationPath := filepath.Join(caseRoot, "workspace", "wave-prior-return.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: wave.Active[0].ShardID, Kind: "returned", ReviewerExitStatus: "completed", ReviewerResultInputSourcePath: resultSource}}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-WhatIf", "-Format", "json"}
	out.Reset()
	if err := Run(args, &out); err == nil || !strings.Contains(err.Error(), "returned result dispatch does not match") {
		t.Fatalf("prior dispatch returned result error = %v", err)
	}
	refreshed := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	if len(refreshed.Active) != 1 || refreshed.Active[0].ReviewerDispatchID != wave.Active[0].ReviewerDispatchID || len(refreshed.Returned) != 0 {
		t.Fatalf("prior dispatch result changed current wave: %+v", refreshed)
	}
}

func TestRunReviewerWaveReturnedPartialReportsInputMutation(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha", "-ItemsPerAgent", "1", "-MaxParallel", "1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	acceptPath := filepath.Join(caseRoot, "workspace", "wave-partial-return-accept.json")
	writeReviewerWaveObservations(t, acceptPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-partial-return"}}})
	acceptArgs := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", acceptPath, "-Format", "json"}
	acceptPreview := reviewerWavePreview(t, append(acceptArgs, "-WhatIf")...)
	out.Reset()
	if err := Run(append(acceptArgs, "-ExpectedReviewerWavePlanSha256", acceptPreview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	activeWave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	packet := decodePlanSubagentsPacket(t, activeWave.PacketPath)
	evidencePath := filepath.Join(caseRoot, "workspace", "features", "feature-login", "review-evidence.md")
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte("bounded reviewer evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	resultSource := filepath.Join(caseRoot, "workspace", "wave-partial-return-result.json")
	if err := os.WriteFile(resultSource, reviewerResultForCLIPlan(t, packet, packet.ShardHandoffs[0], "accept", "accepted", "wave-partial-return"), 0o644); err != nil {
		t.Fatal(err)
	}
	terminalPath := filepath.Join(caseRoot, "workspace", "wave-partial-return.json")
	writeReviewerWaveObservations(t, terminalPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: activeWave.PacketID, Observations: []reviewerWaveObservation{{ShardID: activeWave.Active[0].ShardID, Kind: "returned", ReviewerExitStatus: "completed", ReviewerResultInputSourcePath: resultSource}}})
	terminalArgs := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", activeWave.PacketPath, "-Lane", activeWave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", terminalPath, "-Format", "json"}
	terminalPreview := reviewerWavePreview(t, append(terminalArgs, "-WhatIf")...)
	reviewerWaveBeforeReturnedCompletionHook = func() error { return os.ErrPermission }
	defer func() { reviewerWaveBeforeReturnedCompletionHook = nil }()
	out.Reset()
	if err := Run(append(terminalArgs, "-ExpectedReviewerWavePlanSha256", terminalPreview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out); err != nil {
		t.Fatal(err)
	}
	var partial reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &partial); err != nil {
		t.Fatal(err)
	}
	if !partial.Applied || partial.AppliedCount != 0 || partial.FailedIndex != 1 || partial.Failure == "" || partial.RefreshedWave == nil || len(partial.RefreshedWave.Returned) != 1 {
		t.Fatalf("returned partial receipt = %+v", partial)
	}
}

func TestRunReviewerWavePartialFailurePreservesEarlierObservation(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "review", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	wave := reviewerWaveFromStatus(reviewerWaveStatus(t, caseRoot))
	observationPath := filepath.Join(caseRoot, "workspace", "wave-partial.json")
	writeReviewerWaveObservations(t, observationPath, reviewerWaveObservationFile{SchemaVersion: 1, PacketID: wave.PacketID, Observations: []reviewerWaveObservation{
		{ShardID: wave.SpawnWave[0].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-session-ok"},
		{ShardID: wave.SpawnWave[1].ShardID, Kind: "accepted", ReviewerHarness: "go-test-harness", ReviewerSession: "wave-session-late"},
	}})
	args := []string{"-Command", "run-reviewer-wave", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", wave.PacketPath, "-Lane", wave.TargetLane, "-Actor", "mission-commander", "-ReviewerWaveObservationsPath", observationPath, "-Format", "json"}
	preview := reviewerWavePreview(t, append(args, "-WhatIf")...)
	reviewerWaveBeforeApplyObservationHook = func(index int) error {
		if index == 2 {
			return os.ErrPermission
		}
		return nil
	}
	defer func() { reviewerWaveBeforeApplyObservationHook = nil }()
	out.Reset()
	err := Run(append(args, "-ExpectedReviewerWavePlanSha256", preview.ExpectedReviewerWavePlanSHA256, "-Apply"), &out)
	if err != nil {
		t.Fatalf("reviewer wave partial receipt was not returned: %v\n%s", err, out.String())
	}
	var partial reviewerWavePlan
	if decodeErr := json.Unmarshal(out.Bytes(), &partial); decodeErr != nil {
		t.Fatal(decodeErr)
	}
	if !partial.Applied || partial.AppliedCount != 1 || partial.FailedIndex != 2 || partial.Failure == "" || partial.RefreshedWave == nil || partial.RefreshedWave.ActiveSlots != 1 || len(partial.RefreshedWave.Active) != 1 || partial.RefreshedWave.Active[0].ShardID != wave.SpawnWave[0].ShardID {
		t.Fatalf("reviewer wave partial receipt = %+v", partial)
	}
}

func reviewerWaveStatus(t *testing.T, caseRoot string) statusInventory {
	t.Helper()
	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	return status
}

func reviewerWavePreview(t *testing.T, args ...string) reviewerWavePlan {
	t.Helper()
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var plan reviewerWavePlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	return plan
}

func writeReviewerWaveObservations(t *testing.T, path string, value reviewerWaveObservationFile) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
