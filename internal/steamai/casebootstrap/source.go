package casebootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type gitEntry struct {
	Mode string
	Blob string
	Path string
}

func freezeSource(git, root, pack string) (frozenSource, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return frozenSource{}, err
	}
	if !packNamePattern.MatchString(pack) || strings.HasPrefix(pack, "_") {
		return frozenSource{}, errors.New("selected pack 名称无效")
	}
	if err := requirePlainDirectory(root); err != nil {
		return frozenSource{}, fmt.Errorf("canonical source 根目录无效: %w", err)
	}
	top, err := gitOutput(git, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return frozenSource{}, err
	}
	top = strings.TrimSpace(top)
	topInfo, topErr := os.Stat(top)
	rootInfo, rootErr := os.Stat(root)
	if topErr != nil || rootErr != nil || !os.SameFile(topInfo, rootInfo) {
		return frozenSource{}, errors.New("canonical source 必须是 Git worktree 根目录")
	}
	revision, err := gitOutput(git, root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return frozenSource{}, errors.New("canonical source 没有有效 HEAD")
	}
	revision = strings.TrimSpace(revision)
	roots := []string{
		".claude/skills/steamai/SKILL.md",
		"vnext/learning-feedback.md",
		"vnext/templates",
		"packs/" + pack,
		"common",
	}
	headEntries, err := gitTreeEntries(git, root, revision, roots)
	if err != nil {
		return frozenSource{}, err
	}
	indexEntries, err := gitIndexEntries(git, root, roots)
	if err != nil {
		return frozenSource{}, err
	}
	if len(indexEntries) == 0 {
		return frozenSource{}, ErrCollision
	}
	if err := rejectIntentToAdd(git, root, roots); err != nil {
		return frozenSource{}, err
	}
	if err := rejectUnexpectedSourcePaths(root, roots, indexEntries); err != nil {
		return frozenSource{}, err
	}
	headByPath := make(map[string]gitEntry, len(headEntries))
	for _, entry := range headEntries {
		headByPath[entry.Path] = entry
	}

	records := make([]SourceRecord, 0, len(indexEntries))
	byPath := make(map[string]SourceRecord, len(indexEntries))
	for _, entry := range indexEntries {
		path := filepath.Join(root, filepath.FromSlash(entry.Path))
		if err := requirePlainPath(root, path, false); err != nil {
			return frozenSource{}, fmt.Errorf("Fresh source path %s 无效: %w", entry.Path, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return frozenSource{}, err
		}
		contentBlob, err := gitOutput(git, root, "hash-object", "--path="+entry.Path, "--", entry.Path)
		if err != nil {
			return frozenSource{}, fmt.Errorf("计算 %s 当前 Git blob: %w", entry.Path, err)
		}
		headBlob := ""
		if head, ok := headByPath[entry.Path]; ok {
			headBlob = head.Blob
		}
		currentBlob := strings.TrimSpace(contentBlob)
		record := SourceRecord{
			Path: entry.Path, GitMode: entry.Mode, HeadBlob: headBlob,
			ContentBlob: currentBlob, SHA256: hashBytes(data),
			Bytes: len(data), Changed: currentBlob != headBlob,
			Data: append([]byte(nil), data...),
		}
		records = append(records, record)
		byPath[record.Path] = record
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Path < records[j].Path })
	if err := validateSelectedPack(records, pack); err != nil {
		return frozenSource{}, err
	}
	packTree, err := gitWriteTreeForRoot(git, root, "packs/"+pack)
	if err != nil {
		return frozenSource{}, err
	}
	commonTree, err := gitWriteTreeForRoot(git, root, "common")
	if err != nil {
		return frozenSource{}, err
	}
	diffArgs := append([]string{"diff", "--binary", "--full-index", "--no-ext-diff", revision, "--"}, roots...)
	diff, err := gitOutput(git, root, diffArgs...)
	if err != nil {
		return frozenSource{}, err
	}
	return frozenSource{
		Root: root, Git: git, Revision: revision,
		PackTree: packTree, CommonTree: commonTree,
		Digest: digestRecords(records), Records: records, ByPath: byPath, Diff: diff,
	}, nil
}

func validateSelectedPack(records []SourceRecord, pack string) error {
	byPath := make(map[string]SourceRecord, len(records))
	for _, record := range records {
		byPath[record.Path] = record
	}
	manifestPath := "packs/" + pack + "/manifest.yml"
	manifest, ok := byPath[manifestPath]
	if !ok || manifest.GitMode != "100644" {
		return errors.New("selected pack manifest 不存在或不是普通文件")
	}
	name, router, err := parsePackManifest(manifest.Data)
	if err != nil {
		return err
	}
	if name != pack {
		return errors.New("selected pack manifest name 与目录不一致")
	}
	routerPath := filepath.ToSlash(filepath.Clean(filepath.Join("packs", pack, filepath.FromSlash(router))))
	if !strings.HasPrefix(routerPath, "packs/"+pack+"/") {
		return errors.New("selected pack router 越出 pack")
	}
	if routerRecord, ok := byPath[routerPath]; !ok || routerRecord.GitMode != "100644" {
		return errors.New("selected pack router 不存在或不是普通文件")
	}
	return nil
}

func parsePackManifest(data []byte) (string, string, error) {
	var name, router string
	inEntrypoints := false
	for line := range strings.SplitSeq(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "name: "):
			if name != "" {
				return "", "", errors.New("manifest name 重复")
			}
			value, _ := strings.CutPrefix(line, "name: ")
			name = strings.Trim(strings.TrimSpace(value), "'\"")
		case line == "entrypoints:":
			if inEntrypoints {
				return "", "", errors.New("manifest entrypoints 重复")
			}
			inEntrypoints = true
		case inEntrypoints && strings.HasPrefix(line, "  router: "):
			if router != "" {
				return "", "", errors.New("manifest router 重复")
			}
			value, _ := strings.CutPrefix(line, "  router: ")
			router = strings.Trim(strings.TrimSpace(value), "'\"")
		case inEntrypoints && line != "" && !strings.HasPrefix(line, "  "):
			inEntrypoints = false
		}
	}
	if !packNamePattern.MatchString(name) || router == "" || filepath.IsAbs(filepath.FromSlash(router)) || strings.Contains(router, "\\") {
		return "", "", errors.New("selected pack manifest name/router 无效")
	}
	return name, router, nil
}

func gitTreeEntries(git, root, revision string, roots []string) ([]gitEntry, error) {
	args := append([]string{"ls-tree", "-r", "-z", revision, "--"}, roots...)
	data, err := gitOutputBytes(git, root, args...)
	if err != nil {
		return nil, err
	}
	var entries []gitEntry
	for _, raw := range bytes.Split(data, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		metadata, path, ok := strings.Cut(string(raw), "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			return nil, fmt.Errorf("不支持的 Git tree entry: %q", raw)
		}
		entries = append(entries, gitEntry{Mode: fields[0], Blob: fields[2], Path: filepath.ToSlash(path)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func gitIndexEntries(git, root string, roots []string) ([]gitEntry, error) {
	args := append([]string{"ls-files", "-z", "--stage", "--"}, roots...)
	data, err := gitOutputBytes(git, root, args...)
	if err != nil {
		return nil, err
	}
	var entries []gitEntry
	for _, raw := range bytes.Split(data, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		metadata, path, ok := strings.Cut(string(raw), "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 || fields[2] != "0" || (fields[0] != "100644" && fields[0] != "100755") {
			return nil, fmt.Errorf("Fresh source closure 存在 unmerged 或不支持的 index entry: %q", raw)
		}
		entries = append(entries, gitEntry{Mode: fields[0], Blob: fields[1], Path: filepath.ToSlash(path)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

func rejectIntentToAdd(git, root string, roots []string) error {
	args := append([]string{"ls-files", "--debug", "--"}, roots...)
	data, err := gitOutput(git, root, args...)
	if err != nil {
		return err
	}
	current := ""
	for line := range strings.SplitSeq(data, "\n") {
		switch {
		case line != "" && line[0] != ' ' && line[0] != '\t':
			current = line
		case current != "" && strings.TrimSpace(line) == "size: 0\tflags: 20004000":
			return fmt.Errorf("Fresh source closure 存在 intent-to-add entry: %s", current)
		}
	}
	return nil
}

func gitWriteTreeForRoot(git, root, path string) (string, error) {
	fullTree, err := gitOutput(git, root, "write-tree")
	if err != nil {
		return "", fmt.Errorf("计算 current index tree: %w", err)
	}
	entry, err := gitOutput(git, root, "ls-tree", strings.TrimSpace(fullTree), "--", path)
	if err != nil {
		return "", fmt.Errorf("读取 current index tree %s: %w", path, err)
	}
	fields := strings.Fields(entry)
	if len(fields) < 3 || fields[0] != "040000" || fields[1] != "tree" {
		return "", fmt.Errorf("current index tree %s 不存在或不是目录", path)
	}
	return fields[2], nil
}

func rejectUnexpectedSourcePaths(root string, roots []string, tracked []gitEntry) error {
	allowed := make(map[string]bool, len(tracked))
	for _, entry := range tracked {
		allowed[entry.Path] = true
	}
	for _, relRoot := range roots {
		pathRoot := filepath.Join(root, filepath.FromSlash(relRoot))
		info, err := os.Lstat(pathRoot)
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			if !allowed[filepath.ToSlash(relRoot)] {
				return fmt.Errorf("Fresh source closure 含未跟踪文件: %s", relRoot)
			}
			continue
		}
		err = filepath.WalkDir(pathRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := rejectReparse(path); err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("Fresh source closure 含非普通文件: %s", path)
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if !allowed[rel] {
				return fmt.Errorf("Fresh source closure 含未跟踪文件: %s", rel)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func gitOutput(git, dir string, args ...string) (string, error) {
	data, err := gitOutputBytes(git, dir, args...)
	return string(data), err
}

func gitOutputBytes(git, dir string, args ...string) ([]byte, error) {
	cmd := exec.Command(git, args...)
	cmd.Dir = dir
	cmd.Env = withoutEnvironment(os.Environ(), "CLAUDECODE")
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return data, nil
}

func withoutEnvironment(env []string, names ...string) []string {
	blocked := map[string]bool{}
	for _, name := range names {
		blocked[strings.ToUpper(name)] = true
	}
	out := make([]string, 0, len(env))
	for _, item := range env {
		key, _, ok := strings.Cut(item, "=")
		if ok && blocked[strings.ToUpper(key)] {
			continue
		}
		out = append(out, item)
	}
	return out
}
