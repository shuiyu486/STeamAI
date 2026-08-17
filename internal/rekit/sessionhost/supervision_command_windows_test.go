//go:build windows

package sessionhost

import (
	"os/exec"
	"testing"
)

func configureContainedSupervisorCommandForTest(cmd *exec.Cmd) {
	cmd.SysProcAttr.CreationFlags &^= supervisionBreakaway
}

func TestConfigureContainedSupervisorCommandForTestKeepsDetachWithoutBreakaway(t *testing.T) {
	cmd := exec.Command("unused")
	configureSupervisorCommand(cmd)
	if cmd.SysProcAttr.CreationFlags&supervisionBreakaway == 0 {
		t.Fatal("production supervisor no longer requests job breakaway")
	}
	configureContainedSupervisorCommandForTest(cmd)
	if cmd.SysProcAttr.CreationFlags&supervisionBreakaway != 0 {
		t.Fatal("contained test supervisor still requests job breakaway")
	}
	want := uint32(supervisionDetached | supervisionNewProcessGroup)
	if cmd.SysProcAttr.CreationFlags&want != want || !cmd.SysProcAttr.HideWindow {
		t.Fatalf("contained test supervisor flags=%#x hideWindow=%t", cmd.SysProcAttr.CreationFlags, cmd.SysProcAttr.HideWindow)
	}
}
