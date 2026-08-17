package fs

import (
	"io/fs"
	"os"
)

type exactFileHandle interface {
	Stat() (os.FileInfo, error)
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
	Sync() error
	Close() error
}

type exactCreatedFile interface {
	Stat() (os.FileInfo, error)
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
	Write([]byte) (int, error)
	Sync() error
	Commit() error
	Abort() error
}

type exactMutationGuard interface {
	Validate() error
	Close() error
}

func expectedTreeMode(mode fs.FileMode) fs.FileMode {
	return mode & (fs.ModeType | fs.ModePerm)
}
