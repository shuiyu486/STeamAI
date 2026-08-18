# 当前任务接手文档

> 本文件是新会话接手当前 VMProtect devirtualization 项目的第一入口。每轮推进后必须更新本文件。

## Snapshot

- 日期：<YYYY-MM-DD>
- 项目名：<PROJECT_NAME>
- 工作目录：`<PROJECT_ROOT>`
- 基准 PE：`<rebuilt-or-target-pe>`
- 主 VMEnter：`RVA 0x...` / `VA 0x...`
- 代表长 trace：`captures/<trace>.csv`
- 当前全量 known events：`<known>/<total>`
- 当前主 context known events：`<known>/<total>`

## 当前任务状态

| Task | 状态 | 说明 |
|---:|---|---|
| `#1` | in_progress | 建立或继续 VM handler semantics lowering。 |

## 已完成的最新工作

- <填写最近一轮已合入 opcode/source/role semantics。>
- <说明低样本结论的保守性。>

## 当前待复核 top unknown

| Handler | 优先级 | 当前提示 |
|---|---:|---|
| `0x...` | high | <为什么优先处理。> |

## 子 agent 分片计划

| 字段 | 当前值 |
|---|---|
| route | `binary-re:bounded-review` 或 `none` |
| shard basis | `handler` / `trace` / `file` |
| max parallel | `<n>` |
| main agent owns | `csv-backup,csv-write,validation,handoff-update` |
| deferred policy | 分歧或证据不足默认 deferred |

待分片对象：

```text
<handler-or-item-list>
```

## 下一步 checklist

1. 不要回到裸调试；优先离线 focused trace/value-flow。
2. 优先使用已有 round sidecar；没有 evidence 再新跑 focused trace。
3. 对每个 candidate 检查：
   - old/new VSP reads/writes
   - final write 是否覆盖 early staged write
   - bytecode/key-step 是否只是 bridge/control
   - 是否存在 alias blockers
4. 只有指令级确认 final payload 后才写 `vm_opcode_semantics_confirmed.csv`。
5. no-stable-VSP-payload 只写 `vm_handler_roles_confirmed.csv`。
6. 写入 confirmed CSV 后重建 routine IR。
7. 更新本文件的 coverage、已合入 handler、剩余列表。

## 验证命令

```powershell
python -m py_compile "<workdir>\auto_mine_handler_semantics.py" "<workdir>\build_routine_ir.py" "<workdir>\mine_routine_superinstructions.py"
```

## 维护规则

- 每轮任务结束前更新本文件。
- 不把完整 CSV/trace/反汇编复制到本文件。
- 若本文件超过 12KB，归档历史，只保留当前状态。
- 可复用经验分别写入：
  - `workflow-template.md`
  - `toolchain-router.md`
  - `progressive-disclosure.md`
  - `singleton-handler-review.md`
