package mission

import "strings"

// ReplacementExecutorTakeoverPackage is a read-only, self-contained handoff for
// a replacement executor. It mirrors the focused current driver request so a new
// session can resume from status or durable handoff without relying on prior chat
// context.
type ReplacementExecutorTakeoverPackage struct {
	Ready                   bool                          `json:"ready"`
	Focus                   string                        `json:"focus,omitempty"`
	Scope                   string                        `json:"scope,omitempty"`
	State                   string                        `json:"state,omitempty"`
	Source                  string                        `json:"source,omitempty"`
	Label                   string                        `json:"label,omitempty"`
	ActionID                string                        `json:"actionId,omitempty"`
	DriverKind              string                        `json:"driverKind"`
	CommandExecutable       bool                          `json:"commandExecutable"`
	RequiresReview          bool                          `json:"requiresReview"`
	Blocked                 bool                          `json:"blocked,omitempty"`
	Command                 string                        `json:"command,omitempty"`
	Guidance                string                        `json:"guidance,omitempty"`
	CurrentDriverRequest    MissionCommanderDriverRequest `json:"currentDriverRequest"`
	TargetDocuments         []string                      `json:"targetDocuments,omitempty"`
	RefreshStatusCommand    string                        `json:"refreshStatusCommand,omitempty"`
	DurableArtifactPath     string                        `json:"durableArtifactPath,omitempty"`
	DurableArtifactFresh    bool                          `json:"durableArtifactFresh,omitempty"`
	DurableArtifactState    string                        `json:"durableArtifactState,omitempty"`
	DurableArtifactWarnings []string                      `json:"durableArtifactWarnings,omitempty"`
	RunbookSteps            []string                      `json:"runbookSteps,omitempty"`
	Boundary                []string                      `json:"boundary,omitempty"`
	packagePath             string
}

type ReplacementExecutorTakeoverOptions struct {
	Focus                   string
	Scope                   string
	RefreshStatusCommand    string
	PackagePath             string
	TargetDocuments         []string
	DurableArtifactPath     string
	DurableArtifactFresh    bool
	DurableArtifactState    string
	DurableArtifactWarnings []string
}

func ReplacementExecutorTakeoverPackageFor(request *MissionCommanderDriverRequest, opt ReplacementExecutorTakeoverOptions) *ReplacementExecutorTakeoverPackage {
	if request == nil {
		return nil
	}
	current := MissionCommanderDriverRequestWithRefreshStatusCommand(*request, opt.RefreshStatusCommand)
	current.Boundary = UniqueStrings(current.Boundary)
	current.ExpectedReceipt.Boundary = UniqueStrings(current.ExpectedReceipt.Boundary)
	pkg := &ReplacementExecutorTakeoverPackage{
		Ready:                   true,
		Focus:                   strings.TrimSpace(opt.Focus),
		Scope:                   strings.TrimSpace(opt.Scope),
		State:                   strings.TrimSpace(current.State),
		Source:                  strings.TrimSpace(current.Source),
		Label:                   strings.TrimSpace(current.Label),
		ActionID:                strings.TrimSpace(current.ActionID),
		DriverKind:              firstNonEmpty(current.Kind, "unknown"),
		CommandExecutable:       current.CommandExecutable,
		RequiresReview:          current.RequiresReview,
		Blocked:                 current.Blocked,
		Command:                 strings.TrimSpace(current.Command),
		Guidance:                strings.TrimSpace(current.Guidance),
		CurrentDriverRequest:    current,
		TargetDocuments:         UniqueStrings(opt.TargetDocuments),
		RefreshStatusCommand:    strings.TrimSpace(opt.RefreshStatusCommand),
		DurableArtifactPath:     strings.TrimSpace(opt.DurableArtifactPath),
		DurableArtifactFresh:    opt.DurableArtifactFresh,
		DurableArtifactState:    strings.TrimSpace(opt.DurableArtifactState),
		DurableArtifactWarnings: UniqueStrings(opt.DurableArtifactWarnings),
		packagePath:             strings.TrimSpace(opt.PackagePath),
	}
	pkg.RunbookSteps = replacementExecutorTakeoverRunbookSteps(pkg)
	pkg.Boundary = replacementExecutorTakeoverBoundary(pkg)
	return pkg
}

func replacementExecutorTakeoverRunbookSteps(pkg *ReplacementExecutorTakeoverPackage) []string {
	if pkg == nil || !pkg.Ready {
		return nil
	}
	packagePath := firstNonEmpty(pkg.packagePath, "replacementExecutorTakeoverPackage")
	steps := []string{
		"read " + packagePath + " before using any prior chat context",
		"consume currentDriverRequest exactly; do not reconstruct commands from terminal prose",
	}
	if strings.TrimSpace(pkg.DurableArtifactPath) != "" && !pkg.DurableArtifactFresh {
		steps = append(steps, "do not consume stale durable takeover artifact "+pkg.DurableArtifactPath+"; use currentDriverRequest and refreshStatusCommand instead")
	}
	if pkg.Blocked {
		steps = append(steps, "resolve the currentDriverRequest blocker before running any command or follow-up")
	} else if pkg.CommandExecutable {
		steps = append(steps, "run currentDriverRequest.command exactly when it is still the intended focused action")
	} else {
		steps = append(steps, "review currentDriverRequest.guidance and targetDocuments; do not execute guidance as a shell command")
	}
	if pkg.RequiresReview {
		steps = append(steps, "review expectedReceipt and boundary before any Apply or follow-up")
	}
	if strings.TrimSpace(pkg.RefreshStatusCommand) != "" {
		steps = append(steps, "after the explicit outcome, run refreshStatusCommand and rebuild status before choosing follow-up work")
	}
	return UniqueStrings(steps)
}

func replacementExecutorTakeoverBoundary(pkg *ReplacementExecutorTakeoverPackage) []string {
	boundary := []string{
		"replacement executor takeover package is read-only and self-contained for status/handoff resumption",
		"do not use prior chat context to override currentDriverRequest, expectedReceipt, or boundary",
		"do not write authority/confirmed or execute heavy tools from this package",
		"the Go runtime does not spawn or replace executor sessions",
	}
	if pkg != nil {
		boundary = append(boundary, pkg.CurrentDriverRequest.Boundary...)
		if strings.TrimSpace(pkg.DurableArtifactPath) != "" {
			if pkg.DurableArtifactFresh {
				boundary = append(boundary, "durable takeover artifact freshness matched the current driver request")
			} else {
				boundary = append(boundary, "durable takeover artifact is stale or invalid; do not use it to override the current driver request")
			}
			boundary = append(boundary, pkg.DurableArtifactWarnings...)
		}
		if !pkg.CommandExecutable {
			boundary = append(boundary, "guidance must be reviewed, not executed as a shell command")
		}
	}
	return UniqueStrings(boundary)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
