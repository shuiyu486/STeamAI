package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

const rekitExecutableHelperEnv = "STEAMAI_REKIT_EXECUTABLE_HELPER"

func TestMain(m *testing.M) {
	if os.Getenv(rekitExecutableHelperEnv) == "1" {
		os.Exit(run(os.Args[1:]))
	}
	os.Exit(m.Run())
}

func requireCurrentSyncApplyForProcessTest(t *testing.T) {
	t.Helper()
	if !rekitfs.HandleBoundExactMutationSupported() {
		t.Skip("current sync Apply requires handle-bound exact filesystem mutation")
	}
}

func TestProjectLocalExecutableRecoveryFrontDoorsE2E(t *testing.T) {
	requireCurrentSyncApplyForProcessTest(t)
	repoRoot := rekitTestRepoRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "project")
	preview, err := syncreview.InitPreview(
		repoRoot,
		caseRoot,
		"_template",
		syncreview.ApplyOptions{
			ProjectName:      "recovery-process-before",
			CreateLocalFiles: true,
			Command:          "init",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(
		repoRoot,
		caseRoot,
		"_template",
		syncreview.ApplyOptions{
			ProjectName:        "recovery-process-before",
			CreateLocalFiles:   true,
			Command:            "init",
			ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
		},
	); err != nil {
		t.Fatal(err)
	}

	sourceExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	opt := syncreview.CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "recovery-process",
		SourceExecutable: sourceExecutable,
	}
	plan, err := syncreview.CurrentSyncPreview(
		repoRoot,
		caseRoot,
		"_template",
		opt,
	)
	if err != nil {
		t.Fatal(err)
	}
	restore := syncreview.SetCurrentSyncApplyTransitionHookForTest(
		func(stage string, _ syncreview.CurrentSyncPlan) error {
			if stage == "after-operation-effect:activation-live-to-previous" {
				return errors.New("simulated process recovery interruption")
			}
			return nil
		},
	)
	_, applyErr := syncreview.CurrentSyncApply(
		caseRoot,
		"_template",
		syncreview.CurrentSyncApplyOptions{
			SourceRepoRoot:     repoRoot,
			ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
			CurrentSyncOptions: opt,
		},
	)
	restore()
	if applyErr == nil || !strings.Contains(
		applyErr.Error(),
		"simulated process recovery interruption",
	) {
		t.Fatalf("current sync process fixture did not interrupt: %v", applyErr)
	}
	instancePath := filepath.Join(caseRoot, ".steamai", "instance.yml")
	if _, err := os.Lstat(instancePath); !os.IsNotExist(err) {
		t.Fatalf("process recovery fixture retained active instance: %v", err)
	}
	projectExecutable := filepath.Join(
		caseRoot,
		".steamai",
		"runtime",
		"bin",
		runtimebundle.ExecutableName(),
	)

	statusStdout, statusStderr, statusErr := runRekitExecutable(
		t,
		projectExecutable,
		"runtime",
		"-Command", "status",
		"-Format", "compact-json",
	)
	if statusErr != nil || statusStderr != "" ||
		!strings.Contains(statusStdout, `"mode":"case-maintenance-recovery"`) ||
		len([]byte(statusStdout)) > 4096 || !strings.HasSuffix(statusStdout, "\n") ||
		strings.Contains(statusStdout, `"targetProvided":true`) ||
		strings.Contains(statusStdout, "applyArgs") ||
		strings.Contains(strings.ToLower(statusStdout), strings.ToLower(plan.ExpectedPlanSHA256)) {
		t.Fatalf(
			"project-local recovery status err=%v stderr=%q stdout=%s",
			statusErr,
			statusStderr,
			statusStdout,
		)
	}

	doctorStdout, doctorStderr, doctorErr := runRekitExecutable(
		t,
		projectExecutable,
		"runtime",
		"-Command", "doctor",
		"-Target", caseRoot,
		"-Format", "json",
	)
	if doctorErr != nil || doctorStderr != "" ||
		!strings.Contains(doctorStdout, `"mode": "case-maintenance-recovery"`) {
		t.Fatalf(
			"project-local recovery doctor err=%v stderr=%q stdout=%s",
			doctorErr,
			doctorStderr,
			doctorStdout,
		)
	}

	dailyStdout, dailyStderr, dailyErr := runRekitExecutable(
		t,
		projectExecutable,
		"host",
		"-daily",
		"-claude", filepath.Join(caseRoot, "must-not-run-claude"),
		"-model", "recovery-process-model",
		"-timeout", "1s",
		"-max-attempts", "9",
	)
	if dailyErr != nil || dailyStderr != "" {
		t.Fatalf(
			"project-local recovery daily err=%v stderr=%q stdout=%s",
			dailyErr,
			dailyStderr,
			dailyStdout,
		)
	}
	var daily sessionhost.DailyResult
	if err := json.Unmarshal([]byte(dailyStdout), &daily); err != nil {
		t.Fatalf("decode project-local recovery daily: %v\n%s", err, dailyStdout)
	}
	if daily.FinalState != "maintenance-recovery-required" ||
		!daily.Blocked || daily.SessionLaunches != 0 || len(daily.HostRuns) != 0 {
		t.Fatalf("project-local recovery daily crossed zero-launch boundary: %+v", daily)
	}

	_, packsStderr, packsErr := runRekitExecutable(
		t,
		projectExecutable,
		"runtime",
		"-Command", "packs",
		"-Target", caseRoot,
		"-Format", "json",
	)
	if packsErr == nil ||
		!strings.Contains(packsStderr, "项目已安全停下") ||
		!strings.Contains(packsStderr, "请让 STeamAI") {
		t.Fatalf("project-local recovery packs err=%v stderr=%q", packsErr, packsStderr)
	}

	executableBytes, err := os.ReadFile(projectExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		projectExecutable,
		append(executableBytes, []byte("drifted recovery trailer\n")...),
		0o700,
	); err != nil {
		t.Fatal(err)
	}
	_, driftStderr, driftErr := runRekitExecutable(
		t,
		projectExecutable,
		"runtime",
		"-Command", "status",
		"-Target", caseRoot,
		"-Format", "compact-json",
	)
	if driftErr == nil || !strings.Contains(driftStderr, "bytes do not match") {
		t.Fatalf("drifted recovery executable err=%v stderr=%q", driftErr, driftStderr)
	}
}

func TestProjectLocalExecutableRecoveryCompactStatusFallsBackWhenTargetExceedsBudget(t *testing.T) {
	requireCurrentSyncApplyForProcessTest(t)
	parts := []string{t.TempDir()}
	for range 6 {
		parts = append(parts, strings.Repeat("&", 120))
	}
	caseRoot := filepath.Join(append(parts, "project")...)
	if err := os.MkdirAll(filepath.Dir(caseRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	encodedTarget, err := json.Marshal(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(encodedTarget) <= 4096 {
		t.Fatalf("deep recovery target fixture encoded to only %d bytes", len(encodedTarget))
	}

	repoRoot := rekitTestRepoRoot(t)
	preview, err := syncreview.InitPreview(
		repoRoot,
		caseRoot,
		"_template",
		syncreview.ApplyOptions{
			ProjectName:      "recovery-compact-overflow-before",
			CreateLocalFiles: true,
			Command:          "init",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(
		repoRoot,
		caseRoot,
		"_template",
		syncreview.ApplyOptions{
			ProjectName:        "recovery-compact-overflow-before",
			CreateLocalFiles:   true,
			Command:            "init",
			ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
		},
	); err != nil {
		t.Fatal(err)
	}
	sourceExecutable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	opt := syncreview.CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "recovery-compact-overflow",
		SourceExecutable: sourceExecutable,
	}
	plan, err := syncreview.CurrentSyncPreview(
		repoRoot,
		caseRoot,
		"_template",
		opt,
	)
	if err != nil {
		t.Fatal(err)
	}
	restore := syncreview.SetCurrentSyncApplyTransitionHookForTest(
		func(stage string, _ syncreview.CurrentSyncPlan) error {
			if stage == "after-operation-effect:activation-live-to-previous" {
				return errors.New("simulated compact overflow recovery interruption")
			}
			return nil
		},
	)
	_, applyErr := syncreview.CurrentSyncApply(
		caseRoot,
		"_template",
		syncreview.CurrentSyncApplyOptions{
			SourceRepoRoot:     repoRoot,
			ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
			CurrentSyncOptions: opt,
		},
	)
	restore()
	if applyErr == nil || !strings.Contains(
		applyErr.Error(),
		"simulated compact overflow recovery interruption",
	) {
		t.Fatalf("current sync compact overflow fixture did not interrupt: %v", applyErr)
	}

	projectExecutable := filepath.Join(
		caseRoot,
		".steamai",
		"runtime",
		"bin",
		runtimebundle.ExecutableName(),
	)
	stdout, stderr, runErr := runRekitExecutable(
		t,
		projectExecutable,
		"runtime",
		"-Command", "status",
		"-Format", "compact-json",
	)
	if runErr != nil || stderr != "" || len([]byte(stdout)) > 4096 ||
		!strings.HasSuffix(stdout, "\n") {
		t.Fatalf(
			"overflow recovery compact status err=%v stderr=%q bytes=%d stdout=%s",
			runErr,
			stderr,
			len([]byte(stdout)),
			stdout,
		)
	}
	var envelope struct {
		Command           string `json:"command"`
		SchemaVersion     int    `json:"schemaVersion"`
		IsMutation        bool   `json:"isMutation"`
		State             string `json:"state"`
		Blocked           bool   `json:"blocked"`
		DetailsRequired   bool   `json:"detailsRequired"`
		CommandExecutable bool   `json:"commandExecutable"`
		Reason            string `json:"reason"`
		FullDiagnostics   struct {
			Command                string `json:"command"`
			Format                 string `json:"format"`
			OnDemand               bool   `json:"onDemand"`
			ReuseOriginalSelectors bool   `json:"reuseOriginalSelectors"`
		} `json:"fullDiagnostics"`
		Boundary []string `json:"boundary"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode overflow recovery compact status: %v\n%s", err, stdout)
	}
	if envelope.Command != "status" || envelope.SchemaVersion != 1 ||
		envelope.IsMutation || envelope.State != "details-required" ||
		!envelope.Blocked || !envelope.DetailsRequired ||
		envelope.CommandExecutable ||
		envelope.Reason != "compact-output-budget-exceeded" ||
		envelope.FullDiagnostics.Command != "/steamai status -Format json" ||
		envelope.FullDiagnostics.Format != "json" ||
		!envelope.FullDiagnostics.OnDemand ||
		!envelope.FullDiagnostics.ReuseOriginalSelectors ||
		len(envelope.Boundary) == 0 {
		t.Fatalf("unexpected overflow recovery compact envelope: %+v", envelope)
	}
	for _, forbidden := range []string{
		"currentSyncRecovery",
		"case-maintenance-recovery",
		"applyArgs",
		plan.ExpectedPlanSHA256,
		`"target"`,
		`"currentDriverRequest"`,
		`"choices"`,
	} {
		if strings.Contains(stdout, forbidden) {
			t.Fatalf("overflow recovery compact envelope leaked %q: %s", forbidden, stdout)
		}
	}
}

func TestProjectLocalExecutableRejectsMaintenanceDuringRecoveryRoutes(t *testing.T) {
	requireCurrentSyncApplyForProcessTest(t)
	for _, test := range []struct {
		name  string
		stage string
		state string
	}{
		{name: "resume", stage: "after-initial-progress-publication", state: syncreview.CurrentSyncRecoveryResume},
		{name: "cleanup", stage: "before-terminal-cleanup", state: syncreview.CurrentSyncRecoveryCleanup},
	} {
		t.Run(test.name, func(t *testing.T) {
			repoRoot, caseRoot, projectExecutable := initProjectLocalProcessFixture(t, "maintenance-"+test.name+"-before")
			sourceExecutable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			opt := syncreview.CurrentSyncOptions{
				Command:          "sync",
				ProjectName:      "maintenance-" + test.name + "-after",
				SourceExecutable: sourceExecutable,
			}
			plan, err := syncreview.CurrentSyncPreview(repoRoot, caseRoot, "_template", opt)
			if err != nil {
				t.Fatal(err)
			}
			restore := syncreview.SetCurrentSyncApplyTransitionHookForTest(func(stage string, _ syncreview.CurrentSyncPlan) error {
				if stage == test.stage {
					return errors.New("simulated " + test.name + " interruption")
				}
				return nil
			})
			_, applyErr := syncreview.CurrentSyncApply(caseRoot, "_template", syncreview.CurrentSyncApplyOptions{
				SourceRepoRoot:     repoRoot,
				ExpectedPlanSHA256: plan.ExpectedPlanSHA256,
				CurrentSyncOptions: opt,
			})
			restore()
			if applyErr == nil || !strings.Contains(applyErr.Error(), "simulated "+test.name+" interruption") {
				t.Fatalf("current sync did not stop at %s: %v", test.stage, applyErr)
			}
			recovery, err := syncreview.InspectCurrentSyncRecovery(caseRoot)
			if err != nil || recovery.State != test.state {
				t.Fatalf("recovery state=%+v err=%v, want %s", recovery, err, test.state)
			}

			args := append([]string{"runtime"}, plan.ApplyArgs...)
			_, stderr, runErr := runRekitExecutable(t, projectExecutable, args...)
			if runErr == nil || !strings.Contains(stderr, "cannot execute current project maintenance") {
				t.Fatalf("project-local maintenance route=%s err=%v stderr=%q", test.name, runErr, stderr)
			}
			after, err := syncreview.InspectCurrentSyncRecovery(caseRoot)
			if err != nil || after.State != test.state || !after.Pending {
				t.Fatalf("rejected project-local maintenance changed recovery: %+v err=%v", after, err)
			}
		})
	}
}

func TestProjectLocalExecutableRejectsDifferentProjectTarget(t *testing.T) {
	_, _, executableA := initProjectLocalProcessFixture(t, "owner-a")
	_, projectB, _ := initProjectLocalProcessFixture(t, "owner-b")
	_, stderr, runErr := runRekitExecutable(
		t,
		executableA,
		"runtime",
		"-Command", "status",
		"-Target", projectB,
		"-Format", "compact-json",
	)
	if runErr == nil || !strings.Contains(stderr, "project-local STeamAI executable target mismatch") {
		t.Fatalf("project A executable accepted project B target: err=%v stderr=%q", runErr, stderr)
	}
}

func initProjectLocalProcessFixture(t *testing.T, projectName string) (string, string, string) {
	t.Helper()
	repoRoot := rekitTestRepoRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "project")
	preview, err := syncreview.InitPreview(repoRoot, caseRoot, "_template", syncreview.ApplyOptions{
		ProjectName:      projectName,
		CreateLocalFiles: true,
		Command:          "init",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(repoRoot, caseRoot, "_template", syncreview.ApplyOptions{
		ProjectName:        projectName,
		CreateLocalFiles:   true,
		Command:            "init",
		ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	}); err != nil {
		t.Fatal(err)
	}
	return repoRoot, caseRoot, filepath.Join(
		caseRoot,
		".steamai",
		"runtime",
		"bin",
		runtimebundle.ExecutableName(),
	)
}

func TestProjectLocalExecutableWithoutRecoveryFailsClosed(t *testing.T) {
	repoRoot := rekitTestRepoRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "project")
	preview, err := syncreview.InitPreview(
		repoRoot,
		caseRoot,
		"_template",
		syncreview.ApplyOptions{
			ProjectName:      "no-recovery-process",
			CreateLocalFiles: true,
			Command:          "init",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncreview.Apply(
		repoRoot,
		caseRoot,
		"_template",
		syncreview.ApplyOptions{
			ProjectName:        "no-recovery-process",
			CreateLocalFiles:   true,
			Command:            "init",
			ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
		},
	); err != nil {
		t.Fatal(err)
	}
	projectExecutable := filepath.Join(
		caseRoot,
		".steamai",
		"runtime",
		"bin",
		runtimebundle.ExecutableName(),
	)
	if err := os.Remove(filepath.Join(caseRoot, ".steamai", "instance.yml")); err != nil {
		t.Fatal(err)
	}
	_, stderr, runErr := runRekitExecutable(
		t,
		projectExecutable,
		"runtime",
		"-Command", "status",
		"-Target", caseRoot,
		"-Format", "compact-json",
	)
	if runErr == nil || !strings.Contains(stderr, "no pending durable transaction") {
		t.Fatalf("project-local executable without recovery err=%v stderr=%q", runErr, stderr)
	}
}

func runRekitExecutable(
	t *testing.T,
	executable string,
	args ...string,
) (string, string, error) {
	t.Helper()
	command := exec.Command(executable, args...)
	command.Env = append(os.Environ(), rekitExecutableHelperEnv+"=1")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

func rekitTestRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(cwd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func TestInvocationRecoveryTargetRequiresBoundedFrontDoor(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		args    []string
		want    string
		wantErr string
	}{
		{
			name: "runtime explicit target",
			mode: "runtime",
			args: []string{"-Command", "status", "-Target", "project"},
			want: "project",
		},
		{
			name: "runtime owner target default",
			mode: "runtime",
			args: []string{"-Command", "status"},
			want: "",
		},
		{
			name: "daily host",
			mode: "host",
			args: []string{"-daily", "-target", "project"},
			want: "project",
		},
		{
			name: "daily host owner target default",
			mode: "host",
			args: []string{"-daily"},
			want: "",
		},
		{
			name:    "ordinary host blocked",
			mode:    "host",
			args:    []string{"-target", "project"},
			wantErr: "only the daily host front door",
		},
		{
			name:    "internal host blocked",
			mode:    "host",
			args:    []string{"-internal-supervisor", "spec.json", "-target", "project"},
			wantErr: "only the daily host front door",
		},
		{
			name:    "duplicate target blocked",
			mode:    "host",
			args:    []string{"-daily", "-target", "one", "--target=two"},
			wantErr: "multiple -target",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := invocationRecoveryTarget(test.mode, test.args)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("invocationRecoveryTarget error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("invocationRecoveryTarget = %q, err=%v, want %q", got, err, test.want)
			}
		})
	}
}

func TestInvocationModeRoutesUnifiedExecutable(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantMode string
		wantArgs []string
		wantErr  string
	}{
		{name: "legacy runtime", args: []string{"-Command", "status"}, wantMode: "runtime", wantArgs: []string{"-Command", "status"}},
		{name: "explicit runtime", args: []string{"runtime", "-Command", "status"}, wantMode: "runtime", wantArgs: []string{"-Command", "status"}},
		{name: "host", args: []string{"host", "-daily", "-target", "project"}, wantMode: "host", wantArgs: []string{"-daily", "-target", "project"}},
		{name: "internal child", args: []string{"-internal-supervisor", "spec.json", "-internal-supervisor-sha256", strings.Repeat("a", 64)}, wantMode: "host", wantArgs: []string{"-internal-supervisor", "spec.json", "-internal-supervisor-sha256", strings.Repeat("a", 64)}},
		{name: "unknown", args: []string{"future"}, wantErr: "unknown steamai mode"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mode, args, err := invocationMode(test.args)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("invocationMode error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil || mode != test.wantMode || !reflect.DeepEqual(args, test.wantArgs) {
				t.Fatalf("invocationMode = mode=%q args=%v err=%v", mode, args, err)
			}
		})
	}
}
