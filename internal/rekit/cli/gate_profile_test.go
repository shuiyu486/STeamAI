package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func TestParseGateProfileFlags(t *testing.T) {
	opt, err := Parse([]string{
		"-Command", "gate", "-ProvisionProfile",
		"-ProfilePreset", autonomy.ManagedAutonomousPresetV1,
		"-ProfileExplicitOptIn",
		"-ProfileExternalTargetScope", "target-a,target-b",
		"-ProfileId", "dpc04-main-inspect",
		"-ProfileGrantedBy", "user",
		"-ProfileGrantedAt", "2026-08-11T01:00:00Z",
		"-ProfileExpiresAt", "2026-08-11T01:10:00Z",
		"-ExpectedProfilePlanSha256", strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !opt.Gate.ProvisionProfile || opt.Gate.RevokeProfile || opt.Gate.ProfilePreset != autonomy.ManagedAutonomousPresetV1 || !opt.Gate.ProfileExplicitOptIn || opt.Gate.ProfileExternalTargetScope != "target-a,target-b" || opt.Gate.ProfileID != "dpc04-main-inspect" || opt.Gate.ProfileGrantedBy != "user" || opt.Gate.ExpectedProfilePlanSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("profile flags not parsed: %+v", opt.Gate)
	}
}

func TestRunGateManagedAutonomousPresetRequiresExplicitOptIn(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, defaults.DefaultPack)
	var out bytes.Buffer
	err := Run([]string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack,
		"-ProvisionProfile", "-ProfilePreset", autonomy.ManagedAutonomousPresetV1, "-Lane", "main",
	}, &out)
	if err == nil || !strings.Contains(err.Error(), "requires explicit -ProfileExplicitOptIn") {
		t.Fatalf("managed autonomous preset accepted without explicit opt-in: err=%v", err)
	}
}

func TestRunGateManagedAutonomousPresetProvisionAndRevoke(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, defaults.DefaultPack)
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"main","status":"open","workspace":"workstreams/main/evidence"}]}`+"\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","status":"open"}`+"\n")
	defaultProfile, err := json.MarshalIndent(autonomy.DefaultProfile("main"), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(caseRoot, filepath.FromSlash(autonomy.RelPath("main")))
	initialProfileBytes := append(defaultProfile, '\n')
	writeCaseFile(t, caseRoot, autonomy.RelPath("main"), string(initialProfileBytes))
	now := time.Now().UTC().Truncate(time.Second)
	base := []string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json",
		"-ProvisionProfile", "-ProfilePreset", autonomy.ManagedAutonomousPresetV1, "-ProfileExplicitOptIn",
		"-Lane", "main", "-Action", "debug,dump", "-TargetRef", "sample-alpha,sample-beta",
		"-RuntimeSeconds", "30", "-DiskMB", "8", "-Requests", "1",
		"-StopConditions", "timeout,unexpected-side-effect,scope-drift,output-exceeds-bounded-evidence-packet,sensitive-artifact-detected",
		"-OutputPaths", "workstreams/main/evidence/autonomous",
		"-ProfileGrantedBy", "user", "-ProfileGrantedAt", now.Add(-time.Minute).Format(time.RFC3339),
		"-ProfileExpiresAt", now.Add(9 * time.Minute).Format(time.RFC3339),
	}
	var out bytes.Buffer
	if err := Run(base, &out); err != nil {
		t.Fatal(err)
	}
	var preview autonomy.ProfileMutationPlan
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("managed autonomous preview is not JSON: %v\n%s", err, out.String())
	}
	if preview.ExpectedPlanSHA256 == "" || preview.PlannedProfile.Mode != autonomy.ModeAutonomous || !preview.IsMutation || !preview.RequiresConfirmation {
		t.Fatalf("unexpected managed autonomous preview: %+v", preview)
	}
	if current, err := os.ReadFile(profilePath); err != nil || !bytes.Equal(current, initialProfileBytes) {
		t.Fatalf("managed autonomous preview changed profile: err=%v\n%s", err, current)
	}

	out.Reset()
	wrongApply := append(append([]string{}, base...), "-Apply", "-ExpectedProfilePlanSha256", strings.Repeat("0", 64))
	if err := Run(wrongApply, &out); err == nil {
		t.Fatalf("managed autonomous apply accepted wrong plan hash: %s", out.String())
	}
	if current, err := os.ReadFile(profilePath); err != nil || !bytes.Equal(current, initialProfileBytes) {
		t.Fatalf("wrong managed autonomous plan hash changed profile: err=%v\n%s", err, current)
	}

	out.Reset()
	apply := append(append([]string{}, base...), "-Apply", "-ExpectedProfilePlanSha256", preview.ExpectedPlanSHA256)
	if err := Run(apply, &out); err != nil {
		t.Fatal(err)
	}
	var applied autonomy.ProfileMutationResult
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("managed autonomous apply is not JSON: %v\n%s", err, out.String())
	}
	profile, _, exists, err := autonomy.Read(caseRoot, "main")
	if err != nil || !exists || !applied.Applied || profile.Mode != autonomy.ModeAutonomous || !reflect.DeepEqual(profile.AllowedActions, []string{"debug", "dump"}) {
		t.Fatalf("unexpected managed autonomous apply: result=%+v profile=%+v exists=%t err=%v", applied, profile, exists, err)
	}

	out.Reset()
	revoke := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json", "-RevokeProfile", "-Lane", "main"}
	if err := Run(revoke, &out); err != nil {
		t.Fatal(err)
	}
	var revokePlan autonomy.ProfileMutationPlan
	if err := json.Unmarshal(out.Bytes(), &revokePlan); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(append(revoke, "-Apply", "-ExpectedProfilePlanSha256", revokePlan.ExpectedPlanSHA256), &out); err != nil {
		t.Fatal(err)
	}
	profile, _, exists, err = autonomy.Read(caseRoot, "main")
	if err != nil || !exists || profile.Mode != autonomy.ModeManualGate {
		t.Fatalf("unexpected managed autonomous revoke: profile=%+v exists=%t err=%v", profile, exists, err)
	}
}

func TestRunGateManagedAutonomousPresetUsesCurrentStateRoot(t *testing.T) {
	caseRoot := attachedCaseWithStateDirAndPack(t, projectstate.CurrentDir, defaults.DefaultPack)
	writeCaseFile(t, caseRoot, ".steamai/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","status":"open"}`+"\n")
	defaultProfile, err := json.MarshalIndent(autonomy.DefaultProfile("main"), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	profileRel, err := autonomy.RelPathForCase(caseRoot, "main")
	if err != nil {
		t.Fatal(err)
	}
	if profileRel != ".steamai/lanes/main/autonomy.json" {
		t.Fatalf("current profile path = %q", profileRel)
	}
	initialProfileBytes := append(defaultProfile, '\n')
	writeCaseFile(t, caseRoot, profileRel, string(initialProfileBytes))
	profilePath := filepath.Join(caseRoot, filepath.FromSlash(profileRel))
	now := time.Now().UTC().Truncate(time.Second)
	base := []string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json",
		"-ProvisionProfile", "-ProfilePreset", autonomy.ManagedAutonomousPresetV1, "-ProfileExplicitOptIn",
		"-Lane", "main", "-Action", "debug", "-TargetRef", "sample-alpha",
		"-RuntimeSeconds", "30", "-DiskMB", "8", "-Requests", "1",
		"-StopConditions", "timeout,unexpected-side-effect,scope-drift", "-OutputPaths", "workstreams/main/evidence/autonomous",
		"-ProfileGrantedBy", "user", "-ProfileGrantedAt", now.Add(-time.Minute).Format(time.RFC3339),
		"-ProfileExpiresAt", now.Add(9 * time.Minute).Format(time.RFC3339),
	}

	var out bytes.Buffer
	if err := Run(base, &out); err != nil {
		t.Fatal(err)
	}
	var preview autonomy.ProfileMutationPlan
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.ProfilePath != profileRel {
		t.Fatalf("current preview profilePath = %q, want %q", preview.ProfilePath, profileRel)
	}
	if current, err := os.ReadFile(profilePath); err != nil || !bytes.Equal(current, initialProfileBytes) {
		t.Fatalf("current preview changed profile: err=%v\n%s", err, current)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, projectstate.LegacyDir)); !os.IsNotExist(err) {
		t.Fatalf("current preview created legacy state root: %v", err)
	}

	out.Reset()
	apply := append(append([]string{}, base...), "-Apply", "-ExpectedProfilePlanSha256", preview.ExpectedPlanSHA256)
	if err := Run(apply, &out); err != nil {
		t.Fatal(err)
	}
	profile, _, exists, err := autonomy.Read(caseRoot, "main")
	if err != nil || !exists || profile.Mode != autonomy.ModeAutonomous {
		t.Fatalf("current managed autonomous apply: profile=%+v exists=%t err=%v", profile, exists, err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, projectstate.LegacyDir)); !os.IsNotExist(err) {
		t.Fatalf("current apply created legacy state root: %v", err)
	}

	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.LegacyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	err = Run([]string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json",
		"-RevokeProfile", "-Lane", "main",
	}, &out)
	if err == nil || !strings.Contains(err.Error(), "both .steamai and .rekit") {
		t.Fatalf("dual-root revoke preview error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("dual-root revoke preview emitted partial output: %s", out.String())
	}
}

func TestRunGateManagedAutonomousPresetFailsClosed(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, defaults.DefaultPack)
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"main","status":"open","workspace":"workstreams/main/evidence"}]}`+"\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","status":"open"}`+"\n")
	defaultProfile, err := json.MarshalIndent(autonomy.DefaultProfile("main"), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, autonomy.RelPath("main"), string(append(defaultProfile, '\n')))
	now := time.Now().UTC().Truncate(time.Second)
	base := []string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json",
		"-ProvisionProfile", "-ProfilePreset", autonomy.ManagedAutonomousPresetV1, "-ProfileExplicitOptIn",
		"-Lane", "main", "-Action", "debug", "-TargetRef", "sample-alpha",
		"-RuntimeSeconds", "30", "-DiskMB", "8", "-Requests", "1",
		"-StopConditions", "timeout,unexpected-side-effect,scope-drift", "-OutputPaths", "workstreams/main/evidence/autonomous",
		"-ProfileGrantedBy", "user", "-ProfileGrantedAt", now.Add(-time.Minute).Format(time.RFC3339),
		"-ProfileExpiresAt", now.Add(9 * time.Minute).Format(time.RFC3339),
	}
	for name, mutate := range map[string]func([]string) []string{
		"missing manifest stop": func(args []string) []string {
			return replaceGateProfileArg(args, "-StopConditions", "timeout,scope-drift")
		},
		"unsafe output": func(args []string) []string {
			return replaceGateProfileArg(args, "-OutputPaths", "../escape")
		},
		"network without external scope": func(args []string) []string {
			args = replaceGateProfileArg(args, "-Action", "network")
			return replaceGateProfileArg(args, "-StopConditions", "live-target-ambiguity,unexpected-outbound-request,scope-drift")
		},
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run(mutate(append([]string{}, base...)), &out); err == nil {
				t.Fatalf("managed autonomous CLI accepted %s: %s", name, out.String())
			}
			profile, _, exists, err := autonomy.Read(caseRoot, "main")
			if err != nil || !exists || profile.Mode != autonomy.ModeManualGate {
				t.Fatalf("failed preview changed profile: profile=%+v exists=%t err=%v", profile, exists, err)
			}
		})
	}
}

func TestRunGateManagedAutonomousNetworkRequiresExactExternalScope(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, defaults.DefaultPack)
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"main","status":"open","workspace":"workstreams/main/evidence"}]}`+"\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","status":"open"}`+"\n")
	defaultProfile, err := json.MarshalIndent(autonomy.DefaultProfile("main"), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, autonomy.RelPath("main"), string(append(defaultProfile, '\n')))
	now := time.Now().UTC().Truncate(time.Second)
	target := "https://fixture.invalid:443"
	args := []string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json",
		"-ProvisionProfile", "-ProfilePreset", autonomy.ManagedAutonomousPresetV1, "-ProfileExplicitOptIn",
		"-ProfileExternalTargetScope", target,
		"-Lane", "main", "-Action", "network", "-TargetRef", target,
		"-RuntimeSeconds", "30", "-DiskMB", "8", "-Requests", "1",
		"-StopConditions", "live-target-ambiguity,unexpected-outbound-request,scope-drift",
		"-OutputPaths", "workstreams/main/evidence/autonomous",
		"-ProfileGrantedBy", "user", "-ProfileGrantedAt", now.Add(-time.Minute).Format(time.RFC3339),
		"-ProfileExpiresAt", now.Add(9 * time.Minute).Format(time.RFC3339),
	}
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var preview autonomy.ProfileMutationPlan
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil || preview.PlannedProfile.Mode != autonomy.ModeAutonomous {
		t.Fatalf("network managed autonomous preview=%+v err=%v\n%s", preview, err, out.String())
	}
}

func TestRunGateManagedAutonomousPresetRejectsUnknownPreset(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, defaults.DefaultPack)
	for name, args := range map[string][]string{
		"unknown preset": {
			"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack,
			"-ProvisionProfile", "-ProfilePreset", "unbounded", "-ProfileExplicitOptIn", "-Lane", "main",
		},
		"caller supplied managed id": {
			"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack,
			"-ProvisionProfile", "-ProfilePreset", autonomy.ManagedAutonomousPresetV1, "-ProfileExplicitOptIn", "-ProfileId", "forged", "-Lane", "main",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run(args, &out)
			if err == nil {
				t.Fatalf("managed autonomous preset accepted %s: %s", name, out.String())
			}
		})
	}
}

func TestRunGateProfileProvisionAndRevoke(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, defaults.DefaultPack)
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"main","status":"open","workspace":"workstreams/main/evidence"}]}`+"\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","status":"open"}`+"\n")
	defaultProfile, err := json.MarshalIndent(autonomy.DefaultProfile("main"), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(caseRoot, filepath.FromSlash(autonomy.RelPath("main")))
	initialProfileBytes := append(defaultProfile, '\n')
	writeCaseFile(t, caseRoot, autonomy.RelPath("main"), string(initialProfileBytes))
	now := time.Now().UTC().Truncate(time.Second)
	requestPath := "tooling/ida-agent-bridge/requests/" + strings.Repeat("a", 64) + ".json"
	base := []string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json",
		"-ProvisionProfile", "-Lane", "main", "-Action", "inspect", "-TargetRef", requestPath,
		"-ProfileId", "dpc04-main-inspect", "-ProfileGrantedBy", "user",
		"-ProfileGrantedAt", now.Add(-time.Minute).Format(time.RFC3339),
		"-ProfileExpiresAt", now.Add(9 * time.Minute).Format(time.RFC3339),
		"-RuntimeSeconds", "30", "-DiskMB", "1", "-Requests", "1",
		"-StopConditions", "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
		"-OutputPaths", "workstreams/main/evidence/ida-index",
	}

	var out bytes.Buffer
	if err := Run(base, &out); err != nil {
		t.Fatal(err)
	}
	var provision autonomy.ProfileMutationPlan
	if err := json.Unmarshal(out.Bytes(), &provision); err != nil {
		t.Fatalf("profile provision preview is not JSON: %v\n%s", err, out.String())
	}
	if provision.Operation != autonomy.ProfileOperationProvision || provision.ExpectedPlanSHA256 == "" || !provision.IsMutation || !provision.ReviewRequired || !provision.RequiresConfirmation {
		t.Fatalf("unexpected provision preview: %+v", provision)
	}
	previewProfileBytes, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(previewProfileBytes, initialProfileBytes) {
		t.Fatalf("profile preview changed autonomy.json:\n%s", previewProfileBytes)
	}

	out.Reset()
	apply := append(append([]string{}, base...), "-Apply", "-ExpectedProfilePlanSha256", provision.ExpectedPlanSHA256)
	if err := Run(apply, &out); err != nil {
		t.Fatal(err)
	}
	var applied autonomy.ProfileMutationResult
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("profile provision apply is not JSON: %v\n%s", err, out.String())
	}
	if !applied.Applied || applied.AlreadyApplied || applied.ProfileSHA256 != provision.PlannedProfileSHA256 {
		t.Fatalf("unexpected provision apply: %+v", applied)
	}
	profile, _, exists, err := autonomy.Read(caseRoot, "main")
	if err != nil || !exists || profile.Mode != autonomy.ModePreauthorized || profile.ProfileID != "dpc04-main-inspect" || !reflect.DeepEqual(profile.AllowedActions, []string{"inspect"}) || !reflect.DeepEqual(profile.DeniedActions, []string{"debug", "dump", "full-trace", "inject", "network", "patch", "symex"}) {
		t.Fatalf("unexpected provisioned profile: exists=%t profile=%+v err=%v", exists, profile, err)
	}

	out.Reset()
	revoke := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack, "-Format", "json", "-RevokeProfile", "-Lane", "main"}
	if err := Run(revoke, &out); err != nil {
		t.Fatal(err)
	}
	var revokePlan autonomy.ProfileMutationPlan
	if err := json.Unmarshal(out.Bytes(), &revokePlan); err != nil {
		t.Fatalf("profile revoke preview is not JSON: %v\n%s", err, out.String())
	}
	if revokePlan.Operation != autonomy.ProfileOperationRevoke || revokePlan.ExpectedPlanSHA256 == "" || !revokePlan.IsMutation {
		t.Fatalf("unexpected revoke preview: %+v", revokePlan)
	}
	out.Reset()
	if err := Run(append(revoke, "-Apply", "-ExpectedProfilePlanSha256", revokePlan.ExpectedPlanSHA256), &out); err != nil {
		t.Fatal(err)
	}
	profile, _, exists, err = autonomy.Read(caseRoot, "main")
	if err != nil || !exists || profile.Mode != autonomy.ModeManualGate || profile.ProfileID != "manual-main" {
		t.Fatalf("unexpected revoked profile: exists=%t profile=%+v err=%v", exists, profile, err)
	}
}

func TestRunGateProfileProvisionAcceptsSelectedLaneWorkspaceName(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, defaults.DefaultPack)
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"feature-mission","status":"open","workspace":"captures/feature_analysis/feature-mission"}]}`+"\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-mission/lane.json", `{"schemaVersion":1,"id":"feature-mission","status":"open"}`+"\n")
	profile, err := gateProvisionProfile(runtime.Context{
		RepoRoot: repoRoot(t),
		Target:   caseRoot,
		Pack:     defaults.DefaultPack,
	}, Options{Gate: gate.Options{
		ProvisionProfile: true,
		Lane:             "feature-mission",
		Action:           "inspect",
		TargetRef:        "tooling/ida-agent-bridge/requests/" + strings.Repeat("a", 64) + ".json",
		ProfileID:        "dpc04-feature-inspect",
		ProfileGrantedBy: "user",
		ProfileGrantedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		ProfileExpiresAt: time.Now().UTC().Add(9 * time.Minute).Format(time.RFC3339),
		RuntimeSeconds:   10,
		DiskMB:           4,
		Requests:         1,
		StopConditions:   "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
		OutputPaths:      "captures/feature_analysis/feature-mission/ida-index/session-1",
	}})
	if err != nil || !reflect.DeepEqual(profile.OutputPaths, []string{"captures/feature_analysis/feature-mission/ida-index/session-1"}) {
		t.Fatalf("selected lane workspace profile=%+v err=%v", profile, err)
	}
}

func TestRunGateProfileProvisionRejectsCapabilitiesOutsideFixedVMPIDAContract(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	validRequest := "tooling/ida-agent-bridge/requests/" + strings.Repeat("a", 64) + ".json"
	valid := []string{
		"-Command", "gate", "-Pack", defaults.DefaultPack, "-ProvisionProfile", "-Lane", "main",
		"-Action", "inspect", "-TargetRef", validRequest,
		"-ProfileId", "dpc04-main-inspect", "-ProfileGrantedBy", "user",
		"-ProfileGrantedAt", now.Add(-time.Minute).Format(time.RFC3339),
		"-ProfileExpiresAt", now.Add(9 * time.Minute).Format(time.RFC3339),
		"-RuntimeSeconds", "30", "-DiskMB", "1", "-Requests", "1",
		"-StopConditions", "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
		"-OutputPaths", "workstreams/main/evidence/ida-index",
	}
	for name, mutate := range map[string]func([]string) []string{
		"non-vmp pack": func(args []string) []string {
			return replaceGateProfileArg(args, "-Pack", "_template")
		},
		"debug action": func(args []string) []string {
			return replaceGateProfileArg(args, "-Action", "debug")
		},
		"patch action": func(args []string) []string {
			return replaceGateProfileArg(args, "-Action", "patch")
		},
		"inject action": func(args []string) []string {
			return replaceGateProfileArg(args, "-Action", "inject")
		},
		"dump action": func(args []string) []string {
			return replaceGateProfileArg(args, "-Action", "dump")
		},
		"network action": func(args []string) []string {
			return replaceGateProfileArg(args, "-Action", "network")
		},
		"non-addressed request": func(args []string) []string {
			return replaceGateProfileArg(args, "-TargetRef", "tooling/ida-agent-bridge/requests/current.json")
		},
		"request outside fixed root": func(args []string) []string {
			return replaceGateProfileArg(args, "-TargetRef", "workspace/main/"+strings.Repeat("a", 64)+".json")
		},
		"unbounded requests": func(args []string) []string {
			return replaceGateProfileArg(args, "-Requests", "2")
		},
		"oversized disk": func(args []string) []string {
			return replaceGateProfileArg(args, "-DiskMB", "5")
		},
		"wrong stop conditions": func(args []string) []string {
			return replaceGateProfileArg(args, "-StopConditions", "scope-drift")
		},
		"foreign output": func(args []string) []string {
			return replaceGateProfileArg(args, "-OutputPaths", "workspace/other/ida-index")
		},
	} {
		t.Run(name, func(t *testing.T) {
			caseRoot := attachedCaseWithPack(t, defaults.DefaultPack)
			writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"main","status":"open","workspace":"workstreams/main/evidence"}]}`+"\n")
			writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","status":"open"}`+"\n")
			args := append([]string{}, valid...)
			args = append(args, "-Target", caseRoot)
			var out bytes.Buffer
			if err := Run(mutate(args), &out); err == nil {
				t.Fatalf("profile provision accepted capability outside fixed contract: %s", out.String())
			}
		})
	}
}

func attachedCaseWithStateDirAndPack(t *testing.T, stateDir, pack string) string {
	t.Helper()
	if stateDir != projectstate.CurrentDir && stateDir != projectstate.LegacyDir {
		t.Fatalf("unsupported state root fixture: %s", stateDir)
	}
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := "templateRoot: " + repoRoot(t) + "\ntemplatePack: " + pack + "\nprojectName: demo\nprojectRoot: " + caseRoot + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, stateDir, "instance.yml"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func replaceGateProfileArg(args []string, name, value string) []string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == name {
			args[i+1] = value
			return args
		}
	}
	return args
}

func TestRunGateProfileModeRejectsMixedModesAndUnboundFields(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, defaults.DefaultPack)
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"main","status":"open","workspace":"workstreams/main/evidence"}]}`+"\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","status":"open"}`+"\n")
	for name, args := range map[string][]string{
		"both profile modes": {"-ProvisionProfile", "-RevokeProfile", "-Lane", "main"},
		"dispatch mode":      {"-ProvisionProfile", "-Lane", "main", "-RecordAdapterExecutionDispatch"},
		"unbound field":      {"-Lane", "main", "-ProfileId", "dpc04-main-inspect"},
		"unbound preset":     {"-Lane", "main", "-ProfilePreset", autonomy.ManagedAutonomousPresetV1},
		"unbound opt-in":     {"-Lane", "main", "-ProfileExplicitOptIn"},
		"whatif":             {"-ProvisionProfile", "-Lane", "main", "-WhatIf"},
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			base := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", defaults.DefaultPack}
			if err := Run(append(base, args...), &out); err == nil {
				t.Fatalf("gate profile accepted incompatible args: %v", args)
			}
		})
	}
}
