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

## 输出契约

每个 handler 返回：

```text
handler:
decision: role | semantics | defer
confidence: high | medium | low
evidence: 最多 3 条关键指令或文件定位
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
