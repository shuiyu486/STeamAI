//go:build !windows

package fs

func rejectReparsePath(string) error      { return nil }
func rejectReparseAncestors(string) error { return nil }
