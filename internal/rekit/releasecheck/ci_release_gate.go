package releasecheck

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CIReleaseGate struct {
	WorkflowPath     string                   `json:"workflowPath"`
	Ready            bool                     `json:"ready"`
	Summary          string                   `json:"summary"`
	WorkflowChecks   []CIReleaseWorkflowCheck `json:"workflowChecks"`
	Jobs             []CIReleaseJob           `json:"jobs"`
	RequiredCommands []CIReleaseCommand       `json:"requiredCommands"`
	ForbiddenStrings []CIReleaseForbidden     `json:"forbiddenStrings"`
	Warnings         []string                 `json:"warnings"`
}

type CIReleaseWorkflowCheck struct {
	Name     string `json:"name"`
	Expected string `json:"expected"`
	Present  bool   `json:"present"`
}

type CIReleaseJob struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	RunsOn   string `json:"runsOn"`
	Present  bool   `json:"present"`
	Required bool   `json:"required"`
}

type CIReleaseCommand struct {
	Job      string `json:"job"`
	Command  string `json:"command"`
	Present  bool   `json:"present"`
	Required bool   `json:"required"`
}

type CIReleaseForbidden struct {
	Pattern string `json:"pattern"`
	Present bool   `json:"present"`
}

type ciRequiredJob struct {
	id     string
	name   string
	runsOn string
}

type ciRequiredCommand struct {
	job     string
	command string
}

type ciParsedJob struct {
	id       string
	name     string
	runsOn   string
	commands []string
}

var ciWorkflowChecks = []CIReleaseWorkflowCheck{
	{Name: "workflow name", Expected: "name: release-gate"},
	{Name: "push main trigger", Expected: "branches: [main]"},
	{Name: "pull request trigger", Expected: "pull_request:"},
	{Name: "checkout action", Expected: "uses: actions/checkout@v4"},
	{Name: "setup-go action", Expected: "uses: actions/setup-go@v5"},
	{Name: "go version", Expected: "go-version: '1.26.x'"},
}

var ciRequiredJobs = []ciRequiredJob{
	{id: "go-checks", name: "Go release checks", runsOn: "ubuntu-latest"},
	{id: "windows-facade", name: "Windows facade smoke", runsOn: "windows-latest"},
}

var ciRequiredCommands = []ciRequiredCommand{
	{job: "go-checks", command: "go run ./cmd/rekit -- -Command release-check -Format json"},
	{job: "go-checks", command: "go test ./..."},
	{job: "go-checks", command: "go vet ./..."},
	{job: "windows-facade", command: "go run ./cmd/rekit -- -Command release-check -Format json"},
	{job: "windows-facade", command: ".\\rekit\\rekit.ps1 -Command doctor"},
	{job: "windows-facade", command: ".\\rekit\\tests\\facade-smoke.ps1"},
}

var ciForbiddenStrings = []string{
	"pack-smoke-matrix.ps1",
	"pack-inventory-smoke.ps1",
	"agent-team-dryrun-smoke.ps1",
	"full-trace",
	"debug",
	"inject",
	"patch",
	"dump",
	"network",
	"symex",
}

func ciReleaseGate(repo string) CIReleaseGate {
	const workflowPath = ".github/workflows/release-gate.yml"
	gate := CIReleaseGate{
		WorkflowPath: workflowPath,
		Ready:        true,
		Summary:      "CI release gate inventory ok",
		Warnings:     []string{},
	}
	data, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(workflowPath)))
	if err != nil {
		gate.Ready = false
		gate.Summary = "CI release gate workflow missing"
		gate.Warnings = append(gate.Warnings, err.Error())
		return gate
	}
	text := string(data)
	jobs := parseCIWorkflowJobs(text)
	gate.WorkflowChecks = ciWorkflowReadiness(text)
	gate.Jobs = ciJobReadiness(jobs)
	gate.RequiredCommands = ciCommandReadiness(jobs)
	gate.ForbiddenStrings = ciForbiddenReadiness(text)
	gate.Warnings = append(gate.Warnings, ciReleaseGateWarnings(gate)...)
	if len(gate.Warnings) > 0 {
		gate.Ready = false
		gate.Summary = "CI release gate inventory has warnings"
	}
	return gate
}

func ciWorkflowReadiness(text string) []CIReleaseWorkflowCheck {
	checks := make([]CIReleaseWorkflowCheck, 0, len(ciWorkflowChecks))
	for _, check := range ciWorkflowChecks {
		check.Present = strings.Contains(text, check.Expected)
		checks = append(checks, check)
	}
	return checks
}

func ciJobReadiness(jobs map[string]ciParsedJob) []CIReleaseJob {
	out := make([]CIReleaseJob, 0, len(ciRequiredJobs))
	for _, required := range ciRequiredJobs {
		actual, present := jobs[required.id]
		out = append(out, CIReleaseJob{
			ID:       required.id,
			Name:     actual.name,
			RunsOn:   actual.runsOn,
			Present:  present && strings.TrimSpace(actual.runsOn) == required.runsOn && strings.TrimSpace(actual.name) == required.name,
			Required: true,
		})
	}
	return out
}

func ciCommandReadiness(jobs map[string]ciParsedJob) []CIReleaseCommand {
	out := make([]CIReleaseCommand, 0, len(ciRequiredCommands))
	for _, required := range ciRequiredCommands {
		out = append(out, CIReleaseCommand{
			Job:      required.job,
			Command:  required.command,
			Present:  ciJobHasCommand(jobs[required.job], required.command),
			Required: true,
		})
	}
	return out
}

func ciForbiddenReadiness(text string) []CIReleaseForbidden {
	out := make([]CIReleaseForbidden, 0, len(ciForbiddenStrings))
	for _, pattern := range ciForbiddenStrings {
		out = append(out, CIReleaseForbidden{Pattern: pattern, Present: strings.Contains(text, pattern)})
	}
	return out
}

func ciReleaseGateWarnings(gate CIReleaseGate) []string {
	warnings := []string{}
	for _, check := range gate.WorkflowChecks {
		if !check.Present {
			warnings = append(warnings, fmt.Sprintf("CI workflow missing %s: %s", check.Name, check.Expected))
		}
	}
	for _, job := range gate.Jobs {
		if !job.Present {
			warnings = append(warnings, fmt.Sprintf("CI workflow missing required job %s with expected name/runs-on", job.ID))
		}
	}
	for _, command := range gate.RequiredCommands {
		if !command.Present {
			warnings = append(warnings, fmt.Sprintf("CI workflow missing required command in %s: %s", command.Job, command.Command))
		}
	}
	for _, forbidden := range gate.ForbiddenStrings {
		if forbidden.Present {
			warnings = append(warnings, fmt.Sprintf("CI workflow must not run broad smoke or heavy-tool step: %s", forbidden.Pattern))
		}
	}
	return warnings
}

func parseCIWorkflowJobs(text string) map[string]ciParsedJob {
	jobs := map[string]ciParsedJob{}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	inJobs := false
	current := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "jobs:" {
			inJobs = true
			current = ""
			continue
		}
		if !inJobs {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 && strings.HasSuffix(trimmed, ":") {
			break
		}
		if indent == 2 && strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "- ") {
			current = strings.TrimSuffix(trimmed, ":")
			jobs[current] = ciParsedJob{id: current}
			continue
		}
		if current == "" {
			continue
		}
		job := jobs[current]
		if indent == 4 && strings.HasPrefix(trimmed, "name:") {
			job.name = yamlInlineValue(trimmed)
		}
		if indent == 4 && strings.HasPrefix(trimmed, "runs-on:") {
			job.runsOn = yamlInlineValue(trimmed)
		}
		if indent >= 6 && strings.HasPrefix(trimmed, "run:") {
			job.commands = append(job.commands, yamlInlineValue(trimmed))
		}
		jobs[current] = job
	}
	return jobs
}

func ciJobHasCommand(job ciParsedJob, command string) bool {
	want := normalizeCommand(command)
	for _, actual := range job.commands {
		if normalizeCommand(actual) == want {
			return true
		}
	}
	return false
}

func yamlInlineValue(line string) string {
	_, value, ok := strings.Cut(line, ":")
	if !ok {
		return ""
	}
	value = strings.TrimSpace(value)
	if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		return strings.Trim(value, "\"'")
	}
	return value
}
