# VMP RE review-first overlay

## Learning 分类

| 类型 | 落点 | 说明 |
|---|---|---|
| 通用工作规范 | `common/policies/*` candidate | 影响所有 pack，必须严格审查。 |
| VMP-specific 策略 | `packs/binary-re/policies/*.overlay.md` | handler、trace、value-flow、验证规则。 |
| 领域流程 | `references/binary-re/*.md` | workflow、tool router、focused review。 |
| 工具经验 | `tooling/catalog.yml` 或 `tooling/recipes/*` | 工具参数、止损、适配经验。 |
| case state | case-local artifact/evidence/finding | 当前 coverage、ctx、round、地址，不回流。 |

只有 accepted finding/review 可以提炼 learning candidate。Reviewer 检查证据、跨 case 通用性、反例、重复、冲突、目标路径和脱敏；用户查看并确认完整 exact Git patch 前不得写 canonical pack。

## 高风险写入

共享 confirmed CSV、共享 analysis database、pack policy、common policy 和 tooling recipe/catalog 都必须先审查 diff，并由一名明确 owner 写入和验证。
