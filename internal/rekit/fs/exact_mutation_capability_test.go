package fs

import (
	"runtime"
	"testing"
)

func TestHandleBoundExactMutationCapabilityMatchesPlatform(t *testing.T) {
	want := runtime.GOOS == "windows"
	if got := HandleBoundExactMutationSupported(); got != want {
		t.Fatalf("handle-bound exact mutation supported=%t, want %t on %s", got, want, runtime.GOOS)
	}
}
