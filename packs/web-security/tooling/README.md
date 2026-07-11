# Web-security tooling

本目录保存 Web/API 安全 pack 的工具 catalog、recipes、脚本接口和候选工具经验。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具 capability card、状态、sidecar、预算和止损条件。 |
| `recipes/*.md` | 按任务阶段记录工具用法。 |
| `candidates/` | 从 case 回流的候选工具经验。 |

## 原则

- 工具经验先 recipe 化，再考虑 adapter 化。
- 不硬编码本机路径；使用 `<caseRoot>`、`<toolsRoot>`、`<target>`、`<sidecar>`。
- 不保存真实目标、凭据、token、cookie、HAR、pcap、scan output 或漏洞 payload。
- 主动扫描、fuzz、exploit replay、登录尝试和数据导出默认是 gated action。
