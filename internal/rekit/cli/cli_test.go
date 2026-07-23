package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	"github.com/shuiyu486/re-context-kits/internal/rekit/promote"
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
	if opt.Pack != "_template" || !opt.PackProvided {
		t.Fatalf("Pack = %q PackProvided=%t, want explicit _template", opt.Pack, opt.PackProvided)
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
	if opt.Command != "doctor" || opt.Pack != "_template" || !opt.PackProvided {
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

func TestParseGateExecutionReportPath(t *testing.T) {
	opt, err := Parse([]string{"-Command", "gate", "-ExecutionReportPath", "workspace/main/debug/session-1/adapter-report.json"})
	if err != nil {
		t.Fatal(err)
	}
	if opt.Gate.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" {
		t.Fatalf("ExecutionReportPath = %q", opt.Gate.ExecutionReportPath)
	}
}

func TestParseGateExecutionReportContract(t *testing.T) {
	opt, err := Parse([]string{"-Command", "gate", "-ExecutionReportContract", "-GateEventId", "evt-authorized"})
	if err != nil {
		t.Fatal(err)
	}
	if !opt.Gate.ExecutionReportContract || opt.Gate.GateEventID != "evt-authorized" {
		t.Fatalf("unexpected gate contract options: %+v", opt.Gate)
	}
}

func TestParseGateValidateExecutionReport(t *testing.T) {
	opt, err := Parse([]string{"-Command", "gate", "-ValidateExecutionReport", "-GateEventId", "evt-authorized", "-ExecutionReportPath", "workspace/main/debug/session-1/adapter-report.json"})
	if err != nil {
		t.Fatal(err)
	}
	if !opt.Gate.ValidateExecutionReport || opt.Gate.GateEventID != "evt-authorized" || opt.Gate.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" {
		t.Fatalf("unexpected gate validation options: %+v", opt.Gate)
	}
}

func TestParsePlanSubagentsReviewerPacketRetirement(t *testing.T) {
	opt, err := Parse([]string{"-Command", "plan-subagents", "-RetireInvalidReviewerPacket", "-PacketPath", "packet.json", "-Lane", "feature-review", "-Actor", "mission-commander", "-Reason", "retire invalid packet", "-WhatIf"})
	if err != nil {
		t.Fatal(err)
	}
	if !opt.RetireInvalidReviewerPacket || opt.PacketPath != "packet.json" || opt.Note.Lane != "feature-review" || opt.Note.Actor != "mission-commander" || opt.Note.Reason != "retire invalid packet" || !opt.WhatIf {
		t.Fatalf("unexpected reviewer packet retirement options: %+v", opt)
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

	out.Reset()
	if err := Run([]string{"-Command", "doctor", "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"doctor：mutation=false valid=true mode=pack pack=_template",
		"rows=",
		"summary=pack validation ok",
		"doctor row：file=",
		"bytes=",
		"limit=",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("doctor text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("doctor text should not emit JSON:\n%s", out.String())
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

	out.Reset()
	if err := Run([]string{"-Command", "validate", "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"validate：mutation=false valid=true mode=pack pack=_template",
		"summary=pack validation ok",
		"validate row：file=",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("validate text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("validate text should not emit JSON:\n%s", out.String())
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

	out.Reset()
	if err := Run([]string{"-Command", "doctor", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"doctor：mutation=false valid=true mode=case pack=_template target=",
		"summary=instance validation ok",
		"doctor row：file=",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("case doctor text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("case doctor text should not emit JSON:\n%s", out.String())
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
		PackSource     string `json:"packSource"`
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
		ProjectHandoff struct {
			Ready                         bool     `json:"ready"`
			Summary                       string   `json:"summary"`
			ReadFirst                     []string `json:"readFirst"`
			LatestBatch                   string   `json:"latestBatch"`
			LatestBatchStatus             string   `json:"latestBatchStatus"`
			LatestBatchGoal               string   `json:"latestBatchGoal"`
			LatestValidation              string   `json:"latestValidation"`
			LatestLocalValidationReady    bool     `json:"latestLocalValidationReady"`
			LatestReleaseCheckReady       bool     `json:"latestReleaseCheckReady"`
			LatestRemoteReleaseGate       string   `json:"latestRemoteReleaseGate"`
			LatestRemoteReleaseGateDetail *struct {
				State            string   `json:"state"`
				RunRefs          []string `json:"runRefs"`
				Jobs             []string `json:"jobs"`
				EmptySteps       bool     `json:"emptySteps"`
				CompletedFailure bool     `json:"completedFailure"`
				CanClaimGreen    bool     `json:"canClaimGreen"`
				Boundary         []string `json:"boundary"`
			} `json:"latestRemoteReleaseGateDetail"`
			ReleaseInspectionCadence struct {
				MaxPushes                 int      `json:"maxPushes"`
				ImplementationCommitReady bool     `json:"implementationCommitReady"`
				InspectionCommitReady     bool     `json:"inspectionCommitReady"`
				ThirdInspectionAllowed    bool     `json:"thirdInspectionAllowed"`
				NewRemoteSignal           bool     `json:"newRemoteSignal"`
				State                     string   `json:"state"`
				NextAction                string   `json:"nextAction"`
				Evidence                  []string `json:"evidence"`
				Boundary                  []string `json:"boundary"`
			} `json:"releaseInspectionCadence"`
			LatestNextAction     string   `json:"latestNextAction"`
			LatestEvidence       []string `json:"latestEvidence"`
			PackMemoryCandidates struct {
				Ready bool `json:"ready"`
				Total int  `json:"total"`
				Packs []struct {
					Pack            string `json:"pack"`
					CandidateRoot   string `json:"candidateRoot"`
					ToolingRoot     string `json:"toolingRoot"`
					IndexPath       string `json:"indexPath"`
					CandidateFiles  int    `json:"candidateFiles"`
					ToolingFiles    int    `json:"toolingFiles"`
					IndexEntries    int    `json:"indexEntries"`
					RequiresReview  bool   `json:"requiresReview"`
					RequiresCleanup bool   `json:"requiresCleanup"`
				} `json:"packs"`
				NextAction string `json:"nextAction"`
			} `json:"packMemoryCandidates"`
			KnownGaps          []string `json:"knownGaps"`
			NextActions        []string `json:"nextActions"`
			ValidationCommands []string `json:"validationCommands"`
		} `json:"projectHandoff"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.SchemaVersion != 1 || status.IsMutation || status.Mode != "kit" || status.Pack != "_template" || status.PackSource != "explicit" || status.TargetProvided {
		t.Fatalf("unexpected status JSON envelope: %+v", status)
	}
	if status.RuntimeRoot == "" || status.TemplateRoot == "" || status.Target == "" || status.Case != nil {
		t.Fatalf("unexpected status JSON roots/case: %+v", status)
	}
	if !strings.HasSuffix(filepath.ToSlash(status.Manifest.ManifestPath), "packs/_template/manifest.yml") || status.Manifest.SchemaVersion != "1" || status.Manifest.ManagedFiles != 4 || status.Manifest.PromoteFiles != 4 || status.Manifest.ToolingFiles != 2 {
		t.Fatalf("unexpected manifest summary: %+v", status.Manifest)
	}
	if !status.ProjectHandoff.Ready || status.ProjectHandoff.Summary != "release handoff summary ok" || !strings.HasPrefix(status.ProjectHandoff.LatestBatch, "Batch ") || !strings.Contains(status.ProjectHandoff.LatestBatchStatus, "已完成") || status.ProjectHandoff.LatestBatchGoal == "" || status.ProjectHandoff.LatestValidation == "" {
		t.Fatalf("unexpected project handoff summary: %+v", status.ProjectHandoff)
	}
	if strings.TrimSpace(status.ProjectHandoff.LatestRemoteReleaseGate) == "" || status.ProjectHandoff.LatestRemoteReleaseGateDetail == nil || strings.TrimSpace(status.ProjectHandoff.LatestRemoteReleaseGateDetail.State) == "" || strings.TrimSpace(status.ProjectHandoff.LatestNextAction) == "" {
		t.Fatalf("project handoff omitted latest batch validation handoff: %+v", status.ProjectHandoff)
	}
	if status.ProjectHandoff.LatestRemoteReleaseGateDetail.State != status.ProjectHandoff.LatestRemoteReleaseGate {
		t.Fatalf("project handoff remote gate detail state drifted: %+v", status.ProjectHandoff.LatestRemoteReleaseGateDetail)
	}
	if len(status.ProjectHandoff.LatestRemoteReleaseGateDetail.Boundary) == 0 {
		t.Fatalf("project handoff remote gate detail omitted boundary: %+v", status.ProjectHandoff.LatestRemoteReleaseGateDetail)
	}
	if cadence := status.ProjectHandoff.ReleaseInspectionCadence; cadence.MaxPushes != 2 || cadence.State == "" || cadence.NextAction == "" || cadence.ThirdInspectionAllowed != cadence.NewRemoteSignal || len(cadence.Boundary) == 0 || !containsSubstring(cadence.Boundary, "do not add a third record commit") {
		t.Fatalf("project handoff release inspection cadence drifted: %+v", cadence)
	}
	if status.ProjectHandoff.LatestLocalValidationReady {
		for _, want := range []string{"release-check -Format json recorded", "status handoff recorded", "go test ./... recorded", "git diff --check recorded"} {
			if !slices.Contains(status.ProjectHandoff.LatestEvidence, want) {
				t.Fatalf("project handoff latest evidence missing %q: %+v", want, status.ProjectHandoff.LatestEvidence)
			}
		}
	}
	for _, want := range []string{"docs/context-routing.md", "docs/batch-plan.md", "docs/release-readiness.md", "CHANGELOG.md"} {
		if !slices.Contains(status.ProjectHandoff.ReadFirst, want) {
			t.Fatalf("project handoff readFirst missing %q: %+v", want, status.ProjectHandoff.ReadFirst)
		}
	}
	for _, want := range []string{"go run ./cmd/rekit -- -Command release-check -Format json", "go test ./...", "go vet ./...", "git diff --check"} {
		if !slices.Contains(status.ProjectHandoff.ValidationCommands, want) {
			t.Fatalf("project handoff validation commands missing %q: %+v", want, status.ProjectHandoff.ValidationCommands)
		}
	}
	if !status.ProjectHandoff.PackMemoryCandidates.Ready || status.ProjectHandoff.PackMemoryCandidates.Total != 0 || len(status.ProjectHandoff.PackMemoryCandidates.Packs) != 0 || status.ProjectHandoff.PackMemoryCandidates.NextAction != "no pack-memory candidate cleanup is pending" {
		t.Fatalf("project handoff pack-memory candidate inventory drifted: %+v", status.ProjectHandoff.PackMemoryCandidates)
	}
	if len(status.ProjectHandoff.KnownGaps) == 0 || len(status.ProjectHandoff.NextActions) == 0 {
		t.Fatalf("project handoff should expose known gaps and next actions: %+v", status.ProjectHandoff)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status：mutation=false mode=kit targetProvided=false pack=_template packSource=explicit",
		"status manifest：path=",
		"schema=1 managed=4 promote=4 tooling=2",
		"status case shim：summary=case shim readiness ok ready=true",
		"matchesTemplate=unknown",
		"warnings=0",
		"status project handoff：summary=release handoff summary ok ready=true latestBatch=Batch ",
		"latestStatus=",
		"localValidationReady=",
		"status latest batch remote gate：state=",
		"status latest batch remote gate boundary：",
		"status latest batch release inspection cadence：state=",
		"maxPushes=2",
		"thirdInspectionAllowed=",
		"status latest batch release inspection cadence boundary：do not add a third record commit",
		"status latest batch next action：",
		"status latest batch release inspection cadence boundary：normal batches stop after implementation commit/push plus one release inspection commit/push",
		"status pack-memory candidates：summary=pack-memory candidate inventory ok ready=true total=0 packs=0 nextAction=no pack-memory candidate cleanup is pending",
		"status latest batch goal：",
		"status latest batch validation：",
		"status read first：docs/context-routing.md",
		"status known gap：远程 release-gate",
		"status validation command：go test ./...",
		"status next action：Read docs/context-routing.md first",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status kit text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("status kit text should not emit JSON:\n%s", out.String())
	}

	defaultStatusExpected := []string{
		"rekit go backend:",
		"status project handoff：summary=release handoff summary ok ready=true latestBatch=Batch ",
		"localValidationReady=",
		"status latest batch remote gate：state=",
		"status latest batch remote gate boundary：",
		"status latest batch release inspection cadence：state=",
		"maxPushes=2",
		"thirdInspectionAllowed=",
		"status latest batch release inspection cadence boundary：do not add a third record commit",
		"status latest batch next action：",
		"status latest batch release inspection cadence boundary：normal batches stop after implementation commit/push plus one release inspection commit/push",
		"status pack-memory candidates：summary=pack-memory candidate inventory ok ready=true total=0 packs=0 nextAction=no pack-memory candidate cleanup is pending",
		"status read first：docs/context-routing.md",
		"status validation command：go run ./cmd/rekit -- -Command release-check -Format json",
		"status next action：Read docs/context-routing.md first",
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range defaultStatusExpected {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status default text missing %q:\n%s", expected, out.String())
		}
	}
	if !strings.Contains(out.String(), "pack source: explicit") {
		t.Fatalf("status default text missing explicit pack source:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range defaultStatusExpected {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status no-command default text missing %q:\n%s", expected, out.String())
		}
	}
	if !strings.Contains(out.String(), "pack source: explicit") {
		t.Fatalf("status no-command default text missing explicit pack source:\n%s", out.String())
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
		PackSource    string `json:"packSource"`
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
	if status.Command != "status" || status.SchemaVersion != 1 || status.IsMutation || status.Mode != "kit" || status.Pack != defaults.DefaultPack || status.PackSource != "repo-default" || status.Case != nil {
		t.Fatalf("unexpected default status JSON envelope: %+v", status)
	}
	if !strings.HasSuffix(filepath.ToSlash(status.Manifest.ManifestPath), "packs/"+defaults.DefaultPack+"/manifest.yml") || status.Manifest.SchemaVersion != "1" || status.Manifest.ManagedFiles != 7 || status.Manifest.PromoteFiles != 7 || status.Manifest.ToolingFiles != 12 {
		t.Fatalf("unexpected default manifest summary: %+v", status.Manifest)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "status：mutation=false mode=kit targetProvided=false pack="+defaults.DefaultPack+" packSource=repo-default") {
		t.Fatalf("default status text missing repo-default pack source:\n%s", out.String())
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("default status text should not emit JSON:\n%s", out.String())
	}
}

func TestRunStatusJsonCase(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply", "-Executor", "session-1", "-Actor", "mission-commander", "-Reason", "status case mission coverage"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Command        string `json:"command"`
		SchemaVersion  int    `json:"schemaVersion"`
		IsMutation     bool   `json:"isMutation"`
		Pack           string `json:"pack"`
		PackSource     string `json:"packSource"`
		TargetProvided bool   `json:"targetProvided"`
		Mode           string `json:"mode"`
		Case           struct {
			CaseRoot            string   `json:"caseRoot"`
			MetadataSource      string   `json:"metadataSource"`
			TemplatePack        string   `json:"templatePack"`
			PackMatchesMetadata bool     `json:"packMatchesMetadata"`
			PackDiagnostic      string   `json:"packDiagnostic"`
			NextSteps           []string `json:"nextSteps"`
			ProjectName         string   `json:"projectName"`
			Moved               bool     `json:"moved"`
			ShimPath            string   `json:"shimPath"`
			ShimMatchesTemplate bool     `json:"shimMatchesTemplate"`
		} `json:"case"`
		Manifest any `json:"manifest"`
		CaseShim struct {
			Ready                        bool     `json:"ready"`
			Summary                      string   `json:"summary"`
			InstalledShimPath            string   `json:"installedShimPath"`
			InstalledShimMatchesTemplate *bool    `json:"installedShimMatchesTemplate"`
			RequiredPhrases              int      `json:"requiredPhrases"`
			CanonicalSkillPhrases        int      `json:"canonicalSkillPhrases"`
			ForbiddenStrings             int      `json:"forbiddenStrings"`
			Boundaries                   int      `json:"boundaries"`
			Warnings                     []string `json:"warnings"`
			NextSteps                    []string `json:"nextSteps"`
		} `json:"caseShim"`
		CaseMission struct {
			Ready               bool                        `json:"ready"`
			Summary             string                      `json:"summary"`
			LaneCount           int                         `json:"laneCount"`
			ReadyLaneCount      int                         `json:"readyLaneCount"`
			BlockedLaneCount    int                         `json:"blockedLaneCount"`
			ReadyLanes          []string                    `json:"readyLanes"`
			BlockedLanes        []string                    `json:"blockedLanes"`
			LaneExecutorActions []handoffLaneExecutorAction `json:"laneExecutorActions"`
			PendingGates        []string                    `json:"pendingGates"`
			AuthorizedGates     []string                    `json:"authorizedGates"`
			OpenDecisions       []string                    `json:"openDecisions"`
			Interventions       []string                    `json:"interventions"`
			FactCounts          *struct {
				Observations     int `json:"observations"`
				Requests         int `json:"requests"`
				Candidates       int `json:"candidates"`
				Publications     int `json:"publications"`
				PendingDecisions int `json:"pendingDecisions"`
			} `json:"factCounts"`
			Sections *struct {
				OpenCandidates struct {
					Total int `json:"total"`
				} `json:"openCandidates"`
				PendingGates struct {
					Total int `json:"total"`
				} `json:"pendingGates"`
			} `json:"sections"`
			ExecutionEvidenceReviewCount  int                           `json:"executionEvidenceReviewCount"`
			ExecutionEvidenceReview       []executionEvidenceReviewItem `json:"executionEvidenceReview"`
			HandoffPreviewCommand         string                        `json:"handoffPreviewCommand"`
			HandoffApplyCommand           string                        `json:"handoffApplyCommand"`
			ContinueRequiresExplicitApply string                        `json:"continueRequiresExplicitApply"`
			MissionBriefNextActions       []string                      `json:"missionBriefNextActions"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("case status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.SchemaVersion != 1 || status.IsMutation || status.Pack != "_template" || status.PackSource != "explicit" || !status.TargetProvided || status.Mode != "case" || status.Manifest != nil {
		t.Fatalf("unexpected case status JSON envelope: %+v", status)
	}
	if status.Case.CaseRoot != caseRoot || status.Case.MetadataSource != "instance" || status.Case.TemplatePack != "_template" || !status.Case.PackMatchesMetadata || !strings.Contains(status.Case.PackDiagnostic, "pack matches case metadata templatePack") || len(status.Case.NextSteps) != 0 || status.Case.ProjectName != "demo" || status.Case.Moved || status.Case.ShimPath == "" || !status.Case.ShimMatchesTemplate {
		t.Fatalf("unexpected case status JSON: %+v", status.Case)
	}
	if !status.CaseShim.Ready || status.CaseShim.Summary != "case shim readiness ok" || status.CaseShim.InstalledShimPath != status.Case.ShimPath || status.CaseShim.InstalledShimMatchesTemplate == nil || !*status.CaseShim.InstalledShimMatchesTemplate || status.CaseShim.RequiredPhrases == 0 || status.CaseShim.CanonicalSkillPhrases == 0 || status.CaseShim.ForbiddenStrings == 0 || status.CaseShim.Boundaries == 0 || len(status.CaseShim.Warnings) != 0 || len(status.CaseShim.NextSteps) != 0 {
		t.Fatalf("unexpected case shim status JSON: %+v", status.CaseShim)
	}
	if !status.CaseMission.Ready || !strings.Contains(status.CaseMission.Summary, "openLanes=") || status.CaseMission.LaneCount == 0 || status.CaseMission.ReadyLaneCount == 0 || !containsSubstring(status.CaseMission.ReadyLanes, "login") || len(status.CaseMission.BlockedLanes) != 0 || len(status.CaseMission.LaneExecutorActions) == 0 || len(status.CaseMission.PendingGates) != 0 || len(status.CaseMission.OpenDecisions) != 0 || len(status.CaseMission.Interventions) != 0 || status.CaseMission.FactCounts == nil || status.CaseMission.Sections == nil || status.CaseMission.HandoffPreviewCommand == "" || status.CaseMission.HandoffApplyCommand == "" || status.CaseMission.ContinueRequiresExplicitApply == "" || len(status.CaseMission.MissionBriefNextActions) == 0 {
		t.Fatalf("unexpected case mission status JSON: %+v", status.CaseMission)
	}
	foundLoginLane := false
	for _, lane := range status.CaseMission.LaneExecutorActions {
		if lane.Lane == "feature-login" && lane.Label == "login" && lane.ExecutorAction.Ready && !lane.ExecutorAction.Blocked && lane.ExecutorAction.MissionCommanderAction.PrimaryCommand != "" {
			foundLoginLane = true
			break
		}
	}
	if !foundLoginLane {
		t.Fatalf("case mission should expose ready feature lane executor action: %+v", status.CaseMission.LaneExecutorActions)
	}
	if status.CaseMission.FactCounts.Observations != 0 || status.CaseMission.FactCounts.Requests != 0 || status.CaseMission.FactCounts.Candidates != 0 || status.CaseMission.FactCounts.Publications != 0 || status.CaseMission.FactCounts.PendingDecisions != 0 || status.CaseMission.Sections.OpenCandidates.Total != 0 || status.CaseMission.Sections.PendingGates.Total != 0 {
		t.Fatalf("ready case mission should expose empty ledger/progress counts: %+v", status.CaseMission)
	}
	if status.CaseMission.ExecutionEvidenceReviewCount != 0 || len(status.CaseMission.ExecutionEvidenceReview) != 0 {
		t.Fatalf("ready case mission should not include evidence review items: %+v", status.CaseMission.ExecutionEvidenceReview)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status：mutation=false mode=case targetProvided=true pack=_template packSource=explicit",
		"status case：root=",
		"metadataSource=instance",
		"templatePack=_template packMatchesMetadata=true projectName=demo",
		"status case pack diagnostic：pack matches case metadata templatePack",
		"moved=false",
		"status case shim：summary=case shim readiness ok ready=true",
		"matchesTemplate=true",
		"warnings=0",
		"status case mission：summary=openLanes=",
		"ready=true lanes=",
		"status case mission queue：total=4 unblocked=4 blocked=0 requiresReview=0 followUp=2 current=/rekit continue login",
		"status case mission queue action：bucket=current lane=feature-login label=login state=ready-to-continue source=missionCommanderActions blocked=false requiresReview=false command=/rekit continue login",
		"status case mission queue action：bucket=followUp lane=feature-login label=login state=ready-to-continue source=missionCommanderActions.followUp blocked=false requiresReview=false command=/rekit handoff login",
		"status case mission queue action reason：bucket=followUp lane=feature-login reason=follow Mission Commander handoff after primary action",
		"status case mission lane action：lane=feature-login label=login status=open workspace=workspace/features/feature-login executor=session-1 generation=1 ready=true blocked=false pendingGates=0 openInterventions=0 openDecisions=0 resume=/rekit continue login handoff=/rekit handoff login commanderState=ready-to-continue commanderPrimary=/rekit continue login",
		"status case mission lane boundary：lane=feature-login boundary=no authority/confirmed writes",
		"status case mission ready lane：login",
		"status case mission next action：lane=feature-login",
		"command=/rekit handoff login",
		"status case mission brief next action：/rekit continue login",
		"status case mission handoff：preview=/rekit handoff -Target",
		"continueBoundary=status is read-only; run continue with -WhatIf first",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status case text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("status case text should not emit JSON:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"case:",
		"status case mission：summary=openLanes=",
		"status case mission next action：lane=feature-login",
		"status case mission handoff：preview=/rekit handoff -Target",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status case default text missing %q:\n%s", expected, out.String())
		}
	}
}

func TestRunStatusCaseMissionDoesNotInitializeMissingBoard(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		CaseMission struct {
			Ready                   bool     `json:"ready"`
			Summary                 string   `json:"summary"`
			MissionBriefNextActions []string `json:"missionBriefNextActions"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("case status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.CaseMission.Ready || !strings.Contains(status.CaseMission.Summary, "case board missing") || len(status.CaseMission.MissionBriefNextActions) == 0 || !strings.Contains(status.CaseMission.MissionBriefNextActions[0], "overview") {
		t.Fatalf("unexpected missing-board mission summary: %+v", status.CaseMission)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "board.json")); !os.IsNotExist(err) {
		t.Fatalf("status should not initialize missing board, err=%v", err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status case mission：summary=case board missing",
		"ready=false lanes=0",
		"status case mission brief next action：run /rekit overview -Target",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("missing-board status text missing %q:\n%s", expected, out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "board.json")); !os.IsNotExist(err) {
		t.Fatalf("status text should not initialize missing board, err=%v", err)
	}
}

func TestRunStatusCaseMissionIncludesExecutionEvidenceReview(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeOverviewFixture(t, caseRoot)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		CaseMission struct {
			Ready                  bool                          `json:"ready"`
			BlockedLanes           []string                      `json:"blockedLanes"`
			LaneExecutorActions    []handoffLaneExecutorAction   `json:"laneExecutorActions"`
			PendingGates           []string                      `json:"pendingGates"`
			PendingGateHandoffs    []statusPendingGateHandoff    `json:"pendingGateHandoffs"`
			AuthorizedGates        []string                      `json:"authorizedGates"`
			AuthorizedGateHandoffs []statusAuthorizedGateHandoff `json:"authorizedGateHandoffs"`
			OpenDecisions          []string                      `json:"openDecisions"`
			OpenDecisionHandoffs   []statusOpenDecisionHandoff   `json:"openDecisionHandoffs"`
			Interventions          []string                      `json:"interventions"`
			InterventionHandoffs   []statusInterventionHandoff   `json:"interventionHandoffs"`
			FactCounts             *struct {
				Observations     int `json:"observations"`
				Requests         int `json:"requests"`
				Candidates       int `json:"candidates"`
				Publications     int `json:"publications"`
				PendingDecisions int `json:"pendingDecisions"`
			} `json:"factCounts"`
			Sections *struct {
				OpenCandidates struct {
					Total int `json:"total"`
					Shown int `json:"shown"`
				} `json:"openCandidates"`
				PendingGates struct {
					Total int `json:"total"`
					Shown int `json:"shown"`
				} `json:"pendingGates"`
				Batches struct {
					Total int `json:"total"`
					Shown int `json:"shown"`
				} `json:"batches"`
			} `json:"sections"`
			ExecutionEvidenceReviewCount   int                                    `json:"executionEvidenceReviewCount"`
			ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
			ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("case status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.CaseMission.Ready || status.CaseMission.ExecutionEvidenceReviewCount != 1 || len(status.CaseMission.ExecutionEvidenceReview) != 1 || !containsSubstring(status.CaseMission.BlockedLanes, "main (pending-gate,intervention,open-decision)") || len(status.CaseMission.LaneExecutorActions) != 1 || !containsSubstring(status.CaseMission.PendingGates, "action=debug") || len(status.CaseMission.PendingGateHandoffs) != 1 || len(status.CaseMission.AuthorizedGates) != 0 || len(status.CaseMission.AuthorizedGateHandoffs) != 0 || len(status.CaseMission.OpenDecisionHandoffs) != 3 || len(status.CaseMission.InterventionHandoffs) != 1 || !containsSubstring(status.CaseMission.OpenDecisions, "candidate: handler") || !containsSubstring(status.CaseMission.Interventions, "manual override") || status.CaseMission.FactCounts == nil || status.CaseMission.Sections == nil {
		t.Fatalf("unexpected case mission blocker/evidence review summary: %+v", status.CaseMission)
	}
	if summary := status.CaseMission.ExecutionEvidenceReviewSummary; summary.Total != 1 || summary.ReadyForReviewCount != 1 || summary.MainEscalationCount != 0 || summary.DuplicateCount != 0 || summary.OutputRefCount != 1 || summary.EvidenceRefCount != 1 || summary.LatestEventID != "obs-auth-1" || summary.LatestGateEventID != "gate-auth-1" || summary.LatestStatus != "complete" || summary.LatestAction != "debug" || summary.LatestTarget != "target-alpha" || summary.LatestReviewCommand != "review outputRefs/evidenceRefs for gateEventId gate-auth-1" || summary.LatestHandoffCommand != "/rekit handoff main" || summary.LatestCommanderState != "ready-for-evidence-review" || summary.LatestCommanderPrimary != "/rekit handoff main" || summary.OutcomeCount != 1 || summary.FollowThroughState != "ready-for-evidence-review" || summary.CurrentAction != "/rekit handoff main" || summary.NextActionCount != 5 || summary.ReviewRequiredActionCount != 5 || !strings.Contains(summary.ActionQueueSummary, "total=5") || !containsSubstring(summary.Boundary, "summary is read-only") || !containsSubstring(summary.Boundary, "no authority/confirmed") {
		t.Fatalf("case mission JSON missing execution evidence review summary: %+v", summary)
	}
	pendingHandoff := status.CaseMission.PendingGateHandoffs[0]
	if pendingHandoff.EventID != "" || pendingHandoff.Lane != "main" || pendingHandoff.Subject != "debug gate" || pendingHandoff.Action != "debug" || pendingHandoff.Target != "batch-overview" || pendingHandoff.Status != "pending-gate" || pendingHandoff.Risk != "high" || pendingHandoff.ReviewCommand != "/rekit handoff main" || !strings.Contains(pendingHandoff.WhatIfCommand, `/rekit gate -Target "`+caseRoot+`" -Pack _template -Action debug -Lane main -WhatIf`) || !strings.Contains(pendingHandoff.WhatIfCommand, `-Subject "debug gate"`) || !strings.Contains(pendingHandoff.WhatIfCommand, `-Summary "needs confirmation"`) || !strings.Contains(pendingHandoff.WhatIfCommand, `-TargetRef "batch-overview"`) || !strings.Contains(pendingHandoff.WhatIfCommand, `-Scope "handler only"`) || !strings.Contains(pendingHandoff.WhatIfCommand, `-Budget "30s"`) || !strings.Contains(pendingHandoff.WhatIfCommand, `-TriedLightSteps "overview,static review"`) || !strings.Contains(pendingHandoff.WhatIfCommand, `-StopConditions "timeout"`) || !strings.Contains(pendingHandoff.WhatIfCommand, `-Risk "high" -Format json`) || !strings.Contains(pendingHandoff.ApplyCommand, `/rekit gate -Target "`+caseRoot+`" -Pack _template -Action debug -Lane main -Apply -Actor runtime-test`) || !strings.Contains(pendingHandoff.DecisionBoundary, "apply command only replays/records the gate request decision") || !strings.Contains(pendingHandoff.DecisionBoundary, "does not execute or approve heavy action by itself") || !strings.Contains(pendingHandoff.ContinueBoundary, "blocked lane can only continue with -WhatIf") || !containsSubstring(pendingHandoff.Evidence, "requested budget 30s") || !containsSubstring(pendingHandoff.Evidence, "requested stopConditions timeout") || !containsSubstring(pendingHandoff.Evidence, "triedLightSteps overview,static review") {
		t.Fatalf("unexpected pending gate handoff: %+v", pendingHandoff)
	}
	candidateDecisionHandoff := status.CaseMission.OpenDecisionHandoffs[0]
	if candidateDecisionHandoff.EventID != "cand-main-1" || candidateDecisionHandoff.Kind != "candidate" || candidateDecisionHandoff.Lane != "main" || candidateDecisionHandoff.Subject != "handler" || candidateDecisionHandoff.Summary != "candidate one" || candidateDecisionHandoff.Decision != "" || candidateDecisionHandoff.Status != "open" || candidateDecisionHandoff.Target != "candidate-alpha" || candidateDecisionHandoff.Confidence != "high" || candidateDecisionHandoff.SourceKind != "candidate" || candidateDecisionHandoff.SourcePath != ".rekit/facts/candidates.jsonl" || candidateDecisionHandoff.RecordPath != ".rekit/facts/decisions.jsonl" || candidateDecisionHandoff.SourceCommand != `/rekit note -Target "`+caseRoot+`" -Pack _template -List -Kind candidate -Lane main -Format json` || candidateDecisionHandoff.ReviewCommand != "/rekit handoff main" || !strings.Contains(candidateDecisionHandoff.WhatIfCommand, `/rekit note -Target "`+caseRoot+`" -Pack _template -Kind decision -Lane main`) || !strings.Contains(candidateDecisionHandoff.WhatIfCommand, `-Subject "decision for candidate: handler"`) || !strings.Contains(candidateDecisionHandoff.WhatIfCommand, `-Decision "<accept|reject|defer|supersede>"`) || !strings.Contains(candidateDecisionHandoff.WhatIfCommand, `-TargetRef "candidate-alpha"`) || !strings.Contains(candidateDecisionHandoff.WhatIfCommand, `-Related "cand-main-1"`) || !strings.Contains(candidateDecisionHandoff.WhatIfCommand, `-EvidenceRefs "evidence/candidate.json"`) || !strings.Contains(candidateDecisionHandoff.WhatIfCommand, `-WhatIf -Format json`) || strings.Contains(candidateDecisionHandoff.RecordCommand, "-WhatIf") || !strings.Contains(candidateDecisionHandoff.DecisionBoundary, "only appends case-local decision ledger state") || !strings.Contains(candidateDecisionHandoff.DecisionBoundary, "never writes authority/confirmed or executes heavy-tool") || !strings.Contains(candidateDecisionHandoff.ContinueBoundary, "blocked lane can only continue with -WhatIf") || !containsSubstring(candidateDecisionHandoff.Evidence, "candidate ledger event cand-main-1") || !containsSubstring(candidateDecisionHandoff.Evidence, "confidence high") || !containsSubstring(candidateDecisionHandoff.Evidence, "evidenceRefs evidence/candidate.json") || !containsSubstring(candidateDecisionHandoff.Evidence, "sourcePath .rekit/facts/candidates.jsonl") || !containsSubstring(candidateDecisionHandoff.Evidence, "recordPath .rekit/facts/decisions.jsonl") {
		t.Fatalf("unexpected candidate decision handoff: %+v", candidateDecisionHandoff)
	}
	decisionHandoff := status.CaseMission.OpenDecisionHandoffs[2]
	if decisionHandoff.EventID != "dec-main-1" || decisionHandoff.Kind != "decision" || decisionHandoff.Lane != "main" || decisionHandoff.Subject != "decision subject" || decisionHandoff.Decision != "defer" || decisionHandoff.Reason != "needs review" || decisionHandoff.Target != "decision-alpha" || decisionHandoff.SourceKind != "decision" || decisionHandoff.SourcePath != ".rekit/facts/decisions.jsonl" || decisionHandoff.RecordPath != ".rekit/facts/decisions.jsonl" || decisionHandoff.SourceCommand != `/rekit note -Target "`+caseRoot+`" -Pack _template -List -Kind decision -Lane main -Format json` || !strings.Contains(decisionHandoff.WhatIfCommand, `-Subject "decision for decision: decision subject"`) || !strings.Contains(decisionHandoff.WhatIfCommand, `-Decision "<accept|reject|defer|supersede>"`) || !strings.Contains(decisionHandoff.WhatIfCommand, `-Reason "needs review"`) || !strings.Contains(decisionHandoff.WhatIfCommand, `-Related "dec-main-1"`) || !containsSubstring(decisionHandoff.Evidence, "decision ledger event dec-main-1") || !containsSubstring(decisionHandoff.Evidence, "sourcePath .rekit/facts/decisions.jsonl") || !containsSubstring(decisionHandoff.Evidence, "recordPath .rekit/facts/decisions.jsonl") || !containsSubstring(decisionHandoff.Evidence, "target decision-alpha") || !containsSubstring(decisionHandoff.Evidence, "batchId batch-overview") {
		t.Fatalf("unexpected deferred decision handoff: %+v", decisionHandoff)
	}
	interventionHandoff := status.CaseMission.InterventionHandoffs[0]
	if interventionHandoff.EventID != "int-main-1" || interventionHandoff.Lane != "main" || interventionHandoff.Subject != "manual override" || interventionHandoff.Summary != "needs human" || interventionHandoff.Action != "override" || interventionHandoff.Target != "batch-overview" || interventionHandoff.Status != "open" || interventionHandoff.Scope != "metadata" || interventionHandoff.ApprovedBy != "lead" || interventionHandoff.ReviewCommand != "/rekit handoff main" || !strings.Contains(interventionHandoff.WhatIfCommand, `/rekit reconcile main -Target "`+caseRoot+`" -Pack _template -InterventionId int-main-1 -WhatIf -Format json`) || !strings.Contains(interventionHandoff.ApplyCommand, `/rekit reconcile main -Target "`+caseRoot+`" -Pack _template -InterventionId int-main-1 -Apply -Format json`) || !strings.Contains(interventionHandoff.DecisionBoundary, "only writes case-local intervention/lane/resume/checkpoint/board state") || !strings.Contains(interventionHandoff.DecisionBoundary, "never writes authority/confirmed or executes heavy-tool") || !strings.Contains(interventionHandoff.ContinueBoundary, "blocked lane can only continue with -WhatIf") || !containsSubstring(interventionHandoff.Evidence, "open intervention ledger event int-main-1") || !containsSubstring(interventionHandoff.Evidence, "approvedBy lead") || !containsSubstring(interventionHandoff.Evidence, "scope metadata") {
		t.Fatalf("unexpected intervention handoff: %+v", interventionHandoff)
	}
	laneAction := status.CaseMission.LaneExecutorActions[0]
	if laneAction.Lane != "main" || laneAction.Label != "main" || !laneAction.ExecutorAction.Blocked || laneAction.ExecutorAction.Ready || laneAction.ExecutorAction.PendingGates != 1 || laneAction.ExecutorAction.OpenInterventions != 1 || laneAction.ExecutorAction.OpenDecisions != 3 || !containsSubstring(laneAction.ExecutorAction.BlockerReasons, "pending-gate") || laneAction.ExecutorAction.MissionCommanderAction.State != "needs-reconcile" {
		t.Fatalf("unexpected blocked lane executor action: %+v", laneAction)
	}
	counts := status.CaseMission.FactCounts
	if counts.Observations != 1 || counts.Requests != 1 || counts.Candidates != 2 || counts.Publications != 1 || counts.PendingDecisions != 1 || status.CaseMission.Sections.OpenCandidates.Total != 2 || status.CaseMission.Sections.OpenCandidates.Shown != 2 || status.CaseMission.Sections.PendingGates.Total != 1 || status.CaseMission.Sections.PendingGates.Shown != 1 || status.CaseMission.Sections.Batches.Total != 1 || status.CaseMission.Sections.Batches.Shown != 1 {
		t.Fatalf("unexpected case mission ledger/progress handoff: counts=%+v sections=%+v", status.CaseMission.FactCounts, status.CaseMission.Sections)
	}
	evidence := status.CaseMission.ExecutionEvidenceReview[0]
	if evidence.EventID != "obs-auth-1" || evidence.GateEventID != "gate-auth-1" || evidence.Action != "debug" || !containsSubstring(evidence.OutputRefs, "workspace/main/debug/out.txt") || !containsSubstring(evidence.EvidenceRefs, "evidence/debug.json") || !containsSubstring(evidence.Boundary, "do not replay heavy tool") || evidence.MissionCommanderAction.State != "ready-for-evidence-review" || evidence.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || !containsSubstring(evidence.MissionCommanderAction.FollowUpCommands, "/rekit continue main -WhatIf") || evidence.FollowThrough.State != "ready-for-evidence-review" || !cliExecutionEvidenceFollowThroughContains(evidence.FollowThrough, "recorded-evidence-review", "reviewed outputRefs/evidenceRefs") {
		t.Fatalf("unexpected case mission evidence review item: %+v", evidence)
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status case mission：summary=openLanes=1 ready=0 blocked=1 pendingGates=1 authorizedGates=0 openDecisions=3 interventions=1 ready=false lanes=1 readyLanes=0 blockedLanes=1 evidenceReview=1",
		"status case mission queue：total=5 unblocked=2 blocked=3 requiresReview=5 followUp=3 current=/rekit handoff main",
		"status case mission queue action：bucket=current lane=main label=gate-auth-1 state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=/rekit handoff main",
		"status case mission queue action：bucket=reviewRequired lane=main label=gate-auth-1 state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=/rekit handoff main",
		"status case mission queue action reason：bucket=reviewRequired lane=main reason=review execution evidence for gateEventId gate-auth-1",
		"status case mission queue action boundary：bucket=reviewRequired lane=main boundary=observation evidence is already recorded; do not replay heavy tool",
		"status case mission queue action：bucket=blocked lane=main label=main",
		"status case mission blocked lane：main (pending-gate,intervention,open-decision)",
		"status case mission lane action：lane=main label=main status=open workspace=captures/lanes/main executor=session-main generation=3 ready=false blocked=true pendingGates=1 openInterventions=1 openDecisions=3 resume=/rekit continue main handoff=/rekit handoff main commanderState=needs-reconcile commanderPrimary=/rekit reconcile main -InterventionId <eventId> -Apply",
		"status case mission lane blocker：lane=main reason=pending-gate",
		"status case mission lane blocker：lane=main reason=intervention",
		"status case mission lane blocker：lane=main reason=open-decision",
		"status case mission lane boundary：lane=main boundary=do not run continue for blocked lanes",
		"status case mission pending gate：debug gate | lane=main | risk=high | target=batch-overview | action=debug | scope=handler only | stopConditions=timeout",
		"status case mission pending gate handoff：eventId= lane=main subject=debug gate action=debug target=batch-overview status=pending-gate risk=high auth= profile= review=/rekit handoff main whatIf=/rekit gate -Target \"" + caseRoot + "\" -Pack _template -Action debug -Lane main -WhatIf -Subject \"debug gate\" -Summary \"needs confirmation\" -TargetRef \"batch-overview\" -BatchId \"batch-overview\" -Scope \"handler only\" -Budget \"30s\" -TriedLightSteps \"overview,static review\" -StopConditions \"timeout\" -Risk \"high\" -Format json apply=/rekit gate -Target \"" + caseRoot + "\" -Pack _template -Action debug -Lane main -Apply -Actor runtime-test -Subject \"debug gate\" -Summary \"needs confirmation\" -TargetRef \"batch-overview\" -BatchId \"batch-overview\" -Scope \"handler only\" -Budget \"30s\" -TriedLightSteps \"overview,static review\" -StopConditions \"timeout\" -Risk \"high\" -Format json",
		"status case mission pending gate decision boundary：eventId= boundary=review with the main agent/user or update strict durable autonomy before any heavy action; apply command only replays/records the gate request decision and does not execute or approve heavy action by itself",
		"status case mission pending gate continue boundary：eventId= boundary=blocked lane can only continue with -WhatIf until the pending gate is resolved or deliberately deferred; no heavy-tool, authority, or confirmed writes",
		"status case mission pending gate evidence：eventId= evidence=requested budget 30s",
		"status case mission pending gate evidence：eventId= evidence=requested stopConditions timeout",
		"status case mission pending gate evidence：eventId= evidence=triedLightSteps overview,static review",
		"status case mission open decision：candidate: handler | lane=main | status=open | summary=candidate one",
		"status case mission open decision handoff：eventId=cand-main-1 kind=candidate lane=main subject=handler summary=candidate one decision= reason= status=open target=candidate-alpha confidence=high sourceKind=candidate sourcePath=.rekit/facts/candidates.jsonl recordPath=.rekit/facts/decisions.jsonl review=/rekit handoff main sourceCommand=/rekit note -Target \"" + caseRoot + "\" -Pack _template -List -Kind candidate -Lane main -Format json whatIf=/rekit note -Target \"" + caseRoot + "\" -Pack _template -Kind decision -Lane main -Subject \"decision for candidate: handler\"",
		"-Decision \"<accept|reject|defer|supersede>\" -Reason \"reviewed open candidate/decision item\" -TargetRef \"candidate-alpha\" -Related \"cand-main-1\" -EvidenceRefs \"evidence/candidate.json\" -WhatIf -Format json record=/rekit note -Target \"" + caseRoot + "\" -Pack _template -Kind decision -Lane main -Subject \"decision for candidate: handler\"",
		"status case mission open decision boundary：eventId=cand-main-1 boundary=review evidence and choose accept/reject/defer/supersede before recording a decision note; record command only appends case-local decision ledger state and never writes authority/confirmed or executes heavy-tool",
		"status case mission open decision continue boundary：eventId=cand-main-1 boundary=blocked lane can only continue with -WhatIf after open candidate/decision review is recorded or deliberately deferred; do not continue autonomously while the open decision remains unresolved",
		"status case mission open decision evidence：eventId=cand-main-1 evidence=candidate ledger event cand-main-1",
		"status case mission open decision evidence：eventId=cand-main-1 evidence=confidence high",
		"status case mission open decision evidence：eventId=cand-main-1 evidence=evidenceRefs evidence/candidate.json",
		"status case mission open decision evidence：eventId=cand-main-1 evidence=sourcePath .rekit/facts/candidates.jsonl",
		"status case mission open decision evidence：eventId=cand-main-1 evidence=recordPath .rekit/facts/decisions.jsonl",
		"status case mission open decision handoff：eventId=dec-main-1 kind=decision lane=main subject=decision subject summary= decision=defer reason=needs review status=open target=decision-alpha confidence= sourceKind=decision sourcePath=.rekit/facts/decisions.jsonl recordPath=.rekit/facts/decisions.jsonl review=/rekit handoff main sourceCommand=/rekit note -Target \"" + caseRoot + "\" -Pack _template -List -Kind decision -Lane main -Format json whatIf=/rekit note -Target \"" + caseRoot + "\" -Pack _template -Kind decision -Lane main -Subject \"decision for decision: decision subject\"",
		"-Reason \"needs review\" -TargetRef \"decision-alpha\" -Related \"dec-main-1\" -EvidenceRefs \"evidence/decision.json\" -BatchId \"batch-overview\" -WhatIf -Format json record=/rekit note -Target \"" + caseRoot + "\" -Pack _template -Kind decision -Lane main -Subject \"decision for decision: decision subject\"",
		"status case mission open decision evidence：eventId=dec-main-1 evidence=decision ledger event dec-main-1",
		"status case mission open decision evidence：eventId=dec-main-1 evidence=sourcePath .rekit/facts/decisions.jsonl",
		"status case mission open decision evidence：eventId=dec-main-1 evidence=recordPath .rekit/facts/decisions.jsonl",
		"status case mission open decision evidence：eventId=dec-main-1 evidence=batchId batch-overview",
		"status case mission intervention：manual override | lane=main | action=override | status=open | target=batch-overview",
		"status case mission intervention handoff：eventId=int-main-1 lane=main subject=manual override summary=needs human action=override target=batch-overview status=open scope=metadata approvedBy=lead review=/rekit handoff main whatIf=/rekit reconcile main -Target \"" + caseRoot + "\" -Pack _template -InterventionId int-main-1 -WhatIf -Format json apply=/rekit reconcile main -Target \"" + caseRoot + "\" -Pack _template -InterventionId int-main-1 -Apply -Format json",
		"status case mission intervention decision boundary：eventId=int-main-1 boundary=review the open intervention before reconcile apply; apply command only writes case-local intervention/lane/resume/checkpoint/board state and never writes authority/confirmed or executes heavy-tool",
		"status case mission intervention continue boundary：eventId=int-main-1 boundary=blocked lane can only continue with -WhatIf after the intervention is reconciled or deliberately deferred; do not continue autonomously while intervention remains open",
		"status case mission intervention evidence：eventId=int-main-1 evidence=open intervention ledger event int-main-1",
		"status case mission intervention evidence：eventId=int-main-1 evidence=approvedBy lead",
		"status case mission intervention evidence：eventId=int-main-1 evidence=scope metadata",
		"status case mission facts：observations=1 requests=1 candidates=2 publications=1 pendingDecisions=1",
		"status case mission section：name=openCandidates total=2 shown=2",
		"status case mission section event：section=openCandidates index=1 eventId=cand-main-1 kind=candidate status=open lane=main subject=handler summary=candidate one action= decision=",
		"status case mission section：name=pendingGates total=1 shown=1",
		"status case mission section event：section=pendingGates index=1 eventId= kind=request status=pending-gate lane=main subject=debug gate summary=needs confirmation action= decision=",
		"status case mission section：name=batches total=1 shown=1",
		"status case mission batch：index=1 id=batch-overview events=5",
		"status case mission evidence review summary：total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=1 boundaryHits=0 hasEscalation=false hasExecutionReport=false hasAdapter=false latestEventId=obs-auth-1 latestGateEventId=gate-auth-1 latestStatus=complete latestAction=debug latestTarget=target-alpha",
		"latestReview=review outputRefs/evidenceRefs for gateEventId gate-auth-1 latestHandoff=/rekit handoff main latestCommanderState=ready-for-evidence-review latestCommanderPrimary=/rekit handoff main",
		"outcomes=1 followThrough=ready-for-evidence-review nextActions=5 reviewRequiredActions=5 currentAction=/rekit handoff main",
		"status case mission evidence review summary action queue：total=5 unblocked=2 blocked=3 requiresReview=5 followUp=3 current=/rekit handoff main",
		"status case mission evidence review summary boundary：execution evidence review summary is read-only; full executionEvidenceReview remains available",
		"status case mission evidence review summary boundary：no authority/confirmed writes",
		"status case mission evidence review：eventId=obs-auth-1 gateEventId=gate-auth-1 status=complete action=debug",
		"review=review outputRefs/evidenceRefs for gateEventId gate-auth-1 handoff=/rekit handoff main commanderState=ready-for-evidence-review commanderPrimary=/rekit handoff main",
		"status case mission evidence output ref：eventId=obs-auth-1 ref=workspace/main/debug/out.txt",
		"status case mission evidence evidence ref：eventId=obs-auth-1 ref=evidence/debug.json",
		"status case mission evidence follow-through：eventId=obs-auth-1 state=ready-for-evidence-review gateEventId=gate-auth-1",
		"status case mission evidence outcome：eventId=obs-auth-1 name=recorded-evidence-review",
		"status case mission evidence outcome when：eventId=obs-auth-1 name=recorded-evidence-review when=bounded observation evidence was recorded for an authorized gate",
		"status case mission evidence outcome evidence：eventId=obs-auth-1 name=recorded-evidence-review evidence=workspace/main/debug/out.txt",
		"status case mission evidence boundary：eventId=obs-auth-1 boundary=observation evidence is already recorded; do not replay heavy tool",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status case evidence text missing %q:\n%s", expected, out.String())
		}
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "status case mission queue action：bucket=current lane=main label=gate-auth-1 state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=/rekit handoff main") || !strings.Contains(out.String(), "status case mission queue action：bucket=blocked lane=main label=main") || !strings.Contains(out.String(), "status case mission blocked lane：main (pending-gate,intervention,open-decision)") || !strings.Contains(out.String(), "status case mission lane action：lane=main label=main status=open workspace=captures/lanes/main executor=session-main generation=3 ready=false blocked=true pendingGates=1 openInterventions=1 openDecisions=3") || !strings.Contains(out.String(), "status case mission lane blocker：lane=main reason=pending-gate") || !strings.Contains(out.String(), "status case mission pending gate：debug gate | lane=main") || !strings.Contains(out.String(), "status case mission pending gate handoff：eventId= lane=main subject=debug gate action=debug") || !strings.Contains(out.String(), `whatIf=/rekit gate -Target "`+caseRoot+`" -Pack _template -Action debug -Lane main -WhatIf`) || !strings.Contains(out.String(), "status case mission pending gate continue boundary：eventId= boundary=blocked lane can only continue with -WhatIf") || !strings.Contains(out.String(), "status case mission facts：observations=1 requests=1 candidates=2 publications=1 pendingDecisions=1") || !strings.Contains(out.String(), "status case mission section：name=openCandidates total=2 shown=2") || !strings.Contains(out.String(), "status case mission batch：index=1 id=batch-overview events=5") || !strings.Contains(out.String(), "status case mission open decision handoff：eventId=cand-main-1 kind=candidate lane=main subject=handler") || !strings.Contains(out.String(), "sourceKind=candidate sourcePath=.rekit/facts/candidates.jsonl recordPath=.rekit/facts/decisions.jsonl") || !strings.Contains(out.String(), "sourceCommand=/rekit note -Target \""+caseRoot+"\" -Pack _template -List -Kind candidate -Lane main -Format json whatIf=/rekit note") || !strings.Contains(out.String(), "-Related \"cand-main-1\" -EvidenceRefs \"evidence/candidate.json\" -WhatIf -Format json record=/rekit note") || !strings.Contains(out.String(), "status case mission open decision continue boundary：eventId=cand-main-1 boundary=blocked lane can only continue with -WhatIf") || !strings.Contains(out.String(), "status case mission open decision handoff：eventId=dec-main-1 kind=decision lane=main subject=decision subject") || !strings.Contains(out.String(), "sourceCommand=/rekit note -Target \""+caseRoot+"\" -Pack _template -List -Kind decision -Lane main -Format json whatIf=/rekit note") || !strings.Contains(out.String(), "status case mission intervention handoff：eventId=int-main-1 lane=main subject=manual override") || !strings.Contains(out.String(), "whatIf=/rekit reconcile main -Target \""+caseRoot+"\" -Pack _template -InterventionId int-main-1 -WhatIf") || !strings.Contains(out.String(), "status case mission intervention continue boundary：eventId=int-main-1 boundary=blocked lane can only continue with -WhatIf") || !strings.Contains(out.String(), "status case mission evidence review：eventId=obs-auth-1 gateEventId=gate-auth-1") || !strings.Contains(out.String(), "status case mission evidence outcome evidence：eventId=obs-auth-1 name=recorded-evidence-review evidence=evidence/debug.json") {
		t.Fatalf("status case default text missing lane/queue/ledger/blocker/pending-gate/evidence review handoff:\n%s", out.String())
	}
	assertSnapshotEqual(t, before, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
}

func TestRunStatusJsonCaseShimDrift(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeCaseFile(t, caseRoot, ".claude/skills/rekit/SKILL.md", "drift\n")

	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Case struct {
			ShimMatchesTemplate bool `json:"shimMatchesTemplate"`
		} `json:"case"`
		CaseShim struct {
			Ready                        bool     `json:"ready"`
			Summary                      string   `json:"summary"`
			InstalledShimMatchesTemplate *bool    `json:"installedShimMatchesTemplate"`
			Warnings                     []string `json:"warnings"`
			NextSteps                    []string `json:"nextSteps"`
		} `json:"caseShim"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("case status JSON did not decode: %v\n%s", err, out.String())
	}
	if status.Case.ShimMatchesTemplate || status.CaseShim.Ready || status.CaseShim.Summary != "case shim readiness has warnings" || status.CaseShim.InstalledShimMatchesTemplate == nil || *status.CaseShim.InstalledShimMatchesTemplate || !containsSubstring(status.CaseShim.NextSteps, `/rekit repair -Target "`+caseRoot+`" -Pack _template -WhatIf -Format text previews case-local thin-shim refresh; run repair -Apply only after explicit confirmation`) {
		t.Fatalf("unexpected drift status: %+v", status)
	}
	foundDrift := false
	for _, warning := range status.CaseShim.Warnings {
		if strings.Contains(warning, "shim differs") {
			foundDrift = true
			break
		}
	}
	if !foundDrift {
		t.Fatalf("case shim warnings = %+v, want drift warning", status.CaseShim.Warnings)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status case shim：summary=case shim readiness has warnings ready=false",
		"matchesTemplate=false",
		"warnings=1",
		"status case shim warning：case-local /rekit shim differs from canonical thin shim template",
		`status case shim next step：/rekit repair -Target "` + caseRoot + `" -Pack _template -WhatIf -Format text previews case-local thin-shim refresh; run repair -Apply only after explicit confirmation`,
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status drift text missing %q:\n%s", expected, out.String())
		}
	}
}

func TestRunStatusMovedCaseNextSteps(t *testing.T) {
	caseRoot := movedAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Case struct {
			Moved     bool     `json:"moved"`
			NextSteps []string `json:"nextSteps"`
		} `json:"case"`
		CaseShim struct {
			NextSteps []string `json:"nextSteps"`
		} `json:"caseShim"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("moved case status JSON did not decode: %v\n%s", err, out.String())
	}
	wantRepair := `/rekit repair -Target "` + caseRoot + `" -Pack _template -WhatIf -Format text previews moved-case metadata and thin-shim refresh; run repair -Apply only after explicit confirmation`
	if !status.Case.Moved || !containsSubstring(status.Case.NextSteps, wantRepair) {
		t.Fatalf("moved case status omitted repair next step: %+v", status.Case)
	}
	if !containsSubstring(status.CaseShim.NextSteps, `/rekit repair -Target "`+caseRoot+`" -Pack _template -WhatIf -Format text previews case-local thin-shim refresh; run repair -Apply only after explicit confirmation`) {
		t.Fatalf("moved case status omitted shim repair next step: %+v", status.CaseShim)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status case：root=" + caseRoot,
		"moved=true",
		"status case next step：" + wantRepair,
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("moved case status text missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "case next step: "+wantRepair) || !strings.Contains(out.String(), "detected moved case metadata") {
		t.Fatalf("moved case default status missing repair handoff:\n%s", out.String())
	}
}

func TestRunStatusKitShowsOpenPackMemoryCandidates(t *testing.T) {
	root := repoRoot(t)
	candidateRoot := filepath.Join(root, "packs", "_template", "promote-candidates")
	toolingRoot := filepath.Join(root, "packs", "_template", "tooling", "candidates")
	candidateBefore := snapshotFiles(t, candidateRoot)
	toolingBefore := snapshotFiles(t, toolingRoot)
	t.Cleanup(func() {
		removeNewFiles(t, candidateRoot, candidateBefore)
		removeNewFiles(t, toolingRoot, toolingBefore)
	})
	writePathFile(t, filepath.Join(candidateRoot, "batch501-status.candidate.md"), "# candidate\n")
	writePathFile(t, filepath.Join(toolingRoot, "batch501-tooling.candidate.md"), "# tooling\n")
	writePathFile(t, filepath.Join(candidateRoot, "index.json"), `[
  {
    "path": "references/template/README.md",
    "candidate": "batch501-status.candidate.md"
  }
]
`)
	writePathFile(t, filepath.Join(candidateRoot, "review-artifacts", "batch501-status.candidate-decision-note.md"), "# decision\n")

	var out bytes.Buffer
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		ProjectHandoff struct {
			Ready                bool `json:"ready"`
			PackMemoryCandidates struct {
				Ready      bool   `json:"ready"`
				Summary    string `json:"summary"`
				Total      int    `json:"total"`
				NextAction string `json:"nextAction"`
				Packs      []struct {
					Pack            string                               `json:"pack"`
					CandidateRoot   string                               `json:"candidateRoot"`
					ToolingRoot     string                               `json:"toolingRoot"`
					IndexPath       string                               `json:"indexPath"`
					CandidateFiles  int                                  `json:"candidateFiles"`
					ToolingFiles    int                                  `json:"toolingFiles"`
					IndexEntries    int                                  `json:"indexEntries"`
					RequiresReview  bool                                 `json:"requiresReview"`
					RequiresCleanup bool                                 `json:"requiresCleanup"`
					ReviewSummary   packMemoryCandidateReviewSummaryJSON `json:"reviewSummary"`
					Evidence        []string                             `json:"evidence"`
					Boundary        []string                             `json:"boundary"`
				} `json:"packs"`
			} `json:"packMemoryCandidates"`
		} `json:"projectHandoff"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("status JSON with pack-memory candidates did not decode: %v\n%s", err, out.String())
	}
	candidates := status.ProjectHandoff.PackMemoryCandidates
	if status.ProjectHandoff.Ready || candidates.Ready || candidates.Summary != "pack-memory candidate inventory has open review/cleanup/verification work" || candidates.Total != 3 || len(candidates.Packs) != 1 || !strings.Contains(candidates.NextAction, "review listed pack-memory candidates or complete listed candidate decision verification") {
		t.Fatalf("unexpected status pack-memory candidate handoff: %+v", status.ProjectHandoff)
	}
	pack := candidates.Packs[0]
	if pack.Pack != "_template" || pack.CandidateRoot != "packs/_template/promote-candidates" || pack.ToolingRoot != "packs/_template/tooling/candidates" || pack.IndexPath != "packs/_template/promote-candidates/index.json" || pack.CandidateFiles != 1 || pack.ToolingFiles != 1 || pack.IndexEntries != 1 || !pack.RequiresReview || !pack.RequiresCleanup || !containsSubstring(pack.Evidence, "promote-candidates files=1") || !containsSubstring(pack.Boundary, "does not merge, delete") {
		t.Fatalf("unexpected status pack-memory candidate pack: %+v", pack)
	}
	if summary := pack.ReviewSummary; summary.Total != 3 || summary.CandidateFiles != 1 || summary.ToolingFiles != 1 || summary.IndexEntries != 1 || summary.DecisionArtifactCount != 2 || summary.CleanupArtifactCount != 2 || summary.ReconsumeArtifactCount != 4 || summary.ProofSummary.Total != 8 || summary.ProofSummary.Present != 1 || summary.ProofSummary.Missing != 7 || summary.ProofSummary.DecisionPresent != 1 || summary.ProofSummary.DecisionMissing != 1 || summary.ProofSummary.CleanupMissing != 2 || summary.ProofSummary.ReconsumeMissing != 4 || summary.ProofSummary.ProofProgress != "1/8" || summary.ProofSummary.CurrentStage != "decision-proof-required" || summary.ProofSummary.NextMissingProofType != "candidate-decision-note" || summary.ProofSummary.NextMissingProofPath != "packs/_template/promote-candidates/review-artifacts/batch501-tooling.candidate-decision-note.md" || summary.ProofSummary.NextMissingCandidatePath != "packs/_template/tooling/candidates/batch501-tooling.candidate.md" || summary.ProofSummary.NextMissingPackTarget != "tooling/catalog.yml or tooling/recipes/*" || summary.ProofSummary.NextMissingProof == nil || summary.ProofSummary.NextMissingProof.ProofType != "candidate-decision-note" || summary.ProofSummary.NextMissingProof.Stage != "decision-proof-required" || !strings.Contains(summary.ProofSummary.NextMissingProof.Action, "selected decisionFollowThrough outcome") || !containsSubstring(summary.ProofSummary.NextMissingProof.Evidence, "decision note path/ref") || summary.ProofSummary.Complete || summary.ProofSummary.ProofRoot != "packs/_template/promote-candidates/review-artifacts" || !strings.Contains(summary.ProofSummary.NextAction, "candidate-decision-note") || !containsSubstring(summary.ProofSummary.Boundary, "read-only") || !summary.RequiresReview || !summary.RequiresCleanup || !summary.HasDecisionArtifacts || !summary.HasCleanupArtifacts || !summary.HasReconsumeArtifacts || !containsSubstring(summary.Boundary, "reviewSummary is read-only") {
		t.Fatalf("unexpected status pack-memory candidate review summary: %+v", summary)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status pack-memory candidates：summary=pack-memory candidate inventory has open review/cleanup/verification work ready=false total=3 packs=1 nextAction=review listed pack-memory candidates or complete listed candidate decision verification",
		"status pack-memory candidate pack：pack=_template candidateRoot=packs/_template/promote-candidates toolingRoot=packs/_template/tooling/candidates indexPath=packs/_template/promote-candidates/index.json candidateFiles=1 toolingFiles=1 indexEntries=1 receipts=0 pendingVerification=0 completedVerification=0 review=true cleanup=true verification=false",
		"status pack-memory review summary：pack=_template total=3 candidateFiles=1 toolingFiles=1 indexEntries=1 reviewArtifacts=8 decisionArtifacts=2 cleanupArtifacts=2 reconsumeArtifacts=4 proofTotal=8 proofPresent=1 proofMissing=7 proofProgress=1/8 proofStage=decision-proof-required nextMissingProofType=candidate-decision-note nextMissingProofPath=packs/_template/promote-candidates/review-artifacts/batch501-tooling.candidate-decision-note.md nextMissingCandidatePath=packs/_template/tooling/candidates/batch501-tooling.candidate.md proofComplete=false proofRoot=packs/_template/promote-candidates/review-artifacts",
		"status pack-memory proof summary：pack=_template total=8 present=1 missing=7 progress=1/8 stage=decision-proof-required decisionPresent=1 decisionMissing=1 cleanupPresent=0 cleanupMissing=2 reconsumePresent=0 reconsumeMissing=4 nextMissingType=candidate-decision-note nextMissingPath=packs/_template/promote-candidates/review-artifacts/batch501-tooling.candidate-decision-note.md nextMissingCandidate=packs/_template/tooling/candidates/batch501-tooling.candidate.md nextMissingTarget=tooling/catalog.yml or tooling/recipes/* complete=false proofRoot=packs/_template/promote-candidates/review-artifacts",
		"status pack-memory next missing proof：pack=_template stage=decision-proof-required proofType=candidate-decision-note path=packs/_template/promote-candidates/review-artifacts/batch501-tooling.candidate-decision-note.md candidatePath=packs/_template/tooling/candidates/batch501-tooling.candidate.md packTarget=tooling/catalog.yml or tooling/recipes/*",
		"status pack-memory next missing proof evidence：pack=_template evidence=selected decisionFollowThrough outcome",
		"status pack-memory proof summary boundary：pack=_template boundary=proofSummary is read-only; release/status detects proof files but does not create or validate their contents",
		"status pack-memory review summary boundary：pack=_template boundary=pack-memory reviewSummary is read-only; full candidate paths, indexCandidates, and reviewArtifacts remain available",
		"status pack-memory candidate path：pack=_template path=packs/_template/promote-candidates/",
		"status pack-memory tooling candidate path：pack=_template path=packs/_template/tooling/candidates/",
		"status pack-memory candidate index：pack=_template path=references/template/README.md candidate=packs/_template/promote-candidates/",
		"status pack-memory review artifact：pack=_template name=candidate-decision-note candidatePath=packs/_template/promote-candidates/batch501-status.candidate.md packTarget=references/template/README.md proofPresent=true proofPath=packs/_template/promote-candidates/review-artifacts/batch501-status.candidate-decision-note.md",
		"status pack-memory review artifact：pack=_template name=candidate-cleanup-proof candidatePath=packs/_template/promote-candidates/batch501-status.candidate.md packTarget=references/template/README.md proofPresent=false proofPath= expectedProofs=packs/_template/promote-candidates/review-artifacts/batch501-status.candidate-cleanup-proof.md",
		"status pack-memory review artifact evidence：pack=_template name=candidate-decision-note evidence=selected decisionFollowThrough outcome",
		"status pack-memory review artifact boundary：pack=_template name=candidate-cleanup-proof boundary=cleanup is limited to candidateRoot/toolingRoot and indexPath",
		"status pack-memory candidate evidence：pack=_template evidence=promote-candidates files=1",
		"status pack-memory candidate boundary：pack=_template boundary=release handoff inventories candidate residue and durable decision verification receipts; it does not merge, delete, or validate cases",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status text missing pack-memory candidate handoff %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	err := Run([]string{"-Command", "release-check", "-Format", "text"}, &out)
	if err == nil || !strings.Contains(err.Error(), "pack-memory candidates") {
		t.Fatalf("release-check with open pack-memory candidates error = %v, want pack-memory candidates warning", err)
	}
	for _, expected := range []string{
		"release-check pack-memory review summary：pack=_template total=3 candidateFiles=1 toolingFiles=1 indexEntries=1 reviewArtifacts=8 decisionArtifacts=2 cleanupArtifacts=2 reconsumeArtifacts=4 proofTotal=8 proofPresent=1 proofMissing=7 proofProgress=1/8 proofStage=decision-proof-required nextMissingProofType=candidate-decision-note nextMissingProofPath=packs/_template/promote-candidates/review-artifacts/batch501-tooling.candidate-decision-note.md nextMissingCandidatePath=packs/_template/tooling/candidates/batch501-tooling.candidate.md proofComplete=false proofRoot=packs/_template/promote-candidates/review-artifacts",
		"release-check pack-memory proof summary：pack=_template total=8 present=1 missing=7 progress=1/8 stage=decision-proof-required decisionPresent=1 decisionMissing=1 cleanupPresent=0 cleanupMissing=2 reconsumePresent=0 reconsumeMissing=4 nextMissingType=candidate-decision-note nextMissingPath=packs/_template/promote-candidates/review-artifacts/batch501-tooling.candidate-decision-note.md nextMissingCandidate=packs/_template/tooling/candidates/batch501-tooling.candidate.md nextMissingTarget=tooling/catalog.yml or tooling/recipes/* complete=false proofRoot=packs/_template/promote-candidates/review-artifacts",
		"release-check pack-memory next missing proof：pack=_template stage=decision-proof-required proofType=candidate-decision-note path=packs/_template/promote-candidates/review-artifacts/batch501-tooling.candidate-decision-note.md candidatePath=packs/_template/tooling/candidates/batch501-tooling.candidate.md packTarget=tooling/catalog.yml or tooling/recipes/*",
		"release-check pack-memory next missing proof evidence：pack=_template evidence=selected decisionFollowThrough outcome",
		"release-check pack-memory review artifact：pack=_template name=candidate-decision-note candidatePath=packs/_template/promote-candidates/batch501-status.candidate.md packTarget=references/template/README.md proofPresent=true proofPath=packs/_template/promote-candidates/review-artifacts/batch501-status.candidate-decision-note.md",
		"release-check pack-memory review summary boundary：pack=_template boundary=pack-memory reviewSummary is read-only; full candidate paths, indexCandidates, and reviewArtifacts remain available",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("release-check text missing pack-memory review summary %q:\n%s", expected, out.String())
		}
	}
}

type packMemoryCandidateReviewSummaryJSON struct {
	Total                  int                             `json:"total"`
	CandidateFiles         int                             `json:"candidateFiles"`
	ToolingFiles           int                             `json:"toolingFiles"`
	IndexEntries           int                             `json:"indexEntries"`
	ReviewArtifactCount    int                             `json:"reviewArtifactCount"`
	DecisionArtifactCount  int                             `json:"decisionArtifactCount"`
	CleanupArtifactCount   int                             `json:"cleanupArtifactCount"`
	ReconsumeArtifactCount int                             `json:"reconsumeArtifactCount"`
	ProofSummary           packMemoryCandidateProofSummary `json:"proofSummary"`
	RequiresReview         bool                            `json:"requiresReview"`
	RequiresCleanup        bool                            `json:"requiresCleanup"`
	HasDecisionArtifacts   bool                            `json:"hasDecisionArtifacts"`
	HasCleanupArtifacts    bool                            `json:"hasCleanupArtifacts"`
	HasReconsumeArtifacts  bool                            `json:"hasReconsumeArtifacts"`
	Boundary               []string                        `json:"boundary"`
}

type packMemoryCandidateProofSummary struct {
	Total                    int                     `json:"total"`
	Present                  int                     `json:"present"`
	Missing                  int                     `json:"missing"`
	DecisionPresent          int                     `json:"decisionPresent"`
	DecisionMissing          int                     `json:"decisionMissing"`
	CleanupMissing           int                     `json:"cleanupMissing"`
	ReconsumeMissing         int                     `json:"reconsumeMissing"`
	ProofRoot                string                  `json:"proofRoot"`
	ProofProgress            string                  `json:"proofProgress"`
	CurrentStage             string                  `json:"currentStage"`
	NextMissingProofType     string                  `json:"nextMissingProofType"`
	NextMissingProofPath     string                  `json:"nextMissingProofPath"`
	NextMissingCandidatePath string                  `json:"nextMissingCandidatePath"`
	NextMissingPackTarget    string                  `json:"nextMissingPackTarget"`
	NextMissingProof         *nextMissingProofDetail `json:"nextMissingProof"`
	Complete                 bool                    `json:"complete"`
	NextAction               string                  `json:"nextAction"`
	Boundary                 []string                `json:"boundary"`
}

type nextMissingProofDetail struct {
	Stage         string   `json:"stage"`
	ProofType     string   `json:"proofType"`
	Path          string   `json:"path"`
	CandidatePath string   `json:"candidatePath"`
	PackTarget    string   `json:"packTarget"`
	When          string   `json:"when"`
	Action        string   `json:"action"`
	Format        string   `json:"format"`
	Evidence      []string `json:"evidence"`
	Boundary      []string `json:"boundary"`
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
	assertReleaseCheckDocument(t, result.Documents, "docs/context-routing.md")
	assertReleaseCheckDocument(t, result.Documents, "docs/batch-plan.md")
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

	out.Reset()
	if err := Run([]string{"-Command", "release-check", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"release-check：mutation=false ready=true summary=release gate inventory ok",
		"gateProfile=local-ci-minimum",
		"release-check ci gate：workflow=.github/workflows/release-gate.yml ready=true jobs=3 commands=18",
		"boundary=inventory-ready-not-remote-ci-green",
		"release-check required command：command=go run ./cmd/rekit -- -Command release-check -Format json kind=go-run repoPath=cmd/rekit required=true present=true resolved=true inCatalog=true",
		"release-check document：path=docs/context-routing.md present=true",
		"release-check heavy actions：actions=debug,dump,full-trace,inject,network,patch,symex",
		"release-check go-native public surface：summary=Go-native public command surface inventory ok ready=true entrypoint=cmd/rekit",
		"release-check command group：group=readOnly commands=doctor,packs,release-check,status,validate",
		"release-check command profile：command=release-check boundary=read-only mutation=false",
		"release-check command boundary：boundary=case-local-apply",
		"release-check command policy：policy=no-heavy-tool ready=true violations=0",
		"release-check go-native facade prerequisite：name=entrypoint ready=true",
		"release-check PowerShell deprecation：summary=PowerShell deprecation inventory ok ready=true strategy=docs/powershell-deprecation.md",
		"facadeRuntimeReady=true facade=rekit/rekit.ps1 facadeLegacyImports=false facadeDispatcher=false",
		"release-check PowerShell command owner：area=release-check",
		"release-check PowerShell module：path=rekit/rekit.ps1",
		"release-check PowerShell public facade：summary=",
		"path=rekit/rekit.ps1 present=true retained=true",
		"release-check public facade removal：summary=public facade removal prerequisites ok ready=true",
		"planDocument=docs/powershell-deprecation.md",
		"release-check public facade prerequisite：name=public-facade-retained-boundary ready=true",
		"release-check public facade replacement：name=",
		"release-check public facade deletion gate：name=",
		"release-check public facade execution step：name=",
		"allowsPowerShellRuntime=false allowsExternalEffects=false",
		"release-check public facade boundary check：name=",
		"release-check public facade recovery step：name=",
		"release-check public facade documentation target：path=README.md",
		"release-check public facade reference category：name=",
		"release-check public facade migration target：path=",
		"release-check public facade smoke migration target：path=",
		"release-check case shim：summary=case shim readiness ok ready=true template=rekit/templates/case-shim/SKILL.md",
		"release-check case shim phrase：phrase=Go-native backend present=true",
		"release-check canonical skill phrase：phrase=底层 Go CLI 是 canonical runtime present=true",
		"release-check case shim forbidden：pattern=PowerShell present=false",
		"release-check public default docs：summary=public default docs readiness ok ready=true documents=15",
		"release-check public default doc：path=README.md present=true",
		"release-check public default phrase：path=docs/context-routing.md present=true phrase=按需路由",
		"release-check public default docs boundary：",
		"release-check release handoff：summary=release handoff summary ok ready=true",
		"release-check latest batch：batch=Batch ",
		"localValidationReady=",
		"release-check latest batch remote gate：state=",
		"release-check latest batch remote gate boundary：",
		"release-check latest batch release inspection cadence：state=",
		"maxPushes=2",
		"thirdInspectionAllowed=",
		"release-check latest batch release inspection cadence boundary：do not add a third record commit",
		"release-check latest batch release inspection cadence：state=",
		"release-check release notes：path=CHANGELOG.md present=true",
		"release-check read first：path=docs/context-routing.md present=true",
		"release-check signal：name=CI release gate ready=true summary=CI release gate inventory ok",
		"release-check signal detail：name=latest batch documentation detail=localValidationReady=",
		"release-check signal detail：name=latest batch documentation detail=nextAction=",
		"release-check signal detail：name=Go-native public surface detail=profileGroups readOnly=doctor,packs,release-check,status,validate",
		"release-check pack maturity：summary=pack maturity inventory ok total=10",
		"release-check pack gate：id=vmp-re maturity=mature schemaValid=true schemaVersion=1 heavyToolGates=7 actions=debug,dump,full-trace,inject,network,patch,symex",
		"release-check pack-memory candidates：summary=pack-memory candidate inventory ok ready=true total=0 packs=0 nextAction=no pack-memory candidate cleanup is pending",
		"release-check signal：name=pack-memory candidates ready=true summary=pack-memory candidate inventory ok",
		"release-check signal detail：name=pack-memory candidates detail=openPacks=0 total=0 ready=true",
		"release-check validation：command=go run ./cmd/rekit -- -Command release-check -Format json kind=go-run repoPath=cmd/rekit required=true present=true resolved=true",
		"release-check known gap：index=1 category=ci-release-gate",
		"release-check known gap detail：远程 release-gate",
		"release-check next action：Read docs/context-routing.md first",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("release-check text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("release-check text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "release-check", "-Format", "table"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "release-check: release gate inventory ok") || strings.Contains(out.String(), "release-check：mutation=") {
		t.Fatalf("release-check table should keep legacy text output:\n%s", out.String())
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
	if counts.ReadFirst != 4 || counts.Signals != 13 || counts.KnownGaps == 0 || counts.PackMaturity.Total == 0 || counts.Validation == 0 || counts.NextActions == 0 {
		t.Fatalf("release handoff omitted required sections: %+v", handoff)
	}
	assertReleaseHandoffReadFirst(t, handoff, "docs/context-routing.md")
	assertReleaseHandoffReadFirst(t, handoff, "docs/batch-plan.md")
	assertReleaseHandoffReadFirst(t, handoff, "docs/release-readiness.md")
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
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "profileSummary total=20 readOnly=5 mutating=15 writesCase=14 writesKit=1 reviewFirst=3 applyRequired=13 heavyTool=0 authorityConfirmed=0")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "profileGroups readOnly=doctor,packs,release-check,status,validate reviewFirst=promote,sync,update writesKit=promote")
	assertReleaseHandoffSignalDetail(t, handoff, "Go-native public surface", "profileBoundaries rows=7 caseLocalApply=attach,bootstrap,continue,gate,handoff,init,reconcile,repair,start caseLocalReviewWriteback=plan-subagents caseLocalReviewFirst=sync,update kitReviewFirst=promote readOnly=doctor,packs,release-check,status,validate")
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
	assertReleaseHandoffKnownGap(t, handoff, "ci-release-gate")
	assertReleaseHandoffKnownGap(t, handoff, "cross-platform-product-path")
	assertReleaseHandoffKnownGap(t, handoff, "session-orchestration")
	assertReleaseHandoffKnownGap(t, handoff, "dispatch")
	assertReleaseHandoffKnownGap(t, handoff, "heavy-tool")
	assertReleaseHandoffKnownGap(t, handoff, "authority")
	assertReleaseHandoffKnownGap(t, handoff, "pack-memory")
	assertReleaseHandoffKnownGap(t, handoff, "policy-schema")
	assertReleaseHandoffKnownGap(t, handoff, "powershell-deprecation")
	if handoff.ReleaseNotes.Path != "CHANGELOG.md" || !handoff.ReleaseNotes.Present || handoff.ReleaseNotes.Section != "Unreleased" || handoff.ReleaseNotes.LatestBatchID != handoff.LatestBatch.BatchID || !handoff.ReleaseNotes.Covered || handoff.ReleaseNotes.Summary != "release notes cover latest batch" {
		t.Fatalf("unexpected release handoff release notes: %+v", handoff.ReleaseNotes)
	}
	if handoff.LatestBatch.PlanPath != "docs/batch-plan.md" || !handoff.LatestBatch.Present || !strings.Contains(handoff.LatestBatch.Title, "Batch ") || !strings.Contains(handoff.LatestBatch.Status, "已完成") || strings.TrimSpace(handoff.LatestBatch.Goal) == "" || strings.TrimSpace(handoff.LatestBatch.ValidationResult) == "" {
		t.Fatalf("unexpected release handoff latest batch: %+v", handoff.LatestBatch)
	}
	latestDetail := handoff.LatestBatch.Handoff.RemoteReleaseGateDetail
	if latestDetail == nil || latestDetail.State != handoff.LatestBatch.Handoff.RemoteReleaseGate || len(latestDetail.Boundary) == 0 {
		t.Fatalf("release handoff latest batch omitted remote gate detail: %+v", handoff.LatestBatch.Handoff)
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
	if profileSummaryCounts.Total != 20 || surfaceCounts.ProfileTotal != 20 || profileSummaryCounts.ReadOnly != 5 || profileSummaryCounts.Mutating != 15 || profileSummaryCounts.WritesCase != 14 || profileSummaryCounts.WritesKit != 1 || profileSummaryCounts.ReviewFirst != 3 || profileSummaryCounts.ApplyRequired != 13 || profileSummaryCounts.HeavyTool != 0 || profileSummaryCounts.AuthorityConfirmed != 0 || profileSummaryCounts.BoundaryReadOnly != 5 || profileSummaryCounts.BoundaryCaseLocalApply != 9 || profileSummaryCounts.BoundaryCaseLocalWriteback != 1 || profileSummaryCounts.BoundaryCaseLocalReview != 2 || profileSummaryCounts.BoundaryKitReview != 1 {
		t.Fatalf("Go-native public command profile summary drifted: %+v", surface.CommandProfileSummary)
	}
	if strings.Join(surface.CommandProfileGroups.ReadOnly, ",") != "doctor,packs,release-check,status,validate" || strings.Join(surface.CommandProfileGroups.ReviewFirst, ",") != "promote,sync,update" || strings.Join(surface.CommandProfileGroups.WritesKit, ",") != "promote" || surfaceCounts.Groups.HeavyTool != 0 || surfaceCounts.Groups.AuthorityConfirmed != 0 || surfaceCounts.Groups.CaseLocalApply != 9 || surfaceCounts.Groups.CaseLocalReviewWriteback != 1 || surfaceCounts.Groups.CaseLocalReviewFirst != 2 {
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
	if counts.Documents != 15 || counts.RequiredPhrases == 0 || counts.Boundaries == 0 {
		t.Fatalf("public default docs readiness omitted required sections: %+v", docs)
	}
	assertPublicDefaultDoc(t, docs, "README.md")
	assertPublicDefaultDoc(t, docs, ".claude/skills/rekit/SKILL.md")
	assertPublicDefaultDoc(t, docs, "CLAUDE.md")
	assertPublicDefaultDoc(t, docs, "docs/context-routing.md")
	assertPublicDefaultDoc(t, docs, "docs/batch-plan.md")
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
	assertPublicDefaultPhrase(t, docs, "docs/context-routing.md", "按需路由")
	assertPublicDefaultPhrase(t, docs, "docs/batch-plan.md", "完整历史已拆到 `docs/batch-history.md`")
	assertPublicDefaultPhrase(t, docs, "CLAUDE.md", "PowerShell-free default/product path、Go-native、跨平台")
	assertPublicDefaultPhrase(t, docs, "docs/autonomous-goal.md", "默认继续自主推进")
	assertPublicDefaultPhrase(t, docs, "docs/release-readiness.md", "默认本机验证路径不依赖 PowerShell")
	assertPublicDefaultPhrase(t, docs, "docs/go-first-convergence-plan.md", "不要把大型 PowerShell matrix 作为默认必跑")
	assertPublicDefaultPhrase(t, docs, "docs/go-runtime-migration.md", "当前默认验证应优先运行 Go-native release gate")
	assertPublicDefaultPhrase(t, docs, "docs/powershell-deprecation.md", "Go CLI/backend 是 canonical runtime")
	assertPublicDefaultPhrase(t, docs, "docs/vision.md", "优先运行 Go-native 检查")
	assertPublicDefaultPhrase(t, docs, "docs/reference-absorption.md", "Go-native release readiness 子集")
	assertPublicDefaultPhrase(t, docs, "docs/agent-team-rollout-plan.md", "公共 `/rekit` 默认路径已由 20 个 Go-owned/no-fallback public commands 支撑")
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
		"docs/context-routing.md",
		"docs/batch-plan.md",
		"docs/release-readiness.md",
		"docs/mission-control-product-direction.md",
		"docs/autonomous-goal.md",
		"packs:",
		"heavy-tool gate actions: debug,dump,full-trace,inject,network,patch,symex",
		"PowerShell deprecation: PowerShell deprecation inventory ok ready=true",
		"commands=14 modules=14 freezeGates=10 blocked=5 fallbackRetirement=true noFallback=20 candidates=0 removalModules=0 retiredModules=13 facadeRuntime=true legacyImports=false dispatcher=false publicFacade=true retained=true facadeCommands=20 noFallback=20 moduleRemoval=true removalCandidates=0 retired=13 facadeDeps=0 undocumented=0 moduleReferences=true activeTests=0 fixtures=0 blockers=0 unclassified=0",
		"Go-native public surface: Go-native public command surface inventory ok ready=true entrypoint=cmd/rekit present=true catalog=internal/rekit/commands/commands.go catalogPresent=true default=status commands=20 handlers=20 symbols=20 profiles=20 boundaries=7 boundaryRows=7 policyRows=5 policyViolations=0 facadeRemovalReady=true facadePrerequisites=5 readOnly=5 mutating=15 writesCase=14 writesKit=1 reviewFirst=3 applyRequired=13 heavyTool=0 authorityConfirmed=0 readOnlyCommands=doctor,packs,release-check,status,validate reviewFirstCommands=promote,sync,update writesKitCommands=promote caseLocalApplyCommands=attach,bootstrap,continue,gate,handoff,init,reconcile,repair,start caseLocalReviewWritebackCommands=plan-subagents kitReviewFirstCommands=promote alternative=go run ./cmd/rekit -- -Command <command> unsupportedDiagnostic=true",
		"case shim: case shim readiness ok ready=true",
		"public default docs: public default docs readiness ok ready=true documents=15",
		"public facade removal: public facade removal prerequisites ok ready=true prerequisites=8 removalPlan=true planChecks=9 replacementEntrypoints=4 replacementValidationCommands=32 deletionGates=5 deletionGateValidationCommands=40 deletionGateExitCriteria=15 deletionGateFailureSignals=15 deletionGateEscalationTriggers=15 deletionGateEscalationEvidence=15 deletionGateEscalationRecipients=15 deletionGateEscalationHandoffSteps=15 deletionGateEscalationDecisionOptions=15 deletionGateEscalationRetryConditions=15 deletionGateEscalationStopConditions=15 deletionGateEscalationResolutionArtifacts=15 deletionGateEscalationClosureChecks=15 deletionGateEscalationReopenConditions=15 deletionGateEscalationLedgerEvents=15 deletionGateEscalationStateTransitions=15 deletionGateEscalationBoundaryGuards=15 deletionGateEscalationAuditChecks=15 deletionGateVerificationArtifacts=15 deletionGateBlockedExecutionSteps=10 deletionGateRemediationActions=15 recoverySteps=4 recoveryValidationCommands=32 documentationTargets=9 documentationValidationCommands=72 executionSteps=5 executionFailureSignals=15 executionRemediationActions=15 executionVerificationArtifacts=15 executionLedgerEvents=15 executionStateTransitions=15 executionEscalationTriggers=15 executionEscalationEvidence=15 executionEscalationRecipients=15 executionEscalationHandoffSteps=15 executionEscalationDecisionOptions=15 executionEscalationRetryConditions=15 executionEscalationStopConditions=15 executionEscalationResolutionArtifacts=15 executionEscalationClosureChecks=15 executionEscalationReopenConditions=15 executionEscalationLedgerEvents=15 executionEscalationStateTransitions=15 executionEscalationBoundaryGuards=15 executionEscalationAuditChecks=15 executionBoundaryGuards=15 executionAuditChecks=15 executionValidationCommands=40 boundaryChecks=6 boundaryValidationCommands=48 removalImpact=true impactReferences=",
		"workItems=",
		"validationCommands=",
		"migrationTargets=74 migrationValidationCommands=592",
		"smokeMigrationTargets=29 smokeMigrationValidationCommands=232",
		"release handoff: release handoff summary ok ready=true readFirst=4 signals=13 knownGaps=5 packMaturity=10 packMemoryCandidates=0",
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
	for _, expected := range []string{"release-check：mutation=false ready=false", "warnings=1", "release-check warning：CI workflow missing required command"} {
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

	out.Reset()
	if err := Run([]string{"-Command", "packs", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"packs：mutation=false count=10",
		"packs pack：id=_template name=_template maturity=template schema=ok manifestSchema=1 managed=4 template=1 local=3 promote=4 tooling=2 prompts=0 routes=2 heavyToolGates=7 authority=main version=0.1.0",
		"packs pack：id=vmp-re name=vmp-re maturity=mature schema=ok manifestSchema=1 managed=7 template=1 local=3 promote=7 tooling=12 prompts=4 routes=2 heavyToolGates=7 authority=devirt-main version=0.2.0",
		"packs pack heavy action：id=vmp-re action=debug",
		"packs pack heavy action：id=vmp-re action=symex",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("packs text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("packs text should not emit JSON:\n%s", out.String())
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

	out.Reset()
	if err := Run([]string{"-Command", "attach", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo-case", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"attach plan：mutation=false reviewRequired=true requiresConfirmation=true writes=4 blocked=4",
		"pack=_template projectName=demo-case",
		"attach plan write：path=",
		"kind=instance-metadata action=create",
		"kind=case-local-thin-shim action=create",
		"attach plan blocked action：managed docs sync",
		"attach plan next step：review this plan, then re-run attach with -Apply to write metadata and the thin shim",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("attach preview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("attach preview text should not emit JSON:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "instance.yml")); !os.IsNotExist(err) {
		t.Fatalf("attach text -WhatIf created instance metadata, err=%v", err)
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

	textCaseRoot := filepath.Join(t.TempDir(), "case-text")
	out.Reset()
	if err := Run([]string{"-Command", "attach", "-Target", textCaseRoot, "-Pack", "_template", "-ProjectName", "demo-case", "-Apply", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"attach apply：mutation=true applied=true writes=4",
		"pack=_template projectName=demo-case",
		"attach apply write：path=",
		"kind=instance-metadata action=create",
		"kind=initial-state action=create",
		"attach apply next step：attach wrote only case binding metadata, initial state, and the case-local thin shim",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("attach apply text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("attach apply text should not emit JSON:\n%s", out.String())
	}
	assertFileExists(t, filepath.Join(textCaseRoot, ".rekit", "instance.yml"))
	assertFileExists(t, filepath.Join(textCaseRoot, ".rekit", "state.json"))
	assertFileExists(t, filepath.Join(textCaseRoot, ".claude", "skills", "rekit", "SKILL.md"))

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

func TestRunInstalledCaseShimProductPathStatusAndRefresh(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "kit")
	root := repoRoot(t)
	candidateRoot := filepath.Join(root, "packs", "_template", "promote-candidates")
	toolingRoot := filepath.Join(root, "packs", "_template", "tooling", "candidates")
	candidateBefore := snapshotFiles(t, candidateRoot)
	toolingBefore := snapshotFiles(t, toolingRoot)
	t.Cleanup(func() {
		removeNewFiles(t, candidateRoot, candidateBefore)
		removeNewFiles(t, toolingRoot, toolingBefore)
	})
	var out bytes.Buffer
	if err := Run([]string{"-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "installed-entrypoint", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(caseRoot, "workspace", "main", "main")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	status := decodeInstalledCaseShimStatus(t, out.Bytes())
	if status.Mode != "case" || status.TargetProvided || status.Target != caseRoot || status.Case.CaseRoot != caseRoot || status.Case.ProjectName != "installed-entrypoint" || !status.Case.ShimMatchesTemplate || !status.CaseShim.Ready || status.CaseShim.InstalledShimMatchesTemplate == nil || !*status.CaseShim.InstalledShimMatchesTemplate || status.CaseShim.Entrypoint == nil || status.CaseShim.Entrypoint.CaseLocalFirstScreenCommand != "/rekit" || !strings.Contains(status.CaseShim.Entrypoint.ExplicitFirstScreenCommand, caseRoot) || !slices.Contains(status.CaseShim.Entrypoint.MetadataPaths, ".rekit/instance.yml") || !containsSubstring(status.CaseShim.Entrypoint.DurableArtifacts, ".rekit/lanes/<lane>/prompts/RESUME.md") || !containsSubstring(status.CaseShim.Entrypoint.FirstScreenChecks, "status case mission queue/current action") || !containsSubstring(status.CaseShim.Entrypoint.Boundary, "canonical skill") {
		t.Fatalf("unexpected installed case shim status: %+v", status)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"pack source: case-metadata", "case shim: case shim readiness ok ready=true", "status case shim entrypoint: caseLocal=/rekit", "status case shim durable artifact: .rekit/handovers/<lane>-latest.md", "status case shim first-screen check: status case mission queue/current action", "status case mission handoff：preview=/rekit handoff"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("installed entrypoint default status missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("installed entrypoint default status leaked JSON:\n%s", out.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.Contains(line, "status case shim") && (strings.Contains(line, "rekit.ps1") || strings.Contains(line, "go run") || strings.Contains(line, "PowerShell")) {
			t.Fatalf("installed entrypoint shim handoff leaked low-level entrypoint detail: %s\n%s", line, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Name", "login", "-Apply", "-Executor", "installed-session", "-Actor", "mission-commander", "-Reason", "installed entrypoint replacement executor handoff"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if start.Lane.ID != "feature-login" || start.Lane.CurrentExecutor != "installed-session" || start.Lane.ExecutorGeneration != 1 {
		t.Fatalf("unexpected installed entrypoint start result: %+v", start)
	}
	writeCaseFile(t, caseRoot, "workspace/features/feature-login/review-evidence.md", "bounded installed-entrypoint reviewer evidence\n")

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Lane", "feature-login"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if plan.Pack != "_template" || plan.ReviewerOrchestration.TargetLane != "feature-login" || plan.ReviewerOrchestration.ReviewerCount != 2 || len(packet.ShardHandoffs) != 2 {
		t.Fatalf("unexpected installed entrypoint reviewer plan: result=%+v packet=%+v", plan, packet)
	}

	firstHandoff := packet.ShardHandoffs[0]
	if err := os.WriteFile(firstHandoff.ReviewerResultPath, reviewerResultForCLIPlan(t, packet, firstHandoff, "accept", "accepted", "installed-reviewer-session-1"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", firstHandoff.ReviewerResultPath, "-Lane", "feature-login", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := decodeReviewerIntakeResult(t, out.Bytes())
	if preview.Mode != "reviewer-intake" || preview.IsMutation || preview.Applied || !preview.ReadyForWriteback || preview.Summary.OrchestrationProgress == nil || preview.Summary.OrchestrationProgress.NextOpenShardID != "shard-01" {
		t.Fatalf("unexpected installed entrypoint reviewer intake preview: %+v", preview)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", firstHandoff.ReviewerResultPath, "-Lane", "feature-login", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	applied := decodeReviewerIntakeResult(t, out.Bytes())
	if !applied.IsMutation || !applied.Applied || applied.WritebackStatus != "complete" || applied.Summary.ReviewerWritebackSummary == nil || applied.Summary.ReviewerWritebackSummary.Total != 2 || applied.Summary.OrchestrationProgress == nil || applied.Summary.OrchestrationProgress.NextOpenShardID != "shard-02" {
		t.Fatalf("unexpected installed entrypoint reviewer intake apply: %+v", applied)
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Apply", "login", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var continueApply struct {
		RunID                         string                               `json:"runId"`
		Applied                       bool                                 `json:"applied"`
		Blocked                       bool                                 `json:"blocked"`
		ReviewerWritebackSummary      reviewerWritebackSummaryCLIItem      `json:"reviewerWritebackSummary"`
		ReviewerDispatchIntakeSummary reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		Writes                        []startWrite                         `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &continueApply); err != nil {
		t.Fatalf("installed entrypoint continue apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if continueApply.RunID != "run-preview" || continueApply.Applied || !continueApply.Blocked || continueApply.ReviewerWritebackSummary.Total != 2 || continueApply.ReviewerWritebackSummary.LatestShardID != "shard-01" || continueApply.ReviewerDispatchIntakeSummary.Total != 1 || continueApply.ReviewerDispatchIntakeSummary.LatestShardID != "shard-02" || len(continueApply.Writes) != 0 {
		t.Fatalf("installed entrypoint continue apply did not fail closed on remaining reviewer work: %+v", continueApply)
	}

	lastHandoff := packet.ShardHandoffs[1]
	if err := os.WriteFile(lastHandoff.ReviewerResultPath, reviewerResultForCLIPlan(t, packet, lastHandoff, "accept", "accepted", "reviewer-session-installed-2"), 0o644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", lastHandoff.ReviewerResultPath, "-Lane", "feature-login", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", lastHandoff.ReviewerResultPath, "-Lane", "feature-login", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Apply", "login", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &continueApply); err != nil {
		t.Fatalf("installed entrypoint continue apply after reviewer completion is not JSON: %v\n%s", err, out.String())
	}
	if !continueApply.Applied || continueApply.Blocked || continueApply.RunID == "" || continueApply.RunID == "run-preview" {
		t.Fatalf("installed entrypoint continue did not resume after reviewer completion: %+v", continueApply)
	}
	resumePath := assertStartWrite(t, continueApply.Writes, ".rekit/lanes/feature-login/prompts/RESUME.md", "refresh").TargetPath
	checkpointPath := assertStartWrite(t, continueApply.Writes, ".rekit/lanes/feature-login/checkpoints/latest.json", "refresh").TargetPath
	digestPath := assertStartWrite(t, continueApply.Writes, ".rekit/runs/"+continueApply.RunID+"/digest.md", "write").TargetPath
	resume, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatal(err)
	}
	for label, text := range map[string]string{"lane RESUME": string(resume), "continue digest": string(digest)} {
		for _, expected := range []string{"## Reviewer writeback", "summary: total=`4` verifications=`2` decisions=`2`", "latestShard=`shard-02`", "reviewer-session-installed-2", "## Reviewer dispatch intake handoff", "- none"} {
			if !strings.Contains(text, expected) {
				t.Fatalf("installed entrypoint %s omitted reviewer durable handoff %q:\n%s", label, expected, text)
			}
		}
	}
	checkpointBytes, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		ReviewerWritebackSummary      reviewerWritebackSummaryCLIItem      `json:"reviewerWritebackSummary"`
		ReviewerDispatchIntakeSummary reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("installed entrypoint checkpoint did not decode: %v\n%s", err, string(checkpointBytes))
	}
	if checkpoint.ReviewerWritebackSummary.Total != 4 || checkpoint.ReviewerDispatchIntakeSummary.Total != 0 {
		t.Fatalf("installed entrypoint checkpoint omitted completed reviewer handoff: %+v", checkpoint)
	}

	writeCaseFile(t, caseRoot, "references/template/README.md", "# Installed entrypoint candidate\n\nReusable safe installed-entrypoint pack-memory update.\n")
	writeCaseFile(t, caseRoot, "references/template/workflow-template.md", "# Blocked\n\nDo not promote C:\\case\\artifact\\installed-entrypoint-trace.csv.\n")
	writeCaseFile(t, caseRoot, "references/template/toolchain-router.md", "# Tooling\n\nCase root: "+caseRoot+"\nAbsolute: C:\\cases\\installed.exe\nTrace: artifacts/run/installed-trace.csv\nAddress: 0x401000\nContext: ctx123 round7 Task #99\n")
	out.Reset()
	if err := Run([]string{"-Command", "promote", "-CreateCandidates", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	candidates := decodeCandidateResult(t, out.Bytes())
	if candidates.CaseRoot != caseRoot || candidates.Pack != "_template" || !candidates.Applied || candidates.Created != 2 || candidates.Blocked == 0 || !candidates.RequiresCleanup || candidates.ReviewPlan.ReviewSummary.ProofSummary.NextMissingProof == nil {
		t.Fatalf("unexpected installed entrypoint pack-memory candidates: %+v", candidates)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	status = decodeInstalledCaseShimStatus(t, out.Bytes())
	if status.CaseMission.ReviewerWritebackSummary.Total != 4 || status.CaseMission.ReviewerDispatchIntakeSummary.Total != 0 || status.ProjectHandoff == nil || status.ProjectHandoff.PackMemoryCandidates.Ready || status.ProjectHandoff.PackMemoryCandidates.Total != 3 || len(status.ProjectHandoff.PackMemoryCandidates.Packs) != 1 || status.ProjectHandoff.PackMemoryCandidates.Packs[0].ReviewSummary.ProofSummary.NextMissingProof == nil {
		t.Fatalf("installed entrypoint first-screen JSON omitted reviewer/pack-memory handoff: %+v", status)
	}

	out.Reset()
	if err := Run(nil, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"pack source: case-metadata",
		"status case shim entrypoint: caseLocal=/rekit",
		"status case shim first-screen check: reviewer writeback or reviewer dispatch intake summaries show reviewer state without reopening packet/result JSON",
		"status case shim first-screen check: project handoff pack-memory candidate summary shows candidate review/cleanup/reconsume proof state",
		"status case mission reviewer writeback summary：total=4 verifications=2 decisions=2 lanes=1 latestKind=decision",
		"latestShard=shard-02",
		"latestReviewerSession=reviewer-session-installed-2",
		"status pack-memory candidates：summary=pack-memory candidate inventory has open review/cleanup/verification work ready=false total=3 packs=1 nextAction=review listed pack-memory candidates or complete listed candidate decision verification",
		"status pack-memory review summary：pack=_template total=3 candidateFiles=1 toolingFiles=1 indexEntries=1 reviewArtifacts=8 decisionArtifacts=2 cleanupArtifacts=2 reconsumeArtifacts=4",
		"status pack-memory next missing proof：pack=_template stage=decision-proof-required proofType=candidate-decision-note",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("installed entrypoint first screen missing product handoff %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("installed entrypoint product first screen leaked JSON:\n%s", out.String())
	}

	writeCaseFile(t, caseRoot, ".claude/skills/rekit/SKILL.md", "drift\n")
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	drift := decodeInstalledCaseShimStatus(t, out.Bytes())
	if drift.Case.ShimMatchesTemplate || drift.CaseShim.Ready || drift.CaseShim.InstalledShimMatchesTemplate == nil || *drift.CaseShim.InstalledShimMatchesTemplate || len(drift.CaseShim.Warnings) == 0 {
		t.Fatalf("unexpected drift case shim status: %+v", drift)
	}

	out.Reset()
	if err := Run([]string{"-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "installed-entrypoint", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	refreshed := decodeInstalledCaseShimStatus(t, out.Bytes())
	if !refreshed.Case.ShimMatchesTemplate || !refreshed.CaseShim.Ready || refreshed.CaseShim.InstalledShimMatchesTemplate == nil || !*refreshed.CaseShim.InstalledShimMatchesTemplate {
		t.Fatalf("unexpected refreshed case shim status: %+v", refreshed)
	}

	out.Reset()
	if err := Run([]string{"-Command", "doctor", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var doctor struct {
		Command string `json:"command"`
		Mode    string `json:"mode"`
		Valid   bool   `json:"valid"`
	}
	if err := json.Unmarshal(out.Bytes(), &doctor); err != nil {
		t.Fatalf("installed product-path doctor stdout is not JSON: %v\n%s", err, out.String())
	}
	if doctor.Command != "doctor" || doctor.Mode != "case" || !doctor.Valid {
		t.Fatalf("unexpected installed product-path doctor: %+v", doctor)
	}
}

type installedCaseShimStatus struct {
	Target         string `json:"target"`
	TargetProvided bool   `json:"targetProvided"`
	Mode           string `json:"mode"`
	Case           struct {
		CaseRoot            string `json:"caseRoot"`
		ProjectName         string `json:"projectName"`
		ShimMatchesTemplate bool   `json:"shimMatchesTemplate"`
	} `json:"case"`
	CaseShim struct {
		Ready                        bool     `json:"ready"`
		InstalledShimMatchesTemplate *bool    `json:"installedShimMatchesTemplate"`
		Warnings                     []string `json:"warnings"`
		Entrypoint                   *struct {
			CaseLocalFirstScreenCommand string   `json:"caseLocalFirstScreenCommand"`
			ExplicitFirstScreenCommand  string   `json:"explicitFirstScreenCommand"`
			InstalledShimPath           string   `json:"installedShimPath"`
			CanonicalSkillPath          string   `json:"canonicalSkillPath"`
			MetadataPaths               []string `json:"metadataPaths"`
			DurableArtifacts            []string `json:"durableArtifacts"`
			FirstScreenChecks           []string `json:"firstScreenChecks"`
			Boundary                    []string `json:"boundary"`
		} `json:"entrypoint"`
		NextSteps []string `json:"nextSteps"`
	} `json:"caseShim"`
	CaseMission struct {
		ReviewerWritebackSummary      reviewerWritebackSummaryCLIItem      `json:"reviewerWritebackSummary"`
		ReviewerDispatchIntakeSummary reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
	} `json:"caseMission"`
	ProjectHandoff *struct {
		PackMemoryCandidates struct {
			Ready bool `json:"ready"`
			Total int  `json:"total"`
			Packs []struct {
				ReviewSummary packMemoryCandidateReviewSummaryJSON `json:"reviewSummary"`
			} `json:"packs"`
		} `json:"packMemoryCandidates"`
	} `json:"projectHandoff"`
}

func decodeInstalledCaseShimStatus(t *testing.T, data []byte) installedCaseShimStatus {
	t.Helper()
	var status installedCaseShimStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("installed case shim status stdout is not JSON: %v\n%s", err, string(data))
	}
	return status
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

func TestRunAttachRejectsMovedCaseWithPreviewFirstDiagnostic(t *testing.T) {
	caseRoot := movedAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "attach", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for attach on moved case")
	}
	assertMovedCaseRepairPreviewDiagnostic(t, err, caseRoot, "_template")
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

	out.Reset()
	if err := Run([]string{"-Command", "repair", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"repair plan：mutation=false reviewRequired=true requiresConfirmation=true moved=true",
		"writes=4 blocked=4",
		"pack=_template projectName=demo",
		"repair plan write：path=",
		"kind=legacy-metadata action=create",
		"repair plan blocked action：managed docs sync",
		"repair plan next step：review this plan, then re-run repair with -Apply to refresh metadata and the thin shim",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("repair preview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("repair preview text should not emit JSON:\n%s", out.String())
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

	textCaseRoot := movedAttachedCase(t)
	writeCaseFile(t, textCaseRoot, ".claude/skills/rekit/SKILL.md", "drift\n")
	out.Reset()
	if err := Run([]string{"-Command", "repair", "-Target", textCaseRoot, "-Pack", "_template", "-Apply", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"repair apply：mutation=true applied=true moved=true",
		"writes=4",
		"pack=_template projectName=demo",
		"repair apply write：path=",
		"kind=case-local-thin-shim action=refresh",
		"kind=legacy-metadata action=create",
		"repair apply next step：repair refreshed metadata and the case-local thin shim",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("repair apply text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("repair apply text should not emit JSON:\n%s", out.String())
	}
	assertFileExists(t, filepath.Join(textCaseRoot, ".re-template.yml"))

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

	out.Reset()
	if err := Run([]string{"-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "demo-init", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"init plan：mutation=false reviewRequired=true requiresConfirmation=true writes=",
		"blocked=5",
		"pack=_template projectName=demo-init",
		"init plan write：path=.rekit/instance.yml kind=instance-metadata action=create",
		"init plan write：path=references/template/README.md kind=managed-file action=create-managed-file",
		"init plan blocked action：heavy-tool execution",
		"init plan next step：review this plan, then re-run init with -Apply to initialize the case",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("init preview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("init preview text should not emit JSON:\n%s", out.String())
	}
	if _, err := os.Stat(caseRoot); !os.IsNotExist(err) {
		t.Fatalf("init text -WhatIf created target directory, err=%v", err)
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

	textCaseRoot := filepath.Join(t.TempDir(), "case-text")
	out.Reset()
	if err := Run([]string{"-Command", "init", "-Target", textCaseRoot, "-Pack", "_template", "-ProjectName", "demo-init", "-Apply", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"init apply：mutation=true applied=true writes=",
		"init apply write：path=.rekit/instance.yml kind=instance-metadata action=refresh",
		"init apply write：path=references/template/README.md kind=managed-file action=create-managed-file",
		"init apply write：path=CLAUDE.local.md kind=managed-block action=create-managed-block-host",
		"init apply next step：run doctor after apply",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("init apply text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("init apply text should not emit JSON:\n%s", out.String())
	}
	assertFileExists(t, filepath.Join(textCaseRoot, ".rekit", "instance.yml"))
	assertFileExists(t, filepath.Join(textCaseRoot, "references", "template", "README.md"))
}

func TestRunCaseLocalProductPathUsesCaseMetadataRuntime(t *testing.T) {
	root := repoRoot(t)
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "init", "-Target", caseRoot, "-Pack", "_template", "-ProjectName", "product-path", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(caseRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Command        string `json:"command"`
		TemplateRoot   string `json:"templateRoot"`
		Pack           string `json:"pack"`
		PackSource     string `json:"packSource"`
		Target         string `json:"target"`
		TargetProvided bool   `json:"targetProvided"`
		Mode           string `json:"mode"`
		Case           struct {
			TemplateRoot        string   `json:"templateRoot"`
			TemplatePack        string   `json:"templatePack"`
			PackMatchesMetadata bool     `json:"packMatchesMetadata"`
			PackDiagnostic      string   `json:"packDiagnostic"`
			NextSteps           []string `json:"nextSteps"`
			ProjectName         string   `json:"projectName"`
		} `json:"case"`
		CaseMission struct {
			OpenDecisionHandoffs []statusOpenDecisionHandoff `json:"openDecisionHandoffs"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("case-local status stdout is not JSON: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.Mode != "case" || status.Pack != "_template" || status.PackSource != "explicit" || status.TargetProvided || status.Target != caseRoot || status.TemplateRoot != root || status.Case.TemplateRoot != root || status.Case.TemplatePack != "_template" || status.Case.ProjectName != "product-path" {
		t.Fatalf("unexpected case-local status: %+v", status)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "generic-binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("case-local mismatched-pack status stdout is not JSON: %v\n%s", err, out.String())
	}
	if status.Pack != "generic-binary-re" || status.PackSource != "explicit" || status.Case.TemplatePack != "_template" || status.Case.PackMatchesMetadata || !strings.Contains(status.Case.PackDiagnostic, `explicit pack "generic-binary-re" differs from case metadata templatePack "_template"`) || !containsSubstring(status.Case.NextSteps, `/rekit status -Target "`+caseRoot+`" -Format text uses case metadata templatePack "_template"; keep explicit -Pack "generic-binary-re" only after confirming the override`) {
		t.Fatalf("unexpected case-local mismatched-pack status: %+v", status)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "generic-binary-re", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status：mutation=false mode=case targetProvided=false pack=generic-binary-re packSource=explicit",
		"templatePack=_template packMatchesMetadata=false",
		`status case pack diagnostic：explicit pack "generic-binary-re" differs from case metadata templatePack "_template"`,
		`status case next step：/rekit status -Target "` + caseRoot + `" -Format text uses case metadata templatePack "_template"; keep explicit -Pack "generic-binary-re" only after confirming the override`,
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("case-local mismatched-pack status text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("case-local mismatched-pack status text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "doctor", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var doctor struct {
		Command string `json:"command"`
		Mode    string `json:"mode"`
		Valid   bool   `json:"valid"`
	}
	if err := json.Unmarshal(out.Bytes(), &doctor); err != nil {
		t.Fatalf("case-local doctor stdout is not JSON: %v\n%s", err, out.String())
	}
	if doctor.Command != "doctor" || doctor.Mode != "case" || !doctor.Valid {
		t.Fatalf("unexpected case-local doctor: %+v", doctor)
	}

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Pack", "_template", "-Name", "login", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if !start.IsMutation || !start.Applied || start.Lane.ID != "feature-login" || start.Lane.Workspace != "workspace/features/feature-login" {
		t.Fatalf("unexpected case-local start result: %+v", start)
	}

	writeCaseFile(t, caseRoot, "workspace/features/feature-login/packet.md", "# packet\n\ncase-local product path packet\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/outbox.jsonl", strings.Join([]string{
		`{"eventId":"evt-product-path-observation","kind":"observation","subject":"product path observation","summary":"case-local cwd observed","evidence":"evidence-product-path-observation"}`,
		`{"eventId":"evt-product-path-request","kind":"request","subject":"product path request","summary":"route product path request to main","requestId":"req-product-path-main","targetLane":"main","evidence":"evidence-product-path-request","status":"open"}`,
	}, "\n")+"\n")

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	var cont struct {
		Command string    `json:"command"`
		Applied bool      `json:"applied"`
		Lane    startLane `json:"lane"`
		Writes  []startWrite
	}
	if err := json.Unmarshal(out.Bytes(), &cont); err != nil {
		t.Fatalf("case-local continue stdout is not JSON: %v\n%s", err, out.String())
	}
	if cont.Command != "continue" || !cont.Applied || cont.Lane.ID != "feature-login" {
		t.Fatalf("unexpected case-local continue result: %+v", cont)
	}
	assertContinueWrite(t, cont.Writes, ".rekit/facts/observations.jsonl", "append")
	assertContinueWrite(t, cont.Writes, ".rekit/facts/requests.jsonl", "append")
	assertContinueWrite(t, cont.Writes, ".rekit/lanes/main/tasks.jsonl", "append")

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Pack", "_template", "-Kind", "candidate", "-Lane", "feature-login", "-Subject", "nested source candidate", "-Summary", "candidate needs main review", "-Confidence", "high", "-Status", "open", "-TargetRef", "candidate-product-path", "-EvidenceRefs", "evidence/nested-candidate.json", "-EventId", "cand-product-path-1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var openCandidateNote struct {
		Command    string `json:"command"`
		Pack       string `json:"pack"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		EventID    string `json:"eventId"`
		Path       string `json:"path"`
	}
	if err := json.Unmarshal(out.Bytes(), &openCandidateNote); err != nil {
		t.Fatalf("nested case-local candidate note stdout is not JSON: %v\n%s", err, out.String())
	}
	if openCandidateNote.Command != "note" || openCandidateNote.Pack != "_template" || !openCandidateNote.IsMutation || !openCandidateNote.Applied || openCandidateNote.EventID != "cand-product-path-1" || openCandidateNote.Path != ".rekit/facts/candidates.jsonl" {
		t.Fatalf("unexpected nested case-local candidate note: %+v", openCandidateNote)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Pack", "_template", "-Kind", "decision", "-Lane", "feature-login", "-Subject", "nested deferred decision", "-Decision", "defer", "-Reason", "awaiting main review", "-Status", "open", "-TargetRef", "decision-product-path", "-EvidenceRefs", "evidence/nested-decision.json", "-EventId", "dec-product-path-1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var openDecisionNote struct {
		Command    string `json:"command"`
		Pack       string `json:"pack"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		EventID    string `json:"eventId"`
		Path       string `json:"path"`
	}
	if err := json.Unmarshal(out.Bytes(), &openDecisionNote); err != nil {
		t.Fatalf("nested case-local decision note stdout is not JSON: %v\n%s", err, out.String())
	}
	if openDecisionNote.Command != "note" || openDecisionNote.Pack != "_template" || !openDecisionNote.IsMutation || !openDecisionNote.Applied || openDecisionNote.EventID != "dec-product-path-1" || openDecisionNote.Path != ".rekit/facts/decisions.jsonl" {
		t.Fatalf("unexpected nested case-local decision note: %+v", openDecisionNote)
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	laneHandoff := decodeHandoffResult(t, out.Bytes())
	if !laneHandoff.IsMutation || !laneHandoff.Applied || laneHandoff.Project || laneHandoff.Lane == nil || laneHandoff.Lane.ID != "feature-login" {
		t.Fatalf("unexpected case-local lane handoff result: %+v", laneHandoff)
	}
	assertStartWrite(t, laneHandoff.Writes, ".rekit/handovers/feature-login-latest.md", "write-latest-lane-handoff")

	nested := filepath.Join(caseRoot, "workspace", "features", "feature-login")
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("nested case-local status stdout is not JSON: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.Mode != "case" || status.Pack != "_template" || status.PackSource != "explicit" || status.TargetProvided || status.Target != caseRoot || status.TemplateRoot != root || status.Case.TemplateRoot != root || status.Case.TemplatePack != "_template" {
		t.Fatalf("unexpected nested case-local status: %+v", status)
	}
	if len(status.CaseMission.OpenDecisionHandoffs) != 2 {
		t.Fatalf("nested case-local status missing open decision source handoffs: %+v", status.CaseMission.OpenDecisionHandoffs)
	}
	productCandidateHandoff := status.CaseMission.OpenDecisionHandoffs[0]
	if productCandidateHandoff.EventID != "cand-product-path-1" || productCandidateHandoff.Kind != "candidate" || productCandidateHandoff.Lane != "feature-login" || productCandidateHandoff.SourceKind != "candidate" || productCandidateHandoff.SourcePath != ".rekit/facts/candidates.jsonl" || productCandidateHandoff.RecordPath != ".rekit/facts/decisions.jsonl" || productCandidateHandoff.SourceCommand != `/rekit note -Target "`+caseRoot+`" -Pack _template -List -Kind candidate -Lane feature-login -Format json` || !strings.Contains(productCandidateHandoff.WhatIfCommand, `-Related "cand-product-path-1"`) || !strings.Contains(productCandidateHandoff.WhatIfCommand, `-EvidenceRefs "evidence/nested-candidate.json"`) || strings.Contains(productCandidateHandoff.RecordCommand, "-WhatIf") || !containsSubstring(productCandidateHandoff.Evidence, "sourcePath .rekit/facts/candidates.jsonl") || !containsSubstring(productCandidateHandoff.Evidence, "recordPath .rekit/facts/decisions.jsonl") {
		t.Fatalf("unexpected nested candidate source handoff: %+v", productCandidateHandoff)
	}
	productDecisionHandoff := status.CaseMission.OpenDecisionHandoffs[1]
	if productDecisionHandoff.EventID != "dec-product-path-1" || productDecisionHandoff.Kind != "decision" || productDecisionHandoff.Lane != "feature-login" || productDecisionHandoff.Decision != "defer" || productDecisionHandoff.SourceKind != "decision" || productDecisionHandoff.SourcePath != ".rekit/facts/decisions.jsonl" || productDecisionHandoff.RecordPath != ".rekit/facts/decisions.jsonl" || productDecisionHandoff.SourceCommand != `/rekit note -Target "`+caseRoot+`" -Pack _template -List -Kind decision -Lane feature-login -Format json` || !strings.Contains(productDecisionHandoff.WhatIfCommand, `-Related "dec-product-path-1"`) || !strings.Contains(productDecisionHandoff.WhatIfCommand, `-EvidenceRefs "evidence/nested-decision.json"`) || strings.Contains(productDecisionHandoff.RecordCommand, "-WhatIf") || !containsSubstring(productDecisionHandoff.Evidence, "sourcePath .rekit/facts/decisions.jsonl") || !containsSubstring(productDecisionHandoff.Evidence, "recordPath .rekit/facts/decisions.jsonl") {
		t.Fatalf("unexpected nested decision source handoff: %+v", productDecisionHandoff)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status：mutation=false mode=case targetProvided=false pack=_template packSource=explicit",
		"status case：root=" + caseRoot,
		"status case mission：summary=",
		"status case mission queue：total=",
		"status case mission queue action：bucket=current",
		"status case mission lane action：lane=feature-login",
		"status case mission facts：observations=1 requests=1 candidates=1",
		"status case mission section：name=openCandidates total=1 shown=1",
		"status case mission open decision handoff：eventId=cand-product-path-1 kind=candidate lane=feature-login subject=nested source candidate",
		"sourceKind=candidate sourcePath=.rekit/facts/candidates.jsonl recordPath=.rekit/facts/decisions.jsonl",
		"sourceCommand=/rekit note -Target \"" + caseRoot + "\" -Pack _template -List -Kind candidate -Lane feature-login -Format json whatIf=/rekit note",
		"status case mission open decision handoff：eventId=dec-product-path-1 kind=decision lane=feature-login subject=nested deferred decision",
		"sourceCommand=/rekit note -Target \"" + caseRoot + "\" -Pack _template -List -Kind decision -Lane feature-login -Format json whatIf=/rekit note",
		"status case mission open decision evidence：eventId=cand-product-path-1 evidence=sourcePath .rekit/facts/candidates.jsonl",
		"status case mission open decision evidence：eventId=dec-product-path-1 evidence=recordPath .rekit/facts/decisions.jsonl",
		"status case mission handoff：preview=/rekit handoff -Target",
		"continueBoundary=status is read-only; run continue with -WhatIf first",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested case-local status text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested case-local status text should not emit JSON object:\n%s", out.String())
	}

	defaultStatusExpected := []string{
		"rekit go backend:",
		"template root: " + root,
		"pack: _template",
		"case: " + caseRoot,
		"case templateRoot: " + root,
		"case templatePack: _template",
		"case pack matches metadata: true",
		"case pack diagnostic: pack matches case metadata templatePack",
		"status case mission：summary=",
		"status case mission queue：total=",
		"status case mission queue action：bucket=current",
		"status case mission lane action：lane=feature-login",
		"status case mission facts：observations=1 requests=1 candidates=1",
		"status case mission section：name=openCandidates total=1 shown=1",
		"status case mission open decision handoff：eventId=cand-product-path-1 kind=candidate lane=feature-login subject=nested source candidate",
		"sourceKind=candidate sourcePath=.rekit/facts/candidates.jsonl recordPath=.rekit/facts/decisions.jsonl",
		"sourceCommand=/rekit note -Target \"" + caseRoot + "\" -Pack _template -List -Kind candidate -Lane feature-login -Format json whatIf=/rekit note",
		"status case mission open decision handoff：eventId=dec-product-path-1 kind=decision lane=feature-login subject=nested deferred decision",
		"sourceCommand=/rekit note -Target \"" + caseRoot + "\" -Pack _template -List -Kind decision -Lane feature-login -Format json whatIf=/rekit note",
		"status case mission open decision evidence：eventId=cand-product-path-1 evidence=sourcePath .rekit/facts/candidates.jsonl",
		"status case mission open decision evidence：eventId=dec-product-path-1 evidence=recordPath .rekit/facts/decisions.jsonl",
		"status case mission handoff：preview=/rekit handoff -Target",
		"continueBoundary=status is read-only; run continue with -WhatIf first",
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range defaultStatusExpected {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested case-local default status missing %q:\n%s", expected, out.String())
		}
	}
	if !strings.Contains(out.String(), "pack source: explicit") {
		t.Fatalf("nested case-local explicit default status missing pack source:\n%s", out.String())
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested case-local default status should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range defaultStatusExpected {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested case-local no-command default status missing %q:\n%s", expected, out.String())
		}
	}
	if !strings.Contains(out.String(), "pack source: explicit") {
		t.Fatalf("nested case-local explicit no-command status missing pack source:\n%s", out.String())
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested case-local no-command default status should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("nested case-local no-pack status stdout is not JSON: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.Mode != "case" || status.Pack != "_template" || status.PackSource != "case-metadata" || status.TargetProvided || status.Target != caseRoot || status.Case.TemplatePack != "_template" {
		t.Fatalf("unexpected nested case-local no-pack status: %+v", status)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "status：mutation=false mode=case targetProvided=false pack=_template packSource=case-metadata") {
		t.Fatalf("nested case-local no-pack status text missing pack source:\n%s", out.String())
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested case-local no-pack status text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run(nil, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range defaultStatusExpected {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested case-local no-command no-pack status missing %q:\n%s", expected, out.String())
		}
	}
	if !strings.Contains(out.String(), "pack source: case-metadata") {
		t.Fatalf("nested case-local no-command no-pack status missing pack source:\n%s", out.String())
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested case-local no-command no-pack status should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "doctor", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackDoctor struct {
		Command string `json:"command"`
		Mode    string `json:"mode"`
		Pack    string `json:"pack"`
		Valid   bool   `json:"valid"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackDoctor); err != nil {
		t.Fatalf("nested case-local no-pack doctor stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackDoctor.Command != "doctor" || noPackDoctor.Mode != "case" || noPackDoctor.Pack != "_template" || !noPackDoctor.Valid {
		t.Fatalf("unexpected nested case-local no-pack doctor: %+v", noPackDoctor)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackOverview struct {
		Command  string `json:"command"`
		CaseRoot string `json:"caseRoot"`
		Pack     string `json:"pack"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackOverview); err != nil {
		t.Fatalf("nested case-local no-pack overview stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackOverview.Command != "overview" || noPackOverview.CaseRoot != caseRoot || noPackOverview.Pack != "_template" {
		t.Fatalf("unexpected nested case-local no-pack overview: %+v", noPackOverview)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"overview：mutation=false caseRoot=" + caseRoot,
		"pack=_template",
		"overview mission commander action queue：summary=total=",
		"overview next step：",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested case-local no-pack overview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested case-local no-pack overview text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-WhatIf", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	noPackHandoff := decodeHandoffResult(t, out.Bytes())
	if noPackHandoff.Command != "handoff" || noPackHandoff.Pack != "_template" || noPackHandoff.IsMutation || noPackHandoff.Applied || noPackHandoff.Project || noPackHandoff.Lane == nil || noPackHandoff.Lane.ID != "feature-login" {
		t.Fatalf("unexpected nested case-local no-pack handoff preview: %+v", noPackHandoff)
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-WhatIf", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackContinue struct {
		Command    string    `json:"command"`
		Pack       string    `json:"pack"`
		IsMutation bool      `json:"isMutation"`
		Applied    bool      `json:"applied"`
		Lane       startLane `json:"lane"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackContinue); err != nil {
		t.Fatalf("nested case-local no-pack continue stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackContinue.Command != "continue" || noPackContinue.Pack != "_template" || noPackContinue.IsMutation || noPackContinue.Applied || noPackContinue.Lane.ID != "feature-login" {
		t.Fatalf("unexpected nested case-local no-pack continue preview: %+v", noPackContinue)
	}

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-WhatIf", "-Action", "debug", "-Lane", "feature-login", "-Subject", "nested no-pack gate", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackGatePlan struct {
		Command    string `json:"command"`
		CaseRoot   string `json:"caseRoot"`
		Pack       string `json:"pack"`
		IsMutation bool   `json:"isMutation"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackGatePlan); err != nil {
		t.Fatalf("nested case-local no-pack gate stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackGatePlan.Command != "gate" || noPackGatePlan.CaseRoot != caseRoot || noPackGatePlan.Pack != "_template" || noPackGatePlan.IsMutation {
		t.Fatalf("unexpected nested case-local no-pack gate plan: %+v", noPackGatePlan)
	}

	out.Reset()
	noPackReviewRoot := filepath.Join(caseRoot, ".rekit", "reviews", "nested-product-path-no-pack")
	if err := Run([]string{"-Command", "plan-subagents", "-TaskType", "feature-analysis", "-Items", "gamma,delta", "-ReviewOutputDir", noPackReviewRoot}, &out); err != nil {
		t.Fatal(err)
	}
	noPackPlan := decodePlanSubagentsResult(t, out.Bytes())
	noPackPacket := decodePlanSubagentsPacket(t, noPackPlan.PacketPath)
	if noPackPlan.Command != "plan-subagents" || noPackPlan.ReviewRoot != noPackReviewRoot || noPackPlan.ItemCount != 2 || noPackPlan.ShardCount != 2 || !strings.HasPrefix(noPackPlan.PacketPath, caseRoot) || noPackPacket.Route.ID != "_template:lane-feature-analysis" {
		t.Fatalf("unexpected nested case-local no-pack plan-subagents result: result=%+v packet=%+v", noPackPlan, noPackPacket)
	}

	out.Reset()
	if err := Run([]string{"start", "review-no-pack", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackStart struct {
		Command    string    `json:"command"`
		Pack       string    `json:"pack"`
		IsMutation bool      `json:"isMutation"`
		Applied    bool      `json:"applied"`
		Lane       startLane `json:"lane"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackStart); err != nil {
		t.Fatalf("nested case-local no-pack start stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackStart.Command != "start" || noPackStart.Pack != "_template" || noPackStart.IsMutation || noPackStart.Applied || noPackStart.Lane.ID != "feature-review-no-pack" {
		t.Fatalf("unexpected nested case-local no-pack start preview: %+v", noPackStart)
	}

	out.Reset()
	if err := Run([]string{"-Command", "note", "-Kind", "observation", "-Lane", "feature-login", "-Subject", "no-pack note preview", "-Summary", "metadata pack preview", "-EvidenceRefs", "evidence-no-pack-note", "-EventId", "evt-no-pack-note-preview", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackNote struct {
		Command    string `json:"command"`
		Pack       string `json:"pack"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		EventID    string `json:"eventId"`
		Reason     string `json:"reason"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackNote); err != nil {
		t.Fatalf("nested case-local no-pack note stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackNote.Command != "note" || noPackNote.Pack != "_template" || noPackNote.IsMutation || noPackNote.Applied || noPackNote.EventID != "evt-no-pack-note-preview" || noPackNote.Reason != "what-if" {
		t.Fatalf("unexpected nested case-local no-pack note preview: %+v", noPackNote)
	}

	out.Reset()
	if err := Run([]string{"-Command", "sync", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackSync struct {
		Command   string `json:"command"`
		Direction string `json:"direction"`
		CaseRoot  string `json:"caseRoot"`
		Pack      string `json:"pack"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackSync); err != nil {
		t.Fatalf("nested case-local no-pack sync stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackSync.Command != "sync" || noPackSync.Direction != "kit-to-case" || noPackSync.CaseRoot != caseRoot || noPackSync.Pack != "_template" {
		t.Fatalf("unexpected nested case-local no-pack sync review: %+v", noPackSync)
	}

	out.Reset()
	if err := Run([]string{"-Command", "sync", "-Apply", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackSyncPreview struct {
		Command    string `json:"command"`
		Pack       string `json:"pack"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackSyncPreview); err != nil {
		t.Fatalf("nested case-local no-pack sync apply preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackSyncPreview.Command != "sync" || noPackSyncPreview.Pack != "_template" || noPackSyncPreview.IsMutation || noPackSyncPreview.Applied {
		t.Fatalf("unexpected nested case-local no-pack sync apply preview: %+v", noPackSyncPreview)
	}

	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackPromote struct {
		Command   string `json:"command"`
		Direction string `json:"direction"`
		CaseRoot  string `json:"caseRoot"`
		Pack      string `json:"pack"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackPromote); err != nil {
		t.Fatalf("nested case-local no-pack promote stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackPromote.Command != "promote" || noPackPromote.Direction != "case-to-kit" || noPackPromote.CaseRoot != caseRoot || noPackPromote.Pack != "_template" {
		t.Fatalf("unexpected nested case-local no-pack promote review: %+v", noPackPromote)
	}

	out.Reset()
	if err := Run([]string{"-Command", "promote", "-CreateCandidates", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var noPackPromotePreview struct {
		Command    string `json:"command"`
		Pack       string `json:"pack"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
	}
	if err := json.Unmarshal(out.Bytes(), &noPackPromotePreview); err != nil {
		t.Fatalf("nested case-local no-pack promote candidates preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if noPackPromotePreview.Command != "promote" || noPackPromotePreview.Pack != "_template" || noPackPromotePreview.IsMutation || noPackPromotePreview.Applied {
		t.Fatalf("unexpected nested case-local no-pack promote candidates preview: %+v", noPackPromotePreview)
	}

	out.Reset()
	if err := Run([]string{"-Command", "doctor", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &doctor); err != nil {
		t.Fatalf("nested case-local doctor stdout is not JSON: %v\n%s", err, out.String())
	}
	if doctor.Command != "doctor" || doctor.Mode != "case" || !doctor.Valid {
		t.Fatalf("unexpected nested case-local doctor: %+v", doctor)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var overview struct {
		Command  string `json:"command"`
		CaseRoot string `json:"caseRoot"`
		Pack     string `json:"pack"`
	}
	if err := json.Unmarshal(out.Bytes(), &overview); err != nil {
		t.Fatalf("nested case-local overview stdout is not JSON: %v\n%s", err, out.String())
	}
	if overview.Command != "overview" || overview.CaseRoot != caseRoot || overview.Pack != "_template" {
		t.Fatalf("unexpected nested case-local overview: %+v", overview)
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"overview：mutation=false caseRoot=" + caseRoot,
		"pack=_template",
		"overview lane：id=main",
		"overview lane：id=feature-login",
		"overview mission brief：summary=",
		"overview lane executor action：lane=main",
		"overview mission commander action queue：summary=total=",
		"overview mission commander action queue current：",
		"overview mission commander next action：",
		"overview execution evidence review：items=",
		"overview section：name=openCandidates",
		"overview section：name=batches",
		"overview next step：",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("overview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("overview text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Pack", "_template", "-Format", "table"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "项目概览：") || strings.Contains(out.String(), "overview：mutation=") {
		t.Fatalf("overview table should keep legacy render output:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Pack", "_template", "-WhatIf", "-Action", "debug", "-Lane", "feature-login", "-Subject", "nested gate", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var gatePlan struct {
		Command    string `json:"command"`
		CaseRoot   string `json:"caseRoot"`
		IsMutation bool   `json:"isMutation"`
	}
	if err := json.Unmarshal(out.Bytes(), &gatePlan); err != nil {
		t.Fatalf("nested case-local gate stdout is not JSON: %v\n%s", err, out.String())
	}
	if gatePlan.Command != "gate" || gatePlan.CaseRoot != caseRoot || gatePlan.IsMutation {
		t.Fatalf("unexpected nested case-local gate plan: %+v", gatePlan)
	}

	out.Reset()
	reviewRoot := filepath.Join(caseRoot, ".rekit", "reviews", "nested-product-path")
	if err := Run([]string{"-Command", "plan-subagents", "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ReviewOutputDir", reviewRoot}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if plan.Command != "plan-subagents" || plan.ReviewRoot != reviewRoot || plan.ItemCount != 2 || plan.ShardCount != 2 || !strings.HasPrefix(plan.PacketPath, caseRoot) || packet.Route.ID != "_template:lane-feature-analysis" {
		t.Fatalf("unexpected nested case-local plan-subagents result: result=%+v packet=%+v", plan, packet)
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

	textCaseRoot := filepath.Join(t.TempDir(), "case-text")
	out.Reset()
	if err := Run([]string{"-Command", "bootstrap", "-Target", textCaseRoot, "-Pack", "_template", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"bootstrap plan：mutation=false reviewRequired=true requiresConfirmation=true writes=",
		"bootstrap plan write：path=.rekit/instance.yml kind=instance-metadata action=create",
		"bootstrap plan next step：review this plan, then re-run bootstrap with -Apply to initialize the case",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("bootstrap preview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("bootstrap preview text should not emit JSON:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "bootstrap", "-Target", textCaseRoot, "-Pack", "_template", "-Apply", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"bootstrap apply：mutation=true applied=true writes=",
		"bootstrap apply write：path=.rekit/instance.yml kind=instance-metadata action=refresh",
		"bootstrap apply next step：run doctor after apply",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("bootstrap apply text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("bootstrap apply text should not emit JSON:\n%s", out.String())
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
	assertMovedCaseRepairPreviewDiagnostic(t, err, caseRoot, "_template")
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
	seedSyncDrift := func() {
		writeCaseFile(t, caseRoot, "references/template/README.md", "# Local drift\n\nchanged before sync apply\n")
		writeCaseFile(t, caseRoot, "references/template/task-handoff.md", "# Local handoff\n\nkeep this file on first apply\n")
		writeCaseFile(t, caseRoot, "CLAUDE.local.md", "prefix\n\n<!-- BEGIN template-pack:router -->\nold managed block\n<!-- END template-pack:router -->\n\nsuffix\n")
	}
	seedSyncDrift()

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

	seedSyncDrift()
	out.Reset()
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-WhatIf", "-Format", "text", "-ProjectName", "sync-cli"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"sync apply：mutation=false applied=false writes=",
		"sync apply write：path=.rekit/instance.yml kind=instance-metadata action=refresh",
		"sync apply write：path=references/template/README.md kind=managed-file action=overwrite-with-backup",
		"sync apply write：path=references/template/task-handoff.md kind=template-file action=skip-existing-local-file",
		"sync apply write：path=CLAUDE.local.md kind=managed-block action=replace-managed-block",
		"sync apply next step：review this non-writing preview, then re-run sync with -Apply after confirming the exact scope",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("sync apply what-if text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("sync apply what-if text should not emit JSON:\n%s", out.String())
	}

	seedSyncDrift()
	out.Reset()
	if err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "text", "-ProjectName", "sync-cli"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"sync apply：mutation=true applied=true writes=",
		"sync apply write：path=.rekit/instance.yml kind=instance-metadata action=refresh",
		"sync apply write：path=references/template/README.md kind=managed-file action=overwrite-with-backup",
		"sync apply write：path=references/template/task-handoff.md kind=template-file action=skip-existing-local-file",
		"sync apply write：path=CLAUDE.local.md kind=managed-block action=replace-managed-block",
		"sync apply next step：run doctor after apply",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("sync apply text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("sync apply text should not emit JSON:\n%s", out.String())
	}

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

func TestRunUpdateApplyUsesUpdateCommandIdentity(t *testing.T) {
	caseRoot := attachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Update drift\n\nchanged before update apply\n")

	var out bytes.Buffer
	if err := Run([]string{"-Command", "update", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"update apply：mutation=false applied=false writes=",
		"update apply write：path=references/template/README.md kind=managed-file action=overwrite-with-backup",
		"update apply next step：review this non-writing preview, then re-run update with -Apply after confirming the exact scope",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("update apply text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "sync apply") {
		t.Fatalf("update apply text leaked sync command identity:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "update", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Command   string   `json:"command"`
		NextSteps []string `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("update apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "update" || len(result.NextSteps) == 0 || !strings.Contains(result.NextSteps[0], "re-run update with -Apply") {
		t.Fatalf("update apply JSON omitted update command identity: %+v", result)
	}
}

func TestRunSyncApplyRejectsMovedCase(t *testing.T) {
	caseRoot := movedAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "sync", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for sync -Apply on moved case")
	}
	assertMovedCaseRepairPreviewDiagnostic(t, err, caseRoot, "_template")
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

func TestRunPromoteReviewRejectsMovedCaseWithPreviewFirstDiagnostic(t *testing.T) {
	caseRoot := movedAttachedCase(t)
	var out bytes.Buffer
	err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template"}, &out)
	if err == nil {
		t.Fatal("Run returned nil error for promote on moved case")
	}
	assertMovedCaseRepairPreviewDiagnostic(t, err, caseRoot, "_template")
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
	for _, expected := range []string{"项目概览：", "工作线：", "共享事实：", "Mission Control brief：", "ready lanes", "blocked lanes", "pending gates", "debug gate", "open decisions", "candidate: handler", "interventions", "manual override", "next agent actions", "escalations", "pending-gate requires main-agent/user decision", "Lane executor actions：", "main：blocked=true ready=false pendingGates=1 openInterventions=1 openDecisions=3", "executor: current=session-main generation=3 lastTakeover=2026-01-01T00:00:00Z by=main-agent reason=fixture", "requirements: reconcile=true pendingGate=true openDecision=true", "blocker reasons: pending-gate,intervention,open-decision", "commander: state=needs-reconcile", "先 reconcile open intervention", "Execution evidence review：", "summary: total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=1 boundaryHits=0 latestEventId=obs-auth-1 gateEventId=gate-auth-1 status=complete action=debug", "summary current action: `/rekit handoff main`", "summary boundary:", "gate-auth-1", "outputRefs: workspace/main/debug/out.txt", "evidenceRefs: evidence/debug.json", "observation evidence is already recorded; do not replay heavy tool", "commander: state=ready-for-evidence-review primary=`/rekit handoff main`", "commander follow-up:", "/rekit continue main -WhatIf", "未决 candidate：", "pending-gate", "by=runtime-test", "action=debug", "最近 verification：", "verifier=manual-review", "verdict=accepted", "target=candidate-alpha", "by=reviewer-smoke", "最近 decision：", "batch-overview", "未解决 intervention：", "最近 rollback：", "reconcile open intervention(s) before continuing the affected lane"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("overview missing %q:\n%s", expected, text)
		}
	}
	for _, expected := range []string{"Mission Commander action index：", "main：state=needs-reconcile blocked=true ready=false primary=`/rekit reconcile main -InterventionId <eventId> -Apply`", "follow-up:", "/rekit continue main -WhatIf", "/rekit handoff main", "boundary:", "no authority/confirmed writes", "do not run continue for blocked lanes"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("overview missing Mission Commander action index %q:\n%s", expected, text)
		}
	}
	for _, expected := range []string{"Mission Commander action queue：", "summary: total=5 unblocked=2 blocked=3 requiresReview=5 followUp=3 current=/rekit handoff main", "counts: total=5 unblocked=2 blocked=3 requiresReview=5 followUp=3", "current: gate-auth-1 state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=`/rekit handoff main`"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("overview missing Mission Commander action queue %q:\n%s", expected, text)
		}
	}
	for _, expected := range []string{"Mission Commander next actions：", "gate-auth-1：state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=`/rekit handoff main`", "source=executionEvidenceReview.followUp", "command=`/rekit overview`", "review execution evidence for gateEventId gate-auth-1", "observation evidence is already recorded; do not replay heavy tool", "main：state=needs-reconcile source=missionCommanderActions blocked=true requiresReview=true command=`/rekit reconcile main -InterventionId <eventId> -Apply`", "main：state=needs-reconcile source=missionCommanderActions.followUp blocked=true requiresReview=true command=`/rekit continue main -WhatIf`", "follow-up is available only after resolving current lane blockers", "run as -WhatIf first; do not continue autonomously while lane remains blocked"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("overview missing Mission Commander next actions %q:\n%s", expected, text)
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
			ID                 string `json:"id"`
			Label              string `json:"label"`
			Kind               string `json:"kind"`
			Authority          bool   `json:"authority"`
			CurrentExecutor    string `json:"currentExecutor"`
			ExecutorGeneration int    `json:"executorGeneration"`
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
		LaneExecutorActions         []handoffLaneExecutorAction      `json:"laneExecutorActions"`
		MissionCommanderActions     []missionCommanderActionItem     `json:"missionCommanderActions"`
		MissionCommanderNextActions []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue struct {
			Summary string `json:"summary"`
			Counts  struct {
				Total          int `json:"total"`
				Unblocked      int `json:"unblocked"`
				Blocked        int `json:"blocked"`
				RequiresReview int `json:"requiresReview"`
				FollowUp       int `json:"followUp"`
			} `json:"counts"`
			CurrentAction         *missionCommanderNextActionItem  `json:"currentAction"`
			UnblockedActions      []missionCommanderNextActionItem `json:"unblockedActions"`
			BlockedActions        []missionCommanderNextActionItem `json:"blockedActions"`
			ReviewRequiredActions []missionCommanderNextActionItem `json:"reviewRequiredActions"`
			FollowUpActions       []missionCommanderNextActionItem `json:"followUpActions"`
		} `json:"missionCommanderActionQueue"`
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
		Sections                       struct {
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
	if result.Lanes[0].ID != "main" || result.Lanes[0].Label != "main" || result.Lanes[0].Kind != "main" || !result.Lanes[0].Authority || result.Lanes[0].CurrentExecutor != "session-main" || result.Lanes[0].ExecutorGeneration != 3 {
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
	if len(result.LaneExecutorActions) != 1 || result.LaneExecutorActions[0].Lane != "main" || result.LaneExecutorActions[0].CurrentExecutor != "session-main" || result.LaneExecutorActions[0].ExecutorGeneration != 3 || result.LaneExecutorActions[0].LastTakeoverBy != "main-agent" || result.LaneExecutorActions[0].LastTakeoverReason != "fixture" || !result.LaneExecutorActions[0].ExecutorAction.Blocked || result.LaneExecutorActions[0].ExecutorAction.Ready || result.LaneExecutorActions[0].ExecutorAction.PendingGates != 1 || result.LaneExecutorActions[0].ExecutorAction.OpenInterventions != 1 || result.LaneExecutorActions[0].ExecutorAction.OpenDecisions != 3 {
		t.Fatalf("unexpected overview lane executor actions: %+v", result.LaneExecutorActions)
	}
	if len(result.MissionCommanderActions) != 1 || result.MissionCommanderActions[0].Lane != "main" || result.MissionCommanderActions[0].Label != "main" || !result.MissionCommanderActions[0].Blocked || result.MissionCommanderActions[0].Ready || result.MissionCommanderActions[0].PrimaryCommand != "/rekit reconcile main -InterventionId <eventId> -Apply" || result.MissionCommanderActions[0].Action.State != "needs-reconcile" || !containsSubstring(result.MissionCommanderActions[0].FollowUpCommands, "/rekit continue main -WhatIf") || !containsSubstring(result.MissionCommanderActions[0].Boundary, "do not run continue") || !slices.Contains(result.MissionCommanderActions[0].BlockerReasons, "intervention") {
		t.Fatalf("overview JSON missing Mission Commander action index: %+v", result.MissionCommanderActions)
	}
	if len(result.MissionCommanderNextActions) != 5 || result.MissionCommanderNextActions[0].Command != "/rekit handoff main" || result.MissionCommanderNextActions[0].Source != "executionEvidenceReview" || !result.MissionCommanderNextActions[0].RequiresReview || result.MissionCommanderNextActions[1].Command != "/rekit overview" || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit reconcile main -InterventionId <eventId> -Apply", true, true) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main -WhatIf", true, true) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main", true, true) || !containsSubstring(result.MissionCommanderNextActions[0].Reasons, "gate-auth-1") || !containsSubstring(result.MissionCommanderNextActions[0].Boundary, "do not replay heavy tool") || !containsSubstring(result.MissionCommanderNextActions[3].Reasons, "after resolving current lane blockers") || !containsSubstring(result.MissionCommanderNextActions[3].Boundary, "do not run continue") {
		t.Fatalf("overview JSON missing consumable Mission Commander next actions: %+v", result.MissionCommanderNextActions)
	}
	queue := result.MissionCommanderActionQueue
	if queue.Summary != "total=5 unblocked=2 blocked=3 requiresReview=5 followUp=3 current=/rekit handoff main" || queue.Counts.Total != 5 || queue.Counts.Unblocked != 2 || queue.Counts.Blocked != 3 || queue.Counts.RequiresReview != 5 || queue.Counts.FollowUp != 3 || queue.CurrentAction == nil || queue.CurrentAction.Command != "/rekit handoff main" || queue.CurrentAction.Source != "executionEvidenceReview" || len(queue.UnblockedActions) != 2 || len(queue.BlockedActions) != 3 || len(queue.ReviewRequiredActions) != 5 || len(queue.FollowUpActions) != 3 {
		t.Fatalf("overview JSON missing Mission Commander action queue: %+v", queue)
	}
	if len(result.ExecutionEvidenceReview) != 1 || result.ExecutionEvidenceReview[0].GateEventID != "gate-auth-1" || result.ExecutionEvidenceReview[0].Action != "debug" || !containsSubstring(result.ExecutionEvidenceReview[0].OutputRefs, "workspace/main/debug/out.txt") || !containsSubstring(result.ExecutionEvidenceReview[0].EvidenceRefs, "evidence/debug.json") || !containsSubstring(result.ExecutionEvidenceReview[0].Boundary, "do not replay heavy tool") || result.ExecutionEvidenceReview[0].MissionCommanderAction.State != "ready-for-evidence-review" || result.ExecutionEvidenceReview[0].MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || !containsSubstring(result.ExecutionEvidenceReview[0].MissionCommanderAction.FollowUpCommands, "/rekit continue main -WhatIf") || result.ExecutionEvidenceReview[0].FollowThrough.State != "ready-for-evidence-review" || !cliExecutionEvidenceFollowThroughContains(result.ExecutionEvidenceReview[0].FollowThrough, "recorded-evidence-review", "reviewed outputRefs/evidenceRefs") {
		t.Fatalf("overview JSON missing execution evidence review queue: %+v", result.ExecutionEvidenceReview)
	}
	if result.ExecutionEvidenceReviewSummary.Total != 1 || result.ExecutionEvidenceReviewSummary.ReadyForReviewCount != 1 || result.ExecutionEvidenceReviewSummary.MainEscalationCount != 0 || result.ExecutionEvidenceReviewSummary.OutputRefCount != 1 || result.ExecutionEvidenceReviewSummary.EvidenceRefCount != 1 || result.ExecutionEvidenceReviewSummary.LatestEventID != "obs-auth-1" || result.ExecutionEvidenceReviewSummary.LatestGateEventID != "gate-auth-1" || result.ExecutionEvidenceReviewSummary.LatestStatus != "complete" || result.ExecutionEvidenceReviewSummary.CurrentAction != "/rekit handoff main" || result.ExecutionEvidenceReviewSummary.NextActionCount != 5 || result.ExecutionEvidenceReviewSummary.ReviewRequiredActionCount != 5 || !containsSubstring(result.ExecutionEvidenceReviewSummary.Boundary, "summary is read-only") || !containsSubstring(result.ExecutionEvidenceReviewSummary.Boundary, "no authority/confirmed") {
		t.Fatalf("overview JSON missing execution evidence review summary: %+v", result.ExecutionEvidenceReviewSummary)
	}
	if !containsSubstring(result.NextSteps, "review execution evidence for gateEventId gate-auth-1") || !slices.Contains(result.NextSteps, "/rekit handoff main") || containsSubstring(result.NextSteps, "/rekit continue main") {
		t.Fatalf("overview next steps should promote execution evidence review without recommending blocked-lane continue: %+v", result.NextSteps)
	}
	commander := result.LaneExecutorActions[0].ExecutorAction.MissionCommanderAction
	if commander.State != "needs-reconcile" || commander.PrimaryCommand != "/rekit reconcile main -InterventionId <eventId> -Apply" || !containsSubstring(commander.FollowUpCommands, "/rekit continue main -WhatIf") || !containsSubstring(commander.Boundary, "do not run continue") {
		t.Fatalf("overview lane executor action missing Mission Commander blocker handoff: %+v", commander)
	}
	if !slices.Contains(result.NextSteps, "reconcile open intervention(s) before continuing the affected lane") || slices.Contains(result.NextSteps, "/rekit continue main") {
		t.Fatalf("overview next steps should resolve blockers before continue: %+v", result.NextSteps)
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

func TestRunNoteAppendTextHandoffWritesFactEvent(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "note",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Format", "text",
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
	textOut := out.String()
	for _, expected := range []string{
		"note append：mutation=true applied=true reason=none eventId=",
		"path=.rekit/facts/verifications.jsonl kind=verification lane=main subject=review target",
		"note target：caseRoot=" + caseRoot,
		"pack=_template",
		"note event：eventId=",
		"summary=accepted by reviewer",
		"verifier=manual-review",
		"verdict=accepted",
		"target=candidate-alpha",
		"batch=batch-note",
		"note event evidence ref：eventId=",
		"ref=evidence-one",
		"ref=evidence-two",
		"note mission brief：summary=",
		"note executor action：blocked=false ready=true",
		"note executor action requirements：reconcile=false pendingGate=false openDecision=false",
		"note executor action handoff：continue=`/rekit continue main` handoff=`/rekit handoff main`",
		"note executor action commander action：state=ready-to-continue primary=`/rekit continue main`",
		"mission commander next action：state=ready-to-continue source=missionCommanderActions blocked=false requiresReview=false command=`/rekit continue main`",
		"mission commander next action：state=ready-to-continue source=missionCommanderActions.followUp blocked=false requiresReview=false command=`/rekit handoff main`",
	} {
		if !strings.Contains(textOut, expected) {
			t.Fatalf("note append text missing %q:\n%s", expected, textOut)
		}
	}
	if strings.Contains(textOut, "{\n  ") {
		t.Fatalf("note append text should not emit JSON object:\n%s", textOut)
	}
	ledger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "verifications.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	ledgerText := string(ledger)
	for _, expected := range []string{`"kind":"verification"`, `"verdict":"accepted"`, `"evidenceRefs":["evidence-one","evidence-two"]`} {
		if !strings.Contains(ledgerText, expected) {
			t.Fatalf("ledger missing %q:\n%s", expected, ledgerText)
		}
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
		Command                          string                           `json:"command"`
		IsMutation                       bool                             `json:"isMutation"`
		Applied                          bool                             `json:"applied"`
		EventID                          string                           `json:"eventId"`
		Path                             string                           `json:"path"`
		Event                            map[string]any                   `json:"event"`
		MissionBrief                     missionBrief                     `json:"missionBrief"`
		ExecutorAction                   executorActionSnapshot           `json:"executorAction"`
		MissionCommanderAction           missionCommanderActionSnapshot   `json:"missionCommanderAction"`
		MissionCommanderNextActions      []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		WouldExecutorAction              *executorActionSnapshot          `json:"wouldExecutorAction"`
		WouldMissionCommanderAction      *missionCommanderActionSnapshot  `json:"wouldMissionCommanderAction"`
		WouldMissionCommanderNextActions []missionCommanderNextActionItem `json:"wouldMissionCommanderNextActions"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("note append stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Command != "note" || !result.IsMutation || !result.Applied || result.EventID == "" || result.Path != ".rekit/facts/verifications.jsonl" {
		t.Fatalf("unexpected note append result: %+v", result)
	}
	if result.MissionBrief.Summary == "" || result.ExecutorAction.Blocked || !result.ExecutorAction.Ready || result.WouldExecutorAction != nil || result.WouldMissionCommanderAction != nil || len(result.WouldMissionCommanderNextActions) != 0 {
		t.Fatalf("verification append should return ready post action only: %+v", result)
	}
	if result.MissionCommanderAction.State != "ready-to-continue" || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main", false, false) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main", false, false) {
		t.Fatalf("verification append should expose post commander projection: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
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

func TestRunNoteAppendTableAndTSVKeepJsonCompatibility(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	for _, format := range []string{"table", "tsv"} {
		t.Run(format, func(t *testing.T) {
			var out bytes.Buffer
			if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Format", format, "-Kind", "observation", "-Lane", "main", "-Subject", "legacy json", "-EventId", "evt-note-" + format}, &out); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Command    string `json:"command"`
				Applied    bool   `json:"applied"`
				EventID    string `json:"eventId"`
				Path       string `json:"path"`
				IsMutation bool   `json:"isMutation"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("note append %s should keep JSON compatibility: %v\n%s", format, err, out.String())
			}
			if result.Command != "note" || !result.IsMutation || !result.Applied || result.EventID != "evt-note-"+format || result.Path != ".rekit/facts/observations.jsonl" {
				t.Fatalf("unexpected note append %s JSON compatibility result: %+v", format, result)
			}
		})
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

func TestRunNoteAppendWhatIfTextHandoffDoesNotWrite(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-Format", "text", "-WhatIf", "-Kind", "candidate", "-Lane", "main", "-Subject", "preview blocker", "-Confidence", "high", "-Status", "open"}, &out); err != nil {
		t.Fatal(err)
	}
	textOut := out.String()
	for _, expected := range []string{
		"note append：mutation=false applied=false reason=what-if eventId=",
		"path=.rekit/facts/candidates.jsonl kind=candidate lane=main subject=preview blocker",
		"note event：eventId=",
		"status=open",
		"confidence=high",
		"note mission brief：summary=",
		"note executor action：blocked=false ready=true",
		"note executor action commander action：state=ready-to-continue primary=`/rekit continue main`",
		"mission commander next action：state=ready-to-continue source=missionCommanderActions blocked=false requiresReview=false command=`/rekit continue main`",
		"note would executor action：blocked=true ready=false",
		"note would executor action requirements：reconcile=false pendingGate=false openDecision=true",
		"note would executor action commander action：state=needs-open-decision-review primary=`/rekit handoff main`",
		"note would mission commander next action：state=needs-open-decision-review source=missionCommanderActions blocked=true requiresReview=true command=`/rekit handoff main`",
		"note would mission commander next action：state=needs-open-decision-review source=missionCommanderActions.followUp blocked=true requiresReview=true command=`/rekit continue main -WhatIf`",
	} {
		if !strings.Contains(textOut, expected) {
			t.Fatalf("note what-if text missing %q:\n%s", expected, textOut)
		}
	}
	if strings.Contains(textOut, "{\n  ") {
		t.Fatalf("note what-if text should not emit JSON object:\n%s", textOut)
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunNoteAppendWhatIfDoesNotWrite(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Kind", "candidate", "-Lane", "main", "-Subject", "preview blocker", "-Confidence", "high", "-Status", "open"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		IsMutation                       bool                             `json:"isMutation"`
		Applied                          bool                             `json:"applied"`
		Reason                           string                           `json:"reason"`
		Path                             string                           `json:"path"`
		MissionBrief                     missionBrief                     `json:"missionBrief"`
		ExecutorAction                   executorActionSnapshot           `json:"executorAction"`
		MissionCommanderAction           missionCommanderActionSnapshot   `json:"missionCommanderAction"`
		MissionCommanderNextActions      []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		WouldExecutorAction              executorActionSnapshot           `json:"wouldExecutorAction"`
		WouldMissionCommanderAction      missionCommanderActionSnapshot   `json:"wouldMissionCommanderAction"`
		WouldMissionCommanderNextActions []missionCommanderNextActionItem `json:"wouldMissionCommanderNextActions"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("note what-if stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.IsMutation || result.Applied || result.Reason != "what-if" || result.Path != ".rekit/facts/candidates.jsonl" {
		t.Fatalf("unexpected note what-if result: %+v", result)
	}
	if result.MissionBrief.Summary == "" || result.ExecutorAction.Blocked || !result.ExecutorAction.Ready || !result.WouldExecutorAction.Blocked || result.WouldExecutorAction.Ready || result.WouldExecutorAction.OpenDecisions != 1 || !result.WouldExecutorAction.OpenDecisionRequired {
		t.Fatalf("unexpected note current/would actions: %+v", result)
	}
	if result.MissionCommanderAction.State != "ready-to-continue" || result.WouldMissionCommanderAction.State != "needs-open-decision-review" {
		t.Fatalf("note what-if should expose current/would commander action delta: current=%+v would=%+v", result.MissionCommanderAction, result.WouldMissionCommanderAction)
	}
	if !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main", false, false) || !containsMissionCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions", "/rekit handoff main", true, true) || !containsMissionCommanderNextAction(result.WouldMissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main -WhatIf", true, true) {
		t.Fatalf("note what-if commander next actions drifted: current=%+v would=%+v", result.MissionCommanderNextActions, result.WouldMissionCommanderNextActions)
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunNoteWhatIfDuplicateReturnsCurrentActionOnly(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	writeCaseFile(t, caseRoot, ".rekit/facts/observations.jsonl", `{"kind":"observation","lane":"main","eventId":"evt-preview-note"}`+"\n")
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	var out bytes.Buffer
	if err := Run([]string{"-Command", "note", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Kind", "candidate", "-Lane", "main", "-Subject", "duplicate preview", "-Status", "open", "-EventId", "evt-preview-note"}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Applied                          bool                             `json:"applied"`
		Reason                           string                           `json:"reason"`
		ExecutorAction                   executorActionSnapshot           `json:"executorAction"`
		MissionCommanderAction           missionCommanderActionSnapshot   `json:"missionCommanderAction"`
		MissionCommanderNextActions      []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		WouldExecutorAction              *executorActionSnapshot          `json:"wouldExecutorAction"`
		WouldMissionCommanderAction      *missionCommanderActionSnapshot  `json:"wouldMissionCommanderAction"`
		WouldMissionCommanderNextActions []missionCommanderNextActionItem `json:"wouldMissionCommanderNextActions"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("duplicate note what-if stdout is not JSON: %v\n%s", err, out.String())
	}
	if result.Applied || result.Reason != "duplicate eventId" || result.WouldExecutorAction != nil || result.WouldMissionCommanderAction != nil || len(result.WouldMissionCommanderNextActions) != 0 || result.ExecutorAction.Blocked || !result.ExecutorAction.Ready {
		t.Fatalf("duplicate what-if should preserve current action only: %+v", result)
	}
	if result.MissionCommanderAction.State != "ready-to-continue" || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main", false, false) {
		t.Fatalf("duplicate what-if should preserve current commander projection: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
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
		Applied                          bool                             `json:"applied"`
		Reason                           string                           `json:"reason"`
		ExecutorAction                   executorActionSnapshot           `json:"executorAction"`
		MissionCommanderAction           missionCommanderActionSnapshot   `json:"missionCommanderAction"`
		MissionCommanderNextActions      []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		WouldExecutorAction              *executorActionSnapshot          `json:"wouldExecutorAction"`
		WouldMissionCommanderAction      *missionCommanderActionSnapshot  `json:"wouldMissionCommanderAction"`
		WouldMissionCommanderNextActions []missionCommanderNextActionItem `json:"wouldMissionCommanderNextActions"`
	}
	if err := json.Unmarshal(second.Bytes(), &result); err != nil {
		t.Fatalf("second note append stdout is not JSON: %v\n%s", err, second.String())
	}
	if result.Applied || result.Reason != "duplicate eventId" {
		t.Fatalf("unexpected duplicate result: %+v", result)
	}
	if result.WouldExecutorAction != nil || result.WouldMissionCommanderAction != nil || len(result.WouldMissionCommanderNextActions) != 0 || !result.ExecutorAction.Blocked || result.ExecutorAction.Ready || result.ExecutorAction.OpenDecisions != 1 {
		t.Fatalf("duplicate should preserve the current blocked action without a would delta: %+v", result)
	}
	if result.MissionCommanderAction.State != "needs-open-decision-review" || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit handoff main", true, true) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main -WhatIf", true, true) {
		t.Fatalf("duplicate append should preserve blocked commander projection: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
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
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-WhatIf", "-Executor", "session-preview", "-Actor", "main-agent", "-Reason", "preview claim"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodeStartResult(t, out.Bytes())
	if result.IsMutation || result.Applied || result.Lane.ID != "feature-login" || result.Lane.Workspace != "workspace/features/feature-login" || result.Lane.CurrentExecutor != "session-preview" || result.Lane.ExecutorGeneration != 1 || result.Lane.LastTakeoverBy != "main-agent" || result.Lane.LastTakeoverReason != "preview claim" {
		t.Fatalf("unexpected start preview result: %+v", result)
	}
	if result.MissionBrief.Summary == "" || result.MissionBrief.Summary != "openLanes=0 ready=0 blocked=0 pendingGates=0 authorizedGates=0 openDecisions=0 interventions=0" {
		t.Fatalf("start preview missing pre-apply mission brief: %+v", result.MissionBrief)
	}
	if result.ExecutorAction.Blocked || result.ExecutorAction.Ready || result.ExecutorAction.PendingGates != 0 || result.ExecutorAction.OpenInterventions != 0 || result.ExecutorAction.OpenDecisions != 0 || result.ExecutorAction.ResumeCommand != "/rekit continue login" || result.ExecutorAction.HandoffCommand != "/rekit handoff login" {
		t.Fatalf("start preview executor action drifted: %+v", result.ExecutorAction)
	}
	if result.ExecutorAction.MissionCommanderAction.State != "needs-start-apply" || result.ExecutorAction.MissionCommanderAction.PrimaryCommand != `/rekit start login -Apply -Executor session-preview -Actor main-agent -Reason "preview claim"` || !containsSubstring(result.ExecutorAction.MissionCommanderAction.FollowUpCommands, "/rekit continue login") || !containsSubstring(result.ExecutorAction.MissionCommanderAction.Boundary, "case-local lane/board/resume/checkpoint") {
		t.Fatalf("start preview should expose full start-apply Mission Commander handoff: %+v", result.ExecutorAction.MissionCommanderAction)
	}
	if result.MissionCommanderAction.State != "needs-start-apply" || result.MissionCommanderAction.PrimaryCommand != result.ExecutorAction.MissionCommanderAction.PrimaryCommand || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", `/rekit start login -Apply -Executor session-preview -Actor main-agent -Reason "preview claim"`, false, true) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue login", true, true) {
		t.Fatalf("start preview should expose top-level Mission Commander start-apply projection: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	assertCLIActionQueue(t, result.MissionCommanderActionQueue, 3, 1, 2, 3, 2, `/rekit start login -Apply -Executor session-preview -Actor main-agent -Reason "preview claim"`)
	if len(result.Writes) != 1 || result.Writes[0].Path != ".rekit/lanes/feature-login/lane.json" || result.Writes[0].Action != "would-create-lane-and-claim-executor" {
		t.Fatalf("unexpected start preview writes: %+v", result.Writes)
	}
	after := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, after)
}

func TestRunStartApplyClaimsAndTakesOverExecutor(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply", "-Executor", "session-1", "-Actor", "main-agent", "-Reason", "initial explicit claim"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodeStartResult(t, out.Bytes())
	if !result.IsMutation || !result.Applied || result.Lane.ID != "feature-login" || result.Lane.CurrentExecutor != "session-1" || result.Lane.ExecutorGeneration != 1 || result.Lane.LastTakeoverBy != "main-agent" || result.Lane.LastTakeoverReason != "initial explicit claim" {
		t.Fatalf("unexpected start apply result: %+v", result)
	}
	if result.MissionBrief.Summary == "" || !slices.Contains(result.MissionBrief.ReadyLanes, "main") || !slices.Contains(result.MissionBrief.ReadyLanes, "login") || len(result.MissionBrief.BlockedLanes) != 0 {
		t.Fatalf("start apply JSON missing post-apply mission brief: %+v", result.MissionBrief)
	}
	if result.ExecutorAction.Blocked || !result.ExecutorAction.Ready || result.ExecutorAction.PendingGates != 0 || result.ExecutorAction.OpenInterventions != 0 || result.ExecutorAction.OpenDecisions != 0 || result.ExecutorAction.ResumeCommand != "/rekit continue login" || result.ExecutorAction.HandoffCommand != "/rekit handoff login" {
		t.Fatalf("start apply executor action drifted: %+v", result.ExecutorAction)
	}
	if result.ExecutorAction.MissionCommanderAction.State != "ready-to-continue" || result.ExecutorAction.MissionCommanderAction.PrimaryCommand != "/rekit continue login" || !containsSubstring(result.ExecutorAction.MissionCommanderAction.FollowUpCommands, "/rekit handoff login") {
		t.Fatalf("start apply should expose ready Mission Commander handoff: %+v", result.ExecutorAction.MissionCommanderAction)
	}
	if result.MissionCommanderAction.State != "ready-to-continue" || result.MissionCommanderAction.PrimaryCommand != "/rekit continue login" || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue login", false, false) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff login", false, false) {
		t.Fatalf("start apply should expose top-level Mission Commander continue projection: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	assertCLIActionQueue(t, result.MissionCommanderActionQueue, 2, 2, 0, 0, 1, "/rekit continue login")
	if !slices.Equal(result.NextSteps, []string{"run doctor after apply", "/rekit continue login"}) {
		t.Fatalf("start apply next steps should recommend ready lane continue: %+v", result.NextSteps)
	}
	assertStartWrite(t, result.Writes, ".rekit/policy.yml", "create-policy")
	assertStartWrite(t, result.Writes, ".rekit/lanes/main/lane.json", "create-lane")
	assertStartWrite(t, result.Writes, ".rekit/lanes/feature-login/lane.json", "create-lane-and-executor-claim")
	assertStartWrite(t, result.Writes, ".rekit/lanes/feature-login/events.jsonl", "append-executor-registered")
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
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply", "-Executor", "session-1", "-Actor", "main-agent", "-Reason", "repeat explicit claim"}, &out); err != nil {
		t.Fatal(err)
	}
	again := decodeStartResult(t, out.Bytes())
	assertStartWrite(t, again.Writes, ".rekit/lanes/feature-login/lane.json", "enter-existing-lane")
	if again.Lane.CurrentExecutor != "session-1" || again.Lane.ExecutorGeneration != 1 {
		t.Fatalf("same executor claim should be idempotent: %+v", again.Lane)
	}
	if !again.ExecutorAction.Ready || again.ExecutorAction.Blocked || again.ExecutorAction.ResumeCommand != "/rekit continue login" || again.ExecutorAction.HandoffCommand != "/rekit handoff login" {
		t.Fatalf("start existing-lane executor action drifted: %+v", again.ExecutorAction)
	}

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply", "-Executor", "session-2", "-Actor", "main-agent", "-Reason", "replace stuck session"}, &out); err != nil {
		t.Fatal(err)
	}
	takenOver := decodeStartResult(t, out.Bytes())
	if takenOver.Lane.CurrentExecutor != "session-2" || takenOver.Lane.ExecutorGeneration != 2 || takenOver.Lane.LastTakeoverBy != "main-agent" || takenOver.Lane.LastTakeoverReason != "replace stuck session" {
		t.Fatalf("executor takeover did not update durable lane state: %+v", takenOver.Lane)
	}
	assertStartWrite(t, takenOver.Writes, ".rekit/lanes/feature-login/lane.json", "update-executor-takeover")
	assertStartWrite(t, takenOver.Writes, ".rekit/lanes/feature-login/events.jsonl", "append-executor-takeover")

	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/inbox.jsonl", "{\"eventId\":\"in-1\",\"summary\":\"review queued\"}\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/tasks.jsonl", "{\"taskId\":\"task-1\",\"summary\":\"inspect candidate\",\"status\":\"open\"}\n{\"taskId\":\"task-2\",\"summary\":\"closed task\",\"status\":\"closed\"}\n")
	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply", "-Force"}, &out); err != nil {
		t.Fatal(err)
	}
	forced := decodeStartResult(t, out.Bytes())
	assertStartWrite(t, forced.Writes, ".rekit/lanes/feature-login/lane.json", "refresh-lane-with-force")
	if !forced.ExecutorAction.Ready || forced.ExecutorAction.Blocked || forced.ExecutorAction.PendingGates != 0 || forced.ExecutorAction.OpenInterventions != 0 || forced.ExecutorAction.OpenDecisions != 0 {
		t.Fatalf("start force-refresh executor action drifted: %+v", forced.ExecutorAction)
	}
	assertStartWrite(t, forced.Writes, ".rekit/lanes/feature-login/events.jsonl", "append-lane-refreshed")
	resume, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "feature-login", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resume), "review queued") || !strings.Contains(string(resume), "inspect candidate") || strings.Contains(string(resume), "closed task") {
		t.Fatalf("force refresh resume did not preserve live inbox/tasks:\n%s", string(resume))
	}
}

func TestRunReplaceableSessionExecutorTakeoverFromHandoffProductPath(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeHandoffFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	project := decodeHandoffResult(t, out.Bytes())
	mainBefore := handoffLaneActionFor(t, project.LaneExecutorActions, "main")
	if mainBefore.CurrentExecutor != "session-main" || mainBefore.ExecutorGeneration != 1 || !mainBefore.ExecutorAction.Ready || mainBefore.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("initial project handoff should expose ready main executor owner: %+v", mainBefore)
	}

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "main", "-WhatIf", "-Executor", "session-main-preview", "-Actor", "mission-commander", "-Reason", "preview replacement from handoff"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := decodeStartResult(t, out.Bytes())
	if preview.IsMutation || preview.Applied || preview.Lane.ID != "main" || preview.Lane.CurrentExecutor != "session-main-preview" || preview.Lane.ExecutorGeneration != 2 || len(preview.Writes) != 1 || preview.Writes[0].Path != ".rekit/lanes/main/lane.json" || preview.Writes[0].Action != "would-enter-existing-lane-and-claim-executor" {
		t.Fatalf("-Name main preview should resolve existing main lane for replacement takeover: %+v", preview)
	}
	if preview.ExecutorAction.MissionCommanderAction.State != "needs-start-apply" || preview.ExecutorAction.MissionCommanderAction.PrimaryCommand != `/rekit start main -Apply -Executor session-main-preview -Actor mission-commander -Reason "preview replacement from handoff"` || !containsSubstring(preview.ExecutorAction.MissionCommanderAction.FollowUpCommands, "/rekit continue main") {
		t.Fatalf("-Name main preview should require applying executor takeover before continue: %+v", preview.ExecutorAction.MissionCommanderAction)
	}
	if preview.MissionCommanderAction.State != "needs-start-apply" || preview.MissionCommanderAction.PrimaryCommand != preview.ExecutorAction.MissionCommanderAction.PrimaryCommand || !containsMissionCommanderNextAction(preview.MissionCommanderNextActions, "missionCommanderActions", `/rekit start main -Apply -Executor session-main-preview -Actor mission-commander -Reason "preview replacement from handoff"`, false, true) || !containsMissionCommanderNextAction(preview.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main", true, true) {
		t.Fatalf("-Name main preview should expose top-level Mission Commander takeover projection: action=%+v next=%+v", preview.MissionCommanderAction, preview.MissionCommanderNextActions)
	}

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Apply", "main", "-Executor", "session-main-replacement", "-Actor", "mission-commander", "-Reason", "replace stale main session from handoff"}, &out); err != nil {
		t.Fatal(err)
	}
	takeover := decodeStartResult(t, out.Bytes())
	if !takeover.IsMutation || !takeover.Applied || takeover.Lane.ID != "main" || takeover.Lane.CurrentExecutor != "session-main-replacement" || takeover.Lane.ExecutorGeneration != 2 || takeover.Lane.LastTakeoverBy != "mission-commander" || takeover.Lane.LastTakeoverReason != "replace stale main session from handoff" {
		t.Fatalf("main executor takeover did not update durable lane state: %+v", takeover)
	}
	if takeover.ExecutorAction.Blocked || !takeover.ExecutorAction.Ready || takeover.ExecutorAction.ResumeCommand != "/rekit continue main" || takeover.ExecutorAction.HandoffCommand != "/rekit handoff main" || !slices.Contains(takeover.NextSteps, "/rekit continue main") {
		t.Fatalf("main executor takeover should leave replacement session ready to continue: action=%+v next=%+v", takeover.ExecutorAction, takeover.NextSteps)
	}
	if takeover.ExecutorAction.MissionCommanderAction.State != "ready-to-continue" || takeover.ExecutorAction.MissionCommanderAction.PrimaryCommand != "/rekit continue main" {
		t.Fatalf("main executor takeover should expose Mission Commander continue handoff: %+v", takeover.ExecutorAction.MissionCommanderAction)
	}
	if takeover.MissionCommanderAction.State != "ready-to-continue" || takeover.MissionCommanderAction.PrimaryCommand != "/rekit continue main" || !containsMissionCommanderNextAction(takeover.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue main", false, false) || !containsMissionCommanderNextAction(takeover.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main", false, false) {
		t.Fatalf("main executor takeover should expose top-level Mission Commander continue projection: action=%+v next=%+v", takeover.MissionCommanderAction, takeover.MissionCommanderNextActions)
	}
	assertStartWrite(t, takeover.Writes, ".rekit/lanes/main/lane.json", "update-executor-takeover")
	assertStartWrite(t, takeover.Writes, ".rekit/lanes/main/events.jsonl", "append-executor-takeover")
	assertStartWrite(t, takeover.Writes, ".rekit/lanes/main/prompts/RESUME.md", "refresh")
	assertStartWrite(t, takeover.Writes, ".rekit/lanes/main/checkpoints/latest.json", "refresh")

	checkpointBytes, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "checkpoints", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		CurrentExecutor    string                 `json:"currentExecutor"`
		ExecutorGeneration int                    `json:"executorGeneration"`
		LastTakeoverBy     string                 `json:"lastTakeoverBy"`
		LastTakeoverReason string                 `json:"lastTakeoverReason"`
		ExecutorAction     executorActionSnapshot `json:"executorAction"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("checkpoint did not decode: %v\n%s", err, string(checkpointBytes))
	}
	if checkpoint.CurrentExecutor != "session-main-replacement" || checkpoint.ExecutorGeneration != 2 || checkpoint.LastTakeoverBy != "mission-commander" || checkpoint.LastTakeoverReason != "replace stale main session from handoff" || !checkpoint.ExecutorAction.Ready || checkpoint.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("checkpoint omitted replacement executor takeover state: %+v", checkpoint)
	}
	if checkpoint.ExecutorAction.MissionCommanderAction.State != "ready-to-continue" || checkpoint.ExecutorAction.MissionCommanderAction.PrimaryCommand != "/rekit continue main" {
		t.Fatalf("checkpoint omitted Mission Commander continue handoff: %+v", checkpoint.ExecutorAction.MissionCommanderAction)
	}
	resumeBytes, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"current executor: `session-main-replacement`", "executor generation: `2`", "last takeover by: `mission-commander`", "resume command: `/rekit continue main`", "commander state: `ready-to-continue`", "commander prompt: 按 `main` 接手，然后继续该 lane。"} {
		if !strings.Contains(string(resumeBytes), expected) {
			t.Fatalf("resume missing %q after takeover:\n%s", expected, string(resumeBytes))
		}
	}
	events, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(events), `"kind":"executor-takeover"`) || !strings.Contains(string(events), `"previousExecutor":"session-main"`) || !strings.Contains(string(events), `"currentExecutor":"session-main-replacement"`) {
		t.Fatalf("lane event log omitted executor takeover provenance:\n%s", string(events))
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "main"}, &out); err != nil {
		t.Fatal(err)
	}
	laneHandoff := decodeHandoffResult(t, out.Bytes())
	if !laneHandoff.IsMutation || !laneHandoff.Applied || laneHandoff.Project || laneHandoff.Lane == nil || laneHandoff.Lane.ID != "main" || laneHandoff.Lane.CurrentExecutor != "session-main-replacement" || laneHandoff.Lane.ExecutorGeneration != 2 || laneHandoff.ExecutorAction == nil || !laneHandoff.ExecutorAction.Ready || laneHandoff.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("lane handoff should preserve replacement executor owner and next action: %+v", laneHandoff)
	}
	if laneHandoff.ExecutorAction.MissionCommanderAction.State != "ready-to-continue" || laneHandoff.ExecutorAction.MissionCommanderAction.PrimaryCommand != "/rekit continue main" {
		t.Fatalf("lane handoff should expose Mission Commander continue handoff: %+v", laneHandoff.ExecutorAction.MissionCommanderAction)
	}
	laneLatest := assertStartWrite(t, laneHandoff.Writes, ".rekit/handovers/main-latest.md", "write-latest-lane-handoff")
	laneHandoffText, err := os.ReadFile(laneLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(laneHandoffText), "current executor：session-main-replacement") || !strings.Contains(string(laneHandoffText), "executor generation：2") || !strings.Contains(string(laneHandoffText), "直接说：按 `.rekit/handovers/main-latest.md` 接手，然后执行 `/rekit continue main`。") {
		t.Fatalf("lane handoff omitted replacement executor handoff text:\n%s", string(laneHandoffText))
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var continuation struct {
		IsMutation     bool                   `json:"isMutation"`
		Applied        bool                   `json:"applied"`
		Lane           startLane              `json:"lane"`
		ExecutorAction executorActionSnapshot `json:"executorAction"`
	}
	if err := json.Unmarshal(out.Bytes(), &continuation); err != nil {
		t.Fatalf("continue what-if did not decode: %v\n%s", err, out.String())
	}
	if continuation.IsMutation || continuation.Applied || continuation.Lane.CurrentExecutor != "session-main-replacement" || continuation.Lane.ExecutorGeneration != 2 || !continuation.ExecutorAction.Ready || continuation.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("replacement executor cannot continue from durable handoff state: %+v", continuation)
	}
	if continuation.ExecutorAction.MissionCommanderAction.State != "ready-to-continue" || continuation.ExecutorAction.MissionCommanderAction.PrimaryCommand != "/rekit continue main" {
		t.Fatalf("continue what-if should expose Mission Commander continue handoff: %+v", continuation.ExecutorAction.MissionCommanderAction)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("executor takeover wrote authority ledger or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("executor takeover wrote confirmed ledger or stat failed: %v", err)
	}
}

func TestRunStartProjectsExecutorActionForExistingLaneBlockers(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeHandoffFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	preview := decodeStartResult(t, out.Bytes())
	if !preview.ExecutorAction.Blocked || preview.ExecutorAction.Ready || preview.ExecutorAction.PendingGates != 1 || preview.ExecutorAction.OpenInterventions != 1 || preview.ExecutorAction.OpenDecisions != 1 || !preview.ExecutorAction.ReconcileRequired || !preview.ExecutorAction.PendingGateRequired || !preview.ExecutorAction.OpenDecisionRequired || preview.ExecutorAction.ResumeCommand != "/rekit continue login" || preview.ExecutorAction.HandoffCommand != "/rekit handoff login" || !slices.Contains(preview.ExecutorAction.BlockerReasons, "pending-gate") || !slices.Contains(preview.ExecutorAction.BlockerReasons, "intervention") || !slices.Contains(preview.ExecutorAction.BlockerReasons, "open-decision") {
		t.Fatalf("start preview existing-lane executor action drifted: %+v", preview.ExecutorAction)
	}
	if slices.Contains(preview.ExecutorAction.NextAgentActions, "/rekit continue main") || slices.Contains(preview.ExecutorAction.NextAgentActions, "/rekit continue login") || !containsSubstring(preview.ExecutorAction.NextAgentActions, "reconcile open intervention") || !containsSubstring(preview.ExecutorAction.NextAgentActions, "pending-gate") || !containsSubstring(preview.ExecutorAction.NextAgentActions, "candidate/decision") {
		t.Fatalf("start preview blocked lane next actions should be lane-local blocker resolution: %+v", preview.ExecutorAction.NextAgentActions)
	}
	assertCLIActionQueue(t, preview.MissionCommanderActionQueue, 3, 0, 3, 3, 2, "/rekit reconcile login -InterventionId <eventId> -Apply")

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"executor next action：reconcile open intervention(s) before continuing this lane", "executor next action：resolve or keep deferred pending-gate request(s); gate records the request and never executes heavy-tool", "executor next action：review open candidate/decision item(s) with evidence and authority boundary", "executor action：blocked=true ready=false pendingGates=1 openInterventions=1 openDecisions=1", "executor requirements：reconcile=true pendingGate=true openDecision=true", "executor handoff：continue=`/rekit continue login` handoff=`/rekit handoff login`", "mission commander action queue：summary=total=3 unblocked=0 blocked=3 requiresReview=3 followUp=2 current=/rekit reconcile login -InterventionId <eventId> -Apply", "mission commander action queue current：state=needs-reconcile source=missionCommanderActions blocked=true requiresReview=true command=`/rekit reconcile login -InterventionId <eventId> -Apply`", "mission commander next action：state=needs-reconcile source=missionCommanderActions blocked=true requiresReview=true command=`/rekit reconcile login -InterventionId <eventId> -Apply`", "mission commander next action：state=needs-reconcile source=missionCommanderActions.followUp blocked=true requiresReview=true command=`/rekit continue login -WhatIf`", "mission commander next action reason：follow-up is available only after resolving current lane blockers", "mission commander next action boundary：do not run continue for blocked lanes"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("start text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "继续此支线：/rekit continue login") || strings.Contains(out.String(), "executor next action：/rekit continue main") {
		t.Fatalf("start text should not recommend continue for blocked lane:\n%s", out.String())
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
	if len(result.LaneExecutorActions) != 2 || result.ExecutorAction != nil {
		t.Fatalf("project handoff preview missing lane executor actions: %+v", result)
	}
	main := handoffLaneActionFor(t, result.LaneExecutorActions, "main")
	if main.CurrentExecutor != "session-main" || main.ExecutorGeneration != 1 || main.LastTakeoverBy != "main-agent" || main.LastTakeoverReason != "initial main claim" {
		t.Fatalf("project handoff preview main executor owner snapshot drifted: %+v", main)
	}
	mainAction := main.ExecutorAction
	if mainAction.Blocked || !mainAction.Ready || !slices.Equal(mainAction.NextAgentActions, []string{"/rekit continue main"}) {
		t.Fatalf("project handoff preview main executor action should stay ready and lane-local: %+v", mainAction)
	}
	if mainAction.MissionCommanderAction.State != "ready-to-continue" || mainAction.MissionCommanderAction.PrimaryCommand != "/rekit continue main" {
		t.Fatalf("project handoff preview main action missing Mission Commander continue handoff: %+v", mainAction.MissionCommanderAction)
	}
	login := handoffLaneActionFor(t, result.LaneExecutorActions, "feature-login")
	if login.CurrentExecutor != "session-login" || login.ExecutorGeneration != 2 || login.LastTakeoverBy != "main-agent" || login.LastTakeoverReason != "replace login session" {
		t.Fatalf("project handoff preview login executor owner snapshot drifted: %+v", login)
	}
	loginAction := login.ExecutorAction
	if !loginAction.Blocked || !loginAction.ReconcileRequired || !loginAction.PendingGateRequired || !loginAction.OpenDecisionRequired || loginAction.PendingGates != 1 || loginAction.OpenInterventions != 1 || loginAction.OpenDecisions != 1 || loginAction.ResumeCommand != "/rekit continue login" {
		t.Fatalf("project handoff preview login executor action drifted: %+v", loginAction)
	}
	if slices.Contains(loginAction.NextAgentActions, "/rekit continue main") || slices.Contains(loginAction.NextAgentActions, "/rekit continue login") || !containsSubstring(loginAction.NextAgentActions, "reconcile open intervention") || !containsSubstring(loginAction.NextAgentActions, "pending-gate") || !containsSubstring(loginAction.NextAgentActions, "candidate/decision") {
		t.Fatalf("project handoff preview login next actions should be blocker resolution only: %+v", loginAction.NextAgentActions)
	}
	if loginAction.MissionCommanderAction.State != "needs-reconcile" || loginAction.MissionCommanderAction.PrimaryCommand != "/rekit reconcile login -InterventionId <eventId> -Apply" || !containsSubstring(loginAction.MissionCommanderAction.Boundary, "do not run continue") {
		t.Fatalf("project handoff preview login action missing Mission Commander blocker handoff: %+v", loginAction.MissionCommanderAction)
	}
	if len(result.MissionCommanderNextActions) != 5 || result.MissionCommanderNextActions[0].Command != "/rekit continue main" || result.MissionCommanderNextActions[0].Source != "missionCommanderActions" || result.MissionCommanderNextActions[0].RequiresReview || !containsSubstring(result.MissionCommanderNextActions[0].Reasons, "ready lane primary action") || result.MissionCommanderNextActions[1].Command != "/rekit reconcile login -InterventionId <eventId> -Apply" || !result.MissionCommanderNextActions[1].Blocked || !result.MissionCommanderNextActions[1].RequiresReview || !containsSubstring(result.MissionCommanderNextActions[1].Reasons, "intervention") || !containsSubstring(result.MissionCommanderNextActions[1].Boundary, "do not run continue") || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main", false, false) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue login -WhatIf", true, true) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff login", true, true) || !containsSubstring(result.MissionCommanderNextActions[3].Reasons, "after resolving current lane blockers") {
		t.Fatalf("project handoff preview missing Mission Commander next actions: %+v", result.MissionCommanderNextActions)
	}
	assertCLIActionQueue(t, result.MissionCommanderActionQueue, 5, 2, 3, 3, 3, "/rekit continue main")
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
	if len(project.LaneExecutorActions) != 2 || project.ExecutorAction != nil {
		t.Fatalf("project handoff JSON missing lane executor actions: %+v", project)
	}
	projectLogin := handoffLaneActionFor(t, project.LaneExecutorActions, "feature-login")
	if projectLogin.CurrentExecutor != "session-login" || projectLogin.ExecutorGeneration != 2 || projectLogin.LastTakeoverBy != "main-agent" || projectLogin.LastTakeoverReason != "replace login session" {
		t.Fatalf("project handoff JSON login executor owner snapshot drifted: %+v", projectLogin)
	}
	projectLoginAction := projectLogin.ExecutorAction
	if !projectLoginAction.Blocked || projectLoginAction.PendingGates != 1 || projectLoginAction.OpenInterventions != 1 || projectLoginAction.OpenDecisions != 1 || !slices.Contains(projectLoginAction.BlockerReasons, "pending-gate") || !slices.Contains(projectLoginAction.BlockerReasons, "intervention") || !slices.Contains(projectLoginAction.BlockerReasons, "open-decision") {
		t.Fatalf("project handoff JSON login executor action drifted: %+v", projectLoginAction)
	}
	if slices.Contains(projectLoginAction.NextAgentActions, "/rekit continue main") || slices.Contains(projectLoginAction.NextAgentActions, "/rekit continue login") || !containsSubstring(projectLoginAction.NextAgentActions, "reconcile open intervention") {
		t.Fatalf("project handoff JSON login next actions should remain lane-local and blocker-aware: %+v", projectLoginAction.NextAgentActions)
	}
	if len(project.MissionCommanderNextActions) != 5 || project.MissionCommanderNextActions[0].Command != "/rekit continue main" || project.MissionCommanderNextActions[0].Source != "missionCommanderActions" || project.MissionCommanderNextActions[0].Blocked || project.MissionCommanderNextActions[0].RequiresReview || project.MissionCommanderNextActions[1].Command != "/rekit reconcile login -InterventionId <eventId> -Apply" || project.MissionCommanderNextActions[1].Source != "missionCommanderActions" || !project.MissionCommanderNextActions[1].Blocked || !project.MissionCommanderNextActions[1].RequiresReview || !containsSubstring(project.MissionCommanderNextActions[1].Reasons, "pending-gate") || !containsSubstring(project.MissionCommanderNextActions[1].Boundary, "do not run continue") || !containsMissionCommanderNextAction(project.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main", false, false) || !containsMissionCommanderNextAction(project.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue login -WhatIf", true, true) || !containsMissionCommanderNextAction(project.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff login", true, true) || !containsSubstring(project.MissionCommanderNextActions[3].Reasons, "after resolving current lane blockers") {
		t.Fatalf("project handoff JSON missing consumable Mission Commander next actions: %+v", project.MissionCommanderNextActions)
	}
	assertCLIActionQueue(t, project.MissionCommanderActionQueue, 5, 2, 3, 3, 3, "/rekit continue main")
	latest := assertStartWrite(t, project.Writes, ".rekit/handovers/latest.md", "write-latest-project-handoff")
	text, err := os.ReadFile(latest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 项目接手索引", "## Mission Control brief", "summary: openLanes=2 ready=1 blocked=1", "ready lanes:", "main", "blocked lanes:", "login (pending-gate,intervention,open-decision)", "pending gates:", "debug gate", "open decisions:", "decision subject", "interventions:", "manual override", "next agent actions:", "reconcile open intervention", "escalations:", "pending-gate requires main-agent/user decision", "## 工作线", "blocked=true", "executor owner：current=session-login generation=2 lastTakeover=2026-01-02T00:00:00Z by=main-agent reason=replace login session", "pendingGates=1 openInterventions=1 openDecisions=1", "requirements：reconcile=true pendingGate=true openDecision=true", "next action：reconcile open intervention(s) before continuing this lane", "next action：resolve or keep deferred pending-gate request(s)", "next action：review open candidate/decision item(s)", "continue command：`/rekit continue main`", "ready 后继续：`/rekit continue login`", "/rekit handoff login"} {
		if !strings.Contains(string(text), expected) {
			t.Fatalf("project handoff missing %q:\n%s", expected, string(text))
		}
	}
	if strings.Contains(string(text), "continue command：`/rekit continue login`") {
		t.Fatalf("project handoff should not present blocked lane continue as its current next action:\n%s", string(text))
	}
	for _, expected := range []string{"state=ready-to-continue source=missionCommanderActions blocked=false requiresReview=false command=`/rekit continue main`", "state=ready-to-continue source=missionCommanderActions.followUp blocked=false requiresReview=false command=`/rekit handoff main`", "state=needs-reconcile source=missionCommanderActions blocked=true requiresReview=true command=`/rekit reconcile login -InterventionId <eventId> -Apply`", "state=needs-reconcile source=missionCommanderActions.followUp blocked=true requiresReview=true command=`/rekit continue login -WhatIf`", "follow-up is available only after resolving current lane blockers", "run as -WhatIf first; do not continue autonomously while lane remains blocked", "commander primary：`/rekit continue main`", "commander follow-up：/rekit handoff main", "commander primary：`/rekit reconcile login -InterventionId <eventId> -Apply`", "commander follow-up：/rekit continue login -WhatIf", "commander boundary：do not run continue for blocked lanes"} {
		if !strings.Contains(string(text), expected) {
			t.Fatalf("project handoff missing Mission Commander consumption %q:\n%s", expected, string(text))
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
	if lane.ExecutorAction == nil || !lane.ExecutorAction.Blocked || lane.ExecutorAction.PendingGates != 1 || lane.ExecutorAction.OpenInterventions != 1 || lane.ExecutorAction.OpenDecisions != 1 || !lane.ExecutorAction.ReconcileRequired || !lane.ExecutorAction.PendingGateRequired || !lane.ExecutorAction.OpenDecisionRequired || lane.ExecutorAction.ResumeCommand != "/rekit continue login" || lane.ExecutorAction.HandoffCommand != "/rekit handoff login" || !slices.Contains(lane.ExecutorAction.BlockerReasons, "pending-gate") || !slices.Contains(lane.ExecutorAction.BlockerReasons, "intervention") || !slices.Contains(lane.ExecutorAction.BlockerReasons, "open-decision") {
		t.Fatalf("lane handoff JSON executor action drifted: %+v", lane.ExecutorAction)
	}
	if lane.ExecutorAction.MissionCommanderAction.State != "needs-reconcile" || lane.ExecutorAction.MissionCommanderAction.PrimaryCommand != "/rekit reconcile login -InterventionId <eventId> -Apply" || !containsSubstring(lane.ExecutorAction.MissionCommanderAction.FollowUpCommands, "/rekit continue login -WhatIf") || !containsSubstring(lane.ExecutorAction.MissionCommanderAction.Boundary, "do not run continue for blocked lanes") {
		t.Fatalf("lane handoff JSON omitted Mission Commander action consumption: %+v", lane.ExecutorAction.MissionCommanderAction)
	}
	if len(lane.MissionCommanderNextActions) != 3 || lane.MissionCommanderNextActions[0].Lane != "feature-login" || lane.MissionCommanderNextActions[0].Label != "login" || lane.MissionCommanderNextActions[0].Command != "/rekit reconcile login -InterventionId <eventId> -Apply" || lane.MissionCommanderNextActions[0].Source != "missionCommanderActions" || !lane.MissionCommanderNextActions[0].Blocked || !lane.MissionCommanderNextActions[0].RequiresReview || !containsSubstring(lane.MissionCommanderNextActions[0].Reasons, "open-decision") || !containsSubstring(lane.MissionCommanderNextActions[0].Boundary, "do not run continue") || !containsMissionCommanderNextAction(lane.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue login -WhatIf", true, true) || !containsMissionCommanderNextAction(lane.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff login", true, true) || !containsSubstring(lane.MissionCommanderNextActions[1].Reasons, "after resolving current lane blockers") || !containsSubstring(lane.MissionCommanderNextActions[1].Reasons, "run as -WhatIf first") {
		t.Fatalf("lane handoff JSON missing Mission Commander next action: %+v", lane.MissionCommanderNextActions)
	}
	assertCLIActionQueue(t, lane.MissionCommanderActionQueue, 3, 0, 3, 3, 2, "/rekit reconcile login -InterventionId <eventId> -Apply")
	if slices.Contains(lane.ExecutorAction.NextAgentActions, "/rekit continue login") || !containsSubstring(lane.ExecutorAction.NextAgentActions, "reconcile open intervention") || !containsSubstring(lane.NextSteps, "reconcile open intervention") || slices.Contains(lane.NextSteps, "/rekit continue login") {
		t.Fatalf("lane handoff JSON should expose blocker-aware next steps only: action=%+v next=%+v", lane.ExecutorAction.NextAgentActions, lane.NextSteps)
	}
	laneLatest := assertStartWrite(t, lane.Writes, ".rekit/handovers/feature-login-latest.md", "write-latest-lane-handoff")
	laneText, err := os.ReadFile(laneLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit 工作线接手：feature-login", "workspace/features/feature-login/packet.md", "先处理下列 blocker，不要执行 `/rekit continue login`", "## Mission Control brief", "blocked: true", "pending-gate:", "open intervention:", "open decision:", "next agent action:", "reconcile open intervention(s) before continuing this lane", "resolve or keep deferred pending-gate request(s)", "review open candidate/decision item(s)", "## Mission Commander next actions", "state=needs-reconcile source=missionCommanderActions blocked=true requiresReview=true command=`/rekit reconcile login -InterventionId <eventId> -Apply`", "state=needs-reconcile source=missionCommanderActions.followUp blocked=true requiresReview=true command=`/rekit continue login -WhatIf`", "follow-up is available only after resolving current lane blockers", "run as -WhatIf first; do not continue autonomously while lane remains blocked", "state=needs-reconcile source=missionCommanderActions.followUp blocked=true requiresReview=true command=`/rekit handoff login`", "## Executor action snapshot", "- blocked: `true`", "- ready: `false`", "- pending gates: `1`", "- open interventions: `1`", "- open decisions: `1`", "- reconcile required: `true`", "- pending gate required: `true`", "- open decision required: `true`", "- resume command: `/rekit continue login`", "- handoff command: `/rekit handoff login`", "- commander primary command: `/rekit reconcile login -InterventionId <eventId> -Apply`", "commander follow-up commands:", "/rekit continue login -WhatIf", "commander boundary:", "do not run continue for blocked lanes", "blocker reasons:", "pending-gate", "intervention", "open-decision", "## verification", "verifier=manual-review", "verdict=accepted", "target=candidate-alpha", "by=reviewer-smoke", "## decision", "by=runtime-test", "## pending-gate", "action=debug", "## intervention", "## rollback", "## 边界"} {
		if !strings.Contains(string(laneText), expected) {
			t.Fatalf("lane handoff missing %q:\n%s", expected, string(laneText))
		}
	}
	if strings.Contains(string(laneText), "然后执行 `/rekit continue login`") {
		t.Fatalf("lane handoff opening should not recommend continue for blocked lane:\n%s", string(laneText))
	}
	resume, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "feature-login", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"review queued", "inspect candidate", "- commander primary command: `/rekit reconcile login -InterventionId <eventId> -Apply`", "commander follow-up commands:", "/rekit continue login -WhatIf", "commander boundary:", "do not run continue for blocked lanes"} {
		if !strings.Contains(string(resume), expected) {
			t.Fatalf("lane resume missing %q:\n%s", expected, string(resume))
		}
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
		ExecutorAction              executorActionSnapshot              `json:"executorAction"`
		MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		Writes                      []startWrite                        `json:"writes"`
		NextSteps                   []string                            `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &blocked); err != nil {
		t.Fatalf("blocked continue stdout is not JSON: %v\n%s", err, out.String())
	}
	if blocked.Command != "continue" || blocked.Applied || !blocked.Blocked || !blocked.ReconcileRequired || len(blocked.OpenInterventions) != 1 || blocked.OpenInterventions[0].EventID != "evt-human-stop" || len(blocked.Writes) != 0 {
		t.Fatalf("unexpected blocked continue result: %+v", blocked)
	}
	if !blocked.ExecutorAction.Blocked || blocked.ExecutorAction.Ready || !slices.Contains(blocked.ExecutorAction.NextAgentActions, "reconcile open intervention(s) before continuing this lane") || slices.Contains(blocked.ExecutorAction.NextAgentActions, "/rekit continue login") || !slices.Equal(blocked.NextSteps, blocked.ExecutorAction.NextAgentActions) {
		t.Fatalf("blocked continue should expose reconcile-only executor next steps: action=%+v next=%+v", blocked.ExecutorAction, blocked.NextSteps)
	}
	assertCLIActionQueue(t, blocked.MissionCommanderActionQueue, 3, 0, 3, 3, 2, "/rekit reconcile login -InterventionId <eventId> -Apply")
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
		Executor                    string                           `json:"executor"`
		ExecutorAction              executorActionSnapshot           `json:"executorAction"`
		MissionCommanderAction      missionCommanderActionSnapshot   `json:"missionCommanderAction"`
		MissionCommanderNextActions []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		WouldWrites                 []startWrite                     `json:"wouldWrites"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("reconcile preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if preview.Command != "reconcile" || preview.IsMutation || preview.Applied || preview.Lane.ID != "feature-login" || preview.Intervention.EventID != "evt-human-stop" || preview.Executor != "session-2" || len(preview.WouldWrites) == 0 {
		t.Fatalf("unexpected reconcile preview: %+v", preview)
	}
	if !preview.ExecutorAction.Blocked || preview.ExecutorAction.Ready || preview.ExecutorAction.OpenInterventions != 1 || !preview.ExecutorAction.ReconcileRequired || preview.ExecutorAction.ResumeCommand != "/rekit continue login" || preview.ExecutorAction.HandoffCommand != "/rekit handoff login" {
		t.Fatalf("reconcile preview executor action drifted: %+v", preview.ExecutorAction)
	}
	if !slices.Equal(preview.ExecutorAction.NextAgentActions, []string{"reconcile open intervention(s) before continuing this lane"}) {
		t.Fatalf("reconcile preview should recommend only intervention reconciliation: %+v", preview.ExecutorAction.NextAgentActions)
	}
	wantReconcileApply := `/rekit reconcile login -InterventionId evt-human-stop -Apply -Executor session-2 -Actor main-agent -Reason "accept user correction"`
	if preview.MissionCommanderAction.State != "needs-reconcile" || preview.MissionCommanderAction.PrimaryCommand != wantReconcileApply || !containsMissionCommanderNextAction(preview.MissionCommanderNextActions, "missionCommanderActions", wantReconcileApply, false, true) || !containsMissionCommanderNextAction(preview.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue login -WhatIf", true, true) {
		t.Fatalf("reconcile preview should expose top-level Mission Commander apply projection: action=%+v next=%+v", preview.MissionCommanderAction, preview.MissionCommanderNextActions)
	}
	assertWriteKind(t, preview.WouldWrites, "lane-event", "would-append-executor-takeover")

	out.Reset()
	if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "vmp-re", "-WhatIf", "login", "-InterventionId", "evt-human-stop", "-Executor", "session-2", "-Actor", "main-agent", "-Reason", "accept user correction", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"executor action：blocked=true ready=false pendingGates=0 openInterventions=1 openDecisions=0", "executor requirements：reconcile=true pendingGate=false openDecision=false", "executor handoff：continue=`/rekit continue login` handoff=`/rekit handoff login`", "mission commander action queue：summary=total=3 unblocked=1 blocked=2 requiresReview=3 followUp=2 current=/rekit reconcile login -InterventionId evt-human-stop -Apply -Executor session-2 -Actor main-agent -Reason \"accept user correction\"", "mission commander action queue current：state=needs-reconcile source=missionCommanderActions blocked=false requiresReview=true command=`/rekit reconcile login -InterventionId evt-human-stop -Apply -Executor session-2 -Actor main-agent -Reason \"accept user correction\"`", "mission commander next action：state=needs-reconcile source=missionCommanderActions blocked=false requiresReview=true command=`/rekit reconcile login -InterventionId evt-human-stop -Apply -Executor session-2 -Actor main-agent -Reason \"accept user correction\"`", "mission commander next action：state=needs-reconcile source=missionCommanderActions.followUp blocked=true requiresReview=true command=`/rekit continue login -WhatIf`", "mission commander next action reason：run only after reconcile apply succeeds and the refreshed executor action remains ready", "mission commander next action boundary：reconcile apply only writes case-local intervention/lane/resume/checkpoint state"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reconcile text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "继续此支线：/rekit continue login") {
		t.Fatalf("reconcile preview text should not recommend continue before apply:\n%s", out.String())
	}
	afterPreview := snapshotFiles(t, caseRoot)
	assertSnapshotEqual(t, before, afterPreview)

	out.Reset()
	if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "vmp-re", "-Apply", "login", "-InterventionId", "evt-human-stop", "-Executor", "session-2", "-Actor", "main-agent", "-Reason", "accept user correction"}, &out); err != nil {
		t.Fatal(err)
	}
	var reconciled struct {
		Command                     string                           `json:"command"`
		IsMutation                  bool                             `json:"isMutation"`
		Applied                     bool                             `json:"applied"`
		Lane                        startLane                        `json:"lane"`
		ResolutionEventID           string                           `json:"resolutionEventId"`
		Executor                    string                           `json:"executor"`
		ExecutorGeneration          int                              `json:"executorGeneration"`
		Writes                      []startWrite                     `json:"writes"`
		MissionBrief                missionBrief                     `json:"missionBrief"`
		ExecutorAction              executorActionSnapshot           `json:"executorAction"`
		MissionCommanderAction      missionCommanderActionSnapshot   `json:"missionCommanderAction"`
		MissionCommanderNextActions []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		NextSteps                   []string                         `json:"nextSteps"`
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
	if reconciled.ExecutorAction.Blocked || !reconciled.ExecutorAction.Ready || reconciled.ExecutorAction.OpenInterventions != 0 || reconciled.ExecutorAction.ReconcileRequired || reconciled.ExecutorAction.ResumeCommand != "/rekit continue login" || reconciled.ExecutorAction.HandoffCommand != "/rekit handoff login" {
		t.Fatalf("reconcile apply executor action drifted: %+v", reconciled.ExecutorAction)
	}
	if reconciled.MissionCommanderAction.State != "ready-to-continue" || reconciled.MissionCommanderAction.PrimaryCommand != "/rekit continue login" || !containsMissionCommanderNextAction(reconciled.MissionCommanderNextActions, "missionCommanderActions", "/rekit continue login", false, false) || !containsMissionCommanderNextAction(reconciled.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff login", false, false) {
		t.Fatalf("reconcile apply should expose top-level Mission Commander continue projection: action=%+v next=%+v", reconciled.MissionCommanderAction, reconciled.MissionCommanderNextActions)
	}
	if !slices.Equal(reconciled.NextSteps, []string{"run doctor after apply", "/rekit continue login"}) {
		t.Fatalf("reconcile apply ready lane next steps drifted: %+v", reconciled.NextSteps)
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
		MissionBrief   missionBrief           `json:"missionBrief"`
		ExecutorAction executorActionSnapshot `json:"executorAction"`
		OpenRisks      []string               `json:"openRisks"`
		Writes         []startWrite           `json:"writes"`
		NextSteps      []string               `json:"nextSteps"`
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
	if !result.ExecutorAction.Blocked || result.ExecutorAction.Ready || !slices.Equal(result.ExecutorAction.NextAgentActions, []string{"review open candidate/decision item(s) with evidence and authority boundary"}) || slices.Contains(result.NextSteps, "/rekit continue login") || !containsSubstring(result.NextSteps, "review open candidate/decision") {
		t.Fatalf("continue apply should expose post-apply open-decision next steps: action=%+v next=%+v", result.ExecutorAction, result.NextSteps)
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

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "vmp-re", "-Apply", "-Format", "text", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"工作线被 blocker 阻塞：feature-login reasons=open-decision", "executor next action：review open candidate/decision item(s) with evidence and authority boundary", "mission commander action queue：summary=total=2 unblocked=0 blocked=2 requiresReview=2 followUp=1 current=/rekit handoff login", "mission commander action queue current：state=needs-open-decision-review source=missionCommanderActions blocked=true requiresReview=true command=`/rekit handoff login`"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("continue blocked text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "已选择工作线：feature-login") || strings.Contains(out.String(), "executor next action：/rekit continue login") {
		t.Fatalf("continue blocked text should not recommend lane continuation:\n%s", out.String())
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

func TestRunPlanSubagentsReviewerOrchestrationE2E(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "login", "-Apply", "-Executor", "session-login", "-Actor", "mission-commander", "-Reason", "reviewer orchestration owner"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if start.Lane.ID != "feature-login" || start.Lane.CurrentExecutor != "session-login" || start.Lane.ExecutorGeneration != 1 {
		t.Fatalf("unexpected reviewer orchestration start: %+v", start)
	}
	writeCaseFile(t, caseRoot, "workspace/features/feature-login/review-evidence.md", "bounded reviewer evidence\n")

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-MaxParallel", "2", "-Lane", "feature-login"}, &out); err != nil {
		t.Fatal(err)
	}
	plan := decodePlanSubagentsResult(t, out.Bytes())
	packet := decodePlanSubagentsPacket(t, plan.PacketPath)
	if plan.ReviewerOrchestration.Mode != "manual-main-agent-intake" || plan.ReviewerOrchestration.TargetLane != "feature-login" || plan.ReviewerOrchestration.ReviewerCount != 2 || plan.ReviewerOrchestration.MaxParallel != 2 || len(plan.ReviewerOrchestration.Dispatches) != 2 || len(plan.ReviewerOrchestration.Lifecycle) != 5 {
		t.Fatalf("unexpected reviewer orchestration plan: %+v", plan.ReviewerOrchestration)
	}
	if plan.MissionCommanderAction.State != "ready-for-reviewer-dispatch" || plan.ReviewerOrchestration.MissionCommanderAction == nil || plan.ReviewerOrchestration.MissionCommanderAction.PrimaryCommand != plan.MissionCommanderAction.PrimaryCommand || !containsMissionCommanderNextAction(plan.MissionCommanderNextActions, "reviewerOrchestration.dispatch", plan.MissionCommanderAction.PrimaryCommand, false, true) || !containsMissionCommanderNextAction(plan.MissionCommanderNextActions, "reviewerOrchestration.batchIntake.preview", plan.ReviewerOrchestration.BatchPreviewCommand, true, true) || !containsMissionCommanderNextAction(plan.MissionCommanderNextActions, "reviewerOrchestration.batchIntake.apply", plan.ReviewerOrchestration.BatchApplyCommand, true, true) {
		t.Fatalf("plan-subagents omitted top-level Mission Commander reviewer plan guidance: action=%+v next=%+v orchestration=%+v", plan.MissionCommanderAction, plan.MissionCommanderNextActions, plan.ReviewerOrchestration)
	}
	assertCLIActionQueue(t, plan.MissionCommanderActionQueue, 4, 2, 2, 4, 0, plan.MissionCommanderAction.PrimaryCommand)
	if plan.ReviewerOrchestration.MissionCommanderActionQueue == nil {
		t.Fatalf("plan-subagents omitted nested reviewer orchestration action queue: %+v", plan.ReviewerOrchestration)
	}
	assertCLIActionQueue(t, *plan.ReviewerOrchestration.MissionCommanderActionQueue, 4, 2, 2, 4, 0, plan.MissionCommanderAction.PrimaryCommand)
	if packet.OwnerBinding.CurrentExecutor != "session-login" || packet.OwnerBinding.ExecutorGeneration != 1 || !packet.OwnerBinding.RequiredForIntake || packet.ReviewerOrchestration.Dispatches[0].ReviewerResultPath != packet.ShardHandoffs[0].ReviewerResultPath || !strings.Contains(packet.ReviewerOrchestration.Dispatches[0].PreviewCommand, "-WhatIf -Format json") || !strings.Contains(packet.ReviewerOrchestration.BatchPreviewCommand, "-ReadyReviewerResults") || !strings.Contains(packet.ReviewerOrchestration.BatchApplyCommand, "-Apply -Format json") || !strings.Contains(packet.ReviewerOrchestration.Lifecycle[0].Action, "does not spawn") || packet.ReviewerOrchestration.MissionCommanderAction == nil || len(packet.ReviewerOrchestration.MissionCommanderNextActions) != len(plan.MissionCommanderNextActions) || packet.ReviewerOrchestration.MissionCommanderActionQueue == nil {
		t.Fatalf("unexpected reviewer orchestration packet: %+v", packet)
	}
	assertCLIActionQueue(t, *packet.ReviewerOrchestration.MissionCommanderActionQueue, 4, 2, 2, 4, 0, plan.MissionCommanderAction.PrimaryCommand)

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var statusBeforeDispatch struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &statusBeforeDispatch); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	assertReviewerDispatchIntakeSummary(t, "status before reviewer result", statusBeforeDispatch.CaseMission.ReviewerDispatchIntakeSummary, 2, 2, 0, "shard-02", "waiting-for-reviewer-result")
	firstDispatch, ok := reviewerDispatchIntakeByShard(statusBeforeDispatch.CaseMission.ReviewerDispatchIntakeHandoffs, "shard-01")
	if !ok || firstDispatch.PacketPath != plan.PacketPath || firstDispatch.ReviewerResultPresent || firstDispatch.State != "waiting-for-reviewer-result" || !strings.Contains(firstDispatch.DispatchCommand, "dispatch read-only reviewer for shard-01") || !strings.Contains(firstDispatch.PreviewCommand, "-WhatIf -Format json") || !containsSubstring(firstDispatch.Boundary, "does not spawn") {
		t.Fatalf("status JSON omitted waiting reviewer dispatch intake handoff: %+v", statusBeforeDispatch.CaseMission.ReviewerDispatchIntakeHandoffs)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"status case mission reviewer dispatch intake summary：total=2 waitingForReviewerResult=2 readyForPreview=0", "status case mission reviewer dispatch intake：lane=feature-login shard=shard-01 state=waiting-for-reviewer-result", "status case mission reviewer dispatch intake dispatch：shard=shard-01 command=`dispatch read-only reviewer for shard-01"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status text omitted reviewer dispatch intake handoff %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "login", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "handoff reviewer dispatch intake summary：total=2 waitingForReviewerResult=2 readyForPreview=0") || !strings.Contains(out.String(), "handoff reviewer dispatch intake：lane=feature-login shard=shard-01 state=waiting-for-reviewer-result") {
		t.Fatalf("handoff text omitted reviewer dispatch intake handoff:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "login", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "continue reviewer dispatch intake summary：total=2 waitingForReviewerResult=2 readyForPreview=0") || !strings.Contains(out.String(), "continue reviewer dispatch intake：lane=feature-login shard=shard-01 state=waiting-for-reviewer-result") {
		t.Fatalf("continue text omitted reviewer dispatch intake handoff:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var continueApplyBeforeIntake struct {
		RunID                          string                               `json:"runId"`
		Applied                        bool                                 `json:"applied"`
		Blocked                        bool                                 `json:"blocked"`
		ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
		ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		Writes                         []startWrite                         `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &continueApplyBeforeIntake); err != nil {
		t.Fatalf("continue apply before reviewer intake JSON did not decode: %v\n%s", err, out.String())
	}
	assertReviewerDispatchIntakeSummary(t, "continue apply before reviewer intake", continueApplyBeforeIntake.ReviewerDispatchIntakeSummary, 2, 2, 0, "shard-02", "waiting-for-reviewer-result")
	if _, ok := reviewerDispatchIntakeByShard(continueApplyBeforeIntake.ReviewerDispatchIntakeHandoffs, "shard-01"); !ok {
		t.Fatalf("continue apply omitted open reviewer dispatch handoffs: %+v", continueApplyBeforeIntake.ReviewerDispatchIntakeHandoffs)
	}
	if !continueApplyBeforeIntake.Blocked || continueApplyBeforeIntake.Applied || continueApplyBeforeIntake.RunID != "run-preview" || len(continueApplyBeforeIntake.Writes) != 0 {
		t.Fatalf("continue apply did not fail closed before reviewer intake: %+v", continueApplyBeforeIntake)
	}
	if entries, err := os.ReadDir(filepath.Join(caseRoot, ".rekit", "runs")); err == nil && len(entries) != 0 {
		t.Fatalf("blocked continue apply created run artifacts: %+v", entries)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	for idx, handoff := range packet.ShardHandoffs {
		decision := "accept"
		verdict := "accepted"
		if idx == 1 {
			decision = "reject"
			verdict = "rejected"
		}
		data := reviewerResultForCLIPlan(t, packet, handoff, decision, verdict, "reviewer-session-"+handoff.ShardID)
		if err := os.WriteFile(handoff.ReviewerResultPath, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if idx == 0 {
			out.Reset()
			if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
				t.Fatal(err)
			}
			var statusWithReviewerResult struct {
				CaseMission struct {
					ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
					ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
				} `json:"caseMission"`
			}
			if err := json.Unmarshal(out.Bytes(), &statusWithReviewerResult); err != nil {
				t.Fatalf("status JSON with reviewer result did not decode: %v\n%s", err, out.String())
			}
			assertReviewerDispatchIntakeSummary(t, "status with reviewer result", statusWithReviewerResult.CaseMission.ReviewerDispatchIntakeSummary, 2, 1, 1, "shard-02", "waiting-for-reviewer-result")
			readyDispatch, ok := reviewerDispatchIntakeByShard(statusWithReviewerResult.CaseMission.ReviewerDispatchIntakeHandoffs, "shard-01")
			if !ok || !readyDispatch.ReviewerResultPresent || readyDispatch.State != "ready-for-reviewer-intake-preview" || !strings.Contains(readyDispatch.PreviewCommand, handoff.ReviewerResultPath) || !strings.Contains(readyDispatch.BatchPreviewCommand, "-ReadyReviewerResults") || !strings.Contains(statusWithReviewerResult.CaseMission.ReviewerDispatchIntakeSummary.NextAction, "-ReadyReviewerResults") || statusWithReviewerResult.CaseMission.ReviewerDispatchIntakeSummary.LatestBatchApplyCommand == "" || readyDispatch.DispatchOnly {
				t.Fatalf("status JSON omitted ready reviewer batch intake handoff: %+v", statusWithReviewerResult.CaseMission.ReviewerDispatchIntakeHandoffs)
			}
		}

		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", handoff.ReviewerResultPath, "-Lane", "feature-login", "-Actor", "mission-commander", "-WhatIf", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		preview := decodeReviewerIntakeResult(t, out.Bytes())
		if preview.Mode != "reviewer-intake" || preview.IsMutation || preview.Applied || preview.WritebackStatus != "previewed" || !preview.ReadyForWriteback || preview.OrchestrationSnapshot.DispatchIndex != idx+1 || preview.OrchestrationSnapshot.DispatchTotal != 2 || preview.OrchestrationSnapshot.ShardStatusAfter != "previewed" || preview.Verification == nil || preview.Decision == nil || !preview.PostValidation.Valid {
			t.Fatalf("unexpected reviewer intake preview for %s: %+v", handoff.ShardID, preview)
		}
		if idx == 0 && !slices.Contains(preview.OrchestrationSnapshot.NextDispatches, "shard-02") {
			t.Fatalf("preview did not identify remaining dispatch: %+v", preview.OrchestrationSnapshot)
		}
		previewRemaining := []string{handoff.ShardID}
		if idx == 0 {
			previewRemaining = []string{"shard-01", "shard-02"}
		}
		assertReviewerIntakeOrchestrationProgress(t, "reviewer intake preview for "+handoff.ShardID, preview.Summary.OrchestrationProgress, idx+1, 2, idx, 2-idx, handoff.ShardID, "previewed", handoff.ShardID, previewRemaining)
		if idx == 0 {
			out.Reset()
			if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", handoff.ReviewerResultPath, "-Lane", "feature-login", "-Actor", "mission-commander", "-WhatIf", "-Format", "text"}, &out); err != nil {
				t.Fatal(err)
			}
			for _, expected := range []string{
				"reviewer intake summary orchestration progress：dispatch=1/2 completed=0 open=2 current=shard-01 status=previewed nextOpen=shard-01 remaining=shard-01,shard-02",
				"reviewer intake summary orchestration boundary：orchestration progress is read-only; main Agent remains responsible for reviewer dispatch and intake ordering",
			} {
				if !strings.Contains(out.String(), expected) {
					t.Fatalf("reviewer intake text omitted orchestration progress %q:\n%s", expected, out.String())
				}
			}
		}

		out.Reset()
		if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", plan.PacketPath, "-ReviewerResultPath", handoff.ReviewerResultPath, "-Lane", "feature-login", "-Actor", "mission-commander", "-Apply", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
		applied := decodeReviewerIntakeResult(t, out.Bytes())
		if !applied.IsMutation || !applied.Applied || applied.WritebackStatus != "complete" || !applied.ReadyForWriteback || applied.OrchestrationSnapshot.DispatchIndex != idx+1 || applied.OrchestrationSnapshot.ShardStatusAfter != "complete" || applied.Verification == nil || applied.Decision == nil || !applied.PostValidation.Valid {
			t.Fatalf("unexpected reviewer intake apply for %s: %+v", handoff.ShardID, applied)
		}
		appliedRemaining := []string{}
		appliedNextOpen := ""
		if idx == 0 {
			appliedRemaining = []string{"shard-02"}
			appliedNextOpen = "shard-02"
		}
		assertReviewerIntakeOrchestrationProgress(t, "reviewer intake apply for "+handoff.ShardID, applied.Summary.OrchestrationProgress, idx+1, 2, idx+1, 1-idx, handoff.ShardID, "complete", appliedNextOpen, appliedRemaining)
		if idx == 0 {
			out.Reset()
			if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
				t.Fatal(err)
			}
			var statusAfterFirstIntake struct {
				CaseMission struct {
					ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
					ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
				} `json:"caseMission"`
			}
			if err := json.Unmarshal(out.Bytes(), &statusAfterFirstIntake); err != nil {
				t.Fatalf("status JSON after first reviewer intake did not decode: %v\n%s", err, out.String())
			}
			assertReviewerDispatchIntakeSummary(t, "status after first reviewer intake", statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeSummary, 1, 1, 0, "shard-02", "waiting-for-reviewer-result")
			if statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeSummary.LatestPacketDispatchTotal != 2 || statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeSummary.LatestPacketDispatchCompleted != 1 || statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeSummary.LatestPacketDispatchOpen != 1 || statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeSummary.LatestPacketNextOpenShardID != "shard-02" || statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeSummary.LatestCompletedShardID != "shard-01" || !slices.Equal(statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeSummary.RemainingShardIDs, []string{"shard-02"}) {
				t.Fatalf("status after first reviewer intake omitted packet progress: %+v", statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeSummary)
			}
			remainingDispatch, ok := reviewerDispatchIntakeByShard(statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeHandoffs, "shard-02")
			if !ok || remainingDispatch.DispatchTotal != 2 || remainingDispatch.DispatchCompleted != 1 || remainingDispatch.DispatchOpen != 1 || remainingDispatch.NextOpenShardID != "shard-02" || remainingDispatch.LatestCompletedShardID != "shard-01" || !slices.Equal(remainingDispatch.RemainingShardIDs, []string{"shard-02"}) {
				t.Fatalf("status after first reviewer intake omitted per-dispatch progress: %+v", statusAfterFirstIntake.CaseMission.ReviewerDispatchIntakeHandoffs)
			}

			out.Reset()
			if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login", "-Format", "json"}, &out); err != nil {
				t.Fatal(err)
			}
			var continueApplyAfterFirstIntake struct {
				RunID                          string                               `json:"runId"`
				Applied                        bool                                 `json:"applied"`
				Blocked                        bool                                 `json:"blocked"`
				ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
				ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
				Writes                         []startWrite                         `json:"writes"`
			}
			if err := json.Unmarshal(out.Bytes(), &continueApplyAfterFirstIntake); err != nil {
				t.Fatalf("continue apply after first reviewer intake JSON did not decode: %v\n%s", err, out.String())
			}
			assertReviewerDispatchIntakeSummary(t, "continue apply after first reviewer intake", continueApplyAfterFirstIntake.ReviewerDispatchIntakeSummary, 1, 1, 0, "shard-02", "waiting-for-reviewer-result")
			if continueApplyAfterFirstIntake.ReviewerDispatchIntakeSummary.LatestPacketDispatchCompleted != 1 || continueApplyAfterFirstIntake.ReviewerDispatchIntakeSummary.LatestPacketDispatchOpen != 1 || continueApplyAfterFirstIntake.ReviewerDispatchIntakeSummary.LatestPacketNextOpenShardID != "shard-02" || continueApplyAfterFirstIntake.ReviewerDispatchIntakeSummary.LatestCompletedShardID != "shard-01" {
				t.Fatalf("continue apply after first reviewer intake omitted progress summary: %+v", continueApplyAfterFirstIntake.ReviewerDispatchIntakeSummary)
			}
			if !continueApplyAfterFirstIntake.Blocked || continueApplyAfterFirstIntake.Applied || continueApplyAfterFirstIntake.RunID != "run-preview" || len(continueApplyAfterFirstIntake.Writes) != 0 {
				t.Fatalf("partial reviewer intake did not keep continue fail closed: %+v", continueApplyAfterFirstIntake)
			}
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var statusAfterIntake struct {
		CaseMission struct {
			ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem      `json:"reviewerDispatchIntakeHandoffs"`
			ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem `json:"reviewerDispatchIntakeSummary"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &statusAfterIntake); err != nil {
		t.Fatalf("post-intake status JSON did not decode: %v\n%s", err, out.String())
	}
	if statusAfterIntake.CaseMission.ReviewerDispatchIntakeSummary.Total != 0 || len(statusAfterIntake.CaseMission.ReviewerDispatchIntakeHandoffs) != 0 {
		t.Fatalf("completed reviewer dispatch intake should no longer be open: %+v", statusAfterIntake.CaseMission)
	}

	verifications := readOptionalCaseFile(t, caseRoot, ".rekit/facts/verifications.jsonl")
	decisions := readOptionalCaseFile(t, caseRoot, ".rekit/facts/decisions.jsonl")
	for _, expected := range []string{"reviewer-session-shard-01", "reviewer-session-shard-02", "ownerBindingMode", "current-executor-generation", "feature-login"} {
		if !strings.Contains(verifications, expected) || !strings.Contains(decisions, expected) {
			t.Fatalf("reviewer provenance %q missing:\nverifications=%s\ndecisions=%s", expected, verifications, decisions)
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "login"}, &out); err != nil {
		t.Fatal(err)
	}
	handoffResult := decodeHandoffResult(t, out.Bytes())
	latest := assertStartWrite(t, handoffResult.Writes, ".rekit/handovers/feature-login-latest.md", "write-latest-lane-handoff")
	handoffText, err := os.ReadFile(latest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"reviewerSession=reviewer-session-shard-01", "reviewerSession=reviewer-session-shard-02", "decision=accept", "decision=reject"} {
		if !strings.Contains(string(handoffText), expected) {
			t.Fatalf("reviewer orchestration handoff missing %q:\n%s", expected, string(handoffText))
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
	if packet.OwnerBinding.TargetLane != "devirt-main" || packet.OwnerBinding.BindingMode == "" || packet.ShardHandoffs[0].OwnerBinding.TargetLane != "devirt-main" {
		t.Fatalf("unexpected owner binding: result=%+v packet=%+v", result, packet)
	}
	if !slices.Contains(packet.Observability.BlockedActions, "runtime does not spawn subagents") || !strings.Contains(packet.ReviewLoop.VerdictWriteback, "plan-subagents -ReviewerResultPath") {
		t.Fatalf("unexpected review loop contract: %+v", packet)
	}
	if len(result.ShardHandoffs) != 2 || len(packet.ShardHandoffs) != 2 {
		t.Fatalf("missing shard handoffs: result=%+v packet=%+v", result.ShardHandoffs, packet.ShardHandoffs)
	}
	if result.ReviewerOrchestration.Mode != "manual-main-agent-intake" || packet.ReviewerOrchestration.Mode != result.ReviewerOrchestration.Mode || packet.ReviewerOrchestration.ReviewerCount != 2 || len(packet.ReviewerOrchestration.Dispatches) != 2 || len(packet.ReviewerOrchestration.Lifecycle) != 5 || packet.ReviewerOrchestration.Dispatches[0].ShardID != "shard-01" || !strings.Contains(packet.ReviewerOrchestration.Lifecycle[0].Action, "does not spawn") {
		t.Fatalf("unexpected reviewer orchestration: result=%+v packet=%+v", result.ReviewerOrchestration, packet.ReviewerOrchestration)
	}
	if result.ReviewerOrchestration.MissionCommanderActionQueue == nil || packet.ReviewerOrchestration.MissionCommanderActionQueue == nil || result.ReviewerOrchestration.MissionCommanderActionQueue.Summary != result.MissionCommanderActionQueue.Summary || packet.ReviewerOrchestration.MissionCommanderActionQueue.Summary != result.MissionCommanderActionQueue.Summary {
		t.Fatalf("reviewer orchestration omitted mirrored Mission Commander queue: result=%+v packet=%+v", result.ReviewerOrchestration, packet.ReviewerOrchestration)
	}
	if result.ReviewerOrchestration.Summary.Mode != "manual-main-agent-intake" || result.ReviewerOrchestration.Summary.TargetLane != "devirt-main" || result.ReviewerOrchestration.Summary.DispatchCount != 2 || !result.ReviewerOrchestration.Summary.IntakeAvailable || result.ReviewerOrchestration.Summary.DispatchOnly || result.ReviewerOrchestration.Summary.FirstDispatch == nil || result.ReviewerOrchestration.Summary.FirstDispatch.ShardID != "shard-01" || !strings.Contains(result.ReviewerOrchestration.Summary.FirstDispatch.DispatchCommand, "dispatch read-only reviewer for shard-01") || result.ReviewerOrchestration.Summary.CurrentAction == nil || result.ReviewerOrchestration.Summary.ActionTotal != 4 || !containsStringWith(result.ReviewerOrchestration.Summary.Boundary, "planning summary is read-only") {
		t.Fatalf("reviewer orchestration compact summary omitted takeover state: %+v", result.ReviewerOrchestration.Summary)
	}
	if packet.ReviewerOrchestration.Summary.DispatchCount != result.ReviewerOrchestration.Summary.DispatchCount || packet.ReviewerOrchestration.Summary.FirstDispatch == nil || packet.ReviewerOrchestration.Summary.CurrentAction == nil {
		t.Fatalf("packet reviewer orchestration compact summary drifted: result=%+v packet=%+v", result.ReviewerOrchestration.Summary, packet.ReviewerOrchestration.Summary)
	}
	assertCLIActionQueue(t, result.MissionCommanderActionQueue, 4, 2, 2, 4, 0, result.MissionCommanderAction.PrimaryCommand)
	firstHandoff := packet.ShardHandoffs[0]
	if !strings.Contains(packet.Shards[0].Prompt, "Return one reviewer result JSON object") || strings.Contains(packet.Shards[0].Prompt, "Return the route output contract only") {
		t.Fatalf("unexpected shard prompt: %+v", packet.Shards[0])
	}
	if firstHandoff.ShardID != "shard-01" || firstHandoff.Status != "planned" || strings.Join(firstHandoff.Items, ",") != "alpha,beta" || !strings.Contains(firstHandoff.DispatchPrompt, "read-only reviewer") || !strings.Contains(firstHandoff.DispatchPrompt, "Return exactly one reviewer result JSON object; do not return routeOutput alone") || !strings.Contains(firstHandoff.DispatchPrompt, "Reviewer result JSON skeleton:") || !strings.Contains(firstHandoff.DispatchPrompt, "\"packetId\":\"packet.packetId\"") || !strings.Contains(firstHandoff.DispatchPrompt, "Route output required fields: item=alpha,beta") || !strings.Contains(firstHandoff.DispatchPrompt, "tool_scope=read-only") || !strings.Contains(firstHandoff.DispatchPrompt, "Keep routeOutput.decision and routeOutput.confidence equal to the top-level decision/confidence") || !strings.Contains(firstHandoff.DispatchPrompt, "Do not write files") || !strings.Contains(firstHandoff.ExpectedOutput, "decision") || !strings.Contains(firstHandoff.ReviewerWriteback, "plan-subagents -ReviewerResultPath") || !strings.Contains(firstHandoff.MainAgentNextAction, "reviewerResultContract") || !strings.Contains(firstHandoff.MainAgentNextAction, "previewCommand") || !strings.Contains(firstHandoff.MainAgentNextAction, "applyCommand") || !slices.Contains(firstHandoff.ReadOnlyBoundary, "runtime does not spawn subagents") || !slices.Contains(firstHandoff.CompletionCriteria, "reviewer verdicts are recorded in the ledger before main merge decisions") || firstHandoff.FailureHandling == "" {
		t.Fatalf("unexpected shard handoff: %+v", firstHandoff)
	}
	if firstHandoff.ReviewerResultContract.OutputFormat == "" || !slices.Contains(firstHandoff.ReviewerResultContract.RequiredFields, "reviewerSession") || !slices.Contains(firstHandoff.ReviewerResultContract.RequiredFields, "recommendedVerdict") || !slices.Contains(firstHandoff.ReviewerResultContract.RequiredFields, "routeOutput") || !slices.Contains(firstHandoff.ReviewerResultContract.AllowedDecisions, "needs-more-evidence") || !slices.Contains(firstHandoff.ReviewerResultContract.ConflictSignals, "reviewer requests file writes, ledger append, authority/confirmed changes, heavy tools, or external effects") {
		t.Fatalf("unexpected reviewer result contract: %+v", firstHandoff.ReviewerResultContract)
	}
	if !slices.Contains(firstHandoff.IntakeChecklist, "validate reviewer output against reviewerResultContract before using any writeback template") || !slices.Contains(firstHandoff.IntakeChecklist, "defer the main decision when conflicts, missing evidence, or blocked outputs are present") || !slices.Contains(firstHandoff.IntakeChecklist, "run reviewerIntakeCommands.previewCommand before applyCommand and inspect verification / decision / postValidation before ledger writeback") || !slices.Contains(firstHandoff.IntakeChecklist, "use reviewerIntakeCommands.repairGuidance when preview returns blocked, event-id-collision, or post-validation failed") {
		t.Fatalf("unexpected intake checklist: %+v", firstHandoff.IntakeChecklist)
	}
	if len(firstHandoff.ReviewerDecisionMappings) != 5 || firstHandoff.ReviewerDecisionMappings[0].ReviewerDecision != "accept" || firstHandoff.ReviewerDecisionMappings[0].VerificationVerdict != "accepted" || firstHandoff.ReviewerDecisionMappings[0].MainDecision != "accept" || firstHandoff.ReviewerDecisionMappings[3].ReviewerDecision != "abandon" || firstHandoff.ReviewerDecisionMappings[3].MainDecision != "supersede" || firstHandoff.ReviewerDecisionMappings[4].ReviewerDecision != "needs-more-evidence" || firstHandoff.ReviewerDecisionMappings[4].VerificationVerdict != "needs-more-evidence" || firstHandoff.ReviewerDecisionMappings[4].MainDecision != "defer" {
		t.Fatalf("unexpected reviewer decision mappings: %+v", firstHandoff.ReviewerDecisionMappings)
	}
	if !slices.Contains(firstHandoff.ConflictHandling, "if any conflictSignal is present, map verification verdict to inconclusive or needs-more-evidence and keep main decision deferred unless independently resolved") || !slices.Contains(firstHandoff.ConflictHandling, "if reviewer requests writes, heavy tools, authority/confirmed changes, or external effects, discard that output for ledger purposes and escalate through the lane gate path") {
		t.Fatalf("unexpected conflict handling: %+v", firstHandoff.ConflictHandling)
	}
	if len(firstHandoff.WritebackSequence) != 5 || firstHandoff.WritebackSequence[0].Step != "validate-reviewer-result" || firstHandoff.WritebackSequence[2].Step != "preview-reviewer-intake" || firstHandoff.WritebackSequence[3].Step != "apply-reviewer-intake" || firstHandoff.WritebackSequence[4].Step != "post-review-validation" {
		t.Fatalf("unexpected writeback sequence: %+v", firstHandoff.WritebackSequence)
	}
	if !slices.Contains(firstHandoff.WritebackSequence[0].Uses, "reviewerResultContract") || !slices.Contains(firstHandoff.WritebackSequence[2].Uses, "reviewerIntakeCommands.previewCommand") || !slices.Contains(firstHandoff.WritebackSequence[2].BlockedBy, "wrong packet/case/pack/shard/items") || firstHandoff.WritebackSequence[3].NextOnSuccess != "post-review-validation" {
		t.Fatalf("unexpected writeback sequence details: %+v", firstHandoff.WritebackSequence)
	}
	if len(firstHandoff.WritebackSequence[2].CommandBindings) != 1 || firstHandoff.WritebackSequence[2].CommandBindings[0].Binding != "reviewer-intake-preview" || firstHandoff.WritebackSequence[2].CommandBindings[0].Kind != "reviewer-intake" || !strings.Contains(firstHandoff.WritebackSequence[2].CommandBindings[0].Command, "/rekit plan-subagents") || !strings.Contains(firstHandoff.WritebackSequence[2].CommandBindings[0].Command, "-ReviewerResultPath") || !strings.Contains(firstHandoff.WritebackSequence[2].CommandBindings[0].Command, "-WhatIf -Format json") || !slices.Contains(firstHandoff.WritebackSequence[2].CommandBindings[0].RequiredFields, "reviewerResultPath") {
		t.Fatalf("unexpected reviewer intake preview command binding: %+v", firstHandoff.WritebackSequence[2].CommandBindings)
	}
	if len(firstHandoff.WritebackSequence[3].CommandBindings) != 1 || firstHandoff.WritebackSequence[3].CommandBindings[0].Binding != "reviewer-intake-apply" || firstHandoff.WritebackSequence[3].CommandBindings[0].Kind != "reviewer-intake" || !strings.Contains(firstHandoff.WritebackSequence[3].CommandBindings[0].Command, "-Apply -Format json") || firstHandoff.WritebackSequence[3].NextOnFailure != "retry-same-intake-to-complete-writeback" {
		t.Fatalf("unexpected reviewer intake apply command binding: %+v", firstHandoff.WritebackSequence[3])
	}
	commands := firstHandoff.ReviewerIntakeCommands
	if !strings.Contains(commands.PreviewCommand, "/rekit plan-subagents") || !strings.Contains(commands.PreviewCommand, "-PacketPath") || !strings.Contains(commands.PreviewCommand, "-ReviewerResultPath") || !strings.Contains(commands.PreviewCommand, "-Lane \"devirt-main\"") || !strings.Contains(commands.PreviewCommand, "-Actor <main-agent>") || !strings.Contains(commands.PreviewCommand, "-WhatIf -Format json") || !strings.Contains(commands.ApplyCommand, "-Apply -Format json") || strings.Contains(commands.ApplyCommand, "note -Kind") || !slices.Contains(commands.RequiredFields, "packetPath") || !slices.Contains(commands.RequiredFields, "reviewerResultPath") || !slices.Contains(commands.RequiredFields, "targetLane") || !slices.Contains(commands.PreviewChecks, "confirm reviewer intake returns isMutation=false, applied=false, and readyForWriteback=true") || !slices.Contains(commands.PreviewChecks, "if reviewer intake returns blocked, event-id-collision, or complete-post-validation-failed, consume repairGuidance[] action/evidence/boundary before rerunning previewCommand") || !slices.Contains(commands.BlockedOutputs, "reviewer intake must not execute heavy tools or write authority/confirmed state") || !containsCLIRepairGuidance(commands.RepairGuidance, "reviewer result reports unresolved conflicts", "resolve or split") {
		t.Fatalf("unexpected reviewer intake commands: %+v", commands)
	}
	if !slices.Contains(firstHandoff.PostReviewMerge, "run reviewerIntakeCommands.previewCommand and inspect verification, decision, and postValidation before applyCommand") || !slices.Contains(firstHandoff.PostReviewMerge, "retry the identical applyCommand when an interrupted writeback needs idempotent completion") {
		t.Fatalf("unexpected post-review merge guidance: %+v", firstHandoff.PostReviewMerge)
	}
	summary, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatalf("missing summary: %v", err)
	}
	for _, expected := range []string{"## bounded dispatch observability", "### reviewer orchestration", "reviewer orchestration summary:", "reviewer orchestration summary batch intake:", "-ReadyReviewerResults", "reviewer orchestration summary first dispatch:", "reviewer orchestration summary current action:", "reviewer orchestration summary next action:", "reviewer orchestration summary boundary:", "intakeAvailable=`true`; collectionAvailable=`true`; dispatchOnly=`false`", "mission commander action queue:", "orchestration-step:", "reviewer-dispatch:", "route selected by", "shard-01: `planned`", "runtime does not spawn subagents", "verdict writeback", "reviewer result path:", "reviewer result skeleton:", "\"packetId\":\"packet-", "reviewer routeOutput field hints:", "tool_scope=read-only", "reviewer result binding: packetId=`packet-", "routeOutput evidence inside evidenceRefs", "reviewer result contract", "evidence-rule:", "conflict-signal:", "intake-check:", "decision-map:", "conflict-handling:", "writeback-step:", "command-binding:", "writeback-blocker:", "reviewer intake preview", "-ReviewerResultPath", "-WhatIf -Format json", "preview-check:", "post-review:"} {
		if !strings.Contains(string(summary), expected) {
			t.Fatalf("summary missing %q:\n%s", expected, string(summary))
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "plan-subagents", "-Target", caseRoot, "-Pack", "vmp-re", "-TaskType", "feature-analysis", "-Items", "alpha,beta", "-ItemsPerAgent", "1", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"plan-subagents：writesReviewArtifacts=true reviewRequired=true items=2 shards=2",
		"plan-subagents reviewer orchestration：mode=manual-main-agent-intake",
		"plan-subagents reviewer orchestration summary：mode=manual-main-agent-intake targetLane=devirt-main reviewers=2 dispatches=2 maxParallel=2 intakeAvailable=true collectionAvailable=true dispatchOnly=false",
		"plan-subagents reviewer orchestration summary owner：targetLane=devirt-main mode=attached-case-board-missing currentExecutor=unassigned generation=0 requiredForIntake=false spawnOwner=main-agent",
		"plan-subagents reviewer orchestration summary batch intake：preview=`/rekit plan-subagents",
		"-ReadyReviewerResults",
		"plan-subagents reviewer orchestration summary first dispatch：shard=shard-01 status=planned",
		"plan-subagents reviewer orchestration summary current action：state=ready-for-reviewer-dispatch source=reviewerOrchestration.dispatch blocked=false requiresReview=true command=`dispatch read-only reviewer for shard-01",
		"plan-subagents reviewer orchestration summary next action：state=ready-for-reviewer-batch-intake-preview source=reviewerOrchestration.batchIntake.preview blocked=true requiresReview=true",
		"plan-subagents reviewer orchestration summary boundary：planning summary is read-only; full reviewerOrchestration dispatches, lifecycle, action queue, and shard handoffs remain available",
		"plan-subagents reviewer orchestration scope：scope=dispatch read-only reviewers, save one JSON candidate per shard, publish immutable canonical results with collection preview/apply, then run packet-level ready-result batch intake preview/apply packet=",
		"plan-subagents reviewer orchestration owner：targetLane=devirt-main mode=attached-case-board-missing currentExecutor=unassigned generation=0 requiredForIntake=false spawnOwner=main-agent",
		"plan-subagents reviewer orchestration lifecycle：step=dispatch-reviewers owner=main-agent inputs=reviewerOrchestration.dispatches[].dispatchPrompt,ownerBinding,packetPath mustPass=one reviewerSession is assigned per reviewer result,reviewers receive only read-only boundary and shard items,no reviewer writes files or ledgers nextOnSuccess=collect-results",
		"plan-subagents reviewer orchestration boundary：boundary=runtime does not spawn subagents",
		"plan-subagents reviewer orchestration completion：criteria=each planned shard is accepted, rejected, deferred, or explicitly abandoned",
		"plan-subagents reviewer dispatch：shard=shard-01 status=planned",
		"plan-subagents shard handoff：shard=shard-01 status=planned",
		"plan-subagents shard owner binding：shard=shard-01 targetLane=devirt-main mode=attached-case-board-missing currentExecutor=unassigned generation=0 requiredForIntake=false spawnOwner=main-agent",
		"plan-subagents shard owner boundary：shard=shard-01 boundary=runtime only records reviewer owner provenance; it does not spawn, stop, monitor, or manage reviewer/member sessions",
		"plan-subagents reviewer writeback：shard=shard-01 handoff=/rekit plan-subagents -ReviewerResultPath ... -WhatIf/-Apply validates reviewer results and writes verification-before-decision facts for the main agent",
		"plan-subagents shard next action：shard=shard-01 action=launch a read-only reviewer with agentToolRequest.prompt, inspect its JSON against reviewerResultContract, save the single JSON object at reviewerResultCandidatePath, run reviewerCollectionCommands.previewCommand then applyCommand, then use reviewerIntakeCommands or packet-level batch intake WhatIf before Apply",
		"plan-subagents shard agent tool request：shard=shard-01 tool=Claude Code Agent agentType=read-only-reviewer readOnly=true expectedOutput=exactly one ReviewerResult JSON object; no Markdown fence or surrounding prose",
		"plan-subagents reviewer result candidate：shard=shard-01 path=",
		"canonical=",
		"plan-subagents reviewer collection command：shard=shard-01 candidate=",
		"-CollectReviewerResult",
		"-ShardId \"shard-01\"",
		"-WhatIf -Format json` apply=`/rekit plan-subagents",
		"-Apply -Format json`",
		"plan-subagents shard dispatch prompt：shard=shard-01 prompt=You are a read-only reviewer for rekit plan-subagents shard shard-01.",
		"plan-subagents shard boundary：shard=shard-01 boundary=runtime does not spawn subagents",
		"plan-subagents reviewer result contract：shard=shard-01 format=single JSON object per shard",
		"required=packetId,routeId,shardId,items,reviewerSession,decision,confidence,summary,evidenceRefs,risks,conflicts,recommendedVerdict,routeOutput decisions=accept,reject,defer,abandon,needs-more-evidence",
		"plan-subagents reviewer result skeleton：shard=shard-01 packetId=packet-",
		"routeId=vmp-re:lane-feature-analysis reviewerResultPath=",
		"json={\"packetId\":\"packet-",
		"\"routeId\":\"vmp-re:lane-feature-analysis\"",
		"\"shardId\":\"shard-01\"",
		"\"reviewerSession\":\"reviewer-session-id\"",
		"\"evidenceRefs\":[\"packet-",
		"\"tool_scope\":\"read-only\"",
		"plan-subagents reviewer result routeOutput field：shard=shard-01 field=item required=true valueHint=alpha",
		"plan-subagents reviewer result routeOutput field：shard=shard-01 field=evidence required=true valueHint=packet-",
		"plan-subagents reviewer result routeOutput field：shard=shard-01 field=tool_scope required=true valueHint=read-only",
		"plan-subagents reviewer result routeOutput field：shard=shard-01 field=defer_reason required=true valueHint=fill defer reason",
		"plan-subagents reviewer result evidence rule：shard=shard-01 rule=accepted or rejected reviewer decisions must cite evidenceRefs",
		"plan-subagents reviewer result conflict signal：shard=shard-01 signal=reviewer requests file writes, ledger append, authority/confirmed changes, heavy tools, or external effects",
		"plan-subagents reviewer intake command：shard=shard-01 purpose=strictly validate one reviewer result, preview or append verification-before-decision events, and return overview/handoff/doctor post-validation",
		"plan-subagents reviewer intake preview check：shard=shard-01 check=confirm reviewer intake returns isMutation=false, applied=false, and readyForWriteback=true",
		"plan-subagents reviewer intake preview check：shard=shard-01 check=if reviewer intake returns blocked, event-id-collision, or complete-post-validation-failed, consume repairGuidance[] action/evidence/boundary before rerunning previewCommand",
		"plan-subagents reviewer intake repair guidance：shard=shard-01 reason=reviewer result reports unresolved conflicts action=resolve or split",
		"plan-subagents reviewer intake repair evidence：shard=shard-01 reason=reviewer result reports unresolved conflicts evidence=ReviewerResult.conflicts[]",
		"plan-subagents reviewer intake repair boundary：shard=shard-01 reason=reviewer result reports unresolved conflicts boundary=do not apply reviewer intake until this blocker is resolved",
		"plan-subagents reviewer intake checklist：shard=shard-01 item=use reviewerIntakeCommands.repairGuidance when preview returns blocked, event-id-collision, or post-validation failed",
		"plan-subagents reviewer intake checklist：shard=shard-01 item=validate reviewer output against reviewerResultContract before using any writeback template",
		"plan-subagents reviewer decision map：shard=shard-01 reviewer=accept verification=accepted main=accept",
		"plan-subagents reviewer conflict handling：shard=shard-01 handling=if reviewer requests writes, heavy tools, authority/confirmed changes, or external effects, discard that output for ledger purposes and escalate through the lane gate path",
		"plan-subagents reviewer writeback step：shard=shard-01 step=preview-reviewer-intake owner=main-agent uses=reviewerIntakeCommands.previewCommand mustPass=reviewer intake returns isMutation=false,reviewer intake returns applied=false",
		"plan-subagents reviewer writeback command binding：shard=shard-01 step=preview-reviewer-intake binding=reviewer-intake-preview source=reviewerIntakeCommands.previewCommand kind=reviewer-intake command=`/rekit plan-subagents",
		"plan-subagents reviewer post-review：shard=shard-01 item=run reviewerIntakeCommands.previewCommand and inspect verification, decision, and postValidation before applyCommand",
		"plan-subagents commander action：state=ready-for-reviewer-dispatch",
		"mission commander action queue：summary=total=4 unblocked=2 blocked=2 requiresReview=4 followUp=0 current=dispatch read-only reviewer for shard-01",
		"mission commander action queue current：state=ready-for-reviewer-dispatch source=reviewerOrchestration.dispatch blocked=false requiresReview=true command=`dispatch read-only reviewer for shard-01",
		"mission commander next action：state=ready-for-reviewer-batch-intake-preview source=reviewerOrchestration.batchIntake.preview blocked=true requiresReview=true",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("plan-subagents text output missing %q:\n%s", expected, out.String())
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
	commands := packet.ShardHandoffs[0].ReviewerIntakeCommands
	if packet.ReviewerOrchestration.Summary.CollectionAvailable || packet.ShardHandoffs[0].ReviewerCollectionCommands != nil || packet.ShardHandoffs[0].ReviewerResultCandidatePath != "" {
		t.Fatalf("out-of-case plan advertised reviewer collection: %+v", packet.ReviewerOrchestration)
	}
	if !strings.Contains(commands.PreviewCommand, "n/a: reviewer intake requires an attached rekit case") || !strings.Contains(commands.ApplyCommand, "n/a: reviewer intake requires an attached rekit case") || !slices.Contains(commands.PreviewChecks, "out-of-case review artifacts are dispatch-only; reviewer intake/writeback is unavailable until the target is an attached rekit case") || !slices.Contains(commands.BlockedOutputs, "out-of-case plan packets must not be presented as immediately runnable reviewer intake commands") {
		t.Fatalf("out-of-case plan exposed runnable reviewer intake commands: %+v", commands)
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
	if !strings.Contains(packet.ReviewLoop.VerdictWriteback, "plan-subagents -ReviewerResultPath") || len(packet.Observability.BlockedActions) == 0 {
		t.Fatalf("template route missing review loop contract: %+v", packet)
	}
	if !packet.ReviewerOrchestration.Summary.IntakeAvailable || packet.ReviewerOrchestration.Summary.CollectionAvailable || packet.ShardHandoffs[0].ReviewerCollectionCommands != nil || packet.ShardHandoffs[0].ReviewerResultCandidatePath != "" || strings.Contains(packet.ShardHandoffs[0].MainAgentNextAction, "run reviewerCollectionCommands") {
		t.Fatalf("attached custom artifact overclaimed collection capability: %+v", packet.ReviewerOrchestration)
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

	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"promote apply：mutation=false applied=false changed=",
		"promote apply write：path=references/template/README.md kind=managed-doc action=would-promote",
		"promote apply denied action：authority/confirmed writes",
		"promote apply next step：review would-promote entries before rerunning with -Apply",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("promote apply what-if text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("promote apply what-if text should not emit JSON:\n%s", out.String())
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

func TestRunPromoteApplyTextOutputsWritesValidationAndNextSteps(t *testing.T) {
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
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Apply text\n\nReusable safe text update.\n")
	writeCaseFile(t, caseRoot, "references/template/workflow-template.md", "# Blocked\n\nDo not promote C:\\case\\artifact\\sample-trace.csv.\n")

	var out bytes.Buffer
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"promote apply：mutation=true applied=true changed=",
		"blocked=1",
		"cleanup=true",
		"promote apply write：path=references/template/README.md kind=managed-doc action=promote",
		"promote apply write：path=references/template/workflow-template.md kind=managed-doc action=blocked-deny-pattern",
		"reason=C:\\\\",
		"promote apply validation row：file=",
		"promote apply denied action：authority/confirmed writes",
		"promote apply next step：run doctor after apply",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("promote apply text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("promote apply text should not emit JSON:\n%s", out.String())
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
	if result.ReviewPlan.Mode != "candidate-review-preview" || result.ReviewPlan.ItemCount != len(result.Writes) || len(result.ReviewPlan.DecisionChecklist) != len(result.Writes) || len(result.ReviewPlan.DecisionFollowThrough) != len(result.Writes) || len(result.ReviewPlan.CleanupTargets) == 0 || !containsSubstring(result.ReviewPlan.RuntimeBoundary, "when not WhatIf") {
		t.Fatalf("unexpected promote candidates what-if review plan: %+v", result.ReviewPlan)
	}
	if summary := result.ReviewPlan.ReviewSummary; summary.Mode != "candidate-review-preview" || summary.Total != len(result.Writes) || summary.PendingReviewCount == 0 || summary.BlockedCount != result.Blocked || summary.CleanupTargetCount == 0 || summary.ReviewArtifactCount != len(result.ReviewPlan.ReviewArtifacts) || summary.DecisionChecklistCount != len(result.ReviewPlan.DecisionChecklist) || summary.DecisionFollowThroughCount != len(result.ReviewPlan.DecisionFollowThrough) || summary.ExecutionStepCount != len(result.ReviewPlan.MainAgentExecutionPlan) || summary.ReconsumeCheckCount != len(result.ReviewPlan.Reconsume.VerificationChecklist) || summary.NextActionCount != len(result.ReviewPlan.MissionCommanderNextActions) || !summary.RequiresReview || summary.RequiresCleanup || !summary.HasIndex || !summary.HasDecisionArtifacts || !summary.HasCleanupArtifacts || !summary.HasReconsumeArtifacts || summary.ProofSummary.Total != len(result.ReviewPlan.ReviewArtifacts) || summary.ProofSummary.Present != 0 || summary.ProofSummary.Missing != len(result.ReviewPlan.ReviewArtifacts) || summary.ProofSummary.CurrentStage != "decision-proof-required" || summary.ProofSummary.NextMissingProofType != "candidate-decision-note" || !strings.HasSuffix(summary.ProofSummary.NextMissingProofPath, ".candidate-decision-note.md") || summary.ProofSummary.NextMissingProof == nil || summary.ProofSummary.NextMissingProof.ProofType != "candidate-decision-note" || summary.ProofSummary.NextMissingProof.Stage != "decision-proof-required" || !strings.Contains(summary.ProofSummary.NextMissingProof.Action, "selected decisionFollowThrough outcome") || !containsSubstring(summary.ProofSummary.NextMissingProof.Boundary, "WhatIf did not create") || !containsSubstring(summary.ProofSummary.Boundary, "WhatIf did not create") || !summary.WhatIf || !containsSubstring(summary.Boundary, "reviewSummary is read-only") || !containsSubstring(summary.Boundary, "WhatIf did not write") {
		t.Fatalf("promote what-if review summary missing compact handoff: %+v", summary)
	}
	if !candidateJSONDecisionFollowThroughContains(result.ReviewPlan.DecisionFollowThrough, "references/template/README.md", "accept", "promote -Apply is not a candidate-scoped accept path") || !candidateJSONDecisionFollowThroughContains(result.ReviewPlan.DecisionFollowThrough, "references/template/README.md", "reject", "update or remove indexPath") {
		t.Fatalf("promote what-if review plan missing decision follow-through: %+v", result.ReviewPlan.DecisionFollowThrough)
	}
	if !candidateJSONReviewArtifactContains(result.ReviewPlan.ReviewArtifacts, "references/template/README.md", "candidate-decision-note", "selected decisionFollowThrough outcome") || !candidateJSONReviewArtifactContains(result.ReviewPlan.ReviewArtifacts, "references/template/README.md", "candidate-cleanup-proof", "WhatIf did not create candidatePath") {
		t.Fatalf("promote what-if review plan missing review artifact handoff: %+v", result.ReviewPlan.ReviewArtifacts)
	}
	if !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "collect-review-artifacts", "replacement executor can resume") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "materialize-candidates", "promote -Target <attached-case>") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "review-decisions", "matching decisionFollowThrough outcome") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "cleanup-rejected-or-merged-candidates", "indexPath") {
		t.Fatalf("promote what-if review plan missing main agent execution handoff: %+v", result.ReviewPlan.MainAgentExecutionPlan)
	}
	if !candidateJSONNextActionContains(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.decisionChecklist", "decisionFollowThrough") || !candidateJSONNextActionContains(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.cleanupTargets", "delete candidatePath") || !candidateJSONNextActionContains(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.reconsume.verificationChecklist", "doctor -Pack _template") || !candidateJSONNextActionBoundaryContains(result.ReviewPlan.MissionCommanderNextActions, "WhatIf did not write") || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction == nil || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction.Source != "reviewPlan.decisionChecklist" {
		t.Fatalf("promote what-if review plan missing Mission Commander next actions/queue: actions=%+v queue=%+v", result.ReviewPlan.MissionCommanderNextActions, result.ReviewPlan.MissionCommanderActionQueue)
	}
	commander := result.ReviewPlan.MissionCommanderAction
	if commander.State != "preview-pack-memory-candidates" || commander.PrimaryCommand != "review reviewPlan.reviewItems" || !strings.Contains(commander.Prompt, "WhatIf preview") || !containsSubstring(commander.FollowUpCommands, "promote -CreateCandidates") || !containsSubstring(commander.Boundary, "WhatIf did not write") || !containsSubstring(commander.Boundary, "no authority/confirmed") || !containsSubstring(commander.Boundary, "no heavy-tool") {
		t.Fatalf("promote what-if omitted Mission Commander preview handoff: %+v", commander)
	}
	if _, err := os.Stat(result.Writes[0].TargetPath); !os.IsNotExist(err) {
		t.Fatalf("promote candidates what-if created %s", result.Writes[0].TargetPath)
	}

	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-CreateCandidates", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{
		"promote candidates：applied=false created=2",
		"promote candidates review summary：mode=candidate-review-preview pack=_template",
		"pending=2",
		"created=2",
		"managedDocs=4 toolingCandidates=1 cleanupTargets=2",
		"proofProgress=0/8 currentStage=decision-proof-required nextMissingProofType=candidate-decision-note",
		"nextMissingProofPath=packs/_template/promote-candidates/review-artifacts/",
		"promote candidates review proof summary：progress=0/8 total=8 present=0 missing=8 decisionMissing=2 cleanupMissing=2 reconsumeMissing=4 currentStage=decision-proof-required nextMissingProofType=candidate-decision-note",
		"promote candidates review next missing proof：stage=decision-proof-required proofType=candidate-decision-note path=packs/_template/promote-candidates/review-artifacts/",
		"promote candidates review next missing proof evidence：selected decisionFollowThrough outcome",
		"promote candidates review next missing proof boundary：WhatIf did not create candidatePath",
		"promote candidates review proof summary boundary：WhatIf did not create candidatePath, indexPath, or proof files",
		"promote candidates review proof summary boundary：proofSummary is read-only; promote create-candidates reports expected proof files but does not create or validate their contents",
		"promote candidates review summary boundary：WhatIf did not write candidate files or indexPath",
		"promote candidates review summary boundary：reviewSummary is read-only; full reviewPlan arrays remain available",
		"promote candidates review item：path=references/template/README.md kind=managed-doc decision=pending-review action=would-create-candidate",
		"promote candidates review item merge hint：path=references/template/README.md hint=merge accepted reusable guidance into pack managed doc packTarget",
		"promote candidates review checklist：path=references/template/README.md decision=pending-review reviewAction=inspect candidatePath against packTarget and choose accept, reject, or superseded",
		"promote candidates checklist boundary：path=references/template/README.md boundary=do not write authority/confirmed",
		"promote candidates review artifact：path=references/template/README.md kind=managed-doc name=candidate-decision-note",
		"promote candidates review artifact evidence：path=references/template/README.md name=candidate-decision-note evidence=selected decisionFollowThrough outcome",
		"promote candidates review artifact boundary：path=references/template/README.md name=candidate-cleanup-proof boundary=WhatIf did not create candidatePath",
		"promote candidates execution step：name=collect-review-artifacts when=while reviewing candidates and before declaring cleanup/reconsume complete",
		"promote candidates execution step：name=materialize-candidates when=after reviewing WhatIf preview scope and confirming candidate generation is still desired",
		"promote candidates execution command：name=materialize-candidates command=go run ./cmd/rekit -- -Command promote -Target <attached-case> -Pack _template -CreateCandidates -Format json",
		"promote candidates execution boundary：name=materialize-candidates boundary=WhatIf did not write candidate files or indexPath",
		"promote candidates decision when：path=references/template/README.md decision=accept when=after candidatePath is reviewed as reusable, case-neutral content",
		"promote candidates decision evidence：path=references/template/README.md decision=accept evidence=pack source diff",
		"promote candidates decision follow-through boundary：path=references/template/README.md boundary=decisionFollowThrough is guidance only; runtime does not execute merge, cleanup, init, or doctor commands",
		"promote candidates cleanup action detail：path=references/template/README.md candidatePath=",
		"promote candidates reconsume top-level command：go run ./cmd/rekit -- -Command doctor -Pack _template",
		"promote candidates reconsume check：name=pack-doctor-after-merge when=after accepting any managed-doc or tooling candidate into pack sources expected=pack doctor passes with merged reusable content and no case-specific residue",
		"promote candidates reconsume evidence：name=pack-doctor-after-merge evidence=doctor command output",
		"promote candidates commander action：state=preview-pack-memory-candidates",
		"mission commander action queue：summary=total=7 unblocked=7 blocked=0 requiresReview=3 followUp=0 current=review reviewPlan.decisionChecklist",
		"mission commander next action boundary：WhatIf did not write candidate files or indexPath",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("promote create-candidates what-if text output missing %q:\n%s", expected, text)
		}
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
	if result.ReviewPlan.Mode != "candidate-review" || result.ReviewPlan.ItemCount != len(result.Writes) || len(result.ReviewPlan.DecisionChecklist) != len(result.Writes) || len(result.ReviewPlan.DecisionFollowThrough) != len(result.Writes) || len(result.ReviewPlan.CleanupTargets) != 2 || result.ReviewPlan.Reconsume.Mode != "pack-memory-reconsume-after-merge" {
		t.Fatalf("unexpected promote candidates review plan: %+v", result.ReviewPlan)
	}
	if summary := result.ReviewPlan.ReviewSummary; summary.Mode != "candidate-review" || summary.Total != len(result.Writes) || summary.PendingReviewCount != 2 || summary.BlockedCount != result.Blocked || summary.ManagedDocCount != 4 || summary.ToolingCandidateCount != 1 || summary.CleanupTargetCount != 2 || summary.ReviewArtifactCount != len(result.ReviewPlan.ReviewArtifacts) || summary.DecisionChecklistCount != len(result.ReviewPlan.DecisionChecklist) || summary.DecisionFollowThroughCount != len(result.ReviewPlan.DecisionFollowThrough) || summary.ExecutionStepCount != len(result.ReviewPlan.MainAgentExecutionPlan) || summary.ReconsumeCheckCount != len(result.ReviewPlan.Reconsume.VerificationChecklist) || summary.NextActionCount != len(result.ReviewPlan.MissionCommanderNextActions) || !summary.RequiresReview || !summary.RequiresCleanup || !summary.HasToolingCandidate || !summary.HasBlockedItems || !summary.HasIndex || !summary.HasDecisionArtifacts || !summary.HasCleanupArtifacts || !summary.HasReconsumeArtifacts || summary.ProofSummary.Total != len(result.ReviewPlan.ReviewArtifacts) || summary.ProofSummary.Present != 0 || summary.ProofSummary.Missing != len(result.ReviewPlan.ReviewArtifacts) || summary.ProofSummary.CurrentStage != "decision-proof-required" || summary.ProofSummary.NextMissingProofType != "candidate-decision-note" || !strings.HasSuffix(summary.ProofSummary.NextMissingProofPath, ".candidate-decision-note.md") || summary.ProofSummary.NextMissingProof == nil || summary.ProofSummary.NextMissingProof.Stage != "decision-proof-required" || !strings.Contains(summary.ProofSummary.NextMissingProof.Format, "decision") || !containsSubstring(summary.ProofSummary.NextMissingProof.Evidence, "selected decisionFollowThrough outcome") || !containsSubstring(summary.ProofSummary.Boundary, "proofSummary is read-only") || summary.WhatIf || !containsSubstring(summary.Boundary, "reviewSummary is read-only") || !containsSubstring(summary.Boundary, "does not merge candidates") {
		t.Fatalf("promote create-candidates review summary missing compact handoff: %+v", summary)
	}
	readmeReview := assertCandidateReviewItem(t, result.ReviewPlan.ReviewItems, "references/template/README.md", "pending-review")
	toolingReview := assertCandidateReviewItem(t, result.ReviewPlan.ReviewItems, "references/template/toolchain-router.md", "pending-review")
	assertCandidateReviewItem(t, result.ReviewPlan.ReviewItems, "references/template/workflow-template.md", "blocked")
	if readmeReview.CleanupPath != readmeWrite.TargetPath || !strings.Contains(readmeReview.MergeTargetHint, "pack managed doc") || strings.Contains(readmeReview.MergeTargetHint, "rerun promote -Apply") {
		t.Fatalf("managed candidate review guidance drifted: %+v", readmeReview)
	}
	if !containsSubstring(readmeReview.MainAgentActions, "update or remove indexPath") {
		t.Fatalf("managed candidate review guidance missing index cleanup: %+v", readmeReview.MainAgentActions)
	}
	if !candidateJSONDecisionFollowThroughContains(result.ReviewPlan.DecisionFollowThrough, "references/template/README.md", "accept", "pack source diff") || !candidateJSONDecisionFollowThroughContains(result.ReviewPlan.DecisionFollowThrough, "references/template/README.md", "superseded", "review note naming the replacement") {
		t.Fatalf("managed candidate decision follow-through missing accept/superseded outcomes: %+v", result.ReviewPlan.DecisionFollowThrough)
	}
	if !candidateJSONReviewArtifactContains(result.ReviewPlan.ReviewArtifacts, "references/template/README.md", "candidate-decision-note", "selected decisionFollowThrough outcome") || !candidateJSONReviewArtifactContains(result.ReviewPlan.ReviewArtifacts, "references/template/README.md", "candidate-cleanup-proof", "indexPath update/removal check") || !candidateJSONReviewArtifactContains(result.ReviewPlan.ReviewArtifacts, "references/template/toolchain-router.md", "fresh-case-reconsume-proof", "fresh case doctor output") || !candidateJSONReviewArtifactContains(result.ReviewPlan.ReviewArtifacts, "references/template/toolchain-router.md", "attached-case-reconsume-proof", "templateRoot/templatePack") || !candidateJSONReviewArtifactContains(result.ReviewPlan.ReviewArtifacts, "references/template/workflow-template.md", "blocked-review-note", "blocked item must not be copied") {
		t.Fatalf("promote create-candidates missing downstream review artifacts: %+v", result.ReviewPlan.ReviewArtifacts)
	}
	if toolingReview.CleanupPath != toolingWrite.TargetPath || !strings.Contains(toolingReview.MergeTargetHint, "tooling/catalog.yml") || !strings.Contains(result.ReviewPlan.Reconsume.Tooling, "tooling/recipes") || !candidateJSONDecisionChecklistContains(result.ReviewPlan.DecisionChecklist, "references/template/toolchain-router.md", "fresh-case-reconsume") || !candidateJSONDecisionFollowThroughContains(result.ReviewPlan.DecisionFollowThrough, "references/template/toolchain-router.md", "accept", "doctor -Target <attached-case>") || !candidateJSONReconsumeChecklistContains(result.ReviewPlan.Reconsume.VerificationChecklist, "fresh-case-reconsume") || !containsSubstring(result.ReviewPlan.CompletionCriteria, "fresh or attached case reconsume") || !containsSubstring(result.ReviewPlan.CompletionCriteria, "promote -Apply is not a candidate-scoped accept path") || !containsSubstring(result.ReviewPlan.CompletionCriteria, "indexPath is updated or removed") {
		t.Fatalf("tooling candidate review/reconsume guidance drifted: item=%+v reconsume=%+v criteria=%+v followThrough=%+v", toolingReview, result.ReviewPlan.Reconsume, result.ReviewPlan.CompletionCriteria, result.ReviewPlan.DecisionFollowThrough)
	}
	commander := result.ReviewPlan.MissionCommanderAction
	if commander.State != "ready-to-review-pack-memory-candidates" || commander.PrimaryCommand != "review reviewPlan.reviewItems" || !strings.Contains(commander.Prompt, "已生成") || !containsSubstring(commander.FollowUpCommands, "doctor -Pack _template") || !containsSubstring(commander.FollowUpCommands, "init -Target <fresh-case>") || !containsSubstring(commander.Boundary, "promote -Apply is not a candidate-scoped accept path") || !containsSubstring(commander.Boundary, "accepted tooling candidates require manual") || !containsSubstring(commander.Boundary, "verify fresh or attached case reconsume") || !containsSubstring(commander.Boundary, "no authority/confirmed") || !containsSubstring(commander.Boundary, "no heavy-tool") {
		t.Fatalf("promote create-candidates omitted Mission Commander reconsume handoff: %+v", commander)
	}
	if len(result.ReviewPlan.CleanupTargets) == 0 || !strings.Contains(result.ReviewPlan.CleanupTargets[0].CleanupWhen, "update or remove indexPath") {
		t.Fatalf("candidate cleanup guidance missing index cleanup: %+v", result.ReviewPlan.CleanupTargets)
	}
	if !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "collect-review-artifacts", "replacement executor can resume") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "review-decisions", "matching decisionFollowThrough outcome") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "cleanup-rejected-or-merged-candidates", "indexPath") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "pack-doctor-after-accepted-merge", "doctor -Pack _template") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "fresh-case-reconsume-after-tooling-merge", "fresh-case") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "attached-case-reconsume-after-tooling-merge", "attached-case") {
		t.Fatalf("promote create-candidates missing main agent execution plan: %+v", result.ReviewPlan.MainAgentExecutionPlan)
	}
	if !candidateJSONNextActionContains(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.decisionChecklist", "decisionFollowThrough") || !candidateJSONNextActionContains(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.cleanupTargets", "update/remove indexPath") || !candidateJSONNextActionContains(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.reconsume.verificationChecklist", "init -Target <fresh-case>") || !candidateJSONNextActionContains(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.reconsume.verificationChecklist", "doctor -Target <attached-case>") || !candidateJSONNextActionBoundaryContains(result.ReviewPlan.MissionCommanderNextActions, "runtime does not execute") || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction == nil || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction.Command != "review reviewPlan.decisionChecklist" {
		t.Fatalf("promote create-candidates missing Mission Commander next-action command UX: actions=%+v queue=%+v", result.ReviewPlan.MissionCommanderNextActions, result.ReviewPlan.MissionCommanderActionQueue)
	}

	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-CreateCandidates", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{
		"promote candidates：applied=true created=2",
		"promote candidates review summary：mode=candidate-review pack=_template",
		"pending=2",
		"created=2",
		"managedDocs=4 toolingCandidates=1 cleanupTargets=2",
		"promote candidates review summary boundary：reviewSummary is read-only; full reviewPlan arrays remain available",
		"promote candidates review summary boundary：reviewSummary does not merge candidates, cleanup candidate files, run doctor/init, or create review artifacts",
		"promote candidates review item：path=references/template/README.md kind=managed-doc decision=pending-review action=create-candidate",
		"promote candidates review item action：path=references/template/README.md action=extract reusable guidance and resolve conflicts",
		"promote candidates review item：path=references/template/workflow-template.md kind=managed-doc decision=blocked action=blocked-deny-pattern",
		"promote candidates review checklist：path=references/template/toolchain-router.md decision=pending-review reviewAction=inspect candidatePath against packTarget and choose accept, reject, or superseded",
		"promote candidates checklist accept action：path=references/template/toolchain-router.md action=merge accepted reusable content into packTarget or an explicitly chosen pack tooling recipe/catalog target",
		"promote candidates checklist verification command：path=references/template/toolchain-router.md command=go run ./cmd/rekit -- -Command doctor -Target <fresh-case> -Pack _template",
		"promote candidates checklist boundary：path=references/template/toolchain-router.md boundary=tooling candidates require manual tooling/catalog.yml or tooling/recipes/* merge",
		"promote candidates review artifact：path=references/template/toolchain-router.md kind=tooling-candidate-source name=fresh-case-reconsume-proof",
		"promote candidates review artifact：path=references/template/workflow-template.md kind=managed-doc name=blocked-review-note",
		"promote candidates review artifact evidence：path=references/template/toolchain-router.md name=attached-case-reconsume-proof evidence=attached case doctor output",
		"promote candidates execution step：name=collect-review-artifacts when=while reviewing candidates and before declaring cleanup/reconsume complete",
		"promote candidates execution step：name=review-decisions when=before any merge, cleanup, or reconsume verification expected=every created candidate has an explicit decision before cleanup or merge and the chosen outcome maps to concrete follow-through",
		"promote candidates execution action：name=review-decisions action=follow only the matching decisionFollowThrough outcome for the chosen decision",
		"promote candidates execution step：name=cleanup-rejected-or-merged-candidates when=after reject, superseded decision, or accepted merge into pack source",
		"promote candidates execution command：name=pack-doctor-after-accepted-merge command=go run ./cmd/rekit -- -Command doctor -Pack _template",
		"promote candidates execution boundary：name=fresh-case-reconsume-after-tooling-merge boundary=use a temporary fresh case only",
		"promote candidates decision follow-through：path=references/template/README.md decision=pending-review",
		"promote candidates decision outcome：path=references/template/README.md decision=accept state=pack-memory-candidate:accepted-merge",
		"promote candidates decision when：path=references/template/README.md decision=accept when=after candidatePath is reviewed as reusable, case-neutral content",
		"promote candidates decision evidence：path=references/template/README.md decision=accept evidence=pack source diff",
		"promote candidates decision follow-through boundary：path=references/template/README.md boundary=decisionFollowThrough is guidance only; runtime does not execute merge, cleanup, init, or doctor commands",
		"promote candidates decision cleanup action：path=references/template/README.md decision=reject action=delete candidatePath after reject, superseded decision, or accepted merge into another pack source",
		"promote candidates decision verification command：path=references/template/toolchain-router.md decision=accept command=go run ./cmd/rekit -- -Command doctor -Target <attached-case> -Pack _template",
		"promote candidates cleanup target：path=references/template/README.md",
		"promote candidates cleanup action detail：path=references/template/README.md candidatePath=",
		"promote candidates reconsume top-level command：go run ./cmd/rekit -- -Command doctor -Pack _template",
		"promote candidates reconsume top-level boundary：do not promote case-local samples, traces, dumps, captures, artifacts, payloads, flags, or customer data",
		"promote candidates reconsume check：name=fresh-case-reconsume when=after accepting any tooling candidate into tooling/catalog.yml or tooling/recipes/* expected=fresh case binds templateRoot/templatePack and doctor passes while tooling remains pack-sourced",
		"promote candidates reconsume evidence：name=fresh-case-reconsume evidence=fresh case .rekit/instance.yml",
		"promote candidates reconsume command detail：name=fresh-case-reconsume command=go run ./cmd/rekit -- -Command init -Target <fresh-case> -Pack _template -ProjectName <name> -Apply",
		"promote candidates reconsume boundary detail：name=fresh-case-reconsume boundary=use a temporary fresh case only",
		"promote candidates commander action：state=ready-to-review-pack-memory-candidates",
		"mission commander action queue：summary=total=7 unblocked=7 blocked=0 requiresReview=3 followUp=0 current=review reviewPlan.decisionChecklist",
		"mission commander next action：state=pack-memory-candidates:review-decisions source=reviewPlan.decisionChecklist blocked=false requiresReview=true command=`review reviewPlan.decisionChecklist`",
		"mission commander next action：state=pack-memory-candidates:cleanup-candidate source=reviewPlan.cleanupTargets blocked=false requiresReview=true command=`delete candidatePath and update/remove indexPath`",
		"mission commander next action：state=pack-memory-candidates:fresh-case-init source=reviewPlan.reconsume.verificationChecklist blocked=false requiresReview=false command=`go run ./cmd/rekit -- -Command init -Target <fresh-case> -Pack _template -ProjectName <name> -Apply`",
		"mission commander next action boundary：reviewPlan guidance only; runtime does not execute merge, cleanup, init, or doctor commands",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("promote create-candidates text output missing %q:\n%s", expected, text)
		}
	}
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

func TestRunPromoteCreateCandidatesCaseLocalProductPathUsesMetadataRuntime(t *testing.T) {
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

	var out bytes.Buffer
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "pack-memory", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	start := decodeStartResult(t, out.Bytes())
	if start.Lane.ID != "feature-pack-memory" || start.Lane.Workspace != "workspace/features/feature-pack-memory" {
		t.Fatalf("unexpected pack-memory product-path lane: %+v", start.Lane)
	}
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Product path candidate\n\nReusable safe pack-memory update.\n")
	writeCaseFile(t, caseRoot, "references/template/workflow-template.md", "# Blocked\n\nDo not promote C:\\case\\artifact\\sample-trace.csv.\n")
	writeCaseFile(t, caseRoot, "references/template/toolchain-router.md", "# Tooling\n\nCase root: "+caseRoot+"\nAbsolute: C:\\cases\\demo.exe\nTrace: artifacts/run/demo-trace.csv\nAddress: 0x401000\nContext: ctx123 round7 Task #99\n")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(caseRoot, filepath.FromSlash(start.Lane.Workspace))
	if err := os.Chdir(nested); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	})

	reviewRoot := filepath.Join(caseRoot, ".rekit", "reviews", "pack-memory-product-path")
	out.Reset()
	if err := Run([]string{"-Command", "promote", "-CreateCandidates", "-Review", "-ReviewOutputDir", reviewRoot, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodeCandidateResult(t, out.Bytes())
	if result.Command != "promote" || result.CaseRoot != caseRoot || result.Pack != "_template" || !result.IsMutation || !result.Applied || result.Created != 2 || result.Blocked == 0 || !result.RequiresCleanup || result.ReviewWorkspace == nil || result.ReviewWorkspace.ReviewRoot != reviewRoot {
		t.Fatalf("unexpected nested no-pack promote candidates result: %+v", result)
	}
	for _, path := range []string{result.ReviewWorkspace.PacketPath, result.ReviewWorkspace.SummaryPath, result.ReviewWorkspace.CombinedDiffPath} {
		assertFileExists(t, path)
	}
	packetBytes, err := os.ReadFile(result.ReviewWorkspace.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(packetBytes), `"kind": "pack-memory-candidate-review"`) || !strings.Contains(string(packetBytes), `"candidateResult"`) || !strings.Contains(string(packetBytes), `"reviewInput"`) {
		t.Fatalf("nested product-path candidate review packet omitted durable context:\n%s", string(packetBytes))
	}
	readmeWrite := assertCandidateWrite(t, result.Writes, "references/template/README.md", "create-candidate")
	workflowWrite := assertCandidateWrite(t, result.Writes, "references/template/workflow-template.md", "blocked-deny-pattern")
	toolingWrite := assertCandidateWrite(t, result.Writes, "references/template/toolchain-router.md", "create-candidate")
	if !strings.HasPrefix(readmeWrite.TargetPath, result.CandidateRoot) || !strings.HasPrefix(toolingWrite.TargetPath, result.ToolingRoot) || result.CandidateRoot != candidateRoot || result.ToolingRoot != toolingRoot || workflowWrite.TargetPath != filepath.Join(root, "packs", "_template", filepath.FromSlash("references/template/workflow-template.md")) {
		t.Fatalf("unexpected nested candidate roots/writes: result=%+v readme=%+v tooling=%+v workflow=%+v", result, readmeWrite, toolingWrite, workflowWrite)
	}
	assertFileExists(t, readmeWrite.TargetPath)
	assertFileExists(t, toolingWrite.TargetPath)
	if result.IndexPath == "" {
		t.Fatal("missing nested promote candidate index path")
	}
	assertFileExists(t, result.IndexPath)
	readmeReview := assertCandidateReviewItem(t, result.ReviewPlan.ReviewItems, "references/template/README.md", "pending-review")
	toolingReview := assertCandidateReviewItem(t, result.ReviewPlan.ReviewItems, "references/template/toolchain-router.md", "pending-review")
	assertCandidateReviewItem(t, result.ReviewPlan.ReviewItems, "references/template/workflow-template.md", "blocked")
	if readmeReview.CleanupPath != readmeWrite.TargetPath || toolingReview.CleanupPath != toolingWrite.TargetPath || result.ReviewPlan.Mode != "candidate-review" || result.ReviewPlan.Reconsume.Mode != "pack-memory-reconsume-after-merge" || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction == nil || result.ReviewPlan.MissionCommanderActionQueue.CurrentAction.Command != "review reviewPlan.decisionChecklist" {
		t.Fatalf("nested product-path review plan omitted cleanup/reconsume handoff: reviewPlan=%+v readme=%+v tooling=%+v", result.ReviewPlan, readmeReview, toolingReview)
	}
	if !candidateJSONDecisionChecklistContains(result.ReviewPlan.DecisionChecklist, "references/template/toolchain-router.md", "fresh-case-reconsume") || !candidateJSONDecisionFollowThroughContains(result.ReviewPlan.DecisionFollowThrough, "references/template/toolchain-router.md", "accept", "doctor -Target <attached-case>") || !candidateJSONReviewArtifactContains(result.ReviewPlan.ReviewArtifacts, "references/template/README.md", "candidate-cleanup-proof", "indexPath update/removal check") || !candidateJSONReviewArtifactContains(result.ReviewPlan.ReviewArtifacts, "references/template/toolchain-router.md", "attached-case-reconsume-proof", "templateRoot/templatePack") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "collect-review-artifacts", "replacement executor can resume") || !candidateJSONExecutionPlanContains(result.ReviewPlan.MainAgentExecutionPlan, "fresh-case-reconsume-after-tooling-merge", "fresh-case") || !candidateJSONNextActionContains(result.ReviewPlan.MissionCommanderNextActions, "reviewPlan.reconsume.verificationChecklist", "doctor -Target <attached-case>") || !candidateJSONNextActionBoundaryContains(result.ReviewPlan.MissionCommanderNextActions, "runtime does not execute") {
		t.Fatalf("nested product-path review plan missing reconsume/next-action guidance: reviewPlan=%+v", result.ReviewPlan)
	}
	toolingText, err := os.ReadFile(toolingWrite.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"<caseRoot>", "<absolutePath>", "<artifactsPath>", "<address>", "<ctxNNN>", "<roundN>", "Task #<n>"} {
		if !strings.Contains(string(toolingText), expected) {
			t.Fatalf("nested tooling candidate missing %q:\n%s", expected, string(toolingText))
		}
	}
	for _, unexpected := range []string{caseRoot, "C:\\cases", "demo-trace.csv", "0x401000", "ctx123", "round7", "Task #99"} {
		if strings.Contains(string(toolingText), unexpected) {
			t.Fatalf("nested tooling candidate contains %q:\n%s", unexpected, string(toolingText))
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Command        string `json:"command"`
		Mode           string `json:"mode"`
		Pack           string `json:"pack"`
		PackSource     string `json:"packSource"`
		Target         string `json:"target"`
		TargetProvided bool   `json:"targetProvided"`
		ProjectHandoff *struct {
			Ready                bool `json:"ready"`
			PackMemoryCandidates struct {
				Ready      bool   `json:"ready"`
				Summary    string `json:"summary"`
				Total      int    `json:"total"`
				NextAction string `json:"nextAction"`
				Packs      []struct {
					Pack            string                               `json:"pack"`
					CandidateRoot   string                               `json:"candidateRoot"`
					ToolingRoot     string                               `json:"toolingRoot"`
					IndexPath       string                               `json:"indexPath"`
					CandidateFiles  int                                  `json:"candidateFiles"`
					ToolingFiles    int                                  `json:"toolingFiles"`
					IndexEntries    int                                  `json:"indexEntries"`
					RequiresReview  bool                                 `json:"requiresReview"`
					RequiresCleanup bool                                 `json:"requiresCleanup"`
					ReviewSummary   packMemoryCandidateReviewSummaryJSON `json:"reviewSummary"`
					Evidence        []string                             `json:"evidence"`
					Boundary        []string                             `json:"boundary"`
				} `json:"packs"`
			} `json:"packMemoryCandidates"`
		} `json:"projectHandoff"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("nested status with open pack-memory candidates did not decode: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.Mode != "case" || status.Pack != "_template" || status.PackSource != "case-metadata" || status.TargetProvided || status.Target != caseRoot || status.ProjectHandoff == nil {
		t.Fatalf("unexpected nested status identity with pack-memory candidates: %+v", status)
	}
	candidates := status.ProjectHandoff.PackMemoryCandidates
	if status.ProjectHandoff.Ready || candidates.Ready || candidates.Summary != "pack-memory candidate inventory has open review/cleanup/verification work" || candidates.Total != 3 || len(candidates.Packs) != 1 || !strings.Contains(candidates.NextAction, "review listed pack-memory candidates or complete listed candidate decision verification") {
		t.Fatalf("unexpected nested status pack-memory candidate handoff: %+v", status.ProjectHandoff)
	}
	pack := candidates.Packs[0]
	if pack.Pack != "_template" || pack.CandidateRoot != "packs/_template/promote-candidates" || pack.ToolingRoot != "packs/_template/tooling/candidates" || pack.IndexPath != "packs/_template/promote-candidates/index.json" || pack.CandidateFiles != 1 || pack.ToolingFiles != 1 || pack.IndexEntries != 1 || !pack.RequiresReview || !pack.RequiresCleanup || !containsSubstring(pack.Evidence, "promote-candidates files=1") || !containsSubstring(pack.Boundary, "does not merge, delete") {
		t.Fatalf("unexpected nested status pack-memory candidate pack: %+v", pack)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status：mutation=false mode=case targetProvided=false pack=_template packSource=case-metadata",
		"status project handoff：summary=release handoff summary has warnings ready=false",
		"status pack-memory candidates：summary=pack-memory candidate inventory has open review/cleanup/verification work ready=false total=3 packs=1 nextAction=review listed pack-memory candidates or complete listed candidate decision verification",
		"status pack-memory candidate pack：pack=_template candidateRoot=packs/_template/promote-candidates toolingRoot=packs/_template/tooling/candidates indexPath=packs/_template/promote-candidates/index.json candidateFiles=1 toolingFiles=1 indexEntries=1 receipts=0 pendingVerification=0 completedVerification=0 review=true cleanup=true verification=false",
		"status pack-memory review summary：pack=_template total=3 candidateFiles=1 toolingFiles=1 indexEntries=1 reviewArtifacts=8 decisionArtifacts=2 cleanupArtifacts=2 reconsumeArtifacts=4",
		"status pack-memory review summary boundary：pack=_template boundary=pack-memory reviewSummary is read-only; full candidate paths, indexCandidates, and reviewArtifacts remain available",
		"status pack-memory candidate path：pack=_template path=packs/_template/promote-candidates/",
		"status pack-memory tooling candidate path：pack=_template path=packs/_template/tooling/candidates/",
		"status pack-memory candidate index：pack=_template path=references/template/README.md candidate=packs/_template/promote-candidates/",
		"status pack-memory review artifact：pack=_template name=candidate-decision-note",
		"status pack-memory review artifact evidence：pack=_template name=candidate-decision-note evidence=selected decisionFollowThrough outcome",
		"status pack-memory review artifact boundary：pack=_template name=candidate-cleanup-proof boundary=cleanup is limited to candidateRoot/toolingRoot and indexPath",
		"status pack-memory candidate evidence：pack=_template evidence=promote-candidates files=1",
		"status pack-memory candidate boundary：pack=_template boundary=release handoff inventories candidate residue and durable decision verification receipts; it does not merge, delete, or validate cases",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested status text missing pack-memory handoff %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested status text should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run(nil, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"pack source: case-metadata",
		"status project handoff：summary=release handoff summary has warnings ready=false",
		"status pack-memory candidates：summary=pack-memory candidate inventory has open review/cleanup/verification work ready=false total=3 packs=1 nextAction=review listed pack-memory candidates or complete listed candidate decision verification",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested default status missing pack-memory handoff %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n  ") {
		t.Fatalf("nested default status should not emit JSON object:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "promote", "-CreateCandidates", "-WhatIf", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"promote candidates：applied=false created=2",
		"promote candidates review summary：mode=candidate-review-preview pack=_template",
		"pending=2",
		"created=2",
		"managedDocs=4 toolingCandidates=1 cleanupTargets=2",
		"proofProgress=0/10 currentStage=decision-proof-required nextMissingProofType=candidate-decision-note",
		"nextMissingProofPath=packs/_template/promote-candidates/review-artifacts/",
		"promote candidates review proof summary：progress=0/10 total=10 present=0 missing=10 decisionMissing=4 cleanupMissing=2 reconsumeMissing=4 currentStage=decision-proof-required nextMissingProofType=candidate-decision-note",
		"promote candidates review proof summary boundary：WhatIf did not create candidatePath, indexPath, or proof files",
		"promote candidates review proof summary boundary：proofSummary is read-only; promote create-candidates reports expected proof files but does not create or validate their contents",
		"promote candidates review summary boundary：WhatIf did not write candidate files or indexPath",
		"promote candidates review summary boundary：reviewSummary is read-only; full reviewPlan arrays remain available",
		"promote candidates review item：path=references/template/README.md kind=managed-doc decision=pending-review action=would-create-candidate",
		"promote candidates review item：path=references/template/toolchain-router.md kind=tooling-candidate-source decision=pending-review action=would-create-candidate",
		"promote candidates review artifact：path=references/template/README.md kind=managed-doc name=candidate-decision-note",
		"promote candidates review artifact boundary：path=references/template/README.md name=candidate-cleanup-proof boundary=WhatIf did not create candidatePath",
		"promote candidates execution step：name=collect-review-artifacts",
		"promote candidates reconsume top-level command：go run ./cmd/rekit -- -Command doctor -Pack _template",
		"promote candidates reconsume check：name=fresh-case-reconsume",
		"promote candidates commander action：state=preview-pack-memory-candidates",
		"mission commander action queue：summary=total=7 unblocked=7 blocked=0 requiresReview=3 followUp=0 current=review reviewPlan.decisionChecklist",
		"mission commander next action boundary：WhatIf did not write candidate files or indexPath",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("nested promote create-candidates text output missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("nested promote create-candidates text should not emit JSON:\n%s", out.String())
	}
}

func TestRunPromoteCreateCandidatesWritesDurableReviewWorkspace(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Candidate review workspace\n\nReusable safe update.\n")
	writeCaseFile(t, caseRoot, "references/template/toolchain-router.md", "# Tooling\n\nCase root: "+caseRoot+"\nTrace: artifacts/run/review-workspace-trace.csv\n")
	reviewRoot := filepath.Join(caseRoot, ".rekit", "reviews", "pack-memory-candidates")
	packetPath := filepath.Join(reviewRoot, "candidate-packet.json")
	diffPath := filepath.Join(reviewRoot, "diffs", "candidate-combined.diff")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-CreateCandidates", "-WhatIf", "-Review", "-ReviewOutputDir", reviewRoot, "-PacketPath", packetPath, "-DiffPath", diffPath, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	result := decodeCandidateResult(t, out.Bytes())
	if result.ReviewWorkspace == nil || !result.ReviewWorkspace.WritesArtifacts || result.ReviewWorkspace.ReviewRoot != reviewRoot || result.ReviewWorkspace.PacketPath != packetPath || result.ReviewWorkspace.CombinedDiffPath != diffPath {
		t.Fatalf("candidate review workspace missing from result: %+v", result.ReviewWorkspace)
	}
	for _, path := range []string{packetPath, result.ReviewWorkspace.SummaryPath, diffPath} {
		assertFileExists(t, path)
	}
	packetBytes, err := os.ReadFile(packetPath)
	if err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Kind            string          `json:"kind"`
		CandidateResult candidateResult `json:"candidateResult"`
		ReviewInput     struct {
			Summary struct {
				Changed        int  `json:"changed"`
				Blocked        int  `json:"blocked"`
				ReviewRequired bool `json:"reviewRequired"`
			} `json:"summary"`
			Items []struct {
				Path     string `json:"path"`
				DiffPath string `json:"diffPath"`
			} `json:"items"`
			ToolingItems []struct {
				Path                 string `json:"path"`
				SanitizedPreviewPath string `json:"sanitizedPreviewPath"`
			} `json:"toolingCandidateSources"`
		} `json:"reviewInput"`
		Boundary []string `json:"boundary"`
	}
	if err := json.Unmarshal(packetBytes, &packet); err != nil {
		t.Fatalf("candidate review packet is not JSON: %v\n%s", err, string(packetBytes))
	}
	if packet.Kind != "pack-memory-candidate-review" || packet.CandidateResult.ReviewPlan.ReviewSummary.PendingReviewCount != 2 || packet.ReviewInput.Summary.Changed == 0 || !packet.ReviewInput.Summary.ReviewRequired || !containsSubstring(packet.Boundary, "does not merge or delete candidates") {
		t.Fatalf("candidate review packet omitted operational handoff: %+v", packet)
	}
	foundDiff := false
	for _, item := range packet.ReviewInput.Items {
		if item.Path == "references/template/README.md" && item.DiffPath != "" {
			assertFileExists(t, item.DiffPath)
			foundDiff = true
		}
	}
	if !foundDiff {
		t.Fatalf("candidate review packet missing managed-doc bounded diff: %+v", packet.ReviewInput.Items)
	}
	foundPreview := false
	for _, item := range packet.ReviewInput.ToolingItems {
		if item.Path == "references/template/toolchain-router.md" && item.SanitizedPreviewPath != "" {
			assertFileExists(t, item.SanitizedPreviewPath)
			preview, err := os.ReadFile(item.SanitizedPreviewPath)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(preview), caseRoot) || !strings.Contains(string(preview), "<caseRoot>") {
				t.Fatalf("candidate review sanitized preview leaked case path or omitted placeholder:\n%s", string(preview))
			}
			foundPreview = true
		}
	}
	if !foundPreview {
		t.Fatalf("candidate review packet missing sanitized tooling preview: %+v", packet.ReviewInput.ToolingItems)
	}
	summaryBytes, err := os.ReadFile(result.ReviewWorkspace.SummaryPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# rekit pack-memory candidate review workspace", "## 执行清单", "pending=2", "candidate-decision-note", "does not merge or delete candidates"} {
		if !strings.Contains(string(summaryBytes), expected) {
			t.Fatalf("candidate review workspace summary missing %q:\n%s", expected, string(summaryBytes))
		}
	}
	for _, write := range result.Writes {
		if write.Action == "would-create-candidate" {
			if _, err := os.Stat(write.TargetPath); !os.IsNotExist(err) {
				t.Fatalf("WhatIf review workspace created candidate %s", write.TargetPath)
			}
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-CreateCandidates", "-WhatIf", "-ReviewOutputDir", filepath.Join(caseRoot, ".rekit", "reviews", "pack-memory-text"), "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"promote candidates review workspace：root=", "packet=", "combinedDiff=", "promote candidates review workspace boundary：workspace records bounded review artifacts only"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("candidate review workspace text missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	err = Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-CreateCandidates", "-WhatIf", "-Format", "xml"}, &out)
	if err == nil || !strings.Contains(err.Error(), "unsupported promote create-candidates format") {
		t.Fatalf("error = %v, want unsupported format guard", err)
	}
}

func TestRunPromoteCandidateDecisionCaseLocalPreviewAndApply(t *testing.T) {
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
	writeCaseFile(t, caseRoot, "references/template/README.md", "# Reviewed decision candidate\n\nReusable safe update.\n")
	reviewRoot := filepath.Join(caseRoot, ".rekit", "reviews", "candidate-decision-product-path")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-CreateCandidates", "-Review", "-ReviewOutputDir", reviewRoot, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	created := decodeCandidateResult(t, out.Bytes())
	var managed candidateReviewItem
	for _, item := range created.ReviewPlan.ReviewItems {
		if item.Kind == "managed-doc" && item.ReviewDecision == "pending-review" {
			managed = item
			break
		}
	}
	if managed.CandidatePath == "" || managed.PackTarget == "" || created.ReviewWorkspace == nil {
		t.Fatalf("candidate result omitted reviewed managed item: %+v", created)
	}
	packetBytes, err := os.ReadFile(created.ReviewWorkspace.PacketPath)
	if err != nil {
		t.Fatal(err)
	}
	candidateBytes, err := os.ReadFile(managed.CandidatePath)
	if err != nil {
		t.Fatal(err)
	}
	packBytes, err := os.ReadFile(managed.PackTarget)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.WriteFile(managed.PackTarget, packBytes, 0o644); err != nil {
			t.Fatal(err)
		}
	})
	evidenceBytes, err := os.ReadFile(created.ReviewWorkspace.CombinedDiffPath)
	if err != nil {
		t.Fatal(err)
	}
	decisionPath := filepath.Join(reviewRoot, "decisions.json")
	decisionJSON := fmt.Sprintf(`{"schemaVersion":1,"kind":"pack-memory-candidate-decisions","packetHash":"%s","decisions":[{"candidatePath":%q,"decision":"accept","candidateHash":"%s","packTargetHash":"%s","reason":"reviewed bounded candidate diff","actor":"mission-commander","evidenceRefs":[{"path":%q,"sha256":"%s"}]}]}`+"\n", testSHA256(packetBytes), managed.CandidatePath, testSHA256(candidateBytes), testSHA256(packBytes), created.ReviewWorkspace.CombinedDiffPath, testSHA256(evidenceBytes))
	if err := os.WriteFile(decisionPath, []byte(decisionJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", created.ReviewWorkspace.PacketPath, "-CandidateDecisionPath", decisionPath, "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var preview promote.CandidateDecisionResult
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.IsMutation || preview.Applied || preview.Accepted != 1 || len(preview.Actions) != 1 {
		t.Fatalf("unexpected candidate decision preview: %+v", preview)
	}

	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", created.ReviewWorkspace.PacketPath, "-CandidateDecisionPath", decisionPath, "-Apply", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"promote candidate decision：mode=candidate-decision mutation=true applied=true rolledBack=false recoveryRequired=false", "accepted=1 rejected=0 superseded=0", "promote candidate decision action：candidate=", "candidateBackup=", "targetBackup=", "merge-accepted-candidate-and-cleanup", "no authority/confirmed writes"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("candidate decision text omitted %q:\n%s", expected, out.String())
		}
	}
	merged, err := os.ReadFile(managed.PackTarget)
	if err != nil {
		t.Fatal(err)
	}
	if string(merged) != string(candidateBytes) {
		t.Fatalf("candidate decision did not merge reviewed candidate: %q", string(merged))
	}

	freshCase := filepath.Join(t.TempDir(), "fresh-case")
	attachedCase := filepath.Join(t.TempDir(), "attached-case")
	for _, target := range []string{freshCase, attachedCase} {
		out.Reset()
		if err := Run([]string{"-Command", "init", "-Target", target, "-Pack", "_template", "-Apply", "-Format", "json"}, &out); err != nil {
			t.Fatal(err)
		}
	}
	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", created.ReviewWorkspace.PacketPath, "-CandidateDecisionPath", decisionPath, "-VerifyCandidateDecision", "-FreshCaseRoot", freshCase, "-AttachedCaseRoot", attachedCase, "-WhatIf", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var verification promote.CandidateDecisionVerificationResult
	if err := json.Unmarshal(out.Bytes(), &verification); err != nil {
		t.Fatal(err)
	}
	if verification.IsMutation || verification.Applied || !verification.Ready || verification.FreshDoctorRows == 0 || verification.AttachedDoctorRows == 0 {
		t.Fatalf("unexpected candidate verification preview: %+v", verification)
	}
	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Target", caseRoot, "-Pack", "_template", "-PacketPath", created.ReviewWorkspace.PacketPath, "-CandidateDecisionPath", decisionPath, "-VerifyCandidateDecision", "-FreshCaseRoot", freshCase, "-AttachedCaseRoot", attachedCase, "-Apply", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"promote candidate verification：mutation=true applied=true ready=true", "freshDoctorRows=", "attachedDoctorRows=", "write authority/confirmed"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("candidate verification text omitted %q:\n%s", expected, out.String())
		}
	}
}

func testSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
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

func TestRunSyncAndPromoteReviewPlanTextOutputsItems(t *testing.T) {
	syncCaseRoot := attachedCase(t)
	writeCaseFile(t, syncCaseRoot, "references/template/README.md", "# Sync review drift\n\nchanged before review\n")
	var out bytes.Buffer
	if err := Run([]string{"-Command", "sync", "-Target", syncCaseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"sync review plan：direction=kit-to-case mutation=false changed=",
		"sync review item：path=.rekit/instance.yml + .claude/skills/rekit/SKILL.md kind=case-metadata action=refresh-metadata-and-shim risk=low direction=kit-to-case",
		"sync review item：path=references/template/README.md kind=managed-file action=overwrite-with-backup risk=medium direction=kit-to-case",
		"sync review item paths：path=references/template/README.md source=",
		"sync review item hashes：path=references/template/README.md source=",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("sync review text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("sync review text should not emit JSON:\n%s", out.String())
	}

	promoteCaseRoot := fullAttachedCase(t)
	writeCaseFile(t, promoteCaseRoot, "references/template/README.md", "# Promote review drift\n\nReusable safe candidate.\n")
	out.Reset()
	if err := Run([]string{"-Command", "promote", "-Target", promoteCaseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"promote review plan：direction=case-to-kit mutation=false changed=",
		"promote review item：path=references/template/README.md kind=managed-doc action=candidate-after-llm-review risk=medium direction=case-to-kit recommendation=llm-review-before-merge",
		"promote review item paths：path=references/template/README.md source= target= case=",
		"promote review item hashes：path=references/template/README.md source= target= case=",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("promote review text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("promote review text should not emit JSON:\n%s", out.String())
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
		MissionBrief                missionBrief                     `json:"missionBrief"`
		ExecutorAction              executorActionSnapshot           `json:"executorAction"`
		WouldExecutorAction         executorActionSnapshot           `json:"wouldExecutorAction"`
		MissionCommanderAction      missionCommanderActionSnapshot   `json:"missionCommanderAction"`
		MissionCommanderNextActions []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
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
	if plan.ExecutorAction.Blocked || !plan.ExecutorAction.Ready || plan.ExecutorAction.PendingGates != 0 || plan.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("gate plan current executor action drifted: %+v", plan.ExecutorAction)
	}
	if !plan.WouldExecutorAction.Blocked || plan.WouldExecutorAction.Ready || plan.WouldExecutorAction.PendingGates != 1 || !plan.WouldExecutorAction.PendingGateRequired || plan.WouldExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("gate plan would executor action drifted: %+v", plan.WouldExecutorAction)
	}
	if plan.MissionCommanderAction.State != "needs-gate-apply" || !strings.Contains(plan.MissionCommanderAction.PrimaryCommand, "/rekit gate -Pack _template -Action full-trace -Lane main -Apply -Actor <actor>") || !containsMissionCommanderNextAction(plan.MissionCommanderNextActions, "missionCommanderActions", plan.MissionCommanderAction.PrimaryCommand, false, true) || !containsMissionCommanderNextAction(plan.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main -WhatIf", true, true) {
		t.Fatalf("gate plan should expose top-level Mission Commander apply projection: action=%+v next=%+v", plan.MissionCommanderAction, plan.MissionCommanderNextActions)
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
		MissionBrief                missionBrief                     `json:"missionBrief"`
		ExecutorAction              executorActionSnapshot           `json:"executorAction"`
		MissionCommanderAction      missionCommanderActionSnapshot   `json:"missionCommanderAction"`
		MissionCommanderNextActions []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
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
	if !result.ExecutorAction.Blocked || result.ExecutorAction.Ready || result.ExecutorAction.PendingGates != 1 || !result.ExecutorAction.PendingGateRequired || result.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("gate apply executor action drifted: %+v", result.ExecutorAction)
	}
	if result.ExecutorAction.MissionCommanderAction.State != "needs-gate-decision" || result.ExecutorAction.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || !containsSubstring(result.ExecutorAction.MissionCommanderAction.FollowUpCommands, "/rekit gate <action> -Lane main -Apply -Actor <actor>") || !containsSubstring(result.ExecutorAction.MissionCommanderAction.Boundary, "do not run continue") {
		t.Fatalf("gate apply should expose pending-gate Mission Commander handoff: %+v", result.ExecutorAction.MissionCommanderAction)
	}
	if result.MissionCommanderAction.State != "needs-gate-decision" || result.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", "/rekit handoff main", true, true) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit continue main -WhatIf", true, true) {
		t.Fatalf("gate apply should expose top-level pending-gate Mission Commander projection: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	ledger, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "requests.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ledger), result.EventID) || !strings.Contains(string(ledger), `"pending-gate"`) {
		t.Fatalf("ledger does not contain gate event: %s", string(ledger))
	}
}

func TestRunGateTextOutputsExecutorActions(t *testing.T) {
	caseRoot := attachedCaseWithBoard(t)
	var out bytes.Buffer
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "-Format", "text", "-Action", "debug", "-Lane", "main", "-Subject", "debug gate"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"current executor action：blocked=false ready=true pendingGates=0 openInterventions=0 openDecisions=0",
		"would executor action：blocked=true ready=false pendingGates=1 openInterventions=0 openDecisions=0",
		"would executor action requirements：reconcile=false pendingGate=true openDecision=false",
		"would executor action handoff：continue=`/rekit continue main` handoff=`/rekit handoff main`",
		"would executor action commander action：state=needs-gate-apply primary=`/rekit gate -Pack _template -Action debug -Lane main -Apply -Actor <actor>",
		"would executor action commander action follow-up：/rekit continue main -WhatIf",
		"mission commander next action：state=needs-gate-apply source=missionCommanderActions blocked=false requiresReview=true command=`/rekit gate -Pack _template -Action debug -Lane main -Apply -Actor <actor>",
		"mission commander next action：state=needs-gate-apply source=missionCommanderActions.followUp blocked=true requiresReview=true command=`/rekit continue main -WhatIf`",
		"mission commander next action boundary：pending-gate still requires explicit authorization before heavy action",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("gate what-if text missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-Format", "text", "-Action", "debug", "-Lane", "main", "-Actor", "runtime-test", "-Subject", "debug gate"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"gate ledger：applied=true status=pending-gate",
		"executor action：blocked=true ready=false pendingGates=1 openInterventions=0 openDecisions=0",
		"executor action requirements：reconcile=false pendingGate=true openDecision=false",
		"executor action handoff：continue=`/rekit continue main` handoff=`/rekit handoff main`",
		"executor action commander action：state=needs-gate-decision primary=`/rekit handoff main`",
		"executor action commander action follow-up：/rekit gate <action> -Lane main -Apply -Actor <actor>",
		"mission commander next action：state=needs-gate-decision source=missionCommanderActions blocked=true requiresReview=true command=`/rekit handoff main`",
		"mission commander next action：state=needs-gate-decision source=missionCommanderActions.followUp blocked=true requiresReview=true command=`/rekit continue main -WhatIf`",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("gate apply text missing %q:\n%s", expected, out.String())
		}
	}
}

func TestRunGateDuplicateExecutionEvidenceProjectsIdempotentNextActions(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeAuthorizedGateVisibilityFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "authorized debug",
		"-TargetRef", "target-alpha",
		"-Scope", "handler only",
		"-RuntimeSeconds", "30",
		"-DiskMB", "64",
		"-Requests", "1",
		"-OutputPaths", "workspace/main/debug/session-1",
		"-StopConditions", "timeout",
		"-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var applied struct {
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("authorized gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	recordArgs := []string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-GateEventId", applied.EventID,
		"-ExecutionStatus", "succeeded",
		"-Actor", "executor-1",
		"-ActualRuntimeSeconds", "25",
		"-OutputRefs", "workspace/main/debug/session-1/duplicate-result.json",
		"-Format", "json",
	}
	out.Reset()
	if err := Run(recordArgs, &out); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(recordArgs, &out); err != nil {
		t.Fatal(err)
	}
	var duplicate struct {
		Applied                     bool                                `json:"applied"`
		Reason                      string                              `json:"reason"`
		MissionCommanderAction      missionCommanderActionSnapshot      `json:"missionCommanderAction"`
		ExecutionEvidenceReview     []executionEvidenceReviewItem       `json:"executionEvidenceReview"`
		MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &duplicate); err != nil {
		t.Fatalf("duplicate execution evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if duplicate.Applied || duplicate.Reason != "duplicate eventId" || duplicate.MissionCommanderAction.State != "evidence-already-recorded" || containsSubstring(duplicate.MissionCommanderAction.FollowUpCommands, "/rekit continue") || !containsSubstring(duplicate.MissionCommanderAction.Boundary, "did not append observation evidence") {
		t.Fatalf("duplicate execution evidence omitted idempotent commander action: %+v", duplicate)
	}
	if len(duplicate.ExecutionEvidenceReview) != 1 || duplicate.ExecutionEvidenceReview[0].MissionCommanderAction.State != "evidence-already-recorded" || len(duplicate.MissionCommanderNextActions) != 2 || duplicate.MissionCommanderNextActions[0].State != "evidence-already-recorded" || duplicate.MissionCommanderNextActions[0].Command != "/rekit handoff main" || duplicate.MissionCommanderNextActions[1].Command != "/rekit overview" || cliNextActionContainsCommand(duplicate.MissionCommanderNextActions, "/rekit continue") || cliNextActionContainsSource(duplicate.MissionCommanderNextActions, "missionCommanderActions") || !cliNextActionBoundaryContains(duplicate.MissionCommanderNextActions, "did not append observation evidence") {
		t.Fatalf("duplicate execution evidence next actions should be review-only and idempotent: review=%+v next=%+v", duplicate.ExecutionEvidenceReview, duplicate.MissionCommanderNextActions)
	}
	if duplicate.MissionCommanderActionQueue.Summary != "total=2 unblocked=2 blocked=0 requiresReview=2 followUp=1 current=/rekit handoff main" || duplicate.MissionCommanderActionQueue.CurrentAction == nil || duplicate.MissionCommanderActionQueue.CurrentAction.Command != "/rekit handoff main" || len(duplicate.MissionCommanderActionQueue.FollowUpActions) != 1 {
		t.Fatalf("duplicate execution evidence missing action queue: %+v", duplicate.MissionCommanderActionQueue)
	}

	out.Reset()
	textArgs := append([]string{}, recordArgs...)
	textArgs[len(textArgs)-1] = "text"
	if err := Run(textArgs, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"gate execution evidence：applied=false status=succeeded",
		"evidence commander action：state=evidence-already-recorded primary=`/rekit handoff main`",
		"evidence commander action boundary：duplicate record did not append observation evidence",
		"mission commander action queue：summary=total=2 unblocked=2 blocked=0 requiresReview=2 followUp=1 current=/rekit handoff main",
		"mission commander action queue current：state=evidence-already-recorded source=executionEvidenceReview blocked=false requiresReview=true command=`/rekit handoff main`",
		"mission commander next action：state=evidence-already-recorded source=executionEvidenceReview blocked=false requiresReview=true command=`/rekit handoff main`",
		"mission commander next action：state=evidence-already-recorded source=executionEvidenceReview.followUp blocked=false requiresReview=true command=`/rekit overview`",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("duplicate execution evidence text missing %q:\n%s", expected, out.String())
		}
	}
	for _, unexpected := range []string{"/rekit continue main -WhatIf", "source=missionCommanderActions"} {
		if strings.Contains(out.String(), unexpected) {
			t.Fatalf("duplicate execution evidence text should not contain %q:\n%s", unexpected, out.String())
		}
	}
}

func TestRunGateExecutionEvidenceTextOutputsNextActions(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeAuthorizedGateVisibilityFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "authorized debug",
		"-TargetRef", "target-alpha",
		"-Scope", "handler only",
		"-RuntimeSeconds", "30",
		"-DiskMB", "64",
		"-Requests", "1",
		"-OutputPaths", "workspace/main/debug/session-1",
		"-StopConditions", "timeout",
		"-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var applied struct {
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("authorized gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if applied.EventID == "" {
		t.Fatalf("authorized gate omitted eventId: %+v", applied)
	}

	out.Reset()
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-GateEventId", applied.EventID,
		"-ExecutionStatus", "succeeded",
		"-Actor", "executor-1",
		"-ActualRuntimeSeconds", "25",
		"-OutputRefs", "workspace/main/debug/session-1/text-result.json",
		"-Format", "text",
	}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"gate execution evidence：applied=true status=succeeded",
		"gate execution evidence detail：subject=execution evidence for authorized debug summary=Recorded execution evidence for authorized debug gate target=target-alpha recordRequired=true reportPath=",
		"gate execution evidence budget：runtimeSeconds=25 diskMB=0 requests=0",
		"gate execution evidence outputRefs：workspace/main/debug/session-1/text-result.json",
		"gate execution evidence review summary：total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=0 boundaryHits=0 hasEscalation=false hasExecutionReport=false hasAdapter=false latestEventId=",
		"latestGateEventId=" + applied.EventID,
		"latestStatus=succeeded latestAction=debug latestTarget=target-alpha latestReview=review outputRefs/evidenceRefs for gateEventId " + applied.EventID,
		"latestHandoff=/rekit handoff main latestCommanderState=ready-for-evidence-review latestCommanderPrimary=/rekit handoff main",
		"outcomes=1 followThrough=ready-for-evidence-review nextActions=5 reviewRequiredActions=3 currentAction=/rekit handoff main",
		"gate execution evidence review summary action queue：total=5 unblocked=5 blocked=0 requiresReview=3 followUp=3 current=/rekit handoff main",
		"gate execution evidence review summary boundary：execution evidence review summary is read-only; full executionEvidenceReview remains available",
		"gate execution evidence review summary boundary：observation evidence is already recorded; do not replay heavy tool",
		"gate execution evidence review summary boundary：review outputRefs/evidenceRefs before any authority/confirmed outcome",
		"gate execution evidence review summary boundary：no authority/confirmed writes",
		"execution evidence follow-through：state=ready-for-evidence-review gateEventId=" + applied.EventID,
		"execution evidence follow-through outcome：name=recorded-evidence-review state=ready-for-evidence-review command=`/rekit handoff main`",
		"execution evidence follow-through when：name=recorded-evidence-review when=gate -Apply records bounded observation evidence for the authorized gate",
		"execution evidence follow-through evidence：name=recorded-evidence-review evidence=observation evidence row",
		"evidence commander action：state=ready-for-evidence-review primary=`/rekit handoff main`",
		"mission commander next action：state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=`/rekit handoff main`",
		"mission commander next action：state=ready-for-evidence-review source=executionEvidenceReview.followUp blocked=false requiresReview=true command=`/rekit overview`",
		"mission commander action queue：summary=total=5 unblocked=5 blocked=0 requiresReview=3 followUp=3 current=/rekit handoff main",
		"mission commander action queue current：state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=`/rekit handoff main`",
		"mission commander next action：state=ready-for-evidence-review source=executionEvidenceReview.followUp blocked=false requiresReview=true command=`/rekit continue main -WhatIf`",
		"mission commander next action：state=ready-to-continue source=missionCommanderActions blocked=false requiresReview=false command=`/rekit continue main`",
		"mission commander next action boundary：observation evidence is already recorded; do not replay heavy tool",
		"executor action commander action：state=ready-to-continue primary=`/rekit continue main`",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("gate execution evidence text missing %q:\n%s", expected, out.String())
		}
	}
}

func TestRunGateAdapterReportTextOutputsNextActions(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeAuthorizedGateVisibilityFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "authorized debug",
		"-TargetRef", "target-alpha",
		"-RuntimeSeconds", "30",
		"-DiskMB", "64",
		"-Requests", "1",
		"-OutputPaths", "workspace/main/debug/session-1",
		"-StopConditions", "timeout",
		"-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var applied struct {
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("authorized gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if applied.EventID == "" {
		t.Fatalf("authorized gate omitted eventId: %+v", applied)
	}
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "text-cli-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 20, "diskMB": 32, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter report from text output test"
}`)
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/result.json", `{"ok":true}`)
	wantReportPath := "workspace/main/debug/session-1/adapter-report.json"
	wantValidate := "/rekit gate -Pack _template -GateEventId " + applied.EventID + " -ValidateExecutionReport -ExecutionReportPath " + wantReportPath + " -Format json"
	wantRecord := "/rekit gate -Pack _template -Apply -GateEventId " + applied.EventID + " -ExecutionReportPath " + wantReportPath + " -Actor <executor-id> -Format json"
	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-ExecutionReportContract", "-GateEventId", applied.EventID, "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"gate adapter report contract：gateEventId=" + applied.EventID + " action=debug lane=main reportPath=" + wantReportPath + " mutation=false",
		"gate adapter report contract summary：state=needs-adapter-report-validation gateEventId=" + applied.EventID + " action=debug lane=main reportPath=" + wantReportPath + " defaultReportPath=" + wantReportPath + " reportPresent=false valid=false recordReady=false recordBlocked=true requiresValidation=true requiresRepair=false requiresMainEscalation=false allowedStatuses=5 allowedOutputPaths=1 stopConditions=1 adapterCandidates=0 repairHints=28 recordBlockedHints=28 escalateHints=1 outcomes=3 nextActions=3 reviewRequiredActions=3 currentAction=" + wantValidate,
		"gate adapter report contract summary boundary：adapter report summary is read-only; full contract/validation/follow-through arrays remain available",
		"gate adapter report validate command：rekit -Command gate -Pack _template -GateEventId " + applied.EventID + " -ValidateExecutionReport -ExecutionReportPath " + wantReportPath + " -Format json",
		"gate adapter report record command：rekit -Command gate -Pack _template -Apply -GateEventId " + applied.EventID + " -ExecutionReportPath " + wantReportPath + " -Actor <executor-id> -Format json",
		"gate adapter report live validation：cwd=authorized output workspace listed in authorizedWorkspaces; use reportFileName as workspace-relative -ExecutionReportPath and omit -Target; or use caseRelativeReportPath with case-relative commands from any case-local cwd reportFileName=adapter-report.json caseRelativeReportPath=" + wantReportPath,
		"gate adapter report authorized workspaces：workspace/main/debug/session-1",
		"gate adapter report sidecar template：kind=adapter-execution-report adapterId=<adapter-id> action=debug status=succeeded|failed|boundary-hit|escalated|aborted gateEventId=" + applied.EventID,
		"gate adapter report sidecar outputRefs：<case-relative output under authorized outputPaths>",
		"gate adapter report live validation note：ValidateArgs and CaseRelativeValidateArgs are read-only: isMutation=false, applied=false, and no observations/authority/confirmed writes.",
		"gate adapter report validation repair hint：action=add-boundary-marker recordBlocked=true rerunValidation=true code=boundary-marker-missing stage=boundary",
		"gate adapter report validation repair hint evidence：action=add-boundary-marker evidence=allowedStopConditions=timeout",
		"gate adapter report validation repair hint boundary：action=add-boundary-marker boundary=boundaryHits must use authorized stopConditions: timeout",
		"gate adapter report validation repair hint boundary：action=move-evidence-refs-under-authorized-output-paths boundary=stay under authorized outputPaths: workspace/main/debug/session-1",
		"adapter report contract follow-through：state=needs-adapter-report-validation gateEventId=" + applied.EventID + " reportPath=" + wantReportPath + " outcomes=3",
		"adapter report contract follow-through outcome：name=write-and-validate-report state=needs-adapter-report-validation command=`" + wantValidate + "`",
		"adapter report contract follow-through when：name=write-and-validate-report when=after authorized-gate request is recorded and execution report contract is read",
		"adapter report contract follow-through evidence：name=write-and-validate-report evidence=adapter execution report sidecar",
		"adapter report contract follow-through outcome：name=valid-report-record state=ready-to-record-evidence command=`" + wantRecord + "`",
		"adapter report contract follow-through when：name=valid-report-record when=validation returns valid=true for the bounded sidecar",
		"adapter report contract follow-through evidence：name=valid-report-record evidence=observation evidence row",
		"adapter report contract follow-through outcome：name=invalid-report-repair state=repair-adapter-report",
		"adapter report commander action：state=needs-adapter-report-validation primary=`" + wantValidate + "`",
		"mission commander action queue：summary=total=3 unblocked=2 blocked=1 requiresReview=3 followUp=2 current=" + wantValidate,
		"mission commander action queue current：state=needs-adapter-report-validation source=adapterReportContract.missionCommanderAction blocked=false requiresReview=true command=`" + wantValidate + "`",
		"mission commander next action：state=needs-adapter-report-validation source=adapterReportContract.missionCommanderAction blocked=false requiresReview=true command=`" + wantValidate + "`",
		"mission commander next action：state=needs-adapter-report-validation source=adapterReportContract.missionCommanderAction.followUp blocked=true requiresReview=true command=`" + wantRecord + "`",
		"mission commander next action boundary：do not record evidence until validation returns valid=true",
		"mission commander next action boundary：replace <executor-id> before running record command",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("adapter report contract text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "source=adapterReportValidation.missionCommanderAction") {
		t.Fatalf("contract text should not expose validation-stage record as current action:\n%s", out.String())
	}
	afterContractText := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, afterContractText)

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-ValidateExecutionReport", "-GateEventId", applied.EventID, "-ExecutionReportPath", wantReportPath, "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"gate adapter report validation：valid=true gateEventId=" + applied.EventID + " reportPath=" + wantReportPath + " mutation=false applied=false",
		"gate adapter report validation summary：state=ready-to-record-evidence gateEventId=" + applied.EventID + " action=debug lane=main reportPath=" + wantReportPath + " defaultReportPath=" + wantReportPath + " reportPresent=true valid=true recordReady=true recordBlocked=false requiresValidation=false requiresRepair=false requiresMainEscalation=false allowedStatuses=5 allowedOutputPaths=1 stopConditions=1 adapterCandidates=0 repairHints=0 recordBlockedHints=0 escalateHints=0 outcomes=1 nextActions=2 reviewRequiredActions=2 currentAction=" + wantRecord + " reportStatus=succeeded adapterId=text-cli-adapter actualBudget=runtimeSeconds=20,diskMB=32,requests=1 outputRefs=1 evidenceRefs=0 boundaryHits=0 hasEscalation=false hasSummary=true",
		"gate adapter report validation summary boundary：validation is read-only and must return valid=true before evidence record",
		"gate adapter report sidecar：kind=adapter-execution-report adapterId=text-cli-adapter action=debug status=succeeded gateEventId=" + applied.EventID + " actualBudget=runtimeSeconds=20,diskMB=32,requests=1",
		"gate adapter report sidecar outputRefs：workspace/main/debug/session-1/result.json",
		"gate adapter report sidecar summary：escalation= summary=Adapter report from text output test",
		"adapter report validation follow-through：state=ready-to-record-evidence gateEventId=" + applied.EventID + " reportPath=" + wantReportPath + " outcomes=1",
		"adapter report validation follow-through outcome：name=valid-report-record state=ready-to-record-evidence command=`" + wantRecord + "`",
		"adapter report validation follow-through when：name=valid-report-record when=gate -ValidateExecutionReport returns valid=true for the bounded sidecar",
		"adapter report validation follow-through evidence：name=valid-report-record evidence=valid=true validation envelope",
		"adapter report validation commander action：state=ready-to-record-evidence primary=`" + wantRecord + "`",
		"mission commander action queue：summary=total=2 unblocked=2 blocked=0 requiresReview=2 followUp=1 current=" + wantRecord,
		"mission commander action queue current：state=ready-to-record-evidence source=adapterReportValidation.missionCommanderAction blocked=false requiresReview=true command=`" + wantRecord + "`",
		"mission commander next action：state=ready-to-record-evidence source=adapterReportValidation.missionCommanderAction blocked=false requiresReview=true command=`" + wantRecord + "`",
		"mission commander next action：state=ready-to-record-evidence source=adapterReportValidation.missionCommanderAction.followUp blocked=false requiresReview=true command=`/rekit handoff main`",
		"mission commander next action reason：validation returned valid=true",
		"mission commander next action boundary：replace <executor-id> before running record command",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("adapter report validation text missing %q:\n%s", expected, out.String())
		}
	}
	afterValidationText := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, afterValidationText)

	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-invalid.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "text-cli-adapter",
  "action": "debug",
  "status": "boundary-hit",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 20, "diskMB": 32, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter reported boundary hit without marker"
}`)
	invalidReportPath := "workspace/main/debug/session-1/adapter-invalid.json"
	wantInvalidValidate := "/rekit gate -Pack _template -GateEventId " + applied.EventID + " -ValidateExecutionReport -ExecutionReportPath " + invalidReportPath + " -Format json"
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-ValidateExecutionReport", "-GateEventId", applied.EventID, "-ExecutionReportPath", invalidReportPath, "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"gate adapter report validation：valid=false gateEventId=" + applied.EventID + " reportPath=" + invalidReportPath + " mutation=false applied=false",
		"gate adapter report validation summary：state=repair-adapter-report gateEventId=" + applied.EventID + " action=debug lane=main reportPath=" + invalidReportPath + " defaultReportPath=" + wantReportPath + " reportPresent=true valid=false recordReady=false recordBlocked=true requiresValidation=false requiresRepair=true requiresMainEscalation=false allowedStatuses=5 allowedOutputPaths=1 stopConditions=1 adapterCandidates=0 repairHints=1 recordBlockedHints=1 escalateHints=0 outcomes=1 nextActions=2 reviewRequiredActions=2 currentAction=add-boundary-marker reportStatus=boundary-hit adapterId=text-cli-adapter actualBudget=runtimeSeconds=20,diskMB=32,requests=1 outputRefs=1 evidenceRefs=0 boundaryHits=0 hasEscalation=false hasSummary=true failureCode=boundary-marker-missing failureStage=boundary",
		"gate adapter report validation summary boundary：record command writes bounded observation evidence only; /rekit does not execute the heavy tool",
		"gate adapter report validation failure：code=boundary-marker-missing stage=boundary",
		"gate adapter report repair hint：action=add-boundary-marker recordBlocked=true rerunValidation=true code=boundary-marker-missing stage=boundary fields=boundaryHits,escalation allowedValues= allowedOutputPaths= allowedStopConditions=timeout maxBytes=0 escalateToMain=false detail=boundary-hit or escalated status requires authorized boundaryHits or a bounded escalation",
		"gate adapter report repair hint evidence：action=add-boundary-marker evidence=allowedStopConditions=timeout",
		"gate adapter report repair hint boundary：action=add-boundary-marker boundary=boundaryHits must use authorized stopConditions: timeout",
		"adapter report validation follow-through：state=repair-adapter-report gateEventId=" + applied.EventID + " reportPath=" + invalidReportPath + " outcomes=1",
		"adapter report validation follow-through when：name=invalid-report-repair when=gate -ValidateExecutionReport returns valid=false or a report path is missing",
		"adapter report validation follow-through repair action：name=invalid-report-repair action=add-boundary-marker",
		"adapter report validation follow-through evidence：name=invalid-report-repair evidence=repairHints",
		"adapter report validation commander action：state=repair-adapter-report primary=`" + wantInvalidValidate + "`",
		"mission commander next action：state=repair-adapter-report source=adapterReportValidation.repairHints blocked=false requiresReview=true command=`add-boundary-marker`",
		"mission commander next action：state=repair-adapter-report source=adapterReportValidation.missionCommanderAction blocked=false requiresReview=true command=`" + wantInvalidValidate + "`",
		"mission commander next action reason：allowedStopConditions=timeout",
		"mission commander next action reason：recordBlocked=true; do not record evidence until valid=true",
		"mission commander next action boundary：boundaryHits must use authorized stopConditions: timeout",
		"mission commander next action boundary：do not record evidence until validation returns valid=true",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("invalid adapter report validation text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "source=adapterReportValidation.missionCommanderAction blocked=false requiresReview=true command=`/rekit gate -Pack _template -Apply") {
		t.Fatalf("invalid adapter report validation text should not recommend record apply:\n%s", out.String())
	}
	afterInvalidText := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, afterInvalidText)
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
		Applied bool   `json:"applied"`
		EventID string `json:"eventId"`
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
		MissionBrief                missionBrief                     `json:"missionBrief"`
		ExecutorAction              executorActionSnapshot           `json:"executorAction"`
		MissionCommanderAction      missionCommanderActionSnapshot   `json:"missionCommanderAction"`
		MissionCommanderNextActions []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
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
	authorizedEventID := result.EventID
	reportContract := "/rekit gate -ExecutionReportContract -GateEventId " + authorizedEventID + " -Format json"
	for _, want := range []string{
		"requestedBudget=runtimeSeconds=30,diskMB=64,requests=1",
		"outputPaths=workspace/main/debug/session-1",
		"stopConditions=timeout",
		"eventId=" + authorizedEventID,
		"reportContract=" + reportContract,
	} {
		if !containsSubstring(result.MissionBrief.AuthorizedGates, want) {
			t.Fatalf("authorized gate mission brief omitted %q: %+v", want, result.MissionBrief)
		}
	}
	if result.ExecutorAction.Blocked || !result.ExecutorAction.Ready || result.ExecutorAction.PendingGates != 0 || !slices.Equal(result.ExecutorAction.NextAgentActions, []string{"/rekit continue main"}) {
		t.Fatalf("authorized gate executor action should remain non-blocking: %+v", result.ExecutorAction)
	}
	wantContractCommand := "/rekit gate -Pack _template -GateEventId " + authorizedEventID + " -ExecutionReportContract -Format json"
	if result.MissionCommanderAction.State != "ready-for-execution-report-contract" || result.MissionCommanderAction.PrimaryCommand != wantContractCommand || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions", wantContractCommand, false, true) || !containsMissionCommanderNextAction(result.MissionCommanderNextActions, "missionCommanderActions.followUp", "/rekit handoff main", false, true) {
		t.Fatalf("authorized gate apply should expose top-level report contract handoff: action=%+v next=%+v", result.MissionCommanderAction, result.MissionCommanderNextActions)
	}
	beforeStatus := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		CaseMission struct {
			AuthorizedGates        []string                      `json:"authorizedGates"`
			AuthorizedGateHandoffs []statusAuthorizedGateHandoff `json:"authorizedGateHandoffs"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("authorized gate status stdout is not JSON: %v\n%s", err, out.String())
	}
	if len(status.CaseMission.AuthorizedGates) != 1 || !containsSubstring(status.CaseMission.AuthorizedGates, "reportContract=") || len(status.CaseMission.AuthorizedGateHandoffs) != 1 {
		t.Fatalf("status should expose authorized gate handoff: %+v", status.CaseMission)
	}
	statusHandoff := status.CaseMission.AuthorizedGateHandoffs[0]
	wantStatusContract := "/rekit gate -Target " + statusQuoteCommandArg(caseRoot) + " -Pack _template -GateEventId " + authorizedEventID + " -ExecutionReportContract -Format json"
	if statusHandoff.EventID != authorizedEventID || statusHandoff.Lane != "main" || statusHandoff.Subject != "authorized debug" || statusHandoff.Action != "debug" || statusHandoff.Target != "target-alpha" || statusHandoff.Status != "authorized-gate" || statusHandoff.Authorization != "preauthorized" || statusHandoff.Profile != "prof-main-debug" || statusHandoff.ReportContract != wantStatusContract || statusHandoff.DefaultReportPath != "workspace/main/debug/session-1/adapter-report.json" || statusHandoff.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || statusHandoff.HandoffCommand != "/rekit handoff main" || !containsSubstring(statusHandoff.Evidence, "authorized outputPaths workspace/main/debug/session-1") || !containsSubstring(statusHandoff.Evidence, "authorized stopConditions timeout") || !strings.Contains(statusHandoff.ValidateBoundary, "ValidateExecutionReport") || !strings.Contains(statusHandoff.RecordBoundary, "valid=true") || statusHandoff.ReportSummary == nil || statusHandoff.ReportSummary.State != "needs-adapter-report-validation" || statusHandoff.ReportSummary.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || statusHandoff.ReportSummary.AllowedStatusCount != 5 || statusHandoff.ReportSummary.OutcomeCount != 3 || statusHandoff.LiveValidation == nil || statusHandoff.LiveValidation.CaseRelativeReportPath != "workspace/main/debug/session-1/adapter-report.json" || statusHandoff.LiveValidation.AdapterCandidateCount != 0 || statusHandoff.LiveValidation.SidecarTemplateAdapterID != "<adapter-id>" {
		t.Fatalf("unexpected status authorized gate handoff: %+v", statusHandoff)
	}
	assertSnapshotEqual(t, beforeStatus, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status case mission authorized gate：authorized debug",
		"status case mission authorized gate handoff：eventId=" + authorizedEventID + " lane=main subject=authorized debug action=debug target=target-alpha status=authorized-gate",
		"auth=preauthorized profile=prof-main-debug reportContract=" + wantStatusContract + " defaultReportPath=workspace/main/debug/session-1/adapter-report.json reportPath=workspace/main/debug/session-1/adapter-report.json handoff=/rekit handoff main",
		"status case mission authorized gate report summary：eventId=" + authorizedEventID + " state=needs-adapter-report-validation reportPath=workspace/main/debug/session-1/adapter-report.json defaultReportPath=workspace/main/debug/session-1/adapter-report.json reportPresent=false valid=false recordReady=false recordBlocked=true requiresValidation=true",
		"status case mission authorized gate live validation：eventId=" + authorizedEventID + " reportFileName=adapter-report.json caseRelativeReportPath=workspace/main/debug/session-1/adapter-report.json adapterCandidates=0 selectedAdapter= sidecarAdapter=<adapter-id>",
		"status case mission authorized gate live workspace：eventId=" + authorizedEventID + " workspace=workspace/main/debug/session-1",
		"status case mission authorized gate validate boundary：eventId=" + authorizedEventID,
		"status case mission authorized gate record boundary：eventId=" + authorizedEventID,
		"status case mission authorized gate evidence：eventId=" + authorizedEventID + " evidence=authorized outputPaths workspace/main/debug/session-1",
		"status case mission authorized gate evidence：eventId=" + authorizedEventID + " evidence=authorized stopConditions timeout",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("status authorized gate text missing %q:\n%s", expected, out.String())
		}
	}
	assertSnapshotEqual(t, beforeStatus, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "_template"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status case mission authorized gate：authorized debug",
		"status case mission authorized gate handoff：eventId=" + authorizedEventID,
		"status case mission authorized gate validate boundary：eventId=" + authorizedEventID,
		"status case mission authorized gate record boundary：eventId=" + authorizedEventID,
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("default status authorized gate text missing %q:\n%s", expected, out.String())
		}
	}
	assertSnapshotEqual(t, beforeStatus, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-ExecutionReportContract", "-GateEventId", authorizedEventID, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Kind                             string                                   `json:"kind"`
		ReportKind                       string                                   `json:"reportKind"`
		ReportSchemaVersion              int                                      `json:"reportSchemaVersion"`
		GateEventID                      string                                   `json:"gateEventId"`
		Action                           string                                   `json:"action"`
		ReportSummary                    adapterReportHandoffSummarySnapshot      `json:"reportSummary"`
		AuthorizedExecutionFollowThrough authorizedExecutionFollowThroughSnapshot `json:"authorizedExecutionFollowThrough"`
		AllowedStatuses                  []string                                 `json:"allowedStatuses"`
		AllowedOutputPaths               []string                                 `json:"allowedOutputPaths"`
		BoundaryStatusRequires           []string                                 `json:"boundaryStatusRequires"`
		StatusSummaryRequires            []string                                 `json:"statusSummaryRequires"`
		ValidationFailureStages          []struct {
			Stage string `json:"stage"`
		} `json:"validationFailureStages"`
		ValidationFailureCodes []struct {
			Code  string `json:"code"`
			Stage string `json:"stage"`
		} `json:"validationFailureCodes"`
		ValidationRepairHints []struct {
			Code                  string   `json:"code"`
			RepairAction          string   `json:"repairAction"`
			AllowedOutputPaths    []string `json:"allowedOutputPaths"`
			AllowedStopConditions []string `json:"allowedStopConditions"`
			MaxBytes              int      `json:"maxBytes"`
			Evidence              []string `json:"evidence"`
			Boundary              []string `json:"boundary"`
			RecordBlocked         bool     `json:"recordBlocked"`
			RerunValidation       bool     `json:"rerunValidation"`
		} `json:"validationRepairHints"`
		SummaryMaxBytes    int `json:"summaryMaxBytes"`
		EscalationMaxBytes int `json:"escalationMaxBytes"`
		AuthorizedBudget   struct {
			RuntimeSeconds int `json:"runtimeSeconds"`
		} `json:"authorizedBudget"`
		RefPathRequires []string `json:"refPathRequires"`
		DeniedActions   []string `json:"deniedActions"`
	}
	if err := json.Unmarshal(out.Bytes(), &contract); err != nil {
		t.Fatalf("adapter report contract stdout is not JSON: %v\n%s", err, out.String())
	}
	if contract.Kind != "adapter-execution-report-contract" || contract.ReportKind != "adapter-execution-report" || contract.ReportSchemaVersion != 1 || contract.GateEventID != authorizedEventID || contract.Action != "debug" {
		t.Fatalf("unexpected adapter report contract identity: %+v", contract)
	}
	contractSummary := contract.ReportSummary
	if contractSummary.State != "needs-adapter-report-validation" || contractSummary.GateEventID != authorizedEventID || contractSummary.Action != "debug" || contractSummary.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || contractSummary.ReportPresent || contractSummary.Valid || !contractSummary.RecordBlocked || !contractSummary.RequiresValidation || contractSummary.RequiresRepair || contractSummary.RequiresMainEscalation || contractSummary.OutcomeCount != 3 || contractSummary.NextActionCount != 3 || contractSummary.ReviewRequiredActionCount != 3 || !containsSubstring(contractSummary.Boundary, "summary is read-only") {
		t.Fatalf("adapter report contract omitted compact summary: %+v", contractSummary)
	}
	contractFollow := contract.AuthorizedExecutionFollowThrough
	if contractFollow.State != "needs-adapter-report-validation" || contractFollow.GateEventID != authorizedEventID || contractFollow.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || !cliAuthorizedFollowThroughContains(contractFollow, "write-and-validate-report", "run read-only validation") || !cliAuthorizedFollowThroughContains(contractFollow, "valid-report-record", "bounded observation evidence") || !cliAuthorizedFollowThroughContains(contractFollow, "invalid-report-repair", "record remains blocked") {
		t.Fatalf("adapter report contract omitted authorized execution follow-through: %+v", contractFollow)
	}
	if strings.Join(contract.AllowedStatuses, ",") != "succeeded,failed,boundary-hit,escalated,aborted" || strings.Join(contract.AllowedOutputPaths, ",") != "workspace/main/debug/session-1" || contract.AuthorizedBudget.RuntimeSeconds != 30 || contract.SummaryMaxBytes != 4096 || contract.EscalationMaxBytes != 4096 || !containsSubstring(contract.RefPathRequires, "evidenceRefs must stay under authorized outputPaths") || !containsSubstring(contract.BoundaryStatusRequires, "boundaryHits or escalation") || !containsSubstring(contract.BoundaryStatusRequires, "authorized stopConditions") || !containsSubstring(contract.StatusSummaryRequires, "failed/boundary-hit/escalated/aborted") || !slices.Contains(contract.DeniedActions, "heavy-tool execution") {
		t.Fatalf("adapter report contract omitted live validation boundaries: %+v", contract)
	}
	contractStages := []string{}
	for _, stage := range contract.ValidationFailureStages {
		contractStages = append(contractStages, stage.Stage)
	}
	contractCodes := []string{}
	for _, code := range contract.ValidationFailureCodes {
		contractCodes = append(contractCodes, code.Code+":"+code.Stage)
	}
	if !slices.Contains(contractStages, "decode") || !slices.Contains(contractStages, "boundary") || !slices.Contains(contractCodes, "report-json-invalid:decode") || !slices.Contains(contractCodes, "evidence-refs-out-of-scope:refs") || !slices.Contains(contractCodes, "boundary-marker-missing:boundary") || !slices.Contains(contractCodes, "boundary-hits-not-authorized:boundary") || !slices.Contains(contractCodes, "status-summary-missing:summary") {
		t.Fatalf("adapter report contract omitted validation failure taxonomy: stages=%v codes=%v", contractStages, contractCodes)
	}
	contractRepairHints := map[string]int{}
	for i, hint := range contract.ValidationRepairHints {
		contractRepairHints[hint.Code] = i
	}
	requiredContractHints := []string{"", "report-json-invalid", "evidence-refs-out-of-scope", "boundary-marker-missing", "status-summary-missing"}
	for _, code := range requiredContractHints {
		if _, ok := contractRepairHints[code]; !ok {
			t.Fatalf("adapter report contract omitted validation repair hint %q: %+v", code, contract.ValidationRepairHints)
		}
	}
	missingReportPathHint := contract.ValidationRepairHints[contractRepairHints[""]]
	reportJSONHint := contract.ValidationRepairHints[contractRepairHints["report-json-invalid"]]
	evidenceRefsHint := contract.ValidationRepairHints[contractRepairHints["evidence-refs-out-of-scope"]]
	boundaryHint := contract.ValidationRepairHints[contractRepairHints["boundary-marker-missing"]]
	summaryHint := contract.ValidationRepairHints[contractRepairHints["status-summary-missing"]]
	if missingReportPathHint.RepairAction != "provide-execution-report-path" || reportJSONHint.RepairAction != "fix-report-json" || evidenceRefsHint.RepairAction != "move-evidence-refs-under-authorized-output-paths" || boundaryHint.RepairAction != "add-boundary-marker" || strings.Join(evidenceRefsHint.AllowedOutputPaths, ",") != "workspace/main/debug/session-1" || strings.Join(boundaryHint.AllowedStopConditions, ",") != "timeout" || summaryHint.MaxBytes != 4096 || !boundaryHint.RecordBlocked || !boundaryHint.RerunValidation || !containsSubstring(boundaryHint.Evidence, "allowedStopConditions=timeout") || !containsSubstring(boundaryHint.Boundary, "boundaryHits must use authorized stopConditions: timeout") || !containsSubstring(evidenceRefsHint.Boundary, "stay under authorized outputPaths: workspace/main/debug/session-1") {
		t.Fatalf("adapter report contract omitted validation repair hints: %+v", contract.ValidationRepairHints)
	}
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-GateEventId", authorizedEventID, "-ExecutionStatus", "succeeded", "-Actor", "executor-1", "-ActualRuntimeSeconds", "25", "-ActualDiskMB", "32", "-ActualRequests", "1", "-OutputRefs", "workspace/main/debug/session-1/result.json", "-ExecutionEvidenceRefs", "workspace/main/debug/session-1/result.json", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Command           string `json:"command"`
		Applied           bool   `json:"applied"`
		Path              string `json:"path"`
		ExecutionEvidence struct {
			Kind    string   `json:"kind"`
			Lane    string   `json:"lane"`
			Status  string   `json:"status"`
			Related []string `json:"related"`
			Gate    struct {
				Action        string `json:"action"`
				Authorization struct {
					Decision string `json:"decision"`
				} `json:"authorization"`
			} `json:"gate"`
			Execution struct {
				GateEventID   string   `json:"gateEventId"`
				Authorization string   `json:"authorization"`
				OutputRefs    []string `json:"outputRefs"`
				ActualBudget  struct {
					RuntimeSeconds int `json:"runtimeSeconds"`
					DiskMB         int `json:"diskMB"`
					Requests       int `json:"requests"`
				} `json:"actualBudget"`
				Escalation string `json:"escalation"`
				Adapter    struct {
					AdapterID  string `json:"adapterId"`
					Escalation string `json:"escalation"`
				} `json:"adapter"`
			} `json:"execution"`
		} `json:"executionEvidence"`
	}
	if err := json.Unmarshal(out.Bytes(), &evidence); err != nil {
		t.Fatalf("execution evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if evidence.Command != "gate" || !evidence.Applied || evidence.Path != ".rekit/facts/observations.jsonl" || evidence.ExecutionEvidence.Kind != "observation" || evidence.ExecutionEvidence.Lane != "main" || evidence.ExecutionEvidence.Status != "succeeded" {
		t.Fatalf("unexpected execution evidence result: %+v", evidence)
	}
	if strings.Join(evidence.ExecutionEvidence.Related, ",") != authorizedEventID || evidence.ExecutionEvidence.Execution.GateEventID != authorizedEventID || evidence.ExecutionEvidence.Execution.Authorization != "preauthorized" || evidence.ExecutionEvidence.Execution.ActualBudget.RuntimeSeconds != 25 || strings.Join(evidence.ExecutionEvidence.Execution.OutputRefs, ",") != "workspace/main/debug/session-1/result.json" {
		t.Fatalf("execution evidence did not preserve gate provenance: %+v", evidence.ExecutionEvidence)
	}
	var evidenceCommander struct {
		MissionCommanderAction struct {
			State            string   `json:"state"`
			PrimaryCommand   string   `json:"primaryCommand"`
			FollowUpCommands []string `json:"followUpCommands"`
			Boundary         []string `json:"boundary"`
		} `json:"missionCommanderAction"`
		ExecutionEvidenceReview     []executionEvidenceReviewItem    `json:"executionEvidenceReview"`
		MissionCommanderNextActions []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		NextSteps                   []string                         `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &evidenceCommander); err != nil {
		t.Fatalf("execution evidence commander envelope is not JSON: %v\n%s", err, out.String())
	}
	if evidenceCommander.MissionCommanderAction.State != "ready-for-evidence-review" || evidenceCommander.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || !containsSubstring(evidenceCommander.MissionCommanderAction.FollowUpCommands, "/rekit continue main -WhatIf") || !containsSubstring(evidenceCommander.MissionCommanderAction.Boundary, "bounded observation evidence") || !containsSubstring(evidenceCommander.MissionCommanderAction.Boundary, "did not execute the heavy tool") || !containsSubstring(evidenceCommander.NextSteps, "/rekit handoff main") {
		t.Fatalf("execution evidence omitted Mission Commander review handoff: action=%+v next=%+v", evidenceCommander.MissionCommanderAction, evidenceCommander.NextSteps)
	}
	if len(evidenceCommander.ExecutionEvidenceReview) != 1 || evidenceCommander.ExecutionEvidenceReview[0].GateEventID != authorizedEventID || evidenceCommander.ExecutionEvidenceReview[0].Status != "succeeded" || evidenceCommander.ExecutionEvidenceReview[0].MissionCommanderAction.State != "ready-for-evidence-review" || len(evidenceCommander.MissionCommanderNextActions) != 5 || evidenceCommander.MissionCommanderNextActions[0].Source != "executionEvidenceReview" || evidenceCommander.MissionCommanderNextActions[0].Command != "/rekit handoff main" || evidenceCommander.MissionCommanderNextActions[1].Command != "/rekit overview" || evidenceCommander.MissionCommanderNextActions[2].Command != "/rekit continue main -WhatIf" || evidenceCommander.MissionCommanderNextActions[3].Source != "missionCommanderActions" || evidenceCommander.MissionCommanderNextActions[3].Command != "/rekit continue main" || evidenceCommander.MissionCommanderNextActions[4].Source != "missionCommanderActions.followUp" || evidenceCommander.MissionCommanderNextActions[4].Command != "/rekit handoff main" || !containsSubstring(evidenceCommander.MissionCommanderNextActions[4].Reasons, "follow Mission Commander handoff after primary action") {
		t.Fatalf("execution evidence omitted review queue or next actions: review=%+v next=%+v", evidenceCommander.ExecutionEvidenceReview, evidenceCommander.MissionCommanderNextActions)
	}
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-escalation.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "cli-adapter",
  "action": "debug",
  "status": "escalated",
  "gateEventId": "`+authorizedEventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/adapter-result.json"],
  "escalation": "adapter escalated from CLI E2E",
  "summary": "Adapter reported an escalation"
}`)
	observationsBeforeValidation, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-ValidateExecutionReport", "-GateEventId", authorizedEventID, "-ExecutionReportPath", "workspace/main/debug/session-1/adapter-escalation.json", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var adapterValidation struct {
		Kind          string                              `json:"kind"`
		Valid         bool                                `json:"valid"`
		IsMutation    bool                                `json:"isMutation"`
		Applied       bool                                `json:"applied"`
		Error         string                              `json:"error"`
		ReportPath    string                              `json:"reportPath"`
		ReportSummary adapterReportHandoffSummarySnapshot `json:"reportSummary"`
		Report        *struct {
			AdapterID  string `json:"adapterId"`
			Escalation string `json:"escalation"`
		} `json:"report"`
		Contract struct {
			GateEventID string `json:"gateEventId"`
			Action      string `json:"action"`
		} `json:"contract"`
	}
	if err := json.Unmarshal(out.Bytes(), &adapterValidation); err != nil {
		t.Fatalf("adapter execution report validation stdout is not JSON: %v\n%s", err, out.String())
	}
	if adapterValidation.Kind != "adapter-execution-report-validation" || !adapterValidation.Valid || adapterValidation.IsMutation || adapterValidation.Applied || adapterValidation.Error != "" || adapterValidation.ReportPath != "workspace/main/debug/session-1/adapter-escalation.json" || adapterValidation.Report == nil || adapterValidation.Report.AdapterID != "cli-adapter" || adapterValidation.Report.Escalation != "adapter escalated from CLI E2E" || adapterValidation.Contract.GateEventID != authorizedEventID || adapterValidation.Contract.Action != "debug" {
		t.Fatalf("adapter execution report validation drifted: %+v", adapterValidation)
	}
	validationSummary := adapterValidation.ReportSummary
	if validationSummary.State != "ready-to-record-evidence" || validationSummary.GateEventID != authorizedEventID || validationSummary.Action != "debug" || validationSummary.ReportPath != "workspace/main/debug/session-1/adapter-escalation.json" || !validationSummary.ReportPresent || !validationSummary.Valid || !validationSummary.RecordReady || validationSummary.RecordBlocked || validationSummary.RequiresValidation || validationSummary.RequiresRepair || validationSummary.RequiresMainEscalation || validationSummary.ReportStatus != "escalated" || validationSummary.AdapterID != "cli-adapter" || validationSummary.ActualRuntimeSeconds != 24 || validationSummary.ActualDiskMB != 33 || validationSummary.ActualRequests != 1 || validationSummary.OutputRefCount != 1 || validationSummary.EvidenceRefCount != 0 || validationSummary.BoundaryHitCount != 0 || !validationSummary.HasEscalation || !validationSummary.HasSummary || validationSummary.OutcomeCount != 1 || validationSummary.NextActionCount != 2 || validationSummary.ReviewRequiredActionCount != 2 || !containsSubstring(validationSummary.Boundary, "valid=true") {
		t.Fatalf("adapter execution report validation omitted compact summary: %+v", validationSummary)
	}
	observationsAfterValidation, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(observationsAfterValidation) != string(observationsBeforeValidation) {
		t.Fatalf("read-only adapter validation changed observations before record:\nbefore=%s\nafter=%s", string(observationsBeforeValidation), string(observationsAfterValidation))
	}
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-invalid.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "cli-adapter",
  "action": "debug",
  "status": "boundary-hit",
  "gateEventId": "`+authorizedEventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/adapter-result.json"],
  "summary": "Adapter reported a boundary hit without marker"
}`)
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-ValidateExecutionReport", "-GateEventId", authorizedEventID, "-ExecutionReportPath", "workspace/main/debug/session-1/adapter-invalid.json", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var invalidAdapterValidation struct {
		Valid         bool                                `json:"valid"`
		Error         string                              `json:"error"`
		Errors        []string                            `json:"errors"`
		FailureCode   string                              `json:"failureCode"`
		FailureStage  string                              `json:"failureStage"`
		ReportSummary adapterReportHandoffSummarySnapshot `json:"reportSummary"`
		RepairHints   []struct {
			Code                  string   `json:"code"`
			Stage                 string   `json:"stage"`
			RepairAction          string   `json:"repairAction"`
			Fields                []string `json:"fields"`
			AllowedStopConditions []string `json:"allowedStopConditions"`
			RecordBlocked         bool     `json:"recordBlocked"`
			RerunValidation       bool     `json:"rerunValidation"`
		} `json:"repairHints"`
		NextSteps  []string `json:"nextSteps"`
		IsMutation bool     `json:"isMutation"`
		Applied    bool     `json:"applied"`
		ReportPath string   `json:"reportPath"`
		Report     *struct {
			Status string `json:"status"`
		} `json:"report"`
	}
	if err := json.Unmarshal(out.Bytes(), &invalidAdapterValidation); err != nil {
		t.Fatalf("invalid adapter execution report validation stdout is not JSON: %v\n%s", err, out.String())
	}
	if invalidAdapterValidation.Valid || invalidAdapterValidation.Error == "" || !strings.Contains(invalidAdapterValidation.Error, "requires boundaryHits or escalation") || len(invalidAdapterValidation.Errors) != 1 || invalidAdapterValidation.FailureCode != "boundary-marker-missing" || invalidAdapterValidation.FailureStage != "boundary" || invalidAdapterValidation.IsMutation || invalidAdapterValidation.Applied || invalidAdapterValidation.ReportPath != "workspace/main/debug/session-1/adapter-invalid.json" || invalidAdapterValidation.Report == nil || invalidAdapterValidation.Report.Status != "boundary-hit" {
		t.Fatalf("invalid adapter execution report validation drifted: %+v", invalidAdapterValidation)
	}
	invalidValidationSummary := invalidAdapterValidation.ReportSummary
	if invalidValidationSummary.State != "repair-adapter-report" || invalidValidationSummary.GateEventID != authorizedEventID || invalidValidationSummary.Action != "debug" || invalidValidationSummary.ReportPath != "workspace/main/debug/session-1/adapter-invalid.json" || !invalidValidationSummary.ReportPresent || invalidValidationSummary.Valid || invalidValidationSummary.RecordReady || !invalidValidationSummary.RecordBlocked || invalidValidationSummary.RequiresValidation || !invalidValidationSummary.RequiresRepair || invalidValidationSummary.RequiresMainEscalation || invalidValidationSummary.ReportStatus != "boundary-hit" || invalidValidationSummary.AdapterID != "cli-adapter" || invalidValidationSummary.ActualRuntimeSeconds != 24 || invalidValidationSummary.ActualDiskMB != 33 || invalidValidationSummary.ActualRequests != 1 || invalidValidationSummary.OutputRefCount != 1 || invalidValidationSummary.EvidenceRefCount != 0 || invalidValidationSummary.BoundaryHitCount != 0 || invalidValidationSummary.HasEscalation || !invalidValidationSummary.HasSummary || invalidValidationSummary.ValidationFailureCode != "boundary-marker-missing" || invalidValidationSummary.ValidationFailureStage != "boundary" || invalidValidationSummary.RepairHintCount != 1 || invalidValidationSummary.RecordBlockedHintCount != 1 || invalidValidationSummary.EscalateHintCount != 0 || invalidValidationSummary.OutcomeCount != 1 || invalidValidationSummary.NextActionCount != 2 || invalidValidationSummary.ReviewRequiredActionCount != 2 || !containsSubstring(invalidValidationSummary.Boundary, "valid=true") {
		t.Fatalf("invalid adapter execution report validation omitted compact summary: %+v", invalidValidationSummary)
	}
	if len(invalidAdapterValidation.RepairHints) != 1 || invalidAdapterValidation.RepairHints[0].Code != "boundary-marker-missing" || invalidAdapterValidation.RepairHints[0].Stage != "boundary" || invalidAdapterValidation.RepairHints[0].RepairAction != "add-boundary-marker" || strings.Join(invalidAdapterValidation.RepairHints[0].AllowedStopConditions, ",") != "timeout" || !invalidAdapterValidation.RepairHints[0].RecordBlocked || !invalidAdapterValidation.RepairHints[0].RerunValidation || !strings.Contains(strings.Join(invalidAdapterValidation.NextSteps, ","), "repairAction: add-boundary-marker") {
		t.Fatalf("invalid adapter execution report validation omitted repair hints: %+v", invalidAdapterValidation)
	}
	observationsAfterInvalidValidation, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(observationsAfterInvalidValidation) != string(observationsBeforeValidation) {
		t.Fatalf("invalid read-only adapter validation changed observations before record:\nbefore=%s\nafter=%s", string(observationsBeforeValidation), string(observationsAfterInvalidValidation))
	}
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "_template", "-Apply", "-GateEventId", authorizedEventID, "-ExecutionReportPath", "workspace/main/debug/session-1/adapter-escalation.json", "-Actor", "executor-1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var adapterEvidence struct {
		Applied           bool   `json:"applied"`
		EventID           string `json:"eventId"`
		ExecutionEvidence struct {
			Status    string `json:"status"`
			Execution struct {
				Escalation string `json:"escalation"`
				Adapter    struct {
					AdapterID  string `json:"adapterId"`
					Escalation string `json:"escalation"`
				} `json:"adapter"`
			} `json:"execution"`
		} `json:"executionEvidence"`
	}
	if err := json.Unmarshal(out.Bytes(), &adapterEvidence); err != nil {
		t.Fatalf("adapter execution evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if !adapterEvidence.Applied || adapterEvidence.ExecutionEvidence.Status != "escalated" || adapterEvidence.ExecutionEvidence.Execution.Escalation != "adapter escalated from CLI E2E" || adapterEvidence.ExecutionEvidence.Execution.Adapter.AdapterID != "cli-adapter" || adapterEvidence.ExecutionEvidence.Execution.Adapter.Escalation != "adapter escalated from CLI E2E" {
		t.Fatalf("adapter execution report evidence drifted: %+v", adapterEvidence)
	}
	var adapterEvidenceCommander struct {
		MissionCommanderAction struct {
			State            string   `json:"state"`
			PrimaryCommand   string   `json:"primaryCommand"`
			FollowUpCommands []string `json:"followUpCommands"`
			Boundary         []string `json:"boundary"`
		} `json:"missionCommanderAction"`
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
		MissionCommanderNextActions    []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
		NextSteps                      []string                               `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &adapterEvidenceCommander); err != nil {
		t.Fatalf("adapter execution evidence commander envelope is not JSON: %v\n%s", err, out.String())
	}
	if adapterEvidenceCommander.MissionCommanderAction.State != "needs-main-escalation" || adapterEvidenceCommander.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || containsSubstring(adapterEvidenceCommander.MissionCommanderAction.FollowUpCommands, "/rekit continue") || !containsSubstring(adapterEvidenceCommander.MissionCommanderAction.Boundary, "stop autonomous work") || !containsSubstring(adapterEvidenceCommander.MissionCommanderAction.Boundary, "no authority/confirmed") || !containsSubstring(adapterEvidenceCommander.NextSteps, "boundary hit or escalation") {
		t.Fatalf("adapter execution evidence omitted Mission Commander escalation handoff: action=%+v next=%+v", adapterEvidenceCommander.MissionCommanderAction, adapterEvidenceCommander.NextSteps)
	}
	if len(adapterEvidenceCommander.ExecutionEvidenceReview) != 1 || adapterEvidenceCommander.ExecutionEvidenceReview[0].GateEventID != authorizedEventID || adapterEvidenceCommander.ExecutionEvidenceReview[0].Status != "escalated" || adapterEvidenceCommander.ExecutionEvidenceReview[0].MissionCommanderAction.State != "needs-main-escalation" || len(adapterEvidenceCommander.MissionCommanderNextActions) != 2 || adapterEvidenceCommander.MissionCommanderNextActions[0].Command != "/rekit handoff main" || adapterEvidenceCommander.MissionCommanderNextActions[0].Source != "executionEvidenceReview" || adapterEvidenceCommander.MissionCommanderNextActions[1].Command != "/rekit overview" || containsMissionCommanderNextActionsCommand(adapterEvidenceCommander.MissionCommanderNextActions, "/rekit continue main") || containsMissionCommanderNextActionsCommand(adapterEvidenceCommander.MissionCommanderNextActions, "/rekit continue main -WhatIf") {
		t.Fatalf("adapter execution evidence omitted review queue or suppressed next actions: review=%+v next=%+v", adapterEvidenceCommander.ExecutionEvidenceReview, adapterEvidenceCommander.MissionCommanderNextActions)
	}
	assertEscalatedAdapterExecutionEvidenceReviewSummary(t, "immediate adapter escalation JSON", adapterEvidenceCommander.ExecutionEvidenceReviewSummary, adapterEvidence.EventID, authorizedEventID, 1, 0, 1, 0, 1)
	observations, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(observations), `"gateEventId":"`+authorizedEventID+`"`) || !strings.Contains(string(observations), `"authorization":"preauthorized"`) || !strings.Contains(string(observations), `"escalation":"adapter escalated from CLI E2E"`) || strings.Contains(string(observations), `"kind":"authority"`) || strings.Contains(string(observations), `"kind":"confirmed"`) {
		t.Fatalf("execution evidence ledger mismatch:\n%s", string(observations))
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
	for _, expected := range []string{"authorized gates:", "authorized debug", "authorized-gate（durable autonomy 已授权，非阻塞）", "requestedBudget=runtimeSeconds=30,diskMB=64,requests=1", "outputPaths=workspace/main/debug/session-1", "stopConditions=timeout", "eventId=" + authorizedEventID, "reportContract=" + reportContract, "auth=preauthorized", "profile=prof-main-debug", "follow-through: state=ready-for-evidence-review", "outcome: name=recorded-evidence-review", "when: bounded observation evidence was recorded for an authorized gate", "evidence: workspace/main/debug/session-1/result.json", "follow-through: state=needs-main-escalation", "outcome: name=boundary-or-escalation-review", "when: recorded evidence reports boundaryHits, escalation, boundary-hit, or escalated status", "evidence: workspace/main/debug/session-1/adapter-result.json"} {
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
		MissionBrief                  missionBrief                           `json:"missionBrief"`
		LaneExecutorActions           []handoffLaneExecutorAction            `json:"laneExecutorActions"`
		AuthorizedGateAdapterHandoffs []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
		NextSteps                     []string                               `json:"nextSteps"`
		Sections                      struct {
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
	if overview.Sections.PendingGates.Total != 0 || overview.Sections.AuthorizedGates.Total != 1 || len(overview.Sections.AuthorizedGates.Events) != 1 || !containsSubstring(overview.MissionBrief.AuthorizedGates, "authorized debug") || !containsSubstring(overview.MissionBrief.AuthorizedGates, "outputPaths=workspace/main/debug/session-1") || !containsSubstring(overview.MissionBrief.AuthorizedGates, "stopConditions=timeout") || !containsSubstring(overview.MissionBrief.AuthorizedGates, "eventId="+authorizedEventID) || !containsSubstring(overview.MissionBrief.AuthorizedGates, "reportContract="+reportContract) {
		t.Fatalf("overview JSON missing authorized gate visibility: %+v", overview)
	}
	assertAuthorizedGateAdapterHandoffSnapshot(t, "overview", overview.AuthorizedGateAdapterHandoffs, authorizedEventID, wantContractCommand)
	if len(overview.LaneExecutorActions) != 1 || overview.LaneExecutorActions[0].ExecutorAction.Blocked || !overview.LaneExecutorActions[0].ExecutorAction.Ready || overview.LaneExecutorActions[0].ExecutorAction.PendingGates != 0 || !slices.Equal(overview.LaneExecutorActions[0].ExecutorAction.NextAgentActions, []string{"/rekit continue main"}) {
		t.Fatalf("authorized gate overview executor action should remain non-blocking: %+v", overview.LaneExecutorActions)
	}
	if !containsSubstring(overview.NextSteps, "review execution evidence for gateEventId "+authorizedEventID) || !slices.Contains(overview.NextSteps, "/rekit handoff main") || containsSubstring(overview.NextSteps, "/rekit continue main") || !containsSubstring(overview.NextSteps, "boundary hit or escalation") {
		t.Fatalf("authorized gate overview should prioritize evidence review and suppress autonomous continue while escalation exists: %+v", overview.NextSteps)
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	project := decodeHandoffResult(t, out.Bytes())
	if !containsSubstring(project.MissionBrief.AuthorizedGates, "authorized debug") || !containsSubstring(project.MissionBrief.AuthorizedGates, "outputPaths=workspace/main/debug/session-1") || !containsSubstring(project.MissionBrief.AuthorizedGates, "stopConditions=timeout") || !containsSubstring(project.MissionBrief.AuthorizedGates, "eventId="+authorizedEventID) || !containsSubstring(project.MissionBrief.AuthorizedGates, "reportContract="+reportContract) || len(project.MissionBrief.PendingGates) != 0 {
		t.Fatalf("project handoff JSON missing authorized gate visibility: %+v", project.MissionBrief)
	}
	assertAuthorizedGateAdapterHandoffSnapshot(t, "project handoff", project.AuthorizedGateAdapterHandoffs, authorizedEventID, wantContractCommand)
	if len(project.ExecutionEvidenceReview) != 2 || project.ExecutionEvidenceReview[0].GateEventID != authorizedEventID || project.ExecutionEvidenceReview[0].Status != "succeeded" || !containsSubstring(project.ExecutionEvidenceReview[0].OutputRefs, "workspace/main/debug/session-1/result.json") || !containsSubstring(project.ExecutionEvidenceReview[0].EvidenceRefs, "workspace/main/debug/session-1/result.json") || project.ExecutionEvidenceReview[0].HandoffCommand != "/rekit handoff main" || !containsSubstring(project.ExecutionEvidenceReview[0].Boundary, "do not replay heavy tool") || project.ExecutionEvidenceReview[0].MissionCommanderAction.State != "ready-for-evidence-review" || !containsSubstring(project.ExecutionEvidenceReview[0].MissionCommanderAction.FollowUpCommands, "/rekit continue main -WhatIf") || project.ExecutionEvidenceReview[0].FollowThrough.State != "ready-for-evidence-review" || !cliExecutionEvidenceFollowThroughContains(project.ExecutionEvidenceReview[0].FollowThrough, "recorded-evidence-review", "reviewed outputRefs/evidenceRefs") || project.ExecutionEvidenceReview[1].Status != "escalated" || project.ExecutionEvidenceReview[1].Escalation != "adapter escalated from CLI E2E" || !containsSubstring(project.ExecutionEvidenceReview[1].Boundary, "requires main review") || project.ExecutionEvidenceReview[1].MissionCommanderAction.State != "needs-main-escalation" || containsSubstring(project.ExecutionEvidenceReview[1].MissionCommanderAction.FollowUpCommands, "/rekit continue main") || project.ExecutionEvidenceReview[1].FollowThrough.State != "needs-main-escalation" || !cliExecutionEvidenceFollowThroughContains(project.ExecutionEvidenceReview[1].FollowThrough, "boundary-or-escalation-review", "main Agent reviews boundary/escalation") {
		t.Fatalf("project handoff JSON missing execution evidence review queue: %+v", project.ExecutionEvidenceReview)
	}
	if !containsSubstring(project.NextSteps, "review execution evidence for gateEventId "+authorizedEventID) || !slices.Contains(project.NextSteps, "/rekit handoff main") || containsSubstring(project.NextSteps, "/rekit continue main") || !containsSubstring(project.NextSteps, "boundary hit or escalation") {
		t.Fatalf("project handoff next steps should route through evidence review before continuation: %+v", project.NextSteps)
	}
	assertEvidenceReviewBeforeBlockedAdapterActions(t, "project handoff", project.MissionCommanderNextActions, authorizedEventID)
	if project.MissionCommanderActionQueue.CurrentAction == nil || project.MissionCommanderActionQueue.CurrentAction.Source != "executionEvidenceReview" || project.MissionCommanderActionQueue.CurrentAction.Command != "/rekit handoff main" {
		t.Fatalf("project handoff queue should keep evidence review current: %+v", project.MissionCommanderActionQueue)
	}
	projectLatest := assertStartWrite(t, project.Writes, ".rekit/handovers/latest.md", "write-latest-project-handoff")
	projectText, err := os.ReadFile(projectLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"authorized gates:", "authorized debug", "requestedBudget=runtimeSeconds=30,diskMB=64,requests=1", "outputPaths=workspace/main/debug/session-1", "stopConditions=timeout", "eventId=" + authorizedEventID, "reportContract=" + reportContract, "auth=preauthorized", "evidence next action：review execution evidence for gateEventId " + authorizedEventID, "evidence next action：boundary hit or escalation in execution evidence", "evidence review 后继续候选：`/rekit continue main`", "execution evidence review：execution evidence for authorized debug status=succeeded gateEventId=" + authorizedEventID, "execution evidence review：execution evidence for authorized debug status=escalated gateEventId=" + authorizedEventID, "evidence review command：`review outputRefs/evidenceRefs for gateEventId " + authorizedEventID + "`", "evidence handoff：`/rekit handoff main`", "evidence commander：state=ready-for-evidence-review primary=`/rekit handoff main`", "evidence commander：state=needs-main-escalation primary=`/rekit handoff main`", "evidence follow-through：state=ready-for-evidence-review", "evidence follow-through outcome：name=recorded-evidence-review", "evidence follow-through when：name=recorded-evidence-review when=bounded observation evidence was recorded for an authorized gate", "evidence follow-through evidence：name=recorded-evidence-review evidence=workspace/main/debug/session-1/result.json", "evidence follow-through：state=needs-main-escalation", "evidence follow-through outcome：name=boundary-or-escalation-review", "evidence follow-through when：name=boundary-or-escalation-review when=recorded evidence reports boundaryHits, escalation, boundary-hit, or escalated status", "evidence follow-through evidence：name=boundary-or-escalation-review evidence=workspace/main/debug/session-1/adapter-result.json", "evidence commander follow-up：/rekit overview", "evidence commander follow-up：/rekit continue main -WhatIf", "evidence boundary：observation evidence is already recorded; do not replay heavy tool", "evidence boundary：boundary/escalation requires main review before autonomous continuation"} {
		if !strings.Contains(string(projectText), expected) {
			t.Fatalf("project handoff missing %q:\n%s", expected, string(projectText))
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "_template", "-Apply", "main"}, &out); err != nil {
		t.Fatal(err)
	}
	lane := decodeHandoffResult(t, out.Bytes())
	if !containsSubstring(lane.MissionBrief.AuthorizedGates, "authorized debug") || !containsSubstring(lane.MissionBrief.AuthorizedGates, "outputPaths=workspace/main/debug/session-1") || !containsSubstring(lane.MissionBrief.AuthorizedGates, "stopConditions=timeout") || !containsSubstring(lane.MissionBrief.AuthorizedGates, "eventId="+authorizedEventID) || !containsSubstring(lane.MissionBrief.AuthorizedGates, "reportContract="+reportContract) || len(lane.MissionBrief.PendingGates) != 0 {
		t.Fatalf("lane handoff JSON missing authorized gate visibility: %+v", lane.MissionBrief)
	}
	assertAuthorizedGateAdapterHandoffSnapshot(t, "lane handoff", lane.AuthorizedGateAdapterHandoffs, authorizedEventID, wantContractCommand)
	if len(lane.ExecutionEvidenceReview) != 2 || lane.ExecutionEvidenceReview[0].GateEventID != authorizedEventID || lane.ExecutionEvidenceReview[0].Status != "succeeded" || lane.ExecutionEvidenceReview[0].MissionCommanderAction.State != "ready-for-evidence-review" || lane.ExecutionEvidenceReview[1].Status != "escalated" || !containsSubstring(lane.ExecutionEvidenceReview[1].Boundary, "requires main review") || lane.ExecutionEvidenceReview[1].MissionCommanderAction.State != "needs-main-escalation" {
		t.Fatalf("lane handoff JSON missing execution evidence review queue: %+v", lane.ExecutionEvidenceReview)
	}
	if !containsSubstring(lane.NextSteps, "review execution evidence for gateEventId "+authorizedEventID) || !slices.Contains(lane.NextSteps, "/rekit handoff main") || containsSubstring(lane.NextSteps, "/rekit continue main") || !containsSubstring(lane.NextSteps, "boundary hit or escalation") {
		t.Fatalf("lane handoff next steps should route through evidence review before continuation: %+v", lane.NextSteps)
	}
	assertEvidenceReviewBeforeBlockedAdapterActions(t, "lane handoff", lane.MissionCommanderNextActions, authorizedEventID)
	if lane.MissionCommanderActionQueue.CurrentAction == nil || lane.MissionCommanderActionQueue.CurrentAction.Source != "executionEvidenceReview" || lane.MissionCommanderActionQueue.CurrentAction.Command != "/rekit handoff main" {
		t.Fatalf("lane handoff queue should keep evidence review current: %+v", lane.MissionCommanderActionQueue)
	}
	laneLatest := assertStartWrite(t, lane.Writes, ".rekit/handovers/main-latest.md", "write-latest-lane-handoff")
	laneText, err := os.ReadFile(laneLatest.TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"当前不要执行 `/rekit continue main`", "evidence next action:", "boundary hit or escalation in execution evidence", "## Mission Commander next actions", "state=ready-for-evidence-review source=executionEvidenceReview blocked=true requiresReview=true command=`/rekit handoff main`", "state=ready-for-evidence-review source=executionEvidenceReview.followUp blocked=true requiresReview=true command=`/rekit overview`", "authorized-gate:", "## authorized-gate", "authorized debug", "requestedBudget=runtimeSeconds=30,diskMB=64,requests=1", "outputPaths=workspace/main/debug/session-1", "stopConditions=timeout", "eventId=" + authorizedEventID, "reportContract=" + reportContract, "auth=preauthorized", "## execution evidence review", "status=succeeded | gateEventId=" + authorizedEventID, "status=escalated | gateEventId=" + authorizedEventID, "review command: `review outputRefs/evidenceRefs for gateEventId " + authorizedEventID + "`", "handoff command: `/rekit handoff main`", "commander state: ready-for-evidence-review", "commander state: needs-main-escalation", "commander primary: `/rekit handoff main`", "follow-through: state=ready-for-evidence-review", "outcome: name=recorded-evidence-review", "when: bounded observation evidence was recorded for an authorized gate", "evidence: workspace/main/debug/session-1/result.json", "follow-through: state=needs-main-escalation", "outcome: name=boundary-or-escalation-review", "when: recorded evidence reports boundaryHits, escalation, boundary-hit, or escalated status", "evidence: workspace/main/debug/session-1/adapter-result.json", "commander follow-up:", "/rekit overview", "/rekit continue main -WhatIf", "review boundary:", "observation evidence is already recorded; do not replay heavy tool", "boundary/escalation requires main review before autonomous continuation"} {
		if !strings.Contains(string(laneText), expected) {
			t.Fatalf("lane handoff missing %q:\n%s", expected, string(laneText))
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "main", "-WhatIf", "-Executor", "session-authorized-preview", "-Actor", "mission-commander", "-Reason", "preview authorized adapter handoff"}, &out); err != nil {
		t.Fatal(err)
	}
	startPreview := decodeStartResult(t, out.Bytes())
	if startPreview.IsMutation || startPreview.Applied || startPreview.Lane.ID != "main" {
		t.Fatalf("unexpected authorized gate start preview: %+v", startPreview)
	}
	assertAuthorizedGateAdapterHandoffSnapshot(t, "start preview", startPreview.AuthorizedGateAdapterHandoffs, authorizedEventID, wantContractCommand)
	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "main", "-WhatIf", "-Executor", "session-authorized-preview", "-Actor", "mission-commander", "-Reason", "preview authorized adapter handoff", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"start authorized gate adapter handoff：eventId=" + authorizedEventID, "start authorized gate adapter report summary：eventId=" + authorizedEventID, "start authorized gate adapter live validation：eventId=" + authorizedEventID, "start authorized gate adapter boundary：eventId=" + authorizedEventID + " boundary=projection may read and validate an existing canonical sidecar"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("start text missing %q:\n%s", expected, out.String())
		}
	}
	out.Reset()
	if err := Run([]string{"-Command", "start", "-Target", caseRoot, "-Pack", "_template", "-Name", "main", "-Apply", "-Executor", "session-authorized", "-Actor", "mission-commander", "-Reason", "take over authorized adapter handoff"}, &out); err != nil {
		t.Fatal(err)
	}
	startApply := decodeStartResult(t, out.Bytes())
	if !startApply.IsMutation || !startApply.Applied || startApply.Lane.ID != "main" || startApply.Lane.CurrentExecutor != "session-authorized" {
		t.Fatalf("unexpected authorized gate start apply: %+v", startApply)
	}
	assertAuthorizedGateAdapterHandoffSnapshot(t, "start apply", startApply.AuthorizedGateAdapterHandoffs, authorizedEventID, wantContractCommand)

	writeCaseFile(t, caseRoot, ".rekit/facts/interventions.jsonl", `{"eventId":"int-authorized-main","kind":"intervention","lane":"main","subject":"authorized handoff reconcile","summary":"refresh executor after authorized gate","action":"override","status":"open","target":"workspace/main/main"}`+"\n")
	out.Reset()
	if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "main", "-InterventionId", "int-authorized-main", "-Executor", "session-reconcile", "-Actor", "mission-commander", "-Reason", "resolve authorized adapter handoff intervention"}, &out); err != nil {
		t.Fatal(err)
	}
	var reconcilePreview struct {
		Command                       string                                 `json:"command"`
		IsMutation                    bool                                   `json:"isMutation"`
		Applied                       bool                                   `json:"applied"`
		Lane                          startLane                              `json:"lane"`
		AuthorizedGateAdapterHandoffs []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
	}
	if err := json.Unmarshal(out.Bytes(), &reconcilePreview); err != nil {
		t.Fatalf("reconcile preview stdout is not JSON: %v\n%s", err, out.String())
	}
	if reconcilePreview.Command != "reconcile" || reconcilePreview.IsMutation || reconcilePreview.Applied || reconcilePreview.Lane.ID != "main" {
		t.Fatalf("unexpected authorized gate reconcile preview: %+v", reconcilePreview)
	}
	assertAuthorizedGateAdapterHandoffSnapshot(t, "reconcile preview", reconcilePreview.AuthorizedGateAdapterHandoffs, authorizedEventID, wantContractCommand)
	out.Reset()
	if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "_template", "-WhatIf", "main", "-InterventionId", "int-authorized-main", "-Executor", "session-reconcile", "-Actor", "mission-commander", "-Reason", "resolve authorized adapter handoff intervention", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"reconcile authorized gate adapter handoff：eventId=" + authorizedEventID, "reconcile authorized gate adapter report summary：eventId=" + authorizedEventID, "reconcile authorized gate adapter live validation：eventId=" + authorizedEventID, "reconcile authorized gate adapter boundary：eventId=" + authorizedEventID + " boundary=projection may read and validate an existing canonical sidecar"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reconcile preview text missing %q:\n%s", expected, out.String())
		}
	}
	out.Reset()
	if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "_template", "-Apply", "main", "-InterventionId", "int-authorized-main", "-Executor", "session-reconcile", "-Actor", "mission-commander", "-Reason", "resolve authorized adapter handoff intervention"}, &out); err != nil {
		t.Fatal(err)
	}
	var reconcileApply struct {
		Command                       string                                 `json:"command"`
		IsMutation                    bool                                   `json:"isMutation"`
		Applied                       bool                                   `json:"applied"`
		Lane                          startLane                              `json:"lane"`
		AuthorizedGateAdapterHandoffs []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
	}
	if err := json.Unmarshal(out.Bytes(), &reconcileApply); err != nil {
		t.Fatalf("reconcile apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if reconcileApply.Command != "reconcile" || !reconcileApply.IsMutation || !reconcileApply.Applied || reconcileApply.Lane.ID != "main" {
		t.Fatalf("unexpected authorized gate reconcile apply: %+v", reconcileApply)
	}
	assertAuthorizedGateAdapterHandoffSnapshot(t, "reconcile apply", reconcileApply.AuthorizedGateAdapterHandoffs, authorizedEventID, wantContractCommand)
	interventionsPath := filepath.Join(caseRoot, ".rekit", "facts", "interventions.jsonl")
	interventionsFile, err := os.OpenFile(interventionsPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := interventionsFile.WriteString(`{"eventId":"int-authorized-main-text","kind":"intervention","lane":"main","subject":"authorized handoff reconcile text","summary":"refresh executor text path","action":"override","status":"open","target":"workspace/main/main"}` + "\n"); err != nil {
		_ = interventionsFile.Close()
		t.Fatal(err)
	}
	if err := interventionsFile.Close(); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "reconcile", "-Target", caseRoot, "-Pack", "_template", "-Apply", "main", "-InterventionId", "int-authorized-main-text", "-Executor", "session-reconcile-text", "-Actor", "mission-commander", "-Reason", "resolve authorized adapter handoff intervention text", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"reconcile authorized gate adapter handoff：eventId=" + authorizedEventID, "reconcile authorized gate adapter report summary：eventId=" + authorizedEventID, "reconcile authorized gate adapter live validation：eventId=" + authorizedEventID, "reconcile authorized gate adapter boundary：eventId=" + authorizedEventID + " boundary=projection may read and validate an existing canonical sidecar"} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("reconcile apply text missing %q:\n%s", expected, out.String())
		}
	}

	writeCaseFile(t, caseRoot, ".rekit/facts/candidates.jsonl", `{"eventId":"evt-open-candidate","kind":"candidate","lane":"main","subject":"review candidate","summary":"needs authority review","status":"open","evidenceRefs":["workspace/main/main/packet.md"]}`+"\n")
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/outbox.jsonl", `{"eventId":"evt-authorized-continue","kind":"observation","subject":"post auth observation","summary":"continue after authorized gate","evidence":"evidence-authorized-gate"}`+"\n")
	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "_template", "-Apply", "main"}, &out); err != nil {
		t.Fatal(err)
	}
	var cont struct {
		RunID          string       `json:"runId"`
		MissionBrief   missionBrief `json:"missionBrief"`
		ExecutorAction struct {
			Blocked              bool     `json:"blocked"`
			Ready                bool     `json:"ready"`
			BlockerReasons       []string `json:"blockerReasons"`
			PendingGates         int      `json:"pendingGates"`
			OpenInterventions    int      `json:"openInterventions"`
			OpenDecisions        int      `json:"openDecisions"`
			OpenDecisionRequired bool     `json:"openDecisionRequired"`
			ResumeCommand        string   `json:"resumeCommand"`
		} `json:"executorAction"`
		AuthorizedGateAdapterHandoffs []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
		ExecutionEvidenceReview       []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		MissionCommanderNextActions   []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
		Writes                        []startWrite                           `json:"writes"`
		NextSteps                     []string                               `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &cont); err != nil {
		t.Fatalf("continue apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !containsSubstring(cont.MissionBrief.AuthorizedGates, "authorized debug") || !containsSubstring(cont.MissionBrief.AuthorizedGates, "outputPaths=workspace/main/debug/session-1") || !containsSubstring(cont.MissionBrief.AuthorizedGates, "stopConditions=timeout") || !containsSubstring(cont.MissionBrief.AuthorizedGates, "eventId="+authorizedEventID) || !containsSubstring(cont.MissionBrief.AuthorizedGates, "reportContract="+reportContract) || len(cont.MissionBrief.PendingGates) != 0 {
		t.Fatalf("continue JSON missing authorized gate visibility: %+v", cont.MissionBrief)
	}
	assertAuthorizedGateAdapterHandoffSnapshot(t, "continue", cont.AuthorizedGateAdapterHandoffs, authorizedEventID, wantContractCommand)
	if !cont.ExecutorAction.Blocked || cont.ExecutorAction.Ready || !cont.ExecutorAction.OpenDecisionRequired || cont.ExecutorAction.PendingGates != 0 || cont.ExecutorAction.OpenInterventions != 0 || cont.ExecutorAction.OpenDecisions != 1 || !slices.Contains(cont.ExecutorAction.BlockerReasons, "open-decision") || cont.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("continue JSON missing executor action snapshot: %+v", cont.ExecutorAction)
	}
	if len(cont.ExecutionEvidenceReview) != 2 || cont.ExecutionEvidenceReview[0].GateEventID != authorizedEventID || cont.ExecutionEvidenceReview[1].Status != "escalated" {
		t.Fatalf("continue JSON missing execution evidence review queue: %+v", cont.ExecutionEvidenceReview)
	}
	assertEvidenceReviewBeforeBlockedAdapterActions(t, "continue", cont.MissionCommanderNextActions, authorizedEventID)
	if slices.Contains(cont.NextSteps, "/rekit continue main") || !containsSubstring(cont.NextSteps, "review open candidate/decision") {
		t.Fatalf("authorized-gate continue should stay blocked only by open decision: %+v", cont.NextSteps)
	}
	statusPath := assertStartWrite(t, cont.Writes, ".rekit/runs/"+cont.RunID+"/status.json", "write").TargetPath
	digestPath := assertStartWrite(t, cont.Writes, ".rekit/runs/"+cont.RunID+"/digest.md", "write").TargetPath
	statusBytes, err := os.ReadFile(statusPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(statusBytes), `"authorizedGates"`) || !strings.Contains(string(statusBytes), `"authorizedGateAdapterHandoffs"`) || !strings.Contains(string(statusBytes), `"defaultReportPath": "workspace/main/debug/session-1/adapter-report.json"`) || !strings.Contains(string(statusBytes), `"caseRelativeReportPath": "workspace/main/debug/session-1/adapter-report.json"`) || !strings.Contains(string(statusBytes), `"executorAction"`) || !strings.Contains(string(statusBytes), `"executionEvidenceReview"`) || !strings.Contains(string(statusBytes), `"missionCommanderNextActions"`) || !strings.Contains(string(statusBytes), `"openDecisions": 1`) || !strings.Contains(string(statusBytes), "authorized debug") || !strings.Contains(string(statusBytes), "outputPaths=workspace/main/debug/session-1") || !strings.Contains(string(statusBytes), "stopConditions=timeout") || !strings.Contains(string(statusBytes), "eventId="+authorizedEventID) || !strings.Contains(string(statusBytes), "reportContract="+reportContract) || strings.Contains(string(statusBytes), "pending-gate requires main-agent/user decision") {
		t.Fatalf("continue status missing non-blocking authorized gate visibility or executor action:\n%s", string(statusBytes))
	}
	digest, err := os.ReadFile(digestPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"- authorized gates:", "authorized debug", "requestedBudget=runtimeSeconds=30,diskMB=64,requests=1", "outputPaths=workspace/main/debug/session-1", "stopConditions=timeout", "eventId=" + authorizedEventID, "reportContract=" + reportContract, "auth=preauthorized", "## Authorized gate adapter handoff", "defaultReportPath=workspace/main/debug/session-1/adapter-report.json", "caseRelativeReportPath=workspace/main/debug/session-1/adapter-report.json", "validate: `rekit -Command gate -Pack _template", "case record: `rekit -Command gate -Pack _template -Apply -GateEventId " + authorizedEventID, "boundary: projection may read and validate an existing canonical sidecar", "## Executor action snapshot", "- open decisions: `1`", "- open decision required: `true`", "- resume command: `/rekit continue main`", "## Mission Commander next actions", "state=ready-for-evidence-review source=executionEvidenceReview blocked=true requiresReview=true command=`/rekit handoff main`", "state=ready-for-evidence-review source=executionEvidenceReview.followUp blocked=true requiresReview=true command=`/rekit overview`", "review execution evidence for gateEventId " + authorizedEventID, "boundary hit or escalation in execution evidence", "follow-through: state=ready-for-evidence-review", "outcome: name=recorded-evidence-review", "when: bounded observation evidence was recorded for an authorized gate", "evidence: workspace/main/debug/session-1/result.json", "follow-through: state=needs-main-escalation", "outcome: name=boundary-or-escalation-review", "when: recorded evidence reports boundaryHits, escalation, boundary-hit, or escalated status", "evidence: workspace/main/debug/session-1/adapter-result.json", "- blocker reasons:", "open-decision"} {
		if !strings.Contains(string(digest), expected) {
			t.Fatalf("continue digest missing %q:\n%s", expected, string(digest))
		}
	}
	resumePath := assertStartWrite(t, cont.Writes, ".rekit/lanes/main/prompts/RESUME.md", "refresh").TargetPath
	checkpointPath := assertStartWrite(t, cont.Writes, ".rekit/lanes/main/checkpoints/latest.json", "refresh").TargetPath
	resume, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"## Mission Control brief", "openLanes=1 ready=0 blocked=1", "- blocked lanes:", "main (open-decision)", "- open decisions:", "review candidate", "- next agent actions:", "review open candidate/decision item(s) with evidence and authority boundary", "## Executor action snapshot", "- blocked: `true`", "- ready: `false`", "- open decision required: `true`", "- resume command: `/rekit continue main`", "## Mission Commander next actions", "state=ready-for-evidence-review source=executionEvidenceReview blocked=true requiresReview=true command=`/rekit handoff main`", "state=ready-for-evidence-review source=executionEvidenceReview.followUp blocked=true requiresReview=true command=`/rekit overview`", "review execution evidence for gateEventId " + authorizedEventID, "boundary hit or escalation in execution evidence", "- blocker reasons:", "open-decision", "## Heavy-action gate decisions", "- authorized-gate:", "authorized debug", "requestedBudget=runtimeSeconds=30,diskMB=64,requests=1", "outputPaths=workspace/main/debug/session-1", "stopConditions=timeout", "eventId=" + authorizedEventID, "reportContract=" + reportContract, "auth=preauthorized", "profile=prof-main-debug", "- pending-gate: none", "- execution evidence review:", "status=succeeded | gateEventId=" + authorizedEventID, "status=escalated | gateEventId=" + authorizedEventID, "outputRefs: workspace/main/debug/session-1/result.json", "evidenceRefs: workspace/main/debug/session-1/result.json", "review command: `review outputRefs/evidenceRefs for gateEventId " + authorizedEventID + "`", "handoff command: `/rekit handoff main`", "commander state: ready-for-evidence-review", "commander state: needs-main-escalation", "commander primary: `/rekit handoff main`", "follow-through: state=ready-for-evidence-review", "outcome: name=recorded-evidence-review", "when: bounded observation evidence was recorded for an authorized gate", "evidence: workspace/main/debug/session-1/result.json", "follow-through: state=needs-main-escalation", "outcome: name=boundary-or-escalation-review", "when: recorded evidence reports boundaryHits, escalation, boundary-hit, or escalated status", "evidence: workspace/main/debug/session-1/adapter-result.json", "commander follow-up: /rekit overview", "commander follow-up: /rekit continue main -WhatIf", "boundary: observation evidence is already recorded; do not replay heavy tool", "boundary: boundary/escalation requires main review before autonomous continuation"} {
		if !strings.Contains(string(resume), expected) {
			t.Fatalf("lane resume missing %q:\n%s", expected, string(resume))
		}
	}
	checkpointBytes, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		MissionBrief struct {
			Summary         string   `json:"summary"`
			BlockedLanes    []string `json:"blockedLanes"`
			OpenDecisions   []string `json:"openDecisions"`
			PendingGates    []string `json:"pendingGates"`
			AuthorizedGates []string `json:"authorizedGates"`
		} `json:"missionBrief"`
		ExecutorAction struct {
			Blocked              bool     `json:"blocked"`
			Ready                bool     `json:"ready"`
			BlockerReasons       []string `json:"blockerReasons"`
			PendingGates         int      `json:"pendingGates"`
			OpenInterventions    int      `json:"openInterventions"`
			OpenDecisions        int      `json:"openDecisions"`
			OpenDecisionRequired bool     `json:"openDecisionRequired"`
			ResumeCommand        string   `json:"resumeCommand"`
		} `json:"executorAction"`
		PendingGates                  []string                               `json:"pendingGates"`
		AuthorizedGates               []string                               `json:"authorizedGates"`
		AuthorizedGateAdapterHandoffs []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
		ExecutionEvidenceReview       []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		MissionCommanderNextActions   []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("lane checkpoint did not decode: %v\n%s", err, string(checkpointBytes))
	}
	if checkpoint.MissionBrief.Summary != "openLanes=1 ready=0 blocked=1 pendingGates=0 authorizedGates=1 openDecisions=1 interventions=0" || !slices.Contains(checkpoint.MissionBrief.BlockedLanes, "main (open-decision)") || !containsSubstring(checkpoint.MissionBrief.OpenDecisions, "review candidate") || len(checkpoint.MissionBrief.PendingGates) != 0 || !containsSubstring(checkpoint.MissionBrief.AuthorizedGates, "eventId="+authorizedEventID) || !containsSubstring(checkpoint.MissionBrief.AuthorizedGates, "reportContract="+reportContract) {
		t.Fatalf("lane checkpoint missing Mission Control brief snapshot: %+v", checkpoint.MissionBrief)
	}
	if !checkpoint.ExecutorAction.Blocked || checkpoint.ExecutorAction.Ready || !checkpoint.ExecutorAction.OpenDecisionRequired || checkpoint.ExecutorAction.PendingGates != 0 || checkpoint.ExecutorAction.OpenInterventions != 0 || checkpoint.ExecutorAction.OpenDecisions != 1 || !slices.Contains(checkpoint.ExecutorAction.BlockerReasons, "open-decision") || checkpoint.ExecutorAction.ResumeCommand != "/rekit continue main" {
		t.Fatalf("lane checkpoint missing typed executor action snapshot: %+v", checkpoint.ExecutorAction)
	}
	if len(checkpoint.PendingGates) != 0 || !containsSubstring(checkpoint.AuthorizedGates, "authorized debug") || !containsSubstring(checkpoint.AuthorizedGates, "outputPaths=workspace/main/debug/session-1") || !containsSubstring(checkpoint.AuthorizedGates, "stopConditions=timeout") || !containsSubstring(checkpoint.AuthorizedGates, "eventId="+authorizedEventID) || !containsSubstring(checkpoint.AuthorizedGates, "reportContract="+reportContract) || !containsSubstring(checkpoint.AuthorizedGates, "auth=preauthorized") {
		t.Fatalf("lane checkpoint missing non-blocking authorized gate visibility: %+v", checkpoint)
	}
	assertAuthorizedGateAdapterHandoffSnapshot(t, "lane checkpoint", checkpoint.AuthorizedGateAdapterHandoffs, authorizedEventID, wantContractCommand)
	if len(checkpoint.ExecutionEvidenceReview) != 2 || checkpoint.ExecutionEvidenceReview[0].GateEventID != authorizedEventID || checkpoint.ExecutionEvidenceReview[0].Status != "succeeded" || !containsSubstring(checkpoint.ExecutionEvidenceReview[0].OutputRefs, "workspace/main/debug/session-1/result.json") || checkpoint.ExecutionEvidenceReview[0].HandoffCommand != "/rekit handoff main" || checkpoint.ExecutionEvidenceReview[0].MissionCommanderAction.State != "ready-for-evidence-review" || !containsSubstring(checkpoint.ExecutionEvidenceReview[0].MissionCommanderAction.FollowUpCommands, "/rekit continue main -WhatIf") || checkpoint.ExecutionEvidenceReview[1].Status != "escalated" || !containsSubstring(checkpoint.ExecutionEvidenceReview[1].Boundary, "requires main review") || checkpoint.ExecutionEvidenceReview[1].MissionCommanderAction.State != "needs-main-escalation" || containsSubstring(checkpoint.ExecutionEvidenceReview[1].MissionCommanderAction.FollowUpCommands, "/rekit continue main") {
		t.Fatalf("lane checkpoint missing execution evidence review queue: %+v", checkpoint.ExecutionEvidenceReview)
	}
	assertEvidenceReviewBeforeBlockedAdapterActions(t, "lane checkpoint", checkpoint.MissionCommanderNextActions, authorizedEventID)
}

func TestRunGateAdapterReportNoPackProductPathFromNestedOutputWorkspace(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeAuthorizedGateVisibilityFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "authorized debug",
		"-TargetRef", "target-alpha",
		"-BatchId", "batch-nested-output-workspace",
		"-Scope", "handler only",
		"-RuntimeSeconds", "30",
		"-DiskMB", "64",
		"-Requests", "1",
		"-OutputPaths", "workspace/main/debug/session-1",
		"-StopConditions", "timeout",
		"-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var applied struct {
		EventID string `json:"eventId"`
		Applied bool   `json:"applied"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("authorized gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !applied.Applied || applied.EventID == "" {
		t.Fatalf("unexpected authorized gate result: %+v", applied)
	}

	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "nested-cli-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 20, "diskMB": 32, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter report from nested output workspace"
}`)
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/result.json", `{"ok":true}`)
	workspace := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1")
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	}()

	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-ExecutionReportContract", "-GateEventId", applied.EventID, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Kind                   string   `json:"kind"`
		CaseRoot               string   `json:"caseRoot"`
		Pack                   string   `json:"pack"`
		GateEventID            string   `json:"gateEventId"`
		IsMutation             bool     `json:"isMutation"`
		AllowedOutputPaths     []string `json:"allowedOutputPaths"`
		DefaultReportPath      string   `json:"defaultReportPath"`
		RefPathRequires        []string `json:"refPathRequires"`
		BoundaryStatusRequires []string `json:"boundaryStatusRequires"`
		StatusSummaryRequires  []string `json:"statusSummaryRequires"`
		MissionCommanderAction struct {
			State            string   `json:"state"`
			Prompt           string   `json:"prompt"`
			PrimaryCommand   string   `json:"primaryCommand"`
			FollowUpCommands []string `json:"followUpCommands"`
			Boundary         []string `json:"boundary"`
		} `json:"missionCommanderAction"`
		MissionCommanderNextActions []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		NextSteps                   []string                         `json:"nextSteps"`
		LiveValidation              struct {
			AuthorizedWorkspaces        []string `json:"authorizedWorkspaces"`
			ReportFileName              string   `json:"reportFileName"`
			CaseRelativeReportPath      string   `json:"caseRelativeReportPath"`
			ValidateCommand             string   `json:"validateCommand"`
			RecordCommand               string   `json:"recordCommand"`
			ValidateArgs                []string `json:"validateArgs"`
			RecordArgs                  []string `json:"recordArgs"`
			CaseRelativeValidateCommand string   `json:"caseRelativeValidateCommand"`
			CaseRelativeRecordCommand   string   `json:"caseRelativeRecordCommand"`
			CaseRelativeValidateArgs    []string `json:"caseRelativeValidateArgs"`
			CaseRelativeRecordArgs      []string `json:"caseRelativeRecordArgs"`
			AdapterCandidates           []struct {
				ID                  string   `json:"id"`
				Status              string   `json:"status"`
				GateActions         []string `json:"gateActions"`
				ToolingCatalogPath  string   `json:"toolingCatalogPath"`
				RecordOnlyAfterGate bool     `json:"recordOnlyAfterGate"`
			} `json:"adapterCandidates"`
			ReplayBehavior  string `json:"replayBehavior"`
			SidecarTemplate struct {
				Action       string   `json:"action"`
				GateEventID  string   `json:"gateEventId"`
				EvidenceRefs []string `json:"evidenceRefs"`
			} `json:"sidecarTemplate"`
		} `json:"liveValidation"`
	}
	if err := json.Unmarshal(out.Bytes(), &contract); err != nil {
		t.Fatalf("nested workspace adapter report contract stdout is not JSON: %v\n%s", err, out.String())
	}
	if contract.Kind != "adapter-execution-report-contract" || contract.CaseRoot != caseRoot || contract.Pack != "_template" || contract.GateEventID != applied.EventID || contract.IsMutation || strings.Join(contract.AllowedOutputPaths, ",") != "workspace/main/debug/session-1" || contract.DefaultReportPath != "workspace/main/debug/session-1/adapter-report.json" {
		t.Fatalf("unexpected nested no-pack workspace adapter report contract: %+v", contract)
	}
	if !containsSubstring(contract.RefPathRequires, "evidenceRefs must stay under authorized outputPaths") || !containsSubstring(contract.BoundaryStatusRequires, "authorized stopConditions") || !containsSubstring(contract.StatusSummaryRequires, "failed/boundary-hit/escalated/aborted") {
		t.Fatalf("nested workspace adapter report contract omitted live enforcement rules: %+v", contract)
	}
	wantCaseRelativeValidate := "/rekit gate -Pack _template -GateEventId " + applied.EventID + " -ValidateExecutionReport -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Format json"
	wantCaseRelativeRecord := "/rekit gate -Pack _template -Apply -GateEventId " + applied.EventID + " -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Actor <executor-id> -Format json"
	if contract.MissionCommanderAction.State != "needs-adapter-report-validation" || contract.MissionCommanderAction.PrimaryCommand != wantCaseRelativeValidate || !containsSubstring(contract.MissionCommanderAction.FollowUpCommands, wantCaseRelativeRecord) || !containsSubstring(contract.MissionCommanderAction.Boundary, "read-only") || !containsSubstring(contract.MissionCommanderAction.Boundary, "never executes the heavy tool") || !containsSubstring(contract.NextSteps, wantCaseRelativeValidate) || !containsSubstring(contract.NextSteps, wantCaseRelativeRecord) {
		t.Fatalf("nested workspace adapter report contract omitted Mission Commander handoff: action=%+v next=%+v", contract.MissionCommanderAction, contract.NextSteps)
	}
	if len(contract.MissionCommanderNextActions) != 3 || contract.MissionCommanderNextActions[0].Command != wantCaseRelativeValidate || contract.MissionCommanderNextActions[1].Command != wantCaseRelativeRecord || !contract.MissionCommanderNextActions[1].Blocked || contract.MissionCommanderNextActions[2].Command != "/rekit handoff main" || !cliNextActionBoundaryContains(contract.MissionCommanderNextActions, "do not record evidence until validation returns valid=true") || !cliNextActionBoundaryContains(contract.MissionCommanderNextActions, "replace <executor-id>") {
		t.Fatalf("nested workspace adapter report contract omitted Mission Commander next actions: %+v", contract.MissionCommanderNextActions)
	}
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-ExecutionReportContract", "-GateEventId", applied.EventID, "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"gate adapter report contract：gateEventId=" + applied.EventID + " action=debug lane=main reportPath=workspace/main/debug/session-1/adapter-report.json mutation=false",
		"gate adapter report validate command：rekit -Command gate -Pack _template -GateEventId " + applied.EventID + " -ValidateExecutionReport -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Format json",
		"gate adapter report record command：rekit -Command gate -Pack _template -Apply -GateEventId " + applied.EventID + " -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Actor <executor-id> -Format json",
		"gate adapter report live validation：cwd=authorized output workspace listed in authorizedWorkspaces; use reportFileName as workspace-relative -ExecutionReportPath and omit -Target; or use caseRelativeReportPath with case-relative commands from any case-local cwd reportFileName=adapter-report.json caseRelativeReportPath=workspace/main/debug/session-1/adapter-report.json",
		"gate adapter report sidecar template：kind=adapter-execution-report adapterId=<adapter-id> action=debug status=succeeded|failed|boundary-hit|escalated|aborted gateEventId=" + applied.EventID,
		"gate adapter report live validation note：Replace <executor-id> before running RecordArgs or CaseRelativeRecordArgs; both record observation evidence only after strict sidecar validation and never execute the heavy tool.",
		"adapter report commander action：state=needs-adapter-report-validation primary=`" + wantCaseRelativeValidate + "`",
		"mission commander next action：state=needs-adapter-report-validation source=adapterReportContract.missionCommanderAction blocked=false requiresReview=true command=`" + wantCaseRelativeValidate + "`",
		"mission commander next action：state=needs-adapter-report-validation source=adapterReportContract.missionCommanderAction.followUp blocked=true requiresReview=true command=`" + wantCaseRelativeRecord + "`",
		"mission commander next action boundary：do not record evidence until validation returns valid=true",
		"mission commander next action boundary：replace <executor-id> before running record command",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("adapter report contract text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "source=adapterReportValidation.missionCommanderAction") {
		t.Fatalf("contract text should not expose validation-stage record as current action:\n%s", out.String())
	}
	if strings.Join(contract.LiveValidation.AuthorizedWorkspaces, ",") != "workspace/main/debug/session-1" || contract.LiveValidation.ReportFileName != "adapter-report.json" || contract.LiveValidation.CaseRelativeReportPath != "workspace/main/debug/session-1/adapter-report.json" || strings.Join(contract.LiveValidation.ValidateArgs, " ") != "-Command gate -Pack _template -GateEventId "+applied.EventID+" -ValidateExecutionReport -ExecutionReportPath adapter-report.json -Format json" || strings.Join(contract.LiveValidation.RecordArgs, " ") != "-Command gate -Pack _template -Apply -GateEventId "+applied.EventID+" -ExecutionReportPath adapter-report.json -Actor <executor-id> -Format json" || contract.LiveValidation.ValidateCommand != "rekit "+strings.Join(contract.LiveValidation.ValidateArgs, " ") || contract.LiveValidation.RecordCommand != "rekit "+strings.Join(contract.LiveValidation.RecordArgs, " ") || strings.Join(contract.LiveValidation.CaseRelativeValidateArgs, " ") != "-Command gate -Pack _template -GateEventId "+applied.EventID+" -ValidateExecutionReport -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Format json" || strings.Join(contract.LiveValidation.CaseRelativeRecordArgs, " ") != "-Command gate -Pack _template -Apply -GateEventId "+applied.EventID+" -ExecutionReportPath workspace/main/debug/session-1/adapter-report.json -Actor <executor-id> -Format json" || contract.LiveValidation.CaseRelativeValidateCommand != "rekit "+strings.Join(contract.LiveValidation.CaseRelativeValidateArgs, " ") || contract.LiveValidation.CaseRelativeRecordCommand != "rekit "+strings.Join(contract.LiveValidation.CaseRelativeRecordArgs, " ") || contract.LiveValidation.SidecarTemplate.Action != "debug" || contract.LiveValidation.SidecarTemplate.GateEventID != applied.EventID || !containsSubstring(contract.LiveValidation.SidecarTemplate.EvidenceRefs, "authorized outputPaths") || !strings.Contains(contract.LiveValidation.ReplayBehavior, "CaseRelativeRecordArgs") || !strings.Contains(contract.LiveValidation.ReplayBehavior, "duplicate eventId") {
		t.Fatalf("nested workspace adapter report contract omitted live-validation handoff: %+v", contract.LiveValidation)
	}

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-ValidateExecutionReport", "-GateEventId", applied.EventID, "-ExecutionReportPath", "adapter-report.json", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var validation struct {
		Kind       string `json:"kind"`
		CaseRoot   string `json:"caseRoot"`
		Pack       string `json:"pack"`
		Valid      bool   `json:"valid"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		Error      string `json:"error"`
		ReportPath string `json:"reportPath"`
		Report     *struct {
			AdapterID string `json:"adapterId"`
		} `json:"report"`
	}
	if err := json.Unmarshal(out.Bytes(), &validation); err != nil {
		t.Fatalf("nested workspace adapter report validation stdout is not JSON: %v\n%s", err, out.String())
	}
	if validation.Kind != "adapter-execution-report-validation" || validation.CaseRoot != caseRoot || validation.Pack != "_template" || !validation.Valid || validation.IsMutation || validation.Applied || validation.Error != "" || validation.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || validation.Report == nil || validation.Report.AdapterID != "nested-cli-adapter" {
		t.Fatalf("unexpected nested no-pack workspace adapter report validation: %+v", validation)
	}
	var validationCommander struct {
		MissionCommanderAction struct {
			State            string   `json:"state"`
			PrimaryCommand   string   `json:"primaryCommand"`
			FollowUpCommands []string `json:"followUpCommands"`
			Boundary         []string `json:"boundary"`
		} `json:"missionCommanderAction"`
		MissionCommanderNextActions []missionCommanderNextActionItem `json:"missionCommanderNextActions"`
		NextSteps                   []string                         `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &validationCommander); err != nil {
		t.Fatalf("nested workspace validation commander envelope is not JSON: %v\n%s", err, out.String())
	}
	if validationCommander.MissionCommanderAction.State != "ready-to-record-evidence" || validationCommander.MissionCommanderAction.PrimaryCommand != wantCaseRelativeRecord || !containsSubstring(validationCommander.MissionCommanderAction.FollowUpCommands, "/rekit handoff main") || !containsSubstring(validationCommander.MissionCommanderAction.Boundary, "bounded observation evidence") || !containsSubstring(validationCommander.NextSteps, wantCaseRelativeRecord) {
		t.Fatalf("nested workspace adapter report validation omitted Mission Commander record handoff: action=%+v next=%+v", validationCommander.MissionCommanderAction, validationCommander.NextSteps)
	}
	if len(validationCommander.MissionCommanderNextActions) != 2 || validationCommander.MissionCommanderNextActions[0].Command != wantCaseRelativeRecord || validationCommander.MissionCommanderNextActions[1].Command != "/rekit handoff main" || !cliNextActionBoundaryContains(validationCommander.MissionCommanderNextActions, "replace <executor-id>") {
		t.Fatalf("nested workspace adapter report validation omitted Mission Commander next actions: %+v", validationCommander.MissionCommanderNextActions)
	}
	afterValidation := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, afterValidation)

	out.Reset()
	if err := Run(contract.LiveValidation.CaseRelativeValidateArgs, &out); err != nil {
		t.Fatal(err)
	}
	var caseRelativeValidation struct {
		Kind       string `json:"kind"`
		Valid      bool   `json:"valid"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		ReportPath string `json:"reportPath"`
	}
	if err := json.Unmarshal(out.Bytes(), &caseRelativeValidation); err != nil {
		t.Fatalf("case-relative adapter report validation stdout is not JSON: %v\n%s", err, out.String())
	}
	if caseRelativeValidation.Kind != "adapter-execution-report-validation" || !caseRelativeValidation.Valid || caseRelativeValidation.IsMutation || caseRelativeValidation.Applied || caseRelativeValidation.ReportPath != "workspace/main/debug/session-1/adapter-report.json" {
		t.Fatalf("unexpected case-relative adapter report validation: %+v", caseRelativeValidation)
	}
	afterCaseRelativeValidation := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, afterCaseRelativeValidation)

	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/bad-evidence-refs-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "nested-cli-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 20, "diskMB": 32, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "evidenceRefs": ["workspace/main/other/evidence.json"]
}`)
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-ValidateExecutionReport", "-GateEventId", applied.EventID, "-ExecutionReportPath", "bad-evidence-refs-report.json", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var invalidValidation struct {
		Valid        bool   `json:"valid"`
		IsMutation   bool   `json:"isMutation"`
		Applied      bool   `json:"applied"`
		FailureCode  string `json:"failureCode"`
		FailureStage string `json:"failureStage"`
		RepairHints  []struct {
			RepairAction       string   `json:"repairAction"`
			AllowedOutputPaths []string `json:"allowedOutputPaths"`
			RecordBlocked      bool     `json:"recordBlocked"`
			RerunValidation    bool     `json:"rerunValidation"`
		} `json:"repairHints"`
		Error      string `json:"error"`
		Pack       string `json:"pack"`
		ReportPath string `json:"reportPath"`
	}
	if err := json.Unmarshal(out.Bytes(), &invalidValidation); err != nil {
		t.Fatalf("nested workspace invalid evidenceRefs validation stdout is not JSON: %v\n%s", err, out.String())
	}
	if invalidValidation.Valid || invalidValidation.IsMutation || invalidValidation.Applied || invalidValidation.Pack != "_template" || invalidValidation.FailureCode != "evidence-refs-out-of-scope" || invalidValidation.FailureStage != "refs" || !strings.Contains(invalidValidation.Error, "evidenceRefs must stay within authorized gate outputPaths") || invalidValidation.ReportPath != "workspace/main/debug/session-1/bad-evidence-refs-report.json" {
		t.Fatalf("unexpected nested no-pack workspace invalid evidenceRefs validation: %+v", invalidValidation)
	}
	if len(invalidValidation.RepairHints) != 1 || invalidValidation.RepairHints[0].RepairAction != "move-evidence-refs-under-authorized-output-paths" || strings.Join(invalidValidation.RepairHints[0].AllowedOutputPaths, ",") != "workspace/main/debug/session-1" || !invalidValidation.RepairHints[0].RecordBlocked || !invalidValidation.RepairHints[0].RerunValidation {
		t.Fatalf("nested workspace invalid evidenceRefs validation omitted repair hints: %+v", invalidValidation)
	}
	afterInvalidValidation := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, afterInvalidValidation)

	caseLocalCwd := filepath.Join(caseRoot, "workspace", "main")
	if err := os.Chdir(caseLocalCwd); err != nil {
		t.Fatal(err)
	}
	recordArgs := []string{"-Command", "gate", "-Apply", "-GateEventId", applied.EventID, "-ExecutionReportPath", "debug/session-1/adapter-report.json", "-Actor", "executor-1", "-Format", "json"}
	out.Reset()
	if err := Run(recordArgs, &out); err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Applied           bool   `json:"applied"`
		EventID           string `json:"eventId"`
		Pack              string `json:"pack"`
		Path              string `json:"path"`
		ExecutionEvidence struct {
			Kind      string   `json:"kind"`
			Status    string   `json:"status"`
			Summary   string   `json:"summary"`
			Related   []string `json:"related"`
			Execution struct {
				GateEventID         string   `json:"gateEventId"`
				Authorization       string   `json:"authorization"`
				ExecutionReportPath string   `json:"executionReportPath"`
				OutputRefs          []string `json:"outputRefs"`
				ActualBudget        struct {
					RuntimeSeconds int `json:"runtimeSeconds"`
					DiskMB         int `json:"diskMB"`
					Requests       int `json:"requests"`
				} `json:"actualBudget"`
				Adapter struct {
					AdapterID string `json:"adapterId"`
					Status    string `json:"status"`
				} `json:"adapter"`
			} `json:"execution"`
		} `json:"executionEvidence"`
	}
	if err := json.Unmarshal(out.Bytes(), &evidence); err != nil {
		t.Fatalf("case-relative adapter execution evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if !evidence.Applied || evidence.EventID == "" || evidence.Pack != "_template" || evidence.Path != ".rekit/facts/observations.jsonl" || evidence.ExecutionEvidence.Kind != "observation" || evidence.ExecutionEvidence.Status != "succeeded" || evidence.ExecutionEvidence.Summary != "Adapter report from nested output workspace" {
		t.Fatalf("unexpected no-pack adapter execution evidence: %+v", evidence)
	}
	if strings.Join(evidence.ExecutionEvidence.Related, ",") != applied.EventID || evidence.ExecutionEvidence.Execution.GateEventID != applied.EventID || evidence.ExecutionEvidence.Execution.Authorization != "preauthorized" || evidence.ExecutionEvidence.Execution.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || strings.Join(evidence.ExecutionEvidence.Execution.OutputRefs, ",") != "workspace/main/debug/session-1/result.json" || evidence.ExecutionEvidence.Execution.ActualBudget.RuntimeSeconds != 20 || evidence.ExecutionEvidence.Execution.ActualBudget.DiskMB != 32 || evidence.ExecutionEvidence.Execution.ActualBudget.Requests != 1 || evidence.ExecutionEvidence.Execution.Adapter.AdapterID != "nested-cli-adapter" || evidence.ExecutionEvidence.Execution.Adapter.Status != "succeeded" {
		t.Fatalf("case-relative adapter evidence did not preserve report provenance: %+v", evidence.ExecutionEvidence)
	}
	observations, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(observations), `"executionReportPath":"workspace/main/debug/session-1/adapter-report.json"`) || !strings.Contains(string(observations), `"adapterId":"nested-cli-adapter"`) || strings.Contains(string(observations), `"kind":"authority"`) || strings.Contains(string(observations), `"kind":"confirmed"`) {
		t.Fatalf("case-relative execution evidence ledger mismatch:\n%s", string(observations))
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("case-relative adapter evidence wrote authority ledger or stat failed: %v", err)
	}
	out.Reset()
	if err := Run(recordArgs, &out); err != nil {
		t.Fatal(err)
	}
	var replay struct {
		Applied bool   `json:"applied"`
		EventID string `json:"eventId"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(out.Bytes(), &replay); err != nil {
		t.Fatalf("case-relative adapter replay stdout is not JSON: %v\n%s", err, out.String())
	}
	if replay.Applied || replay.EventID != evidence.EventID || replay.Reason != "duplicate eventId" {
		t.Fatalf("case-relative adapter replay should be idempotent: first=%+v replay=%+v", evidence, replay)
	}
	replayedObservations, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(replayedObservations) != string(observations) {
		t.Fatalf("nested workspace adapter replay appended observations:\nbefore=%s\nafter=%s", string(observations), string(replayedObservations))
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("nested workspace adapter replay wrote authority ledger or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("nested workspace adapter evidence wrote confirmed ledger or stat failed: %v", err)
	}

	beforeStatus := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Command        string `json:"command"`
		Mode           string `json:"mode"`
		Pack           string `json:"pack"`
		PackSource     string `json:"packSource"`
		Target         string `json:"target"`
		TargetProvided bool   `json:"targetProvided"`
		CaseMission    struct {
			ExecutionEvidenceReviewCount   int                                    `json:"executionEvidenceReviewCount"`
			ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
			ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
			MissionCommanderActionQueue    missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("no-pack status after adapter evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if status.Command != "status" || status.Mode != "case" || status.Pack != "_template" || status.PackSource != "case-metadata" || status.Target != caseRoot || status.TargetProvided || status.CaseMission.ExecutionEvidenceReviewCount != 1 || len(status.CaseMission.ExecutionEvidenceReview) != 1 {
		t.Fatalf("unexpected no-pack status after adapter evidence: %+v", status)
	}
	statusEvidence := status.CaseMission.ExecutionEvidenceReview[0]
	if statusEvidence.EventID != evidence.EventID || statusEvidence.GateEventID != applied.EventID || statusEvidence.Status != "succeeded" || statusEvidence.Action != "debug" || !containsSubstring(statusEvidence.OutputRefs, "workspace/main/debug/session-1/result.json") || statusEvidence.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || statusEvidence.ActualBudget == nil || statusEvidence.ActualBudget.RuntimeSeconds != 20 || statusEvidence.ActualBudget.DiskMB != 32 || statusEvidence.ActualBudget.Requests != 1 || statusEvidence.AdapterID != "nested-cli-adapter" || statusEvidence.AdapterStatus != "succeeded" || statusEvidence.MissionCommanderAction.State != "ready-for-evidence-review" || statusEvidence.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || statusEvidence.FollowThrough.State != "ready-for-evidence-review" || !cliExecutionEvidenceFollowThroughContains(statusEvidence.FollowThrough, "recorded-evidence-review", "reviewed outputRefs/evidenceRefs") {
		t.Fatalf("no-pack status omitted recorded adapter evidence review handoff: %+v", statusEvidence)
	}
	assertNestedAdapterExecutionEvidenceReviewSummary(t, "no-pack status JSON", status.CaseMission.ExecutionEvidenceReviewSummary, evidence.EventID, applied.EventID)
	queue := status.CaseMission.MissionCommanderActionQueue
	if queue.CurrentAction == nil || queue.CurrentAction.Source != "executionEvidenceReview" || queue.CurrentAction.Command != "/rekit handoff main" || !queue.CurrentAction.RequiresReview || queue.Counts.Total == 0 || queue.Counts.RequiresReview == 0 {
		t.Fatalf("no-pack status omitted evidence review action queue: %+v", queue)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status：mutation=false mode=case targetProvided=false pack=_template packSource=case-metadata",
		"status case mission evidence review summary：total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=0 boundaryHits=0 hasEscalation=false hasExecutionReport=true hasAdapter=true latestEventId=" + evidence.EventID,
		"latestReport=workspace/main/debug/session-1/adapter-report.json latestAdapterId=nested-cli-adapter latestAdapterStatus=succeeded outcomes=1 followThrough=ready-for-evidence-review",
		"status case mission evidence review summary boundary：no authority/confirmed writes",
		"status case mission evidence review：eventId=" + evidence.EventID + " gateEventId=" + applied.EventID + " status=succeeded action=debug",
		"review=review outputRefs/evidenceRefs for gateEventId " + applied.EventID + " handoff=/rekit handoff main commanderState=ready-for-evidence-review commanderPrimary=/rekit handoff main",
		"status case mission evidence report：eventId=" + evidence.EventID + " path=workspace/main/debug/session-1/adapter-report.json",
		"status case mission evidence budget：eventId=" + evidence.EventID + " runtimeSeconds=20 diskMB=32 requests=1",
		"status case mission evidence adapter：eventId=" + evidence.EventID + " adapterId=nested-cli-adapter status=succeeded",
		"status case mission evidence output ref：eventId=" + evidence.EventID + " ref=workspace/main/debug/session-1/result.json",
		"status case mission evidence follow-through：eventId=" + evidence.EventID + " state=ready-for-evidence-review gateEventId=" + applied.EventID,
		"status case mission evidence outcome evidence：eventId=" + evidence.EventID + " name=recorded-evidence-review evidence=workspace/main/debug/session-1/result.json",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("no-pack status text after adapter evidence missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("no-pack status text after adapter evidence should not emit JSON:\n%s", out.String())
	}

	out.Reset()
	if err := Run(nil, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "pack source: case-metadata") || !strings.Contains(out.String(), "status case mission evidence review：eventId="+evidence.EventID+" gateEventId="+applied.EventID) {
		t.Fatalf("no-command no-pack status after adapter evidence missing case metadata evidence review handoff:\n%s", out.String())
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("no-command no-pack status after adapter evidence should not emit JSON:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"overview execution evidence review summary：total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=0 boundaryHits=0 hasEscalation=false hasExecutionReport=true hasAdapter=true latestEventId=" + evidence.EventID,
		"latestReport=workspace/main/debug/session-1/adapter-report.json latestAdapterId=nested-cli-adapter latestAdapterStatus=succeeded outcomes=1 followThrough=ready-for-evidence-review",
		"overview execution evidence review summary boundary：no authority/confirmed writes",
		"overview execution evidence report：eventId=" + evidence.EventID + " path=workspace/main/debug/session-1/adapter-report.json",
		"overview execution evidence budget：eventId=" + evidence.EventID + " runtimeSeconds=20 diskMB=32 requests=1",
		"overview execution evidence adapter：eventId=" + evidence.EventID + " adapterId=nested-cli-adapter status=succeeded",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("no-pack overview text after adapter evidence missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("no-pack overview text after adapter evidence should not emit JSON:\n%s", out.String())
	}
	assertSnapshotEqual(t, beforeStatus, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	beforeHandoff := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "handoff", "main", "-Format", "json", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	var handoffPreview handoffResult
	if err := json.Unmarshal(out.Bytes(), &handoffPreview); err != nil {
		t.Fatalf("no-pack handoff preview after adapter evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if handoffPreview.Command != "handoff" || handoffPreview.Pack != "_template" || handoffPreview.IsMutation || handoffPreview.Applied || !handoffPreview.RequiresConfirmation || handoffPreview.Project || handoffPreview.Lane == nil || handoffPreview.Lane.ID != "main" || len(handoffPreview.ExecutionEvidenceReview) != 1 {
		t.Fatalf("unexpected no-pack handoff preview after adapter evidence: %+v", handoffPreview)
	}
	handoffEvidence := handoffPreview.ExecutionEvidenceReview[0]
	if handoffEvidence.EventID != evidence.EventID || handoffEvidence.GateEventID != applied.EventID || handoffEvidence.Status != "succeeded" || handoffEvidence.Action != "debug" || !containsSubstring(handoffEvidence.OutputRefs, "workspace/main/debug/session-1/result.json") || handoffEvidence.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || handoffEvidence.ActualBudget == nil || handoffEvidence.ActualBudget.RuntimeSeconds != 20 || handoffEvidence.AdapterID != "nested-cli-adapter" || handoffEvidence.AdapterStatus != "succeeded" || handoffEvidence.MissionCommanderAction.State != "ready-for-evidence-review" || handoffEvidence.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || handoffEvidence.FollowThrough.State != "ready-for-evidence-review" || !cliExecutionEvidenceFollowThroughContains(handoffEvidence.FollowThrough, "recorded-evidence-review", "reviewed outputRefs/evidenceRefs") {
		t.Fatalf("no-pack handoff preview omitted recorded adapter evidence review handoff: %+v", handoffEvidence)
	}
	assertNestedAdapterExecutionEvidenceReviewSummary(t, "no-pack handoff preview JSON", handoffPreview.ExecutionEvidenceReviewSummary, evidence.EventID, applied.EventID)
	handoffQueue := handoffPreview.MissionCommanderActionQueue
	if handoffQueue.CurrentAction == nil || handoffQueue.CurrentAction.Source != "executionEvidenceReview" || handoffQueue.CurrentAction.Command != "/rekit handoff main" || !handoffQueue.CurrentAction.RequiresReview || handoffQueue.Counts.Total == 0 || handoffQueue.Counts.RequiresReview == 0 {
		t.Fatalf("no-pack handoff preview omitted evidence review action queue: %+v", handoffQueue)
	}
	if !containsSubstring(handoffPreview.NextSteps, "review execution evidence for gateEventId "+applied.EventID) || !containsSubstring(handoffPreview.NextSteps, "/rekit handoff main") || !containsSubstring(handoffPreview.NextSteps, "/rekit continue main -WhatIf") {
		t.Fatalf("no-pack handoff preview omitted evidence review next steps: %+v", handoffPreview.NextSteps)
	}
	assertSnapshotEqual(t, beforeHandoff, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "main", "-Format", "text", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"would write workstream handoff: main",
		"handoff execution evidence review summary：total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=0 boundaryHits=0 hasEscalation=false hasExecutionReport=true hasAdapter=true latestEventId=" + evidence.EventID,
		"latestReport=workspace/main/debug/session-1/adapter-report.json latestAdapterId=nested-cli-adapter latestAdapterStatus=succeeded outcomes=1 followThrough=ready-for-evidence-review",
		"handoff execution evidence review summary boundary：no authority/confirmed writes",
		"handoff execution evidence review：eventId=" + evidence.EventID + " gateEventId=" + applied.EventID + " status=succeeded action=debug",
		"review=review outputRefs/evidenceRefs for gateEventId " + applied.EventID + " handoff=/rekit handoff main commanderState=ready-for-evidence-review commanderPrimary=/rekit handoff main",
		"handoff execution evidence report：eventId=" + evidence.EventID + " path=workspace/main/debug/session-1/adapter-report.json",
		"handoff execution evidence budget：eventId=" + evidence.EventID + " runtimeSeconds=20 diskMB=32 requests=1",
		"handoff execution evidence adapter：eventId=" + evidence.EventID + " adapterId=nested-cli-adapter status=succeeded",
		"handoff execution evidence output ref：eventId=" + evidence.EventID + " ref=workspace/main/debug/session-1/result.json",
		"handoff execution evidence follow-through：eventId=" + evidence.EventID + " state=ready-for-evidence-review gateEventId=" + applied.EventID,
		"handoff execution evidence outcome evidence：eventId=" + evidence.EventID + " name=recorded-evidence-review evidence=workspace/main/debug/session-1/result.json",
		"mission commander action queue current：state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=`/rekit handoff main`",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("no-pack handoff text after adapter evidence missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("no-pack handoff text after adapter evidence should not emit JSON:\n%s", out.String())
	}
	assertSnapshotEqual(t, beforeHandoff, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	beforeHandoffApplyFacts := snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts"))
	out.Reset()
	if err := Run([]string{"-Command", "handoff", "main", "-Format", "json", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var handoffApply handoffResult
	if err := json.Unmarshal(out.Bytes(), &handoffApply); err != nil {
		t.Fatalf("no-pack handoff apply after adapter evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if handoffApply.Command != "handoff" || handoffApply.Pack != "_template" || !handoffApply.IsMutation || !handoffApply.Applied || handoffApply.RequiresConfirmation || handoffApply.Project || handoffApply.Lane == nil || handoffApply.Lane.ID != "main" || len(handoffApply.ExecutionEvidenceReview) != 1 {
		t.Fatalf("unexpected no-pack handoff apply after adapter evidence: %+v", handoffApply)
	}
	assertWriteKind(t, handoffApply.Writes, "lane-resume", "refresh")
	assertWriteKind(t, handoffApply.Writes, "lane-checkpoint", "refresh")
	assertWriteKind(t, handoffApply.Writes, "handoff", "write-lane-handoff")
	assertWriteKind(t, handoffApply.Writes, "handoff", "write-latest-lane-handoff")
	handoffApplyEvidence := handoffApply.ExecutionEvidenceReview[0]
	if handoffApplyEvidence.EventID != evidence.EventID || handoffApplyEvidence.GateEventID != applied.EventID || handoffApplyEvidence.Status != "succeeded" || handoffApplyEvidence.Action != "debug" || !containsSubstring(handoffApplyEvidence.OutputRefs, "workspace/main/debug/session-1/result.json") || handoffApplyEvidence.MissionCommanderAction.State != "ready-for-evidence-review" || handoffApplyEvidence.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || handoffApplyEvidence.FollowThrough.State != "ready-for-evidence-review" || !cliExecutionEvidenceFollowThroughContains(handoffApplyEvidence.FollowThrough, "recorded-evidence-review", "reviewed outputRefs/evidenceRefs") {
		t.Fatalf("no-pack handoff apply omitted recorded adapter evidence review handoff: %+v", handoffApplyEvidence)
	}
	assertNestedAdapterExecutionEvidenceReviewSummary(t, "no-pack handoff apply JSON", handoffApply.ExecutionEvidenceReviewSummary, evidence.EventID, applied.EventID)
	handoffApplyQueue := handoffApply.MissionCommanderActionQueue
	if handoffApplyQueue.CurrentAction == nil || handoffApplyQueue.CurrentAction.Source != "executionEvidenceReview" || handoffApplyQueue.CurrentAction.Command != "/rekit handoff main" || !handoffApplyQueue.CurrentAction.RequiresReview || handoffApplyQueue.Counts.Total == 0 || handoffApplyQueue.Counts.RequiresReview == 0 {
		t.Fatalf("no-pack handoff apply omitted evidence review action queue: %+v", handoffApplyQueue)
	}
	latestHandoff, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "handovers", "main-latest.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"## execution evidence review",
		"summary: total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=0 boundaryHits=0 latestEventId=" + evidence.EventID,
		"summary report: path=workspace/main/debug/session-1/adapter-report.json adapterId=nested-cli-adapter adapterStatus=succeeded",
		"summary follow-through: state=ready-for-evidence-review outcomes=1",
		"summary boundary: no authority/confirmed writes",
		"gateEventId=" + applied.EventID,
		"action=debug",
		"outputRefs=workspace/main/debug/session-1/result.json",
		"execution report: workspace/main/debug/session-1/adapter-report.json",
		"actual budget: runtimeSeconds=20 diskMB=32 requests=1",
		"adapter report: adapterId=nested-cli-adapter status=succeeded",
		"review command: `review outputRefs/evidenceRefs for gateEventId " + applied.EventID + "`",
		"handoff command: `/rekit handoff main`",
		"commander state: ready-for-evidence-review",
		"commander primary: `/rekit handoff main`",
		"evidence: workspace/main/debug/session-1/result.json",
	} {
		if !strings.Contains(string(latestHandoff), expected) {
			t.Fatalf("written no-pack handoff after adapter evidence missing %q:\n%s", expected, string(latestHandoff))
		}
	}
	resume, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"- execution evidence review:",
		"gateEventId=" + applied.EventID,
		"outputRefs: workspace/main/debug/session-1/result.json",
		"review command: `review outputRefs/evidenceRefs for gateEventId " + applied.EventID + "`",
		"handoff command: `/rekit handoff main`",
		"commander primary: `/rekit handoff main`",
		"evidence: workspace/main/debug/session-1/result.json",
	} {
		if !strings.Contains(string(resume), expected) {
			t.Fatalf("written no-pack RESUME after adapter evidence missing %q:\n%s", expected, string(resume))
		}
	}
	checkpointBytes, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "checkpoints", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
		MissionCommanderNextActions    []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
		Resume                         string                                 `json:"resume"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("written no-pack checkpoint after adapter evidence is not JSON: %v\n%s", err, string(checkpointBytes))
	}
	if checkpoint.Resume != ".rekit/lanes/main/prompts/RESUME.md" || len(checkpoint.ExecutionEvidenceReview) != 1 || checkpoint.ExecutionEvidenceReview[0].EventID != evidence.EventID || checkpoint.ExecutionEvidenceReview[0].GateEventID != applied.EventID || checkpoint.ExecutionEvidenceReview[0].MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || len(checkpoint.MissionCommanderNextActions) == 0 || checkpoint.MissionCommanderActionQueue.CurrentAction == nil || checkpoint.MissionCommanderActionQueue.CurrentAction.Command != "/rekit handoff main" {
		t.Fatalf("written no-pack checkpoint omitted evidence review handoff: %+v", checkpoint)
	}
	assertNestedAdapterExecutionEvidenceReviewSummary(t, "no-pack checkpoint JSON", checkpoint.ExecutionEvidenceReviewSummary, evidence.EventID, applied.EventID)
	assertSnapshotEqual(t, beforeHandoffApplyFacts, snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts")))
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("no-pack handoff apply wrote authority ledger or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("no-pack handoff apply wrote confirmed ledger or stat failed: %v", err)
	}

	beforeContinue := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "continue", "main", "-Format", "json", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	var continuePreview struct {
		Command                        string                                 `json:"command"`
		Pack                           string                                 `json:"pack"`
		IsMutation                     bool                                   `json:"isMutation"`
		Applied                        bool                                   `json:"applied"`
		RequiresConfirmation           bool                                   `json:"requiresConfirmation"`
		Selector                       string                                 `json:"selector"`
		Lane                           startLane                              `json:"lane"`
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
		MissionCommanderNextActions    []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &continuePreview); err != nil {
		t.Fatalf("no-pack continue preview after adapter evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if continuePreview.Command != "continue" || continuePreview.Pack != "_template" || continuePreview.IsMutation || continuePreview.Applied || !continuePreview.RequiresConfirmation || continuePreview.Selector != "main" || continuePreview.Lane.ID != "main" || len(continuePreview.ExecutionEvidenceReview) != 1 {
		t.Fatalf("unexpected no-pack continue preview after adapter evidence: %+v", continuePreview)
	}
	continueEvidence := continuePreview.ExecutionEvidenceReview[0]
	if continueEvidence.EventID != evidence.EventID || continueEvidence.GateEventID != applied.EventID || continueEvidence.Status != "succeeded" || continueEvidence.Action != "debug" || !containsSubstring(continueEvidence.OutputRefs, "workspace/main/debug/session-1/result.json") || continueEvidence.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || continueEvidence.ActualBudget == nil || continueEvidence.ActualBudget.RuntimeSeconds != 20 || continueEvidence.AdapterID != "nested-cli-adapter" || continueEvidence.AdapterStatus != "succeeded" || continueEvidence.MissionCommanderAction.State != "ready-for-evidence-review" || continueEvidence.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || continueEvidence.FollowThrough.State != "ready-for-evidence-review" || !cliExecutionEvidenceFollowThroughContains(continueEvidence.FollowThrough, "recorded-evidence-review", "reviewed outputRefs/evidenceRefs") {
		t.Fatalf("no-pack continue preview omitted recorded adapter evidence review handoff: %+v", continueEvidence)
	}
	assertNestedAdapterExecutionEvidenceReviewSummary(t, "no-pack continue preview JSON", continuePreview.ExecutionEvidenceReviewSummary, evidence.EventID, applied.EventID)
	continueQueue := continuePreview.MissionCommanderActionQueue
	if continueQueue.CurrentAction == nil || continueQueue.CurrentAction.Source != "executionEvidenceReview" || continueQueue.CurrentAction.Command != "/rekit handoff main" || !continueQueue.CurrentAction.RequiresReview || continueQueue.Counts.Total == 0 || continueQueue.Counts.RequiresReview == 0 {
		t.Fatalf("no-pack continue preview omitted evidence review action queue: %+v", continueQueue)
	}
	if !containsMissionCommanderNextActionsCommand(continuePreview.MissionCommanderNextActions, "/rekit handoff main") {
		t.Fatalf("no-pack continue preview omitted review Mission Commander next actions: %+v", continuePreview.MissionCommanderNextActions)
	}
	assertSnapshotEqual(t, beforeContinue, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "continue", "main", "-Format", "text", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"已选择工作线：main",
		"工作区：workspace/main",
		"continue execution evidence review summary：total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=0 boundaryHits=0 hasEscalation=false hasExecutionReport=true hasAdapter=true latestEventId=" + evidence.EventID,
		"latestReport=workspace/main/debug/session-1/adapter-report.json latestAdapterId=nested-cli-adapter latestAdapterStatus=succeeded outcomes=1 followThrough=ready-for-evidence-review",
		"continue execution evidence review summary boundary：no authority/confirmed writes",
		"continue execution evidence review：eventId=" + evidence.EventID + " gateEventId=" + applied.EventID + " status=succeeded action=debug",
		"review=review outputRefs/evidenceRefs for gateEventId " + applied.EventID + " handoff=/rekit handoff main commanderState=ready-for-evidence-review commanderPrimary=/rekit handoff main",
		"continue execution evidence report：eventId=" + evidence.EventID + " path=workspace/main/debug/session-1/adapter-report.json",
		"continue execution evidence budget：eventId=" + evidence.EventID + " runtimeSeconds=20 diskMB=32 requests=1",
		"continue execution evidence adapter：eventId=" + evidence.EventID + " adapterId=nested-cli-adapter status=succeeded",
		"continue execution evidence output ref：eventId=" + evidence.EventID + " ref=workspace/main/debug/session-1/result.json",
		"continue execution evidence follow-through：eventId=" + evidence.EventID + " state=ready-for-evidence-review gateEventId=" + applied.EventID,
		"continue execution evidence outcome evidence：eventId=" + evidence.EventID + " name=recorded-evidence-review evidence=workspace/main/debug/session-1/result.json",
		"mission commander action queue current：state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=`/rekit handoff main`",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("no-pack continue text after adapter evidence missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("no-pack continue text after adapter evidence should not emit JSON:\n%s", out.String())
	}
	assertSnapshotEqual(t, beforeContinue, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	beforeContinueApplyFacts := snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts"))
	out.Reset()
	if err := Run([]string{"-Command", "continue", "main", "-Format", "json", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var continueApply struct {
		Command                        string                                 `json:"command"`
		Pack                           string                                 `json:"pack"`
		IsMutation                     bool                                   `json:"isMutation"`
		Applied                        bool                                   `json:"applied"`
		RequiresConfirmation           bool                                   `json:"requiresConfirmation"`
		Selector                       string                                 `json:"selector"`
		RunID                          string                                 `json:"runId"`
		Lane                           startLane                              `json:"lane"`
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
		MissionCommanderNextActions    []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
		Writes                         []startWrite                           `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &continueApply); err != nil {
		t.Fatalf("no-pack continue apply after adapter evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if continueApply.Command != "continue" || continueApply.Pack != "_template" || !continueApply.IsMutation || !continueApply.Applied || continueApply.RequiresConfirmation || continueApply.Selector != "main" || continueApply.RunID == "" || continueApply.Lane.ID != "main" || len(continueApply.ExecutionEvidenceReview) != 1 {
		t.Fatalf("unexpected no-pack continue apply after adapter evidence: %+v", continueApply)
	}
	assertWriteKind(t, continueApply.Writes, "lane-resume", "refresh")
	assertWriteKind(t, continueApply.Writes, "lane-checkpoint", "refresh")
	assertWriteKind(t, continueApply.Writes, "board", "refresh")
	assertWriteKind(t, continueApply.Writes, "run-status", "write")
	assertWriteKind(t, continueApply.Writes, "run-digest", "write")
	continueApplyEvidence := continueApply.ExecutionEvidenceReview[0]
	if continueApplyEvidence.EventID != evidence.EventID || continueApplyEvidence.GateEventID != applied.EventID || continueApplyEvidence.Status != "succeeded" || continueApplyEvidence.Action != "debug" || !containsSubstring(continueApplyEvidence.OutputRefs, "workspace/main/debug/session-1/result.json") || continueApplyEvidence.MissionCommanderAction.State != "ready-for-evidence-review" || continueApplyEvidence.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || continueApplyEvidence.FollowThrough.State != "ready-for-evidence-review" || !cliExecutionEvidenceFollowThroughContains(continueApplyEvidence.FollowThrough, "recorded-evidence-review", "reviewed outputRefs/evidenceRefs") {
		t.Fatalf("no-pack continue apply omitted recorded adapter evidence review handoff: %+v", continueApplyEvidence)
	}
	assertNestedAdapterExecutionEvidenceReviewSummary(t, "no-pack continue apply JSON", continueApply.ExecutionEvidenceReviewSummary, evidence.EventID, applied.EventID)
	continueApplyQueue := continueApply.MissionCommanderActionQueue
	if continueApplyQueue.CurrentAction == nil || continueApplyQueue.CurrentAction.Source != "executionEvidenceReview" || continueApplyQueue.CurrentAction.Command != "/rekit handoff main" || !continueApplyQueue.CurrentAction.RequiresReview || continueApplyQueue.Counts.Total == 0 || continueApplyQueue.Counts.RequiresReview == 0 {
		t.Fatalf("no-pack continue apply omitted evidence review action queue: %+v", continueApplyQueue)
	}
	continueStatusBytes, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "runs", continueApply.RunID, "status.json"))
	if err != nil {
		t.Fatal(err)
	}
	var continueStatus struct {
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
		MissionCommanderNextActions    []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(continueStatusBytes, &continueStatus); err != nil {
		t.Fatalf("written no-pack continue status after adapter evidence is not JSON: %v\n%s", err, string(continueStatusBytes))
	}
	if len(continueStatus.ExecutionEvidenceReview) != 1 || continueStatus.ExecutionEvidenceReview[0].EventID != evidence.EventID || continueStatus.ExecutionEvidenceReview[0].GateEventID != applied.EventID || continueStatus.ExecutionEvidenceReview[0].MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || len(continueStatus.MissionCommanderNextActions) == 0 || continueStatus.MissionCommanderActionQueue.CurrentAction == nil || continueStatus.MissionCommanderActionQueue.CurrentAction.Command != "/rekit handoff main" {
		t.Fatalf("written no-pack continue status omitted evidence review handoff: %+v", continueStatus)
	}
	assertNestedAdapterExecutionEvidenceReviewSummary(t, "no-pack continue run status JSON", continueStatus.ExecutionEvidenceReviewSummary, evidence.EventID, applied.EventID)
	continueDigest, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "runs", continueApply.RunID, "digest.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"## Execution evidence review",
		"summary: total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=0 boundaryHits=0 latestEventId=" + evidence.EventID,
		"summary report: path=workspace/main/debug/session-1/adapter-report.json adapterId=nested-cli-adapter adapterStatus=succeeded",
		"summary follow-through: state=ready-for-evidence-review outcomes=1",
		"summary boundary: no authority/confirmed writes",
		"gateEventId=" + applied.EventID,
		"action=debug",
		"outputRefs:",
		"workspace/main/debug/session-1/result.json",
		"execution report: workspace/main/debug/session-1/adapter-report.json",
		"actual budget: runtimeSeconds=20 diskMB=32 requests=1",
		"adapter report: adapterId=nested-cli-adapter status=succeeded",
		"review command: `review outputRefs/evidenceRefs for gateEventId " + applied.EventID + "`",
		"handoff command: `/rekit handoff main`",
		"commander state: ready-for-evidence-review",
		"commander primary: `/rekit handoff main`",
		"evidence: workspace/main/debug/session-1/result.json",
	} {
		if !strings.Contains(string(continueDigest), expected) {
			t.Fatalf("written no-pack continue digest after adapter evidence missing %q:\n%s", expected, string(continueDigest))
		}
	}
	resumeAfterContinue, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resumeAfterContinue), "summary: total=1 readyForReview=1 mainEscalations=0") || !strings.Contains(string(resumeAfterContinue), "summary report: path=workspace/main/debug/session-1/adapter-report.json adapterId=nested-cli-adapter adapterStatus=succeeded") || !strings.Contains(string(resumeAfterContinue), "gateEventId="+applied.EventID) || !strings.Contains(string(resumeAfterContinue), "evidence: workspace/main/debug/session-1/result.json") || !strings.Contains(string(resumeAfterContinue), "execution report: workspace/main/debug/session-1/adapter-report.json") || !strings.Contains(string(resumeAfterContinue), "adapter report: adapterId=nested-cli-adapter status=succeeded") {
		t.Fatalf("no-pack continue apply refreshed RESUME without evidence review handoff:\n%s", string(resumeAfterContinue))
	}
	checkpointAfterContinueBytes, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "checkpoints", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var checkpointAfterContinue struct {
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
		MissionCommanderNextActions    []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue    missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(checkpointAfterContinueBytes, &checkpointAfterContinue); err != nil {
		t.Fatalf("written no-pack checkpoint after continue apply is not JSON: %v\n%s", err, string(checkpointAfterContinueBytes))
	}
	if len(checkpointAfterContinue.ExecutionEvidenceReview) != 1 || checkpointAfterContinue.ExecutionEvidenceReview[0].EventID != evidence.EventID || checkpointAfterContinue.ExecutionEvidenceReview[0].GateEventID != applied.EventID || len(checkpointAfterContinue.MissionCommanderNextActions) == 0 || checkpointAfterContinue.MissionCommanderActionQueue.CurrentAction == nil || checkpointAfterContinue.MissionCommanderActionQueue.CurrentAction.Command != "/rekit handoff main" {
		t.Fatalf("no-pack continue apply refreshed checkpoint without evidence review handoff: %+v", checkpointAfterContinue)
	}
	assertNestedAdapterExecutionEvidenceReviewSummary(t, "no-pack checkpoint after continue JSON", checkpointAfterContinue.ExecutionEvidenceReviewSummary, evidence.EventID, applied.EventID)
	assertSnapshotEqual(t, beforeContinueApplyFacts, snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts")))
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("no-pack continue apply wrote authority ledger or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("no-pack continue apply wrote confirmed ledger or stat failed: %v", err)
	}

	beforeContinueTextApplyFacts := snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts"))
	out.Reset()
	if err := Run([]string{"-Command", "continue", "main", "-Format", "text", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"已选择工作线：main",
		"工作区：workspace/main",
		"continue execution evidence review summary：total=1 readyForReview=1 mainEscalations=0 duplicates=0 outputRefs=1 evidenceRefs=0 boundaryHits=0 hasEscalation=false hasExecutionReport=true hasAdapter=true latestEventId=" + evidence.EventID,
		"latestReport=workspace/main/debug/session-1/adapter-report.json latestAdapterId=nested-cli-adapter latestAdapterStatus=succeeded outcomes=1 followThrough=ready-for-evidence-review",
		"continue execution evidence review summary boundary：no authority/confirmed writes",
		"continue execution evidence review：eventId=" + evidence.EventID + " gateEventId=" + applied.EventID + " status=succeeded action=debug",
		"continue execution evidence report：eventId=" + evidence.EventID + " path=workspace/main/debug/session-1/adapter-report.json",
		"continue execution evidence budget：eventId=" + evidence.EventID + " runtimeSeconds=20 diskMB=32 requests=1",
		"continue execution evidence adapter：eventId=" + evidence.EventID + " adapterId=nested-cli-adapter status=succeeded",
		"continue execution evidence output ref：eventId=" + evidence.EventID + " ref=workspace/main/debug/session-1/result.json",
		"continue execution evidence follow-through：eventId=" + evidence.EventID + " state=ready-for-evidence-review gateEventId=" + applied.EventID,
		"continue execution evidence outcome evidence：eventId=" + evidence.EventID + " name=recorded-evidence-review evidence=workspace/main/debug/session-1/result.json",
		"mission commander action queue current：state=ready-for-evidence-review source=executionEvidenceReview blocked=false requiresReview=true command=`/rekit handoff main`",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("no-pack continue apply text after adapter evidence missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "{\n") {
		t.Fatalf("no-pack continue apply text after adapter evidence should not emit JSON:\n%s", out.String())
	}
	assertSnapshotEqual(t, beforeContinueTextApplyFacts, snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts")))
}

func TestRunGateAdapterReportBoundaryHitNoPackProductPathSuppressesContinue(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeAuthorizedGateVisibilityFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "authorized timeout debug",
		"-TargetRef", "target-alpha",
		"-BatchId", "batch-boundary-hit-product-path",
		"-Scope", "handler only",
		"-RuntimeSeconds", "30",
		"-DiskMB", "64",
		"-Requests", "1",
		"-OutputPaths", "workspace/main/debug/session-1",
		"-StopConditions", "timeout",
		"-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var applied struct {
		EventID string `json:"eventId"`
		Applied bool   `json:"applied"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("authorized boundary-hit gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !applied.Applied || applied.EventID == "" {
		t.Fatalf("unexpected authorized boundary-hit gate result: %+v", applied)
	}

	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "boundary-cli-adapter",
  "action": "debug",
  "status": "boundary-hit",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 20, "diskMB": 32, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/timeout.json"],
  "boundaryHits": ["timeout"],
  "summary": "Adapter stopped at authorized timeout boundary"
}`)
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/timeout.json", `{"stopped":"timeout"}`)

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1")
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	}()

	beforeValidation := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-ValidateExecutionReport", "-GateEventId", applied.EventID, "-ExecutionReportPath", "adapter-report.json", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var validation struct {
		Kind       string `json:"kind"`
		CaseRoot   string `json:"caseRoot"`
		Pack       string `json:"pack"`
		Valid      bool   `json:"valid"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		ReportPath string `json:"reportPath"`
		Report     *struct {
			Status       string   `json:"status"`
			BoundaryHits []string `json:"boundaryHits"`
		} `json:"report"`
	}
	if err := json.Unmarshal(out.Bytes(), &validation); err != nil {
		t.Fatalf("boundary-hit adapter report validation stdout is not JSON: %v\n%s", err, out.String())
	}
	if validation.Kind != "adapter-execution-report-validation" || validation.CaseRoot != caseRoot || validation.Pack != "_template" || !validation.Valid || validation.IsMutation || validation.Applied || validation.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || validation.Report == nil || validation.Report.Status != "boundary-hit" || strings.Join(validation.Report.BoundaryHits, ",") != "timeout" {
		t.Fatalf("unexpected no-pack boundary-hit validation: %+v", validation)
	}
	assertSnapshotEqual(t, beforeValidation, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	if err := os.Chdir(filepath.Join(caseRoot, "workspace", "main")); err != nil {
		t.Fatal(err)
	}
	recordArgs := []string{"-Command", "gate", "-Apply", "-GateEventId", applied.EventID, "-ExecutionReportPath", "debug/session-1/adapter-report.json", "-Actor", "executor-1", "-Format", "json"}
	out.Reset()
	if err := Run(recordArgs, &out); err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Applied           bool   `json:"applied"`
		EventID           string `json:"eventId"`
		Pack              string `json:"pack"`
		ExecutionEvidence struct {
			Status    string `json:"status"`
			Summary   string `json:"summary"`
			Execution struct {
				GateEventID         string   `json:"gateEventId"`
				ExecutionReportPath string   `json:"executionReportPath"`
				OutputRefs          []string `json:"outputRefs"`
				BoundaryHits        []string `json:"boundaryHits"`
			} `json:"execution"`
		} `json:"executionEvidence"`
		MissionCommanderAction      missionCommanderActionSnapshot      `json:"missionCommanderAction"`
		ExecutionEvidenceReview     []executionEvidenceReviewItem       `json:"executionEvidenceReview"`
		MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		NextSteps                   []string                            `json:"nextSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &evidence); err != nil {
		t.Fatalf("boundary-hit adapter evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if !evidence.Applied || evidence.EventID == "" || evidence.Pack != "_template" || evidence.ExecutionEvidence.Status != "boundary-hit" || evidence.ExecutionEvidence.Summary != "Adapter stopped at authorized timeout boundary" || evidence.ExecutionEvidence.Execution.GateEventID != applied.EventID || evidence.ExecutionEvidence.Execution.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || strings.Join(evidence.ExecutionEvidence.Execution.OutputRefs, ",") != "workspace/main/debug/session-1/timeout.json" || strings.Join(evidence.ExecutionEvidence.Execution.BoundaryHits, ",") != "timeout" {
		t.Fatalf("unexpected no-pack boundary-hit evidence: %+v", evidence)
	}
	if evidence.MissionCommanderAction.State != "needs-main-escalation" || evidence.MissionCommanderAction.PrimaryCommand != "/rekit handoff main" || containsSubstring(evidence.MissionCommanderAction.FollowUpCommands, "/rekit continue") || !containsSubstring(evidence.MissionCommanderAction.Boundary, "stop autonomous work") || !containsSubstring(evidence.NextSteps, "boundary hit or escalation") {
		t.Fatalf("boundary-hit evidence omitted main escalation commander handoff: action=%+v next=%+v", evidence.MissionCommanderAction, evidence.NextSteps)
	}
	if len(evidence.ExecutionEvidenceReview) != 1 || evidence.ExecutionEvidenceReview[0].EventID != evidence.EventID || evidence.ExecutionEvidenceReview[0].GateEventID != applied.EventID || evidence.ExecutionEvidenceReview[0].Status != "boundary-hit" || strings.Join(evidence.ExecutionEvidenceReview[0].BoundaryHits, ",") != "timeout" || evidence.ExecutionEvidenceReview[0].MissionCommanderAction.State != "needs-main-escalation" || evidence.ExecutionEvidenceReview[0].FollowThrough.State != "needs-main-escalation" || !cliExecutionEvidenceFollowThroughContains(evidence.ExecutionEvidenceReview[0].FollowThrough, "boundary-or-escalation-review", "main Agent reviews boundary/escalation") {
		t.Fatalf("boundary-hit evidence omitted review follow-through: %+v", evidence.ExecutionEvidenceReview)
	}
	if len(evidence.MissionCommanderNextActions) != 2 || evidence.MissionCommanderNextActions[0].Command != "/rekit handoff main" || !evidence.MissionCommanderNextActions[0].Blocked || evidence.MissionCommanderNextActions[1].Command != "/rekit overview" || containsMissionCommanderNextActionsCommand(evidence.MissionCommanderNextActions, "/rekit continue main") || evidence.MissionCommanderActionQueue.CurrentAction == nil || evidence.MissionCommanderActionQueue.CurrentAction.Command != "/rekit handoff main" || !evidence.MissionCommanderActionQueue.CurrentAction.Blocked || evidence.MissionCommanderActionQueue.Counts.Unblocked != 0 || evidence.MissionCommanderActionQueue.Counts.Blocked != 2 {
		t.Fatalf("boundary-hit evidence next actions should suppress continue: next=%+v queue=%+v", evidence.MissionCommanderNextActions, evidence.MissionCommanderActionQueue)
	}
	observations, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(observations), `"boundaryHits":["timeout"]`) || strings.Contains(string(observations), `"kind":"authority"`) || strings.Contains(string(observations), `"kind":"confirmed"`) {
		t.Fatalf("boundary-hit evidence ledger mismatch:\n%s", string(observations))
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("boundary-hit adapter evidence wrote authority ledger or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("boundary-hit adapter evidence wrote confirmed ledger or stat failed: %v", err)
	}

	beforeStatus := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		Pack        string `json:"pack"`
		PackSource  string `json:"packSource"`
		CaseMission struct {
			ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
			ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
			MissionCommanderNextActions    []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
			MissionCommanderActionQueue    missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("boundary-hit no-pack status stdout is not JSON: %v\n%s", err, out.String())
	}
	if status.Pack != "_template" || status.PackSource != "case-metadata" || len(status.CaseMission.ExecutionEvidenceReview) != 1 || strings.Join(status.CaseMission.ExecutionEvidenceReview[0].BoundaryHits, ",") != "timeout" || status.CaseMission.ExecutionEvidenceReview[0].MissionCommanderAction.State != "needs-main-escalation" || containsMissionCommanderNextActionsCommand(status.CaseMission.MissionCommanderNextActions, "/rekit continue main") || status.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || !status.CaseMission.MissionCommanderActionQueue.CurrentAction.Blocked {
		t.Fatalf("boundary-hit status omitted escalation/no-continue handoff: %+v", status.CaseMission)
	}
	assertBoundaryHitAdapterExecutionEvidenceReviewSummary(t, "boundary-hit status JSON", status.CaseMission.ExecutionEvidenceReviewSummary, evidence.EventID, applied.EventID)
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status case mission evidence review：eventId=" + evidence.EventID + " gateEventId=" + applied.EventID + " status=boundary-hit action=debug",
		"status case mission evidence boundary hit：eventId=" + evidence.EventID + " hit=timeout",
		"status case mission evidence follow-through：eventId=" + evidence.EventID + " state=needs-main-escalation gateEventId=" + applied.EventID,
		"status case mission evidence outcome：eventId=" + evidence.EventID + " name=boundary-or-escalation-review",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("boundary-hit status text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "command=/rekit continue main") || strings.Contains(out.String(), "{\n") {
		t.Fatalf("boundary-hit status text should suppress continue next action and not emit JSON:\n%s", out.String())
	}
	assertSnapshotEqual(t, beforeStatus, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"overview execution evidence item：eventId=" + evidence.EventID + " gateEventId=" + applied.EventID + " status=boundary-hit action=debug",
		"overview execution evidence boundary hit：eventId=" + evidence.EventID + " hit=timeout",
		"overview execution evidence follow-through：eventId=" + evidence.EventID + " state=needs-main-escalation gateEventId=" + applied.EventID,
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("boundary-hit overview text missing %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "handoff", "main", "-Format", "text", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"handoff execution evidence review：eventId=" + evidence.EventID + " gateEventId=" + applied.EventID + " status=boundary-hit action=debug",
		"handoff execution evidence boundary hit：eventId=" + evidence.EventID + " hit=timeout",
		"mission commander action queue current：state=needs-main-escalation source=executionEvidenceReview blocked=true requiresReview=true command=`/rekit handoff main`",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("boundary-hit handoff preview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "command=`/rekit continue main") || strings.Contains(out.String(), "{\n") {
		t.Fatalf("boundary-hit handoff preview text should suppress continue next action and not emit JSON:\n%s", out.String())
	}

	beforeHandoffApplyFacts := snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts"))
	out.Reset()
	if err := Run([]string{"-Command", "handoff", "main", "-Format", "json", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var handoffApply handoffResult
	if err := json.Unmarshal(out.Bytes(), &handoffApply); err != nil {
		t.Fatalf("boundary-hit handoff apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !handoffApply.Applied || len(handoffApply.ExecutionEvidenceReview) != 1 || strings.Join(handoffApply.ExecutionEvidenceReview[0].BoundaryHits, ",") != "timeout" || handoffApply.ExecutionEvidenceReview[0].MissionCommanderAction.State != "needs-main-escalation" || containsMissionCommanderNextActionsCommand(handoffApply.MissionCommanderNextActions, "/rekit continue main") || handoffApply.MissionCommanderActionQueue.CurrentAction == nil || !handoffApply.MissionCommanderActionQueue.CurrentAction.Blocked {
		t.Fatalf("boundary-hit handoff apply omitted escalation/no-continue handoff: %+v", handoffApply)
	}
	latestHandoff, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "handovers", "main-latest.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"当前不要执行 `/rekit continue main`",
		"status=boundary-hit | gateEventId=" + applied.EventID,
		"boundary hit: timeout",
		"follow-through: state=needs-main-escalation",
		"outcome: name=boundary-or-escalation-review",
	} {
		if !strings.Contains(string(latestHandoff), expected) {
			t.Fatalf("boundary-hit written handoff missing %q:\n%s", expected, string(latestHandoff))
		}
	}
	resume, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status=boundary-hit | gateEventId=" + applied.EventID,
		"boundaryHits: timeout",
		"follow-through: state=needs-main-escalation",
		"outcome: name=boundary-or-escalation-review",
	} {
		if !strings.Contains(string(resume), expected) {
			t.Fatalf("boundary-hit RESUME missing %q:\n%s", expected, string(resume))
		}
	}
	assertSnapshotEqual(t, beforeHandoffApplyFacts, snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts")))

	beforeContinueApplyFacts := snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts"))
	out.Reset()
	if err := Run([]string{"-Command", "continue", "main", "-Format", "text", "-WhatIf"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"continue execution evidence review：eventId=" + evidence.EventID + " gateEventId=" + applied.EventID + " status=boundary-hit action=debug",
		"continue execution evidence boundary hit：eventId=" + evidence.EventID + " hit=timeout",
		"mission commander action queue current：state=needs-main-escalation source=executionEvidenceReview blocked=true requiresReview=true command=`/rekit handoff main`",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("boundary-hit continue preview text missing %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "command=`/rekit continue main") || strings.Contains(out.String(), "{\n") {
		t.Fatalf("boundary-hit continue preview text should suppress continue next action and not emit JSON:\n%s", out.String())
	}

	out.Reset()
	if err := Run([]string{"-Command", "continue", "main", "-Format", "json", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var continueApply struct {
		Applied                     bool                                `json:"applied"`
		RunID                       string                              `json:"runId"`
		ExecutionEvidenceReview     []executionEvidenceReviewItem       `json:"executionEvidenceReview"`
		MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		Writes                      []startWrite                        `json:"writes"`
	}
	if err := json.Unmarshal(out.Bytes(), &continueApply); err != nil {
		t.Fatalf("boundary-hit continue apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !continueApply.Applied || continueApply.RunID == "" || len(continueApply.ExecutionEvidenceReview) != 1 || strings.Join(continueApply.ExecutionEvidenceReview[0].BoundaryHits, ",") != "timeout" || continueApply.ExecutionEvidenceReview[0].MissionCommanderAction.State != "needs-main-escalation" || containsMissionCommanderNextActionsCommand(continueApply.MissionCommanderNextActions, "/rekit continue main") || continueApply.MissionCommanderActionQueue.CurrentAction == nil || !continueApply.MissionCommanderActionQueue.CurrentAction.Blocked {
		t.Fatalf("boundary-hit continue apply omitted escalation/no-continue handoff: %+v", continueApply)
	}
	continueDigest, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "runs", continueApply.RunID, "digest.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status=boundary-hit | gateEventId=" + applied.EventID,
		"boundaryHits:",
		"timeout",
		"follow-through: state=needs-main-escalation",
		"outcome: name=boundary-or-escalation-review",
	} {
		if !strings.Contains(string(continueDigest), expected) {
			t.Fatalf("boundary-hit continue digest missing %q:\n%s", expected, string(continueDigest))
		}
	}
	checkpointBytes, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "checkpoints", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		ExecutionEvidenceReview     []executionEvidenceReviewItem       `json:"executionEvidenceReview"`
		MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
		MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("boundary-hit checkpoint did not decode: %v\n%s", err, string(checkpointBytes))
	}
	if len(checkpoint.ExecutionEvidenceReview) != 1 || strings.Join(checkpoint.ExecutionEvidenceReview[0].BoundaryHits, ",") != "timeout" || checkpoint.ExecutionEvidenceReview[0].MissionCommanderAction.State != "needs-main-escalation" || containsMissionCommanderNextActionsCommand(checkpoint.MissionCommanderNextActions, "/rekit continue main") || checkpoint.MissionCommanderActionQueue.CurrentAction == nil || !checkpoint.MissionCommanderActionQueue.CurrentAction.Blocked {
		t.Fatalf("boundary-hit checkpoint omitted escalation/no-continue handoff: %+v", checkpoint)
	}
	assertSnapshotEqual(t, beforeContinueApplyFacts, snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts")))
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("boundary-hit continue apply wrote authority ledger or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("boundary-hit continue apply wrote confirmed ledger or stat failed: %v", err)
	}
}

func TestRunGateProjectsPackToolingAdapterCandidateProductPath(t *testing.T) {
	caseRoot := attachedCaseWithPack(t, "generic-binary-re")
	for _, dir := range []string{".rekit/facts", ".rekit/lanes/main", "workspace/main/debug/session-1"} {
		if err := os.MkdirAll(filepath.Join(caseRoot, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeCaseFile(t, caseRoot, ".rekit/board.json", `{"lanes":[{"id":"main","type":"main","workspace":"workspace/main"}],"factsRoot":".rekit/facts"}`)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","type":"main","status":"open","authority":true,"workspace":"workspace/main","laneRoot":".rekit/lanes/main"}`)
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
  "notifyMainOn": ["boundary-hit"],
  "grantedBy": "user",
  "grantedAt": "2026-01-01T00:00:00Z",
  "expiresAt": "2999-01-01T00:00:00Z"
}`)

	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "generic-binary-re",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "authorized generic binary debug",
		"-TargetRef", "target-alpha",
		"-RuntimeSeconds", "30",
		"-DiskMB", "64",
		"-Requests", "1",
		"-OutputPaths", "workspace/main/debug/session-1",
		"-StopConditions", "timeout",
		"-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var applied struct {
		Applied bool   `json:"applied"`
		EventID string `json:"eventId"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("authorized generic-binary-re gate stdout is not JSON: %v\n%s", err, out.String())
	}
	if !applied.Applied || applied.EventID == "" {
		t.Fatalf("unexpected generic-binary-re gate apply result: %+v", applied)
	}

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "generic-binary-re", "-ExecutionReportContract", "-GateEventId", applied.EventID, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Kind           string `json:"kind"`
		LiveValidation struct {
			AdapterCandidates []struct {
				ID                  string   `json:"id"`
				Status              string   `json:"status"`
				Entry               string   `json:"entry"`
				Purpose             string   `json:"purpose"`
				SideEffects         []string `json:"sideEffects"`
				GateActions         []string `json:"gateActions"`
				ToolingCatalogPath  string   `json:"toolingCatalogPath"`
				ReportGuidance      []string `json:"reportGuidance"`
				EvidenceGuidance    []string `json:"evidenceGuidance"`
				StopConditionHints  []string `json:"stopConditionHints"`
				RecordOnlyAfterGate bool     `json:"recordOnlyAfterGate"`
			} `json:"adapterCandidates"`
			CaseRelativeValidateArgs []string `json:"caseRelativeValidateArgs"`
			CaseRelativeRecordArgs   []string `json:"caseRelativeRecordArgs"`
		} `json:"liveValidation"`
	}
	if err := json.Unmarshal(out.Bytes(), &contract); err != nil {
		t.Fatalf("generic-binary-re adapter contract stdout is not JSON: %v\n%s", err, out.String())
	}
	if contract.Kind != "adapter-execution-report-contract" || len(contract.LiveValidation.AdapterCandidates) != 1 {
		t.Fatalf("generic-binary-re contract omitted concrete adapter candidate: %+v", contract)
	}
	candidate := contract.LiveValidation.AdapterCandidates[0]
	if candidate.ID != "dynamic-debug-or-writeback-action" || candidate.Status != "cautious" || candidate.ToolingCatalogPath != "tooling/catalog.yml" || !slices.Contains(candidate.GateActions, "debug") || !candidate.RecordOnlyAfterGate {
		t.Fatalf("generic-binary-re adapter candidate identity drifted: %+v", candidate)
	}
	if !strings.Contains(candidate.Entry, "dynamic-debug") || !strings.Contains(candidate.Purpose, "bounded debug") || !containsSubstring(candidate.SideEffects, "process execution") || !containsSubstring(candidate.ReportGuidance, "adapterId") || !containsSubstring(candidate.EvidenceGuidance, "ValidateArgs") || strings.Join(candidate.StopConditionHints, ",") != "timeout,unexpected-side-effect,scope-drift" {
		t.Fatalf("generic-binary-re adapter candidate omitted operational guidance: %+v", candidate)
	}

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var status struct {
		CaseMission struct {
			AuthorizedGateHandoffs []statusAuthorizedGateHandoff `json:"authorizedGateHandoffs"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("generic-binary-re status stdout is not JSON: %v\n%s", err, out.String())
	}
	if len(status.CaseMission.AuthorizedGateHandoffs) != 1 {
		t.Fatalf("generic-binary-re status omitted authorized gate handoff: %+v", status.CaseMission)
	}
	statusLive := status.CaseMission.AuthorizedGateHandoffs[0].LiveValidation
	if statusLive == nil || statusLive.SelectedAdapter == nil || statusLive.SelectedAdapter.ID != candidate.ID || !strings.Contains(statusLive.SelectedAdapter.Purpose, "bounded debug") || !containsSubstring(statusLive.SelectedAdapter.SideEffects, "process execution") || !containsSubstring(statusLive.SelectedAdapter.ReportGuidance, "adapterId") || !containsSubstring(statusLive.SelectedAdapter.EvidenceGuidance, "ValidateArgs") || strings.Join(statusLive.SelectedAdapter.StopConditionHints, ",") != "timeout,unexpected-side-effect,scope-drift" {
		t.Fatalf("generic-binary-re status handoff omitted selected adapter detail: %+v", statusLive)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status case mission authorized gate selected adapter：eventId=" + applied.EventID + " id=dynamic-debug-or-writeback-action status=cautious entry=",
		"status case mission authorized gate selected adapter report guidance：eventId=" + applied.EventID + " id=dynamic-debug-or-writeback-action guidance=",
		"status case mission authorized gate selected adapter evidence guidance：eventId=" + applied.EventID + " id=dynamic-debug-or-writeback-action guidance=",
		"status case mission authorized gate selected adapter stop conditions：eventId=" + applied.EventID + " id=dynamic-debug-or-writeback-action hints=timeout,unexpected-side-effect,scope-drift",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("generic-binary-re status text omitted selected adapter handoff %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var overview struct {
		AuthorizedGateAdapterHandoffs []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
	}
	if err := json.Unmarshal(out.Bytes(), &overview); err != nil {
		t.Fatalf("generic-binary-re overview stdout is not JSON: %v\n%s", err, out.String())
	}
	if len(overview.AuthorizedGateAdapterHandoffs) != 1 || overview.AuthorizedGateAdapterHandoffs[0].LiveValidation == nil || overview.AuthorizedGateAdapterHandoffs[0].LiveValidation.SelectedAdapter == nil || overview.AuthorizedGateAdapterHandoffs[0].LiveValidation.SelectedAdapter.ID != candidate.ID || !containsSubstring(overview.AuthorizedGateAdapterHandoffs[0].LiveValidation.SelectedAdapter.EvidenceGuidance, "ValidateArgs") {
		t.Fatalf("generic-binary-re overview omitted selected adapter detail: %+v", overview.AuthorizedGateAdapterHandoffs)
	}
	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"overview authorized gate adapter selected adapter：eventId=" + applied.EventID + " id=dynamic-debug-or-writeback-action status=cautious entry=",
		"overview authorized gate adapter selected adapter evidence guidance：eventId=" + applied.EventID + " id=dynamic-debug-or-writeback-action guidance=",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("generic-binary-re overview text omitted selected adapter handoff %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Target", caseRoot, "-Pack", "generic-binary-re", "-ExecutionReportContract", "-GateEventId", applied.EventID, "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{
		"gate adapter report sidecar template：kind=adapter-execution-report adapterId=dynamic-debug-or-writeback-action action=debug status=succeeded|failed|boundary-hit|escalated|aborted gateEventId=" + applied.EventID,
		"gate adapter report adapter candidate：id=dynamic-debug-or-writeback-action status=cautious entry=",
		"gateActions=debug recordOnlyAfterGate=true toolingCatalogPath=tooling/catalog.yml",
		"gate adapter report adapter candidate purpose：id=dynamic-debug-or-writeback-action purpose=",
		"gate adapter report adapter candidate report guidance：id=dynamic-debug-or-writeback-action guidance=",
		"gate adapter report adapter candidate evidence guidance：id=dynamic-debug-or-writeback-action guidance=",
		"gate adapter report adapter candidate stop conditions：id=dynamic-debug-or-writeback-action hints=timeout,unexpected-side-effect,scope-drift",
		"gate adapter report selected adapter：id=dynamic-debug-or-writeback-action status=cautious entry=",
		"gate adapter report selected adapter report guidance：id=dynamic-debug-or-writeback-action guidance=",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("generic-binary-re adapter contract text missing %q:\n%s", expected, text)
		}
	}
	for _, expected := range []string{"bounded debug", "adapterId", "ValidateArgs"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("generic-binary-re adapter contract text omitted %q:\n%s", expected, text)
		}
	}

	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/result.json", `{"ok":true}`)
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "dynamic-debug-or-writeback-action",
  "action": "debug",
  "status": "boundary-hit",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Generic binary adapter stopped without boundary marker"
}`)
	beforeInvalidLiveStatus := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var invalidLiveStatus struct {
		CaseMission struct {
			AuthorizedGateHandoffs      []statusAuthorizedGateHandoff       `json:"authorizedGateHandoffs"`
			MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
			MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &invalidLiveStatus); err != nil {
		t.Fatalf("generic-binary-re invalid live status stdout is not JSON: %v\n%s", err, out.String())
	}
	invalidLiveHandoff := invalidLiveStatus.CaseMission.AuthorizedGateHandoffs[0]
	if invalidLiveHandoff.ReportSummary == nil || invalidLiveHandoff.ReportSummary.State != "repair-adapter-report" || !invalidLiveHandoff.ReportSummary.ReportPresent || invalidLiveHandoff.ReportSummary.Valid || invalidLiveHandoff.ReportSummary.RecordReady || !invalidLiveHandoff.ReportSummary.RecordBlocked || !invalidLiveHandoff.ReportSummary.RequiresRepair || invalidLiveHandoff.ReportSummary.ValidationFailureCode != "boundary-marker-missing" || invalidLiveHandoff.ReportSummary.ValidationFailureStage != "boundary" || len(invalidLiveHandoff.LiveValidationRepairHints) != 1 || invalidLiveHandoff.LiveValidationRepairHints[0].RepairAction != "add-boundary-marker" || !invalidLiveHandoff.LiveValidationRepairHints[0].RerunValidation || len(invalidLiveHandoff.LiveValidationNextSteps) == 0 || invalidLiveHandoff.LiveValidationError != "" {
		t.Fatalf("generic-binary-re invalid live status did not expose repair state: %+v", invalidLiveHandoff)
	}
	if invalidLiveStatus.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || invalidLiveStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.State != "repair-adapter-report" || invalidLiveStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.Command != "add-boundary-marker" || !invalidLiveStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.RequiresReview || len(invalidLiveStatus.CaseMission.MissionCommanderNextActions) == 0 || strings.Contains(invalidLiveStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.Command, "/rekit continue") {
		t.Fatalf("generic-binary-re invalid sidecar repair did not become Mission Commander current action: %+v", invalidLiveStatus.CaseMission.MissionCommanderActionQueue)
	}
	assertSnapshotEqual(t, beforeInvalidLiveStatus, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "generic-binary-re", "-WhatIf", "main", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"handoff authorized gate adapter report summary：eventId=" + applied.EventID + " state=repair-adapter-report",
		"failureCode=boundary-marker-missing failureStage=boundary",
		"handoff authorized gate adapter live validation repair：eventId=" + applied.EventID + " action=add-boundary-marker code=boundary-marker-missing stage=boundary",
		"handoff authorized gate adapter live validation next step：eventId=" + applied.EventID,
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("generic-binary-re invalid handoff text omitted live repair %q:\n%s", expected, out.String())
		}
	}

	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "dynamic-debug-or-writeback-action",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "evidenceRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Generic binary adapter completed bounded debug handoff"
}`)
	beforeLiveStatus := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var liveStatus struct {
		CaseMission struct {
			AuthorizedGateHandoffs      []statusAuthorizedGateHandoff       `json:"authorizedGateHandoffs"`
			MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &liveStatus); err != nil {
		t.Fatalf("generic-binary-re live status stdout is not JSON: %v\n%s", err, out.String())
	}
	if len(liveStatus.CaseMission.AuthorizedGateHandoffs) != 1 {
		t.Fatalf("generic-binary-re live status omitted authorized gate handoff: %+v", liveStatus.CaseMission)
	}
	liveHandoff := liveStatus.CaseMission.AuthorizedGateHandoffs[0]
	if liveHandoff.ReportSummary == nil || liveHandoff.ReportSummary.State != "ready-to-record-evidence" || !liveHandoff.ReportSummary.ReportPresent || !liveHandoff.ReportSummary.Valid || !liveHandoff.ReportSummary.RecordReady || liveHandoff.ReportSummary.RecordBlocked || liveHandoff.ReportSummary.RequiresValidation || liveHandoff.ReportSummary.RequiresRepair || liveHandoff.ReportSummary.AdapterID != candidate.ID || liveHandoff.LiveValidation == nil || liveHandoff.LiveValidation.SelectedAdapter == nil || liveHandoff.LiveValidation.SelectedAdapter.ID != candidate.ID || liveHandoff.LiveValidationError != "" {
		t.Fatalf("generic-binary-re live status did not promote valid sidecar to record-ready: %+v", liveHandoff)
	}
	if liveStatus.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || liveStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.State != "ready-to-record-evidence" || !strings.Contains(liveStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.Command, " -Apply ") || !liveStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.RequiresReview {
		t.Fatalf("generic-binary-re valid sidecar record did not become review-owned Mission Commander current action: %+v", liveStatus.CaseMission.MissionCommanderActionQueue)
	}
	assertSnapshotEqual(t, beforeLiveStatus, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "generic-binary-re", "-WhatIf", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	liveHandoffPreview := decodeHandoffResult(t, out.Bytes())
	if len(liveHandoffPreview.AuthorizedGateAdapterHandoffs) != 1 || liveHandoffPreview.AuthorizedGateAdapterHandoffs[0].ReportSummary == nil || !liveHandoffPreview.AuthorizedGateAdapterHandoffs[0].ReportSummary.RecordReady || !liveHandoffPreview.AuthorizedGateAdapterHandoffs[0].ReportSummary.Valid {
		t.Fatalf("generic-binary-re handoff preview omitted live record-ready sidecar: %+v", liveHandoffPreview.AuthorizedGateAdapterHandoffs)
	}
	if liveHandoffPreview.MissionCommanderActionQueue.CurrentAction == nil || liveHandoffPreview.MissionCommanderActionQueue.CurrentAction.State != "ready-to-record-evidence" || !strings.Contains(liveHandoffPreview.MissionCommanderActionQueue.CurrentAction.Command, " -Apply ") {
		t.Fatalf("generic-binary-re handoff queue omitted record-ready current action: %+v", liveHandoffPreview.MissionCommanderActionQueue)
	}
	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "generic-binary-re", "-WhatIf", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var liveContinue struct {
		IsMutation                    bool                                   `json:"isMutation"`
		Applied                       bool                                   `json:"applied"`
		AuthorizedGateAdapterHandoffs []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
		MissionCommanderActionQueue   missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(out.Bytes(), &liveContinue); err != nil {
		t.Fatalf("generic-binary-re continue live snapshot stdout is not JSON: %v\n%s", err, out.String())
	}
	if liveContinue.IsMutation || liveContinue.Applied || len(liveContinue.AuthorizedGateAdapterHandoffs) != 1 || liveContinue.AuthorizedGateAdapterHandoffs[0].ReportSummary == nil || !liveContinue.AuthorizedGateAdapterHandoffs[0].ReportSummary.RecordReady || !liveContinue.AuthorizedGateAdapterHandoffs[0].ReportSummary.Valid {
		t.Fatalf("generic-binary-re continue preview omitted live record-ready sidecar: %+v", liveContinue)
	}
	if liveContinue.MissionCommanderActionQueue.CurrentAction == nil || liveContinue.MissionCommanderActionQueue.CurrentAction.State != "ready-to-record-evidence" || !strings.Contains(liveContinue.MissionCommanderActionQueue.CurrentAction.Command, " -Apply ") {
		t.Fatalf("generic-binary-re continue queue omitted record-ready current action: %+v", liveContinue.MissionCommanderActionQueue)
	}
	assertSnapshotEqual(t, beforeLiveStatus, snapshotFiles(t, filepath.Join(caseRoot, ".rekit")))
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(caseRoot, "workspace", "main")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(oldwd); err != nil {
			t.Fatal(err)
		}
	}()
	out.Reset()
	if err := Run(contract.LiveValidation.CaseRelativeValidateArgs, &out); err != nil {
		t.Fatal(err)
	}
	var validation struct {
		Valid          bool `json:"valid"`
		IsMutation     bool `json:"isMutation"`
		Applied        bool `json:"applied"`
		AdapterContext struct {
			Candidates []struct {
				ID string `json:"id"`
			} `json:"candidates"`
			Selected *struct {
				ID string `json:"id"`
			} `json:"selected"`
		} `json:"adapterContext"`
	}
	if err := json.Unmarshal(out.Bytes(), &validation); err != nil {
		t.Fatalf("generic-binary-re adapter validation stdout is not JSON: %v\n%s", err, out.String())
	}
	if !validation.Valid || validation.IsMutation || validation.Applied || len(validation.AdapterContext.Candidates) != 1 || validation.AdapterContext.Selected == nil || validation.AdapterContext.Selected.ID != candidate.ID {
		t.Fatalf("generic-binary-re adapter validation omitted selected candidate context: %+v", validation)
	}

	out.Reset()
	textArgs := append([]string{}, contract.LiveValidation.CaseRelativeValidateArgs...)
	textArgs[len(textArgs)-1] = "text"
	if err := Run(textArgs, &out); err != nil {
		t.Fatal(err)
	}
	validationText := out.String()
	for _, expected := range []string{
		"gate adapter report validation adapter candidate：id=dynamic-debug-or-writeback-action status=cautious entry=",
		"gate adapter report validation selected adapter：id=dynamic-debug-or-writeback-action status=cautious entry=",
		"gate adapter report validation selected adapter evidence guidance：id=dynamic-debug-or-writeback-action guidance=",
	} {
		if !strings.Contains(validationText, expected) {
			t.Fatalf("generic-binary-re adapter validation text missing %q:\n%s", expected, validationText)
		}
	}

	recordArgs := append([]string{}, contract.LiveValidation.CaseRelativeRecordArgs...)
	for i, arg := range recordArgs {
		if arg == "<executor-id>" {
			recordArgs[i] = "executor-1"
		}
	}
	out.Reset()
	if err := Run(recordArgs, &out); err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Applied           bool   `json:"applied"`
		EventID           string `json:"eventId"`
		ExecutionEvidence struct {
			Execution struct {
				ExecutionReportPath string                        `json:"executionReportPath"`
				AdapterContext      *adapterToolCandidateSnapshot `json:"adapterContext"`
				Adapter             struct {
					AdapterID string `json:"adapterId"`
				} `json:"adapter"`
			} `json:"execution"`
		} `json:"executionEvidence"`
	}
	if err := json.Unmarshal(out.Bytes(), &evidence); err != nil {
		t.Fatalf("generic-binary-re adapter evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if !evidence.Applied || evidence.EventID == "" || evidence.ExecutionEvidence.Execution.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || evidence.ExecutionEvidence.Execution.Adapter.AdapterID != candidate.ID || evidence.ExecutionEvidence.Execution.AdapterContext == nil || evidence.ExecutionEvidence.Execution.AdapterContext.ID != candidate.ID {
		t.Fatalf("generic-binary-re adapter evidence omitted selected candidate provenance: %+v", evidence)
	}
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Pack", "generic-binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var recordedHandoffStatus struct {
		CaseMission struct {
			AuthorizedGateHandoffs      []statusAuthorizedGateHandoff       `json:"authorizedGateHandoffs"`
			MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &recordedHandoffStatus); err != nil {
		t.Fatalf("generic-binary-re recorded handoff status stdout is not JSON: %v\n%s", err, out.String())
	}
	recordedHandoff := recordedHandoffStatus.CaseMission.AuthorizedGateHandoffs[0]
	if recordedHandoff.ReportSummary == nil || recordedHandoff.ReportSummary.State != "evidence-already-recorded" || recordedHandoff.ReportSummary.RecordReady || !recordedHandoff.ReportSummary.RecordBlocked || recordedHandoff.ReportSummary.RequiresValidation || recordedHandoff.ReportSummary.CurrentAction != "/rekit handoff main" || len(recordedHandoff.LiveValidationNextSteps) != 2 || !strings.Contains(recordedHandoff.LiveValidationNextSteps[0], "do not record or replay") || recordedHandoff.LiveValidation == nil || recordedHandoff.LiveValidation.RecordCommand != "" || recordedHandoff.LiveValidation.CaseRelativeRecordCommand != "" || recordedHandoff.LiveValidation.ReplayBehavior != "" {
		t.Fatalf("generic-binary-re recorded status still recommends duplicate record: %+v", recordedHandoff)
	}
	if recordedHandoffStatus.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || recordedHandoffStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.Source != "executionEvidenceReview" || strings.Contains(recordedHandoffStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.Command, " -Apply ") {
		t.Fatalf("generic-binary-re recorded status did not deduplicate into evidence review: %+v", recordedHandoffStatus.CaseMission.MissionCommanderActionQueue)
	}
	out.Reset()
	if err := Run([]string{"-Command", "handoff", "-Target", caseRoot, "-Pack", "generic-binary-re", "-WhatIf", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	recordedHandoffPreview := decodeHandoffResult(t, out.Bytes())
	if len(recordedHandoffPreview.AuthorizedGateAdapterHandoffs) != 1 || recordedHandoffPreview.AuthorizedGateAdapterHandoffs[0].ReportSummary == nil || recordedHandoffPreview.AuthorizedGateAdapterHandoffs[0].ReportSummary.State != "evidence-already-recorded" || recordedHandoffPreview.AuthorizedGateAdapterHandoffs[0].ReportSummary.RecordReady || recordedHandoffPreview.AuthorizedGateAdapterHandoffs[0].LiveValidation == nil || recordedHandoffPreview.AuthorizedGateAdapterHandoffs[0].LiveValidation.RecordCommand != "" || recordedHandoffPreview.AuthorizedGateAdapterHandoffs[0].LiveValidation.CaseRelativeRecordCommand != "" || recordedHandoffPreview.AuthorizedGateAdapterHandoffs[0].LiveValidation.ReplayBehavior != "" {
		t.Fatalf("generic-binary-re recorded handoff still recommends duplicate record: %+v", recordedHandoffPreview.AuthorizedGateAdapterHandoffs)
	}
	out.Reset()
	if err := Run([]string{"-Command", "continue", "-Target", caseRoot, "-Pack", "generic-binary-re", "-WhatIf", "main", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var recordedContinue struct {
		IsMutation                    bool                                   `json:"isMutation"`
		Applied                       bool                                   `json:"applied"`
		AuthorizedGateAdapterHandoffs []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
	}
	if err := json.Unmarshal(out.Bytes(), &recordedContinue); err != nil {
		t.Fatalf("generic-binary-re recorded continue stdout is not JSON: %v\n%s", err, out.String())
	}
	if recordedContinue.IsMutation || recordedContinue.Applied || len(recordedContinue.AuthorizedGateAdapterHandoffs) != 1 || recordedContinue.AuthorizedGateAdapterHandoffs[0].ReportSummary == nil || recordedContinue.AuthorizedGateAdapterHandoffs[0].ReportSummary.State != "evidence-already-recorded" || recordedContinue.AuthorizedGateAdapterHandoffs[0].ReportSummary.RecordReady || recordedContinue.AuthorizedGateAdapterHandoffs[0].LiveValidation == nil || recordedContinue.AuthorizedGateAdapterHandoffs[0].LiveValidation.RecordCommand != "" || recordedContinue.AuthorizedGateAdapterHandoffs[0].LiveValidation.CaseRelativeRecordCommand != "" || recordedContinue.AuthorizedGateAdapterHandoffs[0].LiveValidation.ReplayBehavior != "" {
		t.Fatalf("generic-binary-re recorded continue still recommends duplicate record: %+v", recordedContinue)
	}
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re record evidence", evidence.ExecutionEvidence.Execution.AdapterContext, candidate.ID)

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var recordedStatus struct {
		CaseMission struct {
			ExecutionEvidenceReviewCount   int                                    `json:"executionEvidenceReviewCount"`
			ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
			ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &recordedStatus); err != nil {
		t.Fatalf("generic-binary-re recorded status stdout is not JSON: %v\n%s", err, out.String())
	}
	if recordedStatus.CaseMission.ExecutionEvidenceReviewCount != 1 || len(recordedStatus.CaseMission.ExecutionEvidenceReview) != 1 {
		t.Fatalf("generic-binary-re status omitted execution evidence review: %+v", recordedStatus.CaseMission)
	}
	statusEvidence := recordedStatus.CaseMission.ExecutionEvidenceReview[0]
	if statusEvidence.EventID != evidence.EventID || statusEvidence.GateEventID != applied.EventID || statusEvidence.AdapterID != candidate.ID || statusEvidence.AdapterStatus != "succeeded" || statusEvidence.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" {
		t.Fatalf("generic-binary-re status execution evidence identity drifted: %+v", statusEvidence)
	}
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re status evidence", statusEvidence.AdapterContext, candidate.ID)
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re status summary", recordedStatus.CaseMission.ExecutionEvidenceReviewSummary.LatestAdapterContext, candidate.ID)

	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "dynamic-debug-or-writeback-action",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 23, "diskMB": 32, "requests": 0},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "evidenceRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Generic binary adapter produced a changed bounded sidecar"
}`)
	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var changedSidecarStatus struct {
		CaseMission struct {
			AuthorizedGateHandoffs      []statusAuthorizedGateHandoff       `json:"authorizedGateHandoffs"`
			MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
		} `json:"caseMission"`
	}
	if err := json.Unmarshal(out.Bytes(), &changedSidecarStatus); err != nil {
		t.Fatalf("generic-binary-re changed sidecar status stdout is not JSON: %v\n%s", err, out.String())
	}
	if len(changedSidecarStatus.CaseMission.AuthorizedGateHandoffs) != 1 || changedSidecarStatus.CaseMission.AuthorizedGateHandoffs[0].ReportSummary == nil || changedSidecarStatus.CaseMission.AuthorizedGateHandoffs[0].ReportSummary.State != "ready-to-record-evidence" || changedSidecarStatus.CaseMission.MissionCommanderActionQueue.CurrentAction == nil || changedSidecarStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.State != "ready-to-record-evidence" || !strings.Contains(changedSidecarStatus.CaseMission.MissionCommanderActionQueue.CurrentAction.Command, " -Apply ") {
		t.Fatalf("generic-binary-re changed sidecar was hidden by old execution evidence: %+v", changedSidecarStatus.CaseMission)
	}
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "dynamic-debug-or-writeback-action",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 24, "diskMB": 33, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "evidenceRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Generic binary adapter completed bounded debug handoff"
}`)

	out.Reset()
	if err := Run([]string{"-Command", "status", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"status case mission evidence review summary latest adapter context：eventId=" + evidence.EventID + " id=dynamic-debug-or-writeback-action status=cautious entry=",
		"status case mission evidence review summary latest adapter context evidence guidance：eventId=" + evidence.EventID + " id=dynamic-debug-or-writeback-action guidance=",
		"status case mission evidence adapter context：eventId=" + evidence.EventID + " id=dynamic-debug-or-writeback-action status=cautious entry=",
		"status case mission evidence adapter context report guidance：eventId=" + evidence.EventID + " id=dynamic-debug-or-writeback-action guidance=",
		"status case mission evidence adapter context evidence guidance：eventId=" + evidence.EventID + " id=dynamic-debug-or-writeback-action guidance=",
		"status case mission evidence adapter context stop conditions：eventId=" + evidence.EventID + " id=dynamic-debug-or-writeback-action hints=timeout,unexpected-side-effect,scope-drift",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("generic-binary-re status text omitted recorded adapter context %q:\n%s", expected, out.String())
		}
	}

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var recordedOverview struct {
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
	}
	if err := json.Unmarshal(out.Bytes(), &recordedOverview); err != nil {
		t.Fatalf("generic-binary-re recorded overview stdout is not JSON: %v\n%s", err, out.String())
	}
	if len(recordedOverview.ExecutionEvidenceReview) != 1 {
		t.Fatalf("generic-binary-re overview omitted execution evidence review: %+v", recordedOverview)
	}
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re overview evidence", recordedOverview.ExecutionEvidenceReview[0].AdapterContext, candidate.ID)
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re overview summary", recordedOverview.ExecutionEvidenceReviewSummary.LatestAdapterContext, candidate.ID)

	out.Reset()
	if err := Run([]string{"-Command", "overview", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "text"}, &out); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"overview execution evidence review summary latest adapter context：eventId=" + evidence.EventID + " id=dynamic-debug-or-writeback-action status=cautious entry=",
		"overview execution evidence adapter context：eventId=" + evidence.EventID + " id=dynamic-debug-or-writeback-action status=cautious entry=",
		"overview execution evidence adapter context evidence guidance：eventId=" + evidence.EventID + " id=dynamic-debug-or-writeback-action guidance=",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("generic-binary-re overview text omitted recorded adapter context %q:\n%s", expected, out.String())
		}
	}

	beforeDurableFacts := snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts"))
	out.Reset()
	if err := Run([]string{"-Command", "handoff", "main", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "json", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var handoffApply handoffResult
	if err := json.Unmarshal(out.Bytes(), &handoffApply); err != nil {
		t.Fatalf("generic-binary-re handoff apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !handoffApply.Applied || len(handoffApply.ExecutionEvidenceReview) != 1 {
		t.Fatalf("generic-binary-re handoff apply omitted execution evidence review: %+v", handoffApply)
	}
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re handoff apply evidence", handoffApply.ExecutionEvidenceReview[0].AdapterContext, candidate.ID)
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re handoff apply summary", handoffApply.ExecutionEvidenceReviewSummary.LatestAdapterContext, candidate.ID)
	latestHandoff, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "handovers", "main-latest.md"))
	if err != nil {
		t.Fatal(err)
	}
	resume, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "prompts", "RESUME.md"))
	if err != nil {
		t.Fatal(err)
	}
	for label, content := range map[string]string{"handoff": string(latestHandoff), "RESUME": string(resume)} {
		for _, expected := range []string{
			"adapter context: id=dynamic-debug-or-writeback-action status=cautious entry=",
			"adapter context evidence guidance:",
			"adapter context stop conditions: timeout,unexpected-side-effect,scope-drift",
		} {
			if !strings.Contains(content, expected) {
				t.Fatalf("generic-binary-re %s durable handoff omitted adapter context %q:\n%s", label, expected, content)
			}
		}
	}
	checkpointBytes, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "lanes", "main", "checkpoints", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("generic-binary-re checkpoint did not decode: %v\n%s", err, string(checkpointBytes))
	}
	if len(checkpoint.ExecutionEvidenceReview) != 1 {
		t.Fatalf("generic-binary-re checkpoint omitted execution evidence review: %+v", checkpoint)
	}
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re checkpoint evidence", checkpoint.ExecutionEvidenceReview[0].AdapterContext, candidate.ID)
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re checkpoint summary", checkpoint.ExecutionEvidenceReviewSummary.LatestAdapterContext, candidate.ID)
	assertSnapshotEqual(t, beforeDurableFacts, snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts")))

	out.Reset()
	if err := Run([]string{"-Command", "continue", "main", "-Target", caseRoot, "-Pack", "generic-binary-re", "-Format", "json", "-Apply"}, &out); err != nil {
		t.Fatal(err)
	}
	var continueApply struct {
		Applied                        bool                                   `json:"applied"`
		RunID                          string                                 `json:"runId"`
		ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
		ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
	}
	if err := json.Unmarshal(out.Bytes(), &continueApply); err != nil {
		t.Fatalf("generic-binary-re continue apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !continueApply.Applied || continueApply.RunID == "" || len(continueApply.ExecutionEvidenceReview) != 1 {
		t.Fatalf("generic-binary-re continue apply omitted execution evidence review: %+v", continueApply)
	}
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re continue apply evidence", continueApply.ExecutionEvidenceReview[0].AdapterContext, candidate.ID)
	assertExecutionEvidenceAdapterContextSnapshot(t, "generic-binary-re continue apply summary", continueApply.ExecutionEvidenceReviewSummary.LatestAdapterContext, candidate.ID)
	continueDigest, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "runs", continueApply.RunID, "digest.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"adapter context: id=dynamic-debug-or-writeback-action status=cautious entry=",
		"adapter context report guidance:",
		"adapter context evidence guidance:",
		"adapter context stop conditions: timeout,unexpected-side-effect,scope-drift",
	} {
		if !strings.Contains(string(continueDigest), expected) {
			t.Fatalf("generic-binary-re continue digest omitted adapter context %q:\n%s", expected, string(continueDigest))
		}
	}
	assertSnapshotEqual(t, beforeDurableFacts, snapshotFiles(t, filepath.Join(caseRoot, ".rekit", "facts")))

	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("generic-binary-re adapter evidence wrote authority ledger or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("generic-binary-re adapter evidence wrote confirmed ledger or stat failed: %v", err)
	}
}

func TestRunGateAdapterReportReadOnlyPreflightFromCallerCwdBridge(t *testing.T) {
	caseRoot := fullAttachedCase(t)
	writeAuthorizedGateVisibilityFixture(t, caseRoot)
	var out bytes.Buffer
	if err := Run([]string{
		"-Command", "gate",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Apply",
		"-Action", "debug",
		"-Lane", "main",
		"-Actor", "runtime-test",
		"-Subject", "authorized debug via caller cwd bridge",
		"-TargetRef", "target-alpha",
		"-BatchId", "batch-caller-cwd-bridge",
		"-Scope", "handler only",
		"-RuntimeSeconds", "30",
		"-DiskMB", "64",
		"-Requests", "1",
		"-OutputPaths", "workspace/main/debug/session-1",
		"-StopConditions", "timeout",
		"-Format", "json",
	}, &out); err != nil {
		t.Fatal(err)
	}
	var applied struct {
		EventID string `json:"eventId"`
		Applied bool   `json:"applied"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("authorized gate apply stdout is not JSON: %v\n%s", err, out.String())
	}
	if !applied.Applied || applied.EventID == "" {
		t.Fatalf("unexpected authorized gate result: %+v", applied)
	}

	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/adapter-report.json", `{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "caller-cwd-bridge-adapter",
  "action": "debug",
  "status": "succeeded",
  "gateEventId": "`+applied.EventID+`",
  "actualBudget": {"runtimeSeconds": 20, "diskMB": 32, "requests": 1},
  "outputRefs": ["workspace/main/debug/session-1/result.json"],
  "summary": "Adapter report through facade caller cwd bridge"
}`)
	writeCaseFile(t, caseRoot, "workspace/main/debug/session-1/result.json", `{"ok":true}`)
	workspace := filepath.Join(caseRoot, "workspace", "main", "debug", "session-1")
	t.Setenv("REKIT_CALLER_CWD", workspace)

	before := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Pack", "_template", "-ExecutionReportContract", "-GateEventId", applied.EventID, "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Kind        string `json:"kind"`
		CaseRoot    string `json:"caseRoot"`
		GateEventID string `json:"gateEventId"`
		IsMutation  bool   `json:"isMutation"`
	}
	if err := json.Unmarshal(out.Bytes(), &contract); err != nil {
		t.Fatalf("caller cwd bridge contract stdout is not JSON: %v\n%s", err, out.String())
	}
	if contract.Kind != "adapter-execution-report-contract" || contract.CaseRoot != caseRoot || contract.GateEventID != applied.EventID || contract.IsMutation {
		t.Fatalf("unexpected caller cwd bridge contract: %+v", contract)
	}

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Pack", "_template", "-ValidateExecutionReport", "-GateEventId", applied.EventID, "-ExecutionReportPath", "adapter-report.json", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var validation struct {
		Kind       string `json:"kind"`
		CaseRoot   string `json:"caseRoot"`
		Valid      bool   `json:"valid"`
		IsMutation bool   `json:"isMutation"`
		Applied    bool   `json:"applied"`
		Error      string `json:"error"`
		ReportPath string `json:"reportPath"`
		Report     *struct {
			AdapterID string `json:"adapterId"`
		} `json:"report"`
	}
	if err := json.Unmarshal(out.Bytes(), &validation); err != nil {
		t.Fatalf("caller cwd bridge validation stdout is not JSON: %v\n%s", err, out.String())
	}
	if validation.Kind != "adapter-execution-report-validation" || validation.CaseRoot != caseRoot || !validation.Valid || validation.IsMutation || validation.Applied || validation.Error != "" || validation.ReportPath != "workspace/main/debug/session-1/adapter-report.json" || validation.Report == nil || validation.Report.AdapterID != "caller-cwd-bridge-adapter" {
		t.Fatalf("unexpected caller cwd bridge validation: %+v", validation)
	}
	afterValidation := snapshotFiles(t, filepath.Join(caseRoot, ".rekit"))
	assertSnapshotEqual(t, before, afterValidation)

	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Pack", "_template", "-Apply", "-GateEventId", applied.EventID, "-ExecutionReportPath", "adapter-report.json", "-Actor", "executor-1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Applied           bool   `json:"applied"`
		EventID           string `json:"eventId"`
		ExecutionEvidence struct {
			Status    string `json:"status"`
			Execution struct {
				ExecutionReportPath string `json:"executionReportPath"`
				Adapter             struct {
					AdapterID string `json:"adapterId"`
				} `json:"adapter"`
			} `json:"execution"`
		} `json:"executionEvidence"`
	}
	if err := json.Unmarshal(out.Bytes(), &evidence); err != nil {
		t.Fatalf("caller cwd bridge adapter execution evidence stdout is not JSON: %v\n%s", err, out.String())
	}
	if !evidence.Applied || evidence.EventID == "" || evidence.ExecutionEvidence.Status != "succeeded" || evidence.ExecutionEvidence.Execution.ExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || evidence.ExecutionEvidence.Execution.Adapter.AdapterID != "caller-cwd-bridge-adapter" {
		t.Fatalf("unexpected caller cwd bridge adapter execution evidence: %+v", evidence)
	}
	observations, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run([]string{"-Command", "gate", "-Pack", "_template", "-Apply", "-GateEventId", applied.EventID, "-ExecutionReportPath", "adapter-report.json", "-Actor", "executor-1", "-Format", "json"}, &out); err != nil {
		t.Fatal(err)
	}
	var replay struct {
		Applied bool   `json:"applied"`
		EventID string `json:"eventId"`
		Reason  string `json:"reason"`
	}
	if err := json.Unmarshal(out.Bytes(), &replay); err != nil {
		t.Fatalf("caller cwd bridge adapter replay stdout is not JSON: %v\n%s", err, out.String())
	}
	if replay.Applied || replay.EventID != evidence.EventID || replay.Reason != "duplicate eventId" {
		t.Fatalf("caller cwd bridge adapter replay should be idempotent: first=%+v replay=%+v", evidence, replay)
	}
	replayedObservations, err := os.ReadFile(filepath.Join(caseRoot, ".rekit", "facts", "observations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(replayedObservations) != string(observations) {
		t.Fatalf("caller cwd bridge adapter replay appended observations:\nbefore=%s\nafter=%s", string(observations), string(replayedObservations))
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("caller cwd bridge adapter replay wrote authority ledger or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(caseRoot, ".rekit", "facts", "confirmed.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("caller cwd bridge adapter evidence wrote confirmed ledger or stat failed: %v", err)
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
	Command                       string                                 `json:"command"`
	IsMutation                    bool                                   `json:"isMutation"`
	Applied                       bool                                   `json:"applied"`
	Lane                          startLane                              `json:"lane"`
	MissionBrief                  missionBrief                           `json:"missionBrief"`
	AuthorizedGateAdapterHandoffs []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
	ExecutorAction                executorActionSnapshot                 `json:"executorAction"`
	MissionCommanderAction        missionCommanderActionSnapshot         `json:"missionCommanderAction"`
	MissionCommanderNextActions   []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue   missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
	Writes                        []startWrite                           `json:"writes"`
	NextSteps                     []string                               `json:"nextSteps"`
}

type handoffResult struct {
	Command                        string                                 `json:"command"`
	Pack                           string                                 `json:"pack"`
	IsMutation                     bool                                   `json:"isMutation"`
	Applied                        bool                                   `json:"applied"`
	RequiresConfirmation           bool                                   `json:"requiresConfirmation"`
	Project                        bool                                   `json:"project"`
	Lane                           *startLane                             `json:"lane"`
	MissionBrief                   missionBrief                           `json:"missionBrief"`
	ExecutorAction                 *executorActionSnapshot                `json:"executorAction"`
	LaneExecutorActions            []handoffLaneExecutorAction            `json:"laneExecutorActions"`
	ExecutionEvidenceReview        []executionEvidenceReviewItem          `json:"executionEvidenceReview"`
	ExecutionEvidenceReviewSummary executionEvidenceReviewSummarySnapshot `json:"executionEvidenceReviewSummary"`
	ReviewerWritebacks             []reviewerWritebackCLIItem             `json:"reviewerWritebacks"`
	ReviewerWritebackSummary       reviewerWritebackSummaryCLIItem        `json:"reviewerWritebackSummary"`
	ReviewerDispatchIntakeHandoffs []reviewerDispatchIntakeCLIItem        `json:"reviewerDispatchIntakeHandoffs"`
	ReviewerDispatchIntakeSummary  reviewerDispatchIntakeSummaryCLIItem   `json:"reviewerDispatchIntakeSummary"`
	AuthorizedGateAdapterHandoffs  []authorizedGateAdapterHandoffSnapshot `json:"authorizedGateAdapterHandoffs"`
	MissionCommanderNextActions    []missionCommanderNextActionItem       `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue    missionCommanderActionQueueSnapshot    `json:"missionCommanderActionQueue"`
	Writes                         []startWrite                           `json:"writes"`
	NextSteps                      []string                               `json:"nextSteps"`
}

type handoffLaneExecutorAction struct {
	Lane               string                 `json:"lane"`
	Label              string                 `json:"label"`
	Status             string                 `json:"status"`
	Workspace          string                 `json:"workspace"`
	CurrentExecutor    string                 `json:"currentExecutor"`
	ExecutorGeneration int                    `json:"executorGeneration"`
	LastTakeoverAt     string                 `json:"lastTakeoverAt"`
	LastTakeoverBy     string                 `json:"lastTakeoverBy"`
	LastTakeoverReason string                 `json:"lastTakeoverReason"`
	ExecutorAction     executorActionSnapshot `json:"executorAction"`
}

type authorizedGateAdapterHandoffSnapshot struct {
	EventID             string                                `json:"eventId"`
	Lane                string                                `json:"lane"`
	Subject             string                                `json:"subject"`
	Action              string                                `json:"action"`
	Target              string                                `json:"target"`
	Status              string                                `json:"status"`
	Risk                string                                `json:"risk"`
	Authorization       string                                `json:"authorization"`
	Profile             string                                `json:"profile"`
	ReportContract      string                                `json:"reportContract"`
	DefaultReportPath   string                                `json:"defaultReportPath"`
	ReportPath          string                                `json:"reportPath"`
	ReportSummary       *adapterReportHandoffSummarySnapshot  `json:"reportSummary"`
	LiveValidation      *authorizedGateLiveValidationSnapshot `json:"liveValidation"`
	ReportContractError string                                `json:"reportContractError"`
	HandoffCommand      string                                `json:"handoffCommand"`
	Boundary            []string                              `json:"boundary"`
	Evidence            []string                              `json:"evidence"`
}

type authorizedGateLiveValidationSnapshot struct {
	InvocationCwd               string                        `json:"invocationCwd"`
	AuthorizedWorkspaces        []string                      `json:"authorizedWorkspaces"`
	ReportFileName              string                        `json:"reportFileName"`
	CaseRelativeReportPath      string                        `json:"caseRelativeReportPath"`
	ValidateCommand             string                        `json:"validateCommand"`
	RecordCommand               string                        `json:"recordCommand"`
	CaseRelativeValidateCommand string                        `json:"caseRelativeValidateCommand"`
	CaseRelativeRecordCommand   string                        `json:"caseRelativeRecordCommand"`
	AdapterCandidateCount       int                           `json:"adapterCandidateCount"`
	SelectedAdapterID           string                        `json:"selectedAdapterId"`
	SelectedAdapter             *adapterToolCandidateSnapshot `json:"selectedAdapter"`
	SidecarTemplateAdapterID    string                        `json:"sidecarTemplateAdapterId"`
	ReplayBehavior              string                        `json:"replayBehavior"`
}

type adapterToolCandidateSnapshot struct {
	ID                  string   `json:"id"`
	Status              string   `json:"status"`
	Entry               string   `json:"entry"`
	Purpose             string   `json:"purpose"`
	SideEffects         []string `json:"sideEffects"`
	GateActions         []string `json:"gateActions"`
	ToolingCatalogPath  string   `json:"toolingCatalogPath"`
	ReportGuidance      []string `json:"reportGuidance"`
	EvidenceGuidance    []string `json:"evidenceGuidance"`
	StopConditionHints  []string `json:"stopConditionHints"`
	RecordOnlyAfterGate bool     `json:"recordOnlyAfterGate"`
}

func assertExecutionEvidenceAdapterContextSnapshot(t *testing.T, label string, candidate *adapterToolCandidateSnapshot, wantID string) {
	t.Helper()
	if candidate == nil || candidate.ID != wantID || candidate.Status != "cautious" || !strings.Contains(candidate.Entry, "dynamic-debug") || !strings.Contains(candidate.Purpose, "bounded debug") || !containsSubstring(candidate.SideEffects, "process execution") || !slices.Contains(candidate.GateActions, "debug") || candidate.ToolingCatalogPath != "tooling/catalog.yml" || !containsSubstring(candidate.ReportGuidance, "adapterId") || !containsSubstring(candidate.EvidenceGuidance, "ValidateArgs") || strings.Join(candidate.StopConditionHints, ",") != "timeout,unexpected-side-effect,scope-drift" || !candidate.RecordOnlyAfterGate {
		t.Fatalf("%s omitted execution evidence adapter context detail: %+v", label, candidate)
	}
}

func assertAuthorizedGateAdapterHandoffSnapshot(t *testing.T, label string, items []authorizedGateAdapterHandoffSnapshot, eventID, wantContract string) {
	t.Helper()
	wantReportPath := "workspace/main/debug/session-1/adapter-report.json"
	if len(items) != 1 {
		t.Fatalf("%s authorized gate adapter handoff count = %d, want 1: %+v", label, len(items), items)
	}
	item := items[0]
	if item.EventID != eventID || item.Lane != "main" || item.Subject != "authorized debug" || item.Action != "debug" || item.Target != "target-alpha" || item.Status != "authorized-gate" || item.Authorization != "preauthorized" || item.Profile != "prof-main-debug" || item.ReportContract != wantContract || item.DefaultReportPath != wantReportPath || item.ReportPath != wantReportPath || item.HandoffCommand != "/rekit handoff main" {
		t.Fatalf("%s authorized gate adapter handoff identity drifted: %+v", label, item)
	}
	if item.ReportSummary == nil || item.ReportSummary.ReportPath != wantReportPath || item.ReportSummary.DefaultReportPath != wantReportPath || item.ReportSummary.AllowedStatusCount != 5 || item.ReportSummary.AllowedOutputPathCount != 1 || item.ReportSummary.AuthorizedStopCount != 1 || item.ReportSummary.OutcomeCount == 0 {
		t.Fatalf("%s authorized gate adapter handoff missing compact report summary: %+v", label, item.ReportSummary)
	}
	if item.LiveValidation == nil || item.LiveValidation.ReportFileName != "adapter-report.json" || item.LiveValidation.CaseRelativeReportPath != wantReportPath || item.LiveValidation.AdapterCandidateCount != 0 || item.LiveValidation.SidecarTemplateAdapterID != "<adapter-id>" || !strings.Contains(item.LiveValidation.CaseRelativeValidateCommand, wantReportPath) || !strings.Contains(item.LiveValidation.CaseRelativeRecordCommand, wantReportPath) {
		t.Fatalf("%s authorized gate adapter handoff missing live validation: %+v", label, item.LiveValidation)
	}
	if !containsSubstring(item.LiveValidation.AuthorizedWorkspaces, "workspace/main/debug/session-1") || !containsSubstring(item.Evidence, "authorized outputPaths workspace/main/debug/session-1") || !containsSubstring(item.Evidence, "authorized stopConditions timeout") || !containsSubstring(item.Boundary, "projection may read and validate an existing canonical sidecar") || !containsSubstring(item.Boundary, "no heavy-tool replay") {
		t.Fatalf("%s authorized gate adapter handoff missing workspace/evidence/boundary: %+v", label, item)
	}
}

type reviewerDispatchIntakeCLIItem struct {
	PacketID                         string   `json:"packetId"`
	PacketPath                       string   `json:"packetPath"`
	SummaryPath                      string   `json:"summaryPath"`
	ResultRoot                       string   `json:"resultRoot"`
	TargetLane                       string   `json:"targetLane"`
	ShardID                          string   `json:"shardId"`
	DispatchIndex                    int      `json:"dispatchIndex"`
	DispatchTotal                    int      `json:"dispatchTotal"`
	DispatchCompleted                int      `json:"dispatchCompleted"`
	DispatchOpen                     int      `json:"dispatchOpen"`
	DispatchWaitingForReviewerResult int      `json:"dispatchWaitingForReviewerResult"`
	DispatchReadyForPreview          int      `json:"dispatchReadyForPreview"`
	DispatchAttachRequired           int      `json:"dispatchAttachRequired"`
	DispatchOnlyOpen                 int      `json:"dispatchOnlyOpen"`
	LatestCompletedShardID           string   `json:"latestCompletedShardId"`
	NextOpenShardID                  string   `json:"nextOpenShardId"`
	RemainingShardIDs                []string `json:"remainingShardIds"`
	State                            string   `json:"state"`
	ReviewerResultPath               string   `json:"reviewerResultPath"`
	ReviewerResultPresent            bool     `json:"reviewerResultPresent"`
	IntakeAvailable                  bool     `json:"intakeAvailable"`
	DispatchOnly                     bool     `json:"dispatchOnly"`
	VerificationRecorded             bool     `json:"verificationRecorded"`
	DecisionRecorded                 bool     `json:"decisionRecorded"`
	DispatchCommand                  string   `json:"dispatchCommand"`
	PreviewCommand                   string   `json:"previewCommand"`
	ApplyCommand                     string   `json:"applyCommand"`
	BatchPreviewCommand              string   `json:"batchPreviewCommand"`
	BatchApplyCommand                string   `json:"batchApplyCommand"`
	OwnerExecutor                    string   `json:"ownerExecutor"`
	OwnerGeneration                  int      `json:"ownerGeneration"`
	OwnerBindingMode                 string   `json:"ownerBindingMode"`
	Evidence                         []string `json:"evidence"`
	Boundary                         []string `json:"boundary"`
}

type reviewerDispatchIntakeSummaryCLIItem struct {
	Total                         int      `json:"total"`
	WaitingForReviewerResult      int      `json:"waitingForReviewerResult"`
	ReadyForPreview               int      `json:"readyForPreview"`
	AttachRequired                int      `json:"attachRequired"`
	DispatchOnly                  int      `json:"dispatchOnly"`
	LaneCount                     int      `json:"laneCount"`
	Lanes                         []string `json:"lanes"`
	PacketCount                   int      `json:"packetCount"`
	LatestPacketDispatchTotal     int      `json:"latestPacketDispatchTotal"`
	LatestPacketDispatchCompleted int      `json:"latestPacketDispatchCompleted"`
	LatestPacketDispatchOpen      int      `json:"latestPacketDispatchOpen"`
	LatestPacketNextOpenShardID   string   `json:"latestPacketNextOpenShardId"`
	LatestCompletedShardID        string   `json:"latestCompletedShardId"`
	RemainingShardIDs             []string `json:"remainingShardIds"`
	LatestPacketPath              string   `json:"latestPacketPath"`
	LatestShardID                 string   `json:"latestShardId"`
	LatestState                   string   `json:"latestState"`
	LatestReviewerResultPath      string   `json:"latestReviewerResultPath"`
	LatestPreviewCommand          string   `json:"latestPreviewCommand"`
	LatestApplyCommand            string   `json:"latestApplyCommand"`
	LatestBatchPreviewCommand     string   `json:"latestBatchPreviewCommand"`
	LatestBatchApplyCommand       string   `json:"latestBatchApplyCommand"`
	NextAction                    string   `json:"nextAction"`
	Boundary                      []string `json:"boundary"`
}

func assertReviewerDispatchIntakeSummary(t *testing.T, label string, summary reviewerDispatchIntakeSummaryCLIItem, total, waiting, ready int, latestShard, latestState string) {
	t.Helper()
	nextActionMatches := strings.Contains(summary.NextAction, latestShard) || (ready > 0 && strings.Contains(summary.NextAction, "-ReadyReviewerResults"))
	if summary.Total != total || summary.WaitingForReviewerResult != waiting || summary.ReadyForPreview != ready || summary.LatestShardID != latestShard || summary.LatestState != latestState || !containsSubstring(summary.Boundary, "summary is read-only") || !nextActionMatches {
		t.Fatalf("%s missing reviewer dispatch intake summary: %+v", label, summary)
	}
	if total > 0 && (summary.PacketCount == 0 || summary.LatestPacketDispatchTotal == 0 || summary.LatestPacketDispatchOpen == 0 || len(summary.RemainingShardIDs) == 0) {
		t.Fatalf("%s missing reviewer dispatch intake progress summary: %+v", label, summary)
	}
}

func assertReviewerIntakeOrchestrationProgress(t *testing.T, label string, progress *reviewerIntakeOrchestrationProgressItem, dispatchIndex, dispatchTotal, completed, open int, currentShardID, currentStatus, nextOpenShardID string, remainingShardIDs []string) {
	t.Helper()
	if progress == nil {
		t.Fatalf("%s omitted reviewer intake orchestration progress", label)
	}
	if progress.DispatchIndex != dispatchIndex || progress.DispatchTotal != dispatchTotal || progress.Completed != completed || progress.Open != open || progress.CurrentShardID != currentShardID || progress.CurrentShardStatus != currentStatus || progress.NextOpenShardID != nextOpenShardID || !slices.Equal(progress.RemainingShardIDs, remainingShardIDs) || !containsStringWith(progress.Boundary, "orchestration progress is read-only") {
		t.Fatalf("%s missing reviewer intake orchestration progress: %+v", label, progress)
	}
}

func reviewerDispatchIntakeByShard(items []reviewerDispatchIntakeCLIItem, shard string) (reviewerDispatchIntakeCLIItem, bool) {
	for _, item := range items {
		if item.ShardID == shard {
			return item, true
		}
	}
	return reviewerDispatchIntakeCLIItem{}, false
}

type reviewerWritebackCLIItem struct {
	Kind               string            `json:"kind"`
	EventID            string            `json:"eventId"`
	Lane               string            `json:"lane"`
	PacketID           string            `json:"packetId"`
	RouteID            string            `json:"routeId"`
	ShardID            string            `json:"shardId"`
	ReviewerSession    string            `json:"reviewerSession"`
	ReviewerResultPath string            `json:"reviewerResultPath"`
	OwnerBindingTarget string            `json:"ownerBindingTarget"`
	OwnerBindingMode   string            `json:"ownerBindingMode"`
	ReviewerDecision   string            `json:"reviewerDecision"`
	RecommendedVerdict string            `json:"recommendedVerdict"`
	ReviewerRisks      []string          `json:"reviewerRisks"`
	ReviewerConflicts  []string          `json:"reviewerConflicts"`
	RouteOutput        map[string]string `json:"routeOutput"`
	EvidenceRefs       []string          `json:"evidenceRefs"`
}

type reviewerWritebackSummaryCLIItem struct {
	Total                    int      `json:"total"`
	VerificationCount        int      `json:"verificationCount"`
	DecisionCount            int      `json:"decisionCount"`
	LaneCount                int      `json:"laneCount"`
	Lanes                    []string `json:"lanes"`
	LatestKind               string   `json:"latestKind"`
	LatestEventID            string   `json:"latestEventId"`
	LatestLane               string   `json:"latestLane"`
	LatestShardID            string   `json:"latestShardId"`
	LatestReviewerSession    string   `json:"latestReviewerSession"`
	LatestReviewerResult     string   `json:"latestReviewerResult"`
	LatestPacketID           string   `json:"latestPacketId"`
	LatestRouteID            string   `json:"latestRouteId"`
	LatestReviewerDecision   string   `json:"latestReviewerDecision"`
	LatestRecommendedVerdict string   `json:"latestRecommendedVerdict"`
	LatestEvidenceRefs       []string `json:"latestEvidenceRefs"`
	HasReviewerResult        bool     `json:"hasReviewerResult"`
	HasOwnerBinding          bool     `json:"hasOwnerBinding"`
	HasRisks                 bool     `json:"hasRisks"`
	HasConflicts             bool     `json:"hasConflicts"`
	HasRouteOutput           bool     `json:"hasRouteOutput"`
	Boundary                 []string `json:"boundary"`
}

type executionEvidenceReviewSummarySnapshot struct {
	Total                     int                           `json:"total"`
	ReadyForReviewCount       int                           `json:"readyForReviewCount"`
	MainEscalationCount       int                           `json:"mainEscalationCount"`
	DuplicateCount            int                           `json:"duplicateCount"`
	OutputRefCount            int                           `json:"outputRefCount"`
	EvidenceRefCount          int                           `json:"evidenceRefCount"`
	BoundaryHitCount          int                           `json:"boundaryHitCount"`
	HasEscalation             bool                          `json:"hasEscalation"`
	HasExecutionReport        bool                          `json:"hasExecutionReport"`
	HasAdapter                bool                          `json:"hasAdapter"`
	LatestEventID             string                        `json:"latestEventId"`
	LatestGateEventID         string                        `json:"latestGateEventId"`
	LatestStatus              string                        `json:"latestStatus"`
	LatestAction              string                        `json:"latestAction"`
	LatestTarget              string                        `json:"latestTarget"`
	LatestReviewCommand       string                        `json:"latestReviewCommand"`
	LatestHandoffCommand      string                        `json:"latestHandoffCommand"`
	LatestCommanderState      string                        `json:"latestCommanderState"`
	LatestCommanderPrimary    string                        `json:"latestCommanderPrimary"`
	LatestExecutionReportPath string                        `json:"latestExecutionReportPath"`
	LatestAdapterID           string                        `json:"latestAdapterId"`
	LatestAdapterStatus       string                        `json:"latestAdapterStatus"`
	LatestAdapterContext      *adapterToolCandidateSnapshot `json:"latestAdapterContext"`
	LatestBoundaryHits        []string                      `json:"latestBoundaryHits"`
	LatestEscalation          string                        `json:"latestEscalation"`
	OutcomeCount              int                           `json:"outcomeCount"`
	FollowThroughState        string                        `json:"followThroughState"`
	ActionQueueSummary        string                        `json:"actionQueueSummary"`
	CurrentAction             string                        `json:"currentAction"`
	NextActionCount           int                           `json:"nextActionCount"`
	ReviewRequiredActionCount int                           `json:"reviewRequiredActionCount"`
	Boundary                  []string                      `json:"boundary"`
}

func assertNestedAdapterExecutionEvidenceReviewSummary(t *testing.T, label string, summary executionEvidenceReviewSummarySnapshot, eventID, gateEventID string) {
	t.Helper()
	if summary.Total != 1 || summary.ReadyForReviewCount != 1 || summary.MainEscalationCount != 0 || summary.DuplicateCount != 0 || summary.OutputRefCount != 1 || summary.EvidenceRefCount != 0 || summary.BoundaryHitCount != 0 || summary.HasEscalation || !summary.HasExecutionReport || !summary.HasAdapter || summary.LatestEventID != eventID || summary.LatestGateEventID != gateEventID || summary.LatestStatus != "succeeded" || summary.LatestAction != "debug" || summary.LatestTarget != "target-alpha" || summary.LatestReviewCommand != "review outputRefs/evidenceRefs for gateEventId "+gateEventID || summary.LatestHandoffCommand != "/rekit handoff main" || summary.LatestCommanderState != "ready-for-evidence-review" || summary.LatestCommanderPrimary != "/rekit handoff main" || summary.LatestExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || summary.LatestAdapterID != "nested-cli-adapter" || summary.LatestAdapterStatus != "succeeded" || len(summary.LatestBoundaryHits) != 0 || summary.LatestEscalation != "" || summary.OutcomeCount != 1 || summary.FollowThroughState != "ready-for-evidence-review" || summary.CurrentAction != "/rekit handoff main" || summary.NextActionCount == 0 || summary.ReviewRequiredActionCount == 0 || !strings.Contains(summary.ActionQueueSummary, "current=/rekit handoff main") || !containsSubstring(summary.Boundary, "summary is read-only") || !containsSubstring(summary.Boundary, "no authority/confirmed") {
		t.Fatalf("%s missing nested adapter execution evidence review summary: %+v", label, summary)
	}
}

func assertEscalatedAdapterExecutionEvidenceReviewSummary(t *testing.T, label string, summary executionEvidenceReviewSummarySnapshot, eventID, gateEventID string, total, ready, outputRefs, evidenceRefs, outcomes int) {
	t.Helper()
	if summary.Total != total || summary.ReadyForReviewCount != ready || summary.MainEscalationCount != 1 || summary.DuplicateCount != 0 || summary.OutputRefCount != outputRefs || summary.EvidenceRefCount != evidenceRefs || summary.BoundaryHitCount != 0 || !summary.HasEscalation || !summary.HasExecutionReport || !summary.HasAdapter || summary.LatestEventID != eventID || summary.LatestGateEventID != gateEventID || summary.LatestStatus != "escalated" || summary.LatestAction != "debug" || summary.LatestTarget != "target-alpha" || summary.LatestReviewCommand != "review outputRefs/evidenceRefs for gateEventId "+gateEventID || summary.LatestHandoffCommand != "/rekit handoff main" || summary.LatestCommanderState != "needs-main-escalation" || summary.LatestCommanderPrimary != "/rekit handoff main" || summary.LatestExecutionReportPath != "workspace/main/debug/session-1/adapter-escalation.json" || summary.LatestAdapterID != "cli-adapter" || summary.LatestAdapterStatus != "escalated" || summary.LatestEscalation != "adapter escalated from CLI E2E" || summary.OutcomeCount != outcomes || summary.FollowThroughState != "needs-main-escalation" || summary.CurrentAction != "/rekit handoff main" || summary.NextActionCount != 2 || summary.ReviewRequiredActionCount != 2 || !strings.Contains(summary.ActionQueueSummary, "total=2") || !containsSubstring(summary.Boundary, "summary is read-only") || !containsSubstring(summary.Boundary, "no authority/confirmed") {
		t.Fatalf("%s missing escalated adapter execution evidence review summary: %+v", label, summary)
	}
}

func assertBoundaryHitAdapterExecutionEvidenceReviewSummary(t *testing.T, label string, summary executionEvidenceReviewSummarySnapshot, eventID, gateEventID string) {
	t.Helper()
	if summary.Total != 1 || summary.ReadyForReviewCount != 0 || summary.MainEscalationCount != 1 || summary.DuplicateCount != 0 || summary.OutputRefCount != 1 || summary.EvidenceRefCount != 0 || summary.BoundaryHitCount != 1 || summary.HasEscalation || !summary.HasExecutionReport || !summary.HasAdapter || summary.LatestEventID != eventID || summary.LatestGateEventID != gateEventID || summary.LatestStatus != "boundary-hit" || summary.LatestAction != "debug" || summary.LatestTarget != "target-alpha" || summary.LatestExecutionReportPath != "workspace/main/debug/session-1/adapter-report.json" || summary.LatestAdapterID != "boundary-cli-adapter" || summary.LatestAdapterStatus != "boundary-hit" || !containsSubstring(summary.LatestBoundaryHits, "timeout") || summary.OutcomeCount != 1 || summary.FollowThroughState != "needs-main-escalation" || summary.CurrentAction != "/rekit handoff main" || summary.NextActionCount != 2 || summary.ReviewRequiredActionCount != 2 || !strings.Contains(summary.ActionQueueSummary, "total=2") || !containsSubstring(summary.Boundary, "summary is read-only") || !containsSubstring(summary.Boundary, "no authority/confirmed") {
		t.Fatalf("%s missing boundary-hit adapter execution evidence review summary: %+v", label, summary)
	}
}

type executionEvidenceReviewItem struct {
	EventID                string                                 `json:"eventId"`
	GateEventID            string                                 `json:"gateEventId"`
	Subject                string                                 `json:"subject"`
	Status                 string                                 `json:"status"`
	Action                 string                                 `json:"action"`
	OutputRefs             []string                               `json:"outputRefs"`
	EvidenceRefs           []string                               `json:"evidenceRefs"`
	ExecutionReportPath    string                                 `json:"executionReportPath"`
	ActualBudget           *executionEvidenceBudgetSnapshot       `json:"actualBudget"`
	AdapterID              string                                 `json:"adapterId"`
	AdapterStatus          string                                 `json:"adapterStatus"`
	AdapterContext         *adapterToolCandidateSnapshot          `json:"adapterContext"`
	BoundaryHits           []string                               `json:"boundaryHits"`
	Escalation             string                                 `json:"escalation"`
	FollowThrough          executionEvidenceFollowThroughSnapshot `json:"followThrough"`
	ReviewCommand          string                                 `json:"reviewCommand"`
	HandoffCommand         string                                 `json:"handoffCommand"`
	Boundary               []string                               `json:"boundary"`
	MissionCommanderAction missionCommanderActionSnapshot         `json:"missionCommanderAction"`
}

type executionEvidenceBudgetSnapshot struct {
	RuntimeSeconds int `json:"runtimeSeconds"`
	DiskMB         int `json:"diskMB"`
	Requests       int `json:"requests"`
}

type executionEvidenceFollowThroughSnapshot struct {
	State       string                              `json:"state"`
	GateEventID string                              `json:"gateEventId"`
	Outcomes    []executionEvidenceOutcomeSnapshot  `json:"outcomes"`
	Boundary    []string                            `json:"boundary"`
	ActionQueue missionCommanderActionQueueSnapshot `json:"actionQueue"`
}

type executionEvidenceOutcomeSnapshot struct {
	Name                 string   `json:"name"`
	State                string   `json:"state"`
	When                 string   `json:"when"`
	Command              string   `json:"command"`
	Actions              []string `json:"actions"`
	VerificationCommands []string `json:"verificationCommands"`
	Expected             string   `json:"expected"`
	Evidence             []string `json:"evidence"`
	Boundary             []string `json:"boundary"`
}

type executorActionSnapshot struct {
	Blocked                bool                           `json:"blocked"`
	Ready                  bool                           `json:"ready"`
	BlockerReasons         []string                       `json:"blockerReasons"`
	PendingGates           int                            `json:"pendingGates"`
	OpenInterventions      int                            `json:"openInterventions"`
	OpenDecisions          int                            `json:"openDecisions"`
	ReconcileRequired      bool                           `json:"reconcileRequired"`
	PendingGateRequired    bool                           `json:"pendingGateRequired"`
	OpenDecisionRequired   bool                           `json:"openDecisionRequired"`
	ResumeCommand          string                         `json:"resumeCommand"`
	HandoffCommand         string                         `json:"handoffCommand"`
	NextAgentActions       []string                       `json:"nextAgentActions"`
	Escalations            []string                       `json:"escalations"`
	MissionCommanderAction missionCommanderActionSnapshot `json:"missionCommanderAction"`
}

type missionCommanderActionSnapshot struct {
	State            string   `json:"state"`
	Prompt           string   `json:"prompt"`
	PrimaryCommand   string   `json:"primaryCommand"`
	FollowUpCommands []string `json:"followUpCommands"`
	Boundary         []string `json:"boundary"`
}

type missionCommanderActionItem struct {
	Lane             string                         `json:"lane"`
	Label            string                         `json:"label"`
	Status           string                         `json:"status"`
	Blocked          bool                           `json:"blocked"`
	Ready            bool                           `json:"ready"`
	BlockerReasons   []string                       `json:"blockerReasons"`
	PrimaryCommand   string                         `json:"primaryCommand"`
	FollowUpCommands []string                       `json:"followUpCommands"`
	Boundary         []string                       `json:"boundary"`
	Action           missionCommanderActionSnapshot `json:"action"`
}

type missionCommanderNextActionItem struct {
	Lane           string   `json:"lane"`
	Label          string   `json:"label"`
	GateEventID    string   `json:"gateEventId"`
	State          string   `json:"state"`
	Command        string   `json:"command"`
	Source         string   `json:"source"`
	Blocked        bool     `json:"blocked"`
	RequiresReview bool     `json:"requiresReview"`
	Reasons        []string `json:"reasons"`
	Boundary       []string `json:"boundary"`
}

type missionCommanderActionQueueSnapshot struct {
	Summary string `json:"summary"`
	Counts  struct {
		Total          int `json:"total"`
		Unblocked      int `json:"unblocked"`
		Blocked        int `json:"blocked"`
		RequiresReview int `json:"requiresReview"`
		FollowUp       int `json:"followUp"`
	} `json:"counts"`
	CurrentAction         *missionCommanderNextActionItem  `json:"currentAction"`
	UnblockedActions      []missionCommanderNextActionItem `json:"unblockedActions"`
	BlockedActions        []missionCommanderNextActionItem `json:"blockedActions"`
	ReviewRequiredActions []missionCommanderNextActionItem `json:"reviewRequiredActions"`
	FollowUpActions       []missionCommanderNextActionItem `json:"followUpActions"`
}

type authorizedExecutionFollowThroughSnapshot struct {
	State       string                              `json:"state"`
	GateEventID string                              `json:"gateEventId"`
	ReportPath  string                              `json:"reportPath"`
	Outcomes    []authorizedExecutionOutcome        `json:"outcomes"`
	Boundary    []string                            `json:"boundary"`
	ActionQueue missionCommanderActionQueueSnapshot `json:"actionQueue"`
}

type adapterReportHandoffSummarySnapshot struct {
	State                     string   `json:"state"`
	GateEventID               string   `json:"gateEventId"`
	Action                    string   `json:"action"`
	Lane                      string   `json:"lane"`
	ReportPath                string   `json:"reportPath"`
	DefaultReportPath         string   `json:"defaultReportPath"`
	ReportPresent             bool     `json:"reportPresent"`
	Valid                     bool     `json:"valid"`
	RecordReady               bool     `json:"recordReady"`
	RecordBlocked             bool     `json:"recordBlocked"`
	RequiresValidation        bool     `json:"requiresValidation"`
	RequiresRepair            bool     `json:"requiresRepair"`
	RequiresMainEscalation    bool     `json:"requiresMainEscalation"`
	AllowedStatusCount        int      `json:"allowedStatusCount"`
	AllowedOutputPathCount    int      `json:"allowedOutputPathCount"`
	AuthorizedStopCount       int      `json:"authorizedStopCount"`
	AdapterCandidateCount     int      `json:"adapterCandidateCount"`
	RepairHintCount           int      `json:"repairHintCount"`
	RecordBlockedHintCount    int      `json:"recordBlockedHintCount"`
	EscalateHintCount         int      `json:"escalateHintCount"`
	OutcomeCount              int      `json:"outcomeCount"`
	NextActionCount           int      `json:"nextActionCount"`
	ReviewRequiredActionCount int      `json:"reviewRequiredActionCount"`
	ActionQueueSummary        string   `json:"actionQueueSummary"`
	CurrentAction             string   `json:"currentAction"`
	ReportStatus              string   `json:"reportStatus"`
	AdapterID                 string   `json:"adapterId"`
	ActualRuntimeSeconds      int      `json:"actualRuntimeSeconds"`
	ActualDiskMB              int      `json:"actualDiskMB"`
	ActualRequests            int      `json:"actualRequests"`
	OutputRefCount            int      `json:"outputRefCount"`
	EvidenceRefCount          int      `json:"evidenceRefCount"`
	BoundaryHitCount          int      `json:"boundaryHitCount"`
	HasEscalation             bool     `json:"hasEscalation"`
	HasSummary                bool     `json:"hasSummary"`
	ValidationFailureCode     string   `json:"validationFailureCode"`
	ValidationFailureStage    string   `json:"validationFailureStage"`
	Boundary                  []string `json:"boundary"`
}

type authorizedExecutionOutcome struct {
	Name                 string   `json:"name"`
	State                string   `json:"state"`
	When                 string   `json:"when"`
	Command              string   `json:"command"`
	Actions              []string `json:"actions"`
	RepairActions        []string `json:"repairActions"`
	VerificationCommands []string `json:"verificationCommands"`
	Expected             string   `json:"expected"`
	Evidence             []string `json:"evidence"`
	Boundary             []string `json:"boundary"`
}

func assertCLIActionQueue(t *testing.T, queue missionCommanderActionQueueSnapshot, total, unblocked, blocked, requiresReview, followUp int, currentCommand string) {
	t.Helper()
	if queue.Counts.Total != total || queue.Counts.Unblocked != unblocked || queue.Counts.Blocked != blocked || queue.Counts.RequiresReview != requiresReview || queue.Counts.FollowUp != followUp || queue.CurrentAction == nil || queue.CurrentAction.Command != currentCommand || !strings.Contains(queue.Summary, "current="+currentCommand) {
		t.Fatalf("Mission Commander action queue drifted: %+v", queue)
	}
}

func cliExecutionEvidenceFollowThroughContains(follow executionEvidenceFollowThroughSnapshot, outcomeName, want string) bool {
	fields := append([]string{follow.State, follow.GateEventID, follow.ActionQueue.Summary}, follow.Boundary...)
	for _, outcome := range follow.Outcomes {
		if outcome.Name != outcomeName {
			continue
		}
		fields = append(fields, outcome.Name, outcome.State, outcome.When, outcome.Command, outcome.Expected)
		fields = append(fields, outcome.Actions...)
		fields = append(fields, outcome.VerificationCommands...)
		fields = append(fields, outcome.Evidence...)
		fields = append(fields, outcome.Boundary...)
	}
	return containsSubstring(fields, want)
}

func cliAuthorizedFollowThroughContains(follow authorizedExecutionFollowThroughSnapshot, outcomeName, want string) bool {
	fields := append([]string{follow.State, follow.GateEventID, follow.ReportPath, follow.ActionQueue.Summary}, follow.Boundary...)
	for _, outcome := range follow.Outcomes {
		if outcome.Name != outcomeName {
			continue
		}
		fields = append(fields, outcome.Name, outcome.State, outcome.When, outcome.Command, outcome.Expected)
		fields = append(fields, outcome.Actions...)
		fields = append(fields, outcome.RepairActions...)
		fields = append(fields, outcome.VerificationCommands...)
		fields = append(fields, outcome.Evidence...)
		fields = append(fields, outcome.Boundary...)
	}
	return containsSubstring(fields, want)
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
	Command                     string                              `json:"command"`
	PlanRoot                    string                              `json:"planRoot"`
	Pack                        string                              `json:"pack"`
	IsMutation                  bool                                `json:"isMutation"`
	WritesReviewArtifacts       bool                                `json:"writesReviewArtifacts"`
	ReviewRequired              bool                                `json:"reviewRequired"`
	ReviewRoot                  string                              `json:"reviewRoot"`
	PacketPath                  string                              `json:"packetPath"`
	SummaryPath                 string                              `json:"summaryPath"`
	ItemCount                   int                                 `json:"itemCount"`
	ShardCount                  int                                 `json:"shardCount"`
	TargetLane                  string                              `json:"targetLane"`
	OwnerBinding                planSubagentsOwnerBinding           `json:"ownerBinding"`
	ReviewerOrchestration       planSubagentsOrchestration          `json:"reviewerOrchestration"`
	ShardHandoffs               []planSubagentsHandoff              `json:"shardHandoffs"`
	Observability               planSubagentsObservables            `json:"observability"`
	ReviewLoop                  planSubagentsReviewLoop             `json:"reviewLoop"`
	MissionCommanderAction      missionCommanderActionSnapshot      `json:"missionCommanderAction"`
	MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
}

type planSubagentsOwnerBinding struct {
	TargetLane         string `json:"targetLane"`
	CurrentExecutor    string `json:"currentExecutor"`
	ExecutorGeneration int    `json:"executorGeneration"`
	BindingMode        string `json:"bindingMode"`
	RequiredForIntake  bool   `json:"requiredForIntake"`
}

type planSubagentsPacket struct {
	PacketID     string                    `json:"packetId"`
	Command      string                    `json:"command"`
	TargetLane   string                    `json:"targetLane"`
	OwnerBinding planSubagentsOwnerBinding `json:"ownerBinding"`
	Route        struct {
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
		Items  []string `json:"items"`
		Prompt string   `json:"prompt"`
	} `json:"shards"`
	ShardHandoffs         []planSubagentsHandoff     `json:"shardHandoffs"`
	ReviewerOrchestration planSubagentsOrchestration `json:"reviewerOrchestration"`
	Observability         planSubagentsObservables   `json:"observability"`
	ReviewLoop            planSubagentsReviewLoop    `json:"reviewLoop"`
}

type planSubagentsOrchestration struct {
	Mode                string                            `json:"mode"`
	Scope               string                            `json:"scope"`
	TargetLane          string                            `json:"targetLane"`
	PacketPath          string                            `json:"packetPath"`
	ResultRoot          string                            `json:"resultRoot"`
	BatchPreviewCommand string                            `json:"batchPreviewCommand"`
	BatchApplyCommand   string                            `json:"batchApplyCommand"`
	ReviewerCount       int                               `json:"reviewerCount"`
	MaxParallel         int                               `json:"maxParallel"`
	Summary             planSubagentsOrchestrationSummary `json:"summary"`
	Dispatches          []struct {
		ShardID            string   `json:"shardId"`
		ReviewerRole       string   `json:"reviewerRole"`
		Status             string   `json:"status"`
		Items              []string `json:"items"`
		DispatchPrompt     string   `json:"dispatchPrompt"`
		ReviewerResultPath string   `json:"reviewerResultPath"`
		PreviewCommand     string   `json:"previewCommand"`
		ApplyCommand       string   `json:"applyCommand"`
	} `json:"dispatches"`
	Lifecycle []struct {
		Step          string   `json:"step"`
		Owner         string   `json:"owner"`
		Action        string   `json:"action"`
		Inputs        []string `json:"inputs"`
		MustPass      []string `json:"mustPass"`
		NextOnSuccess string   `json:"nextOnSuccess"`
		NextOnFailure string   `json:"nextOnFailure"`
	} `json:"lifecycle"`
	RuntimeBoundary             []string                             `json:"runtimeBoundary"`
	CompletionCriteria          []string                             `json:"completionCriteria"`
	MissionCommanderAction      *missionCommanderActionSnapshot      `json:"missionCommanderAction"`
	MissionCommanderNextActions []missionCommanderNextActionItem     `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue *missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
}

type planSubagentsOrchestrationSummary struct {
	Mode                 string                           `json:"mode"`
	TargetLane           string                           `json:"targetLane"`
	ReviewerCount        int                              `json:"reviewerCount"`
	MaxParallel          int                              `json:"maxParallel"`
	PacketPath           string                           `json:"packetPath"`
	ResultRoot           string                           `json:"resultRoot"`
	BatchPreviewCommand  string                           `json:"batchPreviewCommand"`
	BatchApplyCommand    string                           `json:"batchApplyCommand"`
	OwnerBinding         planSubagentsOwnerBinding        `json:"ownerBinding"`
	DispatchCount        int                              `json:"dispatchCount"`
	IntakeAvailable      bool                             `json:"intakeAvailable"`
	CollectionAvailable  bool                             `json:"collectionAvailable"`
	DispatchOnly         bool                             `json:"dispatchOnly"`
	ActionTotal          int                              `json:"actionTotal"`
	ActionUnblocked      int                              `json:"actionUnblocked"`
	ActionBlocked        int                              `json:"actionBlocked"`
	ActionRequiresReview int                              `json:"actionRequiresReview"`
	ActionFollowUp       int                              `json:"actionFollowUp"`
	QueueSummary         string                           `json:"queueSummary"`
	FirstDispatch        *planSubagentsDispatchSummary    `json:"firstDispatch"`
	Dispatches           []planSubagentsDispatchSummary   `json:"dispatches"`
	CurrentAction        *missionCommanderNextActionItem  `json:"currentAction"`
	NextActions          []missionCommanderNextActionItem `json:"nextActions"`
	Boundary             []string                         `json:"boundary"`
}

type planSubagentsDispatchSummary struct {
	ShardID            string `json:"shardId"`
	Status             string `json:"status"`
	ReviewerResultPath string `json:"reviewerResultPath"`
	DispatchCommand    string `json:"dispatchCommand"`
	PreviewCommand     string `json:"previewCommand"`
	ApplyCommand       string `json:"applyCommand"`
}

type planSubagentsCollectionCommands struct {
	CandidatePath  string `json:"candidatePath"`
	PreviewCommand string `json:"previewCommand"`
	ApplyCommand   string `json:"applyCommand"`
}

type planSubagentsHandoff struct {
	ShardID                     string                           `json:"shardId"`
	Status                      string                           `json:"status"`
	ReviewerResultPath          string                           `json:"reviewerResultPath"`
	ReviewerResultCandidatePath string                           `json:"reviewerResultCandidatePath"`
	ReviewerCollectionCommands  *planSubagentsCollectionCommands `json:"reviewerCollectionCommands"`
	OwnerBinding                planSubagentsOwnerBinding        `json:"ownerBinding"`
	DispatchPrompt              string                           `json:"dispatchPrompt"`
	Items                       []string                         `json:"items"`
	ReadOnlyBoundary            []string                         `json:"readOnlyBoundary"`
	ExpectedOutput              string                           `json:"expectedOutput"`
	ReviewerWriteback           string                           `json:"reviewerWriteback"`
	ReviewerResultContract      planSubagentsReviewerContract    `json:"reviewerResultContract"`
	ReviewerIntakeCommands      planSubagentsIntakeCommands      `json:"reviewerIntakeCommands"`
	MainAgentNextAction         string                           `json:"mainAgentNextAction"`
	IntakeChecklist             []string                         `json:"intakeChecklist"`
	ReviewerDecisionMappings    []planSubagentsDecisionMapping   `json:"reviewerDecisionMappings"`
	ConflictHandling            []string                         `json:"conflictHandling"`
	WritebackSequence           []planSubagentsWritebackStep     `json:"writebackSequence"`
	PostReviewMerge             []string                         `json:"postReviewMerge"`
	CompletionCriteria          []string                         `json:"completionCriteria"`
	FailureHandling             string                           `json:"failureHandling"`
}

type planSubagentsReviewerContract struct {
	OutputFormat     string   `json:"outputFormat"`
	RequiredFields   []string `json:"requiredFields"`
	AllowedDecisions []string `json:"allowedDecisions"`
	EvidenceRules    []string `json:"evidenceRules"`
	ConflictSignals  []string `json:"conflictSignals"`
}

type planSubagentsDecisionMapping struct {
	ReviewerDecision    string   `json:"reviewerDecision"`
	VerificationVerdict string   `json:"verificationVerdict"`
	MainDecision        string   `json:"mainDecision"`
	ApplyWhen           []string `json:"applyWhen"`
	Fallback            string   `json:"fallback"`
}

type planSubagentsWritebackStep struct {
	Step            string                                 `json:"step"`
	Owner           string                                 `json:"owner"`
	Uses            []string                               `json:"uses"`
	CommandBindings []planSubagentsWritebackCommandBinding `json:"commandBindings"`
	MustPass        []string                               `json:"mustPass"`
	BlockedBy       []string                               `json:"blockedBy"`
	NextOnSuccess   string                                 `json:"nextOnSuccess"`
	NextOnFailure   string                                 `json:"nextOnFailure"`
}

type planSubagentsWritebackCommandBinding struct {
	Binding        string   `json:"binding"`
	Source         string   `json:"source"`
	Kind           string   `json:"kind"`
	Command        string   `json:"command"`
	RequiredFields []string `json:"requiredFields"`
	ExpectedOutput string   `json:"expectedOutput"`
}

type planSubagentsIntakeCommands struct {
	Purpose        string                             `json:"purpose"`
	PreviewCommand string                             `json:"previewCommand"`
	ApplyCommand   string                             `json:"applyCommand"`
	RequiredFields []string                           `json:"requiredFields"`
	PreviewChecks  []string                           `json:"previewChecks"`
	BlockedOutputs []string                           `json:"blockedOutputs"`
	RepairGuidance []reviewerIntakeRepairGuidanceItem `json:"repairGuidance"`
}

type reviewerIntakeRepairGuidanceItem struct {
	Reason   string   `json:"reason"`
	Action   string   `json:"action"`
	Evidence []string `json:"evidence"`
	Boundary []string `json:"boundary"`
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
	LastTakeoverAt             string `json:"lastTakeoverAt"`
	LastTakeoverBy             string `json:"lastTakeoverBy"`
	LastTakeoverReason         string `json:"lastTakeoverReason"`
	LastReconciledIntervention string `json:"lastReconciledIntervention"`
}

type startWrite struct {
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Action     string `json:"action"`
	TargetPath string `json:"targetPath"`
}

type candidateResult struct {
	Command         string                    `json:"command"`
	CaseRoot        string                    `json:"caseRoot"`
	Pack            string                    `json:"pack"`
	IsMutation      bool                      `json:"isMutation"`
	Applied         bool                      `json:"applied"`
	CandidateRoot   string                    `json:"candidateRoot"`
	ToolingRoot     string                    `json:"toolingRoot"`
	IndexPath       string                    `json:"indexPath"`
	Created         int                       `json:"created"`
	Blocked         int                       `json:"blocked"`
	RequiresCleanup bool                      `json:"requiresCleanup"`
	ReviewPlan      candidateReviewPlan       `json:"reviewPlan"`
	ReviewWorkspace *candidateReviewWorkspace `json:"reviewWorkspace"`
	Writes          []candidateWrite          `json:"writes"`
}

type candidateReviewWorkspace struct {
	WritesArtifacts  bool   `json:"writesArtifacts"`
	ReviewRoot       string `json:"reviewRoot"`
	PacketPath       string `json:"packetPath"`
	SummaryPath      string `json:"summaryPath"`
	CombinedDiffPath string `json:"combinedDiffPath"`
}

type candidateReviewSummary struct {
	Mode                       string                      `json:"mode"`
	Pack                       string                      `json:"pack"`
	Total                      int                         `json:"total"`
	PendingReviewCount         int                         `json:"pendingReviewCount"`
	BlockedCount               int                         `json:"blockedCount"`
	NotNeededCount             int                         `json:"notNeededCount"`
	CreatedCount               int                         `json:"createdCount"`
	SkippedCount               int                         `json:"skippedCount"`
	ManagedDocCount            int                         `json:"managedDocCount"`
	ToolingCandidateCount      int                         `json:"toolingCandidateCount"`
	CleanupTargetCount         int                         `json:"cleanupTargetCount"`
	ReviewArtifactCount        int                         `json:"reviewArtifactCount"`
	DecisionChecklistCount     int                         `json:"decisionChecklistCount"`
	DecisionFollowThroughCount int                         `json:"decisionFollowThroughCount"`
	ExecutionStepCount         int                         `json:"executionStepCount"`
	ReconsumeCheckCount        int                         `json:"reconsumeCheckCount"`
	NextActionCount            int                         `json:"nextActionCount"`
	ReviewRequiredActionCount  int                         `json:"reviewRequiredActionCount"`
	CurrentAction              string                      `json:"currentAction"`
	CandidateRoot              string                      `json:"candidateRoot"`
	ToolingRoot                string                      `json:"toolingRoot"`
	IndexPath                  string                      `json:"indexPath"`
	RequiresReview             bool                        `json:"requiresReview"`
	RequiresCleanup            bool                        `json:"requiresCleanup"`
	HasToolingCandidate        bool                        `json:"hasToolingCandidate"`
	HasBlockedItems            bool                        `json:"hasBlockedItems"`
	HasIndex                   bool                        `json:"hasIndex"`
	HasDecisionArtifacts       bool                        `json:"hasDecisionArtifacts"`
	HasCleanupArtifacts        bool                        `json:"hasCleanupArtifacts"`
	HasReconsumeArtifacts      bool                        `json:"hasReconsumeArtifacts"`
	ProofSummary               candidateReviewProofSummary `json:"proofSummary"`
	WhatIf                     bool                        `json:"whatIf"`
	Boundary                   []string                    `json:"boundary"`
}

type candidateReviewProofSummary struct {
	Total                    int                     `json:"total"`
	Present                  int                     `json:"present"`
	Missing                  int                     `json:"missing"`
	DecisionPresent          int                     `json:"decisionPresent"`
	DecisionMissing          int                     `json:"decisionMissing"`
	CleanupPresent           int                     `json:"cleanupPresent"`
	CleanupMissing           int                     `json:"cleanupMissing"`
	ReconsumePresent         int                     `json:"reconsumePresent"`
	ReconsumeMissing         int                     `json:"reconsumeMissing"`
	ProofRoot                string                  `json:"proofRoot"`
	ProofProgress            string                  `json:"proofProgress"`
	CurrentStage             string                  `json:"currentStage"`
	NextMissingProofType     string                  `json:"nextMissingProofType"`
	NextMissingProofPath     string                  `json:"nextMissingProofPath"`
	NextMissingCandidatePath string                  `json:"nextMissingCandidatePath"`
	NextMissingPackTarget    string                  `json:"nextMissingPackTarget"`
	NextMissingProof         *nextMissingProofDetail `json:"nextMissingProof"`
	Complete                 bool                    `json:"complete"`
	NextAction               string                  `json:"nextAction"`
	Boundary                 []string                `json:"boundary"`
}

type candidateReviewPlan struct {
	Mode                        string                              `json:"mode"`
	ItemCount                   int                                 `json:"itemCount"`
	ReviewSummary               candidateReviewSummary              `json:"reviewSummary"`
	MissionCommanderAction      missionCommanderActionSnapshot      `json:"missionCommanderAction"`
	MissionCommanderNextActions []missionCommanderNextActionItem    `json:"missionCommanderNextActions"`
	MissionCommanderActionQueue missionCommanderActionQueueSnapshot `json:"missionCommanderActionQueue"`
	CleanupTargets              []struct {
		Path           string   `json:"path"`
		Kind           string   `json:"kind"`
		CandidatePath  string   `json:"candidatePath"`
		IndexPath      string   `json:"indexPath"`
		CleanupWhen    string   `json:"cleanupWhen"`
		CleanupActions []string `json:"cleanupActions"`
	} `json:"cleanupTargets"`
	ReviewArtifacts []struct {
		Path          string   `json:"path"`
		Kind          string   `json:"kind"`
		Name          string   `json:"name"`
		When          string   `json:"when"`
		Action        string   `json:"action"`
		CandidatePath string   `json:"candidatePath"`
		PackTarget    string   `json:"packTarget"`
		Format        string   `json:"format"`
		Evidence      []string `json:"evidence"`
		Boundary      []string `json:"boundary"`
	} `json:"reviewArtifacts"`
	Reconsume struct {
		Mode                  string   `json:"mode"`
		Tooling               string   `json:"tooling"`
		Commands              []string `json:"commands"`
		Boundary              []string `json:"boundary"`
		VerificationChecklist []struct {
			Name     string   `json:"name"`
			When     string   `json:"when"`
			Commands []string `json:"commands"`
			Expected string   `json:"expected"`
			Evidence []string `json:"evidence"`
			Boundary []string `json:"boundary"`
		} `json:"verificationChecklist"`
	} `json:"reconsume"`
	DecisionChecklist []struct {
		Path                 string   `json:"path"`
		Kind                 string   `json:"kind"`
		ReviewDecision       string   `json:"reviewDecision"`
		CandidatePath        string   `json:"candidatePath"`
		PackTarget           string   `json:"packTarget"`
		ReviewAction         string   `json:"reviewAction"`
		AcceptActions        []string `json:"acceptActions"`
		RejectActions        []string `json:"rejectActions"`
		CleanupActions       []string `json:"cleanupActions"`
		VerificationCommands []string `json:"verificationCommands"`
		Boundary             []string `json:"boundary"`
	} `json:"decisionChecklist"`
	DecisionFollowThrough []struct {
		Path           string `json:"path"`
		Kind           string `json:"kind"`
		ReviewDecision string `json:"reviewDecision"`
		CandidatePath  string `json:"candidatePath"`
		PackTarget     string `json:"packTarget"`
		Outcomes       []struct {
			Decision             string   `json:"decision"`
			State                string   `json:"state"`
			When                 string   `json:"when"`
			Actions              []string `json:"actions"`
			CleanupActions       []string `json:"cleanupActions"`
			VerificationCommands []string `json:"verificationCommands"`
			Expected             string   `json:"expected"`
			Evidence             []string `json:"evidence"`
			Boundary             []string `json:"boundary"`
		} `json:"outcomes"`
		Boundary []string `json:"boundary"`
	} `json:"decisionFollowThrough"`
	MainAgentExecutionPlan []struct {
		Name      string   `json:"name"`
		When      string   `json:"when"`
		AppliesTo []string `json:"appliesTo"`
		Actions   []string `json:"actions"`
		Commands  []string `json:"commands"`
		Expected  string   `json:"expected"`
		Evidence  []string `json:"evidence"`
		Boundary  []string `json:"boundary"`
	} `json:"mainAgentExecutionPlan"`
	ReviewItems        []candidateReviewItem `json:"reviewItems"`
	RuntimeBoundary    []string              `json:"runtimeBoundary"`
	CompletionCriteria []string              `json:"completionCriteria"`
}

type candidateReviewItem struct {
	Path             string   `json:"path"`
	Kind             string   `json:"kind"`
	ReviewDecision   string   `json:"reviewDecision"`
	CandidatePath    string   `json:"candidatePath"`
	PackTarget       string   `json:"packTarget"`
	MergeTargetHint  string   `json:"mergeTargetHint"`
	CleanupPath      string   `json:"cleanupPath"`
	MainAgentActions []string `json:"mainAgentActions"`
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

func handoffLaneActionFor(t *testing.T, items []handoffLaneExecutorAction, lane string) handoffLaneExecutorAction {
	t.Helper()
	for _, item := range items {
		if item.Lane == lane {
			return item
		}
	}
	t.Fatalf("handoff lane executor action for %s not found in %+v", lane, items)
	return handoffLaneExecutorAction{}
}

func cliNextActionContainsCommand(items []missionCommanderNextActionItem, want string) bool {
	for _, item := range items {
		if strings.Contains(item.Command, want) {
			return true
		}
	}
	return false
}

func cliNextActionContainsSource(items []missionCommanderNextActionItem, want string) bool {
	for _, item := range items {
		if strings.Contains(item.Source, want) {
			return true
		}
	}
	return false
}

func cliNextActionBoundaryContains(items []missionCommanderNextActionItem, want string) bool {
	for _, item := range items {
		if containsSubstring(item.Boundary, want) {
			return true
		}
	}
	return false
}

func containsSubstring(items []string, want string) bool {
	for _, item := range items {
		if strings.Contains(item, want) {
			return true
		}
	}
	return false
}

func containsCLIRepairGuidance(items []reviewerIntakeRepairGuidanceItem, reasonPart, actionPart string) bool {
	for _, item := range items {
		if strings.Contains(item.Reason, reasonPart) && strings.Contains(item.Action, actionPart) {
			return true
		}
	}
	return false
}

func assertMovedCaseRepairPreviewDiagnostic(t *testing.T, err error, caseRoot, pack string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected moved case diagnostic error")
	}
	text := err.Error()
	for _, expected := range []string{
		"case metadata points to a different directory",
		`/rekit repair -Target "` + caseRoot + `" -Pack ` + pack + ` -WhatIf -Format text`,
		"preview metadata and thin-shim refresh",
		"repair -Apply only after explicit confirmation",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("moved case diagnostic missing %q: %s", expected, text)
		}
	}
	if strings.Contains(text, "Run 'rekit repair -Target") {
		t.Fatalf("moved case diagnostic should not recommend direct repair -Apply first: %s", text)
	}
}

func containsMissionCommanderNextActionsCommand(items []missionCommanderNextActionItem, want string) bool {
	for _, item := range items {
		if strings.Contains(item.Command, want) {
			return true
		}
	}
	return false
}

func assertEvidenceReviewBeforeBlockedAdapterActions(t *testing.T, label string, items []missionCommanderNextActionItem, gateEventID string) {
	t.Helper()
	if len(items) < 3 || items[0].Lane != "main" || items[0].Command != "/rekit handoff main" || items[0].Source != "executionEvidenceReview" || !items[0].Blocked || !items[0].RequiresReview || !containsSubstring(items[0].Reasons, gateEventID) || !containsSubstring(items[0].Boundary, "do not replay heavy tool") || items[1].Source != "executionEvidenceReview.followUp" || items[1].Command != "/rekit overview" {
		t.Fatalf("%s should prioritize execution evidence review: %+v", label, items)
	}
	adapterActions := items[2:]
	if !slices.ContainsFunc(adapterActions, func(item missionCommanderNextActionItem) bool {
		return strings.HasPrefix(item.Source, "adapterReportContract.missionCommanderAction")
	}) {
		t.Fatalf("%s should preserve adapter report actions after evidence review: %+v", label, items)
	}
	for _, item := range adapterActions {
		if strings.HasPrefix(item.Source, "adapterReport") && (!item.Blocked || !item.RequiresReview || !containsSubstring(item.Reasons, "another execution evidence item requires main review") || !containsSubstring(item.Boundary, "until main evidence review completes")) {
			t.Fatalf("%s adapter action should remain visible but blocked by main evidence review: %+v", label, item)
		}
	}
	if slices.ContainsFunc(items, func(item missionCommanderNextActionItem) bool {
		return strings.HasPrefix(item.Command, "/rekit continue main")
	}) {
		t.Fatalf("%s should suppress autonomous continue while main evidence review is open: %+v", label, items)
	}
}

func containsMissionCommanderNextAction(items []missionCommanderNextActionItem, source, command string, blocked, requiresReview bool) bool {
	return slices.ContainsFunc(items, func(item missionCommanderNextActionItem) bool {
		return item.Source == source && item.Command == command && item.Blocked == blocked && item.RequiresReview == requiresReview
	})
}

func candidateJSONNextActionContains(items []missionCommanderNextActionItem, source, want string) bool {
	return slices.ContainsFunc(items, func(item missionCommanderNextActionItem) bool {
		if item.Source != source {
			return false
		}
		fields := []string{item.Lane, item.Label, item.State, item.Command}
		fields = append(fields, item.Reasons...)
		fields = append(fields, item.Boundary...)
		return containsSubstring(fields, want)
	})
}

func candidateJSONNextActionBoundaryContains(items []missionCommanderNextActionItem, want string) bool {
	return slices.ContainsFunc(items, func(item missionCommanderNextActionItem) bool {
		return containsSubstring(item.Boundary, want)
	})
}

func candidateJSONDecisionChecklistContains(items []struct {
	Path                 string   `json:"path"`
	Kind                 string   `json:"kind"`
	ReviewDecision       string   `json:"reviewDecision"`
	CandidatePath        string   `json:"candidatePath"`
	PackTarget           string   `json:"packTarget"`
	ReviewAction         string   `json:"reviewAction"`
	AcceptActions        []string `json:"acceptActions"`
	RejectActions        []string `json:"rejectActions"`
	CleanupActions       []string `json:"cleanupActions"`
	VerificationCommands []string `json:"verificationCommands"`
	Boundary             []string `json:"boundary"`
}, path, want string) bool {
	for _, item := range items {
		if item.Path != path {
			continue
		}
		fields := []string{item.ReviewAction}
		fields = append(fields, item.AcceptActions...)
		fields = append(fields, item.RejectActions...)
		fields = append(fields, item.CleanupActions...)
		fields = append(fields, item.VerificationCommands...)
		fields = append(fields, item.Boundary...)
		if containsSubstring(fields, want) {
			return true
		}
	}
	return false
}

func candidateJSONDecisionFollowThroughContains(items []struct {
	Path           string `json:"path"`
	Kind           string `json:"kind"`
	ReviewDecision string `json:"reviewDecision"`
	CandidatePath  string `json:"candidatePath"`
	PackTarget     string `json:"packTarget"`
	Outcomes       []struct {
		Decision             string   `json:"decision"`
		State                string   `json:"state"`
		When                 string   `json:"when"`
		Actions              []string `json:"actions"`
		CleanupActions       []string `json:"cleanupActions"`
		VerificationCommands []string `json:"verificationCommands"`
		Expected             string   `json:"expected"`
		Evidence             []string `json:"evidence"`
		Boundary             []string `json:"boundary"`
	} `json:"outcomes"`
	Boundary []string `json:"boundary"`
}, path, decision, want string) bool {
	for _, item := range items {
		if item.Path != path {
			continue
		}
		fields := []string{item.ReviewDecision, item.CandidatePath, item.PackTarget}
		fields = append(fields, item.Boundary...)
		for _, outcome := range item.Outcomes {
			if outcome.Decision != decision {
				continue
			}
			fields = append(fields, outcome.State, outcome.When, outcome.Expected)
			fields = append(fields, outcome.Actions...)
			fields = append(fields, outcome.CleanupActions...)
			fields = append(fields, outcome.VerificationCommands...)
			fields = append(fields, outcome.Evidence...)
			fields = append(fields, outcome.Boundary...)
		}
		if containsSubstring(fields, want) {
			return true
		}
	}
	return false
}

func candidateJSONReconsumeChecklistContains(items []struct {
	Name     string   `json:"name"`
	When     string   `json:"when"`
	Commands []string `json:"commands"`
	Expected string   `json:"expected"`
	Evidence []string `json:"evidence"`
	Boundary []string `json:"boundary"`
}, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func candidateJSONReviewArtifactContains(items []struct {
	Path          string   `json:"path"`
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	When          string   `json:"when"`
	Action        string   `json:"action"`
	CandidatePath string   `json:"candidatePath"`
	PackTarget    string   `json:"packTarget"`
	Format        string   `json:"format"`
	Evidence      []string `json:"evidence"`
	Boundary      []string `json:"boundary"`
}, path, name, want string) bool {
	for _, item := range items {
		if item.Path != path || item.Name != name {
			continue
		}
		fields := []string{item.Kind, item.When, item.Action, item.CandidatePath, item.PackTarget, item.Format}
		fields = append(fields, item.Evidence...)
		fields = append(fields, item.Boundary...)
		if containsSubstring(fields, want) {
			return true
		}
	}
	return false
}

func candidateJSONExecutionPlanContains(items []struct {
	Name      string   `json:"name"`
	When      string   `json:"when"`
	AppliesTo []string `json:"appliesTo"`
	Actions   []string `json:"actions"`
	Commands  []string `json:"commands"`
	Expected  string   `json:"expected"`
	Evidence  []string `json:"evidence"`
	Boundary  []string `json:"boundary"`
}, name, want string) bool {
	for _, item := range items {
		if item.Name != name {
			continue
		}
		fields := []string{item.When, item.Expected}
		fields = append(fields, item.AppliesTo...)
		fields = append(fields, item.Actions...)
		fields = append(fields, item.Commands...)
		fields = append(fields, item.Evidence...)
		fields = append(fields, item.Boundary...)
		if containsSubstring(fields, want) {
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

func reviewerResultForCLIPlan(t *testing.T, packet planSubagentsPacket, handoff planSubagentsHandoff, decision, verdict, reviewerSession string) []byte {
	t.Helper()
	evidence := "workspace/features/feature-login/review-evidence.md"
	result := map[string]any{
		"packetId":           packet.PacketID,
		"routeId":            packet.Route.ID,
		"shardId":            handoff.ShardID,
		"items":              append([]string{}, handoff.Items...),
		"reviewerSession":    reviewerSession,
		"decision":           decision,
		"confidence":         "high",
		"summary":            "reviewed " + handoff.ShardID + " against bounded evidence",
		"evidenceRefs":       []string{evidence},
		"risks":              []string{},
		"conflicts":          []string{},
		"recommendedVerdict": verdict,
		"routeOutput": map[string]any{
			"item":           strings.Join(handoff.Items, ","),
			"decision":       decision,
			"confidence":     "high",
			"evidence":       evidence,
			"risk":           "bounded",
			"next_action":    "main-agent-review",
			"tier_used":      "light",
			"tool_scope":     "read-only",
			"feature":        "login",
			"request_id":     "n/a",
			"candidate_path": "n/a",
			"defer_reason":   "n/a",
		},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readOptionalCaseFile(t *testing.T, caseRoot, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(caseRoot, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
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

func assertCandidateReviewItem(t *testing.T, items []candidateReviewItem, path, decision string) candidateReviewItem {
	t.Helper()
	for _, item := range items {
		if item.Path != path || item.ReviewDecision != decision {
			continue
		}
		if decision == "pending-review" && (item.CandidatePath == "" || item.CleanupPath == "") {
			t.Fatalf("candidate review item %s missing candidate/cleanup path: %+v", path, item)
		}
		if len(item.MainAgentActions) == 0 {
			t.Fatalf("candidate review item %s missing main agent actions: %+v", path, item)
		}
		return item
	}
	t.Fatalf("candidate review item %s with decision %q not found in %+v", path, decision, items)
	return candidateReviewItem{}
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

func writePathFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
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
	board := `{"schemaVersion":1,"caseRoot":"` + filepath.ToSlash(caseRoot) + `","repoRoot":"` + filepath.ToSlash(repoRoot(t)) + `","pack":"_template","automationMode":"assist","defaultAuthorityLane":"main","lanes":[{"id":"main","type":"main","title":"Main","status":"open","authority":true,"workspace":"captures/lanes/main","currentExecutor":"session-main","executorGeneration":3,"lastTakeoverAt":"2026-01-01T00:00:00Z","lastTakeoverBy":"main-agent","lastTakeoverReason":"fixture"}],"factsRoot":".rekit/facts"}`
	writeCaseFile(t, caseRoot, ".rekit/board.json", board)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","type":"main","title":"Main","status":"open","authority":true,"workspace":"captures/lanes/main","currentExecutor":"session-main","executorGeneration":3,"lastTakeoverAt":"2026-01-01T00:00:00Z","lastTakeoverBy":"main-agent","lastTakeoverReason":"fixture"}`)
	writeFactFile(t, factsRoot, "observations.jsonl", []string{`{"kind":"observation","eventId":"obs-auth-1","lane":"main","subject":"obs","summary":"preauthorized adapter output ready for review","status":"complete","target":"target-alpha","evidenceRefs":["evidence/debug.json"],"execution":{"gateEventId":"gate-auth-1","authorization":"preauthorized","status":"complete","outputRefs":["workspace/main/debug/out.txt"]},"gate":{"action":"debug","authorization":{"decision":"preauthorized"}}}`})
	writeFactFile(t, factsRoot, "candidates.jsonl", []string{`{"kind":"candidate","eventId":"cand-main-1","lane":"main","subject":"handler","summary":"candidate one","confidence":"high","status":"open","target":"candidate-alpha","evidenceRefs":["evidence/candidate.json"]}`, `{"kind":"candidate","lane":"main","subject":"handler","summary":"candidate two","confidence":"medium","status":"open"}`})
	writeFactFile(t, factsRoot, "requests.jsonl", []string{`{"kind":"request","lane":"main","subject":"debug gate","summary":"needs confirmation","status":"pending-gate","actor":"runtime-test","risk":"high","target":"batch-overview","batchId":"batch-overview","gate":{"action":"debug","scope":"handler only","budget":"30s","triedLightSteps":["overview","static review"],"stopConditions":["timeout"]}}`})
	writeFactFile(t, factsRoot, "publications.jsonl", []string{`{"kind":"publication","lane":"main","subject":"pub","summary":"published"}`})
	writeFactFile(t, factsRoot, "decisions.jsonl", []string{`{"kind":"decision","eventId":"dec-main-1","lane":"main","subject":"decision subject","decision":"defer","actor":"runtime-test","reason":"needs review","target":"decision-alpha","evidenceRefs":["evidence/decision.json"],"batchId":"batch-overview"}`})
	writeFactFile(t, factsRoot, "hypotheses.jsonl", nil)
	writeFactFile(t, factsRoot, "verifications.jsonl", []string{`{"kind":"verification","lane":"main","subject":"review target","actor":"reviewer-smoke","target":"candidate-alpha","verifier":"manual-review","verdict":"accepted","batchId":"batch-overview"}`})
	writeFactFile(t, factsRoot, "interventions.jsonl", []string{`{"kind":"intervention","eventId":"int-main-1","lane":"main","subject":"manual override","summary":"needs human","action":"override","target":"batch-overview","approvedBy":"lead","scope":"metadata","status":"open","batchId":"batch-overview"}`})
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
	board := `{"schemaVersion":1,"caseRoot":"` + filepath.ToSlash(caseRoot) + `","repoRoot":"` + filepath.ToSlash(repoRoot(t)) + `","pack":"_template","automationMode":"assist","defaultAuthorityLane":"main","lanes":[{"id":"main","type":"main","title":"主线","status":"open","authority":true,"workspace":"workspace/main/main","currentExecutor":"session-main","executorGeneration":1,"lastTakeoverAt":"2026-01-01T00:00:00Z","lastTakeoverBy":"main-agent","lastTakeoverReason":"initial main claim"},{"id":"feature-login","type":"feature","title":"功能分析: login","status":"open","authority":false,"workspace":"workspace/features/feature-login","currentExecutor":"session-login","executorGeneration":2,"lastTakeoverAt":"2026-01-02T00:00:00Z","lastTakeoverBy":"main-agent","lastTakeoverReason":"replace login session"}],"factsRoot":".rekit/facts"}`
	writeCaseFile(t, caseRoot, ".rekit/board.json", board)
	writeCaseFile(t, caseRoot, ".rekit/lanes/main/lane.json", `{"schemaVersion":1,"id":"main","type":"main","title":"主线","status":"open","authority":true,"workspace":"workspace/main/main","laneRoot":".rekit/lanes/main","currentExecutor":"session-main","executorGeneration":1,"lastTakeoverAt":"2026-01-01T00:00:00Z","lastTakeoverBy":"main-agent","lastTakeoverReason":"initial main claim"}`)
	writeCaseFile(t, caseRoot, ".rekit/lanes/feature-login/lane.json", `{"schemaVersion":1,"id":"feature-login","type":"feature","name":"login","title":"功能分析: login","status":"open","authority":false,"workspace":"workspace/features/feature-login","laneRoot":".rekit/lanes/feature-login","currentExecutor":"session-login","executorGeneration":2,"lastTakeoverAt":"2026-01-02T00:00:00Z","lastTakeoverBy":"main-agent","lastTakeoverReason":"replace login session"}`)
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
