package vnextcontract

import (
	"strings"
	"testing"
)

// TestIndirectCallResolutionKeepsExclusionBounded 钉住 indirect-call
// resolution 的排除与 unknown 边界：地址 xref 缺失只缺证据，不足以证成
// 也不足以排除，排除需完整证成指针来源/索引/重定位语义的封闭全集；
// 已知重定位不必然 unknown，只有运行期未知写入才阻断。断言只钉可验证
// 要点，不给正常方法写死流程。
func TestIndirectCallResolutionKeepsExclusionBounded(t *testing.T) {
	repo := repoRoot(t)
	recipe := readPrototypeFile(t, repo, "packs/binary-re/tooling/recipes/indirect-call-resolution.md")

	for _, required := range []string{
		"不足以证成也不足以排除",
		"算术合成",
		"运行期解码",
		"分析不全",
		"封闭全集且 F 不在其中",
		"任一来源未覆盖时 F 保持 unknown",
		"运行期未知写入",
		"已知语义且写入值静态可解出的重定位",
		"不因\"是重定位\"就判 unknown",
	} {
		assertContains(t, recipe, required, "indirect-call-resolution.md")
	}
	for _, legacy := range []string{
		"不可能是该间接点的目标",
		"这常可先删掉大部分候选",
		"relocation 生效后",
		"它不是该间接点的目标",
	} {
		if strings.Contains(recipe, legacy) {
			t.Fatalf("indirect-call-resolution.md 保留了过强排除断言 %q", legacy)
		}
	}

	readme := readPrototypeFile(t, repo, "packs/binary-re/references/binary-re/README.md")
	assertContains(t, readme, "运行期未知写入整点 keep unknown", "binary-re README")
}
