package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func TestAcquireReviewerResultPublicationLeaseReturnsTypedHeldBeforePlanBuild(t *testing.T) {
	caseRoot := t.TempDir()
	lane := "reviewer-control-test"
	stateRoot := filepath.Join(caseRoot, ".steamai")
	laneRoot := filepath.Join(stateRoot, "lanes", lane)
	if err := os.MkdirAll(laneRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateRoot, "instance.yml"), []byte("schemaVersion: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	laneData := `{"schemaVersion":1,"id":"` + lane + `","status":"open","currentExecutor":"reviewer-member","executorGeneration":4}` + "\n"
	if err := os.WriteFile(filepath.Join(laneRoot, "lane.json"), []byte(laneData), 0o600); err != nil {
		t.Fatal(err)
	}
	owner, err := laneowner.Read(caseRoot, lane)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := executioncontrol.Preview(caseRoot, executioncontrol.Options{
		Lane: lane, Action: executioncontrol.ActionPause, Actor: "control-test",
		Reason: "pause before reviewer result plan build", PublicationStamp: "2026-08-18T17:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := executioncontrol.Apply(caseRoot, executioncontrol.Options{
		Lane: preview.Lane, Action: preview.Action, Actor: preview.Actor, Reason: preview.Reason,
		PublicationStamp: preview.PublicationStamp, ExpectedPlanSHA256: preview.ExpectedPlanSHA256,
	}); err != nil {
		t.Fatal(err)
	}
	capability, err := capabilitycontract.Bind(capabilitycontract.ReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	publication := executioncontrol.ResultPublicationOptions{
		Lane: lane, Birth: executioncontrol.ResultBirth{
			SchemaVersion:     executioncontrol.ResultBirthSchemaVersion,
			ControlGeneration: 0, Owner: owner, Capability: capability,
		},
		Source: executioncontrol.ResultSource{
			Kind: "host-owned-claude-structured-output", Ref: "claude-result:reviewer:attempt:session",
			SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Bytes: 32,
			SessionKind: "reviewer", AttemptID: "attempt", AttemptSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SessionID: "session",
		},
		Actor: "control-test", ObservedAt: "2026-08-18T17:01:00Z",
	}
	opt := Options{Apply: true, currentLoopReviewerResultPublication: &publication}
	release, err := acquireReviewerResultPublicationLease(runtime.Context{Target: caseRoot, TargetProvided: true}, &opt)
	var held *executioncontrol.ResultHeldError
	if !errors.As(err, &held) || held.Publication.Disposition != executioncontrol.ResultDispositionHeldWhilePaused {
		t.Fatalf("publication owner error = %v", err)
	}
	if release != nil || opt.currentLoopReviewerMutationLease != nil || opt.currentLoopReviewerResultPublished != nil {
		t.Fatalf("held precheck retained mutation ownership: release=%v lease=%v published=%v", release != nil, opt.currentLoopReviewerMutationLease != nil, opt.currentLoopReviewerResultPublished != nil)
	}
	if held.Publication.ReceiptPath == "" || held.Publication.ReceiptSHA256 == "" {
		t.Fatalf("held precheck omitted durable receipt: %+v", held.Publication)
	}
}
