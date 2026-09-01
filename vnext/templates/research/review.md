# `{{REVIEW_ID}}` — Review of `{{FINDING_ID}}`

- Reviewer 单写者：`{{REVIEWER}}`
- Finding owner：`{{FINDING_OWNER}}`

同一 Reviewer 对补证后的同一 finding 只在文件末尾追加完整 round，不覆盖历史 round。若更换 Reviewer，新 Reviewer 创建新的 review 文件。

## Review round `{{REVIEW_ROUND}}`

- Previous round：`{{PREVIOUS_REVIEW_ROUND_OR_NONE}}`
- Reviewer：`{{REVIEWER}}`
- Finding：`{{FINDING_REF}}`
- Finding SHA-256：`{{FINDING_SHA256}}`
- Decision：`{{DECISION}}`
- Confidence：`{{CONFIDENCE}}`

### 判断

`{{REVIEW_SUMMARY}}`

### 检查的证据

`{{REVIEWED_EVIDENCE_REFS_WITH_SHA256}}`

### 风险或缺口

`{{RISKS_OR_GAPS}}`

### 下一步

`{{NEXT_ACTION}}`

当前有效判断只取最后一个字段完整、round 连续且 finding/evidence SHA-256 仍匹配当前文件的 round。发生补证或任一被审查文件变化后，旧 `accepted` 变为 stale，必须追加复审。Reviewer 不直接修改原 finding；`needs-evidence` 返回 finding owner。
