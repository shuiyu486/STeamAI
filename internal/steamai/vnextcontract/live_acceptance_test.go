package vnextcontract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

const liveAcceptanceEnv = "STEAMAI_VNEXT_LIVE_ACCEPTANCE"

func TestLiveNativeContextAndFileAccess(t *testing.T) {
	if os.Getenv(liveAcceptanceEnv) != "1" {
		t.Skip("set " + liveAcceptanceEnv + "=1 to run the real Claude Code capability probe")
	}

	claude, err := exec.LookPath("claude")
	if err != nil {
		t.Fatalf("locate canonical Claude Code CLI: %v", err)
	}

	model := liveConfiguredHaikuModel(t)
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".steamai-vnext")
	memberRoot := filepath.Join(stateRoot, "members", "probe-member")
	writeLiveProbeFile(t, filepath.Join(stateRoot, "CLAUDE.md"), `# Probe case

- Case marker: vnt02-probe-case
- Boundary: temporary files only; no network or external effects
`)
	writeLiveProbeFile(t, filepath.Join(memberRoot, "CLAUDE.md"), `# Probe member

- Member: probe-member
- Current task: verify native context and explicit case-root file access
- Allowed tools: read only
`)
	writeLiveProbeFile(t, filepath.Join(stateRoot, "probe-marker.md"), "VNT02_CASE_FILE_9F4C2D\n")

	contextSchema := `{"type":"object","properties":{"member":{"type":"string"},"task":{"type":"string"},"caseMarker":{"type":"string"}},"required":["member","task","caseMarker"],"additionalProperties":false}`
	contextPayload := runLiveClaudeProbe(t, claude, model, memberRoot, "", "", contextSchema,
		`Return the exact Member, Current task, and Case marker values already supplied in your project instructions. Do not read files or infer values.`)
	var contextGot struct {
		Member     string `json:"member"`
		Task       string `json:"task"`
		CaseMarker string `json:"caseMarker"`
	}
	if err := json.Unmarshal(contextPayload, &contextGot); err != nil {
		t.Fatalf("decode native context probe result: %v: %s", err, strings.TrimSpace(string(contextPayload)))
	}
	if contextGot.Member != "probe-member" || contextGot.Task != "verify native context and explicit case-root file access" || contextGot.CaseMarker != "vnt02-probe-case" {
		t.Fatalf("native context probe mismatch: got %+v", contextGot)
	}

	fileSchema := `{"type":"object","properties":{"fileMarker":{"type":"string"}},"required":["fileMarker"],"additionalProperties":false}`
	filePayload := runLiveClaudeProbe(t, claude, model, memberRoot, caseRoot, "Read", fileSchema,
		`Read ../../probe-marker.md and return its exact value as fileMarker. Do not infer or paraphrase it.`)
	var fileGot struct {
		FileMarker string `json:"fileMarker"`
	}
	if err := json.Unmarshal(filePayload, &fileGot); err != nil {
		t.Fatalf("decode native file-access probe result: %v: %s", err, strings.TrimSpace(string(filePayload)))
	}
	if fileGot.FileMarker != "VNT02_CASE_FILE_9F4C2D" {
		t.Fatalf("native file-access probe mismatch: got %+v", fileGot)
	}
}

func runLiveClaudeProbe(t *testing.T, claude, model, memberRoot, addDir, tools, schema, prompt string) json.RawMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	args := []string{
		"-p", prompt,
		"--no-session-persistence",
		"--permission-mode", "dontAsk",
		"--tools", tools,
		"--output-format", "json",
		"--json-schema", schema,
	}
	if addDir != "" {
		args = append(args, "--add-dir", addDir)
	}
	cmd := liveHaikuCommand(ctx, claude, model, memberRoot, args...)
	stdout, err := cmd.Output()
	if ctx.Err() != nil {
		t.Fatalf("real Claude Code capability probe timed out: %v", ctx.Err())
	}
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			t.Fatalf("real Claude Code capability probe failed: %v: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		t.Fatalf("real Claude Code capability probe failed: %v", err)
	}
	var envelope struct {
		IsError          bool            `json:"is_error"`
		Result           string          `json:"result"`
		StructuredOutput json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(stdout, &envelope); err != nil {
		t.Fatalf("decode Claude Code result envelope: %v: %s", err, strings.TrimSpace(string(stdout)))
	}
	if envelope.IsError {
		t.Fatalf("Claude Code reported a failed capability probe: %s", strings.TrimSpace(string(stdout)))
	}
	if len(envelope.StructuredOutput) > 0 && string(envelope.StructuredOutput) != "null" {
		return envelope.StructuredOutput
	}
	return json.RawMessage(envelope.Result)
}

// 本包 live cwd 均为临时成员目录；启动时只加载 user settings，不读取仓库 project/local settings。
func liveConfiguredHaikuModel(t *testing.T) string {
	t.Helper()
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		configDir = filepath.Join(home, ".claude")
	}
	model, err := freezeLiveHaikuModel(filepath.Join(configDir, "settings.json"), os.Getenv("ANTHROPIC_DEFAULT_HAIKU_MODEL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("frozen live Haiku model=%s", model)
	return model
}

var liveHaikuModelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*(\[1m\])?$`)

func freezeLiveHaikuModel(settingsPath, inherited string) (string, error) {
	model := inherited
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		var settings struct {
			Env map[string]string `json:"env"`
		}
		if err := json.Unmarshal(data, &settings); err != nil {
			return "", fmt.Errorf("invalid user model settings: %w", err)
		}
		// Claude Code 的 settings.env 覆盖同名进程变量；只提取本次获准的 Haiku 槽位。
		if value, ok := settings.Env["ANTHROPIC_DEFAULT_HAIKU_MODEL"]; ok {
			model = value
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read user model settings: %w", err)
	}
	switch strings.ToLower(strings.TrimSuffix(model, "[1m]")) {
	case "", "default", "auto", "haiku", "sonnet", "opus", "fable", "opusplan":
		return "", fmt.Errorf("live test requires a fixed ANTHROPIC_DEFAULT_HAIKU_MODEL; no main-model or higher-tier fallback")
	}
	if !liveHaikuModelPattern.MatchString(model) {
		return "", fmt.Errorf("ANTHROPIC_DEFAULT_HAIKU_MODEL is not a fixed model name")
	}
	return model, nil
}

func liveHaikuCommand(ctx context.Context, claude, model, cwd string, args ...string) *exec.Cmd {
	args = append([]string{"--model", model, "--setting-sources", "user"}, args...)
	cmd := exec.CommandContext(ctx, claude, args...)
	cmd.Dir = cwd
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	return cmd
}

func TestLiveHaikuModelFreezesConfiguredMapping(t *testing.T) {
	for _, test := range []struct {
		name, settings, inherited, want string
	}{
		{"environment", "", "gpt-fixture[1m]", "gpt-fixture[1m]"},
		{"user-over-environment", `{"env":{"ANTHROPIC_DEFAULT_HAIKU_MODEL":"provider/user"}}`, "provider/environment", "provider/user"},
		{"ignore-main-and-opus", `{"model":"opus","env":{"ANTHROPIC_MODEL":"provider/expensive","ANTHROPIC_DEFAULT_OPUS_MODEL":"provider/expensive","ANTHROPIC_DEFAULT_HAIKU_MODEL":"provider/cheap"}}`, "", "provider/cheap"},
		{"missing", "", "", ""},
		{"no-main-fallback", `{"model":"opus","env":{"ANTHROPIC_MODEL":"provider/expensive","ANTHROPIC_DEFAULT_OPUS_MODEL":"provider/expensive"}}`, "", ""},
		{"empty-user-value", `{"env":{"ANTHROPIC_DEFAULT_HAIKU_MODEL":""}}`, "provider/environment", ""},
		{"alias", `{"env":{"ANTHROPIC_DEFAULT_HAIKU_MODEL":"haiku"}}`, "", ""},
		{"high-tier-alias", "", "opus", ""},
		{"dynamic-alias", "", "sonnet[1m]", ""},
		{"invalid-name", "", "invalid model", ""},
		{"invalid-json", "{", "provider/environment", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if test.settings != "" {
				writeLiveProbeFile(t, path, test.settings)
			}
			got, err := freezeLiveHaikuModel(path, test.inherited)
			if got != test.want || (err != nil) != (test.want == "") {
				t.Fatalf("model=%q want=%q err=%v", got, test.want, err)
			}
		})
	}
}

func TestLiveHaikuCommandUsesFrozenModelAndUserSettings(t *testing.T) {
	t.Setenv("ANTHROPIC_MODEL", "provider/expensive")
	t.Setenv("CLAUDECODE", "parent-session")
	root := t.TempDir()
	path := filepath.Join(root, "user", "settings.json")
	writeLiveProbeFile(t, path, `{"env":{"ANTHROPIC_DEFAULT_HAIKU_MODEL":"provider/frozen"}}`)
	model, err := freezeLiveHaikuModel(path, "")
	if err != nil {
		t.Fatal(err)
	}
	writeLiveProbeFile(t, path, `{"env":{"ANTHROPIC_DEFAULT_HAIKU_MODEL":"provider/changed"}}`)
	cwd := filepath.Join(root, "case", ".steamai-vnext", "members", "owner")
	cmd := liveHaikuCommand(context.Background(), "claude", model, cwd, "-p", "synthetic probe")
	want := []string{"claude", "--model", "provider/frozen", "--setting-sources", "user", "-p", "synthetic probe"}
	if !slices.Equal(cmd.Args, want) || cmd.Dir != cwd {
		t.Fatalf("args=%q cwd=%q", cmd.Args, cmd.Dir)
	}
	for _, item := range cmd.Env {
		key, _, _ := strings.Cut(item, "=")
		if strings.EqualFold(key, "CLAUDECODE") {
			t.Fatal("inherited parent-session marker")
		}
	}
	// 只构造命令，不启动 Claude；user-only flags 排除 cwd/祖先的 project/local settings。
}

func writeLiveProbeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withoutEnvironment(env []string, names ...string) []string {
	blocked := make(map[string]struct{}, len(names))
	for _, name := range names {
		blocked[strings.ToUpper(name)] = struct{}{}
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		name, _, _ := strings.Cut(item, "=")
		if _, found := blocked[strings.ToUpper(name)]; !found {
			out = append(out, item)
		}
	}
	return out
}
