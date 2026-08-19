package executioncontrol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
)

func TestPublishResultLinearizesCanonicalOrHeldByExactControlHead(t *testing.T) {
	for _, fixture := range []struct {
		name        string
		mutate      func(*testing.T, string)
		disposition string
		published   bool
	}{
		{name: "exact-running", published: true, disposition: ResultDispositionPublished},
		{
			name: "paused",
			mutate: func(t *testing.T, caseRoot string) {
				applyControl(t, caseRoot, ActionPause, "hold late result", "2026-08-18T15:00:00Z", 1, StatePaused)
			},
			disposition: ResultDispositionHeldWhilePaused,
		},
		{
			name: "resumed-new-head",
			mutate: func(t *testing.T, caseRoot string) {
				applyControl(t, caseRoot, ActionPause, "hold prior result", "2026-08-18T15:00:00Z", 1, StatePaused)
				applyControl(t, caseRoot, ActionResume, "resume future work", "2026-08-18T15:01:00Z", 2, StateRunning)
			},
			disposition: ResultDispositionStaleControl,
		},
		{
			name: "stopped",
			mutate: func(t *testing.T, caseRoot string) {
				applyControl(t, caseRoot, ActionStop, "stop prior result", "2026-08-18T15:00:00Z", 1, StateStopped)
			},
			disposition: ResultDispositionLateAfterStop,
		},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := controlFixture(t, false)
			owner, err := laneowner.Read(caseRoot, testLane)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.mutate != nil {
				fixture.mutate(t, caseRoot)
			}
			calls := 0
			result, err := PublishResult(caseRoot, ResultPublicationOptions{
				Lane:  testLane,
				Birth: ResultBirth{ControlGeneration: 0, Owner: owner},
				Source: ResultSource{
					Kind: "host-owned-claude-result", Ref: "attempt:test/session:test",
					SHA256: strings.Repeat("a", 64), Bytes: 128, SessionKind: "member",
					AttemptID: "attempt-test", AttemptSHA256: strings.Repeat("b", 64), SessionID: "session-test",
				},
				Actor: "result-test", ObservedAt: "2026-08-18T15:02:00Z",
			}, func() error {
				calls++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Published != fixture.published || result.Held == fixture.published || result.Disposition != fixture.disposition {
				t.Fatalf("publication = %+v", result)
			}
			wantCalls := 0
			if fixture.published {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("canonical callback calls = %d, want %d", calls, wantCalls)
			}
			if fixture.published {
				if result.ReceiptPath != "" {
					t.Fatalf("canonical publication wrote held receipt: %+v", result)
				}
				return
			}
			data, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(result.ReceiptPath)))
			if err != nil {
				t.Fatal(err)
			}
			var receipt HeldResultReceipt
			if err := json.Unmarshal(data, &receipt); err != nil {
				t.Fatal(err)
			}
			if receipt.Advanced || receipt.CanonicalPublication || !receipt.NoAuthority || !receipt.NoConfirmed || !receipt.NoHeavyTool || !receipt.NoAutoResume || receipt.Disposition != fixture.disposition {
				t.Fatalf("held receipt crossed its boundary: %+v", receipt)
			}
			inspection, err := Inspect(caseRoot, testLane)
			if err != nil {
				t.Fatalf("held receipt broke control inspection: %v", err)
			}
			if inspection.State == "" {
				t.Fatal("held receipt erased control state")
			}
			if fixture.name == "paused" {
				applyControl(t, caseRoot, ActionResume, "resume only future results", "2026-08-18T15:03:00Z", 2, StateRunning)
			}
			replayed, err := PublishResult(caseRoot, ResultPublicationOptions{
				Lane:  testLane,
				Birth: ResultBirth{ControlGeneration: 0, Owner: owner},
				Source: ResultSource{
					Kind: "host-owned-claude-result", Ref: "attempt:test/session:test",
					SHA256: strings.Repeat("a", 64), Bytes: 128, SessionKind: "member",
					AttemptID: "attempt-test", AttemptSHA256: strings.Repeat("b", 64), SessionID: "session-test",
				},
				Actor: "result-test", ObservedAt: "2026-08-18T15:02:00Z",
			}, func() error {
				calls++
				return nil
			})
			if err != nil || replayed.ReceiptPath != result.ReceiptPath || replayed.ReceiptSHA256 != result.ReceiptSHA256 || calls != 0 {
				t.Fatalf("held replay = %+v calls=%d err=%v", replayed, calls, err)
			}
		})
	}
}

func TestPrepareResultClassifiesWithoutCanonicalPublication(t *testing.T) {
	for _, fixture := range []struct {
		name        string
		pause       bool
		disposition string
		held        bool
	}{
		{name: "exact-running", disposition: ResultDispositionCurrent},
		{name: "paused", pause: true, disposition: ResultDispositionHeldWhilePaused, held: true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := controlFixture(t, false)
			owner, err := laneowner.Read(caseRoot, testLane)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.pause {
				applyControl(t, caseRoot, ActionPause, "prepare held result", "2026-08-18T16:20:00Z", 1, StatePaused)
			}
			result, err := PrepareResult(caseRoot, ResultPublicationOptions{
				Lane:  testLane,
				Birth: ResultBirth{ControlGeneration: 0, Owner: owner},
				Source: ResultSource{
					Kind: "host-owned-claude-result", Ref: "attempt:prepare/session:test",
					SHA256: strings.Repeat("e", 64), Bytes: 32, SessionKind: "reviewer",
					AttemptID: "prepare-attempt", AttemptSHA256: strings.Repeat("f", 64), SessionID: "prepare-session",
				},
				Actor: "prepare-test", ObservedAt: "2026-08-18T16:21:00Z",
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Published || result.Held != fixture.held || result.Disposition != fixture.disposition {
				t.Fatalf("prepared result = %+v", result)
			}
			if fixture.held && result.ReceiptPath == "" {
				t.Fatal("held preparation omitted its durable receipt")
			}
			if !fixture.held && result.ReceiptPath != "" {
				t.Fatalf("exact-current preparation wrote a receipt: %+v", result)
			}
		})
	}
}

func TestPublishResultWithLeaseRejectsDifferentLaneLease(t *testing.T) {
	caseRoot := controlFixture(t, false)
	otherLane := "binary-analysis-review"
	otherLaneRoot := filepath.Join(caseRoot, ".steamai", "lanes", otherLane)
	if err := os.MkdirAll(otherLaneRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	lane := `{"schemaVersion":1,"id":"` + otherLane + `","status":"open","currentExecutor":"reviewer-main","executorGeneration":3}` + "\n"
	if err := os.WriteFile(filepath.Join(otherLaneRoot, "lane.json"), []byte(lane), 0o600); err != nil {
		t.Fatal(err)
	}
	owner, err := laneowner.Read(caseRoot, otherLane)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := lanemutation.AcquireLane(caseRoot, testLane)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lease.Unlock(); err != nil {
			t.Errorf("unlock existing mutation lease: %v", err)
		}
	}()

	calls := 0
	_, err = PublishResultWithLease(caseRoot, lease, ResultPublicationOptions{
		Lane:  otherLane,
		Birth: ResultBirth{ControlGeneration: 0, Owner: owner},
		Source: ResultSource{
			Kind: "host-owned-claude-result", Ref: "attempt:wrong-lease/session:test",
			SHA256: strings.Repeat("1", 64), Bytes: 16, SessionKind: "reviewer",
			AttemptID: "wrong-lease-attempt", AttemptSHA256: strings.Repeat("2", 64), SessionID: "wrong-lease-session",
		},
		Actor: "wrong-lease-test", ObservedAt: "2026-08-18T16:29:00Z",
	}, func() error {
		calls++
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "different canonical lane") {
		t.Fatalf("different-lane publication error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("different-lane publication called canonical callback %d times", calls)
	}
	if err := lease.ValidateLaneFor(caseRoot, testLane); err != nil {
		t.Fatalf("original lane lease became invalid: %v", err)
	}
}

func TestPublishResultWithExistingMutationLeaseMatchesOwnerClassification(t *testing.T) {
	for _, fixture := range []struct {
		name        string
		pause       bool
		disposition string
		published   bool
	}{
		{name: "exact-running", disposition: ResultDispositionPublished, published: true},
		{name: "paused", pause: true, disposition: ResultDispositionHeldWhilePaused},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := controlFixture(t, false)
			owner, err := laneowner.Read(caseRoot, testLane)
			if err != nil {
				t.Fatal(err)
			}
			if fixture.pause {
				applyControl(t, caseRoot, ActionPause, "hold result under existing lease", "2026-08-18T16:30:00Z", 1, StatePaused)
			}
			lease, err := lanemutation.AcquireLane(caseRoot, testLane)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := lease.Unlock(); err != nil {
					t.Errorf("unlock existing mutation lease: %v", err)
				}
			}()
			calls := 0
			result, err := PublishResultWithLease(caseRoot, lease, ResultPublicationOptions{
				Lane:  testLane,
				Birth: ResultBirth{ControlGeneration: 0, Owner: owner},
				Source: ResultSource{
					Kind: "host-owned-claude-result", Ref: "attempt:existing-lease/session:test",
					SHA256: strings.Repeat("c", 64), Bytes: 64, SessionKind: "reviewer",
					AttemptID: "existing-lease-attempt", AttemptSHA256: strings.Repeat("d", 64), SessionID: "existing-lease-session",
				},
				Actor: "existing-lease-test", ObservedAt: "2026-08-18T16:31:00Z",
			}, func() error {
				calls++
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Published != fixture.published || result.Held == fixture.published ||
				result.Disposition != fixture.disposition {
				t.Fatalf("existing-lease publication = %+v", result)
			}
			wantCalls := 0
			if fixture.published {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("existing-lease callback calls = %d, want %d", calls, wantCalls)
			}
			if err := lease.ValidateFor(caseRoot); err != nil {
				t.Fatal(err)
			}
		})
	}
}
