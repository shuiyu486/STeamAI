# 真实使用加固与日常产品收口路线图

## 读取指南

本文件是当前已批准路线的唯一 active source，只内联当前批次卡。先由 `docs/context-routing.md` 路由到本文件；本轮只在维护 GitHub repository identity、当前文档与 release invariant 时读取本卡。Batch 828 及更早历史按 ID 查询 `docs/batch-history.md`，STeamAI 自包含产品合同仍按需读取 `docs/steamai-self-contained-project.md`。

## 实施摘要

此前真实使用、日常闭环、Remote Control Reviewer transport、Windows Mission Control 和 STeamAI 自包含项目路线均已完成。当前路线是 `steamai-repository-identity-v1`：用户已将 GitHub repository 重命名为 `shuiyu486/STeamAI`，本批已把当前公开仓库身份、clone/handoff 示例、skills、模板与 release invariant 迁到新 canonical identity。

本批严格区分 repository identity 与兼容实现 identity。`go.mod` 的 `github.com/shuiyu486/re-context-kits`、既有 Go imports、`internal/rekit`、`cmd/rekit*`、legacy `/rekit` / `.rekit` 和本地 checkout 目录名暂不机械迁移；旧 GitHub URL 只允许作为 GitHub redirect 的历史事实，不能继续作为当前 clone/remote 指南。STeamAI 项目仍拒绝 PATH/外部 kit fallback，默认 quickstart 只保留 `cd <project> → claude → /steamai`。

### 当前指针

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-repository-identity-v1` |
| 当前批次 | `Batch 829` GitHub repository identity migration |
| 状态 | `completed` |
| 唯一允许领取 | `无；Batch 829 已完成，不可继续领取` |
| 上一批 | `Batch 828` STeamAI self-contained project closure 已完成 |
| 下一批 | `无；等待用户明确改变路线` |
| canonical repository | `https://github.com/shuiyu486/STeamAI` |
| 暂保留兼容身份 | Go module `github.com/shuiyu486/re-context-kits`、内部 `rekit` names、legacy `/rekit` / `.rekit` |
| 最近结果 | Repository identity implementation、focused regressions 与独立复核已完成；最终 local validation、commit/push 与 post-push readiness 只由 Git-local machine receipt 和 tracking ref 判定 |

## 执行清单

1. README、根 `CLAUDE.md`、产品方向、vision/design、goal、跨机器接手说明和当前 pack/common 索引已统一称为 STeamAI repository，并给出 canonical GitHub URL 与 clone command。
2. Legacy `/rekit` skill 与 case shim 的 repository 称呼已更新；`.rekit/instance.yml` 的 `templateRoot`、兼容入口和 deterministic runtime 协议保持不变。
3. Go-owned public default-doc readiness 已绑定 canonical repository URL与暂保留 Go module compatibility identity，并拒绝旧 GitHub clone URL回流。
4. Batch 828 已归档到 `docs/batch-history.md`，batch projection、CHANGELOG 与 release guidance 已同步；描述当时事实的历史评估和旧批次未机械改写。
5. Focused defaultdocs/manifest/releasecheck 回归、公开只读 inventory 与独立复核已完成；冻结字节只运行一次完整 local release minimum，成功后按 machine receipt 提交并推送到新 `origin/main`，不把本地 inventory 冒充 remote CI green。

## 当前批次卡

### Batch 829：GitHub repository identity migration

**目标**：让当前公开资料、维护入口和 release gate 一致指向 `https://github.com/shuiyu486/STeamAI`，同时保持旧 Go module/internal/legacy identity 可用，避免把 GitHub rename扩大成高风险机械重命名。

**用户断点**：GitHub repository 已改名，但 checkout 中仍有旧 clone URL和旧 repository称呼；若直接全局替换，又会破坏 Go module/import 与 legacy `.rekit` compatibility。

**范围内**：

- canonical GitHub repository、clone/remote、维护接手与 current repository display identity；
- README、CLAUDE、active docs、skills、case-shim、current examples、common/pack入口文案；
- Go-owned default-doc repository identity invariant；
- Batch 828归档、Batch 829 active projection、CHANGELOG 与本地验证/提交/推送。

**范围外**：

- `go.mod` module path、全仓 Go imports、`internal/rekit`、`cmd/rekit*`；
- `/rekit`、`.rekit`、`rekit.ps1` 或 legacy metadata合同的删除/改名；
- 本地物理 checkout目录强制改名；
- 历史批次、历史评估和当时事实的机械改写；
- GitHub Pages或自有 GitHub Action迁移（当前仓库不存在对应 surface）。

**当前结果**：`completed` implementation。GitHub repository可访问，本地 `origin` 已更新，tracked identity migration、release invariant、focused regressions与独立复核均已完成；完整local release minimum、direct implementation commit/push和post-push readiness只由同批Git-local machine receipt与本地tracking ref证明，不由本卡提前声称。

**完成门槛**：

- current clone/handoff示例只指向 `https://github.com/shuiyu486/STeamAI.git`；
- README和维护边界明确旧 module path只是暂保留 compatibility identity，不宣称已完成 Go module migration；
- release readiness在canonical URL缺失或旧 GitHub clone URL回流时fail-closed；
- active route/projection/history/CHANGELOG一致，focused regressions已通过；`completed` route表示implementation冻结，完整Windows local minimum必须由冻结快照的machine receipt记录；
- receipt绑定的direct implementation commit push后，`HEAD == origin/main`、工作树clean且post-push inspection为ready；remote workflow未读取时保持`not-recorded`。

## 验证标准

- `docs/real-usage-hardening-roadmap.md` 与 `docs/batch-plan.md` 的 route/current/state/claim/next 完全一致；冲突时停止。
- 默认文档与跨机器接手命令包含 `shuiyu486/STeamAI`，且不得把旧 GitHub slug继续当canonical clone URL。
- `module github.com/shuiyu486/re-context-kits` 与现有 imports保持不变，并被明确标记为兼容身份，而非遗漏迁移。
- legacy `.rekit` case与current `.steamai` project语义不因repository rename合并、双写或改路由。
- 完整 local gate只运行一次有增量价值的冻结工作树验证；不重复长测，不声称未读取的remote CI green。

## 风险与注意事项

- GitHub通常重定向旧web和Git remote URL，但维护文档与本地`origin`仍应使用新canonical URL；不要重新占用旧repository slug，以免破坏旧Go module URL redirect。
- 本地目录名不等于GitHub repository identity。不要强制重命名当前checkout，以免旧`.rekit` metadata中的绝对`templateRoot`失效。
- `uses: owner/repository@ref`和GitHub Pages project URL不享受普通rename语义；当前仓库没有自有Action或Pages surface，后续新增时需单独验证。
- 本批不改变runtime schema、权限、authority/confirmed、heavy action或sync/promote边界，也不新增PowerShell runtime logic。

## 路线变更记录

- 2026-08-17：用户完成GitHub repository rename，明确解锁`steamai-repository-identity-v1` / Batch 829；采用repository display identity与Go/internal compatibility identity分层迁移。
- 2026-08-16：Batch 828按Windows口径完成并推送；同批Windows checkout receipt repair已收口，完整证据归档到`docs/batch-history.md`。
