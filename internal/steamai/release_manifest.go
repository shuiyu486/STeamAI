package steamai

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var releaseVersionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`)

type ReleaseManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Version       string `json:"version"`
	Revision      string `json:"revision"`
	WindowsAMD64  string `json:"windowsAmd64Sha256"`
}

func ParseReleaseManifest(reader io.Reader, requested string) (ReleaseManifest, error) {
	decoder := json.NewDecoder(io.LimitReader(reader, 1<<20))
	decoder.DisallowUnknownFields()
	var manifest ReleaseManifest
	if err := decoder.Decode(&manifest); err != nil {
		return ReleaseManifest{}, fmt.Errorf("解析 release manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ReleaseManifest{}, errors.New("release manifest 后存在额外 JSON")
	}
	if manifest.SchemaVersion != 1 || (requested != "" && manifest.Version != requested) || !releaseVersionPattern.MatchString(manifest.Version) ||
		!gitIdentityPattern.MatchString(manifest.Revision) || !sha256Pattern.MatchString(manifest.WindowsAMD64) {
		return ReleaseManifest{}, errors.New("release manifest identity 无效")
	}
	return manifest, nil
}

func parseReleaseManifest(data []byte, requested string) (ReleaseManifest, error) {
	return ParseReleaseManifest(strings.NewReader(string(data)), requested)
}

func parseLatestReleaseManifest(data []byte) (ReleaseManifest, error) {
	return parseReleaseManifest(data, "")
}
