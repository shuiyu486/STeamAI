package workstream

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestProjectStatePathsUseResolvedRoot(t *testing.T) {
	for _, test := range []struct {
		name     string
		stateDir string
	}{
		{name: "current", stateDir: projectstate.CurrentDir},
		{name: "legacy", stateDir: projectstate.LegacyDir},
	} {
		t.Run(test.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, test.stateDir), 0o755); err != nil {
				t.Fatal(err)
			}
			got, err := projectStatePaths(caseRoot, []string{
				".rekit/facts/**",
				".steamai/lanes/**",
				"references/binary-re/**",
			})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{
				test.stateDir + "/facts/**",
				test.stateDir + "/lanes/**",
				"references/binary-re/**",
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("projected paths = %v, want %v", got, want)
			}
		})
	}
}
