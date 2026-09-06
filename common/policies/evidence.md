# Evidence、finding 与 review

目标是让研究结论可追溯、可复核，并明确区分观察、推断和未证明部分。

## Artifact index

artifact 保持 case-local。索引只记录 alias、相对路径、SHA-256、bytes、来源和授权范围；不把真实 artifact、dump、trace、capture、凭据或客户信息复制到 pack。

## Evidence

evidence 记录：

- 可重复的方法与观察；
- artifact 引用和精确定位；
- 限制、反例、样本量与不确定性；
- 必要的完整性信息。

大段原始输出不进入主会话或 Markdown；只保存最短摘要与定位。来源不足、低样本或存在歧义时必须降低 confidence。

## Finding

finding 必须引用 evidence，并写明 owner、可选 verifier、claim、confidence、重要反证和尚未证明部分。猜测不能伪装成事实；出现冲突时先补证或保留 disputed 状态。

## 复合结论

仅当问题跨越实质性处理、请求或状态边界时，才补充连接证明；普通索引和单点检查不强制增加格式。

- 每个影响结论的连接都说明两端的对象/版本/状态、实际传递或转换关系、适用条件与 E 定位。同名、时间接近、局部各自正确或同源导出一致，不证明连接或独立性。
- 在查看新的区分性观察之前记录预测、反证和条件，结果到来后保留原预测并说明支持、反驳、仍不可区分或未执行。事后解释不能称作预测验证；受阻也不等于解释已被推翻。
- 综合 F 直接引用全部支撑连接的 E；局部 F 仅供导航，不假设 F→F 递归依赖已被验证。两个局部 accepted 不等于整体 accepted，整体 review 必须覆盖综合 claim 与相关 E。
- 缺关键连接时保留局部结论，综合结论限定为 unknown/needs-evidence；反证成立就撤回或收窄相应连接。沿用已有 F/E/artifact currentness 与复审，不新增图数据库、任务账本或后台失效传播。

## Review

Reviewer 直接引用 finding/evidence，给出 `accepted`、`needs-evidence`、`disputed` 或 `superseded`。Reviewer 不修改原 evidence/finding；`needs-evidence` 返回原 owner 补证。

只有 `accepted` finding/review 才能成为 learning candidate 的来源；它只证明 V0 Reviewed，不证明可重复、比较改善或 field 效果。V1/V2/V3/V4 分别需要 deterministic、replay、calibrated comparative、multiple opt-in field outcome 的 exact evidence chain，不能互相替代。经验回流还必须通过证据支持、跨 case 通用性、重复、冲突和脱敏审查；用户查看并确认完整 exact Git patch 前，canonical pack 零写。
