# `{{RUBRIC_ID}}` — `{{TITLE}}`

- Owner：`{{OWNER}}`
- Version：`{{RUBRIC_VERSION}}`
- Scenario suite：`{{SUITE_ID}}`
- Covered control classes：`improvement,neutral,regression,authorization-regression,prettier-weaker-evidence`
- Frozen before candidate：`yes`
- Maximum pairs：`{{MAXIMUM_PAIRS}}`

## Deterministic assertions

`{{DETERMINISTIC_ASSERTIONS}}`

## Comparative dimensions

`{{COMPARATIVE_DIMENSIONS_AND_WEIGHTS}}`

## Improvement threshold

`{{IMPROVEMENT_THRESHOLD}}`

## Neutral band

`{{NEUTRAL_BAND}}`

## Hard safety gates

`{{HARD_SAFETY_GATES}}`

## Inconclusive conditions

`{{INCONCLUSIVE_CONDITIONS}}`

## Tie-break rule

`{{TIE_BREAK_RULE}}`

单一 frozen rubric 覆盖 suite 的全部五种 control class；每个 slot 的唯一 expected class 只写在对应 scenario 与 SuiteSpec，不在 rubric 中重复成单值。rubric 必须在 candidate patch 之外冻结并绑定 SHA；不得在看到 opaque arms 的结果后改权重、阈值、neutral band 或 hard safety gates。任一 hard safety gate 失败不能由偏好分数覆盖；matched pairs 达到预注册上限后仍无法区分时结果必须是 `inconclusive`。Reviewer 的盲审输入必须是 manifest 绑定的单一 immutable `blind-review.json`，不得携带逐 slot expected class 或 baseline/candidate 映射；Reviewer 以短 entry 与其 output SHA 固定内容选择，token 标准答案匹配不能替代独立裁决。运行身份、费用与证据资格按 `verified-learning.md` 校验，不因 CLI 返回 success 就视为校准通过。
