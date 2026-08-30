# STeamAI 自包含项目模型

## 读取指南

本文件是 STeamAI 自包含项目、项目内 runtime、`.steamai` 状态根、`/steamai` 日常入口和旧 `.rekit` 迁移的专题合同。新会话先从 `docs/context-routing.md` 进入；只有在实施或排查这些能力时才读本文件。当前完成状态以 `docs/real-usage-hardening-roadmap.md` 的 active card 和真实测试为准，不能因为本文描述了目标合同就声称实现已经完成。

不要默认读取旧 batch 历史、完整 runtime 命令参考或 PowerShell 退役文档。旧 `/rekit`、`.rekit` 和中央 kit/case thin-shim 模型只用于兼容与迁移，不再是新项目默认 UX。

## 实施摘要

产品模型固定为：

```text
一个真实项目目录 = 一个自包含 STeamAI 项目
```

用户已经安装并正在使用本机 Claude Code。STeamAI 不负责安装 Claude Code、登录、全局 plugin 或另一个桌面启动器。日常开始方式只有：

```text
cd <project>
claude
/steamai
```

用户也可以直接说“开始这个项目，目标是……”“继续推进”“现在到哪了”“按这条意见纠偏”“暂停/恢复/停止 verifier lane”。主 Agent / Mission Commander 负责解释意图，项目内 deterministic runtime 负责状态、currentness、授权、写入和恢复。

新项目使用项目级 `.claude/skills/steamai/SKILL.md`、唯一可变状态根 `.steamai/`、项目内 runtime 和所选 pack。项目复制或移动后不能依赖旧绝对路径、机器全局 PATH 或原中央 kit 仓库。旧 `/rekit` 与 `.rekit` 在迁移期间只作为兼容入口；迁移必须显式 preview、确认 exact hash 后 Apply，不能自动双写、合并或择优。

## 执行清单

- [x] 品牌、自然语言称呼和新 slash command 固定为 `STeamAI` / `/steamai`。
- [x] 新项目默认选择 `.steamai`；legacy-only 项目继续单写 `.rekit`；双根 fail-closed。
- [x] `status -Format compact-json` 提供 4 KiB 硬预算的只读投影，保留完整 typed request/choices；无法完整安全输出时返回小型 blocked envelope。
- [x] project-local executable 提供 no-mode `help` / `status` / `continue` 用户入口；默认 summary 与 opt-in `--diagnostics` 分层，维护 flags 不进入普通用户 parser。
- [x] 显式未接入目录的 status 只读投影 schema-valid、非 template pack choices；普通 initializer 只允许选择 mature `binary-re` / `web-security`，skeleton packs 仅保留 inventory 可见；pending onboarding publication 只发布一个绑定 durable identity/stamp/plan 的 exact recovery action，不重新开放 pack 选择。
- [x] daily action 提供“现在、原因、下一步”和 typed recovery 分类；不从 provider detail 编造用户建议。
- [x] `bounded-autonomous-v1` 提供显式 opt-in 的单 lane、exact action/target、有限预算、短时有效自治档位；每次仍重验和留证。
- [x] ordinary init 发布可验证、可重定位的项目内 runtime bundle、selected pack、common 与必要 assets。
- [x] copied-directory 在中央 kit 不可用时，项目内 `status`、`packs`、`doctor` 和 daily 路线全部通过。
- [x] 完成 `.rekit` → `.steamai` 的 zero-write preview、hash-bound Apply、durable receipt、replay 和 drift/dual-root/reparse 负例。
- [x] production owner 通过 shared `projectstate` 选择 current/legacy root，并保留明确的 legacy compatibility 测试。
- [x] case public JSON 按 resolved state root 投影全部 project-local typed command：current `.steamai` 只显示 `/steamai`，legacy `.rekit` 只显示 `/rekit`；投影不改 prose、durable artifact identity 或 source snapshot SHA。
- [x] exact lane的`pause` / `resume` / `stop`使用独立append-only control generation与review-first exact Apply；paused/stopped/旧generation结果不进入live progression，stop保持durable-first并只允许exact local supervisor owned-containment actuation。
- [x] Batch 828 的独立恢复边界复核与完整 Windows release minimum 已关闭；当前产品优化路线与完成证据以 `docs/real-usage-hardening-roadmap.md` 和 fresh validation 为准。

## 1. 日常使用

### 1.1 第一次进入已接入项目

```text
cd <project>
claude
```

在 Claude Code 中输入 `/steamai` 或自然语言即可。用户不需要填写 pack、lane、executor、session ID、generation、内部路径或 SHA。需要直接调用 verified executable 时，只使用 no-mode `help`、`status`、`continue [--lane <selector>]`；默认输出是“现在、原因、下一步”，只有 `--diagnostics` 或 `--format=json` 返回 typed JSON。`continue` 固定 preview-only，不自动 Apply、启动 Claude或运行 heavy tool。

### 1.2 还没有接入的普通目录

未接入目录在 init 前没有项目级 `/steamai` skill，不能从目录内凭空启动自包含流程。可信的外部 STeamAI initializer／maintenance executable 先做只读分类并返回 `directory-adoption-required`；`initialize-in-place` 只生成 canonical init preview，`confirm-exact-plan` 只有携带同一个 `ExpectedInitPlanSHA256` 才 Apply，`cancel` 保持目录不变。确认前零写入、零 Claude launch；stale hash、existing collision、partial state、state-root conflict、symlink/junction/reparse 或 source/target drift 均 fail-closed。确认 Apply 的同一次调用只返回 `ready-to-continue`，不自动启动 onboarding Claude；下一次 fresh daily 才进入项目流程。project-local executable 也不能借这个入口接入另一个普通目录，接入 owner 必须是外部 source-clone maintenance executable。Apply 发布项目内 skill、`.steamai` 状态根、verified runtime bundle 和 selected pack 后，项目才进入上面的日常 `/steamai` 流程。

### 1.3 日常意图

| 用户表达 | 产品动作 |
|---|---|
| “现在到哪了”“下一步是什么” | 项目内 runtime 的 compact status；零写入、零 Claude launch |
| “开始/继续推进” | fresh typed daily owner；resume/goal/correction/control/adoption 只由一个 operation owner 选路，`-Lane` 只是 selector；多 lane 先显示 typed choices |
| mission 完成后“开始另一个新目标” | successor preview→确认→exact Apply；独立generation、保留旧任务audit tree、返回`ready-to-continue`，不自动启动Claude；fresh status再发布initial `start` preview |
| “按这条意见纠偏” | fresh rejection/reopen route；不自建第二状态机 |
| “暂停/恢复/停止某条 lane” | fresh exact lane的`control` WhatIf→确认→exact Apply；多lane先选择 |
| “换新会话接手” | fresh status + scope-bound handoff preview/Apply |
| “同步模板”“沉淀经验” | `sync` / `promote` review-first，等待精确范围确认 |
| heavy action | strict profile + fresh `authorized-gate`；超范围立即停止 |

普通 continue 的 public executable contract 固定为三阶段：fresh status 发布 typed `-WhatIf -Format json`；preview 结果以 `continuePlanSha256` 绑定完整 mutation snapshot，并发布保持同 selector、owner、generation 和其它 typed 参数、携带 `-ExpectedContinuePlanSha256` 的 exact `-Apply`；Apply 结果或后续 fresh status 重新发布 preview。blocked preview 不发布 Apply。主 Agent不得从 command prose 手工拼 phase，不得让 command 与 invocation 分别改写，也不得复用刚执行的 Apply request。

completed mission 的不同新目标不复用 continue/reopen owner。successor preview绑定predecessor mission intent、完整closure与completion receipt，并给每个最终写入提供准确SHA/size；exact Apply按intent→generation artifacts→generation commit→transition commit→active pointer last顺序发布。中断留下的exact durable prefix可由同一request恢复，任何不同字节、stale closure、legacy/dual root或损坏的active binding都fail-closed。激活后mission-scoped board/policy/lanes/facts/runs/reviews/handovers等只指向active generation，project identity/runtime/packs/onboarding/transitions仍留项目根。

默认用户输出只包含：

```text
现在：当前发生了什么
原因：为什么停在这里或为什么可以继续
下一步：唯一安全动作
```

## 2. 项目布局

目标布局为：

```text
<project>/
├─ .claude/
│  └─ skills/
│     └─ steamai/
│        └─ SKILL.md
└─ .steamai/
   ├─ instance.yml
   ├─ state.json
   ├─ runtime/
   │  ├─ manifest.json
   │  └─ bin/
   │     └─ steamai.exe
   ├─ packs/
   │  └─ <selected-pack>/
   ├─ common/
   ├─ lanes/
   ├─ facts/
   ├─ evidence/
   ├─ reviews/
   └─ handoffs/
```

`.steamai/packs/<selected-pack>` 是项目内 pack 的唯一 canonical 布局；不得同时创建 `.steamai/runtime/packs` 作为第二份可变来源。bundle manifest 必须绑定平台、架构、每个 executable 的 SHA-256/bytes、pack manifest 与 asset inventory/tree identity、common/runtime assets 和 installed skill identity。

`instance.yml` 只能记录可重定位的项目内引用和稳定 identity，不能把初始化时的绝对项目路径永久保存为运行依据。项目复制到新目录后，runtime 必须从当前 project root 重新解析这些相对位置。

## 3. 状态根选择

每次操作先对 exact project root 做一次选择：

```text
.steamai only → 只读写 .steamai
.rekit only   → 只读写 .rekit（legacy compatibility）
neither       → 新项目预期使用 .steamai
both          → fail closed
```

禁止：

- 同时写两份状态；
- 自动合并或按时间选择“较新”目录；
- 用 symlink、junction 或其它 reparse object 把两个名字伪装成同一 mutable root；
- 在当前目录发现冲突后跳到祖先目录继续；
- 接受空 project root、绝对动态片段、volume-qualified 片段或 `..` 路径逃逸。

运行中的 owner 还必须固定 state-root 物理 identity；若执行期间切根、替换目录或出现第二状态根，立即停止。

## 4. 项目内 runtime

新项目不得依赖：

- 原中央 STeamAI source checkout 仍位于初始化时路径；
- 全局 PATH 中碰巧存在 `rekit`；
- 用户级 Claude skill 或全局 plugin；
- 复制项目之前的绝对路径。

`/steamai` 使用 `${CLAUDE_PROJECT_DIR}` 定位项目根，验证 `.steamai/runtime/manifest.json` 后只调用 manifest 以 `runtime-executable` role 绑定的 `.steamai/runtime/bin/steamai.exe`。该单一 executable 为普通用户提供 no-mode `help`、`status`、`continue [--lane <selector>]`；`status` 默认消费 compact projection，显式 `--diagnostics` / `--format=json` 才返回 full typed JSON，`continue` 始终强制 `WhatIf` preview。它同时显式支持 `runtime`（deterministic typed CLI）和 `host`（含 `-daily`）模式，供主 Agent、自动化和维护者使用：compact typed status 使用 `steamai.exe runtime -Command status -Target <project> -Format compact-json`，daily 使用 `steamai.exe host -daily -target <project> ...`。`cmd/rekit` 只负责 executable binding、mode recognition 和 dispatch；durable 状态、public command projection 与 mutation 仍由既有 Go owners 持有，不得在 front door 重建。不得把 `rekit.exe`/`rekit-host.exe` 或 developer Go source 当作已发布项目在中央 kit 缺失时的隐式 fallback。

runtime resolution 必须优先识别显式 target 所属的项目内 bundle。即使当前 shell 位于 kit 仓库，也不能用中央 source repo 覆盖一个 current `.steamai` target 的 runtime identity。

Mature production pack 的 Claude session 还必须绑定 project-local instruction identity。Runtime 从 verified bundle 内的 selected manifest、`common/` policies、pack policy overlays 与声明的 prompts 构建只含 path/SHA-256/bytes/mode/receipt kind 的 durable identity；instruction 全文不进入 dispatch、receipt或recovery JSON，只在process start前按该identity从同一bundle稳定重读并内联stdin。Identity原样贯穿external dispatch、adapter execution/evidence-review intent/result、direct Reviewer package、detached supervisor spec与structured-output recovery；任一pack、source、receipt kind或aggregate SHA drift都在launch/recovery前fail-closed，且这些instructions不授予heavy-tool、authority/confirmed或更广文件系统/网络权限。

## 5. Compact status

默认状态格式为 `compact-json`，最终 UTF-8 bytes（含换行）不得超过 4096。它是 full status 的只读投影，不建立新状态机，并满足：

- exact `currentDriverRequest` 与 SHA 完整一致；
- 所有 typed choices 与 invocation 完整保留；
- 不携带完整 project handoff、takeover、queues 或大段诊断对象；
- request identity 无效或完整内容超预算时，不截断、不重建，返回 `details-required` blocked envelope；
- established case 的 `case.enabledSpecialties[]` 在 compact/full JSON 与 text 中保持同一 exact adapter ID 集合；它只表示同 pack project-local executable owner、production contract 与 typed verified catalog `supported` 三者完全一致，不表示已授权、已执行、已有真实 target/tool receipt或 recipe/template 已成为 producer；
- full `json` 只在 typed envelope 明确要求，或用户/维护者显式传 `--diagnostics` / `--format=json` 时按需读取；默认 no-mode `status` 不显示 SHA、durable lane/session ID、generation、absolute path 或内部维护 command。

## 6. 故障恢复

用户可见 recovery 只从 typed failure code、state、attempt budget 和 mutation boundary 投影，不解析或泄露 provider detail。

- `auto-recovered`：只有 exact durable evidence 证明同一有界步骤已经恢复并返回成功结果。
- `retryable`：已识别恢复动作、attempt 尚未耗尽，且 mutation boundary 属于允许的恢复集合。
- `user-decision-required`：lane 歧义、Reviewer correction、authority/confirmed、sync/promote、heavy scope、未知 mutation、attempt exhausted 或其它跨决策边界。

current project refresh 中断时，runtime 在普通 bundle discovery 之前只读验证 durable transaction，并唯一分类为：

- `restore-transaction`：durable intent 已发布，但初始 journal 尚未发布；
- `resume-transaction`：已有非终态 journal，只能继续原 forward transaction；
- `finish-cleanup`：终态 journal/receipt 已匹配，只剩 strict replay 与 exact cleanup；
- `manual-repair-required`：owner、intent、journal、receipt 或 namespace 冲突，无法安全推导自动动作。

此恢复入口不依赖 active `instance.yml`、active runtime manifest 或中央 source repository。项目内 executable 只有在物理上属于 exact target，且 bytes/mode/size 匹配 durable plan 绑定的 old 或 new `runtime-executable` identity 时，才可进入 recovery-only surface：`status`/`doctor` 给人话诊断，daily 固定 zero launch，其它普通命令被 fence；项目内 executable 不能执行维护 Apply，也不能在 transaction 消失后回落到 onboarding、lane mutation、Claude discovery 或 host launch。真正继续事务只能由原 external maintenance executable 使用 exact reviewed Apply。

正在执行 Claude 的 current project 持 shared execution lease，current-sync 在任何 kit/lane mutation 前必须取得 exclusive。Detached supervisor 启动前在同一 stable per-user namespace 发布 exact pending handoff；child 只有在取得并复验自己的 shared lease 后才能 claim。若 parent 在此之前退出或被 hard-kill，maintenance exclusive 先发布 append-only exact per-run cancellation receipt，再精确清理 pending；取消过程崩溃可 replay，同一 run/spec/session 的 ABA 重发永久拒绝，late child 不得启动 Claude。

上述 current-sync Apply 与 current durable detached handoff 都要求 handle-bound exact filesystem mutation。当前 Windows 实现提供该能力；Linux、macOS 及其它非 Windows 平台在获取 mutation lease、写 supervision spec／handoff／transaction intent／cancellation receipt或启动 child之前拒绝该窄路径，避免半事务和 replay poisoning。Source-free recovery inspection、status/doctor、其它 read-only/preview surface仍可用；legacy `.rekit` supervisor继续走既有 zero-handoff compatibility。Linux/Darwin/FreeBSD cross-compile只证明平台集合可编译，不是平台runtime安全证据。

compact recovery status 只输出 state、pending/blocked/recoverable 和“现在、原因、下一步”，不暴露 Apply args、plan SHA、transaction path 或 raw diagnostic。namespace 不完整或冲突时只返回 typed `manual-repair-required`，不猜恢复路线。

Claude Code executable、登录、配额或模型不可用时，STeamAI 只说明当前 Claude Code 调用条件不可用；它不承担安装、登录或 provider 配置。

## 7. Durable execution control

`control` 是独立于 lane status、executor generation、external attempt generation、supervisor run ID 和 gate authorization 的 per-lane append-only stream。genesis 为 `running` / generation 0；合法转换只有 `running → paused|stopped` 与 `paused → running|stopped`，每次成功动作递增 control generation；`stopped` 是终态。

用户从自然语言或 `/steamai` 发起 `pause`、`resume`、`stop` 时，主 Agent必须先从 fresh typed state唯一选择exact lane；多lane先给typed choices。WhatIf零写入并绑定lane/action/actor/reason、current owner、publication stamp与plan SHA；只有用户确认同一preview后才能原样Apply。current-only只写`.steamai`，legacy-only只写`.rekit`，dual root在preview或写入前fail-closed。

每个可能推进状态的结果在birth时捕获exact owner与control binding。current member handoff把transport binding冻结进immutable handoff与checkpoint；external attempt、submission和observation必须保持同一birth lineage，historical missing/stale lineage只读diagnostic，不得在resume或recovery时补采新generation。legacy`.rekit` handoff的nil lineage继续仅作迁移兼容。raw execution truth始终可记录；但paused结果只生成stable `held-while-paused` receipt，stopped后的结果只生成`late-after-stop`，旧generation或head漂移只生成对应stale/changed receipt。它们不得进入live outputs、intake、Reviewer writeback、completion或checkpoint progression；resume只允许新generation下未来结果继续，不自动释放旧held result。

`pause`只改变durable状态，不做OS suspend。`stop`先提交durable stopped receipt；exact local supervisor child观察该head后发布run-scoped actuation request，只能关闭自己持有的Windows Job/containment handle并追加observation。actuation失败不回滚stopped，terminal raw truth仍独立记录，process termination不是durable stop成功判据；不得按裸PID管理进程，也不得用本路径管理opaque Remote Control session。

control receipt、request SHA、transport/endpoint/delivery observation与actuation observation都只证明各自currentness或事实，不授予authority/confirmed、strict profile、`authorized-gate`或heavy action。consumer writer必须在自己的mutation lease内重新验证binding，不能依靠旧preview或公开status projection绕过paused/stopped head。

## 8. 有界自治档位

`bounded-autonomous-v1` 是“在已经确认的窄范围内免逐次询问”，不是无限权限。它必须显式 opt-in，并绑定：

- 当前 attached project 和单一 durable lane；
- selected pack manifest 声明的 exact actions；
- exact target，network/external target 还要有完整显式 scope；
- 正数且有上限的 runtime/disk/request budget；
- manifest stop conditions 全覆盖；
- case-relative output paths；
- `recordRequired=true`；
- 最长 15 分钟 expiry；
- 可重算的完整 SHA-256 managed identity。

每次 Evaluate 都重新检查 owner、profile、expiry、action、target、budget、stop conditions 和 outputs。Profile 不能授予 authority/confirmed，不能自动执行 sync/promote/schema migration，不能把 transport delivery、request SHA 或自然语言当成授权。v1 不包含多 lane/case-wide grant、renewal/rotation 或自动 heavy action。

## 9. Legacy 迁移

迁移是独立 review-first operation，不是 `repair` 的隐式副作用：

1. preview 验证 legacy-only attached project、完整 `.rekit` tree、legacy skill/metadata、无 reparse 和 `.steamai` 缺失；零写入返回 inventory、before/after identity、writes 和 plan SHA；
2. Apply 必须携带同一个有效 SHA，重新验证 project/state-root identity、source tree、metadata、skill 和所有目标 currentness；
3. 迁移保留 lane/facts/evidence/gate/autonomy 等 durable bytes，不推导或提升权限；
4. attached source pack 为 retired `vmp-re` / `generic-binary-re` 时，只有显式 `migrate-state -Pack <exact-retired-pack>` 可进入 pre-runtime migration owner；普通 `status` / `doctor` 仍拒绝 retired identity，unknown pack 与 current-only forged retired bundle不进入迁移；
5. retired migration在同一 plan/lease/fence/receipt 内发布 canonical `binary-re` runtime bundle、managed/template/block/support root projection；若存在 committed schema-v1 onboarding，则确定性重建为 relocatable schema-v2，pending/corrupt或未知 lane 在写入前拒绝；
6. 发布 current relocatable metadata、`/steamai` skill 和 migration receipt，并确保最终只有 `.steamai` 是 mutable root；
7. exact replay 只接受同一 receipt/plan，并从 receipt 绑定的 legacy metadata 恢复 source pack后重验 root/onboarding provenance；source/target drift、provenance omission、dual root、partial publication 或不同 hash fail-closed。

迁移完成前，仍受支持的 legacy project继续从 `/rekit` 单写 `.rekit`；retired source只能先执行显式迁移，不能继续 ordinary runtime。不得为了显示新品牌而只改 skill展示、手工把 retired pack改成`binary-re`，或让状态 owner继续写旧根。

## 验证标准

完成本路线必须同时满足：

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
go mod verify
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
git diff --check
```

并追加 Windows 自包含验收：

1. ordinary directory init 发布完整 project-local bundle；
2. 复制项目到新路径，移除或隐藏中央 kit；
3. 从复制后的项目运行 project-local `status`、`packs`、`doctor` 和 daily route；
4. metadata、skill、bundle、pack 与 executable hashes 仍一致，输出不包含旧项目绝对路径；
5. current、legacy、dual-root、path escape、file/symlink/junction/reparse、root switch 负例全部 fail-closed；
6. current/legacy full status 的 case shim、pack memory、gate、Reviewer、execution evidence、current-loop、member execution 与 takeover public surface 使用各自唯一入口，typed request 逐 root 重算并验证 SHA，prose/fence/durable identity 保持不变；
7. compact status、typed recovery、bounded autonomy provision/evaluate/revoke、legacy migration preview/Apply/replay 全部通过；
8. canonical release-run 的步骤、receipt/inspection Git命令使用有界进程树：Windows suspended→non-breakaway Job→resume，Unix独立process group；root退出、deadline或64 MiB输出上限均终止 containment 内的剩余子孙；随后只做5秒有限pipe drain，逃逸writer未关闭时明确失败而不永久等待EOF。

## 风险与注意事项

- Batch 828 已证明 runtime bundle、copied-directory/no-central-kit、legacy migration、current-sync recovery process E2E、supervisor pre-shared hard-kill cancellation与完整 Windows local minimum；当前路线新增的 relocation、promote、pack/control/adapter闭环必须以各自 fresh evidence重新判定。不得把Git-local machine receipt说成post-push receipt或remote CI green。
- 项目内 bundle 是交付边界，不等于要管理 Claude Code 安装或登录。
- 新品牌不应触发仓库名、Go module、所有内部 package 和 executable 的机械改名；公共/内部命名在产品模型稳定后分阶段迁移。
- `.rekit` production literal 仍可能存在于明确 legacy compatibility、测试 fixture、文档历史和 schema 兼容中；静态门禁应禁止新增 current mutable owner 直写，而不是盲目删除所有字符串。
- actual heavy action、authority/confirmed、sync/promote、外部副作用和公共 façade 删除继续遵守原有升级与验证边界。
