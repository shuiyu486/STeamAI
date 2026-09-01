# Common policies

本目录保存可跨 pack 复用的薄核心规则。它是按需路由索引，不是默认必读清单；case 从与 selected pack 相同的 exact revision 物化完整 `common/**`，以保持简单、自包含且不需要产品级 closure 解析器。

| Policy | 何时读取 |
|---|---|
| `agent-team.md` | 组队、成员协作、纠偏与 durable member 边界。 |
| `subagents.md` | 使用短命 tactical subagent。 |
| `tool-adapters.md` | 选择外部工具或准备 heavy action。 |
| `context-budget.md` | 控制上下文和大文件读取。 |
| `review-first.md` | 覆盖、删除、发布或经验回流前。 |
| `write-boundaries.md` | 判断文件写入与外部副作用。 |
| `verification.md` | 判断本次改动是否完成。 |
| `evidence.md` | 记录 artifact、evidence、finding 和 review。 |

维护原则：

- common 只放跨 pack 规则，领域细节放 `packs/<pack>/`。
- 不在 common 中建立 session、消息、任务或授权状态机。
- 可复用经验只从 accepted finding/review 提炼，经 Reviewer 审查和用户确认 exact patch 后回流。
- 不写真实样本、trace/dump/capture、payload、凭据、客户信息、绝对 case 路径或 case 进度。
