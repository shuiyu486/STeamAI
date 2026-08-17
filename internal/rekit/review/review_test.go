package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestReviewPathsUseResolvedStateRoot(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  string
	}{
		{name: "current", dir: projectstate.CurrentDir},
		{name: "legacy", dir: projectstate.LegacyDir},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, tc.dir), 0o700); err != nil {
				t.Fatal(err)
			}
			paths, err := reviewPaths(Plan{Command: "sync", CaseRoot: caseRoot}, ArtifactOptions{})
			if err != nil {
				t.Fatal(err)
			}
			wantPrefix := filepath.Join(caseRoot, tc.dir, "reviews") + string(filepath.Separator)
			if !strings.HasPrefix(paths.Root, wantPrefix) || !strings.HasSuffix(paths.Root, "-sync") {
				t.Fatalf("review root = %q, want prefix %q", paths.Root, wantPrefix)
			}
		})
	}
}

func TestReviewPathsPreserveExplicitOutputDir(t *testing.T) {
	caseRoot := t.TempDir()
	explicit := filepath.Join(t.TempDir(), "review-output")
	paths, err := reviewPaths(Plan{Command: "promote", CaseRoot: caseRoot}, ArtifactOptions{ReviewOutputDir: explicit})
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if paths.Root != want {
		t.Fatalf("explicit review root = %q, want %q", paths.Root, want)
	}
}

func TestReviewPathsRejectDualStateRoots(t *testing.T) {
	caseRoot := t.TempDir()
	for _, dir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if err := os.Mkdir(filepath.Join(caseRoot, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := reviewPaths(Plan{Command: "sync", CaseRoot: caseRoot}, ArtifactOptions{}); err == nil || !strings.Contains(err.Error(), "must not coexist") {
		t.Fatalf("dual-root review error = %v", err)
	}
}
