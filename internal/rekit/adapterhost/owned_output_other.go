//go:build !windows

package adapterhost

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type ownedOutput struct {
	info os.FileInfo
}

func captureOwnedOutput(file *os.File) (*ownedOutput, error) {
	if file == nil {
		return nil, fmt.Errorf("owned output file is missing")
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("owned output must be a regular file")
	}
	return &ownedOutput{info: info}, nil
}

func validateOwnedOutput(root *os.Root, rel string, owned *ownedOutput, expected []byte) error {
	if root == nil || owned == nil {
		return fmt.Errorf("owned output identity is missing: %s", rel)
	}
	current, err := root.Lstat(filepath.FromSlash(rel))
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(owned.info, current) {
		return fmt.Errorf("owned output identity changed: %s", rel)
	}
	data, err := root.ReadFile(filepath.FromSlash(rel))
	if err != nil || !bytes.Equal(data, expected) {
		return fmt.Errorf("owned output bytes changed: %s", rel)
	}
	return nil
}

func removeOwnedOutput(
	root *os.Root,
	rel string,
	owned *ownedOutput,
	afterIdentity func(string) error,
	afterQuarantineIdentity func(string, string) error,
) error {
	if root == nil || owned == nil {
		return nil
	}
	current, err := root.Lstat(filepath.FromSlash(rel))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(owned.info, current) {
		return fmt.Errorf("refuse cleanup because owned output identity changed: %s", rel)
	}
	if afterIdentity != nil {
		if err := afterIdentity(rel); err != nil {
			return err
		}
	}
	quarantine, err := ownedOutputQuarantinePath(root, rel)
	if err != nil {
		return err
	}
	if err := isolateOwnedOutputNoReplace(root, rel, quarantine); err != nil {
		return fmt.Errorf("isolate exact owned output in cleanup quarantine %s: %w", rel, err)
	}
	quarantined, err := root.Lstat(filepath.FromSlash(quarantine))
	if err != nil || !quarantined.Mode().IsRegular() || !os.SameFile(owned.info, quarantined) {
		return errors.Join(
			fmt.Errorf("refuse cleanup because quarantined output identity changed: %s", rel),
			preserveQuarantinedReplacement(root, quarantine, rel),
		)
	}
	if afterQuarantineIdentity != nil {
		if err := afterQuarantineIdentity(rel, quarantine); err != nil {
			return errors.Join(err, preserveQuarantinedReplacement(root, quarantine, rel))
		}
	}
	quarantined, err = root.Lstat(filepath.FromSlash(quarantine))
	if err != nil || !quarantined.Mode().IsRegular() || !os.SameFile(owned.info, quarantined) {
		return errors.Join(
			fmt.Errorf(
				"refuse cleanup because quarantined output identity changed after validation: %s",
				rel,
			),
			preserveQuarantinedReplacement(root, quarantine, rel),
		)
	}
	return fmt.Errorf(
		"%w; cleanup quarantine retained because handle-bound deletion is unavailable: %s",
		errOwnedOutputIsolated,
		quarantine,
	)
}

func ownedOutputQuarantinePath(root *os.Root, rel string) (string, error) {
	parent := filepath.Dir(filepath.FromSlash(rel))
	base := filepath.Base(filepath.FromSlash(rel))
	for range 8 {
		var nonce [16]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return "", err
		}
		name := "." + base + ".rekit-owned-cleanup-" + hex.EncodeToString(nonce[:])
		quarantine := filepath.ToSlash(filepath.Join(parent, name))
		if _, err := root.Lstat(filepath.FromSlash(quarantine)); os.IsNotExist(err) {
			return quarantine, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("allocate owned output cleanup quarantine: %s", rel)
}

func preserveQuarantinedReplacement(root *os.Root, quarantine, rel string) error {
	if _, err := root.Lstat(filepath.FromSlash(rel)); err == nil {
		return fmt.Errorf(
			"cleanup quarantine preserved because canonical output path is occupied: %s",
			quarantine,
		)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := root.Link(
		filepath.FromSlash(quarantine),
		filepath.FromSlash(rel),
	); err != nil {
		return fmt.Errorf(
			"cleanup quarantine preserved but canonical output restore failed: %s: %w",
			quarantine,
			err,
		)
	}
	return nil
}
