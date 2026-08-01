package sync

import (
	"fmt"
	"os"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/testenv"
)

func TestMain(m *testing.M) {
	if _, err := testenv.ConfigureCanonicalTempRoot(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
