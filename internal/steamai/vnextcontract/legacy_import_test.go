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
	"slices"
	"sort"
	"strings"
	"testing"
)

const importFileSizeLimit = 1 << 20

var contractSourcePaths = []string{
	"vnext/learning-feedback.md",
	"vnext/legacy-import.md",
	"vnext/templates/case/CLAUDE.md",
	"vnext/templates/member/CLAUDE.md",
	"vnext/templates/roles/analysis-member.md",
	"vnext/templates/roles/reviewer.md",
	"vnext/templates/research/artifact-index.md",
	"vnext/templates/research/evidence.md",
	"vnext/templates/research/finding.md",
	"vnext/templates/research/learning-candidate.md",
	"vnext/templates/research/review.md",
}

type legacyImportSource struct {
	RootName string
	Files    map[string][]byte
}

type legacyImportPreview struct {
	SourceKind    string
	Project       string
	Goal          string
	Authorization string
	Prohibited    string
	StopCondition string
	Pack          string
	PackRevision  string
	PackTree      string
	Files         map[string]string
	PlannedWrites map[string][]byte
	Unresolved    []string
	Identity      string
}

var (
	errLegacyImportConflict   = errors.New("legacy import roots conflict")
	errLegacyImportDrift      = errors.New("legacy import source drifted")
	errLegacyImportUnsafe     = errors.New("legacy import source is not a bounded regular in-root file")
	errLegacyImportUnresolved = errors.New("legacy import has unresolved required fields")
	errLegacyImportCollision  = errors.New("legacy import target collides with unrecognized content")
)

func TestCanonicalSkillAndLegacyImportContractAreThinAndReadOnly(t *testing.T) {
	repo := repoRoot(t)
	skill := readPrototypeFile(t, repo, ".claude/skills/steamai/SKILL.md")
	template := readPrototypeFile(t, repo, "vnext/project-skill/SKILL.md")
	if skill != template {
		t.Fatal("canonical skill and project delivery template are not byte-for-byte equal")
	}
	contract := readPrototypeFile(t, repo, "vnext/legacy-import.md")
	for _, required := range []string{
		"一次性只读 importer",
		"不修改、删除、重命名或续写 `.steamai/`、`.rekit/`",
		"不 dual-write",
		"不回退旧 runtime",
		"零写入 import preview",
		"source file 的 relative path、SHA-256 和 bytes",
		"`.steamai-vnext/contracts/`",
	} {
		assertContains(t, skill+"\n"+contract, required, "canonical legacy import contract")
	}
	for _, rejected := range []string{
		"session ID、PID、endpoint",
		"authority、confirmed、authorized-gate",
		"执行旧 executable、script、Apply action",
	} {
		assertContains(t, contract, rejected, "legacy rejected field contract")
	}
}

func TestLegacyImportPreviewIsReadOnlyAndApplyRejectsSourceDrift(t *testing.T) {
	git := requireImportGit(t)
	repo := repoRoot(t)
	revision := strings.TrimSpace(runImportGit(t, git, repo, "rev-parse", "HEAD"))
	caseRoot := t.TempDir()
	legacyRoot := filepath.Join(caseRoot, ".rekit")
	writeCompleteLegacyFixture(t, legacyRoot)
	before := snapshotImportTree(t, caseRoot)

	preview, err := previewLegacyImport(caseRoot, repo, revision, git)
	if err != nil {
		t.Fatal(err)
	}
	if preview.SourceKind != "legacy-rekit" || preview.Goal != "inspect bounded fixture" || preview.Pack != "binary-re" || preview.Identity == "" {
		t.Fatalf("unexpected import preview: %+v", preview)
	}
	if len(preview.Unresolved) != 0 {
		t.Fatalf("complete import unexpectedly unresolved: %v", preview.Unresolved)
	}
	if after := snapshotImportTree(t, caseRoot); !equalImportSnapshots(before, after) {
		t.Fatal("legacy import preview mutated the case")
	}

	writeImportFixture(t, filepath.Join(legacyRoot, "mission.md"), completeMissionFixture("drifted fixture"))
	if err := applyLegacyImport(caseRoot, repo, revision, git, preview, true); !errors.Is(err, errLegacyImportDrift) {
		t.Fatalf("drifted import apply error=%v, want source drift", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai-vnext")); !os.IsNotExist(err) {
		t.Fatalf("drifted import created current state: %v", err)
	}
}

func TestLegacyImportApplyPublishesSelfContainedThinCaseAndPreservesLegacyBytes(t *testing.T) {
	git := requireImportGit(t)
	repo := repoRoot(t)
	revision := strings.TrimSpace(runImportGit(t, git, repo, "rev-parse", "HEAD"))
	caseRoot := t.TempDir()
	legacyRoot := filepath.Join(caseRoot, ".steamai")
	writeCompleteLegacyFixture(t, legacyRoot)
	beforeLegacy := snapshotImportTree(t, legacyRoot)

	preview, err := previewLegacyImport(caseRoot, repo, revision, git)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyLegacyImport(caseRoot, repo, revision, git, preview, false); err == nil {
		t.Fatal("unconfirmed import apply succeeded")
	}
	if err := applyLegacyImport(caseRoot, repo, revision, git, preview, true); err != nil {
		t.Fatal(err)
	}
	if afterLegacy := snapshotImportTree(t, legacyRoot); !equalImportSnapshots(beforeLegacy, afterLegacy) {
		t.Fatal("legacy root changed during import")
	}

	current := filepath.Join(caseRoot, ".steamai-vnext")
	for _, rel := range []string{"CLAUDE.md", "import.md", "contracts/legacy-import.md", "contracts/learning-feedback.md", "contracts/templates/roles/reviewer.md", "pack-snapshot/snapshot.yml", "artifacts/index.md", "evidence", "findings", "reviews", "learnings/candidates"} {
		if _, err := os.Lstat(filepath.Join(current, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing imported thin-core path %s: %v", rel, err)
		}
	}
	assertImportFileEquals(t, filepath.Join(repo, ".claude", "skills", "steamai", "SKILL.md"), filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md"))
	for _, sourceRel := range contractSourcePaths {
		targetRel := strings.TrimPrefix(sourceRel, "vnext/")
		assertImportFileEquals(t, filepath.Join(repo, filepath.FromSlash(sourceRel)), filepath.Join(current, "contracts", filepath.FromSlash(targetRel)))
	}
	for _, sourceRel := range []string{"common/policies/manifest.yml", "common/policies/agent-team.md"} {
		want := runImportGitBytes(t, git, repo, "show", revision+":"+sourceRel)
		got, err := os.ReadFile(filepath.Join(current, "pack-snapshot", filepath.FromSlash(sourceRel)))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("snapshot %s is not exact-revision content", sourceRel)
		}
	}
	packManifestRel := "packs/binary-re/manifest.yml"
	wantManifest := runImportGitBytes(t, git, repo, "show", revision+":"+packManifestRel)
	gotManifest, err := os.ReadFile(filepath.Join(current, "pack-snapshot", filepath.FromSlash(packManifestRel)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotManifest, wantManifest) {
		t.Fatal("imported pack snapshot is not exact-revision content")
	}
	metadata, err := os.ReadFile(filepath.Join(current, "pack-snapshot", "snapshot.yml"))
	if err != nil {
		t.Fatal(err)
	}
	commonTree := strings.TrimSpace(runImportGit(t, git, repo, "rev-parse", revision+":common"))
	for _, required := range []string{"revision: " + preview.PackRevision, "tree: " + preview.PackTree, "common-tree: " + commonTree} {
		if !strings.Contains(string(metadata), required) {
			t.Fatalf("snapshot metadata missing %q", required)
		}
	}
	if _, err := os.Lstat(filepath.Join(current, "runtime")); !os.IsNotExist(err) {
		t.Fatalf("import published legacy runtime: %v", err)
	}
}

func TestLegacyImportRequiresExplicitAuthorizationAndStopBoundary(t *testing.T) {
	git := requireImportGit(t)
	repo := repoRoot(t)
	revision := strings.TrimSpace(runImportGit(t, git, repo, "rev-parse", "HEAD"))
	caseRoot := t.TempDir()
	legacyRoot := filepath.Join(caseRoot, ".rekit")
	writeImportFixture(t, filepath.Join(legacyRoot, "instance.yml"), "project: fixture-case\npack: binary-re\n")
	writeImportFixture(t, filepath.Join(legacyRoot, "mission.md"), "goal: inspect bounded fixture\n")

	preview, err := previewLegacyImport(caseRoot, repo, revision, git)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"authorization", "prohibited", "stop"} {
		if !containsImportString(preview.Unresolved, field) {
			t.Fatalf("missing unresolved field %q in %v", field, preview.Unresolved)
		}
	}
	if err := applyLegacyImport(caseRoot, repo, revision, git, preview, true); !errors.Is(err, errLegacyImportUnresolved) {
		t.Fatalf("incomplete import apply error=%v, want unresolved", err)
	}
}

func TestLegacyImportFailsClosedOnMixedRootsAndSymlinkSource(t *testing.T) {
	git := requireImportGit(t)
	repo := repoRoot(t)
	revision := strings.TrimSpace(runImportGit(t, git, repo, "rev-parse", "HEAD"))

	for _, roots := range [][]string{{".steamai", ".rekit"}, {".steamai-vnext", ".steamai"}, {".steamai-vnext", ".rekit"}} {
		caseRoot := t.TempDir()
		for _, root := range roots {
			if root == ".steamai-vnext" {
				writeImportFixture(t, filepath.Join(caseRoot, root, "CLAUDE.md"), "# Current fixture\n")
				continue
			}
			writeCompleteLegacyFixture(t, filepath.Join(caseRoot, root))
		}
		if _, err := previewLegacyImport(caseRoot, repo, revision, git); !errors.Is(err, errLegacyImportConflict) {
			t.Fatalf("roots %v preview error=%v, want conflict", roots, err)
		}
	}

	symlinkCase := t.TempDir()
	legacy := filepath.Join(symlinkCase, ".rekit")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(symlinkCase, "outside.yml")
	writeImportFixture(t, target, "project: fixture\npack: binary-re\n")
	if err := os.Symlink(target, filepath.Join(legacy, "instance.yml")); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	writeImportFixture(t, filepath.Join(legacy, "mission.md"), completeMissionFixture("bounded"))
	if _, err := previewLegacyImport(symlinkCase, repo, revision, git); !errors.Is(err, errLegacyImportUnsafe) {
		t.Fatalf("symlink preview error=%v, want unsafe", err)
	}
}

func previewLegacyImport(caseRoot, canonicalRoot, revision, git string) (legacyImportPreview, error) {
	if exists, err := importPathExists(filepath.Join(caseRoot, ".steamai-vnext")); err != nil {
		return legacyImportPreview{}, err
	} else if exists {
		return legacyImportPreview{}, errLegacyImportConflict
	}

	roots := []legacyImportSource{}
	for _, rootName := range []string{".steamai", ".rekit"} {
		root := filepath.Join(caseRoot, rootName)
		if exists, err := importPathExists(root); err != nil {
			return legacyImportPreview{}, err
		} else if exists {
			roots = append(roots, legacyImportSource{RootName: rootName, Files: map[string][]byte{}})
		}
	}
	if len(roots) != 1 {
		return legacyImportPreview{}, errLegacyImportConflict
	}

	source := &roots[0]
	for _, rel := range []string{"instance.yml", "mission.md"} {
		path := filepath.Join(caseRoot, source.RootName, rel)
		data, err := readBoundedImportFile(path)
		if err != nil {
			return legacyImportPreview{}, err
		}
		source.Files[rel] = data
	}
	fields := map[string]string{}
	for rel, data := range source.Files {
		sum := sha256.Sum256(data)
		fields[rel] = fmt.Sprintf("sha256:%s bytes:%d", hex.EncodeToString(sum[:]), len(data))
	}
	preview := legacyImportPreview{
		SourceKind:    "legacy-" + strings.TrimPrefix(source.RootName, "."),
		Project:       importField(string(source.Files["instance.yml"]), "project"),
		Goal:          importField(string(source.Files["mission.md"]), "goal"),
		Authorization: importField(string(source.Files["mission.md"]), "authorization"),
		Prohibited:    importField(string(source.Files["mission.md"]), "prohibited"),
		StopCondition: importField(string(source.Files["mission.md"]), "stop"),
		Pack:          importField(string(source.Files["instance.yml"]), "pack"),
		PackRevision:  strings.TrimSpace(revision),
		Files:         fields,
	}
	for key, value := range map[string]string{
		"project":       preview.Project,
		"goal":          preview.Goal,
		"authorization": preview.Authorization,
		"prohibited":    preview.Prohibited,
		"stop":          preview.StopCondition,
		"pack":          preview.Pack,
	} {
		if strings.TrimSpace(value) == "" {
			preview.Unresolved = append(preview.Unresolved, key)
		}
	}
	sort.Strings(preview.Unresolved)
	if preview.Pack != "" {
		if preview.Pack == "_template" {
			preview.Unresolved = append(preview.Unresolved, "pack-maturity")
		} else {
			packTree, err := gitImportText(git, canonicalRoot, "rev-parse", preview.PackRevision+":packs/"+preview.Pack)
			if err != nil {
				preview.Unresolved = append(preview.Unresolved, "pack-revision")
			} else {
				preview.PackTree = strings.TrimSpace(packTree)
			}
		}
	}
	writes, err := buildImportWrites(canonicalRoot, git, preview)
	if err != nil {
		return legacyImportPreview{}, err
	}
	preview.PlannedWrites = writes
	preview.Identity = importPreviewIdentity(preview)
	return preview, nil
}

func buildImportWrites(canonicalRoot, git string, preview legacyImportPreview) (map[string][]byte, error) {
	writes := map[string][]byte{}
	skill, err := readBoundedImportFile(filepath.Join(canonicalRoot, ".claude", "skills", "steamai", "SKILL.md"))
	if err != nil {
		return nil, err
	}
	writes[".claude/skills/steamai/SKILL.md"] = skill
	for _, sourceRel := range contractSourcePaths {
		data, err := readBoundedImportFile(filepath.Join(canonicalRoot, filepath.FromSlash(sourceRel)))
		if err != nil {
			return nil, err
		}
		targetRel := strings.TrimPrefix(sourceRel, "vnext/")
		writes[".steamai-vnext/contracts/"+targetRel] = data
	}
	if preview.Pack != "" && preview.PackTree != "" {
		list, err := gitImportText(git, canonicalRoot, "ls-tree", "-r", "--name-only", preview.PackRevision, "--", "packs/"+preview.Pack, "common")
		if err != nil {
			return nil, err
		}
		for rel := range strings.SplitSeq(strings.TrimSpace(list), "\n") {
			if rel == "" {
				continue
			}
			data, err := gitImportBytes(git, canonicalRoot, "show", preview.PackRevision+":"+rel)
			if err != nil {
				return nil, err
			}
			writes[".steamai-vnext/pack-snapshot/"+rel] = data
		}
		commonTree, err := gitImportText(git, canonicalRoot, "rev-parse", preview.PackRevision+":common")
		if err != nil {
			return nil, err
		}
		writes[".steamai-vnext/pack-snapshot/snapshot.yml"] = []byte(strings.Join([]string{
			"pack: " + preview.Pack,
			"revision: " + preview.PackRevision,
			"tree: " + preview.PackTree,
			"common-tree: " + strings.TrimSpace(commonTree),
			"",
		}, "\n"))
	}
	writes[".steamai-vnext/CLAUDE.md"] = []byte(strings.Join([]string{
		"# Imported STeamAI case",
		"",
		"Project: " + preview.Project,
		"Goal: " + preview.Goal,
		"Authorization: " + preview.Authorization,
		"Prohibited: " + preview.Prohibited,
		"Stop: " + preview.StopCondition,
		"Pack: " + preview.Pack,
		"Source revision: " + preview.PackRevision,
		"Snapshot tree: " + preview.PackTree,
		"",
	}, "\n"))
	writes[".steamai-vnext/artifacts/index.md"] = []byte("# Artifact index\n")
	return writes, nil
}

func applyLegacyImport(caseRoot, canonicalRoot, revision, git string, preview legacyImportPreview, confirmed bool) error {
	if !confirmed {
		return errors.New("legacy import requires user confirmation")
	}
	if len(preview.Unresolved) != 0 {
		return errLegacyImportUnresolved
	}
	fresh, err := previewLegacyImport(caseRoot, canonicalRoot, revision, git)
	if err != nil {
		return err
	}
	if fresh.Identity != preview.Identity {
		return errLegacyImportDrift
	}
	if len(fresh.Unresolved) != 0 {
		return errLegacyImportUnresolved
	}
	for rel, wanted := range fresh.PlannedWrites {
		path := filepath.Join(caseRoot, filepath.FromSlash(rel))
		if existing, err := os.ReadFile(path); err == nil {
			if !bytes.Equal(existing, wanted) {
				return fmt.Errorf("%w: %s", errLegacyImportCollision, rel)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	for _, rel := range []string{".steamai-vnext/evidence", ".steamai-vnext/findings", ".steamai-vnext/reviews", ".steamai-vnext/learnings/candidates"} {
		if err := os.MkdirAll(filepath.Join(caseRoot, filepath.FromSlash(rel)), 0o755); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(fresh.PlannedWrites))
	for rel := range fresh.PlannedWrites {
		keys = append(keys, rel)
	}
	sort.Strings(keys)
	for _, rel := range keys {
		path := filepath.Join(caseRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, fresh.PlannedWrites[rel], 0o644); err != nil {
			return err
		}
	}
	importRecord := strings.Join([]string{
		"# Import",
		"",
		"Source: " + fresh.SourceKind,
		"Identity: " + fresh.Identity,
		"Pack revision: " + fresh.PackRevision,
		"Pack tree: " + fresh.PackTree,
		"Legacy roots remain read-only.",
		"",
	}, "\n")
	return os.WriteFile(filepath.Join(caseRoot, ".steamai-vnext", "import.md"), []byte(importRecord), 0o644)
}

func readBoundedImportFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > importFileSizeLimit {
		return nil, errLegacyImportUnsafe
	}
	return os.ReadFile(path)
}

func importPathExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func importField(text, key string) string {
	prefix := key + ":"
	for line := range strings.SplitSeq(text, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func importPreviewIdentity(preview legacyImportPreview) string {
	var input strings.Builder
	fmt.Fprintf(&input, "%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n", preview.SourceKind, preview.Project, preview.Goal, preview.Authorization, preview.Prohibited, preview.StopCondition, preview.Pack, preview.PackRevision, preview.PackTree)
	for _, key := range sortedImportKeys(preview.Files) {
		fmt.Fprintf(&input, "source:%s=%s\n", key, preview.Files[key])
	}
	writeHashes := map[string]string{}
	for rel, data := range preview.PlannedWrites {
		sum := sha256.Sum256(data)
		writeHashes[rel] = fmt.Sprintf("sha256:%s bytes:%d", hex.EncodeToString(sum[:]), len(data))
	}
	for _, key := range sortedImportKeys(writeHashes) {
		fmt.Fprintf(&input, "write:%s=%s\n", key, writeHashes[key])
	}
	for _, unresolved := range preview.Unresolved {
		fmt.Fprintf(&input, "unresolved:%s\n", unresolved)
	}
	sum := sha256.Sum256([]byte(input.String()))
	return hex.EncodeToString(sum[:])
}

func sortedImportKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func gitImportText(git, repo string, args ...string) (string, error) {
	data, err := gitImportBytes(git, repo, args...)
	return string(data), err
}

func gitImportBytes(git, repo string, args ...string) ([]byte, error) {
	cmd := exec.Command(git, append([]string{"-C", repo}, args...)...)
	data, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return data, nil
}

func snapshotImportTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			out[filepath.ToSlash(rel)+"/"] = "dir"
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func equalImportSnapshots(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func containsImportString(values []string, wanted string) bool {
	return slices.Contains(values, wanted)
}

func completeMissionFixture(goal string) string {
	return strings.Join([]string{
		"goal: " + goal,
		"authorization: analyze local synthetic fixture only",
		"prohibited: no network or heavy action",
		"stop: stop on scope ambiguity",
		"",
	}, "\n")
}

func writeCompleteLegacyFixture(t *testing.T, legacyRoot string) {
	t.Helper()
	writeImportFixture(t, filepath.Join(legacyRoot, "instance.yml"), "project: fixture-case\npack: binary-re\n")
	writeImportFixture(t, filepath.Join(legacyRoot, "mission.md"), completeMissionFixture("inspect bounded fixture"))
}

func requireImportGit(t *testing.T) string {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locate git for legacy import contract: %v", err)
	}
	return git
}

func runImportGit(t *testing.T, git, repo string, args ...string) string {
	t.Helper()
	data := runImportGitBytes(t, git, repo, args...)
	return string(data)
}

func runImportGitBytes(t *testing.T, git, repo string, args ...string) []byte {
	t.Helper()
	data, err := gitImportBytes(git, repo, args...)
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return data
}

func assertImportFileEquals(t *testing.T, left, right string) {
	t.Helper()
	leftData, err := os.ReadFile(left)
	if err != nil {
		t.Fatal(err)
	}
	rightData, err := os.ReadFile(right)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(leftData, rightData) {
		t.Fatalf("files differ: %s != %s", left, right)
	}
}

func writeImportFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
