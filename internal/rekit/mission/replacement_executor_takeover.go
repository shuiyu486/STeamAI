package mission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// ReplacementExecutorTakeoverPackage is a read-only, self-contained handoff for
// a replacement executor. It mirrors the focused current driver request so a new
// session can resume from status or durable handoff without relying on prior chat
// context.
type ReplacementExecutorTakeoverPackage struct {
	Ready                               bool                           `json:"ready"`
	Focus                               string                         `json:"focus,omitempty"`
	Scope                               string                         `json:"scope,omitempty"`
	State                               string                         `json:"state,omitempty"`
	Source                              string                         `json:"source,omitempty"`
	Label                               string                         `json:"label,omitempty"`
	ActionID                            string                         `json:"actionId,omitempty"`
	DriverKind                          string                         `json:"driverKind"`
	CommandExecutable                   bool                           `json:"commandExecutable"`
	RequiresReview                      bool                           `json:"requiresReview"`
	Blocked                             bool                           `json:"blocked,omitempty"`
	Command                             string                         `json:"command,omitempty"`
	Guidance                            string                         `json:"guidance,omitempty"`
	CurrentDriverRequest                MissionCommanderDriverRequest  `json:"currentDriverRequest"`
	TargetDocuments                     []string                       `json:"targetDocuments,omitempty"`
	RefreshStatusCommand                string                         `json:"refreshStatusCommand,omitempty"`
	CurrentDriverRequestSHA256          string                         `json:"currentDriverRequestSha256"`
	DurableArtifactPath                 string                         `json:"durableArtifactPath,omitempty"`
	DurableArtifactFresh                bool                           `json:"durableArtifactFresh,omitempty"`
	DurableArtifactState                string                         `json:"durableArtifactState,omitempty"`
	DurableArtifactSHA256               string                         `json:"durableArtifactSha256,omitempty"`
	DurableArtifactRequestSHA256        string                         `json:"durableArtifactRequestSha256,omitempty"`
	DurableArtifactWarnings             []string                       `json:"durableArtifactWarnings,omitempty"`
	DurableArtifactRefreshDriverRequest *MissionCommanderDriverRequest `json:"durableArtifactRefreshDriverRequest,omitempty"`
	CurrentLoopOperator                 *CurrentLoopOperatorPackage    `json:"currentLoopOperator,omitempty"`
	RunbookSteps                        []string                       `json:"runbookSteps,omitempty"`
	Boundary                            []string                       `json:"boundary,omitempty"`
	packagePath                         string
}

type ReplacementExecutorTakeoverOptions struct {
	Focus                               string
	Scope                               string
	RefreshStatusCommand                string
	PackagePath                         string
	TargetDocuments                     []string
	DurableArtifactPath                 string
	DurableArtifactFresh                bool
	DurableArtifactState                string
	DurableArtifactSHA256               string
	DurableArtifactRequestSHA256        string
	DurableArtifactWarnings             []string
	DurableArtifactRefreshDriverRequest *MissionCommanderDriverRequest
	CurrentLoopOperator                 *CurrentLoopOperatorPackage
}

func ReplacementExecutorTakeoverPackageFor(request *MissionCommanderDriverRequest, opt ReplacementExecutorTakeoverOptions) *ReplacementExecutorTakeoverPackage {
	if request == nil {
		return nil
	}
	current := MissionCommanderDriverRequestWithRefreshStatusCommand(*request, opt.RefreshStatusCommand)
	current.Boundary = UniqueStrings(current.Boundary)
	current.ExpectedReceipt.Boundary = UniqueStrings(current.ExpectedReceipt.Boundary)
	pkg := &ReplacementExecutorTakeoverPackage{
		Ready:                               true,
		Focus:                               strings.TrimSpace(opt.Focus),
		Scope:                               strings.TrimSpace(opt.Scope),
		State:                               strings.TrimSpace(current.State),
		Source:                              strings.TrimSpace(current.Source),
		Label:                               strings.TrimSpace(current.Label),
		ActionID:                            strings.TrimSpace(current.ActionID),
		DriverKind:                          firstNonEmpty(current.Kind, "unknown"),
		CommandExecutable:                   current.CommandExecutable,
		RequiresReview:                      current.RequiresReview,
		Blocked:                             current.Blocked,
		Command:                             strings.TrimSpace(current.Command),
		Guidance:                            strings.TrimSpace(current.Guidance),
		CurrentDriverRequest:                current,
		CurrentDriverRequestSHA256:          ReplacementExecutorDriverRequestSHA256(current),
		TargetDocuments:                     UniqueStrings(opt.TargetDocuments),
		RefreshStatusCommand:                strings.TrimSpace(opt.RefreshStatusCommand),
		DurableArtifactPath:                 strings.TrimSpace(opt.DurableArtifactPath),
		DurableArtifactFresh:                opt.DurableArtifactFresh,
		DurableArtifactState:                strings.TrimSpace(opt.DurableArtifactState),
		DurableArtifactSHA256:               strings.TrimSpace(opt.DurableArtifactSHA256),
		DurableArtifactRequestSHA256:        strings.TrimSpace(opt.DurableArtifactRequestSHA256),
		DurableArtifactWarnings:             UniqueStrings(opt.DurableArtifactWarnings),
		DurableArtifactRefreshDriverRequest: cloneMissionCommanderDriverRequest(opt.DurableArtifactRefreshDriverRequest),
		CurrentLoopOperator:                 opt.CurrentLoopOperator,
		packagePath:                         strings.TrimSpace(opt.PackagePath),
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
	if operator := pkg.CurrentLoopOperator; operator != nil && operator.ObservationInbox != nil && operator.ObservationInbox.State == "ready" && operator.SelectedDriverRequest != nil {
		steps = append(steps,
			"a unique canonical inbox observation is ready; consume currentLoopOperator.selectedDriverRequest before the underlying durable action",
			"review the returned preview and execute only its exact path-only hash-bound Apply command, then refresh status",
		)
	}
	if strings.TrimSpace(pkg.DurableArtifactPath) != "" && !pkg.DurableArtifactFresh {
		steps = append(steps, "do not consume stale durable takeover artifact "+pkg.DurableArtifactPath+"; use currentDriverRequest and refreshStatusCommand instead")
		if pkg.DurableArtifactRefreshDriverRequest != nil {
			steps = append(steps, "run durableArtifactRefreshDriverRequest.command as a handoff preview, then consume its returned apply request before trusting the durable artifact again")
		}
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
				boundary = append(boundary,
					"durable takeover artifact strict decoding and full current driver request identity matched refreshed status",
					"durableArtifactSha256 identifies the exact inspected artifact bytes; durableArtifactRequestSha256 equals currentDriverRequestSha256 when fresh",
				)
			} else {
				boundary = append(boundary, "durable takeover artifact is stale or invalid; do not use it to override the current driver request")
				if pkg.DurableArtifactRefreshDriverRequest != nil {
					boundary = append(boundary, "durable artifact refresh driver request is a handoff preview only; consume the returned apply request before trusting refreshed artifacts")
				}
			}
			boundary = append(boundary, pkg.DurableArtifactWarnings...)
		}
		if !pkg.CommandExecutable {
			boundary = append(boundary, "guidance must be reviewed, not executed as a shell command")
		}
	}
	return UniqueStrings(boundary)
}

func ReplacementExecutorDriverRequestSHA256(request MissionCommanderDriverRequest) string {
	request.Boundary = UniqueStrings(request.Boundary)
	request.ExpectedReceipt.Boundary = UniqueStrings(request.ExpectedReceipt.Boundary)
	data, err := json.Marshal(request)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type replacementExecutorTakeoverArtifactIdentity struct {
	Ready                      bool                          `json:"ready"`
	Focus                      string                        `json:"focus,omitempty"`
	Scope                      string                        `json:"scope,omitempty"`
	State                      string                        `json:"state,omitempty"`
	Source                     string                        `json:"source,omitempty"`
	Label                      string                        `json:"label,omitempty"`
	ActionID                   string                        `json:"actionId,omitempty"`
	DriverKind                 string                        `json:"driverKind"`
	CommandExecutable          bool                          `json:"commandExecutable"`
	RequiresReview             bool                          `json:"requiresReview"`
	Blocked                    bool                          `json:"blocked,omitempty"`
	Command                    string                        `json:"command,omitempty"`
	Guidance                   string                        `json:"guidance,omitempty"`
	CurrentDriverRequest       MissionCommanderDriverRequest `json:"currentDriverRequest"`
	CurrentDriverRequestSHA256 string                        `json:"currentDriverRequestSha256"`
	RefreshStatusCommand       string                        `json:"refreshStatusCommand,omitempty"`
	CurrentLoopOperator        *CurrentLoopOperatorPackage   `json:"currentLoopOperator,omitempty"`
	RunbookSteps               []string                      `json:"runbookSteps,omitempty"`
	Boundary                   []string                      `json:"boundary,omitempty"`
}

// ReplacementExecutorTakeoverArtifactIdentitySHA256 binds every executable or
// instruction-bearing field. TargetDocuments are discovery hints, while durable
// artifact fields describe the current inspection and are intentionally excluded.
func ReplacementExecutorTakeoverArtifactIdentitySHA256(pkg ReplacementExecutorTakeoverPackage) string {
	request := pkg.CurrentDriverRequest
	request.Boundary = UniqueStrings(request.Boundary)
	request.ExpectedReceipt.Boundary = UniqueStrings(request.ExpectedReceipt.Boundary)
	identity := replacementExecutorTakeoverArtifactIdentity{
		Ready:                      pkg.Ready,
		Focus:                      strings.TrimSpace(pkg.Focus),
		Scope:                      strings.TrimSpace(pkg.Scope),
		State:                      strings.TrimSpace(pkg.State),
		Source:                     strings.TrimSpace(pkg.Source),
		Label:                      strings.TrimSpace(pkg.Label),
		ActionID:                   strings.TrimSpace(pkg.ActionID),
		DriverKind:                 strings.TrimSpace(pkg.DriverKind),
		CommandExecutable:          pkg.CommandExecutable,
		RequiresReview:             pkg.RequiresReview,
		Blocked:                    pkg.Blocked,
		Command:                    strings.TrimSpace(pkg.Command),
		Guidance:                   strings.TrimSpace(pkg.Guidance),
		CurrentDriverRequest:       request,
		CurrentDriverRequestSHA256: strings.TrimSpace(pkg.CurrentDriverRequestSHA256),
		RefreshStatusCommand:       strings.TrimSpace(pkg.RefreshStatusCommand),
		CurrentLoopOperator:        pkg.CurrentLoopOperator,
		RunbookSteps:               UniqueStrings(pkg.RunbookSteps),
		Boundary:                   UniqueStrings(pkg.Boundary),
	}
	data, err := json.Marshal(identity)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cloneMissionCommanderDriverRequest(request *MissionCommanderDriverRequest) *MissionCommanderDriverRequest {
	if request == nil {
		return nil
	}
	clone := *request
	clone.Boundary = UniqueStrings(clone.Boundary)
	clone.ExpectedReceipt.Boundary = UniqueStrings(clone.ExpectedReceipt.Boundary)
	return &clone
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
