package vnextcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestProfessionalDelegationUpstreamIsPinnedAndBounded(t *testing.T) {
	repo := repoRoot(t)
	manifest := readPrototypeFile(t, repo, "packs/binary-re/manifest.yml")
	for _, path := range []string{"tooling/integrations/rea.md", "upstream/rea/SOURCE.md"} {
		if !slices.Contains(manifestListValues(manifest, "tooling"), path) {
			t.Errorf("packs/binary-re/manifest.yml 未在 tooling 声明 %s", path)
		}
	}

	upstreamRoot := filepath.Join(repo, "packs", "binary-re", "upstream", "rea")
	var files []string
	err := filepath.WalkDir(upstreamRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, filepath.ToSlash(mustRel(t, repo, path)))
		assertRegularNonSymlink(t, path, "固定上游材料")
		if entry.Name() != "LICENSE" && filepath.Ext(path) != ".md" {
			t.Errorf("固定上游材料只允许 Markdown 与 LICENSE，发现 %s", path)
		}
		if filepath.Ext(path) == ".md" {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(strings.TrimSpace(string(data))) == 0 {
				t.Fatalf("固定上游材料为空: %s", path)
			}
			if len(data) > 16*1024 {
				t.Fatalf("固定上游材料超过 16 KiB 阅读预算: %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("packs/binary-re/upstream/rea 为空")
	}

	for _, original := range []struct {
		parts []string
		bytes int
		sha   string
	}{
		{[]string{"skill-1of2.md", "skill-2of2.md"}, 23510, "976e400f028618926c0b658ee50df79996f5f32402d33b49a23bfdbc232b45e6"},
		{[]string{"native-and-artifacts-1of2.md", "native-and-artifacts-2of2.md"}, 18911, "18b8828aec6476be03ca92bdb9506c47cc9ef8c1415c37d33e4af2f8c55cf8f9"},
		{[]string{"evidence-workflows.md"}, 4363, "2bdee98a843d018cf850c706111bb74c459ba8640da00799e9208122e4d5ab80"},
	} {
		var joined []byte
		for _, part := range original.parts {
			data, err := os.ReadFile(filepath.Join(upstreamRoot, part))
			if err != nil {
				t.Fatalf("固定上游材料缺失 %s: %v", part, err)
			}
			joined = append(joined, data...)
		}
		if len(joined) != original.bytes {
			t.Fatalf("上游 %s 拼接长度 %d 不等于上游原文 %d", original.parts[0], len(joined), original.bytes)
		}
		sum := sha256.Sum256(joined)
		if hex.EncodeToString(sum[:]) != original.sha {
			t.Fatalf("上游 %s 拼接 SHA-256 与上游发布不一致", original.parts[0])
		}
	}

	license := readPrototypeFile(t, repo, "packs/binary-re/upstream/rea/LICENSE")
	assertContains(t, license, "MIT License", "upstream/rea/LICENSE")
	assertContains(t, license, "morluto", "upstream/rea/LICENSE")

	source := readPrototypeFile(t, repo, "packs/binary-re/upstream/rea/SOURCE.md")
	assertContains(t, source, "rea-agents@6.7.0", "upstream/rea/SOURCE.md")
	assertContains(t, source, "27dc42d10249f0410904a75c0997842eb03b931c", "upstream/rea/SOURCE.md")

	targets := manifestListValues(manifest, "learningTargets")
	if len(targets) == 0 {
		t.Fatal("packs/binary-re/manifest.yml 缺少 learningTargets")
	}
	protected := append(files, "packs/binary-re/tooling/integrations/rea.md")
	for _, file := range protected {
		for _, target := range targets {
			match, err := filepath.Match(filepath.FromSlash(target), filepath.FromSlash(file))
			if err != nil || match {
				t.Fatalf("learning target %q 覆盖了固定专业接入材料 %s: %v", target, file, err)
			}
		}
	}
}

func TestProfessionalDelegationContractsKeepBoundaries(t *testing.T) {
	repo := repoRoot(t)
	integration := readPrototypeFile(t, repo, "packs/binary-re/tooling/integrations/rea.md")
	for _, required := range []string{
		"rea-agents", "local-scope", "attached", "headless", "evidence_id", "不自动重试",
		"provider 身份与目标绑定", "不能互相代替", "无法核验仍须停止",
		"导出成功不等于内容已读", "未授权的 case 外缓存",
	} {
		assertContains(t, integration, required, "tooling/integrations/rea.md")
	}
	for _, forbidden := range []string{"npm install", "npx rea-agents@latest"} {
		if strings.Contains(integration, forbidden) {
			t.Errorf("tooling/integrations/rea.md 引入了自动安装入口 %q", forbidden)
		}
	}

	roles := readPrototypeFile(t, repo, "vnext/templates/roles/analysis-member.md")
	assertContains(t, roles, "固定版本", "analysis-member.md")
	assertContains(t, roles, "不先运行较弱的默认流程", "analysis-member.md")
	assertContains(t, roles, "不以替代流程冒充同等交付", "analysis-member.md")

	skill := readPrototypeFile(t, repo, ".claude/skills/steamai/SKILL.md")
	assertContains(t, skill, "tooling/integrations/", "SKILL.md")
	assertContains(t, skill, "upstream/", "SKILL.md")
	assertContains(t, skill, "不先运行较弱的默认流程", "SKILL.md")
	assertContains(t, skill, "不为接入自动安装", "SKILL.md")

	capabilities := readPrototypeFile(t, repo, "vnext/capabilities.md")
	assertContains(t, capabilities, "外部专业工具接入", "capabilities.md")
	assertContains(t, capabilities, "local-scope MCP", "capabilities.md")
	assertContains(t, capabilities, "不是 durable member", "capabilities.md")
	assertContains(t, capabilities, "只在单一 MCP 连接内有效", "capabilities.md")

	acceptance := readPrototypeFile(t, repo, "vnext/acceptance.md")
	assertContains(t, acceptance, "steamai-professional-delegation-v1", "acceptance.md")
	assertContains(t, acceptance, "PD-03", "acceptance.md")
	assertContains(t, acceptance, "PD-05", "acceptance.md")
	for _, required := range []string{
		"ANTHROPIC_DEFAULT_HAIKU_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL",
		"ANTHROPIC_DEFAULT_OPUS_MODEL", "--model haiku", "--model sonnet", "不自动升级",
		"不将汇总 0 当免费或预算保护",
	} {
		assertContains(t, acceptance, required, "acceptance.md")
	}
	for _, required := range []string{
		"任一关键项不成立即维持受观察专项试用", "预算自然停止不代替用户主动停止验收",
		"启动前 instruction 不代替运行中纠偏", "不等于实际响应模型", "未来硬上限",
	} {
		assertContains(t, acceptance, required, "acceptance.md")
	}

	roadmap := readPrototypeFile(t, repo, "docs/professional-delegation-roadmap.md")
	assertContains(t, roadmap, "steamai-professional-delegation-v1", "professional-delegation-roadmap.md")
	assertContains(t, roadmap, "PD-06", "professional-delegation-roadmap.md")

	routing := readPrototypeFile(t, repo, "docs/context-routing.md")
	assertContains(t, routing, "professional-delegation-roadmap.md", "context-routing.md")
}

func mustRel(t *testing.T, base, target string) string {
	t.Helper()
	rel, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}
