package autonomy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
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
			name: "custom autonomous mode",
			mutate: func(profile *Profile) {
				profile.Mode = ModeAutonomous
			},
			wantErr: "requires explicit",
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

func TestManagedAutonomousPresetProvisionEvaluateAndRevoke(t *testing.T) {
	repoRoot, caseRoot, pack := managedAutonomousProvisionFixture(t, true)
	now := time.Now().UTC().Truncate(time.Second)
	opt := managedAutonomousPresetOptions(repoRoot, caseRoot, pack, now)

	built, err := BuildManagedAutonomousPreset(opt)
	if err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		t.Fatal(err)
	}
	if !isManagedAutonomousProfileV1(built, "main", m, caseRoot) {
		t.Fatalf("built profile is not recognized as managed: %+v", built)
	}
	plan, err := PreviewManagedAutonomousPreset(opt)
	if err != nil {
		t.Fatal(err)
	}
	profile := plan.PlannedProfile
	if profile.Mode != ModeAutonomous ||
		!strings.HasPrefix(profile.ProfileID, managedAutonomousProfileIDPrefix+"main-") ||
		!profile.RecordRequired ||
		!slices.Equal(profile.AllowedActions, []string{"debug", "dump"}) ||
		!slices.Equal(profile.TargetScope, []Target{{Match: "exact", Value: "sample-alpha"}, {Match: "exact", Value: "sample-beta"}}) ||
		!slices.Equal(profile.StopConditions, []string{"output-exceeds-bounded-evidence-packet", "scope-drift", "timeout"}) ||
		!slices.Equal(profile.OutputPaths, []string{"workstreams/main/evidence/autonomous"}) {
		t.Fatalf("unexpected managed autonomous profile: %+v", profile)
	}
	applied, err := ApplyProfilePlan(plan, plan.ExpectedPlanSHA256)
	if err != nil || !applied.Applied {
		t.Fatalf("apply managed autonomous profile: result=%+v err=%v", applied, err)
	}

	request := Request{
		Lane:           "main",
		Action:         "debug",
		Target:         "sample-alpha",
		Budget:         Budget{RuntimeSeconds: 30, DiskMB: 8, Requests: 1},
		StopConditions: []string{"timeout", "scope-drift"},
		OutputPaths:    []string{"workstreams/main/evidence/autonomous/run-1"},
	}
	decision := Evaluate(profile, RelPath("main"), true, applied.ProfileSHA256, request, now, m, caseRoot)
	if decision.Decision != DecisionPreauthorized || decision.Mode != ModeAutonomous || decision.RequiresConfirmation {
		t.Fatalf("managed autonomous evaluation = %+v", decision)
	}
	wrongLane := request
	wrongLane.Lane = "other"
	if got := Evaluate(profile, RelPath("main"), true, applied.ProfileSHA256, wrongLane, now, m, caseRoot); got.Decision != DecisionOutOfScope || !got.RequiresConfirmation {
		t.Fatalf("cross-lane evaluation = %+v", got)
	}
	overBudget := request
	overBudget.Budget.RuntimeSeconds = profile.Budget.RuntimeSeconds + 1
	if got := Evaluate(profile, RelPath("main"), true, applied.ProfileSHA256, overBudget, now, m, caseRoot); got.Decision != DecisionBudgetExceeded || !got.RequiresConfirmation {
		t.Fatalf("over-budget autonomous evaluation = %+v", got)
	}
	for name, test := range map[string]struct {
		profile Profile
		request Request
		want    string
	}{
		"action": {profile: profile, request: func() Request {
			changed := request
			changed.Action = "network"
			return changed
		}(), want: DecisionDenied},
		"target": {profile: profile, request: func() Request {
			changed := request
			changed.Target = "sample-other"
			return changed
		}(), want: DecisionOutOfScope},
		"stop": {profile: profile, request: func() Request {
			changed := request
			changed.StopConditions = []string{"unexpected-side-effect"}
			return changed
		}(), want: DecisionStopConditionMismatch},
		"output": {profile: profile, request: func() Request {
			changed := request
			changed.OutputPaths = []string{"workstreams/other/evidence"}
			return changed
		}(), want: DecisionOutputPathDenied},
		"unsafe-output": {profile: profile, request: func() Request {
			changed := request
			changed.OutputPaths = []string{"../escape"}
			return changed
		}(), want: DecisionOutputPathDenied},
		"tampered-profile-id": {profile: func() Profile {
			changed := profile
			changed.ProfileID += "-forged"
			return changed
		}(), request: request, want: DecisionInvalidProfile},
		"tampered-expiry": {profile: func() Profile {
			changed := profile
			changed.ExpiresAt = now.Format(time.RFC3339)
			return changed
		}(), request: request, want: DecisionInvalidProfile},
	} {
		t.Run("evaluate-"+name, func(t *testing.T) {
			got := Evaluate(test.profile, RelPath("main"), true, applied.ProfileSHA256, test.request, now, m, caseRoot)
			if got.Decision != test.want || !got.RequiresConfirmation {
				t.Fatalf("managed autonomous %s evaluation = %+v, want %s", name, got, test.want)
			}
		})
	}

	expiresAt, err := time.Parse(time.RFC3339, profile.ExpiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if got := Evaluate(profile, RelPath("main"), true, applied.ProfileSHA256, request, expiresAt.Add(time.Second), m, caseRoot); got.Decision != DecisionExpired || !got.RequiresConfirmation {
		t.Fatalf("naturally expired managed autonomous evaluation = %+v", got)
	}
	if got := Evaluate(profile, RelPath("main"), true, applied.ProfileSHA256, request, expiresAt.Add(-10*time.Second), m, caseRoot); got.Decision != DecisionExpired || !got.RequiresConfirmation || len(got.Reasons) != 1 || !strings.Contains(got.Reasons[0], "remaining managed autonomous grant duration") {
		t.Fatalf("request crossing managed autonomous expiry = %+v", got)
	}

	revoke, err := PreviewRevoke(ProfileRevokeOptions{RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: pack, Lane: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyProfilePlan(revoke, revoke.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	current, _, exists, err := Read(caseRoot, "main")
	if err != nil || !exists || current.Mode != ModeManualGate {
		t.Fatalf("revoked managed autonomous profile: exists=%t profile=%+v err=%v", exists, current, err)
	}
}

func TestManagedAutonomousIdentityBindsRepoCaseAndPack(t *testing.T) {
	repoRoot, caseRoot, pack := managedAutonomousProvisionFixture(t, true)
	opt := managedAutonomousPresetOptions(repoRoot, caseRoot, pack, time.Now().UTC())
	profile, err := BuildManagedAutonomousPreset(opt)
	if err != nil {
		t.Fatal(err)
	}
	originalID := profile.ProfileID
	m, err := manifest.Load(repoRoot, pack)
	if err != nil {
		t.Fatal(err)
	}
	targets := []string{"sample-alpha", "sample-beta"}

	otherCase := filepath.Join(filepath.Dir(caseRoot), "other-case")
	otherCaseID, err := managedAutonomousProfileID(m.RepoRoot, otherCase, m.Pack, profile, []string{})
	if err != nil {
		t.Fatal(err)
	}
	otherRepoID, err := managedAutonomousProfileID(filepath.Join(filepath.Dir(repoRoot), "other-repo"), caseRoot, m.Pack, profile, []string{})
	if err != nil {
		t.Fatal(err)
	}
	otherPackID, err := managedAutonomousProfileID(m.RepoRoot, caseRoot, "other-pack", profile, []string{})
	if err != nil {
		t.Fatal(err)
	}
	semanticID, err := managedAutonomousProfileID(m.RepoRoot, caseRoot, m.Pack, profile, targets)
	if err != nil {
		t.Fatal(err)
	}
	if originalID == otherCaseID || originalID == otherRepoID || originalID == otherPackID || originalID == semanticID {
		t.Fatalf("managed identity omitted a bound field: original=%s case=%s repo=%s pack=%s semantic=%s", originalID, otherCaseID, otherRepoID, otherPackID, semanticID)
	}
	if len(strings.TrimPrefix(originalID, managedAutonomousProfileIDPrefix+"main-")) != 64 {
		t.Fatalf("managed identity does not carry a full sha256 digest: %s", originalID)
	}
}

func TestPreviewProvisionRejectsAutonomousWithoutManagedOptIn(t *testing.T) {
	repoRoot, caseRoot, pack := managedAutonomousProvisionFixture(t, true)
	profile, err := BuildManagedAutonomousPreset(managedAutonomousPresetOptions(repoRoot, caseRoot, pack, time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	_, err = PreviewProvision(ProfileProvisionOptions{RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: pack, Lane: "main", Profile: profile})
	if err == nil || !strings.Contains(err.Error(), "explicit") || !strings.Contains(err.Error(), "PreviewManagedAutonomousPreset") {
		t.Fatalf("generic autonomous provision error = %v", err)
	}
}

func TestManagedAutonomousPresetFailsClosed(t *testing.T) {
	repoRoot, caseRoot, pack := managedAutonomousProvisionFixture(t, true)
	now := time.Now().UTC().Truncate(time.Second)
	base := managedAutonomousPresetOptions(repoRoot, caseRoot, pack, now)

	tests := []struct {
		name    string
		mutate  func(*ManagedAutonomousPresetOptions)
		wantErr string
	}{
		{name: "no explicit opt-in", mutate: func(opt *ManagedAutonomousPresetOptions) { opt.ExplicitOptIn = false }, wantErr: "explicit opt-in"},
		{name: "wrong preset", mutate: func(opt *ManagedAutonomousPresetOptions) { opt.Preset = "unbounded" }, wantErr: "requires preset"},
		{name: "undeclared action", mutate: func(opt *ManagedAutonomousPresetOptions) { opt.Actions = []string{"patch"} }, wantErr: "not declared"},
		{name: "empty targets", mutate: func(opt *ManagedAutonomousPresetOptions) { opt.Targets = nil }, wantErr: "exact targets"},
		{name: "zero budget", mutate: func(opt *ManagedAutonomousPresetOptions) { opt.Budget.Requests = 0 }, wantErr: "positive"},
		{name: "missing manifest stop", mutate: func(opt *ManagedAutonomousPresetOptions) {
			opt.StopConditions = []string{"timeout", "scope-drift"}
		}, wantErr: "must cover manifest condition"},
		{name: "overlong duration", mutate: func(opt *ManagedAutonomousPresetOptions) {
			opt.ExpiresAt = now.Add(20 * time.Minute).Format(time.RFC3339)
		}, wantErr: "duration exceeds"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opt := base
			opt.Actions = append([]string(nil), base.Actions...)
			opt.Targets = append([]string(nil), base.Targets...)
			test.mutate(&opt)
			if _, err := PreviewManagedAutonomousPreset(opt); err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("PreviewManagedAutonomousPreset error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestManagedAutonomousPresetNetworkRequiresExactExternalTargets(t *testing.T) {
	repoRoot, caseRoot, pack := managedAutonomousProvisionFixture(t, true)
	now := time.Now().UTC().Truncate(time.Second)
	base := managedAutonomousPresetOptions(repoRoot, caseRoot, pack, now)
	base.Actions = []string{"network"}
	base.Targets = []string{"https://fixture.invalid:443"}
	base.StopConditions = []string{"live-target-ambiguity", "unexpected-outbound-request", "scope-drift"}

	if _, err := PreviewManagedAutonomousPreset(base); err == nil || !strings.Contains(err.Error(), "external target scope") {
		t.Fatalf("network preset without explicit external scope error = %v", err)
	}
	base.ExternalTargetScope = []string{"https://other.invalid:443"}
	if _, err := PreviewManagedAutonomousPreset(base); err == nil || !strings.Contains(err.Error(), "every exact network target") {
		t.Fatalf("network preset with mismatched external scope error = %v", err)
	}
	base.ExternalTargetScope = append([]string(nil), base.Targets...)
	plan, err := PreviewManagedAutonomousPreset(base)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plan.PlannedProfile.ProfileID, managedAutonomousProfileIDPrefix+"main-") || plan.PlannedProfile.Mode != ModeAutonomous {
		t.Fatalf("unexpected managed network profile: %+v", plan.PlannedProfile)
	}
}

func TestPreviewRevokeDoesNotMistakeReservedCustomProfileForManaged(t *testing.T) {
	repoRoot, caseRoot, pack := managedAutonomousProvisionFixture(t, true)
	path, err := Path(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	custom, err := BuildManagedAutonomousPreset(managedAutonomousPresetOptions(repoRoot, caseRoot, pack, time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	custom.Mode = ModePreauthorized
	custom.ProfileID = managedAutonomousProfileIDPrefix + "forged"
	if err := writeProfile(path, custom); err != nil {
		t.Fatal(err)
	}
	_, err = PreviewRevoke(ProfileRevokeOptions{RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: pack, Lane: "main"})
	if err == nil || !strings.Contains(err.Error(), "not owned by managed provisioning") {
		t.Fatalf("reserved custom preauthorized revoke error = %v", err)
	}
}

func TestPreviewRevokeRejectsCustomAutonomousProfile(t *testing.T) {
	repoRoot, caseRoot, pack := managedAutonomousProvisionFixture(t, true)
	path, err := Path(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	custom := managedAutonomousPresetOptions(repoRoot, caseRoot, pack, time.Now().UTC())
	profile, err := BuildManagedAutonomousPreset(custom)
	if err != nil {
		t.Fatal(err)
	}
	profile.ProfileID = "custom-autonomous-main"
	if err := writeProfile(path, profile); err != nil {
		t.Fatal(err)
	}
	_, err = PreviewRevoke(ProfileRevokeOptions{RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: pack, Lane: "main"})
	if err == nil || (!strings.Contains(err.Error(), "outside managed preset") && !strings.Contains(err.Error(), "not owned by managed provisioning")) {
		t.Fatalf("custom autonomous revoke error = %v", err)
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

func managedAutonomousPresetOptions(repoRoot, caseRoot, pack string, now time.Time) ManagedAutonomousPresetOptions {
	grantedAt := now.Add(-time.Minute).UTC().Truncate(time.Second)
	return ManagedAutonomousPresetOptions{
		RepoRoot:      repoRoot,
		CaseRoot:      caseRoot,
		Pack:          pack,
		Lane:          "main",
		Preset:        ManagedAutonomousPresetV1,
		ExplicitOptIn: true,
		Actions:       []string{"dump", "debug", "debug"},
		Targets:       []string{"sample-beta", "sample-alpha", "sample-alpha"},
		Budget: Budget{
			RuntimeSeconds: 60,
			DiskMB:         16,
			Requests:       2,
		},
		StopConditions: []string{"timeout", "scope-drift", "output-exceeds-bounded-evidence-packet", "timeout"},
		OutputPaths:    []string{"workstreams/main/evidence/autonomous"},
		GrantedBy:      "user",
		GrantedAt:      grantedAt.Format(time.RFC3339),
		ExpiresAt:      grantedAt.Add(maxProvisionDuration).Format(time.RFC3339),
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

func managedAutonomousProvisionFixture(t *testing.T, currentDefault bool) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	repoRoot := filepath.Join(root, "repo")
	caseRoot := filepath.Join(root, "case")
	pack := "fixture"
	writeProfileProvisionText(t, filepath.Join(repoRoot, "packs", pack, "manifest.yml"), `schemaVersion: 1
name: Fixture
version: 1.0.0
description: Fixture pack
maturity: experimental
heavyToolGates:
  - id: debug
    title: Dynamic debug
    sideEffects: debug,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: timeout,scope-drift
  - id: dump
    title: Bounded dump
    sideEffects: dump,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: scope-drift,output-exceeds-bounded-evidence-packet
  - id: network
    title: Exact network access
    sideEffects: network,filesystem-write
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: live-target-ambiguity,unexpected-outbound-request,scope-drift
`)
	writeProfileProvisionText(t, filepath.Join(caseRoot, ".steamai", "instance.yml"), fmt.Sprintf(
		"templateRoot: %s\ntemplatePack: %s\nprojectName: managed-autonomous-fixture\nprojectRoot: %s\n",
		repoRoot,
		pack,
		caseRoot,
	))
	writeProfileProvisionText(t, filepath.Join(caseRoot, ".steamai", "lanes", "main", "lane.json"), "{\"schemaVersion\":1,\"id\":\"main\",\"status\":\"open\"}\n")
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
maturity: experimental
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
