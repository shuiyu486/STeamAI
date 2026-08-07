# Context routing and progressive disclosure

## 读取指南

本文件是新会话、上下文压缩后接手和每轮自主推进的**唯一完整文档路由入口**。**本项目文档必须做成按需路由、渐进式披露的样式。** 新会话先确认当前 branch、HEAD 与工作树真实状态，再由本文件选择一个当前场景入口；不要先读取路线图、batch plan、CHANGELOG、release readiness 或历史文档来猜场景，也不要默认读取 `docs/batch-history.md` 全文。

## 实施摘要

当前目标不是增加更多必读文档，而是降低上下文压力并消除自由选题：用 `docs/context-routing.md` 做按需路由，用 `docs/real-usage-hardening-roadmap.md` 承载有序路线与解锁条件，用 `docs/batch-plan.md` 顶部投影 active pointer，用 `docs/batch-history.md` 归档旧批次。默认只加载当前批次需要的顶部摘要；历史、release 细节、PowerShell 删除门禁、pack authoring、smoke matrix 等内容都按需进入。

## 执行清单

### 每轮默认读取（两层）

1. **固定层**：根 `CLAUDE.md` 的稳定边界、本文件、`git status --short` 和当前 branch/HEAD；机器 `releaseHandoff.readFirst[]` 只列本文件。
2. **场景层**：从下表只选择一个首选入口，并只读其指定顶部区、当前卡或目标章节；需要验证事实时优先运行 focused command 或查询 CodeGraph，而不是增加默认文档。

路线实施时，当前路线文件拥有选题和验收；`docs/batch-plan.md` 只是短投影，不再作为第二份选题源。`CHANGELOG.md` 只在写 release notes 或核对用户可见变化时按需搜索 `Unreleased` 的相关条目，不默认阅读全文。

### Batch push cadence

- 普通 batch 以 Windows 本机 focused tests 与完整 release minimum 为完成门槛；已授权时只做一次 implementation commit/push，覆盖代码、测试、文档与本机验证。
- implementation push 成功后立即继续下一批，不轮询或等待远程 workflow，不默认创建 release inspection commit。
- 远程 Linux/macOS/Windows CI 保持异步、非阻塞；只在正式发布、跨平台专项或每 3–5 批周期复审时等待并记录实际结果。
- 若用户 goal 尚未授权 commit/push，仍只维护工作树和验证结果；不要为了满足 cadence 规则自行 push。

### 有序批次防偏移

- 当前不再从候选池选题。`docs/real-usage-hardening-roadmap.md` 当前指针是唯一允许领取的批次；它未完成时不得做其它路线项，完成后也只能领取明确解锁的下一批。
- 连续多个 batch 如果都只是把字段、summary、handoff detail 或 text line 从一个 envelope 投影到另一个 envelope，就要视为偏移；这些支撑改动只能并入当前路线批次，不能独立立批。
- 代码事实与路线假设冲突时，先写回路线变更记录、理由、验收和新指针，再实施；不得静默跳批、从历史猜测或用聊天摘要覆盖仓库指针。
- 两处 current pointer 冲突、完成证据缺失、工作树来源不明时，停止实施并报告冲突；不能“选择另一个能做的候选”绕过。

### 上下文节流

- 大文件（如 `internal/rekit/cli/cli_test.go`、`docs/batch-history.md`）只按 symbol、Batch ID 或行号读取小片段；不要为一次批次接手或文档更新全量读取。
- 查看 diff 时优先 `git diff --stat`、目标文件 diff 或 targeted grep；避免把完整大 diff 输出进对话上下文。
- 测试失败时先保留失败摘要、测试名和关键错误；只有定位需要时再读取完整日志文件或重跑 focused test，避免长日志挤占上下文。

### 文档维护时

- 修改或新建 durable docs 时，同样保持按需路由：active 入口顶部保留短 `读取指南` / `实施摘要` / `执行清单` / `验证标准` / `风险与注意事项`，细节按当前卡、章节、专题或 archive 渐进披露。
- 只有本文件拥有完整路由表；README、根 `CLAUDE.md`、goal、route、batch 和 release 文档只能放一条 canonical 指针，不复制整套路由或接手清单。
- 默认 machine `readFirst[]` 最多 2 项，且第一项必须是本文件；不得包含 `CHANGELOG.md`、`docs/batch-history.md`、archive 或完整未来 backlog。
- active batch plan 只投影路线、current、state、next 和一句最新结果；完整验证日志必须归档。active roadmap 只内联当前批次卡；未来批次进入单一按需 backlog。
- 文档索引、推荐话术和接手清单不是默认必读清单；若出现 5 个以上 read-first 文件、同一 current pointer 的多份长副本或单行巨型 inventory，先压缩和去重。
- 新增文档时必须说明它的首选入口、何时按需读取、不要默认读取什么；若只是历史或 release/debug 溯源，优先放入归档或专题文档，不放进默认上下文。
- `CHANGELOG.md` 只记录用户可见变化与关键边界；旧批次细节继续按 Batch ID 归档，不把逐批日志长期堆在 `Unreleased`。

### 按需路由

| 需要判断什么 | 读什么 | 不要默认读什么 |
|---|---|---|
| 产品北极星是否偏移 | `docs/mission-control-product-direction.md` 顶部 80-120 行 | 不读全篇历史章节 |
| 架构四层模型 / stable boundary | `docs/design.md` 顶部和对应小节 | 不把 batch 日志并回架构总览 |
| 短 goal / 接手 cadence | `docs/autonomous-goal.md` 顶部 80-120 行；goal 只启动已批准路线 | 不复制整段 goal 到每次总结，不让 goal 自由选题 |
| 当前多批次真实使用路线 | `docs/real-usage-hardening-roadmap.md` 顶部 + 当前批次卡 | 不预读后续批次全文，不从旧复审报告替换当前顺序 |
| 2026-07-28 项目复审 / 历史中长期建议 | `docs/project-reassessment-2026-07-28.md` 顶部执行区；仅在路线整体复审时按需读取 | 不把复审报告加入每轮默认 read-first，不用它临时选批 |
| 当前 batch 和指针投影 | `docs/batch-plan.md` 顶部 current milestone / roadmap pointer / current state / latest completed；执行旧健康恢复考古时再读 `docs/health-recovery-and-real-executor-plan.md` 顶部 | 不读 `docs/batch-history.md`，也不默认读旧执行专文全文 |
| 旧 batch 细节 / 考古 | `docs/batch-history.md` 中按 Batch ID 搜索 | 不从 Batch 0 顺序读 |
| release / CI 判断 | `docs/release-readiness.md` 顶部和 Known gaps；再看 `release-check -Format json` | 不把 `ciReleaseGate.ready` 当远程 green |
| PowerShell façade / removal | `docs/powershell-deprecation.md` 顶部和相关矩阵行 | 不默认运行 façade smoke |
| Go runtime / command owner | CodeGraph 查询 `internal/rekit/**`，必要时读 `docs/go-first-convergence-plan.md` 或 `docs/go-runtime-migration.md` 顶部 | 不先读历史 migration 全文 |
| ledger / evidence / intervention 字段 | `docs/evidence-ledger.md` 顶部和对应事件类型 | 不读取完整 case ledger 或复制大 sidecar |
| reviewer / lane / gate product path | CodeGraph 查询相关 Go symbols + `docs/agent-team-usage.md` 顶部；真实 Claude host/live gate 按需读 `docs/health-recovery-and-real-executor-plan.md` 顶部；长期分层按需读 `docs/orchestration-plan.md` 顶部 | 不读全部 batch 历史 |
| pack authoring / promote/sync | `docs/pack-authoring.md`、`docs/promote-sync.md` 对应章节 | 不把 case artifact 写回仓库 |
| 旧 case 迁移 / moved metadata | `docs/case-migration.md` 顶部和对应步骤 | 不记录真实 case 路径或 case-specific 进度 |
| 文档减压 / 路由审计 | 本文件 + 目标文档顶部；用搜索定位旧 read-first 列表 | 不批量重写所有历史文档、不把索引当必读清单 |
| smoke 选择 | `rekit/tests/README.md` 对应类别 | 不默认跑大型 matrix |

## 验证标准

- 新会话能从本文件、路线图顶部/当前批次卡和 `docs/batch-plan.md` 顶部恢复唯一允许工作，而不需要读取 10 万 token 级历史或自行选题。
- `docs/real-usage-hardening-roadmap.md` 与 `docs/batch-plan.md` 的路线、当前批次、状态和下一解锁批次一致；冲突时 fail-closed。
- `docs/batch-plan.md` 只保留 active/current/latest 摘要；完整旧批次位于 `docs/batch-history.md`。
- `release-check -Format json` 的 `releaseHandoff.readFirst[]` 优先指向本路由表和短 current state，而不是把所有长文档都列为必读。
- README、reference、usage guide 或 handoff 话术中的文档列表应表达“按需索引”，不能绕过本文件变成新的默认 read-first 清单。
- 本机验证仍以 Go-native minimum 为准：`release-check`、`status`、`packs`、`doctor`、`go test ./...`、`go vet ./...`、`git diff --check`。

## 风险与注意事项

- 渐进式披露不是删除事实；只是把事实放到正确层级，并通过路由按需读取。
- 不要把历史归档重新并回 `docs/batch-plan.md`。
- 当前产品支持和普通 batch 完成门槛以 Windows 本机为准；远程 Linux/macOS/Windows CI 是异步发布/专项/周期复审信号，不阻塞本机 Mission Control 闭环或下一批选择。
- actual heavy-tool、authority/confirmed、sync/promote 写回、runtime schema 迁移和公共 façade 删除仍按对应 gate 升级。