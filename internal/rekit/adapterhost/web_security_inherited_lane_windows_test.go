//go:build windows

package adapterhost

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/websecurity"
)

func TestRunBoundedReplayChildRejectsForgedParentLaneHandleBeforeNetworkSink(t *testing.T) {
	fixture := newWebSecurityAuthorizedFixture(t, websecurity.ReplayAdapterID, true)
	childOpt := boundedReplayChildOptionsForFixture(t, fixture)

	forgedPath := filepath.Join(t.TempDir(), "forged-lane-lock.lease")
	if err := os.WriteFile(forgedPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	pointer, err := syscall.UTF16PtrFromString(forgedPath)
	if err != nil {
		t.Fatal(err)
	}
	const fileShareDelete = 0x00000004
	handle, err := syscall.CreateFile(
		pointer,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|fileShareDelete,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	childOpt.ParentLaneLeaseHandle = uintptr(handle)

	result, err := RunBoundedReplayChild(childOpt)
	if err == nil || !strings.Contains(err.Error(), "inherited lane mutation lease handle changed") {
		t.Fatalf("forged parent lane handle error = %v", err)
	}
	if result.Result.Delivery.Attempts != 0 {
		t.Fatalf("forged parent lane handle crossed bounded replay network sink: %+v", result)
	}
}
