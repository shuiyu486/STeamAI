package sessionhost

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/laneowner"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

type webSecurityLifecycleFixture struct {
	repoRoot  string
	caseRoot  string
	pack      string
	lane      string
	adapterID string
	inputPath string
	gateID    string
}

func newWebSecurityLifecycleFixture(t *testing.T, adapterID string) webSecurityLifecycleFixture {
	t.Helper()
	repoRoot := sessionhostTestRepoRoot(t)
	caseRoot := provisionSessionhostAttachedCase(t, repoRoot, webSecurityPack)
	bootstrap := DailyResult{CaseRoot: caseRoot}
	intent, err := applyDailyOnboarding(caseRoot, "review bounded web-security evidence", "web-security-lifecycle-test", &bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.Lane = intent.Identity.InitialLane
	if err := ensureDailyStarted(caseRoot, intent.Identity.Pack, &bootstrap); err != nil {
		t.Fatal(err)
	}
	repoRoot, err = runtimeContextForDailyPack(caseRoot, intent.Identity.Pack)
	if err != nil {
		t.Fatal(err)
	}
	board, err := mission.ReadBoard(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	selected, found := mission.LookupBoardLane(board.Lanes, bootstrap.Lane, false)
	if !found || strings.TrimSpace(selected.Workspace) == "" {
		t.Fatalf("web-security fixture lane workspace is unavailable: %+v", selected)
	}

	action := "inspect"
	inputPath := "inputs/openapi.json"
	outputRoot := filepath.ToSlash(filepath.Join(selected.Workspace, "openapi", "session-1"))
	stopConditions := []string{"scope-drift", "source-drift", "output-exceeds-bounded-evidence-packet"}
	switch adapterID {
	case websecurity.InventoryAdapterID:
		writeWebSecurityLifecycleFile(t, caseRoot, inputPath, []byte(webSecurityLifecycleOpenAPI(18080)))
	case websecurity.ReplayAdapterID:
		action = "network"
		outputRoot = filepath.ToSlash(filepath.Join(selected.Workspace, "replay", "session-1"))
		stopConditions = []string{"live-target-ambiguity", "unexpected-outbound-request", "scope-drift", "delivery-uncertain", "response-body-limit", "response-read"}
		port := closedWebSecurityLifecyclePort(t)
		sourcePath := "inputs/replay-openapi.json"
		sourceData := []byte(webSecurityLifecycleOpenAPI(port))
		source, bindErr := websecurity.BindFile(sourcePath, sourceData, websecurity.MaxOpenAPIBytes)
		if bindErr != nil {
			t.Fatal(bindErr)
		}
		inventory, importErr := websecurity.ImportOpenAPI(source, sourceData)
		if importErr != nil {
			t.Fatal(importErr)
		}
		inventoryData, encodeErr := websecurity.CanonicalInventoryBytes(inventory)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		inventoryPath := "inputs/openapi-inventory.json"
		inventoryBinding, bindErr := websecurity.BindFile(inventoryPath, inventoryData, websecurity.MaxInventoryBytes)
		if bindErr != nil {
			t.Fatal(bindErr)
		}
		request, requestErr := websecurity.NewReplayRequest(
			inventory,
			inventoryBinding,
			"get /health",
			websecurity.ReplayTarget{Scheme: "http", Host: "127.0.0.1", Port: port, BasePath: "/api"},
			"/health",
			nil,
			websecurity.ExpectedResponse{
				StatusCode: 200,
				Body:       websecurity.DigestExpectation{SHA256: websecurity.SHA256(nil), Bytes: 0},
				Headers:    []websecurity.HeaderExpectation{},
			},
			websecurity.DefaultReplayLimits(),
		)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		requestData, encodeErr := websecurity.CanonicalReplayRequestBytes(request)
		if encodeErr != nil {
			t.Fatal(encodeErr)
		}
		requestBinding, bindErr := websecurity.BindReplayRequest(requestData)
		if bindErr != nil {
			t.Fatal(bindErr)
		}
		inputPath = requestBinding.Path
		writeWebSecurityLifecycleFile(t, caseRoot, sourcePath, sourceData)
		writeWebSecurityLifecycleFile(t, caseRoot, inventoryPath, inventoryData)
		writeWebSecurityLifecycleFile(t, caseRoot, inputPath, requestData)
	default:
		t.Fatalf("unsupported web-security lifecycle fixture adapter: %s", adapterID)
	}

	now := time.Now().UTC()
	plan, err := autonomy.PreviewProvision(autonomy.ProfileProvisionOptions{
		RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: intent.Identity.Pack, Lane: bootstrap.Lane,
		Profile: autonomy.Profile{
			SchemaVersion: 1, ProfileID: "web-security-lifecycle", Lane: bootstrap.Lane, Mode: autonomy.ModePreauthorized,
			AllowedActions: []string{action}, TargetScope: []autonomy.Target{{Match: "exact", Value: inputPath}},
			Budget:         autonomy.Budget{RuntimeSeconds: 10, DiskMB: 4, Requests: 1},
			StopConditions: stopConditions, OutputPaths: []string{outputRoot}, RecordRequired: true,
			NotifyMainOn: []string{"boundary-hit", "new-risk"}, GrantedBy: "web-security-lifecycle-test",
			GrantedAt: now.Add(-time.Minute).Format(time.RFC3339), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := autonomy.ApplyProfilePlan(plan, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	authorized, err := gate.Apply(repoRoot, caseRoot, intent.Identity.Pack, gate.Options{
		Action: action, Lane: bootstrap.Lane, Actor: "web-security-lifecycle-test",
		Subject: "bounded web-security production fixture", Summary: "typed compiled-in adapter evidence",
		TargetRef: inputPath, RuntimeSeconds: 10, DiskMB: 4, Requests: 1,
		OutputPaths: outputRoot, StopConditions: strings.Join(stopConditions, ","),
	})
	if err != nil || !authorized.Applied || authorized.Event == nil || authorized.Event.Gate.Authorization.Decision != autonomy.DecisionPreauthorized {
		t.Fatalf("authorize web-security lifecycle fixture: %+v err=%v", authorized, err)
	}
	return webSecurityLifecycleFixture{
		repoRoot: repoRoot, caseRoot: caseRoot, pack: intent.Identity.Pack, lane: bootstrap.Lane,
		adapterID: adapterID, inputPath: inputPath, gateID: authorized.EventID,
	}
}

func TestWebSecurityOpenAPIAdapterLifecycleAcceptsBindsAndReplaysWithoutRelaunch(t *testing.T) {
	fixture := newWebSecurityLifecycleFixture(t, websecurity.InventoryAdapterID)
	reviewerCalls := installWebSecurityLifecycleReviewer(t, fixture, "accepted")
	adapterCalls := 0
	opt := webSecurityLifecycleDailyOptions(t, fixture, &adapterCalls)

	first, found, err := runWebSecurityAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !first.ReadyForMember || first.State != "ready-for-member" ||
		first.AdapterID != websecurity.InventoryAdapterID || first.ExecutionStatus != "succeeded" ||
		first.ExecutionExitStatus != "completed" || first.EvidenceReviewDecision != "accepted" ||
		first.SelectedEvidenceRef != first.Run.PacketPath || !first.ChildLaunched ||
		first.AdapterReplay || first.EvidenceReviewReplay || adapterCalls != 1 || *reviewerCalls != 1 {
		t.Fatalf("fresh web-security OpenAPI lifecycle = %+v adapterCalls=%d reviewerCalls=%d", first, adapterCalls, *reviewerCalls)
	}
	assertWebSecurityLifecycleBinding(t, fixture, first, webSecurityOpenAPIMemberBindingKind)
	assertWebSecurityReviewLaunchOwner(t, fixture, first)
	assertWebSecurityLifecycleNoAuthority(t, fixture.caseRoot)

	before, err := liveAcceptanceTreeSHA256(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, found, err := runWebSecurityAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	after, err := liveAcceptanceTreeSHA256(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if found || second.Kind != "" || before != after || adapterCalls != 1 || *reviewerCalls != 1 {
		t.Fatalf("completed OpenAPI lifecycle replay = %+v found=%t before=%s after=%s adapterCalls=%d reviewerCalls=%d", second, found, before, after, adapterCalls, *reviewerCalls)
	}
}

func TestWebSecurityEvidenceReviewLaunchRejectsSelfConsistentObservationDrift(t *testing.T) {
	fixture := newWebSecurityLifecycleFixture(t, websecurity.InventoryAdapterID)
	installWebSecurityLifecycleReviewer(t, fixture, "accepted")
	adapterCalls := 0
	lifecycle, found, err := runWebSecurityAdapterLifecycle(
		context.Background(),
		webSecurityLifecycleDailyOptions(t, fixture, &adapterCalls),
		fixture.caseRoot,
		fixture.pack,
		fixture.lane,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !lifecycle.ReadyForMember {
		t.Fatalf("web-security lifecycle did not reach member binding: %+v", lifecycle)
	}
	input, _, present, err := readBinaryREAdapterArtifact[webSecurityOpenAPIReviewInput](
		fixture.caseRoot,
		lifecycle.EvidenceReviewInputPath,
		"web-security adversarial review input",
	)
	if err != nil || !present {
		t.Fatalf("read web-security review input: present=%t err=%v", present, err)
	}
	observation, _, present, err := readBinaryREAdapterArtifact[map[string]any](
		fixture.caseRoot,
		input.ObservationPath,
		"web-security adversarial observation snapshot",
	)
	if err != nil || !present {
		t.Fatalf("read web-security observation snapshot: present=%t err=%v", present, err)
	}
	observation["eventId"] = "evt-self-consistent-drift"
	driftedObservationPath, err := binaryREAdapterArtifactPath(
		fixture.caseRoot,
		fixture.lane,
		fixture.gateID,
		"evidence-review",
		"drifted-observation.json",
	)
	if err != nil {
		t.Fatal(err)
	}
	driftedObservationSHA, err := writeBinaryREAdapterArtifact(
		fixture.caseRoot,
		driftedObservationPath,
		"web-security adversarial observation snapshot",
		observation,
	)
	if err != nil {
		t.Fatal(err)
	}
	for index, ref := range input.EvidenceRefs {
		if ref == input.ObservationPath {
			input.EvidenceRefs[index] = driftedObservationPath
		}
	}
	input.ObservationPath = driftedObservationPath
	input.ObservationSHA256 = driftedObservationSHA
	driftedInputPath, err := binaryREAdapterArtifactPath(
		fixture.caseRoot,
		fixture.lane,
		fixture.gateID,
		"evidence-review",
		"drifted-input.json",
	)
	if err != nil {
		t.Fatal(err)
	}
	driftedInputSHA, err := writeBinaryREAdapterArtifact(
		fixture.caseRoot,
		driftedInputPath,
		"web-security adversarial review input",
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	launch := mission.CurrentLoopExternalSessionHarnessLaunch{
		Input: mission.CurrentLoopExternalSessionHarnessInput{
			Path: driftedInputPath, SHA256: driftedInputSHA, Role: webSecurityOpenAPIReviewInputRole,
		},
	}
	if _, err := missionCommanderEvidenceReviewLaunchOwner(fixture.caseRoot, launch); err == nil ||
		!strings.Contains(err.Error(), "observation lineage changed before launch") {
		t.Fatalf("self-consistent observation drift was not rejected before launch: %v", err)
	}
}

func TestWebSecurityOpenAPIAdapterLifecycleRejectedStopsBeforeAcknowledgementAndBinding(t *testing.T) {
	fixture := newWebSecurityLifecycleFixture(t, websecurity.InventoryAdapterID)
	reviewerCalls := installWebSecurityLifecycleReviewer(t, fixture, "rejected")
	adapterCalls := 0
	opt := webSecurityLifecycleDailyOptions(t, fixture, &adapterCalls)

	first, found, err := runWebSecurityAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	if !found || first.State != "evidence-review-rejected" || first.ReadyForMember ||
		first.EvidenceReviewDecision != "rejected" || first.AcknowledgementEventID != "" ||
		first.ClosurePath != "" || first.TaskBindingPath != "" || adapterCalls != 1 || *reviewerCalls != 1 {
		t.Fatalf("rejected OpenAPI lifecycle = %+v adapterCalls=%d reviewerCalls=%d", first, adapterCalls, *reviewerCalls)
	}
	owner, err := laneowner.Read(fixture.caseRoot, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	binding, _, _, err := memberexecution.ReadTaskBindingForOwner(fixture.caseRoot, fixture.lane, owner.ExecutorGeneration)
	if err != nil {
		t.Fatal(err)
	}
	if binding != nil {
		t.Fatalf("rejected OpenAPI review published member binding: %+v", binding)
	}
	facts, err := mission.ReadStrictLedgerFacts(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if workstream.ExecutionEvidenceReviewAcknowledgedIDs(facts)[fixture.gateID] {
		t.Fatal("rejected OpenAPI review wrote acknowledgement")
	}

	second, found, err := runWebSecurityAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	if !found || second.State != "evidence-review-rejected" || !second.AdapterReplay || !second.EvidenceReviewReplay ||
		adapterCalls != 1 || *reviewerCalls != 1 {
		t.Fatalf("rejected OpenAPI lifecycle replay = %+v adapterCalls=%d reviewerCalls=%d", second, adapterCalls, *reviewerCalls)
	}
}

func TestWebSecurityBoundedReplayUncertainEvidenceIsReviewedAndNeverRetried(t *testing.T) {
	fixture := newWebSecurityLifecycleFixture(t, websecurity.ReplayAdapterID)
	reviewerCalls := installWebSecurityLifecycleReviewer(t, fixture, "accepted")
	adapterCalls := 0
	opt := webSecurityLifecycleDailyOptions(t, fixture, &adapterCalls)

	first, found, err := runWebSecurityAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	if !found || !first.ReadyForMember || first.State != "ready-for-member" ||
		first.AdapterID != websecurity.ReplayAdapterID || first.ExecutionStatus != "aborted" ||
		first.ExecutionExitStatus != "delivery-uncertain" || first.EvidenceReviewDecision != "accepted" ||
		first.Run == nil || first.Run.NoNetwork || first.Run.PacketPath == "" ||
		!first.ChildLaunched || first.AdapterReplay || first.EvidenceReviewReplay || adapterCalls != 1 || *reviewerCalls != 1 {
		t.Fatalf("fresh bounded replay lifecycle = %+v adapterCalls=%d reviewerCalls=%d", first, adapterCalls, *reviewerCalls)
	}
	resultData, err := os.ReadFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(first.Run.PacketPath)))
	if err != nil {
		t.Fatal(err)
	}
	replayResult, err := websecurity.DecodeReplayResult(resultData)
	if err != nil || replayResult.Status != "delivery-uncertain" || replayResult.Delivery.Attempts != 1 ||
		replayResult.Delivery.Certain || replayResult.Actual != nil || replayResult.Diff != nil {
		t.Fatalf("reviewed bounded replay result = %+v err=%v", replayResult, err)
	}
	assertWebSecurityLifecycleBinding(t, fixture, first, webSecurityReplayMemberBindingKind)
	assertWebSecurityReviewLaunchOwner(t, fixture, first)

	before, err := liveAcceptanceTreeSHA256(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, found, err := runWebSecurityAdapterLifecycle(context.Background(), opt, fixture.caseRoot, fixture.pack, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	after, err := liveAcceptanceTreeSHA256(fixture.caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if found || second.Kind != "" || before != after || adapterCalls != 1 || *reviewerCalls != 1 {
		t.Fatalf("terminal bounded replay replayed work = %+v found=%t before=%s after=%s adapterCalls=%d reviewerCalls=%d", second, found, before, after, adapterCalls, *reviewerCalls)
	}
}

func webSecurityLifecycleDailyOptions(t *testing.T, fixture webSecurityLifecycleFixture, adapterCalls *int) DailyOptions {
	t.Helper()
	adapterPath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return DailyOptions{
		Target: fixture.caseRoot, SelectedLane: fixture.lane,
		ExpectedClaudeExecutableSHA256:    strings.Repeat("a", 64),
		ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher,
		webSecurityAdapterPath:            adapterPath,
		evidenceReviewRunner:              webSecurityEvidenceReviewRunClaude,
		webSecurityAdapterRunner: func(path string, opt adapterhost.AuthorizedRunOptions, timeout time.Duration) (adapterhost.AuthorizedRunResult, int, error) {
			*adapterCalls++
			if opt.AdapterID != fixture.adapterID || opt.GateEventID != fixture.gateID || opt.Pack != webSecurityPack || !opt.DeferSuccessfulTaskBinding {
				return adapterhost.AuthorizedRunResult{}, 0, fmt.Errorf("web-security lifecycle runner received wrong immutable binding")
			}
			return adapterhost.RunAuthorizedGateProcess(path, opt, timeout)
		},
	}
}

func installWebSecurityLifecycleReviewer(t *testing.T, fixture webSecurityLifecycleFixture, decision string) *int {
	t.Helper()
	previousNow := binaryREAdapterNow
	previousReviewer := webSecurityEvidenceReviewRunClaude
	binaryREAdapterNow = func() string { return "2026-08-22T01:00:00Z" }
	calls := 0
	webSecurityEvidenceReviewRunClaude = func(_ context.Context, opt Options, pkg mission.CurrentLoopExternalSessionHarnessPackage, sessionID string, _ func() error) claudeRun {
		calls++
		if pkg.Launch == nil || opt.launchControlBinding == nil || pkg.SessionKind != "mission-commander-evidence-review" {
			return claudeRun{failureDetail: "web-security review fixture omitted session or launch control binding"}
		}
		response := evidenceReviewResponse{Decision: decision}
		switch fixture.adapterID {
		case websecurity.InventoryAdapterID:
			if pkg.Launch.Input.Role != webSecurityOpenAPIReviewInputRole {
				return claudeRun{failureDetail: "OpenAPI review fixture received the wrong typed role"}
			}
			input, _, found, err := readBinaryREAdapterArtifact[webSecurityOpenAPIReviewInput](fixture.caseRoot, pkg.Launch.Input.Path, "web-security OpenAPI lifecycle review input")
			if err != nil || !found {
				return claudeRun{failureDetail: "read OpenAPI review input: " + errorText(err)}
			}
			if err := validateWebSecurityReviewObservationFixture(fixture.caseRoot, input.ObservationPath, input.ObservationSHA256, input.ObservationEventID); err != nil {
				return claudeRun{failureDetail: err.Error()}
			}
			response.Summary = "exact secret-free OpenAPI inventory lineage"
			response.Reason = "source, inventory, report, dispatch, receipt, observation, and boundaries agree"
			response.EvidenceRefs = append([]string{}, input.EvidenceRefs...)
			response.SelectedEvidenceRef = input.SelectedEvidenceRef
			response.ObservationEventID = input.ObservationEventID
			response.ReceiptSHA256 = input.ReceiptSHA256
		case websecurity.ReplayAdapterID:
			if pkg.Launch.Input.Role != webSecurityReplayReviewInputRole {
				return claudeRun{failureDetail: "bounded replay review fixture received the wrong typed role"}
			}
			input, _, found, err := readBinaryREAdapterArtifact[webSecurityReplayReviewInput](fixture.caseRoot, pkg.Launch.Input.Path, "web-security bounded replay lifecycle review input")
			if err != nil || !found {
				return claudeRun{failureDetail: "read bounded replay review input: " + errorText(err)}
			}
			if err := validateWebSecurityReviewObservationFixture(fixture.caseRoot, input.ObservationPath, input.ObservationSHA256, input.ObservationEventID); err != nil {
				return claudeRun{failureDetail: err.Error()}
			}
			response.Summary = "exact redacted terminal bounded replay lineage"
			response.Reason = "one attempt, terminal delivery state, digests, report, receipt, and observation agree without retry"
			response.EvidenceRefs = append([]string{}, input.EvidenceRefs...)
			response.SelectedEvidenceRef = input.SelectedEvidenceRef
			response.ObservationEventID = input.ObservationEventID
			response.ReceiptSHA256 = input.ReceiptSHA256
		}
		structured, err := json.Marshal(response)
		if err != nil {
			return claudeRun{failureDetail: err.Error()}
		}
		return claudeRun{
			launchControlBinding: cloneClaudeLaunchControlBinding(opt.launchControlBinding),
			envelope:             claudeEnvelope{Type: "result", Subtype: "success", SessionID: sessionID},
			sessionID:            sessionID, structuredOutput: structured, started: true, exitCode: 0,
			observedAt: "2026-08-22T01:00:01Z",
		}
	}
	t.Cleanup(func() {
		binaryREAdapterNow = previousNow
		webSecurityEvidenceReviewRunClaude = previousReviewer
		if root, err := claudeRecoveryRootPath(fixture.caseRoot); err == nil {
			_ = os.RemoveAll(root)
		}
		if root, err := claudeRawResultRoot(fixture.caseRoot); err == nil {
			_ = os.RemoveAll(root)
		}
	})
	return &calls
}

func validateWebSecurityReviewObservationFixture(caseRoot, path, sha, eventID string) error {
	observation, data, found, err := readBinaryREAdapterArtifact[map[string]any](
		caseRoot,
		path,
		"web-security review observation fixture",
	)
	if err != nil || !found {
		return fmt.Errorf("read web-security review observation fixture: %w", err)
	}
	item, ok := mission.ExecutionEvidenceReviewItemFromObservation(observation, "", nil)
	if !ok || item.EventID != eventID || !strings.EqualFold(bytesSHA256(data), sha) {
		return fmt.Errorf("web-security review observation fixture drifted from its event/hash binding")
	}
	return nil
}

func assertWebSecurityReviewLaunchOwner(t *testing.T, fixture webSecurityLifecycleFixture, lifecycle WebSecurityAdapterLifecycleResult) {
	t.Helper()
	role := webSecurityOpenAPIReviewInputRole
	wrongRole := webSecurityReplayReviewInputRole
	if fixture.adapterID == websecurity.ReplayAdapterID {
		role, wrongRole = webSecurityReplayReviewInputRole, webSecurityOpenAPIReviewInputRole
	}
	launch := mission.CurrentLoopExternalSessionHarnessLaunch{
		Input: mission.CurrentLoopExternalSessionHarnessInput{
			Path: lifecycle.EvidenceReviewInputPath, SHA256: lifecycle.EvidenceReviewInputSHA256, Role: role,
		},
	}
	owner, err := missionCommanderEvidenceReviewLaunchOwner(fixture.caseRoot, launch)
	if err != nil {
		t.Fatal(err)
	}
	current, err := laneowner.Read(fixture.caseRoot, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	if owner != current {
		t.Fatalf("web-security evidence review launch owner = %+v, want %+v", owner, current)
	}
	launch.Input.Role = wrongRole
	if _, err := missionCommanderEvidenceReviewLaunchOwner(fixture.caseRoot, launch); err == nil {
		t.Fatal("web-security evidence review launch owner accepted a role/kind mismatch")
	}
}

func assertWebSecurityLifecycleBinding(t *testing.T, fixture webSecurityLifecycleFixture, lifecycle WebSecurityAdapterLifecycleResult, kind string) {
	t.Helper()
	owner, err := laneowner.Read(fixture.caseRoot, fixture.lane)
	if err != nil {
		t.Fatal(err)
	}
	binding, path, sha, err := memberexecution.ReadTaskBindingForOwner(fixture.caseRoot, fixture.lane, owner.ExecutorGeneration)
	if err != nil {
		t.Fatal(err)
	}
	if binding == nil {
		t.Fatalf("web-security member binding is missing: path=%s sha=%s lifecycle=%+v", path, sha, lifecycle)
	}
	observationPath := binding.Values["observation-path"]
	if err := validateWebSecurityReviewObservationFixture(
		fixture.caseRoot,
		observationPath,
		binding.Values["observation-sha256"],
		lifecycle.Run.ObservationEventID,
	); err != nil {
		t.Fatal(err)
	}
	if binding.Kind != kind || path != lifecycle.TaskBindingPath || !strings.EqualFold(sha, lifecycle.TaskBindingSHA256) ||
		binding.Values["input-path"] != fixture.inputPath || binding.Values["artifact-path"] != lifecycle.Run.PacketPath ||
		binding.Values["artifact-sha256"] != lifecycle.Run.PacketSHA256 || binding.Values["receipt-sha256"] != lifecycle.Run.ReceiptSHA256 ||
		binding.Values["observation-event-id"] != lifecycle.Run.ObservationEventID || observationPath == "" ||
		!validBinaryRESHA256(binding.Values["observation-sha256"]) || binding.Values["execution-status"] != lifecycle.Run.ExecutionStatus ||
		binding.Values["evidence-review-closure-sha256"] != lifecycle.ClosureSHA256 {
		t.Fatalf("web-security member binding = %+v path=%s sha=%s lifecycle=%+v", binding, path, sha, lifecycle)
	}
}

func assertWebSecurityLifecycleNoAuthority(t *testing.T, caseRoot string) {
	t.Helper()
	for _, kind := range []string{"authority.jsonl", "confirmed.jsonl"} {
		path := filepath.Join(caseRoot, ".steamai", "facts", kind)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("web-security lifecycle wrote forbidden fact %s: %v", path, err)
		}
	}
}

func writeWebSecurityLifecycleFile(t *testing.T, caseRoot, rel string, data []byte) {
	t.Helper()
	path := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func webSecurityLifecycleOpenAPI(port int) string {
	return fmt.Sprintf(`{"openapi":"3.0.3","servers":[{"url":"http://127.0.0.1:%d/api"}],"paths":{"/health":{"get":{"operationId":"health","responses":{"200":{"description":"ok"}}}}},"components":{"securitySchemes":{}}}`, port)
}

func closedWebSecurityLifecyclePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}
