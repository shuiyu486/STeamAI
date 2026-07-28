package workstream

import (
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
)

func TestApplyAdapterExecutionReceiptHandoffProjectsCompactLineage(t *testing.T) {
	receipt := &adapterexecution.Receipt{
		Owner: adapterexecution.OwnerBinding{
			CurrentExecutor: "executor-a", ExecutorGeneration: 4,
			AdapterHarness: "claude-code", AdapterSession: "session-a",
		},
		Adapter:   adapterexecution.AdapterBinding{ToolingCatalogSHA256: strings.Repeat("a", 64)},
		Artifacts: []adapterexecution.ArtifactBinding{{Path: "workspace/main/result.bin"}, {Path: "workspace/main/evidence.json"}},
	}
	validation := gate.AdapterExecutionReportValidation{
		ReceiptRequired: true, ReceiptPresent: true, ProvenanceValid: true,
		AdapterExecutionReceiptPath:   ".rekit/lanes/main/adapter-executions/evt-gate/receipt.json",
		AdapterExecutionReceiptSHA256: strings.Repeat("b", 64),
		ReceiptPreviewCommand:         "/rekit gate -RecordAdapterExecutionReceipt",
		AdapterExecution:              receipt,
	}
	var handoff AuthorizedGateLiveValidationHandoff
	applyAdapterExecutionReceiptHandoff(&handoff, validation)
	if !handoff.ReceiptRequired || !handoff.ReceiptPresent || !handoff.ProvenanceValid || handoff.AdapterExecutionReceiptPath == "" || handoff.AdapterExecutionReceiptSHA256 != strings.Repeat("b", 64) || handoff.CurrentExecutor != "executor-a" || handoff.ExecutorGeneration != 4 || handoff.AdapterHarness != "claude-code" || handoff.AdapterSession != "session-a" || handoff.ToolingCatalogSHA256 != strings.Repeat("a", 64) || handoff.ArtifactCount != 2 {
		t.Fatalf("authorized gate handoff omitted compact receipt lineage: %+v", handoff)
	}
}
