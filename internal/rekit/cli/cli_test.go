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
	if opt.Pack != "vmp-re" {
		t.Fatalf("Pack = %q, want vmp-re", opt.Pack)
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
			ManifestPath string `json:"manifestPath"`
			ManagedFiles int    `json:"managedFiles"`
			PromoteFiles int    `json:"promoteFiles"`
			ToolingFiles int    `json:"toolingFiles"`
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
	if !strings.HasSuffix(filepath.ToSlash(status.Manifest.ManifestPath), "packs/_template/manifest.yml") || status.Manifest.ManagedFiles != 4 || status.Manifest.PromoteFiles != 4 || status.Manifest.ToolingFiles != 2 {
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
			ManifestPath string `json:"manifestPath"`
			ManagedFiles int    `json:"managedFiles"`
			PromoteFiles int    `json:"promoteFiles"`
			ToolingFiles int    `json:"toolingFiles"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.SchemaVersion != 1 || status.IsMutation || status.Mode != "kit" || status.Pack != "vmp-re" || status.Case != nil {
		t.Fatalf("unexpected default status JSON envelope: %+v", status)
	}
	if !strings.HasSuffix(filepath.ToSlash(status.Manifest.ManifestPath), "packs/vmp-re/manifest.yml") || status.Manifest.ManagedFiles != 7 || status.Manifest.PromoteFiles != 7 || status.Manifest.ToolingFiles != 12 {
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

type releaseCheckStep struct {
	Command   string `json:"command"`
	Kind      string `json:"kind"`
	RepoPath  string `json:"repoPath"`
	Present   bool   `json:"present"`
	Required  bool   `json:"required"`
	InCatalog bool   `json:"inCatalog"`
	Resolved  bool   `json:"resolved"`
}

type releaseCheckCIReleaseGate struct {
	WorkflowPath   string `json:"workflowPath"`
	Ready          bool   `json:"ready"`
	Summary        string `json:"summary"`
	WorkflowChecks []struct {
		Name     string `json:"name"`
		Expected string `json:"expected"`
		Present  bool   `json:"present"`
	} `json:"workflowChecks"`
	Jobs []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		RunsOn   string `json:"runsOn"`
		Present  bool   `json:"present"`
		Required bool   `json:"required"`
	} `json:"jobs"`
	RequiredCommands []struct {
		Job      string `json:"job"`
		Command  string `json:"command"`
		Present  bool   `json:"present"`
		Required bool   `json:"required"`
	} `json:"requiredCommands"`
	ForbiddenStrings []struct {
		Pattern string `json:"pattern"`
		Present bool   `json:"present"`
	} `json:"forbiddenStrings"`
	Warnings []string `json:"warnings"`
}

type releaseCheckPowerShellDeprecation struct {
	StrategyDocument string `json:"strategyDocument"`
	Ready            bool   `json:"ready"`
	Summary          string `json:"summary"`
	CommandOwnership []struct {
		Area      string `json:"area"`
		Owner     string `json:"owner"`
		Status    string `json:"status"`
		Strategy  string `json:"strategy"`
		GoDefault bool   `json:"goDefault"`
		Blocked   bool   `json:"blocked"`
	} `json:"commandOwnership"`
	ModuleStatus []struct {
		Path   string `json:"path"`
		Status string `json:"status"`
		Notes  string `json:"notes"`
	} `json:"moduleStatus"`
	FreezeGates []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"freezeGates"`
	BlockedMigrations []string `json:"blockedMigrations"`
	Warnings          []string `json:"warnings"`
}

type releaseCheckResult struct {
	Command       string `json:"command"`
	SchemaVersion int    `json:"schemaVersion"`
	IsMutation    bool   `json:"isMutation"`
	Ready         bool   `json:"ready"`
	Summary       string `json:"summary"`
	GateProfile   struct {
		Name               string             `json:"name"`
		Ready              bool               `json:"ready"`
		StepCount          int                `json:"stepCount"`
		LargeMatrixDefault bool               `json:"largeMatrixDefault"`
		Steps              []releaseCheckStep `json:"steps"`
	} `json:"gateProfile"`
	CIReleaseGate      releaseCheckCIReleaseGate `json:"ciReleaseGate"`
	RecommendedMinimum []releaseCheckStep        `json:"recommendedMinimum"`
	RequiredCommands   []releaseCheckStep        `json:"requiredCommands"`
	Documents          []struct {
		Path    string `json:"path"`
		Present bool   `json:"present"`
		Purpose string `json:"purpose"`
	} `json:"documents"`
	Packs []struct {
		ID             string `json:"id"`
		Maturity       string `json:"maturity"`
		SchemaValid    bool   `json:"schemaValid"`
		HeavyToolGates int    `json:"heavyToolGates"`
	} `json:"packs"`
	PowerShellDeprecation releaseCheckPowerShellDeprecation `json:"powerShellDeprecation"`
	HeavyToolGateActions  []string                          `json:"heavyToolGateActions"`
	Boundaries            []string                          `json:"boundaries"`
	KnownGaps             []string                          `json:"knownGaps"`
	Warnings              []string                          `json:"warnings"`
}

func TestRunReleaseCheckJsonInventory(t *testing.T) {
	var out bytes.Buffer
	if err := Run([]string{"-Command", "release-check", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result releaseCheckResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("release-check JSON did not decode: %v\n%s", err, out.String())
	}
	if result.Command != "release-check" || result.SchemaVersion != 1 || result.IsMutation || !result.Ready || result.Summary != "release gate inventory ok" || len(result.Warnings) != 0 {
		t.Fatalf("unexpected release-check JSON envelope: %+v", result)
	}
	assertReleaseCheckCommand(t, result.RequiredCommands, "go run ./cmd/rekit -- -Command release-check -Format json")
	assertReleaseCheckCommand(t, result.RequiredCommands, "go test ./...")
	assertReleaseCheckCommand(t, result.RequiredCommands, "go vet ./...")
	assertReleaseCheckCommand(t, result.RequiredCommands, "rekit/rekit.ps1 -Command doctor")
	assertReleaseCheckCommand(t, result.RequiredCommands, "facade-smoke.ps1")
	assertReleaseCheckCommand(t, result.RequiredCommands, "git diff --check")
	assertReleaseCheckDocument(t, result.Documents, "docs/release-readiness.md")
	assertReleaseCheckDocument(t, result.Documents, "docs/autonomous-goal.md")
	assertReleaseCheckDocument(t, result.Documents, "docs/go-first-convergence-plan.md")
	assertReleaseCheckDocument(t, result.Documents, "docs/powershell-deprecation.md")
	if !result.GateProfile.Ready || result.GateProfile.Name != "local-ci-minimum" || result.GateProfile.StepCount != len(result.RecommendedMinimum) || result.GateProfile.LargeMatrixDefault || len(result.GateProfile.Steps) != len(result.RecommendedMinimum) {
		t.Fatalf("unexpected release-check gate profile: %+v", result.GateProfile)
	}
	assertReleaseCheckStep(t, result.RecommendedMinimum, "go run ./cmd/rekit -- -Command release-check -Format json", "go-run", "cmd/rekit")
	assertReleaseCheckStep(t, result.RecommendedMinimum, "facade-smoke.ps1", "powershell-smoke", "rekit/tests/facade-smoke.ps1")
	assertReleaseCheckStep(t, result.RecommendedMinimum, "rekit/rekit.ps1 -Command doctor", "powershell-facade", "rekit/rekit.ps1")
	assertReleaseCheckCIReleaseGate(t, result.CIReleaseGate)
	assertReleaseCheckPowerShellDeprecation(t, result.PowerShellDeprecation)
	if len(result.RecommendedMinimum) == 0 || len(result.Boundaries) == 0 || len(result.KnownGaps) == 0 || len(result.Packs) == 0 || len(result.HeavyToolGateActions) == 0 {
		t.Fatalf("release-check omitted required inventory: %+v", result)
	}
	if strings.Join(result.HeavyToolGateActions, ",") != "debug,dump,full-trace,inject,network,patch,symex" {
		t.Fatalf("unexpected heavy-tool gate actions: %v", result.HeavyToolGateActions)
	}
	packs := map[string]struct {
		ID             string `json:"id"`
		Maturity       string `json:"maturity"`
		SchemaValid    bool   `json:"schemaValid"`
		HeavyToolGates int    `json:"heavyToolGates"`
	}{}
	for _, pack := range result.Packs {
		packs[pack.ID] = pack
	}
	if pack := packs["vmp-re"]; pack.Maturity != "mature" || !pack.SchemaValid || pack.HeavyToolGates != 7 {
		t.Fatalf("unexpected vmp-re release-check row: %+v", pack)
	}
	if pack := packs["web-security"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.HeavyToolGates != 7 {
		t.Fatalf("unexpected web-security release-check row: %+v", pack)
	}
}

func assertReleaseCheckCommand(t *testing.T, steps []releaseCheckStep, want string) {
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

func assertReleaseCheckStep(t *testing.T, steps []releaseCheckStep, wantCommand, wantKind, wantRepoPath string) {
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

func assertReleaseCheckDocument(t *testing.T, docs []struct {
	Path    string `json:"path"`
	Present bool   `json:"present"`
	Purpose string `json:"purpose"`
}, want string) {
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

func assertReleaseCheckCIReleaseGate(t *testing.T, gate releaseCheckCIReleaseGate) {
	t.Helper()
	if !gate.Ready || gate.WorkflowPath != ".github/workflows/release-gate.yml" || gate.Summary != "CI release gate inventory ok" || len(gate.Warnings) != 0 {
		t.Fatalf("unexpected CI release gate inventory: %+v", gate)
	}
	if len(gate.WorkflowChecks) == 0 || len(gate.Jobs) != 2 || len(gate.RequiredCommands) != 6 || len(gate.ForbiddenStrings) == 0 {
		t.Fatalf("CI release gate inventory omitted required sections: %+v", gate)
	}
	assertCIReleaseJob(t, gate, "go-checks", "Go release checks", "ubuntu-latest")
	assertCIReleaseJob(t, gate, "windows-facade", "Windows facade smoke", "windows-latest")
	assertCIReleaseCommand(t, gate, "go-checks", "go run ./cmd/rekit -- -Command release-check -Format json")
	assertCIReleaseCommand(t, gate, "go-checks", "go test ./...")
	assertCIReleaseCommand(t, gate, "go-checks", "go vet ./...")
	assertCIReleaseCommand(t, gate, "windows-facade", "go run ./cmd/rekit -- -Command release-check -Format json")
	assertCIReleaseCommand(t, gate, "windows-facade", ".\\rekit\\rekit.ps1 -Command doctor")
	assertCIReleaseCommand(t, gate, "windows-facade", ".\\rekit\\tests\\facade-smoke.ps1")
	for _, forbidden := range gate.ForbiddenStrings {
		if forbidden.Present {
			t.Fatalf("CI release gate forbidden pattern present: %+v", forbidden)
		}
	}
}

func assertCIReleaseJob(t *testing.T, gate releaseCheckCIReleaseGate, id, name, runsOn string) {
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

func assertCIReleaseCommand(t *testing.T, gate releaseCheckCIReleaseGate, jobID, command string) {
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

func assertReleaseCheckPowerShellDeprecation(t *testing.T, inventory releaseCheckPowerShellDeprecation) {
	t.Helper()
	if !inventory.Ready || inventory.StrategyDocument != "docs/powershell-deprecation.md" || inventory.Summary != "PowerShell deprecation inventory ok" || len(inventory.Warnings) != 0 {
		t.Fatalf("unexpected PowerShell deprecation inventory: %+v", inventory)
	}
	if len(inventory.CommandOwnership) == 0 || len(inventory.ModuleStatus) == 0 || len(inventory.FreezeGates) == 0 || len(inventory.BlockedMigrations) == 0 {
		t.Fatalf("PowerShell deprecation inventory omitted required sections: %+v", inventory)
	}
	assertPowerShellCommandOwner(t, inventory, "release-check", true, false)
	assertPowerShellCommandOwner(t, inventory, "sync / update", true, false)
	assertPowerShellCommandOwner(t, inventory, "plan-subagents", false, false)
	assertPowerShellCommandOwner(t, inventory, "actual heavy-tool", false, true)
	assertPowerShellModuleStatus(t, inventory, "rekit/rekit.ps1")
	assertPowerShellModuleStatus(t, inventory, "rekit/lib/B3.Commands.ps1")
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
		"CI release gate: .github/workflows/release-gate.yml ready=true jobs=2 commands=6 forbidden=10",
		"required commands:",
		"go run ./cmd/rekit -- -Command release-check -Format json kind=go-run path=cmd/rekit",
		"go test ./... kind=go-check",
		"facade-smoke.ps1 kind=powershell-smoke path=rekit/tests/facade-smoke.ps1",
		"documents:",
		"docs/release-readiness.md",
		"docs/autonomous-goal.md",
		"packs:",
		"heavy-tool gate actions: debug,dump,full-trace,inject,network,patch,symex",
		"PowerShell deprecation: PowerShell deprecation inventory ok ready=true",
		"commands=14 modules=14 freezeGates=8 blocked=5",
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
		"pack\tmaturity\tschema\troutes\tmanaged\ttooling\tauthority\tversion\tdescription",
		"_template\ttemplate\tok\t2\t4\t2\tmain\t0.1.0",
		"vmp-re\tmature\tok\t2\t7\t12\tdevirt-main\t0.2.0",
		"web-security\tskeleton\tok\t2\t4\t4\tmain\t0.1.0",
		"malware-analysis\tskeleton\tok\t2\t4\t4\tmain\t0.1.0",
		"vuln-research\tskeleton\tok\t2\t4\t4\tmain\t0.1.0",
		"ctf\tskeleton\tok\t2\t4\t4\tmain\t0.1.0",
		"unpack-pe\tskeleton\tok\t2\t4\t4\tmain\t0.1.0",
		"ollvm\tskeleton\tok\t2\t4\t4\tmain\t0.1.0",
		"android-native\tskeleton\tok\t2\t4\t4\tmain\t0.1.0",
		"generic-binary-re\tskeleton\tok\t2\t4\t4\tmain\t0.1.0",
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
	if pack := byID["vmp-re"]; pack.Maturity != "mature" || !pack.SchemaValid || pack.Error != "" || pack.ManagedFiles != 7 || pack.ToolingFiles != 12 || pack.SubagentRoutes != 2 || pack.HeavyToolGates != 7 || strings.Join(pack.HeavyToolGateActions, ",") != "debug,dump,full-trace,inject,network,patch,symex" || pack.DefaultAuthorityLane != "devirt-main" {
		t.Fatalf("unexpected vmp-re JSON row: %+v", pack)
	}
	if pack := byID["web-security"]; pack.Maturity != "skeleton" || !pack.SchemaValid || pack.HeavyToolGates != 7 || pack.DefaultAuthorityLane != "main" {
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
	for _, expected := range []string{"项目概览：", "工作线：", "共享事实：", "未决 candidate：", "pending-gate", "by=runtime-test", "action=debug", "最近 verification：", "verifier=manual-review", "verdict=accepted", "target=candidate-alpha", "by=reviewer-smoke", "最近 decision：", "batch-overview", "未解决 intervention：", "最近 rollback：", "/rekit continue main"} {
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
	if result.Sections.OpenCandidates.Total != 2 || result.Sections.OpenCandidates.Shown != 2 || result.Sections.PendingGates.Total != 1 || result.Sections.Verifications.Total != 1 || result.Sections.Batches.Total != 1 {
		t.Fatalf("unexpected overview sections: %+v", result.Sections)
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
		Writes []struct {
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
	latest := assertStartWrite(t, project.Writes, ".rekit/handovers/latest.md", "write-latest-project-handoff")
	text, err := os.ReadFile(latest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 项目接手索引", "## 工作线", "/rekit continue main", "/rekit handoff login"} {
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
	laneLatest := assertStartWrite(t, lane.Writes, ".rekit/handovers/feature-login-latest.md", "write-latest-lane-handoff")
	laneText, err := os.ReadFile(laneLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 工作线接手：feature-login", "workspace/features/feature-login/packet.md", "## verification", "verifier=manual-review", "verdict=accepted", "target=candidate-alpha", "by=reviewer-smoke", "## decision", "by=runtime-test", "## pending-gate", "action=debug", "## intervention", "## rollback", "## 边界"} {
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
		Inputs     []string `json:"inputs"`
		PacketRefs []string `json:"packetRefs"`
		Events     []struct {
			Kind          string       `json:"kind"`
			Decision      string       `json:"decision"`
			TargetLane    string       `json:"targetLane"`
			AuthorityFile string       `json:"authorityFile"`
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
		Command              string                                                                                                                                                              `json:"command"`
		IsMutation           bool                                                                                                                                                                `json:"isMutation"`
		Applied              bool                                                                                                                                                                `json:"applied"`
		RequiresConfirmation bool                                                                                                                                                                `json:"requiresConfirmation"`
		RunID                string                                                                                                                                                              `json:"runId"`
		BatchID              string                                                                                                                                                              `json:"batchId"`
		Lane                 startLane                                                                                                                                                           `json:"lane"`
		Summary              struct{ Collected, Observations, Requests, Routed, Candidates, AcceptedCandidates, Publications, AuthorityApplied, AuthorityWouldAppend, PendingUser, Skipped int } `json:"summary"`
		OpenRisks            []string                                                                                                                                                            `json:"openRisks"`
		Writes               []startWrite                                                                                                                                                        `json:"writes"`
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
	assertContinueWrite(t, result.Writes, ".rekit/facts/observations.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/facts/requests.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/facts/candidates.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/facts/decisions.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/lanes/devirt-main/tasks.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/lanes/devirt-main/inbox.jsonl", "append")
	assertContinueWrite(t, result.Writes, ".rekit/board.json", "refresh")
	var digestPath string
	for _, write := range result.Writes {
		if write.Kind == "run-digest" && write.Action == "write" {
			digestPath = write.TargetPath
		}
	}
	if digestPath == "" {
		t.Fatalf("continue apply did not report run digest write: %+v", result.Writes)
	}
	digest, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(digest), "# rekit continue digest") || !strings.Contains(string(digest), "pendingUser: 1") {
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
		"-StopConditions", "first crash,timeout",
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
	ledger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), result.EventID) || !strings.Contains(string(ledger), `"pending-gate"`) {
		t.Fatalf("ledger does not contain gate event: %s", string(ledger))
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
	Command    string       `json:"command"`
	IsMutation bool         `json:"isMutation"`
	Applied    bool         `json:"applied"`
	Lane       startLane    `json:"lane"`
	Writes     []startWrite `json:"writes"`
}

type handoffResult struct {
	Command              string       `json:"command"`
	IsMutation           bool         `json:"isMutation"`
	Applied              bool         `json:"applied"`
	RequiresConfirmation bool         `json:"requiresConfirmation"`
	Project              bool         `json:"project"`
	Lane                 *startLane   `json:"lane"`
	Writes               []startWrite `json:"writes"`
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
	ID        string `json:"id"`
	Type      string `json:"type"`
	Name      string `json:"name"`
	Workspace string `json:"workspace"`
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
