package memberexecution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArtifactAnalysisTaskBindingValidatesExactCurrentBytes(t *testing.T) {
	caseRoot := t.TempDir()
	path := filepath.Join(caseRoot, "inputs", "sample.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("sample-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	binding, err := ArtifactAnalysisTaskBinding(caseRoot, "inputs/sample.bin")
	if err != nil {
		t.Fatal(err)
	}
	if binding.Kind != TaskBindingArtifactAnalysis || binding.Values[inputArtifactPathKey] != "inputs/sample.bin" || !validSHA(binding.Values[inputArtifactSHAKey]) || binding.Values[inputArtifactBytesKey] != "9" {
		t.Fatalf("artifact binding=%+v", binding)
	}
	if err := ValidateTaskInputBinding(caseRoot, binding); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("sample-v2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTaskInputBinding(caseRoot, binding); err == nil || !strings.Contains(err.Error(), "input changed") {
		t.Fatalf("artifact drift validation=%v", err)
	}
}

func TestArtifactAnalysisTaskBindingRejectsEmptyArtifact(t *testing.T) {
	caseRoot := t.TempDir()
	path := filepath.Join(caseRoot, "inputs", "empty.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ArtifactAnalysisTaskBinding(caseRoot, "inputs/empty.bin"); err == nil ||
		!strings.Contains(err.Error(), "non-empty regular file") {
		t.Fatalf("empty artifact binding error=%v", err)
	}
}

func TestWorkspaceInventoryTaskBindingAcceptsEmptyScope(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, "inputs"), 0o755); err != nil {
		t.Fatal(err)
	}
	binding, err := WorkspaceInventoryTaskBinding(caseRoot, "inputs")
	if err != nil {
		t.Fatal(err)
	}
	if binding.Kind != TaskBindingWorkspaceInventory || binding.Values[inputWorkspaceScopeKey] != "inputs" {
		t.Fatalf("inventory binding=%+v", binding)
	}
	if err := ValidateTaskInputBinding(caseRoot, binding); err != nil {
		t.Fatalf("empty workspace inventory should be valid: %v", err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "inputs", "later.bin"), []byte("later"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTaskInputBinding(caseRoot, binding); err != nil {
		t.Fatalf("workspace inventory binds an explicit scope, not immutable contents: %v", err)
	}
}

func TestTypedTaskBindingRejectsEscapingInputPath(t *testing.T) {
	caseRoot := t.TempDir()
	if _, err := ArtifactAnalysisTaskBinding(caseRoot, "../sample.bin"); err == nil || !strings.Contains(err.Error(), "case-relative") {
		t.Fatalf("escaping artifact path error=%v", err)
	}
	if _, err := WorkspaceInventoryTaskBinding(caseRoot, "."); err != nil {
		t.Fatalf("case root inventory should be valid: %v", err)
	}
}
