package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/hostcmd"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
)

func TestStandaloneHostRefusesProjectInitializationWithoutWrites(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve rekit-host test source")
	}
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	name := "rekit-host-role-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	hostPath := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", hostPath, "./cmd/rekit-host")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build standalone host: %v\n%s", err, output)
	}

	for _, test := range []struct {
		name     string
		existing bool
		args     func(string) []string
	}{
		{
			name:     "ordinary directory adoption",
			existing: true,
			args: func(caseRoot string) []string {
				return []string{
					"-daily", "-target", caseRoot,
					"-directory-adoption-action", "initialize-in-place",
				}
			},
		},
		{
			name: "fresh goal onboarding",
			args: func(caseRoot string) []string {
				return []string{
					"-daily", "-target", caseRoot,
					"-goal", "inspect this fresh project",
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			parent := t.TempDir()
			caseRoot := filepath.Join(parent, "project")
			if test.existing {
				if err := os.MkdirAll(caseRoot, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command(hostPath, test.args(caseRoot)...)
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "verified unified runtime executable source") {
				t.Fatalf("standalone host initialization err=%v output=%q", err, output)
			}
			if test.existing {
				entries, err := os.ReadDir(caseRoot)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 0 {
					t.Fatalf("standalone host initialization wrote project state: %+v", entries)
				}
			} else if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
				t.Fatalf("standalone host initialization wrote project state: %v", err)
			}
		})
	}
}

func TestPublishLiveAcceptanceReceiptFailureClearsPassed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(path, []byte("existing evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := hostcmd.PublishLiveAcceptanceReceipt(path, sessionhost.LiveAcceptanceReceipt{Passed: true}, nil)
	if err == nil || result.Passed || result.ReceiptPublication != "failed" || !strings.Contains(result.ReceiptError, "already exists") {
		t.Fatalf("publication failure result=%+v err=%v", result, err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "existing evidence\n" {
		t.Fatalf("existing receipt changed: %q err=%v", data, readErr)
	}
}

func TestPublishLiveAcceptanceReceiptPersistsPublishedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	result, err := hostcmd.PublishLiveAcceptanceReceipt(path, sessionhost.LiveAcceptanceReceipt{Passed: true}, nil)
	if err != nil || !result.Passed || result.ReceiptPublication != "published" || result.ReceiptError != "" {
		t.Fatalf("publication success result=%+v err=%v", result, err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || !strings.Contains(string(data), `"receiptPublication": "published"`) {
		t.Fatalf("durable receipt omitted publication state: %q err=%v", data, readErr)
	}
}

func TestPublishLiveSoakAcceptanceReceiptFailureClearsPassed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(path, []byte("existing evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := hostcmd.PublishLiveSoakAcceptanceReceipt(path, sessionhost.LiveSoakAcceptanceReceipt{Passed: true}, nil)
	if err == nil || result.Passed || result.ReceiptPublication != "failed" || !strings.Contains(result.ReceiptError, "already exists") {
		t.Fatalf("publication failure result=%+v err=%v", result, err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "existing evidence\n" {
		t.Fatalf("existing receipt changed: %q err=%v", data, readErr)
	}
}

func TestPublishLiveSoakAcceptanceReceiptPersistsPublishedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	result, err := hostcmd.PublishLiveSoakAcceptanceReceipt(path, sessionhost.LiveSoakAcceptanceReceipt{Passed: true}, nil)
	if err != nil || !result.Passed || result.ReceiptPublication != "published" || result.ReceiptError != "" {
		t.Fatalf("publication success result=%+v err=%v", result, err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || !strings.Contains(string(data), `"receiptPublication": "published"`) {
		t.Fatalf("durable soak receipt omitted publication state: %q err=%v", data, readErr)
	}
}

func TestValidateAdapterFlagIsolatedToBinaryRELiveAcceptance(t *testing.T) {
	adapter := filepath.Join(t.TempDir(), "rekit-adapter-host.exe")
	for name, test := range map[string]struct {
		live    bool
		pack    string
		adapter string
		wantErr string
	}{
		"binary default requires adapter":  {live: true, wantErr: "requires -adapter"},
		"binary explicit requires adapter": {live: true, pack: "binary-re", wantErr: "requires -adapter"},
		"binary accepts adapter":           {live: true, pack: "binary-re", adapter: adapter},
		"cross pack omits adapter":         {live: true, pack: "web-security"},
		"cross pack may bind adapter":      {live: true, pack: "web-security", adapter: adapter},
		"daily rejects adapter":            {adapter: adapter, wantErr: "only by -live-acceptance"},
	} {
		t.Run(name, func(t *testing.T) {
			err := hostcmd.ValidateAdapterFlag(test.live, test.pack, test.adapter)
			if test.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("err=%v want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestOrdinaryHostOptionsRequireCurrentDriverRequest(t *testing.T) {
	opt := sessionhost.Options{}
	opt.RequireCurrentDriverRequest()
	if strings.TrimSpace(opt.ExpectedCurrentDriverRequestSHA256) != "" {
		t.Fatalf("requiring a request should not fabricate its identity: %+v", opt)
	}
}

func TestExpectedCurrentDriverRequestFlagIsOrdinaryHostOnly(t *testing.T) {
	if err := hostcmd.ValidateExpectedCurrentDriverRequestFlag(strings.Repeat("a", 64), true); err != nil {
		t.Fatal(err)
	}
	if err := hostcmd.ValidateExpectedCurrentDriverRequestFlag("", false); err != nil {
		t.Fatal(err)
	}
	if err := hostcmd.ValidateExpectedCurrentDriverRequestFlag(strings.Repeat("a", 64), false); err == nil || !strings.Contains(err.Error(), "ordinary rekit-host mode") {
		t.Fatalf("specialized host mode accepted case request identity: %v", err)
	}
}

func TestPublicModeRequestedIncludesLiveSoak(t *testing.T) {
	if hostcmd.PublicModeRequested(false, false, false, false, false) {
		t.Fatal("empty public mode set was selected")
	}
	if !hostcmd.PublicModeRequested(false, false, false, false, true) {
		t.Fatal("live soak public mode was not selected")
	}
}
