# vmp-re tooling

本目录保存 VMProtect x64 trace-based devirtualization 的可复用工具经验、脚本模板规划、工具短测与止损规则。

目标：后续新 case 不再从零摸索工具链；case 私有状态仍留在 `cases/<case>`。

## 内容分层

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具目录、用途、状态、入口占位和止损规则。 |
| `recipes/*.md` | 按任务阶段组织的工具使用方法和注意事项。 |
| `scripts/README.md` | 已在 case 中验证、值得模板化的脚本清单与参数化要求。 |
| `patches/*.md` | 第三方工具补丁/适配经验。 |
| `candidates/` | `/rekit promote` 从 case 生成的 tooling 候选，需人工审查后合入正式文档。 |

## 什么应该进 tooling

- 通用 trace pipeline、context probe、focused trace、value-flow、routine IR 的流程。
- 公开工具短测方法、timeout、日志静音、失败特征、止损条件。
- 调试工具组合的边界，例如 ScyllaHide + 管理员 x64dbg attach。
- `ida-agent-bridge` 这类外部 bridge 的只读 packet contract、sidecar/evidence ref 规则和写入禁区。
- 可参数化脚本的接口设计、输入输出、模板化 TODO。

## 什么不应该进 tooling

- 具体样本名、客户名、授权材料。
- 具体 RVA/VA/seed/ctx/round/coverage。
- dump、trace、artifact、二进制 probe 成品。
- 本机绝对路径；使用 `<caseRoot>`、`<toolsRoot>`、`<target.exe>` 等占位。

## 从 case 回流

在 case 中运行：

```text
/rekit promote
```

`promote` 会同时处理 managed docs 和 tooling 候选。tooling 候选默认写入 `tooling/candidates/`，不会直接覆盖正式 recipe。

> 后端脚本只用于自动化、CI 或排障；日常不要手动执行。
