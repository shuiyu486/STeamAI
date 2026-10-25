package evaluation

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type liveModelSettings struct {
	Model string            `json:"model"`
	Env   map[string]string `json:"env"`
}

// 仅为维护验收解析模型字段，不复制认证、hook 或其它用户配置到 fixture。
func liveConfiguredModel(t *testing.T) string {
	t.Helper()
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		configDir = filepath.Join(home, ".claude")
	}
	project, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var layers []liveModelSettings
	for _, path := range []string{filepath.Join(configDir, "settings.json"), filepath.Join(project, ".claude", "settings.json"), filepath.Join(project, ".claude", "settings.local.json")} {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatalf("读取模型配置失败: %v", err)
		}
		var layer liveModelSettings
		if err := json.Unmarshal(data, &layer); err != nil {
			t.Fatalf("模型配置 JSON 无效: %s", path)
		}
		layers = append(layers, layer)
	}
	env := map[string]string{}
	for _, key := range []string{"STEAMAI_LIVE_MODEL", "ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
		if value, ok := os.LookupEnv(key); ok {
			env[key] = value
		}
	}
	model, err := resolveLiveModel(env, layers)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("冻结配置选择的 evaluation model=%s", model)
	return model
}

func resolveLiveModel(env map[string]string, layers []liveModelSettings) (string, error) {
	merged := map[string]string{}
	for _, layer := range layers {
		for key, value := range layer.Env {
			if key == "ANTHROPIC_MODEL" || strings.HasPrefix(key, "ANTHROPIC_DEFAULT_") && strings.HasSuffix(key, "_MODEL") {
				merged[key] = value
			}
		}
	}
	maps.Copy(merged, env)
	// 高档模型仅通过调用者显式选择；不继承主会话的 ANTHROPIC_MODEL 或 settings.model。
	// 默认档位为 Sonnet（用户 2026-10-25 规则）；未配置 Sonnet 槽位才降级 Haiku。
	model := merged["STEAMAI_LIVE_MODEL"]
	explicit := model != ""
	if !explicit {
		model = merged["ANTHROPIC_DEFAULT_SONNET_MODEL"]
		if model == "" {
			model = merged["ANTHROPIC_DEFAULT_HAIKU_MODEL"]
		}
		if model == "" {
			return "", fmt.Errorf("live test 需要配置 ANTHROPIC_DEFAULT_SONNET_MODEL 或 ANTHROPIC_DEFAULT_HAIKU_MODEL；不回退到主模型或 Opus，显式选择请设置 STEAMAI_LIVE_MODEL")
		}
	}
	switch model {
	case "opus", "sonnet", "haiku":
		if !explicit {
			return "", fmt.Errorf("live test 低成本默认需要配置固定模型，不能冻结别名 %q", model)
		}
		model = merged["ANTHROPIC_DEFAULT_"+strings.ToUpper(model)+"_MODEL"]
	}
	switch strings.ToLower(strings.TrimSuffix(model, "[1m]")) {
	case "default", "auto", "opusplan", "opus", "sonnet", "haiku", "fable":
		return "", fmt.Errorf("live test 需要配置中可解析的固定模型，不能冻结动态别名 %q", model)
	}
	if !modelPattern.MatchString(model) {
		return "", fmt.Errorf("live test 无法从配置解析固定模型；不回退到硬编码模型")
	}
	return model, nil
}

func TestLiveModelConfigurationIsProviderNeutral(t *testing.T) {
	for _, test := range []struct {
		name   string
		env    map[string]string
		layers []liveModelSettings
		want   string
	}{
		{"sonnet-before-haiku", map[string]string{"ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/haiku-fixture", "ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/sonnet-fixture"}, nil, "provider/sonnet-fixture"},
		{"haiku-fallback", map[string]string{"ANTHROPIC_DEFAULT_HAIKU_MODEL": "gpt-fixture[1m]"}, nil, "gpt-fixture[1m]"},
		{"empty-sonnet", map[string]string{"ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/haiku-fixture", "ANTHROPIC_DEFAULT_SONNET_MODEL": ""}, nil, "provider/haiku-fixture"},
		{"main-opus-ignored", map[string]string{"ANTHROPIC_MODEL": "opus", "ANTHROPIC_DEFAULT_OPUS_MODEL": "provider/expensive", "ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/sonnet-fixture"}, nil, "provider/sonnet-fixture"},
		{"fixed-main-model-ignored", map[string]string{"ANTHROPIC_MODEL": "provider/expensive", "ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/sonnet-fixture"}, nil, "provider/sonnet-fixture"},
		{"settings-main-ignored", nil, []liveModelSettings{{Model: "opus", Env: map[string]string{"ANTHROPIC_MODEL": "provider/expensive", "ANTHROPIC_DEFAULT_SONNET_MODEL": "gpt-fixture[1m]"}}}, "gpt-fixture[1m]"},
		{"local-layer", nil, []liveModelSettings{{Env: map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/user"}}, {Env: map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/project"}}, {Env: map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/local"}}}, "provider/local"},
		{"environment-over-settings", map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/environment"}, []liveModelSettings{{Env: map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/settings"}}}, "provider/environment"},
		{"sonnet-tier-before-source", map[string]string{"ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/environment-haiku"}, []liveModelSettings{{Env: map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/settings-sonnet"}}}, "provider/settings-sonnet"},
		{"default-cannot-select-opus-alias", map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "opus", "ANTHROPIC_DEFAULT_OPUS_MODEL": "provider/expensive"}, nil, ""},
		{"default-cannot-select-auto", map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "auto"}, nil, ""},
		{"default-cannot-select-suffixed-alias", map[string]string{"ANTHROPIC_DEFAULT_SONNET_MODEL": "sonnet[1m]"}, nil, ""},
		{"invalid-sonnet-does-not-fallback", map[string]string{"ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/haiku-fixture", "ANTHROPIC_DEFAULT_SONNET_MODEL": "invalid model"}, nil, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveLiveModel(test.env, test.layers)
			if got != test.want || (err != nil) != (test.want == "") {
				t.Fatalf("model=%q want=%q err=%v", got, test.want, err)
			}
		})
	}
}

func TestLiveModelConfigurationRequiresLowCostDefault(t *testing.T) {
	for _, test := range []struct {
		name   string
		env    map[string]string
		layers []liveModelSettings
	}{
		{"missing", nil, nil},
		{"empty-defaults", map[string]string{"ANTHROPIC_DEFAULT_HAIKU_MODEL": "", "ANTHROPIC_DEFAULT_SONNET_MODEL": ""}, nil},
		{"main-opus-only", map[string]string{"ANTHROPIC_MODEL": "opus", "ANTHROPIC_DEFAULT_OPUS_MODEL": "provider/expensive"}, nil},
		{"main-fixed-only", map[string]string{"ANTHROPIC_MODEL": "provider/expensive"}, nil},
		{"settings-main-only", nil, []liveModelSettings{{Model: "provider/expensive", Env: map[string]string{"ANTHROPIC_MODEL": "provider/expensive", "ANTHROPIC_DEFAULT_OPUS_MODEL": "provider/expensive"}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveLiveModel(test.env, test.layers)
			if got != "" || err == nil || !strings.Contains(err.Error(), "ANTHROPIC_DEFAULT_HAIKU_MODEL") || !strings.Contains(err.Error(), "ANTHROPIC_DEFAULT_SONNET_MODEL") {
				t.Fatalf("missing low-cost defaults: model=%q err=%v", got, err)
			}
		})
	}
}

func TestLiveModelConfigurationHonorsExplicitOverride(t *testing.T) {
	for _, test := range []struct {
		name   string
		env    map[string]string
		layers []liveModelSettings
		want   string
	}{
		{"fixed-without-defaults", map[string]string{"STEAMAI_LIVE_MODEL": "gpt-fixture[1m]"}, nil, "gpt-fixture[1m]"},
		{"overrides-default-and-main", map[string]string{"STEAMAI_LIVE_MODEL": "provider/selected", "ANTHROPIC_MODEL": "provider/main", "ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/cheap"}, nil, "provider/selected"},
		{"explicit-opus-alias", map[string]string{"STEAMAI_LIVE_MODEL": "opus"}, []liveModelSettings{{Env: map[string]string{"ANTHROPIC_DEFAULT_OPUS_MODEL": "provider/selected", "ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/cheap"}}}, "provider/selected"},
		{"explicit-sonnet-alias", map[string]string{"STEAMAI_LIVE_MODEL": "sonnet", "ANTHROPIC_DEFAULT_SONNET_MODEL": "provider/selected", "ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/cheap"}, nil, "provider/selected"},
		{"empty-override-uses-default", map[string]string{"STEAMAI_LIVE_MODEL": "", "ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/cheap"}, nil, "provider/cheap"},
		{"unresolved-alias", map[string]string{"STEAMAI_LIVE_MODEL": "sonnet", "ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/cheap"}, nil, ""},
		{"nested-alias-rejected", map[string]string{"STEAMAI_LIVE_MODEL": "haiku", "ANTHROPIC_DEFAULT_HAIKU_MODEL": "opus"}, nil, ""},
		{"dynamic-alias", map[string]string{"STEAMAI_LIVE_MODEL": "opusplan", "ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/cheap"}, nil, ""},
		{"invalid-model", map[string]string{"STEAMAI_LIVE_MODEL": "invalid model", "ANTHROPIC_DEFAULT_HAIKU_MODEL": "provider/cheap"}, nil, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveLiveModel(test.env, test.layers)
			if got != test.want || (err != nil) != (test.want == "") {
				t.Fatalf("model=%q want=%q err=%v", got, test.want, err)
			}
		})
	}
}
