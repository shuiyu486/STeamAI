package fs

import (
	"go/build/constraint"
	"os"
	"strings"
	"testing"
)

func TestRemoveExactBuildConstraintsSelectOneImplementation(t *testing.T) {
	files := []string{
		"remove_exact_windows.go",
		"remove_exact_supported_unix.go",
		"remove_exact_unix.go",
		"remove_exact_other.go",
	}
	targets := []struct {
		name string
		tags map[string]bool
	}{
		{name: "windows", tags: map[string]bool{"windows": true}},
		{name: "linux", tags: map[string]bool{"linux": true, "unix": true}},
		{name: "darwin", tags: map[string]bool{"darwin": true, "unix": true}},
		{name: "freebsd", tags: map[string]bool{"freebsd": true, "unix": true}},
		{name: "plan9", tags: map[string]bool{"plan9": true}},
		{name: "js", tags: map[string]bool{"js": true}},
		{name: "wasip1", tags: map[string]bool{"wasip1": true}},
	}

	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			selected := []string{}
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				line, _, _ := strings.Cut(string(data), "\n")
				expr, err := constraint.Parse(line)
				if err != nil {
					t.Fatalf("parse %s: %v", file, err)
				}
				if expr.Eval(func(tag string) bool { return target.tags[tag] }) {
					selected = append(selected, file)
				}
			}
			if len(selected) != 1 {
				t.Fatalf("selected implementations=%v, want exactly one", selected)
			}
		})
	}
}
