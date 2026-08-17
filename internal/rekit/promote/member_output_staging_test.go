package promote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestMemberOutputStagingRootUsesSingleStateRoot(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  string
	}{
		{name: "current", dir: projectstate.CurrentDir},
		{name: "legacy", dir: projectstate.LegacyDir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, tc.dir), 0o700); err != nil {
				t.Fatal(err)
			}
			root, err := memberOutputStagingRoot(caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(caseRoot, tc.dir, "pack-memory-staging"); root != want {
				t.Fatalf("staging root = %q, want %q", root, want)
			}
		})
	}

	caseRoot := t.TempDir()
	for _, dir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if err := os.Mkdir(filepath.Join(caseRoot, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := memberOutputStagingRoot(caseRoot); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("dual-root staging error = %v", err)
	}
}

func TestStageMemberOutputWhatIfApplyAndReplay(t *testing.T) {
	repoRoot, caseRoot, targetRel, dispatch := memberOutputStagingFixture(t, "# Reusable checklist\n\n1. Confirm bounded input.\n2. Record the reusable decision.\n")
	targetPath := filepath.Join(caseRoot, filepath.FromSlash(targetRel))
	before := readText(t, targetPath)

	preview, err := StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{
		Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "pack-memory.md", ManagedTargetPath: targetRel, WhatIf: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Mode != "preview-review-pending" || preview.IsMutation || preview.Applied || !preview.ReviewPending || !preview.RequiresReview || preview.PlanSHA256 == "" || preview.IntentSHA256 == "" || preview.TargetBeforeSHA256 == "" || preview.SanitizedSHA256 == "" || preview.ApplyCommand == "" {
		t.Fatalf("unexpected staging preview: %+v", preview)
	}
	if got := readText(t, targetPath); got != before {
		t.Fatalf("WhatIf changed managed target: got %q want %q", got, before)
	}
	if _, err := os.Stat(preview.IntentPath); !os.IsNotExist(err) {
		t.Fatalf("WhatIf wrote intent: %v", err)
	}

	applied, err := StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{
		Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "pack-memory.md", ManagedTargetPath: targetRel, ExpectedPlanSHA256: preview.PlanSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied.Mode != "staged-review-pending" || !applied.IsMutation || !applied.Applied || applied.Replay || !applied.ReviewPending || !applied.RequiresReview || applied.ReceiptSHA256 == "" || applied.PlanSHA256 != preview.PlanSHA256 {
		t.Fatalf("unexpected staging apply: %+v", applied)
	}
	staged := readText(t, targetPath)
	if !strings.Contains(staged, "# Reusable checklist") || memberOutputStagingSHA([]byte(staged)) != applied.SanitizedSHA256 {
		t.Fatalf("managed target did not receive exact sanitized bytes: %q", staged)
	}
	for _, artifact := range []string{applied.IntentPath, applied.ReceiptPath} {
		if info, err := os.Lstat(artifact); err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("staging artifact is not a regular file: %s: %v", artifact, err)
		}
	}
	assertMemberOutputStagingNoAuthority(t, applied.IntentPath, applied.ReceiptPath)

	replay, err := StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{
		Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "pack-memory.md", ManagedTargetPath: targetRel, ExpectedPlanSHA256: preview.PlanSHA256,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replay || replay.PlanSHA256 != preview.PlanSHA256 || replay.ReceiptSHA256 != applied.ReceiptSHA256 {
		t.Fatalf("unexpected staging replay: %+v", replay)
	}
}

func TestStageMemberOutputFailsClosedOnDriftAndScope(t *testing.T) {
	t.Run("target predecessor drift", func(t *testing.T) {
		repoRoot, caseRoot, targetRel, dispatch := memberOutputStagingFixture(t, "# Reusable checklist\n")
		preview, err := StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "pack-memory.md", ManagedTargetPath: targetRel, WhatIf: true})
		if err != nil {
			t.Fatal(err)
		}
		writeText(t, filepath.Join(caseRoot, filepath.FromSlash(targetRel)), "operator drift\n")
		_, err = StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "pack-memory.md", ManagedTargetPath: targetRel, ExpectedPlanSHA256: preview.PlanSHA256})
		if err == nil || !strings.Contains(err.Error(), "differs from both") {
			t.Fatalf("target drift error = %v", err)
		}
	})

	t.Run("conflicting receipt does not mutate target", func(t *testing.T) {
		repoRoot, caseRoot, targetRel, dispatch := memberOutputStagingFixture(t, "# Reusable checklist\n")
		targetPath := filepath.Join(caseRoot, filepath.FromSlash(targetRel))
		before := readText(t, targetPath)
		preview, err := StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "pack-memory.md", ManagedTargetPath: targetRel, WhatIf: true})
		if err != nil {
			t.Fatal(err)
		}
		writeText(t, preview.ReceiptPath, "{\"conflict\":true}\n")
		_, err = StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "pack-memory.md", ManagedTargetPath: targetRel, ExpectedPlanSHA256: preview.PlanSHA256})
		if err == nil || !strings.Contains(err.Error(), "different bindings") {
			t.Fatalf("receipt conflict error = %v", err)
		}
		if got := readText(t, targetPath); got != before {
			t.Fatalf("failed Apply mutated target: got %q want %q", got, before)
		}
		if _, err := os.Stat(preview.IntentPath); !os.IsNotExist(err) {
			t.Fatalf("failed Apply published intent: %v", err)
		}
	})

	t.Run("output not declared", func(t *testing.T) {
		repoRoot, caseRoot, targetRel, dispatch := memberOutputStagingFixture(t, "# Reusable checklist\n")
		_, err := StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "other.md", ManagedTargetPath: targetRel, WhatIf: true})
		if err == nil || !strings.Contains(err.Error(), "not declared") {
			t.Fatalf("undeclared output error = %v", err)
		}
	})

	t.Run("target outside promote allowlist", func(t *testing.T) {
		repoRoot, caseRoot, _, dispatch := memberOutputStagingFixture(t, "# Reusable checklist\n")
		_, err := StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "pack-memory.md", ManagedTargetPath: ".rekit/board.json", WhatIf: true})
		if err == nil || !strings.Contains(err.Error(), "declared by both") {
			t.Fatalf("target allowlist error = %v", err)
		}
	})

	t.Run("expected hash mismatch", func(t *testing.T) {
		repoRoot, caseRoot, targetRel, dispatch := memberOutputStagingFixture(t, "# Reusable checklist\n")
		_, err := StageMemberOutput(repoRoot, caseRoot, "_template", MemberOutputStagingOptions{Lane: "feature-analysis", AttemptID: dispatch.AttemptID, OutputPath: "pack-memory.md", ManagedTargetPath: targetRel, ExpectedPlanSHA256: strings.Repeat("0", 64)})
		if err == nil || !strings.Contains(err.Error(), "changed after preview") {
			t.Fatalf("expected hash error = %v", err)
		}
	})
}

func memberOutputStagingFixture(t *testing.T, outputText string) (repoRoot, caseRoot, targetRel string, dispatch memberexecution.Plan) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repository root")
	}
	repoRoot = filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	m, err := manifest.Load(repoRoot, "_template")
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range m.PromoteFiles {
		if slices.Contains(m.ManagedFiles, candidate) {
			targetRel = candidate
			break
		}
	}
	if targetRel == "" {
		t.Fatal("_template has no managed promote target")
	}
	caseRoot = filepath.Join(t.TempDir(), "producer-case")
	writeText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), "schemaVersion: 1\ntemplateRoot: "+repoRoot+"\ntemplatePack: _template\nprojectName: producer\nprojectRoot: "+caseRoot+"\n")
	packTarget, err := m.SourcePath(targetRel)
	if err != nil {
		t.Fatal(err)
	}
	packBytes, err := os.ReadFile(packTarget)
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(caseRoot, filepath.FromSlash(targetRel)), string(packBytes))
	writeText(t, filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "lane.json"), "{\"id\":\"feature-analysis\",\"status\":\"active\"}\n")
	writeText(t, filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "prompts", "RESUME.md"), "# feature-analysis\n\nProduce one reusable checklist.\n")
	writeText(t, filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "checkpoints", "latest.json"), "{\n  \"schemaVersion\": 1,\n  \"lane\": \"feature-analysis\",\n  \"status\": \"active\"\n}\n")
	board := map[string]any{
		"schemaVersion": 1, "caseRoot": caseRoot, "repoRoot": repoRoot, "pack": "_template", "automationMode": "review-first", "defaultAuthorityLane": "main",
		"lanes":     []map[string]any{{"id": "feature-analysis", "type": "feature", "title": "analysis", "status": "active", "authority": false, "workspace": ".rekit/lanes/feature-analysis/workspace", "currentExecutor": "executor-a", "executorGeneration": 1, "updatedAt": "2026-08-08T01:00:00Z"}},
		"factsRoot": ".rekit/facts", "updatedAt": "2026-08-08T01:00:00Z",
	}
	boardBytes, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeText(t, filepath.Join(caseRoot, ".rekit", "board.json"), string(append(boardBytes, '\n')))

	dispatch, err = memberexecution.PreviewDispatch(memberexecution.DispatchOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", RequestSHA256: strings.Repeat("a", 64), CreatedAt: "2026-08-08T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(dispatch, dispatch.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	output := []byte(outputText)
	outputPath := filepath.Join(dispatch.Inspection.OutputsRoot, "pack-memory.md")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, output, 0o600); err != nil {
		t.Fatal(err)
	}
	resultManifest := memberexecution.ResultManifest{
		SchemaVersion: 1, Kind: memberexecution.KindManifest, AttemptID: dispatch.AttemptID, Owner: dispatch.Owner,
		Summary: "bounded reusable checklist fixture", Outputs: []memberexecution.Output{{Path: "pack-memory.md", SHA256: memberOutputStagingSHA(output), Bytes: int64(len(output))}},
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	manifestBytes, err := memberexecution.MarshalResultManifest(resultManifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dispatch.Inspection.ManifestPath, manifestBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{CaseRoot: caseRoot, Pack: "_template", Lane: "feature-analysis", AttemptID: dispatch.AttemptID, Outcome: "returned", Actor: "fixture-harness", ObservedAt: "2026-08-08T01:04:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := memberexecution.Apply(returned, returned.ExpectedPlanSHA256)
	if err != nil || applied.Inspection.State != "intake-ready" {
		t.Fatalf("returned member result = %+v err=%v", applied, err)
	}
	return repoRoot, caseRoot, targetRel, dispatch
}

func assertMemberOutputStagingNoAuthority(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		if value["kind"] == "pack-memory-member-output-staging-receipt" {
			if value["noAuthority"] != true || value["noConfirmed"] != true || value["noHeavyTool"] != true || value["reviewPending"] != true {
				t.Fatalf("receipt granted authority: %s", data)
			}
		}
		if strings.Contains(string(data), `"noAuthority": false`) || strings.Contains(string(data), `"reviewPending": false`) {
			t.Fatalf("staging artifact weakened review boundary: %s", data)
		}
	}
}
