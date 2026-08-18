# VMP handler review subagent overlay

Extends: `common/policies/subagents.md`

## 适用任务

子 agent 适合只读复核固定分片：

- batch handler / trace / value-flow 复核。
- tooling diff 审查和候选经验归纳。
- 对疑似 opcode semantics 的独立反驳。

不适合让子 agent 自行无界扫描整批 captures、大 CSV 或所有 handler。

## 推荐分工

- 主 agent：维护 handler 台账、exclude 列表、CSV 写入、backup、验证和 `task-handoff.md` 更新。
- 子 agent：只读复核固定 handler 分片，返回结构化短结论，不修改文件。
- 反驳 agent：对疑似 opcode semantics 的候选专门尝试反驳；证据不足则建议 deferred。

## 分片粒度

| handler 类型 | 每个子 agent 处理量 | 说明 |
|---|---:|---|
| no-stable-VSP-payload / role-like | 6-10 个 | 指未观察到稳定最终 VSP payload；不等于没有临时写入。 |
| alias-heavy / pointer-heavy / 有 VSP 写入 | 3-5 个 | 需要 instruction-level 复核，避免输出过大。 |
| 疑似可写 opcode semantics | 1-3 个 | 必须说明 final VSP payload、输入/输出槽位和覆盖关系。 |

默认总并行度 2-3。高风险时用 1-2 个分片 agent + 1 个反驳 agent，总并行度不超过 3。

## VMP 分层复核

| 层级 | 输入 | 允许动作 | 禁止 / 升级条件 |
|---|---|---|---|
| L1 handler packet review | 主 agent 脚本抽取的短 packet：handler、occurrences、shape、recommendation、blocking_reasons、alt-reg `new_vsp_write_offsets`、top fits | 只基于 packet 初筛 `accept_role | accept_opcode | defer | needs_l2 | needs_l3` | 不读取完整 trace / instr / memory JSON；不调用 IDA / debugger |
| L2 focused sidecar review | 指定 focused batch sidecars 与少量行范围 | 只读 `focused_runs.csv`、`handler_shape_clusters.csv`、`auto_accept_recommendations.csv`、`handler_value_flow_*.summary.csv`、必要的少量 instruction 定位 | 不重跑 trace；不打开 rebuilt PE；证据不足返回 `defer` 或 `needs_l3` |
| L3 static/deep review | 单 handler / 窄地址范围 / 明确 blocker | 可用 IDA/idalib/x64dbg 等重型工具；优先后台运行 | 不做无界 full binary survey；不在普通 batch 前台 `idalib_open` 全 PE 并 `run_auto_analysis=true` |

L3 只用于少数高价值 blocker，例如：高频 handler、value-flow 与 instruction trace 冲突、handler 边界/静态结构必须确认、或该 blocker 阻止后续批量自动化。若没有可用 IDB/session，L3 agent 应返回 `needs_ida_session` 或由主 agent 显式后台启动，不要阻塞普通 batch。

## Route id / trigger / planner hints

manifest route：`binary-re:bounded-review`。

触发条件：

- focused batch、top unknown、trace/value-flow 或 tooling diff 需要批量只读复核。
- 候选数量 `>= 4`，或需要 instruction-level review。
- 出现 `LOW_OCCURRENCE`、`POINTER_ALIAS`、`SOURCE_POINTER_ALIAS`、`NO_TEMPLATE_MATCH` 等需要独立复核的 blocker。
- 主会话只需要短结论，长 instruction/memory/value-flow 证据应留在子 agent 上下文。

默认 planner 参数：

- `shardBasis=handler`
- `targetItemsPerAgent=4`
- `maxParallel=3`
- `subagentPermissions=read-only`
- `mainAgentOwns=csv-backup,csv-write,validation,handoff-update`

不满足固定分片边界时，先用脚本聚合/缩小输入，不启动无界子 agent。

## 输出契约

每个 handler 返回：

```text
item: <handler RVA>
handler: <handler RVA>
decision: accept_role | accept_opcode | reject | defer | needs_l2 | needs_l3
confidence: high | medium | low
evidence: 最多 3 条关键指令或文件定位
risk: 主要风险或空
next_action: write-role | write-semantics | focused-review | deep-review | no-op
tier_used: L1 | L2 | L3
tool_scope: packet_only | sidecar_only | static_narrow | deep_tool
csv_ready: yes | no
row: 可写时给完整字段；不可写时留空
defer_reason:
```

禁止把长 instruction dump、完整 JSON/CSV 或大段 disassembly 带回主会话。

## 合并规则

- 只有主 agent 写 `vm_opcode_semantics_confirmed.csv` / `vm_handler_roles_confirmed.csv`。
- 子 agent 分歧时，默认 deferred。
- 任何 1-count / alias-heavy semantics 建议，都必须通过独立反驳或主 agent focused review。
- 写入后按 `verification.overlay.md` 重建 routine IR、superinstruction 和 unknown 统计。
