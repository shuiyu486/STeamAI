package currentloop

import (
	"encoding/json"
	"errors"
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

func TestCheckpointResumeClaimNamespaceSwapFailsBeforeConsumption(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit continue main -WhatIf -Format json")
	written, err := Write(repoRoot, caseRoot, "_template", checkpointPayload(t, caseRoot, request))
	if err != nil {
		t.Fatal(err)
	}
	requestSHA256, err := RequestSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	originalSync := syncClaimDirectory
	syncClaimDirectory = func(root *os.Root) error {
		claimPath := filepath.Join(caseRoot, ".rekit", "runs", "current-loop-segment-claims")
		movedPath := claimPath + "-moved"
		if err := os.Rename(claimPath, movedPath); err != nil {
			return err
		}
		if err := os.Mkdir(claimPath, 0o700); err != nil {
			return err
		}
		return syncCurrentLoopRoot(root)
	}
	t.Cleanup(func() { syncClaimDirectory = originalSync })
	err = ClaimResume(repoRoot, caseRoot, "_template", Claim{
		SourceArtifactSHA256:          written.ArtifactSHA256,
		ExpectedCurrentLoopPlanSHA256: strings.Repeat("c", 64),
		CurrentDriverRequestSHA256:    requestSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "namespace changed") {
		t.Fatalf("resume claim namespace swap was accepted: %v", err)
	}
	inspection := Inspect(repoRoot, caseRoot, "_template", &request)
	if !inspection.Ready || inspection.State != "ready" || inspection.ResumeDriverRequest == nil {
		t.Fatalf("namespace swap consumed source in canonical namespace: %+v", inspection)
	}
}

func TestCheckpointResumeClaimSyncFailureDoesNotConsumeSource(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit continue main -WhatIf -Format json")
	written, err := Write(repoRoot, caseRoot, "_template", checkpointPayload(t, caseRoot, request))
	if err != nil {
		t.Fatal(err)
	}
	requestSHA256, err := RequestSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	originalSync := syncClaimDirectory
	syncClaimDirectory = func(*os.Root) error { return os.ErrPermission }
	t.Cleanup(func() { syncClaimDirectory = originalSync })
	err = ClaimResume(repoRoot, caseRoot, "_template", Claim{
		SourceArtifactSHA256:          written.ArtifactSHA256,
		ExpectedCurrentLoopPlanSHA256: strings.Repeat("c", 64),
		CurrentDriverRequestSHA256:    requestSHA256,
	})
	if err == nil || !strings.Contains(err.Error(), "persist current-loop resume claim directory") {
		t.Fatalf("claim directory sync failure was accepted: %v", err)
	}
	inspection := Inspect(repoRoot, caseRoot, "_template", &request)
	if !inspection.Ready || inspection.State != "ready" || inspection.ResumeDriverRequest == nil {
		t.Fatalf("failed claim persistence consumed source: %+v", inspection)
	}
}

func TestCheckpointResumeClaimIsOneShotAndFailsClosed(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit continue main -WhatIf -Format json")
	payload := checkpointPayload(t, caseRoot, request)
	written, err := Write(repoRoot, caseRoot, "_template", payload)
	if err != nil {
		t.Fatal(err)
	}
	requestSHA256, err := RequestSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	claim := Claim{
		SourceArtifactSHA256:          written.ArtifactSHA256,
		ExpectedCurrentLoopPlanSHA256: strings.Repeat("c", 64),
		CurrentDriverRequestSHA256:    requestSHA256,
		Actor:                         "main-agent",
	}
	if err := ClaimResume(repoRoot, caseRoot, "_template", claim); err != nil {
		t.Fatal(err)
	}
	consumed := Inspect(repoRoot, caseRoot, "_template", &request)
	if consumed.Ready || consumed.State != "consumed" || consumed.Continuation != nil || consumed.ResumeDriverRequest != nil {
		t.Fatalf("durably claimed checkpoint remained recoverable: %+v", consumed)
	}
	if err := ClaimResume(repoRoot, caseRoot, "_template", claim); err == nil || !strings.Contains(err.Error(), "consumed") {
		t.Fatalf("duplicate resume claim was accepted: %v", err)
	}
}

func TestCheckpointZeroProgressRecoveryRequiresStrictClaimedLineage(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit continue main -WhatIf -Format json")
	firstPayload := checkpointPayload(t, caseRoot, request)
	first, err := Write(repoRoot, caseRoot, "_template", firstPayload)
	if err != nil {
		t.Fatal(err)
	}
	requestSHA256, err := RequestSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	recovery := checkpointZeroProgressPayload(t, caseRoot, request, first.ArtifactSHA256)
	claim := Claim{
		SourceArtifactSHA256:          first.ArtifactSHA256,
		ExpectedCurrentLoopPlanSHA256: recovery.ExpectedCurrentLoopPlanSHA256,
		CurrentDriverRequestSHA256:    requestSHA256,
		Actor:                         "main-agent",
	}
	if err := ClaimResume(repoRoot, caseRoot, "_template", claim); err != nil {
		t.Fatal(err)
	}
	written, err := Write(repoRoot, caseRoot, "_template", recovery)
	if err != nil {
		t.Fatal(err)
	}
	if !written.Ready || written.State != "ready" || written.Sequence != first.Sequence+1 || written.ResumeSourceSHA256 != first.ArtifactSHA256 || written.AppliedSteps != 0 || written.RemainingMaxSteps != recovery.SegmentMaxSteps {
		t.Fatalf("zero-progress recovery did not preserve strict remaining budget: %+v", written)
	}

	missingClaim := checkpointZeroProgressPayload(t, caseRoot, request, written.ArtifactSHA256)
	missingClaim.ExpectedCurrentLoopPlanSHA256 = strings.Repeat("e", 64)
	if _, err := Write(repoRoot, caseRoot, "_template", missingClaim); err == nil || !strings.Contains(err.Error(), "durable resume claim") {
		t.Fatalf("zero-progress recovery without an exact matching claim was accepted: %v", err)
	}

	for name, mutate := range map[string]func(*Payload){
		"not-zero-progress": func(payload *Payload) { payload.ZeroProgressRecovery = false },
		"status-unavailable": func(payload *Payload) {
			payload.StatusAvailable = false
			payload.RefreshedCurrentDriverRequest = nil
			payload.RefreshedCurrentDriverRequestSHA256 = ""
			payload.Continuation = nil
		},
		"missing-step-hash": func(payload *Payload) { payload.ExpectedInitialCurrentStepSHA256 = "" },
		"wrong-stop":        func(payload *Payload) { payload.Stop.Code = "error" },
		"changed-request": func(payload *Payload) {
			changed := checkpointRequest("/rekit continue other -WhatIf -Format json")
			payload.RefreshedCurrentDriverRequest = &changed
			payload.RefreshedCurrentDriverRequestSHA256, _ = RequestSHA256(changed)
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := checkpointZeroProgressPayload(t, caseRoot, request, written.ArtifactSHA256)
			mutate(&candidate)
			candidate.SchemaVersion = 1
			candidate.Kind = "current-loop-segment-checkpoint"
			candidate.Sequence = 3
			candidate.PreviousArtifactSHA256 = written.ArtifactSHA256
			candidate.CaseIdentitySHA256 = strings.Repeat("a", 64)
			candidate.Pack = "_template"
			candidate.NoAutoApply = true
			candidate.NoAuthority = true
			if err := validatePayload(candidate); err == nil {
				t.Fatalf("invalid zero-progress recovery payload was accepted: %+v", candidate)
			}
		})
	}
}

func TestCheckpointWriteValidatedRejectsPublicationCurrentnessDrift(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit continue main -WhatIf -Format json")
	payload := checkpointPayload(t, caseRoot, request)
	called := false
	inspection, err := WriteValidated(repoRoot, caseRoot, "_template", payload, func() error {
		called = true
		return errors.New("current request drifted")
	})
	if err == nil || !called || !strings.Contains(err.Error(), "publication currentness") || inspection.ArtifactPath != "" {
		t.Fatalf("publication currentness drift was accepted: inspection=%+v err=%v", inspection, err)
	}
	root := filepath.Join(caseRoot, filepath.FromSlash(artifactRelRoot))
	if entries, readErr := os.ReadDir(root); !os.IsNotExist(readErr) && (readErr != nil || len(entries) != 0) {
		t.Fatalf("rejected publication currentness drift wrote an artifact: err=%v entries=%v", readErr, entries)
	}
}

func TestCheckpointResumeClaimBindsSourceCurrentRequest(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit continue main -WhatIf -Format json")
	written, err := Write(repoRoot, caseRoot, "_template", checkpointPayload(t, caseRoot, request))
	if err != nil {
		t.Fatal(err)
	}
	claim := Claim{
		SourceArtifactSHA256:          written.ArtifactSHA256,
		ExpectedCurrentLoopPlanSHA256: strings.Repeat("c", 64),
		CurrentDriverRequestSHA256:    strings.Repeat("d", 64),
		Actor:                         "main-agent",
	}
	if err := ClaimResume(repoRoot, caseRoot, "_template", claim); err == nil || !strings.Contains(err.Error(), "current driver request") {
		t.Fatalf("resume claim accepted a request hash not bound to the source checkpoint: %v", err)
	}
	inspection := Inspect(repoRoot, caseRoot, "_template", &request)
	if !inspection.Ready || inspection.State != "ready" || inspection.ResumeDriverRequest == nil {
		t.Fatalf("rejected request mismatch consumed the checkpoint: %+v", inspection)
	}
}

func TestCheckpointResumeSourceMustBeLatestBeforePublication(t *testing.T) {
	repoRoot, caseRoot := checkpointAttachedCase(t)
	request := checkpointRequest("/rekit continue main -WhatIf -Format json")
	payload := checkpointPayload(t, caseRoot, request)
	first, err := Write(repoRoot, caseRoot, "_template", payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Write(repoRoot, caseRoot, "_template", payload); err != nil {
		t.Fatal(err)
	}
	payload.ResumeSourceArtifactSHA256 = first.ArtifactSHA256
	if _, err := Write(repoRoot, caseRoot, "_template", payload); err == nil || !strings.Contains(err.Error(), "no longer the latest") {
		t.Fatalf("stale resume source published a successor: %v", err)
	}
	root := filepath.Join(caseRoot, filepath.FromSlash(artifactRelRoot))
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("stale resume publication changed chain length: %v entries=%d", err, len(entries))
	}
	inspection := Inspect(repoRoot, caseRoot, "_template", &request)
	if !inspection.Ready || inspection.Sequence != 2 {
		t.Fatalf("stale resume publication polluted valid chain: %+v", inspection)
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

	for _, test := range []struct {
		name  string
		write func(t *testing.T, path string)
	}{
		{
			name: "invalid-resume-claim-json",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("{\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "non-regular-resume-claim",
			write: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlink-resume-claim",
			write: func(t *testing.T, path string) {
				t.Helper()
				outside := filepath.Join(t.TempDir(), "claim.json")
				if err := os.WriteFile(outside, []byte("{}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repoRoot, caseRoot := checkpointAttachedCase(t)
			request := checkpointRequest("/rekit continue main -WhatIf -Format json")
			written, err := Write(repoRoot, caseRoot, "_template", checkpointPayload(t, caseRoot, request))
			if err != nil {
				t.Fatal(err)
			}
			claimRoot := filepath.Join(caseRoot, ".rekit", "runs", "current-loop-segment-claims")
			if err := os.MkdirAll(claimRoot, 0o700); err != nil {
				t.Fatal(err)
			}
			test.write(t, filepath.Join(claimRoot, strings.ToLower(written.ArtifactSHA256)+".json"))
			inspection := Inspect(repoRoot, caseRoot, "_template", &request)
			if inspection.Ready || inspection.State != "invalid" || inspection.Continuation != nil || inspection.ResumeDriverRequest != nil {
				t.Fatalf("invalid resume claim was accepted: %+v", inspection)
			}
		})
	}

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

func checkpointZeroProgressPayload(t *testing.T, caseRoot string, request mission.MissionCommanderDriverRequest, resumeSourceSHA256 string) Payload {
	t.Helper()
	requestSHA256, err := RequestSHA256(request)
	if err != nil {
		t.Fatal(err)
	}
	stepSHA256 := strings.Repeat("d", 64)
	payload := Payload{
		Actor:                               request.Actor,
		RoutePolicy:                         "fixed-initial-route-and-lane",
		InitialCurrentDriverRequest:         request,
		InitialCurrentDriverRequestSHA256:   requestSHA256,
		ExpectedInitialCurrentStepSHA256:    stepSHA256,
		ResumeSourceArtifactSHA256:          resumeSourceSHA256,
		ZeroProgressRecovery:                true,
		SegmentMaxSteps:                     3,
		RemainingMaxSteps:                   3,
		SegmentRoute:                        "case",
		SegmentLane:                         "main",
		Stop:                                Stop{Code: "zero-progress-retry", Phase: "apply-step"},
		StepReceipts:                        []StepReceiptBinding{},
		StatusAvailable:                     true,
		RefreshedCurrentDriverRequest:       &request,
		RefreshedCurrentDriverRequestSHA256: requestSHA256,
		Continuation: &Continuation{
			Kind:                 "current-loop-campaign-continuation",
			State:                "awaiting-fresh-segment-review",
			StopCode:             "zero-progress-retry",
			SegmentMaxSteps:      3,
			RemainingMaxSteps:    3,
			SegmentRoute:         "case",
			SegmentLane:          "main",
			ExpectedRoute:        "case",
			ExpectedLane:         "main",
			WhatIfCommand:        `/rekit run-current-loop -Target "case" -Pack _template -MaxSteps 3 -WhatIf -Format json`,
			FreshPreviewRequired: true,
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
		ResumeSourceArtifactSHA256    string                                `json:"resumeSourceArtifactSha256,omitempty"`
	}{1, caseRoot, "_template", payload.RoutePolicy, payload.SegmentMaxSteps, payload.Actor, payload.SegmentRoute, payload.SegmentLane, payload.InitialCurrentDriverRequest, stepSHA256, resumeSourceSHA256}
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
