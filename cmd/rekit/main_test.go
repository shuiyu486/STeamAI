package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
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
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	restoreExecutableSource := runtimebundle.SetExecutableSourceForTest(executable)
	restoreRuntimeBuilders := syncreview.SetRuntimeBundleBuildersForTest(runtimebundle.BuildWithExecutable)
	code := m.Run()
	restoreRuntimeBuilders()
	restoreExecutableSource()
	os.Exit(code)
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
		envelope.FullDiagnostics.Command != "/steamai status --diagnostics" ||
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

func TestUnifiedRuntimeInitPublishesRunnableProjectExecutable(t *testing.T) {
	repoRoot := rekitTestRepoRoot(t)
	name := "steamai-runtime-init-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	unifiedExecutable := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", unifiedExecutable, "./cmd/rekit")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build unified runtime init fixture: %v\n%s", err, output)
	}

	caseRoot := filepath.Join(t.TempDir(), "project")
	previewOut, previewErrOut, previewErr := runRekitExecutable(
		t,
		unifiedExecutable,
		"runtime",
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-ProjectName", "runtime-init",
		"-WhatIf",
		"-Format", "json",
	)
	if previewErr != nil || previewErrOut != "" {
		t.Fatalf("runtime init preview err=%v stderr=%q stdout=%s", previewErr, previewErrOut, previewOut)
	}
	var preview struct {
		ExpectedPlanSHA256 string   `json:"expectedPlanSha256"`
		ApplyArgs          []string `json:"applyArgs"`
	}
	if err := json.Unmarshal([]byte(previewOut), &preview); err != nil || len(preview.ExpectedPlanSHA256) != 64 || len(preview.ApplyArgs) == 0 {
		t.Fatalf("runtime init preview omitted exact Apply: err=%v preview=%+v stdout=%s", err, preview, previewOut)
	}
	applyOut, applyErrOut, applyErr := runRekitExecutable(t, unifiedExecutable, append([]string{"runtime"}, preview.ApplyArgs...)...)
	if applyErr != nil || applyErrOut != "" || !strings.Contains(applyOut, `"applied": true`) {
		t.Fatalf("runtime init Apply err=%v stderr=%q stdout=%s", applyErr, applyErrOut, applyOut)
	}

	projectExecutable := filepath.Join(caseRoot, ".steamai", "runtime", "bin", runtimebundle.ExecutableName())
	if err := runtimebundle.ValidateUnifiedExecutableRole(projectExecutable); err != nil {
		t.Fatalf("runtime init published executable role: %v", err)
	}
	centralBytes, err := os.ReadFile(unifiedExecutable)
	if err != nil {
		t.Fatal(err)
	}
	projectBytes, err := os.ReadFile(projectExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(projectBytes, centralBytes) {
		t.Fatal("runtime init did not publish the invoking unified executable bytes")
	}
}

func TestUnifiedBootstrapPublishesRunnableProjectWithoutAutoResume(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("source bootstrap mutation is Windows-only")
	}
	repoRoot := rekitTestRepoRoot(t)
	unifiedExecutable := filepath.Join(repoRoot, "steamai-bootstrap-test.exe")
	t.Cleanup(func() { _ = os.Remove(unifiedExecutable) })
	build := exec.Command("go", "build", "-o", unifiedExecutable, "./cmd/rekit")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build unified bootstrap fixture: %v\n%s", err, output)
	}

	caseRoot := filepath.Join(t.TempDir(), "existing-project")
	if err := os.Mkdir(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	keepPath := filepath.Join(caseRoot, "keep.txt")
	if err := os.WriteFile(keepPath, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	goal := "inspect this existing project"
	choicesOut, choicesErrOut, choicesErr := runRekitExecutableWithInput(
		t, unifiedExecutable, "",
		"bootstrap", "-target", caseRoot, "-goal", goal,
	)
	if choicesErr != nil || choicesErrOut != "" || !strings.Contains(choicesOut, `"state": "pack-selection-required"`) ||
		!strings.Contains(choicesOut, `"id": "binary-re"`) || !strings.Contains(choicesOut, `"id": "web-security"`) || strings.Contains(choicesOut, `"id": "_template"`) {
		t.Fatalf("bootstrap pack choices err=%v stderr=%q stdout=%s", choicesErr, choicesErrOut, choicesOut)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(err) {
		t.Fatalf("bootstrap pack choices mutated target: %v", err)
	}

	_, rejectedPackErrOut, rejectedPackErr := runRekitExecutableWithInput(
		t, unifiedExecutable, "APPLY\n",
		"bootstrap", "-target", caseRoot, "-goal", goal, "-pack", "_template",
	)
	if rejectedPackErr == nil || !strings.Contains(rejectedPackErrOut, "not a schema-valid mature selectable pack") {
		t.Fatalf("bootstrap admitted a non-mature pack: err=%v stderr=%q", rejectedPackErr, rejectedPackErrOut)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(err) {
		t.Fatalf("rejected bootstrap pack mutated target: %v", err)
	}

	cancelOut, cancelErrOut, cancelErr := runRekitExecutableWithInput(
		t, unifiedExecutable, "cancel\n",
		"bootstrap", "-target", caseRoot, "-goal", goal, "-pack", "binary-re",
	)
	if cancelErr != nil || !strings.Contains(cancelOut, `"state": "bootstrap-cancelled"`) || !strings.Contains(cancelErrOut, "Type APPLY") {
		t.Fatalf("bootstrap cancellation err=%v stderr=%q stdout=%s", cancelErr, cancelErrOut, cancelOut)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(err) {
		t.Fatalf("cancelled bootstrap mutated target: %v", err)
	}

	previewOut, previewErrOut, previewErr := runRekitExecutableWithInput(
		t, unifiedExecutable, "APPLY\n",
		"bootstrap", "-target", caseRoot, "-goal", goal, "-pack", "binary-re", "-format", "json",
	)
	if previewErr != nil || previewErrOut != "" {
		t.Fatalf("bootstrap JSON preview err=%v stderr=%q stdout=%s", previewErr, previewErrOut, previewOut)
	}
	var preview struct {
		State                string `json:"state"`
		Applied              bool   `json:"applied"`
		RequiresConfirmation bool   `json:"requiresConfirmation"`
		NoAutoResume         bool   `json:"noAutoResume"`
	}
	if err := json.Unmarshal([]byte(previewOut), &preview); err != nil {
		t.Fatalf("decode bootstrap preview: %v\n%s", err, previewOut)
	}
	if preview.State != sessionhost.DailyActionConfirmationRequired || preview.Applied || !preview.RequiresConfirmation || !preview.NoAutoResume {
		t.Fatalf("bootstrap JSON mode crossed preview-only boundary: %+v", preview)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai")); !os.IsNotExist(err) {
		t.Fatalf("bootstrap JSON preview mutated target: %v", err)
	}

	applyOut, applyErrOut, applyErr := runRekitExecutableWithInput(
		t, unifiedExecutable, "APPLY\n",
		"bootstrap", "-target", caseRoot, "-goal", goal, "-pack", "binary-re",
	)
	if applyErr != nil {
		t.Fatalf("bootstrap Apply err=%v stderr=%q stdout=%s", applyErr, applyErrOut, applyOut)
	}
	var applied struct {
		State        string `json:"state"`
		Applied      bool   `json:"applied"`
		NoAutoResume bool   `json:"noAutoResume"`
		Continuation struct {
			Executable             string   `json:"executable"`
			ExecutableSHA256       string   `json:"executableSha256"`
			ExecutableBytes        int64    `json:"executableBytes"`
			BundleManifest         string   `json:"bundleManifest"`
			BundleManifestSHA256   string   `json:"bundleManifestSha256"`
			Arguments              []string `json:"arguments"`
			Goal                   string   `json:"goal"`
			RequiresExplicitChoice bool     `json:"requiresExplicitChoice"`
			NoAutoResume           bool     `json:"noAutoResume"`
		} `json:"continuation"`
		Apply struct {
			SessionLaunches int `json:"sessionLaunches"`
		} `json:"apply"`
	}
	if err := json.Unmarshal([]byte(applyOut), &applied); err != nil {
		t.Fatalf("decode bootstrap Apply: %v\n%s\nstderr=%s", err, applyOut, applyErrOut)
	}
	projectExecutable := filepath.Join(caseRoot, ".steamai", "runtime", "bin", runtimebundle.ExecutableName())
	if applied.State != sessionhost.DailyActionReadyToContinue || !applied.Applied || !applied.NoAutoResume ||
		!applied.Continuation.RequiresExplicitChoice || !applied.Continuation.NoAutoResume || applied.Continuation.Goal != goal ||
		len(applied.Continuation.ExecutableSHA256) != 64 || applied.Continuation.ExecutableBytes <= 0 ||
		applied.Continuation.BundleManifest != "runtime/manifest.json" || len(applied.Continuation.BundleManifestSHA256) != 64 ||
		!reflect.DeepEqual(applied.Continuation.Arguments, []string{"host", "-daily", "-goal", goal}) ||
		!strings.EqualFold(filepath.Clean(applied.Continuation.Executable), filepath.Clean(projectExecutable)) ||
		applied.Apply.SessionLaunches != 0 {
		t.Fatalf("bootstrap Apply did not preserve bounded continuation: %+v", applied)
	}
	if err := runtimebundle.ValidateUnifiedExecutableRole(projectExecutable); err != nil {
		t.Fatalf("bootstrap published executable role: %v", err)
	}
	projectBytes, err := os.ReadFile(projectExecutable)
	if err != nil {
		t.Fatal(err)
	}
	sourceBytes, err := os.ReadFile(unifiedExecutable)
	if err != nil {
		t.Fatal(err)
	}
	if len(projectBytes) != int(applied.Continuation.ExecutableBytes) || !bytes.Equal(projectBytes, sourceBytes) {
		t.Fatal("bootstrap continuation executable binding differs from the exact published unified image")
	}
	keep, err := os.ReadFile(keepPath)
	if err != nil || string(keep) != "keep\n" {
		t.Fatalf("bootstrap changed ordinary project file: %q err=%v", keep, err)
	}
	if strings.Contains(strings.ToLower(applyOut), "powershell") {
		t.Fatalf("bootstrap result introduced a PowerShell runtime path: %s", applyOut)
	}

	_, projectBootstrapErr, projectBootstrapRunErr := runRekitExecutableWithInput(
		t, projectExecutable, "APPLY\n",
		"bootstrap", "-target", filepath.Join(t.TempDir(), "other"), "-goal", "other", "-pack", "binary-re",
	)
	if projectBootstrapRunErr == nil || !strings.Contains(projectBootstrapErr, "cannot bootstrap another directory") {
		t.Fatalf("project-local executable admitted external bootstrap: err=%v stderr=%q", projectBootstrapRunErr, projectBootstrapErr)
	}
}

func TestUnifiedHostFreshGoalPublishesRunnableProjectExecutable(t *testing.T) {
	repoRoot := rekitTestRepoRoot(t)
	name := "steamai-fresh-goal-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	unifiedExecutable := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", unifiedExecutable, "./cmd/rekit")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build unified fresh-goal fixture: %v\n%s", err, output)
	}

	caseRoot := filepath.Join(t.TempDir(), "project")
	stdout, stderr, runErr := runRekitExecutable(
		t,
		unifiedExecutable,
		"host",
		"-daily",
		"-target", caseRoot,
		"-goal", "inspect this fresh project",
		"-lane", "missing-lane",
	)
	if runErr == nil || !strings.Contains(stderr, "board.json") ||
		!strings.Contains(stdout, `"onboardingApplied": true`) ||
		!strings.Contains(stdout, `"sessionLaunches": 0`) {
		t.Fatalf("fresh-goal host did not initialize and stop before Claude: err=%v stderr=%q stdout=%s", runErr, stderr, stdout)
	}

	projectExecutable := filepath.Join(
		caseRoot,
		".steamai",
		"runtime",
		"bin",
		runtimebundle.ExecutableName(),
	)
	if err := runtimebundle.ValidateUnifiedExecutableRole(projectExecutable); err != nil {
		t.Fatalf("published project executable role: %v\nhost stdout=%s\nhost stderr=%s", err, stdout, stderr)
	}
	helpOut, helpErrOut, helpErr := runRekitExecutable(t, projectExecutable, "help")
	if helpErr != nil || helpErrOut != "" || !strings.Contains(helpOut, "STeamAI public commands:") {
		t.Fatalf("published project help err=%v stderr=%q stdout=%s", helpErr, helpErrOut, helpOut)
	}
	statusOut, statusErrOut, statusErr := runRekitExecutable(t, projectExecutable, "status")
	if statusErr != nil || statusErrOut != "" || !strings.Contains(statusOut, "现在：") || !strings.Contains(statusOut, "下一步：") {
		t.Fatalf("published project status err=%v stderr=%q stdout=%s", statusErr, statusErrOut, statusOut)
	}
}

func TestProjectLocalExecutablePublicHelpStatusAndDiagnostics(t *testing.T) {
	_, _, executable := initProjectLocalProcessFixture(t, "public-frontdoor")

	helpOut, helpErrOut, helpErr := runRekitExecutable(t, executable, "help")
	if helpErr != nil || helpErrOut != "" || !strings.Contains(helpOut, "STeamAI public commands:") || strings.Contains(helpOut, "internal-supervisor") {
		t.Fatalf("project-local public help err=%v stderr=%q stdout=%s", helpErr, helpErrOut, helpOut)
	}

	statusOut, statusErrOut, statusErr := runRekitExecutable(t, executable, "status")
	if statusErr != nil || statusErrOut != "" || !strings.Contains(statusOut, "现在：") || !strings.Contains(statusOut, "下一步：") || strings.Contains(statusOut, `"runtimeRoot"`) || strings.Contains(statusOut, "currentDriverRequestSha256") || strings.Contains(statusOut, "/rekit") {
		t.Fatalf("project-local public status err=%v stderr=%q stdout=%s", statusErr, statusErrOut, statusOut)
	}

	diagnosticsOut, diagnosticsErrOut, diagnosticsErr := runRekitExecutable(t, executable, "status", "--diagnostics")
	if diagnosticsErr != nil || diagnosticsErrOut != "" || !strings.Contains(diagnosticsOut, `"command": "status"`) || !strings.Contains(diagnosticsOut, `"missionControlRunbook"`) || strings.Contains(diagnosticsOut, `"/rekit `) {
		t.Fatalf("project-local public diagnostics err=%v stderr=%q stdout=%s", diagnosticsErr, diagnosticsErrOut, diagnosticsOut)
	}

	rejectedOut, rejectedErrOut, rejectedErr := runRekitExecutable(t, executable, "continue", "--apply")
	if rejectedErr == nil || rejectedOut != "" || !strings.Contains(rejectedErrOut, "命令未执行") || !strings.Contains(rejectedErrOut, "steamai help") || strings.Contains(rejectedErrOut, "--apply") {
		t.Fatalf("project-local public continue accepted or leaked internal usage detail: err=%v stdout=%q stderr=%q", rejectedErr, rejectedOut, rejectedErrOut)
	}

	diagnosticFailureOut, diagnosticFailureErrOut, diagnosticFailureErr := runRekitExecutable(t, executable, "continue", "--diagnostics", "--apply")
	if diagnosticFailureErr == nil || diagnosticFailureErrOut != "" {
		t.Fatalf("public diagnostics failure did not use stdout only: err=%v stderr=%q stdout=%s", diagnosticFailureErr, diagnosticFailureErrOut, diagnosticFailureOut)
	}
	var failure struct {
		Kind        string `json:"kind"`
		Command     string `json:"command"`
		ExitCode    int    `json:"exitCode"`
		Diagnostics struct {
			Code             string `json:"code"`
			MutationApplied  bool   `json:"mutationApplied"`
			MutationBoundary string `json:"mutationBoundary"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(diagnosticFailureOut), &failure); err != nil {
		t.Fatalf("decode public diagnostics failure: %v\n%s", err, diagnosticFailureOut)
	}
	if failure.Kind != "steamai-public-failure" || failure.Command != "continue" || failure.ExitCode != 2 || failure.Diagnostics.Code != "public-usage-invalid" || failure.Diagnostics.MutationApplied || failure.Diagnostics.MutationBoundary != "none" {
		t.Fatalf("unexpected public diagnostics failure: %+v", failure)
	}
}

func TestRuntimePlanFailureJSONMatchesProcessExitCode(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	caseRoot := filepath.Join(t.TempDir(), "fresh-project")
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, runErr := runRekitExecutable(
		t,
		executable,
		"runtime",
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Format", "json",
	)
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) || stderr != "" {
		t.Fatalf("runtime plan failure did not use a clean process exit: err=%v stderr=%q stdout=%s", runErr, stderr, stdout)
	}
	var failure struct {
		Kind        string `json:"kind"`
		Command     string `json:"command"`
		ExitCode    int    `json:"exitCode"`
		Diagnostics struct {
			Code            string `json:"code"`
			MutationApplied bool   `json:"mutationApplied"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(stdout), &failure); err != nil {
		t.Fatalf("decode runtime plan failure: %v\n%s", err, stdout)
	}
	if failure.Kind != "steamai-public-failure" || failure.Command != "init" || failure.ExitCode != exitErr.ExitCode() || failure.ExitCode != 2 || failure.Diagnostics.Code != "plan-binding-missing" || failure.Diagnostics.MutationApplied {
		t.Fatalf("runtime process exit and typed failure drifted: exit=%d failure=%+v", exitErr.ExitCode(), failure)
	}
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
	return runRekitExecutableWithInput(t, executable, "", args...)
}

func runRekitExecutableWithInput(
	t *testing.T,
	executable,
	input string,
	args ...string,
) (string, string, error) {
	t.Helper()
	command := exec.Command(executable, args...)
	command.Env = append(os.Environ(), rekitExecutableHelperEnv+"=1")
	command.Stdin = strings.NewReader(input)
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

func TestBootstrapSourceRepoRootDoesNotBindAncestorProject(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "parent-project", "nested-source")
	for _, dir := range []string{
		filepath.Join(repo, "packs"),
		filepath.Join(repo, "cmd", "rekit"),
		filepath.Join(filepath.Dir(repo), ".steamai"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module github.com/shuiyu486/re-context-kits\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(repo, "bin", "steamai-bootstrap.exe")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := bootstrapSourceRepoRoot(executable)
	if err != nil || !rekitfs.SamePath(got, repo) {
		t.Fatalf("bootstrap source root = %q err=%v, want %q", got, err, repo)
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
		{name: "bootstrap", args: []string{"bootstrap", "-target", "project", "-goal", "inspect"}, wantMode: "bootstrap", wantArgs: []string{"-target", "project", "-goal", "inspect"}},
		{name: "uppercase bootstrap", args: []string{"BOOTSTRAP", "-target", "project"}, wantErr: "bootstrap mode token must be exactly"},
		{name: "public help", args: []string{"help"}, wantMode: "public", wantArgs: []string{"help"}},
		{name: "public short help", args: []string{"--help"}, wantMode: "public", wantArgs: []string{"help"}},
		{name: "public status", args: []string{"status", "--diagnostics"}, wantMode: "public", wantArgs: []string{"status", "--diagnostics"}},
		{name: "public continue", args: []string{"continue", "--lane", "main"}, wantMode: "public", wantArgs: []string{"continue", "--lane", "main"}},
		{name: "host", args: []string{"host", "-daily", "-target", "project"}, wantMode: "host", wantArgs: []string{"-daily", "-target", "project"}},
		{name: "uppercase host", args: []string{"HOST", "-daily", "-target", "project"}, wantErr: "host mode token must be exactly"},
		{name: "internal child", args: []string{"-internal-supervisor", "spec.json", "-internal-supervisor-sha256", strings.Repeat("a", 64)}, wantMode: "host", wantArgs: []string{"-internal-supervisor", "spec.json", "-internal-supervisor-sha256", strings.Repeat("a", 64)}},
		{name: "private adapter child", args: []string{"-child-vmp-ida-index-inspector"}, wantMode: "adapter", wantArgs: []string{"-child-vmp-ida-index-inspector"}},
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
