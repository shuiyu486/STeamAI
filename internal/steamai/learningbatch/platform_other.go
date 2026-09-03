//go:build !windows

package learningbatch

func rejectReparse(string) error { return nil }
