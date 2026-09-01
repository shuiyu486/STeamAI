# `{{LEARNING_ID}}` — `{{TITLE}}`

- Kind：`{{KIND}}`
- Applies to：`{{GENERAL_SCOPE}}`
- Source findings/reviews：`{{CASE_LOCAL_REFS}}`
- Confidence：`{{CONFIDENCE}}`
- Proposed destination：`{{PACK_RELATIVE_PATH}}`
- Source pack snapshot：`{{PACK_SNAPSHOT}}`
- Review state：`candidate`

## 可复用经验

`{{GENERALIZED_LESSON}}`

## 适用条件与反例

`{{APPLICABILITY_AND_COUNTEREXAMPLES}}`

## 脱敏检查

- [ ] 不含真实目标身份、客户信息或凭据
- [ ] 不含 artifact、绝对路径、case-local hash/address 或会话标识
- [ ] 不含未验证猜测或 case 流水账
- [ ] 与现有 pack 内容完成去重和冲突检查
- [ ] Reviewer 已检查证据与跨 case 通用性
- [ ] Proposed destination 是 selected pack 下已跟踪的 Markdown 文件
- [ ] exact patch 在隔离 Git clone 中生成，并通过 `git apply --check`
- [ ] canonical target 当前 blob 仍匹配 patch base blob
- [ ] 用户已查看完整 exact patch 并确认回流

用户确认前不得写入共享 pack；确认后若 target base 漂移，必须重新生成并展示 patch。
