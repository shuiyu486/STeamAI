# Batch implementation plan

## 读取指南

本文件只是当前路线的短投影，不选题、不保存完整实施日志。先由 `docs/context-routing.md` 选择场景；实施当前路线时，以 `docs/real-usage-hardening-roadmap.md` 的当前卡为唯一 source。完整历史只在 `docs/batch-history.md` 按 ID 查询。

## 实施摘要

当前路线是 `steamai-repository-identity-v1`。Batch 829只迁移GitHub repository的current display/clone/maintenance identity到`shuiyu486/STeamAI`，并用Go-owned default-doc readiness阻止旧clone URL回流；Go module/import、内部`rekit` names、legacy `/rekit` / `.rekit`和本地checkout目录名继续作为兼容身份保留。

## 执行清单

### Current projection

| 字段 | 当前值 |
|---|---|
| 路线 | `steamai-repository-identity-v1` |
| source | `docs/real-usage-hardening-roadmap.md` |
| 当前批次 | `Batch 829` GitHub repository identity migration |
| 状态 | `completed` |
| 唯一允许领取 | `无；Batch 829 已完成，不可继续领取` |
| 上一批 | `Batch 828` STeamAI self-contained project closure 已完成 |
| 下一批 | `无；等待用户明确改变路线` |

### Current batch state

新GitHub repository与本地`origin`已切换到`shuiyu486/STeamAI`；current docs、skills、templates、examples、active route和release invariant已完成迁移。Go module/import/internal package未机械迁移，历史事实未改写。

### Batch 829：GitHub repository identity migration

状态：已完成 repository identity implementation；release cadence 由 machine receipt 判定。

目标：让canonical GitHub repository、clone/handoff示例和维护文案一致指向`https://github.com/shuiyu486/STeamAI`，同时明确保留`github.com/shuiyu486/re-context-kits` Go module兼容身份与legacy `/rekit` / `.rekit`合同。

验证结果：新repository与本地remote、tracked identity migration、focused regressions和独立复核已完成；冻结工作树的完整local release minimum、direct implementation commit/push与post-push核验只由Git-local machine receipt和本地tracking ref证明，不由本文提前声称，也不声称remote CI green。

## 验证标准

- 本文件与路线图的route/current/state/claim/next必须一致；冲突时fail-closed。
- active plan只保留一个compact batch摘要；Batch 828及更早完整历史只在`docs/batch-history.md`。
- current clone URL只使用`shuiyu486/STeamAI`；旧module path明确作为compatibility identity保留。
- `completed`表示Batch 829 implementation已冻结；focused回归必须已通过，完整Windows local minimum、direct commit/push与post-push readiness必须再由同批Git-local machine receipt和tracking ref证明。

## 风险与注意事项

- 本文件不是第二份 roadmap；范围、边界和完成门槛只在当前卡。
- 不全局替换`re-context-kits`：Go imports、module identity和历史语境需要保留。
- repository rename不等于runtime/schema/entrypoint migration；不删除legacy surface，不强制重命名本地目录。
- local receipt和tracking ref不证明remote CI green。
