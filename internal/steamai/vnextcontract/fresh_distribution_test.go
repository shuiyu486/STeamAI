package vnextcontract

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type freshCaseFacts struct {
	Name          string
	Goal          string
	Authorization string
	Prohibited    string
	Stop          string
	Pack          string
}

type freshPlannedWrite struct {
	SourceKind      string
	SourceRel       string
	SourceBlob      string
	GitMode         string
	TargetRel       string
	TargetAction    string
	TargetPreState  string
	TargetPreSHA256 string
	TargetPreBytes  int
	SHA256          string
	Bytes           []byte
}

type freshDistributionPreview struct {
	Revision       string
	PackTree       string
	CommonTree     string
	SnapshotDigest string
	Facts          freshCaseFacts
	Writes         []freshPlannedWrite
	Identity       string
}

type freshGitEntry struct {
	Mode string
	Blob string
	Path string
}

var (
	errFreshConfirmationRequired = errors.New("fresh distribution requires exact user confirmation")
	errFreshSourceDrift          = errors.New("fresh distribution source revision drifted")
	errFreshTargetDrift          = errors.New("fresh distribution target drifted")
	errFreshPartialTarget        = errors.New("fresh distribution found a partial target")
	errFreshCollision            = errors.New("fresh distribution target collides with unrecognized content")
	errFreshInjectedFailure      = errors.New("fresh distribution injected failure before publication")
)

func TestFreshDistributionRejectsAuthoringOrInvalidPack(t *testing.T) {
	git := requireFreshGit(t)
	repo := repoRoot(t)
	source := buildFreshCanonicalFixture(t, git, repo)
	base := freshCaseFacts{
		Name:          "synthetic-case",
		Goal:          "verify selected pack eligibility",
		Authorization: "temporary fixture files only",
		Prohibited:    "network or real artifacts",
		Stop:          "invalid pack selection",
	}
	for _, pack := range []string{"", "_template", "binary-re/nested", "../binary-re"} {
		t.Run(strings.ReplaceAll(pack, "/", "-"), func(t *testing.T) {
			caseRoot := filepath.Join(t.TempDir(), "case")
			if err := os.MkdirAll(caseRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			facts := base
			facts.Pack = pack
			if _, err := previewFreshDistribution(git, source, caseRoot, facts); !errors.Is(err, errFreshCollision) {
				t.Fatalf("invalid selected pack %q returned %v", pack, err)
			}
		})
	}
}

func TestFreshDistributionPreviewIsZeroWriteAndApplyIsExact(t *testing.T) {
	git := requireFreshGit(t)
	repo := repoRoot(t)
	source := buildFreshCanonicalFixture(t, git, repo)
	caseRoot := filepath.Join(t.TempDir(), "case")
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	facts := freshCaseFacts{
		Name:          "synthetic-case",
		Goal:          "verify the fresh distribution contract",
		Authorization: "temporary fixture files only",
		Prohibited:    "network, external side effects, or real artifacts",
		Stop:          "scope drift or failed currentness check",
		Pack:          "binary-re",
	}
	before := snapshotFreshTree(t, caseRoot)
	preview, err := previewFreshDistribution(git, source, caseRoot, facts)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Identity == "" || preview.Revision == "" || preview.PackTree == "" || preview.CommonTree == "" || preview.SnapshotDigest == "" {
		t.Fatalf("fresh preview omitted identity: %+v", preview)
	}
	if after := snapshotFreshTree(t, caseRoot); !equalFreshSnapshots(before, after) {
		t.Fatal("fresh preview mutated the target")
	}
	if err := applyFreshDistribution(git, source, caseRoot, preview, false, false); !errors.Is(err, errFreshConfirmationRequired) {
		t.Fatalf("unconfirmed fresh apply returned %v", err)
	}
	if after := snapshotFreshTree(t, caseRoot); !equalFreshSnapshots(before, after) {
		t.Fatal("unconfirmed fresh apply mutated the target")
	}
	if err := applyFreshDistribution(git, source, caseRoot, preview, true, false); err != nil {
		t.Fatal(err)
	}
	for _, write := range preview.Writes {
		got, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(write.TargetRel)))
		if err != nil {
			t.Fatalf("read distributed %s: %v", write.TargetRel, err)
		}
		if string(got) != string(write.Bytes) {
			t.Fatalf("distributed bytes differ for %s", write.TargetRel)
		}
	}
	assertFreshSnapshotMatchesRevision(t, git, source, caseRoot, preview)
	gotDigest, err := freshMaterializedSnapshotDigest(caseRoot, preview.Writes)
	if err != nil {
		t.Fatal(err)
	}
	if gotDigest != preview.SnapshotDigest {
		t.Fatalf("materialized snapshot digest %s, want %s", gotDigest, preview.SnapshotDigest)
	}
	metadata, err := os.ReadFile(filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", "snapshot.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metadata), "payload-digest: "+preview.SnapshotDigest) || !strings.Contains(string(metadata), "git-mode:") || !strings.Contains(string(metadata), "git-blob:") {
		t.Fatal("snapshot metadata omitted the complete payload identity")
	}

	var drifted freshPlannedWrite
	for _, write := range preview.Writes {
		if isFreshSnapshotPayload(write) {
			drifted = write
			break
		}
	}
	driftPath := filepath.Join(caseRoot, filepath.FromSlash(drifted.TargetRel))
	if err := os.WriteFile(driftPath, append(append([]byte(nil), drifted.Bytes...), []byte("\nfixture drift\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if digest, err := freshMaterializedSnapshotDigest(caseRoot, preview.Writes); err == nil && digest == preview.SnapshotDigest {
		t.Fatal("snapshot payload drift kept the original digest")
	}
	if err := os.WriteFile(driftPath, drifted.Bytes, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(source); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		".claude/skills/steamai/SKILL.md",
		".steamai-vnext/CLAUDE.md",
		".steamai-vnext/contracts/learning-feedback.md",
		".steamai-vnext/pack-snapshot/snapshot.yml",
	} {
		if _, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("project-local case depends on removed source clone for %s: %v", rel, err)
		}
	}
}

func TestFreshDistributionRejectsDriftAndNeverPublishesPartialState(t *testing.T) {
	git := requireFreshGit(t)
	repo := repoRoot(t)
	facts := freshCaseFacts{
		Name:          "synthetic-case",
		Goal:          "verify fail-closed distribution",
		Authorization: "temporary fixture files only",
		Prohibited:    "external side effects",
		Stop:          "any drift",
		Pack:          "binary-re",
	}

	t.Run("source revision drift", func(t *testing.T) {
		source := buildFreshCanonicalFixture(t, git, repo)
		caseRoot := newFreshCaseRoot(t)
		preview, err := previewFreshDistribution(git, source, caseRoot, facts)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(source, "vnext", "learning-feedback.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, []byte("\nfixture revision drift\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		runFreshGit(t, git, source, "add", "--", "vnext/learning-feedback.md")
		runFreshGit(t, git, source, "commit", "--quiet", "-m", "drift")
		if err := applyFreshDistribution(git, source, caseRoot, preview, true, false); !errors.Is(err, errFreshSourceDrift) {
			t.Fatalf("source drift returned %v", err)
		}
		assertFreshStateRootMissing(t, caseRoot)
	})

	t.Run("loaded canonical skill drift", func(t *testing.T) {
		source := buildFreshCanonicalFixture(t, git, repo)
		caseRoot := newFreshCaseRoot(t)
		preview, err := previewFreshDistribution(git, source, caseRoot, facts)
		if err != nil {
			t.Fatal(err)
		}
		skill := filepath.Join(source, ".claude", "skills", "steamai", "SKILL.md")
		data, err := os.ReadFile(skill)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(skill, append(data, []byte("\nuncommitted drift\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := applyFreshDistribution(git, source, caseRoot, preview, true, false); !errors.Is(err, errFreshSourceDrift) {
			t.Fatalf("loaded skill drift returned %v", err)
		}
		assertFreshStateRootMissing(t, caseRoot)
	})

	t.Run("target pre-state drift", func(t *testing.T) {
		source := buildFreshCanonicalFixture(t, git, repo)
		caseRoot := newFreshCaseRoot(t)
		preview, err := previewFreshDistribution(git, source, caseRoot, facts)
		if err != nil {
			t.Fatal(err)
		}
		skillWrite := freshWriteByTarget(t, preview, ".claude/skills/steamai/SKILL.md")
		if err := writeFreshBytes(filepath.Join(caseRoot, filepath.FromSlash(skillWrite.TargetRel)), skillWrite.Bytes); err != nil {
			t.Fatal(err)
		}
		current, err := previewFreshDistribution(git, source, caseRoot, facts)
		if err != nil {
			t.Fatal(err)
		}
		if current.Identity == preview.Identity || current.Writes[0].TargetAction == preview.Writes[0].TargetAction {
			t.Fatal("absent-to-exact-existing target pre-state did not change the preview identity")
		}
		if err := applyFreshDistribution(git, source, caseRoot, preview, true, false); !errors.Is(err, errFreshTargetDrift) {
			t.Fatalf("target pre-state drift returned %v", err)
		}
		assertFreshStateRootMissing(t, caseRoot)
	})

	t.Run("custom skill collision", func(t *testing.T) {
		source := buildFreshCanonicalFixture(t, git, repo)
		caseRoot := newFreshCaseRoot(t)
		writeFreshFile(t, filepath.Join(caseRoot, ".claude", "skills", "steamai", "SKILL.md"), "custom user skill\n")
		if _, err := previewFreshDistribution(git, source, caseRoot, facts); !errors.Is(err, errFreshCollision) {
			t.Fatalf("custom target skill returned %v", err)
		}
		assertFreshStateRootMissing(t, caseRoot)
	})

	t.Run("staged failure", func(t *testing.T) {
		source := buildFreshCanonicalFixture(t, git, repo)
		caseRoot := newFreshCaseRoot(t)
		preview, err := previewFreshDistribution(git, source, caseRoot, facts)
		if err != nil {
			t.Fatal(err)
		}
		if err := applyFreshDistribution(git, source, caseRoot, preview, true, true); !errors.Is(err, errFreshInjectedFailure) {
			t.Fatalf("injected staged apply returned %v", err)
		}
		assertFreshStateRootMissing(t, caseRoot)
		if _, err := previewFreshDistribution(git, source, caseRoot, facts); err != nil {
			t.Fatalf("staged failure made the fresh target unretryable: %v", err)
		}
	})

	t.Run("existing empty state root is partial", func(t *testing.T) {
		source := buildFreshCanonicalFixture(t, git, repo)
		caseRoot := newFreshCaseRoot(t)
		if err := os.Mkdir(filepath.Join(caseRoot, ".steamai-vnext"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := previewFreshDistribution(git, source, caseRoot, facts); !errors.Is(err, errFreshPartialTarget) {
			t.Fatalf("empty state root returned %v", err)
		}
	})

	t.Run("symlink ancestor", func(t *testing.T) {
		source := buildFreshCanonicalFixture(t, git, repo)
		caseRoot := newFreshCaseRoot(t)
		outside := filepath.Join(t.TempDir(), "outside")
		if err := os.MkdirAll(outside, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(caseRoot, ".claude")); err != nil {
			t.Skipf("directory symlink unavailable: %v", err)
		}
		if _, err := previewFreshDistribution(git, source, caseRoot, facts); !errors.Is(err, errFreshCollision) {
			t.Fatalf("symlink ancestor returned %v", err)
		}
		entries, err := os.ReadDir(outside)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatal("fresh preview wrote through a symlink ancestor")
		}
	})
}

func validateFreshSelectedPack(git, source, revision, pack string) error {
	if pack == "" || strings.HasPrefix(pack, "_") || strings.ContainsAny(pack, `/\\`) || filepath.Base(pack) != pack {
		return errFreshCollision
	}
	packRoot := "packs/" + pack
	manifestRel := packRoot + "/manifest.yml"
	manifestEntry, err := freshGitEntryForPath(git, source, revision, manifestRel)
	if err != nil || manifestEntry.Mode != "100644" {
		return errFreshCollision
	}
	manifestBytes, err := freshGitBytes(git, source, "show", revision+":"+manifestRel)
	if err != nil {
		return err
	}
	fields := map[string]string{}
	inEntrypoints := false
	for line := range strings.SplitSeq(string(manifestBytes), "\n") {
		switch {
		case strings.HasPrefix(line, "name: "):
			fields["name"] = strings.TrimSpace(strings.TrimPrefix(line, "name: "))
		case line == "entrypoints:":
			inEntrypoints = true
		case inEntrypoints && strings.HasPrefix(line, "  router: "):
			fields["router"] = strings.TrimSpace(strings.TrimPrefix(line, "  router: "))
		case inEntrypoints && line != "" && !strings.HasPrefix(line, "  "):
			inEntrypoints = false
		}
	}
	if fields["name"] != pack || fields["router"] == "" || filepath.IsAbs(filepath.FromSlash(fields["router"])) || strings.Contains(fields["router"], "\\") {
		return errFreshCollision
	}
	routerRel := filepath.ToSlash(filepath.Clean(filepath.Join(packRoot, filepath.FromSlash(fields["router"]))))
	if !strings.HasPrefix(routerRel, packRoot+"/") {
		return errFreshCollision
	}
	routerEntry, err := freshGitEntryForPath(git, source, revision, routerRel)
	if err != nil || routerEntry.Mode != "100644" {
		return errFreshCollision
	}
	return nil
}

func previewFreshDistribution(git, source, caseRoot string, facts freshCaseFacts) (freshDistributionPreview, error) {
	if err := validateFreshCaseRoot(caseRoot); err != nil {
		return freshDistributionPreview{}, err
	}
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	if info, err := os.Lstat(stateRoot); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return freshDistributionPreview{}, errFreshCollision
		}
		if _, markerErr := os.Lstat(filepath.Join(stateRoot, "CLAUDE.md")); markerErr == nil {
			return freshDistributionPreview{}, errFreshCollision
		}
		return freshDistributionPreview{}, errFreshPartialTarget
	} else if !os.IsNotExist(err) {
		return freshDistributionPreview{}, err
	}

	revisionBytes, err := freshGitBytes(git, source, "rev-parse", "HEAD")
	if err != nil {
		return freshDistributionPreview{}, err
	}
	revision := strings.TrimSpace(string(revisionBytes))
	if err := validateFreshCanonicalSkill(git, source, revision); err != nil {
		return freshDistributionPreview{}, err
	}
	if err := validateFreshSelectedPack(git, source, revision, facts.Pack); err != nil {
		return freshDistributionPreview{}, err
	}
	packRoot := "packs/" + facts.Pack
	packTreeBytes, err := freshGitBytes(git, source, "rev-parse", revision+":"+packRoot)
	if err != nil {
		return freshDistributionPreview{}, err
	}
	commonTreeBytes, err := freshGitBytes(git, source, "rev-parse", revision+":common")
	if err != nil {
		return freshDistributionPreview{}, err
	}
	packTree := strings.TrimSpace(string(packTreeBytes))
	commonTree := strings.TrimSpace(string(commonTreeBytes))

	var writes []freshPlannedWrite
	appendGitSource := func(sourceRel, targetRel string) error {
		entry, err := freshGitEntryForPath(git, source, revision, sourceRel)
		if err != nil {
			return err
		}
		data, err := freshGitBytes(git, source, "show", revision+":"+sourceRel)
		if err != nil {
			return err
		}
		writes = append(writes, newFreshGitWrite(entry, targetRel, data))
		return nil
	}
	if err := appendGitSource(".claude/skills/steamai/SKILL.md", ".claude/skills/steamai/SKILL.md"); err != nil {
		return freshDistributionPreview{}, err
	}
	if err := appendGitSource("vnext/learning-feedback.md", ".steamai-vnext/contracts/learning-feedback.md"); err != nil {
		return freshDistributionPreview{}, err
	}
	templateEntries, err := freshGitTreeEntries(git, source, revision, "vnext/templates")
	if err != nil {
		return freshDistributionPreview{}, err
	}
	for _, entry := range templateEntries {
		data, err := freshGitBytes(git, source, "show", revision+":"+entry.Path)
		if err != nil {
			return freshDistributionPreview{}, err
		}
		writes = append(writes, newFreshGitWrite(entry, ".steamai-vnext/contracts/"+strings.TrimPrefix(entry.Path, "vnext/"), data))
	}
	for _, sourceRoot := range []string{packRoot, "common"} {
		entries, err := freshGitTreeEntries(git, source, revision, sourceRoot)
		if err != nil {
			return freshDistributionPreview{}, err
		}
		for _, entry := range entries {
			data, err := freshGitBytes(git, source, "show", revision+":"+entry.Path)
			if err != nil {
				return freshDistributionPreview{}, err
			}
			writes = append(writes, newFreshGitWrite(entry, ".steamai-vnext/pack-snapshot/"+entry.Path, data))
		}
	}

	snapshotDigest := freshSnapshotPayloadDigest(writes)
	caseTemplate, err := freshGitBytes(git, source, "show", revision+":vnext/templates/case/CLAUDE.md")
	if err != nil {
		return freshDistributionPreview{}, err
	}
	rendered := renderFreshCaseTemplate(string(caseTemplate), facts, revision, packTree, commonTree, snapshotDigest)
	writes = append(writes, newFreshGeneratedWrite("case-template", ".steamai-vnext/CLAUDE.md", []byte(rendered)))
	writes = append(writes, newFreshGeneratedWrite("artifact-index", ".steamai-vnext/artifacts/index.md", []byte("# Artifact index\n\nNo artifacts indexed.\n")))
	snapshot := buildFreshSnapshotMetadata(revision, facts.Pack, packTree, commonTree, snapshotDigest, writes)
	writes = append(writes, newFreshGeneratedWrite("snapshot-metadata", ".steamai-vnext/pack-snapshot/snapshot.yml", []byte(snapshot)))
	sort.Slice(writes, func(i, j int) bool { return writes[i].TargetRel < writes[j].TargetRel })

	for i := range writes {
		if err := annotateFreshTargetState(caseRoot, &writes[i]); err != nil {
			return freshDistributionPreview{}, err
		}
	}
	preview := freshDistributionPreview{
		Revision:       revision,
		PackTree:       packTree,
		CommonTree:     commonTree,
		SnapshotDigest: snapshotDigest,
		Facts:          facts,
		Writes:         writes,
	}
	preview.Identity = freshPreviewIdentity(preview)
	return preview, nil
}

func applyFreshDistribution(git, source, caseRoot string, preview freshDistributionPreview, confirmed, failBeforePublication bool) error {
	if !confirmed {
		return errFreshConfirmationRequired
	}
	currentRevision, err := freshGitBytes(git, source, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(currentRevision)) != preview.Revision {
		return errFreshSourceDrift
	}
	if err := validateFreshCanonicalSkill(git, source, preview.Revision); err != nil {
		return err
	}
	fresh, err := previewFreshDistribution(git, source, caseRoot, preview.Facts)
	if err != nil {
		if errors.Is(err, errFreshCollision) || errors.Is(err, errFreshPartialTarget) {
			return fmt.Errorf("%w: %v", errFreshTargetDrift, err)
		}
		return err
	}
	if fresh.Identity != preview.Identity {
		return errFreshTargetDrift
	}

	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	staging := filepath.Join(caseRoot, ".steamai-vnext.staging-"+preview.Identity[:12])
	if _, err := os.Lstat(staging); err == nil {
		return errFreshCollision
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Mkdir(staging, 0o755); err != nil {
		return err
	}
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			_ = os.RemoveAll(staging)
		}
	}()
	for _, rel := range []string{"evidence", "findings", "reviews", "learnings/candidates"} {
		if err := os.MkdirAll(filepath.Join(staging, filepath.FromSlash(rel)), 0o755); err != nil {
			return err
		}
	}
	for _, write := range preview.Writes {
		if !strings.HasPrefix(write.TargetRel, ".steamai-vnext/") {
			continue
		}
		rel := strings.TrimPrefix(write.TargetRel, ".steamai-vnext/")
		if err := writeFreshBytes(filepath.Join(staging, filepath.FromSlash(rel)), write.Bytes); err != nil {
			return err
		}
	}
	if err := verifyFreshStaging(staging, preview.Writes); err != nil {
		return err
	}
	if failBeforePublication {
		return errFreshInjectedFailure
	}

	skill := freshWriteByTargetValue(preview, ".claude/skills/steamai/SKILL.md")
	if err := publishFreshSkill(caseRoot, preview.Identity, skill); err != nil {
		return err
	}
	if _, err := os.Lstat(stateRoot); err == nil {
		return errFreshTargetDrift
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staging, stateRoot); err != nil {
		return err
	}
	cleanupStaging = false
	return nil
}

func buildFreshCanonicalFixture(t *testing.T, git, repo string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "canonical")
	for _, rel := range []string{
		".claude/skills/steamai/SKILL.md",
		"vnext/learning-feedback.md",
		"vnext/templates",
		"packs/binary-re",
		"common",
	} {
		copyFreshPath(t, filepath.Join(repo, filepath.FromSlash(rel)), filepath.Join(root, filepath.FromSlash(rel)))
	}
	runFreshGit(t, git, root, "init", "--quiet")
	runFreshGit(t, git, root, "config", "user.name", "STeamAI fixture")
	runFreshGit(t, git, root, "config", "user.email", "fixture@example.invalid")
	runFreshGit(t, git, root, "add", "--", ".")
	runFreshGit(t, git, root, "commit", "--quiet", "-m", "fixture canonical")
	return root
}

func copyFreshPath(t *testing.T, source, target string) {
	t.Helper()
	info, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().IsRegular() {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeFreshBytes(target, data); err != nil {
			t.Fatal(err)
		}
		return
	}
	if !info.IsDir() {
		t.Fatalf("fresh fixture source is not regular or directory: %s", source)
	}
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("fixture source is not regular: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeFreshBytes(destination, data)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func renderFreshCaseTemplate(template string, facts freshCaseFacts, revision, packTree, commonTree, snapshotDigest string) string {
	replacements := map[string]string{
		"{{CASE_NAME}}":            facts.Name,
		"{{GOAL}}":                 facts.Goal,
		"{{AUTHORIZED_SCOPE}}":     facts.Authorization,
		"{{PROHIBITED_ACTIONS}}":   facts.Prohibited,
		"{{STOP_CONDITIONS}}":      facts.Stop,
		"{{PACK_NAME}}":            facts.Pack,
		"{{PACK_REVISION}}":        revision,
		"{{PACK_SNAPSHOT_TREE}}":   packTree,
		"{{COMMON_SNAPSHOT_TREE}}": commonTree,
		"{{SNAPSHOT_DIGEST}}":      snapshotDigest,
		"{{TEAM_ROSTER_ROWS}}":     "| none | execution | inactive | none |",
	}
	for placeholder, value := range replacements {
		template = strings.ReplaceAll(template, placeholder, value)
	}
	return template
}

func buildFreshSnapshotMetadata(revision, pack, packTree, commonTree, snapshotDigest string, writes []freshPlannedWrite) string {
	var lines []string
	for _, write := range freshSnapshotPayloadWrites(writes) {
		lines = append(lines, fmt.Sprintf(
			"  - path: %s\n    git-mode: %s\n    git-blob: %s\n    sha256: %s\n    bytes: %d",
			write.SourceRel, write.GitMode, write.SourceBlob, write.SHA256, len(write.Bytes),
		))
	}
	return fmt.Sprintf(
		"pack: %s\nrevision: %s\npack-tree: %s\ncommon-tree: %s\npayload-digest: %s\nfiles:\n%s\n",
		pack, revision, packTree, commonTree, snapshotDigest, strings.Join(lines, "\n"),
	)
}

func assertFreshSnapshotMatchesRevision(t *testing.T, git, source, caseRoot string, preview freshDistributionPreview) {
	t.Helper()
	for _, root := range []string{"packs/" + preview.Facts.Pack, "common"} {
		entries, err := freshGitTreeEntries(git, source, preview.Revision, root)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			want, err := freshGitBytes(git, source, "show", preview.Revision+":"+entry.Path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(caseRoot, ".steamai-vnext", "pack-snapshot", filepath.FromSlash(entry.Path)))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Fatalf("snapshot differs from exact revision for %s", entry.Path)
			}
		}
	}
}

func freshPreviewIdentity(preview freshDistributionPreview) string {
	parts := []string{
		"revision", preview.Revision,
		"pack-tree", preview.PackTree,
		"common-tree", preview.CommonTree,
		"snapshot-digest", preview.SnapshotDigest,
		"case-name", preview.Facts.Name,
		"goal", preview.Facts.Goal,
		"authorization", preview.Facts.Authorization,
		"prohibited", preview.Facts.Prohibited,
		"stop", preview.Facts.Stop,
		"pack", preview.Facts.Pack,
	}
	for _, write := range preview.Writes {
		parts = append(parts,
			"write", write.SourceKind, write.SourceRel, write.SourceBlob, write.GitMode,
			write.TargetRel, write.TargetAction, write.TargetPreState, write.TargetPreSHA256,
			strconv.Itoa(write.TargetPreBytes), write.SHA256, strconv.Itoa(len(write.Bytes)),
		)
	}
	return freshSHA256([]byte(strings.Join(parts, "\x00") + "\n"))
}

func newFreshGitWrite(entry freshGitEntry, targetRel string, data []byte) freshPlannedWrite {
	return freshPlannedWrite{
		SourceKind: "git-blob",
		SourceRel:  entry.Path,
		SourceBlob: entry.Blob,
		GitMode:    entry.Mode,
		TargetRel:  targetRel,
		SHA256:     freshSHA256(data),
		Bytes:      data,
	}
}

func newFreshGeneratedWrite(name, targetRel string, data []byte) freshPlannedWrite {
	return freshPlannedWrite{
		SourceKind: "generated",
		SourceRel:  "generated:" + name,
		SourceBlob: freshSHA256(data),
		GitMode:    "generated",
		TargetRel:  targetRel,
		SHA256:     freshSHA256(data),
		Bytes:      data,
	}
}

func annotateFreshTargetState(caseRoot string, write *freshPlannedWrite) error {
	if err := validateFreshTargetPath(caseRoot, write.TargetRel); err != nil {
		return err
	}
	target := filepath.Join(caseRoot, filepath.FromSlash(write.TargetRel))
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		write.TargetAction = "create"
		write.TargetPreState = "absent"
		write.TargetPreBytes = -1
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", errFreshCollision, write.TargetRel)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	if write.TargetRel != ".claude/skills/steamai/SKILL.md" || string(data) != string(write.Bytes) {
		return fmt.Errorf("%w: %s", errFreshCollision, write.TargetRel)
	}
	write.TargetAction = "unchanged"
	write.TargetPreState = "regular"
	write.TargetPreSHA256 = freshSHA256(data)
	write.TargetPreBytes = len(data)
	return nil
}

func validateFreshCanonicalSkill(git, source, revision string) error {
	entry, err := freshGitEntryForPath(git, source, revision, ".claude/skills/steamai/SKILL.md")
	if err != nil {
		return err
	}
	blob, err := freshGitBytes(git, source, "show", revision+":"+entry.Path)
	if err != nil {
		return err
	}
	path := filepath.Join(source, filepath.FromSlash(entry.Path))
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: canonical skill worktree", errFreshSourceDrift)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(current) != string(blob) {
		return fmt.Errorf("%w: canonical skill worktree", errFreshSourceDrift)
	}
	return nil
}

func validateFreshCaseRoot(caseRoot string) error {
	info, err := os.Lstat(caseRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errFreshCollision
	}
	return nil
}

func validateFreshTargetPath(caseRoot, targetRel string) error {
	if targetRel == "" || filepath.IsAbs(filepath.FromSlash(targetRel)) || strings.Contains(targetRel, "\\") {
		return errFreshCollision
	}
	target := filepath.Clean(filepath.Join(caseRoot, filepath.FromSlash(targetRel)))
	if !pathWithin(target, caseRoot) {
		return errFreshCollision
	}
	rel, err := filepath.Rel(caseRoot, target)
	if err != nil {
		return err
	}
	current := caseRoot
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%w: %s", errFreshCollision, targetRel)
		}
	}
	return nil
}

func publishFreshSkill(caseRoot, identity string, write freshPlannedWrite) error {
	if write.TargetAction == "unchanged" {
		data, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(write.TargetRel)))
		if err != nil || freshSHA256(data) != write.SHA256 {
			return errFreshTargetDrift
		}
		return nil
	}
	if write.TargetAction != "create" {
		return errFreshCollision
	}
	target := filepath.Join(caseRoot, filepath.FromSlash(write.TargetRel))
	if err := validateFreshTargetPath(caseRoot, write.TargetRel); err != nil {
		return err
	}
	if _, err := os.Lstat(target); err == nil {
		return errFreshTargetDrift
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporary := filepath.Join(filepath.Dir(target), ".SKILL.md.steamai-"+identity[:12]+".tmp")
	if _, err := os.Lstat(temporary); err == nil {
		return errFreshCollision
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(temporary, write.Bytes, 0o644); err != nil {
		return err
	}
	defer os.Remove(temporary)
	data, err := os.ReadFile(temporary)
	if err != nil || freshSHA256(data) != write.SHA256 {
		return errFreshTargetDrift
	}
	if err := os.Link(temporary, target); err != nil {
		if _, targetErr := os.Lstat(target); targetErr == nil {
			return errFreshTargetDrift
		}
		return err
	}
	return nil
}

func verifyFreshStaging(staging string, writes []freshPlannedWrite) error {
	expected := map[string]string{}
	for _, write := range writes {
		if rel, ok := strings.CutPrefix(write.TargetRel, ".steamai-vnext/"); ok {
			expected[rel] = write.SHA256
		}
	}
	actual := map[string]string{}
	err := filepath.WalkDir(staging, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errFreshCollision
		}
		rel, err := filepath.Rel(staging, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		actual[filepath.ToSlash(rel)] = freshSHA256(data)
		return nil
	})
	if err != nil {
		return err
	}
	if !equalFreshSnapshots(expected, actual) {
		return errFreshTargetDrift
	}
	return nil
}

func freshSnapshotPayloadWrites(writes []freshPlannedWrite) []freshPlannedWrite {
	var payload []freshPlannedWrite
	for _, write := range writes {
		if isFreshSnapshotPayload(write) {
			payload = append(payload, write)
		}
	}
	sort.Slice(payload, func(i, j int) bool { return payload[i].SourceRel < payload[j].SourceRel })
	return payload
}

func isFreshSnapshotPayload(write freshPlannedWrite) bool {
	return write.SourceKind == "git-blob" && strings.HasPrefix(write.TargetRel, ".steamai-vnext/pack-snapshot/")
}

func freshSnapshotPayloadDigest(writes []freshPlannedWrite) string {
	var records []string
	for _, write := range freshSnapshotPayloadWrites(writes) {
		records = append(records, strings.Join([]string{
			write.SourceRel,
			write.GitMode,
			write.SourceBlob,
			strconv.Itoa(len(write.Bytes)),
			write.SHA256,
		}, "\x00"))
	}
	return "sha256:" + freshSHA256([]byte(strings.Join(records, "\n")+"\n"))
}

func freshMaterializedSnapshotDigest(caseRoot string, writes []freshPlannedWrite) (string, error) {
	materialized := freshSnapshotPayloadWrites(writes)
	for i := range materialized {
		path := filepath.Join(caseRoot, filepath.FromSlash(materialized[i].TargetRel))
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", errFreshTargetDrift
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		materialized[i].Bytes = data
		materialized[i].SHA256 = freshSHA256(data)
	}
	return freshSnapshotPayloadDigest(materialized), nil
}

func freshGitEntryForPath(git, dir, revision, path string) (freshGitEntry, error) {
	entries, err := freshGitTreeEntries(git, dir, revision, path)
	if err != nil {
		return freshGitEntry{}, err
	}
	for _, entry := range entries {
		if entry.Path == path {
			return entry, nil
		}
	}
	return freshGitEntry{}, fmt.Errorf("git entry not found: %s", path)
}

func freshGitTreeEntries(git, dir, revision, root string) ([]freshGitEntry, error) {
	data, err := freshGitBytes(git, dir, "ls-tree", "-r", revision, "--", root)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, nil
	}
	var entries []freshGitEntry
	for line := range strings.SplitSeq(text, "\n") {
		metadata, path, ok := strings.Cut(line, "\t")
		if !ok {
			return nil, fmt.Errorf("malformed git tree entry: %s", line)
		}
		fields := strings.Fields(metadata)
		if len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			return nil, fmt.Errorf("unsupported git tree entry: %s", line)
		}
		entries = append(entries, freshGitEntry{Mode: fields[0], Blob: fields[2], Path: filepath.ToSlash(path)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func freshWriteByTarget(t *testing.T, preview freshDistributionPreview, target string) freshPlannedWrite {
	t.Helper()
	write := freshWriteByTargetValue(preview, target)
	if write.TargetRel == "" {
		t.Fatalf("fresh preview omitted %s", target)
	}
	return write
}

func freshWriteByTargetValue(preview freshDistributionPreview, target string) freshPlannedWrite {
	for _, write := range preview.Writes {
		if write.TargetRel == target {
			return write
		}
	}
	return freshPlannedWrite{}
}

func freshSHA256(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func newFreshCaseRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "case")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeFreshFile(t *testing.T, path, text string) {
	t.Helper()
	if err := writeFreshBytes(path, []byte(text)); err != nil {
		t.Fatal(err)
	}
}

func writeFreshBytes(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func assertFreshStateRootMissing(t *testing.T, caseRoot string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai-vnext")); !os.IsNotExist(err) {
		t.Fatalf("partial fresh distribution published the state root: %v", err)
	}
}

func snapshotFreshTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
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
		result[filepath.ToSlash(rel)] = freshSHA256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func equalFreshSnapshots(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for path, hash := range left {
		if right[path] != hash {
			return false
		}
	}
	return true
}

func freshGitBytes(git, dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func runFreshGitTest(t *testing.T, git, dir string, args ...string) string {
	t.Helper()
	output, err := freshGitBytes(git, dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func requireFreshGit(t *testing.T) string {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required for fresh distribution contract")
	}
	return git
}

func runFreshGit(t *testing.T, git, dir string, args ...string) string {
	return runFreshGitTest(t, git, dir, args...)
}
