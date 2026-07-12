# Unpack PE tooling

本目录保存 unpack-pe pack 的工具 catalog、recipes、脚本接口和候选工具经验。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具 capability card、状态、sidecar、预算和止损条件。 |
| `recipes/*.md` | 按任务阶段记录工具用法。 |
| `candidates/` | 从 case 回流的候选工具经验。 |

## 原则

- 工具经验先 recipe 化，再考虑 adapter 化。
- 不硬编码本机路径；使用 `<caseRoot>`、`<toolsRoot>`、`<sampleRef>`、`<sidecar>`。
- 不保存样本、hash、unpacked binary、dump、trace、memory snapshot、patch、完整 import table、section bytes、IOC、客户上下文或绝对路径。
- 动态调试、样本执行、dump、patch、解密/解压 payload、外部联网、自动修复 import 和写 unpacked 文件默认是 gated action。
