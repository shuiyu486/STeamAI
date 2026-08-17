//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package sessionhost

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shuiyu486/re-context-kits/internal/rekit/projectexecution"
	"github.com/shuiyu486/re-context-kits/internal/rekit/projectlock"
)

func TestCurrentSupervisionRejectsUnsupportedDurableHandoffWithoutReplayState(t *testing.T) {
	if projectexecution.DurableHandoffSupported() {
		t.Skip("durable supervisor handoff is supported on this platform")
	}
	caseRoot, opt, pkg, childBeforeAcquire, childRelease :=
		projectExecutionLaunchFixture(t)
	opt.ExpectedClaudeExecutableSHA256 = strings.Repeat("e", 64)
	opt.ExpectedClaudeExecutablePublisher = liveAcceptanceClaudePublisher
	opt.Timeout = 250 * time.Millisecond

	cacheHome := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheHome)
	t.Setenv("HOME", cacheHome)
	childDone := filepath.Join(filepath.Dir(childBeforeAcquire), "child-done")
	t.Setenv(projectExecutionHelperBeforeEnv, childBeforeAcquire)
	t.Setenv(projectExecutionHelperReleaseEnv, childRelease)
	t.Setenv(projectExecutionHelperChildDoneEnv, childDone)
	t.Cleanup(func() { _ = os.WriteFile(childRelease, nil, 0o600) })

	lease, err := projectexecution.AcquireShared(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Unlock()
	opt.projectExecutionLease = lease

	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	casePath, err := filepath.Abs(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	supervisionPath := filepath.Join(
		cacheRoot,
		"rekit",
		"session-host",
		"v2",
		"cases",
		bytesSHA256([]byte(filepath.Clean(casePath))),
	)
	lockRoot, err := projectlock.WorkstreamRoot()
	if err != nil {
		t.Fatal(err)
	}
	projectKey, err := projectlock.CanonicalProjectKey(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	pendingPath := filepath.Join(
		lockRoot,
		"case-"+projectKey+".execution-v1.handoff.json",
	)
	cancellationPath := filepath.Join(
		lockRoot,
		"case-"+projectKey+".execution-v1.handoff-canceled",
	)

	for attempt := 1; attempt <= 2; attempt++ {
		run, launched, err := supervisedClaudeRun(
			context.Background(),
			opt,
			pkg,
			pkg.Launch.Attempt.Session,
			nil,
		)
		if err == nil || !strings.Contains(
			err.Error(),
			"requires handle-bound exact filesystem mutation support",
		) {
			t.Errorf("unsupported current supervision attempt %d error=%v", attempt, err)
		}
		if launched || run.started {
			t.Errorf(
				"unsupported current supervision attempt %d launched=%t run=%+v",
				attempt,
				launched,
				run,
			)
		}
	}

	for label, path := range map[string]string{
		"supervision namespace": supervisionPath,
		"pending handoff":       pendingPath,
		"cancellation history":  cancellationPath,
		"child acquire signal":  childBeforeAcquire,
		"child terminal signal": childDone,
	} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("unsupported current supervision wrote %s at %s: %v", label, path, err)
		}
	}
}
