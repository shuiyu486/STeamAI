package onboarding

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
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
	opt.ProjectID = plan.ProjectID
	opt.PublicationStamp = plan.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = plan.OnboardingPlanSHA256
	result, err := Apply(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.Replay || !result.Inspection.Committed {
		t.Fatalf("unexpected apply: %+v", result)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai", "board.json")); !os.IsNotExist(err) {
		t.Fatalf("onboard created board: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai", "lanes")); !os.IsNotExist(err) {
		t.Fatalf("onboard created lanes: %v", err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".steamai", "board.json"), []byte("{\"schemaVersion\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(caseRoot, ".steamai", "lanes", "later"), 0o755); err != nil {
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

func TestCurrentV2ApplyCopyAndMoveRemainReplayable(t *testing.T) {
	repo := testRepoRoot(t)
	base := t.TempDir()
	original := filepath.Join(base, "original")
	opt := testOptions(original)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Identity.SchemaVersion != 2 || preview.Identity.Target != "." || len(preview.ProjectID) != 16 || preview.CaseRoot != original {
		t.Fatalf("current v2 preview identity = %+v caseRoot=%s", preview.Identity, preview.CaseRoot)
	}
	bindingPath := projectstate.CurrentRel("project-binding.json")
	bindingFound := false
	for _, write := range preview.Writes {
		if write.Path == bindingPath && write.Kind == "project-binding" && write.PublicationPhase == 2 {
			bindingFound = true
		}
	}
	if !bindingFound {
		t.Fatalf("current v2 preview omitted project binding: %+v", preview.Writes)
	}
	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	applied, err := Apply(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Inspection.Committed || applied.Inspection.ProjectBinding == nil || applied.Inspection.Identity.ProjectID != preview.ProjectID {
		t.Fatalf("current v2 Apply = %+v", applied)
	}

	copied := filepath.Join(base, "copied")
	if err := os.CopyFS(copied, os.DirFS(original)); err != nil {
		t.Fatal(err)
	}
	copyOpt := opt
	copyOpt.Target = copied
	copyReplay, err := Apply(repo, copyOpt)
	if err != nil {
		t.Fatal(err)
	}
	if copyReplay.Applied || !copyReplay.Replay || copyReplay.CaseRoot != copied || copyReplay.Inspection.Identity.ProjectID != preview.ProjectID {
		t.Fatalf("copied v2 replay = %+v", copyReplay)
	}

	moved := filepath.Join(base, "moved")
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	moveOpt := opt
	moveOpt.Target = moved
	moveReplay, err := Apply(repo, moveOpt)
	if err != nil {
		t.Fatal(err)
	}
	if moveReplay.Applied || !moveReplay.Replay || moveReplay.CaseRoot != moved || moveReplay.Inspection.Identity.ProjectID != preview.ProjectID {
		t.Fatalf("moved v2 replay = %+v", moveReplay)
	}
}

func TestCurrentV2IntentOnlyRecoveryMaterializesCopiedAndMovedRoots(t *testing.T) {
	repo := testRepoRoot(t)
	base := t.TempDir()
	original := filepath.Join(base, "pending-original")
	opt := testOptions(original)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	paths, err := missionintent.Paths(original)
	if err != nil {
		t.Fatal(err)
	}
	restore := syncreview.SetExclusiveInitLeafWriteHookForTest(func(stage, path string) error {
		if stage == "before-publish" && path != paths.Intent {
			return os.ErrClosed
		}
		return nil
	})
	if _, err := Apply(repo, opt); err == nil {
		restore()
		t.Fatal("hooked v2 Apply unexpectedly completed")
	}
	restore()
	inspection, err := missionintent.Inspect(original)
	if err != nil || inspection.State != "pending" || inspection.Identity.ProjectID != preview.ProjectID {
		t.Fatalf("intent-only v2 inspection = %+v err=%v", inspection, err)
	}

	copied := filepath.Join(base, "pending-copied")
	if err := os.CopyFS(copied, os.DirFS(original)); err != nil {
		t.Fatal(err)
	}
	copyOpt := opt
	copyOpt.Target = copied
	copyResult, err := Apply(repo, copyOpt)
	if err != nil {
		t.Fatal(err)
	}
	assertRecoveredTemplateRoot(t, copied, original, copyResult)

	moved := filepath.Join(base, "pending-moved")
	if err := os.Rename(original, moved); err != nil {
		t.Fatal(err)
	}
	moveOpt := opt
	moveOpt.Target = moved
	moveResult, err := Apply(repo, moveOpt)
	if err != nil {
		t.Fatal(err)
	}
	assertRecoveredTemplateRoot(t, moved, original, moveResult)
}

func assertRecoveredTemplateRoot(t *testing.T, caseRoot, oldRoot string, result Result) {
	t.Helper()
	if !result.Inspection.Committed || result.Inspection.Identity.ProjectID == "" {
		t.Fatalf("recovered v2 result = %+v", result)
	}
	templatePath := filepath.Join(caseRoot, "references", "template", "task-handoff.md")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), caseRoot) || strings.Contains(string(content), oldRoot) || strings.Contains(string(content), "<PROJECT_ROOT>") {
		t.Fatalf("recovered template did not bind current physical root %s:\n%s", caseRoot, content)
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
	paths, err := missionintent.Paths(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	restore := syncreview.SetExclusiveInitLeafWriteHookForTest(func(stage, path string) error {
		if stage == "before-publish" && path == paths.MissionIntent {
			return os.ErrClosed
		}
		return nil
	})
	opt.ProjectID = plan.ProjectID
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
	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	restore := syncreview.SetExclusiveInitLeafWriteHookForTest(func(stage, path string) error {
		if stage == "before-publish" && path != ".steamai/onboarding/intent.json" {
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
		if int64(len(published)) != write.Size || !strings.EqualFold(missionintent.SHA256(published), write.SHA256) {
			t.Fatalf("published %s differs from preview binding", write.Path)
		}
	}
}

func TestPendingBundleRecoveryDoesNotRequireOriginalKitRoot(t *testing.T) {
	repo := copyOnboardingRepoFixture(t)
	caseRoot := filepath.Join(t.TempDir(), "repo-root-bound")
	opt := testOptions(caseRoot)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	restore := syncreview.SetExclusiveInitLeafWriteHookForTest(func(stage, path string) error {
		if stage == "before-publish" && path != ".steamai/onboarding/intent.json" {
			return os.ErrClosed
		}
		return nil
	})
	if _, err := Apply(repo, opt); err == nil {
		restore()
		t.Fatal("hooked apply unexpectedly completed")
	}
	restore()
	otherRepo := copyOnboardingRepoFixture(t)
	if err := os.Rename(filepath.Join(repo, "packs"), filepath.Join(repo, "packs-unavailable")); err != nil {
		t.Fatal(err)
	}
	result, err := Apply(otherRepo, opt)
	if err != nil {
		t.Fatalf("bundle recovery depended on original kit root: %v", err)
	}
	if !result.Inspection.Committed {
		t.Fatalf("bundle recovery from a different caller root did not commit: %+v", result)
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

func TestDefaultPackInitialLaneUsesStartRoundTrip(t *testing.T) {
	repo := testRepoRoot(t)
	opt := testOptions(filepath.Join(t.TempDir(), "vmp-round-trip"))
	opt.Pack = defaults.DefaultPack
	opt.InitialLane = "binary-analysis-live-check"
	plan, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if plan.InitialLane != opt.InitialLane {
		t.Fatalf("InitialLane = %q, want %q", plan.InitialLane, opt.InitialLane)
	}

	opt.InitialLane = "feature-live-check"
	if _, err := Preview(repo, opt); err == nil || !strings.Contains(err.Error(), `default start lane type "binary-analysis"`) {
		t.Fatalf("non-default feature lane was accepted by binary-re: %v", err)
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
	opt.ProjectID = plan.ProjectID
	opt.PublicationStamp = plan.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = plan.OnboardingPlanSHA256
	if _, err := Apply(repo, opt); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(caseRoot, ".steamai", "mission-intent.json")
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
	opt.ProjectID = preview.ProjectID
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
		{"_template", "feature-analysis"}, {defaults.DefaultPack, "binary-analysis-case"}, {"web-security", "feature-analysis"},
		{"malware-analysis", "sample-analysis-case"}, {"vuln-research", "vuln-analysis-case"}, {"ctf", "challenge-analysis-case"},
		{"unpack-pe", "unpack-analysis-case"}, {"ollvm", "obfuscation-analysis-case"}, {"android-native", "native-analysis-case"},
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

func copyOnboardingRepoFixture(t *testing.T) string {
	t.Helper()
	sourceRoot := testRepoRoot(t)
	targetRoot := filepath.Join(t.TempDir(), "repo")
	for _, rel := range []string{"packs/_template", "common", "rekit/templates/case-shim", "rekit/templates/steamai-project", "rekit/schemas", "rekit/tests/catalog.json"} {
		source := filepath.Join(sourceRoot, filepath.FromSlash(rel))
		target := filepath.Join(targetRoot, filepath.FromSlash(rel))
		if info, err := os.Stat(source); err != nil {
			t.Fatal(err)
		} else if info.Mode().IsRegular() {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, content, info.Mode().Perm()); err != nil {
				t.Fatal(err)
			}
			continue
		}
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
