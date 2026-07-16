package commands

import (
	"slices"
	"strings"
	"testing"
)

func TestPublicCommandCatalog(t *testing.T) {
	commands := Public()
	if len(commands) != 19 || !slices.IsSorted(commands) {
		t.Fatalf("unexpected public command catalog: %v", commands)
	}
	for _, command := range []string{"attach", "bootstrap", "continue", "doctor", "gate", "handoff", "init", "note", "overview", "packs", "plan-subagents", "promote", "release-check", "repair", "start", "status", "sync", "update", "validate"} {
		if !slices.Contains(commands, command) || !IsPublic(command) || !IsPublic(" "+command+" ") {
			t.Fatalf("public command %s missing or not recognized: %v", command, commands)
		}
	}
	for _, blocked := range []string{"debug", "dump", "network", "authority", "confirmed", ""} {
		if IsPublic(blocked) {
			t.Fatalf("blocked command %q must not be public", blocked)
		}
	}
}

func TestUnsupportedErrorNamesSupportedSurface(t *testing.T) {
	err := UnsupportedError("debug")
	if err == nil {
		t.Fatal("UnsupportedError returned nil")
	}
	message := err.Error()
	for _, expected := range []string{"debug", "supported commands:", "release-check", "status", GoNativeAlternativePattern} {
		if !strings.Contains(message, expected) {
			t.Fatalf("unsupported command error missing %q: %s", expected, message)
		}
	}
}
