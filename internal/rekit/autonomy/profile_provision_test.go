package autonomy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPreviewProvisionStableSemanticHash(t *testing.T) {
	repoRoot, caseRoot, pack := profileProvisionFixture(t, true)
	profile := provisionedProfile(time.Now().UTC())
	path, err := Path(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	first, err := PreviewProvision(ProfileProvisionOptions{
		RepoRoot: repoRoot,
		CaseRoot: caseRoot,
		Pack:     pack,
		Lane:     "main",
		Profile:  profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := PreviewProvision(ProfileProvisionOptions{
		RepoRoot: repoRoot,
		CaseRoot: caseRoot,
		Pack:     pack,
		Lane:     "main",
		Profile:  profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ExpectedPlanSHA256 != second.ExpectedPlanSHA256 {
		t.Fatalf("stable plan hash changed: %s != %s", first.ExpectedPlanSHA256, second.ExpectedPlanSHA256)
	}
	if !first.CurrentProfileExists || first.CurrentProfileSHA256 == "" || first.PlannedProfileSHA256 == "" {
		t.Fatalf("plan omits current/planned profile binding: %+v", first)
	}
	if !first.ReviewRequired || !first.RequiresConfirmation || !first.IsMutation || first.Replay {
		t.Fatalf("unexpected preview flags: %+v", first)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("Preview mutated autonomy.json")
	}

	mutations := map[string]func(*ProfileMutationPlan){
		"repo": func(plan *ProfileMutationPlan) { plan.RepoRoot += "-other" },
		"case": func(plan *ProfileMutationPlan) { plan.CaseRoot += "-other" },
		"pack": func(plan *ProfileMutationPlan) { plan.Pack += "-other" },
		"lane": func(plan *ProfileMutationPlan) { plan.Lane = "other" },
		"current-existence": func(plan *ProfileMutationPlan) {
			plan.CurrentProfileExists = !plan.CurrentProfileExists
		},
		"current-hash": func(plan *ProfileMutationPlan) {
			plan.CurrentProfileSHA256 = strings.Repeat("1", 64)
		},
		"planned-profile": func(plan *ProfileMutationPlan) {
			plan.PlannedProfile.TargetScope[0].Value += "-other"
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := first
			changed.PlannedProfile.TargetScope = append([]Target(nil), first.PlannedProfile.TargetScope...)
			mutate(&changed)
			hash, err := profileMutationPlanSHA256(changed)
			if err != nil {
				t.Fatal(err)
			}
			if hash == first.ExpectedPlanSHA256 {
				t.Fatalf("semantic plan hash did not bind %s", name)
			}
		})
	}
}

func TestPreviewProvisionRejectsExpiryManifestActionAndOutputScope(t *testing.T) {
	repoRoot, caseRoot, pack := profileProvisionFixture(t, true)
	now := time.Now().UTC()
	tests := []struct {
		name    string
		mutate  func(*Profile)
		wantErr string
	}{
		{
			name: "expired",
			mutate: func(profile *Profile) {
				profile.GrantedAt = now.Add(-10 * time.Minute).Format(time.RFC3339)
				profile.ExpiresAt = now.Add(-time.Minute).Format(time.RFC3339)
			},
			wantErr: "expired",
		},
		{
			name: "expiry exceeds fifteen minutes",
			mutate: func(profile *Profile) {
				profile.ExpiresAt = now.Add(20 * time.Minute).Format(time.RFC3339)
			},
			wantErr: "duration exceeds",
		},
		{
			name: "undeclared manifest action",
			mutate: func(profile *Profile) {
				profile.AllowedActions = []string{"dump"}
			},
			wantErr: "not declared",
		},
		{
			name: "unsafe output scope",
			mutate: func(profile *Profile) {
				profile.OutputPaths = []string{"../escape"}
			},
			wantErr: "unsafe path",
		},
		{
			name: "autonomous mode",
			mutate: func(profile *Profile) {
				profile.Mode = ModeAutonomous
			},
			wantErr: "permits only mode=preauthorized",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := provisionedProfile(now)
			test.mutate(&profile)
			_, err := PreviewProvision(ProfileProvisionOptions{
				RepoRoot: repoRoot,
				CaseRoot: caseRoot,
				Pack:     pack,
				Lane:     "main",
				Profile:  profile,
			})
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("PreviewProvision error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestPreviewProvisionRequiresExistingDefaultAndRejectsCustomCurrent(t *testing.T) {
	repoRoot, caseRoot, pack := profileProvisionFixture(t, false)
	profile := provisionedProfile(time.Now().UTC())
	opt := ProfileProvisionOptions{RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: pack, Lane: "main", Profile: profile}
	if _, err := PreviewProvision(opt); err == nil || !strings.Contains(err.Error(), "existing exact default") {
		t.Fatalf("missing current profile error = %v", err)
	}

	path, err := Path(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	custom := DefaultProfile("main")
	custom.ProfileID = "custom-manual-main"
	if err := writeProfile(path, custom); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewProvision(opt); err == nil || !strings.Contains(err.Error(), "non-default") {
		t.Fatalf("custom current profile error = %v", err)
	}
}

func TestPreviewRevokeStrictlyValidatesCurrentProfile(t *testing.T) {
	repoRoot, caseRoot, pack := profileProvisionFixture(t, true)
	path, err := Path(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	malformed := DefaultProfile("main")
	malformed.RecordRequired = false
	if err := writeProfile(path, malformed); err != nil {
		t.Fatal(err)
	}
	_, err = PreviewRevoke(ProfileRevokeOptions{RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: pack, Lane: "main"})
	if err == nil || !strings.Contains(err.Error(), "recordRequired=true") {
		t.Fatalf("PreviewRevoke malformed current error = %v", err)
	}
}

func TestApplyProfilePlanRejectsCurrentDrift(t *testing.T) {
	repoRoot, caseRoot, pack := profileProvisionFixture(t, true)
	profile := provisionedProfile(time.Now().UTC())
	plan, err := PreviewProvision(ProfileProvisionOptions{
		RepoRoot: repoRoot,
		CaseRoot: caseRoot,
		Pack:     pack,
		Lane:     "main",
		Profile:  profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	path, err := Path(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	compact, err := json.Marshal(DefaultProfile("main"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, compact, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ApplyProfilePlan(plan, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "changed after Preview") {
		t.Fatalf("ApplyProfilePlan drift error = %v", err)
	}
	current, _, exists, err := Read(caseRoot, "main")
	if err != nil || !exists {
		t.Fatalf("Read current profile = exists %t err %v", exists, err)
	}
	if current.Mode != ModeManualGate {
		t.Fatalf("drifted profile was overwritten: %+v", current)
	}
}

func TestApplyProfilePlanRevalidatesCurrentBytesImmediatelyBeforeReplace(t *testing.T) {
	repoRoot, caseRoot, pack := profileProvisionFixture(t, true)
	profile := provisionedProfile(time.Now().UTC())
	plan, err := PreviewProvision(ProfileProvisionOptions{
		RepoRoot: repoRoot,
		CaseRoot: caseRoot,
		Pack:     pack,
		Lane:     "main",
		Profile:  profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	path, err := Path(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	compact, err := json.Marshal(DefaultProfile("main"))
	if err != nil {
		t.Fatal(err)
	}
	profileMutationBeforeCommitHook = func(ProfileMutationPlan) error {
		return os.WriteFile(path, compact, 0o644)
	}
	t.Cleanup(func() { profileMutationBeforeCommitHook = nil })

	if _, err := ApplyProfilePlan(plan, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "immediately before atomic replacement") {
		t.Fatalf("ApplyProfilePlan final drift error = %v", err)
	}
	current, _, exists, err := Read(caseRoot, "main")
	if err != nil || !exists {
		t.Fatalf("Read current profile = exists %t err %v", exists, err)
	}
	if current.Mode != ModeManualGate {
		t.Fatalf("final drift profile was overwritten: %+v", current)
	}
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".autonomy.json.profile-mutation-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temps) != 0 {
		t.Fatalf("failed Apply left temporary files: %v", temps)
	}
}

func TestProvisionReplayAndRevoke(t *testing.T) {
	repoRoot, caseRoot, pack := profileProvisionFixture(t, true)
	profile := provisionedProfile(time.Now().UTC())
	provision, err := PreviewProvision(ProfileProvisionOptions{
		RepoRoot: repoRoot,
		CaseRoot: caseRoot,
		Pack:     pack,
		Lane:     "main",
		Profile:  profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := ApplyProfilePlan(provision, provision.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.AlreadyApplied || applied.ProfileSHA256 != provision.PlannedProfileSHA256 {
		t.Fatalf("unexpected provision result: %+v", applied)
	}
	originalReplay, err := ApplyProfilePlan(provision, provision.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if originalReplay.Applied || !originalReplay.AlreadyApplied || originalReplay.ProfileSHA256 != provision.PlannedProfileSHA256 {
		t.Fatalf("unexpected original plan replay result: %+v", originalReplay)
	}

	path, err := Path(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	beforeReplay, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := PreviewProvision(ProfileProvisionOptions{
		RepoRoot: repoRoot,
		CaseRoot: caseRoot,
		Pack:     pack,
		Lane:     "main",
		Profile:  profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Replay || replay.IsMutation || replay.RequiresConfirmation {
		t.Fatalf("same desired profile is not a truthful replay: %+v", replay)
	}
	replayed, err := ApplyProfilePlan(replay, replay.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Applied || !replayed.AlreadyApplied || replayed.ProfileSHA256 != replay.CurrentProfileSHA256 {
		t.Fatalf("unexpected replay result: %+v", replayed)
	}
	afterReplay, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(beforeReplay, afterReplay) {
		t.Fatal("replay replaced autonomy.json instead of remaining idempotent")
	}

	revoke, err := PreviewRevoke(ProfileRevokeOptions{
		RepoRoot: repoRoot,
		CaseRoot: caseRoot,
		Pack:     pack,
		Lane:     "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	if revoke.PlannedProfile.Mode != ModeManualGate || revoke.PlannedProfile.ProfileID != DefaultProfile("main").ProfileID {
		t.Fatalf("revoke does not plan DefaultProfile: %+v", revoke.PlannedProfile)
	}
	revoked, err := ApplyProfilePlan(revoke, revoke.ExpectedPlanSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked.Applied || revoked.AlreadyApplied {
		t.Fatalf("unexpected revoke result: %+v", revoked)
	}
	current, _, exists, err := Read(caseRoot, "main")
	if err != nil || !exists {
		t.Fatalf("Read revoked profile = exists %t err %v", exists, err)
	}
	if equal, err := profilesEqual(current, DefaultProfile("main")); err != nil || !equal {
		t.Fatalf("revoke current profile = %+v, err %v", current, err)
	}
}

func TestApplyProfilePlanRejectsMalformedOrInexactExpectedHash(t *testing.T) {
	repoRoot, caseRoot, pack := profileProvisionFixture(t, true)
	plan, err := PreviewProvision(ProfileProvisionOptions{
		RepoRoot: repoRoot,
		CaseRoot: caseRoot,
		Pack:     pack,
		Lane:     "main",
		Profile:  provisionedProfile(time.Now().UTC()),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"", "abc", strings.ToUpper(plan.ExpectedPlanSHA256), strings.Repeat("0", 64)} {
		if _, err := ApplyProfilePlan(plan, expected); err == nil {
			t.Fatalf("ApplyProfilePlan accepted malformed or inexact hash %q", expected)
		}
	}
	tampered := plan
	tampered.PlannedProfile.OutputPaths = []string{"workstreams/main/evidence/other"}
	if _, err := ApplyProfilePlan(tampered, plan.ExpectedPlanSHA256); err == nil || !strings.Contains(err.Error(), "semantic sha256") {
		t.Fatalf("ApplyProfilePlan tampered plan error = %v", err)
	}
	current, _, exists, err := Read(caseRoot, "main")
	if err != nil || !exists {
		t.Fatalf("Read current profile = exists %t err %v", exists, err)
	}
	if current.Mode != ModeManualGate {
		t.Fatalf("malformed expected hash mutated profile: %+v", current)
	}
}

func provisionedProfile(now time.Time) Profile {
	grantedAt := now.Add(-time.Minute).UTC().Truncate(time.Second)
	return Profile{
		SchemaVersion:  1,
		ProfileID:      "dpc04-main-inspect",
		Lane:           "main",
		Mode:           ModePreauthorized,
		AllowedActions: []string{"inspect"},
		DeniedActions:  []string{},
		TargetScope: []Target{{
			Match: "exact",
			Value: "tooling/ida-agent-bridge/requests/" + strings.Repeat("a", 64) + ".json",
		}},
		Budget: Budget{
			RuntimeSeconds: 30,
			DiskMB:         1,
			Requests:       1,
		},
		StopConditions: []string{"scope-drift", "source-drift", "output-exceeds-bounded-evidence-packet"},
		OutputPaths: []string{
			"workstreams/main/evidence/ida-index",
		},
		RecordRequired: true,
		NotifyMainOn: []string{
			"boundary-hit",
			"new-risk",
		},
		GrantedBy: "user",
		GrantedAt: grantedAt.Format(time.RFC3339),
		ExpiresAt: grantedAt.Add(10 * time.Minute).Format(time.RFC3339),
	}
}

func profileProvisionFixture(t *testing.T, currentDefault bool) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	repoRoot := filepath.Join(root, "repo")
	caseRoot := filepath.Join(root, "case")
	pack := "fixture"
	writeProfileProvisionText(t, filepath.Join(repoRoot, "packs", pack, "manifest.yml"), `schemaVersion: 1
name: Fixture
version: 1.0.0
description: Fixture pack
maturity: stable
heavyToolGates:
  - id: inspect
    title: Read-only inspection
    sideEffects: inspect,filesystem-read,bounded-packet-write
    defaultRisk: medium
    requiresConfirmation: true
    stopConditions: scope-drift,source-drift,output-exceeds-bounded-evidence-packet
`)
	writeProfileProvisionText(t, filepath.Join(caseRoot, ".rekit", "instance.yml"), fmt.Sprintf(
		"templateRoot: %s\ntemplatePack: %s\nprojectName: profile-provision-fixture\nprojectRoot: %s\n",
		repoRoot,
		pack,
		caseRoot,
	))
	writeProfileProvisionText(t, filepath.Join(caseRoot, ".rekit", "lanes", "main", "lane.json"), "{\"schemaVersion\":1,\"id\":\"main\",\"status\":\"open\"}\n")
	if currentDefault {
		path, err := Path(caseRoot, "main")
		if err != nil {
			t.Fatal(err)
		}
		if err := writeProfile(path, DefaultProfile("main")); err != nil {
			t.Fatal(err)
		}
	}
	return repoRoot, caseRoot, pack
}

func writeProfileProvisionText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
