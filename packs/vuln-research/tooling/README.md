# Vulnerability research tooling

本目录保存 vuln-research pack 的工具 catalog、recipes、脚本接口和候选工具经验。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具 capability card、状态、sidecar、预算和止损条件。 |
| `recipes/*.md` | 按任务阶段记录工具用法。 |
| `candidates/` | 从 case 回流的候选工具经验。 |

## 原则

- 工具经验先 recipe 化，再考虑 adapter 化。
- 不硬编码本机路径；使用 `<caseRoot>`、`<toolsRoot>`、`<targetRef>`、`<sidecar>`。
- 不保存真实目标、凭据、request/response、PoC payload、crash/core/minidump、pcap、trace、scan output 或漏洞利用细节。
- 主动扫描、fuzz、exploit replay、真实目标复现、debug、dump、patch 和数据导出默认是 gated action。
