# `{{DECISION_ID}}` — blind comparative decision

- Reviewer 单写者：`{{REVIEWER}}`
- Run bundle blind identity：`{{BLIND_BUNDLE_IDENTITY}}`
- Preferred arm：`{{OPAQUE_ARM_LABEL_OR_NONE}}`
- Comparative result：`{{improved_neutral_regressed_inconclusive}}`
- Hard safety gates：`{{PASS_OR_FAIL}}`

## Evidence and limitations

`{{BLIND_EVIDENCE_AND_LIMITATIONS}}`

Reviewer 必须在读取 `reveal.json` 前，仅根据 blind manifest、opaque arm records、outputs、stderr 与 frozen rubric 创建本文件。`Run bundle blind identity` 必须等于 reveal 中绑定的 blind identity；`Preferred arm` 必须是 manifest 中一个 opaque arm label，或在 neutral/inconclusive 时填 `none`。文件创建后 immutable；promotion attestation 必须绑定本文件的 exact path/SHA，且只有 `Comparative result=improved`、`Hard safety gates=pass`、解盲后 `Preferred arm` 等于 candidate arm 时才可晋级 V3。
