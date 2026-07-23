//go:build windows

package subagents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveReviewerResultObstructionPinsQuarantineNamespace(t *testing.T) {
	root := t.TempDir()
	resultPath := filepath.Join(root, "result.json")
	if err := os.WriteFile(resultPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	quarantineRoot := filepath.Join(root, "recoveries")
	if err := os.Mkdir(quarantineRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	guardPath := filepath.Join(quarantineRoot, "intent.json")
	if err := os.WriteFile(guardPath, []byte("intent"), 0o600); err != nil {
		t.Fatal(err)
	}
	quarantinePath := filepath.Join(quarantineRoot, "result.json")
	movedRoot := filepath.Join(root, "recoveries-moved")

	if err := moveReviewerResultObstructionExact(
		resultPath,
		quarantinePath,
		guardPath,
		func() error {
			if err := os.Rename(quarantineRoot, movedRoot); err == nil {
				t.Fatal("quarantine namespace moved while its guard was held")
			}
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(resultPath); !os.IsNotExist(err) {
		t.Fatalf("canonical result still exists: %v", err)
	}
	if st, err := os.Lstat(quarantinePath); err != nil {
		t.Fatal(err)
	} else if !st.Mode().IsRegular() || st.Size() != 0 {
		t.Fatalf("unexpected quarantined obstruction: mode=%v size=%d", st.Mode(), st.Size())
	}
	if _, err := os.Lstat(movedRoot); !os.IsNotExist(err) {
		t.Fatalf("moved quarantine namespace unexpectedly exists: %v", err)
	}
}
