package steamai

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newModelsTestApp(stdout, stderr io.Writer) *app {
	return newApp(&fakePlatform{supported: true}, strings.NewReader(""), stdout, stderr, "test")
}

func modelsTestPath(t *testing.T) string {
	t.Helper()
	path, err := modelsConfigPath()
	if err != nil {
		t.Fatalf("modelsConfigPath: %v", err)
	}
	return path
}

func TestModelTiersValidAlias(t *testing.T) {
	for alias, want := range map[string]bool{
		"opus": true, "sonnet": true, "haiku": true,
		"": false, "Opus": false, "SONNET": false, "turbo": false,
		"claude-sonnet-5": false, "high": false, "standard": false,
		"--model": false, "opus ": false, " opus": false,
	} {
		if got := validModelAlias(alias); got != want {
			t.Errorf("validModelAlias(%q) = %v, want %v", alias, got, want)
		}
	}
}

// 缺文件时只读查询返回默认 opus/sonnet，且不创建任何文件或目录。
func TestModelTiersDefaultsAndZeroWrite(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	var stdout strings.Builder
	a := newModelsTestApp(&stdout, io.Discard)

	tiers, err := a.configuredModelTiers()
	if err != nil {
		t.Fatalf("configuredModelTiers: %v", err)
	}
	if tiers != (modelTiers{High: "opus", Standard: "sonnet"}) {
		t.Fatalf("tiers = %+v, want 默认 opus/sonnet", tiers)
	}
	if err := a.modelsCommand(nil); err != nil {
		t.Fatalf("modelsCommand: %v", err)
	}
	want := fmt.Sprintf("high=opus\nstandard=sonnet\nconfig=%s\n", modelsTestPath(t))
	if stdout.String() != want {
		t.Fatalf("output = %q, want %q", stdout.String(), want)
	}
	if _, err := os.Lstat(filepath.Join(os.Getenv("LOCALAPPDATA"), "STeamAI")); !os.IsNotExist(err) {
		t.Fatalf("只读查询不应创建 STeamAI 目录: %v", err)
	}
}

// 设置与查询闭环：参数顺序不限，文件是仅含两个字段的 UTF-8 JSON，目录中不残留临时文件。
func TestModelTiersSetAndQuery(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	var stdout strings.Builder
	a := newModelsTestApp(&stdout, io.Discard)

	if err := a.modelsCommand([]string{"--standard", "haiku", "--high", "sonnet"}); err != nil {
		t.Fatalf("modelsCommand set: %v", err)
	}
	path := modelsTestPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 models.json: %v", err)
	}
	if string(data) != "{\"high\":\"sonnet\",\"standard\":\"haiku\"}\n" {
		t.Fatalf("models.json = %q, want 仅两个字段的规范 JSON", string(data))
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "models.json" {
		t.Fatalf("配置目录应只含 models.json，实际 %v", entries)
	}
	tiers, err := a.configuredModelTiers()
	if err != nil {
		t.Fatalf("configuredModelTiers: %v", err)
	}
	if tiers != (modelTiers{High: "sonnet", Standard: "haiku"}) {
		t.Fatalf("tiers = %+v, want sonnet/haiku", tiers)
	}
	stdout.Reset()
	if err := a.modelsCommand(nil); err != nil {
		t.Fatalf("modelsCommand read: %v", err)
	}
	want := fmt.Sprintf("high=sonnet\nstandard=haiku\nconfig=%s\n", path)
	if stdout.String() != want {
		t.Fatalf("output = %q, want %q", stdout.String(), want)
	}
}

// 两档允许相同别名。
func TestModelTiersSameTierAllowed(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	a := newModelsTestApp(io.Discard, io.Discard)
	if err := a.modelsCommand([]string{"--high", "haiku", "--standard", "haiku"}); err != nil {
		t.Fatalf("同档设置: %v", err)
	}
	tiers, err := a.configuredModelTiers()
	if err != nil {
		t.Fatal(err)
	}
	if tiers != (modelTiers{High: "haiku", Standard: "haiku"}) {
		t.Fatalf("tiers = %+v, want haiku/haiku", tiers)
	}
}

// 生效值与请求一致时不改动文件：既不重写已存在的文件，也不为默认值落新文件。
func TestModelTiersSameValueNoRewrite(t *testing.T) {
	t.Run("fresh default", func(t *testing.T) {
		t.Setenv("LOCALAPPDATA", t.TempDir())
		a := newModelsTestApp(io.Discard, io.Discard)
		if err := a.modelsCommand([]string{"--high", "opus", "--standard", "sonnet"}); err != nil {
			t.Fatalf("设置默认值: %v", err)
		}
		if _, err := os.Lstat(modelsTestPath(t)); !os.IsNotExist(err) {
			t.Fatalf("与默认相同的设置不应创建文件: %v", err)
		}
	})
	t.Run("existing file untouched", func(t *testing.T) {
		local := t.TempDir()
		t.Setenv("LOCALAPPDATA", local)
		a := newModelsTestApp(io.Discard, io.Discard)
		if err := a.modelsCommand([]string{"--high", "sonnet", "--standard", "haiku"}); err != nil {
			t.Fatal(err)
		}
		path := modelsTestPath(t)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// 只读文件上被替换的 rename 会失败；成功完成即证明没有尝试写入。
		if err := os.Chmod(path, 0o444); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o666) })
		if err := a.modelsCommand([]string{"--standard", "haiku", "--high", "sonnet"}); err != nil {
			t.Fatalf("相同值重复设置: %v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatalf("文件被改动: %q -> %q", string(before), string(after))
		}
	})
}

// 重复、缺值、未知参数、多余位置参数与非法别名一律拒绝，且不产生任何文件系统副作用。
func TestModelTiersRejectInvalidArgs(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"only high", []string{"--high", "opus"}, "必须同时提供"},
		{"only standard", []string{"--standard", "sonnet"}, "必须同时提供"},
		{"duplicate high", []string{"--high", "opus", "--high", "sonnet", "--standard", "haiku"}, "只能出现一次"},
		{"duplicate standard", []string{"--high", "opus", "--standard", "sonnet", "--standard", "haiku"}, "只能出现一次"},
		{"missing value", []string{"--high", "opus", "--standard"}, "需要模型别名"},
		{"lone flag", []string{"--high"}, "需要模型别名"},
		{"unknown flag", []string{"--verbose", "--high", "opus", "--standard", "sonnet"}, "未知 models 参数"},
		{"extra positional", []string{"--high", "opus", "extra", "--standard", "sonnet"}, "未知 models 参数"},
		{"invalid alias high", []string{"--high", "turbo", "--standard", "sonnet"}, "opus、sonnet、haiku"},
		{"invalid alias standard", []string{"--high", "opus", "--standard", "claude-sonnet-5"}, "opus、sonnet、haiku"},
		{"case sensitive", []string{"--high", "Opus", "--standard", "sonnet"}, "opus、sonnet、haiku"},
		{"empty alias", []string{"--high", "", "--standard", "sonnet"}, "opus、sonnet、haiku"},
		{"flag as value", []string{"--high", "--standard", "opus"}, "opus、sonnet、haiku"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			a := newModelsTestApp(io.Discard, io.Discard)
			err := a.modelsCommand(test.args)
			if err == nil {
				t.Fatalf("期望参数 %v 被拒绝", test.args)
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("错误 %q 应包含 %q", err.Error(), test.want)
			}
			if _, statErr := os.Lstat(filepath.Join(local, "STeamAI")); !os.IsNotExist(statErr) {
				t.Fatalf("被拒绝的输入不应产生文件系统副作用: %v", statErr)
			}
		})
	}
}

// 畸形配置：解析失败、未知字段、缺字段、空值、非法别名与尾部多余内容都报错，不回退默认。
func TestModelTiersMalformedConfig(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"truncated json", `{"high":"opus"`},
		{"not json", `not json`},
		{"unknown field", `{"high":"opus","standard":"sonnet","extra":1}`},
		{"duplicate tier", `{"high":"sonnet","high":"opus","standard":"haiku"}`},
		{"misspelled tier", `{"HIGH":"opus","standard":"sonnet"}`},
		{"trailing object", `{"high":"sonnet","standard":"haiku"}{}`},
		{"missing standard", `{"high":"opus"}`},
		{"missing high", `{"standard":"sonnet"}`},
		{"empty alias", `{"high":"","standard":"sonnet"}`},
		{"invalid alias", `{"high":"opus","standard":"gpt-4"}`},
		{"trailing content", `{"high":"opus","standard":"sonnet"}extra`},
		{"not an object", `["opus","sonnet"]`},
		{"number value", `{"high":1,"standard":"sonnet"}`},
		{"utf8 bom", "\xEF\xBB\xBF" + `{"high":"opus","standard":"sonnet"}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			local := t.TempDir()
			t.Setenv("LOCALAPPDATA", local)
			dir := filepath.Join(local, "STeamAI")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "models.json")
			if err := os.WriteFile(path, []byte(test.content), 0o644); err != nil {
				t.Fatal(err)
			}
			a := newModelsTestApp(io.Discard, io.Discard)
			if _, err := a.configuredModelTiers(); err == nil {
				t.Fatalf("畸形配置 %q 不应回退默认", test.content)
			}
			if err := a.modelsCommand(nil); err == nil {
				t.Fatalf("畸形配置下的只读查询应报错: %q", test.content)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != test.content {
				t.Fatalf("畸形文件被改动: %q", string(data))
			}
		})
	}
}

// 已有畸形配置时，合法的设置请求必须失败且不静默覆盖。
func TestModelTiersSetDoesNotOverwriteMalformed(t *testing.T) {
	local := t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	dir := filepath.Join(local, "STeamAI")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "models.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := newModelsTestApp(io.Discard, io.Discard)
	if err := a.modelsCommand([]string{"--high", "opus", "--standard", "haiku"}); err == nil {
		t.Fatal("畸形配置上的设置应报错而非覆盖")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{broken" {
		t.Fatalf("畸形文件被覆盖: %q", string(data))
	}
}

// LOCALAPPDATA 缺失或非绝对路径时读与设置都必须报错。
func TestModelTiersEnvBoundaries(t *testing.T) {
	for _, env := range []string{"", `relative\path`, `sub\dir`} {
		t.Run("env="+env, func(t *testing.T) {
			t.Setenv("LOCALAPPDATA", env)
			a := newModelsTestApp(io.Discard, io.Discard)
			if _, err := a.configuredModelTiers(); err == nil {
				t.Fatalf("LOCALAPPDATA=%q 不应成功", env)
			}
			if err := a.modelsCommand(nil); err == nil {
				t.Fatalf("LOCALAPPDATA=%q 只读查询不应成功", env)
			}
			if err := a.modelsCommand([]string{"--high", "opus", "--standard", "haiku"}); err == nil {
				t.Fatalf("LOCALAPPDATA=%q 设置不应成功", env)
			}
		})
	}
}

// models.json 本身或其父目录是 symlink/reparse 时必须拒绝读与写。
func TestModelTiersRejectsSymlinks(t *testing.T) {
	t.Run("config file symlink", func(t *testing.T) {
		local := t.TempDir()
		t.Setenv("LOCALAPPDATA", local)
		dir := filepath.Join(local, "STeamAI")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		realFile := filepath.Join(t.TempDir(), "real-models.json")
		if err := os.WriteFile(realFile, []byte("{\"high\":\"opus\",\"standard\":\"sonnet\"}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(realFile, filepath.Join(dir, "models.json")); err != nil {
			t.Skipf("当前环境无法创建 symlink: %v", err)
		}
		a := newModelsTestApp(io.Discard, io.Discard)
		if _, err := a.configuredModelTiers(); err == nil {
			t.Fatal("symlink 配置文件不应可读")
		}
		if err := a.modelsCommand([]string{"--high", "opus", "--standard", "haiku"}); err == nil {
			t.Fatal("symlink 配置文件不应可写")
		}
		data, err := os.ReadFile(realFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "{\"high\":\"opus\",\"standard\":\"sonnet\"}\n" {
			t.Fatalf("symlink 目标被改动: %q", string(data))
		}
	})
	t.Run("config dir symlink", func(t *testing.T) {
		local := t.TempDir()
		t.Setenv("LOCALAPPDATA", local)
		target := t.TempDir()
		if err := os.MkdirAll(filepath.Join(target, "STeamAI"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "STeamAI", "models.json"), []byte("{\"high\":\"opus\",\"standard\":\"sonnet\"}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(target, "STeamAI"), filepath.Join(local, "STeamAI")); err != nil {
			t.Skipf("当前环境无法创建 symlink: %v", err)
		}
		a := newModelsTestApp(io.Discard, io.Discard)
		if _, err := a.configuredModelTiers(); err == nil {
			t.Fatal("symlink 配置目录不应可读")
		}
		if err := a.modelsCommand([]string{"--high", "opus", "--standard", "haiku"}); err == nil {
			t.Fatal("symlink 配置目录不应可写")
		}
	})
}
