package vnextcontract

import (
	"bytes"
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
		"current accepted review",
		"source finding/review",
		"artifact alias、case-relative path、SHA-256、bytes 与 authorized use",
		"实际 artifact bytes 一致",
		"完整排序 file manifest",
		"`learningTargets`",
		"`denyPatterns`",
		"immutable",
		"只有 `Decision: eligible` 才可进入 batch",
		"candidate review 不绑定或授权 patch",
		"用户确认前不得编辑 canonical source pack",
		"git diff --binary --full-index --no-ext-diff",
		"git apply --check <PATCH_PATH>",
		"git apply --numstat -z",
		"一个 batch 可以包含多个 candidate",
		"多个现有 Markdown targets",
		"Preimage SHA-256",
		"Postimage SHA-256",
		"CONFIRM STEAMAI LEARNING BATCH <batch_identity>",
		"不自动 `git add`、commit、push",
		"HEAD、index、当前 case snapshot 必须不变",
		"candidate 文件不得包含自身 exact SHA 字段",
		"更早的 case 仍可继续研究",
		"解析 learning artifact 前明确拒绝",
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

func TestLearningImplementationLivesInProductionPackage(t *testing.T) {
	repo := repoRoot(t)
	for _, rel := range []string{
		"internal/steamai/learningbatch/preview.go",
		"internal/steamai/learningbatch/apply.go",
		"internal/steamai/learningbatch/sourcechain.go",
		"internal/steamai/learningbatch/gate3.go",
		"internal/steamai/learningbatch/types.go",
		"internal/steamai/evaluation_cli.go",
		"internal/steamai/evaluation/types.go",
		"internal/steamai/evaluation/publish_linux.go",
		"internal/steamai/evaluation/publish_other.go",
		"internal/steamai/evaluation/publish_windows.go",
		"internal/steamai/evaluation/run.go",
		"internal/steamai/evaluation/suite.go",
		"internal/steamai/evaluation/verify.go",
	} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("production learning implementation missing: %s: %v", rel, err)
		}
	}
}

func normalizeLearningText(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func runGit(t *testing.T, git, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(errors.New(strings.TrimSpace(stderr.String()) + ": " + err.Error()))
	}
	return stdout.String()
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
