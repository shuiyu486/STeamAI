package statemigration

import (
	"fmt"
	"os"
	"strings"
)

// Identity is a stable filesystem identity used to prove that a migration is
// still operating on the exact project root reviewed during preview.
type Identity struct {
	Scheme       string `json:"scheme"`
	Device       uint64 `json:"device,omitempty"`
	Inode        uint64 `json:"inode,omitempty"`
	VolumeSerial uint32 `json:"volumeSerial,omitempty"`
	FileIndex    uint64 `json:"fileIndex,omitempty"`
}

// Validate rejects incomplete or internally inconsistent identities.
func (identity Identity) Validate() error {
	switch identity.Scheme {
	case "unix-dev-inode-v1":
		if identity.Device == 0 || identity.Inode == 0 || identity.VolumeSerial != 0 || identity.FileIndex != 0 {
			return fmt.Errorf("Unix filesystem identity is incomplete or inconsistent")
		}
	case "windows-volume-file-index-v1":
		if identity.VolumeSerial == 0 || identity.FileIndex == 0 || identity.Device != 0 || identity.Inode != 0 {
			return fmt.Errorf("Windows filesystem identity is incomplete or inconsistent")
		}
	default:
		return fmt.Errorf("unsupported filesystem identity scheme %q", identity.Scheme)
	}
	return nil
}

// IdentityForRoot returns the stable identity of an already-open directory.
// The caller retains ownership of root.
func IdentityForRoot(root *os.Root) (Identity, error) {
	if root == nil {
		return Identity{}, fmt.Errorf("filesystem root is nil")
	}
	identity, err := identityForRoot(root)
	if err != nil {
		return Identity{}, err
	}
	if err := identity.Validate(); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

// IdentityForFile returns the stable physical identity of an already-open
// regular file. The caller retains ownership of file.
func IdentityForFile(file *os.File) (Identity, error) {
	if file == nil {
		return Identity{}, fmt.Errorf("filesystem file is nil")
	}
	info, err := file.Stat()
	if err != nil {
		return Identity{}, err
	}
	if !info.Mode().IsRegular() {
		return Identity{}, fmt.Errorf("filesystem identity requires a regular file")
	}
	identity, err := identityForFile(file)
	if err != nil {
		return Identity{}, err
	}
	if err := identity.Validate(); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

// OpenRootIdentity opens path as an anchored filesystem root and captures its
// stable identity. The caller must close the returned root.
func OpenRootIdentity(path string) (*os.Root, Identity, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, Identity{}, fmt.Errorf("filesystem root path is empty")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, Identity{}, err
	}
	identity, err := IdentityForRoot(root)
	if err != nil {
		_ = root.Close()
		return nil, Identity{}, err
	}
	return root, identity, nil
}
