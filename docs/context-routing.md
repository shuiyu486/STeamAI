# Context routing and progressive disclosure

## 读取指南

本文件是新会话、上下文压缩后接手和每轮自主推进的**第一路由入口**。先读本文件，再只读它指向的少量顶部区；不要默认串读所有 durable docs，也不要默认读取 `docs/batch-history.md` 全文。

## 实施摘要

当前目标不是增加更多必读文档，而是降低上下文压力：用 `docs/context-routing.md` 做按需路由，用 `docs/batch-plan.md` 顶部做 active state，用 `docs/batch-history.md` 归档旧批次。默认只加载当前决策需要的顶部摘要；历史、release 细节、PowerShell 删除门禁、pack authoring、smoke matrix 等内容都按需进入。

## 执行清单

### 每轮默认读取（最小集）

1. `CLAUDE.md`：项目边界、路由规则和验证命令；只读顶部，不把它当完整设计文档。
2. `docs/context-routing.md`：本路由表。
3. `docs/batch-plan.md`：只读 current milestone / current batch state / next candidates / latest completed batch。
4. `CHANGELOG.md` 顶部 `Unreleased`：确认最新用户可见变化是否覆盖当前 batch。
5. 真实状态：`git status --short`、必要的 focused tests、本机 release gate；远程 CI 只在需要 release 判断时检查。

### 按需路由

| 需要判断什么 | 读什么 | 不要默认读什么 |
|---|---|---|
| 产品北极星是否偏移 | `docs/mission-control-product-direction.md` 顶部 80-120 行 | 不读全篇历史章节 |
| 自主 goal / 停止条件 | `docs/autonomous-goal.md` 顶部 80-120 行 | 不复制整段 goal 到每次总结 |
| 当前 batch 和下一步 | `docs/batch-plan.md` 顶部 current/next/latest completed | 不读 `docs/batch-history.md` |
| 旧 batch 细节 / 考古 | `docs/batch-history.md` 中按 Batch ID 搜索 | 不从 Batch 0 顺序读 |
| release / CI 判断 | `docs/release-readiness.md` 顶部和 Known gaps；再看 `release-check -Format json` | 不把 `ciReleaseGate.ready` 当远程 green |
| PowerShell façade / removal | `docs/powershell-deprecation.md` 顶部和相关矩阵行 | 不默认运行 façade smoke |
| Go runtime / command owner | CodeGraph 查询 `internal/rekit/**`，必要时读 `docs/go-first-convergence-plan.md` 顶部 | 不先读历史 migration 全文 |
| reviewer / lane / gate product path | CodeGraph 查询相关 Go symbols + `docs/agent-team-usage.md` 顶部 | 不读全部 batch 历史 |
| pack authoring / promote/sync | `docs/pack-authoring.md`、`docs/promote-sync.md` 对应章节 | 不把 case artifact 写回仓库 |
| smoke 选择 | `rekit/tests/README.md` 对应类别 | 不默认跑大型 matrix |

## 验证标准

- 新会话能从本文件和 `docs/batch-plan.md` 顶部恢复当前方向，而不需要读取 10 万 token 级历史。
- `docs/batch-plan.md` 只保留 active/current/latest 摘要；完整旧批次位于 `docs/batch-history.md`。
- `release-check -Format json` 的 `releaseHandoff.readFirst[]` 优先指向本路由表和短 current state，而不是把所有长文档都列为必读。
- 本机验证仍以 Go-native minimum 为准：`release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`。

## 风险与注意事项

- 渐进式披露不是删除事实；只是把事实放到正确层级，并通过路由按需读取。
- 不要把历史归档重新并回 `docs/batch-plan.md`。
- 当前用户短期优先 Windows 本机稳定；远程 Linux/macOS/Windows CI 因 runner/billing blocker 只保持 known gap 记录，不阻塞本机 Mission Control 闭环。
- actual heavy-tool、authority/confirmed、sync/promote 写回、runtime schema 迁移和公共 façade 删除仍按对应 gate 升级。