package bootstrapcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBootstrapRejectsNonWindowsBeforeTargetMutation(t *testing.T) {
	previous := currentGOOS
	currentGOOS = "linux"
	t.Cleanup(func() { currentGOOS = previous })
	target := t.TempDir()
	path := filepath.Join(target, "keep.txt")
	if err := os.WriteFile(path, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Execute(strings.NewReader("APPLY\n"), &bytes.Buffer{}, t.TempDir(), filepath.Join(t.TempDir(), "missing.exe"), Options{
		Target: target, Goal: "inspect", Pack: "binary-re",
	})
	if err == nil || !strings.Contains(err.Error(), "only on Windows") {
		t.Fatalf("non-Windows bootstrap error = %v", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "keep\n" {
		t.Fatalf("non-Windows bootstrap changed target: %q err=%v", data, readErr)
	}
}

func TestReadConfirmationRequiresExactApplyToken(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{input: "APPLY\n", want: "APPLY"},
		{input: "apply\n", want: "apply"},
		{input: "cancel\n", want: "cancel"},
		{input: "", want: ""},
	} {
		got, err := readConfirmation(strings.NewReader(test.input))
		if err != nil || got != test.want {
			t.Fatalf("confirmation %q = %q err=%v", test.input, got, err)
		}
	}
}

func TestBootstrapJSONModeReturnsPreviewWithoutReadingConfirmation(t *testing.T) {
	previous := currentGOOS
	currentGOOS = "linux"
	t.Cleanup(func() { currentGOOS = previous })

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-format", "json"}, strings.NewReader("APPLY\n"), &stdout, &stderr, t.TempDir(), filepath.Join(t.TempDir(), "missing.exe"))
	if code != 1 || !strings.Contains(stderr.String(), "only on Windows") || stdout.Len() != 0 {
		t.Fatalf("JSON bootstrap platform fence: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestValidateBootstrapGoalBoundsUTF8Bytes(t *testing.T) {
	if _, err := validateBootstrapGoal(strings.Repeat("x", 4096)); err != nil {
		t.Fatalf("4096-byte goal rejected: %v", err)
	}
	if _, err := validateBootstrapGoal(strings.Repeat("x", 4097)); err == nil {
		t.Fatal("oversized bootstrap goal was accepted")
	}
	if _, err := validateBootstrapGoal(string([]byte{0xff})); err == nil {
		t.Fatal("invalid UTF-8 bootstrap goal was accepted")
	}
}

func TestBootstrapCurrentPlatformMarker(t *testing.T) {
	if currentGOOS != runtime.GOOS {
		t.Fatalf("bootstrap platform marker = %s want %s", currentGOOS, runtime.GOOS)
	}
}
