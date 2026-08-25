package adapterhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
)

type webSecurityAuthorizedFixture struct {
	repoRoot    string
	caseRoot    string
	inputPath   string
	outputRoot  string
	gateEventID string
	options     AuthorizedRunOptions
}

func newWebSecurityAuthorizedFixture(t *testing.T, adapterID string, deferBinding bool) webSecurityAuthorizedFixture {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve repository root")
	}
	sourceRepo := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	caseRoot := filepath.Join(t.TempDir(), "case")
	project := publishProductionFixtureProject(t, sourceRepo, caseRoot, webSecurityPack, "web-security-fixture")
	repoRoot := project.RuntimeRepoRoot
	instructionIdentity := productionInstructionIdentityForFixture(t, caseRoot, webSecurityPack)
	lane := "feature-1"
	writeHostFile(t, filepath.Join(caseRoot, ".steamai", "board.json"), `{"lanes":[{"id":"feature-1","status":"open","workspace":"workspace/features/feature-1","currentExecutor":"executor-web","executorGeneration":1}]}`)
	writeHostFile(t, filepath.Join(caseRoot, ".steamai", "lanes", lane, "lane.json"), `{
  "schemaVersion": 1,
  "id": "feature-1",
  "type": "feature",
  "name": "feature-1",
  "title": "Feature 1",
  "status": "open",
  "authority": false,
  "workspace": "workspace/features/feature-1",
  "canWrite": ["own-workspace"],
  "readOnly": [".steamai/facts/**"],
  "outputs": ["observation", "request", "candidate", "summary"],
  "counters": {},
  "currentExecutor": "executor-web",
  "executorGeneration": 1,
  "createdAt": "2026-08-20T00:00:00Z",
  "updatedAt": "2026-08-20T00:00:00Z"
}`)
	if _, _, err := autonomy.EnsureManualProfile(caseRoot, lane); err != nil {
		t.Fatal(err)
	}

	action := "inspect"
	inputPath := "inputs/openapi.json"
	outputRoot := "workspace/features/feature-1/openapi/session-1"
	stopConditions := []string{"scope-drift", "source-drift", "output-exceeds-bounded-evidence-packet"}
	switch adapterID {
	case websecurity.InventoryAdapterID:
		writeHostFile(t, filepath.Join(caseRoot, filepath.FromSlash(inputPath)), syntheticAuthorizedOpenAPI(18080))
	case websecurity.ReplayAdapterID:
		action = "network"
		outputRoot = "workspace/features/feature-1/replay/session-1"
		stopConditions = []string{"live-target-ambiguity", "unexpected-outbound-request", "scope-drift", "delivery-uncertain", "response-body-limit", "response-read"}
		port := closedLoopbackPort(t)
		openAPIData := []byte(syntheticAuthorizedOpenAPI(port))
		source, err := websecurity.BindFile("inputs/replay-openapi.json", openAPIData, websecurity.MaxOpenAPIBytes)
		if err != nil {
			t.Fatal(err)
		}
		inventory, err := websecurity.ImportOpenAPI(source, openAPIData)
		if err != nil {
			t.Fatal(err)
		}
		inventoryData, err := websecurity.CanonicalInventoryBytes(inventory)
		if err != nil {
			t.Fatal(err)
		}
		inventoryPath := "inputs/openapi-inventory.json"
		inventoryBinding, err := websecurity.BindFile(inventoryPath, inventoryData, websecurity.MaxInventoryBytes)
		if err != nil {
			t.Fatal(err)
		}
		writeHostFile(t, filepath.Join(caseRoot, filepath.FromSlash(source.Path)), string(openAPIData))
		writeHostFile(t, filepath.Join(caseRoot, filepath.FromSlash(inventoryPath)), string(inventoryData))
		request, err := websecurity.NewReplayRequest(
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
		if err != nil {
			t.Fatal(err)
		}
		requestData, err := websecurity.CanonicalReplayRequestBytes(request)
		if err != nil {
			t.Fatal(err)
		}
		requestBinding, err := websecurity.BindReplayRequest(requestData)
		if err != nil {
			t.Fatal(err)
		}
		inputPath = requestBinding.Path
		writeHostFile(t, filepath.Join(caseRoot, filepath.FromSlash(inputPath)), string(requestData))
	default:
		t.Fatalf("unsupported web-security fixture adapter: %s", adapterID)
	}

	now := time.Now().UTC()
	profile := autonomy.Profile{
		SchemaVersion: 1,
		ProfileID:     "generated-web-feature-1",
		Lane:          lane,
		Mode:          autonomy.ModePreauthorized,
		AllowedActions: []string{
			action,
		},
		DeniedActions:  []string{},
		TargetScope:    []autonomy.Target{{Match: "exact", Value: inputPath}},
		Budget:         autonomy.Budget{RuntimeSeconds: 10, DiskMB: 4, Requests: 1},
		StopConditions: stopConditions,
		OutputPaths:    []string{outputRoot},
		RecordRequired: true,
		NotifyMainOn:   []string{"boundary-hit", "new-risk"},
		GrantedBy:      "user",
		GrantedAt:      now.Add(-time.Minute).Format(time.RFC3339),
		ExpiresAt:      now.Add(10 * time.Minute).Format(time.RFC3339),
	}
	plan, err := autonomy.PreviewProvision(autonomy.ProfileProvisionOptions{
		RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: webSecurityPack, Lane: lane, Profile: profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := autonomy.ApplyProfilePlan(plan, plan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	authorized, err := gate.Apply(repoRoot, caseRoot, webSecurityPack, gate.Options{
		Action: action, Lane: lane, Actor: "web-test", Subject: "bounded web-security fixture",
		TargetRef: inputPath, RuntimeSeconds: 10, DiskMB: 4, Requests: 1,
		OutputPaths: outputRoot, StopConditions: strings.Join(stopConditions, ","),
	})
	if err != nil || !authorized.Applied || authorized.Event == nil {
		t.Fatalf("authorize web-security fixture: %+v err=%v", authorized, err)
	}
	control := captureAuthorizedAdapterControl(t, caseRoot, lane)
	return webSecurityAuthorizedFixture{
		repoRoot: repoRoot, caseRoot: caseRoot, inputPath: inputPath, outputRoot: outputRoot,
		gateEventID: authorized.EventID,
		options: AuthorizedRunOptions{
			RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: webSecurityPack, GateEventID: authorized.EventID,
			ExecutionReportPath: outputRoot + "/adapter-report.json", AdapterID: adapterID,
			AdapterSession: "web-session-1", Actor: "mission-commander", DeferSuccessfulTaskBinding: deferBinding,
			ExecutionControlBinding: control, InstructionIdentity: instructionIdentity,
		},
	}
}

func syntheticAuthorizedOpenAPI(port int) string {
	return fmt.Sprintf(`{"openapi":"3.0.3","servers":[{"url":"http://127.0.0.1:%d/api"}],"paths":{"/health":{"get":{"operationId":"health","responses":{"200":{"description":"ok"}}}}},"components":{"securitySchemes":{}}}`, port)
}

func closedLoopbackPort(t *testing.T) int {
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

func openAPIInventoryHook(t *testing.T, calls *int) func(OpenAPIInventoryChildOptions) ([]byte, int, error) {
	t.Helper()
	return func(opt OpenAPIInventoryChildOptions) ([]byte, int, error) {
		*calls++
		result, err := RunOpenAPIInventoryChild(opt)
		if err != nil {
			return nil, 0, err
		}
		data, err := json.Marshal(result)
		return data, 7161, err
	}
}

func webSecurityDispatchForDecoderTest(t *testing.T, fixture webSecurityAuthorizedFixture) {
	t.Helper()
	publishAdapterDispatchForDecoderTest(t, fixture.options)
}

func TestDecodeOpenAPIInventoryChildResultRejectsMismatchedInstructionIdentity(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.InventoryAdapterID, true)
	webSecurityDispatchForDecoderTest(t, fixture)
	dispatch, _, dispatchSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		fixture.options.RepoRoot, fixture.options.CaseRoot, fixture.options.Pack, fixture.options.GateEventID,
	)
	if err != nil {
		t.Fatal(err)
	}
	childOpt := OpenAPIInventoryChildOptions{
		RepoRoot:                   fixture.options.RepoRoot,
		CaseRoot:                   fixture.options.CaseRoot,
		Pack:                       fixture.options.Pack,
		GateEventID:                fixture.options.GateEventID,
		ExpectedDispatchSHA256:     dispatchSHA,
		AdapterSession:             dispatch.Owner.AdapterSession,
		Executor:                   dispatch.Owner.CurrentExecutor,
		ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
		SourcePath:                 fixture.inputPath,
		ExecutionControlBinding:    executioncontrol.CloneBinding(fixture.options.ExecutionControlBinding),
		InstructionIdentity:        cloneAdapterInstructionIdentity(fixture.options.InstructionIdentity),
	}
	result, err := RunOpenAPIInventoryChild(childOpt)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeOpenAPIInventoryChildResult(data, fixture.inputPath, childOpt.InstructionIdentity); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeOpenAPIInventoryChildResult(data, fixture.inputPath, mismatchedExpectedInstructionIdentity(childOpt.InstructionIdentity)); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("OpenAPI child decoder accepted mismatched identity: %v", err)
	}
}

func boundedReplayChildOptionsForFixture(t *testing.T, fixture webSecurityAuthorizedFixture) BoundedReplayChildOptions {
	t.Helper()
	webSecurityDispatchForDecoderTest(t, fixture)
	dispatch, _, dispatchSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		fixture.options.RepoRoot, fixture.options.CaseRoot, fixture.options.Pack, fixture.options.GateEventID,
	)
	if err != nil {
		t.Fatal(err)
	}
	return BoundedReplayChildOptions{
		RepoRoot:                   fixture.options.RepoRoot,
		CaseRoot:                   fixture.options.CaseRoot,
		Pack:                       fixture.options.Pack,
		GateEventID:                fixture.options.GateEventID,
		ExpectedDispatchSHA256:     dispatchSHA,
		AdapterSession:             dispatch.Owner.AdapterSession,
		Executor:                   dispatch.Owner.CurrentExecutor,
		ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
		RequestPath:                fixture.inputPath,
		ExecutionControlBinding:    executioncontrol.CloneBinding(fixture.options.ExecutionControlBinding),
		InstructionIdentity:        cloneAdapterInstructionIdentity(fixture.options.InstructionIdentity),
	}
}

func TestDecodeBoundedReplayChildResultRejectsMismatchedInstructionIdentity(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.ReplayAdapterID, true)
	childOpt := boundedReplayChildOptionsForFixture(t, fixture)
	result, err := RunBoundedReplayChild(childOpt)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeBoundedReplayChildResult(data, fixture.inputPath, childOpt.InstructionIdentity); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeBoundedReplayChildResult(data, fixture.inputPath, mismatchedExpectedInstructionIdentity(childOpt.InstructionIdentity)); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("bounded replay child decoder accepted mismatched identity: %v", err)
	}
}

func TestRunBoundedReplayChildRejectsInstructionDriftBeforeNetworkSink(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.ReplayAdapterID, true)
	childOpt := boundedReplayChildOptionsForFixture(t, fixture)
	if childOpt.InstructionIdentity == nil || len(childOpt.InstructionIdentity.Sources) == 0 {
		t.Fatal("bounded replay fixture omitted instruction sources")
	}
	sourcePath := filepath.Join(
		fixture.caseRoot,
		".steamai",
		filepath.FromSlash(childOpt.InstructionIdentity.Sources[0].Path),
	)
	childOpt.beforeExecute = func() error {
		return os.WriteFile(sourcePath, []byte("drifted before network sink\n"), 0o600)
	}
	result, err := RunBoundedReplayChild(childOpt)
	if err == nil || !strings.Contains(err.Error(), "instruction") || result.Result.Delivery.Attempts != 0 {
		t.Fatalf("instruction drift crossed bounded replay network sink: result=%+v err=%v", result, err)
	}
}

func TestRunAuthorizedOpenAPIInventoryRecordsExactLifecycleWithoutCatalogExecution(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.InventoryAdapterID, true)
	childCalls := 0
	fixture.options.testHooks = &hostTestHooks{runOpenAPIInventoryChild: openAPIInventoryHook(t, &childCalls)}
	first, err := RunAuthorizedGate(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind != "openapi-inventory-authorized-run" || first.ExecutionStatus != "succeeded" ||
		first.ExecutionExitStatus != "completed" || first.PacketSHA256 == "" || first.ReportSHA256 == "" ||
		first.ReceiptSHA256 == "" || first.ObservationEventID == "" || first.TaskBindingSHA256 != "" ||
		!first.ChildLaunched || first.ChildProcessID != 7161 || childCalls != 1 || !first.NoNetwork ||
		first.NoNetworkBoundary != fixedChildNoNetworkCodepath || !first.ProfileRevoked {
		t.Fatalf("OpenAPI inventory first lifecycle = %+v childCalls=%d", first, childCalls)
	}
	inventoryData, err := os.ReadFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(first.PacketPath)))
	if err != nil {
		t.Fatal(err)
	}
	inventory, err := websecurity.DecodeInventory(inventoryData)
	if err != nil || inventory.Source.Path != fixture.inputPath || len(inventory.Endpoints) != 1 || !inventory.Boundaries.NoSecretsPersisted {
		t.Fatalf("OpenAPI inventory artifact = %+v err=%v", inventory, err)
	}
	fixture.options.testHooks.runOpenAPIInventoryChild = func(OpenAPIInventoryChildOptions) ([]byte, int, error) {
		childCalls++
		return nil, 0, errors.New("terminal replay must not rerun the OpenAPI child")
	}
	second, err := RunAuthorizedGate(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replay || second.ChildLaunched || childCalls != 1 || second.PacketSHA256 != first.PacketSHA256 ||
		second.ReportSHA256 != first.ReportSHA256 || second.ReceiptSHA256 != first.ReceiptSHA256 ||
		second.ObservationEventID != first.ObservationEventID || !second.ProfileAlreadyManual {
		t.Fatalf("OpenAPI inventory terminal replay = %+v first=%+v childCalls=%d", second, first, childCalls)
	}
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, ".steamai", "facts", "authority.jsonl"))
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, ".steamai", "facts", "confirmed.jsonl"))
}

func TestRunAuthorizedOpenAPIInventoryBindsExactOwnerGeneration(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.InventoryAdapterID, false)
	childCalls := 0
	fixture.options.testHooks = &hostTestHooks{runOpenAPIInventoryChild: openAPIInventoryHook(t, &childCalls)}
	result, err := RunAuthorizedGate(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if result.TaskBindingSHA256 == "" || result.TaskBindingPath == "" || childCalls != 1 {
		t.Fatalf("OpenAPI owner-generation binding missing: %+v", result)
	}
	binding, path, sha, err := memberexecution.ReadTaskBindingForOwner(fixture.caseRoot, "feature-1", 1)
	if err != nil || binding == nil || path != result.TaskBindingPath || sha != result.TaskBindingSHA256 ||
		binding.Kind != "web-security-openapi-inventory-evidence" || binding.Values["artifact-sha256"] != result.PacketSHA256 ||
		binding.Values["receipt-sha256"] != result.ReceiptSHA256 || binding.Values["observation-event-id"] != result.ObservationEventID {
		t.Fatalf("OpenAPI owner-generation binding = %+v path=%s sha=%s err=%v", binding, path, sha, err)
	}
}

func TestRunAuthorizedOpenAPIInventoryRecoversCommittedOutputWithoutChild(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.InventoryAdapterID, true)
	childCalls := 0
	fixture.options.testHooks = &hostTestHooks{
		runOpenAPIInventoryChild: openAPIInventoryHook(t, &childCalls),
		afterWebSecurityOutputCommit: func() error {
			return errors.New("synthetic interruption after web-security output commit")
		},
	}
	if _, err := RunAuthorizedGate(fixture.options); err == nil || !strings.Contains(err.Error(), "synthetic interruption") {
		t.Fatalf("web-security output commit cutpoint error=%v", err)
	}
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(fixture.outputRoot), openAPIInventoryFileName))
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(fixture.outputRoot), "adapter-report.json"))
	fixture.options.testHooks.afterWebSecurityOutputCommit = nil
	fixture.options.testHooks.runOpenAPIInventoryChild = func(OpenAPIInventoryChildOptions) ([]byte, int, error) {
		childCalls++
		return nil, 0, errors.New("commit recovery reran the OpenAPI child")
	}
	recovered, err := RunAuthorizedGate(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if !recovered.Replay || recovered.ChildLaunched || childCalls != 1 || recovered.ExecutionStatus != "succeeded" ||
		recovered.PacketSHA256 == "" || recovered.ReportSHA256 == "" || recovered.ReceiptSHA256 == "" ||
		recovered.ObservationEventID == "" || !recovered.ProfileRevoked {
		t.Fatalf("web-security committed output recovery = %+v childCalls=%d", recovered, childCalls)
	}
}

func TestRunAuthorizedBoundedReplayUncertainDeliveryIsTerminalAndNeverRetried(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.ReplayAdapterID, true)
	childCalls := 0
	fixture.options.testHooks = &hostTestHooks{runBoundedReplayChild: func(opt BoundedReplayChildOptions) ([]byte, int, error) {
		childCalls++
		result, err := RunBoundedReplayChild(opt)
		if err != nil {
			return nil, 0, err
		}
		data, err := json.Marshal(result)
		return data, 7261, err
	}}
	first, err := RunAuthorizedGate(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if first.Kind != "bounded-replay-authorized-run" || first.ExecutionStatus != "aborted" ||
		first.ExecutionExitStatus != "delivery-uncertain" || first.PacketSHA256 == "" || first.ReceiptSHA256 == "" ||
		first.ObservationEventID == "" || first.TaskBindingSHA256 != "" || first.NoNetwork ||
		first.NoNetworkBoundary != boundedReplayNetworkBoundary || childCalls != 1 || !first.ProfileRevoked {
		t.Fatalf("bounded replay uncertain lifecycle = %+v childCalls=%d", first, childCalls)
	}
	resultData, err := os.ReadFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(first.PacketPath)))
	if err != nil {
		t.Fatal(err)
	}
	replayResult, err := websecurity.DecodeReplayResult(resultData)
	if err != nil || replayResult.Status != "delivery-uncertain" || replayResult.Delivery.Attempts != 1 || replayResult.Delivery.Certain || replayResult.Actual != nil || replayResult.Diff != nil {
		t.Fatalf("bounded replay redacted result = %+v err=%v", replayResult, err)
	}
	fixture.options.testHooks.runBoundedReplayChild = func(BoundedReplayChildOptions) ([]byte, int, error) {
		childCalls++
		return nil, 0, errors.New("uncertain delivery replay retried the request")
	}
	second, err := RunAuthorizedGate(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replay || second.ChildLaunched || childCalls != 1 || second.ExecutionExitStatus != "delivery-uncertain" ||
		second.PacketSHA256 != first.PacketSHA256 || second.ReceiptSHA256 != first.ReceiptSHA256 ||
		second.ObservationEventID != first.ObservationEventID || !second.ProfileAlreadyManual {
		t.Fatalf("bounded replay uncertain terminal replay = %+v first=%+v childCalls=%d", second, first, childCalls)
	}
}

func TestRunAuthorizedBoundedReplayLaunchCutClosesWithoutRetry(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.ReplayAdapterID, true)
	childCalls := 0
	fixture.options.testHooks = &hostTestHooks{
		runBoundedReplayChild: func(BoundedReplayChildOptions) ([]byte, int, error) {
			childCalls++
			return nil, 7361, errors.New("synthetic replay child interruption")
		},
		afterWebSecurityChildLaunch: func(int) error {
			return errors.New("stop after durable web-security child launch proof")
		},
	}
	if _, err := RunAuthorizedGate(fixture.options); err == nil || !strings.Contains(err.Error(), "durable web-security child launch proof") {
		t.Fatalf("bounded replay launch cutpoint error=%v", err)
	}
	fixture.options.testHooks.afterWebSecurityChildLaunch = nil
	fixture.options.testHooks.runBoundedReplayChild = func(BoundedReplayChildOptions) ([]byte, int, error) {
		childCalls++
		return nil, 0, errors.New("launch recovery retried the bounded request")
	}
	closed, err := RunAuthorizedGate(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if !closed.Replay || closed.ChildLaunched || childCalls != 1 || closed.ExecutionStatus != "aborted" ||
		closed.ExecutionExitStatus != "delivery-uncertain" || closed.PacketPath != "" || closed.PacketSHA256 != "" ||
		closed.ReceiptSHA256 == "" || closed.ObservationEventID == "" || !closed.ProfileRevoked {
		t.Fatalf("bounded replay launch closure = %+v childCalls=%d", closed, childCalls)
	}
}

func TestReplayAuthEnvironmentCarriesOnlyExactReference(t *testing.T) {
	t.Setenv("STEAMAI_AUTH_EXACT", "secret-exact")
	t.Setenv("STEAMAI_AUTH_OTHER", "secret-other")
	env := replayAuthEnvironment("STEAMAI_AUTH_EXACT")
	exact := 0
	for _, item := range env {
		if item == "STEAMAI_AUTH_EXACT=secret-exact" {
			exact++
		}
		if strings.HasPrefix(item, "STEAMAI_AUTH_OTHER=") {
			t.Fatalf("bounded replay child environment inherited unrelated auth ref: %q", item)
		}
	}
	if exact != 1 {
		t.Fatalf("bounded replay child environment exact auth count=%d env=%v", exact, env)
	}
	for _, item := range replayAuthEnvironment("") {
		if strings.HasPrefix(item, "STEAMAI_AUTH_") {
			t.Fatalf("anonymous bounded replay child environment inherited auth ref: %q", item)
		}
	}
}

func TestRunAuthorizedOpenAPISealedReplayDoesNotRequireLiveSource(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.InventoryAdapterID, true)
	childCalls := 0
	fixture.options.testHooks = &hostTestHooks{runOpenAPIInventoryChild: openAPIInventoryHook(t, &childCalls)}
	first, err := RunAuthorizedGate(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fixture.caseRoot, filepath.FromSlash(fixture.inputPath))); err != nil {
		t.Fatal(err)
	}
	fixture.options.testHooks.runOpenAPIInventoryChild = func(OpenAPIInventoryChildOptions) ([]byte, int, error) {
		childCalls++
		return nil, 0, errors.New("sealed OpenAPI replay relaunched child")
	}
	second, err := RunAuthorizedGate(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replay || second.ChildLaunched || childCalls != 1 || second.PacketSHA256 != first.PacketSHA256 ||
		second.ReportSHA256 != first.ReportSHA256 || second.ReceiptSHA256 != first.ReceiptSHA256 ||
		second.ObservationEventID != first.ObservationEventID || !second.ProfileAlreadyManual {
		t.Fatalf("sealed OpenAPI source-independent replay = %+v first=%+v childCalls=%d", second, first, childCalls)
	}
}

func TestRunAuthorizedWebSecurityFailureClosureRejectsLaunchProofReplacement(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.ReplayAdapterID, true)
	childCalls := 0
	mutated := false
	fixture.options.testHooks = &hostTestHooks{
		runBoundedReplayChild: func(BoundedReplayChildOptions) ([]byte, int, error) {
			childCalls++
			return nil, 7461, errors.New("synthetic bounded replay child interruption")
		},
		afterWebSecurityChildLaunch: func(int) error {
			return errors.New("stop after exact bounded replay launch proof")
		},
	}
	if _, err := RunAuthorizedGate(fixture.options); err == nil || !strings.Contains(err.Error(), "exact bounded replay launch proof") {
		t.Fatalf("bounded replay launch cutpoint error=%v", err)
	}
	fixture.options.testHooks.afterWebSecurityChildLaunch = nil
	fixture.options.testHooks.runBoundedReplayChild = func(BoundedReplayChildOptions) ([]byte, int, error) {
		childCalls++
		return nil, 0, errors.New("failure closure retried bounded replay")
	}
	fixture.options.testHooks.beforeWebSecurityFailureClosureValidation = func() error {
		if mutated {
			return nil
		}
		mutated = true
		path := filepath.Join(fixture.caseRoot, filepath.FromSlash(fixture.outputRoot), webSecurityChildLaunchFileName)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var launch webSecurityChildLaunch
		if err := decodeVMPIDAStrictJSON(data, &launch); err != nil {
			return err
		}
		launch.ChildProcessID++
		data, err = canonicalJSON(launch)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0o600)
	}
	result, err := RunAuthorizedGate(fixture.options)
	if err == nil || !strings.Contains(err.Error(), "exact artifact validation") {
		t.Fatalf("replacement web-security proof closure result=%+v err=%v", result, err)
	}
	if result.ReceiptSHA256 != "" || result.ObservationEventID != "" || childCalls != 1 {
		t.Fatalf("replacement web-security proof wrote closure provenance: %+v childCalls=%d", result, childCalls)
	}
}

func TestRunAuthorizedWebSecurityRejectsUncommittedPublicOutput(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.ReplayAdapterID, true)
	childCalls := 0
	fixture.options.testHooks = &hostTestHooks{
		runBoundedReplayChild: func(BoundedReplayChildOptions) ([]byte, int, error) {
			childCalls++
			return nil, 7561, errors.New("synthetic bounded replay interruption")
		},
		afterWebSecurityChildLaunch: func(int) error {
			return errors.New("stop after bounded replay launch")
		},
	}
	if _, err := RunAuthorizedGate(fixture.options); err == nil || !strings.Contains(err.Error(), "stop after bounded replay launch") {
		t.Fatalf("bounded replay launch cutpoint error=%v", err)
	}
	writeHostFile(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(fixture.outputRoot), boundedReplayResultFileName), `{"partial":true}`)
	fixture.options.testHooks.afterWebSecurityChildLaunch = nil
	fixture.options.testHooks.runBoundedReplayChild = func(BoundedReplayChildOptions) ([]byte, int, error) {
		childCalls++
		return nil, 0, errors.New("uncommitted output recovery retried bounded replay")
	}
	result, err := RunAuthorizedGate(fixture.options)
	if err == nil || !strings.Contains(err.Error(), "uncommitted public output") {
		t.Fatalf("uncommitted web-security public output result=%+v err=%v", result, err)
	}
	if result.ReceiptSHA256 != "" || result.ObservationEventID != "" || childCalls != 1 {
		t.Fatalf("uncommitted web-security output wrote provenance: %+v childCalls=%d", result, childCalls)
	}
}

func TestRunAuthorizedWebSecurityRejectsPartialPublicOutputAfterCommit(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.InventoryAdapterID, true)
	childCalls := 0
	fixture.options.testHooks = &hostTestHooks{
		runOpenAPIInventoryChild: openAPIInventoryHook(t, &childCalls),
		afterWebSecurityOutputCommit: func() error {
			return errors.New("stop after web-security commit before publication")
		},
	}
	if _, err := RunAuthorizedGate(fixture.options); err == nil || !strings.Contains(err.Error(), "before publication") {
		t.Fatalf("web-security output commit cutpoint error=%v", err)
	}
	writeHostFile(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(fixture.outputRoot), openAPIInventoryFileName), `{"partial":true}`)
	fixture.options.testHooks.afterWebSecurityOutputCommit = nil
	fixture.options.testHooks.runOpenAPIInventoryChild = func(OpenAPIInventoryChildOptions) ([]byte, int, error) {
		childCalls++
		return nil, 0, errors.New("partial output recovery reran OpenAPI child")
	}
	result, err := RunAuthorizedGate(fixture.options)
	if err == nil || (!strings.Contains(err.Error(), "atomic replay") && !strings.Contains(err.Error(), "differs") && !strings.Contains(err.Error(), "incomplete or not regular")) {
		t.Fatalf("partial web-security public output result=%+v err=%v", result, err)
	}
	if result.ReceiptSHA256 != "" || result.ObservationEventID != "" || childCalls != 1 {
		t.Fatalf("partial web-security output wrote provenance: %+v childCalls=%d", result, childCalls)
	}
}
