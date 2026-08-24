package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/missionintent"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/sourceartifact"
)

func TestSelfContainedCopiedProjectRunsWithoutCentralKit(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("ordinary in-place init is currently supported on Windows")
	}

	repoRoot := selfContainedRepoRoot(t)
	testRoot := t.TempDir()
	centralDir := filepath.Join(testRoot, "central")
	centralExecutable := filepath.Join(centralDir, runtimebundle.ExecutableName())
	if err := os.MkdirAll(centralDir, 0o755); err != nil {
		t.Fatal(err)
	}
	selfContainedRun(t, repoRoot, "go", "build", "-o", centralExecutable, "./cmd/rekit")

	sourceProject := filepath.Join(testRoot, "source-project")
	if err := os.MkdirAll(sourceProject, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceProject, "user-note.txt"), []byte("preserve me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	previewData := selfContainedRun(
		t,
		repoRoot,
		centralExecutable,
		"runtime",
		"-Command", "init",
		"-Target", sourceProject,
		"-Pack", "_template",
		"-ProjectName", "copied-e2e",
		"-WhatIf",
		"-Format", "json",
	)
	var preview struct {
		Command       string   `json:"command"`
		CaseRoot      string   `json:"caseRoot"`
		AdoptionReady bool     `json:"adoptionReady"`
		ApplyArgs     []string `json:"applyArgs"`
	}
	selfContainedDecode(t, previewData, &preview)
	if preview.Command != "init" || !sameSelfContainedPath(preview.CaseRoot, sourceProject) || !preview.AdoptionReady || len(preview.ApplyArgs) == 0 {
		t.Fatalf("unexpected init preview: %+v", preview)
	}
	applyData := selfContainedRun(
		t,
		repoRoot,
		centralExecutable,
		append([]string{"runtime"}, preview.ApplyArgs...)...,
	)
	var applied struct {
		Command  string `json:"command"`
		CaseRoot string `json:"caseRoot"`
		Applied  bool   `json:"applied"`
	}
	selfContainedDecode(t, applyData, &applied)
	if applied.Command != "init" || !applied.Applied || !sameSelfContainedPath(applied.CaseRoot, sourceProject) {
		t.Fatalf("unexpected init apply result: %+v", applied)
	}

	copiedProject := filepath.Join(testRoot, "copied-project")
	if err := copySelfContainedTree(sourceProject, copiedProject); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(sourceProject); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(centralDir); err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{sourceProject, centralExecutable} {
		if _, err := os.Lstat(removed); !os.IsNotExist(err) {
			t.Fatalf("removed central dependency remains: %s: %v", removed, err)
		}
	}

	nested := filepath.Join(copiedProject, "workspace", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	localExecutable := filepath.Join(copiedProject, ".steamai", "runtime", "bin", runtimebundle.ExecutableName())
	if _, err := os.Stat(localExecutable); err != nil {
		t.Fatal(err)
	}

	statusData := selfContainedRun(t, nested, localExecutable, "runtime", "-Command", "status", "-Format", "json")
	var status struct {
		Command        string `json:"command"`
		RuntimeRoot    string `json:"runtimeRoot"`
		TemplateRoot   string `json:"templateRoot"`
		Pack           string `json:"pack"`
		Target         string `json:"target"`
		TargetProvided bool   `json:"targetProvided"`
		Mode           string `json:"mode"`
		CaseShim       struct {
			Ready                bool  `json:"ready"`
			InstalledShimMatches *bool `json:"installedShimMatchesTemplate"`
		} `json:"caseShim"`
	}
	selfContainedDecode(t, statusData, &status)
	if status.Command != "status" || status.Mode != "case" || status.Pack != "_template" || status.TargetProvided ||
		!sameSelfContainedPath(status.Target, copiedProject) ||
		!sameSelfContainedPath(status.TemplateRoot, filepath.Join(copiedProject, ".steamai")) ||
		!sameSelfContainedPath(status.RuntimeRoot, filepath.Join(copiedProject, ".steamai", "runtime")) ||
		!status.CaseShim.Ready || status.CaseShim.InstalledShimMatches == nil || !*status.CaseShim.InstalledShimMatches {
		t.Fatalf("copied project status did not use its local bundle and installed skill: %+v", status)
	}
	assertInstalledSTeamAISkillBridge(t, repoRoot, copiedProject)

	packsData := selfContainedRun(t, nested, localExecutable, "runtime", "-Command", "packs", "-Format", "json")
	var packs struct {
		Command   string `json:"command"`
		PackCount int    `json:"packCount"`
		Packs     []struct {
			ID string `json:"id"`
		} `json:"packs"`
	}
	selfContainedDecode(t, packsData, &packs)
	foundTemplate := false
	for _, pack := range packs.Packs {
		foundTemplate = foundTemplate || pack.ID == "_template"
	}
	if packs.Command != "packs" || packs.PackCount != len(packs.Packs) || packs.PackCount == 0 || !foundTemplate {
		t.Fatalf("copied project pack inventory is invalid: %+v", packs)
	}

	doctorData := selfContainedRun(t, nested, localExecutable, "runtime", "-Command", "doctor", "-Format", "json")
	var doctor struct {
		Command string `json:"command"`
		Pack    string `json:"pack"`
		Target  string `json:"target"`
		Mode    string `json:"mode"`
		Valid   bool   `json:"valid"`
	}
	selfContainedDecode(t, doctorData, &doctor)
	if doctor.Command != "doctor" || doctor.Mode != "case" || doctor.Pack != "_template" || !doctor.Valid || !sameSelfContainedPath(doctor.Target, copiedProject) {
		t.Fatalf("copied project doctor result is invalid: %+v", doctor)
	}

	dailyCommand := exec.Command(
		localExecutable,
		"host",
		"-daily",
		"-target",
		copiedProject,
	)
	dailyCommand.Dir = nested
	var dailyStdout, dailyStderr bytes.Buffer
	dailyCommand.Stdout = &dailyStdout
	dailyCommand.Stderr = &dailyStderr
	dailyErr := dailyCommand.Run()
	var daily struct {
		Failure struct {
			Detail string `json:"detail"`
		} `json:"failure"`
		Action struct {
			Code string `json:"code"`
		} `json:"action"`
		SessionLaunches    int `json:"sessionLaunches"`
		SessionCompletions int `json:"sessionCompletions"`
		Replacements       int `json:"replacements"`
	}
	selfContainedDecode(t, dailyStdout.Bytes(), &daily)
	if dailyErr == nil ||
		!strings.Contains(dailyStderr.String(), "requires -goal <natural-language goal>") ||
		!strings.Contains(daily.Failure.Detail, "requires -goal <natural-language goal>") ||
		daily.Action.Code != "failed" || daily.SessionLaunches != 0 ||
		daily.SessionCompletions != 0 || daily.Replacements != 0 {
		t.Fatalf(
			"copied project daily front door did not stop before Claude: err=%v daily=%+v stderr=%q",
			dailyErr,
			daily,
			dailyStderr.String(),
		)
	}
	dailyData := append(dailyStdout.Bytes(), dailyStderr.Bytes()...)

	for name, data := range map[string][]byte{
		"status": statusData,
		"packs":  packsData,
		"doctor": doctorData,
		"daily":  dailyData,
	} {
		assertSelfContainedOutputOmits(t, name, data, sourceProject, centralExecutable)
	}

	onboardPreviewData := selfContainedRun(
		t,
		nested,
		localExecutable,
		"runtime",
		"-Command", "onboard",
		"-Target", copiedProject,
		"-Pack", "_template",
		"-ProjectName", "copied-e2e",
		"-Goal", "inspect harmless fixture",
		"-Actor", "operator",
		"-Executor", "executor-a",
		"-InitialLane", "feature-login",
		"-WhatIf",
		"-Format", "json",
	)
	var onboardPreview struct {
		CaseRoot  string   `json:"caseRoot"`
		ProjectID string   `json:"projectId"`
		ApplyArgs []string `json:"applyArgs"`
	}
	selfContainedDecode(t, onboardPreviewData, &onboardPreview)
	if !sameSelfContainedPath(onboardPreview.CaseRoot, copiedProject) || len(onboardPreview.ProjectID) != 16 || len(onboardPreview.ApplyArgs) == 0 {
		t.Fatalf("unexpected project-local onboard preview: %+v", onboardPreview)
	}
	onboardApplyData := selfContainedRun(
		t,
		nested,
		localExecutable,
		append([]string{"runtime"}, onboardPreview.ApplyArgs...)...,
	)
	var onboardApply struct {
		CaseRoot   string                   `json:"caseRoot"`
		ProjectID  string                   `json:"projectId"`
		Applied    bool                     `json:"applied"`
		Replay     bool                     `json:"replay"`
		Inspection missionintent.Inspection `json:"inspection"`
	}
	selfContainedDecode(t, onboardApplyData, &onboardApply)
	if !onboardApply.Applied || onboardApply.Replay || onboardApply.ProjectID != onboardPreview.ProjectID ||
		!sameSelfContainedPath(onboardApply.CaseRoot, copiedProject) || !onboardApply.Inspection.Committed ||
		onboardApply.Inspection.Identity.SchemaVersion != 2 || onboardApply.Inspection.Identity.Target != "." ||
		onboardApply.Inspection.Identity.ProjectID != onboardPreview.ProjectID || onboardApply.Inspection.ProjectBinding == nil ||
		onboardApply.Inspection.ProjectBinding.ProjectID != onboardPreview.ProjectID {
		t.Fatalf("project-local onboard Apply did not commit v2 identity: %+v", onboardApply)
	}

	movedProject := filepath.Join(testRoot, "moved-project")
	if err := os.Rename(copiedProject, movedProject); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(copiedProject); !os.IsNotExist(err) {
		t.Fatalf("pre-move physical root remains: %s: %v", copiedProject, err)
	}
	movedNested := filepath.Join(movedProject, "workspace", "nested")
	movedExecutable := filepath.Join(movedProject, ".steamai", "runtime", "bin", runtimebundle.ExecutableName())
	replayArgs := retargetSelfContainedApplyArgs(t, onboardPreview.ApplyArgs, movedProject)
	replayData := selfContainedRun(t, movedNested, movedExecutable, append([]string{"runtime"}, replayArgs...)...)
	var replay struct {
		CaseRoot   string                   `json:"caseRoot"`
		ProjectID  string                   `json:"projectId"`
		Applied    bool                     `json:"applied"`
		Replay     bool                     `json:"replay"`
		Inspection missionintent.Inspection `json:"inspection"`
	}
	selfContainedDecode(t, replayData, &replay)
	if replay.Applied || !replay.Replay || !sameSelfContainedPath(replay.CaseRoot, movedProject) ||
		replay.ProjectID != onboardPreview.ProjectID || !replay.Inspection.Committed ||
		replay.Inspection.Identity != onboardApply.Inspection.Identity || replay.Inspection.ProjectBinding == nil ||
		replay.Inspection.ProjectBinding.ProjectID != onboardPreview.ProjectID {
		t.Fatalf("moved project did not replay the same v2 identity: %+v", replay)
	}
	movedOutputs := assertMovedSelfContainedProjectHealth(t, movedNested, movedExecutable, movedProject, onboardPreview.ProjectID)
	movedOutputs["onboard-replay"] = replayData
	for name, data := range movedOutputs {
		assertSelfContainedOutputOmits(t, name, data, sourceProject, centralExecutable, copiedProject)
	}

	preserved, err := os.ReadFile(filepath.Join(movedProject, "user-note.txt"))
	if err != nil || string(preserved) != "preserve me\n" {
		t.Fatalf("moved project did not preserve the ordinary directory content: data=%q err=%v", preserved, err)
	}
	assertSelfContainedTypedCommandBridge(t, movedNested, movedExecutable, movedProject, sourceProject, centralExecutable, copiedProject)
}

func retargetSelfContainedApplyArgs(t *testing.T, args []string, target string) []string {
	t.Helper()
	out := append([]string{}, args...)
	targets := 0
	for index := 0; index < len(out); index++ {
		if out[index] != "-Target" {
			continue
		}
		if index+1 >= len(out) {
			t.Fatal("self-contained ApplyArgs contains a missing -Target value")
		}
		out[index+1] = target
		targets++
		index++
	}
	if targets != 1 {
		t.Fatalf("self-contained ApplyArgs contains %d -Target selectors: %v", targets, args)
	}
	return out
}

func assertMovedSelfContainedProjectHealth(t *testing.T, cwd, executable, projectRoot, projectID string) map[string][]byte {
	t.Helper()
	statusData := selfContainedRun(t, cwd, executable, "runtime", "-Command", "status", "-Format", "json")
	var status struct {
		Command      string                    `json:"command"`
		RuntimeRoot  string                    `json:"runtimeRoot"`
		TemplateRoot string                    `json:"templateRoot"`
		Target       string                    `json:"target"`
		Mode         string                    `json:"mode"`
		Onboarding   *missionintent.Inspection `json:"onboarding"`
	}
	selfContainedDecode(t, statusData, &status)
	if status.Command != "status" || status.Mode != "case" ||
		!sameSelfContainedPath(status.Target, projectRoot) ||
		!sameSelfContainedPath(status.TemplateRoot, filepath.Join(projectRoot, ".steamai")) ||
		!sameSelfContainedPath(status.RuntimeRoot, filepath.Join(projectRoot, ".steamai", "runtime")) ||
		status.Onboarding == nil || !status.Onboarding.Committed ||
		status.Onboarding.Identity.SchemaVersion != 2 || status.Onboarding.Identity.Target != "." ||
		status.Onboarding.Identity.ProjectID != projectID || status.Onboarding.ProjectBinding == nil ||
		status.Onboarding.ProjectBinding.ProjectID != projectID {
		t.Fatalf("moved project status did not preserve its local runtime and v2 identity: %+v", status)
	}

	packsData := selfContainedRun(t, cwd, executable, "runtime", "-Command", "packs", "-Format", "json")
	var packs struct {
		Command string `json:"command"`
		Packs   []struct {
			ID string `json:"id"`
		} `json:"packs"`
	}
	selfContainedDecode(t, packsData, &packs)
	foundTemplate := false
	for _, pack := range packs.Packs {
		foundTemplate = foundTemplate || pack.ID == "_template"
	}
	if packs.Command != "packs" || !foundTemplate {
		t.Fatalf("moved project pack inventory is invalid: %+v", packs)
	}

	doctorData := selfContainedRun(t, cwd, executable, "runtime", "-Command", "doctor", "-Format", "json")
	var doctor struct {
		Command string `json:"command"`
		Pack    string `json:"pack"`
		Target  string `json:"target"`
		Mode    string `json:"mode"`
		Valid   bool   `json:"valid"`
	}
	selfContainedDecode(t, doctorData, &doctor)
	if doctor.Command != "doctor" || doctor.Mode != "case" || doctor.Pack != "_template" || !doctor.Valid || !sameSelfContainedPath(doctor.Target, projectRoot) {
		t.Fatalf("moved project doctor result is invalid: %+v", doctor)
	}
	return map[string][]byte{"moved-status": statusData, "moved-packs": packsData, "moved-doctor": doctorData}
}

func assertInstalledSTeamAISkillBridge(t *testing.T, repoRoot, projectRoot string) {
	t.Helper()
	installedPath := filepath.Join(projectRoot, ".claude", "skills", "steamai", "SKILL.md")
	templatePath := filepath.Join(projectRoot, ".steamai", "rekit", "templates", "steamai-project", "SKILL.md")
	canonicalPath := filepath.Join(repoRoot, ".claude", "skills", "steamai", "SKILL.md")
	installed, err := os.ReadFile(installedPath)
	if err != nil {
		t.Fatalf("read installed /steamai skill: %v", err)
	}
	template, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("read bundled /steamai skill template: %v", err)
	}
	canonical, err := os.ReadFile(canonicalPath)
	if err != nil {
		t.Fatalf("read canonical /steamai skill: %v", err)
	}
	installedSemantic := sourceartifact.SemanticText(installed)
	if !bytes.Equal(installedSemantic, sourceartifact.SemanticText(template)) {
		t.Fatalf("installed /steamai skill differs semantically from its bundled template")
	}
	if !bytes.Equal(installedSemantic, sourceartifact.SemanticText(canonical)) {
		t.Fatalf("installed and bundled /steamai skills differ semantically from the canonical skill")
	}
	text := string(installed)
	for _, phrase := range []string{
		"typed `invocation` 是唯一通用命令桥",
		"机器命令附录是固定 front door、deterministic owner bridge、argv 与 Apply binding 的唯一 owner",
		"`commandExecutable=false`",
		"不得拼接 shell command",
		"不自行追加 `-Apply`",
	} {
		if !strings.Contains(text, phrase) {
			t.Fatalf("installed /steamai skill omits typed bridge boundary %q", phrase)
		}
	}
}

func assertSelfContainedTypedCommandBridge(t *testing.T, cwd, executable, projectRoot string, removedPaths ...string) {
	t.Helper()
	startData := selfContainedRun(
		t,
		cwd,
		executable,
		"runtime",
		"-Command", "start",
		"-Target", projectRoot,
		"-Pack", "_template",
		"-Name", "login",
		"-Apply",
		"-Format", "json",
	)
	var started struct {
		Command string `json:"command"`
		Applied bool   `json:"applied"`
	}
	selfContainedDecode(t, startData, &started)
	if started.Command != commands.Start || !started.Applied {
		t.Fatalf("project-local typed bridge fixture did not start: %+v", started)
	}

	statusData := selfContainedRun(
		t,
		cwd,
		executable,
		"runtime",
		"-Command", "status",
		"-Target", projectRoot,
		"-Lane", "feature-login",
		"-Format", "json",
	)
	var status struct {
		MissionControlRunbook struct {
			CurrentDriverRequest       *mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
			CurrentDriverRequestSHA256 string                                 `json:"currentDriverRequestSha256"`
		} `json:"missionControlRunbook"`
	}
	selfContainedDecode(t, statusData, &status)
	request := status.MissionControlRunbook.CurrentDriverRequest
	if request == nil {
		t.Fatal("project-local status omitted typed current driver request")
	}
	if err := mission.ValidateMissionCommanderDriverRequest(*request); err != nil {
		t.Fatalf("project-local status returned invalid typed request: %v", err)
	}
	if request.Invocation == nil || !request.CommandExecutable || request.Blocked || request.Command == "" || request.ExpectedReceipt.Command != request.Command || !strings.Contains(request.Command, "-WhatIf") {
		t.Fatalf("project-local status returned a non-executable or non-preview typed request: %+v", request)
	}
	rendered, err := request.Invocation.RenderForEntrypoint(commands.CurrentPublicEntrypoint)
	if err != nil || rendered != request.Command {
		t.Fatalf("typed invocation does not render the exact current command: rendered=%q command=%q err=%v", rendered, request.Command, err)
	}
	requestSHA256, err := mission.MissionCommanderDriverRequestSHA256(*request)
	if err != nil || !strings.EqualFold(requestSHA256, status.MissionControlRunbook.CurrentDriverRequestSHA256) {
		t.Fatalf("typed request identity mismatch: computed=%s status=%s err=%v", requestSHA256, status.MissionControlRunbook.CurrentDriverRequestSHA256, err)
	}

	bridgeArgs := append([]string{"runtime", "-Command", request.Invocation.Command}, request.Invocation.Arguments...)
	previewData := selfContainedRun(t, cwd, executable, bridgeArgs...)
	var preview map[string]any
	selfContainedDecode(t, previewData, &preview)
	if preview["command"] != request.Invocation.Command {
		t.Fatalf("typed argv bridge ran the wrong command: got=%v want=%s", preview["command"], request.Invocation.Command)
	}
	if applied, ok := preview["applied"].(bool); !ok || applied {
		t.Fatalf("typed argv bridge did not preserve the request preview boundary: %+v", preview)
	}
	assertSelfContainedOutputOmits(t, "typed-command-bridge", previewData, removedPaths...)
}

func selfContainedRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join(wd, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return root
}

func selfContainedRun(t *testing.T, dir, executable string, args ...string) []byte {
	t.Helper()
	command := exec.Command(executable, args...)
	command.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("run %s %v: %v\nstdout:\n%s\nstderr:\n%s", executable, args, err, stdout.String(), stderr.String())
	}
	return append([]byte(nil), stdout.Bytes()...)
}

func selfContainedDecode(t *testing.T, data []byte, target any) {
	t.Helper()
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("decode subprocess JSON: %v\n%s", err, data)
	}
}

func copySelfContainedTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("copy self-contained project refuses symlink: %s", path)
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("copy self-contained project refuses non-regular file: %s", path)
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func assertSelfContainedOutputOmits(t *testing.T, name string, data []byte, paths ...string) {
	t.Helper()
	haystack := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(string(data), `\\`, `\`), "/", `\`))
	for _, path := range paths {
		needle := strings.ToLower(strings.ReplaceAll(filepath.Clean(path), "/", `\`))
		if index := strings.Index(haystack, needle); index >= 0 {
			start := max(0, index-160)
			end := min(len(haystack), index+len(needle)+200)
			t.Fatalf("%s output references removed central dependency %s near %q", name, path, haystack[start:end])
		}
	}
}

func assertSelfContainedOutputOmitsLegacyEntrypoint(t *testing.T, name string, data []byte) {
	t.Helper()
	text := strings.ToLower(string(data))
	needle := strings.ToLower(commands.LegacyPublicEntrypoint)
	for offset := 0; offset < len(text); {
		relative := strings.Index(text[offset:], needle)
		if relative < 0 {
			return
		}
		index := offset + relative
		end := index + len(needle)
		if end == len(text) || isLegacyEntrypointCommandBoundary(text[end]) {
			start := max(0, index-160)
			stop := min(len(text), end+200)
			t.Fatalf("%s output exposes legacy public entrypoint %s near %q", name, commands.LegacyPublicEntrypoint, text[start:stop])
		}
		offset = end
	}
}

func isLegacyEntrypointCommandBoundary(value byte) bool {
	switch value {
	case ' ', '\t', '\r', '\n', '-', '`':
		return true
	default:
		return false
	}
}

func sameSelfContainedPath(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
