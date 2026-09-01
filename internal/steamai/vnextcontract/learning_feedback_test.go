package vnextcontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestLearningFeedbackContractRequiresExactReviewedGitPatch(t *testing.T) {
	repo := repoRoot(t)
	contract := readPrototypeFile(t, repo, "vnext/learning-feedback.md")
	for _, required := range []string{
		"current accepted review",
		"source finding/review",
		"artifact alias、case-relative path、SHA-256、bytes 与 authorized use",
		"实际 artifact bytes 一致",
		"完整 snapshot digest",
		"`learningTargets`",
		"`denyPatterns`",
		"immutable",
		"Eligibility decision: eligible",
		"无权威 exact proposal patch",
		"用户确认前不得编辑 canonical source pack",
		"git diff --binary --full-index --no-ext-diff",
		"git apply --check <PATCH_PATH>",
		"git apply --numstat -z",
		"Patch decision 为 `accepted`",
		"manifest base blob",
		"target base blob",
		"candidate path/SHA、final learning review path/SHA、source finding/review path/SHA",
		"canonical `HEAD` 等于 reviewed base revision",
		"应用不自动 commit/push",
		"完整排序 file manifest",
		"当前 case 的完整 payload digest 必须保持旧值",
		"candidate 文件不得包含自身 exact SHA 字段",
	} {
		assertContains(t, contract, required, "learning feedback contract")
	}
	for _, forbidden := range []string{"截断 diff", "fuzzy apply", "三方合并", "旧 promote/writeback 状态机"} {
		assertContains(t, contract, forbidden, "explicitly forbidden learning path")
	}
}

func TestLearningBaseBlobComparisonIsFilterAwareOnWindowsCheckout(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git for filtered blob contract: %v", err)
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, git, source, "init", "--quiet")
	runGit(t, git, source, "config", "user.name", "STeamAI fixture")
	runGit(t, git, source, "config", "user.email", "fixture@example.invalid")
	targetRel := "packs/binary-re/references/binary-re/general-analysis.md"
	writeLearningFixtureFile(t, filepath.Join(source, filepath.FromSlash(targetRel)), "# General analysis\n\n- One rule.\n")
	runGit(t, git, source, "add", "--", targetRel)
	runGit(t, git, source, "commit", "--quiet", "-m", "base")
	baseBlob := strings.TrimSpace(runGit(t, git, source, "rev-parse", "HEAD:"+targetRel))

	checkout := filepath.Join(root, "checkout")
	runGit(t, git, root, "clone", "--quiet", "--no-local", source, checkout)
	runGit(t, git, checkout, "config", "core.autocrlf", "true")
	checkoutTarget := filepath.Join(checkout, filepath.FromSlash(targetRel))
	lfText := normalizeLearningText(readLearningFixtureFile(t, checkoutTarget))
	writeLearningFixtureFile(t, checkoutTarget, strings.ReplaceAll(lfText, "\n", "\r\n"))
	writeLearningFixtureFile(t, filepath.Join(checkout, ".gitattributes"), "*.md text eol=lf\n")
	filteredBlob := strings.TrimSpace(runGit(t, git, checkout, "hash-object", "--path="+targetRel, "--", targetRel))
	rawBlob := strings.TrimSpace(runGit(t, git, checkout, "hash-object", "--no-filters", "--", targetRel))
	if filteredBlob != baseBlob {
		t.Fatalf("filter-aware blob mismatch: got %s want %s", filteredBlob, baseBlob)
	}
	if rawBlob == baseBlob {
		t.Fatal("fixture did not prove raw CRLF bytes differ from canonical Git blob")
	}
	writeLearningFixtureFile(t, checkoutTarget, strings.ReplaceAll(lfText+"- Real content drift.\n", "\n", "\r\n"))
	driftedBlob := strings.TrimSpace(runGit(t, git, checkout, "hash-object", "--path="+targetRel, "--", targetRel))
	if driftedBlob == baseBlob {
		t.Fatal("filter-aware blob comparison missed real content drift")
	}
}

func TestExactLearningPatchRequiresConfirmationAndCurrentBase(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git for exact learning patch contract: %v", err)
	}
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, git, source, "init", "--quiet")
	runGit(t, git, source, "config", "user.name", "STeamAI fixture")
	runGit(t, git, source, "config", "user.email", "fixture@example.invalid")

	targetRel := "packs/binary-re/references/binary-re/general-analysis.md"
	target := filepath.Join(source, filepath.FromSlash(targetRel))
	baseText := "# General analysis\n\n- Keep claims bounded to evidence.\n"
	writeLearningFixtureFile(t, target, baseText)
	runGit(t, git, source, "add", "--", targetRel)
	runGit(t, git, source, "commit", "--quiet", "-m", "base")
	baseRevision := strings.TrimSpace(runGit(t, git, source, "rev-parse", "HEAD"))
	baseBlob := strings.TrimSpace(runGit(t, git, source, "rev-parse", "HEAD:"+targetRel))

	currentCaseRoot := filepath.Join(root, "current-case")
	caseSnapshot := materializePackSnapshot(t, git, source, currentCaseRoot, baseRevision, targetRel)
	if got := readLearningFixtureFile(t, caseSnapshot); normalizeLearningText(got) != normalizeLearningText(baseText) {
		t.Fatal("current case did not resolve pack instructions from its materialized base snapshot")
	}
	proposal := filepath.Join(root, "proposal")
	runGit(t, git, root, "clone", "--quiet", "--no-local", source, proposal)
	proposedText := baseText + "- Record counterexamples with every generalized rule.\n"
	writeLearningFixtureFile(t, filepath.Join(proposal, filepath.FromSlash(targetRel)), proposedText)
	patch := runGit(t, git, proposal, "diff", "--binary", "--full-index", "--no-ext-diff", "--", targetRel)
	if !strings.Contains(patch, "diff --git a/"+targetRel+" b/"+targetRel) ||
		!strings.Contains(patch, "Record counterexamples") || strings.Contains(patch, "truncated") {
		t.Fatalf("proposal is not a complete standard Git patch:\n%s", patch)
	}
	patchPath := filepath.Join(root, "current-case", ".steamai-vnext", "learnings", "patches", "L-001.patch")
	writeLearningFixtureFile(t, patchPath, patch)
	patchSHA256 := learningSHA256([]byte(patch))
	assertSingleLearningPatchTarget(t, git, proposal, patchPath, targetRel)

	beforeConfirmation := readLearningFixtureFile(t, target)
	if err := applyLearningPatch(git, source, targetRel, patchPath, baseBlob, patchSHA256, false); !errors.Is(err, errLearningConfirmationRequired) {
		t.Fatalf("unconfirmed learning patch returned %v, want confirmation-required", err)
	}
	if got := readLearningFixtureFile(t, target); got != beforeConfirmation {
		t.Fatal("source pack changed before user confirmation")
	}

	verification := filepath.Join(root, "verification")
	runGit(t, git, root, "clone", "--quiet", "--no-local", source, verification)
	runGit(t, git, verification, "checkout", "--quiet", baseRevision)
	runGit(t, git, verification, "apply", "--check", patchPath)

	drift := filepath.Join(root, "drift")
	runGit(t, git, root, "clone", "--quiet", "--no-local", source, drift)
	driftTarget := filepath.Join(drift, filepath.FromSlash(targetRel))
	writeLearningFixtureFile(t, driftTarget, baseText+"- A concurrent pack edit.\n")
	driftBefore := readLearningFixtureFile(t, driftTarget)
	if err := applyLearningPatch(git, drift, targetRel, patchPath, baseBlob, patchSHA256, true); !errors.Is(err, errLearningBaseDrift) {
		t.Fatalf("drifted learning patch returned %v, want base-drift", err)
	}
	if got := readLearningFixtureFile(t, driftTarget); got != driftBefore {
		t.Fatal("drifted target changed despite fail-closed base check")
	}

	tamperedPatch := patch + "\n# tampered after confirmation\n"
	writeLearningFixtureFile(t, patchPath, tamperedPatch)
	if err := applyLearningPatch(git, source, targetRel, patchPath, baseBlob, patchSHA256, true); !errors.Is(err, errLearningPatchDrift) {
		t.Fatalf("tampered learning patch returned %v, want patch-drift", err)
	}
	if got := readLearningFixtureFile(t, target); got != beforeConfirmation {
		t.Fatal("source pack changed after patch bytes drifted")
	}
	writeLearningFixtureFile(t, patchPath, patch)

	if err := applyLearningPatch(git, source, targetRel, patchPath, baseBlob, patchSHA256, true); err != nil {
		t.Fatalf("apply confirmed current learning patch: %v", err)
	}
	if got := normalizeLearningText(readLearningFixtureFile(t, target)); got != normalizeLearningText(proposedText) {
		t.Fatalf("confirmed source pack mismatch:\n%s", got)
	}
	if got := readLearningFixtureFile(t, caseSnapshot); got != baseText {
		t.Fatal("running case pack snapshot drifted after source pack feedback")
	}
	runGit(t, git, source, "add", "--", targetRel)
	runGit(t, git, source, "commit", "--quiet", "-m", "confirmed learning")
	futureRevision := strings.TrimSpace(runGit(t, git, source, "rev-parse", "HEAD"))
	futureSnapshot := materializePackSnapshot(t, git, source, filepath.Join(root, "future-case"), futureRevision, targetRel)
	if got := normalizeLearningText(readLearningFixtureFile(t, futureSnapshot)); got != normalizeLearningText(proposedText) {
		t.Fatal("future case did not explicitly consume the updated pack snapshot")
	}
}

func TestReviewedLearningPatchRequiresAcceptedExactBinding(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required for reviewed learning patch contract")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, git, repo, "init", "--quiet")
	runGit(t, git, repo, "config", "user.name", "STeamAI fixture")
	runGit(t, git, repo, "config", "user.email", "fixture@example.invalid")
	manifestRel := "packs/binary-re/manifest.yml"
	targetRel := "packs/binary-re/references/binary-re/general-analysis.md"
	baseText := "# Analysis\n\n- Keep claims bounded.\n"
	manifest := "schemaVersion: 2\nlearningTargets:\n  - references/binary-re/*.md\ndenyPatterns:\n  - credentials-or-tokens\n"
	writeLearningFixtureFile(t, filepath.Join(repo, filepath.FromSlash(manifestRel)), manifest)
	writeLearningFixtureFile(t, filepath.Join(repo, filepath.FromSlash(targetRel)), baseText)
	runGit(t, git, repo, "add", "--", ".")
	runGit(t, git, repo, "commit", "--quiet", "-m", "base")
	baseRevision := strings.TrimSpace(runGit(t, git, repo, "rev-parse", "HEAD"))
	manifestBlob := strings.TrimSpace(runGit(t, git, repo, "rev-parse", "HEAD:"+manifestRel))
	targetBlob := strings.TrimSpace(runGit(t, git, repo, "rev-parse", "HEAD:"+targetRel))

	proposal := filepath.Join(root, "proposal")
	runGit(t, git, root, "clone", "--quiet", "--no-local", repo, proposal)
	writeLearningFixtureFile(t, filepath.Join(proposal, filepath.FromSlash(targetRel)), baseText+"- Record counterexamples.\n")
	patch := runGit(t, git, proposal, "diff", "--binary", "--full-index", "--no-ext-diff", "--", targetRel)
	patchPath := filepath.Join(root, "L-001.patch")
	writeLearningFixtureFile(t, patchPath, patch)
	candidate := []byte("synthetic immutable candidate")
	review := renderLearningReviewFixture(learningReviewBinding{
		Reviewer:         "reviewer",
		CandidateSHA256:  learningSHA256(candidate),
		TargetRel:        targetRel,
		BaseRevision:     baseRevision,
		ManifestBaseBlob: manifestBlob,
		TargetBaseBlob:   targetBlob,
		PatchSHA256:      learningSHA256([]byte(patch)),
		Eligibility:      "eligible",
		PatchDecision:    "accepted",
	})
	caseRoot := filepath.Join(root, "case")
	snapshotMetadataPath, snapshotDigest := writeLearningSnapshotFixture(t, caseRoot)
	artifactPath := filepath.Join(caseRoot, "fixtures", "primary.bin")
	artifactBytes := []byte("synthetic artifact")
	writeLearningFixtureFile(t, artifactPath, string(artifactBytes))
	artifactSHA := learningSHA256(artifactBytes)
	artifactIndexPath := filepath.Join(caseRoot, ".steamai-vnext", "artifacts", "index.md")
	writeLearningFixtureFile(t, artifactIndexPath, "# Artifact Index\n\n## `fixture-primary`\n\n- 相对路径：`fixtures/primary.bin`\n- SHA-256：`"+artifactSHA+"`\n- Bytes：`18`\n- 来源说明：`synthetic fixture`\n- 授权范围：`read-only contract test`\n- 备注：`none`\n")
	evidencePath := filepath.Join(caseRoot, ".steamai-vnext", "evidence", "E-001.md")
	evidenceText := "# E-001\n\n- Owner：`owner`\n- Artifact alias：`fixture-primary`\n- Artifact path：`fixtures/primary.bin`\n- Artifact SHA-256：`" + artifactSHA + "`\n- Artifact bytes：`18`\n- Authorized use：`read-only contract test`\n"
	writeLearningFixtureFile(t, evidencePath, evidenceText)
	findingPath := filepath.Join(caseRoot, ".steamai-vnext", "findings", "F-001.md")
	findingText := "# F-001\n\n## Evidence\n\n- `../evidence/E-001.md`\n"
	writeLearningFixtureFile(t, findingPath, findingText)
	sourceReviewPath := filepath.Join(caseRoot, ".steamai-vnext", "reviews", "R-001.md")
	sourceReviewText := renderLearningSourceReviewRound("1", "none", "accepted", learningSHA256([]byte(findingText)), learningSHA256([]byte(evidenceText)))
	writeLearningFixtureFile(t, sourceReviewPath, sourceReviewText)
	sourceChain := learningSourceChain{
		ArtifactIndexPath:      artifactIndexPath,
		FindingPath:            findingPath,
		SourceReviewPath:       sourceReviewPath,
		SnapshotMetadataPath:   snapshotMetadataPath,
		ExpectedSnapshotDigest: snapshotDigest,
	}
	confirmation := learningConfirmation{
		CandidateSHA256:   learningSHA256(candidate),
		ReviewSHA256:      learningSHA256(review),
		PatchSHA256:       learningSHA256([]byte(patch)),
		SnapshotDigest:    snapshotDigest,
		SourceFindingPath: filepath.ToSlash(findingPath),
		SourceFindingSHA:  learningSHA256([]byte(findingText)),
		SourceReviewPath:  filepath.ToSlash(sourceReviewPath),
		SourceReviewSHA:   learningSHA256([]byte(sourceReviewText)),
		Confirmed:         true,
	}

	rejected := bytes.Replace(review, []byte("Patch decision：`accepted`"), []byte("Patch decision：`rejected`"), 1)
	rejectedConfirmation := confirmation
	rejectedConfirmation.ReviewSHA256 = learningSHA256(rejected)
	if err := applyReviewedLearningPatch(git, repo, manifestRel, patchPath, sourceChain, rejectedConfirmation, candidate, rejected); !errors.Is(err, errLearningReviewBinding) {
		t.Fatalf("non-accepted exact patch returned %v", err)
	}
	wrongReviewer := bytes.Replace(review, []byte("Reviewer 单写者：`reviewer`"), []byte("Reviewer 单写者：`other-reviewer`"), 1)
	wrongReviewerConfirmation := confirmation
	wrongReviewerConfirmation.ReviewSHA256 = learningSHA256(wrongReviewer)
	if err := applyReviewedLearningPatch(git, repo, manifestRel, patchPath, sourceChain, wrongReviewerConfirmation, candidate, wrongReviewer); !errors.Is(err, errLearningReviewBinding) {
		t.Fatalf("Reviewer mismatch returned %v", err)
	}
	driftedReview := append(bytes.Clone(review), []byte("drift")...)
	if err := applyReviewedLearningPatch(git, repo, manifestRel, patchPath, sourceChain, confirmation, candidate, driftedReview); !errors.Is(err, errLearningReviewBinding) {
		t.Fatalf("review drift returned %v", err)
	}
	writeLearningFixtureFile(t, evidencePath, evidenceText+"drift\n")
	if err := applyReviewedLearningPatch(git, repo, manifestRel, patchPath, sourceChain, confirmation, candidate, review); !errors.Is(err, errLearningReviewBinding) {
		t.Fatalf("source evidence drift returned %v", err)
	}
	writeLearningFixtureFile(t, evidencePath, evidenceText)
	writeLearningFixtureFile(t, artifactPath, "replacement artifact")
	if err := applyReviewedLearningPatch(git, repo, manifestRel, patchPath, sourceChain, confirmation, candidate, review); !errors.Is(err, errLearningReviewBinding) {
		t.Fatalf("source artifact drift returned %v", err)
	}
	writeLearningFixtureFile(t, artifactPath, string(artifactBytes))
	disputedReview := sourceReviewText + renderLearningSourceReviewRound("2", "1", "disputed", learningSHA256([]byte(findingText)), learningSHA256([]byte(evidenceText)))
	writeLearningFixtureFile(t, sourceReviewPath, disputedReview)
	disputedConfirmation := confirmation
	disputedConfirmation.SourceReviewSHA = learningSHA256([]byte(disputedReview))
	if err := applyReviewedLearningPatch(git, repo, manifestRel, patchPath, sourceChain, disputedConfirmation, candidate, review); !errors.Is(err, errLearningReviewBinding) {
		t.Fatalf("non-current historical accepted round returned %v", err)
	}
	writeLearningFixtureFile(t, sourceReviewPath, sourceReviewText)
	snapshotPayloadPath := filepath.Join(filepath.Dir(snapshotMetadataPath), "packs", "binary-re", "references", "binary-re", "general-analysis.md")
	writeLearningFixtureFile(t, snapshotPayloadPath, "# Drifted pinned instruction\n")
	if err := applyReviewedLearningPatch(git, repo, manifestRel, patchPath, sourceChain, confirmation, candidate, review); !errors.Is(err, errLearningReviewBinding) {
		t.Fatalf("source snapshot drift returned %v", err)
	}
	writeLearningFixtureFile(t, snapshotPayloadPath, "# Synthetic pinned instruction\n")
	if err := applyReviewedLearningPatch(git, repo, manifestRel, patchPath, sourceChain, confirmation, candidate, review); err != nil {
		t.Fatalf("apply current reviewed learning patch: %v", err)
	}
	if got := normalizeLearningText(readLearningFixtureFile(t, filepath.Join(repo, filepath.FromSlash(targetRel)))); got != normalizeLearningText(baseText+"- Record counterexamples.\n") {
		t.Fatalf("reviewed learning patch output mismatch: %s", got)
	}
}

func TestReviewedLearningPatchRejectsDisallowedPatchForms(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required for patch-shape contract")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, "packs", "binary-re", "references", "binary-re"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, git, repo, "init", "--quiet")
	runGit(t, git, repo, "config", "user.name", "STeamAI fixture")
	runGit(t, git, repo, "config", "user.email", "fixture@example.invalid")
	manifestRel := "packs/binary-re/manifest.yml"
	targetRel := "packs/binary-re/references/binary-re/general-analysis.md"
	outsideRel := "packs/binary-re/private.md"
	writeLearningFixtureFile(t, filepath.Join(repo, filepath.FromSlash(manifestRel)), "schemaVersion: 2\nlearningTargets:\n  - references/binary-re/*.md\ndenyPatterns:\n  - credentials-or-tokens\n")
	writeLearningFixtureFile(t, filepath.Join(repo, filepath.FromSlash(targetRel)), "# Analysis\n")
	writeLearningFixtureFile(t, filepath.Join(repo, filepath.FromSlash(outsideRel)), "# Private\n")
	runGit(t, git, repo, "add", "--", ".")
	runGit(t, git, repo, "commit", "--quiet", "-m", "base")

	cases := []struct {
		name      string
		targetRel string
		mutate    func(string)
	}{
		{"delete", targetRel, func(clone string) { _ = os.Remove(filepath.Join(clone, filepath.FromSlash(targetRel))) }},
		{"outside allowlist", outsideRel, func(clone string) {
			writeLearningFixtureFile(t, filepath.Join(clone, filepath.FromSlash(outsideRel)), "# Private\n\n- bounded rule\n")
		}},
		{"deny pattern", targetRel, func(clone string) {
			writeLearningFixtureFile(t, filepath.Join(clone, filepath.FromSlash(targetRel)), "# Analysis\n\n- credentials-or-tokens\n")
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			clone := filepath.Join(root, strings.ReplaceAll(test.name, " ", "-"))
			runGit(t, git, root, "clone", "--quiet", "--no-local", repo, clone)
			test.mutate(clone)
			patch := runGit(t, git, clone, "diff", "--binary", "--full-index", "--no-ext-diff", "--", test.targetRel)
			patchPath := filepath.Join(root, strings.ReplaceAll(test.name, " ", "-")+".patch")
			writeLearningFixtureFile(t, patchPath, patch)
			if err := requireReviewedLearningPatchScope(git, repo, manifestRel, patchPath, test.targetRel); !errors.Is(err, errLearningPatchScope) {
				t.Fatalf("disallowed patch returned %v", err)
			}
		})
	}
}

func materializePackSnapshot(t *testing.T, git, source, caseRoot, revision, targetRel string) string {
	t.Helper()
	snapshotRoot := filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot")
	if err := os.MkdirAll(snapshotRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	content := runGit(t, git, source, "show", revision+":"+targetRel)
	target := filepath.Join(snapshotRoot, filepath.FromSlash(targetRel))
	writeLearningFixtureFile(t, target, content)
	tree := strings.TrimSpace(runGit(t, git, source, "rev-parse", revision+":packs/binary-re"))
	metadata := strings.Join([]string{"pack: binary-re", "revision: " + revision, "tree: " + tree, ""}, "\n")
	writeLearningFixtureFile(t, filepath.Join(snapshotRoot, "snapshot.yml"), metadata)
	return target
}

func normalizeLearningText(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func renderLearningSourceReviewRound(round, previous, decision, findingSHA, evidenceSHA string) string {
	return strings.Join([]string{
		"## Review round `" + round + "`",
		"",
		"- Previous round：`" + previous + "`",
		"- Reviewer：`reviewer`",
		"- Finding：`../findings/F-001.md`",
		"- Finding SHA-256：`" + findingSHA + "`",
		"- Decision：`" + decision + "`",
		"- Confidence：`high`",
		"",
		"### 判断",
		"",
		"Synthetic source review.",
		"",
		"### 检查的证据",
		"",
		"- ../evidence/E-001.md — " + evidenceSHA,
		"",
		"### 风险或缺口",
		"",
		"Synthetic-only scope.",
		"",
		"### 下一步",
		"",
		"Apply only while current.",
		"",
	}, "\n")
}

type learningReviewBinding struct {
	Reviewer         string
	CandidateSHA256  string
	TargetRel        string
	BaseRevision     string
	ManifestBaseBlob string
	TargetBaseBlob   string
	PatchSHA256      string
	Eligibility      string
	PatchDecision    string
}

type learningConfirmation struct {
	CandidateSHA256   string
	ReviewSHA256      string
	PatchSHA256       string
	SnapshotDigest    string
	SourceFindingPath string
	SourceFindingSHA  string
	SourceReviewPath  string
	SourceReviewSHA   string
	Confirmed         bool
}

type learningSourceChain struct {
	ArtifactIndexPath      string
	FindingPath            string
	SourceReviewPath       string
	SnapshotMetadataPath   string
	ExpectedSnapshotDigest string
}

type learningArtifactBinding struct {
	Alias         string
	Path          string
	SHA256        string
	Bytes         int
	AuthorizedUse string
}

var (
	errLearningConfirmationRequired = errors.New("learning patch requires user confirmation")
	errLearningBaseDrift            = errors.New("learning patch target base drifted")
	errLearningPatchDrift           = errors.New("learning patch bytes drifted after confirmation")
	errLearningPatchScope           = errors.New("learning patch target scope changed")
	errLearningReviewBinding        = errors.New("learning patch is not bound to a current accepted review")
	errLearningRevisionDrift        = errors.New("learning patch canonical revision drifted")
	errLearningManifestDrift        = errors.New("learning patch manifest drifted")
)

func learningSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertSingleLearningPatchTarget(t *testing.T, git, repo, patchPath, targetRel string) {
	t.Helper()
	if err := requireSingleLearningPatchTarget(git, repo, patchPath, targetRel); err != nil {
		t.Fatal(err)
	}
}

func requireSingleLearningPatchTarget(git, repo, patchPath, targetRel string) error {
	out, err := gitOutput(git, repo, "apply", "--numstat", "-z", patchPath)
	if err != nil {
		return err
	}
	fields := strings.Split(out, "\x00")
	if len(fields) != 2 || fields[1] != "" {
		return errLearningPatchScope
	}
	parts := strings.Split(fields[0], "\t")
	if len(parts) != 3 || filepath.ToSlash(parts[2]) != filepath.ToSlash(targetRel) {
		return errLearningPatchScope
	}
	return nil
}

func requireReviewedLearningPatchScope(git, repo, manifestRel, patchPath, targetRel string) error {
	if filepath.Ext(targetRel) != ".md" || filepath.IsAbs(filepath.FromSlash(targetRel)) || strings.Contains(targetRel, "\\") {
		return errLearningPatchScope
	}
	manifestBytes, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(manifestRel)))
	if err != nil {
		return err
	}
	manifest := string(manifestBytes)
	packRootRel := filepath.ToSlash(filepath.Dir(manifestRel))
	packRoot := filepath.Join(repo, filepath.FromSlash(packRootRel))
	target := filepath.Clean(filepath.Join(repo, filepath.FromSlash(targetRel)))
	if !pathWithin(target, packRoot) {
		return errLearningPatchScope
	}
	allowed := false
	for _, pattern := range manifestListValues(manifest, "learningTargets") {
		match, matchErr := filepath.Match(filepath.FromSlash(pattern), filepath.FromSlash(strings.TrimPrefix(targetRel, packRootRel+"/")))
		if matchErr != nil {
			return matchErr
		}
		if match {
			allowed = true
			break
		}
	}
	if !allowed {
		return errLearningPatchScope
	}
	info, err := os.Lstat(target)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return errLearningPatchScope
	}
	tracked, err := gitOutput(git, repo, "ls-files", "--error-unmatch", "--", targetRel)
	if err != nil || strings.TrimSpace(tracked) != targetRel {
		return errLearningPatchScope
	}
	patch, err := os.ReadFile(patchPath)
	if err != nil {
		return err
	}
	text := string(patch)
	for _, forbidden := range []string{"GIT binary patch", "Binary files ", "new file mode ", "deleted file mode ", "old mode ", "new mode ", "similarity index ", "rename from ", "rename to ", "copy from ", "copy to ", "/dev/null"} {
		if strings.Contains(text, forbidden) {
			return errLearningPatchScope
		}
	}
	if strings.Count(text, "diff --git ") != 1 || !strings.Contains(text, "diff --git a/"+targetRel+" b/"+targetRel+"\n") ||
		!strings.Contains(text, "--- a/"+targetRel+"\n") || !strings.Contains(text, "+++ b/"+targetRel+"\n") {
		return errLearningPatchScope
	}
	if err := requireSingleLearningPatchTarget(git, repo, patchPath, targetRel); err != nil {
		return err
	}
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		added := strings.TrimPrefix(line, "+")
		for _, deny := range manifestListValues(manifest, "denyPatterns") {
			if deny != "" && strings.Contains(added, deny) {
				return errLearningPatchScope
			}
		}
	}
	return nil
}

func applyLearningPatch(git, repo, targetRel, patchPath, expectedBaseBlob, expectedPatchSHA256 string, confirmed bool) error {
	if !confirmed {
		return errLearningConfirmationRequired
	}
	patch, err := os.ReadFile(patchPath)
	if err != nil {
		return err
	}
	if learningSHA256(patch) != expectedPatchSHA256 {
		return errLearningPatchDrift
	}
	if err := requireSingleLearningPatchTarget(git, repo, patchPath, targetRel); err != nil {
		return err
	}
	currentBlob, err := gitOutput(git, repo, "hash-object", "--path="+targetRel, "--", targetRel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(currentBlob) != expectedBaseBlob {
		return errLearningBaseDrift
	}
	if _, err := gitOutput(git, repo, "apply", "--check", patchPath); err != nil {
		return err
	}
	_, err = gitOutput(git, repo, "apply", patchPath)
	return err
}

func writeLearningSnapshotFixture(t *testing.T, caseRoot string) (string, string) {
	t.Helper()
	snapshotRoot := filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot")
	payloadRel := "packs/binary-re/references/binary-re/general-analysis.md"
	payload := []byte("# Synthetic pinned instruction\n")
	payloadPath := filepath.Join(snapshotRoot, filepath.FromSlash(payloadRel))
	writeLearningFixtureFile(t, payloadPath, string(payload))
	blob := strings.Repeat("1", 40)
	record := strings.Join([]string{payloadRel, "100644", blob, strconv.Itoa(len(payload)), learningSHA256(payload)}, "\x00")
	digest := "sha256:" + learningSHA256([]byte(record+"\n"))
	metadata := strings.Join([]string{
		"pack: binary-re",
		"revision: " + strings.Repeat("2", 40),
		"pack-tree: " + strings.Repeat("3", 40),
		"common-tree: " + strings.Repeat("4", 40),
		"payload-digest: " + digest,
		"files:",
		"  - path: " + payloadRel,
		"    git-mode: 100644",
		"    git-blob: " + blob,
		"    sha256: " + learningSHA256(payload),
		"    bytes: " + strconv.Itoa(len(payload)),
		"",
	}, "\n")
	metadataPath := filepath.Join(snapshotRoot, "snapshot.yml")
	writeLearningFixtureFile(t, metadataPath, metadata)
	return metadataPath, digest
}

func validateLearningSnapshot(chain learningSourceChain) error {
	if chain.SnapshotMetadataPath == "" || chain.ExpectedSnapshotDigest == "" {
		return errLearningReviewBinding
	}
	metadataBytes, err := os.ReadFile(chain.SnapshotMetadataPath)
	if err != nil {
		return err
	}
	var declaredDigest string
	type record struct {
		path, mode, blob, sha, bytes string
	}
	var records []record
	for line := range strings.SplitSeq(string(metadataBytes), "\n") {
		switch {
		case strings.HasPrefix(line, "payload-digest: "):
			declaredDigest = strings.TrimSpace(strings.TrimPrefix(line, "payload-digest: "))
		case strings.HasPrefix(line, "  - path: "):
			records = append(records, record{path: strings.TrimSpace(strings.TrimPrefix(line, "  - path: "))})
		case len(records) > 0 && strings.HasPrefix(line, "    git-mode: "):
			records[len(records)-1].mode = strings.TrimSpace(strings.TrimPrefix(line, "    git-mode: "))
		case len(records) > 0 && strings.HasPrefix(line, "    git-blob: "):
			records[len(records)-1].blob = strings.TrimSpace(strings.TrimPrefix(line, "    git-blob: "))
		case len(records) > 0 && strings.HasPrefix(line, "    sha256: "):
			records[len(records)-1].sha = strings.TrimSpace(strings.TrimPrefix(line, "    sha256: "))
		case len(records) > 0 && strings.HasPrefix(line, "    bytes: "):
			records[len(records)-1].bytes = strings.TrimSpace(strings.TrimPrefix(line, "    bytes: "))
		}
	}
	if declaredDigest != chain.ExpectedSnapshotDigest || len(records) == 0 {
		return errLearningReviewBinding
	}
	snapshotRoot := filepath.Dir(chain.SnapshotMetadataPath)
	var canonical []string
	for _, item := range records {
		path := filepath.Clean(filepath.Join(snapshotRoot, filepath.FromSlash(item.path)))
		if item.path == "" || !pathWithin(path, snapshotRoot) || (item.mode != "100644" && item.mode != "100755") || len(item.blob) != 40 {
			return errLearningReviewBinding
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errLearningReviewBinding
		}
		data, err := os.ReadFile(path)
		if err != nil || strconv.Itoa(len(data)) != item.bytes || learningSHA256(data) != item.sha {
			return errLearningReviewBinding
		}
		canonical = append(canonical, strings.Join([]string{filepath.ToSlash(item.path), item.mode, item.blob, item.bytes, item.sha}, "\x00"))
	}
	sort.Strings(canonical)
	actualDigest := "sha256:" + learningSHA256([]byte(strings.Join(canonical, "\n")+"\n"))
	if actualDigest != declaredDigest {
		return errLearningReviewBinding
	}
	return nil
}

func validateLearningSourceChain(chain learningSourceChain) error {
	if err := validateLearningSnapshot(chain); err != nil {
		return errLearningReviewBinding
	}
	indexBytes, err := os.ReadFile(chain.ArtifactIndexPath)
	if err != nil {
		return err
	}
	findingBytes, err := os.ReadFile(chain.FindingPath)
	if err != nil {
		return err
	}
	sourceReviewBytes, err := os.ReadFile(chain.SourceReviewPath)
	if err != nil {
		return err
	}
	rounds, ok := parseDay2ReviewRounds(string(sourceReviewBytes))
	if !ok || len(rounds) == 0 {
		return errLearningReviewBinding
	}
	last := rounds[len(rounds)-1]
	if !day2ReviewCurrent(string(sourceReviewBytes), last.Reviewer, learningSHA256(findingBytes), last.EvidenceRefs, "accepted") {
		return errLearningReviewBinding
	}

	stateRoot := filepath.Dir(filepath.Dir(chain.ArtifactIndexPath))
	caseRoot := filepath.Dir(stateRoot)
	reviewRoot := filepath.Dir(chain.SourceReviewPath)
	for evidenceRef, evidenceSHA := range last.EvidenceRefs {
		evidencePath := filepath.Clean(filepath.Join(reviewRoot, filepath.FromSlash(evidenceRef)))
		if !pathWithin(evidencePath, stateRoot) {
			return errLearningReviewBinding
		}
		evidenceBytes, err := os.ReadFile(evidencePath)
		if err != nil || learningSHA256(evidenceBytes) != evidenceSHA {
			return errLearningReviewBinding
		}
		binding, err := parseLearningArtifactBinding(string(evidenceBytes))
		if err != nil || !artifactIndexContainsBinding(string(indexBytes), binding) {
			return errLearningReviewBinding
		}
		artifactPath := filepath.Clean(filepath.Join(caseRoot, filepath.FromSlash(binding.Path)))
		if !pathWithin(artifactPath, caseRoot) {
			return errLearningReviewBinding
		}
		info, err := os.Lstat(artifactPath)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errLearningReviewBinding
		}
		artifactBytes, err := os.ReadFile(artifactPath)
		if err != nil || len(artifactBytes) != binding.Bytes || learningSHA256(artifactBytes) != binding.SHA256 {
			return errLearningReviewBinding
		}
		findingRef := "- `../evidence/" + filepath.Base(evidencePath) + "`"
		if !strings.Contains(string(findingBytes), findingRef) {
			return errLearningReviewBinding
		}
	}
	return nil
}

func parseLearningArtifactBinding(evidence string) (learningArtifactBinding, error) {
	fields := map[string]string{}
	for line := range strings.SplitSeq(evidence, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") || !strings.HasSuffix(line, "`") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), "：`")
		if ok {
			fields[key] = strings.TrimSuffix(value, "`")
		}
	}
	bytesCount, err := strconv.Atoi(fields["Artifact bytes"])
	binding := learningArtifactBinding{
		Alias:         fields["Artifact alias"],
		Path:          fields["Artifact path"],
		SHA256:        fields["Artifact SHA-256"],
		Bytes:         bytesCount,
		AuthorizedUse: fields["Authorized use"],
	}
	if err != nil || binding.Alias == "" || binding.Path == "" || len(binding.SHA256) != 64 || binding.Bytes < 0 || binding.AuthorizedUse == "" {
		return learningArtifactBinding{}, errLearningReviewBinding
	}
	return binding, nil
}

func artifactIndexContainsBinding(index string, binding learningArtifactBinding) bool {
	marker := "## `" + binding.Alias + "`"
	start := strings.Index(index, marker)
	if start < 0 {
		return false
	}
	entry := index[start:]
	if next := strings.Index(entry[len(marker):], "\n## `"); next >= 0 {
		entry = entry[:len(marker)+next]
	}
	return strings.Contains(entry, "相对路径：`"+binding.Path+"`") &&
		strings.Contains(entry, "SHA-256：`"+binding.SHA256+"`") &&
		strings.Contains(entry, "Bytes：`"+strconv.Itoa(binding.Bytes)+"`") &&
		strings.Contains(entry, "授权范围：`"+binding.AuthorizedUse+"`")
}

func applyReviewedLearningPatch(git, repo, manifestRel, patchPath string, sourceChain learningSourceChain, confirmation learningConfirmation, candidate, review []byte) error {
	if !confirmation.Confirmed {
		return errLearningConfirmationRequired
	}
	findingBytes, findingErr := os.ReadFile(sourceChain.FindingPath)
	sourceReviewBytes, sourceReviewErr := os.ReadFile(sourceChain.SourceReviewPath)
	if learningSHA256(candidate) != confirmation.CandidateSHA256 || learningSHA256(review) != confirmation.ReviewSHA256 ||
		confirmation.SnapshotDigest != sourceChain.ExpectedSnapshotDigest ||
		confirmation.SourceFindingPath != filepath.ToSlash(sourceChain.FindingPath) || findingErr != nil || learningSHA256(findingBytes) != confirmation.SourceFindingSHA ||
		confirmation.SourceReviewPath != filepath.ToSlash(sourceChain.SourceReviewPath) || sourceReviewErr != nil || learningSHA256(sourceReviewBytes) != confirmation.SourceReviewSHA {
		return errLearningReviewBinding
	}
	if validateLearningSourceChain(sourceChain) != nil {
		return errLearningReviewBinding
	}
	binding, err := parseLearningReviewBinding(string(review))
	if err != nil || binding.Reviewer != "reviewer" || binding.Eligibility != "eligible" || binding.PatchDecision != "accepted" ||
		binding.CandidateSHA256 != confirmation.CandidateSHA256 || binding.PatchSHA256 != confirmation.PatchSHA256 {
		return errLearningReviewBinding
	}
	patch, err := os.ReadFile(patchPath)
	if err != nil {
		return err
	}
	if learningSHA256(patch) != confirmation.PatchSHA256 {
		return errLearningPatchDrift
	}
	currentRevision, err := gitOutput(git, repo, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(currentRevision) != binding.BaseRevision {
		return errLearningRevisionDrift
	}
	manifestBlob, err := gitOutput(git, repo, "hash-object", "--path="+manifestRel, "--", manifestRel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(manifestBlob) != binding.ManifestBaseBlob {
		return errLearningManifestDrift
	}
	if err := requireReviewedLearningPatchScope(git, repo, manifestRel, patchPath, binding.TargetRel); err != nil {
		return err
	}
	currentBlob, err := gitOutput(git, repo, "hash-object", "--path="+binding.TargetRel, "--", binding.TargetRel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(currentBlob) != binding.TargetBaseBlob {
		return errLearningBaseDrift
	}
	if _, err := gitOutput(git, repo, "apply", "--check", patchPath); err != nil {
		return err
	}
	_, err = gitOutput(git, repo, "apply", patchPath)
	return err
}

func renderLearningReviewFixture(binding learningReviewBinding) []byte {
	return []byte(strings.Join([]string{
		"# LR-001 — Review of L-001",
		"",
		"- Reviewer 单写者：`" + binding.Reviewer + "`",
		"- Candidate SHA-256：`" + binding.CandidateSHA256 + "`",
		"- Proposed destination：`" + binding.TargetRel + "`",
		"",
		"## Checkpoint A — Eligibility",
		"",
		"- Decision：`" + binding.Eligibility + "`",
		"",
		"## Checkpoint B — Exact proposal patch",
		"",
		"- Canonical target：`" + binding.TargetRel + "`",
		"- Base revision：`" + binding.BaseRevision + "`",
		"- Manifest base blob：`" + binding.ManifestBaseBlob + "`",
		"- Target base blob：`" + binding.TargetBaseBlob + "`",
		"- Patch SHA-256：`" + binding.PatchSHA256 + "`",
		"- Patch target count：`1`",
		"- Added-lines deny result：`clear`",
		"- `git apply --check` result：`pass`",
		"- Patch decision：`" + binding.PatchDecision + "`",
		"",
	}, "\n"))
}

func parseLearningReviewBinding(review string) (learningReviewBinding, error) {
	fields := map[string]string{}
	for line := range strings.SplitSeq(review, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") || !strings.HasSuffix(line, "`") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimPrefix(line, "- "), "：`")
		if !ok {
			continue
		}
		if fields[key] != "" {
			return learningReviewBinding{}, errLearningReviewBinding
		}
		fields[key] = strings.TrimSuffix(value, "`")
	}
	binding := learningReviewBinding{
		Reviewer:         fields["Reviewer 单写者"],
		CandidateSHA256:  fields["Candidate SHA-256"],
		TargetRel:        fields["Canonical target"],
		BaseRevision:     fields["Base revision"],
		ManifestBaseBlob: fields["Manifest base blob"],
		TargetBaseBlob:   fields["Target base blob"],
		PatchSHA256:      fields["Patch SHA-256"],
		Eligibility:      fields["Decision"],
		PatchDecision:    fields["Patch decision"],
	}
	for name, value := range map[string]string{
		"Reviewer": binding.Reviewer, "CandidateSHA256": binding.CandidateSHA256, "TargetRel": binding.TargetRel,
		"BaseRevision": binding.BaseRevision, "ManifestBaseBlob": binding.ManifestBaseBlob, "TargetBaseBlob": binding.TargetBaseBlob,
		"PatchSHA256": binding.PatchSHA256, "Eligibility": binding.Eligibility, "PatchDecision": binding.PatchDecision,
	} {
		if value == "" {
			return learningReviewBinding{}, fmt.Errorf("%w: missing %s", errLearningReviewBinding, name)
		}
	}
	if fields["Patch target count"] != "1" || fields["Added-lines deny result"] != "clear" || fields["`git apply --check` result"] != "pass" {
		return learningReviewBinding{}, errLearningReviewBinding
	}
	return binding, nil
}

func runGit(t *testing.T, git, dir string, args ...string) string {
	t.Helper()
	out, err := gitOutput(git, dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return out
}

func gitOutput(git, dir string, args ...string) (string, error) {
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", errors.New(strings.TrimSpace(stderr.String()) + ": " + err.Error())
	}
	return stdout.String(), nil
}

func writeLearningFixtureFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readLearningFixtureFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
