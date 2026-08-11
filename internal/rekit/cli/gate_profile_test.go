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
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func TestParseGateProfileFlags(t *testing.T) {
	opt, err := Parse([]string{
		"-Command", "gate", "-ProvisionProfile",
		"-ProfileId", "dpc04-main-inspect",
		"-ProfileGrantedBy", "user",
		"-ProfileGrantedAt", "2026-08-11T01:00:00Z",
		"-ProfileExpiresAt", "2026-08-11T01:10:00Z",
		"-ExpectedProfilePlanSha256", strings.Repeat("a", 64),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !opt.Gate.ProvisionProfile || opt.Gate.RevokeProfile || opt.Gate.ProfileID != "dpc04-main-inspect" || opt.Gate.ProfileGrantedBy != "user" || opt.Gate.ExpectedProfilePlanSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("profile flags not parsed: %+v", opt.Gate)
	}
}

func TestRunGateProfileProvisionAndRevoke(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
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
		"-Command", "gate", "-Target", caseRoot, "-Pack", "vmp-re", "-Format", "json",
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
	revoke := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", "vmp-re", "-Format", "json", "-RevokeProfile", "-Lane", "main"}
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
	caseRoot := attachedCaseWithPack(t, "vmp-re")
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
		"-Command", "gate", "-Pack", "vmp-re", "-ProvisionProfile", "-Lane", "main",
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
			caseRoot := attachedCaseWithPack(t, "vmp-re")
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
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"main","status":"open","workspace":"workstreams/main/evidence"}]}`+"\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","status":"open"}`+"\n")
	for name, args := range map[string][]string{
		"both profile modes": {"-ProvisionProfile", "-RevokeProfile", "-Lane", "main"},
		"dispatch mode":      {"-ProvisionProfile", "-Lane", "main", "-RecordAdapterExecutionDispatch"},
		"unbound field":      {"-Lane", "main", "-ProfileId", "dpc04-main-inspect"},
		"whatif":             {"-ProvisionProfile", "-Lane", "main", "-WhatIf"},
	} {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			base := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", "vmp-re"}
			if err := Run(append(base, args...), &out); err == nil {
				t.Fatalf("gate profile accepted incompatible args: %v", args)
			}
		})
	}
}
