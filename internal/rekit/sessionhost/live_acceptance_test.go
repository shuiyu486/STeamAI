package sessionhost

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
)

func TestRunLiveAcceptanceRejectsMissingInputsWithoutCreatingCase(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-case")
	if _, err := RunLiveAcceptance(context.Background(), LiveAcceptanceOptions{CaseRoot: caseRoot, Goal: "goal"}); err == nil || !strings.Contains(err.Error(), "human correction") {
		t.Fatalf("missing correction error=%v", err)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("invalid live gate created case root: %v", err)
	}
}

func TestRunLiveAcceptanceRejectsExistingCase(t *testing.T) {
	caseRoot := t.TempDir()
	if _, err := RunLiveAcceptance(context.Background(), LiveAcceptanceOptions{CaseRoot: caseRoot, Goal: "goal", Correction: "correction"}); err == nil || !strings.Contains(err.Error(), "non-existing fresh case root") {
		t.Fatalf("existing case error=%v", err)
	}
}

func TestRunLiveAcceptanceRejectsReceiptInsideDisposableCase(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-case")
	receiptPath := filepath.Join(caseRoot, "receipt.json")
	if _, err := RunLiveAcceptance(context.Background(), LiveAcceptanceOptions{CaseRoot: caseRoot, Goal: "goal", Correction: "correction", ReceiptPath: receiptPath}); err == nil || !strings.Contains(err.Error(), "receipt must be outside") {
		t.Fatalf("receipt containment error=%v", err)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("invalid receipt path created case root: %v", err)
	}
}

func TestAddLiveAcceptanceSessionsSeparatesLaunchesFromRecoveredCompletions(t *testing.T) {
	receipt := LiveAcceptanceReceipt{}
	addLiveAcceptanceSessions(&receipt, Result{Sessions: []Session{
		{Started: true, AttemptGeneration: 2, RunLaunchOrdinal: 3, SessionID: "member-session", SessionKind: "member", Outcome: "returned"},
		{Recovered: true, AttemptGeneration: 1, SessionID: "reviewer-session", SessionKind: "reviewer", Outcome: "returned-recovered"},
	}}, 2)
	if receipt.MemberLaunches != 1 || receipt.MemberCompletions != 1 || receipt.ReviewerLaunches != 0 || receipt.ReviewerCompletions != 1 {
		t.Fatalf("session lifecycle counts=%+v", receipt)
	}
	if len(receipt.MemberSessions) != 1 || !receipt.MemberSessions[0].Started || receipt.MemberSessions[0].AttemptGeneration != 2 || receipt.MemberSessions[0].HostRun != 2 || receipt.MemberSessions[0].RunLaunchOrdinal != 3 || len(receipt.ReviewerSessions) != 1 || !receipt.ReviewerSessions[0].Recovered || receipt.ReviewerSessions[0].Started || receipt.ReviewerSessions[0].HostRun != 2 || receipt.ReviewerSessions[0].RunLaunchOrdinal != 0 {
		t.Fatalf("session lifecycle records=%+v", receipt)
	}
}

func TestBindLiveAcceptanceOwnerGenerationUsesLatestUnboundMember(t *testing.T) {
	sessions := []LiveAcceptanceSession{
		{Kind: "member", OwnerGeneration: 1, AttemptGeneration: 1},
		{Kind: "reviewer", AttemptGeneration: 1},
		{Kind: "member", AttemptGeneration: 1},
	}
	bindLiveAcceptanceOwnerGeneration(sessions, memberexecution.Inspection{Owner: memberexecution.Owner{ExecutorGeneration: 2}})
	if sessions[0].OwnerGeneration != 1 || sessions[1].OwnerGeneration != 0 || sessions[2].OwnerGeneration != 2 {
		t.Fatalf("owner generations=%+v", sessions)
	}
}

func TestWriteLiveAcceptanceReceiptRecordsFinalCleanupState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := LiveAcceptanceReceipt{SchemaVersion: 1, Kind: "rekit-" + liveAcceptancePack + "-live-acceptance-receipt", Passed: true, Pack: liveAcceptancePack, Cleanup: "removed"}
	if err := WriteLiveAcceptanceReceipt(path, receipt); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded LiveAcceptanceReceipt
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Passed || decoded.Pack != liveAcceptancePack || decoded.Cleanup != "removed" {
		t.Fatalf("receipt=%+v", decoded)
	}
}
