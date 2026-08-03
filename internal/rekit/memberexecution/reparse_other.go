//go:build !windows

package memberexecution

func rejectReparsePath(string) error      { return nil }
func rejectReparseAncestors(string) error { return nil }
