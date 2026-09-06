package vnextcontract

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestResearchExecutionAssetsAreDeclaredAndBounded(t *testing.T) {
	repo := repoRoot(t)
	for _, asset := range []struct{ pack, section, path string }{
		{"binary-re", "tooling", "tooling/scripts/export_function_evidence.py"},
		{"binary-re", "tooling", "tooling/schemas/ida-function-evidence-v1.schema.json"},
		{"binary-re", "tooling", "tooling/recipes/ida-function-evidence.md"},
		{"binary-re", "references", "references/binary-re/bounded-behavior-chain.md"},
		{"web-security", "tooling", "tooling/recipes/curl-read-evidence.md"},
		{"web-security", "references", "references/web-security/request-sequence-review.md"},
		{"web-security", "references", "references/web-security/client-api-joint-review.md"},
	} {
		t.Run(asset.pack+"/"+asset.path, func(t *testing.T) {
			manifestRel := "packs/" + asset.pack + "/manifest.yml"
			manifest := readPrototypeFile(t, repo, manifestRel)
			if !slices.Contains(manifestListValues(manifest, asset.section), asset.path) {
				t.Errorf("%s 未在 %s 声明 %s", manifestRel, asset.section, asset.path)
			}
			path := filepath.Join(repo, "packs", asset.pack, filepath.FromSlash(asset.path))
			assertRegularNonSymlink(t, path, "专业实操资产")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(strings.TrimSpace(string(data))) == 0 {
				t.Fatal("专业实操资产为空")
			}
			if filepath.Ext(asset.path) == ".md" && len(data) > 16*1024 {
				t.Fatal("方法入口超过既有 16 KiB 阅读预算")
			}
		})
	}

	schema := readPrototypeFile(t, repo, "packs/binary-re/tooling/schemas/ida-function-evidence-v1.schema.json")
	var document struct {
		Schema string                     `json:"$schema"`
		Type   string                     `json:"type"`
		Fields map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal([]byte(schema), &document); err != nil {
		t.Fatalf("IDA 输出 schema 不是有效 JSON: %v", err)
	}
	if document.Schema == "" || document.Type != "object" || len(document.Fields) == 0 {
		t.Fatal("IDA 输出 schema 必须描述一个有明确字段的 object")
	}
}

func TestResearchExecutionKeepsOneExporterAndMarkdownLearning(t *testing.T) {
	repo := repoRoot(t)
	var exporters []string
	err := filepath.WalkDir(filepath.Join(repo, "packs"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "export_function_evidence.py" {
			rel, err := filepath.Rel(repo, path)
			if err != nil {
				return err
			}
			exporters = append(exporters, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(exporters, []string{"packs/binary-re/tooling/scripts/export_function_evidence.py"}) {
		t.Fatalf("定点导出器必须只有一份生产实现，实际为 %v", exporters)
	}
	for _, pack := range []string{"binary-re", "web-security"} {
		manifest := readPrototypeFile(t, repo, "packs/"+pack+"/manifest.yml")
		targets := manifestListValues(manifest, "learningTargets")
		if len(targets) == 0 {
			t.Fatalf("%s 缺少 learningTargets", pack)
		}
		for _, target := range targets {
			if filepath.Ext(target) != ".md" {
				t.Errorf("%s 的工具交付扩大了 learning 写入类型: %s", pack, target)
			}
			for _, forbidden := range []string{
				"tooling/scripts/export_function_evidence.py",
				"tooling/schemas/ida-function-evidence-v1.schema.json",
				"tooling/schemas/bounded-replay-request-v1.schema.json",
				"tooling/schemas/bounded-replay-result-v1.schema.json",
			} {
				match, err := filepath.Match(filepath.FromSlash(target), filepath.FromSlash(forbidden))
				if err != nil || match {
					t.Errorf("%s 的 learning target %q 不能包含工具或 schema %s: %v", pack, target, forbidden, err)
				}
			}
		}
	}
}

func TestResearchExecutionMethodsKeepEvidenceAndNativeBoundaries(t *testing.T) {
	repo := repoRoot(t)
	for _, method := range []struct {
		path     string
		required []string
	}{
		{"packs/binary-re/tooling/recipes/ida-function-evidence.md", []string{"export_function_evidence.py", "IDAPython", "只读"}},
		{"packs/web-security/tooling/recipes/curl-read-evidence.md", []string{"curl", "GET", "重试"}},
		{"packs/binary-re/references/binary-re/bounded-behavior-chain.md", []string{"连接", "预测", "unknown"}},
		{"packs/web-security/references/web-security/request-sequence-review.md", []string{"连接", "预测", "unknown"}},
		{"packs/web-security/references/web-security/client-api-joint-review.md", []string{"客户端", "服务端", "版本", "对象", "unknown"}},
	} {
		text := readPrototypeFile(t, repo, method.path)
		for _, required := range method.required {
			assertContains(t, text, required, method.path)
		}
		for _, forbidden := range []string{"task-handoff.md", "packs/binary-re/scripts/", "go run ./cmd/"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s 引入不属于本路线的旧入口 %q", method.path, forbidden)
			}
		}
	}
	for _, route := range []struct{ path, target string }{
		{"packs/binary-re/references/binary-re/README.md", "bounded-behavior-chain.md"},
		{"packs/web-security/references/web-security/README.md", "request-sequence-review.md"},
		{"packs/web-security/references/web-security/README.md", "client-api-joint-review.md"},
		{"packs/web-security/tooling/recipes/request-replay.md", "curl-read-evidence.md"},
	} {
		assertContains(t, readPrototypeFile(t, repo, route.path), route.target, route.path)
	}
}
