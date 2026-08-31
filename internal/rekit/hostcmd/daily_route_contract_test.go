package hostcmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRejectsMutatedSuccessorExactApplyArgs(t *testing.T) {
	sha := strings.Repeat("a", 64)
	canonical := []string{
		"-daily", "-target", "project", "-goal", "successor goal", "-actor", "reviewed-actor",
		"-successor-apply", "-successor-publication-stamp", "20260831-120000000",
		"-expected-successor-plan-sha256", sha,
	}
	fixtures := []struct {
		name string
		args []string
	}{
		{name: "missing daily", args: append([]string{}, canonical[1:]...)},
		{name: "added model", args: append(append([]string{}, canonical...), "-model", "claude-sonnet-5")},
		{name: "reordered", args: []string{
			"-daily", "-goal", "successor goal", "-target", "project", "-actor", "reviewed-actor",
			"-successor-apply", "-successor-publication-stamp", "20260831-120000000",
			"-expected-successor-plan-sha256", sha,
		}},
		{name: "uppercase sha", args: append(append([]string{}, canonical[:len(canonical)-1]...), strings.ToUpper(sha))},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Run(fixture.args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "exact fresh successorMission.applyArgs argv") {
				t.Fatalf("mutated successor argv accepted: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

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
