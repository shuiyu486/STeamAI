# Focused handler review

对低样本、singleton、alias-heavy handler 做窄范围 final-effect 复核。完整判断规则与合成案例见 [Singleton handler 复核流程](../../references/binary-re/singleton-handler-review.md)，不默认加载全部 VMP workflow。

## 最短路径

1. 从任务指定的 artifact/evidence 定位 exact 对象、occurrence、指令范围与入口/出口；对齐坐标和环境，不凭固定轮次文件名选“最新结果”。
2. 优先复用已有有界材料，倒查 final VSP payload write，检查 early staged write 的覆盖、部分写入、pointer/source alias 与 native stack scratch。
3. 对会改变结论的缺口选一个最小区分检查，写明预期与反证；同源不同导出不是独立证明，formula fit 不自动成为语义。
4. 给出本次 opcode/source effect、可证范围内的 bridge/control role，或 keep unknown；观察不全时不能仅因无可见 payload 就降为 bridge。
5. 用现有 E/F/R 记录观察、解释、反证与复核，通过 native session 定向消息反馈。不新增 handoff 文件，不自动合入 confirmed 表或重建派生产物。

## 新观察与输出边界

已有材料不足不意味着自动重新 trace。动态动作须确认具体目标、最小输入变化、隔离、时间/输出预算与停止条件，并取得该动作的用户确认和工具权限；本 recipe 不提供 executor 或可直接调用的 mining/rebuild 脚本。

大输出只留 case-local artifact。E 引用来源与精确范围，F 区分已证明、被否定和未知，重要结论由独立 Reviewer 复核；未完成检查及恢复上下文留现有 F 与 native session。
