# Context routing and progressive disclosure

## 读取指南

本文件是新会话、上下文压缩后接手和每轮自主推进的**第一路由入口**。新会话先确认当前在 `main` 分支、`main` 与 `origin/main` 同步且工作树干净，再读本文件，并只读它指向的少量顶部区；不要默认串读所有 durable docs，也不要默认读取 `docs/batch-history.md` 全文。

## 实施摘要

当前目标不是增加更多必读文档，而是降低上下文压力：用 `docs/context-routing.md` 做按需路由，用 `docs/batch-plan.md` 顶部做 active state，用 `docs/batch-history.md` 归档旧批次。默认只加载当前决策需要的顶部摘要；历史、release 细节、PowerShell 删除门禁、pack authoring、smoke matrix 等内容都按需进入。

## 执行清单

### 每轮默认读取（最小集）

1. `CLAUDE.md`：项目边界、路由规则和验证命令；只读顶部，不把它当完整设计文档。
2. `docs/context-routing.md`：本路由表。
3. `docs/batch-plan.md`：只读 current milestone / current batch state / next candidates / latest completed batch。
4. `CHANGELOG.md` 顶部 `Unreleased`：确认最新用户可见变化是否覆盖当前 batch。
5. 真实状态：`git status --short`、必要的 focused tests、本机 release gate；远程 CI 只在需要 release 判断时检查。

### Batch push cadence

- 一个正常 batch 最多两次 push：implementation commit（代码、测试、文档、本地验证）和 release inspection commit（只记录 implementation commit 触发的远程 run）。
- 不要再为 release inspection commit 自己触发的 CI 追加第三个记录提交；除非该 run 出现不同于既有 GitHub Actions runner/billing `steps=[]` blocker 的新信号，否则只在当前上下文或下一批 planning 中引用既有 blocker。
- 若用户 goal 尚未授权 commit/push，仍只维护工作树和验证结果；不要为了满足 cadence 规则自行 push。

### 批次选题防局部最优

- 连续多个 batch 如果都只是把字段、summary、handoff detail 或 text line 从一个 envelope 投影到另一个 envelope，就要视为“内部 contract 可见性微调”风险，而不是继续寻找下一个 `latestX` / `summaryX` / `contextX`。
- 选题前先写清真实断点：哪个用户、Mission Commander、replacement executor、reviewer、lane executor 或 pack-memory review 流程现在不能完成下一步；没有真实断点时，不把投影补齐单独立批。
- 字段/text/handoff 可以作为中大型 vertical slice 的支撑，但该 slice 必须能通过 package / CLI / 临时 case / product-path 验证体现用户可感知的 operational closure。

### 上下文节流

- 大文件（如 `internal/rekit/cli/cli_test.go`、`docs/batch-history.md`）只按 symbol、Batch ID 或行号读取小片段；不要为一次批次接手或文档更新全量读取。
- 查看 diff 时优先 `git diff --stat`、目标文件 diff 或 targeted grep；避免把完整大 diff 输出进对话上下文。
- 测试失败时先保留失败摘要、测试名和关键错误；只有定位需要时再读取完整日志文件或重跑 focused test，避免长日志挤占上下文。

### 文档维护时

- 修改或新建 durable docs 时，同样保持按需路由：顶部保留短 `读取指南` / `实施摘要` / `执行清单` / `验证标准` / `风险与注意事项`，细节按章节或专文渐进披露。
- 只在 `docs/context-routing.md`、`README.md` 或根 `CLAUDE.md` 放短路由指针；不要把历史、完整实施日志或长设计细节并回 active docs。
- 文档索引、推荐话术和接手清单不是默认必读清单；若出现 5 个以上 read-first 文件，先压缩为 `docs/context-routing.md` + 当前场景入口 + 顶部区。
- 新增文档时必须说明它的首选入口、何时按需读取、不要默认读取什么；若只是历史或 release/debug 溯源，优先放入归档或专题文档，不放进默认上下文。
- `CHANGELOG.md` 只记录用户可见变化与关键边界；旧批次细节继续按 Batch ID 归档到 `docs/batch-history.md`。

### 按需路由

| 需要判断什么 | 读什么 | 不要默认读什么 |
|---|---|---|
| 产品北极星是否偏移 | `docs/mission-control-product-direction.md` 顶部 80-120 行 | 不读全篇历史章节 |
| 架构四层模型 / stable boundary | `docs/design.md` 顶部和对应小节 | 不把 batch 日志并回架构总览 |
| 自主短 goal / 接手 cadence | `docs/autonomous-goal.md` 顶部 80-120 行 | 不复制整段 goal 到每次总结 |
| 2026-07-28 项目复审 / 中长期优化路线 | `docs/project-reassessment-2026-07-28.md` 顶部执行区；仅在重新评估方向或选择中大型主线时按需读取 | 不把复审报告加入每轮默认 read-first |
| 当前 batch 和下一步 | `docs/batch-plan.md` 顶部 current/next/latest completed | 不读 `docs/batch-history.md` |
| 旧 batch 细节 / 考古 | `docs/batch-history.md` 中按 Batch ID 搜索 | 不从 Batch 0 顺序读 |
| release / CI 判断 | `docs/release-readiness.md` 顶部和 Known gaps；再看 `release-check -Format json` | 不把 `ciReleaseGate.ready` 当远程 green |
| PowerShell façade / removal | `docs/powershell-deprecation.md` 顶部和相关矩阵行 | 不默认运行 façade smoke |
| Go runtime / command owner | CodeGraph 查询 `internal/rekit/**`，必要时读 `docs/go-first-convergence-plan.md` 或 `docs/go-runtime-migration.md` 顶部 | 不先读历史 migration 全文 |
| ledger / evidence / intervention 字段 | `docs/evidence-ledger.md` 顶部和对应事件类型 | 不读取完整 case ledger 或复制大 sidecar |
| reviewer / lane / gate product path | CodeGraph 查询相关 Go symbols + `docs/agent-team-usage.md` 顶部；长期分层按需读 `docs/orchestration-plan.md` 顶部 | 不读全部 batch 历史 |
| pack authoring / promote/sync | `docs/pack-authoring.md`、`docs/promote-sync.md` 对应章节 | 不把 case artifact 写回仓库 |
| 旧 case 迁移 / moved metadata | `docs/case-migration.md` 顶部和对应步骤 | 不记录真实 case 路径或 case-specific 进度 |
| 文档减压 / 路由审计 | 本文件 + 目标文档顶部；用搜索定位旧 read-first 列表 | 不批量重写所有历史文档、不把索引当必读清单 |
| smoke 选择 | `rekit/tests/README.md` 对应类别 | 不默认跑大型 matrix |

## 验证标准

- 新会话能从本文件和 `docs/batch-plan.md` 顶部恢复当前方向，而不需要读取 10 万 token 级历史。
- `docs/batch-plan.md` 只保留 active/current/latest 摘要；完整旧批次位于 `docs/batch-history.md`。
- `release-check -Format json` 的 `releaseHandoff.readFirst[]` 优先指向本路由表和短 current state，而不是把所有长文档都列为必读。
- README、reference、usage guide 或 handoff 话术中的文档列表应表达“按需索引”，不能绕过本文件变成新的默认 read-first 清单。
- 本机验证仍以 Go-native minimum 为准：`release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`。

## 风险与注意事项

- 渐进式披露不是删除事实；只是把事实放到正确层级，并通过路由按需读取。
- 不要把历史归档重新并回 `docs/batch-plan.md`。
- 当前用户短期优先 Windows 本机稳定；远程 Linux/macOS/Windows CI 因 runner/billing blocker 只保持 known gap 记录，不阻塞本机 Mission Control 闭环。
- actual heavy-tool、authority/confirmed、sync/promote 写回、runtime schema 迁移和公共 façade 删除仍按对应 gate 升级。