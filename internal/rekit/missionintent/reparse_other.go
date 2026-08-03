//go:build !windows

package missionintent

func rejectReparsePath(string) error { return nil }
