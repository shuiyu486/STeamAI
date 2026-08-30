package sync

import (
	"fmt"
	"os"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	"github.com/shuiyu486/re-context-kits/internal/rekit/testenv"
)

func TestMain(m *testing.M) {
	currentSyncRuntimeBundleBuilder = runtimebundle.BuildWithExecutable
	initRuntimeBundleBuilder = runtimebundle.BuildWithExecutable
	exclusiveInitRuntimeBundleBuilder = runtimebundle.BuildWithExecutable
	restoreExecutableSource := runtimebundle.SetExecutableSourceForTest(mustSyncTestExecutable())
	if _, err := testenv.ConfigureCanonicalTempRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	restoreExecutableSource()
	os.Exit(code)
}

func mustSyncTestExecutable() string {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return executable
}
