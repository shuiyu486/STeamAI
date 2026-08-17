package laneowner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathUsesActiveStateRoot(t *testing.T) {
	for _, stateDir := range []string{".steamai", ".rekit"} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			rel, full, err := Path(caseRoot, "main")
			if err != nil {
				t.Fatal(err)
			}
			wantRel := filepath.ToSlash(filepath.Join(stateDir, "lanes", "main", "lane.json"))
			if rel != wantRel || full != filepath.Join(caseRoot, filepath.FromSlash(wantRel)) {
				t.Fatalf("Path = %q, %q; want %q under active root", rel, full, wantRel)
			}
		})
	}
}

func TestPathRejectsDualStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, stateDir := range []string{".steamai", ".rekit"} {
		if err := os.Mkdir(filepath.Join(caseRoot, stateDir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := Path(caseRoot, "main"); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("Path error = %v, want dual-root rejection", err)
	}
}
