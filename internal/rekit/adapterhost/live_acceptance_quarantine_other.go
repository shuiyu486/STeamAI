//go:build !windows

package adapterhost

func quarantineLiveAcceptanceCase(identity *liveAcceptanceCaseIdentity, quarantine string) error {
	return identity.parent.Rename(identity.name, quarantine)
}
