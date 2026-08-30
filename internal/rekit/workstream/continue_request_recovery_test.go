package workstream

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

func TestContinueRoutedRequestRecoversEachCommittedPrefixExactlyOnce(t *testing.T) {
	tests := []struct {
		name               string
		failAfter          string
		wantPrefix         continueRequestComponentCounts
		wantRecoveryWrites []string
	}{
		{
			name:       "request fact",
			failAfter:  "request",
			wantPrefix: continueRequestComponentCounts{Request: 1},
			wantRecoveryWrites: []string{
				".rekit/facts/decisions.jsonl",
				".rekit/lanes/devirt-main/inbox.jsonl",
				".rekit/lanes/devirt-main/tasks.jsonl",
			},
		},
		{
			name:       "target task",
			failAfter:  "task",
			wantPrefix: continueRequestComponentCounts{Request: 1, Task: 1},
			wantRecoveryWrites: []string{
				".rekit/facts/decisions.jsonl",
				".rekit/lanes/devirt-main/inbox.jsonl",
			},
		},
		{
			name:       "target inbox",
			failAfter:  "inbox",
			wantPrefix: continueRequestComponentCounts{Request: 1, Task: 1, Inbox: 1},
			wantRecoveryWrites: []string{
				".rekit/facts/decisions.jsonl",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot, caseRoot := setupOwnedContinueCase(t)
			const eventID = "evt-routed-request-prefix"
			appendContinueRequestOutput(t, caseRoot, map[string]any{
				"schemaVersion": 1,
				"eventId":       eventID,
				"kind":          "request",
				"requestId":     "routed-request-prefix",
				"targetLane":    "devirt-main",
				"summary":       "recover routed request",
			})

			opt := ContinueOptions{
				Selector:                   "devirt-main",
				Executor:                   "executor-one",
				ExpectedExecutorGeneration: 1,
			}
			preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Summary.Routed != 1 {
				t.Fatalf("initial preview did not route request: %+v", preview)
			}
			opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256

			continueRequestAfterComponentHook = func(component string) error {
				if component == tc.failAfter {
					return errors.New("injected continue request prefix failure")
				}
				return nil
			}
			t.Cleanup(func() { continueRequestAfterComponentHook = nil })
			if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
				t.Fatal("ContinueApply succeeded despite injected prefix failure")
			}
			continueRequestAfterComponentHook = nil

			if got := continueRequestCounts(t, caseRoot, "devirt-main", eventID); got != tc.wantPrefix {
				t.Fatalf("committed prefix = %+v, want %+v", got, tc.wantPrefix)
			}

			recovery, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{
				Selector:                   "devirt-main",
				Executor:                   "executor-one",
				ExpectedExecutorGeneration: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			if recovery.Summary.Collected != 1 || recovery.Summary.Skipped != 0 || recovery.Summary.Routed != 1 {
				t.Fatalf("fresh recovery preview skipped incomplete request: %+v", recovery.Summary)
			}
			gotWrites := continueWritePaths(recovery.WouldWrites)
			wantWrites := append([]string{}, tc.wantRecoveryWrites...)
			sort.Strings(wantWrites)
			if !reflect.DeepEqual(gotWrites, wantWrites) {
				t.Fatalf("recovery writes = %v, want missing components %v", gotWrites, wantWrites)
			}

			recoveryOpt := ContinueOptions{
				Selector:                   "devirt-main",
				Executor:                   "executor-one",
				ExpectedExecutorGeneration: 1,
				ExpectedContinuePlanSHA256: recovery.ContinuePlanSHA256,
			}
			if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, recoveryOpt); err != nil {
				t.Fatal(err)
			}
			wantComplete := continueRequestComponentCounts{Request: 1, Task: 1, Inbox: 1, Decision: 1}
			if got := continueRequestCounts(t, caseRoot, "devirt-main", eventID); got != wantComplete {
				t.Fatalf("recovered request components = %+v, want %+v", got, wantComplete)
			}

			terminal, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{
				Selector:                   "devirt-main",
				Executor:                   "executor-one",
				ExpectedExecutorGeneration: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			if terminal.Summary.Collected != 0 || terminal.Summary.Skipped != 1 || len(terminal.WouldWrites) != 0 {
				t.Fatalf("complete request was not an exactly-once terminal replay: summary=%+v writes=%+v", terminal.Summary, terminal.WouldWrites)
			}
		})
	}
}

func TestContinueRoutedRequestDeduplicatesSemanticRouteWithinSnapshot(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       "evt-semantic-route-a",
		"kind":          "request",
		"requestId":     "semantic-route",
		"targetLane":    "devirt-main",
		"summary":       "route once",
	})
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       "evt-semantic-route-b",
		"kind":          "request",
		"requestId":     "semantic-route",
		"targetLane":    "devirt-main",
		"summary":       "route once",
	})

	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Summary.Collected != 2 || preview.Summary.Routed != 2 {
		t.Fatalf("semantic duplicate preview summary = %+v", preview.Summary)
	}
	assertContinueWritePathCount(t, preview.WouldWrites, ".rekit/facts/requests.jsonl", 2)
	assertContinueWritePathCount(t, preview.WouldWrites, ".rekit/facts/decisions.jsonl", 2)
	assertContinueWritePathCount(t, preview.WouldWrites, ".rekit/lanes/devirt-main/tasks.jsonl", 1)
	assertContinueWritePathCount(t, preview.WouldWrites, ".rekit/lanes/devirt-main/inbox.jsonl", 1)

	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	applied, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	assertContinueWritePathCount(t, applied.Writes, ".rekit/facts/requests.jsonl", 2)
	assertContinueWritePathCount(t, applied.Writes, ".rekit/facts/decisions.jsonl", 2)
	assertContinueWritePathCount(t, applied.Writes, ".rekit/lanes/devirt-main/tasks.jsonl", 1)
	assertContinueWritePathCount(t, applied.Writes, ".rekit/lanes/devirt-main/inbox.jsonl", 1)
	if got := continueRequestCounts(t, caseRoot, "devirt-main", "evt-semantic-route-a"); got != (continueRequestComponentCounts{Request: 1, Task: 1, Inbox: 1, Decision: 1}) {
		t.Fatalf("first semantic request components = %+v", got)
	}
	if got := continueRequestCounts(t, caseRoot, "devirt-main", "evt-semantic-route-b"); got != (continueRequestComponentCounts{Request: 1, Decision: 1}) {
		t.Fatalf("second semantic request components = %+v", got)
	}

	terminal, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Summary.Collected != 0 || terminal.Summary.Skipped != 2 || len(terminal.WouldWrites) != 0 {
		t.Fatalf("semantic duplicate terminal replay was not exactly once: summary=%+v writes=%+v", terminal.Summary, terminal.WouldWrites)
	}
}

func TestContinueRoutedRequestDeduplicatesSemanticRouteAcrossContinues(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	applyRequest := func(eventID string) ContinueResult {
		t.Helper()
		appendContinueRequestOutput(t, caseRoot, map[string]any{
			"schemaVersion": 1,
			"eventId":       eventID,
			"kind":          "request",
			"requestId":     "semantic-route-across-continues",
			"targetLane":    "devirt-main",
			"summary":       "route across continues",
		})
		opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
		preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
		if err != nil {
			t.Fatal(err)
		}
		opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
		applied, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt)
		if err != nil {
			t.Fatal(err)
		}
		return applied
	}

	first := applyRequest("evt-semantic-across-a")
	assertContinueWritePathCount(t, first.Writes, ".rekit/lanes/devirt-main/tasks.jsonl", 1)
	assertContinueWritePathCount(t, first.Writes, ".rekit/lanes/devirt-main/inbox.jsonl", 1)
	second := applyRequest("evt-semantic-across-b")
	assertContinueWritePathCount(t, second.Writes, ".rekit/facts/requests.jsonl", 1)
	assertContinueWritePathCount(t, second.Writes, ".rekit/facts/decisions.jsonl", 1)
	assertContinueWritePathCount(t, second.Writes, ".rekit/lanes/devirt-main/tasks.jsonl", 0)
	assertContinueWritePathCount(t, second.Writes, ".rekit/lanes/devirt-main/inbox.jsonl", 0)
	if got := continueRequestCounts(t, caseRoot, "devirt-main", "evt-semantic-across-b"); got != (continueRequestComponentCounts{Request: 1, Decision: 1}) {
		t.Fatalf("cross-continue semantic duplicate components = %+v", got)
	}
}

func TestContinueRoutedRequestRejectsPartialSemanticPredecessor(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	for _, eventID := range []string{"evt-semantic-prefix-a", "evt-semantic-prefix-b"} {
		appendContinueRequestOutput(t, caseRoot, map[string]any{
			"schemaVersion": 1,
			"eventId":       eventID,
			"kind":          "request",
			"requestId":     "semantic-prefix",
			"targetLane":    "devirt-main",
			"summary":       "partial semantic predecessor",
		})
	}
	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	continueRequestAfterComponentHook = func(component string) error {
		if component == "task" {
			return errors.New("stop after semantic predecessor task")
		}
		return nil
	}
	if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
		t.Fatal("ContinueApply succeeded despite semantic predecessor prefix failure")
	}
	continueRequestAfterComponentHook = nil
	t.Cleanup(func() { continueRequestAfterComponentHook = nil })

	if _, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}); err == nil || !strings.Contains(err.Error(), "incomplete semantic route predecessor") {
		t.Fatalf("partial semantic predecessor did not fail closed: %v", err)
	}
	if got := continueRequestCounts(t, caseRoot, "devirt-main", "evt-semantic-prefix-b"); got != (continueRequestComponentCounts{}) {
		t.Fatalf("semantic duplicate was written after partial predecessor: %+v", got)
	}
}

func TestContinueRoutedRequestRejectsConflictingDuplicateEventIDBeforeWrites(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	for _, summary := range []string{"first payload", "conflicting payload"} {
		appendContinueRequestOutput(t, caseRoot, map[string]any{
			"schemaVersion": 1,
			"eventId":       "evt-conflicting-duplicate",
			"kind":          "request",
			"requestId":     "conflicting-duplicate",
			"targetLane":    "devirt-main",
			"summary":       summary,
		})
	}
	if _, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}); err == nil || !strings.Contains(err.Error(), "conflicting payloads") {
		t.Fatalf("conflicting duplicate eventId did not fail before preview: %v", err)
	}
	if got := continueRequestCounts(t, caseRoot, "devirt-main", "evt-conflicting-duplicate"); got != (continueRequestComponentCounts{}) {
		t.Fatalf("conflicting duplicate event wrote components: %+v", got)
	}
}

func TestContinueRoutedRequestDeduplicatesIdenticalDuplicateEventID(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	event := map[string]any{
		"schemaVersion": 1,
		"eventId":       "evt-identical-duplicate",
		"kind":          "request",
		"requestId":     "identical-duplicate",
		"targetLane":    "devirt-main",
		"summary":       "identical payload",
	}
	appendContinueRequestOutput(t, caseRoot, event)
	appendContinueRequestOutput(t, caseRoot, event)
	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Summary.Collected != 1 || preview.Summary.Routed != 1 {
		t.Fatalf("identical duplicate was not normalized: %+v", preview.Summary)
	}
	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err != nil {
		t.Fatal(err)
	}
	if got := continueRequestCounts(t, caseRoot, "devirt-main", "evt-identical-duplicate"); got != (continueRequestComponentCounts{Request: 1, Task: 1, Inbox: 1, Decision: 1}) {
		t.Fatalf("identical duplicate components = %+v", got)
	}
}

func TestContinueRoutedRequestRejectsSemanticTargetDrift(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	other, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Name: "semantic-other", Selector: "semantic-other"})
	if err != nil {
		t.Fatal(err)
	}
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       "evt-semantic-target-a",
		"kind":          "request",
		"requestId":     "semantic-target",
		"targetLane":    "devirt-main",
		"summary":       "stable semantic target",
	})
	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err != nil {
		t.Fatal(err)
	}
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       "evt-semantic-target-b",
		"kind":          "request",
		"requestId":     "semantic-target",
		"targetLane":    other.Lane.ID,
		"summary":       "stable semantic target",
	})
	if _, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}); err == nil || (!strings.Contains(err.Error(), "already routed to devirt-main") && !strings.Contains(err.Error(), "targets both")) {
		t.Fatalf("semantic target drift was not rejected: %v", err)
	}
	if got := continueRequestCounts(t, caseRoot, other.Lane.ID, "evt-semantic-target-b"); got != (continueRequestComponentCounts{}) {
		t.Fatalf("semantic target drift wrote components: %+v", got)
	}
}

func TestContinueRoutedRequestPrefixRejectsClosedTarget(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	target, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Name: "closing-target", Selector: "closing-target"})
	if err != nil {
		t.Fatal(err)
	}
	const eventID = "evt-routed-closed-target"
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       eventID,
		"kind":          "request",
		"requestId":     "routed-closed-target",
		"targetLane":    target.Lane.ID,
		"summary":       "closed target recovery",
	})
	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	continueRequestAfterComponentHook = func(component string) error {
		if component == "request" {
			return errors.New("stop before routing")
		}
		return nil
	}
	if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
		t.Fatal("ContinueApply succeeded despite request-prefix failure")
	}
	continueRequestAfterComponentHook = nil
	t.Cleanup(func() { continueRequestAfterComponentHook = nil })

	targetState, err := readLaneByID(caseRoot, target.Lane.ID)
	if err != nil {
		t.Fatal(err)
	}
	targetState.Status = "closed"
	laneData, err := json.MarshalIndent(targetState, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	lanePath, err := projectstate.Join(caseRoot, "lanes", target.Lane.ID, "lane.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lanePath, append(laneData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}); err == nil || !strings.Contains(err.Error(), "cannot recover routed request") || !strings.Contains(err.Error(), "closed lane") {
		t.Fatalf("closed target accepted routed prefix recovery: %v", err)
	}
	if got := continueRequestCounts(t, caseRoot, target.Lane.ID, eventID); got != (continueRequestComponentCounts{Request: 1}) {
		t.Fatalf("closed target recovery wrote route components: %+v", got)
	}
}

func TestContinueRoutedRequestApplyRevalidatesTargetLaneBeforeRouteWrites(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	target, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Name: "late-closing-target", Selector: "late-closing-target"})
	if err != nil {
		t.Fatal(err)
	}
	const eventID = "evt-routed-late-closed-target"
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       eventID,
		"kind":          "request",
		"requestId":     "routed-late-closed-target",
		"targetLane":    target.Lane.ID,
		"summary":       "late closed target",
	})
	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	opt.AfterPreviewValidation = func() error {
		targetState, err := readLaneByID(caseRoot, target.Lane.ID)
		if err != nil {
			return err
		}
		targetState.Status = "closed"
		laneData, err := json.MarshalIndent(targetState, "", "  ")
		if err != nil {
			return err
		}
		lanePath, err := projectstate.Join(caseRoot, "lanes", target.Lane.ID, "lane.json")
		if err != nil {
			return err
		}
		return os.WriteFile(lanePath, append(laneData, '\n'), 0o600)
	}
	if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil || !strings.Contains(err.Error(), "target lane is not open") {
		t.Fatalf("Apply routed after target closed following preview validation: %v", err)
	}
	if got := continueRequestCounts(t, caseRoot, target.Lane.ID, eventID); got != (continueRequestComponentCounts{}) {
		t.Fatalf("late closed target received request or route components: %+v", got)
	}
}

func TestContinueRoutedRequestRejectsRequestOnlySemanticPredecessor(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	other, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Name: "request-only-target", Selector: "request-only-target"})
	if err != nil {
		t.Fatal(err)
	}
	const predecessorID = "evt-request-only-predecessor"
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       predecessorID,
		"kind":          "request",
		"requestId":     "request-only-semantic",
		"targetLane":    "devirt-main",
		"summary":       "request-only predecessor",
	})
	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	continueRequestAfterComponentHook = func(component string) error {
		if component == "request" {
			return errors.New("stop at request-only predecessor")
		}
		return nil
	}
	if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
		t.Fatal("ContinueApply succeeded despite request-only prefix failure")
	}
	continueRequestAfterComponentHook = nil
	t.Cleanup(func() { continueRequestAfterComponentHook = nil })

	lane, err := readLaneByID(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(caseRoot, filepath.FromSlash(lane.Workspace), "requests.jsonl")
	if err := os.WriteFile(requestPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       "evt-request-only-replacement",
		"kind":          "request",
		"requestId":     "request-only-semantic",
		"targetLane":    other.Lane.ID,
		"summary":       "request-only predecessor",
	})
	if _, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}); err == nil || !strings.Contains(err.Error(), "incomplete semantic route predecessor") || !strings.Contains(err.Error(), predecessorID) {
		t.Fatalf("request-only semantic predecessor did not block replacement: %v", err)
	}
	if got := continueRequestCounts(t, caseRoot, other.Lane.ID, "evt-request-only-replacement"); got != (continueRequestComponentCounts{}) {
		t.Fatalf("replacement request wrote after request-only predecessor: %+v", got)
	}
}

func TestContinueRoutedRequestRejectsDecisionMissingSemanticPredecessorReplacement(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	const predecessorID = "evt-decision-missing-predecessor"
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       predecessorID,
		"kind":          "request",
		"requestId":     "decision-missing-semantic",
		"targetLane":    "devirt-main",
		"summary":       "decision missing predecessor",
	})
	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	continueRequestAfterComponentHook = func(component string) error {
		if component == "inbox" {
			return errors.New("stop before predecessor decision")
		}
		return nil
	}
	if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
		t.Fatal("ContinueApply succeeded despite decision-missing prefix failure")
	}
	continueRequestAfterComponentHook = nil
	t.Cleanup(func() { continueRequestAfterComponentHook = nil })

	lane, err := readLaneByID(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(caseRoot, filepath.FromSlash(lane.Workspace), "requests.jsonl")
	if err := os.WriteFile(requestPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       "evt-decision-missing-replacement",
		"kind":          "request",
		"requestId":     "decision-missing-semantic",
		"targetLane":    "devirt-main",
		"summary":       "decision missing predecessor",
	})
	if _, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}); err == nil || !strings.Contains(err.Error(), "incomplete semantic route predecessor") || !strings.Contains(err.Error(), predecessorID) {
		t.Fatalf("decision-missing semantic predecessor did not block replacement: %v", err)
	}
	if got := continueRequestCounts(t, caseRoot, "devirt-main", "evt-decision-missing-replacement"); got != (continueRequestComponentCounts{}) {
		t.Fatalf("replacement wrote after decision-missing predecessor: %+v", got)
	}
}

func TestContinueRoutedRequestPrefixFreezesRouteAndIdentity(t *testing.T) {
	repoRoot, caseRoot := setupOwnedContinueCase(t)
	target, err := StartApply(repoRoot, caseRoot, defaults.DefaultPack, StartOptions{Name: "route-target", Selector: "route-target"})
	if err != nil {
		t.Fatal(err)
	}
	targetLane := target.Lane.ID
	const eventID = "evt-routed-request-binding"
	appendContinueRequestOutput(t, caseRoot, map[string]any{
		"schemaVersion": 1,
		"eventId":       eventID,
		"kind":          "request",
		"requestId":     "request-binding-a",
		"targetLane":    targetLane,
		"summary":       "freeze routed request identity",
	})
	opt := ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}
	preview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, opt)
	if err != nil {
		t.Fatal(err)
	}
	opt.ExpectedContinuePlanSHA256 = preview.ContinuePlanSHA256
	continueRequestAfterComponentHook = func(component string) error {
		if component == "request" {
			return errors.New("stop after durable request")
		}
		return nil
	}
	if _, err := ContinueApply(repoRoot, caseRoot, defaults.DefaultPack, opt); err == nil {
		t.Fatal("ContinueApply succeeded despite request-prefix failure")
	}
	continueRequestAfterComponentHook = nil
	t.Cleanup(func() { continueRequestAfterComponentHook = nil })

	targetState, err := readLaneByID(caseRoot, targetLane)
	if err != nil {
		t.Fatal(err)
	}
	targetState.Status = "paused"
	laneData, err := json.MarshalIndent(targetState, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	lanePath, err := projectstate.Join(caseRoot, "lanes", targetLane, "lane.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lanePath, append(laneData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	pausedPreview, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1})
	if err == nil || !strings.Contains(err.Error(), "cannot recover routed request") {
		t.Fatalf("paused routed prefix was downgraded instead of held: err=%v preview=%+v", err, pausedPreview)
	}

	targetState.Status = "open"
	laneData, err = json.MarshalIndent(targetState, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lanePath, append(laneData, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	sourceLane, err := readLaneByID(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(caseRoot, filepath.FromSlash(sourceLane.Workspace), "requests.jsonl")
	requestData, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatal(err)
	}
	requestData = []byte(strings.Replace(string(requestData), "request-binding-a", "request-binding-b", 1))
	if err := os.WriteFile(requestPath, requestData, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ContinuePreview(repoRoot, caseRoot, defaults.DefaultPack, ContinueOptions{Selector: "devirt-main", Executor: "executor-one", ExpectedExecutorGeneration: 1}); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("routed request identity drift was accepted: %v", err)
	}
}

type continueRequestComponentCounts struct {
	Request  int
	Task     int
	Inbox    int
	Decision int
}

func appendContinueRequestOutput(t *testing.T, caseRoot string, event map[string]any) {
	t.Helper()
	lane, err := readLaneByID(caseRoot, "devirt-main")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(caseRoot, filepath.FromSlash(lane.Workspace), "requests.jsonl")
	if err := mission.AppendJSONLine(path, event); err != nil {
		t.Fatal(err)
	}
}

func continueRequestCounts(t *testing.T, caseRoot, targetLane, eventID string) continueRequestComponentCounts {
	t.Helper()
	requestFacts, err := mission.ReadFact(caseRoot, "request")
	if err != nil {
		t.Fatal(err)
	}
	decisionFacts, err := mission.ReadFact(caseRoot, "decision")
	if err != nil {
		t.Fatal(err)
	}
	target, err := readLaneByID(caseRoot, targetLane)
	if err != nil {
		t.Fatal(err)
	}
	laneRoot, err := laneRootPath(caseRoot, target)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := mission.ReadJSONLineObjects(LaneTasksJSONLPath(laneRoot))
	if err != nil {
		t.Fatal(err)
	}
	inbox, err := mission.ReadJSONLineObjects(LaneInboxJSONLPath(laneRoot))
	if err != nil {
		t.Fatal(err)
	}
	return continueRequestComponentCounts{
		Request:  countContinueEventID(requestFacts, eventID),
		Task:     countContinueEventID(tasks, eventID),
		Inbox:    countContinueEventID(inbox, eventID),
		Decision: countContinueEventID(decisionFacts, eventID),
	}
}

func countContinueEventID(items []map[string]any, eventID string) int {
	count := 0
	for _, item := range items {
		if stringFrom(item, "eventId") == eventID {
			count++
		}
	}
	return count
}

func assertContinueWritePathCount(t *testing.T, writes []StartWrite, path string, want int) {
	t.Helper()
	path = filepath.ToSlash(path)
	got := 0
	for _, write := range writes {
		if filepath.ToSlash(write.Path) == path {
			got++
		}
	}
	if got != want {
		t.Fatalf("write count for %s = %d, want %d; writes=%+v", path, got, want, writes)
	}
}

func continueWritePaths(writes []StartWrite) []string {
	paths := make([]string, 0, len(writes))
	for _, write := range writes {
		if write.Kind == "fact-jsonl" || write.Kind == "lane-jsonl" {
			paths = append(paths, filepath.ToSlash(write.Path))
		}
	}
	sort.Strings(paths)
	return paths
}
