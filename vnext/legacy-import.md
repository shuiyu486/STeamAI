# Legacy 一次性只读导入合同

本合同定义 canonical `/steamai` 如何把旧 `.steamai/` 或 `.rekit/` 项目的可证明事实导入 `.steamai-vnext/`。导入不是旧 runtime migration，也不恢复旧 session、lane 或控制状态。

## Source classification

- fresh：三个 state root 均不存在。
- current：只有 `.steamai-vnext/` 存在。
- legacy-steamai：只有旧 `.steamai/` 存在。
- legacy-rekit：只有 `.rekit/` 存在。
- conflict：`.steamai/` 与 `.rekit/` 同时存在，或 `.steamai-vnext/` 与 legacy root 同时存在但没有匹配的 completed import record。

`conflict` 必须 fail-closed；不得选择“更新”的 root，不按 mtime、session、generation 或 runtime health 猜测。

## 允许读取

只读取位于 case root 内、regular、非 symlink 的声明式文本文件。单个文件必须有有界大小，读取后在生成 preview 前重验 exact bytes。

可采用字段：

- case/project display name；
- 用户原始研究目标；
- 明确写出的授权范围；
- 明确写出的禁止事项和停止条件；
- selected pack identity；
- case-local artifact、evidence、finding、review 或 handoff 的相对路径引用。

只有来源文本对字段含义无歧义且不同 source 不冲突时才能采用。旧 runtime 推断、status projection 或命令建议不能提升为用户事实。

## 必须拒绝

- session ID、PID、endpoint、process/liveness observation；
- lane owner、generation、attempt、message delivery state；
- plan SHA、publication stamp、receipt、lease、gate；
- authority、confirmed、authorized-gate 或 heavy-action 授权；
- transcript、聊天历史或模型思考过程；
- 绝对外部路径、跨 case 引用、真实凭据或无法安全脱敏的内容；
- 未知、retired 或无法在 canonical source revision 中精确定位的 pack；
- 任何要求执行旧 executable、script、Apply action、sync、promote、repair 或 migration command 才能得出的字段。

## Preview

导入前生成零写入 preview，至少完整展示：

- source kind 与 legacy root 名称；
- 每个采用字段的 source relative path；
- 被拒绝字段及理由；
- `needs-user-input` 项；
- selected pack name、exact source revision、pack snapshot tree 与同 revision 的 common policy tree；
- 计划创建的 `.steamai-vnext/` 路径；
- `.steamai-vnext/contracts/` 中 `learning-feedback.md`、`legacy-import.md` 与 `templates/**` 的全部 exact writes；每项绑定 canonical source relative path、SHA-256 和 bytes，目标冲突或部分升级 fail-closed；
- 目标 `.claude/skills/steamai/SKILL.md` 的 create/unchanged/replace action 与完整 replacement diff；来源只能是 canonical skill exact bytes，未知自定义内容 fail-closed；
- legacy roots 保持不变、导入后不再作为运行依赖；
- preview identity，绑定所有 source file 的 relative path、SHA-256 和 bytes。

source bytes、用户补充事实或 pack revision 任一变化都使 preview 失效。用户确认只授权该 exact preview，不授权其它修复或迁移。

## Apply

只有 preview 无 unresolved item 且用户明确确认后才能 Apply：

1. 重新读取并验证 preview 绑定的 source regular files、SHA-256 和 bytes；
2. 从同一 exact source revision 物化 selected pack 与 `common/**` policy closure 的 case-local 只读 snapshot，并记录 pack/common tree identity；
3. 按 fresh case 模板创建 `.steamai-vnext/CLAUDE.md`、研究产物目录和按需成员目录；
4. 物化 preview 绑定的 `.steamai-vnext/contracts/` 声明式合同包；分发后的 Reviewer、成员和 learning 路径只从这里读取，不依赖 source clone；
5. 按 preview 的 exact action create/replace 目标 `.claude/skills/steamai/SKILL.md`；目标含来源不明或用户自定义内容时不得 Apply；
6. 写 `.steamai-vnext/import.md`，只记录 source kind、relative source files、采用/拒绝字段、preview identity、pack revision/tree 与只读边界；
7. 不复制真实 artifact，不改写旧 evidence/finding；只在新 index 中保留经检查的 case-local 相对引用；
8. 最后验证 legacy roots exact bytes 未变化。

Apply 不修改、删除、重命名、归档或续写 `.steamai/`、`.rekit/`、旧 skills 和旧研究文件；不 dual-write、不 fallback、不自动 commit/push。发生 collision、partial `.steamai-vnext/`、source drift、symlink/reparse、未知文件类型或写入失败时停止，不把 partial tree 宣称为 imported。

## 导入后

导入完成后所有新身份、任务、artifact index、evidence、finding、review 和 learning candidate 只写 `.steamai-vnext/`。legacy roots 仅作为用户保留的只读历史；canonical `/steamai` 不再读取其 runtime state 来决定当前任务、成员或权限。
