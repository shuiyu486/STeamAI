# `{{LEARNING_ID}}` — `{{TITLE}}`

- Kind：`{{KIND}}`
- Applies to：`{{GENERAL_SCOPE}}`
- Source finding：`{{SOURCE_FINDING_REF}}`
- Source finding SHA-256：`{{SOURCE_FINDING_SHA256}}`
- Source accepted review：`{{SOURCE_REVIEW_REF}}`
- Source review SHA-256：`{{SOURCE_REVIEW_SHA256}}`
- Confidence：`{{CONFIDENCE}}`
- Proposed destination：`{{PACK_RELATIVE_PATH}}`
- Selected pack：`{{PACK_NAME}}`
- Source revision：`{{PACK_REVISION}}`
- Pack tree：`{{PACK_TREE}}`
- Common tree：`{{COMMON_TREE}}`
- Snapshot digest：`{{SNAPSHOT_DIGEST}}`

## 可复用经验

`{{GENERALIZED_LESSON}}`

## 适用条件与反例

`{{APPLICABILITY_AND_COUNTEREXAMPLES}}`

## Eligibility 检查

- [ ] finding 引用 evidence，且 supplied source review 明确针对该 finding 并为 current `accepted`
- [ ] finding/review exact bytes 与上方 SHA-256 一致
- [ ] case-local snapshot 的 pack/common tree 与完整 payload digest current
- [ ] Proposed destination 同时匹配 case snapshot 与 canonical base revision 的 `learningTargets`
- [ ] Proposed destination 是 selected pack 内已有、tracked、regular、非 symlink 的 Markdown
- [ ] candidate 正文未命中 selected pack `denyPatterns`
- [ ] 不含真实目标身份、客户信息、凭据、artifact、绝对路径、case-local hash/address、会话标识、未验证猜测或 case 流水账
- [ ] 与现有 pack 内容完成去重、冲突、反例和不适用条件检查

candidate 创建后保持 immutable。它只证明待审查的 generalized lesson，不包含用户确认状态，也不授权生成后的 patch；Reviewer 需先完成 eligibility，再对最终 exact proposal patch 单独绑定和接受。
