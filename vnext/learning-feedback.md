# vNext 经验回流闭环

本流程把有 current accepted review 的研究结论提炼为跨 case 经验，但不把 case 内容、会话状态或通用控制面带入 pack。一个候选只修改 selected pack 中一个现有、已跟踪、regular、非 symlink 的 Markdown；manifest、YAML/JSON、脚本、runtime、adapter 或 common policy 修改必须作为单独 canonical maintenance change，不借 learning feedback 扩大范围。

## 1. Candidate eligibility

若 project-local Commander 当前不能访问 canonical source clone，先向用户询问其路径，并让用户以 `--add-dir <CANONICAL_SOURCE_CLONE>` 恢复或重新进入同一 Commander 上下文；验证该目录是预期 canonical repository 后再继续。clone path 只作为本次访问范围，不写入 case 状态，不自动搜索、安装或复制 source clone。

Commander 创建 immutable `learnings/candidates/L-*.md` 前必须证明：

- finding 引用至少一项 evidence；
- 每项 evidence 记录 artifact alias、case-relative path、SHA-256、bytes 与 authorized use；该 tuple 同时匹配当前 artifact index 的同 alias entry 和实际 case-local artifact bytes；
- supplied source review 明确针对该 finding，最后完整 round 为 `accepted`，且 finding/evidence SHA-256 与传递 artifact bindings 仍 current；
- candidate 记录 source finding/review 的 case-relative path 与 SHA-256；
- candidate 绑定 selected pack、full source revision、pack tree、common tree 与完整 snapshot digest；
- snapshot digest 重算后仍匹配；
- Proposed destination 同时匹配 case snapshot manifest 与 canonical base revision manifest 的 `learningTargets`，并唯一解析到 selected pack 内一个现有、tracked、regular、非 symlink Markdown；
- generalized lesson 和适用条件/反例正文不命中 selected pack `denyPatterns`；
- 删除真实目标、客户、artifact、hash、地址、绝对路径、凭据、session/task identity 和 case 流水账。

candidate 创建后保持 byte-for-byte immutable。candidate exact file SHA-256 在创建后由 learning review、用户确认 envelope 和 Apply currentness 外部记录；candidate 文件不得包含自身 exact SHA 字段。缺少任一资格时返回 `needs-evidence`、`disputed`、`superseded` 或 `ineligible`，不继续。

## 2. Reviewer eligibility checkpoint

Reviewer 只读 candidate、source finding/review/evidence、case snapshot 和 proposed destination，写 `reviews/R-L-*.md` 的 Checkpoint A。必须判断：

- evidence 是否足以支持泛化；
- 是否跨 case 通用，以及反例/不适用条件是否完整；
- 与目标内容是否重复或冲突；
- candidate 是否脱敏且未命中 denyPatterns；
- target 是否属于 selected pack 当前 `learningTargets`。

只有 `Eligibility decision: eligible` 才能生成 proposal patch。`needs-evidence` 返回原 finding owner；`disputed`、`superseded`、`ineligible` 不生成 proposal。

## 3. 隔离生成无权威 exact proposal patch

用户确认前不得编辑 canonical source pack。Commander 从 canonical source clone 记录 full base revision、manifest base blob 和 target base blob，然后把该 exact revision 克隆到 case 外的临时目录，只编辑已通过 eligibility 的 Markdown target：

```text
git rev-parse HEAD
git rev-parse HEAD:packs/<PACK>/manifest.yml
git rev-parse HEAD:<PACK_TARGET>
git diff --binary --full-index --no-ext-diff -- <PACK_TARGET>
```

完整 patch 保存到 `.steamai-vnext/learnings/patches/L-*.patch`，记录 SHA-256。用 `git apply --numstat -z` 证明恰好一个 target；拒绝 create/delete/rename/copy/mode change、binary patch、`/dev/null`、路径逃逸或非 Markdown target。只扫描 patch 新增文本行执行 `denyPatterns`，不扫描 diff header 和 Git identities。再在第二个同 base 的干净 clone 中执行：

```text
git apply --check <PATCH_PATH>
```

该 patch 在 Reviewer Checkpoint B 接受前没有权威，不能申请用户确认或写 canonical source。禁止截断 diff、自定义摘要代替 patch、fuzzy apply、三方合并或旧 promote/writeback 状态机。

## 4. Reviewer exact-patch checkpoint

同一 Reviewer 在 `R-L-*.md` Checkpoint B 记录并核对：

- candidate SHA-256；
- target、base revision、manifest base blob、target base blob；
- patch path 与 patch SHA-256；
- target count 固定为 1；
- added-lines deny result；
- `git apply --check` result；
- candidate lesson 与完整 patch 的对应关系。

只有 Checkpoint A 为 `eligible` 且 Checkpoint B 的 Patch decision 为 `accepted`，才可向用户申请确认。Reviewer 不修改 candidate/patch/source finding。

## 5. 用户确认 envelope

Commander 一次性展示并绑定：candidate path/SHA、final learning review path/SHA、source finding/review path/SHA、selected pack、snapshot digest、target、base revision、manifest base blob、target base blob、patch path/SHA 和完整未截断 patch。用户确认只授权该 exact tuple；普通确认、旧确认或跨会话消息不能替代。

完整 patch 无法无截断展示时，拆小 candidate 或让用户检查完整 case-local patch 文件；不能只展示摘要。确认前 canonical source pack 必须 byte-for-byte 不变。

## 6. Apply 前完整漂移检查

按以下顺序 fail-closed；任一失败都不继续、不 retry 覆盖，并重新生成、审查、展示：

1. case snapshot payload digest current，`snapshot.yml` 的完整排序 file manifest 与实际 `packs/**`、`common/**` 路径集合完全一致；任何内容漂移、缺失或未声明新增文件都停止；
2. source finding/review SHA current，并解析 source review 最后完整 accepted round；
3. 该 round 列出的全部 evidence path/SHA current；每项 evidence 的 artifact alias/path/SHA-256/bytes/authorized-use tuple 与当前 artifact index 同 alias entry 及实际 artifact bytes 一致；artifact index、finding、source review、evidence、snapshot metadata、snapshot payload 与实际 artifact 从 case root 到最终文件的全部 ancestors 都必须留在 case 内，且非 symlink/reparse；
4. candidate SHA current；
5. learning review SHA current；
6. patch SHA current；
7. canonical `HEAD` 等于 reviewed base revision；
8. manifest filter-aware blob 等于 reviewed manifest base blob；
9. current manifest 仍允许唯一 target，denyPatterns 未改变；
10. target 及所有 ancestors 仍在 pack root 内且非 symlink/reparse；
11. target staged/unstaged content 与 reviewed base 一致，filter-aware target blob 等于 reviewed target base blob；
12. patch 仍为单一 existing Markdown target，无 create/delete/rename/copy/mode/binary/path traversal；
13. `git apply --check <PATCH_PATH>` 通过。

重验通过后才执行：

```text
git apply <PATCH_PATH>
```

应用不自动 commit/push，也不重新导出或更新当前 case snapshot。

## 7. Snapshot 不漂移

case 建立时从同一 exact revision 物化 selected pack 全树与完整 `common/**`。`snapshot.yml` 记录 full revision、pack tree、common tree、完整排序 file manifest，以及 materialized payload digest；digest 覆盖每个 repo-relative path、Git mode/blob、bytes 和 SHA-256，不包含 metadata 文件自身。成员所有 pack/common 指令只从 snapshot 解析。

回流只改变 canonical source pack。运行中 case 不自动 sync、不重导出 snapshot；应用 patch 后当前 case 的完整 payload digest 必须保持旧值。只有后续 case 明确选择包含该 patch 的新 revision 并生成新 digest，才消费新经验。

## 清理

生成或验证 patch 的临时 clone 在流程结束后删除。case-local candidate/review/patch 是可复核材料；不得把临时 clone 路径、session identity 或原始 case artifact 写入 pack。`denyPatterns` 是 fail-closed tripwire，不替代 Reviewer 的人工脱敏判断。
