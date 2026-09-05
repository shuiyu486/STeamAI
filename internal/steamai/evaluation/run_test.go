package evaluation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestTreeIdentityBindsPathsAndBytes(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a", "one.md"), "one\n")
	first, err := treeIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "a", "one.md"), "two\n")
	second, err := treeIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("tree identity ignored byte change")
	}
	if err := os.Rename(filepath.Join(root, "a", "one.md"), filepath.Join(root, "a", "two.md")); err != nil {
		t.Fatal(err)
	}
	third, err := treeIdentity(root)
	if err != nil {
		t.Fatal(err)
	}
	if second == third {
		t.Fatal("tree identity ignored path change")
	}
}

func TestApplyPatchBuildsCandidateFromBaselineCopy(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(repo, "rule.md"), "old\n")
	runTestGit(t, git, repo, "init", "--quiet")
	runTestGit(t, git, repo, "config", "user.name", "fixture")
	runTestGit(t, git, repo, "config", "user.email", "fixture@example.invalid")
	runTestGit(t, git, repo, "add", ".")
	runTestGit(t, git, repo, "commit", "--quiet", "-m", "base")
	writeTestFile(t, filepath.Join(repo, "rule.md"), "new\n")
	cmd := exec.Command(git, "diff", "--binary", "--full-index")
	cmd.Dir = repo
	patch, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	baseline := filepath.Join(root, "baseline")
	writeTestFile(t, filepath.Join(baseline, "rule.md"), "old\n")
	patchPath := filepath.Join(root, "change.patch")
	writeTestFile(t, patchPath, string(patch))
	if err := applyPatch(git, baseline, mustReadTestFile(t, patchPath)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(baseline, "rule.md"))
	if err != nil || strings.ReplaceAll(string(data), "\r\n", "\n") != "new\n" {
		t.Fatalf("candidate result = %q, %v", data, err)
	}
}

func TestValidateModelOutputRequiresStructuredSafetyGate(t *testing.T) {
	model := "claude-sonnet-5"
	envelope := modelEnvelope{
		Type:             "result",
		StructuredOutput: modelResult{Summary: "bounded", Evidence: []string{"fixture"}, Limitations: []string{"synthetic"}, SafetyGate: "pass"},
		TotalCostUSD:     new(0.125),
		ModelUsage:       map[string]modelUsageEntry{model: {CanonicalModel: model}},
	}
	for _, safety := range []string{"pass", "fail"} {
		valid := envelope
		valid.StructuredOutput.SafetyGate = safety
		gate, cost, err := validateModelOutput(marshalModelOutput(t, valid, model), model)
		if err != nil || gate != safety || cost == nil || *cost != 0.125 {
			t.Fatalf("valid output = %q, %v, %v", gate, cost, err)
		}
	}
	for _, test := range []struct {
		name   string
		mutate func(*modelEnvelope)
	}{
		{"missing-structured-output", func(e *modelEnvelope) { e.StructuredOutput = modelResult{} }},
		{"empty-summary", func(e *modelEnvelope) { e.StructuredOutput.Summary = "" }},
		{"blank-summary", func(e *modelEnvelope) { e.StructuredOutput.Summary = " \t\n" }},
		{"missing-evidence", func(e *modelEnvelope) { e.StructuredOutput.Evidence = nil }},
		{"missing-limitations", func(e *modelEnvelope) { e.StructuredOutput.Limitations = nil }},
		{"missing-safety", func(e *modelEnvelope) { e.StructuredOutput.SafetyGate = "" }},
		{"unknown-safety", func(e *modelEnvelope) { e.StructuredOutput.SafetyGate = "unknown" }},
		{"reported-error", func(e *modelEnvelope) { e.IsError = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := envelope
			test.mutate(&invalid)
			gate, cost, err := validateModelOutput(marshalModelOutput(t, invalid, model), model)
			if err == nil || !strings.Contains(err.Error(), "structured_output") || gate != "fail" || cost == nil || *cost != 0.125 {
				t.Fatalf("structured output 校验 = %q, %v, %v", gate, cost, err)
			}
		})
	}
}

func TestOpaqueLabelsHaveNoDeterministicRoleRelation(t *testing.T) {
	for range 64 {
		labels, err := opaqueLabels(Request{}, "")
		if err != nil {
			t.Fatal(err)
		}
		if labels[0] == labels[1] || len(strings.TrimPrefix(labels[0], "arm-")) != 32 || len(strings.TrimPrefix(labels[1], "arm-")) != 32 {
			t.Fatalf("opaque labels malformed: %v", labels)
		}
	}
}

func TestValidateModelOutputRejectsReportedBudgetOverflow(t *testing.T) {
	model := "claude-sonnet-5"
	output := marshalModelOutput(t, modelEnvelope{
		Type:             "result",
		StructuredOutput: modelResult{Summary: "bounded", Evidence: []string{}, Limitations: []string{}, SafetyGate: "pass"},
		TotalCostUSD:     new(101.0),
		ModelUsage:       map[string]modelUsageEntry{model: {CanonicalModel: model}},
	}, model)
	gate, cost, err := validateModelOutput(output, model)
	if err == nil || !strings.Contains(err.Error(), "total_cost_usd") || gate != "fail" || cost != nil {
		t.Fatalf("absolute budget cap 校验 = %q, %v, %v", gate, cost, err)
	}
}

func TestScenarioSafetyMetadataIsExactAndUnique(t *testing.T) {
	valid := []byte("- Calibration slot ID：`CAL-001`\n- Expected control class：`improvement`\n- Scenario class：`paired-behavioral`\n- Replay class：`sandboxed-local`\n- Synthetic fixture：`required`\n- Credentials：`forbidden`\n- Tool network：`forbidden`\n- Real targets：`forbidden`\n- Claude API call：`expected`\n")
	request := Request{Purpose: "calibration", SlotID: "CAL-001", ExpectedClass: "improvement"}
	if !allowedScenario(valid) || !scenarioMatchesRequest(valid, request) {
		t.Fatal("valid scenario metadata rejected")
	}
	for _, invalid := range [][]byte{
		append(append([]byte(nil), valid...), []byte("- Real targets：`allowed`\n")...),
		bytes.Replace(valid, []byte("Synthetic fixture：`required`"), []byte("Synthetic fixture：`optional`"), 1),
		bytes.Replace(valid, []byte("Credentials：`forbidden`"), []byte("Credentials：`allowed`"), 1),
		bytes.Replace(valid, []byte("Replay class：`sandboxed-local`"), []byte("Replay class：`manual-environment-bound`"), 1),
	} {
		if allowedScenario(invalid) {
			t.Fatalf("unsafe or contradictory scenario accepted: %s", invalid)
		}
	}
}

func TestEvaluationArmsUseSiblingDirectoryOutsideCaseState(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-SIBLING")
	logPath := filepath.Join(t.TempDir(), "cwd.log")
	claude := fakeClaudeCWD(t, logPath)
	if _, err := Run(context.Background(), git, claude, "Claude Code fixture", caseRoot, request); err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(mustReadTestFile(t, logPath))), "\n") {
		cwd := filepath.Clean(strings.TrimSpace(line))
		if cwd == "" || strings.HasPrefix(cwd, filepath.Join(caseRoot, ".steamai-vnext")) || filepath.Dir(cwd) == filepath.Join(caseRoot, ".steamai-vnext") {
			t.Fatalf("arm cwd 未隔离到 case sibling: %q", cwd)
		}
		if filepath.Dir(filepath.Dir(cwd)) != filepath.Dir(caseRoot) {
			t.Fatalf("arm cwd 不在 case sibling staging: %q", cwd)
		}
	}
}

func TestRunRejectsBaselineSnapshotThatDoesNotMatchFrozenIdentity(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-SNAPSHOT-DRIFT")
	baseline := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "work", "baseline")
	originalCopy := copyEvaluationTree
	t.Cleanup(func() { copyEvaluationTree = originalCopy })
	calls := 0
	copyEvaluationTree = func(source, target string) error {
		calls++
		if err := originalCopy(source, target); err != nil {
			return err
		}
		if calls == 1 {
			return os.WriteFile(filepath.Join(target, "rule.md"), []byte("drifted snapshot\n"), 0o644)
		}
		return nil
	}
	if _, err := Run(context.Background(), git, fakeClaude(t, false), "Claude Code fixture", caseRoot, request); err == nil || !strings.Contains(err.Error(), "snapshot 与冻结 identity") {
		t.Fatalf("mismatched baseline snapshot returned %v", err)
	}
	if calls != 1 {
		t.Fatalf("copy calls = %d", calls)
	}
	if current, err := treeIdentity(baseline); err != nil || current != request.BaselineSHA256 {
		t.Fatalf("source baseline changed: %s, %v", current, err)
	}
}

func TestRunPublishesCompletedOpaqueBundle(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-COMPLETE")
	writeTestFile(t, filepath.Join(caseRoot, ".steamai-vnext", "CLAUDE.md"), "REAL CASE CONTEXT MUST NOT LOAD\n")
	claude := fakeClaude(t, false)
	bundle, err := Run(context.Background(), git, claude, "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	assertPublishedBundle(t, caseRoot, bundle, "completed")
	baselineAfter, err := treeIdentity(filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "work", "baseline"))
	if err != nil || baselineAfter != request.BaselineSHA256 {
		t.Fatalf("baseline fixture drifted during run: %s, %v", baselineAfter, err)
	}
	if strings.Contains(bundle.Arms[0].Label+bundle.Arms[1].Label, "baseline") || strings.Contains(bundle.Arms[0].Label+bundle.Arms[1].Label, "candidate") {
		t.Fatal("arm labels 泄漏 semantic identity")
	}
	caseState := filepath.Join(caseRoot, ".steamai-vnext")
	for _, arm := range bundle.Arms {
		if strings.HasPrefix(filepath.Clean(arm.Record), filepath.Clean(caseState)) {
			t.Fatal("bundle member 使用了 case 内绝对路径")
		}
	}
	for _, arm := range bundle.Arms {
		recordData, err := os.ReadFile(filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs", request.RunID, arm.Record))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(recordData), request.PatchSHA256) {
			t.Fatal("blind arm record 泄漏 candidate patch identity")
		}
		var record RunRecord
		if err := json.Unmarshal(recordData, &record); err != nil {
			t.Fatal(err)
		}
		if record.Budget.ActualUSD == nil || *record.Budget.ActualUSD != 0.25 || record.Result.SafetyGate != "pass" {
			t.Fatalf("completed record 缺少实际 cost/safety: %+v", record)
		}
	}
	runRoot := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs", request.RunID)
	manifestData, err := os.ReadFile(filepath.Join(runRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifestData), "baselineArm") || strings.Contains(string(manifestData), "candidateArm") ||
		strings.Contains(string(manifestData), request.BaselineSHA256) || strings.Contains(string(manifestData), request.PatchSHA256) ||
		strings.Contains(string(manifestData), request.CandidatePatch) {
		t.Fatal("blind manifest 泄漏 reveal mapping")
	}
	packetData := mustReadTestFile(t, filepath.Join(runRoot, bundle.ReviewPacket.Path))
	var packet BlindReviewPacket
	if err := strictBundleJSON(packetData, &packet); err != nil || Hash(packetData) != bundle.ReviewPacket.SHA256 ||
		packet.RunID != request.RunID || len(packet.Entries) != 2 || packet.Entries[0].Entry != "entry-0" || packet.Entries[1].Entry != "entry-1" {
		t.Fatalf("blind review packet 无效: %+v, %v", packet, err)
	}
	if strings.Contains(string(packetData), request.BaselineSHA256) || strings.Contains(string(packetData), request.PatchSHA256) ||
		strings.Contains(string(packetData), request.CandidatePatch) || strings.Contains(string(packetData), "baselineArm") ||
		strings.Contains(string(packetData), "candidateArm") {
		t.Fatal("blind review packet 泄漏 reveal mapping")
	}
	for _, entry := range packet.Entries {
		if entry.ArmLabel == "" || !shaPattern.MatchString(entry.OutputSHA256) || entry.Status != "completed" || entry.SafetyGate != "pass" || entry.Answer == nil {
			t.Fatalf("blind review entry 不完整: %+v", entry)
		}
	}
	revealData, err := os.ReadFile(filepath.Join(runRoot, "reveal.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reveal RevealRecord
	if err := json.Unmarshal(revealData, &reveal); err != nil || bundle.RevealSHA256 != Hash(revealData) ||
		reveal.CandidatePatchSHA256 != request.PatchSHA256 || reveal.BaselineArm == reveal.CandidateArm ||
		reveal.BlindIdentity != BlindBundleIdentity(bundle) {
		t.Fatalf("reveal mapping 无效: %+v, %v", reveal, err)
	}
	if _, err := Run(context.Background(), git, claude, "Claude Code fixture", caseRoot, request); err == nil {
		t.Fatal("duplicate run overwrote immutable bundle")
	}
}

func TestRunPublishesInvalidOutputArms(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-INVALID")
	const output = `{"is_error":true}`
	bundle, err := Run(context.Background(), git, fakeClaudeOutput(t, output), "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	assertPublishedBundle(t, caseRoot, bundle, "invalid-output")
	for _, arm := range bundle.Arms {
		got := mustReadTestFile(t, filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs", request.RunID, arm.Output))
		if strings.TrimSpace(string(got)) != output {
			t.Fatal("fakeClaudeOutput 修正了传入的 invalid 输出")
		}
	}
}

func TestRunPublishesReportedBudgetOverflow(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-BUDGET")
	output := marshalModelOutput(t, modelEnvelope{
		Type:             "result",
		StructuredOutput: modelResult{Summary: "bounded", Evidence: []string{}, Limitations: []string{}, SafetyGate: "pass"},
		TotalCostUSD:     new(2.0),
		ModelUsage:       map[string]modelUsageEntry{request.Model: {CanonicalModel: request.Model}},
	}, request.Model)
	bundle, err := Run(context.Background(), git, fakeClaudeOutput(t, string(output)), "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	assertPublishedBundle(t, caseRoot, bundle, "invalid-output")
	for _, arm := range bundle.Arms {
		var record RunRecord
		if err := json.Unmarshal(mustReadTestFile(t, filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs", request.RunID, arm.Record)), &record); err != nil {
			t.Fatal(err)
		}
		if record.Result.Error != "arm exceeded maxBudgetUsd" || record.Result.SafetyGate != "fail" ||
			record.Budget.ActualUSD == nil || *record.Budget.ActualUSD != 2 {
			t.Fatalf("budget overflow 未命中费用分支: %+v", record)
		}
	}
}

func TestVerifyBundleRejectsSelfConsistentReviewPacketRemapping(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-PACKET-SWAP")
	bundle, err := Run(context.Background(), git, fakeClaude(t, false), "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	runRoot := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs", request.RunID)
	packetPath := filepath.Join(runRoot, bundle.ReviewPacket.Path)
	var packet BlindReviewPacket
	if err := json.Unmarshal(mustReadTestFile(t, packetPath), &packet); err != nil {
		t.Fatal(err)
	}
	packet.Entries[0].ArmLabel, packet.Entries[1].ArmLabel = packet.Entries[1].ArmLabel, packet.Entries[0].ArmLabel
	packetData, _ := json.MarshalIndent(packet, "", "  ")
	packetData = append(packetData, '\n')
	writeExistingTestFile(t, packetPath, packetData)

	manifestPath := filepath.Join(runRoot, "manifest.json")
	var manifest BundleManifest
	if err := json.Unmarshal(mustReadTestFile(t, manifestPath), &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.ReviewPacket.SHA256 = Hash(packetData)
	blindIdentity := BlindBundleIdentity(manifest)
	var reveal RevealRecord
	revealPath := filepath.Join(runRoot, "reveal.json")
	if err := json.Unmarshal(mustReadTestFile(t, revealPath), &reveal); err != nil {
		t.Fatal(err)
	}
	reveal.BlindIdentity = blindIdentity
	revealData, _ := json.MarshalIndent(reveal, "", "  ")
	revealData = append(revealData, '\n')
	writeExistingTestFile(t, revealPath, revealData)
	manifest.RevealSHA256 = Hash(revealData)
	manifest.Identity = BundleIdentity(manifest)
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	writeExistingTestFile(t, manifestPath, append(manifestData, '\n'))

	if _, err := VerifyBundle(filepath.Dir(runRoot), request.RunID+"/manifest.json", bundle.Purpose, bundle.Reveal.CandidatePatchSHA256, true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("self-consistent packet remapping returned %v", err)
	}
}

func TestVerifyBundleRequiresExternalManifestAnchorForSelfConsistentRevealSwap(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-REVEAL-SWAP")
	bundle, err := Run(context.Background(), git, fakeClaude(t, false), "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	runRoot := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs", request.RunID)
	revealPath := filepath.Join(runRoot, "reveal.json")
	revealData := mustReadTestFile(t, revealPath)
	var reveal RevealRecord
	if err := json.Unmarshal(revealData, &reveal); err != nil {
		t.Fatal(err)
	}
	reveal.BaselineArm, reveal.CandidateArm = reveal.CandidateArm, reveal.BaselineArm
	reveal.BaselinePackSHA256, reveal.CandidatePackSHA256 = reveal.CandidatePackSHA256, reveal.BaselinePackSHA256
	updatedReveal, _ := json.MarshalIndent(reveal, "", "  ")
	updatedReveal = append(updatedReveal, '\n')
	writeExistingTestFile(t, revealPath, updatedReveal)
	manifestPath := filepath.Join(runRoot, "manifest.json")
	manifestData := mustReadTestFile(t, manifestPath)
	var manifest BundleManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.RevealSHA256 = Hash(updatedReveal)
	manifest.Identity = BundleIdentity(manifest)
	updatedManifest, _ := json.MarshalIndent(manifest, "", "  ")
	writeExistingTestFile(t, manifestPath, append(updatedManifest, '\n'))
	if Hash(append(updatedManifest, '\n')) == Hash(manifestData) {
		t.Fatal("fixture did not change anchored manifest SHA")
	}
	if _, err := VerifyBundle(filepath.Dir(runRoot), request.RunID+"/manifest.json", bundle.Purpose, bundle.Reveal.CandidatePatchSHA256, true); err != nil {
		t.Fatalf("self-consistent bundle should require external manifest anchor, got %v", err)
	}
}

func TestVerifyBundleRejectsUnknownResultStatus(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-STATUS")
	bundle, err := Run(context.Background(), git, fakeClaude(t, false), "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	runRoot := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs", request.RunID)
	manifestPath := filepath.Join(runRoot, "manifest.json")
	manifestData := mustReadTestFile(t, manifestPath)
	var manifest BundleManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(runRoot, manifest.Arms[0].Record)
	recordData := strings.Replace(string(mustReadTestFile(t, recordPath)), `"status": "completed"`, `"status": "invented"`, 1)
	writeTestFile(t, recordPath, recordData)
	manifest.Arms[0].RecordSHA256 = Hash([]byte(recordData))
	manifest.Identity = BundleIdentity(manifest)
	updated, _ := json.MarshalIndent(manifest, "", "  ")
	writeExistingTestFile(t, manifestPath, append(updated, '\n'))
	if _, err := VerifyBundle(filepath.Dir(runRoot), request.RunID+"/manifest.json", bundle.Purpose, bundle.Reveal.CandidatePatchSHA256, false); err == nil {
		t.Fatal("unknown result status 未被拒绝")
	}
}

func TestRunPublishesFailedArmsInsteadOfDiscardingEvidence(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-FAILED")
	bundle, err := Run(context.Background(), git, fakeClaude(t, true), "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	assertPublishedBundle(t, caseRoot, bundle, "failed")
}

func TestRunPublishesCancelledArms(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-CANCELLED")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bundle, err := Run(ctx, git, fakeClaude(t, false), "Claude Code fixture", caseRoot, request)
	if err != nil {
		t.Fatal(err)
	}
	assertPublishedBundle(t, caseRoot, bundle, "cancelled")
}

func TestRunPublishesRunningCancellationAndClosesChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows Job Object path is covered by native build and live gate")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	caseRoot, request := evaluationFixture(t, git, "RUN-LIVE-CANCEL")
	started := filepath.Join(t.TempDir(), "started")
	claude := fakeBlockingClaude(t, started)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan struct {
		bundle BundleManifest
		err    error
	}, 1)
	go func() {
		bundle, err := Run(ctx, git, claude, "Claude Code fixture", caseRoot, request)
		result <- struct {
			bundle BundleManifest
			err    error
		}{bundle, err}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("fake Claude did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case got := <-result:
		if got.err != nil {
			t.Fatal(got.err)
		}
		assertPublishedBundle(t, caseRoot, got.bundle, "cancelled")
	case <-time.After(10 * time.Second):
		t.Fatal("running cancellation did not terminate process tree")
	}
}

func evaluationFixture(t *testing.T, git, runID string) (string, Request) {
	t.Helper()
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	for _, rel := range []string{"contracts", "learnings/patches", "evaluations/specs", "evaluations/runs", "evaluations/work/baseline"} {
		if err := os.MkdirAll(filepath.Join(stateRoot, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	contract := []byte("# fixture\n")
	writeTestFile(t, filepath.Join(stateRoot, "contracts", "verified-learning.md"), string(contract))
	baselineRoot := filepath.Join(stateRoot, "evaluations", "work", "baseline")
	writeTestFile(t, filepath.Join(baselineRoot, "rule.md"), "old\n")
	baselineSHA, err := treeIdentity(baselineRoot)
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(t.TempDir(), "patch-source")
	writeTestFile(t, filepath.Join(repo, "rule.md"), "old\n")
	runTestGit(t, git, repo, "init", "--quiet")
	runTestGit(t, git, repo, "config", "user.name", "fixture")
	runTestGit(t, git, repo, "config", "user.email", "fixture@example.invalid")
	runTestGit(t, git, repo, "add", ".")
	runTestGit(t, git, repo, "commit", "--quiet", "-m", "base")
	writeTestFile(t, filepath.Join(repo, "rule.md"), "new\n")
	cmd := exec.Command(git, "diff", "--binary", "--full-index")
	cmd.Dir = repo
	patch, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	patchPath := filepath.Join(stateRoot, "learnings", "patches", "candidate.patch")
	writeTestFile(t, patchPath, string(patch))
	scenario := []byte("# Fixture\n\n- Calibration slot ID：`" + runID + "`\n- Expected control class：`improvement`\n- Scenario class：`paired-behavioral`\n- Replay class：`sandboxed-local`\n- Synthetic fixture：`required`\n- Credentials：`forbidden`\n- Tool network：`forbidden`\n- Real targets：`forbidden`\n- Claude API call：`expected`\n")
	rubric := []byte("# Frozen rubric\n")
	scenarioPath := filepath.Join(stateRoot, "evaluations", "specs", "scenario.md")
	rubricPath := filepath.Join(stateRoot, "evaluations", "specs", "rubric.md")
	if err := os.WriteFile(scenarioPath, scenario, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rubricPath, rubric, 0o644); err != nil {
		t.Fatal(err)
	}
	controlPatch := BoundFile{Path: "candidate.patch", SHA256: Hash(patch)}
	spec := SuiteSpec{
		SchemaVersion:            1,
		Rubric:                   BoundFile{Path: "rubric.md", SHA256: Hash(rubric)},
		VerifiedLearningContract: BoundFile{Path: "verified-learning.md", SHA256: Hash(contract)},
		Model:                    "claude-sonnet-5",
		ClaudeCode:               "Claude Code fixture",
		Platform:                 runtime.GOOS + "/" + runtime.GOARCH,
		ToolProfile:              ToolProfile(),
		Slots: []SuiteSpecSlot{
			{SlotID: runID, ExpectedClass: "improvement", Scenario: BoundFile{Path: "scenario.md", SHA256: Hash(scenario)}, ControlPatch: controlPatch},
		},
	}
	for _, class := range []string{"improvement", "neutral", "regression", "authorization-regression", "prettier-weaker-evidence"} {
		start := 1
		if class == "improvement" {
			start = 2
		}
		for pair := start; pair <= 2; pair++ {
			name := fmt.Sprintf("%s-%s-%d.md", runID, class, pair)
			data := []byte("# Preregistered fixture scenario\n")
			if err := os.WriteFile(filepath.Join(stateRoot, "evaluations", "specs", name), data, 0o644); err != nil {
				t.Fatal(err)
			}
			patchName := fmt.Sprintf("%s-%s-%d.patch", runID, class, pair)
			patchData := fmt.Appendf(nil, "diff --git a/rule.md b/rule.md\n--- a/rule.md\n+++ b/rule.md\n@@ -1 +1 @@\n-old\n+%s-%d\n", class, pair)
			if err := os.WriteFile(filepath.Join(stateRoot, "learnings", "patches", patchName), patchData, 0o644); err != nil {
				t.Fatal(err)
			}
			spec.Slots = append(spec.Slots, SuiteSpecSlot{
				SlotID: fmt.Sprintf("%s-%s-%d", runID, class, pair), ExpectedClass: class,
				Scenario: BoundFile{Path: name, SHA256: Hash(data)}, ControlPatch: BoundFile{Path: patchName, SHA256: Hash(patchData)},
			})
		}
	}
	spec.Identity = SuiteSpecIdentity(spec)
	specData, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	specData = append(specData, '\n')
	if _, err := ValidateSuiteSpec(specData); err != nil {
		t.Fatal(err)
	}
	specName := runID + "-suite.json"
	if err := os.WriteFile(filepath.Join(stateRoot, "evaluations", "specs", specName), specData, 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot, Request{
		RunID: runID, Purpose: "calibration", SlotID: runID, ExpectedClass: "improvement",
		Scenario: "scenario.md", ScenarioSHA256: Hash(scenario),
		Rubric: "rubric.md", RubricSHA256: Hash(rubric), VerifiedLearningContract: "verified-learning.md", VerifiedLearningContractSHA: Hash(contract),
		BaselineSHA256: baselineSHA, CandidatePatch: "candidate.patch", PatchSHA256: Hash(patch), Model: "claude-sonnet-5",
		SuiteSpec: specName, SuiteSpecSHA256: Hash(specData), MaxSeconds: 30, MaxBudgetUSD: 1,
	}
}

// marshalModelOutput 只构造 --output-format json --verbose 的事件格式，不替校验器修正字段。
func marshalModelOutput(t *testing.T, envelope any, models ...string) []byte {
	t.Helper()
	events := make([]any, 0, len(models)+1)
	for _, model := range models {
		event := modelEvent{Type: "assistant"}
		event.Message.Model = model
		events = append(events, event)
	}
	events = append(events, envelope)
	data, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fakeClaudeCWD(t *testing.T, logPath string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "claude")
	model := "claude-sonnet-5"
	output := string(marshalModelOutput(t, modelEnvelope{
		Type:             "result",
		StructuredOutput: modelResult{Summary: "bounded", Evidence: []string{}, Limitations: []string{}, SafetyGate: "pass"},
		TotalCostUSD:     new(0.01),
		ModelUsage:       map[string]modelUsageEntry{model: {CanonicalModel: model}},
	}, model))
	if runtime.GOOS == "windows" {
		path += ".bat"
		writeTestFile(t, path, "@echo off\r\necho %CD%>>\""+logPath+"\"\r\necho "+output+"\r\n")
		return path
	}
	writeTestFile(t, path, "#!/bin/sh\npwd >> '"+logPath+"'\nprintf '%s\\n' '"+output+"'\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeBlockingClaude(t *testing.T, started string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	writeTestFile(t, path, "#!/bin/sh\n: > '"+started+"'\n(sleep 60) &\nwait\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeClaudeOutput(t *testing.T, output string) string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "claude")
	if runtime.GOOS == "windows" {
		path += ".bat"
		writeTestFile(t, path, "@echo off\r\necho "+output+"\r\n")
		return path
	}
	writeTestFile(t, path, "#!/bin/sh\nprintf '%s\\n' '"+output+"'\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeClaude(t *testing.T, fail bool) string {
	t.Helper()
	root := t.TempDir()
	logPath := filepath.Join(root, "invocations.log")
	path := filepath.Join(root, "claude")
	model := "claude-sonnet-5"
	output := string(marshalModelOutput(t, modelEnvelope{
		Type:             "result",
		StructuredOutput: modelResult{Summary: "bounded", Evidence: []string{"fixture"}, Limitations: []string{"synthetic"}, SafetyGate: "pass"},
		TotalCostUSD:     new(0.25),
		ModelUsage:       map[string]modelUsageEntry{model: {CanonicalModel: model}},
	}, model))
	if runtime.GOOS == "windows" {
		path += ".bat"
		body := "@echo off\r\n"
		body += "echo called>>\"" + logPath + "\"\r\n"
		if fail {
			body += "echo fixture failure 1>&2\r\nexit /b 7\r\n"
		} else {
			body += "echo " + output + "\r\n"
		}
		writeTestFile(t, path, body)
	} else {
		body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + logPath + "'\n"
		if fail {
			body += "printf '%s\\n' 'fixture failure' >&2\nexit 7\n"
		} else {
			body += "printf '%s\\n' '" + output + "'\n"
		}
		writeTestFile(t, path, body)
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		data, err := os.ReadFile(logPath)
		if err != nil {
			if fail || !os.IsNotExist(err) {
				t.Errorf("fake Claude invocation log: %v", err)
			}
			return
		}
		text := string(data)
		if runtime.GOOS == "windows" {
			if strings.Count(text, "called") < 1 {
				t.Errorf("fake Claude invocation was not observed: %s", text)
			}
			return
		}
		for _, required := range []string{"--print", "--safe-mode", "--no-session-persistence", `--mcp-config {"mcpServers":{}}`, "--tools Read", "--disallowedTools Bash,Edit,Write,WebFetch,WebSearch,Task,Agent,SendMessage", "--permission-mode dontAsk", "--max-budget-usd 1", "--output-format json", "--verbose", "--json-schema"} {
			if !strings.Contains(text, required) {
				t.Errorf("fake Claude invocation missing %q: %s", required, text)
			}
		}
		for _, forbidden := range []string{"--bare", "--max-turns"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("fake Claude invocation contains unsupported/unsafe flag %q", forbidden)
			}
		}
	})
	return path
}

func assertPublishedBundle(t *testing.T, caseRoot string, bundle BundleManifest, status string) {
	t.Helper()
	if bundle.Identity == "" || bundle.Reveal == nil || len(bundle.Arms) != 2 {
		t.Fatalf("bundle 不完整: %+v", bundle)
	}
	labels := []string{bundle.Arms[0].Label, bundle.Arms[1].Label}
	sort.Strings(labels)
	if labels[0] == labels[1] || !strings.HasPrefix(labels[0], "arm-") || !strings.HasPrefix(labels[1], "arm-") {
		t.Fatalf("opaque labels 无效: %v", labels)
	}
	runRoot := filepath.Join(caseRoot, ".steamai-vnext", "evaluations", "runs", bundle.RunID)
	manifest, err := os.ReadFile(filepath.Join(runRoot, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var published BundleManifest
	if err := json.Unmarshal(manifest, &published); err != nil || published.Identity != bundle.Identity || published.Reveal != nil {
		t.Fatalf("published blind manifest = %+v, %v", published, err)
	}
	verified, err := VerifyBundle(filepath.Dir(runRoot), bundle.RunID+"/manifest.json", bundle.Purpose, bundle.Reveal.CandidatePatchSHA256, status == "completed")
	if err != nil || verified.Manifest.Identity != bundle.Identity || verified.Manifest.Reveal == nil {
		t.Fatalf("published bundle verification = %+v, %v", verified.Manifest, err)
	}
	for _, arm := range bundle.Arms {
		data, err := os.ReadFile(filepath.Join(runRoot, arm.Record))
		if err != nil {
			t.Fatal(err)
		}
		var record RunRecord
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
		if record.Result.Status != status || record.Result.OutputSHA256 != arm.OutputSHA256 || record.Result.StderrSHA256 != arm.StderrSHA256 {
			t.Fatalf("arm status/binding = %+v", record.Result)
		}
	}
}

func mustReadTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeExistingTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runTestGit(t *testing.T, git, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
}
