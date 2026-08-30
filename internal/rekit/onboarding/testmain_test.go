package onboarding

import (
	"fmt"
	"os"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
)

func TestMain(m *testing.M) {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	restoreExecutableSource := runtimebundle.SetExecutableSourceForTest(executable)
	restoreRuntimeBuilders := syncreview.SetRuntimeBundleBuildersForTest(runtimebundle.BuildWithExecutable)
	code := m.Run()
	restoreRuntimeBuilders()
	restoreExecutableSource()
	os.Exit(code)
}
