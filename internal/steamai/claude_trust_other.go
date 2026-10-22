//go:build !windows

package steamai

import "time"

type claudeConfigLock struct{}

func (claudeConfigLock) release() {}

func acquireClaudeConfigLock(string, time.Duration) (claudeConfigLock, error) {
	return claudeConfigLock{}, errUnsupportedPlatform
}

func replaceClaudeConfigAtomically(string, []byte) error {
	return errUnsupportedPlatform
}
