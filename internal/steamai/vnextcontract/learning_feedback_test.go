package vnextcontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearningFeedbackContractRequiresExactReviewedGitPatch(t *testing.T) {
	repo := repoRoot(t)
	contract := readPrototypeFile(t, repo, "vnext/learning-feedback.md")
	for _, required := range []string{
		"accepted` finding/review",
		"证据是否足以支持泛化后的经验",
		"跨 case 通用",
		"是否重复或冲突",
		"是否通过脱敏",
		"用户确认前不得编辑 canonical source pack",
		"git diff --binary --full-index --no-ext-diff",
		"git apply --check <PATCH_PATH>",
		"git apply --numstat -z",
		"patch SHA-256",
		"已跟踪、非 symlink 的 Markdown 文件",
		"git hash-object --path=<PACK_TARGET>",
		"filter-aware `git hash-object --path=<PACK_TARGET>` 的当前 target blob 必须等于 proposal 记录的 `HEAD:<PACK_TARGET>` base blob",
		"不 fuzzy apply、不 retry、不覆盖",
		"应用不自动 commit 或 push",
		"从 selected pack 的 exact source revision 导出 case-local 只读 snapshot",
		"只记录标签而未物化或未绑定读取路径不算 snapshot",
		"所有 pack 指令读取都从该 snapshot 目录解析",
	} {
		assertContains(t, contract, required, "learning feedback contract")
	}
	for _, forbidden := range []string{"BoundedDiff", "/rekit promote", "writeback/reconcile 状态机"} {
		assertContains(t, contract, forbidden, "explicitly forbidden legacy learning path")
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
		!strings.Contains(patch, "Record counterexamples") ||
		strings.Contains(patch, "truncated") {
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
	metadata := strings.Join([]string{
		"pack: binary-re",
		"revision: " + revision,
		"tree: " + tree,
		"",
	}, "\n")
	writeLearningFixtureFile(t, filepath.Join(snapshotRoot, "snapshot.yml"), metadata)
	return target
}

func normalizeLearningText(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

var (
	errLearningConfirmationRequired = errors.New("learning patch requires user confirmation")
	errLearningBaseDrift            = errors.New("learning patch target base drifted")
	errLearningPatchDrift           = errors.New("learning patch bytes drifted after confirmation")
	errLearningPatchScope           = errors.New("learning patch target scope changed")
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
