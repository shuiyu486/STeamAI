# Generic binary RE tooling

本目录保存 generic-binary-re pack 的工具 catalog、recipes、脚本接口和候选工具经验。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具 capability card、状态、sidecar、预算和止损条件。 |
| `recipes/*.md` | 按任务阶段记录工具用法。 |
| `candidates/` | 从 case 回流的候选工具经验。 |

## 原则

- 工具经验先 recipe 化，再考虑 adapter 化。
- 不硬编码本机路径；使用 `<caseRoot>`、`<toolsRoot>`、`<binaryRef>`、`<functionRef>`、`<sidecar>`。
- 不保存样本、hash、完整二进制、dump、trace、memory snapshot、patch、完整函数体、符号表、IOC、客户上下文或绝对路径。
- 动态执行、调试、trace、dump、patch、批量反编译、自动重命名/注释、外部联网和写回分析数据库默认是 gated action。
