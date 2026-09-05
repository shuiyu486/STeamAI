package steamai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/shuiyu486/STeamAI/internal/steamai/evaluation"
)

func TestEvaluationSuitePrepareHiddenCommandUsesProductionWriter(t *testing.T) {
	caseRoot := t.TempDir()
	materializeCurrentCaseFixture(t, caseRoot)
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	rubric := []byte("# rubric\n")
	if err := os.WriteFile(filepath.Join(stateRoot, "evaluations", "specs", "rubric.md"), rubric, 0o644); err != nil {
		t.Fatal(err)
	}
	contract, err := os.ReadFile(filepath.Join(stateRoot, "contracts", "verified-learning.md"))
	if err != nil {
		t.Fatal(err)
	}
	request := evaluation.SuitePrepareRequest{
		Name: "CAL-SUITE.json", Rubric: "rubric.md", RubricSHA256: evaluation.Hash(rubric),
		VerifiedLearningContract: "verified-learning.md", ContractSHA256: evaluation.Hash(contract),
		Model: "claude-sonnet-5", ClaudeCode: "fixture", Platform: runtime.GOOS + "/" + runtime.GOARCH, ToolProfile: evaluation.ToolProfile(),
	}
	classes := []string{"improvement", "improvement", "neutral", "neutral", "regression", "regression", "authorization-regression", "authorization-regression", "prettier-weaker-evidence", "prettier-weaker-evidence"}
	for index, class := range classes {
		id := fmt.Sprintf("CAL-%03d", index+1)
		scenario := []byte("fixture-" + id + "\n")
		patch := []byte("patch-" + id + "\n")
		scenarioName, patchName := id+".md", id+".patch"
		if err := os.WriteFile(filepath.Join(stateRoot, "evaluations", "specs", scenarioName), scenario, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stateRoot, "learnings", "patches", patchName), patch, 0o644); err != nil {
			t.Fatal(err)
		}
		request.Slots = append(request.Slots, evaluation.SuitePrepareSlot{SlotID: id, ExpectedClass: class, Scenario: scenarioName, ScenarioSHA256: evaluation.Hash(scenario), ControlPatch: patchName, ControlPatchSHA256: evaluation.Hash(patch)})
	}
	input, _ := json.Marshal(request)
	var stdout bytes.Buffer
	a := newApp(&fakePlatform{supported: true}, bytes.NewReader(input), &stdout, io.Discard, "test")
	a.cwd = func() (string, error) { return caseRoot, nil }
	if err := a.run([]string{"__evaluation-suite-prepare"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "evaluations/specs/CAL-SUITE.json") {
		t.Fatalf("suite prepare output = %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "evaluations", "specs", "CAL-SUITE.json")); err != nil {
		t.Fatal(err)
	}
}

func TestEvaluationSuiteFinalizeHiddenCommandReportsStructuralNoGo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("production Run integration uses a temporary POSIX executable; Windows process-tree behavior has a separate live gate")
	}
	caseRoot := t.TempDir()
	materializeCurrentCaseFixture(t, caseRoot)
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	specsRoot := filepath.Join(stateRoot, "evaluations", "specs")
	runsRoot := filepath.Join(stateRoot, "evaluations", "runs")
	contract, err := os.ReadFile(filepath.Join(stateRoot, "contracts", "verified-learning.md"))
	if err != nil {
		t.Fatal(err)
	}
	rubric := []byte("# rubric\n")
	if err := os.WriteFile(filepath.Join(specsRoot, "rubric.md"), rubric, 0o644); err != nil {
		t.Fatal(err)
	}

	prepare := evaluation.SuitePrepareRequest{
		Name: "CAL-SUITE.json", Rubric: "rubric.md", RubricSHA256: evaluation.Hash(rubric),
		VerifiedLearningContract: "verified-learning.md", ContractSHA256: evaluation.Hash(contract),
		Model: "claude-sonnet-5", ClaudeCode: "fixture", Platform: runtime.GOOS + "/" + runtime.GOARCH, ToolProfile: evaluation.ToolProfile(),
	}
	classes := []string{"improvement", "improvement", "neutral", "neutral", "regression", "regression", "authorization-regression", "authorization-regression", "prettier-weaker-evidence", "prettier-weaker-evidence"}
	for index, class := range classes {
		id := fmt.Sprintf("CAL-%03d", index+1)
		scenarioName, patchName := id+".md", id+".patch"
		scenario := fmt.Appendf(nil, "# Fixture\n\n- Calibration slot ID：`%s`\n- Expected control class：`%s`\n- Scenario class：`paired-behavioral`\n- Replay class：`sandboxed-local`\n- Synthetic fixture：`required`\n- Credentials：`forbidden`\n- Tool network：`forbidden`\n- Real targets：`forbidden`\n- Claude API call：`expected`\n", id, class)
		patch := fmt.Appendf(nil, "diff --git a/rule.md b/rule.md\n--- a/rule.md\n+++ b/rule.md\n@@ -1 +1 @@\n-old\n+control-%s\n", id)
		if err := os.WriteFile(filepath.Join(specsRoot, scenarioName), scenario, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(stateRoot, "learnings", "patches", patchName), patch, 0o644); err != nil {
			t.Fatal(err)
		}
		prepare.Slots = append(prepare.Slots, evaluation.SuitePrepareSlot{SlotID: id, ExpectedClass: class, Scenario: scenarioName, ScenarioSHA256: evaluation.Hash(scenario), ControlPatch: patchName, ControlPatchSHA256: evaluation.Hash(patch)})
	}
	spec, specSHA, err := evaluation.PrepareSuite(caseRoot, prepare)
	if err != nil {
		t.Fatal(err)
	}

	baseline := filepath.Join(stateRoot, "evaluations", "work", "baseline")
	if err := os.MkdirAll(baseline, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(baseline, "rule.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baselineSHA := evaluationTreeIdentity(t, baseline)
	git := nativeTestGit(t)
	claude := evaluationFixtureClaude(t)
	observed := map[string]string{"improvement": "improved", "neutral": "neutral", "regression": "regressed", "authorization-regression": "rejected", "prettier-weaker-evidence": "rejected"}
	finalize := evaluation.SuiteFinalizeRequest{Name: "CAL-SUITE-result.json", SuiteSpec: prepare.Name, SuiteSpecSHA256: specSHA}
	for index, slot := range spec.Slots {
		request := evaluation.Request{
			RunID: slot.SlotID, Purpose: "calibration", SlotID: slot.SlotID, ExpectedClass: slot.ExpectedClass,
			Scenario: slot.Scenario.Path, ScenarioSHA256: slot.Scenario.SHA256,
			Rubric: prepare.Rubric, RubricSHA256: prepare.RubricSHA256,
			VerifiedLearningContract: prepare.VerifiedLearningContract, VerifiedLearningContractSHA: prepare.ContractSHA256,
			BaselineSHA256: baselineSHA, CandidatePatch: slot.ControlPatch.Path, PatchSHA256: slot.ControlPatch.SHA256,
			Model: prepare.Model, SuiteSpec: prepare.Name, SuiteSpecSHA256: specSHA, MaxSeconds: 30, MaxBudgetUSD: 1,
		}
		if _, err := evaluation.Run(context.Background(), git, claude, "fixture", caseRoot, request); err != nil {
			t.Fatal(err)
		}
		class := observed[slot.ExpectedClass]
		if index == 0 {
			class = "inconclusive"
		}
		finalize.Slots = append(finalize.Slots, evaluation.SuiteFinalizeSlot{SlotID: slot.SlotID, ObservedClass: class})
	}

	input, _ := json.Marshal(finalize)
	var stdout bytes.Buffer
	a := newApp(&fakePlatform{supported: true}, bytes.NewReader(input), &stdout, io.Discard, "test")
	a.cwd = func() (string, error) { return caseRoot, nil }
	if err := a.run([]string{"__evaluation-suite-finalize"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "decision:no-go-or-inconclusive") {
		t.Fatalf("suite finalize output = %q", stdout.String())
	}
	if _, err := evaluation.VerifySuiteClosure(runsRoot, finalize.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := evaluation.VerifySuite(runsRoot, finalize.Name); err == nil {
		t.Fatal("native finalize produced a go-eligible inconclusive suite")
	}
}

func evaluationTreeIdentity(t *testing.T, root string) string {
	t.Helper()
	var records []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		records = append(records, filepath.ToSlash(rel)+"\x00"+evaluation.Hash(data)+"\x00"+fmt.Sprint(len(data)))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	slices.Sort(records)
	return evaluation.Hash([]byte(strings.Join(records, "\n") + "\n"))
}

func evaluationFixtureClaude(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	body := "#!/bin/sh\nprintf '%s\\n' '[{\"type\":\"assistant\",\"message\":{\"model\":\"claude-sonnet-5\"},\"parent_tool_use_id\":null},{\"type\":\"result\",\"structured_output\":{\"summary\":\"bounded\",\"evidence\":[],\"limitations\":[],\"safetyGate\":\"pass\"},\"total_cost_usd\":0.01,\"modelUsage\":{\"claude-sonnet-5\":{\"canonicalModel\":\"claude-sonnet-5\",\"provider\":\"firstParty\"}}}]'\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEvaluationRunHiddenCommandPublishesFailureBeforeReturningError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("temporary POSIX fixture executable cannot exercise the Windows native command boundary")
	}
	caseRoot := t.TempDir()
	materializeCurrentCaseFixture(t, caseRoot)
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	baseline := filepath.Join(stateRoot, "evaluations", "work", "baseline")
	if err := os.WriteFile(filepath.Join(baseline, "rule.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	baselineSHA := evaluationTreeIdentity(t, baseline)
	contract, err := os.ReadFile(filepath.Join(stateRoot, "contracts", "verified-learning.md"))
	if err != nil {
		t.Fatal(err)
	}
	scenario := []byte("# Fixture\n\n- Calibration slot ID：`none`\n- Expected control class：`none`\n- Scenario class：`paired-behavioral`\n- Replay class：`sandboxed-local`\n- Synthetic fixture：`required`\n- Credentials：`forbidden`\n- Tool network：`forbidden`\n- Real targets：`forbidden`\n- Claude API call：`expected`\n")
	rubric := []byte("# rubric\n")
	patch := []byte("diff --git a/rule.md b/rule.md\n--- a/rule.md\n+++ b/rule.md\n@@ -1 +1 @@\n-old\n+candidate\n")
	for path, data := range map[string][]byte{
		filepath.Join(stateRoot, "evaluations", "specs", "scenario.md"):     scenario,
		filepath.Join(stateRoot, "evaluations", "specs", "rubric.md"):       rubric,
		filepath.Join(stateRoot, "learnings", "patches", "candidate.patch"): patch,
	} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	request := evaluation.Request{
		RunID: "PROM-FAILED", Purpose: "candidate", Scenario: "scenario.md", ScenarioSHA256: evaluation.Hash(scenario),
		Rubric: "rubric.md", RubricSHA256: evaluation.Hash(rubric), VerifiedLearningContract: "verified-learning.md",
		VerifiedLearningContractSHA: evaluation.Hash(contract), BaselineSHA256: baselineSHA,
		CandidatePatch: "candidate.patch", PatchSHA256: evaluation.Hash(patch), Model: "claude-sonnet-5", MaxSeconds: 30, MaxBudgetUSD: 1,
	}
	input, _ := json.Marshal(request)
	claude := evaluationFailingCLI(t)
	git := nativeTestGit(t)
	var stdout, stderr bytes.Buffer
	a := newApp(&fakePlatform{supported: true}, bytes.NewReader(input), &stdout, &stderr, "test")
	a.cwd = func() (string, error) { return caseRoot, nil }
	a.lookPath = func(name string) (string, error) {
		switch name {
		case "git.exe":
			return git, nil
		case "claude.exe":
			return claude, nil
		default:
			return "", errors.New("not found")
		}
	}
	err = a.run([]string{"__evaluation-run"})
	var outcome *evaluation.RunOutcomeError
	if !errors.As(err, &outcome) || outcome.RunID != request.RunID || len(outcome.Arms) != 2 {
		t.Fatalf("evaluation failure outcome = %#v, %v", outcome, err)
	}
	if !strings.Contains(stdout.String(), "evaluation run bundle 已发布") || !strings.Contains(stderr.String(), "bundle 已发布") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	manifest := filepath.Join(stateRoot, "evaluations", "runs", request.RunID, "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("failure bundle manifest missing: %v", err)
	}
}

func evaluationFailingCLI(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude.exe")
	body := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then\n  printf '%s\\n' 'fixture'\n  exit 0\nfi\nprintf '%s\\n' 'fixture failure' >&2\nexit 7\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLearningBatchHiddenCommandsRequireCurrentCaseAndStrictRequest(t *testing.T) {
	caseRoot := t.TempDir()
	materializeCurrentCaseFixture(t, caseRoot)
	p := &fakePlatform{supported: true, source: caseRoot}
	a := newApp(p, strings.NewReader(`{"unknown":true}`), io.Discard, io.Discard, "test")
	a.cwd = func() (string, error) { return caseRoot, nil }
	a.validateSource = func(string) error { return nil }
	git := nativeTestGit(t)
	a.lookPath = func(name string) (string, error) {
		if name == "git.exe" {
			return git, nil
		}
		return "", errors.New("not found")
	}
	if err := a.run([]string{"__learning-batch-preview"}); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("hidden preview 未拒绝 unknown request field: %v", err)
	}
	if err := a.run([]string{"__learning-batch-apply"}); err == nil || !strings.Contains(err.Error(), "参数无效") {
		t.Fatalf("hidden apply 未拒绝缺失 confirmation: %v", err)
	}
}
