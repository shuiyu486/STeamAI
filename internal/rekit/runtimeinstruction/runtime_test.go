package runtimeinstruction

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/casebind"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
)

func TestBuildRejectsCurrentSchemaV1CentralRuntimeFallback(t *testing.T) {
	repoRoot := runtimeInstructionRepoRoot(t)
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".steamai")
	if err := os.MkdirAll(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := casebind.InstanceText(caseRoot, repoRoot, defaults.DefaultPack, "current-schema-v1")
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}

	_, production, err := Build(caseRoot, defaults.DefaultPack)
	if !production || err == nil || !strings.Contains(err.Error(), "schema v2 project-local-bundle") {
		t.Fatalf("current schema v1 fallback production=%t err=%v", production, err)
	}
}

func TestBuildRetainsLegacySchemaV1RuntimeCompatibility(t *testing.T) {
	repoRoot := runtimeInstructionRepoRoot(t)
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, ".rekit")
	if err := os.MkdirAll(stateRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := casebind.InstanceText(caseRoot, repoRoot, defaults.DefaultPack, "legacy-schema-v1")
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}

	packet, production, err := Build(caseRoot, defaults.DefaultPack)
	if err != nil || !production || packet.Identity().Pack != defaults.DefaultPack {
		t.Fatalf("legacy schema v1 production=%t identity=%+v err=%v", production, packet.Identity(), err)
	}
}

func runtimeInstructionRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate runtime instruction test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
