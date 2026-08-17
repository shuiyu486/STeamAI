# Common policies

## 读取指南

本目录是 STeamAI repository 的跨 pack 规范注册中心。它保存通用工作原则；具体 pack 只写 overlay，不复制通用规范全文。本文件是 policy 索引，不是默认必读清单；先通过 `docs/context-routing.md` 或 pack `references/<pack>/README.md` 定位具体需求，再只读对应 policy 顶部或相关小节。

## 分层

| 层级 | 路径 | 用途 |
|---|---|---|
| Common policy | `common/policies/*.md` | 跨 pack 通用规则，例如 Agent Team、子 agent、工作线协同、工具 adapter、上下文预算、写入确认、验证标准。 |
| Pack overlay | `packs/<pack>/policies/*.overlay.md` | 某个 pack 对通用规则的领域化补充。 |
| Case reference | `packs/<pack>/references/**` | 任务路由、领域 workflow、接手入口，不承载所有横切规范。 |

## Policy 列表

| Policy | 用途 |
|---|---|
| `agent-team.md` | Agent Team 角色、packet、状态流和人工确认边界。 |
| `tool-adapters.md` | 外部工具能力卡、adapter 输出契约和重型工具门禁。 |
| `context-budget.md` | 上下文预算与渐进式披露。 |
| `subagents.md` | 子 agent 分片、只读复核和输出契约。 |
| `lane-collaboration.md` | 工作线协同、续接和写入权限。 |
| `review-first.md` | 写入前审查、用户确认和回流边界。 |
| `write-boundaries.md` | 写入边界和外部副作用。 |
| `verification.md` | 验证与完成标准。 |
| `evidence.md` | 证据、引用和结论质量。 |
| `tool-output.md` | 工具输出与大产物处理。 |
| `handoff.md` | 接手和会话连续性。 |

## 生命周期

- `sync`：默认引用 common policy；是否下发由 pack overlay 决定。
- `promote`：common policy 候选必须 review-first，不直接从 case 整文覆盖。
- `doctor`：后续应校验 policy manifest、policy 文件存在、预算和 overlay 继承关系。

## 维护原则

- common policy 不写 pack-specific 术语，例如具体 handler、opcode、产品名、路径或 trace 文件。
- pack-specific 细节放到 `packs/<pack>/policies/*.overlay.md`。
- 发现可复用经验时先分类：common policy、pack overlay、reference doc、tooling recipe、case-only handoff。
