//go:build !windows

package packmemoryconsumption

func rejectReparsePath(string) error      { return nil }
func rejectReparseAncestors(string) error { return nil }
