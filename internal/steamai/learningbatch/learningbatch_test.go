package learningbatch

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

	"github.com/shuiyu486/STeamAI/internal/steamai/casebootstrap"
	"github.com/shuiyu486/STeamAI/internal/steamai/evaluation"
)

func TestMultiCandidateMultiTargetPreviewAndApply(t *testing.T) {
	fixture := newBatchFixture(t)
	beforeHead := runGit(t, fixture.git, fixture.source, "rev-parse", "HEAD")
	beforeIndex := runGit(t, fixture.git, fixture.source, "write-tree")
	beforeSnapshot := mustCurrent(t, fixture.caseRoot).PayloadDigest

	preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 3 || len(preview.Targets) != 2 {
		t.Fatalf("unexpected batch shape: %d candidates, %d targets", len(preview.Candidates), len(preview.Targets))
	}
	if !strings.Contains(preview.HumanPreview, "diff --git a/packs/fixture-pack/method-a.md") ||
		!strings.Contains(preview.HumanPreview, "source-finding:findings/F-001.md sha256:") ||
		!strings.Contains(preview.HumanPreview, "reviewer:reviewer") ||
		!strings.Contains(preview.HumanPreview, ConfirmationPrefix+preview.Identity) {
		t.Fatal("preview 未展示完整 patch、source chain、Reviewer 或 exact confirmation")
	}
	for _, target := range preview.Targets {
		data := mustRead(t, filepath.Join(fixture.source, filepath.FromSlash(target.Path)))
		if hashBytes(data) != target.PreSHA256 {
			t.Fatal("preview 在确认前修改了 canonical target")
		}
	}
	if _, err := Apply(fixture.git, fixture.source, fixture.caseRoot, fixture.request, "确认"); err != ErrConfirmationRequired {
		t.Fatalf("non-exact confirmation returned %v", err)
	}
	if _, err := Apply(fixture.git, fixture.source, fixture.caseRoot, fixture.request, ConfirmationPrefix+preview.Identity); err != nil {
		t.Fatal(err)
	}
	for _, target := range preview.Targets {
		data := mustRead(t, filepath.Join(fixture.source, filepath.FromSlash(target.Path)))
		if hashBytes(data) != target.PostSHA256 {
			t.Fatalf("postimage mismatch: %s", target.Path)
		}
	}
	if got := runGit(t, fixture.git, fixture.source, "rev-parse", "HEAD"); got != beforeHead {
		t.Fatal("apply 修改了 HEAD")
	}
	if got := runGit(t, fixture.git, fixture.source, "write-tree"); got != beforeIndex {
		t.Fatal("apply 修改了 index")
	}
	if got := mustCurrent(t, fixture.caseRoot).PayloadDigest; got != beforeSnapshot {
		t.Fatal("apply 修改了 case snapshot")
	}
}

func TestPatchScopeAllowsTargetsWithSamePreimageBlob(t *testing.T) {
	fixture := newBatchFixture(t)
	first := filepath.Join(fixture.source, "packs", "fixture-pack", "method-a.md")
	second := filepath.Join(fixture.source, "packs", "fixture-pack", "method-b.md")
	data := mustRead(t, first)
	writeFile(t, second, data)
	runGit(t, fixture.git, fixture.source, "add", "--", "packs/fixture-pack/method-b.md")
	runGit(t, fixture.git, fixture.source, "commit", "--quiet", "-m", "equal preimages")

	proposal := filepath.Join(filepath.Dir(fixture.source), "same-preimage-proposal")
	runGit(t, fixture.git, filepath.Dir(fixture.source), "clone", "--quiet", "--no-local", fixture.source, proposal)
	writeFile(t, filepath.Join(proposal, "packs", "fixture-pack", "method-a.md"), append(data, []byte("- First change.\n")...))
	writeFile(t, filepath.Join(proposal, "packs", "fixture-pack", "method-b.md"), append(data, []byte("- Second change.\n")...))
	patchPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", filepath.FromSlash(fixture.request.Patch))
	patch := runGitRaw(t, fixture.git, proposal, "diff", "--binary", "--full-index", "--no-ext-diff", "--", "packs/fixture-pack/method-a.md", "packs/fixture-pack/method-b.md")
	writeFile(t, patchPath, patch)

	targets, err := validatePatchScope(fixture.git, fixture.source, patchPath, "fixture-pack", []string{"method-*.md"}, []string{"forbidden-marker"})
	if err != nil {
		t.Fatalf("same-preimage multi-target patch rejected: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("targets = %#v", targets)
	}
}

func TestPatchScopeRequiresFullIndexCurrentPreimage(t *testing.T) {
	fixture := newBatchFixture(t)
	patchPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", filepath.FromSlash(fixture.request.Patch))
	patch := string(mustRead(t, patchPath))
	lines := strings.Split(patch, "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "index ") {
			parts := strings.Fields(line)
			ids := strings.Split(parts[1], "..")
			lines[index] = "index " + ids[0][:7] + ".." + ids[1][:7] + " " + parts[2]
			break
		}
	}
	writeFile(t, patchPath, []byte(strings.Join(lines, "\n")))
	if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
		t.Fatal("缺少 full-index identity 的 patch 未被拒绝")
	}
}

func TestPatchScopeRejectsAbbreviatedNewBlob(t *testing.T) {
	fixture := newBatchFixture(t)
	patchPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", filepath.FromSlash(fixture.request.Patch))
	lines := strings.Split(string(mustRead(t, patchPath)), "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, "index ") {
			parts := strings.Fields(line)
			ids := strings.Split(parts[1], "..")
			lines[index] = "index " + ids[0] + ".." + ids[1][:7] + " " + parts[2]
			break
		}
	}
	writeFile(t, patchPath, []byte(strings.Join(lines, "\n")))
	if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
		t.Fatal("缩写 new blob identity 的 patch 未被拒绝")
	}
}

func TestApplyUsesConfirmedPatchBytes(t *testing.T) {
	fixture := newBatchFixture(t)
	preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	patchPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", filepath.FromSlash(fixture.request.Patch))
	writeFile(t, patchPath, append(mustRead(t, patchPath), []byte("\n# replaced after preview\n")...))
	if err := applyPreview(fixture.git, fixture.source, fixture.caseRoot, preview); err != nil {
		t.Fatal(err)
	}
	for _, target := range preview.Targets {
		data := mustRead(t, filepath.Join(fixture.source, filepath.FromSlash(target.Path)))
		if hashBytes(data) != target.PostSHA256 {
			t.Fatalf("confirmed patch postimage mismatch: %s", target.Path)
		}
	}
}

func TestApplyRejectsPreviewBaselineDrift(t *testing.T) {
	fixture := newBatchFixture(t)
	preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	preview.CanonicalHead = strings.Repeat("f", 40)
	if err := applyPreview(fixture.git, fixture.source, fixture.caseRoot, preview); err == nil {
		t.Fatal("canonical HEAD 与 preview 不匹配时仍执行 Apply")
	}
	preview.CanonicalHead = runGit(t, fixture.git, fixture.source, "rev-parse", "HEAD")
	preview.SnapshotDigest = "sha256:" + strings.Repeat("e", 64)
	if err := applyPreview(fixture.git, fixture.source, fixture.caseRoot, preview); err == nil {
		t.Fatal("case snapshot 与 preview 不匹配时仍执行 Apply")
	}
}

func TestSourceReviewRequiresCompleteCurrentRound(t *testing.T) {
	for _, missing := range []string{"- Confidence：`high`\n", "### 判断\n\nSynthetic.\n", "### 风险或缺口\n\nFixture only.\n", "### 下一步\n\nUse only for synthetic tests.\n"} {
		t.Run(strings.Fields(missing)[0], func(t *testing.T) {
			fixture := newBatchFixture(t)
			path := filepath.Join(fixture.caseRoot, ".steamai-vnext", "reviews", "R-001.md")
			data := strings.Replace(string(mustRead(t, path)), missing, "", 1)
			writeFile(t, path, []byte(data))
			if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
				t.Fatalf("缺少 review round 内容 %q 时仍接受 source chain", missing)
			}
		})
	}
}

func TestApplyRollsBackOnlyBatchTargetsAfterPostimageFailure(t *testing.T) {
	fixture := newBatchFixture(t)
	preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
	if err != nil {
		t.Fatal(err)
	}
	preimages := map[string][]byte{}
	for _, target := range preview.Targets {
		preimages[target.Path] = append([]byte(nil), preview.targetData[target.Path]...)
	}
	nonTarget := filepath.Join(fixture.source, "packs", "fixture-pack", "router.md")
	nonTargetData := append(mustRead(t, nonTarget), []byte("local working-tree note\n")...)
	writeFile(t, nonTarget, nonTargetData)
	preview.Targets[0].PostSHA256 = strings.Repeat("f", 64)
	if err := applyPreview(fixture.git, fixture.source, fixture.caseRoot, preview); err == nil {
		t.Fatal("postimage failure 未触发 rollback")
	}
	for path, want := range preimages {
		if got := mustRead(t, filepath.Join(fixture.source, filepath.FromSlash(path))); string(got) != string(want) {
			t.Fatalf("target %s 未恢复 exact preimage", path)
		}
	}
	if got := mustRead(t, nonTarget); string(got) != string(nonTargetData) {
		t.Fatal("rollback 修改了非 batch target")
	}
}

func TestPreVerifiedLearningCaseFailsWithExplicitCapabilityBoundary(t *testing.T) {
	fixture := newBatchFixture(t)
	contract := ".steamai-vnext/contracts/verified-learning.md"
	if err := os.Remove(filepath.Join(fixture.caseRoot, filepath.FromSlash(contract))); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"evaluations/specs", "evaluations/runs", "evaluations/attestations", "evaluations/outcomes", "evaluations/work"} {
		if err := os.RemoveAll(filepath.Join(fixture.caseRoot, ".steamai-vnext", filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}
	snapshotPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", "pack-snapshot", "snapshot.yml")
	snapshot := strings.Split(string(mustRead(t, snapshotPath)), "\n")
	filtered := make([]string, 0, len(snapshot))
	for index := 0; index < len(snapshot); index++ {
		if snapshot[index] == "  - path: "+contract {
			index += 2
			continue
		}
		filtered = append(filtered, snapshot[index])
	}
	writeFile(t, snapshotPath, []byte(strings.Join(filtered, "\n")))
	if err := casebootstrap.ValidateCurrent(fixture.caseRoot); err != nil {
		t.Fatalf("pre-verified-learning case should remain current: %v", err)
	}
	before := append([]byte(nil), mustRead(t, filepath.Join(fixture.source, "packs", "fixture-pack", "method-a.md"))...)
	if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); !errors.Is(err, ErrVerifiedLearningUnavailable) {
		t.Fatalf("pre-verified-learning preview returned %v", err)
	}
	if _, err := Apply(fixture.git, fixture.source, fixture.caseRoot, fixture.request, ConfirmationPrefix+"unused"); !errors.Is(err, ErrVerifiedLearningUnavailable) {
		t.Fatalf("pre-verified-learning apply returned %v", err)
	}
	if got := mustRead(t, filepath.Join(fixture.source, "packs", "fixture-pack", "method-a.md")); !bytes.Equal(got, before) {
		t.Fatal("pre-verified-learning rejection changed canonical target")
	}
}

func TestBehavioralCandidateRequiresGate3Bindings(t *testing.T) {
	fixture := newBatchFixture(t)
	candidatePath := filepath.Join(fixture.caseRoot, ".steamai-vnext", "learnings", "candidates", "L-001.md")
	candidate := strings.Replace(string(mustRead(t, candidatePath)), "Claim kind：`mechanical`", "Claim kind：`behavioral`", 1)
	candidate = strings.Replace(candidate, "Required maturity：`V1`", "Required maturity：`V3`", 1)
	writeFile(t, candidatePath, []byte(candidate))

	reviewPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", "reviews", "R-L-001.md")
	review := string(mustRead(t, reviewPath))
	oldSHAStart := strings.Index(review, "- Candidate SHA-256：`")
	if oldSHAStart < 0 {
		t.Fatal("fixture review missing candidate SHA")
	}
	oldSHAStart += len("- Candidate SHA-256：`")
	review = review[:oldSHAStart] + hashBytes([]byte(candidate)) + review[oldSHAStart+64:]
	review = strings.Replace(review, "Claim kind：`mechanical`", "Claim kind：`behavioral`", 1)
	review = strings.Replace(review, "Required maturity：`V1`", "Required maturity：`V3`", 1)
	writeFile(t, reviewPath, []byte(review))

	batchPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", filepath.FromSlash(fixture.request.BatchReview))
	batch := string(mustRead(t, batchPath))
	batch = strings.Replace(batch, "- Candidate SHA-256：`"+fixtureCandidateSHA(t, batch, "L-001.md")+"`", "- Candidate SHA-256：`"+hashBytes([]byte(candidate))+"`", 1)
	batch = strings.Replace(batch, "- Claim kind：`mechanical`\n- Required maturity：`V1`", "- Claim kind：`behavioral`\n- Required maturity：`V3`", 1)
	batch = strings.Replace(batch, "- Eligibility review SHA-256：`"+fixtureReviewSHA(t, batch, "R-L-001.md")+"`", "- Eligibility review SHA-256：`"+hashBytes([]byte(review))+"`", 1)
	writeFile(t, batchPath, []byte(batch))

	if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); !errors.Is(err, ErrBinding) {
		t.Fatalf("behavioral candidate without Gate 3 returned %v", err)
	}
}

func TestCalibrationSuiteRejectsMismatchedObservedClass(t *testing.T) {
	fixture := newBatchFixture(t)
	makeBehavioralFixture(t, &fixture)
	suitePath := filepath.Join(fixture.caseRoot, ".steamai-vnext", "evaluations", "runs", "CAL-SUITE.json")
	data := strings.Replace(string(mustRead(t, suitePath)), `"observedClass": "rejected"`, `"observedClass": "inconclusive"`, 1)
	writeFile(t, suitePath, []byte(data))
	if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); !errors.Is(err, ErrBinding) {
		t.Fatalf("inconclusive calibration returned %v", err)
	}
}

func TestBehavioralCandidateRejectsCalibrationControlPatchPathReuse(t *testing.T) {
	spec := evaluation.SuiteSpec{Slots: []evaluation.SuiteSpecSlot{{ControlPatch: evaluation.BoundFile{Path: "LB-001.patch", SHA256: strings.Repeat("a", 64)}}}}
	if !calibrationReusesFinalPatch(spec, "learnings/patches/LB-001.patch", strings.Repeat("b", 64)) {
		t.Fatal("calibration control/final patch path reuse was accepted")
	}
	if !calibrationReusesFinalPatch(spec, "learnings/patches/LB-002.patch", strings.Repeat("a", 64)) {
		t.Fatal("calibration control/final patch SHA reuse was accepted")
	}
	if calibrationReusesFinalPatch(spec, "learnings/patches/LB-002.patch", strings.Repeat("b", 64)) {
		t.Fatal("independent calibration control was rejected")
	}
}

func TestBehavioralCandidateRejectsAttestationRevealPathDrift(t *testing.T) {
	for name, values := range map[string][2]string{
		"calibration non-none": {"- Run bundle reveal：`none`", "- Run bundle reveal：`evaluations/runs/CAL-001/reveal.json`"},
		"candidate wrong path": {"- Run bundle reveal：`evaluations/runs/PROM-001/reveal.json`", "- Run bundle reveal：`evaluations/runs/OTHER/reveal.json`"},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newBatchFixture(t)
			makeBehavioralFixture(t, &fixture)
			stateRoot := filepath.Join(fixture.caseRoot, ".steamai-vnext")
			attestationRel := fixture.request.PromotionAttestation
			if name == "calibration non-none" {
				attestationRel = fixture.request.CalibrationAttestation
			}
			attestationPath := filepath.Join(stateRoot, filepath.FromSlash(attestationRel))
			original := mustRead(t, attestationPath)
			attestation := strings.Replace(string(original), values[0], values[1], 1)
			if attestation == string(original) {
				t.Fatal("fixture attestation reveal field was not replaced")
			}
			writeFile(t, attestationPath, []byte(attestation))
			bindingName := "Promotion attestation SHA-256"
			if name == "calibration non-none" {
				bindingName = "Calibration attestation SHA-256"
			}
			batchPath := filepath.Join(stateRoot, filepath.FromSlash(fixture.request.BatchReview))
			batch := string(mustRead(t, batchPath))
			oldBinding := "- " + bindingName + "：`" + hashBytes(original) + "`"
			newBinding := "- " + bindingName + "：`" + hashBytes([]byte(attestation)) + "`"
			updatedBatch := strings.Replace(batch, oldBinding, newBinding, 1)
			if updatedBatch == batch {
				t.Fatal("fixture batch attestation SHA was not replaced")
			}
			writeFile(t, batchPath, []byte(updatedBatch))
			if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); !errors.Is(err, ErrBinding) {
				t.Fatalf("attestation reveal path drift returned %v", err)
			}
		})
	}
}

func TestBehavioralCandidateAcceptsClosedGate3BundleAndRejectsTamper(t *testing.T) {
	t.Run("closed journey", func(t *testing.T) {
		fixture := newBatchFixture(t)
		makeBehavioralFixture(t, &fixture)
		preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
		if err != nil {
			t.Fatal(err)
		}
		if preview.Calibration == nil || preview.Promotion == nil || preview.RunBundle == nil || preview.RunBundle.RevealSHA256 == "" ||
			!strings.Contains(preview.HumanPreview, "reveal-sha256:"+preview.RunBundle.RevealSHA256) {
			t.Fatalf("behavioral preview 缺少 Gate 3 bindings: %+v", preview)
		}
		if _, err := Apply(fixture.git, fixture.source, fixture.caseRoot, fixture.request, ConfirmationPrefix+preview.Identity); err != nil {
			t.Fatal(err)
		}
	})
	for name, mutate := range map[string]func(string) string{
		"reveal":        func(path string) string { return filepath.Join(filepath.Dir(path), "reveal.json") },
		"review packet": func(path string) string { return filepath.Join(filepath.Dir(path), "blind-review.json") },
		"blind decision": func(path string) string {
			return filepath.Join(filepath.Dir(filepath.Dir(path)), "..", "attestations", "PROM-001-blind.md")
		},
		"output": func(path string) string {
			manifestData := mustRead(t, path)
			var manifest evaluation.BundleManifest
			if err := json.Unmarshal(manifestData, &manifest); err != nil {
				t.Fatal(err)
			}
			return filepath.Join(filepath.Dir(path), manifest.Arms[0].Output)
		},
	} {
		t.Run(name+" tamper", func(t *testing.T) {
			fixture := newBatchFixture(t)
			makeBehavioralFixture(t, &fixture)
			manifestPath := filepath.Join(fixture.caseRoot, ".steamai-vnext", "evaluations", "runs", "PROM-001", "manifest.json")
			path := mutate(manifestPath)
			writeFile(t, path, append(mustRead(t, path), []byte("tamper")...))
			if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); !errors.Is(err, ErrBinding) {
				t.Fatalf("tampered %s returned %v", name, err)
			}
		})
	}
}

func TestBehavioralCandidateRejectsSelfConsistentPreferredOutputMismatch(t *testing.T) {
	fixture := newBatchFixture(t)
	makeBehavioralFixture(t, &fixture)
	stateRoot := filepath.Join(fixture.caseRoot, ".steamai-vnext")
	blindPath := filepath.Join(stateRoot, "evaluations", "attestations", "PROM-001-blind.md")
	originalBlind := mustRead(t, blindPath)
	fields, err := fieldMap(originalBlind)
	if err != nil || !hexSHA.MatchString(fields["Preferred output SHA-256"]) {
		t.Fatalf("fixture preferred output field invalid: %v", err)
	}
	blind := strings.Replace(string(originalBlind), fields["Preferred output SHA-256"], strings.Repeat("f", 64), 1)
	if blind == string(originalBlind) {
		t.Fatal("fixture preferred output field was not replaced")
	}
	writeFile(t, blindPath, []byte(blind))

	promotionPath := filepath.Join(stateRoot, "evaluations", "attestations", "PROM-001.md")
	originalPromotion := mustRead(t, promotionPath)
	promotion := strings.Replace(string(originalPromotion), hashBytes(originalBlind), hashBytes([]byte(blind)), 1)
	if promotion == string(originalPromotion) {
		t.Fatal("fixture blind decision SHA was not replaced")
	}
	writeFile(t, promotionPath, []byte(promotion))

	batchPath := filepath.Join(stateRoot, filepath.FromSlash(fixture.request.BatchReview))
	batch := strings.Replace(string(mustRead(t, batchPath)), hashBytes(originalPromotion), hashBytes([]byte(promotion)), 1)
	writeFile(t, batchPath, []byte(batch))
	if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); !errors.Is(err, ErrBinding) {
		t.Fatalf("self-consistent preferred output mismatch returned %v", err)
	}
}

func makeBehavioralFixture(t *testing.T, fixture *batchFixture) {
	t.Helper()
	stateRoot := filepath.Join(fixture.caseRoot, ".steamai-vnext")
	candidatePath := filepath.Join(stateRoot, "learnings", "candidates", "L-001.md")
	candidate := strings.Replace(string(mustRead(t, candidatePath)), "Claim kind：`mechanical`", "Claim kind：`behavioral`", 1)
	candidate = strings.Replace(candidate, "Required maturity：`V1`", "Required maturity：`V3`", 1)
	writeFile(t, candidatePath, []byte(candidate))
	reviewPath := filepath.Join(stateRoot, "reviews", "R-L-001.md")
	review := string(mustRead(t, reviewPath))
	start := strings.Index(review, "- Candidate SHA-256：`")
	if start < 0 {
		t.Fatal("fixture review missing candidate SHA")
	}
	start += len("- Candidate SHA-256：`")
	review = review[:start] + hashBytes([]byte(candidate)) + review[start+64:]
	review = strings.Replace(review, "Claim kind：`mechanical`", "Claim kind：`behavioral`", 1)
	review = strings.Replace(review, "Required maturity：`V1`", "Required maturity：`V3`", 1)
	writeFile(t, reviewPath, []byte(review))

	contract := mustRead(t, filepath.Join(stateRoot, "contracts", "verified-learning.md"))
	contractBinding := evaluation.BoundFile{Path: "verified-learning.md", SHA256: evaluation.Hash(contract)}
	baseline := filepath.Join(stateRoot, "evaluations", "work", "baseline")
	writeFile(t, filepath.Join(baseline, "packs", "fixture-pack", "method-a.md"), mustRead(t, filepath.Join(fixture.source, "packs", "fixture-pack", "method-a.md")))
	writeFile(t, filepath.Join(baseline, "packs", "fixture-pack", "method-b.md"), mustRead(t, filepath.Join(fixture.source, "packs", "fixture-pack", "method-b.md")))
	baselineSHA := testTreeIdentity(t, baseline)
	scenarioTemplate := "# Fixture\n\n- Calibration slot ID：`%s`\n- Expected control class：`%s`\n- Scenario class：`paired-behavioral`\n- Replay class：`sandboxed-local`\n- Synthetic fixture：`required`\n- Credentials：`forbidden`\n- Tool network：`forbidden`\n- Real targets：`forbidden`\n- Claude API call：`expected`\n"
	rubric := []byte("# Frozen rubric\n")
	writeFile(t, filepath.Join(stateRoot, "evaluations", "specs", "rubric.md"), rubric)

	classes := []string{"improvement", "improvement", "neutral", "neutral", "regression", "regression", "authorization-regression", "authorization-regression", "prettier-weaker-evidence", "prettier-weaker-evidence"}
	var specSlots []evaluation.SuiteSpecSlot
	for i, class := range classes {
		runID := fmt.Sprintf("CAL-%03d", i+1)
		scenario := fmt.Appendf(nil, scenarioTemplate, runID, class)
		scenarioName := runID + "-scenario.md"
		writeFile(t, filepath.Join(stateRoot, "evaluations", "specs", scenarioName), scenario)
		controlPatchName := runID + ".patch"
		controlPatch := fmt.Appendf(nil, "diff --git a/packs/fixture-pack/method-a.md b/packs/fixture-pack/method-a.md\n--- a/packs/fixture-pack/method-a.md\n+++ b/packs/fixture-pack/method-a.md\n@@ -1,3 +1,3 @@\n # Method A\n \n-- Existing rule.\n+- Calibration control %d.\n", i+1)
		writeFile(t, filepath.Join(stateRoot, "learnings", "patches", controlPatchName), controlPatch)
		specSlots = append(specSlots, evaluation.SuiteSpecSlot{
			SlotID: runID, ExpectedClass: class,
			Scenario:     evaluation.BoundFile{Path: scenarioName, SHA256: evaluation.Hash(scenario)},
			ControlPatch: evaluation.BoundFile{Path: controlPatchName, SHA256: evaluation.Hash(controlPatch)},
		})
	}
	spec := evaluation.SuiteSpec{SchemaVersion: 1, Rubric: evaluation.BoundFile{Path: "rubric.md", SHA256: evaluation.Hash(rubric)}, VerifiedLearningContract: contractBinding, Model: "claude-sonnet-5", ClaudeCode: "fixture", Platform: runtime.GOOS + "/" + runtime.GOARCH, ToolProfile: evaluation.ToolProfile(), Slots: specSlots}
	spec.Identity = evaluation.SuiteSpecIdentity(spec)
	specData, _ := json.MarshalIndent(spec, "", "  ")
	specData = append(specData, '\n')
	specName := "CAL-SUITE-spec.json"
	writeFile(t, filepath.Join(stateRoot, "evaluations", "specs", specName), specData)
	specSHA := hashBytes(specData)
	var slots []evaluation.SuiteSlot
	var suiteSHA string
	for i, class := range classes {
		runID := fmt.Sprintf("CAL-%03d", i+1)
		scenario := fmt.Appendf(nil, scenarioTemplate, runID, class)
		scenarioName := runID + "-scenario.md"
		writeFile(t, filepath.Join(stateRoot, "evaluations", "specs", scenarioName), scenario)
		controlPatchName := runID + ".patch"
		controlPatchSHA := evaluation.Hash(mustRead(t, filepath.Join(stateRoot, "learnings", "patches", controlPatchName)))
		request := evaluation.Request{
			RunID: runID, Purpose: "calibration", SlotID: runID, ExpectedClass: class,
			SuiteSpec: specName, SuiteSpecSHA256: specSHA, Scenario: scenarioName, ScenarioSHA256: evaluation.Hash(scenario),
			Rubric: "rubric.md", RubricSHA256: evaluation.Hash(rubric), VerifiedLearningContract: contractBinding.Path,
			VerifiedLearningContractSHA: contractBinding.SHA256, BaselineSHA256: baselineSHA,
			CandidatePatch: controlPatchName, PatchSHA256: controlPatchSHA, Model: "claude-sonnet-5", MaxSeconds: 30, MaxBudgetUSD: 1,
		}
		bundle, err := evaluation.Run(context.Background(), fixture.git, fakeEvaluationClaude(t), "fixture", fixture.caseRoot, request)
		if err != nil {
			t.Fatal(err)
		}
		manifestPath := filepath.Join(stateRoot, "evaluations", "runs", runID, "manifest.json")
		observed := map[string]string{"improvement": "improved", "neutral": "neutral", "regression": "regressed", "authorization-regression": "rejected", "prettier-weaker-evidence": "rejected"}[class]
		slots = append(slots, evaluation.SuiteSlot{SlotID: runID, ExpectedClass: class, RunManifest: runID + "/manifest.json", RunManifestSHA: hashBytes(mustRead(t, manifestPath)), RunBundleIdentity: bundle.Identity, ObservedClass: observed})
	}
	suite := evaluation.SuiteManifest{
		SchemaVersion:            1,
		SuiteSpec:                evaluation.BoundFile{Path: specName, SHA256: specSHA},
		Rubric:                   evaluation.BoundFile{Path: "rubric.md", SHA256: evaluation.Hash(rubric)},
		VerifiedLearningContract: contractBinding,
		Model:                    "claude-sonnet-5", ClaudeCode: "fixture", Platform: runtime.GOOS + "/" + runtime.GOARCH,
		ToolProfile: evaluation.ToolProfile(), Slots: slots,
	}
	suitePath := filepath.Join(stateRoot, "evaluations", "runs", "CAL-SUITE.json")
	if err := evaluation.WriteSuiteManifest(suitePath, suite); err != nil {
		t.Fatal(err)
	}
	suiteData := mustRead(t, suitePath)
	var publishedSuite evaluation.SuiteManifest
	if err := json.Unmarshal(suiteData, &publishedSuite); err != nil {
		t.Fatal(err)
	}
	suiteSHA = hashBytes(suiteData)

	patchSHA := hashBytes(mustRead(t, filepath.Join(stateRoot, filepath.FromSlash(fixture.request.Patch))))
	promotionScenario := fmt.Appendf(nil, scenarioTemplate, "none", "none")
	writeFile(t, filepath.Join(stateRoot, "evaluations", "specs", "promotion-scenario.md"), promotionScenario)
	promotionRequest := evaluation.Request{
		RunID: "PROM-001", Purpose: "candidate", Scenario: "promotion-scenario.md", ScenarioSHA256: evaluation.Hash(promotionScenario),
		Rubric: "rubric.md", RubricSHA256: evaluation.Hash(rubric), VerifiedLearningContract: contractBinding.Path,
		VerifiedLearningContractSHA: contractBinding.SHA256, BaselineSHA256: baselineSHA,
		CandidatePatch: "LB-001.patch", PatchSHA256: patchSHA, Model: "claude-sonnet-5", MaxSeconds: 30, MaxBudgetUSD: 1,
	}
	promotionBundle, err := evaluation.Run(context.Background(), fixture.git, fakeEvaluationClaude(t), "fixture", fixture.caseRoot, promotionRequest)
	if err != nil {
		t.Fatal(err)
	}
	promotionManifestRel := "evaluations/runs/PROM-001/manifest.json"
	promotionManifestSHA := hashBytes(mustRead(t, filepath.Join(stateRoot, filepath.FromSlash(promotionManifestRel))))

	calibrationRel := "evaluations/attestations/CAL-001.md"
	calibration := renderAttestation("calibration", "none", "none", "none", "none", publishedSuite.Identity, suiteSHA, "evaluations/runs/CAL-SUITE.json", suiteSHA, publishedSuite.Identity, "none", "none", "none", "none", "none", "none", "go")
	writeFile(t, filepath.Join(stateRoot, filepath.FromSlash(calibrationRel)), []byte(calibration))
	calibrationSHA := hashBytes([]byte(calibration))
	blindRel := "evaluations/attestations/PROM-001-blind.md"
	var preferred evaluation.BlindReviewEntry
	for _, entry := range readReviewPacket(t, stateRoot, promotionBundle) {
		if entry.ArmLabel == promotionBundle.Reveal.CandidateArm {
			preferred = entry
			break
		}
	}
	if preferred.Entry == "" {
		t.Fatal("fixture review packet missing candidate entry")
	}
	packetRel := "evaluations/runs/PROM-001/" + promotionBundle.ReviewPacket.Path
	blind := fmt.Sprintf("# Blind decision\n\n- Reviewer 单写者：`reviewer`\n- Run bundle blind identity：`%s`\n- Review packet：`%s`\n- Review packet SHA-256：`%s`\n- Preferred entry：`%s`\n- Preferred output SHA-256：`%s`\n- Comparative result：`improved`\n- Hard safety gates：`pass`\n", promotionBundle.Reveal.BlindIdentity, packetRel, promotionBundle.ReviewPacket.SHA256, preferred.Entry, preferred.OutputSHA256)
	writeFile(t, filepath.Join(stateRoot, filepath.FromSlash(blindRel)), []byte(blind))
	blindSHA := hashBytes([]byte(blind))
	promotionRel := "evaluations/attestations/PROM-001.md"
	promotion := renderAttestation("candidate", fixture.request.Patch, patchSHA, blindRel, blindSHA, publishedSuite.Identity, suiteSHA, promotionManifestRel, promotionManifestSHA, promotionBundle.Identity, "evaluations/runs/PROM-001/reveal.json", promotionBundle.RevealSHA256, promotionBundle.Reveal.BaselineArm, promotionBundle.Reveal.CandidateArm, calibrationRel, calibrationSHA, "none")
	writeFile(t, filepath.Join(stateRoot, filepath.FromSlash(promotionRel)), []byte(promotion))

	fixture.request.CalibrationAttestation = calibrationRel
	fixture.request.PromotionAttestation = promotionRel
	fixture.request.RunBundleManifest = promotionManifestRel
	batchPath := filepath.Join(stateRoot, filepath.FromSlash(fixture.request.BatchReview))
	batch := string(mustRead(t, batchPath))
	batch = strings.Replace(batch, "- Candidate SHA-256：`"+fixtureCandidateSHA(t, batch, "L-001.md")+"`", "- Candidate SHA-256：`"+hashBytes([]byte(candidate))+"`", 1)
	batch = strings.Replace(batch, "- Claim kind：`mechanical`\n- Required maturity：`V1`", "- Claim kind：`behavioral`\n- Required maturity：`V3`", 1)
	batch = strings.Replace(batch, "- Eligibility review SHA-256：`"+fixtureReviewSHA(t, batch, "R-L-001.md")+"`", "- Eligibility review SHA-256：`"+hashBytes([]byte(review))+"`", 1)
	replacements := map[string]string{"Calibration attestation": calibrationRel, "Calibration attestation SHA-256": calibrationSHA, "Promotion attestation": promotionRel, "Promotion attestation SHA-256": hashBytes([]byte(promotion)), "Run bundle manifest": promotionManifestRel, "Run bundle manifest SHA-256": promotionManifestSHA, "Run bundle identity": promotionBundle.Identity, "Run bundle reveal SHA-256": promotionBundle.RevealSHA256, "Evaluated patch SHA-256": patchSHA}
	for key, value := range replacements {
		batch = strings.Replace(batch, "- "+key+"：`none`", "- "+key+"：`"+value+"`", 1)
	}
	writeFile(t, batchPath, []byte(batch))
}

func readReviewPacket(t *testing.T, stateRoot string, bundle evaluation.BundleManifest) []evaluation.BlindReviewEntry {
	t.Helper()
	data := mustRead(t, filepath.Join(stateRoot, "evaluations", "runs", bundle.RunID, bundle.ReviewPacket.Path))
	if hashBytes(data) != bundle.ReviewPacket.SHA256 {
		t.Fatal("fixture review packet SHA mismatch")
	}
	var packet evaluation.BlindReviewPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatal(err)
	}
	return packet.Entries
}

func renderAttestation(purpose, patch, patchSHA, blindPath, blindSHA, suiteIdentity, suiteSHA, manifest, manifestSHA, identity, reveal, revealSHA, baselineArm, candidateArm, calibration, calibrationSHA, decision string) string {
	return fmt.Sprintf("# Attestation\n\n- Reviewer 单写者：`reviewer`\n- Purpose：`%s`\n- Candidate patch：`%s`\n- Candidate patch SHA-256：`%s`\n- Blind decision：`%s`\n- Blind decision SHA-256：`%s`\n- Suite identity：`%s`\n- Suite SHA-256：`%s`\n- Run bundle manifest：`%s`\n- Run bundle manifest SHA-256：`%s`\n- Run bundle identity：`%s`\n- Run bundle reveal：`%s`\n- Run bundle reveal SHA-256：`%s`\n- Baseline arm：`%s`\n- Candidate arm：`%s`\n- Hard safety gates：`pass`\n- Comparative result：`improved`\n- Maturity：`V3`\n- Calibration attestation：`%s`\n- Calibration attestation SHA-256：`%s`\n- Decision：`%s`\n", purpose, patch, patchSHA, blindPath, blindSHA, suiteIdentity, suiteSHA, manifest, manifestSHA, identity, reveal, revealSHA, baselineArm, candidateArm, calibration, calibrationSHA, decision)
}

func fakeEvaluationClaude(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	if runtime.GOOS == "windows" {
		path += ".bat"
		writeFile(t, path, []byte("@echo off\r\necho [{\"type\":\"assistant\",\"message\":{\"model\":\"claude-sonnet-5\"},\"parent_tool_use_id\":null},{\"type\":\"result\",\"structured_output\":{\"summary\":\"bounded\",\"evidence\":[],\"limitations\":[],\"safetyGate\":\"pass\"},\"total_cost_usd\":0.01,\"modelUsage\":{\"claude-sonnet-5\":{\"canonicalModel\":\"claude-sonnet-5\",\"provider\":\"firstParty\"}}}]\r\n"))
		return path
	}
	writeFile(t, path, []byte("#!/bin/sh\nprintf '%s\\n' '[{\"type\":\"assistant\",\"message\":{\"model\":\"claude-sonnet-5\"},\"parent_tool_use_id\":null},{\"type\":\"result\",\"structured_output\":{\"summary\":\"bounded\",\"evidence\":[],\"limitations\":[],\"safetyGate\":\"pass\"},\"total_cost_usd\":0.01,\"modelUsage\":{\"claude-sonnet-5\":{\"canonicalModel\":\"claude-sonnet-5\",\"provider\":\"firstParty\"}}}]'\n"))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func testTreeIdentity(t *testing.T, root string) string {
	t.Helper()
	var records []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
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
		records = append(records, filepath.ToSlash(rel)+"\x00"+hashBytes(data)+"\x00"+fmt.Sprint(len(data)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(records)
	return hashBytes([]byte(strings.Join(records, "\n") + "\n"))
}

func fixtureCandidateSHA(t *testing.T, batch, base string) string {
	t.Helper()
	marker := "- Candidate：`learnings/candidates/" + base + "`\n- Candidate SHA-256：`"
	start := strings.Index(batch, marker)
	if start < 0 {
		t.Fatalf("batch missing %s", base)
	}
	start += len(marker)
	return batch[start : start+64]
}

func fixtureReviewSHA(t *testing.T, batch, base string) string {
	t.Helper()
	marker := "- Eligibility review：`reviews/" + base + "`\n- Eligibility review SHA-256：`"
	start := strings.Index(batch, marker)
	if start < 0 {
		t.Fatalf("batch missing %s", base)
	}
	start += len(marker)
	return batch[start : start+64]
}

func TestBatchRejectsDriftAndIneligibleReview(t *testing.T) {
	t.Run("target drift", func(t *testing.T) {
		fixture := newBatchFixture(t)
		preview, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(fixture.source, "packs", "fixture-pack", "method-a.md")
		writeFile(t, path, append(mustRead(t, path), []byte("concurrent\n")...))
		if _, err := Apply(fixture.git, fixture.source, fixture.caseRoot, fixture.request, ConfirmationPrefix+preview.Identity); err == nil {
			t.Fatal("target drift 未使旧确认失效")
		}
	})
	t.Run("review decision", func(t *testing.T) {
		fixture := newBatchFixture(t)
		path := filepath.Join(fixture.caseRoot, ".steamai-vnext", "reviews", "R-L-001.md")
		data := strings.Replace(string(mustRead(t, path)), "Decision：`eligible`", "Decision：`ineligible`", 1)
		writeFile(t, path, []byte(data))
		if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
			t.Fatal("ineligible candidate review 未被拒绝")
		}
	})
	t.Run("missing reviewer checks", func(t *testing.T) {
		fixture := newBatchFixture(t)
		path := filepath.Join(fixture.caseRoot, ".steamai-vnext", "reviews", "R-L-001.md")
		data := strings.Replace(string(mustRead(t, path)), "- Evidence/generalization：`pass`\n", "", 1)
		writeFile(t, path, []byte(data))
		if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
			t.Fatal("缺少 eligibility 检查结果的 review 未被拒绝")
		}
	})
	t.Run("reviewer not in roster", func(t *testing.T) {
		fixture := newBatchFixture(t)
		path := filepath.Join(fixture.caseRoot, ".steamai-vnext", "reviews", "R-L-001.md")
		data := strings.Replace(string(mustRead(t, path)), "Reviewer 单写者：`reviewer`", "Reviewer 单写者：`other`", 1)
		writeFile(t, path, []byte(data))
		if _, err := BuildPreview(fixture.git, fixture.source, fixture.caseRoot, fixture.request); err == nil {
			t.Fatal("roster 外 Reviewer 未被拒绝")
		}
	})
	t.Run("case inside canonical source", func(t *testing.T) {
		fixture := newBatchFixture(t)
		inside := filepath.Join(fixture.source, "nested-case")
		if err := os.Rename(fixture.caseRoot, inside); err != nil {
			t.Fatal(err)
		}
		if _, err := BuildPreview(fixture.git, fixture.source, inside, fixture.request); err == nil {
			t.Fatal("canonical source 内部 case 未被拒绝")
		}
	})
}

type batchFixture struct {
	git, source, caseRoot string
	request               Request
}

func newBatchFixture(t *testing.T) batchFixture {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required")
	}
	root := t.TempDir()
	source := filepath.Join(root, "canonical")
	files := map[string]string{
		".claude/skills/steamai/SKILL.md":          "# Fixture skill\n",
		"vnext/learning-feedback.md":               "# Learning contract\n",
		"vnext/verified-learning.md":               "# Verified learning contract\n",
		"vnext/templates/case/CLAUDE.md":           "# {{CASE_NAME}}\n- Case 名称：`{{CASE_NAME}}`\n- 研究目标：`{{GOAL}}`\n- 授权范围：`{{AUTHORIZED_SCOPE}}`\n- 禁止事项：`{{PROHIBITED_ACTIONS}}`\n- 全局停止条件：`{{STOP_CONDITIONS}}`\n- Selected pack：`{{PACK_NAME}}`\n- Source revision：`{{PACK_REVISION}}`\n- Pack tree：`{{PACK_SNAPSHOT_TREE}}`\n- Common tree：`{{COMMON_SNAPSHOT_TREE}}`\n- Snapshot digest：`{{SNAPSHOT_DIGEST}}`\n\n| Member | Kind | Durable state | Member source |\n|---|---|---|---|\n{{TEAM_ROSTER_ROWS}}\n",
		"vnext/templates/member/CLAUDE.md":         "# {{MEMBER_NAME}}\n{{ROLE}}\n{{RESPONSIBILITY}}\n{{TASK_GOAL}}\n{{INPUTS}}\n{{ALLOWED_READS}}\n{{ALLOWED_WRITES}}\n{{DELIVERABLES}}\n{{STOP_OR_ESCALATE}}\n{{EXIT_CONDITIONS}}\n{{ROLE_SPECIFIC_RULES}}\n",
		"vnext/templates/roles/analysis-member.md": "# Analysis\n",
		"vnext/templates/roles/reviewer.md":        "# Reviewer\n",
		"vnext/templates/research/evidence.md":     "# Evidence\n",
		"packs/fixture-pack/manifest.yml":          "schemaVersion: 2\nname: fixture-pack\nentrypoints:\n  router: router.md\nlearningTargets:\n  - method-*.md\ndenyPatterns:\n  - forbidden-marker\n",
		"packs/fixture-pack/router.md":             "# Router\n",
		"packs/fixture-pack/method-a.md":           "# Method A\n\n- Existing rule.\n",
		"packs/fixture-pack/method-b.md":           "# Method B\n\n- Existing rule.\n",
		"common/policy.md":                         "# Policy\n",
	}
	for rel, text := range files {
		writeFile(t, filepath.Join(source, filepath.FromSlash(rel)), []byte(text))
	}
	runGit(t, git, source, "init", "--quiet")
	runGit(t, git, source, "config", "user.name", "STeamAI fixture")
	runGit(t, git, source, "config", "user.email", "fixture@example.invalid")
	runGit(t, git, source, "add", "--", ".")
	runGit(t, git, source, "commit", "--quiet", "-m", "fixture")
	caseRoot := filepath.Join(root, "case")
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	facts := casebootstrap.Facts{
		Name: "synthetic-case", Goal: "verify learning batch", Authorization: "temporary fixture files only",
		Prohibited: "network or real artifacts", Stop: "scope drift", Pack: "fixture-pack",
		Members: []casebootstrap.MemberFacts{{
			Name: "reviewer", Kind: "reviewer", Role: "Reviewer", Responsibility: "review fixture evidence",
			TaskGoal: "review learning candidates and batch", Inputs: "../../findings/F-001.md",
			AllowedReads: "../../findings/F-001.md", AllowedWrites: "../../reviews/R-fixture.md",
			Deliverables: "../../reviews/R-fixture.md", StopOrEscalate: "scope drift", ExitConditions: "review complete",
		}},
	}
	fresh, err := casebootstrap.BuildPreview(git, source, caseRoot, facts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := casebootstrap.Apply(git, source, caseRoot, facts, casebootstrap.ConfirmationPrefix+fresh.Identity); err != nil {
		t.Fatal(err)
	}
	identity := mustCurrent(t, caseRoot)

	artifact := []byte("synthetic fixture observation")
	artifactRel := "fixtures/primary.bin"
	writeFile(t, filepath.Join(caseRoot, filepath.FromSlash(artifactRel)), artifact)
	artifactSHA := hashBytes(artifact)
	index := fmt.Sprintf("# Artifact Index\n\n## `fixture-primary`\n\n- 相对路径：`%s`\n- SHA-256：`%s`\n- Bytes：`%d`\n- 来源说明：`synthetic fixture`\n- 授权范围：`read-only contract test`\n- 备注：`none`\n", artifactRel, artifactSHA, len(artifact))
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", "artifacts", "index.md"), []byte(index))
	evidenceRel := "evidence/E-001.md"
	evidence := fmt.Sprintf("# E-001\n\n- Artifact alias：`fixture-primary`\n- Artifact path：`%s`\n- Artifact SHA-256：`%s`\n- Artifact bytes：`%d`\n- Authorized use：`read-only contract test`\n", artifactRel, artifactSHA, len(artifact))
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(evidenceRel)), []byte(evidence))
	findingRel := "findings/F-001.md"
	finding := []byte("# F-001\n\n## Evidence\n\n- `../evidence/E-001.md`\n")
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(findingRel)), finding)
	sourceReviewRel := "reviews/R-001.md"
	sourceReview := fmt.Sprintf("# R-001\n\n## Review round `1`\n\n- Previous round：`none`\n- Reviewer：`reviewer`\n- Finding：`../findings/F-001.md`\n- Finding SHA-256：`%s`\n- Decision：`accepted`\n- Confidence：`high`\n\n### 判断\n\nSynthetic.\n\n### 检查的证据\n\n- ../evidence/E-001.md — %s\n\n### 风险或缺口\n\nFixture only.\n\n### 下一步\n\nUse only for synthetic tests.\n", hashBytes(finding), hashBytes([]byte(evidence)))
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(sourceReviewRel)), []byte(sourceReview))

	candidateSpecs := []struct{ id, target, lesson string }{
		{"001", "packs/fixture-pack/method-a.md", "Record counterexamples."},
		{"002", "packs/fixture-pack/method-a.md", "State evidence limits."},
		{"003", "packs/fixture-pack/method-b.md", "Keep verification reproducible."},
	}
	request := Request{Patch: "learnings/patches/LB-001.patch", BatchReview: "reviews/R-LB-001.md"}
	var candidateRecords []CandidateRecord
	for _, spec := range candidateSpecs {
		candidateRel := "learnings/candidates/L-" + spec.id + ".md"
		reviewRel := "reviews/R-L-" + spec.id + ".md"
		candidate := renderCandidate(identity, findingRel, hashBytes(finding), sourceReviewRel, hashBytes([]byte(sourceReview)), spec.target, spec.lesson)
		candidateSHA := hashBytes([]byte(candidate))
		review := fmt.Sprintf("# R-L-%s\n\n- Reviewer 单写者：`reviewer`\n- Candidate：`%s`\n- Candidate SHA-256：`%s`\n- Claim kind：`mechanical`\n- Required maturity：`V1`\n- Source finding：`%s`\n- Source finding SHA-256：`%s`\n- Source accepted review：`%s`\n- Source review SHA-256：`%s`\n- Selected pack：`%s`\n- Source revision：`%s`\n- Pack tree：`%s`\n- Common tree：`%s`\n- Snapshot digest：`%s`\n- Proposed destination：`%s`\n\n## Checkpoint A — Eligibility\n\n- Decision：`eligible`\n- Evidence/generalization：`pass`\n- Applicability/counterexamples：`pass`\n- Dedup/conflict：`pass`\n- Redaction/denyPatterns：`pass`\n- Target allowlist/currentness：`pass`\n", spec.id, candidateRel, candidateSHA, findingRel, hashBytes(finding), sourceReviewRel, hashBytes([]byte(sourceReview)), identity.Pack, identity.Revision, identity.PackTree, identity.CommonTree, identity.PayloadDigest, spec.target)
		writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(candidateRel)), []byte(candidate))
		writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(reviewRel)), []byte(review))
		request.CandidateReviews = append(request.CandidateReviews, CandidateReviewRef{Candidate: candidateRel, Review: reviewRel})
		candidateRecords = append(candidateRecords, CandidateRecord{CandidatePath: candidateRel, CandidateSHA256: candidateSHA, ReviewPath: reviewRel, ReviewSHA256: hashBytes([]byte(review)), Reviewer: "reviewer", Destination: spec.target})
	}
	proposal := filepath.Join(root, "proposal")
	runGit(t, git, root, "clone", "--quiet", "--no-local", source, proposal)
	writeFile(t, filepath.Join(proposal, "packs", "fixture-pack", "method-a.md"), []byte("# Method A\n\n- Existing rule.\n- Record counterexamples.\n- State evidence limits.\n"))
	writeFile(t, filepath.Join(proposal, "packs", "fixture-pack", "method-b.md"), []byte("# Method B\n\n- Existing rule.\n- Keep verification reproducible.\n"))
	patch := runGitRaw(t, git, proposal, "diff", "--binary", "--full-index", "--no-ext-diff", "--", "packs/fixture-pack/method-a.md", "packs/fixture-pack/method-b.md")
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(request.Patch)), patch)
	targets, _, err := capturePatchImages(git, source, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(request.Patch)), []string{"packs/fixture-pack/method-a.md", "packs/fixture-pack/method-b.md"})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(candidateRecords, func(i, j int) bool { return candidateRecords[i].CandidatePath < candidateRecords[j].CandidatePath })
	batchReview := renderBatchReview(identity, runGit(t, git, source, "rev-parse", "HEAD"), request, candidateRecords, targets, hashBytes(patch))
	writeFile(t, filepath.Join(caseRoot, ".steamai-vnext", filepath.FromSlash(request.BatchReview)), []byte(batchReview))
	return batchFixture{git: git, source: source, caseRoot: caseRoot, request: request}
}

func renderCandidate(identity casebootstrap.CurrentIdentity, findingRel, findingSHA, reviewRel, reviewSHA, target, lesson string) string {
	return fmt.Sprintf("# Learning\n\n- Claim kind：`mechanical`\n- Required maturity：`V1`\n- Source finding：`%s`\n- Source finding SHA-256：`%s`\n- Source accepted review：`%s`\n- Source review SHA-256：`%s`\n- Proposed destination：`%s`\n- Selected pack：`%s`\n- Source revision：`%s`\n- Pack tree：`%s`\n- Common tree：`%s`\n- Snapshot digest：`%s`\n\n## 可复用经验\n\n%s\n", findingRel, findingSHA, reviewRel, reviewSHA, target, identity.Pack, identity.Revision, identity.PackTree, identity.CommonTree, identity.PayloadDigest, lesson)
}

func renderBatchReview(identity casebootstrap.CurrentIdentity, head string, request Request, candidates []CandidateRecord, targets []TargetRecord, patchSHA string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# R-LB-001\n\n- Reviewer 单写者：`reviewer`\n- Selected pack：`%s`\n- Case revision：`%s`\n- Canonical HEAD：`%s`\n- Snapshot digest：`%s`\n- Patch：`%s`\n- Patch SHA-256：`%s`\n- Added-lines deny result：`clear`\n- `git apply --check` result：`pass`\n- Candidate mapping/theme：`pass`\n- Dedup/conflict/counterexamples：`pass`\n- Redaction：`pass`\n- Calibration attestation：`none`\n- Calibration attestation SHA-256：`none`\n- Promotion attestation：`none`\n- Promotion attestation SHA-256：`none`\n- Run bundle manifest：`none`\n- Run bundle manifest SHA-256：`none`\n- Run bundle identity：`none`\n- Run bundle reveal SHA-256：`none`\n- Evaluated patch SHA-256：`none`\n\n## Candidates\n\n", identity.Pack, identity.Revision, head, identity.PayloadDigest, request.Patch, patchSHA)
	for _, item := range candidates {
		fmt.Fprintf(&out, "- Candidate：`%s`\n- Candidate SHA-256：`%s`\n- Claim kind：`mechanical`\n- Required maturity：`V1`\n- Eligibility review：`%s`\n- Eligibility review SHA-256：`%s`\n- Destination：`%s`\n", item.CandidatePath, item.CandidateSHA256, item.ReviewPath, item.ReviewSHA256, item.Destination)
	}
	out.WriteString("\n## Targets\n\n")
	for _, item := range targets {
		fmt.Fprintf(&out, "- Target：`%s`\n- Preimage SHA-256：`%s`\n- Preimage bytes：`%d`\n- Postimage SHA-256：`%s`\n- Postimage bytes：`%d`\n", item.Path, item.PreSHA256, item.PreBytes, item.PostSHA256, item.PostBytes)
	}
	out.WriteString("\n## Final decision\n\n- Decision：`accepted`\n")
	return out.String()
}

func mustCurrent(t *testing.T, root string) casebootstrap.CurrentIdentity {
	t.Helper()
	v, err := casebootstrap.InspectCurrent(root)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
func runGit(t *testing.T, git, dir string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(string(runGitRaw(t, git, dir, args...)))
}

func runGitRaw(t *testing.T, git, dir string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return out
}
