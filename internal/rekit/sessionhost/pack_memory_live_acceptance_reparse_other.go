//go:build !windows

package sessionhost

func rejectPackMemoryAcceptanceReparse(string) error { return nil }
