# Web-security tooling

本目录保存 Web/API 安全 pack 的工具 catalog、recipes、脚本接口和候选工具经验。

## 内容

| 路径 | 用途 |
|---|---|
| `catalog.yml` | fixed compiled-in adapter capability card、状态、sidecar、预算和止损条件；`entry` 只作 provenance，不执行。 |
| `recipes/*.md` | 按任务阶段记录工具用法。 |
| `schemas/openapi-inventory-v1.schema.json` | OpenAPI 3 JSON typed inventory 的 strict output schema。 |
| `schemas/bounded-replay-request-v1.schema.json` | content-addressed、secret-free、exact-loopback replay request schema。 |
| `schemas/bounded-replay-result-v1.schema.json` | terminal delivery + digest-only diff result schema。 |
| `candidates/` | 从 case 回流的候选工具经验；不等于 production adapter。 |

## 原则

- 当前 production vertical slice 的 fixed adapter 只包括 `openapi-v3-json-inventory` 与 `bounded-http-replay`；其它工具经验先 recipe/candidate 化，不进入动态 plugin registry。manifest 的 `maturity: mature` 与 adapter/fixture/verifier/instruction 四要素 admission 分开验证，不能用 catalog 条目存在性替代 consumption receipt。
- 不硬编码本机路径；使用 case-relative binding。Replay request 只允许 `inputs/bounded-replay-requests/<request-sha256>.json`。
- 不保存真实目标、凭据、token、cookie、HAR、pcap、scan output、response body 或漏洞 payload；认证只允许 `authRef` 环境引用。
- Inventory 不联网；replay 只允许 exact loopback/injected transport、one request、zero redirects/retries、no ambient proxy 与 digest-only result。
- 主动扫描、fuzz、exploit replay、登录尝试和数据导出仍是 gated action，且不由这两个 fixed adapter 执行。
