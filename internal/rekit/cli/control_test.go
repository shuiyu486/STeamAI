package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

func TestParseControlOwnsTypedFlags(t *testing.T) {
	opt, err := Parse([]string{
		"-Command", "control",
		"-Lane", "binary-analysis-main",
		"-Action", "pause",
		"-Actor", "main-agent",
		"-Reason", "operator requested pause",
		"-ControlPublicationStamp", "2026-08-18T12:00:00Z",
		"-ExpectedControlPlanSha256", strings.Repeat("a", 64),
		"-Apply", "-Format", "json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opt.Command != commands.Control || opt.Control.Lane != "binary-analysis-main" || opt.Control.Action != executioncontrol.ActionPause || opt.Control.Actor != "main-agent" || opt.Control.Reason != "operator requested pause" || opt.Control.PublicationStamp != "2026-08-18T12:00:00Z" || opt.Control.ExpectedPlanSHA256 != strings.Repeat("a", 64) || !opt.Apply {
		t.Fatalf("control flags were not parsed into their owner: %+v", opt.Control)
	}
}

func TestRunControlWhatIfThenApplyPublishesOnlyDurableControl(t *testing.T) {
	caseRoot := cliControlFixture(t)
	ctx := runtime.Context{Target: caseRoot, TargetProvided: true}
	previewOpt := Options{
		Command: commands.Control,
		WhatIf:  true,
		Format:  "json",
		Control: executioncontrol.Options{
			Lane:   "binary-analysis-main",
			Action: executioncontrol.ActionPause,
			Actor:  "main-agent",
			Reason: "operator requested pause",
		},
	}
	var out bytes.Buffer
	if err := runControl(ctx, previewOpt, &out); err != nil {
		t.Fatal(err)
	}
	var preview executioncontrol.Plan
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("decode control preview: %v\n%s", err, out.String())
	}
	if preview.Applied || preview.State != executioncontrol.StatePaused || preview.ControlGeneration != 1 || !strings.HasPrefix(preview.ApplyCommand, "/steamai control ") {
		t.Fatalf("unexpected control preview: %+v", preview)
	}
	invocation, err := commands.ParsePublicInvocation(preview.ApplyCommand)
	if err != nil || invocation.Command != commands.Control || !invocation.HasFlag("-Apply") {
		t.Fatalf("control preview did not return a typed Apply command: invocation=%+v err=%v", invocation, err)
	}
	controlRoot := filepath.Join(caseRoot, ".steamai", "lanes", "binary-analysis-main", "execution-control")
	if _, err := os.Lstat(controlRoot); !os.IsNotExist(err) {
		t.Fatalf("control preview wrote state: %v", err)
	}

	out.Reset()
	applyOpt := previewOpt
	applyOpt.WhatIf = false
	applyOpt.Apply = true
	applyOpt.Control.PublicationStamp = preview.PublicationStamp
	applyOpt.Control.ExpectedPlanSHA256 = preview.ExpectedPlanSHA256
	if err := runControl(ctx, applyOpt, &out); err != nil {
		t.Fatal(err)
	}
	var applied executioncontrol.Plan
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("decode control Apply: %v\n%s", err, out.String())
	}
	if !applied.Applied || applied.AlreadyApplied || applied.State != executioncontrol.StatePaused || applied.ControlGeneration != 1 || applied.NoAuthority != true || applied.NoConfirmed != true || applied.NoHeavyTool != true {
		t.Fatalf("unexpected control Apply: %+v", applied)
	}
	entries, err := os.ReadDir(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "00000000000000000001.intent.json" || entries[1].Name() != "00000000000000000001.json" {
		t.Fatalf("control Apply published unexpected artifacts: %+v", entries)
	}
}

func TestRunControlRejectsAmbiguousModeBeforeWriting(t *testing.T) {
	caseRoot := cliControlFixture(t)
	ctx := runtime.Context{Target: caseRoot, TargetProvided: true}
	opt := Options{
		Command: commands.Control,
		Format:  "json",
		Control: executioncontrol.Options{
			Lane: "binary-analysis-main", Action: executioncontrol.ActionStop,
			Actor: "main-agent", Reason: "must choose a mode",
		},
	}
	if err := runControl(ctx, opt, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("ambiguous control mode was accepted: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(caseRoot, ".steamai", "lanes", "binary-analysis-main", "execution-control")); !os.IsNotExist(err) {
		t.Fatalf("ambiguous control command wrote state: %v", err)
	}
}

func cliControlFixture(t *testing.T) string {
	t.Helper()
	caseRoot := filepath.Join(t.TempDir(), "case")
	laneRoot := filepath.Join(caseRoot, ".steamai", "lanes", "binary-analysis-main")
	if err := os.MkdirAll(laneRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, ".steamai", "instance.yml"), []byte("schemaVersion: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lane := `{"schemaVersion":1,"id":"binary-analysis-main","status":"open","currentExecutor":"member-main","executorGeneration":3}` + "\n"
	if err := os.WriteFile(filepath.Join(laneRoot, "lane.json"), []byte(lane), 0o600); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}
