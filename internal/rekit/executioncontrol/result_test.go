package executioncontrol

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/capabilitycontract"
	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func testResultBirth(owner laneowner.Snapshot) ResultBirth {
	capability, err := capabilitycontract.Bind(capabilitycontract.Transport())
	if err != nil {
		panic(err)
	}
	return ResultBirth{
		SchemaVersion: ResultBirthSchemaVersion,
		Owner:         owner,
		Capability:    capability,
	}
}

func TestDecodeVersionedResultBirthKeepsLegacyOutOfPublication(t *testing.T) {
	owner := laneowner.Snapshot{Lane: testLane, CurrentExecutor: "member-main", ExecutorGeneration: 7}
	legacy := `{"controlGeneration":0,"owner":{"lane":"` + testLane + `","currentExecutor":"member-main","executorGeneration":7}}`
	decoded, err := DecodeVersionedResultBirth([]byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Version != LegacyResultBirthSchemaVersion || decoded.Legacy == nil || decoded.Current != nil || decoded.WholeSHA256 != hash([]byte(legacy)) || !bytes.Equal(decoded.Raw, []byte(legacy)) {
		t.Fatalf("legacy result birth decode = %+v", decoded)
	}
	current := testResultBirth(owner)
	currentData, err := json.Marshal(current)
	if err != nil {
		t.Fatal(err)
	}
	currentDecoded, err := DecodeVersionedResultBirth(currentData)
	if err != nil || currentDecoded.Version != ResultBirthSchemaVersion || currentDecoded.Current == nil || currentDecoded.Legacy != nil {
		t.Fatalf("current result birth decode = %+v err=%v", currentDecoded, err)
	}
}

func TestReadHeldResultHistoryKeepsLegacyDecodeOnly(t *testing.T) {
	caseRoot := controlFixture(t, false)
	owner, err := laneowner.Read(caseRoot, testLane)
	if err != nil {
		t.Fatal(err)
	}
	source := ResultSource{
		Kind: "host-owned-claude-result", Ref: "attempt:legacy/session:legacy",
		SHA256: strings.Repeat("a", 64), Bytes: 24, SessionKind: "member",
		AttemptID: "legacy-attempt", AttemptSHA256: strings.Repeat("b", 64), SessionID: "legacy-session",
	}
	legacy := LegacyHeldResultReceipt{
		SchemaVersion: LegacyHeldResultReceiptSchemaVersion,
		Kind:          "lane-execution-held-result",
		Lane:          testLane,
		Birth: LegacyResultBirth{
			ControlGeneration: 0,
			Owner:             owner,
		},
		ArrivalControlGeneration: 0,
		ArrivalControlState:      StatePaused,
		ArrivalOwner:             &owner,
		Source:                   source,
		Disposition:              ResultDispositionHeldWhilePaused,
		Actor:                    "legacy-result-test",
		ObservedAt:               "2026-08-18T15:02:00Z",
		Reason:                   "lane execution is paused",
		Advanced:                 false,
		CanonicalPublication:     false,
		NoAuthority:              true,
		NoConfirmed:              true,
		NoHeavyTool:              true,
		NoAutoResume:             true,
	}
	legacyData, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	identityData, err := json.Marshal(struct {
		Lane   string            `json:"lane"`
		Birth  LegacyResultBirth `json:"birth"`
		Source ResultSource      `json:"source"`
	}{Lane: testLane, Birth: legacy.Birth, Source: source})
	if err != nil {
		t.Fatal(err)
	}
	legacy.ReceiptPath, err = projectstate.Rel(caseRoot, "lanes", testLane, controlDir, heldResultDir, hash(identityData)+".json")
	if err != nil {
		t.Fatal(err)
	}
	legacyData, err = json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(caseRoot, filepath.FromSlash(legacy.ReceiptPath))
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, legacyData, 0o600); err != nil {
		t.Fatal(err)
	}

	history, found, err := ReadHeldResultHistory(caseRoot, legacy.ReceiptPath)
	if err != nil || !found || history.Version != LegacyHeldResultReceiptSchemaVersion ||
		history.Legacy == nil || history.Current != nil || history.WholeSHA256 != hash(legacyData) ||
		!bytes.Equal(history.Raw, legacyData) {
		t.Fatalf("legacy held result history = %+v found=%t err=%v", history, found, err)
	}
	if history.Legacy.Birth.Owner != owner || history.Legacy.ReceiptPath != legacy.ReceiptPath {
		t.Fatalf("legacy held result identity changed: %+v", history.Legacy)
	}

	currentOptions := ResultPublicationOptions{
		Lane: testLane, Birth: testResultBirth(owner), Source: source,
		Actor: "current-result-test", ObservedAt: "2026-08-18T15:03:00Z",
	}
	canonicalCalls := 0
	if _, err := PublishResult(caseRoot, currentOptions, func() error {
		canonicalCalls++
		return nil
	}); err == nil || !strings.Contains(err.Error(), "decode-only") {
		t.Fatalf("current publication accepted legacy held receipt: %v", err)
	}
	if canonicalCalls != 0 {
		t.Fatalf("legacy held receipt crossed canonical publication boundary %d times", canonicalCalls)
	}
	unchanged, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unchanged, legacyData) {
		t.Fatalf("legacy held receipt bytes changed during current publication attempt")
	}
}

func TestDecodeVersionedHeldResultReceiptRejectsLegacyBoundaryDrift(t *testing.T) {
	owner := laneowner.Snapshot{Lane: testLane, CurrentExecutor: "member-main", ExecutorGeneration: 7}
	receipt := LegacyHeldResultReceipt{
		SchemaVersion: LegacyHeldResultReceiptSchemaVersion, Kind: "lane-execution-held-result", Lane: testLane,
		Birth: LegacyResultBirth{Owner: owner}, ArrivalControlState: StatePaused, ArrivalOwner: &owner,
		Source:      ResultSource{Kind: "result", Ref: "legacy-ref", SHA256: strings.Repeat("c", 64), Bytes: 1, SessionKind: "member", AttemptID: "attempt", AttemptSHA256: strings.Repeat("d", 64), SessionID: "session"},
		Disposition: ResultDispositionHeldWhilePaused, Actor: "legacy-test", ObservedAt: "2026-08-18T15:02:00Z", Reason: "paused", ReceiptPath: ".steamai/lanes/" + testLane + "/execution-control/held-results/legacy.json",
		NoAuthority: true, NoConfirmed: true, NoHeavyTool: true, NoAutoResume: true,
	}
	for _, fixture := range []struct {
		name   string
		mutate func(*LegacyHeldResultReceipt)
	}{
		{name: "authority", mutate: func(value *LegacyHeldResultReceipt) { value.NoAuthority = false }},
		{name: "canonical publication", mutate: func(value *LegacyHeldResultReceipt) { value.CanonicalPublication = true }},
		{name: "owner", mutate: func(value *LegacyHeldResultReceipt) { value.ArrivalOwner.ExecutorGeneration = 0 }},
		{name: "source hash", mutate: func(value *LegacyHeldResultReceipt) { value.Source.SHA256 = "bad" }},
		{name: "disposition", mutate: func(value *LegacyHeldResultReceipt) { value.Disposition = ResultDispositionPublished }},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			drifted := receipt
			ownerCopy := *receipt.ArrivalOwner
			drifted.ArrivalOwner = &ownerCopy
			fixture.mutate(&drifted)
			data, err := json.Marshal(drifted)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeVersionedHeldResultReceipt(data); err == nil {
				t.Fatal("legacy held receipt boundary drift was accepted")
			}
		})
	}
}

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
				Birth: testResultBirth(owner),
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
			if err := capabilitycontract.RequireBindingPolicy(receipt.Capability, capabilitycontract.PolicyClassTransport); err != nil {
				t.Fatalf("held receipt capability contract drifted: %v", err)
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
				Birth: testResultBirth(owner),
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

func TestHeldResultRejectsCapabilityHashDrift(t *testing.T) {
	caseRoot := controlFixture(t, false)
	owner, err := laneowner.Read(caseRoot, testLane)
	if err != nil {
		t.Fatal(err)
	}
	applyControl(t, caseRoot, ActionPause, "hold capability-bound result", "2026-08-18T15:10:00Z", 1, StatePaused)
	options := ResultPublicationOptions{
		Lane: testLane, Birth: testResultBirth(owner),
		Source: ResultSource{Kind: "host-owned-claude-result", Ref: "attempt:capability/session:test", SHA256: strings.Repeat("3", 64), Bytes: 32, SessionKind: "reviewer", AttemptID: "capability-attempt", AttemptSHA256: strings.Repeat("4", 64), SessionID: "capability-session"},
		Actor:  "capability-test", ObservedAt: "2026-08-18T15:11:00Z",
	}
	result, err := PrepareResult(caseRoot, options)
	if err != nil || !result.Held {
		t.Fatalf("prepare capability-bound held result: result=%+v err=%v", result, err)
	}
	path := filepath.Join(caseRoot, filepath.FromSlash(result.ReceiptPath))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipt HeldResultReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Capability.SHA256 = strings.Repeat("5", 64)
	data, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareResult(caseRoot, options); err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("held result capability hash drift was accepted: %v", err)
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
				Birth: testResultBirth(owner),
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
		Birth: testResultBirth(owner),
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
				Birth: testResultBirth(owner),
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
