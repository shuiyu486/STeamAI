# `{{LEARNING_REVIEW_ID}}` — Review of `{{LEARNING_ID}}`

- Reviewer 单写者：`{{REVIEWER}}`
- Candidate：`{{CANDIDATE_REF}}`
- Candidate SHA-256：`{{CANDIDATE_SHA256}}`
- Claim kind：`{{CLAIM_KIND}}`
- Required maturity：`{{REQUIRED_MATURITY}}`
- Source finding：`{{SOURCE_FINDING_REF}}`
- Source finding SHA-256：`{{SOURCE_FINDING_SHA256}}`
- Source accepted review：`{{SOURCE_REVIEW_REF}}`
- Source review SHA-256：`{{SOURCE_REVIEW_SHA256}}`
- Selected pack：`{{PACK_NAME}}`
- Source revision：`{{PACK_REVISION}}`
- Pack tree：`{{PACK_TREE}}`
- Common tree：`{{COMMON_TREE}}`
- Snapshot digest：`{{SNAPSHOT_DIGEST}}`
- Proposed destination：`{{PACK_RELATIVE_PATH}}`

## Checkpoint A — Eligibility

- Decision：`{{ELIGIBILITY_DECISION}}`
- Evidence/generalization：`{{EVIDENCE_GENERALIZATION_RESULT}}`
- Applicability/counterexamples：`{{APPLICABILITY_COUNTEREXAMPLES_RESULT}}`
- Dedup/conflict：`{{DEDUP_CONFLICT_RESULT}}`
- Redaction/denyPatterns：`{{REDACTION_DENY_RESULT}}`
- Target allowlist/currentness：`{{TARGET_CURRENTNESS_RESULT}}`

`Selected pack`/`Pack tree` 指主包；`Snapshot digest` 绑定包括可选辅助包的完整 pinned 输入。辅助方法只供参考，辅助或主辅混合写回不合法。若 candidate 借鉴辅助方法，Reviewer 在现有 Evidence/generalization、Applicability/counterexamples 与 Target allowlist/currentness 检查中确认 current accepted evidence chain、主包适用性及自包含性：未来单主包 case 不依赖隐藏辅助路径或步骤。抄录辅助内容不能替代证据；这些是 Reviewer 语义判断，不是自动机械保障。

只有 `eligible` 才能进入 thematic learning batch。Reviewer 必须确认 `Claim kind`/`Required maturity` 与 candidate exact 一致，并符合 `mechanical→V1`、`analysis-method→V2`、`behavioral→V3`；`eligible` 本身不证明已达到该成熟度。`needs-evidence` 返回原 finding owner；`disputed`、`superseded` 或 `ineligible` 不进入 batch。本文件只判断 candidate eligibility；candidate review 不绑定或授权任何 patch，也不接受任何 patch；最终 exact patch 由独立的 learning batch review 绑定。candidate、source evidence 或 snapshot 任一漂移都使本 review stale。
