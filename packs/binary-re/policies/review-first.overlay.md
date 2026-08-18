# VMP RE review-first overlay

Extends: `common/policies/review-first.md`

## 经验回流分类

VMP RE 中发现的经验先分类，再决定落点：

| 类型 | 落点 | 说明 |
|---|---|---|
| 通用工作规范 | `common/policies/*` candidate | 影响所有 pack，必须严格 review。 |
| VMP-specific 策略 | `packs/binary-re/policies/*.overlay.md` | handler、trace、value-flow、验证规则。 |
| 领域流程 | `references/binary-re/*.md` | workflow、toolchain router、singleton review。 |
| 工具经验 | `tooling/catalog.yml` 或 `tooling/recipes/*` | 工具参数、止损条件、适配经验。 |
| case live state | `task-handoff.md` 或机器数据 | 当前 coverage、ctx、round、handler 地址，不回流模板。 |

## promote 规则

- 日常使用 `/steamai promote`，由 `/steamai` 先生成 review 包。
- 不要把底层 runtime 命令放入常规流程；用户只需要使用 `/steamai`。
- 用户确认具体 target、pack、文件范围和动作后，才生成候选或写回。
- 命中 case-specific deny pattern 的 managed docs 不做 whole-file apply。

## high-risk 写入

以下写入必须 review-first，并由主 agent 最终执行：

- `vm_opcode_semantics_confirmed.csv`
- `vm_handler_roles_confirmed.csv`
- pack policy overlay
- common policy
- tooling recipe / catalog
