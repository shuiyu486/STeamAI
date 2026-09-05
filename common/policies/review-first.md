# Review-first writes and learning feedback

覆盖、删除、移动、外部发布或向 canonical pack 回流经验前，必须先生成可审查事实，再取得与 exact action 绑定的用户确认。

## 通用规则

- review 展示目标、范围、完整 diff、收益、风险、冲突和推荐动作。
- “继续”“好”或对另一动作的确认不能扩大授权。
- preview 不是确认；source/target bytes 漂移后旧确认失效。
- push、发布、远程 API 写入和不可逆本地动作都必须单独确认。

## Learning feedback

1. Commander 只从 `accepted` finding/review 提炼脱敏 learning candidate。
2. Reviewer 检查证据支持、跨 case 通用性、反例、重复、冲突、目标路径、脱敏，以及 `mechanical→V1`、`analysis-method→V2`、`behavioral→V3` 的最低 maturity；eligibility、maturity、batch acceptance 与用户 confirmation 不能互相替代。
3. 将同主题 eligible candidates 组成 batch；每条 candidate 保持一个 destination，一个 batch 可修改同一 selected pack 中多个现有 Markdown targets。
4. 在隔离 clone 中生成完整、标准、`--full-index` exact Git patch，并记录 base revision、全部 target pre/postimage 与 patch SHA-256。
5. Reviewer 绑定完整 batch patch；用户查看 candidates、reviews 与完整 patch 后，确认只授权这一份 exact batch。
6. Apply 前重验 patch、exact target set、base currentness 和 `git apply --check`；漂移则停止并重新生成。
7. 不自动 commit/push。当前 case 继续读取创建时的 pack snapshot，不随 source pack 更新漂移。
