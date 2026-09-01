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

本 round 只能由 review 文件指定的 Reviewer 追加。round 编号必须连续，Previous round 必须指向直接前一轮；字段不完整、finding/evidence SHA-256 不再匹配，或任一 evidence 绑定的 artifact tuple 与当前 index/bytes 不一致时，不能作为 current `accepted`。
