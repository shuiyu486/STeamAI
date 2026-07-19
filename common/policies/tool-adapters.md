# Tool adapter policy

## 目的

定义外部 RE 工具接入的通用契约。工具可以是 IDA、x64dbg、Frida、unidbg、Unicorn、Triton、Ghidra、radare2、公开 unpack/import fixer 或项目自写脚本。

本文件只定义能力卡、风险边界和输出契约；具体工具用法写入 pack 的 `tooling/catalog.yml` 与 `tooling/recipes/*.md`。

## 工具状态

| 状态 | 含义 |
|---|---|
| `mainline-template` | 多个 case 验证有效，可作为推荐主线模板。 |
| `auxiliary` | 可用但只适合特定阶段或辅助查询。 |
| `candidate` | 值得短测，尚未充分验证。 |
| `cautious` | 有明显风险，需要确认、预算和止损。 |
| `short-test-stoploss` | 只能短测，失败即止损。 |
| `deprecated` | 不再推荐，保留历史说明。 |

## Capability card

每个工具至少应能写成以下卡片：

```yaml
id: <tool-id>
status: candidate | auxiliary | mainline-template | cautious | short-test-stoploss | deprecated
entry: <命令、MCP、脚本或外部路径占位>
purpose: <解决什么问题>
inputs:
  - <需要的输入>
outputs:
  - <输出 sidecar / summary / artifact>
side_effects:
  - none | writes-idb | debug-process | inject | patch | dump | network | filesystem
risks:
  - <反调试、卡死、大输出、误判等>
stop_conditions:
  - <何时停止>
confirmation_required: true | false
notes:
  - <可复用经验>
```

## 输出契约

工具输出默认分三层：

1. **Sidecar**：完整输出、trace、反编译、log、dump metadata 等机器或大文本文件。
2. **Summary**：短摘要，说明命令、输入、输出路径、结论和限制。
3. **Evidence ref**：可被 candidate 引用的文件定位、行范围、filter 或 hash。

禁止把完整 trace、完整 disasm、完整 decompile、完整 build log 粘进 Markdown。

工具报告建议格式（写入 summary sidecar 或 packet）：

```text
command:
output_path:
summary:
key_findings:
errors:
next_action:
```

lane executor / tool adapter 执行前可以从 Mission brief / overview / handoff / continue artifacts 的 authorized-gate line 直接取得 `eventId` 与可复制 `reportContract=/rekit gate -ExecutionReportContract -GateEventId <id> -Format json` handoff，也可以由主 Agent 在 case-local 或 authorized output workspace cwd 中调用同一 command 读取只读 contract，省略 `-Target` 时 Go runtime 会向上解析 attached case root。该 contract 投影已授权 gate 的 action、budget、output paths、allowed statuses、stop conditions、record/notify、report path rule、summary/escalation size、boundary/escalation requirements、validation failure stages/codes、`validationRepairHints[]` 与 denied actions，并提供 `liveValidation` handoff：包含 `authorizedWorkspaces[]`、`reportFileName`、`caseRelativeReportPath`、可直接填充的 bounded sidecar template、workspace-relative `validateCommand` / `recordCommand` 与 `validateArgs` / `recordArgs`、case-relative `caseRelativeValidateCommand` / `caseRelativeRecordCommand` 与 `caseRelativeValidateArgs` / `caseRelativeRecordArgs`、从 pack `tooling/catalog.yml` 结合本次 `heavyToolGates` action 投影出的 `adapterCandidates[]`、默认 `selectedAdapter`、replay behavior（重复 `RecordArgs` / `CaseRelativeRecordArgs` 返回 `duplicate eventId` 且不追加 observations）、`<executor-id>` replacement guidance 与 no-write/no-heavy-tool notes；contract 同时给出 `defaultReportPath` 作为首选 case-relative sidecar 位置。adapter 应从 `adapterCandidates[]` 中选择具体工具，把 sidecar `adapterId` 设为该 candidate id，使后续 read-only validation 与 observation evidence 能保留具体 pack tooling provenance；若没有候选，仍必须显式填写 bounded adapter id 并遵守同一授权边界。adapter 可在不重扫 ledger 或手工拼接命令的情况下做 live validation，并在执行前知道 `gate -ValidateExecutionReport` 可能返回的 stable failure taxonomy 与 repair actions；该读取不写 ledger、不执行工具、不写 authority/confirmed。

lane executor / tool adapter 完成 `authorized-gate` 覆盖的实际动作并写出 sidecar 后，主 Agent 可以在 case-local 或 authorized output workspace cwd 中先调用 `gate -ValidateExecutionReport -GateEventId <id> -ExecutionReportPath <path> -Format json` 做只读 strict validation preflight；`<path>` 可以是 case-relative、case-contained absolute，或相对当前 authorized output workspace 的 sidecar 文件名。该路径复用正式 intake 的 path/schema/action/status/gateEventId/budget/ref/boundary/escalation 校验，输出 `isMutation=false` / `applied=false` 且 `valid=true` 或 `valid=false` 的 envelope；valid/invalid envelope 都会在可用时携带 `adapterContext.candidates[]` 与 `adapterContext.selected`，让 Mission Commander 或替换 executor 确认 sidecar `adapterId` 是否对应 pack tooling catalog 中的具体 adapter；invalid sidecar 会保留 `error`、`errors[]`、stable `failureCode`、`failureStage`、`repairHints[]`、`reportPath`、可用时的 partial normalized `report` 与 contract boundaries；`repairHints[]` 给出 stable `repairAction`、相关字段、允许的 output paths / stop conditions、size limit、record blocked 与 rerun validation 标记，供 adapter 按 path/decode/schema/identity/refs/budget/boundary/summary 等阶段修正后重跑，不写 observations ledger、不执行工具、不写 authority/confirmed。

lane executor / tool adapter 完成 `authorized-gate` 覆盖的实际动作后，可以额外写一个 bounded JSON sidecar，并由主 Agent 在 case-local 或 authorized output workspace cwd 中通过 `gate -Apply -GateEventId <id> -ExecutionReportPath <path>` 记录 observation evidence；省略 `-Target` 时 Go runtime 会向上解析 attached case root，`<path>` 可以是 case-relative、case-contained absolute，或相对当前 authorized output workspace 的 sidecar 文件名。该 sidecar 必须留在 case 内、位于本次 authorized gate 的 output paths 下，且只包含 refs/summary/bounded escalation，不包含完整 trace/dump/log：

```json
{
  "schemaVersion": 1,
  "kind": "adapter-execution-report",
  "adapterId": "<adapter-id>",
  "action": "<manifest-heavyToolGates.id>",
  "status": "succeeded|failed|boundary-hit|escalated|aborted",
  "gateEventId": "<authorized-gate-event-id>",
  "actualBudget": {"runtimeSeconds": 0, "diskMB": 0, "requests": 0},
  "outputRefs": ["<case-relative output under authorized outputPaths>"],
  "evidenceRefs": ["<case-relative bounded evidence ref under authorized outputPaths>"],
  "boundaryHits": ["<authorized stopCondition token>"],
  "escalation": "<short bounded escalation reason when status/budget requires it>",
  "summary": "<short bounded summary>"
}
```

Go runtime 可先只读校验该 report，也可在正式记录时读取并校验该 report 后写 observation evidence；重复提交同一 sidecar 会返回 `duplicate eventId` 并保持 observations ledger 不重复 append：`schemaVersion`、`kind`、`action`、`status`、`gateEventId`、actual budget、case-relative refs、authorized output path 边界、boundary token 与 bounded escalation 都会 fail-closed 校验；`outputRefs`、`evidenceRefs` 与 report path 都必须落在本次 authorized gate 的 `outputPaths` 下；`boundaryHits` 必须来自本次 authorized gate 的 `stopConditions`，不能把 lane profile 中未被该 gate request 采用的 stop condition 写入 sidecar 或 explicit evidence flags；`failed` / `boundary-hit` / `escalated` / `aborted` sidecar 必须包含 bounded `summary`；`boundary-hit` / `escalated` status 或 actual budget 超出授权预算时，sidecar 本身必须包含 `boundaryHits` 或 `escalation` marker，不能只靠外部命令参数补齐；正式 evidence 会在 `execution.adapterContext` 中嵌入匹配到的 concrete tooling candidate（若存在），并在 `execution.adapter` 中保留 sidecar 原文 provenance；它仍不执行 heavy-tool、不写 authority/confirmed。

能用脚本统计的不手工复制长表；需要 LLM 判断时先抽取小样本、摘要或 bounded diff。

## 重型工具门禁与预授权 autonomy

以下动作必须有明确原因、预算、输出位置、止损条件和授权来源：

- 动态调试、attach、进程注入、hook。
- patch 字节、修改 IDB 共享状态、dump 进程内存。
- full trace、长时间符号执行、大规模反编译导出。
- 网络访问、扫描、请求回放、exploit replay、上传、发布或安装外部组件。

lane 文档和 task packet 只能表达授权意图，不能单独构成 heavy-action grant。若没有本次具体动作的显式用户确认，确定性执行依据必须是 strict validated `.rekit/lanes/<lane>/autonomy.json` 加覆盖 action、exact target、typed budget、stop conditions、output paths、record/notify 边界的 `authorized-gate` decision；超出范围、出现新风险或需要 confirmed/authority/promote 时必须升级。

每个 pack 必须在 `manifest.yml` 的 `heavyToolGates` 中声明可申请的 heavy action。manifest 的静态 `requiresConfirmation=true` 表示该 action 必须经过 gate，不是 ledger 中的最终确认结果；Go `gate -WhatIf/-Apply` 只接受清单里的 action，将 manifest 的 `defaultRisk` 和 `stopConditions` 带入 preview / request decision，并根据 autonomy preflight 动态设置 `gate.requiresConfirmation`：`pending-gate=true`，`authorized-gate=false`。用户覆盖 `-Risk` 时必须使用 `medium`、`high` 或 `critical` 小写 scalar，覆盖 `-StopConditions` 时必须使用小写 slug/snake token 列表。`gate -Apply` 只写 `pending-gate` 或 `authorized-gate` request decision，不执行实际 heavy action；实际执行由 lane executor / tool adapter 在当前授权边界内完成，并写回 evidence/ledger。

门禁记录建议：

```yaml
heavy_action: debug | inject | patch | dump | full-trace | symex | network
risk: medium | high | critical
decision_reason: <为什么轻路径无法闭合>
tried_light_steps:
  - <已尝试动作>
budget:
  runtime_s: <估计>
  disk_mb: <估计>
outputs:
  - <sidecar path>
stop_conditions:
  - <lowercase-slug-or_snake-token>
status: pending-gate | authorized-gate
requires_user_confirmation: true | false
```

## Adapter 设计原则

- 先 recipe 化，再 adapter 化；不要一开始做复杂统一接口。
- Adapter 不应把工具输出直接灌入主上下文，应返回 packet 或 sidecar refs。
- Adapter 失败必须返回结构化失败原因和下一步，而不是无界重试。
- Adapter 不应绕过 pack policy、write boundary 或 review-first gate。
- 可替换性优先：同一能力应允许多个工具竞争，不能把单个工具变成 framework 硬依赖。
