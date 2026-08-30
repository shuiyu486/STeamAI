package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestParsePublicOptions(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    publicOptions
		wantErr string
	}{
		{name: "empty shows help", args: nil, want: publicOptions{Command: "help", Mode: publicOutputInteraction}},
		{name: "status", args: []string{"status"}, want: publicOptions{Command: "status", Mode: publicOutputInteraction}},
		{name: "status diagnostics", args: []string{"status", "--diagnostics", "--target", "project"}, want: publicOptions{Command: "status", Target: "project", Mode: publicOutputDiagnostics}},
		{name: "continue lane", args: []string{"continue", "--lane", "authz"}, want: publicOptions{Command: "continue", Lane: "authz", Mode: publicOutputInteraction}},
		{name: "short help", args: []string{"--help"}, want: publicOptions{Command: "help", Mode: publicOutputInteraction}},
		{name: "format json is diagnostics", args: []string{"status", "--format=json"}, want: publicOptions{Command: "status", Mode: publicOutputDiagnostics}},
		{name: "reject internal command flag", args: []string{"status", "-Format", "json"}, wantErr: "does not accept"},
		{name: "reject apply", args: []string{"continue", "--apply"}, wantErr: "does not accept"},
		{name: "reject lane on status", args: []string{"status", "--lane", "main"}, wantErr: "only by public continue"},
		{name: "reject non-json format", args: []string{"status", "--format", "text"}, wantErr: "supports only --format json"},
		{name: "reject empty target", args: []string{"status", "--target="}, wantErr: "non-empty --target"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parsePublicOptions(test.args)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("parsePublicOptions error=%v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("parsePublicOptions=%+v, want %+v", got, test.want)
			}
		})
	}
}

func TestPublicInvocationRecognizesOnlyUserCommands(t *testing.T) {
	for _, test := range []struct {
		args []string
		want bool
	}{
		{args: []string{"help"}, want: true},
		{args: []string{"status"}, want: true},
		{args: []string{"continue"}, want: true},
		{args: []string{"--help"}, want: true},
		{args: []string{"runtime", "-Command", "status"}, want: false},
		{args: []string{"-Command", "status"}, want: false},
		{args: []string{"host", "-daily"}, want: false},
		{args: []string{"future"}, want: false},
	} {
		if got := PublicInvocation(test.args); got != test.want {
			t.Errorf("PublicInvocation(%v)=%t, want %t", test.args, got, test.want)
		}
	}
}

func TestRunPublicHelpDoesNotDispatchRuntime(t *testing.T) {
	var out bytes.Buffer
	if err := RunPublic([]string{"help"}, &out, "missing-project-root"); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{
		"STeamAI public commands:\n",
		"  steamai status",
		"\n  steamai continue",
		"\nstatus is read-only.",
		"continue creates a fresh preview only",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("help output missing %q: %s", expected, text)
		}
	}
}

func TestPublicSummaryKeepsDiagnosticsOutOfDefaultOutput(t *testing.T) {
	var status bytes.Buffer
	statusRaw := json.RawMessage(`{
		"mode":"case",
		"pack":"binary-re",
		"caseMission":{"summary":"ready=1 blocked=0"},
		"missionControlRunbook":{"currentDriverRequest":{
			"state":"refresh-required",
			"command":"/steamai continue -Lane main -WhatIf -Format json",
			"expectedReceipt":{"refreshStatusCommand":"/steamai status"}
		}}
	}`)
	if err := writePublicInteraction(&status, "status", statusRaw); err != nil {
		t.Fatal(err)
	}
	statusText := status.String()
	if !strings.Contains(statusText, "现在：") || !strings.Contains(statusText, "下一步：") {
		t.Fatalf("status summary is not user-facing: %s", statusText)
	}
	if strings.Contains(statusText, "-Lane") || strings.Contains(statusText, "refresh-required") || strings.Contains(statusText, "binary-re") {
		t.Fatalf("status summary leaked typed diagnostics: %s", statusText)
	}

	var continuation bytes.Buffer
	continueRaw := json.RawMessage(`{
		"lane":{"id":"feature-authz","label":"authz"},
		"continuePlanSha256":"` + strings.Repeat("a", 64) + `",
		"blocked":false
	}`)
	if err := writePublicInteraction(&continuation, "continue", continueRaw); err != nil {
		t.Fatal(err)
	}
	continueText := continuation.String()
	if !strings.Contains(continueText, "已完成继续前的检查") || !strings.Contains(continueText, "不会自动更改项目") {
		t.Fatalf("continue summary omitted safety contract: %s", continueText)
	}
	if !strings.Contains(continueText, "\n原因：") || !strings.Contains(continueText, "\n下一步：") {
		t.Fatalf("continue summary is not split into user-facing lines: %s", continueText)
	}
	if strings.Contains(continueText, "authz") || strings.Contains(continueText, "feature-authz") || strings.Contains(continueText, strings.Repeat("a", 64)) {
		t.Fatalf("continue summary leaked durable or typed diagnostics: %s", continueText)
	}
}

func TestRunPublicStatusShowsFreshOnboardingChoices(t *testing.T) {
	caseRoot := t.TempDir()
	var out bytes.Buffer
	if err := RunPublic([]string{"status", "--target", caseRoot}, &out, ""); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"项目尚未完成首次接入", "binary-re，可直接使用，推荐", "web-security，可直接使用", "ctf，功能骨架，不可直接选择"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("fresh public status omitted %q: %s", expected, text)
		}
	}
	for _, internal := range []string{"packChoices", "selectedPack", `"runtimeRoot"`, "recommended", "selectable"} {
		if strings.Contains(text, internal) {
			t.Fatalf("fresh public status leaked internal field %q: %s", internal, text)
		}
	}

	out.Reset()
	if err := RunPublic([]string{"status", "--diagnostics", "--target", caseRoot}, &out, ""); err != nil {
		t.Fatal(err)
	}
	var diagnostics struct {
		Onboarding struct {
			PackChoices []struct {
				ID         string `json:"id"`
				Maturity   string `json:"maturity"`
				Selectable bool   `json:"selectable"`
			} `json:"packChoices"`
		} `json:"onboarding"`
	}
	if err := json.Unmarshal(out.Bytes(), &diagnostics); err != nil {
		t.Fatal(err)
	}
	selectable := map[string]bool{}
	for _, choice := range diagnostics.Onboarding.PackChoices {
		selectable[choice.ID] = choice.Selectable
		if choice.Maturity != "mature" && choice.Selectable {
			t.Fatalf("non-mature pack is selectable during onboarding: %+v", choice)
		}
	}
	if !selectable["binary-re"] || !selectable["web-security"] || selectable["ctf"] {
		t.Fatalf("unexpected public onboarding selection policy: %+v", diagnostics.Onboarding.PackChoices)
	}
}

func TestRunPublicStatusRequiresGoalForInitializedCurrentProject(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	runInitApplyFromPreview(
		t,
		&out,
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-ProjectName", "public-goal-required",
	)
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	before := snapshotFiles(t, stateRoot)

	out.Reset()
	if err := RunPublic([]string{"status", "--target", caseRoot}, &out, ""); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"项目已接入", "缺少当前任务目标", "这个任务的目标", "已固定使用 _template pack", "status 不会自动写入项目"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("initialized current status omitted %q: %s", expected, text)
		}
	}
	for _, forbidden := range []string{"选择一个 pack", "任务面板", "overview", "-Apply", "/rekit", caseRoot} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("initialized current status leaked or suggested %q: %s", forbidden, text)
		}
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, stateRoot))
	if _, err := os.Stat(filepath.Join(stateRoot, "board.json")); !os.IsNotExist(err) {
		t.Fatalf("read-only current status initialized board: %v", err)
	}

	out.Reset()
	if err := RunPublic([]string{"status", "--diagnostics", "--target", caseRoot}, &out, ""); err != nil {
		t.Fatal(err)
	}
	var diagnostics struct {
		Mode       string `json:"mode"`
		Onboarding struct {
			State        string `json:"state"`
			SelectedPack string `json:"selectedPack"`
			PackChoices  []any  `json:"packChoices"`
		} `json:"onboarding"`
		MissionControlRunbook struct {
			CurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
		} `json:"missionControlRunbook"`
	}
	if err := json.Unmarshal(out.Bytes(), &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Mode != "case-onboarding-required" || diagnostics.Onboarding.State != "absent" || diagnostics.Onboarding.SelectedPack != "_template" || len(diagnostics.Onboarding.PackChoices) != 0 || diagnostics.MissionControlRunbook.CurrentDriverRequest != nil {
		t.Fatalf("initialized current diagnostics reopened selection or bootstrap: %+v", diagnostics)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, stateRoot))
}

func TestRunPublicStatusFailsClosedWhenCurrentMissionIntentIsMissingWithBoard(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	runInitApplyFromPreview(
		t,
		&out,
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-ProjectName", "public-intent-conflict",
	)
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	if err := os.WriteFile(filepath.Join(stateRoot, "board.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, stateRoot)

	out.Reset()
	if err := RunPublic([]string{"status", "--target", caseRoot}, &out, ""); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"任务信息缺失", "已经有任务面板", "完整状态"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("current lifecycle conflict omitted %q: %s", expected, text)
		}
	}
	for _, forbidden := range []string{"选择一个 pack", "已固定使用", "可以继续推进", "-Apply", "/rekit", caseRoot} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("current lifecycle conflict leaked or suggested %q: %s", forbidden, text)
		}
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, stateRoot))

	out.Reset()
	if err := RunPublic([]string{"status", "--diagnostics", "--target", caseRoot}, &out, ""); err != nil {
		t.Fatal(err)
	}
	var diagnostics struct {
		Mode       string `json:"mode"`
		Onboarding struct {
			State        string `json:"state"`
			SelectedPack string `json:"selectedPack"`
			PackChoices  []any  `json:"packChoices"`
		} `json:"onboarding"`
		MissionControlRunbook struct {
			CurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
		} `json:"missionControlRunbook"`
	}
	if err := json.Unmarshal(out.Bytes(), &diagnostics); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Mode != "case-onboarding-conflict" || diagnostics.Onboarding.State != "absent" || diagnostics.Onboarding.SelectedPack != "_template" || len(diagnostics.Onboarding.PackChoices) != 0 || diagnostics.MissionControlRunbook.CurrentDriverRequest != nil {
		t.Fatalf("current lifecycle conflict published a route: %+v", diagnostics)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, stateRoot))
}

func TestRunPublicStatusExplainsCommittedMissingBoard(t *testing.T) {
	fixture := publicEntrypointProductFixtures()[0]
	caseRoot := publicEntrypointProductCase(t, fixture, "public-missing-board")
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	before := snapshotFiles(t, stateRoot)

	var out bytes.Buffer
	if err := RunPublic([]string{"status", "--target", caseRoot}, &out, ""); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"任务面板尚未建立", "当前项目需要先建立任务面板", "唯一的初始化步骤", "查看状态本身不会写入项目"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing-board public status omitted %q: %s", expected, text)
		}
	}
	for _, forbidden := range []string{"case-board-missing", "start -Apply", "-Expected", "/rekit", caseRoot} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("missing-board public status leaked %q: %s", forbidden, text)
		}
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, stateRoot))
	if _, err := os.Stat(filepath.Join(stateRoot, "board.json")); !os.IsNotExist(err) {
		t.Fatalf("public status initialized missing board: %v", err)
	}

	out.Reset()
	if err := RunPublic([]string{"status", "--diagnostics", "--target", caseRoot}, &out, ""); err != nil {
		t.Fatal(err)
	}
	var diagnostics struct {
		Onboarding struct {
			State       string `json:"state"`
			PackChoices []any  `json:"packChoices"`
		} `json:"onboarding"`
		MissionControlRunbook struct {
			CurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
		} `json:"missionControlRunbook"`
	}
	if err := json.Unmarshal(out.Bytes(), &diagnostics); err != nil {
		t.Fatal(err)
	}
	request := diagnostics.MissionControlRunbook.CurrentDriverRequest
	if request == nil || request.State != "case-board-missing" || !request.CommandExecutable || request.Blocked || request.Invocation == nil || request.Invocation.Command != commands.Overview || !strings.HasPrefix(request.Command, commands.CurrentPublicEntrypoint+" overview ") || len(diagnostics.Onboarding.PackChoices) != 0 {
		t.Fatalf("missing-board diagnostics omitted its unique typed bootstrap: onboarding=%+v request=%+v", diagnostics.Onboarding, request)
	}
}

func TestPublicStatusSummaryExplainsFreshOnboardingChoices(t *testing.T) {
	var out bytes.Buffer
	raw := json.RawMessage(`{
		"mode":"case-onboarding-required",
		"caseMission":{"summary":"project onboarding requires a goal and pack selection"},
		"onboarding":{
			"state":"absent",
			"packChoices":[
				{"id":"binary-re","name":"binary-re","maturity":"mature","recommended":true,"selectable":true},
				{"id":"web-security","name":"web-security","maturity":"mature","selectable":true},
				{"id":"ctf","name":"ctf","maturity":"skeleton","selectable":false}
			]
		},
		"missionControlRunbook":{}
	}`)
	if err := writePublicInteraction(&out, "status", raw); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"项目尚未完成首次接入", "告诉主 Agent 你的目标并选择一个 pack", "binary-re，可直接使用，推荐", "web-security，可直接使用", "ctf，功能骨架，不可直接选择"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("fresh onboarding summary omitted %q: %s", expected, text)
		}
	}
	for _, internal := range []string{"packChoices", "selectedPack", "mature", "recommended", "selectable"} {
		if strings.Contains(text, internal) {
			t.Fatalf("fresh onboarding summary leaked internal choice detail %q: %s", internal, text)
		}
	}
}

func TestPublicStatusSummaryHandlesDetailsRequiredEnvelope(t *testing.T) {
	var out bytes.Buffer
	raw := json.RawMessage(`{
		"command":"status",
		"state":"details-required",
		"blocked":true,
		"detailsRequired":true,
		"commandExecutable":false,
		"reason":"compact-output-budget-exceeded",
		"fullDiagnostics":{"command":"/steamai status -Format json","format":"json"}
	}`)
	if err := writePublicInteraction(&out, "status", raw); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"完整情况暂时无法", "当前下一步无法", "完整状态", "不要猜测"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("details-required summary omitted %q: %s", expected, text)
		}
	}
	for _, forbidden := range []string{"compact-output-budget-exceeded", "-Format json", "commandExecutable", "fullDiagnostics"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("details-required summary leaked %q: %s", forbidden, text)
		}
	}
}

func TestPublicDiagnosticsPreservesTypedJSON(t *testing.T) {
	var out bytes.Buffer
	const largeGeneration = "9007199254740993"
	if err := writePublicDiagnostics(&out, []byte(`{"command":"status","generation":`+largeGeneration+`,"isMutation":false,"choices":[{"lane":"main"}]}`)); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, `"command": "status"`) || !strings.Contains(text, `"choices"`) || !strings.Contains(text, `"lane": "main"`) || !strings.Contains(text, largeGeneration) {
		t.Fatalf("diagnostics lost typed fields or integer fidelity: %s", text)
	}
	if strings.Contains(text, "9007199254740992") {
		t.Fatalf("diagnostics rounded a typed integer: %s", text)
	}
}

func TestPublicStatusGuidanceFailsClosedForRefreshAndUnknownStates(t *testing.T) {
	for _, test := range []struct {
		name       string
		state      string
		executable bool
		want       string
		notWant    string
	}{
		{name: "fresh ready", state: "ready-to-continue", executable: true, want: "可以继续推进"},
		{name: "needs apply", state: "needs-start-apply", executable: true, want: "可以继续推进"},
		{name: "refresh", state: "refresh-required", executable: true, want: "先刷新", notWant: "已就绪"},
		{name: "unknown", state: "future-opaque-state", executable: true, want: "无法安全归类", notWant: "已就绪"},
		{name: "not executable", state: "ready-to-continue", executable: false, want: "还不能执行", notWant: "可以继续推进"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reason, next := publicStatusGuidance(test.state, true, test.executable, false)
			text := reason + " " + next
			if !strings.Contains(text, test.want) || (test.notWant != "" && strings.Contains(text, test.notWant)) {
				t.Fatalf("guidance=%q want=%q notWant=%q", text, test.want, test.notWant)
			}
		})
	}
}

func TestRenderPublicFailureSeparatesDefaultAndDiagnostics(t *testing.T) {
	_, err := plancontract.Match(
		"continue",
		"-ExpectedContinuePlanSha256",
		strings.Repeat("a", 64),
		strings.Repeat("b", 64),
	)
	if err == nil {
		t.Fatal("plancontract.Match returned nil error")
	}
	var stdout, stderr bytes.Buffer
	code := RenderPublicFailure([]string{"continue"}, err, PublicFailureSourceRuntime, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "本次没有执行 Apply") || strings.Contains(stderr.String(), strings.Repeat("a", 64)) || strings.Contains(stderr.String(), "ExpectedContinue") {
		t.Fatalf("default failure code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = RenderPublicFailure([]string{"continue", "--diagnostics"}, err, PublicFailureSourceRuntime, &stdout, &stderr)
	if code != 1 || stderr.Len() != 0 {
		t.Fatalf("diagnostics failure code=%d stderr=%q", code, stderr.String())
	}
	var envelope publicFailureEnvelope
	if decodeErr := json.Unmarshal(stdout.Bytes(), &envelope); decodeErr != nil {
		t.Fatalf("decode diagnostics failure: %v\n%s", decodeErr, stdout.String())
	}
	if envelope.Kind != "steamai-public-failure" || envelope.Diagnostics.Code != plancontract.CodePlanMismatch || envelope.Diagnostics.Expected != strings.Repeat("a", 64) || envelope.Diagnostics.Actual != strings.Repeat("b", 64) || envelope.Diagnostics.MutationApplied {
		t.Fatalf("unexpected diagnostics envelope: %+v", envelope)
	}
}

func TestRenderRuntimePlanFailureUsesRuntimeJSONAndTypedCommand(t *testing.T) {
	_, err := plancontract.ValidatePhase(
		"init",
		"-ExpectedInitPlanSha256",
		false,
		true,
		"",
	)
	if err == nil {
		t.Fatal("plancontract.ValidatePhase returned nil error")
	}
	var stdout, stderr bytes.Buffer
	code, handled := RenderRuntimePlanFailure(
		[]string{"-Command", "init", "-Apply", "-Format", "json"},
		err,
		&stdout,
		&stderr,
	)
	var envelope publicFailureEnvelope
	if decodeErr := json.Unmarshal(stdout.Bytes(), &envelope); !handled || code != 2 || stderr.Len() != 0 || decodeErr != nil || envelope.Command != "init" || envelope.ExitCode != code || envelope.Diagnostics.Code != plancontract.CodePlanMissing {
		t.Fatalf("runtime plan failure handled=%t code=%d envelope=%+v stderr=%q decodeErr=%v", handled, code, envelope, stderr.String(), decodeErr)
	}

	stdout.Reset()
	stderr.Reset()
	if code, handled := RenderRuntimePlanFailure([]string{"-Command", "status"}, errors.New("ordinary maintenance failure"), &stdout, &stderr); handled || code != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("ordinary runtime failure was publicized: handled=%t code=%d stdout=%q stderr=%q", handled, code, stdout.String(), stderr.String())
	}
}

func TestRenderPublicUsageFailureUsesExitTwo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := wrapPublicUsageError(parsePublicErrorForTest())
	code := RenderPublicFailure([]string{"continue", "--apply"}, err, PublicFailureSourceRuntime, &stdout, &stderr)
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "命令未执行") || !strings.Contains(stderr.String(), "steamai help") || strings.Contains(stderr.String(), "--apply") {
		t.Fatalf("usage failure code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = RenderPublicFailure([]string{"continue", "--diagnostics", "--apply"}, err, PublicFailureSourceRuntime, &stdout, &stderr)
	if code != 2 || stderr.Len() != 0 {
		t.Fatalf("diagnostics usage failure code=%d stderr=%q", code, stderr.String())
	}
	var envelope publicFailureEnvelope
	if decodeErr := json.Unmarshal(stdout.Bytes(), &envelope); decodeErr != nil || envelope.Diagnostics.Code != "public-usage-invalid" || !strings.Contains(envelope.Diagnostics.Detail, "--apply") {
		t.Fatalf("diagnostics usage failure=%+v decodeErr=%v", envelope, decodeErr)
	}
}

func TestRenderPublicExecutableFailureDoesNotLeakDefaultDetail(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := errors.New(`project-local executable bytes do not match C:\private\project\.steamai\runtime\manifest.json`)
	code := RenderPublicFailure([]string{"status"}, err, PublicFailureSourceExecutable, &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "完整性校验") || strings.Contains(stderr.String(), `C:\private`) || strings.Contains(stderr.String(), "manifest.json") {
		t.Fatalf("default executable failure code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = RenderPublicFailure([]string{"status", "--diagnostics"}, err, PublicFailureSourceExecutable, &stdout, &stderr)
	var envelope publicFailureEnvelope
	if decodeErr := json.Unmarshal(stdout.Bytes(), &envelope); code != 1 || stderr.Len() != 0 || decodeErr != nil || envelope.Diagnostics.Code != "public-executable-invalid" || !strings.Contains(envelope.Diagnostics.Detail, `C:\private`) {
		t.Fatalf("diagnostics executable failure code=%d envelope=%+v stderr=%q decodeErr=%v", code, envelope, stderr.String(), decodeErr)
	}
}

func parsePublicErrorForTest() error {
	_, err := parsePublicOptions([]string{"continue", "--apply"})
	return err
}
