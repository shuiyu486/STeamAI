package hostcmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRejectsLaneForMaintenanceRoutes(t *testing.T) {
	for _, fixture := range []struct {
		name string
		args []string
	}{
		{name: "live acceptance", args: []string{"-live-acceptance", "-lane", "binary-analysis-main"}},
		{name: "internal supervisor", args: []string{"-internal-supervisor", "spec.json", "-internal-supervisor-sha256", strings.Repeat("a", 64), "-lane", "binary-analysis-main"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(fixture.args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "-lane is supported only") {
				t.Fatalf("route accepted lane selector: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}
