# CTF Agent Team routes

## 1. 角色边界

- `main`：确认 challenge 范围、比赛/靶场边界、远程连接限制、case-local sidecar 位置和 gate 状态；合并 reviewer verdict，写 ledger / handoff，并在确认后更新 authority 文档。
- `challenge-analysis`：在自己的 workspace 中分析单个 challenge、artifact、hypothesis、solver candidate 或 writeup candidate，产出 observation / request / candidate。
- `reviewer`：只读复核 bounded solution candidates、evidence notes、solver snippets、writeup drafts 或 tooling notes，输出 verdict，不访问远程服务、不 brute force、不写文件。
- `tooling`：描述工具能力、输入输出、sidecar、预算、隔离要求和止损；默认不执行远程或高流量动作。

## 2. 默认 routes

| route | 适用任务 | 分片 | 权限 | 输出 |
|---|---|---|---|---|
| `ctf:bounded-review` | solution / evidence / writeup / exploit / tooling review | challenge-or-solution | read-only | reviewer verdict |
| `ctf:challenge-analysis` | pwn / web / crypto / rev / forensics / misc challenge 分析 | challenge-or-category | read-only-or-workspace-only | observation / request / candidate |

`plan-subagents` 只生成 review packet 与 observability，不自动 spawn agent。主会话负责启动 Agent 工具、收集输出，并用 `/rekit note` 写回 verification / decision。

## 3. Packet 输出契约

所有子 agent 输出都必须包含：

```text
item, decision, confidence, evidence, risk, next_action, tier_used, tool_scope, defer_reason
```

CTF route 可追加：

```text
challenge_ref, category, artifact_ref, flag_status, solver_ref, candidate_path
```

`challenge_ref`、`artifact_ref` 与 `solver_ref` 应是 case-local 脱敏引用或 sidecar id，不是 flag、远程地址、完整 payload、私有 solver 路径或绝对路径。`decision` 是 reviewer output decision，不等同于 ledger canonical decision；main 合并后再写 `/rekit note -Kind verification` 与 `/rekit note -Kind decision`。

## 4. Review-first 门禁

- accepted solution / solver / writeup candidate 只能进入 main 合并队列，不能直接写 confirmed / report / authority。
- 证据不足时使用 `defer` 或 `needs-more-evidence`，并给出下一步轻量验证。
- 需要远程连接、bruteforce、fuzz、exploit replay、高流量请求、debug、dump、patch 或外部副作用时，先登记 pending-gate request。
- 每个 shard 的失败只影响本 shard；不要阻塞无关 challenge、category 或 solver candidate。

## 5. 证据与 sidecar

- evidence 应引用 case-local sidecar 路径、challenge alias、artifact id、solver ref、工具名称、时间窗口和脱敏 row id。
- 不在 pack reference 中保存 flag、远程靶场地址、账号凭据、payload、challenge 原始文件、pcap、dump、trace 或比赛私有上下文。
- 任何可复用经验进入 pack 前必须清理 flag、payload、远程地址、solver 私有细节、比赛名和 challenge 特定解法。
