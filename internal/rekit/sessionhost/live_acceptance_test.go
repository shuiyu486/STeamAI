package sessionhost

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestLiveAcceptanceEvidenceIsPublishedOnlyForCorrection(t *testing.T) {
	caseRoot := t.TempDir()
	goal := liveAcceptanceBoundGoal("Inspect one harmless feature note.")
	correction := liveAcceptanceBoundCorrection("Revise the analysis.")
	attachedGoal := liveAcceptanceAttachedGoal()
	if !strings.Contains(goal, liveAcceptanceEvidencePath) || !strings.Contains(goal, "Acceptance requires reading and citing") || !strings.Contains(goal, "mandatory acceptance requirement is unmet") {
		t.Fatalf("goal did not bind the intentional initial evidence gap: %s", goal)
	}
	if !strings.Contains(correction, liveAcceptanceEvidencePath) || !strings.Contains(correction, "newly published bounded case-local evidence") {
		t.Fatalf("correction did not bind the published evidence: %s", correction)
	}
	if !strings.Contains(attachedGoal, liveAcceptanceEvidencePath) || !strings.Contains(attachedGoal, "already published") || strings.Contains(attachedGoal, "mandatory acceptance requirement is unmet") {
		t.Fatalf("attached goal did not bind the pre-published evidence: %s", attachedGoal)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, filepath.FromSlash(liveAcceptanceEvidencePath))); !os.IsNotExist(err) {
		t.Fatalf("evidence existed before correction publication: %v", err)
	}
	evidence, err := publishLiveAcceptanceEvidence(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.Path != liveAcceptanceEvidencePath || evidence.Bytes != len(liveAcceptanceEvidenceText) || len(evidence.SHA256) != 64 || evidence.Publish != "exclusive-case-local" {
		t.Fatalf("evidence receipt=%+v", evidence)
	}
	data, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(evidence.Path)))
	if err != nil || string(data) != liveAcceptanceEvidenceText {
		t.Fatalf("published evidence differs: err=%v data=%q", err, data)
	}
	if err := validateLiveAcceptanceEvidence(caseRoot, evidence); err != nil {
		t.Fatalf("published evidence failed strict validation: %v", err)
	}
	if _, err := publishLiveAcceptanceEvidence(caseRoot); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("evidence publication was not exclusive: %v", err)
	}
}

func TestRunLiveAcceptanceRejectsMissingInputsWithoutCreatingCase(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-case")
	if _, err := RunLiveAcceptance(context.Background(), LiveAcceptanceOptions{CaseRoot: caseRoot, Goal: "goal"}); err == nil || !strings.Contains(err.Error(), "human correction") {
		t.Fatalf("missing correction error=%v", err)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("invalid live gate created case root: %v", err)
	}
}

func TestRunLiveAcceptanceRejectsPackOutsideAllowlistBeforeCreatingCase(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-case")
	_, err := RunLiveAcceptance(context.Background(), LiveAcceptanceOptions{CaseRoot: caseRoot, Pack: "unknown-pack", Goal: "goal", Correction: "correction"})
	if err == nil || !strings.Contains(err.Error(), "outside the explicit cross-pack allowlist") {
		t.Fatalf("pack allowlist error=%v", err)
	}
	if _, statErr := os.Lstat(caseRoot); !os.IsNotExist(statErr) {
		t.Fatalf("pack allowlist rejection created case root: %v", statErr)
	}
}

func TestInitLiveAcceptanceCaseConsumesExactApplyRequest(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-case")
	if err := initLiveAcceptanceCase(caseRoot, liveAcceptancePack, "live-init-contract", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "instance.yml")); err != nil {
		t.Fatalf("live acceptance init did not publish attached case metadata: %v", err)
	}
}

func TestRunLiveAcceptanceCanonicalizesAllowlistedPackBeforeExecutableDiscovery(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-case")
	fakeClaude := filepath.Join(t.TempDir(), "fake-claude")
	if err := os.WriteFile(fakeClaude, []byte("fixture must never execute\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := RunLiveAcceptance(context.Background(), LiveAcceptanceOptions{CaseRoot: caseRoot, Pack: " WEB-SECURITY ", Goal: "goal", Correction: "correction", ClaudePath: fakeClaude})
	if err == nil || !strings.Contains(err.Error(), "refuses a custom Claude executable") {
		t.Fatalf("canonical allowlisted pack error=%v", err)
	}
	if _, statErr := os.Lstat(caseRoot); !os.IsNotExist(statErr) {
		t.Fatalf("canonical pack validation created case root: %v", statErr)
	}
}

func TestRunLiveAcceptanceRejectsCustomClaudeExecutableBeforeCreatingCase(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-case")
	fakeClaude := filepath.Join(t.TempDir(), "fake-claude")
	if err := os.WriteFile(fakeClaude, []byte("fixture must never execute\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := RunLiveAcceptance(context.Background(), LiveAcceptanceOptions{
		CaseRoot:   caseRoot,
		Goal:       "goal",
		Correction: "correction",
		ClaudePath: fakeClaude,
	})
	if err == nil || !strings.Contains(err.Error(), "refuses a custom Claude executable") {
		t.Fatalf("custom Claude executable error=%v", err)
	}
	if _, statErr := os.Lstat(caseRoot); !os.IsNotExist(statErr) {
		t.Fatalf("custom executable rejection created case root: %v", statErr)
	}
}

func TestResolveLiveAcceptanceClaudeIgnoresPATHAndRequiresSignedCanonicalInstall(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the installed signed Claude Code executable")
	}
	fakeDir := t.TempDir()
	fakeClaude := filepath.Join(fakeDir, "claude.exe")
	if err := os.WriteFile(fakeClaude, []byte("fixture must never execute\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	identity, err := resolveLiveAcceptanceClaude("")
	if err != nil {
		t.Skipf("signed canonical Claude Code installation unavailable: %v", err)
	}
	if same, _ := filepath.Abs(fakeClaude); strings.EqualFold(identity.Path, same) {
		t.Fatalf("trusted discovery accepted PATH-injected executable: %+v", identity)
	}
	if identity.Publisher != liveAcceptanceClaudePublisher || len(identity.SHA256) != 64 || !strings.HasSuffix(identity.Version, " (Claude Code)") {
		t.Fatalf("trusted Claude identity=%+v", identity)
	}
}

func TestAcquireClaudeExecutableLaunchBindingRejectsHashDrift(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the installed signed Claude Code executable")
	}
	identity, err := resolveLiveAcceptanceClaude("")
	if err != nil {
		t.Skipf("signed canonical Claude Code installation unavailable: %v", err)
	}
	_, err = acquireClaudeExecutableLaunchBinding(Options{
		ClaudePath:                        identity.Path,
		ExpectedClaudeExecutableSHA256:    strings.Repeat("0", 64),
		ExpectedClaudeExecutablePublisher: identity.Publisher,
	})
	if err == nil || !strings.Contains(err.Error(), "SHA-256 drift") {
		t.Fatalf("hash drift error=%v", err)
	}
}

func TestLockTrustedClaudeExecutableRejectsAncestorNamespaceReplacement(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows namespace binding test")
	}
	root := t.TempDir()
	parent := filepath.Join(root, "install")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(parent, "claude.exe")
	if err := os.WriteFile(executable, []byte("trusted fixture\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	binding, err := lockTrustedClaudeExecutable(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer binding.Close()
	moved := filepath.Join(root, "install-original")
	if err := os.Rename(parent, moved); err == nil {
		t.Fatal("trusted namespace binding allowed ancestor directory replacement")
	}
	if err := binding.Validate(); err != nil {
		t.Fatalf("trusted namespace drifted after blocked replacement: %v", err)
	}
}

func TestRunLiveAcceptanceRejectsExistingCase(t *testing.T) {
	caseRoot := t.TempDir()
	if _, err := RunLiveAcceptance(context.Background(), LiveAcceptanceOptions{CaseRoot: caseRoot, Goal: "goal", Correction: "correction"}); err == nil || !strings.Contains(err.Error(), "non-existing fresh and attached case roots") {
		t.Fatalf("existing case error=%v", err)
	}
}

func TestRunLiveAcceptanceRejectsReceiptInsideDisposableCase(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-case")
	receiptPath := filepath.Join(caseRoot, "receipt.json")
	if _, err := RunLiveAcceptance(context.Background(), LiveAcceptanceOptions{CaseRoot: caseRoot, Goal: "goal", Correction: "correction", ReceiptPath: receiptPath}); err == nil || !strings.Contains(err.Error(), "receipt must be outside") {
		t.Fatalf("receipt containment error=%v", err)
	}
	if _, err := os.Lstat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("invalid receipt path created case root: %v", err)
	}
}

func TestRunDailyPublicRouteBootstrapsFreshCaseWithoutLLMResultFixtures(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "fresh-case")
	goal := "Analyze a harmless synthetic acceptance target"
	result, err := RunDaily(context.Background(), DailyOptions{Target: caseRoot, Goal: goal, Actor: "live-public-route-test", ClaudePath: filepath.Join(t.TempDir(), "missing-claude.exe"), ExpectedClaudeExecutableSHA256: strings.Repeat("0", 64), ExpectedClaudeExecutablePublisher: liveAcceptanceClaudePublisher})
	if err == nil || !strings.Contains(err.Error(), "validate trusted Claude Code executable") {
		t.Fatalf("daily public route result=%+v err=%v", result, err)
	}
	if !result.OnboardingApplied || result.Pack != liveAcceptancePack || result.Lane == "" || result.SessionLaunches != 0 || !containsDailyStep(result.DriverSteps, "overview") || !containsDailyStep(result.DriverSteps, "start") {
		t.Fatalf("daily public route result=%+v", result)
	}
	inspection, ok, inspectErr := memberexecution.Latest(caseRoot, result.Lane)
	if inspectErr != nil {
		t.Fatal(inspectErr)
	}
	if ok || inspection.Manifest != nil {
		t.Fatalf("public start must leave member execution to the ordinary host after executable resolution fails: found=%t inspection=%+v", ok, inspection)
	}
}

func TestLiveAcceptanceProductionPackageHasNoDirectPackageMutationSelectors(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	violations := []string{}
	productionFiles := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		productionFiles++
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, violation := range liveAcceptanceMutationSelectors(file) {
			violations = append(violations, name+":"+violation)
		}
	}
	if productionFiles == 0 {
		t.Fatal("sessionhost production package scan found no Go files")
	}
	if len(violations) != 0 {
		t.Fatalf("sessionhost production package bypasses the public route: %v", violations)
	}
}

func TestLiveAcceptanceMutationGuardRejectsAliasedHelperSelector(t *testing.T) {
	for _, source := range []string{
		`package fixture
import ws "github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
func helper() { mutation := ws.StartApply; _ = mutation }
`,
		`package fixture
import . "github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
func helper() { mutation := StartApply; _ = mutation }
`,
		"package fixture\nimport ws `github.com/shuiyu486/re-context-kits/internal/rekit/workstream`\nfunc helper() { mutation := ws.StartApply; _ = mutation }\n",
		"package fixture\nimport ws \"github.com\\x2fshuiyu486\\x2fre-context-kits\\x2finternal\\x2frekit\\x2fworkstream\"\nfunc helper() { mutation := ws.StartApply; _ = mutation }\n",
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source, 0)
		if err != nil {
			t.Fatal(err)
		}
		violations := liveAcceptanceMutationSelectors(file)
		if len(violations) != 1 || !strings.Contains(violations[0], "workstream.StartApply") {
			t.Fatalf("alias/helper mutation violations=%v", violations)
		}
	}
}

func liveAcceptanceMutationSelectors(file *ast.File) []string {
	forbidden := map[string]map[string]bool{
		"github.com/shuiyu486/re-context-kits/internal/rekit/onboarding": {"Apply": true},
		"github.com/shuiyu486/re-context-kits/internal/rekit/note":       {"Append": true},
		"github.com/shuiyu486/re-context-kits/internal/rekit/workstream": {
			"StartApply": true, "ReconcileApply": true, "CompleteApply": true,
		},
	}
	aliases := map[string]string{}
	dotImports := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return []string{"invalid-import-literal:" + spec.Path.Value}
		}
		if forbidden[path] == nil {
			continue
		}
		alias := filepath.Base(path)
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		if alias == "." {
			for name := range forbidden[path] {
				dotImports[name] = path
			}
			continue
		}
		aliases[alias] = path
	}
	violations := []string{}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.SelectorExpr:
			ident, ok := typed.X.(*ast.Ident)
			if !ok {
				return true
			}
			path := aliases[ident.Name]
			if forbidden[path][typed.Sel.Name] {
				violations = append(violations, fmt.Sprintf("%s.%s", filepath.Base(path), typed.Sel.Name))
			}
		case *ast.Ident:
			path := dotImports[typed.Name]
			if path != "" {
				violations = append(violations, fmt.Sprintf("%s.%s", filepath.Base(path), typed.Name))
			}
		}
		return true
	})
	return violations
}

func TestValidateLiveAcceptanceReviewerRejectionStopRequiresIsolatedMemberAndReviewer(t *testing.T) {
	result := DailyResult{
		FinalState:      "reviewer-rejected-awaiting-correction",
		SessionLaunches: 2, SessionCompletions: 2,
		HostRuns: []Result{
			{
				FinalMode:       "reviewer-ready",
				SessionLaunches: 1, SessionCompletions: 1,
				Sessions: []Session{
					{Started: true, SessionKind: "member", Outcome: "returned"},
				},
			},
			{
				FinalMode:       "reviewer-rejected-awaiting-correction",
				SessionLaunches: 1, SessionCompletions: 1,
				Sessions: []Session{
					{Started: true, SessionKind: "reviewer", Outcome: "returned"},
				},
			},
		},
	}
	if err := validateLiveAcceptanceReviewerRejectionStop(result, 3); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*DailyResult){
		"missing segment":    func(value *DailyResult) { value.HostRuns = value.HostRuns[:1] },
		"missing completion": func(value *DailyResult) { value.SessionCompletions = 1 },
		"wrong final state":  func(value *DailyResult) { value.FinalState = "attention-required" },
		"failed reviewer":    func(value *DailyResult) { value.HostRuns[1].Sessions[0].Outcome = "replacement-requested" },
	} {
		t.Run(name, func(t *testing.T) {
			copy := result
			copy.HostRuns = append([]Result{}, result.HostRuns...)
			for index := range copy.HostRuns {
				copy.HostRuns[index].Sessions = append([]Session{}, result.HostRuns[index].Sessions...)
			}
			mutate(&copy)
			if err := validateLiveAcceptanceReviewerRejectionStop(copy, 3); err == nil {
				t.Fatalf("invalid bounded rejection stop was accepted: %+v", copy)
			}
		})
	}
}

func TestAddLiveAcceptanceSessionsCountsLaunchesWithoutInventingCompletions(t *testing.T) {
	receipt := LiveAcceptanceReceipt{}
	addLiveAcceptanceSessions(&receipt, Result{Sessions: []Session{
		{Started: true, AttemptGeneration: 2, RunLaunchOrdinal: 3, SessionID: "member-session", SessionKind: "member", Outcome: "host-failed"},
		{Recovered: true, AttemptGeneration: 1, SessionID: "reviewer-session", SessionKind: "reviewer", Outcome: "intake-failed"},
	}}, 2)
	if receipt.MemberLaunches != 1 || receipt.MemberCompletions != 0 || receipt.ReviewerLaunches != 0 || receipt.ReviewerCompletions != 0 {
		t.Fatalf("session lifecycle counts=%+v", receipt)
	}
	if len(receipt.MemberSessions) != 1 || !receipt.MemberSessions[0].Started || receipt.MemberSessions[0].AttemptGeneration != 2 || receipt.MemberSessions[0].HostRun != 2 || receipt.MemberSessions[0].RunLaunchOrdinal != 3 || len(receipt.ReviewerSessions) != 1 || !receipt.ReviewerSessions[0].Recovered || receipt.ReviewerSessions[0].Started || receipt.ReviewerSessions[0].HostRun != 2 || receipt.ReviewerSessions[0].RunLaunchOrdinal != 0 {
		t.Fatalf("session lifecycle records=%+v", receipt)
	}
}

func TestLiveAcceptanceRejectionProjectsCanonicalIdentity(t *testing.T) {
	input := workstream.MemberReviewerRejection{
		ManifestRef: ".rekit/manifest.json", ManifestSHA256: strings.Repeat("a", 64),
		PacketID: "packet", RouteID: "route", ShardID: "shard", ReviewerResultInputSHA256: strings.Repeat("b", 64),
		ReviewerSession: "reviewer-session", VerificationEventID: "verification", DecisionEventID: "decision",
		Summary: "missing acceptance code", EvidenceRefs: []string{".rekit/manifest.json#review"}, OwnerExecutor: "member-g1", OwnerGeneration: 1,
	}
	projected := liveAcceptanceRejection(input)
	if projected == nil || projected.ManifestPath != input.ManifestRef || projected.PacketID != input.PacketID || projected.ReviewerSession != input.ReviewerSession || projected.DecisionEventID != input.DecisionEventID || projected.OwnerGeneration != 1 || !sameLiveAcceptanceStrings(projected.EvidenceRefs, input.EvidenceRefs) {
		t.Fatalf("rejection projection=%+v", projected)
	}
	projected.EvidenceRefs[0] = "changed"
	if input.EvidenceRefs[0] == "changed" {
		t.Fatal("rejection projection aliases canonical evidence refs")
	}
}

func TestLiveAcceptanceAcceptanceProjectsCanonicalIdentity(t *testing.T) {
	input := workstream.MemberReviewerAcceptance{
		ManifestRef: ".rekit/replacement-manifest.json", ManifestSHA256: strings.Repeat("a", 64),
		PacketID: "replacement-packet", RouteID: "replacement-route", ShardID: "replacement-shard", ReviewerResultInputSHA256: strings.Repeat("b", 64),
		ReviewerSession: "replacement-reviewer", VerificationEventID: "replacement-verification", DecisionEventID: "replacement-decision", OwnerExecutor: "member-g2", OwnerGeneration: 2,
	}
	projected := liveAcceptanceAcceptance(input)
	if projected == nil || projected.ManifestPath != input.ManifestRef || projected.ManifestSHA256 != input.ManifestSHA256 || projected.PacketID != input.PacketID || projected.RouteID != input.RouteID || projected.ShardID != input.ShardID || projected.ReviewerResultInputSHA256 != input.ReviewerResultInputSHA256 || projected.ReviewerSession != input.ReviewerSession || projected.VerificationEventID != input.VerificationEventID || projected.DecisionEventID != input.DecisionEventID || projected.OwnerExecutor != input.OwnerExecutor || projected.OwnerGeneration != input.OwnerGeneration {
		t.Fatalf("acceptance projection=%+v", projected)
	}
}

func TestBindLiveAcceptanceOwnerGenerationUsesLatestUnboundMember(t *testing.T) {
	sessions := []LiveAcceptanceSession{
		{Kind: "member", OwnerGeneration: 1, AttemptGeneration: 1},
		{Kind: "reviewer", AttemptGeneration: 1},
		{Kind: "member", AttemptGeneration: 1},
	}
	bindLiveAcceptanceOwnerGeneration(sessions, memberexecution.Inspection{Owner: memberexecution.Owner{ExecutorGeneration: 2}})
	if sessions[0].OwnerGeneration != 1 || sessions[1].OwnerGeneration != 0 || sessions[2].OwnerGeneration != 2 {
		t.Fatalf("owner generations=%+v", sessions)
	}
}

func TestWriteLiveAcceptanceReceiptRecordsFinalCleanupState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := LiveAcceptanceReceipt{SchemaVersion: 1, Kind: "rekit-" + liveAcceptancePack + "-live-acceptance-receipt", Passed: true, Pack: liveAcceptancePack, Cleanup: "removed"}
	if err := WriteLiveAcceptanceReceipt(path, receipt); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded LiveAcceptanceReceipt
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Passed || decoded.Pack != liveAcceptancePack || decoded.Cleanup != "removed" {
		t.Fatalf("receipt=%+v", decoded)
	}
	if err := WriteLiveAcceptanceReceipt(path, LiveAcceptanceReceipt{Passed: false}); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("receipt publication replaced existing evidence: %v", err)
	}
	unchanged, err := os.ReadFile(path)
	if err != nil || string(unchanged) != string(data) {
		t.Fatalf("existing receipt changed: err=%v bytes=%q", err, unchanged)
	}
}

func TestWriteLiveAcceptanceReceiptRejectsAncestorSymlink(t *testing.T) {
	base := t.TempDir()
	realParent := filepath.Join(base, "real-parent")
	if err := os.Mkdir(realParent, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedParent := filepath.Join(base, "linked-parent")
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	path := filepath.Join(linkedParent, "nested", "receipt.json")
	err := WriteLiveAcceptanceReceipt(path, LiveAcceptanceReceipt{Passed: true})
	if err == nil || (!strings.Contains(err.Error(), "symlink") && !strings.Contains(err.Error(), "reparse point")) {
		t.Fatalf("ancestor symlink receipt error=%v", err)
	}
	if _, err := os.Lstat(filepath.Join(realParent, "nested", "receipt.json")); !os.IsNotExist(err) {
		t.Fatalf("ancestor symlink publication escaped into target: %v", err)
	}
}

func TestRemoveLiveAcceptanceCaseRefusesReplacement(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "case")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "owned.txt"), []byte("owned\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var identity liveAcceptanceCaseIdentity
	if err := captureLiveAcceptanceCaseRoot(path, &identity); err != nil {
		t.Fatal(err)
	}
	defer identity.Close()
	original := path + "-original"
	if err := os.Rename(path, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(path, "replacement.txt")
	if err := os.WriteFile(replacement, []byte("replacement\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeLiveAcceptanceCase(path, &identity); err == nil || !strings.Contains(err.Error(), "replaced case root") {
		t.Fatalf("replacement cleanup error=%v", err)
	}
	if data, err := os.ReadFile(replacement); err != nil || string(data) != "replacement\n" {
		t.Fatalf("replacement was changed: %q err=%v", data, err)
	}
	if data, err := os.ReadFile(filepath.Join(original, "owned.txt")); err != nil || string(data) != "owned\n" {
		t.Fatalf("original identity was changed: %q err=%v", data, err)
	}
}

func TestRemoveLiveAcceptanceCaseRemovesCapturedIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := []byte("owned\n")
	if err := os.WriteFile(filepath.Join(path, "owned.txt"), marker, 0o600); err != nil {
		t.Fatal(err)
	}
	var identity liveAcceptanceCaseIdentity
	if err := captureLiveAcceptanceCaseRoot(path, &identity); err != nil {
		t.Fatal(err)
	}
	defer identity.Close()
	if err := bindLiveAcceptanceCaseMarker(&identity, "owned.txt", marker); err != nil {
		t.Fatal(err)
	}
	if err := removeLiveAcceptanceCase(path, &identity); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("captured case root remains: %v", err)
	}
	if _, err := os.Lstat(path + ".cleanup"); !os.IsNotExist(err) {
		t.Fatalf("cleanup quarantine remains: %v", err)
	}
}

func TestRemoveLiveAcceptanceCaseRefusesChangedMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "case")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(path, "marker")
	if err := os.WriteFile(markerPath, []byte("expected\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var identity liveAcceptanceCaseIdentity
	if err := captureLiveAcceptanceCaseRoot(path, &identity); err != nil {
		t.Fatal(err)
	}
	defer identity.Close()
	if err := bindLiveAcceptanceCaseMarker(&identity, "marker", []byte("expected\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerPath, []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeLiveAcceptanceCase(path, &identity); err == nil || !strings.Contains(err.Error(), "marker changed") {
		t.Fatalf("changed marker cleanup error=%v", err)
	}
	if data, err := os.ReadFile(markerPath); err != nil || string(data) != "changed\n" {
		t.Fatalf("changed marker was removed: %q err=%v", data, err)
	}
}
