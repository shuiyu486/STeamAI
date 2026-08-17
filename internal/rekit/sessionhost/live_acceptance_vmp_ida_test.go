package sessionhost

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestAssertLiveAcceptanceNoAuthorityUsesSelectedStateRoot(t *testing.T) {
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		t.Run(stateDir, func(t *testing.T) {
			caseRoot := t.TempDir()
			if err := os.Mkdir(filepath.Join(caseRoot, stateDir), 0o700); err != nil {
				t.Fatal(err)
			}
			if _, _, err := autonomy.EnsureManualProfile(caseRoot, "main"); err != nil {
				t.Fatal(err)
			}
			if err := assertLiveAcceptanceNoAuthority(caseRoot, "main"); err != nil {
				t.Fatal(err)
			}
			_, authorityPath, err := mission.FactPath(caseRoot, "authority")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(authorityPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(authorityPath, []byte("{}\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := assertLiveAcceptanceNoAuthority(caseRoot, "main"); err == nil || !strings.Contains(err.Error(), "authority ledger state") {
				t.Fatalf("authority ledger check error = %v", err)
			}
		})
	}

	dualRoot := t.TempDir()
	for _, stateDir := range []string{projectstate.CurrentDir, projectstate.LegacyDir} {
		if err := os.Mkdir(filepath.Join(dualRoot, stateDir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := assertLiveAcceptanceNoAuthority(dualRoot, "main"); err == nil || !strings.Contains(err.Error(), "both .steamai and .rekit") {
		t.Fatalf("dual-root authority check error = %v", err)
	}
}

func TestValidateLiveAcceptanceVMPIDAMemberArtifactRequiresExactLineage(t *testing.T) {
	member, evidence, proof := vmpIDAMemberArtifactFixture(t)
	if err := validateLiveAcceptanceVMPIDAMemberArtifact(member, evidence, proof); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*memberexecution.Inspection, *liveAcceptanceVMPIDAEvidenceReviewInput, *LiveAcceptanceVMPIDA){
		"binding": func(member *memberexecution.Inspection, _ *liveAcceptanceVMPIDAEvidenceReviewInput, _ *LiveAcceptanceVMPIDA) {
			member.TaskContext.Binding.Values["receipt-sha256"] = strings.Repeat("0", 64)
		},
		"extra binding": func(member *memberexecution.Inspection, _ *liveAcceptanceVMPIDAEvidenceReviewInput, _ *LiveAcceptanceVMPIDA) {
			member.TaskContext.Binding.Values["unexpected"] = "value"
		},
		"reviewer items path": func(member *memberexecution.Inspection, _ *liveAcceptanceVMPIDAEvidenceReviewInput, _ *LiveAcceptanceVMPIDA) {
			member.Manifest.ReviewerItemsPath = "other.txt"
		},
		"output hash": func(member *memberexecution.Inspection, _ *liveAcceptanceVMPIDAEvidenceReviewInput, _ *LiveAcceptanceVMPIDA) {
			member.Manifest.Outputs[0].SHA256 = strings.Repeat("0", 64)
		},
		"selected row": func(_ *memberexecution.Inspection, evidence *liveAcceptanceVMPIDAEvidenceReviewInput, _ *LiveAcceptanceVMPIDA) {
			evidence.Selected.Row = "missing exact source row"
		},
		"selected ref": func(_ *memberexecution.Inspection, evidence *liveAcceptanceVMPIDAEvidenceReviewInput, _ *LiveAcceptanceVMPIDA) {
			evidence.Selected.EvidenceRef = "ida-index:missing#L9"
		},
		"packet": func(_ *memberexecution.Inspection, evidence *liveAcceptanceVMPIDAEvidenceReviewInput, _ *LiveAcceptanceVMPIDA) {
			evidence.PacketPath = "missing-packet.json"
		},
		"receipt": func(_ *memberexecution.Inspection, evidence *liveAcceptanceVMPIDAEvidenceReviewInput, _ *LiveAcceptanceVMPIDA) {
			evidence.ReceiptPath = "missing-receipt.json"
		},
		"observation": func(_ *memberexecution.Inspection, evidence *liveAcceptanceVMPIDAEvidenceReviewInput, _ *LiveAcceptanceVMPIDA) {
			evidence.ObservationEventID = "obs-missing"
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneVMPIDAMemberInspection(member)
			candidateEvidence := evidence
			candidateProof := *proof
			mutate(&candidate, &candidateEvidence, &candidateProof)
			if err := validateLiveAcceptanceVMPIDAMemberArtifact(candidate, candidateEvidence, &candidateProof); err == nil {
				t.Fatalf("member validator accepted %s drift", name)
			}
		})
	}
}

func TestValidateLiveAcceptanceVMPIDAMemberArtifactAcceptsExactJSONEscapedTSVRow(t *testing.T) {
	member, evidence, proof := vmpIDAMemberArtifactFixture(t)
	path := filepath.Join(member.OutputsRoot, member.Manifest.Outputs[0].Path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), evidence.Selected.Row, strings.ReplaceAll(evidence.Selected.Row, "\t", `\t`)))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	member.Manifest.Outputs[0].Bytes = int64(len(data))
	member.Manifest.Outputs[0].SHA256 = bytesSHA256(data)
	if err := validateLiveAcceptanceVMPIDAMemberArtifact(member, evidence, proof); err != nil {
		t.Fatalf("exact JSON-escaped TSV row was rejected: %v", err)
	}
}

func TestValidateLiveAcceptanceVMPIDAMemberArtifactRejectsKeywordOnlyOutput(t *testing.T) {
	member, evidence, proof := vmpIDAMemberArtifactFixture(t)
	data := []byte(liveAcceptanceVMPIDAQueryTerm + "\n")
	path := filepath.Join(member.OutputsRoot, member.Manifest.Outputs[0].Path)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	member.Manifest.Outputs[0].Bytes = int64(len(data))
	member.Manifest.Outputs[0].SHA256 = bytesSHA256(data)
	if err := validateLiveAcceptanceVMPIDAMemberArtifact(member, evidence, proof); err == nil {
		t.Fatal("member validator accepted query-term-only output without immutable lineage")
	}
}

func TestValidateLiveAcceptanceVMPIDAReviewerArtifactAcceptsLineageInRouteOutput(t *testing.T) {
	caseRoot, member, acceptance, evidence, result := vmpIDAReviewerArtifactFixture(t)
	result.Summary = "accepted exact VMP IDA lineage"
	result.RouteOutput["evidence"] = evidence.Selected.EvidenceRef
	result.RouteOutput["request_id"] = evidence.ObservationEventID
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	acceptance.ReviewerResultInputSHA256 = bytesSHA256(data)
	acceptance.ReviewerResultInputBytes = int64(len(data))
	if err := validateLiveAcceptanceVMPIDAReviewerArtifact(caseRoot, member, acceptance, evidence, data); err != nil {
		t.Fatalf("routeOutput exact lineage was rejected: %v", err)
	}
}

func TestValidateLiveAcceptanceVMPIDAReviewerArtifactRequiresExactLineage(t *testing.T) {
	caseRoot, member, acceptance, evidence, result := vmpIDAReviewerArtifactFixture(t)
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateLiveAcceptanceVMPIDAReviewerArtifact(caseRoot, member, acceptance, evidence, data); err != nil {
		t.Fatal(err)
	}
	manifestRef := relativeLiveAcceptancePath(caseRoot, member.ManifestPath)
	for name, mutate := range map[string]func(*workstream.MemberReviewerAcceptance, *reviewerresult.Result){
		"owner generation": func(acceptance *workstream.MemberReviewerAcceptance, _ *reviewerresult.Result) {
			acceptance.OwnerGeneration++
		},
		"manifest": func(acceptance *workstream.MemberReviewerAcceptance, _ *reviewerresult.Result) {
			acceptance.ManifestSHA256 = strings.Repeat("2", 64)
		},
		"decision": func(_ *workstream.MemberReviewerAcceptance, result *reviewerresult.Result) {
			result.Decision = "reject"
		},
		"verdict": func(_ *workstream.MemberReviewerAcceptance, result *reviewerresult.Result) {
			result.RecommendedVerdict = "rejected"
		},
		"items": func(_ *workstream.MemberReviewerAcceptance, result *reviewerresult.Result) {
			result.Items = []string{"other-manifest.json"}
		},
		"selected ref": func(_ *workstream.MemberReviewerAcceptance, result *reviewerresult.Result) {
			result.Summary = strings.ReplaceAll(result.Summary, evidence.Selected.EvidenceRef, "missing-selected-ref")
			result.RouteOutput = map[string]any{"item": manifestRef}
		},
		"packet path": func(_ *workstream.MemberReviewerAcceptance, result *reviewerresult.Result) {
			result.EvidenceRefs = []string{evidence.ReceiptPath}
		},
		"receipt path": func(_ *workstream.MemberReviewerAcceptance, result *reviewerresult.Result) {
			result.EvidenceRefs = []string{evidence.PacketPath}
		},
		"observation": func(_ *workstream.MemberReviewerAcceptance, result *reviewerresult.Result) {
			result.Summary = strings.ReplaceAll(result.Summary, evidence.ObservationEventID, "obs-missing")
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidateAcceptance := acceptance
			candidateResult := result
			candidateResult.Items = append([]string{}, result.Items...)
			candidateResult.EvidenceRefs = append([]string{}, result.EvidenceRefs...)
			candidateResult.RouteOutput = maps.Clone(result.RouteOutput)
			mutate(&candidateAcceptance, &candidateResult)
			candidateData, marshalErr := json.Marshal(candidateResult)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if name != "owner generation" && name != "manifest" {
				candidateAcceptance.ReviewerResultInputSHA256 = bytesSHA256(candidateData)
				candidateAcceptance.ReviewerResultInputBytes = int64(len(candidateData))
			}
			if err := validateLiveAcceptanceVMPIDAReviewerArtifact(caseRoot, member, candidateAcceptance, evidence, candidateData); err == nil {
				t.Fatalf("Reviewer validator accepted %s drift", name)
			}
		})
	}
}

func vmpIDAReviewerArtifactFixture(t *testing.T) (string, memberexecution.Inspection, workstream.MemberReviewerAcceptance, liveAcceptanceVMPIDAEvidenceReviewInput, reviewerresult.Result) {
	t.Helper()
	caseRoot := t.TempDir()
	manifestPath := filepath.Join(caseRoot, ".rekit", "lanes", "feature-analysis", "member-executions", "g000002", "result", "manifest.json")
	member := memberexecution.Inspection{
		Owner:    memberexecution.Owner{Lane: "feature-analysis", Executor: "member-g2", ExecutorGeneration: 2},
		Manifest: &memberexecution.ResultManifest{}, ManifestPath: manifestPath, ManifestSHA256: strings.Repeat("1", 64),
		TaskContext: &memberexecution.TaskContext{OutputContract: &memberexecution.OutputContract{RouteID: liveAcceptancePack + ":lane-feature-analysis"}},
	}
	manifestRef := relativeLiveAcceptancePath(caseRoot, manifestPath)
	evidence := liveAcceptanceVMPIDAEvidenceReviewInput{
		PacketPath:  "captures/feature_analysis/feature-mission/ida-index/session-1/evidence-packet.json",
		ReceiptPath: ".rekit/adapter-executions/receipt.json", ObservationEventID: "obs-exact",
		Selected: liveAcceptanceVMPIDASelectedEvidence{EvidenceRef: "ida-index:function_index.tsv#L2"},
	}
	result := reviewerresult.Result{
		PacketID: "packet-exact", RouteID: liveAcceptancePack + ":lane-feature-analysis", ShardID: "shard-01", Items: []string{manifestRef}, ReviewerSession: "reviewer-session",
		Decision: "accept", Confidence: "high", Summary: "accepted exact VMP IDA lineage " + evidence.Selected.EvidenceRef + " " + evidence.ObservationEventID, EvidenceRefs: []string{evidence.PacketPath, evidence.ReceiptPath},
		Risks: []string{}, Conflicts: []string{}, RecommendedVerdict: "accepted", RouteOutput: map[string]any{"item": manifestRef},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	acceptance := workstream.MemberReviewerAcceptance{
		ManifestSHA256: member.ManifestSHA256, PacketID: result.PacketID, RouteID: result.RouteID, ShardID: result.ShardID,
		ReviewerResultInputSHA256: bytesSHA256(data), ReviewerResultInputBytes: int64(len(data)), ReviewerSession: result.ReviewerSession,
		OwnerExecutor: member.Owner.Executor, OwnerGeneration: member.Owner.ExecutorGeneration,
	}
	return caseRoot, member, acceptance, evidence, result
}

func vmpIDAMemberArtifactFixture(t *testing.T) (memberexecution.Inspection, liveAcceptanceVMPIDAEvidenceReviewInput, *LiveAcceptanceVMPIDA) {
	t.Helper()
	root := t.TempDir()
	outputRoot := filepath.Join(root, "outputs")
	if err := os.MkdirAll(outputRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	proof := &LiveAcceptanceVMPIDA{
		GateEventID: "gate-exact", ProfileSHA256: strings.Repeat("a", 64), RequestPath: ".rekit/requests/request.json", RequestSHA256: strings.Repeat("b", 64),
		Run: adapterhost.AuthorizedRunResult{
			PacketPath: "captures/feature_analysis/feature-mission/ida-index/session-1/evidence-packet.json", PacketSHA256: strings.Repeat("c", 64),
			ReportPath: "captures/feature_analysis/feature-mission/ida-index/session-1/adapter-report.json", ReportSHA256: strings.Repeat("d", 64),
			DispatchPath: ".rekit/adapter-executions/dispatch.json", DispatchSHA256: strings.Repeat("e", 64),
			ReceiptPath: ".rekit/adapter-executions/receipt.json", ReceiptSHA256: strings.Repeat("f", 64), ObservationEventID: "obs-exact",
		},
	}
	evidence := liveAcceptanceVMPIDAEvidenceReviewInput{
		PacketPath: proof.Run.PacketPath, ReceiptPath: proof.Run.ReceiptPath, ObservationEventID: proof.Run.ObservationEventID,
		Selected: liveAcceptanceVMPIDASelectedEvidence{Row: "0x1000\tneedle_dispatch\t32", EvidenceRef: "ida-index:function_index.tsv#L2"},
	}
	data := []byte(strings.Join([]string{evidence.Selected.Row, evidence.Selected.EvidenceRef, evidence.PacketPath, evidence.ReceiptPath, evidence.ObservationEventID}, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(outputRoot, "review-items.txt"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"gate-event-id": proof.GateEventID, "profile-hash": proof.ProfileSHA256,
		"request-path": proof.RequestPath, "request-sha256": proof.RequestSHA256,
		"packet-path": proof.Run.PacketPath, "packet-sha256": proof.Run.PacketSHA256,
		"report-path": proof.Run.ReportPath, "report-sha256": proof.Run.ReportSHA256,
		"dispatch-path": proof.Run.DispatchPath, "dispatch-sha256": proof.Run.DispatchSHA256,
		"receipt-path": proof.Run.ReceiptPath, "receipt-sha256": proof.Run.ReceiptSHA256,
		"observation-event-id": proof.Run.ObservationEventID,
	}
	member := memberexecution.Inspection{
		TaskContext: &memberexecution.TaskContext{Binding: &memberexecution.TaskBinding{Kind: "vmp-ida-index-evidence", Values: values}},
		Manifest:    &memberexecution.ResultManifest{ReviewerItemsPath: "review-items.txt", Outputs: []memberexecution.Output{{Path: "review-items.txt", SHA256: bytesSHA256(data), Bytes: int64(len(data))}}},
		OutputsRoot: outputRoot,
	}
	return member, evidence, proof
}

func cloneVMPIDAMemberInspection(input memberexecution.Inspection) memberexecution.Inspection {
	clone := input
	task := *input.TaskContext
	binding := *input.TaskContext.Binding
	binding.Values = make(map[string]string, len(input.TaskContext.Binding.Values))
	maps.Copy(binding.Values, input.TaskContext.Binding.Values)
	task.Binding = &binding
	manifest := *input.Manifest
	manifest.Outputs = append([]memberexecution.Output{}, input.Manifest.Outputs...)
	clone.TaskContext = &task
	clone.Manifest = &manifest
	return clone
}
