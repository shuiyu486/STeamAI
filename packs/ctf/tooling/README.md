# CTF tooling

本目录保存 ctf pack 的工具 catalog、recipes、脚本接口和候选工具经验。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | 工具 capability card、状态、sidecar、预算和止损条件。 |
| `recipes/*.md` | 按任务阶段记录工具用法。 |
| `candidates/` | 从 case 回流的候选工具经验。 |

## 原则

- 工具经验先 recipe 化，再考虑 adapter 化。
- 不硬编码本机路径；使用 `<caseRoot>`、`<toolsRoot>`、`<challengeRef>`、`<sidecar>`。
- 不保存 flag、远程靶场地址、账号凭据、payload、solver 私有脚本、challenge 原始文件、pcap、dump、trace 或比赛私有细节。
- 远程连接、bruteforce、fuzz、exploit replay、高流量请求、debug、dump、patch 和外部网络动作默认是 gated action。
