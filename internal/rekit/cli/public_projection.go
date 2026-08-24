package cli

import (
	"fmt"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/commands"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectstate"
)

// projectPublicProjection is the resolved state-root owner for one public
// response envelope. Typed field renderers consume its entrypoint; they do not
// resolve or choose between current and legacy state independently.
type projectPublicProjection struct {
	entrypoint string
}

func resolveProjectPublicProjection(caseRoot string) (projectPublicProjection, error) {
	entrypoint, err := projectstate.PublicEntrypoint(caseRoot)
	if err != nil {
		return projectPublicProjection{}, err
	}
	projection := projectPublicProjection{entrypoint: strings.TrimSpace(entrypoint)}
	if err := projection.validate(); err != nil {
		return projectPublicProjection{}, err
	}
	return projection, nil
}

func (projection projectPublicProjection) validate() error {
	switch projection.entrypoint {
	case commands.CurrentPublicEntrypoint, commands.LegacyPublicEntrypoint:
		return nil
	default:
		return fmt.Errorf("unsupported project public entrypoint %q", projection.entrypoint)
	}
}

func (projection projectPublicProjection) command(command string) (string, error) {
	if err := projection.validate(); err != nil {
		return "", err
	}
	return projectPublicCommandForEntrypoint(command, projection.entrypoint)
}

func (projection projectPublicProjection) visibleCommand(command string) (string, error) {
	if err := projection.validate(); err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(command)
	if strings.HasPrefix(trimmed, projection.entrypoint+" ") {
		return trimmed, nil
	}
	return projection.command(trimmed)
}
