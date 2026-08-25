package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

func TestRunCurrentHandoffPublishesFreshFinalDriverRequest(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "case")
	var out bytes.Buffer
	runInitApplyFromPreview(
		t,
		&out,
		"-Command", "init",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-ProjectName", "current-handoff-identity",
	)

	out.Reset()
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

	statusArgs := []string{
		"-Command", "status",
		"-Target", caseRoot,
		"-Pack", "_template",
		"-Lane", "feature-login",
		"-Format", "json",
	}
	before := runCurrentHandoffStatus(t, statusArgs)
	request := before.MissionControlRunbook.CurrentDriverRequest
	if request == nil || !strings.HasPrefix(request.Command, "/steamai ") || !strings.Contains(request.Command, "-Target "+caseRoot) || !strings.Contains(request.Command, "-Lane feature-login") || !strings.Contains(request.Command, "-WhatIf") || !strings.Contains(request.Command, "-Format json") {
		t.Fatalf("current status did not return the final qualified /steamai request: %+v", request)
	}
	if refresh := request.ExpectedReceipt.RefreshStatusCommand; !strings.HasPrefix(refresh, "/steamai status ") || !strings.Contains(refresh, "-Target "+caseRoot) || !strings.Contains(refresh, "-Lane feature-login") || !strings.Contains(refresh, "-Format compact-json") {
		t.Fatalf("current status request did not bind the selected compact refresh: %q", refresh)
	}
	requestSHA, err := mission.MissionCommanderDriverRequestSHA256(*request)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(requestSHA, before.MissionControlRunbook.CurrentDriverRequestSHA256) {
		t.Fatalf("current status request SHA mismatch: computed=%s returned=%s", requestSHA, before.MissionControlRunbook.CurrentDriverRequestSHA256)
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
	var applied struct {
		Applied                            bool                                        `json:"applied"`
		MissionCommanderActionQueue        mission.MissionCommanderActionQueue         `json:"missionCommanderActionQueue"`
		DailyMissionControlRunbook         *workstream.DailyMissionControlRunbook      `json:"dailyMissionControlRunbook"`
		ReplacementExecutorTakeoverPackage *mission.ReplacementExecutorTakeoverPackage `json:"replacementExecutorTakeoverPackage"`
	}
	if err := json.Unmarshal(out.Bytes(), &applied); err != nil {
		t.Fatalf("current handoff apply JSON did not decode: %v\n%s", err, out.String())
	}
	published := applied.MissionCommanderActionQueue.CurrentDriverRequest
	pkg := applied.ReplacementExecutorTakeoverPackage
	if !applied.Applied || published == nil || applied.DailyMissionControlRunbook == nil || applied.DailyMissionControlRunbook.CurrentDriverRequest == nil || pkg == nil {
		t.Fatalf("current handoff omitted final request publications: %+v", applied)
	}
	publishedSHA, err := mission.MissionCommanderDriverRequestSHA256(*published)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(publishedSHA, requestSHA) || applied.DailyMissionControlRunbook.CurrentDriverRequest.Command != request.Command || pkg.CurrentDriverRequest.Command != request.Command || !strings.EqualFold(pkg.CurrentDriverRequestSHA256, requestSHA) {
		t.Fatalf("current handoff drifted from fresh status identity: before=%+v queue=%+v runbook=%+v package=%+v", request, published, applied.DailyMissionControlRunbook.CurrentDriverRequest, pkg)
	}

	artifactPath, err := projectstate.Join(caseRoot, "handovers", "feature-login-latest-replacement-executor-takeover.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("current handoff artifact is missing: %v", err)
	}
	resumePath, err := projectstate.Join(caseRoot, "lanes", "feature-login", "prompts", "RESUME.md")
	if err != nil {
		t.Fatal(err)
	}
	resume, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resume), "driver request：") || !strings.Contains(string(resume), "command=`"+request.Command+"`") || !strings.Contains(string(resume), "refreshStatusCommand=`"+request.ExpectedReceipt.RefreshStatusCommand+"`") {
		t.Fatalf("current RESUME did not preserve the final request identity:\n%s", string(resume))
	}
	checkpointPath, err := projectstate.Join(caseRoot, "lanes", "feature-login", "checkpoints", "latest.json")
	if err != nil {
		t.Fatal(err)
	}
	checkpointBytes, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	var checkpoint struct {
		MissionCommanderActionQueue mission.MissionCommanderActionQueue `json:"missionCommanderActionQueue"`
	}
	if err := json.Unmarshal(checkpointBytes, &checkpoint); err != nil {
		t.Fatalf("current handoff checkpoint did not decode: %v\n%s", err, string(checkpointBytes))
	}
	checkpointRequest := checkpoint.MissionCommanderActionQueue.CurrentDriverRequest
	if checkpointRequest == nil || checkpointRequest.Command != request.Command || checkpointRequest.ExpectedReceipt.RefreshStatusCommand != request.ExpectedReceipt.RefreshStatusCommand {
		t.Fatalf("current checkpoint did not preserve the final request identity: %+v", checkpointRequest)
	}
	checkpointSHA, err := mission.MissionCommanderDriverRequestSHA256(*checkpointRequest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(checkpointSHA, requestSHA) {
		t.Fatalf("current checkpoint request SHA mismatch: got=%s want=%s", checkpointSHA, requestSHA)
	}

	after := runCurrentHandoffStatus(t, statusArgs)
	afterRunbook := after.MissionControlRunbook
	afterPkg := afterRunbook.ReplacementExecutorTakeoverPackage
	if afterRunbook.CurrentDriverRequest == nil || afterPkg == nil || !afterPkg.DurableArtifactFresh || afterPkg.DurableArtifactState != "fresh" || afterPkg.DurableArtifactPath != ".steamai/handovers/feature-login-latest-replacement-executor-takeover.json" || !strings.EqualFold(afterPkg.DurableArtifactRequestSHA256, afterRunbook.CurrentDriverRequestSHA256) || !strings.EqualFold(afterPkg.CurrentDriverRequestSHA256, afterRunbook.CurrentDriverRequestSHA256) || afterPkg.CurrentDriverRequest.Command != afterRunbook.CurrentDriverRequest.Command {
		t.Fatalf("fresh current status did not accept the durable handoff identity: requestSha=%s package=%+v", afterRunbook.CurrentDriverRequestSHA256, afterPkg)
	}
	for _, name := range []string{"authority.jsonl", "confirmed.jsonl"} {
		path, err := projectstate.Join(caseRoot, "facts", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("handoff wrote forbidden fact %s or stat failed: %v", name, err)
		}
	}

	if err := os.Mkdir(filepath.Join(caseRoot, projectstate.LegacyDir), 0o755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := Run(statusArgs, &out); err == nil || !strings.Contains(err.Error(), "both .steamai and .rekit") || out.Len() != 0 {
		t.Fatalf("dual-root status did not fail closed: err=%v stdout=%q", err, out.String())
	}
}

func TestStatusProjectsCaseCommandsToSelectedEntrypoint(t *testing.T) {
	fixtures := []struct {
		name       string
		stateDir   string
		entrypoint string
		setup      func(*testing.T) string
	}{
		{
			name:       "current",
			stateDir:   projectstate.CurrentDir,
			entrypoint: "/steamai",
			setup: func(t *testing.T) string {
				caseRoot := filepath.Join(t.TempDir(), "case")
				var out bytes.Buffer
				runInitApplyFromPreview(
					t,
					&out,
					"-Command", "init",
					"-Target", caseRoot,
					"-Pack", "_template",
					"-ProjectName", "current-command-projection",
				)
				return caseRoot
			},
		},
		{
			name:       "legacy",
			stateDir:   projectstate.LegacyDir,
			entrypoint: "/rekit",
			setup: func(t *testing.T) string {
				return fullAttachedCase(t)
			},
		},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			caseRoot := fixture.setup(t)
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
			if err := Run([]string{
				"-Command", "plan-subagents",
				"-Target", caseRoot,
				"-Pack", "_template",
				"-TaskType", "feature-analysis",
				"-Items", "alpha,beta",
				"-ItemsPerAgent", "1",
				"-MaxParallel", "2",
				"-Lane", "feature-login",
				"-Format", "json",
			}, &out); err != nil {
				t.Fatal(err)
			}
			out.Reset()
			if err := Run([]string{
				"-Command", "status",
				"-Target", caseRoot,
				"-Pack", "_template",
				"-Lane", "feature-login",
				"-Format", "json",
			}, &out); err != nil {
				t.Fatal(err)
			}
			var status map[string]any
			if err := json.Unmarshal(out.Bytes(), &status); err != nil {
				t.Fatalf("status JSON did not decode: %v\n%s", err, out.String())
			}
			typedRequestCount := 0
			for _, key := range []string{
				"caseShim",
				"packMemoryConsumption",
				"caseMission",
				"missionControlRunbook",
				"memberExecution",
			} {
				assertSelectedPublicEntrypoint(t, key, status[key], fixture.entrypoint)
				typedRequestCount += assertDeepProjectionDriverRequests(t, key, status[key], fixture.entrypoint)
			}
			if typedRequestCount == 0 {
				t.Fatal("real status omitted typed project-local driver requests")
			}
			var typed statusInventory
			if err := json.Unmarshal(out.Bytes(), &typed); err != nil {
				t.Fatalf("typed status JSON did not decode: %v\n%s", err, out.String())
			}
			if typed.CaseShim.Entrypoint == nil || typed.CaseShim.Entrypoint.CaseLocalFirstScreenCommand != fixture.entrypoint {
				t.Fatalf("case-local first screen = %+v, want %q", typed.CaseShim.Entrypoint, fixture.entrypoint)
			}
			request := typed.MissionControlRunbook.CurrentDriverRequest
			if request == nil {
				t.Fatal("real status omitted current driver request")
			}
			requestSHA, err := mission.MissionCommanderDriverRequestSHA256(*request)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.EqualFold(requestSHA, typed.MissionControlRunbook.CurrentDriverRequestSHA256) {
				t.Fatalf("real status request SHA mismatch: computed=%s returned=%s", requestSHA, typed.MissionControlRunbook.CurrentDriverRequestSHA256)
			}
			operator := typed.CaseMission.ReviewerDispatchIntakeSummary.OperatorPackage
			if operator == nil || operator.Wave == nil || len(operator.Wave.Shards) != 2 {
				t.Fatalf("real reviewer wave omitted source identity or shards: %+v", operator)
			}
			if fixture.name == "current" {
				if operator.Ready || !operator.Paused || operator.Wave.Ready || len(operator.Wave.SpawnWave) != 0 || operator.Wave.SnapshotSHA256 != "" {
					t.Fatalf("unassigned current lane exposed executable reviewer wave: %+v", operator)
				}
			} else if operator.Wave.SnapshotSHA256 == "" {
				t.Fatalf("legacy reviewer wave omitted snapshot identity: %+v", operator.Wave)
			}
			packetPath := filepath.ToSlash(operator.Wave.PacketPath)
			if !strings.Contains(packetPath, "/"+fixture.stateDir+"/") && !strings.HasPrefix(packetPath, fixture.stateDir+"/") {
				t.Fatalf("real reviewer wave packet path %q does not use %s", operator.Wave.PacketPath, fixture.stateDir)
			}
		})
	}
}

func assertSelectedPublicEntrypoint(t *testing.T, path string, value any, entrypoint string) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			childPath := path + "." + key
			if statusPublicCommandField(key) {
				assertSelectedPublicCommandValue(t, childPath, child, entrypoint)
			}
			assertSelectedPublicEntrypoint(t, childPath, child, entrypoint)
		}
	case []any:
		for index, child := range typed {
			assertSelectedPublicEntrypoint(t, fmt.Sprintf("%s[%d]", path, index), child, entrypoint)
		}
	}
}

func statusPublicCommandField(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return key == "command" || strings.HasSuffix(key, "command") || strings.HasSuffix(key, "commands")
}

func assertSelectedPublicCommandValue(t *testing.T, path string, value any, entrypoint string) {
	t.Helper()
	check := func(text string) {
		text = strings.TrimSpace(text)
		if !strings.HasPrefix(text, "/rekit ") && !strings.HasPrefix(text, "/steamai ") {
			return
		}
		if !strings.HasPrefix(text, entrypoint+" ") {
			t.Errorf("%s uses the wrong project entrypoint: %q; want %s", path, text, entrypoint)
		}
	}
	switch typed := value.(type) {
	case string:
		check(typed)
	case []any:
		for _, child := range typed {
			if text, ok := child.(string); ok {
				check(text)
			}
		}
	}
}

type currentHandoffStatus struct {
	MissionControlRunbook struct {
		CurrentDriverRequest               *mission.MissionCommanderDriverRequest      `json:"currentDriverRequest"`
		CurrentDriverRequestSHA256         string                                      `json:"currentDriverRequestSha256"`
		ReplacementExecutorTakeoverPackage *mission.ReplacementExecutorTakeoverPackage `json:"replacementExecutorTakeoverPackage"`
	} `json:"missionControlRunbook"`
}

func runCurrentHandoffStatus(t *testing.T, args []string) currentHandoffStatus {
	t.Helper()
	var out bytes.Buffer
	if err := Run(args, &out); err != nil {
		t.Fatal(err)
	}
	var status currentHandoffStatus
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("current handoff status JSON did not decode: %v\n%s", err, out.String())
	}
	return status
}
