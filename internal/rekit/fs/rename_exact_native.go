package fs

import (
	"io/fs"
	"os"
)

type exactRenameRequest struct {
	RootPath      string
	SourceRel     string
	TargetRel     string
	SourceParent  *os.Root
	TargetParent  *os.Root
	SourceName    string
	TargetName    string
	ExpectedInfo  os.FileInfo
	ExpectedBytes []byte
	ExpectedMode  fs.FileMode
	ExpectedTree  *ExpectedTree
}
