# vmp-re policy overlays

本目录保存 VMP RE pack 对 `common/policies/` 的领域化补充。通用规范不在这里复制全文；这里只写 VMProtect trace-based devirtualization 场景中的具体执行规则。

## Overlay 列表

| Overlay | Extends | 用途 |
|---|---|---|
| `subagents.overlay.md` | `subagents` | handler / trace / value-flow 复核的子 agent 分片、输出契约和合并规则。 |
| `lane-collaboration.overlay.md` | `lane-collaboration` | B3 lane 下主线与功能支线协同推进、续接、standalone 和 VMP request/candidate 边界。 |
| `context-budget.overlay.md` | `context-budget` | captures、routine IR、trace、归档文档的读取预算和禁止模式。 |
| `review-first.overlay.md` | `review-first` | VMP RE 经验回流、confirmed CSV 写入、tooling 候选的 review-first 约束。 |
| `verification.overlay.md` | `verification` | confirmed CSV、routine IR、superinstruction 和 handoff 的验证要求。 |

## 维护原则

- common policy 改进先进入 `common/policies/*` 的 review-first 候选。
- VMP-specific 经验进入本目录 overlay 或 `references/vmp-re/*.md`。
- 当前 round、handler 地址、ctx、coverage、artifact 路径只留在 case handoff 或机器数据中，不进入 overlay。
