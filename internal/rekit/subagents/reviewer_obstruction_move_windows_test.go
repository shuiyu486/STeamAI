//go:build windows

package subagents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewerObstructionCanonicalHandlePath(t *testing.T) {
	for _, test := range []struct {
		name string
		in   string
		want string
	}{
		{name: "local", in: `\\?\C:\cases\review.json`, want: `C:\cases\review.json`},
		{name: "unc", in: `\\?\UNC\server\share\cases\review.json`, want: `\\server\share\cases\review.json`},
		{name: "unc-case-insensitive", in: `\\?\unc\server\share\review.json`, want: `\\server\share\review.json`},
		{name: "plain-unc", in: `\\server\share\review.json`, want: `\\server\share\review.json`},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := reviewerObstructionCanonicalHandlePath(test.in); got != test.want {
				t.Fatalf("canonical handle path = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMoveReviewerResultExactRegularPinsSourceIdentity(t *testing.T) {
	root := t.TempDir()
	resultPath := filepath.Join(root, "result.json")
	expected := []byte(`{"expected":true}`)
	if err := os.WriteFile(resultPath, expected, 0o644); err != nil {
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
	replacementPath := filepath.Join(root, "replacement.json")
	if err := os.WriteFile(replacementPath, []byte(`{"replacement":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := moveReviewerResultExact(
		resultPath,
		quarantinePath,
		guardPath,
		reviewerResultExactMoveExpectation{Kind: "regular-file", Contents: expected},
		func() error {
			if err := os.Rename(replacementPath, resultPath); err == nil {
				t.Fatal("regular reviewer result source was replaced while its exact-move handle was held")
			}
			return nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if moved, err := os.ReadFile(quarantinePath); err != nil {
		t.Fatal(err)
	} else if string(moved) != string(expected) {
		t.Fatalf("quarantined bytes = %q", moved)
	}
	if _, err := os.Lstat(resultPath); !os.IsNotExist(err) {
		t.Fatalf("canonical result still exists: %v", err)
	}
	if replacement, err := os.ReadFile(replacementPath); err != nil {
		t.Fatal(err)
	} else if string(replacement) != `{"replacement":true}` {
		t.Fatalf("replacement bytes changed: %q", replacement)
	}
}

func TestMoveReviewerResultExactRegularRejectsUnexpectedHandleBytes(t *testing.T) {
	root := t.TempDir()
	resultPath := filepath.Join(root, "result.json")
	actual := []byte(`{"actual":true}`)
	if err := os.WriteFile(resultPath, actual, 0o644); err != nil {
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

	err := moveReviewerResultExact(
		resultPath,
		quarantinePath,
		guardPath,
		reviewerResultExactMoveExpectation{Kind: "regular-file", Contents: []byte(`{"expected":true}`)},
		func() error { return nil },
	)
	if err == nil || (!strings.Contains(err.Error(), "source handle bytes changed") && !strings.Contains(err.Error(), "not the expected bounded")) {
		t.Fatalf("unexpected exact-move error: %v", err)
	}
	if current, readErr := os.ReadFile(resultPath); readErr != nil {
		t.Fatal(readErr)
	} else if string(current) != string(actual) {
		t.Fatalf("canonical bytes changed: %q", current)
	}
	if _, statErr := os.Lstat(quarantinePath); !os.IsNotExist(statErr) {
		t.Fatalf("quarantine unexpectedly exists: %v", statErr)
	}
}

func TestMoveReviewerResultExactRegularDoesNotReplaceQuarantine(t *testing.T) {
	root := t.TempDir()
	resultPath := filepath.Join(root, "result.json")
	expected := []byte(`{"expected":true}`)
	if err := os.WriteFile(resultPath, expected, 0o644); err != nil {
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
	concurrent := []byte(`{"concurrent":true}`)
	if err := os.WriteFile(quarantinePath, concurrent, 0o644); err != nil {
		t.Fatal(err)
	}

	err := moveReviewerResultExact(
		resultPath,
		quarantinePath,
		guardPath,
		reviewerResultExactMoveExpectation{Kind: "regular-file", Contents: expected},
		func() error { return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "NTSTATUS") {
		t.Fatalf("unexpected no-replace error: %v", err)
	}
	if current, readErr := os.ReadFile(resultPath); readErr != nil {
		t.Fatal(readErr)
	} else if string(current) != string(expected) {
		t.Fatalf("canonical bytes changed: %q", current)
	}
	if current, readErr := os.ReadFile(quarantinePath); readErr != nil {
		t.Fatal(readErr)
	} else if string(current) != string(concurrent) {
		t.Fatalf("quarantine bytes changed: %q", current)
	}
}

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

	if err := moveReviewerResultExact(
		resultPath,
		quarantinePath,
		guardPath,
		reviewerResultExactMoveExpectation{Kind: "empty-file", Obstruction: reviewerResultObstructionSnapshot{Kind: "empty-file"}},
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
