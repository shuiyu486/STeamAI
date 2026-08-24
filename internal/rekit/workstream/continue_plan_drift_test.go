package workstream

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
)

func TestContinueApplyRejectsSameGenerationOutboxDriftWithoutWrites(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	opt := ContinueOptions{
		Selector:                   "devirt-main",
		Executor:                   "executor-one",
		ExpectedExecutorGeneration: 1,
	}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.ContinuePlanSHA256) != 64 {
		t.Fatalf("continue preview omitted plan identity: %+v", preview)
	}
	outbox := filepath.Join(caseRoot, ".rekit", "lanes", "devirt-main", "outbox.jsonl")
	outboxBefore, err := os.ReadFile(outbox)
	if err != nil {
		t.Fatal(err)
	}
	before := snapshotWorkstreamTree(t, caseRoot)
	file, err := os.OpenFile(outbox, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"eventId":"evt-continue-plan-drift","kind":"observation","summary":"same generation output after review","opaque":{"unprojected":"must bind raw event"}}` + "\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	_, err = ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	failure, typed := plancontract.FromError(err)
	if err == nil || !typed || failure.Code != plancontract.CodePlanMismatch || failure.MutationApplied || failure.MutationBoundary != "none" || !IsZeroProgress(err) {
		t.Fatalf("same-generation outbox drift error = %v failure=%+v typed=%t", err, failure, typed)
	}
	outboxAfter, readErr := os.ReadFile(outbox)
	if readErr != nil {
		t.Fatal(readErr)
	}
	appended := `{"eventId":"evt-continue-plan-drift","kind":"observation","summary":"same generation output after review","opaque":{"unprojected":"must bind raw event"}}` + "\n"
	if string(outboxAfter) != string(outboxBefore)+appended {
		t.Fatalf("stale Apply changed outbox beyond the intentional drift: %s", outboxAfter)
	}
	after := snapshotWorkstreamTree(t, caseRoot)
	beforeWithoutOutbox := strings.Replace(before, "lanes/devirt-main/outbox.jsonl="+string(outboxBefore), "", 1)
	afterWithoutOutbox := strings.Replace(after, "lanes/devirt-main/outbox.jsonl="+string(outboxAfter), "", 1)
	if afterWithoutOutbox != beforeWithoutOutbox {
		t.Fatalf("stale Apply changed files beyond the intentional outbox drift\nbefore:\n%s\nafter:\n%s", before, after)
	}
	entries, readErr := os.ReadDir(filepath.Join(caseRoot, ".rekit", "runs"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("stale Apply created run artifacts: %+v", entries)
	}
}
