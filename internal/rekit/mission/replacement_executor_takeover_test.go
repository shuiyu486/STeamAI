package mission

import (
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
)

func TestReplacementExecutorDriverRequestSHA256BindsFullIdentity(t *testing.T) {
	invocation, err := commands.NewPublicInvocation(commands.Continue, "main", "-WhatIf", "-Format", "json")
	if err != nil {
		t.Fatal(err)
	}
	request := MissionCommanderDriverRequest{
		Kind:              "preview-command",
		RunLoopStepID:     "apply-or-run-current",
		Actor:             "main-agent",
		State:             "ready-to-continue",
		Source:            "missionCommanderActions",
		Lane:              "main",
		Label:             "main",
		ActionID:          "continue-main",
		Invocation:        &invocation,
		Command:           "/rekit continue main -WhatIf -Format json",
		CommandExecutable: true,
		RequiresReview:    true,
		ExpectedReceipt: MissionCommanderDriverReceiptExpectation{
			State:                "refresh-required",
			Command:              "/rekit continue main -WhatIf -Format json",
			RefreshStatusCommand: "/rekit status -Format json",
			Description:          "refresh after preview",
			Boundary:             []string{"receipt-boundary"},
		},
		Boundary: []string{"request-boundary"},
	}
	base := ReplacementExecutorDriverRequestSHA256(request)
	if base == "" {
		t.Fatal("driver request SHA-256 should not be empty")
	}
	mutations := []struct {
		name   string
		change func(*MissionCommanderDriverRequest)
	}{
		{name: "action-id", change: func(value *MissionCommanderDriverRequest) { value.ActionID = "tampered" }},
		{name: "driver-kind", change: func(value *MissionCommanderDriverRequest) { value.Kind = "execute-command" }},
		{name: "expected-receipt", change: func(value *MissionCommanderDriverRequest) { value.ExpectedReceipt.Description = "tampered" }},
		{name: "expected-boundary", change: func(value *MissionCommanderDriverRequest) {
			value.ExpectedReceipt.Boundary = append(value.ExpectedReceipt.Boundary, "tampered")
		}},
		{name: "request-boundary", change: func(value *MissionCommanderDriverRequest) { value.Boundary = append(value.Boundary, "tampered") }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := request
			changed.Boundary = append([]string{}, request.Boundary...)
			changed.ExpectedReceipt.Boundary = append([]string{}, request.ExpectedReceipt.Boundary...)
			mutation.change(&changed)
			if got := ReplacementExecutorDriverRequestSHA256(changed); got == "" || got == base {
				t.Fatalf("full request identity mutation did not change SHA-256: base=%s got=%s", base, got)
			}
		})
	}
}

func TestReplacementExecutorDriverRequestSHA256IgnoresCommandQuoting(t *testing.T) {
	invocation, err := commands.NewPublicInvocation(commands.Continue, "main", "-WhatIf", "-Format", "json")
	if err != nil {
		t.Fatal(err)
	}
	request := MissionCommanderDriverRequest{
		Kind: "preview-command", RunLoopStepID: "preview-current", Invocation: &invocation,
		Command: "/rekit continue \"main\" -WhatIf -Format json", CommandExecutable: true,
		ExpectedReceipt: MissionCommanderDriverReceiptExpectation{Command: "/rekit continue \"main\" -WhatIf -Format json"},
	}
	first := ReplacementExecutorDriverRequestSHA256(request)
	request.Command = "/rekit continue main -WhatIf -Format json"
	request.ExpectedReceipt.Command = request.Command
	second := ReplacementExecutorDriverRequestSHA256(request)
	if first == "" || first != second {
		t.Fatalf("quoting-only projection changed typed request identity: first=%s second=%s", first, second)
	}
}

func TestReplacementExecutorTakeoverArtifactIdentityBindsConsumablePackage(t *testing.T) {
	request := MissionCommanderDriverRequest{Command: "/rekit continue main", CommandExecutable: true}
	pkg := ReplacementExecutorTakeoverPackageFor(&request, ReplacementExecutorTakeoverOptions{
		Focus:                "durable-handoff-current-action",
		Scope:                "lane:main",
		RefreshStatusCommand: "/rekit status -Format json",
		PackagePath:          ".rekit/handovers/main-latest-replacement-executor-takeover.json",
		TargetDocuments:      []string{".rekit/handovers/main-latest.md"},
	})
	base := ReplacementExecutorTakeoverArtifactIdentitySHA256(*pkg)
	if base == "" {
		t.Fatal("takeover artifact identity should not be empty")
	}
	mutations := []struct {
		name   string
		change func(*ReplacementExecutorTakeoverPackage)
	}{
		{name: "refresh-command", change: func(value *ReplacementExecutorTakeoverPackage) {
			value.RefreshStatusCommand = "/rekit continue main -Apply"
		}},
		{name: "runbook", change: func(value *ReplacementExecutorTakeoverPackage) {
			value.RunbookSteps = append(value.RunbookSteps, "tampered")
		}},
		{name: "boundary", change: func(value *ReplacementExecutorTakeoverPackage) { value.Boundary = append(value.Boundary, "tampered") }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := *pkg
			changed.TargetDocuments = append([]string{}, pkg.TargetDocuments...)
			changed.RunbookSteps = append([]string{}, pkg.RunbookSteps...)
			changed.Boundary = append([]string{}, pkg.Boundary...)
			mutation.change(&changed)
			if got := ReplacementExecutorTakeoverArtifactIdentitySHA256(changed); got == "" || got == base {
				t.Fatalf("consumable package mutation did not change identity: base=%s got=%s", base, got)
			}
		})
	}
	observed := *pkg
	observed.DurableArtifactPath = "artifact.json"
	observed.DurableArtifactFresh = true
	observed.DurableArtifactState = "fresh"
	observed.DurableArtifactSHA256 = "artifact-sha"
	observed.DurableArtifactRequestSHA256 = "request-sha"
	observed.TargetDocuments = append(observed.TargetDocuments, "discovery-only")
	if got := ReplacementExecutorTakeoverArtifactIdentitySHA256(observed); got != base {
		t.Fatalf("discovery-only fields changed durable package identity: base=%s got=%s", base, got)
	}
}

func TestReplacementExecutorTakeoverConsumesHarnessPackage(t *testing.T) {
	request := MissionCommanderDriverRequest{Command: "/rekit status -Format json", CommandExecutable: true}
	operator := &CurrentLoopOperatorPackage{ExternalSessionJob: &CurrentLoopExternalSessionJob{
		State:          "awaiting-submission",
		HarnessPackage: &CurrentLoopExternalSessionHarnessPackage{State: "running"},
	}}
	pkg := ReplacementExecutorTakeoverPackageFor(&request, ReplacementExecutorTakeoverOptions{CurrentLoopOperator: operator})
	steps := strings.Join(pkg.RunbookSteps, "\n")
	if !strings.Contains(steps, "externalSessionJob.harnessPackage") || !strings.Contains(steps, "launch.ready=true") || !strings.Contains(steps, "input path/sha256") || !strings.Contains(steps, "submissionPath last") {
		t.Fatalf("takeover runbook did not consume the stateful harness package: %s", steps)
	}
	operator.ExternalSessionJob.State = "submission-ready"
	pkg = ReplacementExecutorTakeoverPackageFor(&request, ReplacementExecutorTakeoverOptions{CurrentLoopOperator: operator})
	steps = strings.Join(pkg.RunbookSteps, "\n")
	if !strings.Contains(steps, "harnessPackage.return.reviewRequest") || !strings.Contains(steps, "harnessPackage.return.relayRecoveryRequest") {
		t.Fatalf("takeover runbook omitted harness return review paths: %s", steps)
	}
}

func TestReplacementExecutorTakeoverPackageCarriesRequestIdentity(t *testing.T) {
	invocation, err := commands.NewPublicInvocation(commands.Status, "-Format", "json")
	if err != nil {
		t.Fatal(err)
	}
	request := MissionCommanderDriverRequest{
		Kind:              "execute-command",
		RunLoopStepID:     "apply-or-run-current",
		Invocation:        &invocation,
		Command:           "/rekit status -Format json",
		CommandExecutable: true,
		ExpectedReceipt: MissionCommanderDriverReceiptExpectation{
			State:   "refresh-required",
			Command: "/rekit status -Format json",
		},
	}
	pkg := ReplacementExecutorTakeoverPackageFor(&request, ReplacementExecutorTakeoverOptions{})
	if pkg == nil || pkg.CurrentDriverRequestSHA256 == "" || pkg.CurrentDriverRequestSHA256 != ReplacementExecutorDriverRequestSHA256(pkg.CurrentDriverRequest) {
		t.Fatalf("takeover package omitted canonical current request identity: %+v", pkg)
	}
}
