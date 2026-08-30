package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

type publicEntrypointProductFixture struct {
	name       string
	stateDir   string
	entrypoint string
}

func publicEntrypointProductFixtures() []publicEntrypointProductFixture {
	return []publicEntrypointProductFixture{
		{name: "current", stateDir: projectstate.CurrentDir, entrypoint: commands.CurrentPublicEntrypoint},
		{name: "legacy", stateDir: projectstate.LegacyDir, entrypoint: commands.LegacyPublicEntrypoint},
	}
}

func publicEntrypointProductCase(t *testing.T, fixture publicEntrypointProductFixture, projectName string) string {
	t.Helper()
	if fixture.stateDir == projectstate.LegacyDir {
		return fullAttachedCase(t)
	}
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	runInitApplyFromPreview(
		t,
		&out,
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-ProjectName", projectName,
	)
	onboardArgs := []string{
		"-Command", "onboard",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-ProjectName", projectName,
		"-Goal", "validate current project public entrypoint",
		"-Actor", "mission-commander",
		"-Executor", "public-entrypoint-executor",
		"-InitialLane", "main",
		"-WhatIf",
		"-Format", "json",
	}
	out.Reset()
	if err := Run(onboardArgs, &out); err != nil {
		t.Fatal(err)
	}
	var onboard onboardCLIPlan
	if err := json.Unmarshal(out.Bytes(), &onboard); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(onboard.ApplyArgs, &out); err != nil {
		t.Fatal(err)
	}
	return caseRoot
}

func TestStatusAndOverviewProjectEntrypointProjectionAcrossFormats(t *testing.T) {
	commandsAndFormats := []struct {
		command string
		formats []string
	}{
		{command: commands.Status, formats: []string{"table", "tsv", "text", "json", "compact-json"}},
		{command: commands.Overview, formats: []string{"table", "tsv", "text", "json"}},
	}
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := publicEntrypointProductCase(t, fixture, "format-entrypoint-projection")
			for _, commandAndFormats := range commandsAndFormats {
				for _, format := range commandAndFormats.formats {
					t.Run(commandAndFormats.command+"/"+format, func(t *testing.T) {
						var out bytes.Buffer
						if err := Run([]string{
							"-Command", commandAndFormats.command,
							"-Target", caseRoot,
							"-Pack", "_template",
							"-Format", format,
						}, &out); err != nil {
							t.Fatal(err)
						}
						if format == "json" || format == "compact-json" {
							var public map[string]any
							if err := json.Unmarshal(out.Bytes(), &public); err != nil {
								t.Fatalf("%s %s did not decode: %v\n%s", commandAndFormats.command, format, err, out.String())
							}
							assertSelectedPublicEntrypoint(t, commandAndFormats.command, public, fixture.entrypoint)
							return
						}
						assertRenderedPublicEntrypoint(t, out.String(), fixture.entrypoint)
					})
				}
			}
		})
	}
}

func assertRenderedPublicEntrypoint(t *testing.T, text, entrypoint string) {
	t.Helper()
	if !strings.Contains(text, entrypoint+" ") {
		t.Fatalf("rendered output omitted selected entrypoint %s:\n%s", entrypoint, text)
	}
	otherEntrypoint := commands.LegacyPublicEntrypoint
	if entrypoint == commands.LegacyPublicEntrypoint {
		otherEntrypoint = commands.CurrentPublicEntrypoint
	}
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- "+otherEntrypoint+" ") {
			t.Fatalf("rendered command list uses unselected entrypoint %s:\n%s", otherEntrypoint, line)
		}
		if _, value, ok := strings.Cut(trimmed, "："); ok && strings.HasPrefix(strings.TrimSpace(value), otherEntrypoint+" ") {
			t.Fatalf("rendered command field uses unselected entrypoint %s:\n%s", otherEntrypoint, line)
		}
		for _, marker := range []string{
			"command=" + otherEntrypoint + " ",
			"command=`" + otherEntrypoint + " ",
			"current=" + otherEntrypoint + " ",
			"primary=" + otherEntrypoint + " ",
			"primary=`" + otherEntrypoint + " ",
			"continue: " + otherEntrypoint + " ",
			"handoff: " + otherEntrypoint + " ",
		} {
			if strings.Contains(trimmed, marker) {
				t.Fatalf("rendered command carrier uses unselected entrypoint %s:\n%s", otherEntrypoint, line)
			}
		}
	}
}

func TestStatusFreshCurrentProductPathOmitsLegacyEntrypoint(t *testing.T) {
	fixture := publicEntrypointProductFixture{
		name:       "current",
		stateDir:   projectstate.CurrentDir,
		entrypoint: commands.CurrentPublicEntrypoint,
	}
	caseRoot := publicEntrypointProductCase(t, fixture, "fresh-current-entrypoint-projection")
	data, status := runPublicEntrypointProductStatus(t, caseRoot, "")
	if status.Mode != "case" || status.Onboarding == nil || status.Onboarding.State != "committed" || status.ProjectHandoff == nil || len(status.ProjectHandoff.ValidationCommands) != 0 || status.ProjectHandoff.NextBatchSelectionPackage != nil || status.ProjectHandoff.MissionCommanderActionQueue.Counts.Total != 0 {
		t.Fatalf("fresh current status retained central handoff state or left onboarding: mode=%s onboarding=%+v project=%+v", status.Mode, status.Onboarding, status.ProjectHandoff)
	}
	if index := strings.Index(string(data), commands.LegacyPublicEntrypoint); index >= 0 {
		start := max(0, index-160)
		end := min(len(data), index+len(commands.LegacyPublicEntrypoint)+200)
		t.Fatalf("fresh current status leaks legacy entrypoint near %q", data[start:end])
	}
	assertPublicEntrypointProductStatus(t, data, commands.CurrentPublicEntrypoint)
}

func TestStatusGateProductPathProjectsSelectedEntrypoint(t *testing.T) {
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := publicEntrypointProductCase(t, fixture, "gate-entrypoint-projection")
			var out bytes.Buffer
			if err := Run([]string{
				"-Command", "start",
				"-Target", caseRoot,
				"-Pack", "_template",
				"-Name", "login",
				"-Apply",
				"-Format", "json",
			}, &out); err != nil {
				t.Fatal(err)
			}

			factsRoot, err := projectstate.Join(caseRoot, "facts")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(factsRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			writeFactFile(t, factsRoot, "requests.jsonl", []string{`{"kind":"request","lane":"feature-login","subject":"debug gate","summary":"needs bounded debug decision","status":"pending-gate","risk":"high","target":"target.bin","actor":"mission-commander","gate":{"action":"debug","scope":"single function","requestedBudget":{"runtimeSeconds":30,"diskMB":64,"requests":1},"outputPaths":["workspace/feature-login/debug/session-1"],"stopConditions":["timeout","unexpected-side-effect","scope-drift"],"authorization":{"decision":"needs-user","profileId":"manual-feature-login"}}}`})
			writeFactFile(t, factsRoot, "candidates.jsonl", nil)
			writeFactFile(t, factsRoot, "decisions.jsonl", nil)
			writeFactFile(t, factsRoot, "interventions.jsonl", nil)

			profilePath, err := projectstate.Join(caseRoot, "lanes", "feature-login", "autonomy.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
				t.Fatal(err)
			}
			profile := `{
  "schemaVersion": 1,
  "profileId": "feature-login-debug-preauth",
  "lane": "feature-login",
  "mode": "preauthorized",
  "allowedActions": ["debug"],
  "deniedActions": [],
  "targetScope": [{"match":"exact","value":"target.bin"}],
  "budget": {"runtimeSeconds": 60, "diskMB": 128, "requests": 2},
  "stopConditions": ["timeout", "unexpected-side-effect", "scope-drift"],
  "outputPaths": ["workspace/feature-login/debug"],
  "recordRequired": true,
  "notifyMainOn": ["boundary-hit"],
  "grantedBy": "user",
  "grantedAt": "2026-08-15T00:00:00Z",
  "expiresAt": "2999-01-01T00:00:00Z"
}`
			if err := os.WriteFile(profilePath, []byte(profile), 0o644); err != nil {
				t.Fatal(err)
			}

			pendingData, pending := runPublicEntrypointProductStatus(t, caseRoot, "feature-login")
			assertPublicEntrypointProductStatus(t, pendingData, fixture.entrypoint)
			if pending.CaseMission == nil || len(pending.CaseMission.PendingGateHandoffs) != 1 || len(pending.CaseMission.AuthorizedGateHandoffs) != 0 {
				t.Fatalf("pending gate status = %+v", pending.CaseMission)
			}
			handoff := pending.CaseMission.PendingGateHandoffs[0]
			if !strings.HasPrefix(handoff.WhatIfCommand, fixture.entrypoint+" ") || !strings.HasPrefix(handoff.ApplyCommand, fixture.entrypoint+" ") {
				t.Fatalf("pending gate commands use mixed entrypoint: %+v", handoff)
			}

			invocation, err := commands.ParsePublicInvocation(handoff.WhatIfCommand)
			if err != nil {
				t.Fatal(err)
			}
			previewArgs, err := invocation.CLIArgs()
			if err != nil {
				t.Fatal(err)
			}
			previewArgs = append(previewArgs,
				"-TargetRef", "target.bin",
				"-RuntimeSeconds", "30",
				"-DiskMB", "64",
				"-Requests", "1",
				"-OutputPaths", "workspace/feature-login/debug/session-1",
				"-StopConditions", "timeout,unexpected-side-effect,scope-drift",
				"-Format", "json",
			)
			out.Reset()
			if err := Run(previewArgs, &out); err != nil {
				t.Fatal(err)
			}
			assertGateProductResponseEntrypoint(t, "preview", out.Bytes(), fixture.entrypoint)
			var preview gate.Plan
			if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
				t.Fatalf("gate preview did not decode: %v\n%s", err, out.String())
			}
			if preview.EventPreview.Status != "authorized-gate" || preview.EventPreview.Gate.Authorization.Decision != "preauthorized" {
				t.Fatalf("gate preview did not authorize the bounded request: %+v", preview)
			}
			applyArgs := append([]string{}, previewArgs...)
			for index := range applyArgs {
				if applyArgs[index] == "-WhatIf" {
					applyArgs[index] = "-Apply"
				}
			}
			applyArgs = append(applyArgs, "-Actor", "mission-commander")
			out.Reset()
			if err := Run(applyArgs, &out); err != nil {
				t.Fatal(err)
			}
			assertGateProductResponseEntrypoint(t, "apply", out.Bytes(), fixture.entrypoint)
			var applied gate.ApplyResult
			if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
				t.Fatalf("gate Apply did not decode: %v\n%s", err, out.String())
			}
			if !applied.Applied || applied.Event == nil || applied.Event.Status != "authorized-gate" {
				t.Fatalf("bounded gate Apply = %+v", applied)
			}

			authorizedData, authorized := runPublicEntrypointProductStatus(t, caseRoot, "feature-login")
			assertPublicEntrypointProductStatus(t, authorizedData, fixture.entrypoint)
			if authorized.CaseMission == nil || len(authorized.CaseMission.AuthorizedGateHandoffs) != 1 {
				t.Fatalf("authorized gate status = %+v", authorized.CaseMission)
			}
			authorizedHandoff := authorized.CaseMission.AuthorizedGateHandoffs[0]
			if !strings.HasPrefix(authorizedHandoff.ReportContract, fixture.entrypoint+" ") || authorizedHandoff.LiveValidation == nil {
				t.Fatalf("authorized gate handoff uses mixed entrypoint or omitted validation: %+v", authorizedHandoff)
			}
		})
	}
}

func assertGateProductResponseEntrypoint(t *testing.T, label string, data []byte, entrypoint string) {
	t.Helper()
	other := commands.LegacyPublicEntrypoint
	if entrypoint == commands.LegacyPublicEntrypoint {
		other = commands.CurrentPublicEntrypoint
	}
	text := string(data)
	if !strings.Contains(text, entrypoint+" ") {
		t.Fatalf("gate %s response omitted public entrypoint %s", label, entrypoint)
	}
	if index := strings.Index(text, other+" "); index >= 0 {
		start := max(0, index-160)
		end := min(len(text), index+len(other)+200)
		t.Fatalf("gate %s response leaks %s near %q", label, other, text[start:end])
	}
}

func TestStatusDurableHandoffProductPathProjectsSelectedEntrypoint(t *testing.T) {
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := publicEntrypointProductCase(t, fixture, "handoff-entrypoint-projection")
			var out bytes.Buffer
			if err := Run([]string{
				"-Command", "start",
				"-Target", caseRoot,
				"-Pack", "_template",
				"-Name", "login",
				"-Apply",
				"-Format", "json",
			}, &out); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			if err := runHashBoundHandoffApply(t, []string{
				"-Command", "handoff",
				"-Target", caseRoot,
				"-Pack", "_template",
				"-Lane", "feature-login",
				"-Apply",
				"-Format", "json",
			}, &out); err != nil {
				t.Fatal(err)
			}

			statusData, status := runPublicEntrypointProductStatus(t, caseRoot, "feature-login")
			assertPublicEntrypointProductStatus(t, statusData, fixture.entrypoint)
			pkg := status.MissionControlRunbook.ReplacementExecutorTakeover
			if pkg == nil || !pkg.DurableArtifactFresh || pkg.DurableArtifactState != "fresh" {
				t.Fatalf("durable takeover package = %+v", pkg)
			}
			if err := mission.ValidateMissionCommanderDriverRequest(pkg.CurrentDriverRequest); err != nil {
				t.Fatalf("durable takeover request is invalid: %v", err)
			}
			if !strings.HasPrefix(pkg.CurrentDriverRequest.Command, fixture.entrypoint+" ") || !strings.HasPrefix(pkg.CurrentDriverRequest.ExpectedReceipt.RefreshStatusCommand, fixture.entrypoint+" ") {
				t.Fatalf("durable takeover request uses mixed entrypoint: %+v", pkg.CurrentDriverRequest)
			}
			if !strings.HasPrefix(filepath.ToSlash(pkg.DurableArtifactPath), fixture.stateDir+"/") {
				t.Fatalf("durable takeover path %q does not use %s", pkg.DurableArtifactPath, fixture.stateDir)
			}
		})
	}
}

func TestStatusCurrentLoopExternalMemberProductPathProjectsSelectedEntrypoint(t *testing.T) {
	for _, fixture := range publicEntrypointProductFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := publicEntrypointProductCase(t, fixture, "current-loop-entrypoint-projection")
			if err := Run([]string{
				"-Command", "overview",
				"-Target", caseRoot,
				"-Pack", "_template",
				"-Format", "json",
			}, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			board, err := mission.ReadBoard(caseRoot)
			if err != nil {
				t.Fatal(err)
			}
			if len(board.Lanes) == 0 {
				t.Fatal("overview omitted main lane")
			}
			board.Lanes[0].CurrentExecutor = "entrypoint-member"
			board.Lanes[0].ExecutorGeneration = 1
			board.Lanes[0].UpdatedAt = "2026-08-15T01:00:00Z"
			boardData, err := json.MarshalIndent(board, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			boardPath, err := projectstate.Join(caseRoot, "board.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(boardPath, append(boardData, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			lanePath, err := projectstate.Join(caseRoot, "lanes", "main", "lane.json")
			if err != nil {
				t.Fatal(err)
			}
			laneData, err := os.ReadFile(lanePath)
			if err != nil {
				t.Fatal(err)
			}
			var lane map[string]any
			if err := json.Unmarshal(laneData, &lane); err != nil {
				t.Fatal(err)
			}
			lane["currentExecutor"] = "entrypoint-member"
			lane["executorGeneration"] = 1
			lane["updatedAt"] = "2026-08-15T01:00:00Z"
			laneData, err = json.MarshalIndent(lane, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(lanePath, append(laneData, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}

			preview := runCurrentLoopPreviewWith(t, caseRoot, 2, "-Lane", "main")
			if preview.InitialCurrentStep == nil || preview.InitialCurrentStep.MemberExecution == nil {
				t.Fatalf("member dispatch preview missing: %+v", preview)
			}
			memberPlan := preview.InitialCurrentStep.MemberExecution
			published, err := memberexecution.Apply(*memberPlan, memberPlan.ExpectedPlanSHA256)
			if err != nil {
				t.Fatal(err)
			}
			if !published.Applied || published.AlreadyApplied {
				t.Fatalf("member dispatch fixture was not freshly published: %+v", published)
			}
			recovered := runCurrentLoopApplyWith(
				t,
				caseRoot,
				preview,
				"-Lane", "main",
				"-ExpectedMemberExecutionPlanSha256", memberPlan.ExpectedPlanSHA256,
			)
			if !recovered.Applied || recovered.StopReason.Code != "external-member-handoff" {
				t.Fatalf("current-loop external member result = %+v", recovered)
			}

			statusData, status := runPublicEntrypointProductStatus(t, caseRoot, "main")
			assertPublicEntrypointProductStatus(t, statusData, fixture.entrypoint)
			segment := status.MissionControlRunbook.CurrentLoopSegment
			operator := status.MissionControlRunbook.CurrentLoopOperator
			if segment == nil || !segment.Ready || segment.State != "ready" || segment.StopCode != "external-member-handoff" || operator == nil || operator.ExternalMemberHandoff == nil {
				t.Fatalf("current-loop status omitted durable external member handoff: segment=%+v operator=%+v", segment, operator)
			}
			if fixture.stateDir == projectstate.LegacyDir &&
				(operator.ExternalMemberHandoff.LaunchControl != nil ||
					operator.ExternalSessionJob == nil ||
					operator.State != "external-session-ready-for-attempt") {
				t.Fatalf("legacy nil-lineage external member handoff lost compatibility: %+v", operator)
			}
			if segment.ResumeDriverRequest != nil && !strings.HasPrefix(segment.ResumeDriverRequest.Command, fixture.entrypoint+" ") {
				t.Fatalf("current-loop resume request uses mixed entrypoint: %+v", segment.ResumeDriverRequest)
			}
		})
	}
}

func runPublicEntrypointProductStatus(t *testing.T, caseRoot, lane string) ([]byte, statusInventory) {
	t.Helper()
	args := []string{
		"-Command", "status",
		"-Target", caseRoot,
		"-Pack", "_template",
	}
	if strings.TrimSpace(lane) != "" {
		args = append(args, "-Lane", lane)
	}
	args = append(args, "-Format", "json")
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), out.Bytes()...)
	var status statusInventory
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
	}
	return data, status
}

func assertPublicEntrypointProductStatus(t *testing.T, data []byte, entrypoint string) {
	t.Helper()
	var public map[string]any
	if err := json.Unmarshal(data, &public); err != nil {
		t.Fatal(err)
	}
	typedRequests := 0
	for _, key := range []string{
		"caseShim",
		"packMemoryConsumption",
		"caseMission",
		"missionControlRunbook",
		"memberExecution",
	} {
		assertSelectedPublicEntrypoint(t, key, public[key], entrypoint)
		typedRequests += assertDeepProjectionDriverRequests(t, key, public[key], entrypoint)
	}
	if typedRequests == 0 {
		t.Fatal("real status omitted typed project-local driver requests")
	}
}
