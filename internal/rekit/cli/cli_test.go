package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/caseshim"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaultdocs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/defaults"
	"github.com/shuiyu486/re-context-kits/internal/rekit/manifest"
	"github.com/shuiyu486/re-context-kits/internal/rekit/releasecheck"
	"github.com/shuiyu486/re-context-kits/internal/rekit/review"
)

func TestParsePowerShellStyleOptions(t *testing.T) {
	opt, err := Parse([]string{"doctor", "-Pack", "_template", "-Target", "."})
	if err != nil {
		t.Fatal(err)
	}
	if opt.Command != "doctor" {
		t.Fatalf("Command = %q, want doctor", opt.Command)
	}
	if opt.Pack != "_template" {
		t.Fatalf("Pack = %q, want _template", opt.Pack)
	}
	if opt.Target != "." {
		t.Fatalf("Target = %q, want .", opt.Target)
	}
}

func TestParseIgnoresGoRunSeparator(t *testing.T) {
	opt, err := Parse([]string{"--", "-Command", "doctor", "-Pack", "_template"})
	if err != nil {
		t.Fatal(err)
	}
	if opt.Command != "doctor" || opt.Pack != "_template" {
		t.Fatalf("unexpected options after -- separator: %+v", opt)
	}
}

func TestParseDefaults(t *testing.T) {
	opt, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if opt.Command != "status" {
		t.Fatalf("Command = %q, want status", opt.Command)
	}
	if opt.Pack != defaults.DefaultPack {
		t.Fatalf("Pack = %q, want %s", opt.Pack, defaults.DefaultPack)
	}
}

func TestParsePlanSubagentsNumericOptionsRejectTrailingJunk(t *testing.T) {
	_, err := Parse([]string{"-Command", "plan-subagents", "-ItemsPerAgent", "2x"})
	if err == nil || !strings.Contains(err.Error(), "invalid -ItemsPerAgent") {
		t.Fatalf("error = %v, want invalid -ItemsPerAgent", err)
	}
	_, err = Parse([]string{"-Command", "plan-subagents", "-MaxParallel", "3x"})
	if err == nil || !strings.Contains(err.Error(), "invalid -MaxParallel") {
		t.Fatalf("error = %v, want invalid -MaxParallel", err)
	}
}

func TestRunDoctorRejectsNonCaseTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "missing-case")
	var out bytes.Buffer
	err := Run([]string{"-Command", "doctor", "-Target", target}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for non-case target")
	}
	if strings.Contains(out.String(), "pack validation ok") {
		t.Fatalf("doctor reported pack validation for non-case target: %q", out.String())
	}
	if !strings.Contains(err.Error(), "target is neither this kit root nor an attached rekit case") {
		t.Fatalf("error = %q, want non-case target error", err.Error())
	}
}

func TestRunDoctorJsonPack(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"-Command", "doctor", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command       string `json:"command"`
		SchemaVersion int    `json:"schemaVersion"`
		IsMutation    bool   `json:"isMutation"`
		Pack          string `json:"pack"`
		Mode          string `json:"mode"`
		Valid         bool   `json:"valid"`
		Summary       string `json:"summary"`
		Rows          []struct {
			File  string `json:"file"`
			Bytes int64  `json:"bytes"`
			Limit int64  `json:"limit"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("doctor JSON did not decode: %v\n%s", err, out.String())
	}
	if result.Command != "doctor" || result.SchemaVersion != 1 || result.IsMutation || result.Pack != "_template" || result.Mode != "pack" || !result.Valid || result.Summary != "pack validation ok" || len(result.Rows) == 0 {
		t.Fatalf("unexpected doctor JSON: %+v", result)
	}
	if result.Rows[0].File == "" || result.Rows[0].Bytes <= 0 || result.Rows[0].Limit <= 0 {
		t.Fatalf("unexpected doctor row: %+v", result.Rows[0])
	}
}

func TestRunValidateJsonUsesValidateCommand(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"-Command", "validate", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command string `json:"command"`
		Mode    string `json:"mode"`
		Valid   bool   `json:"valid"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("validate JSON did not decode: %v\n%s", err, out.String())
	}
	if result.Command != "validate" || result.Mode != "pack" || !result.Valid {
		t.Fatalf("unexpected validate JSON: %+v", result)
	}
}

func TestRunCaseDoctorValidatesAttachedCase(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "doctor", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "instance validation ok") || !strings.Contains(out.String(), ".rekit") {
		t.Fatalf("unexpected case doctor output: %s", out.String())
	}
}

func TestRunCaseDoctorJson(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "doctor", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command string `json:"command"`
		Pack    string `json:"pack"`
		Target  string `json:"target"`
		Mode    string `json:"mode"`
		Valid   bool   `json:"valid"`
		Summary string `json:"summary"`
		Rows    []struct {
			File string `json:"file"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("case doctor JSON did not decode: %v\n%s", err, out.String())
	}
	if result.Command != "doctor" || result.Pack != "_template" || result.Target != caseRoot || result.Mode != "case" || !result.Valid || result.Summary != "instance validation ok" || len(result.Rows) == 0 {
		t.Fatalf("unexpected case doctor JSON: %+v", result)
	}
}

func TestRunDoctorRejectsUnsupportedFormat(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "doctor", "-Format", "yaml"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported doctor format") {
		t.Fatalf("error = %v, want unsupported doctor format", err)
	}
}

func TestRunStatusJsonKit(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Command        string `json:"command"`
		SchemaVersion  int    `json:"schemaVersion"`
		IsMutation     bool   `json:"isMutation"`
		RuntimeRoot    string `json:"runtimeRoot"`
		TemplateRoot   string `json:"templateRoot"`
		Pack           string `json:"pack"`
		Target         string `json:"target"`
		TargetProvided bool   `json:"targetProvided"`
		Mode           string `json:"mode"`
		Case           any    `json:"case"`
		Manifest       struct {
			ManifestPath  string `json:"manifestPath"`
			SchemaVersion string `json:"schemaVersion"`
			ManagedFiles  int    `json:"managedFiles"`
			PromoteFiles  int    `json:"promoteFiles"`
			ToolingFiles  int    `json:"toolingFiles"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.SchemaVersion != 1 || status.IsMutation || status.Mode != "kit" || status.Pack != "_template" || status.TargetProvided {
		t.Fatalf("unexpected status JSON envelope: %+v", status)
	}
	if status.RuntimeRoot == "" || status.TemplateRoot == "" || status.Target == "" || status.Case != nil {
		t.Fatalf("unexpected status JSON roots/case: %+v", status)
	}
	if !strings.HasSuffix(filepath.ToSlash(status.Manifest.ManifestPath), "packs/_template/manifest.yml") || status.Manifest.SchemaVersion != "1" || status.Manifest.ManagedFiles != 4 || status.Manifest.PromoteFiles != 4 || status.Manifest.ToolingFiles != 2 {
		t.Fatalf("unexpected manifest summary: %+v", status.Manifest)
	}
}

func TestRunStatusJsonDefaultPackContract(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Command       string `json:"command"`
		SchemaVersion int    `json:"schemaVersion"`
		IsMutation    bool   `json:"isMutation"`
		Pack          string `json:"pack"`
		Mode          string `json:"mode"`
		Case          any    `json:"case"`
		Manifest      struct {
			ManifestPath  string `json:"manifestPath"`
			SchemaVersion string `json:"schemaVersion"`
			ManagedFiles  int    `json:"managedFiles"`
			PromoteFiles  int    `json:"promoteFiles"`
			ToolingFiles  int    `json:"toolingFiles"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.SchemaVersion != 1 || status.IsMutation || status.Mode != "kit" || status.Pack != defaults.DefaultPack || status.Case != nil {
		t.Fatalf("unexpected default status JSON envelope: %+v", status)
	}
	if !strings.HasSuffix(filepath.ToSlash(status.Manifest.ManifestPath), "packs/"+defaults.DefaultPack+"/manifest.yml") || status.Manifest.SchemaVersion != "1" || status.Manifest.ManagedFiles != 7 || status.Manifest.PromoteFiles != 7 || status.Manifest.ToolingFiles != 12 {
		t.Fatalf("unexpected default manifest summary: %+v", status.Manifest)
	}
}

func TestRunStatusJsonCase(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Command        string `json:"command"`
		SchemaVersion  int    `json:"schemaVersion"`
		IsMutation     bool   `json:"isMutation"`
		Pack           string `json:"pack"`
		TargetProvided bool   `json:"targetProvided"`
		Mode           string `json:"mode"`
		Case           struct {
			CaseRoot       string `json:"caseRoot"`
			MetadataSource string `json:"metadataSource"`
			TemplatePack   string `json:"templatePack"`
			ProjectName    string `json:"projectName"`
			Moved          bool   `json:"moved"`
		} `json:"case"`
		Manifest any `json:"manifest"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("case status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.SchemaVersion != 1 || status.IsMutation || status.Pack != "_template" || !status.TargetProvided || status.Mode != "case" || status.Manifest != nil {
		t.Fatalf("unexpected case status JSON envelope: %+v", status)
	}
	if status.Case.CaseRoot != caseRoot || status.Case.MetadataSource != "instance" || status.Case.TemplatePack != "_template" || status.Case.ProjectName != "demo" || status.Case.Moved {
		t.Fatalf("unexpected case status JSON: %+v", status.Case)
	}
}

func TestRunStatusRejectsUnsupportedFormat(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "status", "-Format", "yaml"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported status format") {
		t.Fatalf("error = %v, want unsupported status format", err)
	}
}

type releaseCheckPowerShellDeprecation = releasecheck.PowerShellDeprecation

type releaseCheckResult = releasecheck.Result

func TestRunReleaseCheckJsonInventory(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"-Command", "release-check", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result releaseCheckResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("release-check JSON did not decode: %v\n%s", err, out.String())
	}
	resultCounts := releasecheck.ReleaseCheckResultCountsFor(result)
	if result.Command != "release-check" || result.SchemaVersion != 1 || result.IsMutation || !result.Ready || result.Summary != "release gate inventory ok" || resultCounts.Warnings != 0 {
		t.Fatalf("unexpected release-check JSON envelope: %+v", result)
	}
	assertReleaseCheckCommand(t, result.RequiredCommands, "go run ./cmd/rekit -- -Command release-check -Format json")
	assertReleaseCheckCommand(t, result.RequiredCommands, "go run ./cmd/rekit -- -Command status")
	assertReleaseCheckCommand(t, result.RequiredCommands, "go run ./cmd/rekit -- -Command packs")
	assertReleaseCheckCommand(t, result.RequiredCommands, "go run ./cmd/rekit -- -Command doctor")
	assertReleaseCheckCommand(t, result.RequiredCommands, "go test ./...")
	assertReleaseCheckCommand(t, result.RequiredCommands, "go vet ./...")
	assertReleaseCheckCommand(t, result.RequiredCommands, "git diff --check")
	assertReleaseCheckDocument(t, result.Documents, "docs/release-readiness.md")
	assertReleaseCheckDocument(t, result.Documents, "docs/mission-control-product-direction.md")
	assertReleaseCheckDocument(t, result.Documents, "docs/autonomous-goal.md")
	assertReleaseCheckDocument(t, result.Documents, "docs/go-first-convergence-plan.md")
	assertReleaseCheckDocument(t, result.Documents, "docs/powershell-deprecation.md")
	if !result.GateProfile.Ready || result.GateProfile.Name != "local-ci-minimum" || result.GateProfile.StepCount != resultCounts.RecommendedMinimum || result.GateProfile.LargeMatrixDefault || resultCounts.GateProfileSteps != resultCounts.RecommendedMinimum {
		t.Fatalf("unexpected release-check gate profile: %+v", result.GateProfile)
	}
	assertReleaseCheckStep(t, result.RecommendedMinimum, "go run ./cmd/rekit -- -Command release-check -Format json", "go-run", "cmd/rekit")
	assertReleaseCheckStep(t, result.RecommendedMinimum, "go run ./cmd/rekit -- -Command status", "go-run", "cmd/rekit")
	assertReleaseCheckStep(t, result.RecommendedMinimum, "go run ./cmd/rekit -- -Command packs", "go-run", "cmd/rekit")
	assertReleaseCheckStep(t, result.RecommendedMinimum, "go run ./cmd/rekit -- -Command doctor", "go-run", "cmd/rekit")
	assertReleaseCheckCIReleaseGate(t, result.CIReleaseGate)
	assertReleaseCheckPowerShellDeprecation(t, result.PowerShellDeprecation)
	assertReleaseCheckGoNativePublicSurface(t, result.GoNativePublicSurface)
	assertReleaseCheckPublicFacadeRemoval(t, result.PublicFacadeRemoval)
	assertReleaseCheckCaseShim(t, result.CaseShim)
	assertReleaseCheckPublicDefaultDocs(t, result.PublicDefaultDocs)
	assertReleaseCheckHandoff(t, result.ReleaseHandoff)
	if resultCounts.RecommendedMinimum == 0 || resultCounts.Boundaries == 0 || resultCounts.KnownGaps == 0 || resultCounts.Packs == 0 || resultCounts.HeavyToolGateActions == 0 {
		t.Fatalf("release-check omitted required inventory: %+v", result)
	}
	if strings.Join(result.HeavyToolGateActions, ",") != "debug,dump,full-trace,inject,network,patch,symex" {
		t.Fatalf("unexpected heavy-tool gate actions: %v", result.HeavyToolGateActions)
	}
	packs := map[string]manifest.PackSummary{}
	for _, pack := range result.Packs {
		packs[pack.ID] = pack
	}
	if pack := packs[defaults.DefaultPack]; pack.Maturity != "mature" || !pack.SchemaValid || pack.SchemaVersion != "1" || pack.HeavyToolGates != 7 {
		t.Fatalf("unexpected default pack release-check row: %+v", pack)
	}
	if pack := packs["web-security"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.SchemaVersion != "1" || pack.HeavyToolGates != 7 {
		t.Fatalf("unexpected web-security release-check row: %+v", pack)
	}
}

func assertReleaseCheckCommand(t *testing.T, steps []releasecheck.GateStep, want string) {
	t.Helper()
	for _, step := range steps {
		if step.Command == want {
			if !step.Required || !step.InCatalog || !step.Present || !step.Resolved || strings.TrimSpace(step.Kind) == "" {
				t.Fatalf("release-check command %q flags = required:%t inCatalog:%t present:%t resolved:%t kind:%q", want, step.Required, step.InCatalog, step.Present, step.Resolved, step.Kind)
			}
			return
		}
	}
	t.Fatalf("release-check missing command %q: %+v", want, steps)
}

func assertReleaseCheckStep(t *testing.T, steps []releasecheck.GateStep, wantCommand, wantKind, wantRepoPath string) {
	t.Helper()
	for _, step := range steps {
		if step.Command == wantCommand {
			if step.Kind != wantKind || step.RepoPath != wantRepoPath || !step.Present || !step.Resolved {
				t.Fatalf("release-check step %q = kind:%q repoPath:%q present:%t resolved:%t, want kind:%q repoPath:%q present/resolved", wantCommand, step.Kind, step.RepoPath, step.Present, step.Resolved, wantKind, wantRepoPath)
			}
			return
		}
	}
	t.Fatalf("release-check missing step %q: %+v", wantCommand, steps)
}

func assertReleaseCheckDocument(t *testing.T, docs []releasecheck.DocumentCheck, want string) {
	t.Helper()
	for _, doc := range docs {
		if doc.Path == want {
			if !doc.Present || strings.TrimSpace(doc.Purpose) == "" {
				t.Fatalf("release-check document %q flags = present:%t purpose:%q", want, doc.Present, doc.Purpose)
			}
			return
		}
	}
	t.Fatalf("release-check missing document %q: %+v", want, docs)
}

func assertReleaseCheckCIReleaseGate(t *testing.T, gate releasecheck.CIReleaseGate) {
	t.Helper()
	counts := releasecheck.CIReleaseGateCountsFor(gate)
	if !gate.Ready || gate.WorkflowPath != ".github/workflows/release-gate.yml" || gate.Summary != "CI release gate inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected CI release gate inventory: %+v", gate)
	}
	if counts.WorkflowChecks == 0 || counts.Jobs != 3 || counts.RequiredCommands != 18 || counts.ForbiddenStrings == 0 {
		t.Fatalf("CI release gate inventory omitted required sections: %+v", gate)
	}
	assertCIReleaseJob(t, gate, "go-checks-linux", "Go release checks (Linux)", "ubuntu-latest")
	assertCIReleaseJob(t, gate, "go-checks-windows", "Go release checks (Windows)", "windows-latest")
	assertCIReleaseJob(t, gate, "go-checks-macos", "Go release checks (macOS)", "macos-latest")
	for _, job := range []string{"go-checks-linux", "go-checks-windows", "go-checks-macos"} {
		assertCIReleaseCommand(t, gate, job, "go run ./cmd/rekit -- -Command release-check -Format json")
		assertCIReleaseCommand(t, gate, job, "go run ./cmd/rekit -- -Command status")
		assertCIReleaseCommand(t, gate, job, "go run ./cmd/rekit -- -Command packs")
		assertCIReleaseCommand(t, gate, job, "go run ./cmd/rekit -- -Command doctor")
		assertCIReleaseCommand(t, gate, job, "go test ./...")
		assertCIReleaseCommand(t, gate, job, "go vet ./...")
	}
	for _, forbidden := range gate.ForbiddenStrings {
		if forbidden.Present {
			t.Fatalf("CI release gate forbidden pattern present: %+v", forbidden)
		}
	}
}

func assertCIReleaseJob(t *testing.T, gate releasecheck.CIReleaseGate, id, name, runsOn string) {
	t.Helper()
	for _, job := range gate.Jobs {
		if job.ID == id {
			if !job.Present || !job.Required || job.Name != name || job.RunsOn != runsOn {
				t.Fatalf("CI release job %s = %+v, want name=%q runsOn=%q present/required", id, job, name, runsOn)
			}
			return
		}
	}
	t.Fatalf("CI release gate missing job %s: %+v", id, gate.Jobs)
}

func assertCIReleaseCommand(t *testing.T, gate releasecheck.CIReleaseGate, jobID, command string) {
	t.Helper()
	for _, item := range gate.RequiredCommands {
		if item.Job == jobID && item.Command == command {
			if !item.Present || !item.Required {
				t.Fatalf("CI release command %s/%s = %+v, want present/required", jobID, command, item)
			}
			return
		}
	}
	t.Fatalf("CI release gate missing command %s/%s: %+v", jobID, command, gate.RequiredCommands)
}

func assertReleaseCheckHandoff(t *testing.T, handoff releasecheck.ReleaseHandoff) {
	t.Helper()
	counts := releasecheck.ReleaseHandoffCountsFor(handoff)
	if !handoff.Ready || handoff.Summary != "release handoff summary ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected release handoff summary: %+v", handoff)
	}
	if counts.ReadFirst != 7 || counts.Signals != 12 || counts.KnownGaps == 0 || counts.PackMaturity.Total == 0 || counts.Validation == 0 || counts.NextActions == 0 {
		t.Fatalf("release handoff omitted required sections: %+v", handoff)
	}
	assertReleaseHandoffReadFirst(t, handoff, "docs/release-readiness.md")
	assertReleaseHandoffReadFirst(t, handoff, "docs/mission-control-product-direction.md")
	assertReleaseHandoffReadFirst(t, handoff, "docs/autonomous-goal.md")
	assertReleaseHandoffReadFirst(t, handoff, "docs/go-first-convergence-plan.md")
	assertReleaseHandoffReadFirst(t, handoff, "docs/powershell-deprecation.md")
	assertReleaseHandoffReadFirst(t, handoff, "docs/batch-plan.md")
	assertReleaseHandoffReadFirst(t, handoff, "CHANGELOG.md")
	assertReleaseHandoffSignal(t, handoff, "release-check inventory")
	assertReleaseHandoffSignal(t, handoff, "CI release gate")
	assertReleaseHandoffSignalDetail(t, handoff, "PowerShell deprecation", "fallbackRetirement=true noFallback=20 candidates=0 removalModules=0 retiredModules=13")
	assertReleaseHandoffSignalDetail(t, handoff, "PowerShell deprecation", "facadeRuntime=true legacyImports=false dispatcher=false")
	assertReleaseHandoffSignalDetail(t, handoff, "PowerShell deprecation", "publicFacade=true retained=true facadeCommands=20 noFallback=20")
	assertReleaseHandoffSignalDetail(t, handoff, "PowerShell deprecation", "moduleRemoval=true candidates=0 retired=13 facadeDeps=0 undocumented=0")
	assertReleaseHandoffSignalDetail(t, handoff, "PowerShell deprecation", "moduleReferences=true activeTests=0 fixtures=0 blockers=0 unclassified=0")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "entrypoint=cmd/rekit present=true catalog=internal/rekit/commands/commands.go catalogPresent=true")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "default=status commands=20 handlers=20 symbols=20 profiles=20 boundaries=7 alternative=go run ./cmd/rekit -- -Command <command>")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "profileSummary total=20 readOnly=5 mutating=15 writesCase=14 writesKit=1 reviewFirst=3 applyRequired=12 heavyTool=0 authorityConfirmed=0")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "profileGroups readOnly=doctor,packs,release-check,status,validate reviewFirst=promote,sync,update writesKit=promote")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "profileBoundaries rows=7 caseLocalApply=attach,bootstrap,continue,gate,handoff,init,reconcile,repair,start kitReviewFirst=promote readOnly=doctor,packs,release-check,status,validate")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "profilePolicies rows=5 violations=0")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "facadeRemovalReady=true prerequisites=5")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "unsupportedDiagnostic=true")
	assertReleaseHandoffSignalDetail(t, handoff, "public facade removal prerequisites", "ready=true prerequisites=8")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "removalPlan=true planChecks=9")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "replacementEntrypoints=4")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "replacementValidationCommands=32")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGates=5")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateValidationCommands=40")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateExitCriteria=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateFailureSignals=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationTriggers=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationEvidence=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationRecipients=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationHandoffSteps=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationDecisionOptions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationRetryConditions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationStopConditions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationResolutionArtifacts=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationClosureChecks=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationReopenConditions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationLedgerEvents=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationStateTransitions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationBoundaryGuards=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateEscalationAuditChecks=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateVerificationArtifacts=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateBlockedExecutionSteps=10")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "deletionGateRemediationActions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "recoverySteps=4")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "recoveryValidationCommands=32")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "documentationTargets=9")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "documentationValidationCommands=72")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionSteps=5")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionFailureSignals=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionRemediationActions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionVerificationArtifacts=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionLedgerEvents=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionStateTransitions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationTriggers=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationEvidence=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationRecipients=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationHandoffSteps=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationDecisionOptions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationRetryConditions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationStopConditions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationResolutionArtifacts=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationClosureChecks=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationReopenConditions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationLedgerEvents=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationStateTransitions=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationBoundaryGuards=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionEscalationAuditChecks=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionBoundaryGuards=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionAuditChecks=15")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "executionValidationCommands=40")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "boundaryChecks=6")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "boundaryValidationCommands=48")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "removalImpact=true impactReferences=")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "workItems=")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "validationCommands=")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "migrationTargets=74")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "migrationValidationCommands=592")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "smokeMigrationTargets=29")
	assertReleaseHandoffSignalDetailContains(t, handoff, "public facade removal prerequisites", "smokeMigrationValidationCommands=232")
	assertReleaseHandoffSignalDetail(t, handoff, "public facade removal prerequisites", "public-facade-retained-boundary ready=true publicFacadeReady=true present=true retained=true migrationBoundary=true removalBoundary=true")
	assertReleaseHandoffSignalDetail(t, handoff, "public facade removal prerequisites", "go-native-public-surface ready=true goNativeReady=true facadeRemovalReady=true prerequisites=5")
	assertReleaseHandoffSignal(t, handoff, "case shim readiness")
	assertReleaseHandoffSignal(t, handoff, "public default docs")
	assertReleaseHandoffSignal(t, handoff, "heavy-tool gate manifests")
	assertReleaseHandoffSignal(t, handoff, "pack maturity summary")
	assertReleaseHandoffPackMaturity(t, handoff)
	assertReleaseHandoffSignal(t, handoff, "latest batch documentation")
	assertReleaseHandoffSignal(t, handoff, "release notes freshness")
	assertReleaseHandoffSignal(t, handoff, "known gaps summary")
	assertReleaseHandoffKnownGap(t, handoff, "dispatch")
	assertReleaseHandoffKnownGap(t, handoff, "heavy-tool")
	assertReleaseHandoffKnownGap(t, handoff, "authority")
	assertReleaseHandoffKnownGap(t, handoff, "policy-schema")
	assertReleaseHandoffKnownGap(t, handoff, "powershell-deprecation")
	if handoff.ReleaseNotes.Path != "CHANGELOG.md" || !handoff.ReleaseNotes.Present || handoff.ReleaseNotes.Section != "Unreleased" || handoff.ReleaseNotes.LatestBatchID != handoff.LatestBatch.BatchID || !handoff.ReleaseNotes.Covered || handoff.ReleaseNotes.Summary != "release notes cover latest batch" {
		t.Fatalf("unexpected release handoff release notes: %+v", handoff.ReleaseNotes)
	}
	if handoff.LatestBatch.PlanPath != "docs/batch-plan.md" || !handoff.LatestBatch.Present || !strings.Contains(handoff.LatestBatch.Title, "Batch ") || !strings.Contains(handoff.LatestBatch.Status, "已完成") || strings.TrimSpace(handoff.LatestBatch.Goal) == "" || strings.TrimSpace(handoff.LatestBatch.ValidationResult) == "" {
		t.Fatalf("unexpected release handoff latest batch: %+v", handoff.LatestBatch)
	}
}

func assertReleaseHandoffReadFirst(t *testing.T, handoff releasecheck.ReleaseHandoff, path string) {
	t.Helper()
	for _, doc := range handoff.ReadFirst {
		if doc.Path == path {
			if !doc.Present || strings.TrimSpace(doc.Purpose) == "" {
				t.Fatalf("release handoff read-first doc %s = %+v, want present with purpose", path, doc)
			}
			return
		}
	}
	t.Fatalf("release handoff missing read-first doc %s: %+v", path, handoff.ReadFirst)
}

func assertReleaseHandoffSignal(t *testing.T, handoff releasecheck.ReleaseHandoff, name string) {
	t.Helper()
	for _, signal := range handoff.Signals {
		if signal.Name == name {
			if !signal.Ready || strings.TrimSpace(signal.Summary) == "" || len(signal.Details) == 0 {
				t.Fatalf("release handoff signal %s = %+v, want ready with summary/details", name, signal)
			}
			return
		}
	}
	t.Fatalf("release handoff missing signal %s: %+v", name, handoff.Signals)
}

func assertReleaseHandoffSignalDetail(t *testing.T, handoff releasecheck.ReleaseHandoff, name, detail string) {
	t.Helper()
	for _, signal := range handoff.Signals {
		if signal.Name == name {
			if !signal.Ready || strings.TrimSpace(signal.Summary) == "" || len(signal.Details) == 0 {
				t.Fatalf("release handoff signal %s = %+v, want ready with summary/details", name, signal)
			}
			if slices.Contains(signal.Details, detail) {
				return
			}
			t.Fatalf("release handoff signal %s missing detail %q: %+v", name, detail, signal.Details)
		}
	}
	t.Fatalf("release handoff missing signal %s: %+v", name, handoff.Signals)
}

func assertReleaseHandoffSignalDetailContains(t *testing.T, handoff releasecheck.ReleaseHandoff, name, detail string) {
	t.Helper()
	for _, signal := range handoff.Signals {
		if signal.Name == name {
			if !signal.Ready || strings.TrimSpace(signal.Summary) == "" || len(signal.Details) == 0 {
				t.Fatalf("release handoff signal %s = %+v, want ready with summary/details", name, signal)
			}
			for _, actual := range signal.Details {
				if strings.Contains(actual, detail) {
					return
				}
			}
			t.Fatalf("release handoff signal %s missing detail containing %q: %+v", name, detail, signal.Details)
		}
	}
	t.Fatalf("release handoff missing signal %s: %+v", name, handoff.Signals)
}

func assertReleaseHandoffPackMaturity(t *testing.T, handoff releasecheck.ReleaseHandoff) {
	t.Helper()
	inventory := handoff.PackMaturity
	if inventory.Total != 10 || !inventory.SchemaValid || !inventory.SchemaVersionReady || !inventory.HeavyToolGateReady || inventory.Summary != "pack maturity inventory ok" {
		t.Fatalf("unexpected release handoff pack maturity inventory: %+v", inventory)
	}
	if inventory.MaturityCounts["template"] != 1 || inventory.MaturityCounts["mature"] != 1 || inventory.MaturityCounts["skeleton"] != 8 {
		t.Fatalf("unexpected release handoff maturity counts: %+v", inventory.MaturityCounts)
	}
	if strings.Join(inventory.HeavyToolGateActions, ",") != "debug,dump,full-trace,inject,network,patch,symex" {
		t.Fatalf("unexpected release handoff heavy-tool gate actions: %v", inventory.HeavyToolGateActions)
	}
	assertReleaseHandoffMaturityPack(t, inventory.PacksByMaturity, "template", "_template")
	assertReleaseHandoffMaturityPack(t, inventory.PacksByMaturity, "mature", defaults.DefaultPack)
	assertReleaseHandoffMaturityPack(t, inventory.PacksByMaturity, "skeleton", "web-security")
	counts := releasecheck.ReleaseHandoffPackMaturityCountsFor(inventory)
	if counts.HeavyToolGatesByPack != counts.Total {
		t.Fatalf("release handoff pack gate rows = %d, want total %d", counts.HeavyToolGatesByPack, counts.Total)
	}
	for _, pack := range inventory.HeavyToolGatesByPack {
		if strings.TrimSpace(pack.ID) == "" || strings.TrimSpace(pack.Maturity) == "" || !pack.SchemaValid || pack.SchemaVersion != "1" || pack.HeavyToolGates == 0 || len(pack.Actions) == 0 {
			t.Fatalf("unexpected release handoff pack gate row: %+v", pack)
		}
	}
}

func assertReleaseHandoffMaturityPack(t *testing.T, packsByMaturity map[string][]string, maturity, packID string) {
	t.Helper()
	if slices.Contains(packsByMaturity[maturity], packID) {
		return
	}
	t.Fatalf("release handoff pack maturity %s missing %s: %+v", maturity, packID, packsByMaturity)
}

func assertReleaseHandoffKnownGap(t *testing.T, handoff releasecheck.ReleaseHandoff, category string) {
	t.Helper()
	for _, gap := range handoff.KnownGaps {
		if strings.Contains(gap.Category, category) {
			if gap.Index <= 0 || strings.TrimSpace(gap.Summary) == "" {
				t.Fatalf("release handoff known gap %s = %+v, want index and summary", category, gap)
			}
			return
		}
	}
	t.Fatalf("release handoff missing known gap category %s: %+v", category, handoff.KnownGaps)
}

func assertReleaseCheckGoNativePublicSurface(t *testing.T, surface struct {
	Ready                               bool                                             `json:"ready"`
	Summary                             string                                           `json:"summary"`
	Entrypoint                          string                                           `json:"entrypoint"`
	EntrypointPresent                   bool                                             `json:"entrypointPresent"`
	CommandCatalogPath                  string                                           `json:"commandCatalogPath"`
	CommandCatalogPresent               bool                                             `json:"commandCatalogPresent"`
	DefaultCommand                      string                                           `json:"defaultCommand"`
	Commands                            []string                                         `json:"commands"`
	HandlerCommands                     []string                                         `json:"handlerCommands"`
	SymbolCommands                      map[string]string                                `json:"symbolCommands"`
	CommandProfiles                     []commands.PublicProfile                         `json:"commandProfiles"`
	CommandProfileSummary               commands.PublicProfileSummary                    `json:"commandProfileSummary"`
	CommandProfileGroups                commands.PublicProfileGroups                     `json:"commandProfileGroups"`
	CommandProfileBoundaries            []commands.PublicProfileBoundary                 `json:"commandProfileBoundaries"`
	CommandProfilePolicies              []commands.PublicProfilePolicy                   `json:"commandProfilePolicies"`
	FacadeRemovalReady                  bool                                             `json:"facadeRemovalReady"`
	FacadeRemovalPrerequisites          []releasecheck.GoNativePublicSurfacePrerequisite `json:"facadeRemovalPrerequisites"`
	MutationBoundaries                  []string                                         `json:"mutationBoundaries"`
	AlternativePattern                  string                                           `json:"alternativePattern"`
	UnsupportedCommandDiagnostic        string                                           `json:"unsupportedCommandDiagnostic"`
	UnsupportedCommandDiagnosticPresent bool                                             `json:"unsupportedCommandDiagnosticPresent"`
	Warnings                            []string                                         `json:"warnings"`
}) {
	t.Helper()
	surfaceCounts := releasecheck.GoNativePublicSurfaceCountsFor(surface)
	if !surface.Ready || surface.Summary != "Go-native public command surface inventory ok" || surface.Entrypoint != "cmd/rekit" || !surface.EntrypointPresent || surface.CommandCatalogPath != "internal/rekit/commands/commands.go" || !surface.CommandCatalogPresent || surface.DefaultCommand != "status" || surface.AlternativePattern != "go run ./cmd/rekit -- -Command <command>" || !surface.UnsupportedCommandDiagnosticPresent || surfaceCounts.Warnings != 0 {
		t.Fatalf("unexpected Go-native public surface inventory: %+v", surface)
	}
	coverageCounts := surfaceCounts.Coverage
	if surfaceCounts.Catalog.Commands != 20 || surfaceCounts.Catalog.Empty != 0 || surfaceCounts.Catalog.Duplicates != 0 || coverageCounts.Commands != 20 || coverageCounts.HandlerCommands != 20 || coverageCounts.SymbolCommands != 20 || coverageCounts.ProfileCommands != 20 || coverageCounts.CommandProfiles != 20 || coverageCounts.HandlerMissing != 0 || coverageCounts.HandlerUnknown != 0 || coverageCounts.SymbolMissing != 0 || coverageCounts.SymbolUnknown != 0 || coverageCounts.ProfileMissing != 0 || coverageCounts.ProfileUnknown != 0 || surfaceCounts.MutationBoundaryInventory.Rows != 7 || surfaceCounts.MutationBoundaryInventory.Unknown != 0 {
		t.Fatalf("Go-native public surface command coverage omitted expected commands: %+v", surface)
	}
	for _, command := range []string{"attach", "bootstrap", "continue", "doctor", "gate", "handoff", "init", "note", "overview", "packs", "plan-subagents", "promote", "reconcile", "release-check", "repair", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(surface.Commands, command) || !slices.Contains(surface.HandlerCommands, command) {
			t.Fatalf("Go-native public command %s missing from catalog or handler coverage: %+v", command, surface)
		}
	}
	if surfaceCounts.SymbolCatalog.Symbols != 20 || surfaceCounts.SymbolCatalog.EmptySymbols != 0 || surfaceCounts.SymbolCatalog.EmptyCommands != 0 || surface.SymbolCommands["PlanSubagents"] != "plan-subagents" || surface.SymbolCommands["ReleaseCheck"] != "release-check" {
		t.Fatalf("Go-native public symbol catalog drifted: %+v", surface.SymbolCommands)
	}
	profiles := map[string]commands.PublicProfile{}
	for _, profile := range surface.CommandProfiles {
		profiles[profile.Command] = profile
	}
	if surfaceCounts.ProfileCatalog.Rows != 20 || surfaceCounts.ProfileCatalog.Empty != 0 || surfaceCounts.ProfileCatalog.Duplicates != 0 || surfaceCounts.ProfileCatalog.UnknownBoundaries != 0 || surfaceCounts.ProfileCatalog.HeavyTool != 0 || surfaceCounts.ProfileCatalog.AuthorityConfirmed != 0 || surfaceCounts.ProfileCatalog.WritesKitNoReview != 0 || surfaceCounts.ProfileCatalog.ReviewNoApply != 0 || profiles["release-check"].MutationBoundary != commands.BoundaryReadOnly || profiles["release-check"].IsMutation || !profiles["promote"].WritesKit || !profiles["promote"].ReviewFirst || profiles["sync"].WritesKit || !profiles["sync"].WritesCase || !slices.Contains(surface.MutationBoundaries, commands.BoundaryKitReviewFirst) {
		t.Fatalf("Go-native public command profiles drifted: profiles=%+v boundaries=%+v", surface.CommandProfiles, surface.MutationBoundaries)
	}
	profileSummaryCounts := surfaceCounts.ProfileSummary
	if profileSummaryCounts.Total != 20 || surfaceCounts.ProfileTotal != 20 || profileSummaryCounts.ReadOnly != 5 || profileSummaryCounts.Mutating != 15 || profileSummaryCounts.WritesCase != 14 || profileSummaryCounts.WritesKit != 1 || profileSummaryCounts.ReviewFirst != 3 || profileSummaryCounts.ApplyRequired != 12 || profileSummaryCounts.HeavyTool != 0 || profileSummaryCounts.AuthorityConfirmed != 0 || profileSummaryCounts.BoundaryReadOnly != 5 || profileSummaryCounts.BoundaryCaseLocalApply != 9 || profileSummaryCounts.BoundaryCaseLocalReview != 2 || profileSummaryCounts.BoundaryKitReview != 1 {
		t.Fatalf("Go-native public command profile summary drifted: %+v", surface.CommandProfileSummary)
	}
	if strings.Join(surface.CommandProfileGroups.ReadOnly, ",") != "doctor,packs,release-check,status,validate" || strings.Join(surface.CommandProfileGroups.ReviewFirst, ",") != "promote,sync,update" || strings.Join(surface.CommandProfileGroups.WritesKit, ",") != "promote" || surfaceCounts.Groups.HeavyTool != 0 || surfaceCounts.Groups.AuthorityConfirmed != 0 || surfaceCounts.Groups.CaseLocalApply != 9 || surfaceCounts.Groups.CaseLocalReviewFirst != 2 {
		t.Fatalf("Go-native public command profile groups drifted: %+v", surface.CommandProfileGroups)
	}
	firstBoundaryCounts := releasecheck.GoNativePublicSurfaceBoundaryRowCountsFor(surface.CommandProfileBoundaries[0])
	lastBoundaryCounts := releasecheck.GoNativePublicSurfaceBoundaryRowCountsFor(surface.CommandProfileBoundaries[len(surface.CommandProfileBoundaries)-1])
	if surfaceCounts.Boundaries.Rows != 7 || surfaceCounts.Boundaries.Commands != 20 || surfaceCounts.Boundaries.CountedCommands != 20 || surfaceCounts.Boundaries.Unknown != 0 || surfaceCounts.Boundaries.Duplicates != 0 || surfaceCounts.Boundaries.CountMismatches != 0 || surfaceCounts.Boundaries.Unsorted != 0 || surfaceCounts.Boundaries.SummaryMismatches != 0 || surfaceCounts.Boundaries.GroupMismatches != 0 || surfaceCounts.Boundaries.Missing != 0 || surfaceCounts.Boundaries.CoverageMismatches != 0 || surface.CommandProfileBoundaries[0].Boundary != commands.BoundaryCaseLocalAppend || firstBoundaryCounts.Count != 1 || firstBoundaryCounts.Commands != 1 || strings.Join(surface.CommandProfileBoundaries[1].Commands, ",") != "attach,bootstrap,continue,gate,handoff,init,reconcile,repair,start" || surface.CommandProfileBoundaries[len(surface.CommandProfileBoundaries)-1].Boundary != commands.BoundaryReadOnly || lastBoundaryCounts.Count != 5 || lastBoundaryCounts.Commands != 5 {
		t.Fatalf("Go-native public command profile boundary rows drifted: %+v", surface.CommandProfileBoundaries)
	}
	if surfaceCounts.Policies.Rows != 5 || surfaceCounts.Policies.Violations != 0 || surfaceCounts.Policies.ViolationCommands != 0 || surface.CommandProfilePolicies[0].Policy != commands.PublicProfilePolicyNoHeavyTool || !surface.CommandProfilePolicies[0].Ready || surface.CommandProfilePolicies[3].Policy != commands.PublicProfilePolicyReviewFirstApplyRequired || releasecheck.GoNativePublicSurfacePolicyRowCountsFor(surface.CommandProfilePolicies[3]).Commands != 0 {
		t.Fatalf("Go-native public command profile policy rows drifted: %+v", surface.CommandProfilePolicies)
	}
	if !surface.FacadeRemovalReady || surfaceCounts.FacadeRemoval.Rows != 5 || surfaceCounts.FacadeRemoval.NotReady != 0 || surface.FacadeRemovalPrerequisites[0].Name != "entrypoint" || !surface.FacadeRemovalPrerequisites[0].Ready || surface.FacadeRemovalPrerequisites[4].Name != "unsupported-command-diagnostic" || !surface.FacadeRemovalPrerequisites[4].Ready {
		t.Fatalf("Go-native public surface facade removal prerequisites drifted: ready=%t prerequisites=%+v", surface.FacadeRemovalReady, surface.FacadeRemovalPrerequisites)
	}
}

func assertReleaseCheckPublicFacadeRemoval(t *testing.T, inventory releasecheck.PublicFacadeRemoval) {
	t.Helper()
	counts := releasecheck.PublicFacadeRemovalCountsFor(inventory)
	if !inventory.Ready || inventory.Summary != "public facade removal prerequisites ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected public facade removal inventory: %+v", inventory)
	}
	if counts.Prerequisites != 8 || inventory.Prerequisites[0].Name != "public-facade-retained-boundary" || !inventory.Prerequisites[0].Ready || inventory.Prerequisites[2].Name != "go-native-public-surface" || !inventory.Prerequisites[2].Ready || inventory.Prerequisites[5].Name != "module-reference-blockers-clear" || !inventory.Prerequisites[5].Ready || inventory.Prerequisites[6].Name != "removal-plan-documented" || !inventory.Prerequisites[6].Ready || inventory.Prerequisites[7].Name != "removal-impact-inventoried" || !inventory.Prerequisites[7].Ready {
		t.Fatalf("public facade removal prerequisites drifted: %+v", inventory.Prerequisites)
	}
	planCounts := counts.Plan
	deletionGateCounts := counts.DeletionGates
	executionCounts := counts.ExecutionSteps
	impactCounts := counts.Impact
	if !inventory.RemovalPlan.Ready || inventory.RemovalPlan.Document != "docs/powershell-deprecation.md" || planCounts.Warnings != 0 || planCounts.RequiredPhrases != 9 || planCounts.ReplacementEntrypoints != 4 || planCounts.ReplacementValidationCommands != 32 || deletionGateCounts.Gates != 5 || deletionGateCounts.ValidationCommands != 40 || deletionGateCounts.ExitCriteria != 15 || deletionGateCounts.FailureSignals != 15 || deletionGateCounts.EscalationTriggers != 15 || deletionGateCounts.EscalationEvidence != 15 || deletionGateCounts.EscalationRecipients != 15 || deletionGateCounts.EscalationHandoffSteps != 15 || deletionGateCounts.EscalationDecisionOptions != 15 || deletionGateCounts.EscalationRetryConditions != 15 || deletionGateCounts.EscalationStopConditions != 15 || deletionGateCounts.EscalationResolutionArtifacts != 15 || deletionGateCounts.EscalationClosureChecks != 15 || deletionGateCounts.EscalationReopenConditions != 15 || deletionGateCounts.EscalationLedgerEvents != 15 || deletionGateCounts.EscalationStateTransitions != 15 || deletionGateCounts.EscalationBoundaryGuards != 15 || deletionGateCounts.EscalationAuditChecks != 15 || deletionGateCounts.VerificationArtifacts != 15 || deletionGateCounts.BlockedExecutionSteps != 10 || deletionGateCounts.RemediationActions != 15 || executionCounts.Steps != 5 || executionCounts.FailureSignals != 15 || executionCounts.RemediationActions != 15 || executionCounts.VerificationArtifacts != 15 || executionCounts.LedgerEvents != 15 || executionCounts.StateTransitions != 15 || executionCounts.EscalationTriggers != 15 || executionCounts.EscalationEvidence != 15 || executionCounts.EscalationRecipients != 15 || executionCounts.EscalationHandoffSteps != 15 || executionCounts.EscalationDecisionOptions != 15 || executionCounts.EscalationRetryConditions != 15 || executionCounts.EscalationStopConditions != 15 || executionCounts.EscalationResolutionArtifacts != 15 || executionCounts.EscalationClosureChecks != 15 || executionCounts.EscalationReopenConditions != 15 || executionCounts.EscalationLedgerEvents != 15 || executionCounts.EscalationStateTransitions != 15 || executionCounts.EscalationBoundaryGuards != 15 || executionCounts.EscalationAuditChecks != 15 || executionCounts.BoundaryGuards != 15 || executionCounts.AuditChecks != 15 || executionCounts.ValidationCommands != 40 || planCounts.BoundaryChecks != 6 || planCounts.BoundaryValidationCommands != 48 || planCounts.RecoverySteps != 4 || planCounts.RecoveryValidationCommands != 32 || planCounts.DocumentationTargets != 9 || planCounts.DocumentationValidationCommands != 72 || !releaseCheckPublicFacadeRemovalHasReplacementEntrypoint(inventory.RemovalPlan, "canonical-rekit-skill") || !releaseCheckPublicFacadeRemovalHasReplacementEntrypoint(inventory.RemovalPlan, "direct-go-cli") || !releaseCheckPublicFacadeRemovalHasDeletionGate(inventory.RemovalPlan, "go-native-alternatives-ready") || !releaseCheckPublicFacadeRemovalHasDeletionGate(inventory.RemovalPlan, "release-gate-green") || !releaseCheckPublicFacadeRemovalHasExecutionStep(inventory.RemovalPlan, "delete-public-facade") || !releaseCheckPublicFacadeRemovalHasExecutionStep(inventory.RemovalPlan, "rerun-release-gate") || !releaseCheckPublicFacadeRemovalHasBoundaryCheck(inventory.RemovalPlan, "no-powershell-runtime-logic") || !releaseCheckPublicFacadeRemovalHasBoundaryCheck(inventory.RemovalPlan, "no-external-effects") || !releaseCheckPublicFacadeRemovalHasRecoveryStep(inventory.RemovalPlan, "restore-public-facade") || !releaseCheckPublicFacadeRemovalHasDocumentationTarget(inventory.RemovalPlan, "docs/release-readiness.md") || !releaseCheckPublicFacadeRemovalHasDocumentationTarget(inventory.RemovalPlan, "CHANGELOG.md") {
		t.Fatalf("public facade removal plan drifted: %+v", inventory.RemovalPlan)
	}
	if !inventory.RemovalImpact.Ready || inventory.RemovalImpact.FacadePath != "rekit/rekit.ps1" || !inventory.RemovalImpact.FacadePresent || impactCounts.Warnings != 0 || impactCounts.References == 0 || impactCounts.ReferenceCategories == 0 || impactCounts.WorkItems != impactCounts.ReferenceCategories || impactCounts.WorkItemValidationCommands != impactCounts.WorkItems*8 || impactCounts.MigrationTargets != 74 || impactCounts.MigrationValidationCommands != 592 || impactCounts.SmokeMigrationTargets != 29 || impactCounts.SmokeMigrationValidationCommands != 232 || impactCounts.UnclassifiedReferences != 0 || !releaseCheckPublicFacadeRemovalHasImpactCategory(inventory.RemovalImpact, "public-facade-entrypoint") || !releaseCheckPublicFacadeRemovalHasImpactCategory(inventory.RemovalImpact, "facade-compatibility-smoke") || !releaseCheckPublicFacadeRemovalHasImpactWorkItem(inventory.RemovalImpact, "release-inventory-and-tests") || !releaseCheckPublicFacadeRemovalHasMigrationTarget(inventory.RemovalImpact, "rekit/rekit.ps1") || !releaseCheckPublicFacadeRemovalHasMigrationTarget(inventory.RemovalImpact, "docs/powershell-deprecation.md") || !releaseCheckPublicFacadeRemovalHasSmokeMigrationTarget(inventory.RemovalImpact, "rekit/tests/facade-smoke.ps1") || !releaseCheckPublicFacadeRemovalHasSmokeMigrationTarget(inventory.RemovalImpact, "rekit/tests/continue-whatif-smoke.ps1") {
		t.Fatalf("public facade removal impact drifted: %+v", inventory.RemovalImpact)
	}
}

func releaseCheckPublicFacadeRemovalHasValidationSmoke(count int, commands []string) bool {
	return count == 8 && slices.Contains(commands, "go run ./cmd/rekit -- -Command release-check -Format json")
}

func releaseCheckPublicFacadeRemovalHasReplacementEntrypoint(plan releasecheck.PublicFacadeRemovalPlan, name string) bool {
	for _, entrypoint := range plan.ReplacementEntrypoints {
		counts := releasecheck.PublicFacadeRemovalReplacementEntrypointCountsFor(entrypoint)
		if entrypoint.Name == name && entrypoint.Required && entrypoint.GoNativeBacked && strings.TrimSpace(entrypoint.Entrypoint) != "" && strings.TrimSpace(entrypoint.Audience) != "" && strings.TrimSpace(entrypoint.Purpose) != "" && releaseCheckPublicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, entrypoint.ValidationCommands) {
			return true
		}
	}
	return false
}

func releaseCheckPublicFacadeRemovalHasDeletionGate(plan releasecheck.PublicFacadeRemovalPlan, name string) bool {
	for _, gate := range plan.DeletionGates {
		counts := releasecheck.PublicFacadeRemovalDeletionGateRowCountsFor(gate)
		if gate.Name == name && gate.Required && gate.BlocksRemoval && strings.TrimSpace(gate.Gate) != "" && counts.InputInventory > 0 && counts.BlockedExecutionSteps == 2 && slices.Contains(gate.BlockedExecutionSteps, "delete-public-facade") && slices.Contains(gate.BlockedExecutionSteps, "rerun-release-gate") && counts.ExitCriteria == 3 && counts.FailureSignals == 3 && counts.EscalationTriggers == 3 && counts.EscalationEvidence == 3 && counts.EscalationRecipients == 3 && counts.EscalationHandoffSteps == 3 && counts.EscalationDecisionOptions == 3 && counts.EscalationRetryConditions == 3 && counts.EscalationStopConditions == 3 && counts.EscalationResolutionArtifacts == 3 && counts.EscalationClosureChecks == 3 && counts.EscalationReopenConditions == 3 && counts.EscalationLedgerEvents == 3 && counts.EscalationStateTransitions == 3 && counts.EscalationBoundaryGuards == 3 && counts.EscalationAuditChecks == 3 && counts.VerificationArtifacts == 3 && counts.RemediationActions == 3 && releaseCheckPublicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, gate.ValidationCommands) {
			return true
		}
	}
	return false
}

func releaseCheckPublicFacadeRemovalHasExecutionStep(plan releasecheck.PublicFacadeRemovalPlan, name string) bool {
	for _, step := range plan.ExecutionSteps {
		counts := releasecheck.PublicFacadeRemovalExecutionStepRowCountsFor(step)
		if step.Name == name && step.Required && strings.TrimSpace(step.Action) != "" && counts.DependsOn > 0 && counts.InputInventory > 0 && counts.OutputArtifacts > 0 && counts.FailureSignals == 3 && counts.RemediationActions == 3 && counts.VerificationArtifacts == 3 && counts.LedgerEvents == 3 && counts.StateTransitions == 3 && counts.EscalationTriggers == 3 && counts.EscalationEvidence == 3 && counts.EscalationRecipients == 3 && counts.EscalationHandoffSteps == 3 && counts.EscalationDecisionOptions == 3 && counts.EscalationRetryConditions == 3 && counts.EscalationStopConditions == 3 && counts.EscalationResolutionArtifacts == 3 && counts.EscalationClosureChecks == 3 && counts.EscalationReopenConditions == 3 && counts.EscalationLedgerEvents == 3 && counts.EscalationStateTransitions == 3 && counts.EscalationBoundaryGuards == 3 && counts.EscalationAuditChecks == 3 && counts.BoundaryGuards == 3 && counts.AuditChecks == 3 && releaseCheckPublicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, step.ValidationCommands) && !step.AllowsPowerShellRuntime && !step.AllowsExternalEffects {
			return true
		}
	}
	return false
}

func releaseCheckPublicFacadeRemovalHasBoundaryCheck(plan releasecheck.PublicFacadeRemovalPlan, name string) bool {
	for _, check := range plan.BoundaryChecks {
		counts := releasecheck.PublicFacadeRemovalPlanBoundaryCheckCountsFor(check)
		if check.Name == name && check.Required && check.Preserved && strings.TrimSpace(check.Boundary) != "" && counts.Evidence > 0 && releaseCheckPublicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, check.ValidationCommands) {
			return true
		}
	}
	return false
}

func releaseCheckPublicFacadeRemovalHasRecoveryStep(plan releasecheck.PublicFacadeRemovalPlan, name string) bool {
	for _, step := range plan.RecoverySteps {
		counts := releasecheck.PublicFacadeRemovalRecoveryStepCountsFor(step)
		if step.Name == name && step.Required && strings.TrimSpace(step.Action) != "" && counts.Paths > 0 && releaseCheckPublicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, step.ValidationCommands) {
			return true
		}
	}
	return false
}

func releaseCheckPublicFacadeRemovalHasImpactCategory(impact releasecheck.PublicFacadeRemovalImpact, name string) bool {
	for _, category := range impact.ReferenceCategories {
		if category.Name == name && category.Count > 0 {
			return true
		}
	}
	return false
}

func releaseCheckPublicFacadeRemovalHasImpactWorkItem(impact releasecheck.PublicFacadeRemovalImpact, category string) bool {
	for _, item := range impact.WorkItems {
		counts := releasecheck.PublicFacadeRemovalImpactWorkItemCountsFor(item)
		if item.Category == category && item.Required && item.Count > 0 && counts.Paths > 0 && strings.TrimSpace(item.Action) != "" && releaseCheckPublicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, item.ValidationCommands) {
			return true
		}
	}
	return false
}

func releaseCheckPublicFacadeRemovalHasDocumentationTarget(plan releasecheck.PublicFacadeRemovalPlan, path string) bool {
	for _, target := range plan.DocumentationTargets {
		counts := releasecheck.PublicFacadeRemovalDocumentationTargetCountsFor(target)
		if target.Path == path && target.Required && strings.TrimSpace(target.Purpose) != "" && strings.TrimSpace(target.Action) != "" && releaseCheckPublicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, target.ValidationCommands) {
			return true
		}
	}
	return false
}

func releaseCheckPublicFacadeRemovalHasMigrationTarget(impact releasecheck.PublicFacadeRemovalImpact, path string) bool {
	for _, target := range impact.MigrationTargets {
		counts := releasecheck.PublicFacadeRemovalMigrationTargetCountsFor(target)
		if target.Path == path && target.Required && target.GoNativePreferred && strings.TrimSpace(target.Action) != "" && releaseCheckPublicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, target.ValidationCommands) {
			return true
		}
	}
	return false
}

func releaseCheckPublicFacadeRemovalHasSmokeMigrationTarget(impact releasecheck.PublicFacadeRemovalImpact, path string) bool {
	for _, target := range impact.SmokeMigrationTargets {
		counts := releasecheck.PublicFacadeRemovalSmokeMigrationTargetCountsFor(target)
		if target.Path == path && target.Required && target.GoNativePreferred && !target.AllowFacadeCompat && target.RetireFacadeAssertions && strings.TrimSpace(target.Action) != "" && releaseCheckPublicFacadeRemovalHasValidationSmoke(counts.ValidationCommands, target.ValidationCommands) {
			return true
		}
	}
	return false
}

func assertReleaseCheckCaseShim(t *testing.T, shim caseshim.Readiness) {
	t.Helper()
	counts := caseshim.ReadinessCountsFor(shim)
	if !shim.Ready || shim.TemplatePath != "rekit/templates/case-shim/SKILL.md" || shim.CanonicalSkillPath != ".claude/skills/rekit/SKILL.md" || shim.Summary != "case shim readiness ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected case shim readiness inventory: %+v", shim)
	}
	if counts.RequiredPhrases == 0 || counts.CanonicalSkillPhrases == 0 || counts.ForbiddenStrings == 0 || counts.Boundaries == 0 {
		t.Fatalf("case shim readiness omitted required sections: %+v", shim)
	}
	assertCaseShimPhrase(t, shim.RequiredPhrases, "Go-native backend")
	assertCaseShimPhrase(t, shim.RequiredPhrases, "不展示底层脚本或 CLI 命令")
	for _, forbidden := range shim.ForbiddenStrings {
		if forbidden.Present {
			t.Fatalf("case shim forbidden pattern present: %+v", forbidden)
		}
	}
}

func assertCaseShimPhrase(t *testing.T, checks []caseshim.PhraseCheck, want string) {
	t.Helper()
	for _, check := range checks {
		if check.Phrase == want {
			if !check.Present {
				t.Fatalf("case shim phrase %q present=false", want)
			}
			return
		}
	}
	t.Fatalf("case shim missing phrase check %q: %+v", want, checks)
}

func assertReleaseCheckPublicDefaultDocs(t *testing.T, docs defaultdocs.Readiness) {
	t.Helper()
	counts := defaultdocs.ReadinessCountsFor(docs)
	if !docs.Ready || docs.Summary != "public default docs readiness ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected public default docs readiness inventory: %+v", docs)
	}
	if counts.Documents != 13 || counts.RequiredPhrases == 0 || counts.Boundaries == 0 {
		t.Fatalf("public default docs readiness omitted required sections: %+v", docs)
	}
	assertPublicDefaultDoc(t, docs, "README.md")
	assertPublicDefaultDoc(t, docs, ".claude/skills/rekit/SKILL.md")
	assertPublicDefaultDoc(t, docs, "CLAUDE.md")
	assertPublicDefaultDoc(t, docs, "docs/mission-control-product-direction.md")
	assertPublicDefaultDoc(t, docs, "docs/autonomous-goal.md")
	assertPublicDefaultDoc(t, docs, "docs/release-readiness.md")
	assertPublicDefaultDoc(t, docs, "docs/go-first-convergence-plan.md")
	assertPublicDefaultDoc(t, docs, "docs/go-runtime-migration.md")
	assertPublicDefaultDoc(t, docs, "docs/powershell-deprecation.md")
	assertPublicDefaultDoc(t, docs, "docs/vision.md")
	assertPublicDefaultDoc(t, docs, "docs/reference-absorption.md")
	assertPublicDefaultDoc(t, docs, "docs/agent-team-rollout-plan.md")
	assertPublicDefaultDoc(t, docs, "rekit/tests/README.md")
	assertPublicDefaultPhrase(t, docs, "README.md", "用户主要指挥主 Agent / Mission Commander")
	assertPublicDefaultPhrase(t, docs, "docs/mission-control-product-direction.md", "Lane-centric Agent Team Mission Control")
	assertPublicDefaultPhrase(t, docs, ".claude/skills/rekit/SKILL.md", "底层 Go CLI 是 canonical runtime")
	assertPublicDefaultPhrase(t, docs, "CLAUDE.md", "PowerShell-free / Go-native / 跨平台收敛")
	assertPublicDefaultPhrase(t, docs, "docs/autonomous-goal.md", "默认继续自主推进")
	assertPublicDefaultPhrase(t, docs, "docs/release-readiness.md", "默认本机验证路径不依赖 PowerShell")
	assertPublicDefaultPhrase(t, docs, "docs/go-first-convergence-plan.md", "不要把大型 PowerShell matrix 作为默认必跑")
	assertPublicDefaultPhrase(t, docs, "docs/go-runtime-migration.md", "当前默认验证应优先运行 Go-native release gate")
	assertPublicDefaultPhrase(t, docs, "docs/powershell-deprecation.md", "Go CLI/backend 是 canonical runtime")
	assertPublicDefaultPhrase(t, docs, "docs/vision.md", "优先运行 Go-native 检查")
	assertPublicDefaultPhrase(t, docs, "docs/reference-absorption.md", "Go-native release readiness 子集")
	assertPublicDefaultPhrase(t, docs, "docs/agent-team-rollout-plan.md", "公共 `/rekit` 默认路径继续向 Go-native / PowerShell-free 收敛")
	assertPublicDefaultPhrase(t, docs, "rekit/tests/README.md", "推荐最小回归组合")
	for _, forbidden := range docs.ForbiddenCommands {
		if forbidden.Present {
			t.Fatalf("public default docs forbidden command present: %+v", forbidden)
		}
	}
	for _, forbidden := range docs.ForbiddenShellFences {
		if forbidden.Present {
			t.Fatalf("public default docs forbidden shell fence present: %+v", forbidden)
		}
	}
}

func assertPublicDefaultDoc(t *testing.T, docs defaultdocs.Readiness, path string) {
	t.Helper()
	for _, doc := range docs.Documents {
		if doc.Path == path {
			if !doc.Present || strings.TrimSpace(doc.Purpose) == "" {
				t.Fatalf("public default doc %s = %+v, want present with purpose", path, doc)
			}
			return
		}
	}
	t.Fatalf("public default docs missing document %s: %+v", path, docs.Documents)
}

func assertPublicDefaultPhrase(t *testing.T, docs defaultdocs.Readiness, path, phrase string) {
	t.Helper()
	for _, check := range docs.RequiredPhrases {
		if check.Path == path && check.Phrase == phrase {
			if !check.Present {
				t.Fatalf("public default phrase %s/%q present=false", path, phrase)
			}
			return
		}
	}
	t.Fatalf("public default docs missing phrase check %s/%q: %+v", path, phrase, docs.RequiredPhrases)
}

func assertReleaseCheckPowerShellDeprecation(t *testing.T, inventory releaseCheckPowerShellDeprecation) {
	t.Helper()
	counts := releasecheck.PowerShellDeprecationCountsFor(inventory)
	if !inventory.Ready || inventory.StrategyDocument != "docs/powershell-deprecation.md" || inventory.Summary != "PowerShell deprecation inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell deprecation inventory: %+v", inventory)
	}
	if counts.CommandOwnership == 0 || counts.ModuleStatus == 0 || counts.FreezeGates == 0 || counts.BlockedMigrations == 0 {
		t.Fatalf("PowerShell deprecation inventory omitted required sections: %+v", inventory)
	}
	assertPowerShellCommandOwner(t, inventory, "release-check", true, false)
	assertPowerShellCommandOwner(t, inventory, "sync / update", true, false)
	assertPowerShellCommandOwner(t, inventory, "plan-subagents", true, false)
	assertPowerShellCommandOwner(t, inventory, "actual heavy-tool", false, true)
	assertPowerShellModuleStatus(t, inventory, "rekit/rekit.ps1")
	assertPowerShellModuleStatus(t, inventory, "rekit/lib/B3.Commands.ps1")
	assertPowerShellFallbackRetirement(t, inventory)
	assertPowerShellFacadeRuntime(t, inventory)
	assertPowerShellPublicFacade(t, inventory)
	assertPowerShellModuleRemoval(t, inventory)
	assertPowerShellModuleReferences(t, inventory)
}

func assertPowerShellCommandOwner(t *testing.T, inventory releaseCheckPowerShellDeprecation, areaContains string, wantGoDefault, wantBlocked bool) {
	t.Helper()
	for _, row := range inventory.CommandOwnership {
		if strings.Contains(row.Area, areaContains) {
			if row.GoDefault != wantGoDefault || row.Blocked != wantBlocked || strings.TrimSpace(row.Owner) == "" || strings.TrimSpace(row.Status) == "" || strings.TrimSpace(row.Strategy) == "" {
				t.Fatalf("PowerShell command owner row %q = %+v, want goDefault=%t blocked=%t with populated owner/status/strategy", areaContains, row, wantGoDefault, wantBlocked)
			}
			return
		}
	}
	t.Fatalf("PowerShell deprecation inventory missing command row containing %q: %+v", areaContains, inventory.CommandOwnership)
}

func assertPowerShellModuleStatus(t *testing.T, inventory releaseCheckPowerShellDeprecation, path string) {
	t.Helper()
	for _, module := range inventory.ModuleStatus {
		if module.Path == path {
			if strings.TrimSpace(module.Status) == "" || strings.TrimSpace(module.Notes) == "" {
				t.Fatalf("PowerShell module %s has empty status/notes: %+v", path, module)
			}
			return
		}
	}
	t.Fatalf("PowerShell deprecation inventory missing module %s: %+v", path, inventory.ModuleStatus)
}

func assertPowerShellFallbackRetirement(t *testing.T, inventory releaseCheckPowerShellDeprecation) {
	t.Helper()
	fallback := inventory.FallbackRetirement
	counts := releasecheck.PowerShellDeprecationCountsFor(inventory).FallbackRetirement
	if !fallback.Ready || fallback.Summary != "PowerShell fallback retirement inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell fallback retirement inventory: %+v", fallback)
	}
	if counts.GoDefaultCommands != 20 || counts.NoFallbackCommands != 20 || counts.CandidateCommands != 0 || counts.RemovalCandidateModules != 0 || counts.RetiredModules != 13 {
		t.Fatalf("fallback retirement inventory omitted expected sections: %+v", fallback)
	}
	for _, command := range []string{"attach", "bootstrap", "continue", "doctor", "gate", "handoff", "init", "note", "overview", "packs", "plan-subagents", "promote", "reconcile", "release-check", "repair", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(fallback.NoFallbackCommands, command) {
			t.Fatalf("NoFallbackCommands = %v, missing %s", fallback.NoFallbackCommands, command)
		}
	}
}

func assertPowerShellFacadeRuntime(t *testing.T, inventory releaseCheckPowerShellDeprecation) {
	t.Helper()
	facade := inventory.FacadeRuntime
	counts := releasecheck.PowerShellDeprecationCountsFor(inventory).FacadeRuntime
	if !facade.Ready || facade.Summary != "PowerShell facade runtime dependency inventory ok" || facade.FacadePath != "rekit/rekit.ps1" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell facade runtime inventory: %+v", facade)
	}
	if facade.LegacyModuleImportsPresent || facade.CommandDispatcherPresent || !facade.NoFallbackGuardPresent || !facade.GoDelegationPresent || !facade.RetiredDispatcherError {
		t.Fatalf("unexpected PowerShell facade runtime dependency flags: %+v", facade)
	}
	if counts.ForbiddenPatterns == 0 || counts.RequiredPatterns == 0 {
		t.Fatalf("PowerShell facade runtime inventory omitted required pattern lists: %+v", facade)
	}
}

func assertPowerShellPublicFacade(t *testing.T, inventory releaseCheckPowerShellDeprecation) {
	t.Helper()
	facade := inventory.PublicFacade
	counts := releasecheck.PowerShellDeprecationCountsFor(inventory).PublicFacade
	if !facade.Ready || facade.Summary != "PowerShell public facade retention inventory ok" || facade.FacadePath != "rekit/rekit.ps1" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell public facade inventory: %+v", facade)
	}
	if !facade.Present || !facade.Retained || !facade.MigrationBoundaryDocumented || !facade.RemovalBoundaryDocumented || facade.GoNativeAlternative != "go run ./cmd/rekit -- -Command <command>" {
		t.Fatalf("unexpected PowerShell public facade retention flags: %+v", facade)
	}
	if counts.CommandSurface != 20 || counts.GoDefaultCommands != 20 || counts.NoFallbackCommands != 20 {
		t.Fatalf("PowerShell public facade inventory omitted expected command lists: %+v", facade)
	}
	for _, command := range []string{"attach", "bootstrap", "continue", "doctor", "gate", "handoff", "init", "note", "overview", "packs", "plan-subagents", "promote", "reconcile", "release-check", "repair", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(facade.CommandSurface, command) || !slices.Contains(facade.GoDefaultCommands, command) || !slices.Contains(facade.NoFallbackCommands, command) {
			t.Fatalf("public facade command %s missing from command lists: %+v", command, facade)
		}
	}
}

func assertPowerShellModuleRemoval(t *testing.T, inventory releaseCheckPowerShellDeprecation) {
	t.Helper()
	removal := inventory.ModuleRemoval
	counts := releasecheck.PowerShellDeprecationCountsFor(inventory).ModuleRemoval
	if !removal.Ready || removal.Summary != "PowerShell module removal inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell module removal inventory: %+v", removal)
	}
	if counts.CandidateModules != 0 || counts.RetiredModules != 13 || counts.FacadeRuntimeDependencies != 0 || counts.UndocumentedModules != 0 {
		t.Fatalf("PowerShell module removal inventory omitted expected sections: %+v", removal)
	}
	for _, module := range removal.RetiredModules {
		if strings.TrimSpace(module.Path) == "" || strings.TrimSpace(module.Status) == "" || strings.TrimSpace(module.Notes) == "" || module.Present || module.ReferencedByFacade {
			t.Fatalf("unexpected PowerShell retired module: %+v", module)
		}
	}
}

func assertPowerShellModuleReferences(t *testing.T, inventory releaseCheckPowerShellDeprecation) {
	t.Helper()
	refs := inventory.ModuleReferences
	counts := releasecheck.PowerShellDeprecationCountsFor(inventory).ModuleReferences
	if !refs.Ready || refs.Summary != "PowerShell module reference inventory ok" || counts.Warnings != 0 {
		t.Fatalf("unexpected PowerShell module reference inventory: %+v", refs)
	}
	if counts.TotalReferences == 0 || counts.ActiveTestDependencies != 0 || counts.CompatibilityFixtures != 0 || counts.InventoryGuards == 0 || counts.RemovalBlockers != 0 || counts.UnclassifiedReferences != 0 {
		t.Fatalf("PowerShell module reference inventory omitted expected sections: %+v", refs)
	}
}

func TestRunReleaseCheckTextInventory(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"-Command", "release-check"}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{
		"release-check: release gate inventory ok",
		"ready: true",
		"gate profile: local-ci-minimum ready=true",
		"CI release gate: .github/workflows/release-gate.yml ready=true jobs=3 commands=18 forbidden=12",
		"required commands:",
		"go run ./cmd/rekit -- -Command release-check -Format json kind=go-run path=cmd/rekit",
		"go run ./cmd/rekit -- -Command status kind=go-run path=cmd/rekit",
		"go run ./cmd/rekit -- -Command packs kind=go-run path=cmd/rekit",
		"go run ./cmd/rekit -- -Command doctor kind=go-run path=cmd/rekit",
		"go test ./... kind=go-check",
		"documents:",
		"docs/release-readiness.md",
		"docs/mission-control-product-direction.md",
		"docs/autonomous-goal.md",
		"packs:",
		"heavy-tool gate actions: debug,dump,full-trace,inject,network,patch,symex",
		"PowerShell deprecation: PowerShell deprecation inventory ok ready=true",
		"commands=14 modules=14 freezeGates=10 blocked=5 fallbackRetirement=true noFallback=20 candidates=0 removalModules=0 retiredModules=13 facadeRuntime=true legacyImports=false dispatcher=false publicFacade=true retained=true facadeCommands=20 noFallback=20 moduleRemoval=true removalCandidates=0 retired=13 facadeDeps=0 undocumented=0 moduleReferences=true activeTests=0 fixtures=0 blockers=0 unclassified=0",
		"Go-native public surface: Go-native public command surface inventory ok ready=true entrypoint=cmd/rekit present=true catalog=internal/rekit/commands/commands.go catalogPresent=true default=status commands=20 handlers=20 symbols=20 profiles=20 boundaries=7 boundaryRows=7 policyRows=5 policyViolations=0 facadeRemovalReady=true facadePrerequisites=5 readOnly=5 mutating=15 writesCase=14 writesKit=1 reviewFirst=3 applyRequired=12 heavyTool=0 authorityConfirmed=0 readOnlyCommands=doctor,packs,release-check,status,validate reviewFirstCommands=promote,sync,update writesKitCommands=promote caseLocalApplyCommands=attach,bootstrap,continue,gate,handoff,init,reconcile,repair,start kitReviewFirstCommands=promote alternative=go run ./cmd/rekit -- -Command <command> unsupportedDiagnostic=true",
		"case shim: case shim readiness ok ready=true",
		"public default docs: public default docs readiness ok ready=true documents=13",
		"public facade removal: public facade removal prerequisites ok ready=true prerequisites=8 removalPlan=true planChecks=9 replacementEntrypoints=4 replacementValidationCommands=32 deletionGates=5 deletionGateValidationCommands=40 deletionGateExitCriteria=15 deletionGateFailureSignals=15 deletionGateEscalationTriggers=15 deletionGateEscalationEvidence=15 deletionGateEscalationRecipients=15 deletionGateEscalationHandoffSteps=15 deletionGateEscalationDecisionOptions=15 deletionGateEscalationRetryConditions=15 deletionGateEscalationStopConditions=15 deletionGateEscalationResolutionArtifacts=15 deletionGateEscalationClosureChecks=15 deletionGateEscalationReopenConditions=15 deletionGateEscalationLedgerEvents=15 deletionGateEscalationStateTransitions=15 deletionGateEscalationBoundaryGuards=15 deletionGateEscalationAuditChecks=15 deletionGateVerificationArtifacts=15 deletionGateBlockedExecutionSteps=10 deletionGateRemediationActions=15 recoverySteps=4 recoveryValidationCommands=32 documentationTargets=9 documentationValidationCommands=72 executionSteps=5 executionFailureSignals=15 executionRemediationActions=15 executionVerificationArtifacts=15 executionLedgerEvents=15 executionStateTransitions=15 executionEscalationTriggers=15 executionEscalationEvidence=15 executionEscalationRecipients=15 executionEscalationHandoffSteps=15 executionEscalationDecisionOptions=15 executionEscalationRetryConditions=15 executionEscalationStopConditions=15 executionEscalationResolutionArtifacts=15 executionEscalationClosureChecks=15 executionEscalationReopenConditions=15 executionEscalationLedgerEvents=15 executionEscalationStateTransitions=15 executionEscalationBoundaryGuards=15 executionEscalationAuditChecks=15 executionBoundaryGuards=15 executionAuditChecks=15 executionValidationCommands=40 boundaryChecks=6 boundaryValidationCommands=48 removalImpact=true impactReferences=",
		"workItems=",
		"validationCommands=",
		"migrationTargets=74 migrationValidationCommands=592",
		"smokeMigrationTargets=29 smokeMigrationValidationCommands=232",
		"release handoff: release handoff summary ok ready=true readFirst=7 signals=12 knownGaps=5 packMaturity=10",
		"releaseNotes=true",
		"latest=Batch ",
		"known gaps:",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("release-check text missing %q:\n%s", expected, text)
		}
	}
}

func TestWriteReleaseCheckReturnsErrorAfterJsonInventoryWhenNotReady(t *testing.T) {
	result := releasecheck.Result{Command: "release-check", SchemaVersion: 1, Ready: false, Summary: "release gate inventory has warnings", Warnings: []string{"missing required document"}}
	var out bytes.Buffer
	err := writeReleaseCheckResult(&out, result, "json")
	if err == nil || !strings.Contains(err.Error(), "release-check not ready") || !strings.Contains(err.Error(), "missing required document") {
		t.Fatalf("error = %v, want not-ready error with warning", err)
	}
	var decoded struct {
		Ready    bool     `json:"ready"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("not-ready JSON was not emitted before error: %v\n%s", err, out.String())
	}
	if decoded.Ready || strings.Join(decoded.Warnings, ",") != "missing required document" {
		t.Fatalf("unexpected not-ready JSON: %+v", decoded)
	}
}

func TestWriteReleaseCheckReturnsErrorAfterTextInventoryWhenNotReady(t *testing.T) {
	result := releasecheck.Result{Command: "release-check", Ready: false, Summary: "release gate inventory has warnings", Warnings: []string{"CI workflow missing required command"}}
	var out bytes.Buffer
	err := writeReleaseCheckResult(&out, result, "text")
	if err == nil || !strings.Contains(err.Error(), "release-check not ready") {
		t.Fatalf("error = %v, want not-ready error", err)
	}
	text := out.String()
	for _, expected := range []string{"ready: false", "warnings:", "CI workflow missing required command"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("not-ready text missing %q:\n%s", expected, text)
		}
	}
}

func TestRunReleaseCheckRejectsTargetAndMutationFlags(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "release-check", "-Target", repoRoot(t)}, &out)
	if err == nil || !strings.Contains(err.Error(), "omit -Target") {
		t.Fatalf("error = %v, want target guard", err)
	}
	err = Run([]string{"-Command", "release-check", "-Apply"}, &out)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("error = %v, want read-only guard", err)
	}
}

func TestRunReleaseCheckRejectsUnsupportedFormat(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "release-check", "-Format", "yaml"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported release-check format") {
		t.Fatalf("error = %v, want unsupported release-check format", err)
	}
}

func TestRunPacksListsPackMatrix(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"-Command", "packs"}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{
		"pack\tmaturity\tschema\tmanifestSchema\troutes\tmanaged\ttooling\tauthority\tversion\tdescription",
		"_template\ttemplate\tok\t1\t2\t4\t2\tmain\t0.1.0",
		"vmp-re\tmature\tok\t1\t2\t7\t12\tdevirt-main\t0.2.0",
		"web-security\tskeleton\tok\t1\t2\t4\t4\tmain\t0.1.0",
		"malware-analysis\tskeleton\tok\t1\t2\t4\t4\tmain\t0.1.0",
		"vuln-research\tskeleton\tok\t1\t2\t4\t4\tmain\t0.1.0",
		"ctf\tskeleton\tok\t1\t2\t4\t4\tmain\t0.1.0",
		"unpack-pe\tskeleton\tok\t1\t2\t4\t4\tmain\t0.1.0",
		"ollvm\tskeleton\tok\t1\t2\t4\t4\tmain\t0.1.0",
		"android-native\tskeleton\tok\t1\t2\t4\t4\tmain\t0.1.0",
		"generic-binary-re\tskeleton\tok\t1\t2\t4\t4\tmain\t0.1.0",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("packs output missing %q:\n%s", expected, text)
		}
	}
}

func TestRunPacksJsonInventory(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"-Command", "packs", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Command       string `json:"command"`
		SchemaVersion int    `json:"schemaVersion"`
		IsMutation    bool   `json:"isMutation"`
		PackCount     int    `json:"packCount"`
		Packs         []struct {
			ID                   string   `json:"id"`
			Maturity             string   `json:"maturity"`
			SchemaValid          bool     `json:"schemaValid"`
			SchemaVersion        string   `json:"schemaVersion"`
			Error                string   `json:"error"`
			ManagedFiles         int      `json:"managedFiles"`
			ToolingFiles         int      `json:"toolingFiles"`
			SubagentRoutes       int      `json:"subagentRoutes"`
			HeavyToolGates       int      `json:"heavyToolGates"`
			HeavyToolGateActions []string `json:"heavyToolGateActions"`
			DefaultAuthorityLane string   `json:"defaultAuthorityLane"`
		} `json:"packs"`
	}
	if err := json.Unmarshal(out.Bytes(), &inventory); err != nil {
		t.Fatalf("packs JSON did not decode: %v\n%s", err, out.String())
	}
	if inventory.Command != "packs" || inventory.SchemaVersion != 1 || inventory.IsMutation || inventory.PackCount != len(inventory.Packs) {
		t.Fatalf("unexpected packs JSON envelope: %+v", inventory)
	}
	byID := map[string]struct {
		ID                   string   `json:"id"`
		Maturity             string   `json:"maturity"`
		SchemaValid          bool     `json:"schemaValid"`
		SchemaVersion        string   `json:"schemaVersion"`
		Error                string   `json:"error"`
		ManagedFiles         int      `json:"managedFiles"`
		ToolingFiles         int      `json:"toolingFiles"`
		SubagentRoutes       int      `json:"subagentRoutes"`
		HeavyToolGates       int      `json:"heavyToolGates"`
		HeavyToolGateActions []string `json:"heavyToolGateActions"`
		DefaultAuthorityLane string   `json:"defaultAuthorityLane"`
	}{}
	for _, pack := range inventory.Packs {
		byID[pack.ID] = pack
	}
	if pack := byID[defaults.DefaultPack]; pack.Maturity != "mature" || !pack.SchemaValid || pack.SchemaVersion != "1" || pack.Error != "" || pack.ManagedFiles != 7 || pack.ToolingFiles != 12 || pack.SubagentRoutes != 2 || pack.HeavyToolGates != 7 || strings.Join(pack.HeavyToolGateActions, ",") != "debug,dump,full-trace,inject,network,patch,symex" || pack.DefaultAuthorityLane != "devirt-main" {
		t.Fatalf("unexpected default pack JSON row: %+v", pack)
	}
	if pack := byID["web-security"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.SchemaVersion != "1" || pack.HeavyToolGates != 7 || pack.DefaultAuthorityLane != "main" {
		t.Fatalf("unexpected web-security JSON row: %+v", pack)
	}
	if pack := byID["malware-analysis"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.ManagedFiles != 4 || pack.ToolingFiles != 4 || pack.SubagentRoutes != 2 || pack.HeavyToolGates != 7 || pack.DefaultAuthorityLane != "main" {
		t.Fatalf("unexpected malware-analysis JSON row: %+v", pack)
	}
	if pack := byID["vuln-research"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.ManagedFiles != 4 || pack.ToolingFiles != 4 || pack.SubagentRoutes != 2 || pack.HeavyToolGates != 7 || pack.DefaultAuthorityLane != "main" {
		t.Fatalf("unexpected vuln-research JSON row: %+v", pack)
	}
	if pack := byID["ctf"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.ManagedFiles != 4 || pack.ToolingFiles != 4 || pack.SubagentRoutes != 2 || pack.HeavyToolGates != 7 || pack.DefaultAuthorityLane != "main" {
		t.Fatalf("unexpected ctf JSON row: %+v", pack)
	}
	if pack := byID["unpack-pe"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.ManagedFiles != 4 || pack.ToolingFiles != 4 || pack.SubagentRoutes != 2 || pack.HeavyToolGates != 7 || pack.DefaultAuthorityLane != "main" {
		t.Fatalf("unexpected unpack-pe JSON row: %+v", pack)
	}
	if pack := byID["ollvm"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.ManagedFiles != 4 || pack.ToolingFiles != 4 || pack.SubagentRoutes != 2 || pack.HeavyToolGates != 7 || pack.DefaultAuthorityLane != "main" {
		t.Fatalf("unexpected ollvm JSON row: %+v", pack)
	}
	if pack := byID["android-native"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.ManagedFiles != 4 || pack.ToolingFiles != 4 || pack.SubagentRoutes != 2 || pack.HeavyToolGates != 7 || pack.DefaultAuthorityLane != "main" {
		t.Fatalf("unexpected android-native JSON row: %+v", pack)
	}
	if pack := byID["generic-binary-re"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.ManagedFiles != 4 || pack.ToolingFiles != 4 || pack.SubagentRoutes != 2 || pack.HeavyToolGates != 7 || pack.DefaultAuthorityLane != "main" {
		t.Fatalf("unexpected generic-binary-re JSON row: %+v", pack)
	}
}

func TestRunPacksRejectsUnsupportedFormat(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "packs", "-Format", "yaml"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported packs format") {
		t.Fatalf("error = %v, want unsupported packs format", err)
	}
}

func TestRunCaseDoctorRejectsShimDrift(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeCaseFile(t, caseRoot, ".claude/skills/rekit/SKILL.md", "drift\n")
	var out bytes.Buffer
	err := Run([]string{"-Command", "doctor", "-Target", caseRoot, "-Pack", "_template"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for shim drift")
	}
	if !strings.Contains(err.Error(), "shim differs") {
		t.Fatalf("error = %q, want shim drift", err.Error())
	}
}

func TestRunAttachRequiresExplicitMode(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "attach", "-Target", filepath.Join(t.TempDir(), "case"), "-Pack", "_template"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for attach without -WhatIf or -Apply")
	}
	if !strings.Contains(err.Error(), "-Apply") || !strings.Contains(err.Error(), "-WhatIf") {
		t.Fatalf("error = %q, want apply/whatif guard", err.Error())
	}
}

func TestRunAttachPreviewDoesNotCreateFiles(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "attach", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo-case", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Command              string `json:"command"`
		CaseRoot             string `json:"caseRoot"`
		ProjectName          string `json:"projectName"`
		IsMutation           bool   `json:"isMutation"`
		ReviewRequired       bool   `json:"reviewRequired"`
		RequiresConfirmation bool   `json:"requiresConfirmation"`
		Writes               []struct {
			Path   string `json:"path"`
			Kind   string `json:"kind"`
			Action string `json:"action"`
		} `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("attach preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if plan.Command != "attach" || plan.IsMutation || !plan.ReviewRequired || !plan.RequiresConfirmation {
		t.Fatalf("unexpected attach preview flags: %+v", plan)
	}
	if plan.CaseRoot != caseRoot || plan.ProjectName != "demo-case" || len(plan.Writes) != 4 {
		t.Fatalf("unexpected attach preview: %+v", plan)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "instance.yml")); !os.IsNotExist(err) {
		t.Fatalf("attach -WhatIf created instance metadata, err=%v", err)
	}
}

func TestRunAttachApplyWritesBindingMetadataStateAndShim(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "attach", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo-case", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command    string `json:"command"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		Writes     []struct {
			Path string `json:"path"`
			Kind string `json:"kind"`
		} `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("attach apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "attach" || !result.IsMutation || !result.Applied || len(result.Writes) != 4 {
		t.Fatalf("unexpected attach apply result: %+v", result)
	}
	metadata, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "instance.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(metadata)
	if !strings.Contains(text, "templatePack: _template") || !strings.Contains(text, "projectName: demo-case") || !strings.Contains(text, "mode: case-local-shim") {
		t.Fatalf("instance metadata missing expected values: %s", text)
	}
	shim, err := os.ReadFile(filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile(filepath.Join(repoRoot(t), "rekit", "templates", "case-shim", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shim, canonical) {
		t.Fatal("case shim does not match canonical thin shim")
	}
	legacy, err := os.ReadFile(filepath.Join(caseRoot, ".re-template.yml"))
	if err != nil {
		t.Fatal(err)
	}
	legacyText := string(legacy)
	if !strings.Contains(legacyText, "templateRoot: "+repoRoot(t)) || !strings.Contains(legacyText, "templatePack: _template") || !strings.Contains(legacyText, "rekitMode: case-local-shim") {
		t.Fatalf("attach legacy metadata missing expected values: %s", legacyText)
	}
	state, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	stateText := string(state)
	if !strings.Contains(stateText, `"templatePack": "_template"`) || !strings.Contains(stateText, `"candidates": []`) {
		t.Fatalf("attach state missing expected values: %s", stateText)
	}
}

func TestRunAttachRejectsDifferentBinding(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, ".rekit/instance.yml", "templateRoot: C:\\other\\kit\ntemplatePack: _template\nprojectName: demo\nprojectRoot: "+caseRoot+"\n")
	var out bytes.Buffer
	err := Run([]string{"-Command", "attach", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for different templateRoot")
	}
	if !strings.Contains(err.Error(), "different templateRoot") {
		t.Fatalf("error = %q, want different templateRoot", err.Error())
	}
}

func TestRunRepairPreviewDoesNotWrite(t *testing.T) {
	caseRoot := movedAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "repair", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Command              string `json:"command"`
		IsMutation           bool   `json:"isMutation"`
		ReviewRequired       bool   `json:"reviewRequired"`
		RequiresConfirmation bool   `json:"requiresConfirmation"`
		Moved                bool   `json:"moved"`
		RecordedProjectRoot  string `json:"recordedProjectRoot"`
		NewProjectRoot       string `json:"newProjectRoot"`
		Writes               []struct {
			Kind string `json:"kind"`
		} `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("repair preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if plan.Command != "repair" || plan.IsMutation || !plan.ReviewRequired || !plan.RequiresConfirmation || !plan.Moved {
		t.Fatalf("unexpected repair preview flags: %+v", plan)
	}
	if plan.RecordedProjectRoot == plan.NewProjectRoot || len(plan.Writes) != 4 {
		t.Fatalf("unexpected repair preview: %+v", plan)
	}
	metadata, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "instance.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(metadata), "projectRoot: "+caseRoot) {
		t.Fatalf("repair preview updated instance metadata: %s", string(metadata))
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".re-template.yml")); !os.IsNotExist(err) {
		t.Fatalf("repair preview wrote legacy metadata, err=%v", err)
	}
}

func TestRunRepairApplyRefreshesMetadataShimAndLegacy(t *testing.T) {
	caseRoot := movedAttachedCase(t)
	writeCaseFile(t, caseRoot, ".claude/skills/rekit/SKILL.md", "drift\n")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "repair", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command    string `json:"command"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		Moved      bool   `json:"moved"`
		Writes     []struct {
			Kind string `json:"kind"`
		} `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("repair apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "repair" || !result.IsMutation || !result.Applied || !result.Moved || len(result.Writes) != 4 {
		t.Fatalf("unexpected repair apply result: %+v", result)
	}
	metadata, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "instance.yml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(metadata)
	if !strings.Contains(text, "projectRoot: "+caseRoot) || !strings.Contains(text, "templatePack: _template") {
		t.Fatalf("instance metadata not refreshed: %s", text)
	}
	shim, err := os.ReadFile(filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile(filepath.Join(repoRoot(t), "rekit", "templates", "case-shim", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(shim, canonical) {
		t.Fatal("repair did not refresh the case shim")
	}
	legacy, err := os.ReadFile(filepath.Join(caseRoot, ".re-template.yml"))
	if err != nil {
		t.Fatal(err)
	}
	legacyText := string(legacy)
	if !strings.Contains(legacyText, "currentProjectPath: "+caseRoot) || !strings.Contains(legacyText, "templatePack: _template") {
		t.Fatalf("legacy metadata not refreshed: %s", legacyText)
	}
	state, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), `"templatePack": "_template"`) {
		t.Fatalf("repair state missing expected values: %s", string(state))
	}
}

func TestRunRepairRejectsDifferentBinding(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, ".rekit/instance.yml", "templateRoot: C:\\other\\kit\ntemplatePack: _template\nprojectName: demo\nprojectRoot: C:\\old\\case\n")
	var out bytes.Buffer
	err := Run([]string{"-Command", "repair", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for repair with different templateRoot")
	}
	if !strings.Contains(err.Error(), "different templateRoot") {
		t.Fatalf("error = %q, want different templateRoot", err.Error())
	}
}

func TestRunInitRequiresExplicitMode(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "init", "-Target", filepath.Join(t.TempDir(), "case"), "-Pack", "_template"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for init without -WhatIf or -Apply")
	}
	if !strings.Contains(err.Error(), "-Apply") || !strings.Contains(err.Error(), "-WhatIf") {
		t.Fatalf("error = %q, want apply/whatif guard", err.Error())
	}
}

func TestRunInitPreviewDoesNotCreateFiles(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo-init", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Command              string      `json:"command"`
		CaseRoot             string      `json:"caseRoot"`
		ProjectName          string      `json:"projectName"`
		IsMutation           bool        `json:"isMutation"`
		ReviewRequired       bool        `json:"reviewRequired"`
		RequiresConfirmation bool        `json:"requiresConfirmation"`
		Writes               []syncWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("init preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if plan.Command != "init" || plan.CaseRoot != caseRoot || plan.ProjectName != "demo-init" || plan.IsMutation || !plan.ReviewRequired || !plan.RequiresConfirmation {
		t.Fatalf("unexpected init preview: %+v", plan)
	}
	if len(plan.Writes) == 0 {
		t.Fatalf("init preview did not report planned writes: %+v", plan)
	}
	if _, err := os.Stat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("init -WhatIf created target directory, err=%v", err)
	}
}

func TestRunInitApplyCreatesFullCase(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo-init", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command    string      `json:"command"`
		IsMutation bool        `json:"isMutation"`
		Applied    bool        `json:"applied"`
		Writes     []syncWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("init apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "init" || !result.IsMutation || !result.Applied {
		t.Fatalf("unexpected init apply result: %+v", result)
	}
	assertSyncWrite(t, result.Writes, "references/template/README.md", "create-managed-file", false)
	assertSyncWrite(t, result.Writes, "references/template/task-handoff.md", "create-local-template-file", false)
	assertSyncWrite(t, result.Writes, "CLAUDE.local.md", "create-managed-block-host", false)
	assertFileExists(t, filepath.Join(caseRoot, ".rekit", "instance.yml"))
	assertFileExists(t, filepath.Join(caseRoot, ".re-template.yml"))
	assertFileExists(t, filepath.Join(caseRoot, ".rekit", "state.json"))
	assertFileExists(t, filepath.Join(caseRoot, ".claude", "skills", "rekit", "SKILL.md"))
	assertFileExists(t, filepath.Join(caseRoot, "references", "template", "README.md"))
	assertFileExists(t, filepath.Join(caseRoot, "references", "template", "task-handoff.md"))
	if err := Run([]string{"-Command", "doctor", "-Target", caseRoot, "-Pack", "_template"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("doctor after init apply failed: %v", err)
	}
}

func TestRunBootstrapApplyUsesBootstrapCommand(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "bootstrap", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command string `json:"command"`
		Applied bool   `json:"applied"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("bootstrap apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "bootstrap" || !result.Applied {
		t.Fatalf("unexpected bootstrap apply result: %+v", result)
	}
}

func TestRunInitRejectsKitRepoRoot(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "init", "-Target", repoRoot(t), "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for init on kit repo root")
	}
	if !strings.Contains(err.Error(), "kit repo root") {
		t.Fatalf("error = %q, want kit repo root guard", err.Error())
	}
}

func TestRunInitRejectsMovedCase(t *testing.T) {
	caseRoot := movedAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for init on moved case")
	}
	if !strings.Contains(err.Error(), "case metadata points to a different directory") {
		t.Fatalf("error = %q, want moved case guard", err.Error())
	}
}

func TestRunInitRejectsDifferentBinding(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, ".rekit/instance.yml", "templateRoot: C:\\other\\kit\ntemplatePack: _template\nprojectName: demo\nprojectRoot: "+caseRoot+"\n")
	var out bytes.Buffer
	err := Run([]string{"-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for init with different templateRoot")
	}
	if !strings.Contains(err.Error(), "different templateRoot") {
		t.Fatalf("error = %q, want different templateRoot", err.Error())
	}
}

func TestRunSyncApplyRequiresAttachedCase(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", t.TempDir(), "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for sync -Apply on a non-case target")
	}
	if !strings.Contains(err.Error(), "target is not an attached rekit case") {
		t.Fatalf("error = %q, want attached case guard", err.Error())
	}
}

func TestRunSyncRejectsWhatIfWithoutApply(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-WhatIf"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for sync -WhatIf")
	}
	if !strings.Contains(err.Error(), "only supported with -Apply") {
		t.Fatalf("error = %q, want what-if/apply guard", err.Error())
	}
}

func TestRunSyncApplyWritesManagedContent(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Local drift\n\nchanged before sync apply\n")
	writeCaseFile(t, caseRoot, "references/template/task-handoff.md", "# Local handoff\n\nkeep this file on first apply\n")
	writeCaseFile(t, caseRoot, "CLAUDE.local.md", "prefix\n\n<!-- BEGIN template-pack:router -->\nold managed block\n<!-- END template-pack:router -->\n\nsuffix\n")

	var out bytes.Buffer
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-ProjectName", "sync-cli"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command    string      `json:"command"`
		IsMutation bool        `json:"isMutation"`
		Applied    bool        `json:"applied"`
		BackupRoot string      `json:"backupRoot"`
		Writes     []syncWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("sync apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "sync" || !result.IsMutation || !result.Applied || strings.TrimSpace(result.BackupRoot) == "" {
		t.Fatalf("unexpected sync apply result: %+v", result)
	}
	assertSyncWrite(t, result.Writes, "references/template/README.md", "overwrite-with-backup", true)
	assertSyncWrite(t, result.Writes, "references/template/task-handoff.md", "skip-existing-local-file", false)
	assertSyncWrite(t, result.Writes, "CLAUDE.local.md", "replace-managed-block", true)
	readme, err := os.ReadFile(filepath.Join(caseRoot, "references", "template", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(readme), "Local drift") {
		t.Fatalf("sync apply did not overwrite managed README: %s", string(readme))
	}
	state, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), "targetHashAtSync") {
		t.Fatalf("sync apply did not refresh state: %s", string(state))
	}
}

func TestRunSyncApplyRejectsMovedCase(t *testing.T) {
	caseRoot := movedAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for sync -Apply on moved case")
	}
	if !strings.Contains(err.Error(), "case metadata points to a different directory") {
		t.Fatalf("error = %q, want moved case guard", err.Error())
	}
}

func TestRunSyncApplyRejectsDifferentBinding(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, ".rekit/instance.yml", "templateRoot: C:\\other\\kit\ntemplatePack: _template\nprojectName: demo\nprojectRoot: "+caseRoot+"\n")
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for sync -Apply with different templateRoot")
	}
	if !strings.Contains(err.Error(), "different templateRoot") {
		t.Fatalf("error = %q, want different templateRoot", err.Error())
	}
}

func TestRunPromoteReviewRequiresAttachedCase(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", t.TempDir()}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for non-case promote target")
	}
	if !strings.Contains(err.Error(), "target is not an attached rekit case") {
		t.Fatalf("error = %q, want attached case guard", err.Error())
	}
}

func TestRunOverviewInitializesMissingBoard(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command    string `json:"command"`
		IsMutation bool   `json:"isMutation"`
		Lanes      []struct {
			ID        string `json:"id"`
			Label     string `json:"label"`
			Kind      string `json:"kind"`
			Authority bool   `json:"authority"`
		} `json:"lanes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("overview JSON did not decode: %v\n%s", err, out.String())
	}
	if result.Command != "overview" || !result.IsMutation || len(result.Lanes) != 1 || result.Lanes[0].ID != "main" || result.Lanes[0].Label != "main" || result.Lanes[0].Kind != "main" || !result.Lanes[0].Authority {
		t.Fatalf("unexpected initialized overview result: %+v", result)
	}
	for _, rel := range []string{
		".rekit/board.json",
		".rekit/policy.yml",
		".rekit/facts/observations.jsonl",
		".rekit/facts/candidates.jsonl",
		".rekit/facts/requests.jsonl",
		".rekit/facts/publications.jsonl",
		".rekit/facts/decisions.jsonl",
		".rekit/facts/hypotheses.jsonl",
		".rekit/facts/verifications.jsonl",
		".rekit/facts/interventions.jsonl",
		".rekit/facts/rollbacks.jsonl",
		".rekit/lanes/main/lane.json",
	} {
		if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("overview initialization missing %s: %v", rel, err)
		}
	}
}

func TestRunOverviewEmitsReadOnlySummary(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeOverviewFixture(t, caseRoot)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"项目概览：", "工作线：", "共享事实：", "Mission Control brief：", "ready lanes", "blocked lanes", "pending gates", "debug gate", "open decisions", "candidate: handler", "interventions", "manual override", "next agent actions", "escalations", "pending-gate requires main-agent/user decision", "未决 candidate：", "pending-gate", "by=runtime-test", "action=debug", "最近 verification：", "verifier=manual-review", "verdict=accepted", "target=candidate-alpha", "by=reviewer-smoke", "最近 decision：", "batch-overview", "未解决 intervention：", "最近 rollback：", "/rekit continue main"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("overview missing %q:\n%s", expected, text)
		}
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunOverviewJsonEmitsReadOnlyInventory(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeOverviewFixture(t, caseRoot)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion int    `json:"schemaVersion"`
		Command       string `json:"command"`
		CaseRoot      string `json:"caseRoot"`
		Pack          string `json:"pack"`
		IsMutation    bool   `json:"isMutation"`
		Lanes         []struct {
			ID        string `json:"id"`
			Label     string `json:"label"`
			Kind      string `json:"kind"`
			Authority bool   `json:"authority"`
		} `json:"lanes"`
		Counts struct {
			Observations     int `json:"observations"`
			Requests         int `json:"requests"`
			Candidates       int `json:"candidates"`
			Publications     int `json:"publications"`
			PendingDecisions int `json:"pendingDecisions"`
		} `json:"counts"`
		MissionBrief struct {
			Summary          string   `json:"summary"`
			ReadyLanes       []string `json:"readyLanes"`
			BlockedLanes     []string `json:"blockedLanes"`
			PendingGates     []string `json:"pendingGates"`
			AuthorizedGates  []string `json:"authorizedGates"`
			OpenDecisions    []string `json:"openDecisions"`
			Interventions    []string `json:"interventions"`
			NextAgentActions []string `json:"nextAgentActions"`
			Escalations      []string `json:"escalations"`
		} `json:"missionBrief"`
		Sections struct {
			OpenCandidates struct {
				Total  int              `json:"total"`
				Shown  int              `json:"shown"`
				Events []map[string]any `json:"events"`
			} `json:"openCandidates"`
			PendingGates struct {
				Total  int              `json:"total"`
				Events []map[string]any `json:"events"`
			} `json:"pendingGates"`
			AuthorizedGates struct {
				Total  int              `json:"total"`
				Events []map[string]any `json:"events"`
			} `json:"authorizedGates"`
			Verifications struct {
				Total  int              `json:"total"`
				Events []map[string]any `json:"events"`
			} `json:"verifications"`
			Batches struct {
				Total   int `json:"total"`
				Batches []struct {
					ID     string         `json:"id"`
					Events int            `json:"events"`
					Kinds  map[string]int `json:"kinds"`
				} `json:"batches"`
			} `json:"batches"`
		} `json:"sections"`
		NextSteps []string `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("overview JSON did not decode: %v\n%s", err, out.String())
	}
	if result.SchemaVersion != 1 || result.Command != "overview" || result.CaseRoot != caseRoot || result.Pack != "_template" || result.IsMutation || len(result.Lanes) != 1 || len(result.NextSteps) == 0 {
		t.Fatalf("unexpected overview JSON envelope: %+v", result)
	}
	if result.Lanes[0].ID != "main" || result.Lanes[0].Label != "main" || result.Lanes[0].Kind != "main" || !result.Lanes[0].Authority {
		t.Fatalf("unexpected overview lanes: %+v", result.Lanes)
	}
	if result.Counts.Observations != 1 || result.Counts.Requests != 1 || result.Counts.Candidates != 2 || result.Counts.Publications != 1 || result.Counts.PendingDecisions != 1 {
		t.Fatalf("unexpected overview counts: %+v", result.Counts)
	}
	if result.Sections.OpenCandidates.Total != 2 || result.Sections.OpenCandidates.Shown != 2 || result.Sections.PendingGates.Total != 1 || result.Sections.AuthorizedGates.Total != 0 || result.Sections.Verifications.Total != 1 || result.Sections.Batches.Total != 1 {
		t.Fatalf("unexpected overview sections: %+v", result.Sections)
	}
	if result.MissionBrief.Summary == "" || len(result.MissionBrief.ReadyLanes) != 0 || len(result.MissionBrief.BlockedLanes) != 1 || len(result.MissionBrief.PendingGates) != 1 || len(result.MissionBrief.AuthorizedGates) != 0 || len(result.MissionBrief.OpenDecisions) != 3 || len(result.MissionBrief.Interventions) != 1 || len(result.MissionBrief.NextAgentActions) == 0 || len(result.MissionBrief.Escalations) != 3 {
		t.Fatalf("unexpected mission brief: %+v", result.MissionBrief)
	}
	if !slices.Contains(result.MissionBrief.BlockedLanes, "main (pending-gate,intervention,open-decision)") || !strings.Contains(result.MissionBrief.PendingGates[0], "action=debug") || !strings.Contains(result.MissionBrief.OpenDecisions[0], "candidate: handler") || !strings.Contains(result.MissionBrief.Interventions[0], "manual override") {
		t.Fatalf("unexpected mission brief details: %+v", result.MissionBrief)
	}
	if result.Sections.PendingGates.Events[0]["actor"] != "runtime-test" || result.Sections.Verifications.Events[0]["verdict"] != "accepted" || result.Sections.Batches.Batches[0].ID != "batch-overview" || result.Sections.Batches.Batches[0].Events != 5 || result.Sections.Batches.Batches[0].Kinds["request"] != 1 {
		t.Fatalf("unexpected overview section details: %+v", result.Sections)
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunOverviewRejectsUnsupportedFormat(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeOverviewFixture(t, caseRoot)
	var out bytes.Buffer
	err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "yaml"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported overview format") {
		t.Fatalf("error = %v, want unsupported overview format", err)
	}
}

func TestRunNoteListEmitsReadOnlySummary(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeOverviewFixture(t, caseRoot)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-List"}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"[observation] (1 条)", "obs | lane=main", "[candidate] (2 条)", "handler | lane=main | confidence=high | status=open", "[request] (1 条)", "pending-gate", "action=debug", "[decision] (1 条)", "decision=defer", "[verification] (1 条)", "verifier=manual-review", "[intervention] (1 条)", "approvedBy=lead", "[rollback] (1 条)", "status=resolved"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("note list missing %q:\n%s", expected, text)
		}
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunNoteListJsonEmitsReadOnlyEvents(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeOverviewFixture(t, caseRoot)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-List", "-Kind", "verification", "-Lane", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion int    `json:"schemaVersion"`
		Command       string `json:"command"`
		IsMutation    bool   `json:"isMutation"`
		Pack          string `json:"pack"`
		Kind          string `json:"kind"`
		Lane          string `json:"lane"`
		EventCount    int    `json:"eventCount"`
		Groups        []struct {
			Kind   string           `json:"kind"`
			Total  int              `json:"total"`
			Shown  int              `json:"shown"`
			Events []map[string]any `json:"events"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("note list JSON did not decode: %v\n%s", err, out.String())
	}
	if result.SchemaVersion != 1 || result.Command != "note" || result.IsMutation || result.Pack != "_template" || result.Kind != "verification" || result.Lane != "main" || result.EventCount != 1 || len(result.Groups) != 1 {
		t.Fatalf("unexpected note list JSON envelope: %+v", result)
	}
	group := result.Groups[0]
	if group.Kind != "verification" || group.Total != 1 || group.Shown != 1 || len(group.Events) != 1 || group.Events[0]["verifier"] != "manual-review" || group.Events[0]["verdict"] != "accepted" {
		t.Fatalf("unexpected note list JSON group: %+v", group)
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunNoteListFiltersKindAndLane(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeOverviewFixture(t, caseRoot)
	writeFactFile(t, filepath.Join(caseRoot, ".rekit", "facts"), "candidates.jsonl", []string{
		`{"kind":"candidate","lane":"main","subject":"main candidate","confidence":"high","status":"open","batchId":"batch-main"}`,
		`{"kind":"candidate","lane":"feature-login","subject":"feature candidate","confidence":"medium","status":"open","batchId":"batch-feature"}`,
	})
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-List", "-Kind", "candidate", "-Lane", "feature-login"}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{"[candidate] (1 条)", "feature candidate", "lane=feature-login", "batch=batch-feature"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("filtered note list missing %q:\n%s", expected, text)
		}
	}
	for _, unexpected := range []string{"[observation]", "main candidate", "batch-main"} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("filtered note list contains %q:\n%s", unexpected, text)
		}
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunNoteListRejectsInvalidKind(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-List", "-Kind", "unknown"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for invalid note kind")
	}
	if !strings.Contains(err.Error(), "invalid note kind") {
		t.Fatalf("error = %q, want invalid kind guard", err.Error())
	}
}

func TestRunNoteListRejectsUnsupportedFormat(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-List", "-Format", "yaml"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported note list format") {
		t.Fatalf("error = %v, want unsupported note list format", err)
	}
}

func TestRunNoteAppendWritesFactEvent(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "note",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Kind", "verification",
		"-Lane", "main",
		"-Subject", "review target",
		"-Summary", "accepted by reviewer",
		"-Actor", "runtime-test",
		"-Verifier", "manual-review",
		"-Verdict", "accepted",
		"-TargetRef", "candidate-alpha",
		"-BatchId", "batch-note",
		"-EvidenceRefs", "evidence-one,evidence-two",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command    string         `json:"command"`
		IsMutation bool           `json:"isMutation"`
		Applied    bool           `json:"applied"`
		EventID    string         `json:"eventId"`
		Path       string         `json:"path"`
		Event      map[string]any `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("note append stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "note" || !result.IsMutation || !result.Applied || result.EventID == "" || result.Path != ".rekit/facts/verifications.jsonl" {
		t.Fatalf("unexpected note append result: %+v", result)
	}
	if result.Event["kind"] != "verification" || result.Event["lane"] != "main" || result.Event["verifier"] != "manual-review" || result.Event["verdict"] != "accepted" || result.Event["target"] != "candidate-alpha" || result.Event["batchId"] != "batch-note" {
		t.Fatalf("unexpected event fields: %+v", result.Event)
	}
	refs, ok := result.Event["evidenceRefs"].([]any)
	if !ok || len(refs) != 2 {
		t.Fatalf("unexpected evidence refs: %+v", result.Event["evidenceRefs"])
	}
	ledger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(ledger)
	for _, expected := range []string{result.EventID, `"kind":"verification"`, `"verdict":"accepted"`, `"evidenceRefs":["evidence-one","evidence-two"]`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("ledger missing %q:\n%s", expected, text)
		}
	}
}

func TestRunNoteAppendSupportsAllKinds(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	cases := []struct {
		kind  string
		path  string
		extra []string
	}{
		{kind: "observation", path: "observations.jsonl"},
		{kind: "hypothesis", path: "hypotheses.jsonl"},
		{kind: "candidate", path: "candidates.jsonl", extra: []string{"-Confidence", "medium"}},
		{kind: "verification", path: "verifications.jsonl", extra: []string{"-Verifier", "manual-review", "-Verdict", "inconclusive"}},
		{kind: "decision", path: "decisions.jsonl", extra: []string{"-Decision", "defer"}},
		{kind: "intervention", path: "interventions.jsonl", extra: []string{"-Action", "override", "-ApprovedBy", "lead"}},
		{kind: "rollback", path: "rollbacks.jsonl", extra: []string{"-TargetRef", "batch-one", "-Status", "resolved"}},
		{kind: "publication", path: "publications.jsonl"},
		{kind: "request", path: "requests.jsonl", extra: []string{"-Status", "pending-gate", "-Risk", "high"}},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			args := []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", tc.kind, "-Lane", "main", "-Subject", tc.kind + " subject"}
			args = append(args, tc.extra...)
			var out bytes.Buffer
			if err := Run(args, &out); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Applied bool   `json:"applied"`
				EventID string `json:"eventId"`
				Path    string `json:"path"`
				Event   struct {
					Kind string `json:"kind"`
				} `json:"event"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("note append stdout is not JSON: %v\n%s", err, out.String())
			}
			if !result.Applied || result.EventID == "" || result.Event.Kind != tc.kind || result.Path != ".rekit/facts/"+tc.path {
				t.Fatalf("unexpected note append result for %s: %+v", tc.kind, result)
			}
			ledger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", tc.path))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(ledger), result.EventID) || !strings.Contains(string(ledger), `"kind":"`+tc.kind+`"`) {
				t.Fatalf("ledger missing %s event:\n%s", tc.kind, string(ledger))
			}
		})
	}
}

func TestRunNoteAppendWhatIfDoesNotWrite(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Kind", "observation", "-Lane", "main", "-Subject", "preview only"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		Reason     string `json:"reason"`
		Path       string `json:"path"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("note what-if stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.IsMutation || result.Applied || result.Reason != "what-if" || result.Path != ".rekit/facts/observations.jsonl" {
		t.Fatalf("unexpected note what-if result: %+v", result)
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunNoteAppendDedupesByEventID(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	args := []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "candidate", "-Lane", "main", "-Subject", "candidate one", "-Confidence", "high", "-EventId", "evt-fixed-note"}
	var first bytes.Buffer
	if err := Run(args, &first); err != nil {
		t.Fatal(err)
	}
	var second bytes.Buffer
	if err := Run(args, &second); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Applied bool   `json:"applied"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(second.Bytes(), &result); err != nil {
		t.Fatalf("second note append stdout is not JSON: %v\n%s", err, second.String())
	}
	if result.Applied || result.Reason != "duplicate eventId" {
		t.Fatalf("unexpected duplicate result: %+v", result)
	}
	ledger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "candidates.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(ledger), "evt-fixed-note") != 1 {
		t.Fatalf("duplicate event id written more than once:\n%s", string(ledger))
	}
}

func TestRunNoteAppendRejectsInvalidInputs(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing kind", args: []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Lane", "main"}, want: "requires -Kind"},
		{name: "missing lane", args: []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "observation"}, want: "requires -Lane"},
		{name: "unknown lane", args: []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "observation", "-Lane", "missing"}, want: "unknown lane"},
		{name: "invalid confidence", args: []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "candidate", "-Lane", "main", "-Confidence", "certain"}, want: "invalid Confidence"},
		{name: "invalid decision", args: []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "decision", "-Lane", "main", "-Decision", "confirm"}, want: "invalid Decision"},
		{name: "invalid verifier", args: []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "verification", "-Lane", "main", "-Verifier", "unknown"}, want: "invalid Verifier"},
		{name: "invalid verdict", args: []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "verification", "-Lane", "main", "-Verdict", "maybe"}, want: "invalid Verdict"},
		{name: "invalid action", args: []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "intervention", "-Lane", "main", "-Action", "delete"}, want: "invalid Action"},
		{name: "invalid status", args: []string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "observation", "-Lane", "main", "-Status", "pending"}, want: "invalid Status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run(tc.args, &out)
			if err == nil {
				t.Fatal("Run returned nil error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestRunNoteRejectsUnsupportedWriteFlags(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "observation", "-Lane", "main", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for note -Apply")
	}
	if !strings.Contains(err.Error(), "does not support -Apply") {
		t.Fatalf("error = %q, want -Apply guard", err.Error())
	}

	err = Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-List", "-WhatIf"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for note -List -WhatIf")
	}
	if !strings.Contains(err.Error(), "note -List cannot be combined with -WhatIf") {
		t.Fatalf("error = %q, want list/whatif guard", err.Error())
	}
}

func TestRunStartPreviewDoesNotWriteBoard(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		IsMutation bool `json:"isMutation"`
		Applied    bool `json:"applied"`
		Lane       struct {
			ID        string `json:"id"`
			Workspace string `json:"workspace"`
		} `json:"lane"`
		MissionBrief missionBrief `json:"missionBrief"`
		Writes       []struct {
			Path   string `json:"path"`
			Action string `json:"action"`
		} `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.IsMutation || result.Applied || result.Lane.ID != "feature-login" || result.Lane.Workspace != "workspace/features/feature-login" {
		t.Fatalf("unexpected start preview result: %+v", result)
	}
	if result.MissionBrief.Summary == "" || result.MissionBrief.Summary != "openLanes=0 ready=0 blocked=0 pendingGates=0 authorizedGates=0 openDecisions=0 interventions=0" {
		t.Fatalf("start preview missing pre-apply mission brief: %+v", result.MissionBrief)
	}
	if len(result.Writes) != 1 || result.Writes[0].Path != ".rekit/lanes/feature-login/lane.json" || result.Writes[0].Action != "would-create-lane" {
		t.Fatalf("unexpected start preview writes: %+v", result.Writes)
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunStartApplyCreatesFeatureLane(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodeStartResult(t, out.Bytes())
	if !result.IsMutation || !result.Applied || result.Lane.ID != "feature-login" {
		t.Fatalf("unexpected start apply result: %+v", result)
	}
	if result.MissionBrief.Summary == "" || !slices.Contains(result.MissionBrief.ReadyLanes, "main") || !slices.Contains(result.MissionBrief.ReadyLanes, "login") || len(result.MissionBrief.BlockedLanes) != 0 {
		t.Fatalf("start apply JSON missing post-apply mission brief: %+v", result.MissionBrief)
	}
	assertStartWrite(t, result.Writes, ".rekit/policy.yml", "create-policy")
	assertStartWrite(t, result.Writes, ".rekit/lanes/main/lane.json", "create-lane")
	assertStartWrite(t, result.Writes, ".rekit/lanes/feature-login/lane.json", "create-lane")
	assertStartWrite(t, result.Writes, ".rekit/board.json", "refresh")
	for _, rel := range []string{".rekit/board.json", ".rekit/lanes/main/lane.json", ".rekit/lanes/feature-login/lane.json", ".rekit/lanes/feature-login/prompts/RESUME.md", "workspace/features/feature-login/summary.md"} {
		if _, err := os.Stat(filepath.Join(caseRoot, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing start artifact %s: %v", rel, err)
		}
	}
	var doctorOut bytes.Buffer
	if err := Run([]string{"-Command", "doctor", "-Target", caseRoot, "-Pack", "_template"}, &doctorOut); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	again := decodeStartResult(t, out.Bytes())
	assertStartWrite(t, again.Writes, ".rekit/lanes/feature-login/lane.json", "enter-existing-lane")

	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/inbox.jsonl", "{\"eventId\":\"in-1\",\"summary\":\"review queued\"}\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/tasks.jsonl", "{\"taskId\":\"task-1\",\"summary\":\"inspect candidate\",\"status\":\"open\"}\n{\"taskId\":\"task-2\",\"summary\":\"closed task\",\"status\":\"closed\"}\n")
	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply", "-Force"}, &out); err != nil {
		t.Fatal(err)
	}
	forced := decodeStartResult(t, out.Bytes())
	assertStartWrite(t, forced.Writes, ".rekit/lanes/feature-login/lane.json", "refresh-lane-with-force")
	assertStartWrite(t, forced.Writes, ".rekit/lanes/feature-login/events.jsonl", "append-lane-refreshed")
	resume, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "feature-login", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resume), "review queued") || !strings.Contains(string(resume), "inspect candidate") || strings.Contains(string(resume), "closed task") {
		t.Fatalf("force refresh resume did not preserve live inbox/tasks:\n%s", string(resume))
	}
}

func TestRunStartRequiresMode(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for start without mode")
	}
	if !strings.Contains(err.Error(), "-Apply") || !strings.Contains(err.Error(), "-WhatIf") {
		t.Fatalf("error = %q, want apply/whatif guard", err.Error())
	}
}

func TestRunHandoffPreviewDoesNotWrite(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeHandoffFixture(t, caseRoot)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodeHandoffResult(t, out.Bytes())
	if result.Command != "handoff" || result.IsMutation || result.Applied || !result.Project || !result.RequiresConfirmation {
		t.Fatalf("unexpected handoff preview result: %+v", result)
	}
	assertStartWrite(t, result.Writes, ".rekit/handovers/latest.md", "would-write-latest-project-handoff")
	if result.MissionBrief.Summary == "" || !slices.Contains(result.MissionBrief.BlockedLanes, "login (pending-gate,intervention,open-decision)") || len(result.MissionBrief.PendingGates) == 0 || len(result.MissionBrief.OpenDecisions) == 0 || len(result.MissionBrief.Interventions) == 0 {
		t.Fatalf("handoff preview missing structured mission brief: %+v", result.MissionBrief)
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunHandoffApplyWritesProjectAndLane(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeHandoffFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	project := decodeHandoffResult(t, out.Bytes())
	if !project.IsMutation || !project.Applied || !project.Project {
		t.Fatalf("unexpected project handoff result: %+v", project)
	}
	if !slices.Contains(project.MissionBrief.ReadyLanes, "main") || !slices.Contains(project.MissionBrief.BlockedLanes, "login (pending-gate,intervention,open-decision)") || !containsSubstring(project.MissionBrief.NextAgentActions, "review open candidate/decision item(s)") {
		t.Fatalf("project handoff JSON missing structured mission brief: %+v", project.MissionBrief)
	}
	latest := assertStartWrite(t, project.Writes, ".rekit/handovers/latest.md", "write-latest-project-handoff")
	text, err := os.ReadFile(latest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 项目接手索引", "## Mission Control brief", "summary: openLanes=2 ready=1 blocked=1", "ready lanes:", "main", "blocked lanes:", "login (pending-gate,intervention,open-decision)", "pending gates:", "debug gate", "open decisions:", "decision subject", "interventions:", "manual override", "next agent actions:", "reconcile open intervention", "escalations:", "pending-gate requires main-agent/user decision", "## 工作线", "/rekit continue main", "/rekit handoff login"} {
		if !strings.Contains(string(text), expected) {
			t.Fatalf("project handoff missing %q:\n%s", expected, string(text))
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	lane := decodeHandoffResult(t, out.Bytes())
	if !lane.IsMutation || !lane.Applied || lane.Project || lane.Lane == nil || lane.Lane.ID != "feature-login" {
		t.Fatalf("unexpected lane handoff result: %+v", lane)
	}
	if !slices.Contains(lane.MissionBrief.BlockedLanes, "login (pending-gate,intervention,open-decision)") || !containsSubstring(lane.MissionBrief.PendingGates, "debug gate") || !containsSubstring(lane.MissionBrief.OpenDecisions, "decision subject") || !containsSubstring(lane.MissionBrief.NextAgentActions, "review open candidate/decision item(s)") {
		t.Fatalf("lane handoff JSON missing structured mission brief: %+v", lane.MissionBrief)
	}
	laneLatest := assertStartWrite(t, lane.Writes, ".rekit/handovers/feature-login-latest.md", "write-latest-lane-handoff")
	laneText, err := os.ReadFile(laneLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 工作线接手：feature-login", "workspace/features/feature-login/packet.md", "## Mission Control brief", "blocked: true", "pending-gate:", "open intervention:", "open decision:", "next agent action:", "reconcile user intervention", "## verification", "verifier=manual-review", "verdict=accepted", "target=candidate-alpha", "by=reviewer-smoke", "## decision", "by=runtime-test", "## pending-gate", "action=debug", "## intervention", "## rollback", "## 边界"} {
		if !strings.Contains(string(laneText), expected) {
			t.Fatalf("lane handoff missing %q:\n%s", expected, string(laneText))
		}
	}
	resume, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "feature-login", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resume), "review queued") || !strings.Contains(string(resume), "inspect candidate") {
		t.Fatalf("lane resume missing live inbox/tasks:\n%s", string(resume))
	}
}

func TestRunProjectHandoffMissionBriefBlocksOpenDecisions(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeHandoffFixture(t, caseRoot)
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	writeFactFile(t, factsRoot, "candidates.jsonl", []string{`{"kind":"candidate","lane":"feature-login","subject":"project candidate blocker","summary":"awaiting main decision","confidence":"high","status":"open","batchId":"batch-project-handoff-parity"}`})
	writeFactFile(t, factsRoot, "decisions.jsonl", nil)
	writeFactFile(t, factsRoot, "requests.jsonl", nil)
	writeFactFile(t, factsRoot, "interventions.jsonl", nil)

	var out bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	project := decodeHandoffResult(t, out.Bytes())
	latest := assertStartWrite(t, project.Writes, ".rekit/handovers/latest.md", "write-latest-project-handoff")
	text, err := os.ReadFile(latest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"## Mission Control brief", "summary: openLanes=2 ready=1 blocked=1 pendingGates=0 authorizedGates=0 openDecisions=1 interventions=0", "ready lanes:", "main", "blocked lanes:", "login (open-decision)", "pending gates: none", "authorized gates: none", "open decisions:", "candidate: project candidate blocker", "lane=feature-login", "interventions: none", "next agent actions:", "review open candidate/decision item(s)", "/rekit continue main", "escalations:", "authority/confirmed outcome remains deferred"} {
		if !strings.Contains(string(text), expected) {
			t.Fatalf("project handoff missing %q:\n%s", expected, string(text))
		}
	}
	for _, unexpected := range []string{"pending-gate requires main-agent/user decision", "open intervention must be reconciled"} {
		if strings.Contains(string(text), unexpected) {
			t.Fatalf("project handoff contained unexpected %q:\n%s", unexpected, string(text))
		}
	}
}

func TestRunHandoffMissionBriefBlocksOpenDecisions(t *testing.T) {
	cases := []struct {
		name       string
		candidates []string
		decisions  []string
		wantText   []string
	}{
		{
			name: "candidate only",
			candidates: []string{
				`{"kind":"candidate","lane":"feature-login","subject":"unmerged candidate","summary":"awaiting main decision","confidence":"high","status":"open","batchId":"batch-handoff-parity"}`,
			},
			wantText: []string{"blocked: true", "pending-gate: none", "open intervention: none", "open decision:", "candidate: unmerged candidate", "review open candidate/decision item(s)"},
		},
		{
			name: "decision only",
			decisions: []string{
				`{"kind":"decision","lane":"feature-login","subject":"merge deferred","decision":"defer","actor":"runtime-test","reason":"needs main approval","batchId":"batch-handoff-parity"}`,
			},
			wantText: []string{"blocked: true", "pending-gate: none", "open intervention: none", "open decision:", "merge deferred", "decision=defer", "review open candidate/decision item(s)"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot := fullAttachedCase(t)
			writeHandoffFixture(t, caseRoot)
			factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
			writeFactFile(t, factsRoot, "candidates.jsonl", tc.candidates)
			writeFactFile(t, factsRoot, "decisions.jsonl", tc.decisions)
			writeFactFile(t, factsRoot, "requests.jsonl", nil)
			writeFactFile(t, factsRoot, "interventions.jsonl", nil)

			var out bytes.Buffer
			if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
				t.Fatal(err)
			}
			lane := decodeHandoffResult(t, out.Bytes())
			laneLatest := assertStartWrite(t, lane.Writes, ".rekit/handovers/feature-login-latest.md", "write-latest-lane-handoff")
			laneText, err := os.ReadFile(laneLatest.TargetPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range tc.wantText {
				if !strings.Contains(string(laneText), expected) {
					t.Fatalf("lane handoff missing %q:\n%s", expected, string(laneText))
				}
			}
			for _, unexpected := range []string{"## pending-gate", "## intervention", "resolve or explicitly defer pending gate"} {
				if strings.Contains(string(laneText), unexpected) {
					t.Fatalf("lane handoff contained unexpected %q:\n%s", unexpected, string(laneText))
				}
			}

			out.Reset()
			if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
				t.Fatal(err)
			}
			var overview struct {
				MissionBrief struct {
					BlockedLanes  []string `json:"blockedLanes"`
					OpenDecisions []string `json:"openDecisions"`
				} `json:"missionBrief"`
			}
			if err := json.Unmarshal(out.Bytes(), &overview); err != nil {
				t.Fatalf("overview JSON did not decode: %v\n%s", err, out.String())
			}
			if !slices.Contains(overview.MissionBrief.BlockedLanes, "login (open-decision)") || len(overview.MissionBrief.OpenDecisions) == 0 {
				t.Fatalf("overview mission brief is not decision-blocked: %+v", overview.MissionBrief)
			}
		})
	}
}

func TestRunHandoffRequiresModeAndBoard(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf"}, &out)
	if err == nil || !strings.Contains(err.Error(), "board.json") {
		t.Fatalf("error = %v, want missing board guard", err)
	}
	writeHandoffFixture(t, caseRoot)
	err = Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for handoff without mode")
	}
	if !strings.Contains(err.Error(), "-Apply") || !strings.Contains(err.Error(), "-WhatIf") {
		t.Fatalf("error = %q, want apply/whatif guard", err.Error())
	}
}

func TestRunHandoffRejectsUnsafeLaneMetadata(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeHandoffFixture(t, caseRoot)
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/lane.json", `{"schemaVersion":1,"id":"../../outside","type":"feature","name":"login","title":"功能分析: login","status":"open","authority":false,"workspace":"workspace/features/feature-login","laneRoot":".rekit/lanes/feature-login"}`)
	var out bytes.Buffer
	err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login"}, &out)
	if err == nil || !strings.Contains(err.Error(), "lane id mismatch") {
		t.Fatalf("error = %v, want lane id mismatch guard", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, "outside-latest.md")); !os.IsNotExist(err) {
		t.Fatalf("unsafe handoff path was created or stat failed: %v", err)
	}
}

func TestRunHandoffFallsBackToDerivedLaneRoot(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeHandoffFixture(t, caseRoot)
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/lane.json", `{"schemaVersion":1,"id":"feature-login","type":"feature","name":"login","title":"功能分析: login","status":"open","authority":false,"workspace":"workspace/features/feature-login"}`)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodeHandoffResult(t, out.Bytes())
	assertStartWrite(t, result.Writes, ".rekit/lanes/feature-login/prompts/RESUME.md", "refresh")
}

func TestRunContinueBlocksUntilReconcileClosesIntervention(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	writeContinueFixture(t, caseRoot)
	writeCaseFile(t, caseRoot, ".rekit/facts/interventions.jsonl", `{"eventId":"evt-human-stop","kind":"intervention","lane":"feature-login","subject":"human correction","summary":"user changed lane direction","action":"override","status":"open","target":"workspace/features/feature-login"}`+"\n")
	before := snapshotFiles(t, caseRoot)

	var out bytes.Buffer
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "vmp-re", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	var blocked struct {
		Command           string `json:"command"`
		Applied           bool   `json:"applied"`
		Blocked           bool   `json:"blocked"`
		ReconcileRequired bool   `json:"reconcileRequired"`
		OpenInterventions []struct {
			EventID string `json:"eventId"`
			Lane    string `json:"lane"`
		} `json:"openInterventions"`
		Writes []startWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &blocked); err != nil {
		t.Fatalf("blocked continue stdout is not JSON: %v\n%s", err, out.String())
	}
	if blocked.Command != "continue" || blocked.Applied || !blocked.Blocked || !blocked.ReconcileRequired || len(blocked.OpenInterventions) != 1 || blocked.OpenInterventions[0].EventID != "evt-human-stop" || len(blocked.Writes) != 0 {
		t.Fatalf("unexpected blocked continue result: %+v", blocked)
	}
	afterBlocked := snapshotFiles(t, caseRoot)
	assertSnapshotEqual(t, before, afterBlocked)

	out.Reset()
	if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "vmp-re", "-WhatIf", "login", "-InterventionId", "evt-human-stop", "-Executor", "session-2", "-Actor", "main-agent", "-Reason", "accept user correction"}, &out); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Command      string    `json:"command"`
		IsMutation   bool      `json:"isMutation"`
		Applied      bool      `json:"applied"`
		Lane         startLane `json:"lane"`
		Intervention struct {
			EventID string `json:"eventId"`
		} `json:"intervention"`
		Executor    string       `json:"executor"`
		WouldWrites []startWrite `json:"wouldWrites"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("reconcile preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if preview.Command != "reconcile" || preview.IsMutation || preview.Applied || preview.Lane.ID != "feature-login" || preview.Intervention.EventID != "evt-human-stop" || preview.Executor != "session-2" || len(preview.WouldWrites) == 0 {
		t.Fatalf("unexpected reconcile preview: %+v", preview)
	}
	assertWriteKind(t, preview.WouldWrites, "lane-event", "would-append-executor-takeover")
	afterPreview := snapshotFiles(t, caseRoot)
	assertSnapshotEqual(t, before, afterPreview)

	out.Reset()
	if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "vmp-re", "-Apply", "login", "-InterventionId", "evt-human-stop", "-Executor", "session-2", "-Actor", "main-agent", "-Reason", "accept user correction"}, &out); err != nil {
		t.Fatal(err)
	}
	var reconciled struct {
		Command            string       `json:"command"`
		IsMutation         bool         `json:"isMutation"`
		Applied            bool         `json:"applied"`
		Lane               startLane    `json:"lane"`
		ResolutionEventID  string       `json:"resolutionEventId"`
		Executor           string       `json:"executor"`
		ExecutorGeneration int          `json:"executorGeneration"`
		Writes             []startWrite `json:"writes"`
		MissionBrief       missionBrief `json:"missionBrief"`
	}
	if err := json.Unmarshal(out.Bytes(), &reconciled); err != nil {
		t.Fatalf("reconcile apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if reconciled.Command != "reconcile" || !reconciled.IsMutation || !reconciled.Applied || reconciled.Lane.ID != "feature-login" || reconciled.ResolutionEventID == "" || reconciled.Executor != "session-2" || reconciled.ExecutorGeneration != 1 || reconciled.Lane.CurrentExecutor != "session-2" || reconciled.Lane.LastReconciledIntervention != "evt-human-stop" {
		t.Fatalf("unexpected reconcile apply: %+v", reconciled)
	}
	assertContinueWrite(t, reconciled.Writes, ".rekit/facts/interventions.jsonl", "append")
	assertWriteKind(t, reconciled.Writes, "lane-event", "append-intervention-reconciled")
	assertWriteKind(t, reconciled.Writes, "lane-event", "append-executor-takeover")
	assertContinueWrite(t, reconciled.Writes, ".rekit/lanes/feature-login/lane.json", "update-reconcile-state")
	assertContinueWrite(t, reconciled.Writes, ".rekit/lanes/feature-login/prompts/RESUME.md", "refresh")
	if !slices.Contains(reconciled.MissionBrief.ReadyLanes, "login") || containsSubstring(reconciled.MissionBrief.BlockedLanes, "intervention") {
		t.Fatalf("reconcile did not clear intervention blocker: %+v", reconciled.MissionBrief)
	}
	interventions, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(interventions), `"resolvesEventId":"evt-human-stop"`) || !strings.Contains(string(interventions), `"status":"resolved"`) {
		t.Fatalf("reconcile did not append resolution event:\n%s", string(interventions))
	}
	laneText, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "feature-login", "lane.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(laneText), `"currentExecutor": "session-2"`) || !strings.Contains(string(laneText), `"lastReconciledIntervention": "evt-human-stop"`) {
		t.Fatalf("lane state missing executor/reconcile fields:\n%s", string(laneText))
	}
	resume, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "feature-login", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resume), "current executor: `session-2`") || !strings.Contains(string(resume), "Open interventions") || strings.Contains(string(resume), "human correction") {
		t.Fatalf("resume missing executor state or still lists resolved intervention:\n%s", string(resume))
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "vmp-re", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	var cont struct {
		Command string                  `json:"command"`
		Applied bool                    `json:"applied"`
		Blocked bool                    `json:"blocked"`
		Summary struct{ Collected int } `json:"summary"`
		Writes  []startWrite            `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &cont); err != nil {
		t.Fatalf("post-reconcile continue stdout is not JSON: %v\n%s", err, out.String())
	}
	if cont.Command != "continue" || !cont.Applied || cont.Blocked || cont.Summary.Collected != 3 {
		t.Fatalf("post-reconcile continue did not proceed: %+v", cont)
	}
	assertWriteKind(t, cont.Writes, "run-status", "write")
}

func TestRunContinueWhatIfDoesNotWrite(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	writeContinueFixture(t, caseRoot)
	before := snapshotFiles(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "vmp-re", "-WhatIf", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command              string    `json:"command"`
		IsMutation           bool      `json:"isMutation"`
		Applied              bool      `json:"applied"`
		RequiresConfirmation bool      `json:"requiresConfirmation"`
		Selector             string    `json:"selector"`
		Lane                 startLane `json:"lane"`
		Summary              struct {
			Collected, Observations, Requests, Routed, Candidates, AuthorityApplied, AuthorityWouldAppend, PendingUser int
		} `json:"summary"`
		MissionBrief missionBrief `json:"missionBrief"`
		Inputs       []string     `json:"inputs"`
		PacketRefs   []string     `json:"packetRefs"`
		Events       []struct {
			Kind          string       `json:"kind"`
			Decision      string       `json:"decision"`
			Reason        string       `json:"reason"`
			TargetLane    string       `json:"targetLane"`
			AuthorityFile string       `json:"authorityFile"`
			Rows          int          `json:"rows"`
			WouldWrites   []startWrite `json:"wouldWrites"`
		} `json:"events"`
		WouldWrites []startWrite `json:"wouldWrites"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("continue what-if stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "continue" || result.IsMutation || result.Applied || !result.RequiresConfirmation || result.Selector != "login" || result.Lane.ID != "feature-login" {
		t.Fatalf("unexpected continue preview result: %+v", result)
	}
	if result.Summary.Collected != 3 || result.Summary.Observations != 1 || result.Summary.Requests != 1 || result.Summary.Routed != 1 || result.Summary.Candidates != 1 || result.Summary.AuthorityApplied != 0 || result.Summary.AuthorityWouldAppend != 1 || result.Summary.PendingUser != 0 {
		t.Fatalf("unexpected continue summary: %+v", result.Summary)
	}
	var authorityEvent struct {
		Decision    string       `json:"decision"`
		Reason      string       `json:"reason"`
		Rows        int          `json:"rows"`
		WouldWrites []startWrite `json:"wouldWrites"`
	}
	for _, event := range result.Events {
		if event.Kind == "candidate" && event.AuthorityFile == "captures/vm_opcode_semantics_confirmed.csv" {
			authorityEvent.Decision = event.Decision
			authorityEvent.Reason = event.Reason
			authorityEvent.Rows = event.Rows
			authorityEvent.WouldWrites = event.WouldWrites
		}
	}
	if authorityEvent.Decision != "accept" || authorityEvent.Reason != "passed authority append policy" || authorityEvent.Rows != 1 {
		t.Fatalf("unexpected continue authority preview: %+v", result.Events)
	}
	assertContinueWrite(t, authorityEvent.WouldWrites, "captures/vm_opcode_semantics_confirmed.csv", "would-append")
	assertContinueWrite(t, authorityEvent.WouldWrites, ".rekit/runs/run-preview/backups/captures/vm_opcode_semantics_confirmed.csv", "would-create")
	assertContinueWrite(t, authorityEvent.WouldWrites, ".rekit/runs/run-preview/diffs/captures_vm_opcode_semantics_confirmed.csv.diff", "would-create")
	if result.MissionBrief.Summary == "" || !slices.Contains(result.MissionBrief.ReadyLanes, "main") || !slices.Contains(result.MissionBrief.ReadyLanes, "login") || len(result.MissionBrief.BlockedLanes) != 0 {
		t.Fatalf("continue what-if JSON missing pre-apply mission brief: %+v", result.MissionBrief)
	}
	if len(result.Inputs) != 1 || result.Inputs[0] != ".rekit/lanes/feature-login/outbox.jsonl" || len(result.PacketRefs) != 1 || result.PacketRefs[0] != "workspace/features/feature-login/packet.md" {
		t.Fatalf("unexpected continue refs: inputs=%v packets=%v", result.Inputs, result.PacketRefs)
	}
	if len(result.Events) != 3 || len(result.WouldWrites) == 0 {
		t.Fatalf("unexpected continue events/writes: %+v", result)
	}
	assertContinueWrite(t, result.WouldWrites, "captures/vm_opcode_semantics_confirmed.csv", "would-append")
	assertContinueWrite(t, result.WouldWrites, ".rekit/lanes/devirt-main/tasks.jsonl", "would-append")
	after := snapshotFiles(t, caseRoot)
	assertSnapshotEqual(t, before, after)
}

func TestRunContinueApplyWritesDigestAndFacts(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	writeContinueFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "vmp-re", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command              string    `json:"command"`
		IsMutation           bool      `json:"isMutation"`
		Applied              bool      `json:"applied"`
		RequiresConfirmation bool      `json:"requiresConfirmation"`
		RunID                string    `json:"runId"`
		BatchID              string    `json:"batchId"`
		Lane                 startLane `json:"lane"`
		Summary              struct {
			Collected, Observations, Requests, Routed, Candidates, AcceptedCandidates, Publications, AuthorityApplied, AuthorityWouldAppend, PendingUser, Skipped int
		} `json:"summary"`
		MissionBrief missionBrief `json:"missionBrief"`
		OpenRisks    []string     `json:"openRisks"`
		Writes       []startWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("continue apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "continue" || !result.IsMutation || !result.Applied || result.RequiresConfirmation || result.Lane.ID != "feature-login" || !strings.HasPrefix(result.RunID, "run-") || result.BatchID != "batch-"+result.RunID {
		t.Fatalf("unexpected continue apply result: %+v", result)
	}
	if result.Summary.Collected != 3 || result.Summary.Observations != 1 || result.Summary.Requests != 1 || result.Summary.Routed != 1 || result.Summary.Candidates != 1 || result.Summary.AuthorityApplied != 0 || result.Summary.AuthorityWouldAppend != 0 || result.Summary.PendingUser != 1 || result.Summary.Skipped != 0 {
		t.Fatalf("unexpected continue apply summary: %+v", result.Summary)
	}
	if result.MissionBrief.Summary == "" || !slices.Contains(result.MissionBrief.ReadyLanes, "main") || !slices.Contains(result.MissionBrief.BlockedLanes, "login (open-decision)") || !containsSubstring(result.MissionBrief.OpenDecisions, "candidate: authority candidate") || !containsSubstring(result.MissionBrief.NextAgentActions, "review open candidate/decision item(s)") {
		t.Fatalf("continue apply JSON missing post-apply mission brief: %+v", result.MissionBrief)
	}
	assertContinueWrite(t, result.Writes, ".rekit/facts/observations.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/facts/requests.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/facts/candidates.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/facts/decisions.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/lanes/devirt-main/tasks.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/lanes/devirt-main/inbox.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/board.json", "refresh")
	var statusPath string
	var digestPath string
	for _, write := range result.Writes {
		if write.Kind == "run-status" && write.Action == "write" {
			statusPath = write.TargetPath
		}
		if write.Kind == "run-digest" && write.Action == "write" {
			digestPath = write.TargetPath
		}
	}
	if statusPath == "" || digestPath == "" {
		t.Fatalf("continue apply did not report run artifact writes: %+v", result.Writes)
	}
	status, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(status), "\"missionBrief\"") || !strings.Contains(string(status), "candidate: authority candidate") {
		t.Fatalf("continue status missing mission brief:\n%s", string(status))
	}
	digest, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(digest), "# rekit continue digest") || !strings.Contains(string(digest), "pendingUser: 1") || !strings.Contains(string(digest), "## Mission Control brief") || !strings.Contains(string(digest), "candidate: authority candidate") {
		t.Fatalf("unexpected continue digest:\n%s", string(digest))
	}
	decisions, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(decisions), "evt-continue-") != 3 || !strings.Contains(string(decisions), "authority append requires explicit user confirmation") {
		t.Fatalf("unexpected continue decisions:\n%s", string(decisions))
	}
	tasks, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "devirt-main", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tasks), "req-continue") {
		t.Fatalf("continue apply did not route request:\n%s", string(tasks))
	}
	authority, err := os.ReadFile(filepath.Join(caseRoot, "captures", "vm_opcode_semantics_confirmed.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(authority), "OP_CONTINUE") {
		t.Fatalf("continue apply wrote authority csv without confirmation:\n%s", string(authority))
	}
}

func TestRunGoWorkstreamE2EStartNoteContinueHandoff(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if !start.IsMutation || !start.Applied || start.Lane.ID != "feature-login" || start.Lane.Workspace != "workspace/features/feature-login" {
		t.Fatalf("unexpected start result: %+v", start)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "observation", "-Lane", "feature-login", "-Subject", "review note", "-Summary", "manual review accepted", "-Actor", "runtime-test", "-EventId", "evt-e2e-note"}, &out); err != nil {
		t.Fatal(err)
	}
	observations, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(observations), "evt-e2e-note") {
		t.Fatalf("note append did not write observation:\n%s", string(observations))
	}

	writeCaseFile(t, caseRoot, "workspace/features/feature-login/packet.md", "# packet\n\nfeature login packet\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/outbox.jsonl", strings.Join([]string{
		`{"eventId":"evt-e2e-observation","kind":"observation","subject":"workspace observation","summary":"feature login branch observed","evidence":"evidence-observation-token"}`,
		`{"eventId":"evt-e2e-request","kind":"request","subject":"main review request","summary":"route accepted candidate to main","requestId":"req-e2e-main","targetLane":"main","evidence":"evidence-request-token","status":"open"}`,
		`{"eventId":"evt-e2e-candidate","kind":"candidate","subject":"candidate login flow","summary":"candidate from feature lane","confidence":"0.95","evidence":"evidence-candidate-token","status":"open"}`,
		`{"eventId":"evt-e2e-publication","kind":"publication","subject":"publication summary","summary":"share feature summary","evidence":"evidence-publication-token"}`,
	}, "\n")+"\n")

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	var cont struct {
		Command    string                                                                                                                      `json:"command"`
		IsMutation bool                                                                                                                        `json:"isMutation"`
		Applied    bool                                                                                                                        `json:"applied"`
		RunID      string                                                                                                                      `json:"runId"`
		Lane       startLane                                                                                                                   `json:"lane"`
		Summary    struct{ Collected, Observations, Requests, Routed, Candidates, AcceptedCandidates, Publications, PendingUser, Skipped int } `json:"summary"`
		Writes     []startWrite                                                                                                                `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &cont); err != nil {
		t.Fatalf("continue apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if cont.Command != "continue" || !cont.IsMutation || !cont.Applied || cont.Lane.ID != "feature-login" || !strings.HasPrefix(cont.RunID, "run-") {
		t.Fatalf("unexpected continue result: %+v", cont)
	}
	if cont.Summary.Collected != 4 || cont.Summary.Observations != 1 || cont.Summary.Requests != 1 || cont.Summary.Routed != 1 || cont.Summary.Candidates != 1 || cont.Summary.AcceptedCandidates != 1 || cont.Summary.Publications != 1 || cont.Summary.PendingUser != 0 || cont.Summary.Skipped != 0 {
		t.Fatalf("unexpected continue summary: %+v", cont.Summary)
	}
	assertContinueWrite(t, cont.Writes, ".rekit/facts/observations.jsonl", "append")
	assertContinueWrite(t, cont.Writes, ".rekit/facts/requests.jsonl", "append")
	assertContinueWrite(t, cont.Writes, ".rekit/facts/candidates.jsonl", "append")
	assertContinueWrite(t, cont.Writes, ".rekit/facts/publications.jsonl", "append")
	assertContinueWrite(t, cont.Writes, ".rekit/facts/decisions.jsonl", "append")
	assertContinueWrite(t, cont.Writes, ".rekit/lanes/main/tasks.jsonl", "append")
	assertContinueWrite(t, cont.Writes, ".rekit/board.json", "refresh")

	mainTasks, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "tasks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainTasks), "req-e2e-main") || !strings.Contains(string(mainTasks), "feature-login") {
		t.Fatalf("continue did not route request to main tasks:\n%s", string(mainTasks))
	}
	candidates, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "candidates.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(candidates), "evt-e2e-candidate") || !strings.Contains(string(candidates), "accepted-shared") {
		t.Fatalf("continue did not accept non-authority candidate:\n%s", string(candidates))
	}
	decisions, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "decisions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(decisions), "evt-e2e-candidate") || !strings.Contains(string(decisions), "candidate has evidence") {
		t.Fatalf("continue decisions missing accepted candidate reason:\n%s", string(decisions))
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	laneHandoff := decodeHandoffResult(t, out.Bytes())
	if !laneHandoff.IsMutation || !laneHandoff.Applied || laneHandoff.Project || laneHandoff.Lane == nil || laneHandoff.Lane.ID != "feature-login" {
		t.Fatalf("unexpected lane handoff result: %+v", laneHandoff)
	}
	laneLatest := assertStartWrite(t, laneHandoff.Writes, ".rekit/handovers/feature-login-latest.md", "write-latest-lane-handoff")
	laneText, err := os.ReadFile(laneLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 工作线接手：feature-login", "workspace/features/feature-login/packet.md", "## decision", "candidate has evidence", "digest.md"} {
		if !strings.Contains(string(laneText), expected) {
			t.Fatalf("lane handoff missing %q:\n%s", expected, string(laneText))
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	projectHandoff := decodeHandoffResult(t, out.Bytes())
	if !projectHandoff.IsMutation || !projectHandoff.Applied || !projectHandoff.Project {
		t.Fatalf("unexpected project handoff result: %+v", projectHandoff)
	}
	projectLatest := assertStartWrite(t, projectHandoff.Writes, ".rekit/handovers/latest.md", "write-latest-project-handoff")
	projectText, err := os.ReadFile(projectLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 项目接手索引", "/rekit continue login", "/rekit handoff login", "digest.md"} {
		if !strings.Contains(string(projectText), expected) {
			t.Fatalf("project handoff missing %q:\n%s", expected, string(projectText))
		}
	}
}

func TestRunGoGateDispatchE2EPlanGateOverviewHandoff(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if !start.IsMutation || !start.Applied || start.Lane.ID != "feature-login" {
		t.Fatalf("unexpected start result: %+v", start)
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "request-review", "-Items", "req-debug,candidate-login", "-ItemsPerAgent", "1", "-MaxParallel", "2"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	if plan.Command != "plan-subagents" || plan.IsMutation || !plan.WritesReviewArtifacts || !plan.ReviewRequired || plan.ItemCount != 2 || plan.ShardCount != 2 {
		t.Fatalf("unexpected plan-subagents result: %+v", plan)
	}
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if packet.Route.ID != "_template:lane-feature-analysis" || packet.Observability.RouteDebug.SelectedBy != "taskType" || packet.ShardPolicy.TargetItemsPerAgent != 1 || packet.ShardPolicy.MaxParallel != 2 {
		t.Fatalf("unexpected dispatch packet: %+v", packet)
	}
	if packet.Observability.DispatchMode != "manual-main-agent" || len(packet.Observability.ShardStatuses) != 2 || packet.Observability.ShardStatuses[0].Status != "planned" || packet.ReviewLoop.SpawnOwner != "main-agent" || !slices.Contains(packet.Observability.BlockedActions, "runtime does not spawn subagents") {
		t.Fatalf("unexpected dispatch observability: %+v", packet)
	}
	summary, err := os.ReadFile(plan.SummaryPath)
	if err != nil {
		t.Fatalf("missing plan summary: %v", err)
	}
	for _, expected := range []string{"## bounded dispatch observability", "route selected by", "shard-01: `planned`", "runtime does not spawn subagents", "verdict writeback"} {
		if !strings.Contains(string(summary), expected) {
			t.Fatalf("plan summary missing %q:\n%s", expected, string(summary))
		}
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "feature-login",
		"-Actor", "runtime-test",
		"-Subject", "debug gate",
		"-Summary", "needs user confirmation before debug",
		"-TargetRef", "dispatch-review",
		"-BatchId", "batch-gate-dispatch",
		"-Scope", "handler only",
		"-Budget", "30s",
		"-TriedLightSteps", "plan-subagents,static review",
		"-StopConditions", "timeout",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var gateResult struct {
		Command    string `json:"command"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		EventID    string `json:"eventId"`
		Path       string `json:"path"`
		Event      struct {
			Kind    string `json:"kind"`
			Lane    string `json:"lane"`
			Subject string `json:"subject"`
			Status  string `json:"status"`
			Gate    struct {
				Action          string   `json:"action"`
				Scope           string   `json:"scope"`
				Budget          string   `json:"budget"`
				TriedLightSteps []string `json:"triedLightSteps"`
				StopConditions  []string `json:"stopConditions"`
			} `json:"gate"`
		} `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &gateResult); err != nil {
		t.Fatalf("gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if gateResult.Command != "gate" || !gateResult.IsMutation || !gateResult.Applied || gateResult.EventID == "" || gateResult.Path != ".rekit/facts/requests.jsonl" {
		t.Fatalf("unexpected gate apply result: %+v", gateResult)
	}
	if gateResult.Event.Kind != "request" || gateResult.Event.Lane != "feature-login" || gateResult.Event.Subject != "debug gate" || gateResult.Event.Status != "pending-gate" || gateResult.Event.Gate.Action != "debug" || gateResult.Event.Gate.Scope != "handler only" || gateResult.Event.Gate.Budget != "30s" || !slices.Contains(gateResult.Event.Gate.TriedLightSteps, "plan-subagents") || !slices.Contains(gateResult.Event.Gate.StopConditions, "timeout") {
		t.Fatalf("unexpected gate event: %+v", gateResult.Event)
	}
	requests, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(requests), gateResult.EventID) || !strings.Contains(string(requests), `"pending-gate"`) || !strings.Contains(string(requests), `"debug"`) {
		t.Fatalf("gate ledger missing pending request:\n%s", string(requests))
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var overviewResult struct {
		Counts struct {
			Requests int `json:"requests"`
		} `json:"counts"`
		Sections struct {
			PendingGates struct {
				Total  int              `json:"total"`
				Events []map[string]any `json:"events"`
			} `json:"pendingGates"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(out.Bytes(), &overviewResult); err != nil {
		t.Fatalf("overview JSON did not decode: %v\n%s", err, out.String())
	}
	if overviewResult.Counts.Requests != 1 || overviewResult.Sections.PendingGates.Total != 1 || len(overviewResult.Sections.PendingGates.Events) != 1 || overviewResult.Sections.PendingGates.Events[0]["subject"] != "debug gate" || overviewResult.Sections.PendingGates.Events[0]["lane"] != "feature-login" {
		t.Fatalf("overview did not expose pending gate: %+v", overviewResult)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "pending-gate（heavy-tool 待确认）") || !strings.Contains(text, "debug gate") || !strings.Contains(text, "action=debug") {
		t.Fatalf("overview text missing pending gate:\n%s", text)
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	handoff := decodeHandoffResult(t, out.Bytes())
	if !handoff.IsMutation || !handoff.Applied || handoff.Project || handoff.Lane == nil || handoff.Lane.ID != "feature-login" {
		t.Fatalf("unexpected handoff result: %+v", handoff)
	}
	latest := assertStartWrite(t, handoff.Writes, ".rekit/handovers/feature-login-latest.md", "write-latest-lane-handoff")
	handoffText, err := os.ReadFile(latest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 工作线接手：feature-login", "## pending-gate", "debug gate", "action=debug", "scope=handler only", "budget=30s"} {
		if !strings.Contains(string(handoffText), expected) {
			t.Fatalf("handoff missing %q:\n%s", expected, string(handoffText))
		}
	}
}

func TestRunGoReviewerDecisionE2ENoteOverviewHandoff(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if !start.IsMutation || !start.Applied || start.Lane.ID != "feature-login" {
		t.Fatalf("unexpected start result: %+v", start)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "candidate", "-Lane", "feature-login", "-Subject", "candidate login flow", "-Summary", "candidate awaiting reviewer verdict", "-Actor", "feature-agent", "-Confidence", "high", "-Status", "open", "-Risk", "medium", "-TargetRef", "candidate-login", "-BatchId", "batch-review", "-EvidenceRefs", "workspace/features/feature-login/packet.md"}, &out); err != nil {
		t.Fatal(err)
	}
	var candidate struct {
		Applied bool           `json:"applied"`
		EventID string         `json:"eventId"`
		Path    string         `json:"path"`
		Event   map[string]any `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &candidate); err != nil {
		t.Fatalf("candidate note stdout is not JSON: %v\n%s", err, out.String())
	}
	if !candidate.Applied || candidate.EventID == "" || candidate.Path != ".rekit/facts/candidates.jsonl" || candidate.Event["kind"] != "candidate" || candidate.Event["status"] != "open" || candidate.Event["confidence"] != "high" {
		t.Fatalf("unexpected candidate append result: %+v", candidate)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "verification", "-Lane", "feature-login", "-Subject", "review target", "-Summary", "reviewer accepted candidate evidence", "-Actor", "reviewer-smoke", "-Verifier", "manual-review", "-Verdict", "accepted", "-TargetRef", "candidate-login", "-BatchId", "batch-review", "-EvidenceRefs", "workspace/features/feature-login/packet.md"}, &out); err != nil {
		t.Fatal(err)
	}
	var verification struct {
		Applied bool           `json:"applied"`
		Event   map[string]any `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &verification); err != nil {
		t.Fatalf("verification note stdout is not JSON: %v\n%s", err, out.String())
	}
	if !verification.Applied || verification.Event["kind"] != "verification" || verification.Event["verifier"] != "manual-review" || verification.Event["verdict"] != "accepted" || verification.Event["target"] != "candidate-login" {
		t.Fatalf("unexpected verification append result: %+v", verification)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Kind", "decision", "-Lane", "feature-login", "-Subject", "merge decision", "-Summary", "main accepted reviewed candidate", "-Actor", "runtime-test", "-Decision", "accept", "-Reason", "reviewer accepted evidence", "-TargetRef", "candidate-login", "-BatchId", "batch-review"}, &out); err != nil {
		t.Fatal(err)
	}
	var decision struct {
		Applied bool           `json:"applied"`
		Event   map[string]any `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &decision); err != nil {
		t.Fatalf("decision note stdout is not JSON: %v\n%s", err, out.String())
	}
	if !decision.Applied || decision.Event["kind"] != "decision" || decision.Event["decision"] != "accept" || decision.Event["actor"] != "runtime-test" || decision.Event["reason"] != "reviewer accepted evidence" {
		t.Fatalf("unexpected decision append result: %+v", decision)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-List", "-Kind", "verification", "-Lane", "feature-login", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var list struct {
		EventCount int `json:"eventCount"`
		Groups     []struct {
			Kind   string           `json:"kind"`
			Events []map[string]any `json:"events"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		t.Fatalf("verification list stdout is not JSON: %v\n%s", err, out.String())
	}
	if list.EventCount != 1 || len(list.Groups) != 1 || list.Groups[0].Kind != "verification" || len(list.Groups[0].Events) != 1 || list.Groups[0].Events[0]["verdict"] != "accepted" {
		t.Fatalf("unexpected verification list: %+v", list)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-List", "-Kind", "decision", "-Lane", "feature-login"}, &out); err != nil {
		t.Fatal(err)
	}
	if text := out.String(); !strings.Contains(text, "[decision] (1 条)") || !strings.Contains(text, "merge decision") || !strings.Contains(text, "decision=accept") || !strings.Contains(text, "by=runtime-test") || !strings.Contains(text, "batch=batch-review") {
		t.Fatalf("decision note list missing expected fields:\n%s", text)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var overviewResult struct {
		Counts struct {
			Candidates       int `json:"candidates"`
			PendingDecisions int `json:"pendingDecisions"`
		} `json:"counts"`
		Sections struct {
			OpenCandidates struct {
				Total  int              `json:"total"`
				Events []map[string]any `json:"events"`
			} `json:"openCandidates"`
			Verifications struct {
				Total  int              `json:"total"`
				Events []map[string]any `json:"events"`
			} `json:"verifications"`
			Decisions struct {
				Total  int              `json:"total"`
				Events []map[string]any `json:"events"`
			} `json:"decisions"`
			Batches struct {
				Total   int `json:"total"`
				Batches []struct {
					ID     string         `json:"id"`
					Events int            `json:"events"`
					Kinds  map[string]int `json:"kinds"`
				} `json:"batches"`
			} `json:"batches"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(out.Bytes(), &overviewResult); err != nil {
		t.Fatalf("overview JSON did not decode: %v\n%s", err, out.String())
	}
	if overviewResult.Counts.Candidates != 1 || overviewResult.Counts.PendingDecisions != 0 || overviewResult.Sections.OpenCandidates.Total != 1 || overviewResult.Sections.Verifications.Total != 1 || overviewResult.Sections.Decisions.Total != 1 || overviewResult.Sections.Batches.Total != 1 {
		t.Fatalf("unexpected overview reviewer/decision counts: %+v", overviewResult)
	}
	if overviewResult.Sections.OpenCandidates.Events[0]["subject"] != "candidate login flow" || overviewResult.Sections.Verifications.Events[0]["verdict"] != "accepted" || overviewResult.Sections.Decisions.Events[0]["decision"] != "accept" || overviewResult.Sections.Batches.Batches[0].ID != "batch-review" || overviewResult.Sections.Batches.Batches[0].Events != 3 || overviewResult.Sections.Batches.Batches[0].Kinds["verification"] != 1 {
		t.Fatalf("unexpected overview reviewer/decision details: %+v", overviewResult.Sections)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"未决 candidate：", "candidate login flow", "最近 verification：", "verifier=manual-review", "verdict=accepted", "最近 decision：", "decision=accept", "by=runtime-test", "reviewer accepted evidence"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("overview text missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	handoff := decodeHandoffResult(t, out.Bytes())
	if !handoff.IsMutation || !handoff.Applied || handoff.Project || handoff.Lane == nil || handoff.Lane.ID != "feature-login" {
		t.Fatalf("unexpected handoff result: %+v", handoff)
	}
	latest := assertStartWrite(t, handoff.Writes, ".rekit/handovers/feature-login-latest.md", "write-latest-lane-handoff")
	handoffText, err := os.ReadFile(latest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 工作线接手：feature-login", "## verification", "verifier=manual-review", "verdict=accepted", "target=candidate-login", "by=reviewer-smoke", "## decision", "decision=accept", "by=runtime-test", "reviewer accepted evidence"} {
		if !strings.Contains(string(handoffText), expected) {
			t.Fatalf("handoff missing %q:\n%s", expected, string(handoffText))
		}
	}
}

func TestRunGoGenericBinaryPackNeutralE2EStartPlanGateOverviewHandoff(t *testing.T) {
	const pack = "generic-binary-re"
	caseRoot := attachedCaseWithPack(t, pack)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", pack, "-Name", "sample", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if !start.IsMutation || !start.Applied || start.Lane.ID != "binary-analysis-sample" || start.Lane.Type != "binary-analysis" || start.Lane.Workspace != "workspace/binary/binary-analysis-sample" {
		t.Fatalf("unexpected generic binary start result: %+v", start)
	}
	assertStartWrite(t, start.Writes, ".rekit/lanes/main/lane.json", "create-lane")
	assertStartWrite(t, start.Writes, ".rekit/lanes/binary-analysis-sample/lane.json", "create-lane")

	writeCaseFile(t, caseRoot, "workspace/binary/binary-analysis-sample/packet.md", "# packet\n\ngeneric binary analysis packet\n")

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", pack, "-TaskType", "binary-analysis", "-Items", "function-init,string-login", "-ItemsPerAgent", "1", "-MaxParallel", "2"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	if plan.Command != "plan-subagents" || plan.IsMutation || !plan.WritesReviewArtifacts || !plan.ReviewRequired || plan.ItemCount != 2 || plan.ShardCount != 2 {
		t.Fatalf("unexpected generic binary plan result: %+v", plan)
	}
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if packet.Route.ID != "generic-binary-re:binary-analysis" || packet.Observability.RouteDebug.SelectedBy != "taskType" || packet.ShardPolicy.TargetItemsPerAgent != 1 || packet.ShardPolicy.MaxParallel != 2 {
		t.Fatalf("unexpected generic binary dispatch packet: %+v", packet)
	}
	if packet.Observability.DispatchMode != "manual-main-agent" || len(packet.Observability.ShardStatuses) != 2 || packet.Observability.ShardStatuses[0].Status != "planned" || packet.ReviewLoop.SpawnOwner != "main-agent" || !slices.Contains(packet.ReviewLoop.MainAgentOwns, "canonical-write") || !slices.Contains(packet.Observability.BlockedActions, "runtime does not spawn subagents") {
		t.Fatalf("unexpected generic binary dispatch observability: %+v", packet)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", pack, "-Kind", "candidate", "-Lane", "binary-analysis-sample", "-Subject", "binary behavior candidate", "-Summary", "candidate awaiting bounded generic binary review", "-Actor", "binary-agent", "-Confidence", "high", "-Status", "open", "-Risk", "medium", "-TargetRef", "function-init", "-BatchId", "batch-pack-neutral", "-EvidenceRefs", "workspace/binary/binary-analysis-sample/packet.md"}, &out); err != nil {
		t.Fatal(err)
	}
	var candidate struct {
		Applied bool           `json:"applied"`
		Path    string         `json:"path"`
		Event   map[string]any `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &candidate); err != nil {
		t.Fatalf("generic binary candidate stdout is not JSON: %v\n%s", err, out.String())
	}
	if !candidate.Applied || candidate.Path != ".rekit/facts/candidates.jsonl" || candidate.Event["kind"] != "candidate" || candidate.Event["lane"] != "binary-analysis-sample" || candidate.Event["status"] != "open" || candidate.Event["target"] != "function-init" {
		t.Fatalf("unexpected generic binary candidate append result: %+v", candidate)
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", pack,
		"-Apply",
		"-Action", "debug",
		"-Lane", "binary-analysis-sample",
		"-Actor", "runtime-test",
		"-Subject", "generic binary debug gate",
		"-Summary", "needs user confirmation before interactive binary debugging",
		"-TargetRef", "function-init",
		"-BatchId", "batch-pack-neutral",
		"-Scope", "function behavior only",
		"-Budget", "30s",
		"-TriedLightSteps", "plan-subagents,static triage",
		"-StopConditions", "timeout",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var gateResult struct {
		Command    string `json:"command"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		Event      struct {
			Kind    string `json:"kind"`
			Lane    string `json:"lane"`
			Subject string `json:"subject"`
			Status  string `json:"status"`
			Gate    struct {
				Action string `json:"action"`
				Scope  string `json:"scope"`
				Budget string `json:"budget"`
			} `json:"gate"`
		} `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &gateResult); err != nil {
		t.Fatalf("generic binary gate stdout is not JSON: %v\n%s", err, out.String())
	}
	if gateResult.Command != "gate" || !gateResult.IsMutation || !gateResult.Applied || gateResult.Event.Kind != "request" || gateResult.Event.Lane != "binary-analysis-sample" || gateResult.Event.Status != "pending-gate" || gateResult.Event.Gate.Action != "debug" || gateResult.Event.Gate.Scope != "function behavior only" || gateResult.Event.Gate.Budget != "30s" {
		t.Fatalf("unexpected generic binary gate result: %+v", gateResult)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", pack, "-Kind", "verification", "-Lane", "binary-analysis-sample", "-Subject", "bounded review target", "-Summary", "bounded reviewer accepted generic binary evidence", "-Actor", "reviewer-smoke", "-Verifier", "tool-review", "-Verdict", "accepted", "-TargetRef", "function-init", "-BatchId", "batch-pack-neutral", "-EvidenceRefs", "workspace/binary/binary-analysis-sample/packet.md"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", pack, "-Kind", "decision", "-Lane", "binary-analysis-sample", "-Subject", "generic binary merge decision", "-Summary", "main accepted reviewed generic binary candidate", "-Actor", "runtime-test", "-Decision", "accept", "-Reason", "reviewer accepted generic binary evidence", "-TargetRef", "function-init", "-BatchId", "batch-pack-neutral"}, &out); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", pack, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var overviewResult struct {
		Counts struct {
			Candidates       int `json:"candidates"`
			Requests         int `json:"requests"`
			PendingDecisions int `json:"pendingDecisions"`
		} `json:"counts"`
		Sections struct {
			OpenCandidates struct {
				Total int `json:"total"`
			} `json:"openCandidates"`
			PendingGates struct {
				Total int `json:"total"`
			} `json:"pendingGates"`
			Verifications struct {
				Total int `json:"total"`
			} `json:"verifications"`
			Decisions struct {
				Total int `json:"total"`
			} `json:"decisions"`
			Batches struct {
				Total int `json:"total"`
			} `json:"batches"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(out.Bytes(), &overviewResult); err != nil {
		t.Fatalf("generic binary overview JSON did not decode: %v\n%s", err, out.String())
	}
	if overviewResult.Counts.Candidates != 1 || overviewResult.Counts.Requests != 1 || overviewResult.Counts.PendingDecisions != 0 || overviewResult.Sections.OpenCandidates.Total != 1 || overviewResult.Sections.PendingGates.Total != 1 || overviewResult.Sections.Verifications.Total != 1 || overviewResult.Sections.Decisions.Total != 1 || overviewResult.Sections.Batches.Total != 1 {
		t.Fatalf("unexpected generic binary overview counts: %+v", overviewResult)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", pack}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"binary behavior candidate", "generic binary debug gate", "action=debug", "verifier=tool-review", "verdict=accepted", "decision=accept", "reviewer accepted generic binary evidence"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("generic binary overview text missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", pack, "-Apply", "sample"}, &out); err != nil {
		t.Fatal(err)
	}
	handoff := decodeHandoffResult(t, out.Bytes())
	if !handoff.IsMutation || !handoff.Applied || handoff.Project || handoff.Lane == nil || handoff.Lane.ID != "binary-analysis-sample" || handoff.Lane.Workspace != "workspace/binary/binary-analysis-sample" {
		t.Fatalf("unexpected generic binary handoff result: %+v", handoff)
	}
	latest := assertStartWrite(t, handoff.Writes, ".rekit/handovers/binary-analysis-sample-latest.md", "write-latest-lane-handoff")
	handoffText, err := os.ReadFile(latest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 工作线接手：binary-analysis-sample", "执行 `/rekit continue binary-analysis-sample`", "workspace/binary/binary-analysis-sample/packet.md", "## verification", "verifier=tool-review", "## decision", "decision=accept", "## pending-gate", "action=debug", "scope=function behavior only"} {
		if !strings.Contains(string(handoffText), expected) {
			t.Fatalf("generic binary handoff missing %q:\n%s", expected, string(handoffText))
		}
	}
	for _, forbidden := range []string{"feature-login", "workspace/features", "references/template"} {
		if strings.Contains(string(handoffText), forbidden) {
			t.Fatalf("generic binary handoff leaked template/feature token %q:\n%s", forbidden, string(handoffText))
		}
	}
}

func TestRunGoWebSecurityPackNeutralE2EStartPlanGateOverviewHandoff(t *testing.T) {
	const pack = "web-security"
	caseRoot := attachedCaseWithPack(t, pack)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", pack, "-Name", "authz", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if !start.IsMutation || !start.Applied || start.Lane.ID != "feature-authz" || start.Lane.Type != "feature" || start.Lane.Workspace != "workspace/features/feature-authz" {
		t.Fatalf("unexpected web-security start result: %+v", start)
	}
	assertStartWrite(t, start.Writes, ".rekit/lanes/main/lane.json", "create-lane")
	assertStartWrite(t, start.Writes, ".rekit/lanes/feature-authz/lane.json", "create-lane")

	writeCaseFile(t, caseRoot, "workspace/features/feature-authz/packet.md", "# packet\n\nweb endpoint authorization packet\n")

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", pack, "-TaskType", "endpoint-analysis", "-Items", "endpoint-login,api-flow-authz", "-ItemsPerAgent", "1", "-MaxParallel", "2"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	if plan.Command != "plan-subagents" || plan.IsMutation || !plan.WritesReviewArtifacts || !plan.ReviewRequired || plan.ItemCount != 2 || plan.ShardCount != 2 {
		t.Fatalf("unexpected web-security plan result: %+v", plan)
	}
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if packet.Route.ID != "web-security:feature-analysis" || packet.Observability.RouteDebug.SelectedBy != "taskType" || packet.ShardPolicy.TargetItemsPerAgent != 1 || packet.ShardPolicy.MaxParallel != 2 {
		t.Fatalf("unexpected web-security dispatch packet: %+v", packet)
	}
	if packet.Observability.DispatchMode != "manual-main-agent" || len(packet.Observability.ShardStatuses) != 2 || packet.Observability.ShardStatuses[0].Status != "planned" || packet.ReviewLoop.SpawnOwner != "main-agent" || !slices.Contains(packet.ReviewLoop.MainAgentOwns, "canonical-write") || !slices.Contains(packet.Observability.BlockedActions, "runtime does not spawn subagents") {
		t.Fatalf("unexpected web-security dispatch observability: %+v", packet)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", pack, "-Kind", "candidate", "-Lane", "feature-authz", "-Subject", "endpoint authz candidate", "-Summary", "candidate awaiting bounded web review", "-Actor", "web-agent", "-Confidence", "high", "-Status", "open", "-Risk", "high", "-TargetRef", "endpoint-login", "-BatchId", "batch-web-neutral", "-EvidenceRefs", "workspace/features/feature-authz/packet.md"}, &out); err != nil {
		t.Fatal(err)
	}
	var candidate struct {
		Applied bool           `json:"applied"`
		Path    string         `json:"path"`
		Event   map[string]any `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &candidate); err != nil {
		t.Fatalf("web-security candidate stdout is not JSON: %v\n%s", err, out.String())
	}
	if !candidate.Applied || candidate.Path != ".rekit/facts/candidates.jsonl" || candidate.Event["kind"] != "candidate" || candidate.Event["lane"] != "feature-authz" || candidate.Event["status"] != "open" || candidate.Event["target"] != "endpoint-login" || candidate.Event["risk"] != "high" {
		t.Fatalf("unexpected web-security candidate append result: %+v", candidate)
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", pack,
		"-Apply",
		"-Action", "network",
		"-Lane", "feature-authz",
		"-Actor", "runtime-test",
		"-Subject", "web request replay gate",
		"-Summary", "needs user confirmation before request replay",
		"-TargetRef", "endpoint-login",
		"-BatchId", "batch-web-neutral",
		"-Scope", "single endpoint replay",
		"-Budget", "30s",
		"-TriedLightSteps", "plan-subagents,passive triage",
		"-StopConditions", "timeout",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var gateResult struct {
		Command    string `json:"command"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		Event      struct {
			Kind    string `json:"kind"`
			Lane    string `json:"lane"`
			Subject string `json:"subject"`
			Status  string `json:"status"`
			Gate    struct {
				Action string `json:"action"`
				Scope  string `json:"scope"`
				Budget string `json:"budget"`
			} `json:"gate"`
		} `json:"event"`
	}
	if err := json.Unmarshal(out.Bytes(), &gateResult); err != nil {
		t.Fatalf("web-security gate stdout is not JSON: %v\n%s", err, out.String())
	}
	if gateResult.Command != "gate" || !gateResult.IsMutation || !gateResult.Applied || gateResult.Event.Kind != "request" || gateResult.Event.Lane != "feature-authz" || gateResult.Event.Status != "pending-gate" || gateResult.Event.Gate.Action != "network" || gateResult.Event.Gate.Scope != "single endpoint replay" || gateResult.Event.Gate.Budget != "30s" {
		t.Fatalf("unexpected web-security gate result: %+v", gateResult)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", pack, "-Kind", "verification", "-Lane", "feature-authz", "-Subject", "bounded endpoint review", "-Summary", "bounded reviewer accepted web endpoint evidence", "-Actor", "reviewer-smoke", "-Verifier", "manual-review", "-Verdict", "accepted", "-TargetRef", "endpoint-login", "-BatchId", "batch-web-neutral", "-EvidenceRefs", "workspace/features/feature-authz/packet.md"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", pack, "-Kind", "decision", "-Lane", "feature-authz", "-Subject", "web merge decision", "-Summary", "main accepted reviewed web endpoint candidate", "-Actor", "runtime-test", "-Decision", "accept", "-Reason", "reviewer accepted web endpoint evidence", "-TargetRef", "endpoint-login", "-BatchId", "batch-web-neutral"}, &out); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", pack, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var overviewResult struct {
		Counts struct {
			Candidates       int `json:"candidates"`
			Requests         int `json:"requests"`
			PendingDecisions int `json:"pendingDecisions"`
		} `json:"counts"`
		Sections struct {
			OpenCandidates struct {
				Total int `json:"total"`
			} `json:"openCandidates"`
			PendingGates struct {
				Total int `json:"total"`
			} `json:"pendingGates"`
			Verifications struct {
				Total int `json:"total"`
			} `json:"verifications"`
			Decisions struct {
				Total int `json:"total"`
			} `json:"decisions"`
			Batches struct {
				Total int `json:"total"`
			} `json:"batches"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(out.Bytes(), &overviewResult); err != nil {
		t.Fatalf("web-security overview JSON did not decode: %v\n%s", err, out.String())
	}
	if overviewResult.Counts.Candidates != 1 || overviewResult.Counts.Requests != 1 || overviewResult.Counts.PendingDecisions != 0 || overviewResult.Sections.OpenCandidates.Total != 1 || overviewResult.Sections.PendingGates.Total != 1 || overviewResult.Sections.Verifications.Total != 1 || overviewResult.Sections.Decisions.Total != 1 || overviewResult.Sections.Batches.Total != 1 {
		t.Fatalf("unexpected web-security overview counts: %+v", overviewResult)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", pack}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"endpoint authz candidate", "web request replay gate", "action=network", "verifier=manual-review", "verdict=accepted", "decision=accept", "reviewer accepted web endpoint evidence"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("web-security overview text missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", pack, "-Apply", "authz"}, &out); err != nil {
		t.Fatal(err)
	}
	handoff := decodeHandoffResult(t, out.Bytes())
	if !handoff.IsMutation || !handoff.Applied || handoff.Project || handoff.Lane == nil || handoff.Lane.ID != "feature-authz" || handoff.Lane.Workspace != "workspace/features/feature-authz" {
		t.Fatalf("unexpected web-security handoff result: %+v", handoff)
	}
	latest := assertStartWrite(t, handoff.Writes, ".rekit/handovers/feature-authz-latest.md", "write-latest-lane-handoff")
	handoffText, err := os.ReadFile(latest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 工作线接手：feature-authz", "执行 `/rekit continue authz`", "workspace/features/feature-authz/packet.md", "## verification", "verifier=manual-review", "## decision", "decision=accept", "## pending-gate", "action=network", "scope=single endpoint replay"} {
		if !strings.Contains(string(handoffText), expected) {
			t.Fatalf("web-security handoff missing %q:\n%s", expected, string(handoffText))
		}
	}
	for _, forbidden := range []string{"generic-binary-re", "workspace/binary", "binary-analysis-sample", "references/template", "vmp-re"} {
		if strings.Contains(string(handoffText), forbidden) {
			t.Fatalf("web-security handoff leaked non-web token %q:\n%s", forbidden, string(handoffText))
		}
	}
}

func TestRunContinueRejectsUnsupportedModes(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	writeContinueFixture(t, caseRoot)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no mode", []string{"-Command", "continue", "-Target", caseRoot, "-Pack", "vmp-re", "login"}, "requires -WhatIf or -Apply"},
		{"what-if apply", []string{"-Command", "continue", "-Target", caseRoot, "-Pack", "vmp-re", "-WhatIf", "-Apply", "login"}, "cannot combine"},
		{"create candidates", []string{"-Command", "continue", "-Target", caseRoot, "-Pack", "vmp-re", "-CreateCandidates", "login"}, "does not support -CreateCandidates"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := Run(tc.args, &out)
			if err == nil {
				t.Fatal("Run returned nil error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tc.want)
			}
		})
	}
}

func TestRunContinueRequiresSelectorForMultipleOpenLanes(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	writeContinueFixture(t, caseRoot)
	var out bytes.Buffer
	err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "vmp-re", "-WhatIf"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error")
	}
	if !strings.Contains(err.Error(), "requires a lane selector") {
		t.Fatalf("error = %q, want selector guard", err.Error())
	}
}

func TestRunPlanSubagentsWritesReviewArtifacts(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "vmp-re", "-TaskType", "feature-analysis", "-Items", "alpha,beta gamma", "-ItemsPerAgent", "2", "-MaxParallel", "7"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodePlanSubagentsResult(t, out.Bytes())
	if result.Command != "plan-subagents" || result.IsMutation || !result.WritesReviewArtifacts || !result.ReviewRequired || result.ItemCount != 3 || result.ShardCount != 2 {
		t.Fatalf("unexpected plan-subagents result: %+v", result)
	}
	packet := decodePlanSubagentsPacket(t, result.PacketPath)
	if packet.Command != "plan-subagents" || packet.Route.ID != "vmp-re:lane-feature-analysis" || packet.ShardPolicy.TargetItemsPerAgent != 2 || packet.ShardPolicy.MaxParallel != 7 {
		t.Fatalf("unexpected plan-subagents packet: %+v", packet)
	}
	if len(packet.Shards) != 2 || strings.Join(packet.Shards[0].Items, ",") != "alpha,beta" || strings.Join(packet.Shards[1].Items, ",") != "gamma" {
		t.Fatalf("unexpected shards: %+v", packet.Shards)
	}
	if result.Observability.DispatchMode != "manual-main-agent" || result.Observability.RouteDebug.SelectedBy != "taskType" || result.ReviewLoop.SpawnOwner != "main-agent" {
		t.Fatalf("unexpected result observability: result=%+v", result)
	}
	if packet.Observability.DispatchMode != "manual-main-agent" || packet.Observability.RouteDebug.RouteID != "vmp-re:lane-feature-analysis" || len(packet.Observability.ShardStatuses) != 2 || packet.Observability.ShardStatuses[0].Status != "planned" || packet.ReviewLoop.MergeOwner != "main-agent" {
		t.Fatalf("unexpected packet observability: %+v", packet)
	}
	if !slices.Contains(packet.Observability.BlockedActions, "runtime does not spawn subagents") || !strings.Contains(packet.ReviewLoop.VerdictWriteback, "note -Kind verification") {
		t.Fatalf("unexpected review loop contract: %+v", packet)
	}
	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatalf("missing summary: %v", err)
	}
	for _, expected := range []string{"## bounded dispatch observability", "route selected by", "shard-01: `planned`", "runtime does not spawn subagents", "verdict writeback"} {
		if !strings.Contains(string(summary), expected) {
			t.Fatalf("summary missing %q:\n%s", expected, string(summary))
		}
	}
}

func TestRunPlanSubagentsUnknownTaskTypeReportsDefaultRoute(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "vmp-re", "-TaskType", "unknown-task", "-Items", "alpha"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, result.PacketPath)
	if packet.Route.ID != "vmp-re:bounded-review" || packet.Observability.RouteDebug.SelectedBy != "manifest-default" {
		t.Fatalf("unexpected fallback route observability: %+v", packet)
	}
}

func TestRunPlanSubagentsItemsFileAndOutOfCaseGuard(t *testing.T) {
	target := t.TempDir()
	itemsFile := filepath.Join(t.TempDir(), "items.txt")
	if err := os.WriteFile(itemsFile, []byte("one\ntwo;three"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := Run([]string{"-Command", "plan-subagents", "-Target", target, "-Pack", "vmp-re", "-ItemsFile", itemsFile}, &out)
	if err == nil || !strings.Contains(err.Error(), "unless -ReviewOutputDir") {
		t.Fatalf("error = %v, want out-of-case guard", err)
	}
	out.Reset()
	reviewRoot := filepath.Join(t.TempDir(), "review")
	if err := Run([]string{"-Command", "plan-subagents", "-Target", target, "-Pack", "vmp-re", "-Route", "vmp-re:bounded-review", "-ItemsFile", itemsFile, "-ReviewOutputDir", reviewRoot}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, result.PacketPath)
	if result.ItemCount != 3 || result.ShardCount != 1 || packet.Route.ID != "vmp-re:bounded-review" || packet.Input.ItemsFile == "" {
		t.Fatalf("unexpected out-of-case plan: result=%+v packet=%+v", result, packet)
	}
	if !samePath(result.ReviewRoot, reviewRoot) {
		t.Fatalf("review root = %q, want %q", result.ReviewRoot, reviewRoot)
	}
}

func TestRunPlanSubagentsTemplatePackRoutes(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ReviewOutputDir", t.TempDir()}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, result.PacketPath)
	if packet.Route.ID != "_template:lane-feature-analysis" || packet.Observability.RouteDebug.SelectedBy != "taskType" || result.ItemCount != 2 || result.ShardCount != 2 {
		t.Fatalf("unexpected template plan: result=%+v packet=%+v", result, packet)
	}
	if !strings.Contains(packet.ReviewLoop.VerdictWriteback, "note -Kind verification") || len(packet.Observability.BlockedActions) == 0 {
		t.Fatalf("template route missing review loop contract: %+v", packet)
	}
}

func TestRunPlanSubagentsRejectsDefaultArtifactEscape(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	var out bytes.Buffer
	err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "vmp-re", "-Items", "alpha", "-PacketPath", filepath.Join(t.TempDir(), "packet.json")}, &out)
	if err == nil || !strings.Contains(err.Error(), "packet path escapes review root") {
		t.Fatalf("error = %v, want packet path containment guard", err)
	}
}

func TestRunPlanSubagentsRejectsMutationFlags(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "vmp-re")
	for _, flag := range []string{"-Apply", "-WhatIf", "-CreateCandidates"} {
		var out bytes.Buffer
		err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "vmp-re", flag}, &out)
		if err == nil || !strings.Contains(err.Error(), "only writes review artifacts") {
			t.Fatalf("%s error = %v, want mutation flag guard", flag, err)
		}
	}
}

func TestRunPromoteApplyWhatIfEmitsNonMutatingPlan(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Apply\n\nReusable safe update.\n")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodePromoteApplyResult(t, out.Bytes())
	if result.Command != "promote" || result.IsMutation || result.Applied || result.Changed == 0 || result.BackupRoot != "" {
		t.Fatalf("unexpected promote apply what-if result: %+v", result)
	}
	readmeWrite := assertPromoteApplyWrite(t, result.Writes, "references/template/README.md", "would-promote")
	if _, err := os.Stat(readmeWrite.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("promote apply what-if created backup %s", readmeWrite.BackupPath)
	}
}

func TestRunPromoteApplyWritesPackWithBackup(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	root := repoRoot(t)
	candidateRoot := filepath.Join(root, "packs", "_template", "promote-candidates")
	candidateBefore := snapshotFiles(t, candidateRoot)
	packRefsRoot := filepath.Join(root, "packs", "_template", "references", "template")
	packRefsBefore := snapshotFiles(t, packRefsRoot)
	t.Cleanup(func() {
		removeNewFiles(t, packRefsRoot, packRefsBefore)
		removeNewFiles(t, candidateRoot, candidateBefore)
	})
	packReadme := filepath.Join(packRefsRoot, "README.md")
	original, err := os.ReadFile(packReadme)
	if err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Apply\n\nReusable safe update.\n")
	writeCaseFile(t, caseRoot, "references/template/workflow-template.md", "# Blocked\n\nDo not promote C:\\case\\artifact\\sample-trace.csv.\n")

	var out bytes.Buffer
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodePromoteApplyResult(t, out.Bytes())
	if result.Command != "promote" || !result.IsMutation || !result.Applied || result.Changed == 0 || result.Blocked == 0 || !result.RequiresCleanup || len(result.ValidationRows) == 0 {
		t.Fatalf("unexpected promote apply result: %+v", result)
	}
	readmeWrite := assertPromoteApplyWrite(t, result.Writes, "references/template/README.md", "promote")
	workflowWrite := assertPromoteApplyWrite(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	if !strings.HasPrefix(readmeWrite.TargetPath, filepath.Join(root, "packs", "_template")) || !strings.HasPrefix(readmeWrite.BackupPath, filepath.Join(candidateRoot, ".backup")) {
		t.Fatalf("promote apply paths outside pack/candidate roots: %+v", readmeWrite)
	}
	assertFileExists(t, readmeWrite.BackupPath)
	backup, err := os.ReadFile(readmeWrite.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backup, original) {
		t.Fatal("promote apply backup did not preserve original pack README")
	}
	updated, err := os.ReadFile(packReadme)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "Reusable safe update") {
		t.Fatalf("promote apply did not update pack README: %s", string(updated))
	}
	workflowPack, err := os.ReadFile(filepath.Join(packRefsRoot, "workflow-template.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(workflowPack), "sample-trace") {
		t.Fatal("blocked workflow was written to pack source")
	}
	if workflowWrite.BackupPath != "" {
		t.Fatalf("blocked write unexpectedly has backup path: %+v", workflowWrite)
	}
}

func TestRunPromoteApplyRejectsCreateCandidatesAndArtifacts(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-CreateCandidates"}, &out)
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("error = %v, want apply/create-candidates combination guard", err)
	}
	err = Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-ReviewOutputDir", filepath.Join(t.TempDir(), "review")}, &out)
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("error = %v, want apply/review artifact combination guard", err)
	}
}

func TestRunPromoteCreateCandidatesWhatIf(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Candidate\n\nReusable safe update.\n")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-CreateCandidates", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodeCandidateResult(t, out.Bytes())
	if result.Command != "promote" || result.IsMutation || result.Applied || result.Created == 0 {
		t.Fatalf("unexpected promote candidates what-if result: %+v", result)
	}
	assertCandidateWrite(t, result.Writes, "references/template/README.md", "would-create-candidate")
	if _, err := os.Stat(result.Writes[0].TargetPath); !os.IsNotExist(err) {
		t.Fatalf("promote candidates what-if created %s", result.Writes[0].TargetPath)
	}
}

func TestRunPromoteCreateCandidatesWritesCandidates(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	root := repoRoot(t)
	candidateRoot := filepath.Join(root, "packs", "_template", "promote-candidates")
	toolingRoot := filepath.Join(root, "packs", "_template", "tooling", "candidates")
	candidateBefore := snapshotFiles(t, candidateRoot)
	toolingBefore := snapshotFiles(t, toolingRoot)
	t.Cleanup(func() {
		removeNewFiles(t, candidateRoot, candidateBefore)
		removeNewFiles(t, toolingRoot, toolingBefore)
	})
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Candidate\n\nReusable safe update.\n")
	writeCaseFile(t, caseRoot, "references/template/workflow-template.md", "# Blocked\n\nDo not promote C:\\case\\artifact\\sample-trace.csv.\n")
	writeCaseFile(t, caseRoot, "references/template/toolchain-router.md", "# Tooling\n\nCase root: "+caseRoot+"\nAbsolute: C:\\cases\\demo.exe\nTrace: artifacts/run/demo-trace.csv\nAddress: 0x401000\nContext: ctx123 round7 Task #99\n")

	var out bytes.Buffer
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-CreateCandidates"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodeCandidateResult(t, out.Bytes())
	if result.Command != "promote" || !result.IsMutation || !result.Applied || result.Created != 2 || result.Blocked == 0 || !result.RequiresCleanup {
		t.Fatalf("unexpected promote candidates apply result: %+v", result)
	}
	readmeWrite := assertCandidateWrite(t, result.Writes, "references/template/README.md", "create-candidate")
	workflowWrite := assertCandidateWrite(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	toolingWrite := assertCandidateWrite(t, result.Writes, "references/template/toolchain-router.md", "create-candidate")
	if !strings.HasPrefix(readmeWrite.TargetPath, result.CandidateRoot) {
		t.Fatalf("managed candidate target %q outside %q", readmeWrite.TargetPath, result.CandidateRoot)
	}
	if !strings.HasPrefix(toolingWrite.TargetPath, result.ToolingRoot) {
		t.Fatalf("tooling candidate target %q outside %q", toolingWrite.TargetPath, result.ToolingRoot)
	}
	assertFileExists(t, readmeWrite.TargetPath)
	assertFileExists(t, toolingWrite.TargetPath)
	if workflowWrite.TargetPath != filepath.Join(repoRoot(t), "packs", "_template", filepath.FromSlash("references/template/workflow-template.md")) {
		t.Fatalf("blocked write target = %q, want pack source", workflowWrite.TargetPath)
	}
	if result.IndexPath == "" {
		t.Fatal("missing candidate index path")
	}
	assertFileExists(t, result.IndexPath)
	toolingText, err := os.ReadFile(toolingWrite.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"<caseRoot>", "<absolutePath>", "<artifactsPath>", "<address>", "<ctxNNN>", "<roundN>", "Task #<n>"} {
		if !strings.Contains(string(toolingText), expected) {
			t.Fatalf("tooling candidate missing %q:\n%s", expected, string(toolingText))
		}
	}
	for _, unexpected := range []string{caseRoot, "C:\\cases", "demo-trace.csv", "0x401000", "ctx123", "round7", "Task #99"} {
		if strings.Contains(string(toolingText), unexpected) {
			t.Fatalf("tooling candidate contains %q:\n%s", unexpected, string(toolingText))
		}
	}
}

func TestRunPromoteCreateCandidatesRejectsReviewArtifacts(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-CreateCandidates", "-ReviewOutputDir", filepath.Join(t.TempDir(), "review")}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for promote -CreateCandidates with review artifacts")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("error = %q, want combination guard", err.Error())
	}
}

func TestRunSyncReviewEmitsNonMutatingPlan(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	plan := decodePlan(t, out.Bytes())
	if plan.Command != "sync" || plan.IsMutation {
		t.Fatalf("unexpected sync review plan: %+v", plan)
	}
}

func TestRunPromoteReviewEmitsNonMutatingPlan(t *testing.T) {
	caseRoot := attachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	plan := decodePlan(t, out.Bytes())
	if plan.Command != "promote" || plan.IsMutation {
		t.Fatalf("unexpected promote review plan: %+v", plan)
	}
}

func TestRunSyncReviewWritesArtifacts(t *testing.T) {
	caseRoot := attachedCase(t)
	reviewRoot := filepath.Join(t.TempDir(), "sync-review")
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-ReviewOutputDir", reviewRoot}, &out)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeArtifactResult(t, out.Bytes())
	if result.Command != "sync" || result.IsMutation || !result.WritesArtifacts {
		t.Fatalf("unexpected artifact result: %+v", result)
	}
	assertFileExists(t, result.PacketPath)
	assertFileExists(t, result.SummaryPath)
	assertFileExists(t, result.CombinedDiffPath)
	packet, err := os.ReadFile(result.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(packet), `"command": "sync"`) || !strings.Contains(string(packet), `"reviewRequired": true`) {
		t.Fatalf("sync packet missing expected fields: %s", string(packet))
	}
}

func TestRunSyncApplyPreviewDoesNotWrite(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Local drift\n\nchanged\n")
	writeCaseFile(t, caseRoot, "references/template/task-handoff.md", "# Local handoff\n\nkeep\n")
	writeCaseFile(t, caseRoot, "CLAUDE.local.md", "prefix\n\n<!-- BEGIN template-pack:router -->\nold block\n<!-- END template-pack:router -->\n\nsuffix\n")
	before := snapshotFiles(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-WhatIf", "-ProjectName", "demo-sync-preview"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command    string      `json:"command"`
		IsMutation bool        `json:"isMutation"`
		Applied    bool        `json:"applied"`
		BackupRoot string      `json:"backupRoot"`
		Writes     []syncWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("sync apply preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "sync" || result.IsMutation || result.Applied || result.BackupRoot == "" {
		t.Fatalf("unexpected sync apply preview result: %+v", result)
	}
	assertSyncPreviewWrite(t, result.Writes, "references/template/README.md", "overwrite-with-backup", true)
	assertSyncPreviewWrite(t, result.Writes, "references/template/task-handoff.md", "skip-existing-local-file", false)
	assertSyncPreviewWrite(t, result.Writes, "CLAUDE.local.md", "replace-managed-block", true)
	if _, err := os.Stat(result.BackupRoot); !os.IsNotExist(err) {
		t.Fatalf("sync apply preview created backup root or returned stat error: %s err=%v", result.BackupRoot, err)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, caseRoot))
}

func TestRunSyncApplyWritesManagedFilesBackupAndState(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Local drift\n\nchanged\n")
	writeCaseFile(t, caseRoot, "references/template/task-handoff.md", "# Local handoff\n\nkeep\n")
	writeCaseFile(t, caseRoot, "CLAUDE.local.md", "prefix\n\n<!-- BEGIN template-pack:router -->\nold block\n<!-- END template-pack:router -->\n\nsuffix\n")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-ProjectName", "demo-sync"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command    string      `json:"command"`
		IsMutation bool        `json:"isMutation"`
		Applied    bool        `json:"applied"`
		BackupRoot string      `json:"backupRoot"`
		Writes     []syncWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("sync apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "sync" || !result.IsMutation || !result.Applied || result.BackupRoot == "" {
		t.Fatalf("unexpected sync apply result: %+v", result)
	}
	readme, err := os.ReadFile(filepath.Join(caseRoot, "references", "template", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	packReadme, err := os.ReadFile(filepath.Join(repoRoot(t), "packs", "_template", "references", "template", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readme, packReadme) {
		t.Fatal("managed README was not overwritten with pack content")
	}
	state, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(state), "targetHashAtSync") || !strings.Contains(string(state), "references/template/README.md") {
		t.Fatalf("sync state missing managed hashes: %s", string(state))
	}
	legacy, err := os.ReadFile(filepath.Join(caseRoot, ".re-template.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(legacy), "templateRoot: "+repoRoot(t)) || strings.Contains(string(legacy), "currentProjectPath") {
		t.Fatalf("legacy metadata should match attach semantics, got: %s", string(legacy))
	}
	assertSyncWrite(t, result.Writes, "references/template/README.md", "overwrite-with-backup", true)
	assertSyncWrite(t, result.Writes, "references/template/task-handoff.md", "skip-existing-local-file", false)
	assertSyncWrite(t, result.Writes, "CLAUDE.local.md", "replace-managed-block", true)
}

func TestRunSyncApplyForceOverwritesLocalTemplate(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/task-handoff.md", "# Local handoff\n")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Force", "-ProjectName", "forced-demo"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Writes []syncWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("sync apply stdout is not JSON: %v\n%s", err, out.String())
	}
	assertSyncWrite(t, result.Writes, "references/template/task-handoff.md", "overwrite-local-template-file-with-force", true)
	text, err := os.ReadFile(filepath.Join(caseRoot, "references", "template", "task-handoff.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "forced-demo") || !strings.Contains(string(text), caseRoot) {
		t.Fatalf("forced template did not replace placeholders: %s", string(text))
	}
}

func TestRunPromoteReviewWritesArtifactsAndPreview(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Case README\n\nlocal safe change\n")
	writeCaseFile(t, caseRoot, "references/template/toolchain-router.md", "# Tool route\n\nlocal safe candidate\n")
	reviewRoot := filepath.Join(t.TempDir(), "promote-review")
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-ReviewOutputDir", reviewRoot}, &out)
	if err != nil {
		t.Fatal(err)
	}
	result := decodeArtifactResult(t, out.Bytes())
	if result.Command != "promote" || result.IsMutation || !result.WritesArtifacts {
		t.Fatalf("unexpected artifact result: %+v", result)
	}
	assertFileExists(t, result.PacketPath)
	assertFileExists(t, result.SummaryPath)
	assertFileExists(t, result.CombinedDiffPath)
	assertFileExists(t, filepath.Join(reviewRoot, "previews", "references_template_toolchain-router.md.sanitized-preview.md"))
	diff, err := os.ReadFile(result.CombinedDiffPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(diff), "local safe change") {
		t.Fatalf("promote diff missing case change: %s", string(diff))
	}
}

func TestRunGateRequiresWhatIfOrApply(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Action", "debug", "-Lane", "main"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for gate without -WhatIf or -Apply")
	}
	if !strings.Contains(err.Error(), "-Apply") || !strings.Contains(err.Error(), "-WhatIf") {
		t.Fatalf("error = %q, want apply/whatif guard", err.Error())
	}
}

func TestRunGateRejectsWhatIfWithApply(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Apply", "-Action", "debug", "-Lane", "main"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for gate -WhatIf -Apply")
	}
	if !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("error = %q, want combination guard", err.Error())
	}
}

func TestRunGateDryRunEmitsNonMutatingPlan(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-WhatIf",
		"-Action", "full-trace",
		"-Lane", "main",
		"-Subject", "trace handler",
		"-TargetRef", "batch-test",
		"-BatchId", "batch-test",
		"-Scope", "handler only",
		"-Budget", "60s",
		"-TriedLightSteps", "static review,focused grep",
		"-StopConditions", "first-crash,timeout",
	}, &out)
	if err != nil {
		t.Fatal(err)
	}
	var plan struct {
		Command              string `json:"command"`
		IsMutation           bool   `json:"isMutation"`
		ReviewRequired       bool   `json:"reviewRequired"`
		RequiresConfirmation bool   `json:"requiresConfirmation"`
		EventPreview         struct {
			Kind    string `json:"kind"`
			Status  string `json:"status"`
			Lane    string `json:"lane"`
			Target  string `json:"target"`
			BatchID string `json:"batchId"`
			Gate    struct {
				Action                      string   `json:"action"`
				DeniedUntilUserConfirmation []string `json:"deniedUntilUserConfirmation"`
			} `json:"gate"`
		} `json:"eventPreview"`
		MissionBrief missionBrief `json:"missionBrief"`
	}
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("gate plan stdout is not JSON: %v\n%s", err, out.String())
	}
	if plan.Command != "gate" || plan.IsMutation || !plan.ReviewRequired || !plan.RequiresConfirmation {
		t.Fatalf("unexpected gate plan flags: %+v", plan)
	}
	if plan.EventPreview.Kind != "request" || plan.EventPreview.Status != "pending-gate" || plan.EventPreview.Lane != "main" {
		t.Fatalf("unexpected event preview: %+v", plan.EventPreview)
	}
	if plan.EventPreview.Target != "batch-test" || plan.EventPreview.BatchID != "batch-test" {
		t.Fatalf("batch/target not preserved: %+v", plan.EventPreview)
	}
	if plan.EventPreview.Gate.Action != "full-trace" || len(plan.EventPreview.Gate.DeniedUntilUserConfirmation) != 1 {
		t.Fatalf("unexpected gate detail: %+v", plan.EventPreview.Gate)
	}
	if plan.MissionBrief.Summary == "" || !slices.Contains(plan.MissionBrief.ReadyLanes, "main") || len(plan.MissionBrief.PendingGates) != 0 {
		t.Fatalf("gate plan missing pre-apply mission brief: %+v", plan.MissionBrief)
	}
}

func TestRunGateDryRunRejectsUnknownLane(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Action", "debug", "-Lane", "missing"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for unknown gate lane")
	}
	if !strings.Contains(err.Error(), "unknown lane") {
		t.Fatalf("error = %q, want unknown lane", err.Error())
	}
}

func TestRunGateDryRunRejectsInvalidRiskOverride(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Action", "debug", "-Lane", "main", "-Risk", "High"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for invalid gate risk")
	}
	if !strings.Contains(err.Error(), "gate risk has unsupported value: High") {
		t.Fatalf("error = %q, want invalid risk", err.Error())
	}
	if _, statErr := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl")); statErr == nil || !os.IsNotExist(statErr) {
		t.Fatalf("requests ledger exists or stat failed unexpectedly: %v", statErr)
	}
}

func TestRunGateApplyRequiresActor(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Action", "debug", "-Lane", "main"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for gate -Apply without -Actor")
	}
	if !strings.Contains(err.Error(), "requires -Actor") {
		t.Fatalf("error = %q, want actor guard", err.Error())
	}
}

func TestRunGateApplyAppendsPendingGateRequest(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	args := []string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "debug gate",
		"-TargetRef", "batch-apply",
		"-BatchId", "batch-apply",
		"-Scope", "display only",
	}
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Applied    bool   `json:"applied"`
		IsMutation bool   `json:"isMutation"`
		EventID    string `json:"eventId"`
		Path       string `json:"path"`
		Event      struct {
			Kind   string `json:"kind"`
			Status string `json:"status"`
		} `json:"event"`
		MissionBrief missionBrief `json:"missionBrief"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !result.Applied || !result.IsMutation || result.EventID == "" {
		t.Fatalf("unexpected apply result: %+v", result)
	}
	if result.Path != ".rekit/facts/requests.jsonl" || result.Event.Kind != "request" || result.Event.Status != "pending-gate" {
		t.Fatalf("unexpected event result: %+v", result)
	}
	if result.MissionBrief.Summary == "" || !slices.Contains(result.MissionBrief.BlockedLanes, "main (pending-gate)") || !containsSubstring(result.MissionBrief.PendingGates, "debug gate") {
		t.Fatalf("gate apply missing post-apply mission brief: %+v", result.MissionBrief)
	}
	ledger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), result.EventID) || !strings.Contains(string(ledger), `"pending-gate"`) {
		t.Fatalf("ledger does not contain gate event: %s", string(ledger))
	}
}

func TestRunGoGateApplyAppendsAuthorizedGateRequestVisibility(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeAuthorizedGateVisibilityFixture(t, caseRoot)
	var out bytes.Buffer
	args := []string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "authorized debug",
		"-TargetRef", "target-alpha",
		"-BatchId", "batch-authorized-gate",
		"-Scope", "handler only",
		"-RuntimeSeconds", "30",
		"-DiskMB", "64",
		"-Requests", "1",
		"-OutputPaths", "workspace/main/debug/session-1",
		"-StopConditions", "timeout",
	}
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Applied bool `json:"applied"`
		Event   struct {
			Kind   string `json:"kind"`
			Status string `json:"status"`
			Gate   struct {
				Authorization struct {
					Decision  string `json:"decision"`
					ProfileID string `json:"profileId"`
				} `json:"authorization"`
			} `json:"gate"`
		} `json:"event"`
		MissionBrief missionBrief `json:"missionBrief"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("authorized gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !result.Applied || result.Event.Kind != "request" || result.Event.Status != "authorized-gate" || result.Event.Gate.Authorization.Decision != "preauthorized" || result.Event.Gate.Authorization.ProfileID != "prof-main-debug" {
		t.Fatalf("unexpected authorized gate result: %+v", result)
	}
	if len(result.MissionBrief.PendingGates) != 0 || !slices.Contains(result.MissionBrief.ReadyLanes, "main") || len(result.MissionBrief.BlockedLanes) != 0 || !containsSubstring(result.MissionBrief.AuthorizedGates, "authorized debug") || !containsSubstring(result.MissionBrief.AuthorizedGates, "auth=preauthorized") {
		t.Fatalf("authorized gate mission brief should be visible and non-blocking: %+v", result.MissionBrief)
	}
	ledger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), `"authorized-gate"`) || !strings.Contains(string(ledger), `"preauthorized"`) {
		t.Fatalf("ledger does not contain authorized gate decision:\n%s", string(ledger))
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	overviewText := out.String()
	for _, expected := range []string{"authorized gates:", "authorized debug", "authorized-gate（durable autonomy 已授权，非阻塞）", "auth=preauthorized", "profile=prof-main-debug"} {
		if !strings.Contains(overviewText, expected) {
			t.Fatalf("overview missing %q:\n%s", expected, overviewText)
		}
	}
	for _, unexpected := range []string{"pending-gate（heavy-tool 待确认）", "pending-gate requires main-agent/user decision"} {
		if strings.Contains(overviewText, unexpected) {
			t.Fatalf("overview contained unexpected pending blocker %q:\n%s", unexpected, overviewText)
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var overview struct {
		MissionBrief missionBrief `json:"missionBrief"`
		Sections     struct {
			AuthorizedGates struct {
				Total  int              `json:"total"`
				Events []map[string]any `json:"events"`
			} `json:"authorizedGates"`
			PendingGates struct {
				Total int `json:"total"`
			} `json:"pendingGates"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(out.Bytes(), &overview); err != nil {
		t.Fatalf("overview JSON did not decode: %v\n%s", err, out.String())
	}
	if overview.Sections.PendingGates.Total != 0 || overview.Sections.AuthorizedGates.Total != 1 || len(overview.Sections.AuthorizedGates.Events) != 1 || !containsSubstring(overview.MissionBrief.AuthorizedGates, "authorized debug") {
		t.Fatalf("overview JSON missing authorized gate visibility: %+v", overview)
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	project := decodeHandoffResult(t, out.Bytes())
	if !containsSubstring(project.MissionBrief.AuthorizedGates, "authorized debug") || len(project.MissionBrief.PendingGates) != 0 {
		t.Fatalf("project handoff JSON missing authorized gate visibility: %+v", project.MissionBrief)
	}
	projectLatest := assertStartWrite(t, project.Writes, ".rekit/handovers/latest.md", "write-latest-project-handoff")
	projectText, err := os.ReadFile(projectLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"authorized gates:", "authorized debug", "auth=preauthorized"} {
		if !strings.Contains(string(projectText), expected) {
			t.Fatalf("project handoff missing %q:\n%s", expected, string(projectText))
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "main"}, &out); err != nil {
		t.Fatal(err)
	}
	lane := decodeHandoffResult(t, out.Bytes())
	if !containsSubstring(lane.MissionBrief.AuthorizedGates, "authorized debug") || len(lane.MissionBrief.PendingGates) != 0 {
		t.Fatalf("lane handoff JSON missing authorized gate visibility: %+v", lane.MissionBrief)
	}
	laneLatest := assertStartWrite(t, lane.Writes, ".rekit/handovers/main-latest.md", "write-latest-lane-handoff")
	laneText, err := os.ReadFile(laneLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"authorized-gate:", "## authorized-gate", "authorized debug", "auth=preauthorized"} {
		if !strings.Contains(string(laneText), expected) {
			t.Fatalf("lane handoff missing %q:\n%s", expected, string(laneText))
		}
	}

	writeCaseFile(t, caseRoot, ".rekit/lanes/main/outbox.jsonl", `{"eventId":"evt-authorized-continue","kind":"observation","subject":"post auth observation","summary":"continue after authorized gate","evidence":"evidence-authorized-gate"}`+"\n")
	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "-Apply", "main"}, &out); err != nil {
		t.Fatal(err)
	}
	var cont struct {
		RunID        string       `json:"runId"`
		MissionBrief missionBrief `json:"missionBrief"`
		Writes       []startWrite `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &cont); err != nil {
		t.Fatalf("continue apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !containsSubstring(cont.MissionBrief.AuthorizedGates, "authorized debug") || len(cont.MissionBrief.PendingGates) != 0 {
		t.Fatalf("continue JSON missing authorized gate visibility: %+v", cont.MissionBrief)
	}
	statusPath := assertStartWrite(t, cont.Writes, ".rekit/runs/"+cont.RunID+"/status.json", "write").TargetPath
	digestPath := assertStartWrite(t, cont.Writes, ".rekit/runs/"+cont.RunID+"/digest.md", "write").TargetPath
	status, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(status), `"authorizedGates"`) || !strings.Contains(string(status), "authorized debug") || strings.Contains(string(status), "pending-gate requires main-agent/user decision") {
		t.Fatalf("continue status missing non-blocking authorized gate visibility:\n%s", string(status))
	}
	digest, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"- authorized gates:", "authorized debug", "auth=preauthorized"} {
		if !strings.Contains(string(digest), expected) {
			t.Fatalf("continue digest missing %q:\n%s", expected, string(digest))
		}
	}
}

func TestRunGateApplyIsIdempotentByEventID(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	args := []string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Action", "debug", "-Lane", "main", "-Actor", "runtime-test", "-Subject", "debug gate"}
	var first bytes.Buffer
	if err := Run(args, &first); err != nil {
		t.Fatal(err)
	}
	var second bytes.Buffer
	if err := Run(args, &second); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Applied bool   `json:"applied"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(second.Bytes(), &result); err != nil {
		t.Fatalf("second gate apply stdout is not JSON: %v\n%s", err, second.String())
	}
	if result.Applied || result.Reason != "duplicate eventId" {
		t.Fatalf("unexpected duplicate result: %+v", result)
	}
}

func decodePlan(t *testing.T, b []byte) review.Plan {
	t.Helper()
	var plan review.Plan
	if err := json.Unmarshal(b, &plan); err != nil {
		t.Fatalf("review plan stdout is not JSON: %v\n%s", err, string(b))
	}
	if !plan.Summary.ReviewRequired {
		t.Fatalf("reviewRequired = false: %+v", plan.Summary)
	}
	return plan
}

type artifactResult struct {
	Command          string `json:"command"`
	IsMutation       bool   `json:"isMutation"`
	WritesArtifacts  bool   `json:"writesArtifacts"`
	PacketPath       string `json:"packetPath"`
	SummaryPath      string `json:"summaryPath"`
	CombinedDiffPath string `json:"combinedDiffPath"`
}

type syncWrite struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	BackupPath string `json:"backupPath"`
}

type promoteApplyResult struct {
	Command         string              `json:"command"`
	IsMutation      bool                `json:"isMutation"`
	Applied         bool                `json:"applied"`
	BackupRoot      string              `json:"backupRoot"`
	Changed         int                 `json:"changed"`
	Blocked         int                 `json:"blocked"`
	RequiresCleanup bool                `json:"requiresCleanup"`
	ValidationRows  []map[string]any    `json:"validationRows"`
	Writes          []promoteApplyWrite `json:"writes"`
}

type promoteApplyWrite struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
	BackupPath string `json:"backupPath"`
}

type startResult struct {
	Command      string       `json:"command"`
	IsMutation   bool         `json:"isMutation"`
	Applied      bool         `json:"applied"`
	Lane         startLane    `json:"lane"`
	MissionBrief missionBrief `json:"missionBrief"`
	Writes       []startWrite `json:"writes"`
}

type handoffResult struct {
	Command              string       `json:"command"`
	IsMutation           bool         `json:"isMutation"`
	Applied              bool         `json:"applied"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Project              bool         `json:"project"`
	Lane                 *startLane   `json:"lane"`
	MissionBrief         missionBrief `json:"missionBrief"`
	Writes               []startWrite `json:"writes"`
}

type missionBrief struct {
	Summary          string   `json:"summary"`
	ReadyLanes       []string `json:"readyLanes"`
	BlockedLanes     []string `json:"blockedLanes"`
	PendingGates     []string `json:"pendingGates"`
	AuthorizedGates  []string `json:"authorizedGates"`
	OpenDecisions    []string `json:"openDecisions"`
	Interventions    []string `json:"interventions"`
	NextAgentActions []string `json:"nextAgentActions"`
	Escalations      []string `json:"escalations"`
}

type planSubagentsResult struct {
	Command               string                   `json:"command"`
	IsMutation            bool                     `json:"isMutation"`
	WritesReviewArtifacts bool                     `json:"writesReviewArtifacts"`
	ReviewRequired        bool                     `json:"reviewRequired"`
	ReviewRoot            string                   `json:"reviewRoot"`
	PacketPath            string                   `json:"packetPath"`
	SummaryPath           string                   `json:"summaryPath"`
	ItemCount             int                      `json:"itemCount"`
	ShardCount            int                      `json:"shardCount"`
	Observability         planSubagentsObservables `json:"observability"`
	ReviewLoop            planSubagentsReviewLoop  `json:"reviewLoop"`
}

type planSubagentsPacket struct {
	Command string `json:"command"`
	Route   struct {
		ID string `json:"id"`
	} `json:"route"`
	Input struct {
		ItemsFile string `json:"itemsFile"`
	} `json:"input"`
	ShardPolicy struct {
		TargetItemsPerAgent int `json:"targetItemsPerAgent"`
		MaxParallel         int `json:"maxParallel"`
	} `json:"shardPolicy"`
	Shards []struct {
		Items []string `json:"items"`
	} `json:"shards"`
	Observability planSubagentsObservables `json:"observability"`
	ReviewLoop    planSubagentsReviewLoop  `json:"reviewLoop"`
}

type planSubagentsObservables struct {
	DispatchMode string `json:"dispatchMode"`
	RouteDebug   struct {
		SelectedBy string `json:"selectedBy"`
		RouteID    string `json:"routeId"`
	} `json:"routeDebug"`
	ShardStatuses []struct {
		ShardID   string `json:"shardId"`
		Status    string `json:"status"`
		ItemCount int    `json:"itemCount"`
	} `json:"shardStatuses"`
	BlockedActions []string `json:"blockedActions"`
}

type planSubagentsReviewLoop struct {
	SpawnOwner         string   `json:"spawnOwner"`
	MergeOwner         string   `json:"mergeOwner"`
	MainAgentOwns      []string `json:"mainAgentOwns"`
	VerdictWriteback   string   `json:"verdictWriteback"`
	CompletionCriteria []string `json:"completionCriteria"`
}

type startLane struct {
	ID                         string `json:"id"`
	Type                       string `json:"type"`
	Name                       string `json:"name"`
	Workspace                  string `json:"workspace"`
	CurrentExecutor            string `json:"currentExecutor"`
	ExecutorGeneration         int    `json:"executorGeneration"`
	LastReconciledIntervention string `json:"lastReconciledIntervention"`
}

type startWrite struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	TargetPath string `json:"targetPath"`
}

type candidateResult struct {
	Command         string           `json:"command"`
	IsMutation      bool             `json:"isMutation"`
	Applied         bool             `json:"applied"`
	CandidateRoot   string           `json:"candidateRoot"`
	ToolingRoot     string           `json:"toolingRoot"`
	IndexPath       string           `json:"indexPath"`
	Created         int              `json:"created"`
	Blocked         int              `json:"blocked"`
	RequiresCleanup bool             `json:"requiresCleanup"`
	Writes          []candidateWrite `json:"writes"`
}

type candidateWrite struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	SourcePath string `json:"sourcePath"`
	TargetPath string `json:"targetPath"`
}

func decodeArtifactResult(t *testing.T, b []byte) artifactResult {
	t.Helper()
	var result artifactResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("artifact stdout is not JSON: %v\n%s", err, string(b))
	}
	return result
}

func decodeCandidateResult(t *testing.T, b []byte) candidateResult {
	t.Helper()
	var result candidateResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("promote create-candidates stdout is not JSON: %v\n%s", err, string(b))
	}
	return result
}

func decodePromoteApplyResult(t *testing.T, b []byte) promoteApplyResult {
	t.Helper()
	var result promoteApplyResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("promote apply stdout is not JSON: %v\n%s", err, string(b))
	}
	return result
}

func decodeStartResult(t *testing.T, b []byte) startResult {
	t.Helper()
	var result startResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("start stdout is not JSON: %v\n%s", err, string(b))
	}
	return result
}

func decodeHandoffResult(t *testing.T, b []byte) handoffResult {
	t.Helper()
	var result handoffResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("handoff stdout is not JSON: %v\n%s", err, string(b))
	}
	return result
}

func containsSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}

func decodePlanSubagentsResult(t *testing.T, b []byte) planSubagentsResult {
	t.Helper()
	var result planSubagentsResult
	if err := json.Unmarshal(b, &result); err != nil {
		t.Fatalf("plan-subagents stdout is not JSON: %v\n%s", err, string(b))
	}
	return result
}

func decodePlanSubagentsPacket(t *testing.T, path string) planSubagentsPacket {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var packet planSubagentsPacket
	if err := json.Unmarshal(b, &packet); err != nil {
		t.Fatalf("plan-subagents packet is not JSON: %v\n%s", err, string(b))
	}
	return packet
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
	if st.IsDir() {
		t.Fatalf("expected file, got directory: %s", path)
	}
}

func assertCandidateWrite(t *testing.T, writes []candidateWrite, path, action string) candidateWrite {
	t.Helper()
	for _, write := range writes {
		if write.Path != path || write.Action != action {
			continue
		}
		if write.TargetPath == "" {
			t.Fatalf("candidate write %s missing target path", path)
		}
		return write
	}
	t.Fatalf("candidate write %s with action %q not found in %+v", path, action, writes)
	return candidateWrite{}
}

func assertPromoteApplyWrite(t *testing.T, writes []promoteApplyWrite, path, action string) promoteApplyWrite {
	t.Helper()
	for _, write := range writes {
		if write.Path != path || write.Action != action {
			continue
		}
		if write.TargetPath == "" {
			t.Fatalf("promote apply write %s missing target path", path)
		}
		return write
	}
	t.Fatalf("promote apply write %s with action %q not found in %+v", path, action, writes)
	return promoteApplyWrite{}
}

func assertContinueWrite(t *testing.T, writes []startWrite, path, action string) {
	t.Helper()
	for _, write := range writes {
		if write.Path == path && write.Action == action {
			return
		}
	}
	t.Fatalf("continue write %s with action %q not found in %+v", path, action, writes)
}

func assertWriteKind(t *testing.T, writes []startWrite, kind, action string) {
	t.Helper()
	for _, write := range writes {
		if write.Kind == kind && write.Action == action {
			return
		}
	}
	t.Fatalf("write kind %s with action %q not found in %+v", kind, action, writes)
}

func assertStartWrite(t *testing.T, writes []startWrite, path, action string) startWrite {
	t.Helper()
	for _, write := range writes {
		if write.Path != path || write.Action != action {
			continue
		}
		if write.TargetPath == "" {
			t.Fatalf("start write %s missing target path", path)
		}
		return write
	}
	t.Fatalf("start write %s with action %q not found in %+v", path, action, writes)
	return startWrite{}
}

func assertSyncWrite(t *testing.T, writes []syncWrite, path, action string, wantBackup bool) {
	t.Helper()
	for _, write := range writes {
		if write.Path != path {
			continue
		}
		if write.Action != action {
			t.Fatalf("sync write %s action = %q, want %q", path, write.Action, action)
		}
		if wantBackup {
			if write.BackupPath == "" {
				t.Fatalf("sync write %s missing backup path", path)
			}
			assertFileExists(t, write.BackupPath)
		} else if write.BackupPath != "" {
			t.Fatalf("sync write %s backup path = %q, want empty", path, write.BackupPath)
		}
		return
	}
	t.Fatalf("sync write %s not found in %+v", path, writes)
}

func assertSyncPreviewWrite(t *testing.T, writes []syncWrite, path, action string, wantBackup bool) {
	t.Helper()
	for _, write := range writes {
		if write.Path != path {
			continue
		}
		if write.Action != action {
			t.Fatalf("sync preview write %s action = %q, want %q", path, write.Action, action)
		}
		if wantBackup && write.BackupPath == "" {
			t.Fatalf("sync preview write %s missing backup path", path)
		}
		if !wantBackup && write.BackupPath != "" {
			t.Fatalf("sync preview write %s backup path = %q, want empty", path, write.BackupPath)
		}
		if write.BackupPath != "" {
			if _, err := os.Stat(write.BackupPath); !os.IsNotExist(err) {
				t.Fatalf("sync preview write %s created backup path or returned stat error: %s err=%v", path, write.BackupPath, err)
			}
		}
		return
	}
	t.Fatalf("sync preview write %s not found in %+v", path, writes)
}

func writeCaseFile(t *testing.T, caseRoot, rel, text string) {
	t.Helper()
	path := filepath.Join(caseRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

type fileSnapshot struct {
	rootExisted bool
	files       map[string][]byte
	dirs        map[string]bool
}

func snapshotFiles(t *testing.T, root string) fileSnapshot {
	t.Helper()
	out := fileSnapshot{files: map[string][]byte{}, dirs: map[string]bool{}}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return out
	} else if err != nil {
		t.Fatal(err)
	}
	out.rootExisted = true
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if rel != "." {
				out.dirs[rel] = true
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out.files[rel] = append([]byte(nil), content...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func assertSnapshotEqual(t *testing.T, before, after fileSnapshot) {
	t.Helper()
	if before.rootExisted != after.rootExisted {
		t.Fatalf("snapshot root existence changed: before=%v after=%v", before.rootExisted, after.rootExisted)
	}
	if len(before.files) != len(after.files) || len(before.dirs) != len(after.dirs) {
		t.Fatalf("snapshot shape changed: before=%+v after=%+v", before, after)
	}
	for rel, content := range before.files {
		afterContent, ok := after.files[rel]
		if !ok {
			t.Fatalf("snapshot missing file after overview: %s", rel)
		}
		if !bytes.Equal(content, afterContent) {
			t.Fatalf("snapshot file changed after overview: %s", rel)
		}
	}
	for rel := range before.dirs {
		if !after.dirs[rel] {
			t.Fatalf("snapshot missing directory after overview: %s", rel)
		}
	}
}

func removeNewFiles(t *testing.T, root string, before fileSnapshot) {
	t.Helper()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return
	} else if err != nil {
		t.Fatal(err)
	}
	if !before.rootExisted {
		if err := os.RemoveAll(root); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, ok := before.files[rel]; !ok {
			return os.Remove(path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for rel, content := range before.files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dirs := []string{}
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel != "." && !before.dirs[rel] {
			dirs = append(dirs, rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, rel := range dirs {
		path := filepath.Join(root, rel)
		entries, err := os.ReadDir(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == 0 {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
		}
	}
}

func writeOverviewFixture(t *testing.T, caseRoot string) {
	t.Helper()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	lanesRoot := filepath.Join(caseRoot, ".rekit", "lanes")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(lanesRoot, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	board := `{"schemaVersion":1,"caseRoot":"` + filepath.ToSlash(caseRoot) + `","repoRoot":"` + filepath.ToSlash(repoRoot(t)) + `","pack":"_template","automationMode":"assist","defaultAuthorityLane":"main","lanes":[{"id":"main","type":"main","title":"Main","status":"open","authority":true,"workspace":"captures/lanes/main"}],"factsRoot":".rekit/facts"}`
	writeCaseFile(t, caseRoot, ".rekit/board.json", board)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","type":"main","title":"Main","status":"open","authority":true,"workspace":"captures/lanes/main"}`)
	writeFactFile(t, factsRoot, "observations.jsonl", []string{`{"kind":"observation","lane":"main","subject":"obs","summary":"seen"}`})
	writeFactFile(t, factsRoot, "candidates.jsonl", []string{`{"kind":"candidate","lane":"main","subject":"handler","summary":"candidate one","confidence":"high","status":"open"}`, `{"kind":"candidate","lane":"main","subject":"handler","summary":"candidate two","confidence":"medium","status":"open"}`})
	writeFactFile(t, factsRoot, "requests.jsonl", []string{`{"kind":"request","lane":"main","subject":"debug gate","summary":"needs confirmation","status":"pending-gate","actor":"runtime-test","risk":"high","target":"batch-overview","batchId":"batch-overview","gate":{"action":"debug","scope":"handler only","budget":"30s","triedLightSteps":["overview","static review"],"stopConditions":["timeout"]}}`})
	writeFactFile(t, factsRoot, "publications.jsonl", []string{`{"kind":"publication","lane":"main","subject":"pub","summary":"published"}`})
	writeFactFile(t, factsRoot, "decisions.jsonl", []string{`{"kind":"decision","lane":"main","subject":"decision subject","decision":"defer","actor":"runtime-test","reason":"needs review","batchId":"batch-overview"}`})
	writeFactFile(t, factsRoot, "hypotheses.jsonl", nil)
	writeFactFile(t, factsRoot, "verifications.jsonl", []string{`{"kind":"verification","lane":"main","subject":"review target","actor":"reviewer-smoke","target":"candidate-alpha","verifier":"manual-review","verdict":"accepted","batchId":"batch-overview"}`})
	writeFactFile(t, factsRoot, "interventions.jsonl", []string{`{"kind":"intervention","lane":"main","subject":"manual override","summary":"needs human","action":"override","target":"batch-overview","approvedBy":"lead","scope":"metadata","status":"open","batchId":"batch-overview"}`})
	writeFactFile(t, factsRoot, "rollbacks.jsonl", []string{`{"kind":"rollback","lane":"main","subject":"rollback item","target":"batch-overview","status":"resolved","reason":"cleanup","batchId":"batch-overview"}`})
}

func writeHandoffFixture(t *testing.T, caseRoot string) {
	t.Helper()
	factsRoot := filepath.Join(caseRoot, ".rekit", "facts")
	if err := os.MkdirAll(factsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{
		".rekit/lanes/main",
		".rekit/lanes/feature-login",
		"workspace/main/main",
		"workspace/features/feature-login",
	} {
		if err := os.MkdirAll(filepath.Join(caseRoot, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	board := `{"schemaVersion":1,"caseRoot":"` + filepath.ToSlash(caseRoot) + `","repoRoot":"` + filepath.ToSlash(repoRoot(t)) + `","pack":"_template","automationMode":"assist","defaultAuthorityLane":"main","lanes":[{"id":"main","type":"main","title":"主线","status":"open","authority":true,"workspace":"workspace/main/main"},{"id":"feature-login","type":"feature","title":"功能分析: login","status":"open","authority":false,"workspace":"workspace/features/feature-login"}],"factsRoot":".rekit/facts"}`
	writeCaseFile(t, caseRoot, ".rekit/board.json", board)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","type":"main","title":"主线","status":"open","authority":true,"workspace":"workspace/main/main","laneRoot":".rekit/lanes/main"}`)
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/lane.json", `{"schemaVersion":1,"id":"feature-login","type":"feature","name":"login","title":"功能分析: login","status":"open","authority":false,"workspace":"workspace/features/feature-login","laneRoot":".rekit/lanes/feature-login"}`)
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/inbox.jsonl", `{"eventId":"in-1","summary":"review queued"}`+"\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/tasks.jsonl", `{"taskId":"task-1","summary":"inspect candidate","status":"open"}`+"\n")
	writeCaseFile(t, caseRoot, "workspace/features/feature-login/packet.md", "# packet\n")
	writeFactFile(t, factsRoot, "observations.jsonl", nil)
	writeFactFile(t, factsRoot, "candidates.jsonl", nil)
	writeFactFile(t, factsRoot, "publications.jsonl", nil)
	writeFactFile(t, factsRoot, "hypotheses.jsonl", nil)
	writeFactFile(t, factsRoot, "verifications.jsonl", []string{`{"kind":"verification","lane":"feature-login","subject":"review target","actor":"reviewer-smoke","target":"candidate-alpha","verifier":"manual-review","verdict":"accepted","batchId":"batch-handoff"}`})
	writeFactFile(t, factsRoot, "requests.jsonl", []string{`{"kind":"request","lane":"feature-login","subject":"debug gate","summary":"needs confirmation","status":"pending-gate","actor":"runtime-test","risk":"high","target":"batch-handoff","batchId":"batch-handoff","gate":{"action":"debug","scope":"handler only","budget":"30s","triedLightSteps":["overview","static review"],"stopConditions":["timeout"]}}`})
	writeFactFile(t, factsRoot, "decisions.jsonl", []string{`{"kind":"decision","lane":"feature-login","subject":"decision subject","decision":"defer","actor":"runtime-test","reason":"needs review","batchId":"batch-handoff"}`})
	writeFactFile(t, factsRoot, "interventions.jsonl", []string{`{"kind":"intervention","lane":"feature-login","subject":"manual override","summary":"needs human","action":"override","target":"batch-handoff","approvedBy":"lead","scope":"metadata","status":"open","batchId":"batch-handoff"}`})
	writeFactFile(t, factsRoot, "rollbacks.jsonl", []string{`{"kind":"rollback","lane":"feature-login","subject":"rollback item","target":"batch-handoff","status":"resolved","reason":"cleanup","batchId":"batch-handoff"}`})
}

func writeAuthorizedGateVisibilityFixture(t *testing.T, caseRoot string) {
	t.Helper()
	for _, dir := range []string{
		".rekit/facts",
		".rekit/lanes/main",
		"workspace/main/main",
		"workspace/main/debug",
	} {
		if err := os.MkdirAll(filepath.Join(caseRoot, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	board := `{"schemaVersion":1,"caseRoot":"` + filepath.ToSlash(caseRoot) + `","repoRoot":"` + filepath.ToSlash(repoRoot(t)) + `","pack":"_template","automationMode":"assist","defaultAuthorityLane":"main","lanes":[{"id":"main","type":"main","title":"Main","status":"open","authority":true,"workspace":"workspace/main/main"}],"factsRoot":".rekit/facts"}`
	writeCaseFile(t, caseRoot, ".rekit/board.json", board)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","type":"main","title":"Main","status":"open","authority":true,"workspace":"workspace/main/main","laneRoot":".rekit/lanes/main"}`)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/autonomy.json", `{
  "schemaVersion": 1,
  "profileId": "prof-main-debug",
  "lane": "main",
  "mode": "preauthorized",
  "allowedActions": ["debug"],
  "deniedActions": ["symex"],
  "targetScope": [{"match":"exact","value":"target-alpha"}],
  "budget": {"runtimeSeconds": 60, "diskMB": 128, "requests": 2},
  "stopConditions": ["timeout"],
  "outputPaths": ["workspace/main/debug"],
  "recordRequired": true,
  "notifyMainOn": ["boundary-hit", "new-risk"],
  "grantedBy": "user",
  "grantedAt": "2026-01-01T00:00:00Z",
  "expiresAt": "2999-01-01T00:00:00Z"
}`)
}

func writeContinueFixture(t *testing.T, caseRoot string) {
	t.Helper()
	for _, dir := range []string{
		".rekit/facts",
		".rekit/lanes/devirt-main",
		".rekit/lanes/feature-login",
		"captures/devirt_main",
		"workspace/features/feature-login",
	} {
		if err := os.MkdirAll(filepath.Join(caseRoot, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	board := `{"schemaVersion":1,"caseRoot":"` + filepath.ToSlash(caseRoot) + `","repoRoot":"` + filepath.ToSlash(repoRoot(t)) + `","pack":"vmp-re","automationMode":"assist","defaultAuthorityLane":"devirt-main","lanes":[{"id":"devirt-main","type":"devirt-main","title":"VMProtect 脱壳主线","status":"open","authority":true,"workspace":"captures/devirt_main"},{"id":"feature-login","type":"feature-analysis","title":"功能分析: login","status":"open","authority":false,"workspace":"workspace/features/feature-login"}],"factsRoot":".rekit/facts"}`
	writeCaseFile(t, caseRoot, ".rekit/board.json", board)
	writeCaseFile(t, caseRoot, ".rekit/lanes/devirt-main/lane.json", `{"schemaVersion":1,"id":"devirt-main","type":"devirt-main","title":"VMProtect 脱壳主线","status":"open","authority":true,"workspace":"captures/devirt_main","laneRoot":".rekit/lanes/devirt-main"}`)
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/lane.json", `{"schemaVersion":1,"id":"feature-login","type":"feature-analysis","name":"login","title":"功能分析: login","status":"open","authority":false,"workspace":"workspace/features/feature-login","laneRoot":".rekit/lanes/feature-login"}`)
	writeCaseFile(t, caseRoot, "captures/vm_opcode_semantics_confirmed.csv", "opcode,semantics,status\nOP_EXISTING,known,confirmed\n")
	writeCaseFile(t, caseRoot, "workspace/features/feature-login/packet.md", "# packet\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/outbox.jsonl", strings.Join([]string{
		`{"eventId":"evt-continue-observation","kind":"observation","subject":"continue observation","summary":"preview observation","evidence":"evidence-observation-token"}`,
		`{"eventId":"evt-continue-request","kind":"request","subject":"route request","summary":"route to authority lane","requestId":"req-continue","targetLane":"devirt-main","evidence":"evidence-request-token"}`,
		`{"eventId":"evt-continue-authority","kind":"candidate","subject":"authority candidate","summary":"append opcode row","authorityFile":"captures/vm_opcode_semantics_confirmed.csv","confidence":"0.95","evidence":"evidence-authority-token","row":{"opcode":"OP_CONTINUE","semantics":"continue-preview","status":"confirmed"}}`,
	}, "\n")+"\n")
}

func writeFactFile(t *testing.T, root, name string, lines []string) {
	t.Helper()
	text := ""
	if len(lines) > 0 {
		text = strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func attachedCase(t *testing.T) string {
	t.Helper()
	return attachedCaseWithPack(t, "_template")
}

func attachedCaseWithPack(t *testing.T, pack string) string {
	t.Helper()
	caseRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(caseRoot, ".rekit"), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := "templateRoot: " + repoRoot(t) + "\ntemplatePack: " + pack + "\nprojectName: demo\nprojectRoot: " + caseRoot + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func attachedCaseWithBoard(t *testing.T) string {
	t.Helper()
	caseRoot := attachedCase(t)
	board := `{"lanes":[{"id":"main"},{"id":"feature-demo"}]}`
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "board.json"), []byte(board), 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func movedAttachedCase(t *testing.T) string {
	t.Helper()
	caseRoot := attachedCase(t)
	oldRoot := filepath.Join(t.TempDir(), "old-case")
	metadata := "templateRoot: " + repoRoot(t) + "\ntemplatePack: _template\nprojectName: demo\nprojectRoot: " + oldRoot + "\n"
	if err := os.WriteFile(filepath.Join(caseRoot, ".rekit", "instance.yml"), []byte(metadata), 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func fullAttachedCase(t *testing.T) string {
	t.Helper()
	caseRoot := attachedCase(t)
	root := repoRoot(t)
	copyRepoFile(t, root, "rekit/templates/case-shim/SKILL.md", caseRoot, ".claude/skills/rekit/SKILL.md")
	copyRepoFile(t, root, "packs/_template/references/template/README.md", caseRoot, "references/template/README.md")
	copyRepoFile(t, root, "packs/_template/references/template/agent-team.md", caseRoot, "references/template/agent-team.md")
	copyRepoFile(t, root, "packs/_template/references/template/workflow-template.md", caseRoot, "references/template/workflow-template.md")
	copyRepoFile(t, root, "packs/_template/references/template/toolchain-router.md", caseRoot, "references/template/toolchain-router.md")
	copyRepoFile(t, root, "packs/_template/CLAUDE.local.snippet.md", caseRoot, "CLAUDE.local.md")
	template, err := os.ReadFile(filepath.Join(root, "packs", "_template", "references", "template", "task-handoff.template.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(template), "<PROJECT_NAME>", "demo")
	text = strings.ReplaceAll(text, "<PROJECT_ROOT>", caseRoot)
	writeCaseFile(t, caseRoot, "references/template/task-handoff.md", text)
	return caseRoot
}

func copyRepoFile(t *testing.T, repoRoot, sourceRel, caseRoot, targetRel string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(sourceRel)))
	if err != nil {
		t.Fatal(err)
	}
	writeCaseFile(t, caseRoot, targetRel, string(content))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
