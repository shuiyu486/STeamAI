package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
)

func TestParsePowerShellStyleOptions(t *testing.T) {
	opt, err := Parse([]string{"doctor", "-Pack", "_template", "-Target", "."})
	if err != nil {
		t.Fatal(err)
	}
	if opt.Command != "doctor" {
		t.Fatalf("Command = %q, want doctor", opt.Command)
	}
	if opt.Pack != "_template" {
		t.Fatalf("Pack = %q, want _template", opt.Pack)
	}
	if opt.Target != "." {
		t.Fatalf("Target = %q, want .", opt.Target)
	}
}

func TestParseIgnoresGoRunSeparator(t *testing.T) {
	opt, err := Parse([]string{"--", "-Command", "doctor", "-Pack", "_template"})
	if err != nil {
		t.Fatal(err)
	}
	if opt.Command != "doctor" || opt.Pack != "_template" {
		t.Fatalf("unexpected options after -- separator: %+v", opt)
	}
}

func TestParseDefaults(t *testing.T) {
	opt, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if opt.Command != "status" {
		t.Fatalf("Command = %q, want status", opt.Command)
	}
	if opt.Pack != "vmp-re" {
		t.Fatalf("Pack = %q, want vmp-re", opt.Pack)
	}
}

func TestRunDoctorRejectsNonCaseTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "missing-case")
	var out bytes.Buffer
	err := Run([]string{"-Command", "doctor", "-Target", target}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for non-case target")
	}
	if strings.Contains(out.String(), "pack validation ok") {
		t.Fatalf("doctor reported pack validation for non-case target: %q", out.String())
	}
	if !strings.Contains(err.Error(), "target is neither this kit root nor an attached rekit case") {
		t.Fatalf("error = %q, want non-case target error", err.Error())
	}
}

func TestRunCaseDoctorValidatesAttachedCase(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "doctor", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "instance validation ok") || !strings.Contains(out.String(), ".rekit") {
		t.Fatalf("unexpected case doctor output: %s", out.String())
	}
}

func TestRunCaseDoctorRejectsShimDrift(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeCaseFile(t, caseRoot, ".claude/skills/rekit/SKILL.md", "drift\n")
	var out bytes.Buffer
	err := Run([]string{"-Command", "doctor", "-Target", caseRoot, "-Pack", "_template"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for shim drift")
	}
	if !strings.Contains(err.Error(), "shim differs") {
		t.Fatalf("error = %q, want shim drift", err.Error())
	}
}

func TestRunAttachRequiresExplicitMode(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "attach", "-Target", filepath.Join(t.TempDir(), "case"), "-Pack", "_template"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for attach without -WhatIf or -Apply")
	}
	if !strings.Contains(err.Error(), "-Apply") || !strings.Contains(err.Error(), "-WhatIf") {
		t.Fatalf("error = %q, want apply/whatif guard", err.Error())
	}
}

func TestRunAttachPreviewDoesNotCreateFiles(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "attach", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo-case", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Command              string `json:"command"`
		CaseRoot             string `json:"caseRoot"`
		ProjectName          string `json:"projectName"`
		IsMutation           bool   `json:"isMutation"`
		ReviewRequired       bool   `json:"reviewRequired"`
		RequiresConfirmation bool   `json:"requiresConfirmation"`
		Writes               []struct {
			Path   string `json:"path"`
			Kind   string `json:"kind"`
			Action string `json:"action"`
		} `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("attach preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if plan.Command != "attach" || plan.IsMutation || !plan.ReviewRequired || !plan.RequiresConfirmation {
		t.Fatalf("unexpected attach preview flags: %+v", plan)
	}
	if plan.CaseRoot != caseRoot || plan.ProjectName != "demo-case" || len(plan.Writes) != 2 {
		t.Fatalf("unexpected attach preview: %+v", plan)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "instance.yml")); !os.IsNotExist(err) {
		t.Fatalf("attach -WhatIf created instance metadata, err=%v", err)
	}
}

func TestRunAttachApplyWritesOnlyMetadataAndShim(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "attach", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo-case", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command    string `json:"command"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		Writes     []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("attach apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "attach" || !result.IsMutation || !result.Applied || len(result.Writes) != 2 {
		t.Fatalf("unexpected attach apply result: %+v", result)
	}
	metadata, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "instance.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(metadata)
	if !strings.Contains(text, "templatePack: _template") || !strings.Contains(text, "projectName: demo-case") || !strings.Contains(text, "mode: case-local-shim") {
		t.Fatalf("instance metadata missing expected values: %s", text)
	}
	shim, err := os.ReadFile(filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile(filepath.Join(repoRoot(t), "rekit", "templates", "case-shim", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shim, canonical) {
		t.Fatal("case shim does not match canonical thin shim")
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".re-template.yml")); !os.IsNotExist(err) {
		t.Fatalf("attach -Apply wrote legacy metadata unexpectedly, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "state.json")); !os.IsNotExist(err) {
		t.Fatalf("attach -Apply wrote state unexpectedly, err=%v", err)
	}
}

func TestRunAttachRejectsDifferentBinding(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, ".rekit/instance.yml", "templateRoot: C:\\other\\kit\ntemplatePack: _template\nprojectName: demo\nprojectRoot: "+caseRoot+"\n")
	var out bytes.Buffer
	err := Run([]string{"-Command", "attach", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for different templateRoot")
	}
	if !strings.Contains(err.Error(), "different templateRoot") {
		t.Fatalf("error = %q, want different templateRoot", err.Error())
	}
}

func TestRunSyncReviewRejectsWriteFlags(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", t.TempDir(), "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for sync -Apply")
	}
	if !strings.Contains(err.Error(), "review-only") {
		t.Fatalf("error = %q, want review-only guard", err.Error())
	}
}

func TestRunPromoteReviewRequiresAttachedCase(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", t.TempDir()}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for non-case promote target")
	}
	if !strings.Contains(err.Error(), "target is not an attached rekit case") {
		t.Fatalf("error = %q, want attached case guard", err.Error())
	}
}

func TestRunPromoteReviewRejectsWriteFlags(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", t.TempDir(), "-CreateCandidates"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for promote -CreateCandidates")
	}
	if !strings.Contains(err.Error(), "review-only") {
		t.Fatalf("error = %q, want review-only guard", err.Error())
	}
}

func TestRunSyncReviewEmitsNonMutatingPlan(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	plan := decodePlan(t, out.Bytes())
	if plan.Command != "sync" || plan.IsMutation {
		t.Fatalf("unexpected sync review plan: %+v", plan)
	}
}

func TestRunPromoteReviewEmitsNonMutatingPlan(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	plan := decodePlan(t, out.Bytes())
	if plan.Command != "promote" || plan.IsMutation {
		t.Fatalf("unexpected promote review plan: %+v", plan)
	}
}

func TestRunSyncReviewWritesArtifacts(t *testing.T) {
	caseRoot := attachedCase(t)
	reviewRoot := filepath.Join(t.TempDir(), "sync-review")
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-ReviewOutputDir", reviewRoot}, &out)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeArtifactResult(t, out.Bytes())
	if result.Command != "sync" || result.IsMutation || !result.WritesArtifacts {
		t.Fatalf("unexpected artifact result: %+v", result)
	}
	assertFileExists(t, result.PacketPath)
	assertFileExists(t, result.SummaryPath)
	assertFileExists(t, result.CombinedDiffPath)
	packet, err := os.ReadFile(result.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(packet), `"command": "sync"`) || !strings.Contains(string(packet), `"reviewRequired": true`) {
		t.Fatalf("sync packet missing expected fields: %s", string(packet))
	}
}

func TestRunPromoteReviewWritesArtifactsAndPreview(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Case README\n\nlocal safe change\n")
	writeCaseFile(t, caseRoot, "references/template/toolchain-router.md", "# Tool route\n\nlocal safe candidate\n")
	reviewRoot := filepath.Join(t.TempDir(), "promote-review")
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-ReviewOutputDir", reviewRoot}, &out)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeArtifactResult(t, out.Bytes())
	if result.Command != "promote" || result.IsMutation || !result.WritesArtifacts {
		t.Fatalf("unexpected artifact result: %+v", result)
	}
	assertFileExists(t, result.PacketPath)
	assertFileExists(t, result.SummaryPath)
	assertFileExists(t, result.CombinedDiffPath)
	assertFileExists(t, filepath.Join(reviewRoot, "previews", "references_template_toolchain-router.md.sanitized-preview.md"))
	diff, err := os.ReadFile(result.CombinedDiffPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(diff), "local safe change") {
		t.Fatalf("promote diff missing case change: %s", string(diff))
	}
}

func TestRunGateRequiresWhatIfOrApply(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Action", "debug", "-Lane", "main"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for gate without -WhatIf or -Apply")
	}
	if !strings.Contains(err.Error(), "-Apply") || !strings.Contains(err.Error(), "-WhatIf") {
		t.Fatalf("error = %q, want apply/whatif guard", err.Error())
	}
}

func TestRunGateRejectsWhatIfWithApply(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Apply", "-Action", "debug", "-Lane", "main"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for gate -WhatIf -Apply")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("error = %q, want combination guard", err.Error())
	}
}

func TestRunGateDryRunEmitsNonMutatingPlan(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-WhatIf",
		"-Action", "full-trace",
		"-Lane", "main",
		"-Subject", "trace handler",
		"-TargetRef", "batch-test",
		"-BatchId", "batch-test",
		"-Scope", "handler only",
		"-Budget", "60s",
		"-TriedLightSteps", "static review,focused grep",
		"-StopConditions", "first crash,timeout",
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Command              string `json:"command"`
		IsMutation           bool   `json:"isMutation"`
		ReviewRequired       bool   `json:"reviewRequired"`
		RequiresConfirmation bool   `json:"requiresConfirmation"`
		EventPreview         struct {
			Kind    string `json:"kind"`
			Status  string `json:"status"`
			Lane    string `json:"lane"`
			Target  string `json:"target"`
			BatchID string `json:"batchId"`
			Gate    struct {
				Action                      string   `json:"action"`
				DeniedUntilUserConfirmation []string `json:"deniedUntilUserConfirmation"`
			} `json:"gate"`
		} `json:"eventPreview"`
	}
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("gate plan stdout is not JSON: %v\n%s", err, out.String())
	}
	if plan.Command != "gate" || plan.IsMutation || !plan.ReviewRequired || !plan.RequiresConfirmation {
		t.Fatalf("unexpected gate plan flags: %+v", plan)
	}
	if plan.EventPreview.Kind != "request" || plan.EventPreview.Status != "pending-gate" || plan.EventPreview.Lane != "main" {
		t.Fatalf("unexpected event preview: %+v", plan.EventPreview)
	}
	if plan.EventPreview.Target != "batch-test" || plan.EventPreview.BatchID != "batch-test" {
		t.Fatalf("batch/target not preserved: %+v", plan.EventPreview)
	}
	if plan.EventPreview.Gate.Action != "full-trace" || len(plan.EventPreview.Gate.DeniedUntilUserConfirmation) != 1 {
		t.Fatalf("unexpected gate detail: %+v", plan.EventPreview.Gate)
	}
}

func TestRunGateDryRunRejectsUnknownLane(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Action", "debug", "-Lane", "missing"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for unknown gate lane")
	}
	if !strings.Contains(err.Error(), "unknown lane") {
		t.Fatalf("error = %q, want unknown lane", err.Error())
	}
}

func TestRunGateApplyRequiresActor(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Action", "debug", "-Lane", "main"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for gate -Apply without -Actor")
	}
	if !strings.Contains(err.Error(), "requires -Actor") {
		t.Fatalf("error = %q, want actor guard", err.Error())
	}
}

func TestRunGateApplyAppendsPendingGateRequest(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	args := []string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "debug gate",
		"-TargetRef", "batch-apply",
		"-BatchId", "batch-apply",
		"-Scope", "display only",
	}
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Applied    bool   `json:"applied"`
		IsMutation bool   `json:"isMutation"`
		EventID    string `json:"eventId"`
		Path       string `json:"path"`
		Event      struct {
			Kind   string `json:"kind"`
			Status string `json:"status"`
		} `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !result.Applied || !result.IsMutation || result.EventID == "" {
		t.Fatalf("unexpected apply result: %+v", result)
	}
	if result.Path != ".rekit/facts/requests.jsonl" || result.Event.Kind != "request" || result.Event.Status != "pending-gate" {
		t.Fatalf("unexpected event result: %+v", result)
	}
	ledger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), result.EventID) || !strings.Contains(string(ledger), `"pending-gate"`) {
		t.Fatalf("ledger does not contain gate event: %s", string(ledger))
	}
}

func TestRunGateApplyIsIdempotentByEventID(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	args := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Action", "debug", "-Lane", "main", "-Actor", "runtime-test", "-Subject", "debug gate"}
	var first bytes.Buffer
	if err := Run(args, &first); err != nil {
		t.Fatal(err)
	}
	var second bytes.Buffer
	if err := Run(args, &second); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Applied bool   `json:"applied"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(second.Bytes(), &result); err != nil {
		t.Fatalf("second gate apply stdout is not JSON: %v\n%s", err, second.String())
	}
	if result.Applied || result.Reason != "duplicate eventId" {
		t.Fatalf("unexpected duplicate result: %+v", result)
	}
}

func decodePlan(t *testing.T, b []byte) review.Plan {
	t.Helper()
	var plan review.Plan
	if err := json.Unmarshal(b, &plan); err != nil {
		t.Fatalf("review plan stdout is not JSON: %v\n%s", err, string(b))
	}
	if !plan.Summary.ReviewRequired {
		t.Fatalf("reviewRequired = false: %+v", plan.Summary)
	}
	return plan
}

type artifactResult struct {
	Command          string `json:"command"`
	IsMutation       bool   `json:"isMutation"`
	WritesArtifacts  bool   `json:"writesArtifacts"`
	PacketPath       string `json:"packetPath"`
	SummaryPath      string `json:"summaryPath"`
	CombinedDiffPath string `json:"combinedDiffPath"`
}

func decodeArtifactResult(t *testing.T, b []byte) artifactResult {
	t.Helper()
	var result artifactResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("artifact stdout is not JSON: %v\n%s", err, string(b))
	}
	return result
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
	if st.IsDir() {
		t.Fatalf("expected file, got directory: %s", path)
	}
}

func writeCaseFile(t *testing.T, caseRoot, rel, text string) {
	t.Helper()
	path := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func attachedCase(t *testing.T) string {
	t.Helper()
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := "templateRoot: " + repoRoot(t) + "\ntemplatePack: _template\nprojectName: demo\nprojectRoot: " + caseRoot + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func attachedCaseWithBoard(t *testing.T) string {
	t.Helper()
	caseRoot := attachedCase(t)
	board := `{"lanes":[{"id":"main"},{"id":"feature-demo"}]}`
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), []byte(board), 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func fullAttachedCase(t *testing.T) string {
	t.Helper()
	caseRoot := attachedCase(t)
	root := repoRoot(t)
	copyRepoFile(t, root, "rekit/templates/case-shim/SKILL.md", caseRoot, ".claude/skills/rekit/SKILL.md")
	copyRepoFile(t, root, "packs/_template/references/template/README.md", caseRoot, "references/template/README.md")
	copyRepoFile(t, root, "packs/_template/references/template/workflow-template.md", caseRoot, "references/template/workflow-template.md")
	copyRepoFile(t, root, "packs/_template/references/template/toolchain-router.md", caseRoot, "references/template/toolchain-router.md")
	copyRepoFile(t, root, "packs/_template/CLAUDE.local.snippet.md", caseRoot, "CLAUDE.local.md")
	template, err := os.ReadFile(filepath.Join(root, "packs", "_template", "references", "template", "task-handoff.template.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(template), "<PROJECT_NAME>", "demo")
	text = strings.ReplaceAll(text, "<PROJECT_ROOT>", caseRoot)
	writeCaseFile(t, caseRoot, "references/template/task-handoff.md", text)
	return caseRoot
}

func copyRepoFile(t *testing.T, repoRoot, sourceRel, caseRoot, targetRel string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(sourceRel)))
	if err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, targetRel, string(content))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
