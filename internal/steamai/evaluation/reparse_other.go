//go:build !windows

package evaluation

func rejectReparse(string) error { return nil }
