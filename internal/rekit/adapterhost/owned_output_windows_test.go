//go:build windows

package adapterhost

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCleanupDeletesExactOwnedArtifactNotReplacement(t *testing.T) {
	fixture := newHostFixture(t, "fixture/input.txt", 10, 4, 1)
	artifactRel := filepath.ToSlash(filepath.Join("workspace", "main", "inspect", "session-1", "inspection.json"))
	artifactPath := filepath.Join(fixture.caseRoot, filepath.FromSlash(artifactRel))
	movedPath := filepath.Join(filepath.Dir(artifactPath), "owned-moved.json")
	competitor := []byte("competing artifact\n")
	fixture.options.testHooks = &hostTestHooks{
		beforeReportWrite: func() error { return os.ErrPermission },
		afterCleanupIdentityOpen: func(rel string) error {
			if rel != artifactRel {
				return nil
			}
			if err := os.Rename(artifactPath, movedPath); err != nil {
				return err
			}
			return os.WriteFile(artifactPath, competitor, 0o600)
		},
	}
	if _, err := Run(fixture.options); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("injected report failure should surface: %v", err)
	}
	if got, err := os.ReadFile(artifactPath); err != nil || !bytes.Equal(got, competitor) {
		t.Fatalf("cleanup changed competing canonical artifact: got=%q err=%v", got, err)
	}
	if _, err := os.Lstat(movedPath); !os.IsNotExist(err) {
		t.Fatalf("exact owned artifact was not deleted after rename: err=%v", err)
	}
	reportPath := filepath.Join(filepath.Dir(artifactPath), "adapter-report.json")
	if _, err := os.Lstat(reportPath); err == nil || !strings.Contains(strings.ToLower(err.Error()), "cannot find") && !os.IsNotExist(err) {
		t.Fatalf("unexpected report publication: err=%v", err)
	}
}
