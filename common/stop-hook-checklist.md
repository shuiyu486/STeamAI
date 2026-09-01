# Stop hook checklist

每轮修改项目文件后：

1. 更新相关 README、`CLAUDE.md`、reference 和 task handoff。
2. 若产生可复用经验，只从 accepted finding/review 提炼 learning candidate；按 `vnext/learning-feedback.md` 完成 Reviewer 审查、exact patch 和用户确认边界，不直接覆盖 pack。
3. 若代码或合同测试变更，运行最短 focused tests；收尾再运行 canonical suite 与 `go vet`。
4. 若模板、pack 或文档变更，检查 source-clone 与 case-local snapshot/contract 引用没有漂移，也没有引入真实 case 数据。
5. 运行 `git diff --check`；如实记录未执行的 live acceptance、remote CI 或 formal release。
6. 如果当前目录不是 Git 仓库，在最终回复说明无法 commit/push。
7. 不自动 commit/push；只有用户明确要求时执行外部 Git 操作。路线未完成时，完成当前批次审核复评后继续下一批。
