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
	for _, key := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
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
	settingsModel := ""
	merged := map[string]string{}
	for _, layer := range layers {
		if layer.Model != "" {
			settingsModel = layer.Model
		}
		for key, value := range layer.Env {
			if key == "ANTHROPIC_MODEL" || strings.HasPrefix(key, "ANTHROPIC_DEFAULT_") && strings.HasSuffix(key, "_MODEL") {
				merged[key] = value
			}
		}
	}
	maps.Copy(merged, env)
	model := merged["ANTHROPIC_MODEL"]
	if model == "" {
		model = settingsModel
	}
	switch model {
	case "opus", "sonnet", "haiku":
		model = merged["ANTHROPIC_DEFAULT_"+strings.ToUpper(model)+"_MODEL"]
	case "default", "opusplan", "sonnet[1m]", "opus[1m]":
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
		{"environment", map[string]string{"ANTHROPIC_MODEL": "gpt-fixture[1m]"}, []liveModelSettings{{Model: "claude-fixture"}}, "gpt-fixture[1m]"},
		{"settings", nil, []liveModelSettings{{Model: "gpt-fixture[1m]"}}, "gpt-fixture[1m]"},
		{"alias", nil, []liveModelSettings{{Model: "opus", Env: map[string]string{"ANTHROPIC_DEFAULT_OPUS_MODEL": "gpt-fixture[1m]"}}}, "gpt-fixture[1m]"},
		{"settings-env", nil, []liveModelSettings{{Model: "opus", Env: map[string]string{"ANTHROPIC_MODEL": "gpt-fixture[1m]"}}}, "gpt-fixture[1m]"},
		{"local-layer", nil, []liveModelSettings{{Model: "provider/first"}, {Model: "provider/second"}}, "provider/second"},
		{"missing", nil, nil, ""},
		{"unresolved-alias", nil, []liveModelSettings{{Model: "sonnet"}}, ""},
		{"dynamic-alias", nil, []liveModelSettings{{Model: "opusplan"}}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveLiveModel(test.env, test.layers)
			if got != test.want || (err != nil) != (test.want == "") {
				t.Fatalf("model=%q err=%v", got, err)
			}
		})
	}
}
