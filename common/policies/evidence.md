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

## Review

Reviewer 直接引用 finding/evidence，给出 `accepted`、`needs-evidence`、`disputed` 或 `superseded`。Reviewer 不修改原 evidence/finding；`needs-evidence` 返回原 owner 补证。

只有 `accepted` finding/review 才能成为 learning candidate 的来源。经验回流还必须通过证据支持、跨 case 通用性、重复、冲突和脱敏审查；用户查看并确认完整 exact Git patch 前，canonical pack 零写。
