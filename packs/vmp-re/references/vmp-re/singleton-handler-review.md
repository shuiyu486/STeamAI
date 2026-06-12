# Singleton handler 复核流程

> 适用场景：1-count / low-count opcode-like unknown。目标是避免 alias-heavy value-flow 污染 confirmed CSV。

## 核心原则

- 1-count candidate 默认不自动合入。
- formula fit 只是线索，不是证据。
- 以 focused instruction trace 中的 final VSP payload write 为准。
- early staged write 若被 later write 覆盖，语义记录 final effect，并在 notes 中说明 staged/overwritten。
- no-stable-VSP-payload handler 只能作为 bridge/control role。

## 复核步骤

1. 找候选来源：
   - `routine_ir.events.csv` top unknown
   - 最新 `auto_accept_recommendations_round*.csv`
   - 最新 `handler_shape_clusters_round*.csv`
2. 汇总已有 evidence：
   - `handler_value_flow_auto_*round*.csv`
   - `auto_*round*_instr.json`
   - `.fits.csv`
3. 检查 value-flow：
   - old VSP reads
   - new VSP writes
   - VSP delta
   - bytecode reads
   - write widths
4. 看 instruction trace：
   - 哪些写入是真实 VM stack payload
   - 哪些只是 native `rsp` scratch/junk
   - 是否有 later overwrite
   - 最后一次对 final `newVSP+offset` 的写入来源
5. 分类：opcode semantics / source layout / bridge role / keep unknown。

## 阻塞标志

遇到以下任一情况，默认不合入 opcode CSV，除非 instruction trace 能消歧：

```text
LOW_OCCURRENCE
ADD_XOR_OR_ALIAS
MANY_EXACT_FORMULAS
POINTER_ALIAS
SOURCE_POINTER_ALIAS
```

## 常见分类

| 现象 | 分类 |
|---|---|
| final `newVSP+0`/`+offset` 有稳定 payload，来源可解释 | opcode/source semantics |
| 只有 key/VIP/VSP/control/native stack-scratch 变化，无 stable VSP payload | bridge role |
| 多次写同一 slot，最终覆盖 early arithmetic | 记录 final effect，notes 说明 early staged write |
| push 多个 qword，来源是 stack/source path，closed-form 不清晰 | `SOURCE_*_PUSH*` conservative semantics |
| handler 末尾进入 VMExit-style bridge 且 payload 不稳定 | 优先 role 或 keep unknown |

## 命令模板

已有 round evidence 时，优先用脚本汇总，不重新 trace。若需要新跑：

```powershell
python "<workdir>\auto_mine_handler_semantics.py" `
  --context-index <N> `
  --handler 0xHANDLER `
  --run-trace `
  --run-extract `
  --out-prefix "<workdir>\captures\auto_semantics_candidates_manual_0xHANDLER"
```

写入 confirmed CSV 后：

```powershell
python "<workdir>\build_routine_ir.py"
python "<workdir>\mine_routine_superinstructions.py"
```

## CSV notes 模板

```text
RoundXX manual instruction review of singleton: <关键 RIP> load/write 描述；<final slot> is <effect>. Only 1 sample, so confidence kept medium_low and semantics remains conservative.
```

## 接受标准

- `vm_opcode_semantics_confirmed.csv`：必须能说明 final VSP payload。
- `vm_handler_roles_confirmed.csv`：必须能说明无 stable VSP payload、主要为 control/key/dispatch/VMExit bridge。
- 不能确定：保持 unknown，并在 `task-handoff.md` 留下下一步。
