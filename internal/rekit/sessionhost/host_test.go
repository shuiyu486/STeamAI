package sessionhost

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCurrentReviewerRejectionProbeAllowsMissingBootstrapBoard(t *testing.T) {
	rejected, err := currentReviewerRejectionAwaitingCorrection(t.TempDir(), liveAcceptancePack)
	if err != nil || rejected {
		t.Fatalf("missing bootstrap board rejection probe: rejected=%t err=%v", rejected, err)
	}
}

func TestResolveClaudePathReportsTypedExecutableFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-claude-executable")
	if runtime.GOOS == "windows" {
		missing += ".exe"
	}
	result, err := Run(context.Background(), Options{Target: t.TempDir(), ClaudePath: missing, MaxAttempts: 3})
	if err == nil {
		t.Fatal("missing Claude executable was accepted")
	}
	if result.Failure == nil || result.Failure.Code != "claude-executable-unavailable" || result.Failure.Stage != "executable-resolution" || result.Failure.State != failureStateRecoverable {
		t.Fatalf("typed executable diagnosis = %+v", result.Failure)
	}
	if result.Failure.MutationApplied || result.Failure.MutationBoundary != "none" || result.Failure.NextAction == "" {
		t.Fatalf("executable mutation boundary or next action = %+v", result.Failure)
	}
}

func TestRunResolvesOmittedPackFromAttachedCaseMetadata(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := []byte("templatePack: _template\n")
	if err := os.WriteFile(
		filepath.Join(caseRoot, ".rekit", "instance.yml"),
		metadata,
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(caseRoot, "missing-claude-executable")
	if runtime.GOOS == "windows" {
		missing += ".exe"
	}
	result, err := Run(context.Background(), Options{
		Target:      caseRoot,
		ClaudePath:  missing,
		MaxAttempts: 3,
	})
	if err == nil {
		t.Fatal("missing Claude executable was accepted")
	}
	if result.Pack != "_template" {
		t.Fatalf("omitted host pack was not resolved from case metadata: %+v", result)
	}
}

func TestResolveClaudePathRejectsWindowsCommandScripts(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows executable resolution only")
	}
	path := filepath.Join(t.TempDir(), "claude.cmd")
	if err := os.WriteFile(path, []byte("@exit /b 0\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveClaudePath(path); err == nil || !strings.Contains(err.Error(), "native claude.exe") {
		t.Fatalf("command script resolution error=%v", err)
	}
}
