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
	"github.com/shuiyu486/re-context-kits/internal/rekit/testfixture"
	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
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

func TestRunGateProfileProvisionAndAuthorizeWebSecurityOpenAPI(t *testing.T) {
	const (
		pack       = "web-security"
		lane       = "feature-api"
		inputPath  = "inputs/openapi.json"
		outputPath = "workspace/features/feature-api/openapi/session-1"
	)
	caseRoot := testfixture.NewProject(t, testfixture.ProjectOptions{
		Layout: testfixture.CurrentProject, SourceRepo: repoRoot(t), Pack: pack, ProjectName: "web-profile",
	}).CaseRoot
	writeCaseFile(t, caseRoot, ".steamai/board.json", `{"lanes":[{"id":"feature-api","status":"open","workspace":"workspace/features/feature-api","currentExecutor":"executor-web","executorGeneration":1}]}`+"\n")
	writeCaseFile(t, caseRoot, ".steamai/lanes/feature-api/lane.json", `{
  "schemaVersion": 1,
  "id": "feature-api",
  "type": "feature",
  "name": "feature-api",
  "title": "Feature API",
  "status": "open",
  "authority": false,
  "workspace": "workspace/features/feature-api",
  "canWrite": ["own-workspace"],
  "readOnly": [".steamai/facts/**"],
  "outputs": ["observation", "request", "candidate", "summary"],
  "counters": {},
  "currentExecutor": "executor-web",
  "executorGeneration": 1,
  "createdAt": "2026-08-20T00:00:00Z",
  "updatedAt": "2026-08-20T00:00:00Z"
}`+"\n")
	if _, _, err := autonomy.EnsureManualProfile(caseRoot, lane); err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, inputPath, `{"openapi":"3.0.3","servers":[{"url":"http://127.0.0.1:18080/api"}],"paths":{"/health":{"get":{"operationId":"health","responses":{"200":{"description":"ok"}}}}},"components":{"securitySchemes":{}}}`)

	now := time.Now().UTC().Truncate(time.Second)
	base := []string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-Format", "json",
		"-ProvisionProfile", "-Lane", lane, "-Action", "inspect", "-TargetRef", inputPath,
		"-ProfileId", "web-feature-api-openapi", "-ProfileGrantedBy", "user",
		"-ProfileGrantedAt", now.Add(-time.Minute).Format(time.RFC3339),
		"-ProfileExpiresAt", now.Add(9 * time.Minute).Format(time.RFC3339),
		"-RuntimeSeconds", "10", "-DiskMB", "4", "-Requests", "1",
		"-StopConditions", "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
		"-OutputPaths", outputPath,
	}
	var out bytes.Buffer
	if err := Run(base, &out); err != nil {
		t.Fatal(err)
	}
	var preview autonomy.ProfileMutationPlan
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.ExpectedPlanSHA256 == "" || preview.PlannedProfile.Mode != autonomy.ModePreauthorized ||
		!reflect.DeepEqual(preview.PlannedProfile.AllowedActions, []string{"inspect"}) ||
		!reflect.DeepEqual(preview.PlannedProfile.TargetScope, []autonomy.Target{{Match: "exact", Value: inputPath}}) {
		t.Fatalf("unexpected web-security profile preview: %+v", preview)
	}
	out.Reset()
	if err := Run(append(append([]string{}, base...), "-Apply", "-ExpectedProfilePlanSha256", preview.ExpectedPlanSHA256), &out); err != nil {
		t.Fatal(err)
	}
	var applied autonomy.ProfileMutationResult
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil || !applied.Applied {
		t.Fatalf("web-security profile apply = %+v err=%v", applied, err)
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "gate", "-Target", caseRoot, "-Pack", pack, "-Format", "json", "-Apply",
		"-Action", "inspect", "-Lane", lane, "-Actor", "web-profile-test",
		"-Subject", "bounded OpenAPI inventory", "-Summary", "typed compiled-in adapter evidence",
		"-TargetRef", inputPath, "-RuntimeSeconds", "10", "-DiskMB", "4", "-Requests", "1",
		"-StopConditions", "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
		"-OutputPaths", outputPath,
	}, &out); err != nil {
		t.Fatal(err)
	}
	var authorized gate.ApplyResult
	if err := json.Unmarshal(out.Bytes(), &authorized); err != nil {
		t.Fatal(err)
	}
	if !authorized.Applied || authorized.Event == nil || authorized.Event.Status != "authorized-gate" ||
		authorized.Event.Gate.Authorization.Decision != autonomy.DecisionPreauthorized {
		t.Fatalf("web-security exact gate was not preauthorized: %+v", authorized)
	}
}

func TestGateProvisionProfileRejectsInvalidWebSecurityContracts(t *testing.T) {
	const (
		pack       = "web-security"
		lane       = "feature-api"
		inputPath  = "inputs/openapi.json"
		outputPath = "workspace/features/feature-api/openapi/session-1"
	)
	caseRoot := testfixture.NewProject(t, testfixture.ProjectOptions{
		Layout: testfixture.CurrentProject, SourceRepo: repoRoot(t), Pack: pack, ProjectName: "web-profile-reject",
	}).CaseRoot
	writeCaseFile(t, caseRoot, ".steamai/board.json", `{"lanes":[{"id":"feature-api","status":"open","workspace":"workspace/features/feature-api"}]}`+"\n")
	writeCaseFile(t, caseRoot, inputPath, `{"openapi":"3.0.3","servers":[{"url":"http://127.0.0.1:18080/api"}],"paths":{},"components":{"securitySchemes":{}}}`)
	now := time.Now().UTC().Truncate(time.Second)
	valid := Options{Gate: gate.Options{
		ProvisionProfile: true, Lane: lane, Action: "inspect", TargetRef: inputPath,
		ProfileID: "web-feature-api-openapi", ProfileGrantedBy: "user",
		ProfileGrantedAt: now.Add(-time.Minute).Format(time.RFC3339), ProfileExpiresAt: now.Add(9 * time.Minute).Format(time.RFC3339),
		RuntimeSeconds: 10, DiskMB: 4, Requests: 1,
		StopConditions: "scope-drift,source-drift,output-exceeds-bounded-evidence-packet", OutputPaths: outputPath,
	}}
	ctx := runtime.Context{RepoRoot: repoRoot(t), Target: caseRoot, Pack: pack}
	for name, mutate := range map[string]func(*Options, *runtime.Context){
		"unknown action": func(opt *Options, _ *runtime.Context) { opt.Gate.Action = "debug" },
		"wrong stops":    func(opt *Options, _ *runtime.Context) { opt.Gate.StopConditions = "scope-drift" },
		"foreign output": func(opt *Options, _ *runtime.Context) { opt.Gate.OutputPaths = "workspace/other/openapi/session-1" },
		"wrong output kind": func(opt *Options, _ *runtime.Context) {
			opt.Gate.OutputPaths = "workspace/features/feature-api/replay/session-1"
		},
		"oversized runtime":   func(opt *Options, _ *runtime.Context) { opt.Gate.RuntimeSeconds = 31 },
		"multiple requests":   func(opt *Options, _ *runtime.Context) { opt.Gate.Requests = 2 },
		"missing target":      func(opt *Options, _ *runtime.Context) { opt.Gate.TargetRef = "inputs/missing.json" },
		"non-production pack": func(_ *Options, ctx *runtime.Context) { ctx.Pack = "_template" },
	} {
		t.Run(name, func(t *testing.T) {
			opt := valid
			localCtx := ctx
			mutate(&opt, &localCtx)
			if _, err := gateProvisionProfile(localCtx, opt); err == nil {
				t.Fatal("accepted invalid web-security profile contract")
			}
		})
	}
	t.Run("non-OpenAPI input", func(t *testing.T) {
		invalidPath := "inputs/not-openapi.json"
		writeCaseFile(t, caseRoot, invalidPath, `{"not":"openapi"}`)
		opt := valid
		opt.Gate.TargetRef = invalidPath
		if _, err := gateProvisionProfile(ctx, opt); err == nil {
			t.Fatal("accepted invalid OpenAPI profile target")
		}
	})
}

func TestGateProvisionProfileAcceptsCanonicalBoundedReplayRequest(t *testing.T) {
	const (
		pack          = "web-security"
		lane          = "feature-api"
		inventoryPath = "inputs/openapi-inventory.json"
		outputPath    = "workspace/features/feature-api/replay/session-1"
	)
	caseRoot := testfixture.NewProject(t, testfixture.ProjectOptions{
		Layout: testfixture.CurrentProject, SourceRepo: repoRoot(t), Pack: pack, ProjectName: "web-replay-profile",
	}).CaseRoot
	writeCaseFile(t, caseRoot, ".steamai/board.json", `{"lanes":[{"id":"feature-api","status":"open","workspace":"workspace/features/feature-api"}]}`+"\n")
	sourceData := []byte(`{"openapi":"3.0.3","servers":[{"url":"http://127.0.0.1:18080/api"}],"paths":{"/health":{"get":{"operationId":"health","responses":{"200":{"description":"ok"}}}}},"components":{"securitySchemes":{}}}`)
	source, err := websecurity.BindFile("inputs/openapi.json", sourceData, websecurity.MaxOpenAPIBytes)
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := websecurity.ImportOpenAPI(source, sourceData)
	if err != nil {
		t.Fatal(err)
	}
	inventoryData, err := websecurity.CanonicalInventoryBytes(inventory)
	if err != nil {
		t.Fatal(err)
	}
	inventoryBinding, err := websecurity.BindFile(inventoryPath, inventoryData, websecurity.MaxInventoryBytes)
	if err != nil {
		t.Fatal(err)
	}
	request, err := websecurity.NewReplayRequest(
		inventory, inventoryBinding, "get /health",
		websecurity.ReplayTarget{Scheme: "http", Host: "127.0.0.1", Port: 18080, BasePath: "/api"},
		"/health", nil,
		websecurity.ExpectedResponse{StatusCode: 200, Body: websecurity.DigestExpectation{SHA256: websecurity.SHA256(nil)}, Headers: []websecurity.HeaderExpectation{}},
		websecurity.DefaultReplayLimits(),
	)
	if err != nil {
		t.Fatal(err)
	}
	requestData, err := websecurity.CanonicalReplayRequestBytes(request)
	if err != nil {
		t.Fatal(err)
	}
	requestBinding, err := websecurity.BindReplayRequest(requestData)
	if err != nil {
		t.Fatal(err)
	}
	requestPath := requestBinding.Path
	writeCaseFile(t, caseRoot, requestPath, string(requestData))
	now := time.Now().UTC().Truncate(time.Second)
	profile, err := gateProvisionProfile(runtime.Context{RepoRoot: repoRoot(t), Target: caseRoot, Pack: pack}, Options{Gate: gate.Options{
		ProvisionProfile: true, Lane: lane, Action: "network", TargetRef: requestPath,
		ProfileID: "web-feature-api-replay", ProfileGrantedBy: "user",
		ProfileGrantedAt: now.Add(-time.Minute).Format(time.RFC3339), ProfileExpiresAt: now.Add(9 * time.Minute).Format(time.RFC3339),
		RuntimeSeconds: 10, DiskMB: 4, Requests: 1,
		StopConditions: "live-target-ambiguity,unexpected-outbound-request,scope-drift,delivery-uncertain,response-body-limit,response-read",
		OutputPaths:    outputPath,
	}})
	if err != nil || !reflect.DeepEqual(profile.AllowedActions, []string{"network"}) || profile.TargetScope[0].Value != requestPath {
		t.Fatalf("bounded replay profile = %+v err=%v", profile, err)
	}

	legacyPath := "inputs/bounded-replay-request.json"
	writeCaseFile(t, caseRoot, legacyPath, string(requestData))
	legacyOptions := Options{Gate: gate.Options{
		ProvisionProfile: true, Lane: lane, Action: "network", TargetRef: legacyPath,
		ProfileID: "web-feature-api-replay-legacy-path", ProfileGrantedBy: "user",
		ProfileGrantedAt: now.Add(-time.Minute).Format(time.RFC3339), ProfileExpiresAt: now.Add(9 * time.Minute).Format(time.RFC3339),
		RuntimeSeconds: 10, DiskMB: 4, Requests: 1,
		StopConditions: "live-target-ambiguity,unexpected-outbound-request,scope-drift,delivery-uncertain,response-body-limit,response-read",
		OutputPaths:    outputPath,
	}}
	if _, err := gateProvisionProfile(runtime.Context{RepoRoot: repoRoot(t), Target: caseRoot, Pack: pack}, legacyOptions); err == nil || !strings.Contains(err.Error(), "content-addressed bounded replay request") {
		t.Fatalf("accepted canonical replay bytes at a non-content-addressed path: %v", err)
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
