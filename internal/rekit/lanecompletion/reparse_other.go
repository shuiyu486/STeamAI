//go:build !windows

package lanecompletion

func rejectReparsePath(string) error { return nil }
