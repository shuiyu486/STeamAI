# STeamAI vNext 薄核心路线

## 读取指南

本文件是已完成 `steamai-vnext-thin-core-v1` 的验收事实源。新会话先由 `docs/context-routing.md` 路由到这里，只读取顶部完成区与验收卡；`docs/batch-plan.md` 只是短投影，不是第二选题源。新的条件卡只在真实反馈或明确目标出现时按需读取 `docs/real-usage-hardening-backlog.md`。更早历史从 Git history 或 `CHANGELOG.md` 追溯。

## 实施摘要

`steamai-vnext-thin-core-v1` 已完成。当前产品是薄的安全研究团队核心：复用 Claude Code 原生会话、消息、恢复和工具；用成员目录与 `CLAUDE.md` 承载身份和当前任务；用 artifact、evidence、finding、review 与经确认的 learning 回流形成研究闭环。canonical `/steamai` 已切换，旧产品控制面已整体删除。

保持 `source-clone-first`，不实现 installer、GUI/TUI 或新 PowerShell runtime logic，并拒绝 PATH/外部 kit fallback。切换后默认 quickstart 只保留 `cd <project> → claude → /steamai`。vNext 不以长期无人值守自治为目标，不自建 Agent runtime、session supervisor、消息总线、任务数据库或通用事务框架。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-vnext-thin-core-v1` |
| 当前批次 | `VNT-06 收尾发布验证` |
| 状态 | `completed` |
| 唯一允许领取 | 无；批准路线已完成 |
| 前置 | `VNT-05` 已完成旧控制面物理删除、pack/common 声明式收敛、policy closure 与终审修复 |
| 下一批 | 无；后续仅按真实使用反馈立项 |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| canonical module | `github.com/shuiyu486/STeamAI`；仅承载 test-only acceptance |
| legacy inputs | 旧 `.steamai` / `.rekit` 只读 importer source，不是兼容 runtime |

## 执行清单

- [x] 删除 `cmd/**`、`internal/rekit/**`、legacy `/rekit`/`verify` skills、PowerShell façade、adapter/runtime assets 与旧生产 Go package。
- [x] 将 project-local skill template 迁入 `vnext/project-skill/`，保持 canonical exact bytes，不保留 generator 或第二运行入口。
- [x] 重写 CI 与 stop-hook，不再调用已删除 CLI、release-check、doctor、sync/promote 或 runtime smoke。
- [x] 将 `common/**` 收敛为 Claude Code 原生团队、evidence/review/learning 与权限确认语义。
- [x] 将 `packs/**` 收敛为声明式领域资产，删除旧 lane/ledger/gate/runtime schema、失效命令与 executable 脚本。
- [x] 增加 repository no-legacy-production、manifest/policy closure 与 pack/common exact snapshot 验收；fresh suite、vet、diff、Windows live probe 与终审通过。

## 完成验收卡

### VNT-06：收尾发布验证

**状态**：completed。

**目标结果**：

1. 仓库不再包含旧 mega CLI、session/driver/reviewer 状态机、unified runtime、adapter host、PowerShell façade 或可发现的 legacy `/rekit` skill。
2. 产品源只保留 canonical skill、vNext 声明式模板/合同、声明式 pack/common 与 test-only acceptance；生产 Go package 数量为零。
3. selected pack snapshot 包含 pack 及其 current common closure，全部来自同一 exact Git revision；case 日常不读取 mutable source clone。
4. CI、README、router、stop-hook 和 current pack/common 不引用已删除命令、状态根或旧 runtime authority/gate/ledger 语义。

**非目标**：本批不抹除 Git history 中的旧架构事实，不引入 installer、GUI/TUI、生产 helper 或新控制状态机，不执行真实 heavy action，不自动 commit/push。

**验收结果**：tracked production paths 无 `cmd/**`、`internal/rekit/**`、`rekit/**`、legacy skills 或 PowerShell runtime；唯一 Go package 为 test-only `internal/steamai/vnextcontract`，module 为 canonical `github.com/shuiyu486/STeamAI`；canonical/template exact、importer、pack/common snapshot、manifest/policy closure、repository no-legacy contracts、fresh suite、vet、diff、Windows live probe 与终审均通过。未执行 remote CI、跨机器 Remote Control E2E 或 formal release，不将 workflow definition 冒充这些结果。

## 路线级验证标准

- 一个项目对应一个明确授权的安全研究 case；不同 case 只共享经 Reviewer 审查、脱敏和用户确认的 pack 经验。
- Commander 按需组队；成员身份和当前任务由专属目录 `CLAUDE.md` 承载，默认可见独立 Claude Code 会话。
- 成员直接定向协作但主任务优先；每个问题默认一名 owner、最多一名 verifier；durable member 创建只由 Commander 决定。
- artifact 有 case-local 引用与完整性信息；finding 可追溯到 evidence；review 直接引用 finding/evidence。
- learning 只有通过证据、通用性、冲突、重复和脱敏审查，并由用户确认精确变更后才能回流。
- 新核心不得引入 global Options、通用 action transaction、session supervisor 或 durable message queue。
- 当前不保留双写、旧 runtime fallback、legacy skill 或第三套 adapter。

## 风险与注意事项

- `CLAUDE.md` 是角色上下文，不是授权边界；危险动作仍依赖用户权限确认和 case 授权。
- 原生消息不是 exactly-once 队列；人在环模式不以消息状态机补洞。
- 正式成员是从专属目录启动的独立 Claude Code 会话，不依赖 experimental Agent Team teammate。
- 不写真实样本、trace/dump/capture、artifact、绝对 case 路径、payload、凭据、客户信息或 case 进度到模板仓库。
- 不声称未实际运行的 remote CI green、Linux/macOS product-path acceptance、真实跨机器 Remote Control E2E 或 formal release。

## 路线变更记录

- 2026-09-01：用户批准完整 `steamai-vnext-thin-core-v1` 路线并授权各批次连续实施、每批自行审核复评后继续。路线基于当前控制面审查和 Claude Code 原生能力核验，选择替换式薄核心而非继续逐层拆旧架构。
