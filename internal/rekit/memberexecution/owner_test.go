package memberexecution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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
	writeBoard(t, root, executor, generation)
	return root
}

func writeBoard(t *testing.T, root, executor string, generation int) {
	t.Helper()
	board := missionBoardFixture(root, executor, generation)
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
