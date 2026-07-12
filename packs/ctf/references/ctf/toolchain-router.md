# CTF toolchain router

## 工具状态

| 状态 | 含义 |
|---|---|
| `mainline-template` | 多个 challenge / case 验证有效，可作为推荐主线模板。 |
| `auxiliary` | 可用但只适合特定阶段或辅助查询。 |
| `candidate` | 值得短测，尚未充分验证。 |
| `cautious` | 有明显外部副作用或数据风险，需要确认、预算、隔离和止损。 |
| `short-test-stoploss` | 只能短测，失败即止损。 |
| `deprecated` | 不再推荐，保留历史说明。 |

## 按任务路由

| 任务 | 首选工具/方式 | 备用/辅助 | 注意事项 |
|---|---|---|---|
| scope / challenge inventory | case-local aliases、题目描述摘要 | category checklist | 不把 flag、远程地址或比赛私有上下文写入 pack。 |
| artifact triage | metadata / strings / schema / file summary sidecar | static tooling summary | 原始二进制、pcap、dump、payload 留 case-local。 |
| solver hypothesis | constraint notes、local repro summary | small solver sidecar | 不把完整 solver 或 flag 粘入模板。 |
| writeup review | writeup draft sidecar | reviewer checklist | 清理 flag、payload 和比赛私有细节后再考虑 promote。 |
| remote/gated action | pending-gate request | local harness first | 远程连接、高流量、bruteforce、fuzz、exploit replay 必须 gate。 |
| tooling adapter | capability card + dry-run | candidate recipe | 工具先 recipe 化，不做硬依赖。 |

## 重型工具门禁

```yaml
heavy_action: remote-connect | bruteforce | fuzz | exploit-replay | high-rate-requests | debug | dump | patch | network
scope: <challenge aliases and authorization summary>
decision_reason: <why passive/local path failed>
tried_light_steps:
  - <step>
budget:
  requests: <estimate>
  runtime_s: <estimate>
  inputs: <max corpus or payload count>
  rate_limit: <limit>
outputs:
  - <case-local sidecar path>
stop_conditions:
  - <stop condition>
requires_user_confirmation: true
```

## 维护规则

- 新工具先进入 `candidate` 或 `cautious`。
- 短测必须有 timeout、请求量/输入量上限、速率限制、输出大小上限和止损条件。
- 不保存 flag、远程靶场地址、payload、solver 私有脚本、challenge 原始文件、pcap、dump、trace 或比赛私有细节到 pack。
- 工具成为 mainline-template 前至少经过多个授权 challenge 或稳定靶场复现验证。
