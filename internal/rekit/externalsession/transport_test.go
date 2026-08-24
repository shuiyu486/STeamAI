package externalsession

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewerresult"
	"github.com/shuiyu486/re-context-kits/internal/rekit/reviewersession"
)

const (
	remoteControlTestActor   = "mission-commander"
	remoteControlTestSession = "remote-reviewer-binding-a"

	transportReviewerShapePrefix    = "Reviewer result JSON shape template: "
	transportReviewerShapeSuffix    = ". The shape template contains instructions, not default verdict values: inspect the listed evidence before selecting decision, confidence, recommendedVerdict, risk, next_action, and defer_reason; do not copy instruction text into the result."
	transportReviewerEvidencePrefix = "For each member evidence manifest, use its own exact binding: "
	transportReviewerEvidenceSuffix = " The deterministic runtime revalidates declared SHA-256 and byte counts during strict completion; do not invent a missing semantic or provenance check."
)

type remoteControlFixture struct {
	job      Job
	attempt  AttemptInspection
	dispatch DispatchInspection
	item     string
}

func TestRemoteControlTransportVerticalSlicePublishesSelfContainedBundle(t *testing.T) {
	fixture := newRemoteControlFixture(t)

	inspection, err := InspectTransport(fixture.job, fixture.attempt, fixture.dispatch)
	if err != nil || inspection.State != "endpoint-required" {
		t.Fatalf("transport=%+v err=%v", inspection, err)
	}
	endpoint, err := PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T01:02:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Endpoint == nil || endpoint.Endpoint.Envelope.Operation != "SendMessage" ||
		endpoint.Endpoint.Envelope.Recipient != "reviewer [opaque-ref]" ||
		endpoint.Endpoint.Capability != fixture.job.Capability ||
		endpoint.Endpoint.Envelope.Capability != fixture.job.Capability ||
		!endpoint.Endpoint.Envelope.NoFileTransfer ||
		endpoint.BundlePath == "" || endpoint.BundleSHA256 == "" || endpoint.BundleBytes == 0 {
		t.Fatalf("endpoint plan=%+v", endpoint)
	}
	if !bytes.Contains([]byte(endpoint.Endpoint.Envelope.Message), []byte("bounded reviewer evidence")) ||
		!bytes.Contains([]byte(endpoint.Endpoint.Envelope.Message), []byte("--- BEGIN COMMITTED EVIDENCE BUNDLE ---")) ||
		strings.Contains(strings.ToLower(endpoint.Endpoint.Envelope.Message), strings.ToLower(fixture.job.CaseRoot)) {
		t.Fatal("transport message is not a self-contained root-free evidence bundle")
	}
	applied, err := ApplyTransportCurrent(endpoint, endpoint.ExpectedPlanSHA256, func() (Job, error) {
		return fixture.job, nil
	})
	if err != nil || !applied.Applied || applied.AlreadyApplied {
		t.Fatalf("endpoint apply=%+v err=%v", applied, err)
	}

	transport, err := InspectTransport(fixture.job, fixture.attempt, fixture.dispatch)
	if err != nil || transport.State != "delivery-required" || transport.Endpoint == nil {
		t.Fatalf("transport=%+v err=%v", transport, err)
	}
	bundleData := readTestFile(t, fixture.job.CaseRoot, transport.BundlePath)
	var bundle TransportEvidenceBundle
	if err := decodeCanonicalTransport(bundleData, &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.ArtifactCount != 4 || len(bundle.Closures) != 1 || bundle.Closures[0].Item != fixture.item ||
		bundle.Prompt.Content == "" || bundle.NoFileTransfer != true || bundle.NoHeavyTool != true ||
		bundle.NoAuthority != true || bundle.NoConfirmed != true {
		t.Fatalf("bundle=%+v", bundle)
	}

	delivery, err := PreviewTransportDelivery(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"accepted",
		strings.Repeat("e", 64),
		remoteControlTestActor,
		"2026-08-12T01:03:00Z",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransportCurrent(delivery, delivery.ExpectedPlanSHA256, func() (Job, error) {
		return fixture.job, nil
	}); err != nil {
		t.Fatal(err)
	}
	transport, err = InspectTransport(fixture.job, fixture.attempt, fixture.dispatch)
	if err != nil || transport.State != "delivery-accepted" {
		t.Fatalf("transport=%+v err=%v", transport, err)
	}
	outcome, actor, observedAt, harness, session, reason, err := TransportLaunchTransition(transport)
	if err != nil || outcome != "accepted" || actor != remoteControlTestActor || observedAt != "2026-08-12T01:03:00Z" ||
		harness != RemoteControlHarness || session != remoteControlTestSession || reason != "" {
		t.Fatalf("launch transition=%q %q %q %q %q %q err=%v", outcome, actor, observedAt, harness, session, reason, err)
	}
}

func TestCompactTransportReviewerPromptRequiresExactCanonicalContract(t *testing.T) {
	item := ".rekit/lanes/feature-analysis/member-executions/g000001-a000001/evidence/manifest.json"
	fields := []string{"item", "decision", "confidence", "evidence", "risk", "next_action", "tier_used", "tool_scope", "defer_reason"}
	receipt := reviewersession.DispatchReceipt{
		PacketID: "packet-a", RouteID: "route-a", ShardID: "shard-a", Items: []string{item},
		TargetLane: "feature-review", EffectiveOwner: reviewersession.Owner{
			CurrentExecutor: "review-owner", ExecutorGeneration: 3, BindingMode: "current-executor-generation",
		},
		ReviewerSession: "reviewer-session-a",
	}
	contract := reviewerresult.CurrentContract()
	shape, err := transportCanonicalSourceReviewerShape(receipt, fields)
	if err != nil {
		t.Fatal(err)
	}
	attemptRoot := strings.TrimSuffix(item, "/evidence/manifest.json")
	source := strings.Join([]string{
		"You are a read-only reviewer for rekit plan-subagents shard " + receipt.ShardID + ".",
		"Route: " + receipt.RouteID + ".",
		"Items: " + strings.Join(receipt.Items, ", ") + ".",
		"Owner binding: targetLane=feature-review, mode=current-executor-generation, currentExecutor=review-owner, executorGeneration=3.",
		"Return exactly one reviewer result JSON object; do not return routeOutput alone.",
		"Reviewer result contract: " + contract.OutputFormat + ".",
		"Required result fields: " + strings.Join(contract.RequiredFields, ", ") + ".",
		"Route output required fields: item=exact item.",
		transportReviewerShapePrefix + shape + transportReviewerShapeSuffix,
		"Choose accept with recommendedVerdict=accepted when the immutable evidence and its declared hashes support the bounded item; choose reject only with inspected contrary evidence; choose defer or needs-more-evidence only when a concrete evidence gap remains.",
		"Use the exact canonical decision mapping for recommendedVerdict: accept=accepted, reject=rejected, defer=inconclusive, abandon=inconclusive, needs-more-evidence=needs-more-evidence. Do not invent synonyms such as deferred.",
		transportReviewerEvidencePrefix + "item " + string(mustJSON(item)) + " uses immutable task context " + string(mustJSON(attemptRoot+"/task-context.json")) + " and exact canonical outputs root " + string(mustJSON(attemptRoot+"/evidence/outputs")) + ". For each binding, first read that immutable task context and the manifest item. Treat explicit acceptance requirements in the task context as mandatory output conditions: contrary evidence must produce decision=reject with recommendedVerdict=rejected, not accept or defer. A self-consistent manifest does not satisfy a missing goal, acceptance requirement, or correction requirement. Keep every item exactly as listed in Items and keep routeOutput.item equal to the reviewed item required by the route contract." + transportReviewerEvidenceSuffix,
		"Replace packet.packetId with the packet packetId, set routeId to " + receipt.RouteID + ", shardId to " + receipt.ShardID + ", and set reviewerSession to your session identifier supplied by the main agent.",
		"Allowed decisions: " + strings.Join(contract.AllowedDecisions, ", ") + ".",
		"Return the result to the main agent. The main agent will save it directly at reviewerResultPath for strict reviewer intake: result.json. Do not write ledger paths yourself.",
		"Do not write files, run heavy tools, append ledgers, or change authority/confirmed state.",
		"The main agent owns reviewer spawn, merge, validation, handoff, and ledger writeback: /rekit plan-subagents -ReviewerResultPath result.json.",
	}, " ")

	compacted, ok := compactTransportReviewerPrompt(source, receipt, fields)
	if !ok || compacted == source || len(compacted) >= len(source) {
		t.Fatalf("canonical Reviewer prompt was not compacted: ok=%t source=%d compacted=%d", ok, len(source), len(compacted))
	}
	for _, expected := range []string{
		"packetId=packet-a", "reviewerSession=reviewer-session-a", mustJSON(receipt.Items),
		"member-task-context", "member-evidence-manifest", "member-evidence-output",
		"explicit acceptance requirements", "decision=reject", "recommendedVerdict=rejected",
		"Required routeOutput fields: " + strings.Join(fields, ", "), "tool_scope must be read-only",
		"Do not write files or ledgers, run heavy tools", "authority/confirmed state",
	} {
		if !strings.Contains(compacted, expected) {
			t.Fatalf("compacted Reviewer prompt omitted %q:\n%s", expected, compacted)
		}
	}

	short := "Review this custom bounded evidence exactly as written."
	shortResult, shortCompacted := compactTransportReviewerPrompt(short, receipt, fields)
	if shortCompacted || shortResult != short {
		t.Fatalf("transport renderer expanded a source that does not benefit from compaction: compacted=%t got=%q", shortCompacted, shortResult)
	}
}

func transportCanonicalSourceReviewerShape(receipt reviewersession.DispatchReceipt, fields []string) (string, error) {
	routeOutput := map[string]string{}
	for _, field := range fields {
		routeOutput[field] = "fill " + field
	}
	shape := struct {
		PacketID           string            `json:"packetId"`
		RouteID            string            `json:"routeId"`
		ShardID            string            `json:"shardId"`
		Items              []string          `json:"items"`
		ReviewerSession    string            `json:"reviewerSession"`
		Decision           string            `json:"decision"`
		Confidence         string            `json:"confidence"`
		Summary            string            `json:"summary"`
		EvidenceRefs       []string          `json:"evidenceRefs"`
		Risks              []string          `json:"risks"`
		Conflicts          []string          `json:"conflicts"`
		RecommendedVerdict string            `json:"recommendedVerdict"`
		RouteOutput        map[string]string `json:"routeOutput"`
	}{
		PacketID: "packet.packetId", RouteID: receipt.RouteID, ShardID: receipt.ShardID,
		Items: append([]string{}, receipt.Items...), ReviewerSession: "reviewer-session-id",
		Decision: "select after inspecting evidence", Confidence: "select after inspecting evidence",
		Summary: "fill evidence-based summary for this shard", EvidenceRefs: []string{"packet.packetId"},
		Risks: []string{}, Conflicts: []string{}, RecommendedVerdict: "map from the selected decision", RouteOutput: routeOutput,
	}
	data, err := json.Marshal(shape)
	return string(data), err
}

func TestRemoteControlTransportRejectsMemberAndNonExplicitReviewerBindings(t *testing.T) {
	memberJob := dispatchTestJob(t)
	memberPlan, err := PreviewAttempt(memberJob, RemoteControlHarness, "member-binding", "actor", "2026-08-12T02:00:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	memberPlan = bindDispatchTestPlan(t, memberPlan)
	if _, err := ApplyAttempt(memberPlan, memberPlan.JobSHA256, memberPlan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	memberAttempt, err := InspectAttempt(memberJob)
	if err != nil {
		t.Fatal(err)
	}
	memberDispatch, err := InspectCurrentDispatch(memberJob, memberAttempt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InspectTransport(memberJob, memberAttempt, memberDispatch); err == nil || !strings.Contains(err.Error(), "explicit matching durable reviewer") {
		t.Fatalf("member transport error=%v", err)
	}

	fixture := newRemoteControlFixture(t)
	local := fixture.job
	copy := *local.Reviewer
	copy.Harness = "claude-code"
	local.Reviewer = &copy
	if _, err := InspectTransport(local, fixture.attempt, fixture.dispatch); err == nil || !strings.Contains(err.Error(), "explicit matching durable reviewer") {
		t.Fatalf("implicit reviewer transport error=%v", err)
	}
}

func TestRemoteControlDeliveryOutcomesDeriveOnlyCertainLaunchTruth(t *testing.T) {
	for _, tt := range []struct {
		outcome string
		reason  string
		want    string
	}{
		{outcome: "rejected", reason: "peer refused assignment", want: "failed"},
		{outcome: "uncertain", reason: "provider returned no stable acknowledgement"},
	} {
		t.Run(tt.outcome, func(t *testing.T) {
			fixture := newRemoteControlFixture(t)
			applyRemoteControlEndpoint(t, fixture)
			delivery, err := PreviewTransportDelivery(
				fixture.job,
				fixture.attempt,
				fixture.dispatch,
				tt.outcome,
				"",
				remoteControlTestActor,
				"2026-08-12T03:03:00Z",
				tt.reason,
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ApplyTransportCurrent(delivery, delivery.ExpectedPlanSHA256, nil); err != nil {
				t.Fatal(err)
			}
			transport, err := InspectTransport(fixture.job, fixture.attempt, fixture.dispatch)
			if err != nil || transport.State != "delivery-"+tt.outcome {
				t.Fatalf("transport=%+v err=%v", transport, err)
			}
			outcome, _, _, harness, session, reason, transitionErr := TransportLaunchTransition(transport)
			if tt.outcome == "uncertain" {
				if transitionErr == nil || !strings.Contains(transitionErr.Error(), "new durable Reviewer dispatch") || outcome != "" || harness != "" || session != "" {
					t.Fatalf("uncertain transition=%q %q %q %q err=%v", outcome, harness, session, reason, transitionErr)
				}
				return
			}
			if transitionErr != nil || outcome != tt.want || harness != "" || session != "" || reason != "Remote Control SendMessage rejected: "+tt.reason {
				t.Fatalf("rejected transition=%q %q %q %q err=%v", outcome, harness, session, reason, transitionErr)
			}
		})
	}
}

func TestRemoteControlDispatchLaunchRequiresExactDeliveryDerivation(t *testing.T) {
	fixture := newRemoteControlFixture(t)
	if _, err := PreviewDispatchTransition(
		fixture.job,
		fixture.attempt,
		"accepted",
		remoteControlTestActor,
		"2026-08-12T03:03:00Z",
		RemoteControlHarness,
		remoteControlTestSession,
		"",
	); err == nil || !strings.Contains(err.Error(), "delivery observation") {
		t.Fatalf("launch without delivery error=%v", err)
	}

	applyRemoteControlEndpoint(t, fixture)
	uncertainPlan, err := PreviewTransportDelivery(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"uncertain",
		"",
		remoteControlTestActor,
		"2026-08-12T03:03:00Z",
		"provider returned no stable acknowledgement",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransportCurrent(uncertainPlan, uncertainPlan.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewDispatchTransition(
		fixture.job,
		fixture.attempt,
		"accepted",
		remoteControlTestActor,
		"2026-08-12T03:03:00Z",
		RemoteControlHarness,
		remoteControlTestSession,
		"",
	); err == nil || !strings.Contains(err.Error(), "cannot become launch truth") {
		t.Fatalf("uncertain delivery launch error=%v", err)
	}

	fixture = newRemoteControlFixture(t)
	applyRemoteControlEndpoint(t, fixture)
	acceptedPlan, err := PreviewTransportDelivery(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"accepted",
		strings.Repeat("e", 64),
		remoteControlTestActor,
		"2026-08-12T03:03:00Z",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransportCurrent(acceptedPlan, acceptedPlan.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewDispatchTransition(
		fixture.job,
		fixture.attempt,
		"accepted",
		remoteControlTestActor,
		"2026-08-12T03:04:00Z",
		RemoteControlHarness,
		remoteControlTestSession,
		"",
	); err == nil || !strings.Contains(err.Error(), "derive exactly") {
		t.Fatalf("drifted accepted launch error=%v", err)
	}
	if _, err := PreviewDispatchTransition(
		fixture.job,
		fixture.attempt,
		"accepted",
		remoteControlTestActor,
		"2026-08-12T03:03:00Z",
		RemoteControlHarness,
		remoteControlTestSession,
		"",
	); err != nil {
		t.Fatalf("exact accepted launch error=%v", err)
	}

	fixture = newRemoteControlFixture(t)
	applyRemoteControlEndpoint(t, fixture)
	rejectedPlan, err := PreviewTransportDelivery(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"rejected",
		"",
		remoteControlTestActor,
		"2026-08-12T03:03:00Z",
		"peer refused assignment",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransportCurrent(rejectedPlan, rejectedPlan.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewDispatchTransition(
		fixture.job,
		fixture.attempt,
		"failed",
		remoteControlTestActor,
		"2026-08-12T03:03:00Z",
		"",
		"",
		"Remote Control SendMessage rejected: peer refused assignment",
	); err != nil {
		t.Fatalf("exact rejected launch error=%v", err)
	}
}

func TestRemoteControlEndpointApplyRecoversOnlyBundlePrefix(t *testing.T) {
	fixture := newRemoteControlFixture(t)
	endpoint, err := PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T04:02:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, fixture.job.CaseRoot, endpoint.BundlePath, endpoint.bundleData)
	if _, err := ApplyTransportCurrent(endpoint, endpoint.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(fixture.job.CaseRoot, filepath.FromSlash(endpoint.ArtifactPath))); err != nil {
		t.Fatal(err)
	}

	fixture = newRemoteControlFixture(t)
	endpoint, err = PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T04:02:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, fixture.job.CaseRoot, endpoint.ArtifactPath, endpoint.data)
	if _, err := ApplyTransportCurrent(endpoint, endpoint.ExpectedPlanSHA256, nil); err == nil || !strings.Contains(err.Error(), "without its evidence bundle") {
		t.Fatalf("endpoint-without-bundle error=%v", err)
	}
}

func TestRemoteControlTransportBundleIsDeterministicAndBounded(t *testing.T) {
	fixture := newRemoteControlFixture(t)
	first, err := PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T04:02:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T04:02:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.BundleSHA256 != second.BundleSHA256 || !bytes.Equal(first.bundleData, second.bundleData) ||
		first.ArtifactSHA256 != second.ArtifactSHA256 || !bytes.Equal(first.data, second.data) ||
		first.Endpoint == nil || second.Endpoint == nil ||
		first.Endpoint.Envelope.MessageSHA256 != second.Endpoint.Envelope.MessageSHA256 ||
		first.Endpoint.Envelope.Message != second.Endpoint.Envelope.Message {
		t.Fatal("identical Remote Control evidence did not produce deterministic bundle, endpoint, and message bytes")
	}
	if first.BundleBytes > maxTransportBundleBytes || first.Endpoint.Envelope.MessageBytes > maxTransportMessageBytes {
		t.Fatalf("transport size bounds were exceeded: bundle=%d message=%d", first.BundleBytes, first.Endpoint.Envelope.MessageBytes)
	}

	fixture = newRemoteControlFixtureWithOutput(t, bytes.Repeat([]byte("x"), memberexecution.MaxReviewerEvidenceArtifactBytes+1))
	if _, err := PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T04:02:00Z",
	); err == nil || !strings.Contains(err.Error(), "bounded") {
		t.Fatalf("oversized transport artifact error=%v", err)
	}

	fixture = newRemoteControlFixtureWithPayloads(
		t,
		bytes.Repeat([]byte("o"), 24*1024),
		bytes.Repeat([]byte("p"), 24*1024),
		1,
	)
	if _, err := PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T04:02:00Z",
	); err == nil || !strings.Contains(err.Error(), "raw bytes") {
		t.Fatalf("oversized transport raw bundle error=%v", err)
	}

	fixture = newRemoteControlFixtureWithPayloads(
		t,
		bytes.Repeat([]byte("\\"), 18*1024),
		bytes.Repeat([]byte("\\"), 18*1024),
		1,
	)
	if _, err := PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T04:02:00Z",
	); err == nil || !strings.Contains(err.Error(), "canonical evidence bundle") {
		t.Fatalf("oversized canonical transport bundle error=%v", err)
	}
}

func TestRemoteControlTransportRejectsDuplicateAndExcessItems(t *testing.T) {
	for _, tt := range []struct {
		name  string
		items func(string) []string
		want  string
	}{
		{name: "duplicate", items: func(item string) []string { return []string{item, item} }, want: "duplicated"},
		{name: "case-fold duplicate", items: func(item string) []string { return []string{item, strings.ToUpper(item)} }, want: "duplicated"},
		{name: "too many", items: func(item string) []string {
			items := make([]string, maxTransportBundleItems+1)
			for index := range items {
				items[index] = item
			}
			return items
		}, want: "1..16"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newRemoteControlFixtureWithItems(
				t,
				[]byte("bounded reviewer evidence\n"),
				[]byte("Review the exact bounded evidence bundle for this shard.\n"),
				tt.items,
			)
			if _, err := PreviewTransportEndpoint(
				fixture.job,
				fixture.attempt,
				fixture.dispatch,
				"reviewer [opaque-ref]",
				remoteControlTestActor,
				"2026-08-12T04:02:00Z",
			); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("reviewer items error=%v want %q", err, tt.want)
			}
		})
	}
}

func TestRemoteControlTransportPropagatesStrictManifestClosureChecks(t *testing.T) {
	for _, tt := range []struct {
		name    string
		mutate  func(t *testing.T, fixture remoteControlFixture)
		wantAny []string
	}{
		{
			name: "drifted output",
			mutate: func(t *testing.T, fixture remoteControlFixture) {
				writeTestFile(t, fixture.job.CaseRoot, remoteControlOutputRel(fixture), []byte("drifted\n"))
			},
			wantAny: []string{"drift", "sha256", "size"},
		},
		{
			name: "missing output",
			mutate: func(t *testing.T, fixture remoteControlFixture) {
				if err := os.Remove(filepath.Join(fixture.job.CaseRoot, filepath.FromSlash(remoteControlOutputRel(fixture)))); err != nil {
					t.Fatal(err)
				}
			},
			wantAny: []string{"not exist", "cannot find", "no such file"},
		},
		{
			name: "output symlink",
			mutate: func(t *testing.T, fixture remoteControlFixture) {
				output := filepath.Join(fixture.job.CaseRoot, filepath.FromSlash(remoteControlOutputRel(fixture)))
				if err := os.Remove(output); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(t.TempDir(), "outside.txt")
				if err := os.WriteFile(target, []byte("bounded reviewer evidence\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, output); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			},
			wantAny: []string{"regular", "symlink", "reparse", "invalid entry"},
		},
		{
			name: "undeclared extra output",
			mutate: func(t *testing.T, fixture remoteControlFixture) {
				manifestFull := filepath.Join(fixture.job.CaseRoot, filepath.FromSlash(fixture.item))
				writeTestFile(t, fixture.job.CaseRoot, filepath.ToSlash(filepath.Join(filepath.Dir(fixture.item), "outputs", "extra.txt")), []byte("extra\n"))
				if _, err := os.Lstat(manifestFull); err != nil {
					t.Fatal(err)
				}
			},
			wantAny: []string{"exactly match manifest", "not declared"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newRemoteControlFixture(t)
			tt.mutate(t, fixture)
			if _, err := PreviewTransportEndpoint(
				fixture.job,
				fixture.attempt,
				fixture.dispatch,
				"reviewer [opaque-ref]",
				remoteControlTestActor,
				"2026-08-12T04:02:00Z",
			); err == nil || !transportTestContainsAny(err.Error(), tt.wantAny) {
				t.Fatalf("strict closure error=%v want one of %v", err, tt.wantAny)
			}
		})
	}
}

func TestRemoteControlTransportRejectsTamperAndStaleApply(t *testing.T) {
	fixture := newRemoteControlFixture(t)
	endpoint, err := PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T04:02:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	staleJob := fixture.job
	staleJob.CheckpointSHA256 = strings.Repeat("9", 64)
	if _, err := ApplyTransportCurrent(endpoint, endpoint.ExpectedPlanSHA256, func() (Job, error) {
		return staleJob, nil
	}); err == nil || !strings.Contains(err.Error(), "job is no longer current") {
		t.Fatalf("stale job error=%v", err)
	}
	staleAttempt := endpoint
	staleAttempt.AttemptSHA256 = strings.Repeat("8", 64)
	if _, err := ApplyTransportCurrent(staleAttempt, staleAttempt.ExpectedPlanSHA256, nil); err == nil || !strings.Contains(err.Error(), "attempt is no longer current") {
		t.Fatalf("stale attempt error=%v", err)
	}
	staleClaim := endpoint
	staleClaim.ClaimSHA256 = strings.Repeat("7", 64)
	if _, err := ApplyTransportCurrent(staleClaim, staleClaim.ExpectedPlanSHA256, nil); err == nil || !strings.Contains(err.Error(), "claim is no longer current") {
		t.Fatalf("stale claim error=%v", err)
	}

	if _, err := ApplyTransportCurrent(endpoint, endpoint.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, fixture.job.CaseRoot, endpoint.BundlePath, []byte("tampered\n"))
	if _, err := InspectTransport(fixture.job, fixture.attempt, fixture.dispatch); err == nil || !strings.Contains(err.Error(), "evidence bundle") {
		t.Fatalf("bundle tamper error=%v", err)
	}

	fixture = newRemoteControlFixture(t)
	endpoint, err = PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T04:02:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransportCurrent(endpoint, endpoint.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	endpoint.Endpoint.Envelope.MessageSHA256 = strings.Repeat("6", 64)
	writeTestJSON(t, fixture.job.CaseRoot, endpoint.ArtifactPath, *endpoint.Endpoint)
	if _, err := InspectTransport(fixture.job, fixture.attempt, fixture.dispatch); err == nil || !strings.Contains(err.Error(), "message envelope") {
		t.Fatalf("endpoint envelope tamper error=%v", err)
	}
}

func TestRemoteControlTransportProviderAcknowledgementDoesNotChangeCapability(t *testing.T) {
	fixture := newRemoteControlFixture(t)
	applyRemoteControlEndpoint(t, fixture)
	delivery, err := PreviewTransportDelivery(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"accepted",
		strings.Repeat("e", 64),
		remoteControlTestActor,
		"2026-08-12T03:03:00Z",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if delivery.Delivery == nil || delivery.Delivery.ProviderAckFingerprint == "" ||
		delivery.Delivery.Capability != fixture.job.Capability {
		t.Fatalf("provider acknowledgement changed capability lineage: %+v", delivery.Delivery)
	}
}

func TestRemoteControlTransportRejectsCapabilityHashAndPolicyDrift(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*TransportDeliveryObservation)
	}{
		{name: "hash", mutate: func(delivery *TransportDeliveryObservation) { delivery.Capability.SHA256 = strings.Repeat("7", 64) }},
		{name: "policy", mutate: func(delivery *TransportDeliveryObservation) {
			delivery.Capability = reviewersession.ReadOnlyCapability()
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newRemoteControlFixture(t)
			applyRemoteControlEndpoint(t, fixture)
			delivery, err := PreviewTransportDelivery(fixture.job, fixture.attempt, fixture.dispatch, "accepted", strings.Repeat("e", 64), remoteControlTestActor, "2026-08-12T03:03:00Z", "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ApplyTransportCurrent(delivery, delivery.ExpectedPlanSHA256, nil); err != nil {
				t.Fatal(err)
			}
			test.mutate(delivery.Delivery)
			writeTestJSON(t, fixture.job.CaseRoot, delivery.ArtifactPath, *delivery.Delivery)
			if _, err := InspectTransport(fixture.job, fixture.attempt, fixture.dispatch); err == nil || !strings.Contains(err.Error(), "capability") {
				t.Fatalf("delivery capability drift error=%v", err)
			}
		})
	}
}

func TestRemoteControlTransportRejectsDeliveryBindingTamper(t *testing.T) {
	fixture := newRemoteControlFixture(t)
	applyRemoteControlEndpoint(t, fixture)
	delivery, err := PreviewTransportDelivery(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"accepted",
		strings.Repeat("e", 64),
		remoteControlTestActor,
		"2026-08-12T04:03:00Z",
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransportCurrent(delivery, delivery.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	delivery.Delivery.EndpointSnapshotSHA256 = strings.Repeat("5", 64)
	writeTestJSON(t, fixture.job.CaseRoot, delivery.ArtifactPath, *delivery.Delivery)
	if _, err := InspectTransport(fixture.job, fixture.attempt, fixture.dispatch); err == nil || !strings.Contains(err.Error(), "delivery contract") {
		t.Fatalf("delivery binding tamper error=%v", err)
	}
}

func remoteControlOutputRel(fixture remoteControlFixture) string {
	return filepath.ToSlash(filepath.Join(filepath.Dir(fixture.item), "outputs", "review-items.json"))
}

func transportTestContainsAny(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func newRemoteControlFixture(t *testing.T) remoteControlFixture {
	return newRemoteControlFixtureWithOutput(t, []byte("bounded reviewer evidence\n"))
}

func newRemoteControlFixtureWithOutput(t *testing.T, output []byte) remoteControlFixture {
	return newRemoteControlFixtureWithPayloads(t, output, []byte("Review the exact bounded evidence bundle for this shard.\n"), 1)
}

func newRemoteControlFixtureWithPayloads(t *testing.T, output, prompt []byte, itemCount int) remoteControlFixture {
	return newRemoteControlFixtureWithItems(t, output, prompt, func(item string) []string {
		items := make([]string, itemCount)
		for index := range items {
			items[index] = item
		}
		return items
	})
}

func newRemoteControlFixtureWithItems(t *testing.T, output, prompt []byte, itemFactory func(string) []string) remoteControlFixture {
	t.Helper()
	caseRoot, memberPlan, item := remoteControlEvidenceFixture(t, output)
	items := itemFactory(item)
	itemsData, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	packetRel, err := projectstate.Rel(caseRoot, "reviews", "packet-a", "packet.json")
	if err != nil {
		t.Fatal(err)
	}
	packet := []byte(`{"packetId":"packet-a","route":{"id":"route-a","outputContract":"item,decision,candidate_path"},"shards":[{"id":"shard-a","items":` + string(itemsData) + `}],"outputContract":"item,decision,candidate_path"}`)
	writeTestFile(t, caseRoot, packetRel, packet)
	promptRel := filepath.ToSlash(filepath.Join(filepath.Dir(packetRel), "prompt.md"))
	writeTestFile(t, caseRoot, promptRel, prompt)
	dispatchRel := filepath.ToSlash(filepath.Join(filepath.Dir(packetRel), "sessions", "shard-a", "dispatches", "dispatch-remote-a.json"))
	owner := reviewersession.Owner{CurrentExecutor: memberPlan.Owner.Executor, ExecutorGeneration: memberPlan.Owner.ExecutorGeneration, BindingMode: "durable-lane-owner"}
	receipt := reviewersession.DispatchReceipt{
		SchemaVersion: 1, Kind: "reviewer-session-dispatch", DispatchID: "dispatch-remote-a",
		PacketID: "packet-a", PacketPath: packetRel, PacketSHA256: hash(packet), RouteID: "route-a",
		ShardID: "shard-a", Items: append([]string{}, items...), PromptPath: promptRel, PromptSHA256: hash(prompt),
		AgentType: "read-only-reviewer", ReadOnly: true, TargetLane: memberPlan.Owner.Lane,
		PacketOwner: owner, EffectiveOwner: owner, ReviewerHarness: RemoteControlHarness,
		ReviewerSession: remoteControlTestSession, Actor: remoteControlTestActor,
		RecordedAt: "2026-08-12T01:00:00Z", Capability: reviewersession.ReadOnlyCapability(), NoSpawn: true, NoHeavyTool: true, NoAuthority: true,
	}
	dispatchData, err := canonical(receipt)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, caseRoot, dispatchRel, dispatchData)
	reviewer := ReviewerIdentity{
		AttemptSHA256: strings.Repeat("b", 64), PacketID: receipt.PacketID, RouteID: receipt.RouteID,
		ShardID: receipt.ShardID, Items: append([]string{}, receipt.Items...),
		OutputFields: []string{"item", "decision", "candidate_path"}, DispatchPath: dispatchRel,
		DispatchSHA256: hash(dispatchData), DispatchID: receipt.DispatchID,
		Harness: receipt.ReviewerHarness, Session: receipt.ReviewerSession,
	}
	job, err := NewReviewerJob(caseRoot, defaults.DefaultPack, testCheckpointSHA, reviewer, []string{"returned"})
	if err != nil {
		t.Fatal(err)
	}
	job.DispatchRequired = true
	plan, err := PreviewAttempt(job, RemoteControlHarness, remoteControlTestSession, remoteControlTestActor, "2026-08-12T01:00:30Z", "")
	if err != nil {
		t.Fatal(err)
	}
	plan, err = BindAttemptDispatch(plan, remoteControlDispatchTicket(plan, promptRel, prompt))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyAttempt(plan, plan.JobSHA256, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	attempt, err := InspectAttempt(job)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := PreviewDispatchTransition(job, attempt, "claimed", remoteControlTestActor, "2026-08-12T01:01:00Z", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDispatchTransitionCurrent(claim, claim.JobSHA256, claim.DispatchSHA256, "", claim.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
	dispatch, err := InspectCurrentDispatch(job, attempt)
	if err != nil || dispatch.State != "claimed" {
		t.Fatalf("dispatch=%+v err=%v", dispatch, err)
	}
	return remoteControlFixture{job: job, attempt: attempt, dispatch: dispatch, item: item}
}

func remoteControlDispatchTicket(plan AttemptPlan, promptRel string, prompt []byte) DispatchTicket {
	templates := make([]DispatchSubmissionTemplate, 0, len(plan.Attempt.AllowedOutcomes))
	for _, outcome := range plan.Attempt.AllowedOutcomes {
		templates = append(templates, DispatchSubmissionTemplate{
			Outcome: outcome, JSON: `{"outcome":"` + outcome + `"}`,
			RequiredWrites: []string{plan.Attempt.SubmissionResult, plan.Attempt.SubmissionPath + " (last)"},
		})
	}
	return DispatchTicket{
		Launch: DispatchLaunch{
			Ready: true, Tool: "Claude Code Remote Control", AgentType: "read-only-reviewer", ReadOnly: true,
			Capability:     reviewersession.ReadOnlyCapability(),
			Input:          DispatchInput{Path: promptRel, SHA256: hash(prompt), Role: "reviewer-dispatch-prompt"},
			ExpectedOutput: "exactly one ReviewerResult JSON object", Boundary: []string{"read-only Reviewer"},
		},
		Return: DispatchReturn{
			SubmissionPath: plan.Attempt.SubmissionPath, SubmissionResult: plan.Attempt.SubmissionResult,
			SubmissionLast: true, Templates: templates, Boundary: []string{"result first", "submission last"},
		},
		RefreshStatusCommand: "/rekit status", Boundary: []string{"no heavy tool"},
	}
}

func applyRemoteControlEndpoint(t *testing.T, fixture remoteControlFixture) {
	t.Helper()
	endpoint, err := PreviewTransportEndpoint(
		fixture.job,
		fixture.attempt,
		fixture.dispatch,
		"reviewer [opaque-ref]",
		remoteControlTestActor,
		"2026-08-12T03:02:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyTransportCurrent(endpoint, endpoint.ExpectedPlanSHA256, nil); err != nil {
		t.Fatal(err)
	}
}

func remoteControlEvidenceFixture(t *testing.T, output []byte) (string, memberexecution.Plan, string) {
	t.Helper()
	caseRoot := remoteControlMemberCase(t, "executor-a", 1)
	plan, err := memberexecution.PreviewDispatch(memberexecution.DispatchOptions{
		CaseRoot: caseRoot, Pack: defaults.DefaultPack, Lane: "feature-analysis",
		RequestSHA256: strings.Repeat("a", 64), CreatedAt: "2026-08-12T00:01:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(plan, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	accepted, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{
		CaseRoot: caseRoot, Pack: defaults.DefaultPack, Lane: "feature-analysis", AttemptID: plan.AttemptID,
		Outcome: "accepted", Actor: "harness", ObservedAt: "2026-08-12T00:02:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := memberexecution.Apply(accepted, accepted.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(plan.Inspection.OutputsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(plan.Inspection.OutputsRoot, "review-items.json")
	if err := os.WriteFile(outputPath, output, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := memberexecution.ResultManifest{
		SchemaVersion: memberexecution.SchemaVersion, Kind: memberexecution.KindManifest,
		AttemptID: plan.AttemptID, Owner: plan.Owner, Summary: "bounded reviewer evidence",
		Outputs:           []memberexecution.Output{{Path: "review-items.json", SHA256: hash(output), Bytes: int64(len(output))}},
		ReviewerItemsPath: "review-items.json", NoAuthority: true, NoConfirmed: true, NoHeavyTool: true,
	}
	manifestData, err := memberexecution.MarshalResultManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.Inspection.ManifestPath, manifestData, 0o600); err != nil {
		t.Fatal(err)
	}
	returned, err := memberexecution.PreviewObservation(memberexecution.ObservationOptions{
		CaseRoot: caseRoot, Pack: defaults.DefaultPack, Lane: "feature-analysis", AttemptID: plan.AttemptID,
		Outcome: "returned", Actor: "harness", ObservedAt: "2026-08-12T00:03:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := memberexecution.Apply(returned, returned.ExpectedPlanSHA256)
	if err != nil || applied.Inspection.State != "intake-ready" {
		t.Fatalf("returned=%+v err=%v", applied, err)
	}
	item, err := filepath.Rel(caseRoot, applied.Inspection.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	return caseRoot, plan, filepath.ToSlash(item)
}

func remoteControlMemberCase(t *testing.T, executor string, generation int) string {
	t.Helper()
	caseRoot := t.TempDir()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve kit root")
	}
	kitRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	for rel, data := range map[string][]byte{
		".rekit/instance.yml":                                   []byte("schemaVersion: 1\ntemplateRoot: " + kitRoot + "\ntemplatePack: " + defaults.DefaultPack + "\nprojectRoot: " + caseRoot + "\n"),
		".rekit/lanes/feature-analysis/lane.json":               []byte("{\"id\":\"feature-analysis\",\"status\":\"active\"}\n"),
		".rekit/lanes/feature-analysis/prompts/RESUME.md":       []byte("# feature-analysis\n\nContinue the durable lane task.\n"),
		".rekit/lanes/feature-analysis/checkpoints/latest.json": []byte("{\n  \"schemaVersion\": 1,\n  \"lane\": \"feature-analysis\",\n  \"status\": \"active\"\n}\n"),
	} {
		writeTestFile(t, caseRoot, rel, data)
	}
	board := map[string]any{
		"schemaVersion": 1, "caseRoot": caseRoot, "repoRoot": filepath.Dir(caseRoot), "pack": defaults.DefaultPack,
		"automationMode": "review-first", "defaultAuthorityLane": "main", "factsRoot": ".rekit/facts",
		"updatedAt": "2026-08-12T00:00:00Z",
		"lanes": []map[string]any{{
			"id": "feature-analysis", "type": "feature", "title": "analysis", "status": "active",
			"authority": false, "workspace": ".rekit/lanes/feature-analysis/workspace",
			"currentExecutor": executor, "executorGeneration": generation, "updatedAt": "2026-08-12T00:00:00Z",
		}},
	}
	data, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, caseRoot, ".rekit/board.json", append(data, '\n'))
	return caseRoot
}
