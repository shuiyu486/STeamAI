# Stop hook checklist

每轮修改项目文件后：

1. 更新相关 README / CLAUDE / reference / task handoff。
2. 若产生可复用经验，运行 `/rekit promote` 的 review-first 流程，避免把 case 私有状态回流模板。
3. 若代码变更，运行语法检查或测试。
4. 若 confirmed CSV 变更，重建 routine IR / superinstruction。
5. 运行 `/rekit doctor`，验证 Markdown 大小不超预算。
6. 如果当前目录不是 git 仓库，在最终回复说明无法 commit/push。
7. 涉及 batch release inspection 或 CI 记录时，遵守最多两次 push cadence：implementation commit + 只记录该 implementation run 的 release inspection commit；不要为 inspection commit 自己触发的 CI 追加第三个记录提交，除非出现不同于既有 `steps=[]` runner/billing blocker 的新信号。
