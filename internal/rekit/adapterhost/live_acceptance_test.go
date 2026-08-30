package adapterhost

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
)

func TestRunLiveAcceptanceRejectsAcceptanceOnlyRuntimeBeforeCaseCreation(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve adapterhost test source")
	}
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	name := "adapter-acceptance-role-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	acceptanceOnly := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", acceptanceOnly, "./cmd/rekit-adapter-acceptance")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build adapter acceptance-only image: %v\n%s", err, output)
	}
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	result, err := RunLiveAcceptance(LiveAcceptanceOptions{
		RepoRoot: repoRoot, AdapterPath: acceptanceOnly,
		RuntimePath: acceptanceOnly, ReceiptPath: receiptPath,
	})
	if err == nil || !strings.Contains(err.Error(), "unified runtime executable role mismatch") {
		t.Fatalf("acceptance-only runtime result=%+v err=%v", result, err)
	}
	if result.CaseRoot != "" || result.Cleanup != "pending" {
		t.Fatalf("acceptance-only runtime crossed case creation: %+v", result)
	}
	if _, err := os.Lstat(receiptPath); !os.IsNotExist(err) {
		t.Fatalf("acceptance-only runtime wrote receipt: %v", err)
	}
}

func TestRunLiveAcceptancePublishesRunnableUnifiedRuntimeBeforeAdapterExecution(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve adapterhost test source")
	}
	repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	name := "steamai-adapter-acceptance-runtime-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	runtimePath := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", runtimePath, "./cmd/rekit")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build unified adapter acceptance runtime: %v\n%s", err, output)
	}

	stopAfterInit := "stop after verifying published adapter acceptance runtime"
	result, err := RunLiveAcceptance(LiveAcceptanceOptions{
		RepoRoot: repoRoot, AdapterPath: runtimePath,
		RuntimePath: runtimePath, ReceiptPath: filepath.Join(t.TempDir(), "receipt.json"),
		testHooks: &liveAcceptanceTestHooks{afterInit: func(caseRoot string) error {
			projectExecutable := filepath.Join(
				caseRoot, ".steamai", "runtime", "bin",
				runtimebundle.ExecutableName(),
			)
			for _, command := range []string{"help", "status"} {
				cmd := exec.Command(projectExecutable, command)
				cmd.Dir = caseRoot
				output, runErr := cmd.CombinedOutput()
				if runErr != nil {
					return fmt.Errorf("published adapter acceptance runtime %s: %w: %s", command, runErr, output)
				}
				want := "STeamAI public commands:"
				if command == "status" {
					want = "现在："
				}
				if !strings.Contains(string(output), want) {
					return fmt.Errorf("published adapter acceptance runtime %s omitted %q: %s", command, want, output)
				}
			}
			return errors.New(stopAfterInit)
		}},
	})
	if err == nil || !strings.Contains(err.Error(), stopAfterInit) {
		t.Fatalf("adapter acceptance did not reach runtime verification hook: result=%+v err=%v", result, err)
	}
	if result.CaseRoot == "" || result.Cleanup != "removed" {
		t.Fatalf("runtime verification fixture was not safely removed: %+v", result)
	}
	if _, err := os.Lstat(result.CaseRoot); !os.IsNotExist(err) {
		t.Fatalf("runtime verification case still exists: %v", err)
	}
}

func TestValidateLiveAdapterResultRejectsForgedProcessAndProvenance(t *testing.T) {
	result := Result{
		SchemaVersion: 1, Kind: "rekit-readonly-adapter-host-result",
		CaseRoot: `C:\case`, Pack: liveAcceptancePack, GateEventID: "evt-a",
		Lane: "feature-readonly-inspection", Executor: liveAcceptanceExecutor, Generation: 1,
		AdapterID: readonlyInspectorID, AdapterHarness: adapterHarness, AdapterSession: "rh06-adapter-session-1",
		ExecutableSHA256: strings.Repeat("a", 64), ProcessID: 42,
		DispatchPath:   ".rekit/lanes/feature-readonly-inspection/adapter-executions/evt-a/dispatch.json",
		DispatchSHA256: strings.Repeat("b", 64), InputPath: liveAcceptanceTarget,
		ReportPath:   liveAcceptanceReport,
		ArtifactPath: filepath.ToSlash(filepath.Join(filepath.Dir(liveAcceptanceReport), "inspection.json")),
		InputSHA256:  strings.Repeat("c", 64), ArtifactSHA256: strings.Repeat("d", 64), ReportSHA256: strings.Repeat("e", 64),
		ReadOnlyInput: true, NoNetwork: true, NoAuthority: true,
	}
	if err := validateLiveAdapterResult(result, 41, result.ExecutableSHA256, result.CaseRoot, result.Lane, result.GateEventID, result.AdapterSession, result.DispatchPath, result.DispatchSHA256); err == nil {
		t.Fatal("forged child pid was accepted")
	}
	result.ProcessID = 41
	result.ReportSHA256 = strings.Repeat("f", 63)
	if err := validateLiveAdapterResult(result, 41, result.ExecutableSHA256, result.CaseRoot, result.Lane, result.GateEventID, result.AdapterSession, result.DispatchPath, result.DispatchSHA256); err == nil {
		t.Fatal("invalid stdout provenance hash was accepted")
	}
}

func TestRunLiveAcceptanceRejectsRepositoryReceiptBeforeCaseCreation(t *testing.T) {
	repoRoot := t.TempDir()
	adapterPath := filepath.Join(t.TempDir(), "adapter.exe")
	if err := os.WriteFile(adapterPath, []byte("not executed"), 0o700); err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(repoRoot, "receipt.json")
	result, err := RunLiveAcceptance(LiveAcceptanceOptions{RepoRoot: repoRoot, AdapterPath: adapterPath, RuntimePath: adapterPath, ReceiptPath: receiptPath})
	if err == nil || !strings.Contains(err.Error(), "outside the repository") {
		t.Fatalf("repository receipt path should fail closed: result=%+v err=%v", result, err)
	}
	if result.CaseRoot != "" {
		t.Fatalf("repository receipt rejection created a disposable case: %+v", result)
	}
}

func TestWriteLiveAcceptanceReceiptRejectsExistingAndAncestorSymlink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := LiveAcceptanceReceipt{SchemaVersion: 1, Kind: "receipt", Passed: true, Cleanup: "removed"}
	if err := WriteLiveAcceptanceReceipt(path, receipt); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteLiveAcceptanceReceipt(path, LiveAcceptanceReceipt{Passed: false}); err == nil {
		t.Fatal("existing receipt was replaced")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("existing receipt changed: err=%v", err)
	}

	base := t.TempDir()
	realParent := filepath.Join(base, "real-parent")
	if err := os.Mkdir(realParent, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedParent := filepath.Join(base, "linked-parent")
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	err = WriteLiveAcceptanceReceipt(filepath.Join(linkedParent, "nested", "receipt.json"), receipt)
	if err == nil || (!strings.Contains(err.Error(), "symlink") && !strings.Contains(err.Error(), "reparse point")) {
		t.Fatalf("ancestor symlink receipt error=%v", err)
	}
}
