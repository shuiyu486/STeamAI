package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func requireCurrentSyncApplyForCLITest(t *testing.T) {
	t.Helper()
	if !rekitfs.HandleBoundExactMutationSupported() {
		t.Skip("current sync Apply requires handle-bound exact filesystem mutation")
	}
}

func TestRunCurrentSyncMaintenanceRejectsInvalidCombinationsBeforeRuntimeDiscovery(t *testing.T) {
	sha := strings.Repeat("a", 64)
	sourcePair := []string{
		"-SourceRepoRoot", "source",
		"-SourceExecutable", "steamai.exe",
	}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "source-root-only",
			args: []string{"-Command", "sync", "-Target", "target", "-Pack", "_template", "-SourceRepoRoot", "source"},
			want: "requires non-empty -SourceRepoRoot and -SourceExecutable together",
		},
		{
			name: "empty-source-executable",
			args: []string{"-Command", "sync", "-Target", "target", "-Pack", "_template", "-SourceRepoRoot", "source", "-SourceExecutable", ""},
			want: "requires non-empty -SourceRepoRoot and -SourceExecutable together",
		},
		{
			name: "update",
			args: append([]string{"-Command", "update", "-Target", "target", "-Pack", "_template"}, sourcePair...),
			want: "supported only by sync",
		},
		{
			name: "missing-target",
			args: append([]string{"-Command", "sync", "-Pack", "_template"}, sourcePair...),
			want: "requires an explicit -Target",
		},
		{
			name: "missing-explicit-pack",
			args: append([]string{"-Command", "sync", "-Target", "target"}, sourcePair...),
			want: "requires an explicit -Pack",
		},
		{
			name: "what-if",
			args: append([]string{"-Command", "sync", "-Target", "target", "-Pack", "_template", "-WhatIf"}, sourcePair...),
			want: "without -WhatIf",
		},
		{
			name: "force",
			args: append([]string{"-Command", "sync", "-Target", "target", "-Pack", "_template", "-Force"}, sourcePair...),
			want: "does not accept -Force",
		},
		{
			name: "review-artifact",
			args: append([]string{"-Command", "sync", "-Target", "target", "-Pack", "_template", "-ReviewOutputDir", "review"}, sourcePair...),
			want: "does not accept -Force or review artifact options",
		},
		{
			name: "preview-with-plan-sha",
			args: append(append([]string{"-Command", "sync", "-Target", "target", "-Pack", "_template"}, sourcePair...), "-ExpectedCurrentSyncPlanSha256", sha),
			want: "preview does not accept -ExpectedCurrentSyncPlanSha256",
		},
		{
			name: "apply-without-plan-sha",
			args: append(append([]string{"-Command", "sync", "-Target", "target", "-Pack", "_template"}, sourcePair...), "-Apply"),
			want: "-Apply requires -ExpectedCurrentSyncPlanSha256",
		},
		{
			name: "apply-with-malformed-plan-sha",
			args: append(append([]string{"-Command", "sync", "-Target", "target", "-Pack", "_template"}, sourcePair...), "-ExpectedCurrentSyncPlanSha256", "not-a-sha", "-Apply"),
			want: "requires a valid -ExpectedCurrentSyncPlanSha256",
		},
		{
			name: "duplicate-source-root",
			args: append(append([]string{"-Command", "sync", "-Target", "target", "-Pack", "_template", "-SourceRepoRoot", "first"}, sourcePair...), "-ExpectedCurrentSyncPlanSha256", sha, "-Apply"),
			want: "argument is duplicated",
		},
		{
			name: "unrelated-option",
			args: append([]string{"-Command", "sync", "-Target", "target", "-Pack", "_template", "-Goal", "not-current-sync"}, sourcePair...),
			want: "does not accept argument: -Goal",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run(test.args, &out)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("current sync CLI validation error = %v, want %q", err, test.want)
			}
			if out.Len() != 0 {
				t.Fatalf("current sync CLI validation wrote output: %s", out.String())
			}
		})
	}
}

func TestRunCurrentSyncMaintenanceApplyArgsRoundTrip(t *testing.T) {
	requireCurrentSyncApplyForCLITest(t)
	fixture := newCLIFixture(t, cliFixtureOptions{})
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	runInitApplyFromPreview(
		t,
		&out,
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-ProjectName", "current-sync-cli",
	)
	if err := os.WriteFile(
		filepath.Join(fixture.repoRoot, "common", "policies", "current-sync-cli-refresh.md"),
		[]byte("current sync CLI refresh\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	sourceExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	previewArgs := []string{
		"-Command", "sync",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-SourceRepoRoot", fixture.repoRoot,
		"-SourceExecutable", sourceExecutable,
		"-Format", "json",
	}
	out.Reset()
	if err := Run(previewArgs, &out); err != nil {
		t.Fatal(err)
	}
	var plan syncreview.CurrentSyncPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("current sync preview is not JSON: %v\n%s", err, out.String())
	}
	if plan.Status != "ready-to-refresh" || plan.AlreadyCurrent ||
		plan.ExpectedPlanSHA256 == "" || len(plan.ApplyArgs) == 0 {
		t.Fatalf("unexpected current sync CLI preview: %+v", plan)
	}
	parsed, err := Parse(plan.ApplyArgs)
	if err != nil {
		t.Fatalf("current sync ApplyArgs did not parse: %v", err)
	}
	if err := validateCurrentSyncMaintenanceOptions(parsed); err != nil {
		t.Fatalf("current sync ApplyArgs did not pass exact validation: %v", err)
	}
	if !parsed.Apply || !parsed.sourceRepoRootProvided ||
		!parsed.sourceExecutableProvided ||
		!parsed.expectedCurrentSyncPlanSHA256Provided ||
		parsed.SourceRepoRoot != fixture.repoRoot ||
		parsed.SourceExecutable != sourceExecutable ||
		parsed.ExpectedCurrentSyncPlanSHA256 != plan.ExpectedPlanSHA256 {
		t.Fatalf("current sync ApplyArgs lost identity: %+v", parsed)
	}

	out.Reset()
	if err := Run(plan.ApplyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var applied struct {
		Status         string `json:"status"`
		Applied        bool   `json:"applied"`
		Replay         bool   `json:"replay"`
		AlreadyCurrent bool   `json:"alreadyCurrent"`
		PlanSHA256     string `json:"planSha256"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("current sync Apply is not JSON: %v\n%s", err, out.String())
	}
	if applied.Status != "refreshed" || !applied.Applied || applied.Replay ||
		applied.AlreadyCurrent || applied.PlanSHA256 != plan.ExpectedPlanSHA256 {
		t.Fatalf("unexpected current sync CLI Apply: %+v", applied)
	}
	for _, rel := range []string{
		"maintenance/current-sync-v1/intent.json",
		"maintenance/current-sync-v1/owner.json",
	} {
		if _, err := os.Lstat(filepath.Join(
			caseRoot,
			projectstate.CurrentDir,
			filepath.FromSlash(rel),
		)); !os.IsNotExist(err) {
			t.Fatalf("current sync CLI Apply retained %s: %v", rel, err)
		}
	}

	out.Reset()
	if err := Run(plan.ApplyArgs, &out); err != nil {
		t.Fatal(err)
	}
	var replay struct {
		Status         string `json:"status"`
		Applied        bool   `json:"applied"`
		Replay         bool   `json:"replay"`
		AlreadyCurrent bool   `json:"alreadyCurrent"`
		PlanSHA256     string `json:"planSha256"`
	}
	if err := json.Unmarshal(out.Bytes(), &replay); err != nil {
		t.Fatalf("current sync replay is not JSON: %v\n%s", err, out.String())
	}
	if replay.Status != "already-current" || replay.Applied || !replay.Replay ||
		!replay.AlreadyCurrent || replay.PlanSHA256 != plan.ExpectedPlanSHA256 {
		t.Fatalf("unexpected current sync CLI replay: %+v", replay)
	}
}

func TestRunCurrentSyncRecoveryFrontDoorWorksWithoutActiveInstance(t *testing.T) {
	requireCurrentSyncApplyForCLITest(t)
	fixture := newCLIFixture(t, cliFixtureOptions{})
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	runInitApplyFromPreview(
		t,
		&out,
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-ProjectName", "current-sync-recovery-front-door",
	)
	refreshPath := filepath.Join(
		fixture.repoRoot,
		"common",
		"policies",
		"current-sync-recovery-front-door.md",
	)
	if err := os.WriteFile(
		refreshPath,
		[]byte("current sync recovery front door\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	sourceExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	plan, err := syncreview.CurrentSyncPreview(
		fixture.repoRoot,
		caseRoot,
		"_template",
		syncreview.CurrentSyncOptions{
			Command:          "sync",
			ProjectName:      "current-sync-recovery-front-door",
			SourceExecutable: sourceExecutable,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.AlreadyCurrent || len(plan.ApplyArgs) == 0 {
		t.Fatalf("current sync recovery fixture produced no refresh: %+v", plan)
	}
	restore := syncreview.SetCurrentSyncApplyTransitionHookForTest(
		func(stage string, _ syncreview.CurrentSyncPlan) error {
			if stage == "after-operation-effect:activation-live-to-previous" {
				return fmt.Errorf("simulated activation interruption")
			}
			return nil
		},
	)
	out.Reset()
	err = Run(plan.ApplyArgs, &out)
	restore()
	if err == nil || !strings.Contains(err.Error(), "simulated activation interruption") {
		t.Fatalf("current sync recovery fixture did not interrupt activation: %v", err)
	}
	instancePath := filepath.Join(caseRoot, projectstate.CurrentDir, "instance.yml")
	if _, err := os.Lstat(instancePath); !os.IsNotExist(err) {
		t.Fatalf("current sync recovery fixture retained active instance: %v", err)
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "status",
		"-Target", caseRoot,
		"-Format", "compact-json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var compact struct {
		Mode                string `json:"mode"`
		CurrentSyncRecovery struct {
			State       string   `json:"state"`
			Pending     bool     `json:"pending"`
			Blocked     bool     `json:"blocked"`
			Recoverable bool     `json:"recoverable"`
			ApplyArgs   []string `json:"applyArgs"`
		} `json:"currentSyncRecovery"`
	}
	if err := json.Unmarshal(out.Bytes(), &compact); err != nil {
		t.Fatalf("compact recovery status is not JSON: %v\n%s", err, out.String())
	}
	if compact.Mode != "case-maintenance-recovery" ||
		compact.CurrentSyncRecovery.State != syncreview.CurrentSyncRecoveryResume ||
		!compact.CurrentSyncRecovery.Pending ||
		!compact.CurrentSyncRecovery.Blocked ||
		!compact.CurrentSyncRecovery.Recoverable ||
		compact.CurrentSyncRecovery.ApplyArgs != nil {
		t.Fatalf("unexpected compact recovery status: %+v", compact)
	}
	if strings.Contains(out.String(), "expectedCurrentSyncPlan") ||
		strings.Contains(out.String(), "applyArgs") {
		t.Fatalf("compact recovery status exposed internal Apply identity: %s", out.String())
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "doctor",
		"-Target", caseRoot,
		"-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var doctor struct {
		Mode                string `json:"mode"`
		Valid               bool   `json:"valid"`
		CurrentSyncRecovery struct {
			State string `json:"state"`
		} `json:"currentSyncRecovery"`
	}
	if err := json.Unmarshal(out.Bytes(), &doctor); err != nil {
		t.Fatalf("recovery doctor is not JSON: %v\n%s", err, out.String())
	}
	if doctor.Mode != "case-maintenance-recovery" || doctor.Valid ||
		doctor.CurrentSyncRecovery.State != syncreview.CurrentSyncRecoveryResume {
		t.Fatalf("unexpected recovery doctor: %+v", doctor)
	}

	out.Reset()
	err = Run([]string{
		"-Command", "packs",
		"-Target", caseRoot,
		"-Format", "json",
	}, &out)
	if err == nil || !strings.Contains(err.Error(), "项目已安全停下") || out.Len() != 0 {
		t.Fatalf("ordinary command crossed recovery fence: err=%v output=%s", err, out.String())
	}
}

func TestRunCurrentSyncMaintenanceBypassesOrdinaryRuntimeValidation(t *testing.T) {
	fixture := newCLIFixture(t, cliFixtureOptions{})
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	runInitApplyFromPreview(
		t,
		&out,
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-ProjectName", "current-sync-recovery",
	)
	manifestPath := filepath.Join(
		caseRoot,
		projectstate.CurrentDir,
		"runtime",
		"manifest.json",
	)
	if err := os.WriteFile(manifestPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	err = Run([]string{
		"-Command", "sync",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-SourceRepoRoot", fixture.repoRoot,
		"-SourceExecutable", sourceExecutable,
		"-Format", "json",
	}, &out)
	if err == nil || !strings.Contains(err.Error(), "current sync target bundle is not a valid refresh base") {
		t.Fatalf("current sync maintenance did not reach recovery-aware backend: %v", err)
	}
	if strings.Contains(err.Error(), "validate project-local STeamAI runtime bundle") {
		t.Fatalf("ordinary runtime validation intercepted current sync maintenance: %v", err)
	}
}
