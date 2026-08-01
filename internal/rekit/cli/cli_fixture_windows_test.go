//go:build windows

package cli

import "testing"

func TestCLIFixtureTargetWithinSourceAcrossWindowsVolumes(t *testing.T) {
	within, err := cliFixtureTargetWithinSource(`C:\source\repo`, `D:\temp\fixture`)
	if err != nil {
		t.Fatal(err)
	}
	if within {
		t.Fatal("different Windows volumes should be accepted")
	}
}
