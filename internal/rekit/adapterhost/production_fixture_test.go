package adapterhost

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/instructionpacket"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimeinstruction"
	"github.com/shuiyu486/re-context-kits/internal/rekit/testfixture"
)

func productionFixtureSourceRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func copyProductionFixtureInputs(t *testing.T, targetRoot, pack string) {
	t.Helper()
	sourceRoot := productionFixtureSourceRoot(t)
	for _, rel := range []string{
		"common",
		filepath.ToSlash(filepath.Join("packs", pack)),
		"rekit/templates/steamai-project/SKILL.md",
		"rekit/schemas/instance.schema.yml",
		"rekit/schemas/pack-manifest.schema.yml",
		"rekit/tests/catalog.json",
	} {
		copyProductionFixturePath(t, filepath.Join(sourceRoot, filepath.FromSlash(rel)), filepath.Join(targetRoot, filepath.FromSlash(rel)))
	}
}

func copyProductionFixturePath(t *testing.T, source, target string) {
	t.Helper()
	info, err := os.Lstat(source)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("production fixture source cannot be a symlink: %s", source)
	}
	if info.IsDir() {
		if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("production fixture source cannot contain a symlink: %s", path)
			}
			rel, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			destination := filepath.Join(target, rel)
			if entry.IsDir() {
				return os.MkdirAll(destination, 0o755)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			return os.WriteFile(destination, data, 0o644)
		}); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func publishProductionFixtureProject(t *testing.T, sourceRepo, caseRoot, pack, projectName string) testfixture.Project {
	t.Helper()
	return testfixture.NewProject(t, testfixture.ProjectOptions{
		Layout:      testfixture.CurrentProject,
		SourceRepo:  sourceRepo,
		CaseRoot:    caseRoot,
		Pack:        pack,
		ProjectName: projectName,
		Components:  testfixture.Components{InitialState: true},
	})
}

func productionInstructionIdentityForFixture(t *testing.T, caseRoot, pack string) *instructionpacket.Identity {
	t.Helper()
	packet, production, err := runtimeinstruction.Build(caseRoot, pack)
	if err != nil {
		t.Fatal(err)
	}
	if !production {
		t.Fatalf("fixture pack is not production: %s", pack)
	}
	identity := packet.Identity()
	if strings.TrimSpace(identity.SHA256) == "" {
		t.Fatalf("fixture production instruction identity is empty: %+v", identity)
	}
	return &identity
}
