package adapterhost

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

type liveAcceptanceCaseIdentity struct {
	parent     *os.Root
	parentPath string
	name       string
	parentInfo os.FileInfo
	caseInfo   os.FileInfo
	markerName string
	marker     []byte
}

func captureLiveAcceptanceCase(path, markerName string, marker []byte) (*liveAcceptanceCaseIdentity, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	parentPath := filepath.Dir(path)
	name := filepath.Base(path)
	parentInfo, err := os.Lstat(parentPath)
	if err != nil || !parentInfo.IsDir() || parentInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("adapter live case parent must be a non-symlink directory: %s: %w", parentPath, err)
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*liveAcceptanceCaseIdentity, error) {
		_ = parent.Close()
		return nil, err
	}
	openedParent, err := parent.Lstat(".")
	if err != nil || !openedParent.IsDir() || !os.SameFile(parentInfo, openedParent) {
		return fail(fmt.Errorf("adapter live case parent changed while opening: %s", parentPath))
	}
	caseInfo, err := parent.Lstat(name)
	if err != nil || !caseInfo.IsDir() || caseInfo.Mode()&os.ModeSymlink != 0 {
		return fail(fmt.Errorf("adapter live case root must be a non-symlink directory: %s: %w", path, err))
	}
	identity := &liveAcceptanceCaseIdentity{
		parent: parent, parentPath: parentPath, name: name,
		parentInfo: openedParent, caseInfo: caseInfo,
		markerName: markerName, marker: append([]byte{}, marker...),
	}
	if err := identity.validateNamedRoot(name, false); err != nil {
		return fail(err)
	}
	return identity, nil
}

func (identity *liveAcceptanceCaseIdentity) bindMarker(markerName string, marker []byte) error {
	if identity == nil || identity.parent == nil || markerName == "" || len(marker) == 0 {
		return fmt.Errorf("adapter live case marker binding is missing")
	}
	identity.markerName = markerName
	identity.marker = append([]byte{}, marker...)
	return identity.validateNamedRoot(identity.name, false)
}

func (identity *liveAcceptanceCaseIdentity) Close() error {
	if identity == nil || identity.parent == nil {
		return nil
	}
	err := identity.parent.Close()
	identity.parent = nil
	return err
}

func (identity *liveAcceptanceCaseIdentity) cleanup(afterQuarantine func(string) error) error {
	if identity == nil || identity.parent == nil {
		return fmt.Errorf("adapter live cleanup has no captured case identity")
	}
	if err := identity.validateNamedRoot(identity.name, false); err != nil {
		return err
	}
	quarantine := identity.name + ".cleanup"
	if _, err := identity.parent.Lstat(quarantine); err == nil {
		return fmt.Errorf("adapter live cleanup quarantine already exists: %s", filepath.Join(identity.parentPath, quarantine))
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := quarantineLiveAcceptanceCase(identity, quarantine); err != nil {
		return err
	}
	if err := identity.validateNamedRoot(quarantine, true); err != nil {
		return err
	}
	quarantinePath := filepath.Join(identity.parentPath, quarantine)
	if afterQuarantine != nil {
		if err := afterQuarantine(quarantinePath); err != nil {
			return err
		}
	}
	if current, err := identity.parent.Lstat(identity.name); err == nil {
		return fmt.Errorf("adapter live cleanup refuses a replacement created at the case root: %s mode=%s", filepath.Join(identity.parentPath, identity.name), current.Mode())
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := identity.validateNamedRoot(quarantine, true); err != nil {
		return err
	}
	if err := validateLiveAcceptanceCleanupTree(quarantinePath); err != nil {
		return err
	}
	if err := identity.parent.RemoveAll(quarantine); err != nil {
		return err
	}
	currentParent, err := os.Lstat(identity.parentPath)
	if err != nil || !os.SameFile(identity.parentInfo, currentParent) {
		return fmt.Errorf("adapter live case parent changed during cleanup: %s", identity.parentPath)
	}
	return nil
}

func (identity *liveAcceptanceCaseIdentity) validateNamedRoot(name string, quarantined bool) error {
	info, err := identity.parent.Lstat(name)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(identity.caseInfo, info) {
		stage := "case root"
		if quarantined {
			stage = "quarantined case root"
		}
		return fmt.Errorf("adapter live %s identity changed: %s: %w", stage, filepath.Join(identity.parentPath, name), err)
	}
	root, err := identity.parent.OpenRoot(name)
	if err != nil {
		return err
	}
	defer root.Close()
	opened, err := root.Lstat(".")
	if err != nil || !opened.IsDir() || !os.SameFile(identity.caseInfo, opened) {
		return fmt.Errorf("adapter live case identity changed while opening: %s", filepath.Join(identity.parentPath, name))
	}
	if identity.markerName != "" {
		marker, err := root.ReadFile(identity.markerName)
		if err != nil || !bytes.Equal(marker, identity.marker) {
			return fmt.Errorf("adapter live case marker changed: %s: %w", filepath.Join(identity.parentPath, name, identity.markerName), err)
		}
	}
	return nil
}
