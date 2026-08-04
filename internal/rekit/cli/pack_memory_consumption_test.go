package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/packmemoryconsumption"
)

func TestPackMemoryConsumptionStatusRoutesSelectedPreviewToFirstScreen(t *testing.T) {
	caseRoot := attachedCase(t)
	changeID := "pack-memory-change-" + strings.Repeat("a", 64)
	oldDiscover := packMemoryConsumptionDiscover
	packMemoryConsumptionDiscover = func(repoRoot, target, pack string) (packmemoryconsumption.Discovery, error) {
		return packmemoryconsumption.Discovery{
			SchemaVersion: 1,
			Kind:          packmemoryconsumption.KindDiscovery,
			RepoRoot:      repoRoot,
			CaseRoot:      target,
			Pack:          pack,
			Available: []packmemoryconsumption.ChangeStatus{{
				ChangeID:       changeID,
				ManagedPath:    "references/template/README.md",
				SourceSHA256:   strings.Repeat("b", 64),
				State:          "available",
				PreviewCommand: "/rekit sync -Target \"" + target + "\" -Pack _template -SelectPackMemoryChange " + changeID + " -WhatIf -Format json",
			}},
			Boundary: []string{"producer proof does not authorize target writes"},
		}, nil
	}
	t.Cleanup(func() { packMemoryConsumptionDiscover = oldDiscover })

	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status statusInventory
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	consumption := status.PackMemoryConsumption
	if consumption == nil || !consumption.Ready || consumption.MissionCommanderActionQueue.CurrentAction == nil || !consumption.MissionCommanderActionQueue.CurrentAction.RequiresReview {
		t.Fatalf("status omitted review-first consumption queue: %+v", consumption)
	}
	runbook := status.MissionControlRunbook
	if runbook == nil || !statusRunbookHasQueue(runbook, "pack-memory", 1, true) {
		t.Fatalf("status omitted consumption from the first-screen queues: %+v", runbook)
	}
	consumptionRunbook := buildStatusMissionControlRunbookWithConsumption(caseRoot, nil, nil, consumption)
	if consumptionRunbook.Scope != "pack-memory" || consumptionRunbook.CurrentDriverRequest == nil || consumptionRunbook.CurrentDriverRequest.ActionID != "consume-pack-memory-change-"+changeID || !consumptionRunbook.CurrentDriverRequest.RequiresReview || consumptionRunbook.ReplacementExecutorTakeover == nil {
		t.Fatalf("available consumption did not become the focused takeover route when higher-priority case work was absent: %+v", consumptionRunbook)
	}
}

func statusRunbookHasQueue(runbook *statusMissionControlRunbook, scope string, total int, requiresReview bool) bool {
	if runbook == nil {
		return false
	}
	for _, queue := range runbook.Queues {
		if queue.Scope == scope && queue.Total == total && (!requiresReview || queue.RequiresReview > 0) {
			return true
		}
	}
	return false
}

func TestPackMemoryConsumptionStatusDegradesDiscoveryFailureToWarning(t *testing.T) {
	caseRoot := attachedCase(t)
	oldDiscover := packMemoryConsumptionDiscover
	packMemoryConsumptionDiscover = func(string, string, string) (packmemoryconsumption.Discovery, error) {
		return packmemoryconsumption.Discovery{}, errors.New("fixture discovery failure")
	}
	t.Cleanup(func() { packMemoryConsumptionDiscover = oldDiscover })

	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatalf("status must remain readable: %v", err)
	}
	var status statusInventory
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.PackMemoryConsumption == nil || status.PackMemoryConsumption.Ready || len(status.PackMemoryConsumption.Warnings) != 1 || !strings.Contains(status.PackMemoryConsumption.Warnings[0], "fixture discovery failure") {
		t.Fatalf("status did not expose degraded discovery warning: %+v", status.PackMemoryConsumption)
	}
}

func TestRunPackMemorySelectedSyncValidatesReviewContract(t *testing.T) {
	caseRoot := attachedCase(t)
	changeID := "pack-memory-change-" + strings.Repeat("a", 64)
	planHash := strings.Repeat("b", 64)
	oldPreview, oldApply := packMemoryConsumptionPreview, packMemoryConsumptionApply
	packMemoryConsumptionPreview = func(repoRoot, target, pack, selected string) (packmemoryconsumption.Plan, error) {
		if target != caseRoot || pack != "_template" || selected != changeID {
			t.Fatalf("preview bindings drifted: target=%s pack=%s selected=%s", target, pack, selected)
		}
		return packmemoryconsumption.Plan{SchemaVersion: 1, Kind: packmemoryconsumption.KindPlan, ChangeID: selected, ExpectedPlanSHA256: planHash}, nil
	}
	packMemoryConsumptionApply = func(repoRoot, target, pack, selected, expected string) (packmemoryconsumption.Result, error) {
		if target != caseRoot || pack != "_template" || selected != changeID || expected != planHash {
			t.Fatalf("apply bindings drifted: target=%s pack=%s selected=%s expected=%s", target, pack, selected, expected)
		}
		return packmemoryconsumption.Result{Plan: packmemoryconsumption.Plan{SchemaVersion: 1, Kind: packmemoryconsumption.KindPlan, ChangeID: selected, Applied: true}, Receipt: packmemoryconsumption.Receipt{SchemaVersion: 1, Kind: packmemoryconsumption.KindReceipt, ChangeID: selected}}, nil
	}
	t.Cleanup(func() {
		packMemoryConsumptionPreview = oldPreview
		packMemoryConsumptionApply = oldApply
	})

	var out bytes.Buffer
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-SelectPackMemoryChange", changeID, "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var plan packmemoryconsumption.Plan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil || plan.ExpectedPlanSHA256 != planHash {
		t.Fatalf("unexpected preview: plan=%+v err=%v", plan, err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-SelectPackMemoryChange", changeID, "-ExpectedPackMemoryConsumptionPlanSha256", planHash, "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-SelectPackMemoryChange", changeID, "-Apply", "-Format", "json"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "requires -ExpectedPackMemoryConsumptionPlanSha256") {
		t.Fatalf("Apply without reviewed hash did not fail closed: %v", err)
	}
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-ExpectedPackMemoryConsumptionPlanSha256", planHash, "-Apply", "-Format", "json"}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "requires -SelectPackMemoryChange") {
		t.Fatalf("orphan expected hash did not fail closed: %v", err)
	}
}
