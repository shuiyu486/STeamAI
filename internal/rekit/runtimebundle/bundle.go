package runtimebundle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	rekitfs "github.com/shuiyu486/re-context-kits/internal/rekit/fs"
	"github.com/shuiyu486/re-context-kits/internal/rekit/packidentity"
)

const (
	SchemaVersion = 1
	Kind          = "steamai-runtime-bundle"
	Layout        = "steamai-v1"
	ManifestRel   = "runtime/manifest.json"

	maxManifestBytes   = 1 << 20
	maxAssetFileBytes  = 8 << 20
	maxExecutableBytes = 64 << 20
	maxBundleFiles     = 2048
)

type Artifact struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Manifest struct {
	SchemaVersion int        `json:"schemaVersion"`
	Kind          string     `json:"kind"`
	Layout        string     `json:"layout"`
	GOOS          string     `json:"goos"`
	GOARCH        string     `json:"goarch"`
	AssetRoot     string     `json:"assetRoot"`
	PacksRoot     string     `json:"packsRoot"`
	Pack          string     `json:"pack"`
	Executable    Artifact   `json:"executable"`
	Files         []Artifact `json:"files"`
}

type Publication struct {
	Path       string
	Kind       string
	SourcePath string
	Content    []byte
}

type Plan struct {
	Pack           string
	Manifest       Manifest
	ManifestData   []byte
	ManifestSHA256 string
	Publications   []Publication
}

var executableSource = defaultExecutableSource

func defaultExecutableSource() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current STeamAI runtime executable: %w", err)
	}
	return filepath.Abs(path)
}

func SetExecutableSourceForTest(path string) func() {
	previous := executableSource
	executableSource = func() (string, error) { return filepath.Abs(path) }
	return func() { executableSource = previous }
}

func SourceExecutable() (string, error) { return executableSource() }

func Build(repoRoot, pack string) (Plan, error) {
	executable, err := executableSource()
	if err != nil {
		return Plan{}, err
	}
	return BuildWithExecutable(repoRoot, pack, executable)
}

func BuildWithExecutable(repoRoot, pack, executable string) (Plan, error) {
	repoRoot, err := filepath.Abs(strings.TrimSpace(repoRoot))
	if err != nil {
		return Plan{}, err
	}
	pack = strings.TrimSpace(pack)
	if err := packidentity.Validate(pack); err != nil {
		return Plan{}, err
	}
	if pack == "" || filepath.IsAbs(pack) || strings.ContainsAny(pack, `/\\`) || pack == "." || pack == ".." || strings.Contains(pack, "..") {
		return Plan{}, fmt.Errorf("invalid STeamAI bundle pack: %s", pack)
	}
	executable, err = filepath.Abs(strings.TrimSpace(executable))
	if err != nil {
		return Plan{}, err
	}

	publications := []Publication{}
	artifacts := []Artifact{}
	addSource := func(rel, kind, source string, limit int64) error {
		rel, err = cleanRel(rel)
		if err != nil {
			return err
		}
		data, err := readStableRegular(source, "STeamAI bundle source", limit)
		if err != nil {
			return err
		}
		artifact := artifactFor(rel, kind, data)
		artifacts = append(artifacts, artifact)
		publications = append(publications, Publication{Path: rel, Kind: kind, SourcePath: source})
		return nil
	}

	packRoot := filepath.Join(repoRoot, "packs", pack)
	if err := collectTree(repoRoot, packRoot, filepath.ToSlash(filepath.Join("packs", pack)), "pack-asset", skipPackRuntimeAsset, addSource); err != nil {
		return Plan{}, err
	}
	commonRoot := filepath.Join(repoRoot, "common")
	if err := collectTree(repoRoot, commonRoot, "common", "common-asset", nil, addSource); err != nil {
		return Plan{}, err
	}
	for _, rel := range []string{
		"rekit/templates/steamai-project/SKILL.md",
		"rekit/schemas/instance.schema.yml",
		"rekit/schemas/pack-manifest.schema.yml",
		"rekit/tests/catalog.json",
	} {
		if err := addSource(rel, "runtime-asset", filepath.Join(repoRoot, filepath.FromSlash(rel)), maxAssetFileBytes); err != nil {
			return Plan{}, err
		}
	}
	if len(artifacts) == 0 || len(artifacts) > maxBundleFiles {
		return Plan{}, fmt.Errorf("STeamAI bundle file count is outside bounds: %d", len(artifacts))
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	sort.Slice(publications, func(i, j int) bool { return publications[i].Path < publications[j].Path })
	if err := validateArtifactSet(artifacts, pack); err != nil {
		return Plan{}, err
	}

	executableData, err := readStableRegular(executable, "STeamAI runtime executable", maxExecutableBytes)
	if err != nil {
		return Plan{}, err
	}
	executableRel := filepath.ToSlash(filepath.Join("runtime", "bin", executableName()))
	executableArtifact := artifactFor(executableRel, "runtime-executable", executableData)
	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		Kind:          Kind,
		Layout:        Layout,
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
		AssetRoot:     ".",
		PacksRoot:     "packs",
		Pack:          pack,
		Executable:    executableArtifact,
		Files:         artifacts,
	}
	manifestData, err := canonicalManifest(manifest)
	if err != nil {
		return Plan{}, err
	}
	manifestSHA := hash(manifestData)
	publications = append(publications,
		Publication{Path: executableRel, Kind: "runtime-executable", SourcePath: executable},
		Publication{Path: ManifestRel, Kind: "runtime-bundle-manifest", Content: manifestData},
	)
	return Plan{Pack: pack, Manifest: manifest, ManifestData: manifestData, ManifestSHA256: manifestSHA, Publications: publications}, nil
}

func Validate(assetRoot, manifestRel, expectedManifestSHA256, expectedPack string) (Manifest, error) {
	assetRoot, err := filepath.Abs(strings.TrimSpace(assetRoot))
	if err != nil {
		return Manifest{}, err
	}
	if _, err := rekitfs.ValidateNonReparseDirectory(assetRoot, "STeamAI asset root"); err != nil {
		return Manifest{}, err
	}
	if strings.TrimSpace(manifestRel) == "" {
		manifestRel = ManifestRel
	}
	manifestRel, err = cleanRel(manifestRel)
	if err != nil || manifestRel != ManifestRel {
		return Manifest{}, fmt.Errorf("STeamAI bundle manifest must use %s", ManifestRel)
	}
	manifestPath := filepath.Join(assetRoot, filepath.FromSlash(manifestRel))
	data, err := rekitfs.ReadStableRegularFileAnchored(assetRoot, manifestPath, "STeamAI bundle manifest", maxManifestBytes)
	if err != nil {
		return Manifest{}, err
	}
	manifest, err := ValidateManifestData(data, expectedManifestSHA256, expectedPack)
	if err != nil {
		return Manifest{}, err
	}
	all := append([]Artifact{manifest.Executable}, manifest.Files...)
	for _, artifact := range all {
		path := filepath.Join(assetRoot, filepath.FromSlash(artifact.Path))
		limit := int64(maxAssetFileBytes)
		if artifact.Kind == "runtime-executable" {
			limit = maxExecutableBytes
		}
		content, err := rekitfs.ReadStableRegularFileAnchored(assetRoot, path, "STeamAI bundle artifact", limit)
		if err != nil {
			return Manifest{}, fmt.Errorf("STeamAI bundle artifact is unavailable: %s: %w", artifact.Path, err)
		}
		if int64(len(content)) != artifact.Size || hash(content) != strings.ToLower(artifact.SHA256) {
			return Manifest{}, fmt.Errorf("STeamAI bundle artifact hash or layout mismatch: %s", artifact.Path)
		}
		if artifact.Kind == "runtime-executable" && runtime.GOOS != "windows" {
			info, statErr := os.Stat(path)
			if statErr != nil || info.Mode().Perm()&0o111 == 0 {
				return Manifest{}, fmt.Errorf("STeamAI runtime is not executable: %s", path)
			}
		}
	}
	if err := validateExactLayout(assetRoot, manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// ValidateManifestData validates the canonical manifest contract without
// consulting an installation. Recovery envelopes use it to bind their exact
// embedded asset inventory before any project leaf is published.
func ValidateManifestData(data []byte, expectedManifestSHA256, expectedPack string) (Manifest, error) {
	if len(data) == 0 || len(data) > maxManifestBytes {
		return Manifest{}, fmt.Errorf("STeamAI bundle manifest is empty or exceeds %d bytes", maxManifestBytes)
	}
	expectedManifestSHA256 = strings.ToLower(strings.TrimSpace(expectedManifestSHA256))
	if expectedManifestSHA256 != "" && (!validSHA256(expectedManifestSHA256) || hash(data) != expectedManifestSHA256) {
		return Manifest{}, fmt.Errorf("STeamAI bundle manifest SHA-256 does not match project metadata")
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("invalid STeamAI bundle manifest: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return Manifest{}, fmt.Errorf("invalid STeamAI bundle manifest trailing data")
	}
	canonical, err := canonicalManifest(manifest)
	if err != nil || !bytes.Equal(data, canonical) {
		return Manifest{}, fmt.Errorf("STeamAI bundle manifest is not canonical")
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.Kind != Kind || manifest.Layout != Layout || manifest.GOOS != runtime.GOOS || manifest.GOARCH != runtime.GOARCH || manifest.AssetRoot != "." || manifest.PacksRoot != "packs" {
		return Manifest{}, fmt.Errorf("STeamAI bundle manifest identity or platform is invalid")
	}
	if expectedPack = strings.TrimSpace(expectedPack); expectedPack != "" {
		if err := packidentity.Validate(expectedPack); err != nil {
			return Manifest{}, err
		}
		if manifest.Pack != expectedPack {
			if err := packidentity.Validate(manifest.Pack); err != nil {
				return Manifest{}, err
			}
			return Manifest{}, fmt.Errorf("STeamAI bundle pack does not match project metadata: %s", manifest.Pack)
		}
	}
	if err := packidentity.Validate(manifest.Pack); err != nil {
		return Manifest{}, err
	}
	if err := validateArtifactSet(manifest.Files, manifest.Pack); err != nil {
		return Manifest{}, err
	}
	executableRel := filepath.ToSlash(filepath.Join("runtime", "bin", executableName()))
	if manifest.Executable.Path != executableRel || manifest.Executable.Kind != "runtime-executable" || !validSHA256(manifest.Executable.SHA256) || manifest.Executable.Size < 1 || manifest.Executable.Size > maxExecutableBytes {
		return Manifest{}, fmt.Errorf("STeamAI bundle executable layout or binding is invalid")
	}
	return manifest, nil
}

func ManifestSHA256(assetRoot string) (string, error) {
	assetRoot, err := filepath.Abs(strings.TrimSpace(assetRoot))
	if err != nil {
		return "", err
	}
	path := filepath.Join(assetRoot, filepath.FromSlash(ManifestRel))
	data, err := rekitfs.ReadStableRegularFileAnchored(assetRoot, path, "STeamAI bundle manifest", maxManifestBytes)
	if err != nil {
		return "", err
	}
	return hash(data), nil
}

func ExecutablePath(assetRoot string, manifest Manifest) string {
	return filepath.Join(assetRoot, filepath.FromSlash(manifest.Executable.Path))
}

// AssetRootForExecutable identifies a project-local process by its canonical
// .steamai/runtime/bin location. Any executable beneath .steamai but outside
// that exact location fails closed instead of being treated as a central kit
// maintenance binary.
func AssetRootForExecutable(executable string) (string, bool, error) {
	executable, err := filepath.Abs(strings.TrimSpace(executable))
	if err != nil {
		return "", false, err
	}
	if err := rekitfs.ValidateNoReparseComponents(executable); err != nil {
		return "", false, fmt.Errorf("validate running STeamAI executable path: %w", err)
	}
	for current := filepath.Dir(executable); ; current = filepath.Dir(current) {
		if strings.EqualFold(filepath.Base(current), ".steamai") {
			expected := filepath.Join(current, "runtime", "bin", executableName())
			if !rekitfs.SamePath(executable, expected) {
				return "", false, fmt.Errorf("project-local STeamAI executable is outside the canonical runtime layout: %s", executable)
			}
			return current, true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false, nil
		}
	}
}

func ValidateExecutableIdentity(assetRoot, executable string, manifest Manifest) error {
	assetRoot, err := filepath.Abs(strings.TrimSpace(assetRoot))
	if err != nil {
		return err
	}
	executable, err = filepath.Abs(strings.TrimSpace(executable))
	if err != nil {
		return err
	}
	expected := ExecutablePath(assetRoot, manifest)
	same, err := rekitfs.SameExistingPath(executable, expected)
	if err != nil || !same || !rekitfs.SamePath(executable, expected) {
		return fmt.Errorf("running STeamAI executable does not match the manifest-bound process image: %s", executable)
	}
	return nil
}

func ExecutableName() string { return executableName() }

func PublishForTest(caseRoot, repoRoot, pack, executable string) (Plan, error) {
	plan, err := BuildWithExecutable(repoRoot, pack, executable)
	if err != nil {
		return Plan{}, err
	}
	assetRoot := filepath.Join(caseRoot, ".steamai")
	for _, publication := range plan.Publications {
		target := filepath.Join(assetRoot, filepath.FromSlash(publication.Path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return Plan{}, err
		}
		data := publication.Content
		if publication.SourcePath != "" {
			data, err = readStableRegular(publication.SourcePath, "STeamAI bundle test publication", maxExecutableBytes)
			if err != nil {
				return Plan{}, err
			}
		}
		mode := os.FileMode(0o644)
		if publication.Kind == "runtime-executable" {
			mode = 0o755
		}
		if err := os.WriteFile(target, data, mode); err != nil {
			return Plan{}, err
		}
	}
	return plan, nil
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "steamai.exe"
	}
	return "steamai"
}

func skipPackRuntimeAsset(rel string) bool {
	lower := strings.ToLower(filepath.ToSlash(rel))
	return strings.HasSuffix(lower, ".ps1") ||
		strings.HasPrefix(lower, "scripts/") ||
		lower == "promote-candidates" || strings.HasPrefix(lower, "promote-candidates/") ||
		lower == "tooling/candidates" || strings.HasPrefix(lower, "tooling/candidates/") ||
		strings.HasSuffix(lower, "/.gitkeep") || lower == ".gitkeep"
}

func collectTree(_ string, sourceRoot, destinationRoot, kind string, skip func(string) bool, add func(string, string, string, int64) error) error {
	if err := rekitfs.ValidateTreeNoReparse(sourceRoot, "STeamAI bundle source tree"); err != nil {
		return err
	}
	return filepath.WalkDir(sourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skip != nil && skip(rel) {
			return nil
		}
		destination := filepath.ToSlash(filepath.Join(filepath.FromSlash(destinationRoot), filepath.FromSlash(rel)))
		return add(destination, kind, path, maxAssetFileBytes)
	})
}

func validateArtifactSet(files []Artifact, pack string) error {
	if len(files) == 0 || len(files) > maxBundleFiles {
		return fmt.Errorf("STeamAI bundle file count is outside bounds: %d", len(files))
	}
	seen := map[string]bool{}
	last := ""
	required := map[string]bool{
		filepath.ToSlash(filepath.Join("packs", pack, "manifest.yml")): false,
		"common/policies/manifest.yml":                                 false,
		"common/policies/README.md":                                    false,
		"rekit/templates/steamai-project/SKILL.md":                     false,
		"rekit/schemas/instance.schema.yml":                            false,
		"rekit/schemas/pack-manifest.schema.yml":                       false,
		"rekit/tests/catalog.json":                                     false,
	}
	for _, artifact := range files {
		rel, err := cleanRel(artifact.Path)
		if err != nil || rel != artifact.Path || rel <= last || seen[strings.ToLower(rel)] || strings.HasSuffix(strings.ToLower(rel), ".ps1") {
			return fmt.Errorf("STeamAI bundle artifact set is invalid or unordered: %s", artifact.Path)
		}
		expectedKind := ""
		switch {
		case strings.HasPrefix(rel, filepath.ToSlash(filepath.Join("packs", pack))+"/"):
			expectedKind = "pack-asset"
		case strings.HasPrefix(rel, "common/"):
			expectedKind = "common-asset"
		case strings.HasPrefix(rel, "rekit/"):
			expectedKind = "runtime-asset"
		}
		if artifact.Kind != expectedKind || !validSHA256(artifact.SHA256) || artifact.Size < 1 || artifact.Size > maxAssetFileBytes {
			return fmt.Errorf("STeamAI bundle artifact role or binding is invalid: %s", artifact.Path)
		}
		seen[strings.ToLower(rel)] = true
		last = rel
		if _, ok := required[rel]; ok {
			required[rel] = true
		}
	}
	for path, present := range required {
		if !present {
			return fmt.Errorf("STeamAI bundle is missing required asset: %s", path)
		}
	}
	return nil
}

func validateExactLayout(assetRoot string, manifest Manifest) error {
	expected := map[string]bool{ManifestRel: true, manifest.Executable.Path: true}
	for _, artifact := range manifest.Files {
		expected[artifact.Path] = true
	}
	actual := map[string]bool{}
	for _, root := range []string{"runtime", "packs", "common", "rekit"} {
		paths, err := rekitfs.WalkRegularFilesAnchored(assetRoot, root, "STeamAI bundle controlled layout", maxBundleFiles+2)
		if err != nil {
			return err
		}
		for _, path := range paths {
			rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
			if filepath.IsAbs(filepath.FromSlash(rel)) {
				var err error
				rel, err = filepath.Rel(assetRoot, filepath.FromSlash(rel))
				if err != nil {
					return err
				}
				rel = filepath.ToSlash(rel)
			}
			actual[rel] = true
		}
	}
	if len(actual) != len(expected) {
		return fmt.Errorf("STeamAI bundle controlled layout contains unplanned files")
	}
	for path := range expected {
		if !actual[path] {
			return fmt.Errorf("STeamAI bundle controlled layout is missing %s", path)
		}
	}
	return nil
}

func canonicalManifest(manifest Manifest) ([]byte, error) {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func artifactFor(path, kind string, data []byte) Artifact {
	return Artifact{Path: path, Kind: kind, SHA256: hash(data), Size: int64(len(data))}
}

func cleanRel(value string) (string, error) {
	rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(value))))
	if rel == "" || rel == "." || filepath.IsAbs(filepath.FromSlash(rel)) || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("invalid STeamAI bundle relative path: %q", value)
	}
	return rel, nil
}

func readStableRegular(path, label string, limit int64) ([]byte, error) {
	if err := rekitfs.ValidateNoReparseComponents(path); err != nil {
		return nil, err
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("%s must be a bounded regular file: %s: %w", label, path, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return nil, fmt.Errorf("%s must be a bounded regular file: %s", label, path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	opened, statErr := file.Stat()
	data, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	after, afterErr := os.Lstat(path)
	if statErr != nil || readErr != nil || closeErr != nil || afterErr != nil || !os.SameFile(before, opened) || !os.SameFile(opened, after) || int64(len(data)) != opened.Size() || int64(len(data)) > limit {
		return nil, fmt.Errorf("%s changed while reading: %s", label, path)
	}
	return data, nil
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
