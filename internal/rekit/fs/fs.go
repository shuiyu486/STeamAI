package fs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func FullPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return filepath.Abs(".")
	}
	return filepath.Abs(path)
}

func SafeJoin(root, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("relative path is empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be relative: %s", rel)
	}
	rootFull, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pathFull, err := filepath.Abs(filepath.Join(rootFull, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	rootClean := strings.TrimRight(filepath.Clean(rootFull), string(filepath.Separator))
	pathClean := strings.TrimRight(filepath.Clean(pathFull), string(filepath.Separator))
	if strings.EqualFold(pathClean, rootClean) {
		return pathClean, nil
	}
	prefix := rootClean + string(filepath.Separator)
	if !strings.HasPrefix(strings.ToLower(pathClean), strings.ToLower(prefix)) {
		return "", fmt.Errorf("path escapes root: %s", rel)
	}
	return pathClean, nil
}

func ReadText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func FileHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func FileSize(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsTextNonEmptyUnder(path string, limit int64) (int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("missing file: %s", path)
	}
	if strings.TrimSpace(string(b)) == "" {
		return 0, fmt.Errorf("empty file: %s", path)
	}
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if st.Size() > limit {
		return st.Size(), fmt.Errorf("file too large: %s %d > %d", path, st.Size(), limit)
	}
	return st.Size(), nil
}
