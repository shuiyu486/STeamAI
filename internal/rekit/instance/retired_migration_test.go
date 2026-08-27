package instance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
)

func TestReadRetiredMigrationSourceIsNarrowAndExact(t *testing.T) {
	for _, retired := range packidentity.RetiredIDs() {
		t.Run(retired, func(t *testing.T) {
			root := t.TempDir()
			repoRoot := filepath.Join(root, "repo")
			caseRoot := filepath.Join(root, "case")
			if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(repoRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			instanceText := "schemaVersion: 1\n" +
				"templateRoot: " + repoRoot + "\n" +
				"templatePack: " + retired + "\n" +
				"projectName: retired-case\n" +
				"projectRoot: " + caseRoot + "\n" +
				"mode: case-local-shim\n"
			if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(instanceText), 0o644); err != nil {
				t.Fatal(err)
			}
			legacyText := "templateRoot: " + repoRoot + "\r\n" +
				"rekitMode: case-local-shim\r\n" +
				"templatePack: " + retired + "\r\n" +
				"currentProjectPath: " + caseRoot + "\r\n"
			if err := os.WriteFile(filepath.Join(caseRoot, ".re-template.yml"), []byte(legacyText), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := Read(caseRoot); err == nil || !packidentity.IsMigrationRequired(err) {
				t.Fatalf("ordinary Read error = %v, want typed migration requirement", err)
			}
			inst, err := ReadRetiredMigrationSource(caseRoot, retired)
			if err != nil {
				t.Fatal(err)
			}
			if inst.Source != "instance" || inst.StateDir != StateDirRekit || inst.SchemaVersion != 1 || inst.Mode != "case-local-shim" || inst.TemplatePack != retired || inst.TemplateRoot != repoRoot || inst.ProjectRoot != caseRoot || inst.Moved() {
				t.Fatalf("migration source = %+v", inst)
			}
			if _, err := ReadRetiredMigrationSource(caseRoot, packidentity.Canonical); err == nil || !strings.Contains(err.Error(), "retired") {
				t.Fatalf("active selector accepted by migration-only reader: %v", err)
			}
			other := packidentity.RetiredVMP
			if retired == other {
				other = packidentity.RetiredGeneric
			}
			if _, err := ReadRetiredMigrationSource(caseRoot, other); err == nil || !strings.Contains(err.Error(), "selector") {
				t.Fatalf("different retired selector accepted: %v", err)
			}

			drifted := strings.Replace(legacyText, "templatePack: "+retired, "templatePack: "+other, 1)
			if err := os.WriteFile(filepath.Join(caseRoot, ".re-template.yml"), []byte(drifted), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadRetiredMigrationSource(caseRoot, retired); err == nil || !strings.Contains(err.Error(), "legacy metadata") {
				t.Fatalf("legacy metadata pack drift accepted: %v", err)
			}
		})
	}
}
