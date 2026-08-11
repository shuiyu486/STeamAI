package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
)

func TestPrepareVMPIDAIndexRequestPreviewAndHashBoundApply(t *testing.T) {
	caseRoot := t.TempDir()
	writeCLIFile(t, filepath.Join(caseRoot, filepath.FromSlash(adapterhost.VMPIDAIndexQueryPath)), `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":3}`)
	writeCLIFile(t, filepath.Join(caseRoot, filepath.FromSlash(adapterhost.VMPIDAIndexDefaultExportRoot), "function_index.tsv"), "rva\tname\n0x1000\tneedle\n")

	code, stdout, stderr := captureRun(t, []string{"-prepare-vmp-ida-index-request", "-target", caseRoot})
	if code != 0 || stderr != "" {
		t.Fatalf("preview code=%d stderr=%s", code, stderr)
	}
	var preview adapterhost.VMPIDAIndexRequestPreview
	if err := json.Unmarshal([]byte(stdout), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.RequestPath == "" || preview.RequestSHA256 == "" {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(preview.RequestPath))); !os.IsNotExist(err) {
		t.Fatalf("preview wrote request: %v", err)
	}

	code, _, stderr = captureRun(t, []string{"-prepare-vmp-ida-index-request", "-target", caseRoot, "-apply", "-expected-request-sha256", strings.Repeat("a", 64)})
	if code != 1 || !strings.Contains(stderr, "sha256 mismatch") {
		t.Fatalf("wrong apply code=%d stderr=%s", code, stderr)
	}
	code, stdout, stderr = captureRun(t, []string{"-prepare-vmp-ida-index-request", "-target", caseRoot, "-apply", "-expected-request-sha256", preview.RequestSHA256})
	if code != 0 || stderr != "" {
		t.Fatalf("apply code=%d stderr=%s", code, stderr)
	}
	var published adapterhost.VMPIDAIndexRequestPublication
	if err := json.Unmarshal([]byte(stdout), &published); err != nil {
		t.Fatal(err)
	}
	if published.RequestPath != preview.RequestPath || published.RequestSHA256 != preview.RequestSHA256 {
		t.Fatalf("published = %+v preview=%+v", published, preview)
	}
	if _, err := adapterhost.ReadVMPIDAIndexRequest(caseRoot, preview.RequestPath); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareVMPIDAIndexRequestAcceptsTypedLiteralTermsWithoutQueryFile(t *testing.T) {
	caseRoot := t.TempDir()
	writeCLIFile(t, filepath.Join(caseRoot, filepath.FromSlash(adapterhost.VMPIDAIndexDefaultExportRoot), "function_index.tsv"), "rva\tname\n0x1000\tneedle_dispatch\n")
	code, stdout, stderr := captureRun(t, []string{
		"-prepare-vmp-ida-index-request", "-target", caseRoot,
		"-terms", "needle, Dispatcher", "-max-rows-per-index", "7",
	})
	if code != 0 || stderr != "" {
		t.Fatalf("typed preview code=%d stderr=%s", code, stderr)
	}
	var preview adapterhost.VMPIDAIndexRequestPreview
	if err := json.Unmarshal([]byte(stdout), &preview); err != nil {
		t.Fatal(err)
	}
	if strings.Join(preview.Request.Query.Terms, ",") != "needle,Dispatcher" || preview.Request.Query.MaxRowsPerIndex != 7 {
		t.Fatalf("typed preview query = %+v", preview.Request.Query)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(adapterhost.VMPIDAIndexQueryPath))); !os.IsNotExist(err) {
		t.Fatalf("typed preview wrote mutable query file: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(preview.RequestPath))); !os.IsNotExist(err) {
		t.Fatalf("typed preview wrote request: %v", err)
	}
}

func TestPrivateVMPIDAChildRejectsUnboundInvocation(t *testing.T) {
	caseRoot := t.TempDir()
	writeCLIFile(t, filepath.Join(caseRoot, filepath.FromSlash(adapterhost.VMPIDAIndexQueryPath)), `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":3}`)
	writeCLIFile(t, filepath.Join(caseRoot, filepath.FromSlash(adapterhost.VMPIDAIndexDefaultExportRoot), "function_index.tsv"), "rva\tname\n0x1000\tneedle\n")
	preview, err := adapterhost.PreviewVMPIDAIndexRequest(caseRoot, adapterhost.VMPIDAIndexDefaultExportRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapterhost.PublishVMPIDAIndexRequest(caseRoot, preview); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := captureRun(t, []string{
		"-child-vmp-ida-index-inspector",
		"-target", caseRoot,
		"-child-request-path", preview.RequestPath,
	})
	if code != 1 || !strings.Contains(stdout, `"schemaVersion": 0`) || !strings.Contains(stderr, "requires exact") {
		t.Fatalf("unbound child code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

func TestPrivateAdapterModesAreMutuallyExclusive(t *testing.T) {
	code, _, stderr := captureRun(t, []string{"-prepare-vmp-ida-index-request", "-child-vmp-ida-index-inspector", "-target", t.TempDir()})
	if code != 2 || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	code, _, stderr = captureRun(t, []string{"-child-vmp-ida-index-inspector", "-target", t.TempDir(), "-actor", "unexpected", "-child-request-path", "request.json"})
	if code != 2 || !strings.Contains(stderr, "cannot be combined") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
}

func captureRun(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	code := run(args)
	_ = outW.Close()
	_ = errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	var stdout, stderr bytes.Buffer
	if _, err := io.Copy(&stdout, outR); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(&stderr, errR); err != nil {
		t.Fatal(err)
	}
	_ = outR.Close()
	_ = errR.Close()
	return code, stdout.String(), stderr.String()
}

func writeCLIFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
