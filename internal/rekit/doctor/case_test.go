package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestValidateWorkstreamStateUsesResolvedRoot(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  string
	}{
		{name: "current", dir: projectstate.CurrentDir},
		{name: "legacy", dir: projectstate.LegacyDir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			stateRoot := filepath.Join(caseRoot, tc.dir)
			if err := os.MkdirAll(filepath.Join(stateRoot, "lanes"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stateRoot, "board.json"), []byte("{\"caseRoot\":"+quotedJSON(caseRoot)+"}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := validateWorkstreamState(caseRoot, &manifest.Manifest{}); err != nil {
				t.Fatal(err)
			}

			otherDir := projectstate.LegacyDir
			if tc.dir == projectstate.LegacyDir {
				otherDir = projectstate.CurrentDir
			}
			if err := os.MkdirAll(filepath.Join(caseRoot, otherDir, "lanes"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(caseRoot, otherDir, "board.json"), []byte("{not-json\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := validateWorkstreamState(caseRoot, &manifest.Manifest{}); err == nil || !strings.Contains(err.Error(), "must not coexist") {
				t.Fatalf("dual-root validation error = %v", err)
			}
		})
	}
}

func quotedJSON(value string) string {
	return `"` + strings.ReplaceAll(value, `\`, `\\`) + `"`
}
