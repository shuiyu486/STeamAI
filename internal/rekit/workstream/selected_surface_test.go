package workstream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestSelectedMissionCommanderSurfaceUsesSelectedStateRoot(t *testing.T) {
	for _, test := range []struct {
		stateDir   string
		entrypoint string
	}{
		{stateDir: projectstate.CurrentDir, entrypoint: "/steamai"},
		{stateDir: projectstate.LegacyDir, entrypoint: "/rekit"},
	} {
		t.Run(test.stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, test.stateDir), 0o700); err != nil {
				t.Fatal(err)
			}

			boardPath, entrypoint, err := selectedMissionCommanderSurface(caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			if boardPath != test.stateDir+"/board.json" || entrypoint != test.entrypoint {
				t.Fatalf("selected surface board=%q entrypoint=%q", boardPath, entrypoint)
			}

			onboarding := MissingBoardOnboardingAction(caseRoot)
			if onboarding.Blocked || !strings.HasPrefix(onboarding.Command, test.entrypoint+" overview ") {
				t.Fatalf("onboarding action = %+v", onboarding)
			}
			bootstrap := StartBootstrapAction(caseRoot)
			if bootstrap.Blocked || !strings.HasPrefix(bootstrap.Command, test.entrypoint+" start ") {
				t.Fatalf("bootstrap action = %+v", bootstrap)
			}
		})
	}
}

func TestMissionCommanderSurfaceFailsClosedForDualRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if err := os.Mkdir(filepath.Join(caseRoot, stateDir), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	if _, _, err := selectedMissionCommanderSurface(caseRoot); err == nil || !strings.Contains(err.Error(), "both .steamai and .rekit") {
		t.Fatalf("selected surface dual-root error = %v", err)
	}
	assertBlocked := func(name string, blocked bool, command string, reasons []string) {
		t.Helper()
		if !blocked || command != "" || len(reasons) != 1 || !strings.Contains(reasons[0], "both .steamai and .rekit") {
			t.Fatalf("%s dual-root action: blocked=%t command=%q reasons=%v", name, blocked, command, reasons)
		}
	}
	onboarding := MissingBoardOnboardingAction(caseRoot)
	assertBlocked("onboarding", onboarding.Blocked, onboarding.Command, onboarding.Reasons)
	bootstrap := StartBootstrapAction(caseRoot)
	assertBlocked("bootstrap", bootstrap.Blocked, bootstrap.Command, bootstrap.Reasons)
}
