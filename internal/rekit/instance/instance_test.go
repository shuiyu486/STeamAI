package instance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadScalarFileStripsSimpleQuotes(t *testing.T) {
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "templateRoot: \"C:\\kit\"\ntemplatePack: 'vmp-re'\nprojectName: demo\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	inst, err := Read(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if inst.TemplateRoot != `C:\kit` {
		t.Fatalf("TemplateRoot = %q, want C:\\kit", inst.TemplateRoot)
	}
	if inst.TemplatePack != "vmp-re" {
		t.Fatalf("TemplatePack = %q, want vmp-re", inst.TemplatePack)
	}
}
