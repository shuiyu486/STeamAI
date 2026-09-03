# STeamAI 经验批次回流

本流程只把有 current accepted review 与 current evidence chain 的、已脱敏且跨 case 通用的经验回流到本机 canonical working tree。case 不被扫描或汇总；未确认 candidate 始终留在原 case。一个 case 可产生任意多个 immutable candidate；candidate 是逐条审查单位，不是数量上限。

## 1. Candidate eligibility

每个 candidate 只提出 selected pack 内一个现有、tracked、regular、非 symlink/reparse 的 Markdown destination。创建 candidate 前必须证明：

- source finding 引用至少一项 evidence；supplied source review 的最后完整连续 round 为 current `accepted`；
- finding、source review 和全部 evidence 的 exact SHA-256 current；
- 每项 evidence 的 artifact alias、case-relative path、SHA-256、bytes 与 authorized use 同时匹配 artifact index，并与实际 artifact bytes 一致；
- case snapshot 的完整排序 file manifest、实际 `packs/**`/`common/**` path set 与 payload digest current；
- candidate 绑定 source finding/review path 与 SHA、selected pack、full source revision、pack/common tree、snapshot digest 和 Proposed destination；
- Proposed destination 同时匹配 case snapshot 与 canonical current manifest 的 `learningTargets`；
- generalized lesson、适用条件与反例不命中 `denyPatterns`，且不包含真实目标、客户、artifact、hash/address、绝对路径、凭据、session/task identity 或 case 流水账。

candidate 创建后 byte-for-byte immutable；其 exact file SHA 由 review、batch 与 Apply 外部绑定，candidate 文件不得包含自身 exact SHA 字段。Reviewer 按 `learning-review.md` 逐条检查证据、通用性、反例、去重/冲突、脱敏和 destination，只写 eligibility。只有 `Decision: eligible` 才可进入 batch；candidate review 不绑定或授权 patch。

## 2. Thematic exact batch

Commander 将主题相近的 eligible candidates 组成一个或多个可完整阅读的 batch。一个 batch 可以包含多个 candidate，并修改同一 selected pack 中多个现有 Markdown targets；每个 candidate 仍只对应自己的单一 destination，且每个 target 至少由一个 candidate 支持。不同 selected pack 不得进入同一 batch。

用户确认前不得编辑 canonical source pack。Commander 基于 canonical working-tree 当前 bytes 在 case 外的隔离 clone 生成一个完整标准 Git patch，并保存到 `.steamai-vnext/learnings/patches/LB-*.patch`：

```text
git diff --binary --full-index --no-ext-diff -- <SORTED_TARGETS...>
git apply --numstat -z <PATCH_PATH>
git apply --check <PATCH_PATH>
```

patch 只允许修改 exact target set；拒绝 create/delete/rename/copy/mode change、binary patch、`/dev/null`、路径逃逸、未跟踪文件、symlink/reparse、非 Markdown 或不匹配 `learningTargets` 的 target。只扫描新增文本行执行 `denyPatterns`；tripwire 不替代 Reviewer 人工脱敏。

## 3. Batch review

Reviewer 按 `learning-batch-review.md` 写唯一 batch 级审查文件，完整绑定：

- selected pack、case revision、canonical HEAD 和 snapshot digest；
- 排序 candidate path/SHA、eligibility review path/SHA 与 destination；
- 排序 target path、canonical working-tree Preimage SHA-256/bytes 和 patch Postimage SHA-256/bytes；
- patch path/SHA、added-lines deny result 和 `git apply --check` result；
- 最终 `Decision: accepted`。

Reviewer 必须阅读完整未截断 patch并核对 candidate-to-target mapping、主题一致性、重复、冲突、反例和脱敏。batch review 不建立 registry、inbox、Hub 或跨 case 索引。

## 4. Zero-write preview 与用户确认

Commander 从 case 根把严格 JSON request 写入 `steamai __learning-batch-preview` stdin：

```json
{"candidateReviews":[{"candidate":"learnings/candidates/L-001.md","review":"reviews/R-L-001.md"}],"patch":"learnings/patches/LB-001.patch","batchReview":"reviews/R-LB-001.md"}
```

原生入口重算并展示 candidate/review/source chain、snapshot、manifest、canonical HEAD、target pre/postimage、batch review、patch SHA 与完整 patch。preview 时 canonical pack 必须零写。只有用户在当前 Commander 窗口输入：

```text
CONFIRM STEAMAI LEARNING BATCH <batch_identity>
```

才把同一 JSON request 写入 `steamai __learning-batch-apply --confirmation "<完整确认串>"` stdin。普通确认、旧确认、截断 identity 或跨会话消息不能替代。

## 5. Apply currentness 与 rollback

Apply 从磁盘完整重建 preview，不信任旧内存。以下任一漂移都 fail-closed：case snapshot/source chain、candidate/reviews/patch bytes、canonical HEAD、manifest policy、target set/preimages、path safety 或 `git apply --check`。通过后执行 `git apply <PATCH_PATH>`，验证全部 postimages；失败时仅把本 batch targets 恢复为 exact preimages。

Apply 前后必须证明 HEAD、index、当前 case snapshot 必须不变，并显式重验 snapshot digest。应用只修改 canonical working-tree targets，不自动 `git add`、commit、push，也不更新已有 case snapshot。多个 batch 顺序 preview/确认/apply；后一批自然基于前一批的新 working tree。已确认并应用的本机 working-tree 经验可立即进入后续 Fresh；Git history/push 只在用户另行明确授权后发生。

## 清理

生成/验证 patch 的临时 clone 在流程结束后删除。case-local candidate、eligibility review、batch review 和 patch 是可复核材料；不得把临时路径、session identity、真实 artifact 或 case 细节写入 pack。禁止截断 diff、自定义摘要代替 patch、fuzzy apply、三方合并、旧 promote/writeback 状态机和任何自动跨 case 搜索/聚合。
