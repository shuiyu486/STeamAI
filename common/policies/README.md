# Common policies

本目录是 `re-context-kits` 的跨 pack 规范注册中心。它保存通用工作原则；具体 pack 只写 overlay，不复制通用规范全文。

## 分层

| 层级 | 路径 | 用途 |
|---|---|---|
| Common policy | `common/policies/*.md` | 跨 pack 通用规则，例如子 agent、上下文预算、写入确认、验证标准。 |
| Pack overlay | `packs/<pack>/policies/*.overlay.md` | 某个 pack 对通用规则的领域化补充。 |
| Case reference | `packs/<pack>/references/**` | 任务路由、领域 workflow、接手入口，不承载所有横切规范。 |

## 生命周期

- `sync`：默认引用 common policy；是否下发由 pack overlay 决定。
- `promote`：common policy 候选必须 review-first，不直接从 case 整文覆盖。
- `doctor`：后续应校验 policy manifest、policy 文件存在、预算和 overlay 继承关系。

## 维护原则

- common policy 不写 pack-specific 术语，例如具体 handler、opcode、产品名、路径或 trace 文件。
- pack-specific 细节放到 `packs/<pack>/policies/*.overlay.md`。
- 发现可复用经验时先分类：common policy、pack overlay、reference doc、tooling recipe、case-only handoff。
