package promote

import (
	"fmt"
	"os"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/runtimebundle"
	syncreview "github.com/shuiyu486/re-context-kits/internal/rekit/sync"
	"github.com/shuiyu486/re-context-kits/internal/rekit/testenv"
)

func TestMain(m *testing.M) {
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	restoreExecutableSource := runtimebundle.SetExecutableSourceForTest(executable)
	restoreRuntimeBuilders := syncreview.SetRuntimeBundleBuildersForTest(runtimebundle.BuildWithExecutable)
	if _, err := testenv.ConfigureCanonicalTempRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	restoreRuntimeBuilders()
	restoreExecutableSource()
	os.Exit(code)
}
