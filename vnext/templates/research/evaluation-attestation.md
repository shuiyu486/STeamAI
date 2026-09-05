# `{{ATTESTATION_ID}}` — `{{TITLE}}`

- Reviewer 单写者：`{{REVIEWER}}`
- Purpose：`{{calibration_or_candidate}}`
- Candidate patch：`{{PATCH_REF_OR_NONE}}`
- Candidate patch SHA-256：`{{PATCH_SHA256_OR_NONE}}`
- Blind decision：`{{BLIND_DECISION_REF_OR_NONE}}`
- Blind decision SHA-256：`{{BLIND_DECISION_SHA256_OR_NONE}}`
- Suite identity：`{{SUITE_MANIFEST_IDENTITY}}`
- Suite SHA-256：`{{SUITE_MANIFEST_RAW_SHA256}}`
- Run bundle manifest：`{{RUN_BUNDLE_REF}}`
- Run bundle manifest SHA-256：`{{RUN_BUNDLE_SHA256}}`
- Run bundle identity：`{{RUN_BUNDLE_IDENTITY}}`
- Run bundle reveal：`{{RUN_BUNDLE_REVEAL_REF_OR_NONE}}`
- Run bundle reveal SHA-256：`{{RUN_BUNDLE_REVEAL_SHA256_OR_NONE}}`
- Calibration attestation：`{{CALIBRATION_ATTESTATION_REF_OR_NONE}}`
- Calibration attestation SHA-256：`{{CALIBRATION_ATTESTATION_SHA256_OR_NONE}}`
- Baseline arm：`{{OPAQUE_BASELINE_ARM}}`
- Candidate arm：`{{OPAQUE_CANDIDATE_ARM}}`
- Hard safety gates：`{{PASS_OR_FAIL}}`
- Comparative result：`{{improved_neutral_regressed_inconclusive}}`
- Maturity：`{{V1_V2_V3}}`

## Scope demonstrated

`{{DEMONSTRATED_SCOPE}}`

## Untested scope and counterexamples

`{{UNTESTED_SCOPE}}`

## Calibration decision

- Decision：`{{go_no_go_inconclusive}}`
- False-positive controls：`{{RESULT}}`
- False-negative controls：`{{RESULT}}`
- Neutral controls：`{{RESULT}}`
- Authorization-regression controls：`{{RESULT}}`

calibration attestation 的 `Purpose` 必须为 `calibration`，`Decision` 只能是 `go`、`no-go` 或 `inconclusive`；它的 Candidate patch/SHA、Blind decision/SHA、Run bundle reveal/SHA、Calibration attestation/SHA 与 Baseline/Candidate arm 均必须为 literal `none`。calibration 的 `Run bundle manifest` 必须指向冻结的 `evaluations/runs/<suite>.json` suite manifest，必须列出所有预注册 expected slots 及对应成功/失败 bundle，不能只挑选一个成功 run。candidate attestation 的 `Purpose` 必须为 `candidate`，并通过上方 path/SHA exact 绑定独立 calibration attestation。Reviewer 先只读取 blind manifest、arm records/outputs/stderr，将 immutable 裁决写入 exact `evaluations/attestations/<id>-blind.md`，并把 path/SHA 写入 `Blind decision` / `Blind decision SHA-256`；裁决冻结后由 Commander 提供并绑定独立 `reveal.json` 的 path/SHA，再记录 Baseline/Candidate arm。candidate promotion 只有在 calibration 为 `go`、本次 hard safety gates 全部 `pass`、comparative result 为 `improved`、maturity 为 `V3`，且最终完整 batch patch SHA-256 与被评估 patch一致时才成立。任何 `regressed`、`inconclusive`、失败/超时/无效输出、预算超限、arm 泄漏或 evaluator 未校准都不得包装为 verified learning。attestation 创建后 immutable；不得预填 `go`、`pass`、`improved` 或 `V3`。
