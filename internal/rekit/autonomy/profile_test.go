package autonomy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
)

func TestProfilePathUsesCurrentOrLegacyCaseStateRoot(t *testing.T) {
	currentCase := t.TempDir()
	if err := os.MkdirAll(filepath.Join(currentCase, ".steamai"), 0o755); err != nil {
		t.Fatal(err)
	}
	currentRel, err := RelPathForCase(currentCase, "main")
	if err != nil {
		t.Fatal(err)
	}
	if currentRel != ".steamai/lanes/main/autonomy.json" {
		t.Fatalf("current profile rel = %s", currentRel)
	}
	currentPath, err := Path(currentCase, "main")
	if err != nil || filepath.ToSlash(currentPath) != filepath.ToSlash(filepath.Join(currentCase, ".steamai", "lanes", "main", "autonomy.json")) {
		t.Fatalf("current profile path = %s err=%v", currentPath, err)
	}
	if rel, _, err := EnsureManualProfile(currentCase, "main"); err != nil || rel != currentRel {
		t.Fatalf("current manual profile rel = %s err=%v", rel, err)
	}
	summary := ReadSummary(currentCase, "main", nil)
	if summary.ProfilePath != currentRel || !summary.Valid {
		t.Fatalf("current profile summary = %+v", summary)
	}

	legacyCase := t.TempDir()
	if err := os.MkdirAll(filepath.Join(legacyCase, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyRel, err := RelPathForCase(legacyCase, "main")
	if err != nil || legacyRel != RelPath("main") {
		t.Fatalf("legacy profile rel = %s compatibility=%s err=%v", legacyRel, RelPath("main"), err)
	}
}

func TestDefaultProfileIsManualGate(t *testing.T) {
	profile := DefaultProfile("main")
	if err := Validate(profile, "main", nil, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	decision := Evaluate(profile, RelPath("main"), false, "", Request{Lane: "main", Action: "debug"}, time.Now().UTC(), nil, "")
	if decision.Decision != DecisionManualConfirmationRequired || !decision.RequiresConfirmation || decision.Source != "lane-autonomy-profile" {
		t.Fatalf("unexpected manual decision: %+v", decision)
	}
}

func TestEvaluatePreauthorizedProfileRequiresExactTargetBudgetAndOutput(t *testing.T) {
	profile := preauthorizedProfile()
	m := manifestWithActions("debug", "dump")
	caseRoot := t.TempDir()
	if err := Validate(profile, "main", m, caseRoot); err != nil {
		t.Fatal(err)
	}
	base := Request{
		Lane:           "main",
		Action:         "debug",
		Target:         "target-alpha",
		Budget:         Budget{RuntimeSeconds: 30, DiskMB: 64, Requests: 1},
		StopConditions: []string{"timeout", "scope-drift"},
		OutputPaths:    []string{"workspace/main/debug/session-1"},
	}
	decision := Evaluate(profile, RelPath("main"), true, "hash", base, time.Now().UTC(), m, caseRoot)
	if decision.Decision != DecisionPreauthorized || decision.RequiresConfirmation {
		t.Fatalf("unexpected preauthorized decision: %+v", decision)
	}
	incompleteStops := base
	incompleteStops.StopConditions = []string{"timeout"}
	if got := Evaluate(profile, RelPath("main"), true, "hash", incompleteStops, time.Now().UTC(), m, caseRoot); got.Decision != DecisionStopConditionMismatch || !got.RequiresConfirmation {
		t.Fatalf("incomplete manifest stop conditions decision = %+v", got)
	}
	mismatch := base
	mismatch.Target = "target-beta"
	if got := Evaluate(profile, RelPath("main"), true, "hash", mismatch, time.Now().UTC(), m, caseRoot); got.Decision != DecisionOutOfScope || !got.RequiresConfirmation {
		t.Fatalf("target mismatch decision = %+v", got)
	}
	excess := base
	excess.Budget.RuntimeSeconds = 61
	if got := Evaluate(profile, RelPath("main"), true, "hash", excess, time.Now().UTC(), m, caseRoot); got.Decision != DecisionBudgetExceeded || !got.RequiresConfirmation {
		t.Fatalf("budget decision = %+v", got)
	}
	outsideOutput := base
	outsideOutput.OutputPaths = []string{"captures/debug.bin"}
	if got := Evaluate(profile, RelPath("main"), true, "hash", outsideOutput, time.Now().UTC(), m, caseRoot); got.Decision != DecisionOutputPathDenied || !got.RequiresConfirmation {
		t.Fatalf("output decision = %+v", got)
	}
}

func TestValidateProfileFailsClosedOnUnsafePreauthorization(t *testing.T) {
	m := manifestWithActions("debug")
	caseRoot := t.TempDir()
	profile := preauthorizedProfile()
	profile.AllowedActions = []string{"network"}
	if err := Validate(profile, "main", m, caseRoot); err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Fatalf("undeclared action error = %v", err)
	}
	profile = preauthorizedProfile()
	profile.OutputPaths = []string{"../escape"}
	if err := Validate(profile, "main", manifestWithActions("debug", "dump"), caseRoot); err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("unsafe output path error = %v", err)
	}
	profile = preauthorizedProfile()
	profile.RecordRequired = false
	if err := Validate(profile, "main", manifestWithActions("debug", "dump"), caseRoot); err == nil || !strings.Contains(err.Error(), "recordRequired=true") {
		t.Fatalf("recordRequired error = %v", err)
	}
	profile = preauthorizedProfile()
	profile.NotifyMainOn = nil
	if err := Validate(profile, "main", manifestWithActions("debug", "dump"), caseRoot); err == nil || !strings.Contains(err.Error(), "requires notifyMainOn") {
		t.Fatalf("notifyMainOn error = %v", err)
	}
}

func TestEvaluateRechecksRecordRequirement(t *testing.T) {
	profile := preauthorizedProfile()
	profile.RecordRequired = false
	decision := Evaluate(profile, RelPath("main"), true, "hash", Request{
		Lane:           "main",
		Action:         "debug",
		Target:         "target-alpha",
		Budget:         Budget{RuntimeSeconds: 30, DiskMB: 64, Requests: 1},
		StopConditions: []string{"timeout", "scope-drift"},
		OutputPaths:    []string{"workspace/main/debug/session-1"},
	}, time.Now().UTC(), manifestWithActions("debug", "dump"), t.TempDir())
	if decision.Decision != DecisionInvalidProfile || !decision.RequiresConfirmation {
		t.Fatalf("record requirement decision = %+v", decision)
	}
}

func TestEvaluateUsesCanonicalModeForWhitespace(t *testing.T) {
	profile := preauthorizedProfile()
	profile.Mode = " manual-gate "
	decision := Evaluate(profile, RelPath("main"), true, "hash", Request{
		Lane:           "main",
		Action:         "debug",
		Target:         "target-alpha",
		Budget:         Budget{RuntimeSeconds: 30, DiskMB: 64, Requests: 1},
		StopConditions: []string{"timeout", "scope-drift"},
		OutputPaths:    []string{"workspace/main/debug/session-1"},
	}, time.Now().UTC(), manifestWithActions("debug", "dump"), t.TempDir())
	if decision.Decision != DecisionManualConfirmationRequired || !decision.RequiresConfirmation {
		t.Fatalf("whitespace manual mode decision = %+v", decision)
	}
	profile = preauthorizedProfile()
	profile.Mode = " preauthorized "
	profile.GrantedBy = ""
	if err := Validate(profile, "main", manifestWithActions("debug", "dump"), t.TempDir()); err == nil || !strings.Contains(err.Error(), "requires grantedBy") {
		t.Fatalf("whitespace preauthorized mode error = %v", err)
	}
}

func TestReadRejectsTrailingProfileData(t *testing.T) {
	caseRoot := t.TempDir()
	path, err := Path(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{
  "schemaVersion": 1,
  "profileId": "manual-main",
  "lane": "main",
  "mode": "manual-gate",
  "allowedActions": [],
  "deniedActions": [],
  "targetScope": [],
  "budget": {"runtimeSeconds": 0, "diskMB": 0, "requests": 0},
  "stopConditions": [],
  "outputPaths": [],
  "recordRequired": true,
  "notifyMainOn": []
}
{}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Read(caseRoot, "main"); err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("Read error = %v, want trailing data", err)
	}
}

func preauthorizedProfile() Profile {
	return Profile{
		SchemaVersion:  1,
		ProfileID:      "prof-main-debug",
		Lane:           "main",
		Mode:           ModePreauthorized,
		AllowedActions: []string{"debug"},
		DeniedActions:  []string{"dump"},
		TargetScope:    []Target{{Match: "exact", Value: "target-alpha"}},
		Budget:         Budget{RuntimeSeconds: 60, DiskMB: 128, Requests: 2},
		StopConditions: []string{"timeout", "scope-drift"},
		OutputPaths:    []string{"workspace/main/debug"},
		RecordRequired: true,
		NotifyMainOn:   []string{"boundary-hit", "new-risk"},
		GrantedBy:      "user",
		GrantedAt:      "2026-01-01T00:00:00Z",
		ExpiresAt:      "2999-01-01T00:00:00Z",
	}
}

func manifestWithActions(actions ...string) *manifest.Manifest {
	m := &manifest.Manifest{}
	for _, action := range actions {
		m.HeavyToolGates = append(m.HeavyToolGates, manifest.HeavyToolGate{
			ID:             action,
			StopConditions: []string{"timeout", "scope-drift"},
		})
	}
	return m
}
