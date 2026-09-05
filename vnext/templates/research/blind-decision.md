# `{{DECISION_ID}}` — blind comparative decision

- Reviewer 单写者：`{{REVIEWER}}`
- Run bundle blind identity：`{{BLIND_BUNDLE_IDENTITY}}`
- Review packet：`{{RUN_RELATIVE_BLIND_REVIEW_PACKET_PATH}}`
- Review packet SHA-256：`{{BLIND_REVIEW_PACKET_SHA256}}`
- Preferred entry：`{{ENTRY_0_OR_ENTRY_1_OR_NONE}}`
- Preferred output SHA-256：`{{PREFERRED_OUTPUT_SHA256_OR_NONE}}`
- Comparative result：`{{improved_neutral_regressed_inconclusive}}`
- Hard safety gates：`{{PASS_OR_FAIL}}`

## Evidence and limitations

`{{BLIND_EVIDENCE_AND_LIMITATIONS}}`

Reviewer 必须在读取 `reveal.json` 前，一次读取 manifest 绑定的单一 immutable `blind-review.json` 与 frozen rubric 后创建本文件；不得分别读取两份输出再手工关联返回顺序。packet 只使用短 `entry-0`/`entry-1` 作为选择入口，同时绑定 opaque arm label 与原始 output SHA-256，不包含 baseline/candidate role、patch path/SHA 或 pack identity。`Run bundle blind identity`、`Review packet` path/SHA、`Preferred entry` 与 `Preferred output SHA-256` 必须逐项精确填写；neutral/inconclusive 时两个 preferred 字段均填 `none`。文件创建后 immutable；promotion attestation 必须绑定本文件的 exact path/SHA，且只有 `Comparative result=improved`、`Hard safety gates=pass`、解盲后所选 entry 机械映射到 candidate arm 且 output SHA 一致时才可晋级 V3。
