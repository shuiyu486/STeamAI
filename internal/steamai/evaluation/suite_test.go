package evaluation

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrepareAndFinalizeSuiteUseProductionClosure(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, first := evaluationFixture(t, git, "CAL-001")
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	specsRoot := filepath.Join(stateRoot, "evaluations", "specs")
	patchRoot := filepath.Join(stateRoot, "learnings", "patches")
	contract := mustReadTestFile(t, filepath.Join(stateRoot, "contracts", "verified-learning.md"))
	rubric := mustReadTestFile(t, filepath.Join(specsRoot, "rubric.md"))
	prepared := SuitePrepareRequest{
		Name: "CAL-SUITE.json", Rubric: "rubric.md", RubricSHA256: Hash(rubric),
		VerifiedLearningContract: "verified-learning.md", ContractSHA256: Hash(contract),
		Model: "claude-sonnet-5", ClaudeCode: "Claude Code fixture", Platform: runtime.GOOS + "/" + runtime.GOARCH,
		ToolProfile: ToolProfile(),
	}
	classes := []string{"improvement", "improvement", "neutral", "neutral", "regression", "regression", "authorization-regression", "authorization-regression", "prettier-weaker-evidence", "prettier-weaker-evidence"}
	for index, class := range classes {
		runID := "CAL-" + []string{"001", "002", "003", "004", "005", "006", "007", "008", "009", "010"}[index]
		scenarioName := runID + ".md"
		scenario := []byte("# Fixture\n\n- Calibration slot ID：`" + runID + "`\n- Expected control class：`" + class + "`\n- Scenario class：`paired-behavioral`\n- Replay class：`sandboxed-local`\n- Synthetic fixture：`required`\n- Credentials：`forbidden`\n- Tool network：`forbidden`\n- Real targets：`forbidden`\n- Claude API call：`expected`\n")
		writeTestFile(t, filepath.Join(specsRoot, scenarioName), string(scenario))
		patchName := runID + ".patch"
		patch := []byte("diff --git a/rule.md b/rule.md\n--- a/rule.md\n+++ b/rule.md\n@@ -1 +1 @@\n-old\n+control-" + runID + "\n")
		writeTestFile(t, filepath.Join(patchRoot, patchName), string(patch))
		prepared.Slots = append(prepared.Slots, SuitePrepareSlot{
			SlotID: runID, ExpectedClass: class, Scenario: scenarioName, ScenarioSHA256: Hash(scenario),
			ControlPatch: patchName, ControlPatchSHA256: Hash(patch),
		})
	}
	spec, specSHA, err := PrepareSuite(caseRoot, prepared)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Identity == "" || specSHA == "" {
		t.Fatal("prepared suite spec missing identity")
	}
	observed := map[string]string{"improvement": "improved", "neutral": "neutral", "regression": "regressed", "authorization-regression": "rejected", "prettier-weaker-evidence": "rejected"}
	finalize := SuiteFinalizeRequest{Name: "CAL-SUITE-result.json", SuiteSpec: prepared.Name, SuiteSpecSHA256: specSHA}
	for _, slot := range spec.Slots {
		request := first
		request.RunID, request.SlotID, request.ExpectedClass = slot.SlotID, slot.SlotID, slot.ExpectedClass
		request.Scenario, request.ScenarioSHA256 = slot.Scenario.Path, slot.Scenario.SHA256
		request.CandidatePatch, request.PatchSHA256 = slot.ControlPatch.Path, slot.ControlPatch.SHA256
		request.SuiteSpec, request.SuiteSpecSHA256 = prepared.Name, specSHA
		if _, err := Run(context.Background(), git, fakeClaude(t, false), "Claude Code fixture", caseRoot, request); err != nil {
			t.Fatal(err)
		}
		finalize.Slots = append(finalize.Slots, SuiteFinalizeSlot{SlotID: slot.SlotID, ObservedClass: observed[slot.ExpectedClass]})
	}
	suite, err := FinalizeSuite(caseRoot, finalize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifySuite(filepath.Join(stateRoot, "evaluations", "runs"), finalize.Name); err != nil {
		t.Fatal(err)
	}
	if suite.ManifestSHA256 == "" || len(suite.Manifest.Slots) != 10 {
		t.Fatalf("unexpected finalized suite: %+v", suite.Manifest)
	}
	if _, err := FinalizeSuite(caseRoot, finalize); err == nil {
		t.Fatal("suite finalize overwrote immutable result")
	}

	noGo := finalize
	noGo.Name = "CAL-SUITE-no-go.json"
	noGo.Slots = append([]SuiteFinalizeSlot(nil), finalize.Slots...)
	noGo.Slots[0].ObservedClass = "inconclusive"
	if _, err := FinalizeSuite(caseRoot, noGo); err != nil {
		t.Fatalf("finalize structural no-go suite: %v", err)
	}
	runsRoot := filepath.Join(stateRoot, "evaluations", "runs")
	if _, err := VerifySuiteClosure(runsRoot, noGo.Name); err != nil {
		t.Fatalf("verify structural no-go closure: %v", err)
	}
	if _, err := VerifySuite(runsRoot, noGo.Name); err == nil {
		t.Fatal("inconclusive suite became Gate 3 go eligible")
	}

	for name, path := range map[string]string{
		"rubric":        filepath.Join(specsRoot, spec.Rubric.Path),
		"scenario":      filepath.Join(specsRoot, spec.Slots[0].Scenario.Path),
		"control patch": filepath.Join(patchRoot, spec.Slots[0].ControlPatch.Path),
		"contract":      filepath.Join(stateRoot, "contracts", spec.VerifiedLearningContract.Path),
	} {
		t.Run(name+" drift", func(t *testing.T) {
			original := mustReadTestFile(t, path)
			writeTestFile(t, path, string(append(original, []byte("drift\n")...)))
			if _, err := VerifySuite(runsRoot, finalize.Name); err == nil {
				t.Fatalf("suite accepted drifted %s", name)
			}
			writeTestFile(t, path, string(original))
			if _, err := VerifySuite(runsRoot, finalize.Name); err != nil {
				t.Fatalf("suite did not recover after restoring %s: %v", name, err)
			}
		})
	}
}

func TestSuiteRequestDecodersAreStrict(t *testing.T) {
	if _, err := DecodeSuitePrepareRequest(strings.NewReader(`{"name":"suite.json","unknown":true}`)); err == nil {
		t.Fatal("prepare request accepted unknown field")
	}
	if _, err := DecodeSuiteFinalizeRequest(strings.NewReader(`{"name":"suite.json","suiteSpec":"spec.json","suiteSpecSha256":"` + strings.Repeat("a", 64) + `","slots":[]}{} `)); err == nil {
		t.Fatal("finalize request accepted trailing JSON")
	}
}
