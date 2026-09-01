# Function behavior review recipe

## 目标

只读复核已存在的 disassembly、decompiler、API behavior、format finding 或 saved debug/trace summary sidecar，用于验证 binary behavior finding；本 recipe 不执行样本、不调试、不 trace、不 dump、不 patch、不写 rename/comment 或 analysis database。

## 输入与输出

输入必须是授权 case 内的脱敏 binary/function/API alias、bounded sidecar 和 evidence ref。输出包括：行为摘要、precondition、input/output shape、side effects、evidence refs、confidence、limitations 和 open questions；owner 将其写入 evidence/finding，Reviewer 独立复核。

## 需要升级时

如果需要动态执行、debug、trace、dump、patch、bulk decompile、rename/comment、外部联网或 database writeback，先向用户展示 exact action/target、轻路径为何不足、隔离、运行/磁盘/request 预算、case-relative output、rollback 与 stop conditions。只有用户确认该具体动作且 Claude Code 工具权限允许时才能执行；任一边界变化都必须停止并重新确认。

## 禁止

- 不主动执行样本或修改 binary、IDB、database 和共享状态。
- 不把样本、hash、IOC、dump、trace、memory snapshot、patch bytes、完整函数体、符号表、客户上下文、token 或绝对路径写入 pack。
- 证据不足时返回 `needs-evidence` 或 open question，不把推测升级为 accepted finding。
