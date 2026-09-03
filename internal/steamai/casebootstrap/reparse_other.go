//go:build !windows

package casebootstrap

func rejectReparse(string) error { return nil }
