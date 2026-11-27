package vnextcontract

import "testing"

// 防止分发时再次漏掉原负责人的入口或把局部阻塞交给用户调度；不代表真实团队表现。
func TestOwnerContinuationKeepsFixHandbackAndInterruptionWithCommander(t *testing.T) {
	skill := readPrototypeFile(t, repoRoot(t), ".claude/skills/steamai/SKILL.md")
	for _, guard := range []string{
		"按成员实际工作目录核对过的入口", "成员单写自己的任务文件",
		"成品已满足退出条件就接着集成", "只继续不受影响且已获准的工作", "已发未明的操作不重放",
	} {
		assertContains(t, skill, guard, "SKILL.md 负责人修复与续办")
	}
}
