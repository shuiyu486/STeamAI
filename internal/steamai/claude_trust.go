package steamai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 成员工作区自动信任：复用 Claude Code 原生 `projects["<path>"].hasTrustDialogAccepted`
// 记录，只写已授权 exact 成员目录对应的键，其余 JSON 内容逐字节保留。
// 写入遵守实测的原生并发协议：`.claude.json.lock` 目录锁（CLI 对外来持有者等待
// 10 秒后才接管），持锁窗口内完成 读取→合并→备份→tmp→原子替换，全程毫秒级。

const trustFieldName = "hasTrustDialogAccepted"

// claudeConfigLockTimeout 是获取配置锁的重试窗口，刻意远小于原生 CLI 的
// 10 秒 stale 接管，保证我们永远不触发接管竞争。
var claudeConfigLockTimeout = 5 * time.Second

var errConfigLockBusy = errors.New("claude 配置锁被其它 Claude Code 进程持续持有")

// claudeConfigPath 解析当前配置域的 .claude.json：与成员进程一致，优先
// CLAUDE_CONFIG_DIR，否则用户主目录。
func claudeConfigPath() (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(filepath.Clean(dir), ".claude.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("解析用户主目录: %w", err)
	}
	return filepath.Join(home, ".claude.json"), nil
}

// decodeStrictObject 把 JSON 对象解析为 RawMessage 映射并拒绝重复键：
// 静默去重会掩盖用户配置里的真实冲突，宁可停止也不猜哪个键生效。
func decodeStrictObject(data []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	object := make(map[string]json.RawMessage)
	if err := expectDelimiter(decoder, '{'); err != nil {
		return nil, err
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("解析 JSON 对象键: %w", err)
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("JSON 结构不是对象")
		}
		if _, exists := object[key]; exists {
			return nil, fmt.Errorf("JSON 对象存在重复键 %q", key)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("解析键 %q 的值: %w", key, err)
		}
		object[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("解析 JSON 对象结束: %w", err)
	}
	return object, nil
}

func expectDelimiter(decoder *json.Decoder, expected rune) error {
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("读取 JSON 起始符: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != json.Delim(expected) {
		return errors.New("JSON 内容不是对象")
	}
	return nil
}

// locateObjectValue 返回 key 的 value 在 data 中的字节范围。decoder.Token 读到
// key 后 InputOffset 停在 key 之后；Decode 到 RawMessage 的长度就是 value 的
// 原始字节长度，由结束位置回退得到起始位置。
func locateObjectValue(data []byte, key string) (int, int, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := expectDelimiter(decoder, '{'); err != nil {
		return 0, 0, err
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return 0, 0, err
		}
		name, ok := token.(string)
		if !ok {
			return 0, 0, errors.New("JSON 结构不是对象")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return 0, 0, err
		}
		if name == key {
			end := int(decoder.InputOffset())
			return end - len(value), end, nil
		}
	}
	return 0, 0, fmt.Errorf("键 %q 不存在", key)
}

// renderTrustEntryValue 生成与原生 2 空格缩进对齐的 project entry 对象文本
// （`{` 开头、字段 6 空格、闭括号 4 空格，不含键名）。
func renderTrustEntryValue(entry map[string]json.RawMessage) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("    ", "  ")
	if err := encoder.Encode(entry); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// renderTrustEntryBlock 生成 key 对应、可直接插入 projects 对象的成员文本。
func renderTrustEntryBlock(key string, entry map[string]json.RawMessage) ([]byte, error) {
	value, err := renderTrustEntryValue(entry)
	if err != nil {
		return nil, err
	}
	return append([]byte("    "+strconv.Quote(key)+": "), value...), nil
}

// insertObjectMember 在 JSON 对象文本的闭括号前插入成员文本块，返回插入后的
// 完整对象文本；closeIndent 是该对象闭括号所在行的缩进。
func insertObjectMember(object []byte, member []byte, closeIndent string) ([]byte, error) {
	trimmed := bytes.TrimRight(object, " \t\r\n")
	if len(trimmed) < 2 || trimmed[len(trimmed)-1] != '}' || trimmed[0] != '{' {
		return nil, errors.New("JSON 对象定位失败")
	}
	open := bytes.TrimRight(trimmed[1:len(trimmed)-1], " \t\r\n")
	prefix := object[:len(object)-len(trimmed)]
	result := make([]byte, 0, len(object)+len(member)+len(closeIndent)+4)
	if len(open) == 0 {
		// 空对象：首个成员无需逗号。
		result = append(result, prefix...)
		result = append(result, '{', '\n')
		result = append(result, member...)
		result = append(result, '\n')
		result = append(result, closeIndent...)
		return append(result, '}'), nil
	}
	insertAt := len(prefix) + len(trimmed) - 1
	result = append(result, object[:insertAt]...)
	result = append(result, ',', '\n')
	result = append(result, member...)
	result = append(result, '\n')
	result = append(result, closeIndent...)
	return append(result, '}'), nil
}

// mergeTrustEntry 对原始配置字节做最小文本合并：只重写目标 project entry，
// 其它键、格式与顺序逐字节保留。返回合并结果与是否已处于目标状态。
func mergeTrustEntry(original []byte, projectKey string) (merged []byte, alreadyTrusted bool, err error) {
	top, err := decodeStrictObject(original)
	if err != nil {
		return nil, false, fmt.Errorf("解析配置顶层: %w", err)
	}
	rawProjects, hasProjects := top["projects"]
	entry := make(map[string]json.RawMessage)
	entry[trustFieldName] = json.RawMessage("true")
	if hasProjects {
		projects, err := decodeStrictObject(rawProjects)
		if err != nil {
			return nil, false, fmt.Errorf("解析 projects: %w", err)
		}
		for existing := range projects {
			if existing != projectKey && strings.EqualFold(existing, projectKey) {
				return nil, false, fmt.Errorf("projects 已存在大小写不同但等价的键 %q 与目标 %q，拒绝猜测写入", existing, projectKey)
			}
		}
		rawEntry, exists := projects[projectKey]
		if exists {
			entryObject, err := decodeStrictObject(rawEntry)
			if err != nil {
				return nil, false, fmt.Errorf("解析 project entry: %w", err)
			}
			if raw, ok := entryObject[trustFieldName]; ok {
				switch string(bytes.TrimSpace(raw)) {
				case "true":
					return original, true, nil
				case "false":
				default:
					return nil, false, fmt.Errorf("project entry 的 %s 不是布尔值", trustFieldName)
				}
				delete(entryObject, trustFieldName)
			}
			for field, value := range entryObject {
				entry[field] = value
			}
			block, err := renderTrustEntryValue(entry)
			if err != nil {
				return nil, false, err
			}
			// 范围必须在 original 的字节上定位：Decode 出的 RawMessage
			// 是复制件，其内部偏移与 original 无关。
			projectsStart, projectsEnd, err := locateObjectValue(original, "projects")
			if err != nil {
				return nil, false, fmt.Errorf("定位 projects: %w", err)
			}
			entryStart, entryEnd, err := locateObjectValue(original[projectsStart:projectsEnd], projectKey)
			if err != nil {
				return nil, false, fmt.Errorf("定位 project entry: %w", err)
			}
			entryStart += projectsStart
			entryEnd += projectsStart
			combined := make([]byte, 0, len(original))
			combined = append(combined, original[:entryStart]...)
			combined = append(combined, block...)
			combined = append(combined, original[entryEnd:]...)
			return combined, false, nil
		}
	}
	block, err := renderTrustEntryBlock(projectKey, entry)
	if err != nil {
		return nil, false, err
	}
	if !hasProjects {
		projectsBlock := append([]byte("  \"projects\": {\n"), append(block, []byte("\n  }")...)...)
		updated, err := insertObjectMember(original, projectsBlock, "")
		if err != nil {
			return nil, false, fmt.Errorf("写入 projects 对象: %w", err)
		}
		return updated, false, nil
	}
	start, end, err := locateObjectValue(original, "projects")
	if err != nil {
		return nil, false, fmt.Errorf("定位 projects: %w", err)
	}
	updatedEntrySet, err := insertObjectMember(original[start:end], block, "  ")
	if err != nil {
		return nil, false, fmt.Errorf("写入 project entry: %w", err)
	}
	combined := make([]byte, 0, len(original)+len(updatedEntrySet))
	combined = append(combined, original[:start]...)
	combined = append(combined, updatedEntrySet...)
	combined = append(combined, original[end:]...)
	return combined, false, nil
}

// validateTrustScope 核对信任目标不会扩大到成员目录之外：成员目录必须存在且
// 不是 reparse point；祖先链上出现 .git 时原生信任会锚定整个 repo root，
// 超出本次授权的 exact 成员范围，必须停止。
func validateTrustScope(memberRoot string) error {
	if !filepath.IsAbs(memberRoot) {
		return errors.New("信任目标必须是绝对路径")
	}
	info, err := os.Stat(memberRoot)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("信任目标目录不存在: %w", err)
	}
	if err := rejectReparse(memberRoot); err != nil {
		return fmt.Errorf("信任目标无效: %w", err)
	}
	for dir := memberRoot; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return fmt.Errorf("成员目录位于 git 仓库内（%s），原生信任会扩大到整个仓库根，拒绝自动信任", dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
	}
}

// prepareMemberWorkspaceTrust 在当前配置域为目标成员目录写入原生信任记录。
// 已信任时零写入返回 true；任何解析、冲突或并发障碍都以具体原因失败，
// 不重置配置、不静默重试。
func prepareMemberWorkspaceTrust(memberRoot string) (bool, error) {
	if err := validateTrustScope(memberRoot); err != nil {
		return false, err
	}
	configPath, err := claudeConfigPath()
	if err != nil {
		return false, err
	}
	projectKey := filepath.ToSlash(filepath.Clean(memberRoot))
	return writeClaudeConfig(configPath, func(original []byte) ([]byte, bool, error) {
		merged, already, err := mergeTrustEntry(original, projectKey)
		return merged, already, err
	})
}

// writeClaudeConfig 在原生锁协议保护下执行一次 读取→合并→备份→原子替换。
// 不持锁的读者可能读到 CLI 的中间状态，所有判断都在锁内基于最新字节。
func writeClaudeConfig(configPath string, merge func([]byte) ([]byte, bool, error)) (bool, error) {
	lock, err := acquireClaudeConfigLock(configPath, claudeConfigLockTimeout)
	if err != nil {
		return false, fmt.Errorf("获取配置锁: %w", err)
	}
	defer lock.release()
	original, err := os.ReadFile(configPath)
	if err != nil {
		return false, fmt.Errorf("读取配置: %w", err)
	}
	merged, alreadyTrusted, err := merge(original)
	if err != nil {
		return false, err
	}
	if alreadyTrusted || bytes.Equal(original, merged) {
		return alreadyTrusted, nil
	}
	if err := backupClaudeConfig(configPath, original); err != nil {
		return false, fmt.Errorf("备份配置: %w", err)
	}
	if err := replaceClaudeConfigAtomically(configPath, merged); err != nil {
		return false, fmt.Errorf("写入配置: %w", err)
	}
	return false, nil
}

// backupClaudeConfig 按原生轮换约定保存原件副本（保留最新 5 份）。
func backupClaudeConfig(configPath string, original []byte) error {
	backups := filepath.Join(filepath.Dir(configPath), "backups")
	if err := os.MkdirAll(backups, 0o755); err != nil {
		return err
	}
	name := filepath.Join(backups, fmt.Sprintf(".claude.json.backup.%d", time.Now().UnixMilli()))
	if err := os.WriteFile(name, original, 0o644); err != nil {
		return err
	}
	entries, err := os.ReadDir(backups)
	if err != nil {
		return err
	}
	var existing []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".claude.json.backup.") {
			existing = append(existing, entry.Name())
		}
	}
	sort.Strings(existing)
	for len(existing) > 5 {
		if err := os.Remove(filepath.Join(backups, existing[0])); err != nil {
			return err
		}
		existing = existing[1:]
	}
	return nil
}
