//go:build !windows

package sync

func rejectExclusiveInitReparsePath(string) error { return nil }
