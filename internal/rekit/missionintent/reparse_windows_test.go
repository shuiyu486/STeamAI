//go:build windows

package missionintent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectRejectsWindowsAncestorReparsePoint(t *testing.T) {
	for _, committed := range []bool{false, true} {
		name := "pending"
		if committed {
			name = "committed"
		}
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			realParent := filepath.Join(base, "real-parent")
			linkParent := filepath.Join(base, "linked-parent")
			if err := os.Mkdir(realParent, 0o755); err != nil {
				t.Fatal(err)
			}
			createWindowsDirectoryReparse(t, linkParent, realParent)
			caseRoot := filepath.Join(linkParent, "case")
			identity := Identity{SchemaVersion: 1, Target: caseRoot, Pack: "_template", ProjectName: "demo", Goal: "goal", Actor: "actor", Executor: "executor", InitialLane: "main"}
			intentBytes, err := MarshalIntent(Intent{SchemaVersion: 1, Kind: "mission-onboarding-intent", PublicationStamp: "20260803-010203004", OnboardingPlanSHA256: strings.Repeat("a", 64), Identity: identity, Recovery: testRecovery(identity)})
			if err != nil {
				t.Fatal(err)
			}
			artifacts := map[string][]byte{IntentRel: intentBytes}
			if committed {
				missionBytes, err := MarshalMissionIntent(identity)
				if err != nil {
					t.Fatal(err)
				}
				commitBytes, err := MarshalCommit(Commit{SchemaVersion: 1, Kind: "mission-onboarding-commit", PublicationStamp: "20260803-010203004", OnboardingPlanSHA256: strings.Repeat("a", 64), MissionIntentSHA256: SHA256(missionBytes), IntentSHA256: SHA256(intentBytes)})
				if err != nil {
					t.Fatal(err)
				}
				artifacts[MissionIntentRel] = missionBytes
				artifacts[CommitRel] = commitBytes
			}
			for rel, content := range artifacts {
				path := filepath.Join(caseRoot, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, content, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			inspection, err := Inspect(caseRoot)
			if err == nil || inspection.State != "corrupt" || len(inspection.ApplyArgs) != 0 {
				t.Fatalf("ancestor reparse accepted: inspection=%+v err=%v", inspection, err)
			}
		})
	}
}

func createWindowsDirectoryReparse(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	}
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("Windows directory reparse creation unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}
}

func TestInspectRejectsCaseRootRebindAfterPin(t *testing.T) {
	root := filepath.Join(t.TempDir(), "case")
	identity := Identity{SchemaVersion: 1, Target: root, Pack: "_template", ProjectName: "demo", Goal: "goal", Actor: "actor", Executor: "executor", InitialLane: "main"}
	intentBytes, err := MarshalIntent(Intent{SchemaVersion: 1, Kind: "mission-onboarding-intent", PublicationStamp: "20260803-010203004", OnboardingPlanSHA256: strings.Repeat("a", 64), Identity: identity, Recovery: testRecovery(identity)})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(IntentRel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, intentBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	oldRoot := root + "-pinned"
	restore := SetInspectPinnedHookForTest(func(caseRoot string) error {
		if err := os.Rename(caseRoot, oldRoot); err != nil {
			return err
		}
		return os.MkdirAll(caseRoot, 0o755)
	})
	inspection, inspectErr := Inspect(root)
	restore()
	if inspectErr == nil || inspection.State != "corrupt" || len(inspection.ApplyArgs) != 0 {
		t.Fatalf("case root rebind accepted: inspection=%+v err=%v", inspection, inspectErr)
	}
}

func TestInspectRejectsRekitSwitchAfterFirstArtifactRead(t *testing.T) {
	root := filepath.Join(t.TempDir(), "case")
	identity := Identity{SchemaVersion: 1, Target: root, Pack: "_template", ProjectName: "demo", Goal: "goal", Actor: "actor", Executor: "executor", InitialLane: "main"}
	missionBytes, err := MarshalMissionIntent(identity)
	if err != nil {
		t.Fatal(err)
	}
	intentBytes, err := MarshalIntent(Intent{SchemaVersion: 1, Kind: "mission-onboarding-intent", PublicationStamp: "20260803-010203004", OnboardingPlanSHA256: strings.Repeat("a", 64), Identity: identity, Recovery: testRecovery(identity)})
	if err != nil {
		t.Fatal(err)
	}
	commitBytes, err := MarshalCommit(Commit{SchemaVersion: 1, Kind: "mission-onboarding-commit", PublicationStamp: "20260803-010203004", OnboardingPlanSHA256: strings.Repeat("a", 64), MissionIntentSHA256: SHA256(missionBytes), IntentSHA256: SHA256(intentBytes)})
	if err != nil {
		t.Fatal(err)
	}
	for rel, content := range map[string][]byte{MissionIntentRel: missionBytes, IntentRel: intentBytes, CommitRel: commitBytes} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	triggered := false
	restore := SetInspectArtifactReadHookForTest(func(rel string) error {
		if triggered || rel != MissionIntentRel {
			return nil
		}
		triggered = true
		rekit := filepath.Join(root, ".rekit")
		pinned := rekit + "-pinned"
		if err := os.Rename(rekit, pinned); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(rekit, "onboarding"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(rekit, "mission-intent.json"), missionBytes, 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(rekit, "onboarding", "intent.json"), intentBytes, 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(rekit, "onboarding", "commit.json"), commitBytes, 0o644)
	})
	inspection, inspectErr := Inspect(root)
	restore()
	if !triggered || inspectErr == nil || inspection.State != "corrupt" || len(inspection.ApplyArgs) != 0 {
		t.Fatalf(".rekit switch after first read accepted: triggered=%t inspection=%+v err=%v", triggered, inspection, inspectErr)
	}
}

func TestRejectReparsePathRejectsWindowsReparsePoint(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	link := filepath.Join(root, "link")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("Windows symlink creation unavailable: %v", err)
	}
	if err := rejectReparsePath(real); err != nil {
		t.Fatalf("regular directory rejected: %v", err)
	}
	if err := rejectReparsePath(link); err == nil || !strings.Contains(err.Error(), "reparse point") {
		t.Fatalf("Windows reparse point error = %v", err)
	}
}
