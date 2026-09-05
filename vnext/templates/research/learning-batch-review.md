# `{{LEARNING_BATCH_REVIEW_ID}}` — `{{BATCH_TITLE}}`

- Reviewer 单写者：`{{REVIEWER}}`
- Selected pack：`{{PACK_NAME}}`
- Case revision：`{{PACK_REVISION}}`
- Canonical HEAD：`{{CANONICAL_HEAD}}`
- Snapshot digest：`{{SNAPSHOT_DIGEST}}`
- Patch：`{{PATCH_REF}}`
- Patch SHA-256：`{{PATCH_SHA256}}`
- Added-lines deny result：`{{ADDED_LINES_DENY_RESULT}}`
- `git apply --check` result：`{{GIT_APPLY_CHECK_RESULT}}`
- Candidate mapping/theme：`{{CANDIDATE_MAPPING_THEME_RESULT}}`
- Dedup/conflict/counterexamples：`{{DEDUP_CONFLICT_COUNTEREXAMPLES_RESULT}}`
- Redaction：`{{REDACTION_RESULT}}`
- Calibration attestation：`{{CALIBRATION_ATTESTATION_REF_OR_NONE}}`
- Calibration attestation SHA-256：`{{CALIBRATION_ATTESTATION_SHA256_OR_NONE}}`
- Promotion attestation：`{{PROMOTION_ATTESTATION_REF_OR_NONE}}`
- Promotion attestation SHA-256：`{{PROMOTION_ATTESTATION_SHA256_OR_NONE}}`
- Run bundle manifest：`{{RUN_BUNDLE_MANIFEST_REF_OR_NONE}}`
- Run bundle manifest SHA-256：`{{RUN_BUNDLE_MANIFEST_SHA256_OR_NONE}}`
- Run bundle identity：`{{RUN_BUNDLE_IDENTITY_OR_NONE}}`
- Run bundle reveal SHA-256：`{{RUN_BUNDLE_REVEAL_SHA256_OR_NONE}}`
- Evaluated patch SHA-256：`{{EVALUATED_PATCH_SHA256_OR_NONE}}`

## Candidates

按 candidate path 排序；每项必须有独立且 current 的 `eligible` review。

{{CANDIDATE_BINDINGS}}

`CANDIDATE_BINDINGS` 必须替换为连续的实际记录；每项依次填写 Candidate、Candidate SHA-256、Claim kind、Required maturity、Eligibility review、Eligibility review SHA-256 与 Destination，不保留示例或占位符。

## Targets

按 target path 排序；preimage 绑定 canonical working-tree 当前 bytes，postimage 绑定完整 patch 应用后的 exact bytes。

{{TARGET_BINDINGS}}

`TARGET_BINDINGS` 必须替换为连续的实际记录；每项依次填写 Target、Preimage SHA-256/bytes 与 Postimage SHA-256/bytes，不保留示例或占位符。

## Final decision

- Decision：`{{BATCH_DECISION}}`

只有 `accepted` 才能请求用户确认。Reviewer 必须完整阅读 candidate、eligibility review 和未截断 patch，确认每个 candidate 只映射到自己的 destination、所有 targets 都由至少一个 eligible candidate 支持、batch 主题一致、无重复/冲突/敏感内容，且 patch 不 create/delete/rename/copy/change mode。

若全部 candidate 的最高门槛低于 V3，上方四组 verified-learning 字段必须填 `none`；若任一 candidate 为 `behavioral`/`V3`，则不得填 `none`，必须绑定 current `go` calibration、`pass`/`improved`/`V3` promotion、完整 candidate run bundle。promotion 必须绑定 Reviewer 在解盲前一次读取 manifest-bound immutable `blind-review.json` 后形成的 blind decision（packet path/SHA、preferred entry/output SHA），并将 `Run bundle reveal` path/SHA exact 绑定为该 run manifest 同目录的 `reveal.json`；本 review 的 `Run bundle reveal SHA-256` 必须与之相同，且 `Evaluated patch SHA-256` 必须等于最终 `Patch SHA-256`。任一 binding 漂移后重新生成 batch review，不修改旧字段伪装 current。
