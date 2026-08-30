package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/autonomy"
	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/executioncontrol"
	"github.com/shuiyu486/re-context-kits/internal/rekit/gate"
	"github.com/shuiyu486/re-context-kits/internal/rekit/memberexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
	"github.com/shuiyu486/re-context-kits/internal/rekit/workstream"
)

const (
	installedRealClaudeE2EEnv      = "STEAMAI_RUN_INSTALLED_REAL_CLAUDE_E2E"
	installedRealClaudeE2EModelEnv = "STEAMAI_INSTALLED_REAL_CLAUDE_E2E_MODEL"
	installedRealClaudeEvidence    = "fixtures/installed-e2e/feature-note.txt"
)

const installedRealClaudeEvidenceText = "Feature: installed-project-local-real-claude\nRequest ID: installed-e2e-harmless-note\nCandidate path: none\nEndpoint: local-only\nMethod: read-only\nImpact: none\nObservation: this harmless project-local note exists solely for installed real-Claude acceptance.\nRequired next action: report the inspected evidence without external effects.\n"

func TestInstalledProjectLocalRealClaudeE2E(t *testing.T) {
	if os.Getenv(installedRealClaudeE2EEnv) != "1" {
		t.Skip("set STEAMAI_RUN_INSTALLED_REAL_CLAUDE_E2E=1 to run the explicit installed real-Claude gate")
	}
	if runtime.GOOS != "windows" {
		t.Skip("installed trusted-Claude acceptance currently requires Windows")
	}

	repoRoot := selfContainedRepoRoot(t)
	testRoot := t.TempDir()
	centralDir := filepath.Join(testRoot, "central-bootstrap")
	centralExecutable := filepath.Join(centralDir, runtimebundle.ExecutableName())
	if err := os.MkdirAll(centralDir, 0o755); err != nil {
		t.Fatal(err)
	}
	selfContainedRun(t, repoRoot, "go", "build", "-o", centralExecutable, "./cmd/rekit")

	projectRoot := filepath.Join(testRoot, "installed-project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	preservedPath := filepath.Join(projectRoot, "ordinary-user-file.txt")
	if err := os.WriteFile(preservedPath, []byte("preserve installed project content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	previewData := selfContainedRun(
		t,
		repoRoot,
		centralExecutable,
		"runtime",
		"-Command", "init",
		"-Target", projectRoot,
		"-Pack", "_template",
		"-ProjectName", "installed-real-claude-e2e",
		"-WhatIf",
		"-Format", "json",
	)
	var preview struct {
		CaseRoot      string   `json:"caseRoot"`
		AdoptionReady bool     `json:"adoptionReady"`
		ApplyArgs     []string `json:"applyArgs"`
	}
	selfContainedDecode(t, previewData, &preview)
	if !sameSelfContainedPath(preview.CaseRoot, projectRoot) || !preview.AdoptionReady || len(preview.ApplyArgs) == 0 {
		t.Fatalf("installed real-Claude init preview is invalid: %+v", preview)
	}
	applyData := selfContainedRun(
		t,
		repoRoot,
		centralExecutable,
		append([]string{"runtime"}, preview.ApplyArgs...)...,
	)
	var applied struct {
		Applied bool `json:"applied"`
	}
	selfContainedDecode(t, applyData, &applied)
	if !applied.Applied {
		t.Fatalf("installed real-Claude init did not apply: %+v", applied)
	}

	localExecutable := filepath.Join(projectRoot, ".steamai", "runtime", "bin", runtimebundle.ExecutableName())
	if _, err := os.Stat(localExecutable); err != nil {
		t.Fatal(err)
	}
	assertInstalledSTeamAISkillBridge(t, repoRoot, projectRoot)

	nested := filepath.Join(projectRoot, "workspace", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	statusData := installedRealClaudeRun(
		t,
		2*time.Minute,
		nested,
		localExecutable,
		"runtime", "-Command", "status", "-Format", "json",
	)
	var status struct {
		Mode         string `json:"mode"`
		Target       string `json:"target"`
		TemplateRoot string `json:"templateRoot"`
		RuntimeRoot  string `json:"runtimeRoot"`
	}
	selfContainedDecode(t, statusData, &status)
	if status.Mode != "case-onboarding-required" || !sameSelfContainedPath(status.Target, projectRoot) ||
		!sameSelfContainedPath(status.TemplateRoot, filepath.Join(projectRoot, ".steamai")) ||
		!sameSelfContainedPath(status.RuntimeRoot, filepath.Join(projectRoot, ".steamai", "runtime")) {
		t.Fatalf("installed status did not bind the project-local runtime: %+v", status)
	}
	retiredCentralDir := filepath.Join(testRoot, "retired-central-bootstrap")
	if err := os.Rename(centralDir, retiredCentralDir); err != nil {
		t.Fatalf("retire central bootstrap namespace after project-local preflight: %v", err)
	}
	if _, err := os.Lstat(centralDir); !os.IsNotExist(err) {
		t.Fatalf("central bootstrap namespace remains at its bound path: %v", err)
	}

	model := strings.TrimSpace(os.Getenv(installedRealClaudeE2EModelEnv))
	if model == "" {
		model = "sonnet"
	}
	goal := "Inspect one harmless bounded project-local feature note. Acceptance requires reading and citing the exact note at " + installedRealClaudeEvidence + ". Always return a bounded analysis. If the note is absent or unreadable, state that this mandatory requirement is unmet so the independent Reviewer can reject it; do not invent content or perform external effects."
	firstData := installedRealClaudeRun(
		t,
		20*time.Minute,
		nested,
		localExecutable,
		"host", "-daily",
		"-goal", goal,
		"-model", model,
		"-timeout", "8m",
		"-max-attempts", "3",
	)
	var first sessionhost.DailyResult
	selfContainedDecode(t, firstData, &first)
	if !first.OnboardingApplied || first.Pack != "_template" || first.Lane == "" ||
		!first.Blocked || first.FinalState != "reviewer-rejected-awaiting-correction" ||
		first.SessionLaunches < 2 || first.SessionCompletions+first.Replacements != first.SessionLaunches || first.Failure != nil {
		t.Fatalf("installed first real-Claude pass did not reach canonical Reviewer rejection: %+v", first)
	}
	firstSessions := assertInstalledRealClaudeSessions(t, first, 1, 1)
	firstMember, ok, err := memberexecution.Latest(projectRoot, first.Lane)
	if err != nil || !ok || firstMember.TaskContext == nil || firstMember.Manifest == nil || firstMember.Owner.ExecutorGeneration != 1 {
		t.Fatalf("installed first member is not durably reviewable: found=%t inspection=%+v err=%v", ok, firstMember, err)
	}
	firstManifestRef := installedRealClaudeRelativePath(t, projectRoot, firstMember.ManifestPath)
	firstRejection, rejected, err := workstream.CurrentMemberManifestReviewerRejection(projectRoot, first.Lane, firstManifestRef)
	if err != nil || !rejected || firstRejection.ManifestSHA256 != firstMember.ManifestSHA256 || firstRejection.ReviewerSession == "" {
		t.Fatalf("installed first Reviewer rejection lacks exact manifest/session lineage: rejected=%t rejection=%+v err=%v", rejected, firstRejection, err)
	}

	evidencePath := filepath.Join(projectRoot, filepath.FromSlash(installedRealClaudeEvidence))
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, []byte(installedRealClaudeEvidenceText), 0o600); err != nil {
		t.Fatal(err)
	}
	correction := "Replace the rejected result using only the newly published bounded note at " + installedRealClaudeEvidence + ". Cite its exact feature, request ID, candidate path, endpoint, method, impact, observation, and required next action. Do not infer external facts or perform external effects."
	correctedData := installedRealClaudeRun(
		t,
		20*time.Minute,
		nested,
		localExecutable,
		"host", "-daily",
		"-lane", first.Lane,
		"-correction", correction,
		"-model", model,
		"-timeout", "8m",
		"-max-attempts", "3",
	)
	var corrected sessionhost.DailyResult
	selfContainedDecode(t, correctedData, &corrected)
	if corrected.CorrectionEventID == "" || corrected.ExecutorGeneration != 2 ||
		corrected.FinalState != "lane-closed" || corrected.Blocked || corrected.Failure != nil ||
		corrected.SessionLaunches < 2 || corrected.SessionCompletions+corrected.Replacements != corrected.SessionLaunches ||
		corrected.Completion == nil || !corrected.Completion.Applied || corrected.Completion.Lane.Status != "closed" {
		t.Fatalf("installed correction did not replace, review, and close the lane: %+v", corrected)
	}
	correctedSessions := assertInstalledRealClaudeSessions(t, corrected, 1, 1)
	secondMember, ok, err := memberexecution.Latest(projectRoot, first.Lane)
	if err != nil || !ok || secondMember.TaskContext == nil || secondMember.TaskContext.Correction == nil ||
		secondMember.Manifest == nil || secondMember.Owner.ExecutorGeneration != 2 || secondMember.ManifestSHA256 == firstMember.ManifestSHA256 {
		t.Fatalf("installed replacement member lacks correction/current-generation lineage: found=%t inspection=%+v err=%v", ok, secondMember, err)
	}
	secondManifestRef := installedRealClaudeRelativePath(t, projectRoot, secondMember.ManifestPath)
	acceptance, accepted, err := workstream.CurrentMemberManifestReviewerAcceptance(projectRoot, first.Lane, secondManifestRef)
	if err != nil || !accepted || acceptance.ManifestSHA256 != secondMember.ManifestSHA256 ||
		acceptance.ReviewerSession == "" || acceptance.ReviewerSession == firstRejection.ReviewerSession ||
		acceptance.OwnerGeneration != secondMember.Owner.ExecutorGeneration {
		t.Fatalf("installed replacement Reviewer acceptance lacks independent current lineage: accepted=%t acceptance=%+v err=%v", accepted, acceptance, err)
	}
	completion, err := workstream.InspectLaneCompletion(projectRoot, first.Lane)
	if err != nil || !completion.NoAuthority || !completion.NoConfirmed || !completion.NoHeavyTool || completion.Lane != first.Lane {
		t.Fatalf("installed lane completion weakened the safety boundary: completion=%+v err=%v", completion, err)
	}

	replayData := installedRealClaudeRun(
		t,
		2*time.Minute,
		nested,
		localExecutable,
		"host", "-daily", "-lane", first.Lane,
	)
	var replay sessionhost.DailyResult
	selfContainedDecode(t, replayData, &replay)
	if !replay.Replay || replay.FinalState != "lane-closed" || replay.SessionLaunches != 0 ||
		replay.SessionCompletions != 0 || len(replay.HostRuns) != 0 || replay.Failure != nil {
		t.Fatalf("installed terminal replay relaunched Claude or mutated lifecycle state: %+v", replay)
	}

	allSessions := append(append([]string{}, firstSessions...), correctedSessions...)
	seen := map[string]bool{}
	for _, sessionID := range allSessions {
		if seen[sessionID] {
			t.Fatalf("installed E2E reused Claude session %s", sessionID)
		}
		seen[sessionID] = true
		assertInstalledRealClaudeTranscript(t, sessionID)
	}
	if len(seen) < 4 {
		t.Fatalf("installed E2E completed only %d distinct real Claude sessions", len(seen))
	}

	preserved, err := os.ReadFile(preservedPath)
	if err != nil || string(preserved) != "preserve installed project content\n" {
		t.Fatalf("installed E2E changed ordinary project content: data=%q err=%v", preserved, err)
	}
	for name, data := range map[string][]byte{
		"status":    statusData,
		"first":     firstData,
		"corrected": correctedData,
		"replay":    replayData,
	} {
		assertSelfContainedOutputOmits(t, name, data, centralExecutable, centralDir, retiredCentralDir)
	}
}

func TestInstalledProjectLocalWebSecurityRealClaudeE2E(t *testing.T) {
	if os.Getenv(installedRealClaudeE2EEnv) != "1" {
		t.Skip("set STEAMAI_RUN_INSTALLED_REAL_CLAUDE_E2E=1 to run the explicit installed real-Claude gate")
	}
	if runtime.GOOS != "windows" {
		t.Skip("installed trusted-Claude acceptance currently requires Windows")
	}

	const (
		pack       = "web-security"
		lane       = "feature-mission"
		inputPath  = "inputs/openapi.json"
		outputPath = "workspace/features/feature-mission/openapi/installed-session-1"
	)
	repoRoot := selfContainedRepoRoot(t)
	testRoot := t.TempDir()
	centralDir := filepath.Join(testRoot, "central-web-bootstrap")
	centralExecutable := filepath.Join(centralDir, runtimebundle.ExecutableName())
	if err := os.MkdirAll(centralDir, 0o755); err != nil {
		t.Fatal(err)
	}
	selfContainedRun(t, repoRoot, "go", "build", "-o", centralExecutable, "./cmd/rekit")

	projectRoot := filepath.Join(testRoot, "installed-web-security-project")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	preservedPath := filepath.Join(projectRoot, "ordinary-user-file.txt")
	if err := os.WriteFile(preservedPath, []byte("preserve installed web-security project content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previewData := selfContainedRun(
		t, repoRoot, centralExecutable, "runtime", "-Command", "init", "-Target", projectRoot,
		"-Pack", pack, "-ProjectName", "installed-web-security-real-claude", "-WhatIf", "-Format", "json",
	)
	var initPreview struct {
		CaseRoot      string   `json:"caseRoot"`
		AdoptionReady bool     `json:"adoptionReady"`
		ApplyArgs     []string `json:"applyArgs"`
	}
	selfContainedDecode(t, previewData, &initPreview)
	if !sameSelfContainedPath(initPreview.CaseRoot, projectRoot) || !initPreview.AdoptionReady || len(initPreview.ApplyArgs) == 0 {
		t.Fatalf("installed web-security init preview is invalid: %+v", initPreview)
	}
	applyData := selfContainedRun(t, repoRoot, centralExecutable, append([]string{"runtime"}, initPreview.ApplyArgs...)...)
	var initApplied struct {
		Applied bool `json:"applied"`
	}
	selfContainedDecode(t, applyData, &initApplied)
	if !initApplied.Applied {
		t.Fatalf("installed web-security init did not apply: %+v", initApplied)
	}
	localExecutable := filepath.Join(projectRoot, ".steamai", "runtime", "bin", runtimebundle.ExecutableName())
	if _, err := os.Stat(localExecutable); err != nil {
		t.Fatal(err)
	}
	assertInstalledSTeamAISkillBridge(t, repoRoot, projectRoot)
	nested := filepath.Join(projectRoot, "workspace", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	statusData := installedRealClaudeRun(t, 2*time.Minute, nested, localExecutable, "runtime", "-Command", "status", "-Format", "json")
	var status struct {
		Mode, Target, TemplateRoot, RuntimeRoot, Pack string
	}
	selfContainedDecode(t, statusData, &status)
	if status.Mode != "case-onboarding-required" || status.Pack != pack || !sameSelfContainedPath(status.Target, projectRoot) ||
		!sameSelfContainedPath(status.TemplateRoot, filepath.Join(projectRoot, ".steamai")) ||
		!sameSelfContainedPath(status.RuntimeRoot, filepath.Join(projectRoot, ".steamai", "runtime")) {
		t.Fatalf("installed web-security status did not bind the project-local runtime: %+v", status)
	}
	retiredCentralDir := filepath.Join(testRoot, "retired-central-web-bootstrap")
	retireInstalledBootstrap(t, centralDir, retiredCentralDir)

	goal := "Inspect only the reviewed secret-free OpenAPI inventory that Mission Control binds into the task context. Report the exact server, auth-scheme, endpoint, parameter, media-type, warning, receipt, and evidence-review lineage available in that binding. Do not make a network request, execute a replay, infer response bytes, read credentials, or perform external effects."
	onboardPreviewData := installedRealClaudeRun(
		t, 2*time.Minute, nested, localExecutable, "runtime", "-Command", "onboard", "-Target", projectRoot,
		"-Pack", pack, "-ProjectName", "installed-web-security-real-claude", "-Goal", goal,
		"-Actor", "installed-web-security-e2e", "-Executor", "installed-web-member", "-InitialLane", lane,
		"-WhatIf", "-Format", "json",
	)
	var onboardPreview struct {
		ApplyArgs []string `json:"applyArgs"`
	}
	selfContainedDecode(t, onboardPreviewData, &onboardPreview)
	if len(onboardPreview.ApplyArgs) == 0 {
		t.Fatalf("installed web-security onboard preview omitted exact Apply args: %s", onboardPreviewData)
	}
	onboardApplyData := installedRealClaudeRun(t, 2*time.Minute, nested, localExecutable, append([]string{"runtime"}, onboardPreview.ApplyArgs...)...)
	var onboardApplied struct {
		Applied bool `json:"applied"`
	}
	selfContainedDecode(t, onboardApplyData, &onboardApplied)
	if !onboardApplied.Applied {
		t.Fatalf("installed web-security onboarding did not apply: %+v", onboardApplied)
	}
	installedWebSecurityRunCurrentDriverRequest(t, nested, localExecutable, projectRoot, pack, "overview")
	installedWebSecurityRunCurrentDriverRequest(t, nested, localExecutable, projectRoot, pack, "start")
	control, err := executioncontrol.Inspect(projectRoot, lane)
	if err != nil || control.Pending || control.State != executioncontrol.StateRunning {
		t.Fatalf("installed web-security lane lacks running execution control: %+v err=%v", control, err)
	}

	openAPIData := []byte(`{"openapi":"3.0.3","servers":[{"url":"http://127.0.0.1:18080/api"}],"paths":{"/health":{"get":{"operationId":"health","responses":{"200":{"description":"ok"}}}}},"components":{"securitySchemes":{}}}`)
	openAPIPath := filepath.Join(projectRoot, filepath.FromSlash(inputPath))
	if err := os.MkdirAll(filepath.Dir(openAPIPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(openAPIPath, openAPIData, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	profileArgs := []string{
		"runtime", "-Command", "gate", "-Target", projectRoot, "-Pack", pack, "-Format", "json",
		"-ProvisionProfile", "-Lane", lane, "-Action", "inspect", "-TargetRef", inputPath,
		"-ProfileId", "installed-web-openapi", "-ProfileGrantedBy", "installed-web-security-e2e",
		"-ProfileGrantedAt", now.Add(-time.Minute).Format(time.RFC3339),
		"-ProfileExpiresAt", now.Add(13 * time.Minute).Format(time.RFC3339),
		"-RuntimeSeconds", "10", "-DiskMB", "4", "-Requests", "1",
		"-StopConditions", "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
		"-OutputPaths", outputPath,
	}
	profilePreviewData := installedRealClaudeRun(t, 2*time.Minute, nested, localExecutable, profileArgs...)
	var profilePreview autonomy.ProfileMutationPlan
	selfContainedDecode(t, profilePreviewData, &profilePreview)
	if profilePreview.ExpectedPlanSHA256 == "" || profilePreview.PlannedProfile.Mode != autonomy.ModePreauthorized {
		t.Fatalf("installed web-security profile preview is invalid: %+v", profilePreview)
	}
	profileApplyArgs := append(append([]string{}, profileArgs...), "-Apply", "-ExpectedProfilePlanSha256", profilePreview.ExpectedPlanSHA256)
	profileApplyData := installedRealClaudeRun(t, 2*time.Minute, nested, localExecutable, profileApplyArgs...)
	var profileApplied autonomy.ProfileMutationResult
	selfContainedDecode(t, profileApplyData, &profileApplied)
	if !profileApplied.Applied || profileApplied.ProfileSHA256 != profilePreview.PlannedProfileSHA256 {
		t.Fatalf("installed web-security profile did not apply exact preview: %+v", profileApplied)
	}

	gateData := installedRealClaudeRun(
		t, 2*time.Minute, nested, localExecutable, "runtime", "-Command", "gate", "-Target", projectRoot,
		"-Pack", pack, "-Format", "json", "-Apply", "-Action", "inspect", "-Lane", lane,
		"-Actor", "installed-web-security-e2e", "-Subject", "bounded OpenAPI inventory",
		"-Summary", "typed compiled-in adapter evidence", "-TargetRef", inputPath,
		"-RuntimeSeconds", "10", "-DiskMB", "4", "-Requests", "1",
		"-StopConditions", "scope-drift,source-drift,output-exceeds-bounded-evidence-packet",
		"-OutputPaths", outputPath,
	)
	var authorized gate.ApplyResult
	selfContainedDecode(t, gateData, &authorized)
	if !authorized.Applied || authorized.Event == nil || authorized.Event.Status != "authorized-gate" ||
		authorized.Event.Gate.Authorization.Decision != autonomy.DecisionPreauthorized {
		t.Fatalf("installed web-security exact gate is not authorized: %+v", authorized)
	}

	model := strings.TrimSpace(os.Getenv(installedRealClaudeE2EModelEnv))
	if model == "" {
		model = "sonnet"
	}
	firstData := installedRealClaudeRun(
		t, 20*time.Minute, nested, localExecutable, "host", "-daily", "-lane", lane,
		"-model", model, "-timeout", "8m", "-max-attempts", "3",
	)
	var first sessionhost.DailyResult
	selfContainedDecode(t, firstData, &first)
	web := first.WebSecurityAdapter
	if web == nil || web.AdapterID != websecurity.InventoryAdapterID || !web.ReadyForMember || web.State != "ready-for-member" ||
		web.ExecutionStatus != "succeeded" || web.ExecutionExitStatus != "completed" || web.EvidenceReviewDecision != "accepted" ||
		web.EvidenceReviewSession == "" || !web.ChildLaunched || web.AdapterReplay || web.EvidenceReviewReplay || web.Run == nil ||
		web.Run.AdapterID != websecurity.InventoryAdapterID || !web.Run.NoNetwork || web.Run.PacketPath == "" ||
		first.Pack != pack || first.Lane != lane || first.Blocked || first.Failure != nil ||
		first.FinalState != "lane-closed" || first.Completion == nil || !first.Completion.Applied ||
		first.SessionLaunches < 2 || first.SessionCompletions+first.Replacements != first.SessionLaunches {
		t.Fatalf("installed web-security production lifecycle did not execute, review, bind, and close: %+v", first)
	}
	memberSessions := assertInstalledRealClaudeSessions(t, first, 1, 1)
	member, found, err := memberexecution.Latest(projectRoot, lane)
	if err != nil || !found || member.TaskContext == nil || member.TaskContext.Binding == nil ||
		member.TaskContext.Binding.Kind != "web-security-openapi-inventory-evidence" ||
		member.TaskContext.Binding.Values["gate-event-id"] != authorized.EventID ||
		member.TaskContext.Binding.Values["artifact-path"] != web.Run.PacketPath ||
		member.TaskContext.Binding.Values["artifact-sha256"] != web.Run.PacketSHA256 ||
		member.TaskContext.Binding.Values["evidence-review-closure-sha256"] != web.ClosureSHA256 ||
		member.Owner.ExecutorGeneration != 1 {
		t.Fatalf("installed web-security member lacks reviewed owner-generation binding: found=%t member=%+v err=%v", found, member, err)
	}
	binding, bindingPath, bindingSHA, err := memberexecution.ReadTaskBindingForOwner(projectRoot, lane, member.Owner.ExecutorGeneration)
	if err != nil || binding == nil || binding.Kind != member.TaskContext.Binding.Kind || bindingPath != web.TaskBindingPath ||
		!strings.EqualFold(bindingSHA, web.TaskBindingSHA256) {
		t.Fatalf("installed web-security durable task binding drifted: binding=%+v path=%s sha=%s err=%v", binding, bindingPath, bindingSHA, err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".steamai", "facts", "authority.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("installed web-security lifecycle wrote authority facts: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".steamai", "facts", "confirmed.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("installed web-security lifecycle wrote confirmed facts: %v", err)
	}

	replayData := installedRealClaudeRun(t, 2*time.Minute, nested, localExecutable, "host", "-daily", "-lane", lane)
	var replay sessionhost.DailyResult
	selfContainedDecode(t, replayData, &replay)
	if !replay.Replay || replay.FinalState != "lane-closed" || replay.SessionLaunches != 0 || replay.SessionCompletions != 0 ||
		len(replay.HostRuns) != 0 || replay.WebSecurityAdapter != nil || replay.Failure != nil {
		t.Fatalf("installed web-security terminal replay relaunched adapter or Claude: %+v", replay)
	}

	allSessions := append([]string{web.EvidenceReviewSession}, memberSessions...)
	seen := map[string]bool{}
	for _, sessionID := range allSessions {
		if sessionID == "" || seen[sessionID] {
			t.Fatalf("installed web-security E2E reused or omitted Claude session %q", sessionID)
		}
		seen[sessionID] = true
		assertInstalledRealClaudeTranscript(t, sessionID)
	}
	if len(seen) < 3 {
		t.Fatalf("installed web-security E2E completed only %d distinct real Claude sessions", len(seen))
	}
	preserved, err := os.ReadFile(preservedPath)
	if err != nil || string(preserved) != "preserve installed web-security project content\n" {
		t.Fatalf("installed web-security E2E changed ordinary project content: data=%q err=%v", preserved, err)
	}
	for name, data := range map[string][]byte{
		"status": statusData, "onboard-preview": onboardPreviewData, "onboard-apply": onboardApplyData,
		"profile-preview": profilePreviewData, "profile-apply": profileApplyData, "gate": gateData,
		"first": firstData, "replay": replayData,
	} {
		assertSelfContainedOutputOmits(t, name, data, centralExecutable, centralDir, retiredCentralDir)
		assertSelfContainedOutputOmitsLegacyEntrypoint(t, name, data)
	}
}

func retireInstalledBootstrap(t *testing.T, source, target string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.Rename(source, target)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("retire central bootstrap namespace: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if _, err := os.Lstat(source); !os.IsNotExist(err) {
		t.Fatalf("central bootstrap namespace remains at its bound path: %v", err)
	}
}

func installedWebSecurityRunCurrentDriverRequest(
	t *testing.T,
	dir,
	executable,
	projectRoot,
	pack,
	command string,
) {
	t.Helper()
	statusData := installedRealClaudeRun(
		t, 2*time.Minute, dir, executable, "runtime", "-Command", "status", "-Target", projectRoot,
		"-Pack", pack, "-Format", "json",
	)
	var status struct {
		MissionControlRunbook *struct {
			CurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
		} `json:"missionControlRunbook"`
	}
	selfContainedDecode(t, statusData, &status)
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		t.Fatalf("installed web-security status omitted current %s request: %s", command, statusData)
	}
	request := status.MissionControlRunbook.CurrentDriverRequest
	if err := mission.ValidateMissionCommanderDriverRequest(*request); err != nil || request.Invocation == nil ||
		request.Invocation.Command != command || !request.CommandExecutable {
		t.Fatalf("installed web-security %s request is invalid: %+v err=%v", command, request, err)
	}
	args, err := request.Invocation.CLIArgs()
	if err != nil {
		t.Fatal(err)
	}
	firstData := installedRealClaudeRun(t, 2*time.Minute, dir, executable, append([]string{"runtime"}, args...)...)
	if command == commands.Overview {
		return
	}
	var preview struct {
		MissionCommanderActionQueue struct {
			CurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
		} `json:"missionCommanderActionQueue"`
	}
	selfContainedDecode(t, firstData, &preview)
	applyRequest := preview.MissionCommanderActionQueue.CurrentDriverRequest
	if applyRequest == nil || applyRequest.Invocation == nil || !applyRequest.CommandExecutable ||
		applyRequest.Invocation.Command != command || !applyRequest.Invocation.HasFlag("-Apply") {
		t.Fatalf("installed web-security %s preview omitted exact Apply request: %+v", command, applyRequest)
	}
	applyArgs, err := applyRequest.Invocation.CLIArgs()
	if err != nil {
		t.Fatal(err)
	}
	applyArgs = append(applyArgs, "-Target", projectRoot, "-Pack", pack, "-Format", "json")
	installedRealClaudeRun(t, 2*time.Minute, dir, executable, append([]string{"runtime"}, applyArgs...)...)
}

func installedRealClaudeRun(t *testing.T, timeout time.Duration, dir, executable string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = dir
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("run installed %s %v: %v\nstdout:\n%s\nstderr:\n%s", executable, args, err, stdout.String(), stderr.String())
	}
	return append([]byte(nil), stdout.Bytes()...)
}

func assertInstalledRealClaudeSessions(t *testing.T, result sessionhost.DailyResult, wantMembers, wantReviewers int) []string {
	t.Helper()
	members := 0
	reviewers := 0
	sessions := []string{}
	for _, hostRun := range result.HostRuns {
		for _, session := range hostRun.Sessions {
			if !session.Started || session.SessionID == "" || session.Failure != nil || session.Outcome == "" {
				t.Fatalf("installed real-Claude session is not a successful exact launch: %+v", session)
			}
			switch session.SessionKind {
			case "member":
				members++
			case "reviewer":
				reviewers++
			default:
				t.Fatalf("installed real-Claude E2E launched unexpected session kind %q", session.SessionKind)
			}
			sessions = append(sessions, session.SessionID)
		}
	}
	if members < wantMembers || reviewers < wantReviewers || len(sessions) != result.SessionLaunches {
		t.Fatalf("installed real-Claude session inventory drifted: members=%d reviewers=%d sessions=%d result=%+v", members, reviewers, len(sessions), result)
	}
	return sessions
}

func installedRealClaudeRelativePath(t *testing.T, root, path string) string {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(rel)
}

func assertInstalledRealClaudeTranscript(t *testing.T, sessionID string) {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	projectsRoot := filepath.Join(home, ".claude", "projects")
	matches := []string{}
	err = filepath.WalkDir(projectsRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == sessionID+".jsonl" {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("locate exact Claude transcript %s: %v", sessionID, err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one exact Claude transcript for %s, found %d", sessionID, len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || len(data) > 32<<20 {
		t.Fatalf("Claude transcript %s is empty or unbounded: %d bytes", sessionID, len(data))
	}
	prompts := installedRealClaudeBoundPrompts(data)
	if len(prompts) == 0 {
		t.Fatalf("Claude transcript %s omitted the host-injected bound input", sessionID)
	}
	for _, prompt := range prompts {
		for _, forbidden := range []string{
			"Read the immutable task input at the exact absolute path",
			"using the Read tool before answering",
			"exact absolute path",
		} {
			if strings.Contains(prompt, forbidden) {
				t.Fatalf("Claude transcript %s retained hand-copied bound input guidance %q", sessionID, forbidden)
			}
		}
		_, encoded, present := strings.Cut(prompt, "boundInput=")
		if !present {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(encoded))
		var bound map[string]any
		if err := decoder.Decode(&bound); err != nil {
			t.Fatalf("decode transcript bound input for %s: %v", sessionID, err)
		}
		if _, present := bound["path"]; present {
			t.Fatalf("Claude transcript %s exposed a model-operated bound input path", sessionID)
		}
		content, contentOK := bound["content"].(string)
		sha, shaOK := bound["sha256"].(string)
		bytesValue, bytesOK := bound["bytes"].(float64)
		sum := sha256.Sum256([]byte(content))
		if !contentOK || content == "" || !shaOK || !strings.EqualFold(sha, hex.EncodeToString(sum[:])) ||
			!bytesOK || int(bytesValue) != len([]byte(content)) {
			t.Fatalf("Claude transcript %s contains an invalid inline bound input envelope: %+v", sessionID, bound)
		}
	}
}

func installedRealClaudeBoundPrompts(data []byte) []string {
	prompts := []string{}
	for line := range bytes.SplitSeq(data, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var row struct {
			Message struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &row) != nil || row.Message.Role != "user" {
			continue
		}
		installedRealClaudeCollectBoundPrompts(row.Message.Content, &prompts)
	}
	return prompts
}

func installedRealClaudeCollectBoundPrompts(value any, prompts *[]string) {
	switch typed := value.(type) {
	case string:
		if strings.Contains(typed, "boundInput=") {
			*prompts = append(*prompts, typed)
		}
	case []any:
		for _, item := range typed {
			installedRealClaudeCollectBoundPrompts(item, prompts)
		}
	case map[string]any:
		for _, item := range typed {
			installedRealClaudeCollectBoundPrompts(item, prompts)
		}
	}
}
