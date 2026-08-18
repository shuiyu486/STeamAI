# Focused handler review

## 目标

对低样本、singleton、alias-heavy handler 做指令级复核，避免把 overwritten staged write 或 bridge/control handler 误写为 opcode semantics。

## 流程

```text
top unknown handler
  -> focused instruction trace
  -> occurrence summary
  -> final VSP payload write 判断
  -> value-flow extraction
  -> formula/source/bridge proposal
  -> manual review
  -> confirmed opcode CSV 或 handler role CSV
  -> rebuild routine IR
```

## 判断规则

- 以 final payload write 为准，不以 early staged write 为准。
- no-stable-VSP-payload handler 只降为 bridge/control/VMExit role。
- 1-count / alias-heavy / pointer alias 不允许只凭 formula 自动合入。
- 如果 final write 被 later write 覆盖，记录 final effect 而不是中间 effect。

## 输出

- 只把结论摘要写入 reference/task handoff。
- 具体 trace、occurrence、value-flow 大表保留机器文件。
