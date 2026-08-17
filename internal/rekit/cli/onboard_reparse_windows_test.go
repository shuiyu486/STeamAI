//go:build windows

package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/onboarding"
)

func TestStatusAndMutationRejectWindowsOnboardingAncestorReparse(t *testing.T) {
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
			if err := os.Mkdir(linkParent, 0o755); err != nil {
				t.Fatal(err)
			}
			caseRoot := filepath.Join(linkParent, "case")
			identity := missionintent.Identity{SchemaVersion: 1, Target: caseRoot, Pack: "_template", ProjectName: "demo", Goal: "goal", Actor: "actor", Executor: "executor", InitialLane: "feature-analysis"}
			plan, err := onboarding.Preview(testCLIRepoRoot(t), onboarding.Options{Target: identity.Target, Pack: identity.Pack, ProjectName: identity.ProjectName, Goal: identity.Goal, Actor: identity.Actor, Executor: identity.Executor, InitialLane: identity.InitialLane})
			if err != nil {
				t.Fatal(err)
			}
			var intentBytes []byte
			artifacts := map[string][]byte{}
			for _, write := range plan.ExclusivePlan.Writes {
				switch write.Path {
				case missionintent.IntentRel:
					intentBytes = write.Content
				case missionintent.MissionIntentRel, missionintent.CommitRel:
					artifacts[write.Path] = write.Content
				}
			}
			artifacts[missionintent.IntentRel] = intentBytes
			if !committed {
				delete(artifacts, missionintent.MissionIntentRel)
				delete(artifacts, missionintent.CommitRel)
			}
			if err := os.Remove(linkParent); err != nil {
				t.Fatal(err)
			}
			createCLIWindowsDirectoryReparse(t, linkParent, realParent)
			for rel, content := range artifacts {
				path := filepath.Join(caseRoot, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, content, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			for _, command := range []string{"status", "overview"} {
				var out bytes.Buffer
				err := Run([]string{"-Command", command, "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out)
				if err == nil || !strings.Contains(err.Error(), "path must not traverse symlink") || out.Len() != 0 {
					t.Fatalf("%s did not fail closed on onboarding ancestor reparse: err=%v stdout=%q", command, err, out.String())
				}
			}
		})
	}
}

func testCLIRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func createCLIWindowsDirectoryReparse(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("Windows directory reparse creation timed out: %v", ctx.Err())
	}
	if err != nil {
		t.Skipf("Windows directory reparse creation unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}
}
