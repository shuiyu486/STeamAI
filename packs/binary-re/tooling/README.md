# binary-re tooling

本目录同时承载通用 binary/function/API passive analysis tooling、compiled static triage，以及对已有 IDA TSV export 的 bounded inspection。VMProtect trace/devirtualization 当前仅保留 recipe/template，不是已启用 producer；case 私有目标与 raw output 始终留在 project-local artifact/sidecar。

## 内容分层

| 路径 | 用途 |
|---|---|
| `catalog.yml` | capability、状态、输入输出、side effect 和止损。 |
| `recipes/static-binary-triage.md` | passive metadata/section/import/string sidecar。 |
| `recipes/function-behavior-review.md` | saved function/API summary 的只读复核。 |
| 其余 `recipes/*.md` | VMP/IDA、trace、value-flow、lane 协同与公开工具短测。 |
| `scripts/README.md` | 已验证、值得模板化的脚本接口与参数化要求。 |
| `patches/*.md` | 第三方工具补丁/适配经验。 |
| `candidates/` | `/steamai promote` 生成的 review-first tooling 候选。 |

## 可进入 pack 的内容

- passive sidecar schema、bounded query、tool capability 与 stop conditions。
- 通用 trace/context/value-flow/routine IR 流程，以及 VMP/IDA 专项 recipe。
- 可参数化接口、输入输出、预算、隔离和失败特征。

## 不得进入 pack 的内容

- 样本、客户信息、hash、IOC、RVA/VA/seed/context/coverage 等 case-specific 数据。
- 完整二进制、function body、symbol table、dump、trace、patch、probe 成品或 raw tool output。
- 本机绝对路径；使用 `<caseRoot>`、`<toolsRoot>`、`<target.exe>` 等占位。

catalog 只描述能力，不是命令解释器。actual heavy action 仍要求 strict durable profile、fresh `authorized-gate` 和 executor observation。
