package onboarding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func TestRecoverySnapshotUsesExactSelectedIntentArtifact(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.CurrentDir), 0o700); err != nil {
		t.Fatal(err)
	}
	paths, err := missionintent.Paths(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := []missionintent.SnapshotArtifact{{Path: projectstate.CurrentRel("instance.yml"), Kind: "instance-metadata", SHA256: strings.Repeat("a", 64), Size: 1}}
	intentBytes, err := json.Marshal(missionintent.Intent{
		SchemaVersion: 1,
		Kind:          "mission-onboarding-intent",
		Identity:      missionintent.Identity{SchemaVersion: 1, Target: caseRoot},
		Recovery:      missionintent.RecoveryEnvelope{SchemaVersion: 1, RepoRoot: ".", CreatedAt: "2026-08-13T00:00:00Z", Mode: "attached-adoption", AttachedSnapshot: snapshot},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan := syncreview.ExclusiveInitPlan{CaseRoot: caseRoot, Writes: []syncreview.ExclusiveInitWrite{{
		Path: paths.Intent, Kind: "onboarding-intent", TargetPath: filepath.Join(caseRoot, filepath.FromSlash(paths.Intent)), Content: intentBytes,
	}}}
	got, err := recoverySnapshot(plan)
	if err != nil || len(got) != 1 || got[0] != snapshot[0] {
		t.Fatalf("current recovery snapshot = %+v err=%v", got, err)
	}
	plan.Writes[0].Path = missionintent.IntentRel
	if _, err := recoverySnapshot(plan); err == nil || !strings.Contains(err.Error(), "omits its exact intent artifact") {
		t.Fatalf("legacy constant accepted for current plan: %v", err)
	}
}

func TestAttachedAdoptionPreviewApplyAndReplay(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := provisionAttachedCase(t, repo)
	opt := testOptions(caseRoot)

	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.ExclusivePlan.Command != attachedPlanCommand || len(preview.Writes) != 3 {
		t.Fatalf("unexpected attached adoption preview: %+v", preview)
	}
	for index, want := range []string{missionintent.IntentRel, missionintent.MissionIntentRel, missionintent.CommitRel} {
		if preview.Writes[index].Path != want || preview.Writes[index].PublicationPhase != index {
			t.Fatalf("attached write %d = %+v, want path=%s phase=%d", index, preview.Writes[index], want, index)
		}
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(missionintent.IntentRel))); !os.IsNotExist(err) {
		t.Fatalf("attached preview wrote intent: %v", err)
	}

	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	applied, err := Apply(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.Replay || !applied.Inspection.Committed || applied.Inspection.Recovery.Mode != "attached-adoption" {
		t.Fatalf("unexpected attached adoption Apply: %+v", applied)
	}
	if _, err := os.ReadFile(filepath.Join(caseRoot, "case-local.txt")); err != nil {
		t.Fatalf("attached adoption changed ordinary case content: %v", err)
	}

	replay, err := Apply(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Applied || !replay.Replay {
		t.Fatalf("unexpected attached adoption replay: %+v", replay)
	}
}

func TestAttachedAdoptionRejectsMissionControlState(t *testing.T) {
	for _, rel := range []string{".rekit/board.json", ".rekit/policy.yml", ".rekit/verification-role.json", ".rekit/backups/one", ".rekit/reviewer-adoptions/one", ".rekit/reopen-operations/one"} {
		t.Run(filepath.Base(rel), func(t *testing.T) {
			repo := testRepoRoot(t)
			caseRoot := provisionAttachedCase(t, repo)
			path := filepath.Join(caseRoot, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Preview(repo, testOptions(caseRoot)); err == nil || !strings.Contains(err.Error(), "refuses existing Mission Control state") {
				t.Fatalf("attached active-state preview error = %v", err)
			}
		})
	}
}

func TestAttachedAdoptionRejectsSnapshotDriftWithoutWriting(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := provisionAttachedCase(t, repo)
	opt := testOptions(caseRoot)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(caseRoot, ".rekit", "state.json")
	state, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(state, ' '), 0o600); err != nil {
		t.Fatal(err)
	}
	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	if _, err := Apply(repo, opt); err == nil || !strings.Contains(err.Error(), "snapshot changed") {
		t.Fatalf("attached snapshot drift Apply error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(missionintent.IntentRel))); !os.IsNotExist(err) {
		t.Fatalf("snapshot drift wrote intent: %v", err)
	}
}

func TestAttachedAdoptionRejectsDoctorInvalidCaseWithoutWriting(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := provisionAttachedCase(t, repo)
	if err := os.Remove(filepath.Join(caseRoot, "CLAUDE.local.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Preview(repo, testOptions(caseRoot)); err == nil || !strings.Contains(err.Error(), "doctor-ready case files") {
		t.Fatalf("doctor-invalid attached preview error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(missionintent.IntentRel))); !os.IsNotExist(err) {
		t.Fatalf("doctor-invalid attached preview wrote intent: %v", err)
	}
}

func TestAttachedAdoptionPendingRecoveryRejectsMissionControlState(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := provisionAttachedCase(t, repo)
	opt := testOptions(caseRoot)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAttachedTestArtifact(caseRoot, preview.ExclusivePlan.Writes[0]); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	if _, err := Apply(repo, opt); err == nil || !strings.Contains(err.Error(), "refuses existing Mission Control state") {
		t.Fatalf("pending attached takeover error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(missionintent.MissionIntentRel))); !os.IsNotExist(err) {
		t.Fatalf("pending attached takeover wrote mission intent: %v", err)
	}
}

func TestAttachedAdoptionRevalidatesSnapshotBeforeCommit(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := provisionAttachedCase(t, repo)
	opt := testOptions(caseRoot)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	attachedAdoptionBeforeCommitHook = func() error {
		path := filepath.Join(caseRoot, ".rekit", "state.json")
		state, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(state, ' '), 0o600)
	}
	t.Cleanup(func() { attachedAdoptionBeforeCommitHook = nil })
	if _, err := Apply(repo, opt); err == nil || !strings.Contains(err.Error(), "snapshot changed") {
		t.Fatalf("attached commit-time snapshot drift error = %v", err)
	}
	attachedAdoptionBeforeCommitHook = nil
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil || inspection.State != "pending" || inspection.Committed {
		t.Fatalf("snapshot drift should stop before commit: inspection=%+v err=%v", inspection, err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(missionintent.CommitRel))); !os.IsNotExist(err) {
		t.Fatalf("snapshot drift wrote commit: %v", err)
	}
}

func TestAttachedAdoptionRevalidatesMissionControlBeforeCommit(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := provisionAttachedCase(t, repo)
	opt := testOptions(caseRoot)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	attachedAdoptionBeforeCommitHook = func() error {
		return os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), []byte("{}\n"), 0o600)
	}
	t.Cleanup(func() { attachedAdoptionBeforeCommitHook = nil })
	if _, err := Apply(repo, opt); err == nil || !strings.Contains(err.Error(), "refuses existing Mission Control state") {
		t.Fatalf("attached commit-time Mission Control insertion error = %v", err)
	}
	attachedAdoptionBeforeCommitHook = nil
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(missionintent.CommitRel))); !os.IsNotExist(err) {
		t.Fatalf("Mission Control insertion wrote commit: %v", err)
	}
}

func TestAttachedAdoptionRecoversIntentOnlyPublication(t *testing.T) {
	repo := testRepoRoot(t)
	caseRoot := provisionAttachedCase(t, repo)
	opt := testOptions(caseRoot)
	preview, err := Preview(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	intent := preview.ExclusivePlan.Writes[0]
	if err := writeAttachedTestArtifact(caseRoot, intent); err != nil {
		t.Fatal(err)
	}
	inspection, err := missionintent.Inspect(caseRoot)
	if err != nil || inspection.State != "pending" || inspection.Recovery.Mode != "attached-adoption" {
		t.Fatalf("intent-only attached inspection = %+v err=%v", inspection, err)
	}
	opt.ProjectID = preview.ProjectID
	opt.PublicationStamp = preview.PublicationStamp
	opt.ExpectedOnboardingPlanSHA256 = preview.OnboardingPlanSHA256
	result, err := Apply(repo, opt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Inspection.Committed {
		t.Fatalf("attached pending recovery not committed: %+v", result)
	}
}

func provisionAttachedCase(t *testing.T, repo string) string {
	t.Helper()
	caseRoot := filepath.Join(t.TempDir(), "attached")
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := casebind.WriteInstance(caseRoot, repo, "_template", "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := casebind.WriteCaseShim(caseRoot, repo); err != nil {
		t.Fatal(err)
	}
	preview, err := syncreview.InitPreview(repo, caseRoot, "_template", syncreview.ApplyOptions{ProjectName: "demo", CreateLocalFiles: true, Command: "init"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(repo, caseRoot, "_template", syncreview.ApplyOptions{ProjectName: "demo", CreateLocalFiles: true, Command: "init", ExpectedPlanSHA256: preview.ExpectedPlanSHA256}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "case-local.txt"), []byte("preserve me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func writeAttachedTestArtifact(caseRoot string, write syncreview.ExclusiveInitWrite) error {
	path := filepath.Join(caseRoot, filepath.FromSlash(write.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, write.Content, 0o600)
}
