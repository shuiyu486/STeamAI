# vNext 经验回流闭环

本流程把已接受研究结论提炼为跨 case 经验，但不把 case 内容、会话状态或旧 promote 控制面带入 pack。一个候选只修改 selected pack 中一个明确的、已跟踪的 Markdown 目标；需要新增文件、修改 manifest、脚本、runtime 或 adapter 时拆成单独变更，不在 VNT-03 自动扩大范围。

## 1. 提炼候选

Commander 只从 `accepted` finding/review 提炼 `learnings/candidates/L-*.md`：

- 经验必须能追溯到 finding、review 和 evidence；
- 只保留跨 case 可复用的方法、证据标准、反例或协作规则；
- 删除真实目标、客户、artifact、hash、地址、绝对路径、凭据、session ID 和 case 流水账；
- `Proposed destination` 必须是 selected pack 下已跟踪的 Markdown 文件；
- 当前 case 的 selected pack snapshot 在整个流程中保持不变。

## 2. Reviewer 审查

Reviewer 只读 candidate、source finding/review/evidence 和 proposed destination，写入普通 `reviews/R-L-*.md`。必须逐项判断：

- 证据是否足以支持泛化后的经验；
- 是否确实跨 case 通用，以及反例/不适用条件是否完整；
- 与目标文档是否重复或冲突；
- 是否通过脱敏；
- 目标路径是否属于 selected pack 的已跟踪 Markdown 内容。

只有 `accepted` 才能生成 patch。`needs-evidence` 返回原 finding owner；`disputed` 或 `superseded` 不生成 patch。

## 3. 在隔离 Git 临时仓库生成 exact patch

用户确认前不得编辑 canonical source pack。Commander 从 canonical source clone 读取：

```text
git rev-parse HEAD
git rev-parse HEAD:<PACK_TARGET>
```

分别记录 proposal 的 base revision 与 target base blob。然后把该 revision 克隆到 case 外的临时目录，只在临时 clone 中编辑目标 Markdown，并生成标准 patch：

```text
git diff --binary --full-index --no-ext-diff -- <PACK_TARGET>
```

完整输出保存到 case-local `.steamai-vnext/learnings/patches/L-*.patch`，记录 patch SHA-256，并用 `git apply --numstat -z` 证明 patch 只修改已审查的单一 target。目标必须是 selected pack 下已跟踪、非 symlink 的 Markdown 文件，且所有祖先目录都不是 symlink。再在第二个同 base 的干净临时 clone 中执行：

```text
git apply --check <PATCH_PATH>
```

patch 必须完整、可应用，且只包含已审查目标；禁止 `BoundedDiff`、截断标记、自定义摘要代替 patch、旧 `/rekit promote` 或 writeback/reconcile 状态机。若完整 patch 不能在用户界面中无截断展示，应拆小候选或让用户检查完整 case-local patch 文件，不能只展示摘要后申请确认。

## 4. 用户确认与应用

Commander 向用户展示 candidate、Reviewer decision、base revision、target base blob、patch SHA-256、目标路径和完整 exact patch。用户确认只授权这一份 patch；确认前 source pack 必须保持 byte-for-byte 不变。

确认后、应用前在 canonical source clone 重验：

```text
git hash-object --path=<PACK_TARGET> -- <PACK_TARGET>
git apply --check <PATCH_PATH>
```

filter-aware `git hash-object --path=<PACK_TARGET>` 的当前 target blob 必须等于 proposal 记录的 `HEAD:<PACK_TARGET>` base blob；不要用 `--no-filters` 混淆 Windows CRLF checkout 与真实内容漂移。当前 patch SHA-256 必须等于用户确认的 patch SHA-256，且 patch 仍只修改已审查 target。任一不匹配都 fail closed：不 fuzzy apply、不 retry、不覆盖，重新提炼或基于新目标生成并展示新 patch。重验通过后才执行：

```text
git apply <PATCH_PATH>
```

应用不自动 commit 或 push。只验证受影响 pack 目标与直接合同；提交、推送仍按用户的外部操作授权处理。

## 5. Snapshot 不漂移

case 建立时必须从 selected pack 的 exact source revision 导出 case-local 只读 snapshot，并记录 pack name、source revision 和 snapshot Git tree identity；成员所有 pack 指令读取都从该 snapshot 目录解析，不读取 mutable source pack。只记录标签而未物化或未绑定读取路径不算 snapshot。

回流只改变 source pack。运行中 case 不自动 sync、不重新导出 snapshot。应用 patch 后再次读取当前 case 仍必须得到旧内容；只有后续 case 明确选择包含该 patch 的新 source revision 并导出新 snapshot，才消费新经验。

## 清理

生成或验证 patch 的临时 clone 在流程结束后删除。case-local candidate/review/patch 是用户确认前后的可复核材料；不得把临时 clone 路径、session identity 或原始 case artifact 写入 pack。
