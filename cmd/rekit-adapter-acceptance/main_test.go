package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAcceptanceOnlyRuntimeRoleMismatchDoesNotPublishReceipt(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve adapter acceptance test source")
	}
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	name := "rekit-adapter-acceptance-role-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	acceptanceOnly := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", acceptanceOnly, "./cmd/rekit-adapter-acceptance")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build adapter acceptance image: %v\n%s", err, output)
	}
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	command := exec.Command(acceptanceOnly,
		"-repo", repoRoot,
		"-adapter", acceptanceOnly,
		"-runtime", acceptanceOnly,
		"-receipt", receiptPath,
	)
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "unified runtime executable role mismatch") {
		t.Fatalf("acceptance-only runtime result err=%v output=%s", err, output)
	}
	if _, err := os.Lstat(receiptPath); !os.IsNotExist(err) {
		t.Fatalf("role mismatch published receipt: %v", err)
	}
}
