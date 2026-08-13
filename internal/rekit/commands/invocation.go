package commands

import (
	"fmt"
	"strings"
)

const PublicInvocationSchemaVersion = 1

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
	if len(fields) < 2 || fields[0] != "/rekit" || strings.TrimSpace(fields[1]) == "" {
		return PublicInvocation{}, fmt.Errorf("public invocation must begin with /rekit <command>")
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
	case "-target", "--target", "-pack", "--pack", "-format", "--format", "-name", "--name", "-executor", "--executor", "-actor", "--actor", "-reason", "--reason", "-summary", "--summary", "-evidencerefs", "--evidence-refs", "-interventionid", "--intervention-id", "-expectedexecutorgeneration", "--expected-executor-generation", "-expectedcompleteplansha256", "--expected-complete-plan-sha256", "-expectedhandoffplansha256", "--expected-handoff-plan-sha256", "-expectedreopenplansha256", "--expected-reopen-plan-sha256", "-handoffpublicationstamp", "--handoff-publication-stamp", "-reopenpublicationstamp", "--reopen-publication-stamp":
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
	if err := invocation.Validate(); err != nil {
		return "", err
	}
	parts := []string{"/rekit", invocation.Command}
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
