package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/mission"
	"github.com/shuiyu486/re-context-kits/internal/rekit/runtime"
)

type currentStepPlan struct {
	SchemaVersion                 int                                   `json:"schemaVersion"`
	Command                       string                                `json:"command"`
	CaseRoot                      string                                `json:"caseRoot"`
	Pack                          string                                `json:"pack"`
	Route                         string                                `json:"route"`
	IsMutation                    bool                                  `json:"isMutation"`
	Applied                       bool                                  `json:"applied"`
	ReviewRequired                bool                                  `json:"reviewRequired"`
	RequiresConfirmation          bool                                  `json:"requiresConfirmation"`
	CurrentDriverRequest          mission.MissionCommanderDriverRequest `json:"currentDriverRequest"`
	DriverStep                    *driverStepPlan                       `json:"driverStep,omitempty"`
	ReviewerStep                  *reviewerStepPlan                     `json:"reviewerStep,omitempty"`
	ExpectedCurrentStepPlanSHA256 string                                `json:"expectedCurrentStepPlanSha256,omitempty"`
	Receipt                       *currentStepReceipt                   `json:"receipt,omitempty"`
	RefreshedStatus               *statusInventory                      `json:"refreshedStatus,omitempty"`
	Boundary                      []string                              `json:"boundary"`
}

type currentStepReceipt struct {
	State                         string                                 `json:"state"`
	Outcome                       string                                 `json:"outcome"`
	Route                         string                                 `json:"route"`
	NestedCommand                 string                                 `json:"nestedCommand"`
	RefreshedCurrentDriverRequest *mission.MissionCommanderDriverRequest `json:"refreshedCurrentDriverRequest,omitempty"`
	Boundary                      []string                               `json:"boundary"`
}

type currentStepPlanIdentity struct {
	Route                        string                                `json:"route"`
	RoutedDriverRequest          mission.MissionCommanderDriverRequest `json:"routedDriverRequest"`
	NestedDriverRequest          mission.MissionCommanderDriverRequest `json:"nestedDriverRequest"`
	ExpectedNestedStepPlanSHA256 string                                `json:"expectedNestedStepPlanSha256"`
}

func runCurrentStep(ctx runtime.Context, opt Options, out io.Writer) error {
	if !ctx.TargetProvided {
		return fmt.Errorf("run-current-step requires -Target for an attached case")
	}
	if opt.WhatIf && opt.Apply {
		return fmt.Errorf("run-current-step cannot combine -WhatIf and -Apply")
	}
	if !opt.WhatIf && !opt.Apply {
		return fmt.Errorf("run-current-step requires -WhatIf or -Apply")
	}
	if strings.ToLower(strings.TrimSpace(opt.Format)) != "json" {
		return fmt.Errorf("run-current-step supports only -Format json")
	}
	if err := validateCurrentStepOuterArgs(opt); err != nil {
		return err
	}
	plan, err := buildCurrentStepPlan(ctx, opt)
	if err != nil {
		return err
	}
	if opt.WhatIf {
		if strings.TrimSpace(opt.ExpectedCurrentStepPlanSHA256) != "" {
			return fmt.Errorf("run-current-step -WhatIf does not accept -ExpectedCurrentStepPlanSha256")
		}
		return writeJSON(out, plan)
	}
	if plan.ExpectedCurrentStepPlanSHA256 == "" {
		return fmt.Errorf("run-current-step current route requires an external harness action before Apply")
	}
	expected := strings.TrimSpace(opt.ExpectedCurrentStepPlanSHA256)
	if expected == "" {
		return fmt.Errorf("run-current-step -Apply requires -ExpectedCurrentStepPlanSha256 from -WhatIf")
	}
	if !strings.EqualFold(expected, plan.ExpectedCurrentStepPlanSHA256) {
		return fmt.Errorf("run-current-step expected plan sha256 mismatch: got %s want %s", expected, plan.ExpectedCurrentStepPlanSHA256)
	}
	plan, err = applyCurrentStepPlan(ctx, opt, plan)
	if err != nil {
		return err
	}
	return writeJSON(out, plan)
}

func buildCurrentStepPlan(ctx runtime.Context, opt Options) (currentStepPlan, error) {
	status, err := buildStatusInventory(ctx, statusPackSource(ctx, opt))
	if err != nil {
		return currentStepPlan{}, err
	}
	if status.MissionControlRunbook == nil || status.MissionControlRunbook.CurrentDriverRequest == nil {
		return currentStepPlan{}, fmt.Errorf("run-current-step requires missionControlRunbook.currentDriverRequest")
	}
	if status.MissionControlRunbook.Scope != "case" && status.MissionControlRunbook.Scope != "reviewer" {
		return currentStepPlan{}, fmt.Errorf("run-current-step supports only focused case or reviewer requests; got scope %q", status.MissionControlRunbook.Scope)
	}
	routedRequest := *status.MissionControlRunbook.CurrentDriverRequest
	plan := currentStepPlan{
		SchemaVersion:        1,
		Command:              commands.RunCurrentStep,
		CaseRoot:             ctx.Target,
		Pack:                 ctx.Pack,
		Route:                status.MissionControlRunbook.Scope,
		ReviewRequired:       true,
		RequiresConfirmation: true,
		CurrentDriverRequest: routedRequest,
		Boundary: []string{
			"router selects only the focused case or reviewer request from refreshed missionControlRunbook status",
			"case steps retain the run-driver-step lane mutation lease and preview hash guards",
			"reviewer steps retain the run-reviewer-step packet, artifact, receipt, candidate hash, and reviewer intake lock guards",
			"the Go runtime does not invoke a shell or Agent tool, spawn or poll sessions, fabricate reviewer output, execute heavy tools, or write authority/confirmed state",
			"status is rebuilt by the selected nested runner after Apply before follow-up work is selected",
		},
	}
	nestedSHA256 := ""
	switch plan.Route {
	case "case":
		if currentStepHasReviewerObservation(opt) {
			return currentStepPlan{}, fmt.Errorf("run-current-step case route does not accept reviewer observation inputs")
		}
		nested, err := buildDriverStepPlan(ctx, opt)
		if err != nil {
			return currentStepPlan{}, fmt.Errorf("run-current-step case route: %w", err)
		}
		plan.DriverStep = &nested
		plan.CurrentDriverRequest = nested.CurrentDriverRequest
		nestedSHA256 = nested.ExpectedDriverStepPlanSHA256
	case "reviewer":
		nested, err := buildReviewerStepPlanFromStatus(ctx, opt, status)
		if err != nil {
			return currentStepPlan{}, fmt.Errorf("run-current-step reviewer route: %w", err)
		}
		if !currentStepReviewerRequestsMatch(routedRequest, nested.CurrentDriverRequest) {
			return currentStepPlan{}, fmt.Errorf("run-current-step reviewer route request drift: missionControlRunbook current request does not match reviewer operator package request")
		}
		plan.ReviewerStep = &nested
		plan.CurrentDriverRequest = nested.CurrentDriverRequest
		plan.RequiresConfirmation = nested.ExternalHandoff == nil
		nestedSHA256 = nested.ExpectedReviewerStepPlanSHA256
	}
	if nestedSHA256 == "" {
		return plan, nil
	}
	identity := currentStepPlanIdentity{
		Route:                        plan.Route,
		RoutedDriverRequest:          routedRequest,
		NestedDriverRequest:          plan.CurrentDriverRequest,
		ExpectedNestedStepPlanSHA256: nestedSHA256,
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return currentStepPlan{}, err
	}
	sum := sha256.Sum256(encoded)
	plan.ExpectedCurrentStepPlanSHA256 = hex.EncodeToString(sum[:])
	return plan, nil
}

func applyCurrentStepPlan(ctx runtime.Context, opt Options, plan currentStepPlan) (currentStepPlan, error) {
	var refreshed *statusInventory
	nestedCommand := ""
	switch plan.Route {
	case "case":
		if plan.DriverStep == nil {
			return currentStepPlan{}, fmt.Errorf("run-current-step case route omitted driver step plan")
		}
		nested, err := applyDriverStepPlan(ctx, opt, *plan.DriverStep)
		if err != nil {
			return currentStepPlan{}, err
		}
		plan.DriverStep = &nested
		plan.Applied = nested.Applied
		refreshed = nested.RefreshedStatus
		nestedCommand = commands.RunDriverStep
	case "reviewer":
		if plan.ReviewerStep == nil || plan.ReviewerStep.ExternalHandoff != nil {
			return currentStepPlan{}, fmt.Errorf("run-current-step reviewer route requires a deterministic reviewer step plan")
		}
		nested, err := applyReviewerStepPlan(ctx, opt, *plan.ReviewerStep)
		if err != nil {
			return currentStepPlan{}, err
		}
		plan.ReviewerStep = &nested
		plan.Applied = nested.Applied
		refreshed = nested.RefreshedStatus
		nestedCommand = commands.RunReviewerStep
	default:
		return currentStepPlan{}, fmt.Errorf("run-current-step route %q is unsupported", plan.Route)
	}
	plan.IsMutation = true
	plan.ReviewRequired = false
	plan.RequiresConfirmation = false
	plan.RefreshedStatus = refreshed
	plan.Receipt = &currentStepReceipt{
		State:         "refreshed",
		Outcome:       "current-step-applied",
		Route:         plan.Route,
		NestedCommand: nestedCommand,
		Boundary: []string{
			"receipt identifies the selected nested runner; it does not prove external session execution",
			"consume refreshedCurrentDriverRequest only after the selected nested runner rebuilt durable status",
			"no authority/confirmed state or heavy-tool execution is produced by this router",
		},
	}
	if refreshed != nil && refreshed.MissionControlRunbook != nil {
		plan.Receipt.RefreshedCurrentDriverRequest = refreshed.MissionControlRunbook.CurrentDriverRequest
	}
	return plan, nil
}

func validateCurrentStepOuterArgs(opt Options) error {
	valueFlags := map[string]bool{
		"-command": true, "--command": true,
		"-target": true, "--target": true,
		"-pack": true, "--pack": true,
		"-format": true, "--format": true,
		"-expectedcurrentstepplansha256": true, "--expected-current-step-plan-sha256": true,
		"-actor": true, "--actor": true,
		"-reviewerresultinputsourcepath": true, "--reviewer-result-input-source-path": true,
		"-reviewerharness": true, "--reviewer-harness": true,
		"-reviewersession": true, "--reviewer-session": true,
		"-revieweroutcome": true, "--reviewer-outcome": true,
		"-reviewerexitstatus": true, "--reviewer-exit-status": true,
	}
	switchFlags := map[string]bool{
		"-whatif": true, "--what-if": true,
		"-apply": true, "--apply": true,
	}
	seen := map[string]bool{}
	separatorSeen := false
	for i := 0; i < len(opt.rawArgs); i++ {
		token := opt.rawArgs[i]
		if token == "--" {
			if i != 0 || separatorSeen {
				return fmt.Errorf("run-current-step accepts -- only once at the start of the argument list")
			}
			separatorSeen = true
			continue
		}
		key := strings.ToLower(strings.SplitN(token, "=", 2)[0])
		if !strings.HasPrefix(key, "-") {
			return fmt.Errorf("run-current-step contains unsupported positional argument %s", token)
		}
		canonical := currentStepCanonicalOuterFlag(key)
		if key != canonical && !valueFlags[key] && !switchFlags[key] {
			return fmt.Errorf("run-current-step contains unsupported flag %s", token)
		}
		if seen[canonical] {
			return fmt.Errorf("run-current-step repeats flag %s", token)
		}
		seen[canonical] = true
		if switchFlags[key] {
			continue
		}
		if !valueFlags[key] {
			return fmt.Errorf("run-current-step contains unsupported flag %s", token)
		}
		if !strings.Contains(token, "=") {
			if i+1 >= len(opt.rawArgs) || strings.HasPrefix(opt.rawArgs[i+1], "-") {
				return fmt.Errorf("run-current-step flag %s is missing a value", token)
			}
			i++
		}
	}
	return nil
}

func currentStepCanonicalOuterFlag(key string) string {
	switch key {
	case "--command":
		return "-command"
	case "--target":
		return "-target"
	case "--pack":
		return "-pack"
	case "--format":
		return "-format"
	case "--expected-current-step-plan-sha256":
		return "-expectedcurrentstepplansha256"
	case "--what-if":
		return "-whatif"
	case "--apply":
		return "-apply"
	case "--actor":
		return "-actor"
	case "--reviewer-result-input-source-path":
		return "-reviewerresultinputsourcepath"
	case "--reviewer-harness":
		return "-reviewerharness"
	case "--reviewer-session":
		return "-reviewersession"
	case "--reviewer-outcome":
		return "-revieweroutcome"
	case "--reviewer-exit-status":
		return "-reviewerexitstatus"
	default:
		return key
	}
}

func currentStepReviewerRequestsMatch(routed, nested mission.MissionCommanderDriverRequest) bool {
	if strings.TrimSpace(routed.Source) != "reviewerDispatchOperatorPackage" || strings.TrimSpace(nested.Source) != "reviewerDispatchOperatorPackage" {
		return false
	}
	if strings.TrimSpace(routed.Lane) != strings.TrimSpace(nested.Lane) ||
		strings.TrimSpace(routed.Label) != strings.TrimSpace(nested.Label) ||
		strings.TrimSpace(routed.ActionID) != strings.TrimSpace(nested.ActionID) ||
		strings.TrimSpace(routed.State) != strings.TrimSpace(nested.State) ||
		strings.TrimSpace(routed.RunLoopStepID) != strings.TrimSpace(nested.RunLoopStepID) {
		return false
	}
	return currentStepReviewerCommandIdentity(routed.Command) == currentStepReviewerCommandIdentity(nested.Command)
}

func currentStepReviewerCommandIdentity(command string) string {
	fields, err := splitDriverCommand(command)
	if err != nil {
		return ""
	}
	identity := []string{}
	for idx := 0; idx < len(fields); idx++ {
		if strings.EqualFold(fields[idx], "-Target") || strings.EqualFold(fields[idx], "--target") {
			idx++
			continue
		}
		identity = append(identity, fields[idx])
	}
	return joinDriverCommand(identity)
}

func currentStepHasReviewerObservation(opt Options) bool {
	return strings.TrimSpace(opt.Note.Actor) != "" ||
		strings.TrimSpace(opt.ReviewerResultInputSourcePath) != "" ||
		strings.TrimSpace(opt.ReviewerHarness) != "" ||
		strings.TrimSpace(opt.ReviewerSession) != "" ||
		strings.TrimSpace(opt.ReviewerOutcome) != "" ||
		strings.TrimSpace(opt.ReviewerExitStatus) != ""
}
