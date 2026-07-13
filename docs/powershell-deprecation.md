# PowerShell runtime deprecation strategy

## 读取指南

本文件定义 `rekit` PowerShell 层的冻结、保留、迁移与删除策略，配合 `docs/release-readiness.md` 和 `docs/go-first-convergence-plan.md` 使用。维护者在修改 `rekit/rekit.ps1`、`rekit/lib/*.ps1`、Go façade 委托集合、fallback 逻辑或旧文本 flow 前应先读本文件顶部区域。

本文件不是立即删除 PowerShell runtime 的计划。当前用户入口仍是 `/rekit` / `rekit/rekit.ps1`，PowerShell 仍承担 façade、参数兼容、legacy text flow 和 Windows parity smoke 职责；任何删除、冻结或默认委托变化都必须单独批次验证。

## 实施摘要

当前策略：

- **Go-owned**：确定性状态、结构化输出、低风险写入、`release-check` inventory 和 release invariant 优先由 `cmd/rekit/**` 与 `internal/rekit/**` 维护。
- **PowerShell façade**：`rekit/rekit.ps1` 保持公共入口与兼容层，负责 Go binary 查找、参数兼容、fallback、环境变量开关和旧 case 用户体验。
- **Legacy-only**：无 `-Apply` 的文本工作线 flow、文本 sync/promote what-if、内部命令和非 Go-owned 写入路径暂时保留，禁止扩展新能力。
- **Parity smoke**：少量 PowerShell smoke 保留为 Windows façade / fallback 回归；大型 pack matrix 不作为默认 release gate。
- **删除前置条件**：只有当对应命令已有 Go owner、文档入口、release invariant、fallback 替代和临时 case验证后，才能考虑冻结或删除 PowerShell 实现。

## 执行清单

修改 PowerShell 或新增 Go-owned 路径时按以下流程执行：

1. 判断目标属于 **Go-owned**、**façade-only**、**legacy-only** 还是 **blocked**。
2. Go-owned 命令的新逻辑优先进入 Go package；PowerShell 只改委托、参数透传或 fallback。
3. Legacy-only 路径只做 bug fix、安全边界修复或兼容性维护；不新增功能面。
4. 任何默认委托变化必须保留 `REKIT_GO_DISABLE=1` fallback，除非单独 deprecation batch 明确删除并验证。
5. 删除 PowerShell 代码前，先完成 freeze 期：文档标记、tests/invariants 覆盖、release gate 通过、至少一个批次无回退需求。
6. 每批更新 `docs/batch-plan.md`、`docs/release-readiness.md`、`docs/go-first-convergence-plan.md` 或本文件，避免只留在聊天上下文中。

## 验证标准

PowerShell deprecation 相关变更至少满足：

- Go package tests 覆盖新 owner 的 deterministic 行为。
- `go test ./...` 与 `go vet ./...` 通过。
- `./rekit/rekit.ps1 -Command doctor` 通过。
- 涉及 façade 委托时运行 `./rekit/tests/facade-smoke.ps1`。
- 涉及 legacy fallback 时显式验证 `REKIT_GO_DISABLE=1`。
- 涉及 workstream / ledger / gate / sync / promote 写入时使用临时 case 验证 containment、backup、review-first 和 no authority/confirmed 边界。
- `git diff --check` 无 whitespace error。

## 风险与注意事项

- 不把 PowerShell 删除当作 release readiness 的前提；当前阶段是先冻结语义和减少扩张。
- 不删除 case-local thin shim；case-local `/rekit` 必须继续回到 kit 仓库 canonical runtime。
- 不用 Go 默认委托绕过 review-first、backup、deny、restore、gate 或人工确认。
- 不把 actual heavy-tool、authority/confirmed 写入、policy schema 迁移、外部副作用纳入自动迁移。
- 不默认运行大型 matrix；只在 pack helper/matrix 或跨 pack skeleton 变更时选择运行。

## 命令归属矩阵

| 区域 | 当前 owner | PowerShell 状态 | 冻结/删除策略 |
|---|---|---|---|
| `release-check` | Go default | façade delegate + no PowerShell fallback | 只输出 release gate inventory，不执行测试、不写状态；保持 Go-owned。 |
| `status` / `packs` / `doctor` / `validate` | Go default | façade + fallback | 保留 façade；PowerShell 实现只做兼容修复。 |
| case lifecycle `attach` / `repair` / `init` / `bootstrap` preview/apply | Go default | façade + fallback | 保留 `REKIT_GO_DISABLE=1` fallback；删除需旧 case smoke 通过。 |
| `sync` review/apply/JSON preview | Go default | façade + text dry-run fallback | 文本 `sync -Apply -WhatIf` 维持 legacy-only；不扩功能。 |
| `promote` review/artifacts/candidates/apply/JSON preview | Go default | façade + text what-if fallback | 文本 promote what-if 维持 legacy-only；pack source 写入仍要求 review-first/backup。 |
| `overview` text/JSON 与缺 board 初始化 | Go default | façade + fallback | PowerShell read layer 只修兼容，不新增展示语义。 |
| `note -List` text/table/tsv/JSON、`note` append、`note -WhatIf` | Go default | façade + fallback | 新 ledger schema 校验优先 Go；PowerShell 不新增 kind。 |
| `gate -WhatIf` / `gate -Apply` pending-gate | Go default | façade + fallback | 只预览或写 pending-gate request；不执行 heavy-tool。 |
| `start` / `handoff` JSON preview/apply | Go default | façade + text fallback | 文本 preview 可保留 legacy；结构化语义以 Go 为准。 |
| `continue -WhatIf -Format json` / explicit `continue -Apply` | Go default safe subset | façade + fallback | `continue -Apply` 不写 authority/confirmed；text flow legacy-only。 |
| `plan-subagents` review artifacts | Go manual path + PowerShell internal flow | internal/fallback | 不自动 spawn agent；默认 façade 委托变化需单独 batch。 |
| 无 `-Apply` 的文本工作线 flow | PowerShell legacy | legacy-only | 冻结语义；只修 bug，不新增状态模型。 |
| actual heavy-tool 执行 | 未迁移 | blocked / manual gate | 不自动迁移；需要用户确认和单独设计。 |
| authority/confirmed 写入 | 人工 gate / legacy guarded | blocked | 不由 Go apply 自动执行；删除/迁移需 schema 与恢复策略。 |

## PowerShell 模块状态

| 模块 | 状态 | 说明 |
|---|---|---|
| `rekit/rekit.ps1` | façade-stable | 公共入口、参数兼容、Go binary 查找、fallback 和错误透传；不放业务语义。 |
| `rekit/lib/Manifest.ps1` | compatibility | manifest 读取/兼容 helper；Go manifest package 是 release invariant owner。 |
| `rekit/lib/Validate.ps1` | parity / compatibility | pack/case doctor 兼容层；Go doctor 是默认路径。 |
| `rekit/lib/Instance.ps1` | compatibility | case instance/shim 兼容；case lifecycle Go-owned。 |
| `rekit/lib/Sync.ps1` | legacy fallback | sync 文本 dry-run 与 fallback；写入语义 Go-owned。 |
| `rekit/lib/Promote.ps1` | legacy fallback | promote 文本 fallback 与兼容；candidate/apply 语义 Go-owned。 |
| `rekit/lib/Review.ps1` | compatibility | review artifact helper；新增确定性 review 语义优先 Go。 |
| `rekit/lib/B3.Core.ps1` | shared compatibility | PowerShell B3 基础 helper；只修兼容和安全边界。 |
| `rekit/lib/B3.State.ps1` | legacy / compatibility | board/facts/lane 状态兼容；Go workstream 是新 owner。 |
| `rekit/lib/B3.Policy.ps1` | legacy / compatibility | policy helper；schema 迁移需单独 gate。 |
| `rekit/lib/B3.Lane.ps1` | legacy text flow | lane prompt/workspace helper；结构化 start/handoff/continue 以 Go 为准。 |
| `rekit/lib/B3.Auto.ps1` | legacy guarded | continue auto / authority guard 旧实现；不扩 authority/confirmed 自动写入。 |
| `rekit/lib/B3.Handoff.ps1` | legacy display fallback | handoff fallback 展示；Go handoff 是默认结构化 owner。 |
| `rekit/lib/B3.Commands.ps1` | legacy command layer | 文本工作线和内部命令入口；冻结语义，避免新增 runtime owner。 |

## Freeze / deprecation gates

1. **Documented**：本文件列出命令和模块状态。
2. **Go-owned**：Go package tests 和 CLI tests 覆盖 deterministic 行为。
3. **Façade default**：公共 `/rekit` 默认委托 Go，并保留 `REKIT_GO_DISABLE=1` fallback。
4. **Release invariant**：Go release invariant 锁定 checklist、边界、known gaps 或 deprecation 状态。
5. **Legacy freeze**：PowerShell 只允许 bug fix / compatibility / safety boundary 修复。
6. **Fallback retirement candidate**：至少一个 release cycle 无 fallback 需求，且旧 case smoke、doctor、facade smoke 通过。
7. **Removal batch**：删除必须是单独批次，含恢复计划、diff review、CHANGELOG、docs、tests 和用户明确授权。

## 禁止迁移清单

以下内容不得作为普通 Go-first 收口批次顺手迁移：

- actual full-trace/debug/inject/patch/dump/network/heavy-tool 执行。
- authority/confirmed 自动写入。
- policy schema 迁移或历史 case state 自动改写。
- 外部服务发布、真实扫描、fuzz、exploit replay、设备连接、hook。
- 将 runtime 逻辑复制进 case-local shim。
