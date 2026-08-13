package sessionhost

import (
	"strings"
	"testing"
)

func TestRunPublicApplyCommandRejectsUnexpectedOrReboundRoute(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "unexpected command", command: `/rekit status -Apply -Format json`, want: "expected bounded reopen route"},
		{name: "missing Apply", command: `/rekit reopen mission -Format json`, want: "expected bounded reopen route"},
		{name: "command override", command: `/rekit reopen mission -Apply -Command status -Format json`, want: "bounded command"},
		{name: "target override", command: `/rekit reopen mission -Apply -Target other -Format json`, want: "must not override"},
		{name: "pack override", command: `/rekit reopen mission -Apply --pack other -Format json`, want: "must not override"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runPublicApplyCommand(test.command, "reopen", t.TempDir(), "_template", nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("runPublicApplyCommand error=%v want substring %q", err, test.want)
			}
		})
	}
}
