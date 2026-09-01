package vnextcontract

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
	contextPayload := runLiveClaudeProbe(t, claude, memberRoot, "", "", contextSchema,
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
	filePayload := runLiveClaudeProbe(t, claude, memberRoot, caseRoot, "Read", fileSchema,
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

func runLiveClaudeProbe(t *testing.T, claude, memberRoot, addDir, tools, schema, prompt string) json.RawMessage {
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
	cmd := exec.CommandContext(ctx, claude, args...)
	cmd.Dir = memberRoot
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
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
