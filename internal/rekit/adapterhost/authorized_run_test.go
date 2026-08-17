package adapterhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
)

type vmpAuthorizedFixture struct {
	repoRoot    string
	caseRoot    string
	requestPath string
	gateEventID string
	dispatch    gate.AdapterExecutionDispatchResult
	options     Options
}

func newVMPAuthorizedFixture(t *testing.T, recordDispatch bool) vmpAuthorizedFixture {
	t.Helper()
	return newVMPAuthorizedFixtureWithStateRoot(t, recordDispatch, ".rekit")
}

func newVMPAuthorizedFixtureWithStateRoot(t *testing.T, recordDispatch bool, stateDir string) vmpAuthorizedFixture {
	t.Helper()
	root := t.TempDir()
	repoRoot := filepath.Join(root, "repo")
	caseRoot := filepath.Join(root, "case")
	writeHostFile(t, filepath.Join(repoRoot, "packs", "vmp-re", "manifest.yml"), `schemaVersion: 1
name: vmp-re
version: 0.2.0
description: VMP IDA adapter fixture
maturity: mature
managedFiles: []
templateFiles: []
localNeverOverwrite: []
managedBlock:
  file: CLAUDE.local.md
  blockId: vmp-re:router
  source: CLAUDE.local.snippet.md
syncPolicy:
  managedFiles: overwrite-with-backup
  templateFiles: create-if-missing
  localFiles: never-overwrite
workstreamDefaults:
  defaultAuthorityLane: main
  defaultStartLaneType: main
  handoffPath: handoff.md
  backupRoot: `+stateDir+`/backups
  requestDefaultTargetLane: main
authorityFiles:
  - handoff.md
commonPolicies: []
policyOverlays: []
subagentRoutes: []
promoteFiles: []
toolingFiles:
  - tooling/catalog.yml
promptFiles: []
laneTypes:
  - id: main
    title: Main
    authority: false
    workspaceRoot: workspace/main
    canWrite: own-workspace
    readOnly: `+stateDir+`/facts/**
    outputs: observation
toolingCandidateSources: []
heavyToolGates:
  - id: debug
    title: Debug fixture
    sideEffects: debug
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: scope-drift
  - id: inspect
    title: Existing IDA sidecar index inspection
    sideEffects: inspect,filesystem-read,bounded-packet-write
    defaultRisk: medium
    requiresConfirmation: true
    stopConditions: scope-drift,source-drift,output-exceeds-bounded-evidence-packet
  - id: patch
    title: Patch fixture
    sideEffects: patch
    defaultRisk: high
    requiresConfirmation: true
    stopConditions: scope-drift
promoteDenyPatterns: []
budgets:
  defaultMarkdown: 16384
`)
	sentinel := filepath.ToSlash(filepath.Join(root, "catalog-entry-must-not-run"))
	writeHostFile(t, filepath.Join(repoRoot, "packs", "vmp-re", "tooling", "catalog.yml"), `schemaVersion: 1
pack: vmp-re
purpose: VMP IDA adapter fixture

tools:
  - id: vmp-ida-index-inspector
    status: mainline
    entry: `+sentinel+`
    purpose: Inspect fixed existing IDA TSV indexes.
    sideEffects: filesystem-read,bounded-packet-write
    gateActions: inspect
`)
	writeHostFile(t, filepath.Join(caseRoot, stateDir, "instance.yml"), "templateRoot: \""+repoRoot+"\"\ntemplatePack: \"vmp-re\"\nprojectName: \"vmp-adapter-fixture\"\nprojectRoot: \""+caseRoot+"\"\n")
	writeHostFile(t, filepath.Join(caseRoot, stateDir, "board.json"), `{"lanes":[{"id":"main","status":"open","workspace":"workspace/main","currentExecutor":"executor-vmp","executorGeneration":1}]}`)
	writeHostFile(t, filepath.Join(caseRoot, stateDir, "lanes", "main", "lane.json"), `{
  "schemaVersion": 1,
  "id": "main",
  "type": "main",
  "name": "main",
  "title": "Main",
  "status": "open",
  "authority": false,
  "workspace": "workspace/main",
  "canWrite": ["own-workspace"],
  "readOnly": ["`+stateDir+`/facts/**"],
  "outputs": ["observation"],
  "counters": {},
  "currentExecutor": "executor-vmp",
  "executorGeneration": 1,
  "createdAt": "2026-08-10T00:00:00Z",
  "updatedAt": "2026-08-10T00:00:00Z"
}`)
	writeHostFile(t, filepath.Join(caseRoot, filepath.FromSlash(VMPIDAIndexQueryPath)), `{"schemaVersion":1,"terms":["needle"],"maxRowsPerIndex":5}`)
	writeHostFile(t, filepath.Join(caseRoot, filepath.FromSlash(VMPIDAIndexDefaultExportRoot), "function_index.tsv"), "rva\tname\n0x1000\tneedle_dispatch\n")
	preview, err := PreviewVMPIDAIndexRequest(caseRoot, VMPIDAIndexDefaultExportRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PublishVMPIDAIndexRequest(caseRoot, preview); err != nil {
		t.Fatal(err)
	}
	outputRoot := "workspace/main/ida/session-1"
	if _, _, err := autonomy.EnsureManualProfile(caseRoot, "main"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	profilePlan, err := autonomy.PreviewProvision(autonomy.ProfileProvisionOptions{
		RepoRoot: repoRoot,
		CaseRoot: caseRoot,
		Pack:     "vmp-re",
		Lane:     "main",
		Profile: autonomy.Profile{
			SchemaVersion: 1, ProfileID: "generated-vmp-ida-main", Lane: "main", Mode: autonomy.ModePreauthorized,
			AllowedActions: []string{"inspect"}, DeniedActions: []string{"debug", "patch"},
			TargetScope:    []autonomy.Target{{Match: "exact", Value: preview.RequestPath}},
			Budget:         autonomy.Budget{RuntimeSeconds: 10, DiskMB: 4, Requests: 1},
			StopConditions: []string{"scope-drift", "source-drift", "output-exceeds-bounded-evidence-packet"},
			OutputPaths:    []string{outputRoot}, RecordRequired: true,
			NotifyMainOn: []string{"boundary-hit", "new-risk", "destructive-change", "authority-write-needed"},
			GrantedBy:    "user", GrantedAt: now.Add(-time.Minute).Format(time.RFC3339), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := autonomy.ApplyProfilePlan(profilePlan, profilePlan.ExpectedPlanSHA256); err != nil {
		t.Fatal(err)
	}
	authorized, err := gate.Apply(repoRoot, caseRoot, "vmp-re", gate.Options{
		Action: "inspect", Lane: "main", Actor: "vmp-test", Subject: "inspect existing IDA indexes",
		TargetRef: preview.RequestPath, RuntimeSeconds: 10, DiskMB: 4, Requests: 1,
		OutputPaths: outputRoot, StopConditions: "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
	})
	if err != nil || !authorized.Applied || authorized.Event == nil {
		t.Fatalf("authorize VMP IDA request: %+v err=%v", authorized, err)
	}
	fixture := vmpAuthorizedFixture{repoRoot: repoRoot, caseRoot: caseRoot, requestPath: preview.RequestPath, gateEventID: authorized.EventID}
	if recordDispatch {
		dispatchOpt := gate.Options{
			GateEventID: authorized.EventID, ExecutionReportPath: outputRoot + "/adapter-report.json",
			AdapterID: VMPIDAIndexAdapterID, Executor: "executor-vmp", ExpectedExecutorGeneration: 1,
			AdapterHarness: adapterHarness, AdapterSession: "vmp-session-1", Actor: "mission-commander",
		}
		dispatchPreview, err := gate.RecordAdapterExecutionDispatch(repoRoot, caseRoot, "vmp-re", dispatchOpt)
		if err != nil {
			t.Fatal(err)
		}
		dispatchOpt.ExpectedAdapterExecutionDispatchBindingSHA256 = dispatchPreview.BindingSHA256
		fixture.dispatch, err = gate.RecordAdapterExecutionDispatch(repoRoot, caseRoot, "vmp-re", dispatchOpt)
		if err != nil || !fixture.dispatch.Applied {
			t.Fatalf("record VMP dispatch: %+v err=%v", fixture.dispatch, err)
		}
		fixture.options = Options{RepoRoot: repoRoot, CaseRoot: caseRoot, Pack: "vmp-re", GateEventID: authorized.EventID, ExpectedDispatchSHA256: fixture.dispatch.DispatchSHA256}
	}
	return fixture
}

func vmpProfileJSON(requestPath, outputRoot, expiresAt, grantedBy string) string {
	return `{
  "schemaVersion": 1,
  "profileId": "generated-vmp-ida-main",
  "lane": "main",
  "mode": "preauthorized",
  "allowedActions": ["inspect"],
  "deniedActions": [],
  "targetScope": [{"match":"exact","value":"` + requestPath + `"}],
  "budget": {"runtimeSeconds":10,"diskMB":4,"requests":1},
  "stopConditions": ["scope-drift","source-drift","output-exceeds-bounded-evidence-packet"],
  "outputPaths": ["` + outputRoot + `"],
  "recordRequired": true,
  "notifyMainOn": ["boundary-hit","new-risk"],
  "grantedBy": "` + grantedBy + `",
  "grantedAt": "2026-08-10T00:00:00Z",
  "expiresAt": "` + expiresAt + `"
}`
}

func childOptionsForFixture(
	t *testing.T,
	fixture vmpAuthorizedFixture,
) VMPIDAIndexChildOptions {
	t.Helper()
	dispatch, _, dispatchSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		fixture.repoRoot,
		fixture.caseRoot,
		"vmp-re",
		fixture.gateEventID,
	)
	if err != nil {
		t.Fatal(err)
	}
	return VMPIDAIndexChildOptions{
		RepoRoot:                   fixture.repoRoot,
		CaseRoot:                   fixture.caseRoot,
		Pack:                       "vmp-re",
		GateEventID:                fixture.gateEventID,
		ExpectedDispatchSHA256:     dispatchSHA,
		AdapterSession:             dispatch.Owner.AdapterSession,
		Executor:                   dispatch.Owner.CurrentExecutor,
		ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
		RequestPath:                fixture.requestPath,
	}
}

func strictChildBytes(t *testing.T, opt VMPIDAIndexChildOptions) []byte {
	t.Helper()
	result, err := RunVMPIDAIndexChild(opt)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRunVMPIDAIndexReusesDurableAttemptRuntimeStart(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, true)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	executableData, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, dispatchPath, dispatchSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		fixture.repoRoot,
		fixture.caseRoot,
		"vmp-re",
		fixture.gateEventID,
	)
	if err != nil {
		t.Fatal(err)
	}
	request, err := ReadVMPIDAIndexRequest(fixture.caseRoot, fixture.requestPath)
	if err != nil {
		t.Fatal(err)
	}
	attemptStarted := time.Now().UTC().Add(-11 * time.Second)
	if _, _, err := publishVMPIDAExecutionAttempt(
		fixture.caseRoot,
		dispatch,
		dispatchPath,
		dispatchSHA,
		request.RequestSHA256,
		sha256Hex(executableData),
		dispatch.ReportPath,
		attemptStarted,
	); err != nil {
		t.Fatal(err)
	}
	launches := 0
	fixture.options.testHooks = &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 4141, nil
		},
	}
	result, err := Run(fixture.options)
	if err == nil || !strings.Contains(err.Error(), "exhausted its runtime budget before child launch") {
		t.Fatalf("durable attempt runtime result=%+v err=%v", result, err)
	}
	if launches != 0 || result.ExecutionStatus != "" || result.ExecutionExitStatus != "" {
		t.Fatalf("durable exhausted attempt launched child or claimed terminal execution: result=%+v launches=%d", result, launches)
	}
}

func TestRunVMPIDAIndexRejectsProfileHashDriftAndExpiryBeforeChild(t *testing.T) {
	for name, profile := range map[string]struct {
		profile func(vmpAuthorizedFixture) string
		want    string
	}{
		"hash drift": {profile: func(f vmpAuthorizedFixture) string {
			return vmpProfileJSON(f.requestPath, "workspace/main/ida/session-1", "2999-01-01T00:00:00Z", "different-user")
		}, want: "profile hash"},
		"expiry": {profile: func(f vmpAuthorizedFixture) string {
			return vmpProfileJSON(f.requestPath, "workspace/main/ida/session-1", "2020-01-01T00:00:00Z", "user")
		}, want: "expired"},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newVMPAuthorizedFixture(t, true)
			writeHostFile(t, filepath.Join(fixture.caseRoot, ".rekit", "lanes", "main", "autonomy.json"), profile.profile(fixture))
			launched := 0
			fixture.options.testHooks = &hostTestHooks{runVMPIDAChild: func(VMPIDAIndexChildOptions) ([]byte, int, error) {
				launched++
				return nil, 0, errors.New("must not launch")
			}}
			if _, err := Run(fixture.options); err == nil || !strings.Contains(strings.ToLower(err.Error()), profile.want) {
				t.Fatalf("Run error = %v, want %q", err, profile.want)
			}
			if launched != 0 {
				t.Fatalf("child launches = %d, want 0", launched)
			}
		})
	}
}

func TestRunVMPIDAIndexRejectsWrongRequestTargetAndCandidate(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, true)
	dispatch, path, sha, _, err := gate.ReadCurrentAdapterExecutionDispatch(fixture.repoRoot, fixture.caseRoot, "vmp-re", fixture.gateEventID)
	if err != nil {
		t.Fatal(err)
	}
	packetPath := filepath.ToSlash(filepath.Join(filepath.Dir(dispatch.ReportPath), vmpIDAIndexPacketFileName))
	wrongTarget := dispatch
	wrongTarget.Gate.Target = "tooling/ida-agent-bridge/requests/" + strings.Repeat("a", 64) + ".json"
	if err := validateVMPIDADispatch(wrongTarget, fixture.requestPath, dispatch.ReportPath, packetPath); err == nil || !strings.Contains(err.Error(), "invalid request") {
		t.Fatalf("wrong request target error = %v", err)
	}
	wrongCandidate := dispatch
	wrongCandidate.Adapter.Candidate.ID = "catalog-entry"
	if err := validateVMPIDADispatch(wrongCandidate, fixture.requestPath, dispatch.ReportPath, packetPath); err == nil || !strings.Contains(err.Error(), "compiled-in candidate") {
		t.Fatalf("wrong candidate error = %v", err)
	}
	if path == "" || sha == "" {
		t.Fatal("fixture omitted immutable dispatch identity")
	}
}

func TestRunVMPIDAIndexNeverExecutesCatalogEntryAndPublishesPacket(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, true)
	launches := 0
	fixture.options.testHooks = &hostTestHooks{runVMPIDAChild: func(opt VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return strictChildBytes(t, opt), 4242, nil
	}}
	result, err := Run(fixture.options)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || result.AdapterID != VMPIDAIndexAdapterID || filepath.Base(result.ArtifactPath) != vmpIDAIndexPacketFileName || result.ProcessID != 4242 || result.NoNetworkBoundary != fixedChildNoNetworkCodepath {
		t.Fatalf("VMP host result = %+v launches=%d", result, launches)
	}
	packetData, err := os.ReadFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(result.ArtifactPath)))
	if err != nil || !strings.Contains(string(packetData), "needle_dispatch") {
		t.Fatalf("packet = %s err=%v", packetData, err)
	}
	catalogEntry := filepath.Join(filepath.Dir(fixture.repoRoot), "catalog-entry-must-not-run")
	if _, err := os.Lstat(catalogEntry); !os.IsNotExist(err) {
		t.Fatalf("catalog entry was treated as executable: %v", err)
	}
}

func TestRunAuthorizedGateUsesSTeamAIStateRoot(t *testing.T) {
	fixture := newVMPAuthorizedFixtureWithStateRoot(t, false, ".steamai")
	if _, err := os.Stat(filepath.Join(fixture.caseRoot, ".steamai", "lanes", "main", "autonomy.json")); err != nil {
		t.Fatalf("current STeamAI autonomy profile is missing: %v", err)
	}
	launches := 0
	result, err := RunAuthorizedGate(authorizedRunOptionsForFixture(fixture, &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 5050, nil
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || result.DispatchPath != ".steamai/lanes/main/adapter-executions/"+fixture.gateEventID+"/dispatch.json" || result.ReceiptPath != ".steamai/lanes/main/adapter-executions/"+fixture.gateEventID+"/receipt.json" || result.ProfilePath != ".steamai/lanes/main/autonomy.json" {
		t.Fatalf("STeamAI authorized run paths drifted: result=%+v launches=%d", result, launches)
	}
	if _, err := os.Lstat(filepath.Join(fixture.caseRoot, ".rekit")); !os.IsNotExist(err) {
		t.Fatalf("STeamAI authorized run wrote legacy root: %v", err)
	}
}

func TestRunAuthorizedGateRejectsDualStateRoots(t *testing.T) {
	fixture := newVMPAuthorizedFixtureWithStateRoot(t, false, ".steamai")
	if _, err := os.Stat(filepath.Join(fixture.caseRoot, ".steamai", "lanes", "main", "autonomy.json")); err != nil {
		t.Fatalf("current STeamAI autonomy profile is missing before dual-root conflict: %v", err)
	}
	if err := os.Mkdir(filepath.Join(fixture.caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	launches := 0
	_, err := RunAuthorizedGate(authorizedRunOptionsForFixture(fixture, &hostTestHooks{
		runVMPIDAChild: func(VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return nil, 0, errors.New("must not launch")
		},
	}))
	if err == nil || !strings.Contains(err.Error(), "must not coexist") || launches != 0 {
		t.Fatalf("dual-root authorized run error=%v launches=%d", err, launches)
	}
}

func TestRunAuthorizedGateTerminalReportReplaysWithoutChild(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	opt := AuthorizedRunOptions{
		RepoRoot: fixture.repoRoot, CaseRoot: fixture.caseRoot, Pack: "vmp-re", GateEventID: fixture.gateEventID,
		ExecutionReportPath: "workspace/main/ida/session-1/adapter-report.json", AdapterSession: "vmp-parent-session", Actor: "mission-commander",
		testHooks: &hostTestHooks{runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 5151, nil
		}},
	}
	first, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ChildLaunched || first.Replay || launches != 1 || first.ReceiptSHA256 == "" || first.ObservationEventID == "" || first.TaskBindingSHA256 == "" || !first.ProfileRevoked || first.ProfileAlreadyManual {
		t.Fatalf("first authorized run = %+v launches=%d", first, launches)
	}
	profile, _, exists, err := autonomy.Read(fixture.caseRoot, "main")
	if err != nil || !exists || profile.Mode != autonomy.ModeManualGate {
		t.Fatalf("first run did not revoke profile: profile=%+v exists=%t err=%v", profile, exists, err)
	}
	opt.testHooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("terminal replay launched child")
	}
	second, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replay || second.ChildLaunched || launches != 1 || second.ReceiptSHA256 != first.ReceiptSHA256 || second.ObservationEventID != first.ObservationEventID || !second.ProfileAlreadyManual || second.ProfileRevoked || second.TaskBindingSHA256 != first.TaskBindingSHA256 {
		t.Fatalf("terminal replay = %+v first=%+v launches=%d", second, first, launches)
	}
}

func authorizedRunOptionsForFixture(
	fixture vmpAuthorizedFixture,
	hooks *hostTestHooks,
) AuthorizedRunOptions {
	return AuthorizedRunOptions{
		RepoRoot: fixture.repoRoot, CaseRoot: fixture.caseRoot,
		Pack: "vmp-re", GateEventID: fixture.gateEventID,
		ExecutionReportPath: "workspace/main/ida/session-1/adapter-report.json",
		AdapterSession:      "vmp-parent-session", Actor: "mission-commander",
		testHooks: hooks,
	}
}

func TestRunAuthorizedGateTerminalizesPostLaunchAuthorizationDrift(t *testing.T) {
	for _, phase := range []string{
		authorizationPhasePreExecution,
		authorizationPhasePrePublication,
		authorizationPhasePostPublication,
	} {
		t.Run(phase, func(t *testing.T) {
			fixture := newVMPAuthorizedFixture(t, false)
			launches := 0
			profileChanged := false
			hooks := &hostTestHooks{
				runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
					launches++
					return strictChildBytes(t, child), 5656, nil
				},
				beforeVMPAuthorizationCurrentness: func(current string) error {
					if current != phase || profileChanged {
						return nil
					}
					profileChanged = true
					profile := autonomy.DefaultProfile("main")
					data, err := json.MarshalIndent(profile, "", "  ")
					if err != nil {
						return err
					}
					return os.WriteFile(
						filepath.Join(fixture.caseRoot, ".rekit", "lanes", "main", "autonomy.json"),
						append(data, '\n'),
						0o600,
					)
				},
			}
			result, err := RunAuthorizedGate(authorizedRunOptionsForFixture(fixture, hooks))
			if phase == authorizationPhasePreExecution {
				if err == nil || launches != 0 || result.ObservationEventID != "" || result.ReceiptSHA256 != "" {
					t.Fatalf("pre-execution drift must fail before launch: result=%+v launches=%d err=%v", result, launches, err)
				}
				assertHostFileMissing(t, filepath.Join(fixture.caseRoot, "workspace", "main", "ida", "session-1", "adapter-report.json"))
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if launches != 1 || result.ExecutionStatus != "failed" || result.ExecutionExitStatus != "authorization-drift" || result.ObservationEventID == "" || result.ReceiptSHA256 == "" || result.TaskBindingSHA256 != "" || !result.ProfileAlreadyManual || result.ProfileRevoked {
				t.Fatalf("post-launch drift omitted terminal evidence or preserved profile: result=%+v launches=%d", result, launches)
			}
			assertHostFileMissing(t, filepath.Join(fixture.caseRoot, "workspace", "main", "ida", "session-1", vmpIDAIndexPacketFileName))
		})
	}
}

func TestRunAuthorizedGateTerminalizesUnsealedRecoveryAuthorizationDrift(t *testing.T) {
	for _, phase := range []string{
		authorizationPhasePrePublication,
		authorizationPhasePostPublication,
	} {
		t.Run(phase, func(t *testing.T) {
			fixture := newVMPAuthorizedFixture(t, false)
			launches := 0
			hooks := &hostTestHooks{
				runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
					launches++
					return strictChildBytes(t, child), 5666, nil
				},
				afterVMPStageCommit: func() error {
					return errors.New("stop after unsealed output commit")
				},
			}
			opt := authorizedRunOptionsForFixture(fixture, hooks)
			if _, err := RunAuthorizedGate(opt); err == nil || !strings.Contains(err.Error(), "stop after unsealed output commit") {
				t.Fatalf("unsealed recovery setup error=%v", err)
			}
			hooks.afterVMPStageCommit = nil
			profileChanged := false
			hooks.beforeVMPAuthorizationCurrentness = func(current string) error {
				if current != phase || profileChanged {
					return nil
				}
				profileChanged = true
				profile := autonomy.DefaultProfile("main")
				data, err := json.MarshalIndent(profile, "", "  ")
				if err != nil {
					return err
				}
				return os.WriteFile(
					filepath.Join(fixture.caseRoot, ".rekit", "lanes", "main", "autonomy.json"),
					append(data, '\n'),
					0o600,
				)
			}
			hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
				launches++
				return nil, 0, errors.New("unsealed authorization recovery relaunched child")
			}
			result, err := RunAuthorizedGate(opt)
			if err != nil {
				t.Fatal(err)
			}
			if launches != 1 || !result.Replay || result.ChildLaunched || result.ExecutionStatus != "failed" || result.ExecutionExitStatus != "authorization-drift" || result.ObservationEventID == "" || result.ReceiptSHA256 == "" || result.TaskBindingSHA256 != "" || !result.ProfileAlreadyManual || result.ProfileRevoked {
				t.Fatalf("unsealed recovery drift omitted terminal evidence: result=%+v launches=%d", result, launches)
			}
			assertHostFileMissing(t, filepath.Join(fixture.caseRoot, "workspace", "main", "ida", "session-1", vmpIDAIndexPacketFileName))
		})
	}
}

func TestRunAuthorizedGatePreservesSameBytesReplacementDuringCleanup(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	var replacement os.FileInfo
	packetPath := filepath.Join(fixture.caseRoot, "workspace", "main", "ida", "session-1", vmpIDAIndexPacketFileName)
	hooks := &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 5757, nil
		},
		beforeVMPSuccessSeal: func() error {
			packet, err := os.ReadFile(packetPath)
			if err != nil {
				return err
			}
			if err := os.Rename(packetPath, packetPath+".original"); err != nil {
				return err
			}
			if err := os.WriteFile(packetPath, packet, 0o600); err != nil {
				return err
			}
			replacement, err = os.Lstat(packetPath)
			if err != nil {
				return err
			}
			return os.WriteFile(
				filepath.Join(fixture.caseRoot, filepath.FromSlash(VMPIDAIndexDefaultExportRoot), "function_index.tsv"),
				[]byte("rva\tname\n0x6000\treplacement_drift\n"),
				0o600,
			)
		},
	}
	opt := authorizedRunOptionsForFixture(fixture, hooks)
	result, err := RunAuthorizedGate(opt)
	if err == nil || !strings.Contains(err.Error(), "owned output identity") {
		t.Fatalf("same-bytes replacement cleanup was not rejected: result=%+v err=%v", result, err)
	}
	current, statErr := os.Lstat(packetPath)
	if statErr != nil || replacement == nil || !os.SameFile(replacement, current) {
		t.Fatalf("same-bytes replacement was removed or displaced: replacement=%v current=%v err=%v", replacement, current, statErr)
	}
	if launches != 1 {
		t.Fatalf("initial replacement scenario launches=%d, want 1", launches)
	}

	hooks.beforeVMPSuccessSeal = nil
	hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("ownership-conflict recovery relaunched child")
	}
	if _, err := RunAuthorizedGate(opt); err == nil || !errors.Is(err, errVMPIDARecoveryOutputOwnership) {
		t.Fatalf("ownership-conflict recovery did not fail closed: %v", err)
	}
	current, statErr = os.Lstat(packetPath)
	if statErr != nil || !os.SameFile(replacement, current) || launches != 1 {
		t.Fatalf("ownership-conflict replay changed replacement or relaunched: current=%v err=%v launches=%d", current, statErr, launches)
	}
}

func vmpIDAExecutionReceiptPath(fixture vmpAuthorizedFixture) string {
	return filepath.Join(
		fixture.caseRoot,
		".rekit", "lanes", "main", "adapter-executions",
		fixture.gateEventID, "receipt.json",
	)
}

func vmpIDAExitError(t *testing.T, code int) error {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestVMPIDAExitHelper$")
	cmd.Env = append(os.Environ(), "REKIT_VMP_IDA_EXIT_HELPER="+strconv.Itoa(code))
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != code {
		t.Fatalf("helper exit error=%v code=%d", err, code)
	}
	return err
}

func TestVMPIDAExitHelper(t *testing.T) {
	value := os.Getenv("REKIT_VMP_IDA_EXIT_HELPER")
	if value == "" {
		return
	}
	code, err := strconv.Atoi(value)
	if err != nil || code < 1 {
		t.Fatalf("invalid helper exit code %q", value)
	}
	os.Exit(code)
}

func TestRunAuthorizedGateRejectsFailureReportWithoutParentLaunchProof(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, true)
	opt := authorizedRunOptionsForFixture(fixture, &hostTestHooks{})
	opt.AdapterSession = "vmp-session-1"
	dispatch, dispatchPath, dispatchSHA, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		fixture.repoRoot, fixture.caseRoot, "vmp-re", fixture.gateEventID,
	)
	if err != nil {
		t.Fatal(err)
	}
	report := gate.AdapterReport{
		SchemaVersion: 1, Kind: "adapter-execution-report",
		AdapterID: VMPIDAIndexAdapterID, Action: "inspect", Status: "failed",
		GateEventID: fixture.gateEventID,
		Dispatch: &adapterexecution.ReportDispatchBinding{
			DispatchID: dispatch.DispatchID, Path: dispatchPath, SHA256: dispatchSHA,
		},
		ActualBudget: autonomy.Budget{RuntimeSeconds: 1, Requests: 1},
		Escalation:   vmpIDAFailureEscalation("child-exit-7", strings.Repeat("a", 64)),
		Summary:      "forged failure report",
	}
	data, err := canonicalJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	writeHostFile(t, filepath.Join(fixture.caseRoot, filepath.FromSlash(dispatch.ReportPath)), string(data))
	launches := 0
	opt.testHooks = &hostTestHooks{runVMPIDAChild: func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("must not launch after forged report")
	}}
	result, err := RunAuthorizedGate(opt)
	if err == nil || !strings.Contains(err.Error(), "child launch proof") {
		t.Fatalf("forged terminal report result=%+v err=%v", result, err)
	}
	if launches != 0 || result.ReceiptSHA256 != "" || result.ObservationEventID != "" || result.ProfileRevoked {
		t.Fatalf("forged report caused lifecycle side effects: result=%+v launches=%d", result, launches)
	}
	assertHostFileMissing(t, vmpIDAExecutionReceiptPath(fixture))
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, ".rekit", "facts", "observations.jsonl"))
	profile, _, exists, err := autonomy.Read(fixture.caseRoot, "main")
	if err != nil || !exists || profile.Mode != autonomy.ModePreauthorized {
		t.Fatalf("forged report changed profile: profile=%+v exists=%t err=%v", profile, exists, err)
	}
}

func TestRunAuthorizedGateClosesLaunchProofWithoutTerminalReport(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	stop := true
	hooks := &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 7171, nil
		},
		afterVMPChildLaunch: func(int) error {
			if stop {
				return errors.New("stop after child launch proof")
			}
			return nil
		},
	}
	opt := authorizedRunOptionsForFixture(fixture, hooks)
	if _, err := RunAuthorizedGate(opt); err == nil || !strings.Contains(err.Error(), "stop after child launch proof") {
		t.Fatalf("launch proof cutpoint error=%v", err)
	}
	stop = false
	hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("orphan launch proof relaunched child")
	}
	result, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || result.ChildLaunched || !result.Replay ||
		result.ExecutionStatus != "aborted" ||
		result.ExecutionExitStatus != "parent-interrupted" ||
		result.ReceiptSHA256 == "" || result.ObservationEventID == "" ||
		result.TaskBindingSHA256 != "" || !result.ProfileRevoked {
		t.Fatalf("orphan launch proof was not closed exactly once: result=%+v launches=%d", result, launches)
	}
	second, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || !second.Replay || second.ChildLaunched ||
		second.ExecutionStatus != "aborted" ||
		second.ExecutionExitStatus != "parent-interrupted" ||
		second.ReceiptSHA256 != result.ReceiptSHA256 ||
		second.ObservationEventID != result.ObservationEventID ||
		!second.ProfileAlreadyManual {
		t.Fatalf("closed orphan attempt did not replay: first=%+v second=%+v launches=%d", result, second, launches)
	}
}

func TestRunAuthorizedGateClosesOrphanAfterOwnerAndCatalogDrift(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	stop := true
	hooks := &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 7272, nil
		},
		afterVMPChildLaunch: func(int) error {
			if stop {
				return errors.New("stop after child launch proof")
			}
			return nil
		},
	}
	opt := authorizedRunOptionsForFixture(fixture, hooks)
	if _, err := RunAuthorizedGate(opt); err == nil || !strings.Contains(err.Error(), "stop after child launch proof") {
		t.Fatalf("launch proof cutpoint error=%v", err)
	}
	stop = false
	writeHostFile(t, filepath.Join(fixture.caseRoot, ".rekit", "board.json"), `{"lanes":[{"id":"main","status":"open","workspace":"workspace/main","currentExecutor":"executor-replacement","executorGeneration":2}]}`)
	writeHostFile(t, filepath.Join(fixture.caseRoot, ".rekit", "lanes", "main", "lane.json"), `{
  "schemaVersion": 1,
  "id": "main",
  "type": "main",
  "name": "main",
  "title": "Main",
  "status": "open",
  "authority": false,
  "workspace": "workspace/main",
  "canWrite": ["own-workspace"],
  "readOnly": [".rekit/facts/**"],
  "outputs": ["observation"],
  "counters": {},
  "currentExecutor": "executor-replacement",
  "executorGeneration": 2,
  "createdAt": "2026-08-10T00:00:00Z",
  "updatedAt": "2026-08-10T00:00:00Z"
}`)
	catalogPath := filepath.Join(fixture.repoRoot, "packs", "vmp-re", "tooling", "catalog.yml")
	catalog, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalogPath, bytes.Replace(catalog, []byte("Inspect fixed existing IDA TSV indexes."), []byte("Replacement catalog purpose."), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("stale orphan recovery relaunched child")
	}
	result, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || result.ChildLaunched || !result.Replay ||
		result.ExecutionStatus != "aborted" ||
		result.ExecutionExitStatus != "parent-interrupted" ||
		result.ReceiptSHA256 == "" || result.ObservationEventID == "" ||
		result.TaskBindingSHA256 != "" || !result.ProfileRevoked ||
		result.ProfileAlreadyManual {
		t.Fatalf("stale orphan attempt was not closed without new authority: result=%+v launches=%d", result, launches)
	}
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, ".rekit", "facts", "authority.jsonl"))
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, ".rekit", "facts", "confirmed.jsonl"))
}

func TestRunAuthorizedGateRecordsChildTerminalFailuresAndReplaysWithoutChild(t *testing.T) {
	for name, childResult := range map[string]struct {
		stdout     []byte
		err        error
		wantStatus string
		wantExit   string
	}{
		"timeout": {
			err:        errContainedProcessTimeout,
			wantStatus: "aborted",
			wantExit:   "child-timeout",
		},
		"nonzero": {
			err:        vmpIDAExitError(t, 7),
			wantStatus: "failed",
			wantExit:   "child-exit-7",
		},
		"invalid stdout": {
			stdout:     []byte(`{"schemaVersion":1}`),
			wantStatus: "failed",
			wantExit:   "child-invalid-stdout",
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newVMPAuthorizedFixture(t, false)
			launches := 0
			opt := AuthorizedRunOptions{
				RepoRoot: fixture.repoRoot, CaseRoot: fixture.caseRoot, Pack: "vmp-re", GateEventID: fixture.gateEventID,
				ExecutionReportPath: "workspace/main/ida/session-1/adapter-report.json", AdapterSession: "vmp-parent-session", Actor: "mission-commander",
				testHooks: &hostTestHooks{runVMPIDAChild: func(VMPIDAIndexChildOptions) ([]byte, int, error) {
					launches++
					return childResult.stdout, 6464, childResult.err
				}},
			}
			first, err := RunAuthorizedGate(opt)
			if err != nil {
				t.Fatal(err)
			}
			if launches != 1 || !first.ChildLaunched || first.Replay || first.ExecutionStatus != childResult.wantStatus || first.ExecutionExitStatus != childResult.wantExit || first.ReportSHA256 == "" || first.PacketPath != "" || first.PacketSHA256 != "" || first.ReceiptSHA256 == "" || first.ObservationEventID == "" || first.TaskBindingSHA256 != "" || !first.ProfileRevoked {
				t.Fatalf("terminal failure result=%+v launches=%d", first, launches)
			}
			if _, err := os.Stat(filepath.Join(fixture.caseRoot, "workspace", "main", "ida", "session-1", vmpIDAIndexPacketFileName)); !os.IsNotExist(err) {
				t.Fatalf("terminal failure published a packet: %v", err)
			}
			if _, err := os.Stat(filepath.Join(fixture.caseRoot, "workspace", "main", "ida", "session-1", vmpIDAOutputCommitFileName)); !os.IsNotExist(err) {
				t.Fatalf("terminal failure published a success output commit: %v", err)
			}
			receiptData, err := os.ReadFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(first.ReceiptPath)))
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := adapterexecution.Decode(receiptData)
			if err != nil || receipt.Execution.Outcome != childResult.wantStatus || receipt.Execution.ExitStatus != childResult.wantExit || len(receipt.Artifacts) != 0 {
				t.Fatalf("terminal failure receipt=%+v err=%v", receipt, err)
			}
			opt.testHooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
				launches++
				return nil, 0, errors.New("terminal replay relaunched child")
			}
			second, err := RunAuthorizedGate(opt)
			if err != nil {
				t.Fatal(err)
			}
			if launches != 1 || !second.Replay || second.ChildLaunched || second.ExecutionStatus != first.ExecutionStatus || second.ExecutionExitStatus != first.ExecutionExitStatus || second.ReceiptSHA256 != first.ReceiptSHA256 || second.ObservationEventID != first.ObservationEventID || !second.ProfileAlreadyManual {
				t.Fatalf("terminal failure replay=%+v first=%+v launches=%d", second, first, launches)
			}
		})
	}
}

func TestRunAuthorizedGateResumesFailureReportWithoutLosingExitStatus(t *testing.T) {
	for name, childResult := range map[string]struct {
		stdout     []byte
		err        error
		wantStatus string
		wantExit   string
	}{
		"timeout": {
			err:        errContainedProcessTimeout,
			wantStatus: "aborted",
			wantExit:   "child-timeout",
		},
		"nonzero": {
			err:        vmpIDAExitError(t, 9),
			wantStatus: "failed",
			wantExit:   "child-exit-9",
		},
		"invalid stdout": {
			stdout:     []byte(`{"schemaVersion":1}`),
			wantStatus: "failed",
			wantExit:   "child-invalid-stdout",
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newVMPAuthorizedFixture(t, false)
			launches := 0
			stop := true
			hooks := &hostTestHooks{
				runVMPIDAChild: func(VMPIDAIndexChildOptions) ([]byte, int, error) {
					launches++
					return childResult.stdout, 6565, childResult.err
				},
				afterVMPOutputsPublished: func() error {
					if stop {
						return errors.New("stop after failure report")
					}
					return nil
				},
			}
			opt := AuthorizedRunOptions{
				RepoRoot: fixture.repoRoot, CaseRoot: fixture.caseRoot, Pack: "vmp-re", GateEventID: fixture.gateEventID,
				ExecutionReportPath: "workspace/main/ida/session-1/adapter-report.json", AdapterSession: "vmp-parent-session", Actor: "mission-commander",
				testHooks: hooks,
			}
			first, err := RunAuthorizedGate(opt)
			if err == nil || !strings.Contains(err.Error(), "stop after failure report") || first.ExecutionExitStatus != childResult.wantExit {
				t.Fatalf("failure cutpoint result=%+v err=%v", first, err)
			}
			stop = false
			hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
				launches++
				return nil, 0, errors.New("failure recovery relaunched child")
			}
			result, err := RunAuthorizedGate(opt)
			if err != nil {
				t.Fatal(err)
			}
			if launches != 1 || !result.Replay || result.ChildLaunched || result.ExecutionStatus != childResult.wantStatus || result.ExecutionExitStatus != childResult.wantExit || result.ReceiptSHA256 == "" || result.ObservationEventID == "" || result.TaskBindingSHA256 != "" || !result.ProfileRevoked {
				t.Fatalf("failure recovery result=%+v launches=%d", result, launches)
			}
		})
	}
}

func TestRunAuthorizedGateRejectsReceiptExitStatusDifferentFromReport(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	stop := true
	hooks := &hostTestHooks{
		runVMPIDAChild: func(VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return nil, 6767, vmpIDAExitError(t, 7)
		},
		afterVMPOutputsPublished: func() error {
			if stop {
				return errors.New("stop after failure report")
			}
			return nil
		},
	}
	opt := authorizedRunOptionsForFixture(fixture, hooks)
	if _, err := RunAuthorizedGate(opt); err == nil || !strings.Contains(err.Error(), "stop after failure report") {
		t.Fatalf("failure report cutpoint error=%v", err)
	}
	dispatch, _, _, _, err := gate.ReadCurrentAdapterExecutionDispatch(
		fixture.repoRoot, fixture.caseRoot, "vmp-re", fixture.gateEventID,
	)
	if err != nil {
		t.Fatal(err)
	}
	receiptOpt := gate.Options{
		GateEventID:                fixture.gateEventID,
		ExecutionReportPath:        dispatch.ReportPath,
		AdapterID:                  VMPIDAIndexAdapterID,
		Executor:                   dispatch.Owner.CurrentExecutor,
		ExpectedExecutorGeneration: dispatch.Owner.ExecutorGeneration,
		AdapterHarness:             adapterHarness,
		AdapterSession:             dispatch.Owner.AdapterSession,
		ExecutionExitStatus:        "child-exit-9",
		Actor:                      "mission-commander",
	}
	preview, err := gate.RecordAdapterExecutionReceipt(
		fixture.repoRoot, fixture.caseRoot, "vmp-re", receiptOpt,
	)
	if err != nil {
		t.Fatal(err)
	}
	receiptOpt.ExpectedAdapterExecutionBindingSHA256 = preview.BindingSHA256
	if _, err := gate.RecordAdapterExecutionReceipt(
		fixture.repoRoot, fixture.caseRoot, "vmp-re", receiptOpt,
	); err != nil {
		t.Fatal(err)
	}
	stop = false
	hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("receipt mismatch relaunched child")
	}
	result, err := RunAuthorizedGate(opt)
	if err == nil || !strings.Contains(err.Error(), "differs from the terminal report") {
		t.Fatalf("receipt mismatch result=%+v err=%v", result, err)
	}
	if launches != 1 || result.ObservationEventID != "" || result.ProfileRevoked {
		t.Fatalf("receipt mismatch caused terminal side effects: result=%+v launches=%d", result, launches)
	}
	assertHostFileMissing(t, filepath.Join(fixture.caseRoot, ".rekit", "facts", "observations.jsonl"))
	profile, _, exists, err := autonomy.Read(fixture.caseRoot, "main")
	if err != nil || !exists || profile.Mode != autonomy.ModePreauthorized {
		t.Fatalf("receipt mismatch changed profile: profile=%+v exists=%t err=%v", profile, exists, err)
	}
}

func TestRunAuthorizedGateResumesEvidenceLifecycleWithoutRelaunchingChild(t *testing.T) {
	for name, hook := range map[string]func(*hostTestHooks){
		"after stage commit": func(hooks *hostTestHooks) {
			hooks.afterVMPStageCommit = func() error {
				return errors.New("stop after stage commit")
			}
		},
		"during committed publication": func(hooks *hostTestHooks) {
			hooks.beforeReportWrite = func() error {
				return errors.New("stop after committed packet publication")
			}
		},
		"after output commit": func(hooks *hostTestHooks) {
			hooks.afterVMPOutputCommit = func() error {
				return errors.New("stop after output commit")
			}
		},
		"after outputs": func(hooks *hostTestHooks) {
			hooks.afterVMPOutputsPublished = func() error {
				return errors.New("stop after outputs")
			}
		},
		"after receipt": func(hooks *hostTestHooks) {
			hooks.afterVMPReceiptRecorded = func() error {
				return errors.New("stop after receipt")
			}
		},
		"after observation": func(hooks *hostTestHooks) {
			hooks.afterVMPObservation = func() error {
				return errors.New("stop after observation")
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newVMPAuthorizedFixture(t, false)
			launches := 0
			hooks := &hostTestHooks{
				runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
					launches++
					return strictChildBytes(t, child), 5353, nil
				},
			}
			hook(hooks)
			opt := AuthorizedRunOptions{
				RepoRoot:            fixture.repoRoot,
				CaseRoot:            fixture.caseRoot,
				Pack:                "vmp-re",
				GateEventID:         fixture.gateEventID,
				ExecutionReportPath: "workspace/main/ida/session-1/adapter-report.json",
				AdapterSession:      "vmp-parent-session",
				Actor:               "mission-commander",
				testHooks:           hooks,
			}
			if _, err := RunAuthorizedGate(opt); err == nil || !strings.Contains(err.Error(), "stop after") {
				t.Fatalf("lifecycle cutpoint was not exercised: %v", err)
			}
			hooks.afterVMPStageCommit = nil
			hooks.beforeReportWrite = nil
			hooks.afterVMPOutputCommit = nil
			hooks.afterVMPOutputsPublished = nil
			hooks.afterVMPReceiptRecorded = nil
			hooks.afterVMPObservation = nil
			hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
				launches++
				return nil, 0, errors.New("recovery relaunched child")
			}
			result, err := RunAuthorizedGate(opt)
			if err != nil {
				t.Fatal(err)
			}
			if launches != 1 || !result.Replay || result.ChildLaunched ||
				result.ReceiptSHA256 == "" || result.ObservationEventID == "" ||
				result.TaskBindingSHA256 == "" || !result.ProfileRevoked {
				t.Fatalf("recovery result=%+v launches=%d", result, launches)
			}
		})
	}
}

func TestRunAuthorizedGateRejectsTakeoverAfterObservation(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	opt := AuthorizedRunOptions{
		RepoRoot: fixture.repoRoot, CaseRoot: fixture.caseRoot, Pack: "vmp-re", GateEventID: fixture.gateEventID,
		ExecutionReportPath: "workspace/main/ida/session-1/adapter-report.json", AdapterSession: "vmp-parent-session", Actor: "mission-commander",
		testHooks: &hostTestHooks{
			runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
				launches++
				return strictChildBytes(t, child), 5454, nil
			},
			afterVMPObservation: func() error {
				writeHostFile(
					t,
					filepath.Join(fixture.caseRoot, ".rekit", "board.json"),
					`{"lanes":[{"id":"main","status":"open","workspace":"workspace/main","currentExecutor":"executor-replacement","executorGeneration":2}]}`,
				)
				return nil
			},
		},
	}
	result, err := RunAuthorizedGate(opt)
	if err == nil || !strings.Contains(err.Error(), "task binding owner changed") {
		t.Fatalf("takeover after observation did not fail closed: result=%+v err=%v", result, err)
	}
	if launches != 1 || result.ObservationEventID == "" || result.TaskBindingSHA256 != "" || result.ProfileRevoked {
		t.Fatalf("takeover result=%+v launches=%d", result, launches)
	}
	if _, statErr := os.Stat(filepath.Join(
		fixture.caseRoot,
		".rekit",
		"lanes",
		"main",
		"member-task-bindings",
		"g000002.json",
	)); !os.IsNotExist(statErr) {
		t.Fatalf("old execution evidence was rebound to replacement owner: %v", statErr)
	}
}

func TestRunAuthorizedGateRevokeFailureKeepsTerminalReplayable(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	profilePath := filepath.Join(fixture.caseRoot, ".rekit", "lanes", "main", "autonomy.json")
	opt := AuthorizedRunOptions{
		RepoRoot: fixture.repoRoot, CaseRoot: fixture.caseRoot, Pack: "vmp-re", GateEventID: fixture.gateEventID,
		ExecutionReportPath: "workspace/main/ida/session-1/adapter-report.json", AdapterSession: "vmp-parent-session", Actor: "mission-commander",
		testHooks: &hostTestHooks{
			runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
				launches++
				return strictChildBytes(t, child), 5252, nil
			},
			beforeVMPProfileRevoke: func() error {
				return os.WriteFile(profilePath, []byte(vmpProfileJSON(fixture.requestPath, "workspace/main/ida/session-1", "2999-01-01T00:00:00Z", "replacement-user")), 0o600)
			},
		},
	}
	first, err := RunAuthorizedGate(opt)
	if err == nil || !strings.Contains(err.Error(), "changed immediately before exact revoke") {
		t.Fatalf("revoke drift error = %v result=%+v", err, first)
	}
	if launches != 1 || first.ObservationEventID == "" || first.TaskBindingSHA256 == "" || first.ProfileRevoked {
		t.Fatalf("revoke drift lost terminal evidence: result=%+v launches=%d", first, launches)
	}
	opt.testHooks.beforeVMPProfileRevoke = nil
	opt.testHooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("recovery replay launched child")
	}
	if _, err := RunAuthorizedGate(opt); err == nil || !strings.Contains(err.Error(), "changed after execution observation") {
		t.Fatalf("recovery accepted replacement profile: %v", err)
	}
	if launches != 1 {
		t.Fatalf("revoke recovery replay launches = %d, want 1", launches)
	}
}

func TestRunAuthorizedGateRejectsReceiptBoundPacketTampering(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	opt := AuthorizedRunOptions{
		RepoRoot:            fixture.repoRoot,
		CaseRoot:            fixture.caseRoot,
		Pack:                "vmp-re",
		GateEventID:         fixture.gateEventID,
		ExecutionReportPath: "workspace/main/ida/session-1/adapter-report.json",
		AdapterSession:      "vmp-parent-session",
		Actor:               "mission-commander",
		testHooks: &hostTestHooks{
			runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
				launches++
				return strictChildBytes(t, child), 5454, nil
			},
			afterVMPReceiptRecorded: func() error {
				return errors.New("stop after receipt")
			},
		},
	}
	first, err := RunAuthorizedGate(opt)
	if err == nil || !strings.Contains(err.Error(), "stop after receipt") {
		t.Fatalf("receipt cutpoint was not exercised: result=%+v err=%v", first, err)
	}
	if first.PacketPath == "" || first.ReceiptSHA256 == "" {
		t.Fatalf("receipt cutpoint omitted bindings: %+v", first)
	}
	packetPath := filepath.Join(fixture.caseRoot, filepath.FromSlash(first.PacketPath))
	packet, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	packet = bytes.Replace(packet, []byte("needle_dispatch"), []byte("tamper_dispatch"), 1)
	if err := os.WriteFile(packetPath, packet, 0o600); err != nil {
		t.Fatal(err)
	}
	opt.testHooks.afterVMPReceiptRecorded = nil
	opt.testHooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("tamper recovery relaunched child")
	}
	if _, err := RunAuthorizedGate(opt); err == nil || !strings.Contains(strings.ToLower(err.Error()), "receipt") {
		t.Fatalf("receipt-bound packet tampering was accepted: %v", err)
	}
	if launches != 1 {
		t.Fatalf("tamper recovery launches=%d, want 1", launches)
	}
}

func TestRunVMPIDAIndexRejectsTamperedOutputCommitWithoutRelaunch(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, true)
	launches := 0
	fixture.options.testHooks = &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 6262, nil
		},
		afterVMPStageCommit: func() error {
			return errors.New("stop after stage commit")
		},
	}
	if _, err := Run(fixture.options); err == nil || !strings.Contains(err.Error(), "stop after stage commit") {
		t.Fatalf("stage cutpoint error = %v", err)
	}
	commitPath := filepath.Join(
		fixture.caseRoot,
		"workspace",
		"main",
		"ida",
		"session-1",
		vmpIDAOutputCommitFileName,
	)
	data, err := os.ReadFile(commitPath)
	if err != nil {
		t.Fatal(err)
	}
	var commit vmpIDAOutputCommit
	if err := decodeVMPIDAStrictJSON(data, &commit); err != nil {
		t.Fatal(err)
	}
	commit.PacketBytes = bytes.Replace(
		commit.PacketBytes,
		[]byte("needle_dispatch"),
		[]byte("tamper_dispatch"),
		1,
	)
	data, err = canonicalJSON(commit)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(commitPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.options.testHooks.afterVMPStageCommit = nil
	fixture.options.testHooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("tampered commit recovery relaunched child")
	}
	if _, err := Run(fixture.options); err == nil || !strings.Contains(strings.ToLower(err.Error()), "commit") {
		t.Fatalf("tampered output commit was accepted: %v", err)
	}
	if launches != 1 {
		t.Fatalf("tampered commit recovery launches=%d, want 1", launches)
	}
}

func TestRunAuthorizedGateRecoversSealedSuccessAfterSourceAndProfileChange(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	hooks := &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 6767, nil
		},
		afterVMPOutputCommit: func() error {
			return errors.New("stop after sealed output publication")
		},
	}
	opt := authorizedRunOptionsForFixture(fixture, hooks)
	if _, err := RunAuthorizedGate(opt); err == nil || !strings.Contains(err.Error(), "sealed output publication") {
		t.Fatalf("sealed publication cutpoint error=%v", err)
	}
	writeHostFile(
		t,
		filepath.Join(fixture.caseRoot, ".rekit", "lanes", "main", "autonomy.json"),
		`{"schemaVersion":1,"profileId":"manual-main","lane":"main","mode":"manual-gate","recordRequired":true,"notifyMainOn":["boundary-hit","new-risk","destructive-change","authority-write-needed"]}`,
	)
	if err := os.WriteFile(
		filepath.Join(fixture.caseRoot, filepath.FromSlash(VMPIDAIndexDefaultExportRoot), "function_index.tsv"),
		[]byte("rva\tname\n0x5000\tpost_seal_drift\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	hooks.afterVMPOutputCommit = nil
	hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("sealed recovery relaunched child")
	}
	result, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || !result.Replay || result.ChildLaunched || result.ExecutionStatus != "succeeded" || result.ExecutionExitStatus != "completed" || result.ObservationEventID == "" || result.TaskBindingSHA256 == "" || !result.ProfileAlreadyManual {
		t.Fatalf("sealed success recovery=%+v launches=%d", result, launches)
	}
}

func TestRunAuthorizedGateDoesNotRecoverUnsealedSuccessAfterSourceDrift(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	drifted := false
	hooks := &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 6868, nil
		},
		beforeVMPSuccessSeal: func() error {
			if drifted {
				return nil
			}
			drifted = true
			return os.WriteFile(
				filepath.Join(fixture.caseRoot, filepath.FromSlash(VMPIDAIndexDefaultExportRoot), "function_index.tsv"),
				[]byte("rva\tname\n0x3000\tlate_drift\n"),
				0o600,
			)
		},
	}
	opt := authorizedRunOptionsForFixture(fixture, hooks)
	first, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || first.ExecutionStatus != "failed" || first.ExecutionExitStatus != "source-drift" || first.ObservationEventID == "" || first.TaskBindingSHA256 != "" || !first.ProfileRevoked || first.ReceiptSHA256 == "" {
		t.Fatalf("late source drift did not close as typed failure: result=%+v launches=%d", first, launches)
	}
	assertHostFileMissing(t, filepath.Join(
		fixture.caseRoot, "workspace", "main", "ida", "session-1",
		vmpIDASuccessSealFileName,
	))
	hooks.beforeVMPSuccessSeal = nil
	hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("unsealed drift recovery relaunched child")
	}
	second, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || !second.Replay || second.ChildLaunched || second.ExecutionStatus != "failed" || second.ExecutionExitStatus != "source-drift" || second.ObservationEventID != first.ObservationEventID || second.ReceiptSHA256 != first.ReceiptSHA256 || second.TaskBindingSHA256 != "" || !second.ProfileAlreadyManual {
		t.Fatalf("late source drift replay=%+v first=%+v launches=%d", second, first, launches)
	}
}

func TestRunAuthorizedGateRejectsSourceDriftAtUnsealedRecoverySealCutpoint(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	hooks := &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 6968, nil
		},
		afterVMPStageCommit: func() error {
			return errors.New("stop before success seal")
		},
	}
	opt := authorizedRunOptionsForFixture(fixture, hooks)
	if _, err := RunAuthorizedGate(opt); err == nil || !strings.Contains(err.Error(), "stop before success seal") {
		t.Fatalf("unsealed stage cutpoint error=%v", err)
	}
	hooks.afterVMPStageCommit = nil
	hooks.beforeVMPSuccessSeal = func() error {
		return os.WriteFile(
			filepath.Join(fixture.caseRoot, filepath.FromSlash(VMPIDAIndexDefaultExportRoot), "function_index.tsv"),
			[]byte("rva\tname\n0x4000\trecovery_drift\n"),
			0o600,
		)
	}
	hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("unsealed recovery relaunched child")
	}
	result, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || result.ExecutionStatus != "failed" || result.ExecutionExitStatus != "source-drift" || result.ObservationEventID == "" || result.TaskBindingSHA256 != "" || !result.ProfileRevoked {
		t.Fatalf("unsealed recovery seal-cutpoint drift did not close as typed failure: result=%+v launches=%d", result, launches)
	}
	assertHostFileMissing(t, filepath.Join(
		fixture.caseRoot, "workspace", "main", "ida", "session-1",
		vmpIDASuccessSealFileName,
	))
	if result.ReceiptSHA256 == "" {
		t.Fatalf("unsealed recovery drift omitted terminal receipt: %+v", result)
	}
}

func TestRunAuthorizedGatePersistsRuntimeOverrunAfterStageCommit(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	hooks := &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 6970, nil
		},
		afterVMPStageCommit: func() error {
			time.Sleep(10 * time.Second)
			return nil
		},
	}
	opt := authorizedRunOptionsForFixture(fixture, hooks)
	result, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || result.ExecutionStatus != "aborted" || result.ExecutionExitStatus != "runtime-budget-exceeded" || result.PacketPath != "" || result.ObservationEventID == "" || result.TaskBindingSHA256 != "" || !result.ProfileRevoked {
		t.Fatalf("post-stage runtime overrun result=%+v launches=%d", result, launches)
	}
	assertHostFileMissing(t, filepath.Join(
		fixture.caseRoot, "workspace", "main", "ida", "session-1",
		vmpIDASuccessSealFileName,
	))
	assertHostFileMissing(t, filepath.Join(
		fixture.caseRoot, "workspace", "main", "ida", "session-1",
		vmpIDAIndexPacketFileName,
	))
	hooks.afterVMPStageCommit = nil
	hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("runtime terminal replay relaunched child")
	}
	replay, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || !replay.Replay || replay.ChildLaunched || replay.ExecutionStatus != "aborted" || replay.ExecutionExitStatus != "runtime-budget-exceeded" || replay.ObservationEventID != result.ObservationEventID || !replay.ProfileAlreadyManual {
		t.Fatalf("post-stage runtime replay=%+v first=%+v launches=%d", replay, result, launches)
	}
}

func TestRunAuthorizedGatePersistsRuntimeOverrunAfterPublicOutputs(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, false)
	launches := 0
	hooks := &hostTestHooks{
		runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return strictChildBytes(t, child), 6971, nil
		},
		beforeVMPSuccessSeal: func() error {
			time.Sleep(10 * time.Second)
			return nil
		},
	}
	opt := authorizedRunOptionsForFixture(fixture, hooks)
	result, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || result.ExecutionStatus != "aborted" || result.ExecutionExitStatus != "runtime-budget-exceeded" || result.PacketPath != "" || result.ObservationEventID == "" || result.TaskBindingSHA256 != "" || !result.ProfileRevoked {
		t.Fatalf("post-publication runtime overrun result=%+v launches=%d", result, launches)
	}
	assertHostFileMissing(t, filepath.Join(
		fixture.caseRoot, "workspace", "main", "ida", "session-1",
		vmpIDASuccessSealFileName,
	))
	assertHostFileMissing(t, filepath.Join(
		fixture.caseRoot, "workspace", "main", "ida", "session-1",
		vmpIDAIndexPacketFileName,
	))
	hooks.beforeVMPSuccessSeal = nil
	hooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
		launches++
		return nil, 0, errors.New("post-publication runtime replay relaunched child")
	}
	replay, err := RunAuthorizedGate(opt)
	if err != nil {
		t.Fatal(err)
	}
	if launches != 1 || !replay.Replay || replay.ChildLaunched || replay.ExecutionStatus != "aborted" || replay.ExecutionExitStatus != "runtime-budget-exceeded" || replay.ObservationEventID != result.ObservationEventID || !replay.ProfileAlreadyManual {
		t.Fatalf("post-publication runtime replay=%+v first=%+v launches=%d", replay, result, launches)
	}
}

func TestRunVMPIDAIndexClassifiesInvalidPacketAndRuntimeOverrun(t *testing.T) {
	t.Run("invalid packet binding", func(t *testing.T) {
		fixture := newVMPAuthorizedFixture(t, false)
		launches := 0
		opt := authorizedRunOptionsForFixture(fixture, &hostTestHooks{
			runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
				launches++
				var result VMPIDAIndexChildResult
				if err := json.Unmarshal(strictChildBytes(t, child), &result); err != nil {
					t.Fatal(err)
				}
				result.PacketSHA256 = strings.Repeat("a", 64)
				data, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				return data, 6969, nil
			},
		})
		result, err := RunAuthorizedGate(opt)
		if err != nil {
			t.Fatal(err)
		}
		if launches != 1 || result.ExecutionStatus != "failed" || result.ExecutionExitStatus != "child-invalid-packet" {
			t.Fatalf("invalid packet result=%+v launches=%d", result, launches)
		}
	})

	t.Run("runtime overrun after normal child return", func(t *testing.T) {
		fixture := newVMPAuthorizedFixture(t, false)
		launches := 0
		opt := authorizedRunOptionsForFixture(fixture, &hostTestHooks{
			runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
				launches++
				time.Sleep(11 * time.Second)
				return strictChildBytes(t, child), 7070, nil
			},
		})
		result, err := RunAuthorizedGate(opt)
		if err != nil {
			t.Fatal(err)
		}
		if launches != 1 || result.ExecutionStatus != "aborted" || result.ExecutionExitStatus != "runtime-budget-exceeded" {
			t.Fatalf("runtime overrun result=%+v launches=%d", result, launches)
		}
	})
}

func TestRunVMPIDAIndexRejectsSourceDriftAndOutputCollision(t *testing.T) {
	t.Run("source drift", func(t *testing.T) {
		fixture := newVMPAuthorizedFixture(t, false)
		launches := 0
		drifted := false
		opt := AuthorizedRunOptions{
			RepoRoot: fixture.repoRoot, CaseRoot: fixture.caseRoot, Pack: "vmp-re", GateEventID: fixture.gateEventID,
			ExecutionReportPath: "workspace/main/ida/session-1/adapter-report.json", AdapterSession: "vmp-parent-session", Actor: "mission-commander",
			testHooks: &hostTestHooks{
				runVMPIDAChild: func(child VMPIDAIndexChildOptions) ([]byte, int, error) {
					launches++
					return strictChildBytes(t, child), 6161, nil
				},
				beforeVMPPublication: func() error {
					drifted = true
					return os.WriteFile(filepath.Join(fixture.caseRoot, filepath.FromSlash(VMPIDAIndexDefaultExportRoot), "function_index.tsv"), []byte("rva\tname\n0x2000\tdrift\n"), 0o600)
				},
			},
		}
		first, err := RunAuthorizedGate(opt)
		if err != nil {
			t.Fatal(err)
		}
		if !drifted || launches != 1 || first.ExecutionStatus != "failed" || first.ExecutionExitStatus != "source-drift" || first.PacketPath != "" || first.ReceiptSHA256 == "" || first.ObservationEventID == "" || first.TaskBindingSHA256 != "" || !first.ProfileRevoked {
			t.Fatalf("source drift terminal result=%+v launches=%d", first, launches)
		}
		assertHostFileMissing(t, filepath.Join(fixture.caseRoot, "workspace", "main", "ida", "session-1", vmpIDAIndexPacketFileName))
		opt.testHooks.beforeVMPPublication = nil
		opt.testHooks.runVMPIDAChild = func(VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return nil, 0, errors.New("source drift replay relaunched child")
		}
		second, err := RunAuthorizedGate(opt)
		if err != nil {
			t.Fatal(err)
		}
		if launches != 1 || !second.Replay || second.ChildLaunched || second.ExecutionExitStatus != "source-drift" || second.ReceiptSHA256 != first.ReceiptSHA256 || second.ObservationEventID != first.ObservationEventID || !second.ProfileAlreadyManual {
			t.Fatalf("source drift replay=%+v first=%+v launches=%d", second, first, launches)
		}
	})
	t.Run("output collision before child", func(t *testing.T) {
		fixture := newVMPAuthorizedFixture(t, true)
		packet := filepath.Join(fixture.caseRoot, "workspace", "main", "ida", "session-1", vmpIDAIndexPacketFileName)
		writeHostFile(t, packet, "competing\n")
		launches := 0
		fixture.options.testHooks = &hostTestHooks{runVMPIDAChild: func(VMPIDAIndexChildOptions) ([]byte, int, error) {
			launches++
			return nil, 0, nil
		}}
		if _, err := Run(fixture.options); err == nil || !strings.Contains(strings.ToLower(err.Error()), "partial") {
			t.Fatalf("output collision error = %v", err)
		}
		if launches != 0 {
			t.Fatalf("child launches = %d, want 0", launches)
		}
		data, err := os.ReadFile(packet)
		if err != nil || string(data) != "competing\n" {
			t.Fatalf("competing packet changed: %q err=%v", data, err)
		}
	})
}

func TestDecodeAuthorizedRunProcessResultRequiresStrictSingleJSON(t *testing.T) {
	valid, err := json.Marshal(AuthorizedRunResult{
		SchemaVersion: 1, Kind: "vmp-ida-index-authorized-run", Pack: "vmp-re",
		GateEventID: "evt-gate", AdapterID: VMPIDAIndexAdapterID, AdapterSession: "session-1",
		NoNetwork: true, NoNetworkBoundary: fixedChildNoNetworkCodepath, NoAuthority: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeAuthorizedRunProcessResult(valid); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"trailing object": append(append([]byte{}, valid...), []byte(` {}`)...),
		"unknown field":   []byte(strings.Replace(string(valid), `"schemaVersion":1`, `"schemaVersion":1,"unknown":true`, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeAuthorizedRunProcessResult(data); err == nil {
				t.Fatalf("strict authorized process decoder accepted %s", data)
			}
		})
	}
}

func TestRunVMPIDAIndexChildRequiresExactAuthorizedDispatchBinding(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, true)
	exact := childOptionsForFixture(t, fixture)
	if _, err := RunVMPIDAIndexChild(exact); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*VMPIDAIndexChildOptions){
		"missing dispatch": func(opt *VMPIDAIndexChildOptions) {
			opt.ExpectedDispatchSHA256 = ""
		},
		"wrong gate": func(opt *VMPIDAIndexChildOptions) {
			opt.GateEventID = "evt-wrong"
		},
		"wrong session": func(opt *VMPIDAIndexChildOptions) {
			opt.AdapterSession = "wrong-session"
		},
		"stale owner generation": func(opt *VMPIDAIndexChildOptions) {
			opt.ExpectedExecutorGeneration++
		},
	} {
		t.Run(name, func(t *testing.T) {
			opt := exact
			mutate(&opt)
			if _, err := RunVMPIDAIndexChild(opt); err == nil {
				t.Fatalf("private child accepted %s", name)
			}
		})
	}
}

func TestDecodeVMPIDAChildResultRequiresStrictSingleJSON(t *testing.T) {
	fixture := newVMPAuthorizedFixture(t, true)
	valid := strictChildBytes(t, childOptionsForFixture(t, fixture))
	if _, err := decodeVMPIDAChildResult(valid, fixture.requestPath); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"trailing object": append(append([]byte{}, valid...), []byte(` {}`)...),
		"unknown field":   []byte(strings.Replace(string(valid), `"schemaVersion":1`, `"schemaVersion":1,"unknown":true`, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeVMPIDAChildResult(data, fixture.requestPath); err == nil {
				t.Fatalf("strict child decoder accepted %s", data)
			}
		})
	}
}
