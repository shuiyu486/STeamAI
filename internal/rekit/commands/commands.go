package commands

import (
	"fmt"
	"sort"
	"strings"
)

const (
	Attach        = "attach"
	Bootstrap     = "bootstrap"
	Continue      = "continue"
	Doctor        = "doctor"
	Gate          = "gate"
	Handoff       = "handoff"
	Init          = "init"
	Note          = "note"
	Overview      = "overview"
	Packs         = "packs"
	PlanSubagents = "plan-subagents"
	Promote       = "promote"
	ReleaseCheck  = "release-check"
	Repair        = "repair"
	Start         = "start"
	Status        = "status"
	Sync          = "sync"
	Update        = "update"
	Validate      = "validate"
)

const (
	DefaultCommand             = Status
	GoNativeEntrypoint         = "cmd/rekit"
	GoNativeAlternativePattern = "go run ./cmd/rekit -- -Command <command>"
)

var publicCommands = []string{
	Attach,
	Bootstrap,
	Continue,
	Doctor,
	Gate,
	Handoff,
	Init,
	Note,
	Overview,
	Packs,
	PlanSubagents,
	Promote,
	ReleaseCheck,
	Repair,
	Start,
	Status,
	Sync,
	Update,
	Validate,
}

func Public() []string {
	out := append([]string{}, publicCommands...)
	sort.Strings(out)
	return out
}

func PublicSet() map[string]bool {
	set := map[string]bool{}
	for _, command := range publicCommands {
		set[command] = true
	}
	return set
}

func IsPublic(name string) bool {
	return PublicSet()[strings.ToLower(strings.TrimSpace(name))]
}

func SupportedList() string {
	return strings.Join(Public(), ", ")
}

func AlternativeFor(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "<command>"
	}
	return "go run ./cmd/rekit -- -Command " + name
}

func UnsupportedError(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "<empty>"
	}
	return fmt.Errorf("go backend does not implement public command %q; supported commands: %s; use %s", name, SupportedList(), AlternativeFor("<command>"))
}
