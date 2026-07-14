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

能用脚本统计的不手工复制长表；需要 LLM 判断时先抽取小样本、摘要或 bounded diff。

## 重型工具门禁

以下动作必须有明确原因、预算、输出位置和用户确认：

- 动态调试、attach、进程注入。
- patch 字节、修改 IDB 共享状态、dump 进程内存。
- full trace、长时间符号执行、大规模反编译导出。
- 网络访问、上传、发布或安装外部组件。

每个 pack 必须在 `manifest.yml` 的 `heavyToolGates` 中声明可申请的 heavy action。Go `gate -WhatIf/-Apply` 只接受该清单里的 action，并把 manifest 的 `defaultRisk`、`requiresConfirmation` 和 `stopConditions` 写入 preview / pending-gate request；用户覆盖 `-Risk` 时必须使用 `medium`、`high` 或 `critical` 小写 scalar，覆盖 `-StopConditions` 时必须使用小写 slug/snake token 列表。这仍然不是实际执行授权。

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
requires_user_confirmation: true
```

## Adapter 设计原则

- 先 recipe 化，再 adapter 化；不要一开始做复杂统一接口。
- Adapter 不应把工具输出直接灌入主上下文，应返回 packet 或 sidecar refs。
- Adapter 失败必须返回结构化失败原因和下一步，而不是无界重试。
- Adapter 不应绕过 pack policy、write boundary 或 review-first gate。
- 可替换性优先：同一能力应允许多个工具竞争，不能把单个工具变成 framework 硬依赖。
