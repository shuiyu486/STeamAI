package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
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
	if !strings.Contains(continueText, "fresh continue 预览") || !strings.Contains(continueText, "不会自动 Apply") {
		t.Fatalf("continue summary omitted safety contract: %s", continueText)
	}
	if !strings.Contains(continueText, "\n原因：") || !strings.Contains(continueText, "\n下一步：") {
		t.Fatalf("continue summary is not split into user-facing lines: %s", continueText)
	}
	if strings.Contains(continueText, "authz") || strings.Contains(continueText, "feature-authz") || strings.Contains(continueText, strings.Repeat("a", 64)) {
		t.Fatalf("continue summary leaked durable or typed diagnostics: %s", continueText)
	}
}

func TestPublicStatusSummaryExplainsFreshOnboardingWithoutInternalChoices(t *testing.T) {
	var out bytes.Buffer
	raw := json.RawMessage(`{
		"mode":"case-onboarding-required",
		"caseMission":{"summary":"project onboarding requires a goal and pack selection"},
		"onboarding":{
			"state":"absent",
			"selectedPack":"binary-re",
			"packChoices":[{"id":"binary-re","name":"binary-re","maturity":"mature"}]
		},
		"missionControlRunbook":{}
	}`)
	if err := writePublicInteraction(&out, "status", raw); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "项目尚未完成首次接入") || !strings.Contains(text, "告诉主 Agent 你的目标并选择一个 pack") {
		t.Fatalf("fresh onboarding summary omitted user action: %s", text)
	}
	for _, internal := range []string{"binary-re", "packChoices", "selectedPack", "mature"} {
		if strings.Contains(text, internal) {
			t.Fatalf("fresh onboarding summary leaked internal choice detail %q: %s", internal, text)
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
		{name: "fresh ready", state: "ready-to-continue", executable: true, want: "fresh typed action 已就绪"},
		{name: "needs apply", state: "needs-start-apply", executable: true, want: "fresh typed action 已就绪"},
		{name: "refresh", state: "refresh-required", executable: true, want: "先刷新", notWant: "已就绪"},
		{name: "unknown", state: "future-opaque-state", executable: true, want: "无法安全归类", notWant: "已就绪"},
		{name: "not executable", state: "ready-to-continue", executable: false, want: "尚未达到可执行状态", notWant: "已就绪"},
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
