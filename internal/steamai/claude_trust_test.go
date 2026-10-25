package steamai

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const trustFixtureConfig = `{
  "numStartups": 123,
  "bigNumber": 9007199254740991,
  "floatNumber": 1.25,
  "unknownVendorField": {"nested": ["a", "b"], "unicode": "中文Ａ陷阱"},
  "projects": {
    "C:/Users/fixture/another-project": {
      "allowedTools": [],
      "hasTrustDialogAccepted": false
    },
    "C:/Users/fixture/researcher": {
      "allowedTools": ["Read", "Edit"],
      "hasTrustDialogAccepted": false,
      "mcpServers": {"local": {"command": "tool", "args": []}}
    }
  },
  "lastOnboardingVersion": "9.9.9"
}`

func TestMergeTrustEntrySetsTargetAndPreservesEverythingElse(t *testing.T) {
	merged, already, err := mergeTrustEntry([]byte(trustFixtureConfig), "C:/Users/fixture/researcher")
	if err != nil {
		t.Fatal(err)
	}
	if already {
		t.Fatal("fixture researcher is not trusted yet")
	}
	// 目标 entry 只翻转 trust 字段，其余字节与结构保留。
	top := decodeJSON(t, merged)
	entry := top["projects"].(map[string]any)["C:/Users/fixture/researcher"].(map[string]any)
	if entry["hasTrustDialogAccepted"] != true {
		t.Fatalf("trust flag = %v want true", entry["hasTrustDialogAccepted"])
	}
	if entry["allowedTools"].([]any)[0] != "Read" || entry["allowedTools"].([]any)[1] != "Edit" {
		t.Fatalf("allowedTools = %#v", entry["allowedTools"])
	}
	if _, ok := entry["mcpServers"].(map[string]any)["local"]; !ok {
		t.Fatal("mcpServers lost")
	}
	// 未触碰的键逐字节保留。
	assertContainsBytes(t, merged, []byte(`"bigNumber": 9007199254740991`))
	assertContainsBytes(t, merged, []byte(`"floatNumber": 1.25`))
	assertContainsBytes(t, merged, []byte(`"unicode": "中文Ａ陷阱"`))
	assertContainsBytes(t, merged, []byte(`"C:/Users/fixture/another-project"`))
	if !json.Valid(merged) {
		t.Fatal("merged config is not valid JSON")
	}
}

func TestMergeTrustEntryAlreadyTrustedIsZeroWrite(t *testing.T) {
	trusted := strings.Replace(trustFixtureConfig,
		`"C:/Users/fixture/researcher": {
      "allowedTools": ["Read", "Edit"],
      "hasTrustDialogAccepted": false,`,
		`"C:/Users/fixture/researcher": {
      "allowedTools": ["Read", "Edit"],
      "hasTrustDialogAccepted": true,`, 1)
	merged, already, err := mergeTrustEntry([]byte(trusted), "C:/Users/fixture/researcher")
	if err != nil {
		t.Fatal(err)
	}
	if !already {
		t.Fatal("already-trusted entry not detected")
	}
	if string(merged) != trusted {
		t.Fatal("already-trusted merge must not rewrite any byte")
	}
}

func TestMergeTrustEntryInsertsMissingEntryAndProjects(t *testing.T) {
	for _, test := range []struct {
		name   string
		config string
	}{
		{name: "missing entry", config: trustFixtureConfig},
		{name: "empty projects", config: `{"numStartups": 1, "projects": {}}`},
		{name: "missing projects", config: "{\n  \"numStartups\": 1\n}"},
		{name: "empty top object", config: "{}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			merged, already, err := mergeTrustEntry([]byte(test.config), "C:/Users/fixture/newcomer")
			if err != nil {
				t.Fatal(err)
			}
			if already {
				t.Fatal("newcomer reported as already trusted")
			}
			top := decodeJSON(t, merged)
			entry, ok := top["projects"].(map[string]any)["C:/Users/fixture/newcomer"].(map[string]any)
			if !ok || entry["hasTrustDialogAccepted"] != true {
				t.Fatalf("newcomer entry missing: %#v", top["projects"])
			}
		})
	}
}

func TestMergeTrustEntryRejections(t *testing.T) {
	for _, test := range []struct {
		name   string
		config string
		key    string
	}{
		{name: "duplicate top-level key", config: `{"a": 1, "a": 2, "projects": {}}`, key: "C:/x/y"},
		{name: "duplicate entry key", config: `{"projects": {"C:/x/y": {"b": 1, "b": 2}}}`, key: "C:/x/y"},
		{name: "case alias conflict", config: trustFixtureConfig, key: "c:/users/fixture/researcher"},
		{name: "non-bool trust flag", config: `{"projects": {"C:/x/y": {"hasTrustDialogAccepted": "yes"}}}`, key: "C:/x/y"},
		{name: "malformed json", config: `{"projects": {`, key: "C:/x/y"},
		{name: "top level not object", config: `[]`, key: "C:/x/y"},
		{name: "projects not object", config: `{"projects": 5}`, key: "C:/x/y"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := mergeTrustEntry([]byte(test.config), test.key); err == nil {
				t.Fatal("merge accepted invalid input")
			}
		})
	}
}

func TestValidateTrustScopeAnchorsGitTrustToCaseBoundary(t *testing.T) {
	root := t.TempDir()
	caseRoot := filepath.Join(root, "case")
	plain := filepath.Join(caseRoot, ".steamai-vnext", "members", "researcher")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateTrustScope(plain, caseRoot); err != nil {
		t.Fatalf("plain member dir rejected: %v", err)
	}
	// case 根即仓库根：原生信任锚定 case 根，范围恰好等于本次 case 授权边界。
	if err := os.Mkdir(filepath.Join(caseRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateTrustScope(plain, caseRoot); err != nil {
		t.Fatalf("case-root repo rejected: %v", err)
	}
	// case 位于仓库子目录：锚=外层仓库根，超出 case 授权，拒绝并给出指引。
	nested := filepath.Join(root, "repo", "case")
	memberInRepo := filepath.Join(nested, ".steamai-vnext", "members", "researcher")
	if err := os.MkdirAll(memberInRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "repo", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := validateTrustScope(memberInRepo, nested)
	if err == nil || !strings.Contains(err.Error(), "超出本次 case 授权") || !strings.Contains(err.Error(), "信任框") {
		t.Fatalf("case inside parent repo accepted or lacks guidance: %v", err)
	}
	// 成员目录与 case 根之间的嵌套仓库：锚超出 exact 成员，拒绝。
	mid := filepath.Join(root, "repo2", "case")
	memberInMid := filepath.Join(mid, ".steamai-vnext", "members", "x")
	if err := os.MkdirAll(memberInMid, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(mid, ".steamai-vnext", "members", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateTrustScope(memberInMid, mid); err == nil || !strings.Contains(err.Error(), "嵌套") {
		t.Fatalf("nested repo between member and case accepted: %v", err)
	}
	if err := validateTrustScope(filepath.Join(root, "missing"), caseRoot); err == nil {
		t.Fatal("missing member dir accepted")
	}
	if err := validateTrustScope("relative/path", caseRoot); err == nil {
		t.Fatal("relative member dir accepted")
	}
	if err := validateTrustScope(plain, "relative/case"); err == nil {
		t.Fatal("relative case root accepted")
	}
}

func TestWriteClaudeConfigPersistsBackupAndCleansUp(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	memberRoot := filepath.Join(configDir, "members", "researcher")
	if err := os.MkdirAll(memberRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, ".claude.json")
	if err := os.WriteFile(configPath, []byte(trustFixtureConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	already, err := prepareMemberWorkspaceTrust(filepath.Join(configDir, "members", "researcher"), configDir)
	if err != nil {
		t.Fatal(err)
	}
	if already {
		t.Fatal("first write reported already trusted")
	}
	// 第二次调用：已信任零写。
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	already, err = prepareMemberWorkspaceTrust(filepath.Join(configDir, "members", "researcher"), configDir)
	if err != nil || !already {
		t.Fatalf("second call = (%v, %v) want (true, nil)", already, err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("already-trusted path rewrote the config")
	}
	// 备份保留的是原件。
	backups, err := filepath.Glob(filepath.Join(configDir, "backups", ".claude.json.backup.*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("backups = %d want 1", len(backups))
	}
	backup, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != trustFixtureConfig {
		t.Fatal("backup does not preserve the original bytes")
	}
	// 锁与临时文件都清理干净。
	if _, err := os.Stat(configPath + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock dir left behind: %v", err)
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp.") {
			t.Fatalf("tmp file left behind: %s", entry.Name())
		}
	}
	// 写入结果可被原生 CLI 语义接受：JSON 有效且目标键为 true。
	top := decodeJSON(t, after)
	entry := top["projects"].(map[string]any)[filepath.ToSlash(filepath.Join(configDir, "members", "researcher"))]
	if entry.(map[string]any)["hasTrustDialogAccepted"] != true {
		t.Fatalf("trust record = %#v", entry)
	}
}

func TestWriteClaudeConfigFailsClosedWhileLockHeld(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	configPath := filepath.Join(configDir, ".claude.json")
	if err := os.WriteFile(configPath, []byte(trustFixtureConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	// 模拟另一个 Claude Code 进程持有的锁目录。
	if err := os.Mkdir(configPath+".lock", 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(configPath + ".lock")
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	// 缩短重试窗口，避免测试等待完整超时。
	savedTimeout := claudeConfigLockTimeout
	claudeConfigLockTimeout = 300 * time.Millisecond
	defer func() { claudeConfigLockTimeout = savedTimeout }()
	if _, err := writeClaudeConfig(configPath, func(original []byte) ([]byte, bool, error) {
		merged, already, err := mergeTrustEntry(original, "C:/Users/fixture/researcher")
		return merged, already, err
	}); err == nil || !errors.Is(err, errConfigLockBusy) {
		t.Fatalf("expected errConfigLockBusy, got %v", err)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("busy lock path must not touch the config")
	}
}

func TestWriteClaudeConfigConcurrentWritersAllPersist(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	configPath := filepath.Join(configDir, ".claude.json")
	if err := os.WriteFile(configPath, []byte(`{"projects": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	const writers = 8
	var wait sync.WaitGroup
	failures := make([]error, writers)
	for i := 0; i < writers; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			key := "C:/Users/fixture/member-" + string(rune('a'+index))
			_, err := writeClaudeConfig(configPath, func(original []byte) ([]byte, bool, error) {
				merged, already, err := mergeTrustEntry(original, key)
				return merged, already, err
			})
			failures[index] = err
		}(i)
	}
	wait.Wait()
	for index, err := range failures {
		if err != nil {
			t.Fatalf("writer %d failed: %v", index, err)
		}
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	top := decodeJSON(t, data)
	projects := top["projects"].(map[string]any)
	if len(projects) != writers {
		t.Fatalf("persisted entries = %d want %d", len(projects), writers)
	}
	if !json.Valid(data) {
		t.Fatal("concurrent writes corrupted the config")
	}
}

func decodeJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var top map[string]any
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("decode json: %v\n%s", err, data)
	}
	return top
}

func assertContainsBytes(t *testing.T, haystack []byte, needle []byte) {
	t.Helper()
	if !strings.Contains(string(haystack), string(needle)) {
		t.Fatalf("merged output lost %q", needle)
	}
}

// prepareTrustedMemberCase 物化一个带正式成员的 current case fixture，返回
// case 根与成员目录。
func prepareTrustedMemberCase(t *testing.T, member string) (string, string) {
	t.Helper()
	caseRoot := t.TempDir()
	materializeCurrentCaseFixture(t, caseRoot)
	memberRoot := filepath.Join(caseRoot, ".steamai-vnext", "members", member)
	if err := os.MkdirAll(memberRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memberRoot, "CLAUDE.md"), []byte("# Member\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	markerPath := filepath.Join(caseRoot, ".steamai-vnext", "CLAUDE.md")
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatal(err)
	}
	marker = []byte(strings.Replace(string(marker),
		"| none | execution | inactive | none |",
		"| "+member+" | execution | active | `.steamai-vnext/members/"+member+"/CLAUDE.md` |", 1))
	if err := os.WriteFile(markerPath, marker, 0o644); err != nil {
		t.Fatal(err)
	}
	return caseRoot, memberRoot
}

func TestOpenMemberTrustWorkspaceWritesRecordAndLaunches(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("ANTHROPIC_MODEL", "configured-model")
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	configPath := filepath.Join(configDir, ".claude.json")
	if err := os.WriteFile(configPath, []byte(`{"projects": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	caseRoot, memberRoot := prepareTrustedMemberCase(t, "static-analysis")
	p := &fakePlatform{supported: true}
	a := testApp(t, p, caseRoot, makeFakeExecutable(t, "claude.exe"))
	if err := a.run([]string{"__open-member", "--trust-workspace", "static-analysis"}); err != nil {
		t.Fatal(err)
	}
	if len(p.visible) != 1 || p.visible[0].Dir != memberRoot {
		t.Fatalf("visible launches = %#v", p.visible)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	projects := decodeJSON(t, data)["projects"].(map[string]any)
	entry, ok := projects[filepath.ToSlash(memberRoot)].(map[string]any)
	if !ok || entry["hasTrustDialogAccepted"] != true {
		t.Fatalf("trust record = %#v", projects)
	}
}

func TestOpenMemberTrustWorkspaceStopsLaunchOnConfigFailure(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("ANTHROPIC_MODEL", "configured-model")
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	if err := os.WriteFile(filepath.Join(configDir, ".claude.json"), []byte(`{"projects": {`), 0o644); err != nil {
		t.Fatal(err)
	}
	caseRoot, _ := prepareTrustedMemberCase(t, "static-analysis")
	p := &fakePlatform{supported: true}
	a := testApp(t, p, caseRoot, makeFakeExecutable(t, "claude.exe"))
	err := a.run([]string{"__open-member", "--trust-workspace", "static-analysis"})
	if err == nil || !strings.Contains(err.Error(), "自动信任") {
		t.Fatalf("expected trust failure, got %v", err)
	}
	if len(p.visible) != 0 {
		t.Fatal("member window must not open when trust preparation fails")
	}
}

func TestOpenMemberWithoutTrustOptionKeepsOriginalBehavior(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CLAUDE_CODE_CHILD_SESSION", "1")
	t.Setenv("ANTHROPIC_MODEL", "configured-model")
	configDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", configDir)
	configPath := filepath.Join(configDir, ".claude.json")
	if err := os.WriteFile(configPath, []byte(`{"projects": {}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	caseRoot, _ := prepareTrustedMemberCase(t, "static-analysis")
	p := &fakePlatform{supported: true}
	a := testApp(t, p, caseRoot, makeFakeExecutable(t, "claude.exe"))
	if err := a.run([]string{"__open-member", "static-analysis"}); err != nil {
		t.Fatal(err)
	}
	if len(p.visible) != 1 {
		t.Fatalf("visible launches = %d", len(p.visible))
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "hasTrustDialogAccepted") {
		t.Fatal("legacy open-member must not write trust records")
	}
}
