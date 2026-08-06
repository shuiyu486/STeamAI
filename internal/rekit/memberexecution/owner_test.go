package memberexecution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/onboarding"
)

func TestDispatchObservationManifestAndReplay(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	requestHash := strings.Repeat("a", 64)
	dispatch, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: requestHash, CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := Apply(dispatch, dispatch.ExpectedPlanSHA256)
	if err != nil || !result.Applied {
		t.Fatalf("dispatch apply=%+v err=%v", result, err)
	}
	replay, err := Apply(dispatch, dispatch.ExpectedPlanSHA256)
	if err != nil || !replay.AlreadyApplied {
		t.Fatalf("dispatch replay=%+v err=%v", replay, err)
	}

	accepted, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-03T01:03:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}

	output := []byte("review this member result\n")
	outputPath := filepath.Join(dispatch.Inspection.OutputsRoot, "review-items.json")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := ResultManifest{SchemaVersion: 1, Kind: KindManifest, AttemptID: dispatch.AttemptID, Owner: dispatch.Owner, Summary: "member returned bounded output", Outputs: []Output{{Path: "review-items.json", SHA256: hash(output), Bytes: int64(len(output))}}, ReviewerItemsPath: "review-items.json", NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	manifestBytes, err := MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatch.Inspection.ManifestPath, manifestBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-03T01:04:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := Apply(returned, returned.ExpectedPlanSHA256)
	if err != nil || applied.Inspection.State != "intake-ready" {
		t.Fatalf("returned=%+v err=%v", applied, err)
	}
	if err := os.WriteFile(outputPath, []byte("drift"), 0o600); err != nil {
		t.Fatal(err)
	}
	latest, err := Inspect(caseRoot, "feature-analysis", dispatch.AttemptID)
	if err != nil || latest.State != "intake-ready" {
		t.Fatalf("immutable result snapshot did not survive source drift: latest=%+v err=%v", latest, err)
	}
}

func TestReturnedCanCommitDirectlyFromHandoffReady(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	dispatch, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("f", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	output := []byte("direct returned result\n")
	outputPath := filepath.Join(dispatch.Inspection.OutputsRoot, "result.txt")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := ResultManifest{SchemaVersion: 1, Kind: KindManifest, AttemptID: dispatch.AttemptID, Owner: dispatch.Owner, Summary: "direct returned", Outputs: []Output{{Path: "result.txt", SHA256: hash(output), Bytes: int64(len(output))}}, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	data, err := MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatch.Inspection.ManifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-03T01:04:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := Apply(returned, returned.ExpectedPlanSHA256)
	if err != nil || !applied.Applied || applied.Inspection.State != "intake-ready" || applied.Inspection.Latest == nil || applied.Inspection.Latest.Sequence != 1 {
		t.Fatalf("direct returned=%+v err=%v", applied, err)
	}
}

func TestReturnedPreviewUsesExactPlannedResultSnapshot(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	dispatch, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("e", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	output := []byte("planned returned result\n")
	manifest := ResultManifest{SchemaVersion: 1, Kind: KindManifest, AttemptID: dispatch.AttemptID, Owner: dispatch.Owner, Summary: "planned returned", Outputs: []Output{{Path: "result.txt", SHA256: hash(output), Bytes: int64(len(output))}}, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	manifestData, err := MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	opt := ObservationOptions{
		CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID,
		Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-03T01:04:00Z",
		ResultSnapshot: &ResultSnapshot{
			ManifestPath: dispatch.Inspection.ManifestPath,
			ManifestData: manifestData,
			OutputsRoot:  dispatch.Inspection.OutputsRoot,
			Outputs:      map[string][]byte{"result.txt": output},
		},
	}
	planned, err := PreviewObservation(opt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dispatch.Inspection.ManifestPath); !os.IsNotExist(err) {
		t.Fatalf("planned snapshot preview wrote manifest: %v", err)
	}
	if _, err := Apply(planned, planned.ExpectedPlanSHA256); err == nil {
		t.Fatal("Apply accepted a planned snapshot before its source was durably published")
	}
	if err := os.MkdirAll(dispatch.Inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dispatch.Inspection.OutputsRoot, "result.txt"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatch.Inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	fresh, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-03T01:04:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ExpectedPlanSHA256 != planned.ExpectedPlanSHA256 {
		t.Fatalf("planned snapshot hash=%s filesystem hash=%s", planned.ExpectedPlanSHA256, fresh.ExpectedPlanSHA256)
	}
	applied, err := Apply(planned, planned.ExpectedPlanSHA256)
	if err != nil || !applied.Applied || applied.Inspection.State != "intake-ready" {
		t.Fatalf("applied=%+v err=%v", applied, err)
	}
}

func TestDispatchUsesCommittedNaturalLanguageMissionGoal(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	goal := "用自然语言分析目标，并只输出可复核的结论"
	writeCommittedMissionIntent(t, caseRoot, goal)
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "checkpoints"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "prompts", "RESUME.md"), []byte("# resume\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "checkpoints", "latest.json"), []byte("{\n  \"lane\": \"feature-analysis\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeBoard(t, caseRoot, "executor-a", 1)
	plan, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("3", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	context := plan.Inspection.TaskContext
	if context == nil || context.Goal != goal || context.GoalSource != "committed-mission-intent" || context.MissionIntent == nil || context.MissionIntent.Path != missionintent.MissionIntentRel || context.MissionIntent.SHA256 == "" {
		t.Fatalf("mission task context = %+v", context)
	}
}

func TestDispatchBindsDurableGoalCorrectionAndRejectsContextDrift(t *testing.T) {
	caseRoot := memberCase(t, "executor-b", 2)
	writeBoardWithCorrection(t, caseRoot, "executor-b", 2, "int-human-correction")
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	correctionFacts := strings.Join([]string{
		`{"schemaVersion":1,"eventId":"int-human-correction","kind":"intervention","lane":"feature-analysis","subject":"只输出纠偏后的结论","summary":"不要沿用第一代猜测","target":"analysis.md","status":"open"}`,
		`{"schemaVersion":1,"eventId":"int-human-correction-resolved","kind":"intervention","lane":"feature-analysis","subject":"采用人工纠偏","summary":"第二代必须按人工指示重做","action":"reconcile","status":"resolved","resolvesEventId":"int-human-correction","actor":"operator","executor":"executor-b","reason":"human correction accepted","time":"2026-08-03T01:01:00Z"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(factsRoot, "interventions.jsonl"), []byte(correctionFacts), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("2", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	context := plan.Inspection.TaskContext
	if context == nil || context.GoalSource != "lane-resume-fallback" || context.Correction == nil || context.Correction.SourceEventID != "int-human-correction" || context.Correction.ResolutionReason != "human correction accepted" || context.Resume.Content == "" || context.Checkpoint.Content == "" {
		t.Fatalf("task context = %+v", context)
	}
	resumePath := filepath.Join(caseRoot, filepath.FromSlash(context.Resume.Path))
	if err := os.WriteFile(resumePath, []byte("drifted resume\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("task context drift error = %v", err)
	}
}

func TestAcceptedObservationRejectsTaskContextDrift(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	dispatch, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("5", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	checkpointPath := filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "checkpoints", "latest.json")
	if err := os.WriteFile(checkpointPath, []byte("{\n  \"lane\": \"feature-analysis\",\n  \"drifted\": true\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-03T01:03:00Z"}); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("task context drift observation error=%v", err)
	}
}

func TestReturnedAttemptRemainsInspectableAfterCheckpointRefresh(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	dispatch, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("4", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	output := []byte("stable returned result\n")
	if err := os.MkdirAll(dispatch.Inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dispatch.Inspection.OutputsRoot, "result.txt"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := ResultManifest{SchemaVersion: SchemaVersion, Kind: KindManifest, AttemptID: dispatch.AttemptID, Owner: dispatch.Owner, Summary: "stable returned result", Outputs: []Output{{Path: "result.txt", SHA256: hash(output), Bytes: int64(len(output))}}, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	manifestData, err := MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatch.Inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-03T01:03:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(returned, returned.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	checkpointPath := filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "checkpoints", "latest.json")
	if err := os.WriteFile(checkpointPath, []byte("{\n  \"lane\": \"feature-analysis\",\n  \"refreshed\": true\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	inspection, err := Inspect(caseRoot, "feature-analysis", dispatch.AttemptID)
	if err != nil || inspection.State != "intake-ready" || inspection.TaskContext == nil || inspection.Manifest == nil {
		t.Fatalf("historical returned attempt inspection=%+v err=%v", inspection, err)
	}
}

func TestCurrentOwnerMatchesAfterExecutorTakeover(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	owner := Owner{Lane: "feature-analysis", Executor: "executor-a", ExecutorGeneration: 1}
	current, err := CurrentOwnerMatches(caseRoot, "_template", owner)
	if err != nil || !current {
		t.Fatalf("initial owner current=%t err=%v", current, err)
	}
	writeBoard(t, caseRoot, "executor-b", 2)
	current, err = CurrentOwnerMatches(caseRoot, "_template", owner)
	if err != nil || current {
		t.Fatalf("replaced owner current=%t err=%v", current, err)
	}
}

func TestFailedRetryAndGenerationFence(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	first, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("b", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(first, first.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	failed, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: first.AttemptID, Outcome: "failed", Actor: "harness", Reason: "external member failed", ObservedAt: "2026-08-03T01:03:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(failed, failed.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	retry, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("b", 64), CreatedAt: "2026-08-03T01:04:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if retry.AttemptID == first.AttemptID {
		t.Fatal("failed attempt did not advance retry sequence")
	}
	writeBoard(t, caseRoot, "executor-b", 2)
	if _, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: first.AttemptID, Outcome: "accepted", Actor: "late", ObservedAt: "2026-08-03T01:05:00Z"}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("late generation observation error=%v", err)
	}
}

func TestApplyLeaseRebuildRejectsGenerationRaceAndSerializesFinalObservation(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	t.Cleanup(func() { applyLeaseHook = nil })
	dispatch, _ := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("8", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	applyLeaseHook = func(plan Plan) error {
		applyLeaseHook = nil
		writeBoard(t, caseRoot, "executor-b", 2)
		return nil
	}
	if _, err := Apply(dispatch, dispatch.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("generation race was not rejected: %v", err)
	}
	writeBoard(t, caseRoot, "executor-a", 1)
	dispatch, _ = PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("8", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if _, err := Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	accepted, _ := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-03T01:03:00Z"})
	if _, err := Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	failed, _ := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "failed", Actor: "harness", ObservedAt: "2026-08-03T01:04:00Z"})
	returned := returnedObservationPlan(t, caseRoot, dispatch, "2026-08-03T01:04:01Z")
	start := make(chan struct{})
	errs := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for _, plan := range []Plan{failed, returned} {
		go func() {
			ready.Done()
			<-start
			_, err := Apply(plan, plan.ExpectedPlanSHA256)
			errs <- err
		}()
	}
	ready.Wait()
	close(start)
	successes := 0
	for range 2 {
		if err := <-errs; err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("final observations successes=%d want=1", successes)
	}
	inspection, err := Inspect(caseRoot, "feature-analysis", dispatch.AttemptID)
	if err != nil || inspection.Latest == nil || inspection.Latest.Sequence != 2 || (inspection.State != "failed" && inspection.State != "intake-ready") {
		t.Fatalf("serialized final chain invalid: %+v err=%v", inspection, err)
	}
}

func returnedObservationPlan(t *testing.T, caseRoot string, dispatch Plan, observedAt string) Plan {
	t.Helper()
	output := []byte("immutable result\n")
	if err := os.MkdirAll(dispatch.Inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dispatch.Inspection.OutputsRoot, "result.txt"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := ResultManifest{SchemaVersion: 1, Kind: KindManifest, AttemptID: dispatch.AttemptID, Owner: dispatch.Owner, Summary: "bounded", Outputs: []Output{{Path: "result.txt", SHA256: hash(output), Bytes: int64(len(output))}}, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	data, err := MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatch.Inspection.ManifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "returned", Actor: "harness", ObservedAt: observedAt})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestReturnedSnapshotRecoversOnlyExactPublicationPrefixes(t *testing.T) {
	for _, prefix := range []int{1, 2} {
		t.Run(fmt.Sprintf("prefix-%d", prefix), func(t *testing.T) {
			caseRoot := memberCase(t, "executor-a", 1)
			dispatch, _ := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("7", 64), CreatedAt: "2026-08-03T01:02:03Z"})
			if _, err := Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
				t.Fatal(err)
			}
			accepted, _ := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-03T01:03:00Z"})
			if _, err := Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
				t.Fatal(err)
			}
			returned := returnedObservationPlan(t, caseRoot, dispatch, "2026-08-03T01:04:00Z")
			for index := range prefix {
				if err := os.MkdirAll(filepath.Dir(returned.writes[index].path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(returned.writes[index].path, returned.writes[index].data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			result, err := Apply(returned, returned.ExpectedPlanSHA256)
			if err != nil || !result.Applied || result.Inspection.State != "intake-ready" {
				t.Fatalf("returned prefix recovery=%+v err=%v", result, err)
			}
			replay, err := Apply(returned, returned.ExpectedPlanSHA256)
			if err != nil || !replay.AlreadyApplied {
				t.Fatalf("returned replay=%+v err=%v", replay, err)
			}
		})
	}

	t.Run("observation-without-evidence", func(t *testing.T) {
		caseRoot := memberCase(t, "executor-a", 1)
		dispatch, _ := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("6", 64), CreatedAt: "2026-08-03T01:02:03Z"})
		if _, err := Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
			t.Fatal(err)
		}
		accepted, _ := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-03T01:03:00Z"})
		if _, err := Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
			t.Fatal(err)
		}
		returned := returnedObservationPlan(t, caseRoot, dispatch, "2026-08-03T01:04:00Z")
		last := returned.writes[len(returned.writes)-1]
		if err := os.MkdirAll(filepath.Dir(last.path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(last.path, last.data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Apply(returned, returned.ExpectedPlanSHA256); err == nil || (!strings.Contains(err.Error(), "non-prefix") && !strings.Contains(err.Error(), "evidence")) {
			t.Fatalf("observation-only result publication error=%v", err)
		}
	})
}

func TestObservationRequiresDurableObservedAt(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	plan, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("9", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: plan.AttemptID, Outcome: "accepted", Actor: "harness"}); err == nil || !strings.Contains(err.Error(), "requires observedAt") {
		t.Fatalf("missing observedAt error=%v", err)
	}
}

func TestDispatchRecoversExactPrefixAndRejectsNonPrefix(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	plan, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("d", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(plan.writes[0].path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.writes[0].path, plan.writes[0].data, 0o600); err != nil {
		t.Fatal(err)
	}
	recovered, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("d", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if recovered.ExpectedPlanSHA256 != plan.ExpectedPlanSHA256 || recovered.AttemptID != plan.AttemptID {
		t.Fatalf("public preview did not reconstruct exact pending dispatch: recovered=%+v original=%+v", recovered, plan)
	}
	result, err := Apply(recovered, recovered.ExpectedPlanSHA256)
	if err != nil || !result.Applied || result.Inspection.State != "handoff-ready" {
		t.Fatalf("prefix recovery=%+v err=%v", result, err)
	}

	otherRoot := memberCase(t, "executor-a", 1)
	other, err := PreviewDispatch(DispatchOptions{CaseRoot: otherRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("e", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(other.writes[2].path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(other.writes[2].path, other.writes[2].data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(other, other.ExpectedPlanSHA256); err == nil || (!strings.Contains(err.Error(), "non-prefix") && !strings.Contains(err.Error(), "missing durable intent")) {
		t.Fatalf("non-prefix publication error=%v", err)
	}
	if _, err := os.Lstat(other.writes[0].path); !os.IsNotExist(err) {
		t.Fatalf("non-prefix Apply wrote intent: %v", err)
	}
}

func TestLatestClassifiesOnlyExactDispatchPrefixesAsPending(t *testing.T) {
	for _, prefix := range []int{1, 2} {
		t.Run(fmt.Sprintf("prefix-%d", prefix), func(t *testing.T) {
			caseRoot := memberCase(t, "executor-a", 1)
			plan, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("f", 64), CreatedAt: "2026-08-03T01:02:03Z"})
			if err != nil {
				t.Fatal(err)
			}
			for index := range prefix {
				if err := os.MkdirAll(filepath.Dir(plan.writes[index].path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(plan.writes[index].path, plan.writes[index].data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, ok, err := Latest(caseRoot, "feature-analysis"); ok || !IsPendingDispatch(err) {
				t.Fatalf("exact dispatch prefix was not typed pending: ok=%v err=%v", ok, err)
			}
		})
	}

	for _, test := range []struct {
		name      string
		published []int
		remove    int
		extra     string
		mutate    func(*testing.T, Plan)
	}{
		{name: "commit-without-handoff", published: []int{0, 2}},
		{name: "committed-missing-handoff", published: []int{0, 1, 2}, remove: 1},
		{name: "committed-missing-intent", published: []int{0, 1, 2}, remove: 0},
		{name: "intent-with-result-artifact", published: []int{0}, extra: "result"},
		{name: "intent-with-observation-artifact", published: []int{0}, extra: "observations"},
		{name: "handoff-next-steps-drift", published: []int{0, 1}, mutate: func(t *testing.T, plan Plan) {
			var handoff Handoff
			if err := json.Unmarshal(plan.writes[1].data, &handoff); err != nil {
				t.Fatal(err)
			}
			handoff.NextSteps = []string{"tampered instruction"}
			data, err := canonical(handoff)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(plan.writes[1].path, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "handoff-boundary-drift", published: []int{0, 1}, mutate: func(t *testing.T, plan Plan) {
			var handoff Handoff
			if err := json.Unmarshal(plan.writes[1].data, &handoff); err != nil {
				t.Fatal(err)
			}
			handoff.Boundary = []string{"tampered boundary"}
			data, err := canonical(handoff)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(plan.writes[1].path, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "intent-created-at-drift", published: []int{0}, mutate: func(t *testing.T, plan Plan) {
			var intent Intent
			if err := json.Unmarshal(plan.writes[0].data, &intent); err != nil {
				t.Fatal(err)
			}
			intent.CreatedAt = "not-a-time"
			data, err := canonical(intent)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(plan.writes[0].path, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := memberCase(t, "executor-a", 1)
			plan, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("e", 64), CreatedAt: "2026-08-03T01:02:03Z"})
			if err != nil {
				t.Fatal(err)
			}
			for _, index := range test.published {
				if err := os.MkdirAll(filepath.Dir(plan.writes[index].path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(plan.writes[index].path, plan.writes[index].data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if len(test.published) == 3 && test.remove >= 0 {
				if err := os.Remove(plan.writes[test.remove].path); err != nil {
					t.Fatal(err)
				}
			}
			if test.extra != "" {
				if err := os.MkdirAll(filepath.Join(plan.Inspection.AttemptRoot, test.extra), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if test.mutate != nil {
				test.mutate(t, plan)
			}
			if _, ok, err := Latest(caseRoot, "feature-analysis"); err == nil || ok || IsPendingDispatch(err) {
				t.Fatalf("corrupt dispatch was hidden as pending: ok=%v err=%v", ok, err)
			}
		})
	}
}

func TestRejectsManifestPathAndUnknownField(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	plan, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("c", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	accepted, _ := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: plan.AttemptID, Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-03T01:03:00Z"})
	if _, err := Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(plan.Inspection.ManifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := `{"schemaVersion":1,"kind":"member-lane-execution-result-manifest","unknown":true}`
	if err := os.WriteFile(plan.Inspection.ManifestPath, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: plan.AttemptID, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-03T01:04:00Z"}); err == nil {
		t.Fatal("unknown manifest field accepted")
	}
}

func TestReturnedApplyRejectsManifestDriftAfterPreview(t *testing.T) {
	caseRoot := memberCase(t, "executor-a", 1)
	plan, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("1", 64), CreatedAt: "2026-08-03T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	accepted, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: plan.AttemptID, Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-03T01:03:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plan.Inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	output := []byte("stable")
	if err := os.WriteFile(filepath.Join(plan.Inspection.OutputsRoot, "result.txt"), output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := ResultManifest{SchemaVersion: 1, Kind: KindManifest, AttemptID: plan.AttemptID, Owner: plan.Owner, Summary: "stable", Outputs: []Output{{Path: "result.txt", SHA256: hash(output), Bytes: int64(len(output))}}, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
	data, _ := MarshalResultManifest(manifest)
	if err := os.WriteFile(plan.Inspection.ManifestPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: plan.AttemptID, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-03T01:04:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	manifest.Summary = "drifted"
	drifted, _ := MarshalResultManifest(manifest)
	if err := os.WriteFile(plan.Inspection.ManifestPath, drifted, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(returned, returned.ExpectedPlanSHA256); err == nil || (!strings.Contains(err.Error(), "changed after preview") && !strings.Contains(err.Error(), "plan changed")) {
		t.Fatalf("returned Apply manifest drift error=%v", err)
	}
}

func TestRejectsManifestTraversalDuplicateAndOutputSymlink(t *testing.T) {
	for _, test := range []struct {
		name    string
		outputs []Output
		prepare func(t *testing.T, root string)
		want    string
	}{
		{name: "traversal", outputs: []Output{{Path: "../escape", SHA256: strings.Repeat("a", 64), Bytes: 1}}, want: "output contract"},
		{name: "case-insensitive-duplicate", outputs: []Output{{Path: "A.txt", SHA256: hash([]byte("a")), Bytes: 1}, {Path: "a.txt", SHA256: hash([]byte("a")), Bytes: 1}}, prepare: func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "A.txt"), []byte("a"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, want: "output contract"},
		{name: "symlink", outputs: []Output{{Path: "linked.txt", SHA256: hash([]byte("a")), Bytes: 1}}, prepare: func(t *testing.T, root string) {
			target := filepath.Join(t.TempDir(), "target.txt")
			if err := os.WriteFile(target, []byte("a"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(root, "linked.txt")); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
		}, want: "bounded regular file"},
		{name: "undeclared-extra", outputs: []Output{{Path: "declared.txt", SHA256: hash([]byte("a")), Bytes: 1}}, prepare: func(t *testing.T, root string) {
			for _, name := range []string{"declared.txt", "extra.txt"} {
				if err := os.WriteFile(filepath.Join(root, name), []byte("a"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		}, want: "exactly match manifest"},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := memberCase(t, "executor-a", 1)
			plan, err := PreviewDispatch(DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("f", 64), CreatedAt: "2026-08-03T01:02:03Z"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Apply(plan, plan.ExpectedPlanSHA256); err != nil {
				t.Fatal(err)
			}
			accepted, err := PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: plan.AttemptID, Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-03T01:03:00Z"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(plan.Inspection.OutputsRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			if test.prepare != nil {
				test.prepare(t, plan.Inspection.OutputsRoot)
			}
			manifest := ResultManifest{SchemaVersion: 1, Kind: KindManifest, AttemptID: plan.AttemptID, Owner: plan.Owner, Summary: "bounded", Outputs: test.outputs, NoAuthority: true, NoConfirmed: true, NoHeavyTool: true}
			data, err := MarshalResultManifest(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(plan.Inspection.ManifestPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err = PreviewObservation(ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: plan.AttemptID, Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-03T01:04:00Z"})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want %q", err, test.want)
			}
		})
	}
}

func memberCase(t *testing.T, executor string, generation int) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".rekit", "lanes", "feature-analysis"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rekit", "instance.yml"), []byte("schemaVersion: 1\ntemplateRoot: test\ntemplatePack: _template\nprojectRoot: "+root+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".rekit", "lanes", "feature-analysis", "lane.json"), []byte("{\"id\":\"feature-analysis\",\"status\":\"active\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for path, data := range map[string][]byte{
		filepath.Join(root, ".rekit", "lanes", "feature-analysis", "prompts", "RESUME.md"):       []byte("# feature-analysis\n\nContinue the durable lane task.\n"),
		filepath.Join(root, ".rekit", "lanes", "feature-analysis", "checkpoints", "latest.json"): []byte("{\n  \"schemaVersion\": 1,\n  \"lane\": \"feature-analysis\",\n  \"status\": \"active\"\n}\n"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeBoard(t, root, executor, generation)
	return root
}

func writeCommittedMissionIntent(t *testing.T, root, goal string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve kit root")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	opt := onboarding.Options{Target: root, Pack: "_template", ProjectName: "demo", Goal: goal, Actor: "operator", Executor: "executor-a", InitialLane: "feature-analysis", PublicationStamp: "20260803-010203004"}
	preview, err := onboarding.Preview(repoRoot, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	if _, err := onboarding.Apply(repoRoot, opt); err != nil {
		t.Fatal(err)
	}
}

func writeBoard(t *testing.T, root, executor string, generation int) {
	t.Helper()
	writeBoardValue(t, root, missionBoardFixture(root, executor, generation))
}

func writeBoardWithCorrection(t *testing.T, root, executor string, generation int, interventionID string) {
	t.Helper()
	board := missionBoardFixture(root, executor, generation)
	lanes := board["lanes"].([]map[string]any)
	lanes[0]["lastReconciledIntervention"] = interventionID
	lanes[0]["lastReconcileAt"] = "2026-08-03T01:01:00Z"
	writeBoardValue(t, root, board)
}

func writeBoardValue(t *testing.T, root string, board map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".rekit", "board.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}

func missionBoardFixture(root, executor string, generation int) map[string]any {
	return map[string]any{"schemaVersion": 1, "caseRoot": root, "repoRoot": filepath.Dir(root), "pack": "_template", "automationMode": "review-first", "defaultAuthorityLane": "main", "lanes": []map[string]any{{"id": "feature-analysis", "type": "feature", "title": "analysis", "status": "active", "authority": false, "workspace": ".rekit/lanes/feature-analysis/workspace", "currentExecutor": executor, "executorGeneration": generation, "updatedAt": "2026-08-03T01:00:00Z"}}, "factsRoot": ".rekit/facts", "updatedAt": "2026-08-03T01:00:00Z"}
}
