package hostcmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
)

// This test crosses the public flag parser and typed JSON result boundary used
// by the project-local skill. The sessionhost classifier tests cover the pure
// operation matrix; this one protects the externally advertised lane contract.
func TestRunDailyLaneSelectorDoesNotBecomeControl(t *testing.T) {
	caseRoot := filepath.Join(t.TempDir(), "missing-case")
	for _, fixture := range []struct {
		name      string
		extra     []string
		wantMode  string
		wantError string
	}{
		{name: "resume", wantMode: string(sessionhost.DailyOperationResume), wantError: "daily target without committed onboarding requires -goal"},
		{name: "correction", extra: []string{"-correction", "recheck the evidence"}, wantMode: string(sessionhost.DailyOperationCorrection), wantError: "daily target without committed onboarding requires -goal"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			args := []string{"-daily", "-target", caseRoot, "-lane", "binary-analysis-main"}
			args = append(args, fixture.extra...)
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), fixture.wantError) || strings.Contains(stderr.String(), "daily control") {
				t.Fatalf("daily %s exit=%d stderr=%q", fixture.name, code, stderr.String())
			}
			var result sessionhost.DailyResult
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("daily %s JSON: %v\n%s", fixture.name, err, stdout.String())
			}
			if result.Mode != fixture.wantMode || result.ExecutionControl != nil || result.SessionLaunches != 0 {
				t.Fatalf("daily %s result = %+v", fixture.name, result)
			}
		})
	}
}
