package memberexecution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotReviewerEvidenceClosureBindsExactCurrentClosure(t *testing.T) {
	caseRoot, plan, item := reviewerEvidenceFixture(t, []byte("bounded reviewer evidence\n"))
	closure, err := SnapshotReviewerEvidenceClosure(caseRoot, "_template", item)
	if err != nil {
		t.Fatal(err)
	}
	if closure.Item != item || closure.AttemptID != plan.AttemptID || closure.Owner != plan.Owner || len(closure.Artifacts) != 3 {
		t.Fatalf("closure=%+v", closure)
	}
	roles := []string{"member-task-context", "member-evidence-manifest", "member-evidence-output"}
	for index, artifact := range closure.Artifacts {
		if artifact.Role != roles[index] || artifact.Path == "" || artifact.SHA256 == "" || artifact.Bytes != int64(len([]byte(artifact.Content))) {
			t.Fatalf("artifact[%d]=%+v", index, artifact)
		}
	}
	if _, err := SnapshotReviewerEvidenceClosure(caseRoot, "_template", "./"+item); err == nil || !strings.Contains(err.Error(), "exact canonical path") {
		t.Fatalf("non-canonical item error=%v", err)
	}

	writeBoard(t, caseRoot, "executor-b", 2)
	if _, err := SnapshotReviewerEvidenceClosure(caseRoot, "_template", item); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale owner error=%v", err)
	}
}

func TestSnapshotReviewerEvidenceClosureRejectsBinaryAndDrift(t *testing.T) {
	caseRoot, _, item := reviewerEvidenceFixture(t, []byte{0, 1, 2})
	if _, err := SnapshotReviewerEvidenceClosure(caseRoot, "_template", item); err == nil || !strings.Contains(err.Error(), "UTF-8 text") {
		t.Fatalf("binary evidence error=%v", err)
	}

	caseRoot, plan, item := reviewerEvidenceFixture(t, []byte("bounded reviewer evidence\n"))
	inspection, err := Inspect(caseRoot, "feature-analysis", plan.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inspection.OutputsRoot, "review-items.json"), []byte("drifted\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SnapshotReviewerEvidenceClosure(caseRoot, "_template", item); err == nil {
		t.Fatal("drifted evidence output should fail closed")
	}
}

func reviewerEvidenceFixture(t *testing.T, output []byte) (string, Plan, string) {
	t.Helper()
	caseRoot := memberCase(t, "executor-a", 1)
	plan, err := PreviewDispatch(DispatchOptions{
		CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis",
		RequestSHA256: strings.Repeat("a", 64), CreatedAt: "2026-08-11T01:02:03Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(plan, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	accepted, err := PreviewObservation(ObservationOptions{
		CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: plan.AttemptID,
		Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-11T01:03:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plan.Inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(plan.Inspection.OutputsRoot, "review-items.json")
	if err := os.WriteFile(outputPath, output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := ResultManifest{
		SchemaVersion: SchemaVersion, Kind: KindManifest, AttemptID: plan.AttemptID, Owner: plan.Owner,
		Summary: "bounded reviewer evidence", Outputs: []Output{{Path: "review-items.json", SHA256: hash(output), Bytes: int64(len(output))}},
		ReviewerItemsPath: "review-items.json", NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	manifestData, err := MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.Inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := PreviewObservation(ObservationOptions{
		CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: plan.AttemptID,
		Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-11T01:04:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := Apply(returned, returned.ExpectedPlanSHA256)
	if err != nil || applied.Inspection.State != "intake-ready" {
		t.Fatalf("returned=%+v err=%v", applied, err)
	}
	item, err := filepath.Rel(caseRoot, applied.Inspection.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	return caseRoot, plan, filepath.ToSlash(item)
}
