package currentloop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/lanemutation"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
)

func TestCheckpointWriteInspectAndStaleCurrentness(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit reconcile main -InterventionId int-1 -WhatIf -Format json")
	payload := checkpointPayload(t, caseRoot, request)
	fixed := time.Date(2026, 8, 1, 12, 0, 0, 123456789, time.UTC)
	nowUTC = func() time.Time { return fixed }
	t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })

	inspection, err := Write(repoRoot, caseRoot, "_template", payload)
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Ready || inspection.State != "ready" || inspection.Sequence != 1 || inspection.Continuation == nil || inspection.Continuation.RemainingMaxSteps != 3 || inspection.AppliedSteps != 2 || !strings.HasSuffix(inspection.ArtifactPath, "/00000000000000000001.json") {
		t.Fatalf("unexpected checkpoint inspection: %+v", inspection)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(inspection.ArtifactPath))); err != nil {
		t.Fatalf("checkpoint artifact missing: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(caseRoot, filepath.FromSlash(artifactRelRoot)))
	if err != nil || len(entries) != 1 || strings.HasPrefix(entries[0].Name(), ".") {
		t.Fatalf("checkpoint publication left unexpected files: %v %+v", err, entries)
	}

	fresh := Inspect(repoRoot, caseRoot, "_template", &request)
	if !fresh.Ready || fresh.PayloadSHA256 != inspection.PayloadSHA256 || fresh.ArtifactSHA256 == "" || fresh.ArtifactBytes == 0 {
		t.Fatalf("fresh checkpoint was not recovered: %+v", fresh)
	}
	drift := checkpointRequest("/rekit continue main -WhatIf -Format json")
	stale := Inspect(repoRoot, caseRoot, "_template", &drift)
	if stale.Ready || stale.State != "stale-current-driver-request" || stale.Continuation != nil || len(stale.Warnings) == 0 {
		t.Fatalf("stale checkpoint exposed continuation: %+v", stale)
	}
}

func TestCheckpointWriteBindsIdentityAfterPublicationLease(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit continue main -WhatIf -Format json")
	payload := checkpointPayload(t, caseRoot, request)
	lease, err := lanemutation.AcquireProject(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Unlock() })

	acquireAttempted := make(chan struct{})
	originalAcquireProject := acquireProject
	acquireProject = func(root string) (*lanemutation.Lease, error) {
		close(acquireAttempted)
		return originalAcquireProject(root)
	}
	t.Cleanup(func() { acquireProject = originalAcquireProject })

	type writeResult struct {
		inspection Inspection
		err        error
	}
	result := make(chan writeResult, 1)
	go func() {
		inspection, err := Write(repoRoot, caseRoot, "_template", payload)
		result <- writeResult{inspection: inspection, err: err}
	}()
	select {
	case <-acquireAttempted:
	case <-time.After(5 * time.Second):
		t.Fatal("checkpoint writer did not attempt to acquire the project lease")
	}
	instanceText := "templateRoot: " + repoRoot + "\ntemplatePack: _template\nprojectName: checkpoint-renamed\nprojectRoot: " + caseRoot + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(instanceText), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := lease.Unlock(); err != nil {
		t.Fatal(err)
	}
	lease = nil

	var written writeResult
	select {
	case written = <-result:
	case <-time.After(5 * time.Second):
		t.Fatal("checkpoint writer did not finish after the project lease was released")
	}
	if written.err != nil {
		t.Fatal(written.err)
	}
	fresh := Inspect(repoRoot, caseRoot, "_template", &request)
	if !written.inspection.Ready || !fresh.Ready || written.inspection.PayloadSHA256 != fresh.PayloadSHA256 || written.inspection.ArtifactSHA256 != fresh.ArtifactSHA256 {
		t.Fatalf("checkpoint identity was not bound to the metadata protected by its publication lease: written=%+v fresh=%+v", written.inspection, fresh)
	}
}

func TestCheckpointLatestArtifactFailsClosedWithoutFallback(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit continue main -WhatIf -Format json")
	payload := checkpointPayload(t, caseRoot, request)
	stamps := []time.Time{
		time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 1, 12, 0, 1, 0, time.UTC),
	}
	nowUTC = func() time.Time {
		stamp := stamps[0]
		stamps = stamps[1:]
		return stamp
	}
	t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })
	first, err := Write(repoRoot, caseRoot, "_template", payload)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Write(repoRoot, caseRoot, "_template", payload)
	if err != nil {
		t.Fatal(err)
	}
	if first.ArtifactPath == second.ArtifactPath {
		t.Fatalf("two segment checkpoints reused one artifact path: %s", first.ArtifactPath)
	}
	latestPath := filepath.Join(caseRoot, filepath.FromSlash(second.ArtifactPath))
	if err := os.WriteFile(latestPath, []byte(`{"tampered":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	inspection := Inspect(repoRoot, caseRoot, "_template", &request)
	if inspection.Ready || inspection.State != "invalid" || inspection.Continuation != nil || !strings.Contains(strings.Join(inspection.Warnings, " "), "chain is invalid") {
		t.Fatalf("tampered latest artifact fell back to older history: %+v", inspection)
	}
}

func TestCheckpointSequenceChainAndCanonicalBytesFailClosed(t *testing.T) {
	t.Run("refresh-failed-cannot-be-ready", func(t *testing.T) {
		_, caseRoot := checkpointAttachedCase(t)
		request := checkpointRequest("/rekit continue main -WhatIf -Format json")
		payload := checkpointPayload(t, caseRoot, request)
		last := &payload.StepReceipts[len(payload.StepReceipts)-1]
		last.CurrentStepReceipt.State = "refresh-failed"
		last.CurrentStepReceipt.Outcome = "current-step-applied-status-refresh-failed"
		last.CurrentStepReceiptSHA256, _ = ValueSHA256(last.CurrentStepReceipt)
		payload.SchemaVersion = 1
		payload.Kind = "current-loop-segment-checkpoint"
		payload.Sequence = 1
		payload.CaseIdentitySHA256 = strings.Repeat("a", 64)
		payload.Pack = "_template"
		payload.NoAutoApply = true
		payload.NoAuthority = true
		if err := validatePayload(payload); err == nil || !strings.Contains(err.Error(), "refresh-failed") {
			t.Fatalf("refresh-failed receipt was accepted with available status: %v", err)
		}
	})

	t.Run("clock-rollback-keeps-publication-order", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		request := checkpointRequest("/rekit continue main -WhatIf -Format json")
		payload := checkpointPayload(t, caseRoot, request)
		stamps := []time.Time{
			time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
			time.Date(2026, 8, 1, 11, 59, 0, 0, time.UTC),
		}
		nowUTC = func() time.Time {
			stamp := stamps[0]
			stamps = stamps[1:]
			return stamp
		}
		t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })
		first, err := Write(repoRoot, caseRoot, "_template", payload)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Write(repoRoot, caseRoot, "_template", payload)
		if err != nil {
			t.Fatal(err)
		}
		if first.Sequence != 1 || second.Sequence != 2 || !strings.HasSuffix(second.ArtifactPath, "/00000000000000000002.json") || second.RecordedAt >= first.RecordedAt {
			t.Fatalf("wall-clock rollback changed publication order: first=%+v second=%+v", first, second)
		}
	})

	t.Run("broken-previous-hash", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		request := checkpointRequest("/rekit continue main -WhatIf -Format json")
		payload := checkpointPayload(t, caseRoot, request)
		stamps := []time.Time{
			time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
			time.Date(2026, 8, 1, 12, 0, 1, 0, time.UTC),
		}
		nowUTC = func() time.Time {
			stamp := stamps[0]
			stamps = stamps[1:]
			return stamp
		}
		t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })
		if _, err := Write(repoRoot, caseRoot, "_template", payload); err != nil {
			t.Fatal(err)
		}
		second, err := Write(repoRoot, caseRoot, "_template", payload)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(caseRoot, filepath.FromSlash(second.ArtifactPath))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var artifact envelope
		if err := json.Unmarshal(data, &artifact); err != nil {
			t.Fatal(err)
		}
		var secondPayload Payload
		if err := json.Unmarshal(artifact.Payload, &secondPayload); err != nil {
			t.Fatal(err)
		}
		secondPayload.PreviousArtifactSHA256 = strings.Repeat("0", 64)
		artifact.Payload, err = json.Marshal(secondPayload)
		if err != nil {
			t.Fatal(err)
		}
		artifact.PayloadSHA256 = sha256Hex(artifact.Payload)
		artifact.PayloadBytes = len(artifact.Payload)
		data, err = canonicalEnvelope(artifact)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		inspection := Inspect(repoRoot, caseRoot, "_template", &request)
		if inspection.Ready || inspection.State != "invalid" || inspection.Continuation != nil || !strings.Contains(strings.Join(inspection.Warnings, " "), "previous hash chain") {
			t.Fatalf("broken previous hash was accepted: %+v", inspection)
		}
	})

	t.Run("crash-temp-does-not-block-next-sequence", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		root := filepath.Join(caseRoot, filepath.FromSlash(artifactRelRoot))
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		stale := ".00000000000000000001.0123456789abcdef0123456789abcdef.json.tmp"
		if err := os.WriteFile(filepath.Join(root, stale), []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
		request := checkpointRequest("/rekit continue main -WhatIf -Format json")
		payload := checkpointPayload(t, caseRoot, request)
		nowUTC = func() time.Time { return time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC) }
		t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })
		written, err := Write(repoRoot, caseRoot, "_template", payload)
		if err != nil {
			t.Fatal(err)
		}
		if written.Sequence != 1 || !written.Ready {
			t.Fatalf("crash temp blocked new publication: %+v", written)
		}
	})

	t.Run("canonical-byte-drift", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		request := checkpointRequest("/rekit continue main -WhatIf -Format json")
		payload := checkpointPayload(t, caseRoot, request)
		nowUTC = func() time.Time { return time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC) }
		t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })
		written, err := Write(repoRoot, caseRoot, "_template", payload)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(caseRoot, filepath.FromSlash(written.ArtifactPath))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, ' '), 0o600); err != nil {
			t.Fatal(err)
		}
		inspection := Inspect(repoRoot, caseRoot, "_template", &request)
		if inspection.Ready || inspection.State != "invalid" || inspection.Continuation != nil {
			t.Fatalf("non-canonical byte mutation was accepted: %+v", inspection)
		}
	})

	t.Run("sequence-gap", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		request := checkpointRequest("/rekit continue main -WhatIf -Format json")
		payload := checkpointPayload(t, caseRoot, request)
		stamps := []time.Time{
			time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC),
			time.Date(2026, 8, 1, 12, 0, 1, 0, time.UTC),
		}
		nowUTC = func() time.Time {
			stamp := stamps[0]
			stamps = stamps[1:]
			return stamp
		}
		t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })
		first, err := Write(repoRoot, caseRoot, "_template", payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Write(repoRoot, caseRoot, "_template", payload); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(caseRoot, filepath.FromSlash(first.ArtifactPath))); err != nil {
			t.Fatal(err)
		}
		inspection := Inspect(repoRoot, caseRoot, "_template", &request)
		if inspection.Ready || inspection.State != "invalid" || inspection.Continuation != nil || !strings.Contains(strings.Join(inspection.Warnings, " "), "sequence gap") {
			t.Fatalf("sequence gap was accepted: %+v", inspection)
		}
	})
}

func TestCheckpointNamespaceAndTerminalStatesFailClosed(t *testing.T) {
	t.Run("unexpected-entry", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		root := filepath.Join(caseRoot, filepath.FromSlash(artifactRelRoot))
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "latest.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		inspection := Inspect(repoRoot, caseRoot, "_template", nil)
		if inspection.Ready || inspection.State != "invalid" || inspection.Continuation != nil {
			t.Fatalf("unexpected namespace entry was accepted: %+v", inspection)
		}
	})

	t.Run("symlink-namespace", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		runs := filepath.Join(caseRoot, ".rekit", "runs")
		outside := t.TempDir()
		if err := os.MkdirAll(runs, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(runs, "current-loop-segments")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		inspection := Inspect(repoRoot, caseRoot, "_template", nil)
		if inspection.Ready || inspection.State != "invalid" || inspection.Continuation != nil {
			t.Fatalf("symlink namespace was accepted: %+v", inspection)
		}
	})

	t.Run("terminal", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		request := checkpointRequest("/rekit continue main -WhatIf -Format json")
		payload := checkpointPayload(t, caseRoot, request)
		payload.RefreshedCurrentDriverRequest = nil
		payload.RefreshedCurrentDriverRequestSHA256 = ""
		payload.Continuation = nil
		last := &payload.StepReceipts[len(payload.StepReceipts)-1]
		last.RequestAfter = nil
		last.RequestAfterSHA256 = ""
		last.CurrentStepReceipt.RefreshedCurrentDriverRequest = nil
		last.CurrentStepReceiptSHA256, _ = ValueSHA256(last.CurrentStepReceipt)
		nowUTC = func() time.Time { return time.Date(2026, 8, 1, 12, 0, 2, 0, time.UTC) }
		t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })
		inspection, err := Write(repoRoot, caseRoot, "_template", payload)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.Ready || inspection.State != "terminal" || inspection.Continuation != nil || !strings.Contains(strings.Join(inspection.Warnings, " "), "terminal history") {
			t.Fatalf("terminal checkpoint was misclassified: %+v", inspection)
		}
	})

	t.Run("terminal-current-request-drift", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		request := checkpointRequest("/rekit continue main -WhatIf -Format json")
		payload := checkpointPayload(t, caseRoot, request)
		payload.RefreshedCurrentDriverRequest = nil
		payload.RefreshedCurrentDriverRequestSHA256 = ""
		payload.Continuation = nil
		last := &payload.StepReceipts[len(payload.StepReceipts)-1]
		last.RequestAfter = nil
		last.RequestAfterSHA256 = ""
		last.CurrentStepReceipt.RefreshedCurrentDriverRequest = nil
		last.CurrentStepReceiptSHA256, _ = ValueSHA256(last.CurrentStepReceipt)
		nowUTC = func() time.Time { return time.Date(2026, 8, 1, 12, 0, 3, 0, time.UTC) }
		t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })
		if _, err := Write(repoRoot, caseRoot, "_template", payload); err != nil {
			t.Fatal(err)
		}
		inspection := Inspect(repoRoot, caseRoot, "_template", &request)
		if inspection.Ready || inspection.State != "stale-current-driver-request" || inspection.Continuation != nil {
			t.Fatalf("terminal checkpoint ignored newly current durable request: %+v", inspection)
		}
	})

	t.Run("status-unavailable", func(t *testing.T) {
		repoRoot, caseRoot := checkpointAttachedCase(t)
		request := checkpointRequest("/rekit continue main -WhatIf -Format json")
		payload := checkpointPayload(t, caseRoot, request)
		payload.StatusAvailable = false
		payload.RefreshedCurrentDriverRequest = nil
		payload.RefreshedCurrentDriverRequestSHA256 = ""
		payload.Continuation = nil
		last := &payload.StepReceipts[len(payload.StepReceipts)-1]
		last.RequestAfter = nil
		last.RequestAfterSHA256 = ""
		last.CurrentStepReceipt.State = "refresh-failed"
		last.CurrentStepReceipt.Outcome = "current-step-applied-status-refresh-failed"
		last.CurrentStepReceipt.RefreshedCurrentDriverRequest = nil
		last.CurrentStepReceiptSHA256, _ = ValueSHA256(last.CurrentStepReceipt)
		nowUTC = func() time.Time { return time.Date(2026, 8, 1, 12, 0, 2, 0, time.UTC) }
		t.Cleanup(func() { nowUTC = func() time.Time { return time.Now().UTC() } })
		inspection, err := Write(repoRoot, caseRoot, "_template", payload)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.Ready || inspection.State != "status-unavailable" || inspection.Continuation != nil {
			t.Fatalf("status-unavailable checkpoint exposed continuation: %+v", inspection)
		}
	})
}

func checkpointPayload(t *testing.T, caseRoot string, request mission.MissionCommanderDriverRequest) Payload {
	t.Helper()
	requestSHA256, err := RequestSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	steps := checkpointStepReceipts(t, request)
	payload := Payload{
		Actor:                               request.Actor,
		RoutePolicy:                         "fixed-initial-route-and-lane",
		InitialCurrentDriverRequest:         steps[0].RequestBefore,
		InitialCurrentDriverRequestSHA256:   steps[0].RequestBeforeSHA256,
		ExpectedCurrentLoopPlanSHA256:       strings.Repeat("a", 64),
		SegmentMaxSteps:                     5,
		AppliedStepsInSegment:               2,
		RemainingMaxSteps:                   3,
		SegmentRoute:                        "case",
		SegmentLane:                         "main",
		Stop:                                Stop{Code: "human-intervention", Phase: "before-step"},
		StepReceipts:                        steps,
		StatusAvailable:                     true,
		RefreshedCurrentDriverRequest:       &request,
		RefreshedCurrentDriverRequestSHA256: requestSHA256,
		Continuation: &Continuation{
			Kind:                  "current-loop-campaign-continuation",
			State:                 "awaiting-fresh-segment-review",
			StopCode:              "human-intervention",
			SegmentMaxSteps:       5,
			AppliedStepsInSegment: 2,
			RemainingMaxSteps:     3,
			SegmentRoute:          "case",
			SegmentLane:           "main",
			ExpectedRoute:         "case",
			ExpectedLane:          "main",
			WhatIfCommand:         `/rekit run-current-loop -Target "case" -Pack _template -MaxSteps 3 -WhatIf -Format json`,
			FreshPreviewRequired:  true,
			CumulativeReceipts:    false,
		},
	}
	identity := struct {
		SchemaVersion                 int                                   `json:"schemaVersion"`
		CaseRoot                      string                                `json:"caseRoot"`
		Pack                          string                                `json:"pack"`
		RoutePolicy                   string                                `json:"routePolicy"`
		MaxSteps                      int                                   `json:"maxSteps"`
		Actor                         string                                `json:"actor"`
		InitialRoute                  string                                `json:"initialRoute"`
		InitialLane                   string                                `json:"initialLane"`
		InitialCurrentDriverRequest   mission.MissionCommanderDriverRequest `json:"initialCurrentDriverRequest"`
		ExpectedCurrentStepPlanSHA256 string                                `json:"expectedCurrentStepPlanSha256"`
	}{1, caseRoot, "_template", payload.RoutePolicy, payload.SegmentMaxSteps, payload.Actor, payload.SegmentRoute, payload.SegmentLane, payload.InitialCurrentDriverRequest, steps[0].ExpectedCurrentStepPlanSHA256}
	encoded, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	payload.ExpectedCurrentLoopPlanSHA256 = sha256Hex(encoded)
	return payload
}

func checkpointStepReceipts(t *testing.T, final mission.MissionCommanderDriverRequest) []StepReceiptBinding {
	t.Helper()
	initial := checkpointRequest("/rekit continue main -WhatIf -Format json")
	middle := checkpointRequest("/rekit continue main -Executor main-agent -WhatIf -Format json")
	requests := []mission.MissionCommanderDriverRequest{initial, middle, final}
	bindings := make([]StepReceiptBinding, 0, 2)
	for index := range 2 {
		beforeSHA256, err := RequestSHA256(requests[index])
		if err != nil {
			t.Fatal(err)
		}
		afterSHA256, err := RequestSHA256(requests[index+1])
		if err != nil {
			t.Fatal(err)
		}
		receipt := StepReceipt{
			State:                         "refreshed",
			Outcome:                       "current-step-applied",
			Route:                         "case",
			NestedCommand:                 "run-driver-step",
			RefreshedCurrentDriverRequest: &requests[index+1],
			Boundary:                      []string{"test receipt"},
		}
		receiptSHA256, err := ValueSHA256(receipt)
		if err != nil {
			t.Fatal(err)
		}
		bindings = append(bindings, StepReceiptBinding{
			Step:                          index + 1,
			Route:                         "case",
			Lane:                          "main",
			RunLoopStepID:                 "continue",
			ExpectedCurrentStepPlanSHA256: strings.Repeat(string(rune('b'+index)), 64),
			RequestBefore:                 requests[index],
			RequestBeforeSHA256:           beforeSHA256,
			CurrentStepReceipt:            receipt,
			CurrentStepReceiptSHA256:      receiptSHA256,
			RequestAfter:                  &requests[index+1],
			RequestAfterSHA256:            afterSHA256,
		})
	}
	return bindings
}

func checkpointRequest(command string) mission.MissionCommanderDriverRequest {
	return mission.MissionCommanderDriverRequest{
		Kind:              "preview-command",
		RunLoopStepID:     "continue",
		Actor:             "main-agent",
		State:             "review-required",
		Source:            "checkpoint-test",
		Lane:              "main",
		Command:           command,
		CommandExecutable: true,
		RequiresReview:    true,
	}
}

func checkpointAttachedCase(t *testing.T) (string, string) {
	t.Helper()
	repoRoot := filepath.Join(t.TempDir(), "kit")
	caseRoot := filepath.Join(t.TempDir(), "case")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	instanceText := "templateRoot: " + repoRoot + "\ntemplatePack: _template\nprojectName: checkpoint-test\nprojectRoot: " + caseRoot + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(instanceText), 0o644); err != nil {
		t.Fatal(err)
	}
	return repoRoot, caseRoot
}
