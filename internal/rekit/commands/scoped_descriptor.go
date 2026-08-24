package commands

import (
	"fmt"
	"sort"
	"strings"
)

// CommandScope identifies one command route without pretending that a command
// with several mutation protocols has one command-wide currentness contract.
type CommandScope struct {
	Command string `json:"command"`
	Mode    string `json:"mode"`
}

// ScopedCommandDescriptor is a composed view over the existing command profile
// and mutation contract catalogs. Runtime packages attach their binder,
// validator, and handler to Scope; they do not duplicate policy fields here.
type ScopedCommandDescriptor struct {
	Scope    CommandScope      `json:"scope"`
	Profile  PublicProfile     `json:"profile"`
	Mutation *MutationContract `json:"mutation,omitempty"`
}

// ScopedCommandDescriptors returns one descriptor per exact mutation mode and
// one default descriptor for each non-mutating command.
func ScopedCommandDescriptors() ([]ScopedCommandDescriptor, error) {
	profiles := PublicProfiles()
	contracts := MutationContracts()
	byCommand := map[string][]MutationContract{}
	for _, contract := range contracts {
		byCommand[contract.Command] = append(byCommand[contract.Command], contract)
	}

	descriptors := make([]ScopedCommandDescriptor, 0, len(profiles)+len(contracts))
	for _, profile := range profiles {
		commandContracts := byCommand[profile.Command]
		if len(commandContracts) == 0 {
			descriptors = append(descriptors, ScopedCommandDescriptor{
				Scope:   CommandScope{Command: profile.Command, Mode: MutationModeDefault},
				Profile: profile,
			})
			continue
		}
		for _, contract := range commandContracts {
			contractCopy := cloneMutationContract(contract)
			descriptors = append(descriptors, ScopedCommandDescriptor{
				Scope: CommandScope{
					Command: contract.Command,
					Mode:    contract.Mode,
				},
				Profile:  profile,
				Mutation: &contractCopy,
			})
		}
	}
	sort.Slice(descriptors, func(i, j int) bool {
		if descriptors[i].Scope.Command == descriptors[j].Scope.Command {
			return descriptors[i].Scope.Mode < descriptors[j].Scope.Mode
		}
		return descriptors[i].Scope.Command < descriptors[j].Scope.Command
	})
	if err := ValidateScopedCommandDescriptors(descriptors); err != nil {
		return nil, err
	}
	return cloneScopedCommandDescriptors(descriptors), nil
}

// ScopedCommandDescriptorFor resolves only an exact scope. An omitted mode is
// the canonical default mode; mixed-mode commands therefore fail closed.
func ScopedCommandDescriptorFor(command, mode string) (ScopedCommandDescriptor, bool) {
	command = strings.ToLower(strings.TrimSpace(command))
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = MutationModeDefault
	}
	descriptors, err := ScopedCommandDescriptors()
	if err != nil {
		return ScopedCommandDescriptor{}, false
	}
	for _, descriptor := range descriptors {
		if descriptor.Scope.Command == command && descriptor.Scope.Mode == mode {
			return cloneScopedCommandDescriptor(descriptor), true
		}
	}
	return ScopedCommandDescriptor{}, false
}

func ScopedCommandDescriptorsFor(command string) []ScopedCommandDescriptor {
	command = strings.ToLower(strings.TrimSpace(command))
	descriptors, err := ScopedCommandDescriptors()
	if err != nil {
		return nil
	}
	out := []ScopedCommandDescriptor{}
	for _, descriptor := range descriptors {
		if descriptor.Scope.Command == command {
			out = append(out, cloneScopedCommandDescriptor(descriptor))
		}
	}
	return out
}

// ValidateScopedCommandDescriptors verifies catalog coverage and composition.
// It accepts an explicit slice so runtime registries can validate the same
// descriptor identities before binding handlers.
func ValidateScopedCommandDescriptors(descriptors []ScopedCommandDescriptor) error {
	profiles := PublicProfileMap()
	public := PublicSet()
	seenScopes := map[CommandScope]bool{}
	seenCommands := map[string]int{}
	seenMutationScopes := map[CommandScope]bool{}

	for _, contract := range MutationContracts() {
		seenMutationScopes[CommandScope{Command: contract.Command, Mode: contract.Mode}] = false
	}
	for _, descriptor := range descriptors {
		scope := CommandScope{
			Command: strings.ToLower(strings.TrimSpace(descriptor.Scope.Command)),
			Mode:    strings.ToLower(strings.TrimSpace(descriptor.Scope.Mode)),
		}
		if descriptor.Scope != scope || scope.Command == "" || scope.Mode == "" {
			return fmt.Errorf("scoped command descriptor has non-canonical scope: %+v", descriptor.Scope)
		}
		if seenScopes[scope] {
			return fmt.Errorf("duplicate scoped command descriptor: %s mode %s", scope.Command, scope.Mode)
		}
		seenScopes[scope] = true
		seenCommands[scope.Command]++

		profile, ok := profiles[scope.Command]
		if !ok || !public[scope.Command] {
			return fmt.Errorf("scoped command descriptor is outside the public profile catalog: %s", scope.Command)
		}
		if descriptor.Profile != profile {
			return fmt.Errorf("scoped command descriptor profile drifted for %s mode %s", scope.Command, scope.Mode)
		}
		if descriptor.Mutation == nil {
			if profile.IsMutation || profile.MutationBoundary != BoundaryReadOnly || scope.Mode != MutationModeDefault {
				return fmt.Errorf("scoped command descriptor omits mutation policy for %s mode %s", scope.Command, scope.Mode)
			}
			continue
		}
		contract := descriptor.Mutation
		if !profile.IsMutation || contract.Command != scope.Command || contract.Mode != scope.Mode || !contract.Confirmed {
			return fmt.Errorf("scoped command descriptor mutation policy drifted for %s mode %s", scope.Command, scope.Mode)
		}
		contractScope := CommandScope{Command: contract.Command, Mode: contract.Mode}
		if _, ok := seenMutationScopes[contractScope]; !ok {
			return fmt.Errorf("scoped command descriptor references an unknown mutation policy: %s mode %s", scope.Command, scope.Mode)
		}
		seenMutationScopes[contractScope] = true
	}

	for command := range public {
		if seenCommands[command] == 0 {
			return fmt.Errorf("public command has no scoped descriptor: %s", command)
		}
	}
	for scope, covered := range seenMutationScopes {
		if !covered {
			return fmt.Errorf("mutation contract has no scoped descriptor: %s mode %s", scope.Command, scope.Mode)
		}
	}
	return nil
}

func cloneScopedCommandDescriptors(descriptors []ScopedCommandDescriptor) []ScopedCommandDescriptor {
	out := make([]ScopedCommandDescriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		out = append(out, cloneScopedCommandDescriptor(descriptor))
	}
	return out
}

func cloneScopedCommandDescriptor(descriptor ScopedCommandDescriptor) ScopedCommandDescriptor {
	if descriptor.Mutation != nil {
		contract := cloneMutationContract(*descriptor.Mutation)
		descriptor.Mutation = &contract
	}
	return descriptor
}
