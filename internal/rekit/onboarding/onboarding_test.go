package onboarding

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func TestPreviewApplyReplayAndIdentityDrift(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "case")
	opt := testOptions(caseRoot)
	plan, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if plan.OnboardingPlanSHA256 == "" || plan.PublicationStamp == "" || !strings.HasPrefix(plan.ApplyCommand, "/rekit onboard ") || !strings.Contains(plan.ApplyCommand, plan.OnboardingPlanSHA256) || len(plan.ApplyArgs) == 0 {
		t.Fatalf("incomplete preview: %+v", plan)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("preview wrote target: %v", err)
	}
	opt.PublicationStamp = plan.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = plan.OnboardingPlanSHA256
	result, err := Apply(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Replay || !result.Inspection.Committed {
		t.Fatalf("unexpected apply: %+v", result)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".rekit", "board.json")); !os.IsNotExist(err) {
		t.Fatalf("onboard created board: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".rekit", "lanes")); !os.IsNotExist(err) {
		t.Fatalf("onboard created lanes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), []byte("{\"schemaVersion\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit", "lanes", "later"), 0o755); err != nil {
		t.Fatal(err)
	}
	replay, err := Apply(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Applied || !replay.Replay {
		t.Fatalf("unexpected replay: %+v", replay)
	}
	drift := opt
	drift.Goal = "different"
	if _, err := Apply(repo, drift); err == nil || !strings.Contains(err.Error(), "identity differs") {
		t.Fatalf("identity drift error = %v", err)
	}
}

func TestApplyRecoversPartialExactPublication(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "partial")
	opt := testOptions(caseRoot)
	plan, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	restore := syncreview.SetExclusiveInitLeafWriteHookForTest(func(stage, path string) error {
		if stage == "before-publish" && path == missionintent.MissionIntentRel {
			return os.ErrClosed
		}
		return nil
	})
	opt.PublicationStamp = plan.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = plan.OnboardingPlanSHA256
	if _, err := Apply(repo, opt); err == nil {
		t.Fatal("hooked apply unexpectedly completed")
	}
	restore()
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil || inspection.State != "pending" {
		t.Fatalf("partial inspection = %+v err=%v", inspection, err)
	}
	result, err := Apply(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Inspection.Committed {
		t.Fatalf("partial recovery not committed: %+v", result)
	}
}

func TestApplyIntentOnlyRecoversAfterLiveSourceDrift(t *testing.T) {
	repo := copyOnboardingRepoFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "intent-only-drift")
	opt := testOptions(caseRoot)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	restore := syncreview.SetExclusiveInitLeafWriteHookForTest(func(stage, path string) error {
		if stage == "before-publish" && path != missionintent.IntentRel {
			return os.ErrClosed
		}
		return nil
	})
	if _, err := Apply(repo, opt); err == nil {
		restore()
		t.Fatal("hooked apply unexpectedly completed")
	}
	restore()

	if err := os.Rename(filepath.Join(repo, "packs"), filepath.Join(repo, "packs-unavailable")); err != nil {
		t.Fatal(err)
	}
	result, err := Apply(repo, opt)
	if err != nil {
		t.Fatalf("durable recovery read live source after intent-only publication: %v", err)
	}
	if !result.Inspection.Committed {
		t.Fatalf("durable recovery not committed: %+v", result)
	}
	for _, write := range preview.ExclusivePlan.Writes {
		published, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(write.Path)))
		if err != nil {
			t.Fatalf("read published %s: %v", write.Path, err)
		}
		if !bytes.Equal(published, write.Content) {
			t.Fatalf("published %s differs from preview snapshot", write.Path)
		}
	}
}

func TestPendingRecoveryRejectsDifferentKitRootWithoutWriting(t *testing.T) {
	repo := copyOnboardingRepoFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "repo-root-bound")
	opt := testOptions(caseRoot)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	restore := syncreview.SetExclusiveInitLeafWriteHookForTest(func(stage, path string) error {
		if stage == "before-publish" && path != missionintent.IntentRel {
			return os.ErrClosed
		}
		return nil
	})
	if _, err := Apply(repo, opt); err == nil {
		restore()
		t.Fatal("hooked apply unexpectedly completed")
	}
	restore()
	before := onboardingSnapshot(t, caseRoot)
	otherRepo := copyOnboardingRepoFixture(t)
	if _, err := Apply(otherRepo, opt); err == nil || !strings.Contains(err.Error(), "different canonical kit root") {
		t.Fatalf("different kit root apply error = %v", err)
	}
	after := onboardingSnapshot(t, caseRoot)
	if len(before) != len(after) {
		t.Fatalf("different kit root changed file count: before=%d after=%d", len(before), len(after))
	}
	for path, content := range before {
		if !bytes.Equal(content, after[path]) {
			t.Fatalf("different kit root changed %s", path)
		}
	}
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil || inspection.State != "pending" {
		t.Fatalf("different kit root changed pending state: inspection=%+v err=%v", inspection, err)
	}
}

func TestPreviewPreservesOpaqueGoalInApplyArgs(t *testing.T) {
	repo := testRepoRoot(t)
	opt := testOptions(filepath.Join(t.TempDir(), "opaque"))
	opt.Goal = " inspect \"quoted\" evidence\nwithout semantic trim "
	plan, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	goal := ""
	for i := 0; i+1 < len(plan.ApplyArgs); i++ {
		if plan.ApplyArgs[i] == "-Goal" {
			goal = plan.ApplyArgs[i+1]
		}
	}
	if goal != opt.Goal || !strings.Contains(plan.ApplyCommand, "applyArgs") {
		t.Fatalf("opaque Goal route drifted: command=%q args=%+v", plan.ApplyCommand, plan.ApplyArgs)
	}
}

func TestPreviewRejectsInvalidInitialLane(t *testing.T) {
	repo := testRepoRoot(t)
	for _, lane := range []string{"bad\nlane", "feature lane", "../feature", "analysis"} {
		opt := testOptions(filepath.Join(t.TempDir(), "invalid"))
		opt.InitialLane = lane
		if _, err := Preview(repo, opt); err == nil || (!strings.Contains(err.Error(), "valid lane selector") && !strings.Contains(err.Error(), "cannot be generated exactly")) {
			t.Fatalf("InitialLane %q error = %v", lane, err)
		}
	}
}

func TestApplyRejectsNonPrefixPublicationWithoutWriting(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "non-prefix")
	opt := testOptions(caseRoot)
	plan, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	commit := plan.ExclusivePlan.Writes[len(plan.ExclusivePlan.Writes)-1]
	commitPath := filepath.Join(caseRoot, filepath.FromSlash(commit.Path))
	if err := os.MkdirAll(filepath.Dir(commitPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(commitPath, commit.Content, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(commitPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.ApplyExclusiveInit(plan.ExclusivePlan); err == nil || !strings.Contains(err.Error(), "non-prefix publication") {
		t.Fatalf("ApplyExclusiveInit error = %v", err)
	}
	after, err := os.ReadFile(commitPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("fail-closed non-prefix replay changed committed leaf")
	}
	for _, write := range plan.ExclusivePlan.Writes[:len(plan.ExclusivePlan.Writes)-1] {
		if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(write.Path))); !os.IsNotExist(err) {
			t.Fatalf("non-prefix replay published predecessor %s: %v", write.Path, err)
		}
	}
}

func TestStrictTamperRejected(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "tamper")
	opt := testOptions(caseRoot)
	plan, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.PublicationStamp = plan.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = plan.OnboardingPlanSHA256
	if _, err := Apply(repo, opt); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(caseRoot, filepath.FromSlash(missionintent.MissionIntentRel))
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := missionintent.Inspect(caseRoot); err == nil {
		t.Fatal("tampered mission intent accepted")
	}
}

func TestFreshApplyRejectsSourceDriftWithoutWriting(t *testing.T) {
	repo := copyOnboardingRepoFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "source-drift")
	opt := testOptions(caseRoot)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(repo, "packs", "_template", "references", "template", "README.md")
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, append(append([]byte{}, original...), []byte("\nsource drift fixture\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	if _, err := Apply(repo, opt); err == nil {
		t.Fatal("fresh apply accepted source drift")
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("source drift failure wrote case root: %v", err)
	}
}

func TestAllPackPreviewsSatisfyRecoveryContract(t *testing.T) {
	repo := testRepoRoot(t)
	packs := []struct{ pack, lane string }{
		{"_template", "feature-analysis"}, {defaults.DefaultPack, "feature-analysis-case"}, {"web-security", "feature-analysis"},
		{"malware-analysis", "sample-analysis-case"}, {"vuln-research", "vuln-analysis-case"}, {"ctf", "challenge-analysis-case"},
		{"unpack-pe", "unpack-analysis-case"}, {"ollvm", "obfuscation-analysis-case"}, {"android-native", "native-analysis-case"},
		{"generic-binary-re", "binary-analysis-case"},
	}
	for _, fixture := range packs {
		t.Run(fixture.pack, func(t *testing.T) {
			opt := testOptions(filepath.Join(t.TempDir(), "case"))
			opt.Pack, opt.InitialLane = fixture.pack, fixture.lane
			if _, err := Preview(repo, opt); err != nil {
				t.Fatalf("honest pack preview rejected by recovery contract: %v", err)
			}
		})
	}
}

func onboardingSnapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("snapshot rejects symlink: %s", path)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("snapshot rejects non-regular file: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = content
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func copyOnboardingRepoFixture(t *testing.T) string {
	t.Helper()
	sourceRoot := testRepoRoot(t)
	targetRoot := filepath.Join(t.TempDir(), "repo")
	for _, rel := range []string{"packs/_template", "rekit/templates/case-shim"} {
		source := filepath.Join(sourceRoot, filepath.FromSlash(rel))
		target := filepath.Join(targetRoot, filepath.FromSlash(rel))
		err := filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("onboarding repo fixture rejects symlink: %s", path)
			}
			if !info.IsDir() && !info.Mode().IsRegular() {
				return fmt.Errorf("onboarding repo fixture rejects non-regular file: %s", path)
			}
			relPath, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			dest := filepath.Join(target, relPath)
			if info.IsDir() {
				return os.MkdirAll(dest, info.Mode().Perm())
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(dest, content, info.Mode().Perm())
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return targetRoot
}

func testOptions(target string) Options {
	return Options{Target: target, Pack: "_template", ProjectName: "demo", Goal: "opaque goal text", Actor: "operator", Executor: "executor-a", InitialLane: "feature-analysis"}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
