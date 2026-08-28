package commands

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/plancontract"
)

const (
	PublicInvocationSchemaVersion = 1
	CurrentPublicEntrypoint       = "/steamai"
	LegacyPublicEntrypoint        = "/rekit"
)

type PublicInvocation struct {
	SchemaVersion int      `json:"schemaVersion"`
	Command       string   `json:"command"`
	Arguments     []string `json:"arguments,omitempty"`
}

func NewPublicInvocation(command string, arguments ...string) (PublicInvocation, error) {
	invocation := PublicInvocation{
		SchemaVersion: PublicInvocationSchemaVersion,
		Command:       strings.ToLower(strings.TrimSpace(command)),
		Arguments:     append([]string{}, arguments...),
	}
	if len(invocation.Arguments) == 0 {
		invocation.Arguments = nil
	}
	if err := invocation.Validate(); err != nil {
		return PublicInvocation{}, err
	}
	return invocation, nil
}

func ParsePublicInvocation(text string) (PublicInvocation, error) {
	fields, err := splitPublicInvocation(text)
	if err != nil {
		return PublicInvocation{}, err
	}
	if len(fields) < 2 ||
		(fields[0] != "/rekit" && fields[0] != "/steamai") ||
		strings.TrimSpace(fields[1]) == "" {
		return PublicInvocation{}, fmt.Errorf("public invocation must begin with /rekit or /steamai <command>")
	}
	return NewPublicInvocation(fields[1], fields[2:]...)
}

func (invocation PublicInvocation) Validate() error {
	if invocation.SchemaVersion != PublicInvocationSchemaVersion {
		return fmt.Errorf("public invocation schemaVersion must be %d", PublicInvocationSchemaVersion)
	}
	command := strings.ToLower(strings.TrimSpace(invocation.Command))
	profile, ok := PublicProfileMap()[command]
	if !ok || command == "" {
		return fmt.Errorf("public invocation command is not in the ReKit catalog: %s", strings.TrimSpace(invocation.Command))
	}
	if invocation.Command != command {
		return fmt.Errorf("public invocation command must use canonical lowercase identity: %s", command)
	}
	if profile.HeavyTool || profile.AuthorityConfirmed {
		return fmt.Errorf("public invocation command exceeds the no-heavy-tool/no-authority boundary: %s", command)
	}
	for _, argument := range invocation.Arguments {
		if strings.ContainsRune(argument, '\x00') {
			return fmt.Errorf("public invocation arguments must not contain NUL bytes")
		}
		if strings.EqualFold(argument, "-Command") || strings.EqualFold(argument, "--command") {
			return fmt.Errorf("public invocation arguments must not override the command identity")
		}
	}
	return ValidatePublicInvocationSelectors(command, invocation.Arguments)
}

func ValidatePublicInvocationSelectors(command string, arguments []string) error {
	command = strings.ToLower(strings.TrimSpace(command))
	positionalSelector := false
	laneFlags := 0
	nameFlags := 0
	for index := 0; index < len(arguments); index++ {
		argument := strings.TrimSpace(arguments[index])
		if strings.EqualFold(argument, "-Lane") || strings.EqualFold(argument, "--lane") {
			laneFlags++
			if laneFlags > 1 {
				return fmt.Errorf("public invocation repeats -Lane selector")
			}
			if index+1 >= len(arguments) || strings.TrimSpace(arguments[index+1]) == "" || strings.HasPrefix(strings.TrimSpace(arguments[index+1]), "-") {
				return fmt.Errorf("public invocation -Lane selector is missing a value")
			}
			index++
			continue
		}
		if command == Start && (strings.EqualFold(argument, "-Name") || strings.EqualFold(argument, "--name")) {
			nameFlags++
			if nameFlags > 1 {
				return fmt.Errorf("public invocation repeats -Name selector")
			}
			if index+1 >= len(arguments) || strings.TrimSpace(arguments[index+1]) == "" || strings.HasPrefix(strings.TrimSpace(arguments[index+1]), "-") {
				return fmt.Errorf("public invocation -Name selector is missing a value")
			}
			index++
			continue
		}
		if strings.HasPrefix(argument, "-") {
			if publicInvocationSelectorValueFlag(argument) && index+1 < len(arguments) {
				index++
			}
			continue
		}
		if argument != "" && publicInvocationCommandHasPositionalSelector(command) {
			positionalSelector = true
		}
	}
	if laneFlags > 0 && positionalSelector {
		return fmt.Errorf("public invocation must not specify both positional and -Lane selectors")
	}
	if command == Start && nameFlags > 0 && positionalSelector {
		return fmt.Errorf("public invocation must not specify both positional and -Name selectors")
	}
	return nil
}

func publicInvocationCommandHasPositionalSelector(command string) bool {
	switch command {
	case Start, Handoff, Complete, Reopen, Continue, Reconcile:
		return true
	default:
		return false
	}
}

func publicInvocationSelectorValueFlag(flag string) bool {
	switch strings.ToLower(strings.TrimSpace(flag)) {
	case "-target", "--target", "-pack", "--pack", "-format", "--format", "-name", "--name", "-executor", "--executor", "-actor", "--actor", "-reason", "--reason", "-action", "--action", "-summary", "--summary", "-evidencerefs", "--evidence-refs", "-interventionid", "--intervention-id", "-expectedexecutorgeneration", "--expected-executor-generation", "-expectedcontinueplansha256", "--expected-continue-plan-sha256", "-expectedcompleteplansha256", "--expected-complete-plan-sha256", "-expectedcontrolplansha256", "--expected-control-plan-sha256", "-expectedhandoffplansha256", "--expected-handoff-plan-sha256", "-expectedinitplansha256", "--expected-init-plan-sha256", "-expectedonboardingplansha256", "--expected-onboarding-plan-sha256", "-expectedmigrationplansha256", "--expected-migration-plan-sha256", "-expectedreopenplansha256", "--expected-reopen-plan-sha256", "-controlpublicationstamp", "--control-publication-stamp", "-handoffpublicationstamp", "--handoff-publication-stamp", "-reopenpublicationstamp", "--reopen-publication-stamp":
		return true
	default:
		return false
	}
}

func (invocation PublicInvocation) CLIArgs() ([]string, error) {
	if err := invocation.Validate(); err != nil {
		return nil, err
	}
	return append([]string{"-Command", invocation.Command}, invocation.Arguments...), nil
}

func (invocation PublicInvocation) Render() (string, error) {
	return invocation.RenderForEntrypoint(LegacyPublicEntrypoint)
}

func (invocation PublicInvocation) RenderForEntrypoint(entrypoint string) (string, error) {
	if err := invocation.Validate(); err != nil {
		return "", err
	}
	entrypoint = strings.TrimSpace(entrypoint)
	if entrypoint != CurrentPublicEntrypoint && entrypoint != LegacyPublicEntrypoint {
		return "", fmt.Errorf("unsupported public invocation entrypoint: %s", entrypoint)
	}
	parts := []string{entrypoint, invocation.Command}
	for _, argument := range invocation.Arguments {
		parts = append(parts, renderPublicInvocationArgument(argument))
	}
	return strings.Join(parts, " "), nil
}

func (invocation PublicInvocation) HasFlag(name string) bool {
	for _, argument := range invocation.Arguments {
		if strings.EqualFold(argument, name) {
			return true
		}
	}
	return false
}

// QualifyPublicNextAction applies the safe public default for executable next
// actions. Exact, valid preview/apply invocations remain unchanged.
func QualifyPublicNextAction(invocation PublicInvocation) (PublicInvocation, bool, error) {
	if err := invocation.Validate(); err != nil {
		return PublicInvocation{}, false, err
	}
	qualified := invocation
	qualified.Arguments = append([]string{}, invocation.Arguments...)
	contract, ok := MutationContractFor(qualified.Command, "")
	if !ok || !contract.ExecutablePlanValidation {
		return qualified, false, nil
	}
	hasWhatIf := qualified.HasFlag("-WhatIf") || qualified.HasFlag("--what-if")
	hasApply := qualified.HasFlag("-Apply") || qualified.HasFlag("--apply")
	if hasWhatIf && hasApply {
		return PublicInvocation{}, false, fmt.Errorf("public %s next action must use exactly one WhatIf or Apply phase", qualified.Command)
	}
	format, formatPresent, formatValid := qualified.FlagValue("-Format", "--format")
	if formatPresent && !formatValid {
		return PublicInvocation{}, false, fmt.Errorf("public next action contains an invalid or duplicate format binding")
	}
	if formatPresent && !strings.EqualFold(strings.TrimSpace(format), "json") {
		return PublicInvocation{}, false, fmt.Errorf("public executable plan next action requires -Format json")
	}
	changed := false
	if !hasWhatIf && !hasApply {
		qualified.Arguments = append(qualified.Arguments, "-WhatIf")
		changed = true
	}
	if !formatPresent {
		qualified.Arguments = append(qualified.Arguments, "-Format", "json")
		changed = true
	}
	if changed {
		var err error
		qualified, err = NewPublicInvocation(qualified.Command, qualified.Arguments...)
		if err != nil {
			return PublicInvocation{}, false, err
		}
	}
	if err := ValidateExecutablePlanInvocation(qualified); err != nil {
		return PublicInvocation{}, false, err
	}
	return qualified, changed, nil
}

// ValidateExecutableContinueInvocation enforces the public review/apply phase
// contract. Preview requests are unbound; Apply requests carry one exact plan.
func ValidateExecutableContinueInvocation(invocation PublicInvocation) error {
	return ValidateExecutablePlanInvocation(invocation)
}

// ValidateExecutablePlanInvocation validates the phase and expected plan binding
// for a typed public mutation that has a stable plan-hash flag.
func ValidateExecutablePlanInvocation(invocation PublicInvocation) error {
	if err := invocation.Validate(); err != nil {
		return err
	}
	flag, aliases, ok := publicPlanBinding(invocation.Command)
	if !ok {
		return fmt.Errorf("executable plan validation is unsupported for %s", invocation.Command)
	}
	format, formatPresent, formatValid := invocation.FlagValue("-Format", "--format")
	if !formatPresent || !formatValid || !strings.EqualFold(strings.TrimSpace(format), "json") {
		return fmt.Errorf("executable plan invocation requires a unique -Format json binding")
	}
	hasWhatIf := invocation.HasFlag("-WhatIf") || invocation.HasFlag("--what-if")
	hasApply := invocation.HasFlag("-Apply") || invocation.HasFlag("--apply")
	planSHA256, planPresent, planValid := invocation.FlagValue(aliases...)
	if !planValid {
		return plancontract.InvalidBinding(invocation.Command, flag, hasApply, planSHA256)
	}
	if planPresent {
		if _, err := plancontract.ValidatePhase(invocation.Command, flag, hasWhatIf, hasApply, planSHA256); err != nil {
			return err
		}
		return nil
	}
	_, err := plancontract.ValidatePhase(invocation.Command, flag, hasWhatIf, hasApply, "")
	return err
}

func publicPlanBinding(command string) (string, []string, bool) {
	contract, ok := MutationContractFor(command, "")
	if !ok || contract.Currentness != MutationCurrentnessStrictPlan || contract.ExpectedFlag == "" {
		return "", nil, false
	}
	return contract.ExpectedFlag, append([]string{}, contract.ExpectedAliases...), true
}

// PromoteContinuePreviewToApply converts the exact typed preview returned by
// the public next-action owner into its explicit, snapshot-bound Apply phase.
func PromoteContinuePreviewToApply(invocation PublicInvocation, expectedPlanSHA256 string) (PublicInvocation, error) {
	if err := invocation.Validate(); err != nil {
		return PublicInvocation{}, err
	}
	if invocation.Command != Continue {
		return PublicInvocation{}, fmt.Errorf("continue preview promotion requires a continue invocation")
	}
	hasWhatIf := invocation.HasFlag("-WhatIf") || invocation.HasFlag("--what-if")
	hasApply := invocation.HasFlag("-Apply") || invocation.HasFlag("--apply")
	if !hasWhatIf || hasApply {
		return PublicInvocation{}, fmt.Errorf("continue preview promotion requires an exact WhatIf-only invocation")
	}
	planSHA256 := strings.ToLower(strings.TrimSpace(expectedPlanSHA256))
	if !validPublicInvocationSHA256(planSHA256) {
		return PublicInvocation{}, fmt.Errorf("continue preview promotion requires a valid expected plan sha256")
	}
	if _, present, valid := invocation.FlagValue("-ExpectedContinuePlanSha256", "--expected-continue-plan-sha256"); present || !valid {
		return PublicInvocation{}, fmt.Errorf("continue preview invocation must not already bind an expected continue plan sha256")
	}
	arguments := append([]string{}, invocation.Arguments...)
	for index, argument := range arguments {
		if strings.EqualFold(argument, "-WhatIf") || strings.EqualFold(argument, "--what-if") {
			arguments[index] = "-Apply"
		}
	}
	arguments = append(arguments, "-ExpectedContinuePlanSha256", planSHA256)
	promoted, err := NewPublicInvocation(invocation.Command, arguments...)
	if err != nil {
		return PublicInvocation{}, err
	}
	if err := ValidateExecutableContinueInvocation(promoted); err != nil {
		return PublicInvocation{}, err
	}
	return promoted, nil
}

func validPublicInvocationSHA256(value string) bool {
	_, err := plancontract.NormalizeSHA256(value)
	return err == nil
}

func (invocation PublicInvocation) FlagValue(names ...string) (string, bool, bool) {
	value := ""
	present := false
	for index, argument := range invocation.Arguments {
		matched := false
		for _, name := range names {
			if strings.EqualFold(argument, name) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if present || index+1 >= len(invocation.Arguments) || strings.HasPrefix(strings.TrimSpace(invocation.Arguments[index+1]), "-") {
			return "", true, false
		}
		present = true
		value = invocation.Arguments[index+1]
	}
	return value, present, true
}

func (invocation PublicInvocation) Equivalent(other PublicInvocation) bool {
	if invocation.SchemaVersion != other.SchemaVersion || invocation.Command != other.Command || len(invocation.Arguments) != len(other.Arguments) {
		return false
	}
	for index := range invocation.Arguments {
		if invocation.Arguments[index] != other.Arguments[index] {
			return false
		}
	}
	return true
}

func renderPublicInvocationArgument(argument string) string {
	if argument != "" && !strings.ContainsAny(argument, " \t\r\n\"") {
		return argument
	}
	return `"` + strings.ReplaceAll(argument, `"`, `\"`) + `"`
}

func splitPublicInvocation(text string) ([]string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("public invocation is empty")
	}
	fields := []string{}
	var current strings.Builder
	inQuote := false
	inField := false
	for index := 0; index < len(text); index++ {
		char := text[index]
		if inQuote {
			inField = true
			if char == '\\' && index+1 < len(text) && text[index+1] == '"' {
				current.WriteByte('"')
				index++
				continue
			}
			if char == '"' {
				inQuote = false
				continue
			}
			current.WriteByte(char)
			continue
		}
		switch char {
		case ' ', '\t', '\n', '\r':
			if inField {
				fields = append(fields, current.String())
				current.Reset()
				inField = false
			}
		case '"':
			inQuote = true
			inField = true
		default:
			inField = true
			current.WriteByte(char)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote in public invocation")
	}
	if inField {
		fields = append(fields, current.String())
	}
	return fields, nil
}
