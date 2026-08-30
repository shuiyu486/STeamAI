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

func TestProjectStatePathsUseActiveMissionNamespace(t *testing.T) {
	caseRoot := t.TempDir()
	stateRoot := filepath.Join(caseRoot, projectstate.CurrentDir)
	activeMissionRoot := filepath.Join(stateRoot, projectstate.MissionsDir, "g000002")
	view := projectstate.MissionView{
		Root:       projectstate.Root{Dir: projectstate.CurrentDir, Path: stateRoot, Existing: true},
		Generation: 2,
		Path:       activeMissionRoot,
	}

	got := projectStatePathsInMissionView(view, []string{
		".rekit/facts/**",
		".steamai/lanes/**",
		".steamai/instance.yml",
		"references/binary-re/**",
	})
	want := []string{
		".steamai/missions/g000002/facts/**",
		".steamai/missions/g000002/lanes/**",
		".steamai/instance.yml",
		"references/binary-re/**",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projected paths = %v, want %v", got, want)
	}
}
