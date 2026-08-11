# 日常产品闭环实施方案

> 状态：`completed`
>
> 本文件保存已批准并完成的四闭环详细设计，不替代 `docs/real-usage-hardening-roadmap.md`、`docs/batch-plan.md` 或 `.rekit` 中的当前状态。`DPC-01`～`DPC-04`、最终源码真实入口验收和独立终审均已完成；本文件现在只供按需复核完成证据与共同边界，不能自行解锁下一路线。

## 读取指南

- 首选入口：先由 `docs/context-routing.md` 的“日常产品收口实施方案”场景路由到本文件。
- 只想判断方案是否符合目标：读“实施摘要”“验证标准”和“四个闭环总览”。
- 准备实施某一批：只读“共同架构边界”、对应 `DPC-*` 卡和“每批共同完成门槛”，不要默认加载其它批次细节。
- 本文件解决的是 **Windows + Claude Code + `vmp-re` 的日常产品收口**；不把 installer、GUI、跨平台对等或八个 skeleton pack 同步成熟塞进这四批。
- 本文件不是新的默认 read-first。实施时只消费 active route 指向的当前卡；不得从本文件自行提升下一批。

## 实施摘要

当前底层已经具备真实 Claude member、独立 Reviewer、纠偏、恢复、防重复、授权 gate、adapter dispatch/receipt 和证据账本。主要断点不是“能力不存在”，而是用户仍要知道 `/rekit`、两段 daily 调用、lane 名、已有目录初始化规则和 adapter 内部步骤。

下一阶段集中做四个中型垂直闭环：

1. **DPC-01：薄自然语言入口与人话控制面**——普通话能自动进入正确入口，查询只读，结果只显示用户需要做什么。
2. **DPC-02：一次用户操作完成 member → Reviewer → completion/correction**——隐藏第二段调用规则，但不放宽 fresh currentness、strict intake 或人工纠偏边界。
3. **DPC-03：普通已有目录安全接入**——先识别、预览和确认，再复用现有 `init`；不静默接管，也不自制跨模块事务。
4. **DPC-04：`vmp-re` IDA index 只读 adapter**——在已授权 gate 下读取有界导出，形成可复核证据，再由现有 member/Reviewer 链消费。

四批只包装既有能力，不建立第二套产品状态机。目标预算如下：

| 项目 | 预算 |
|---|---:|
| 新 public `rekit` command | 0；`DPC-04` 只给现有 `gate` 增加 strict autonomy profile 的 review-first 配置模式 |
| 新 durable `.rekit` schema / 状态文件 | 0；复用现有 `autonomy.json` v1、gate 和 adapter receipt schema |
| 新 workflow engine / product router | 0 |
| 新 manifest schema 字段 | 0 |
| 新 Go package | 0；IDA parser 保持在 `adapterhost`，若无法清晰实现则先升级方案，不能静默新增 |
| 新 case-local 输入 contract | 1 个内容寻址的 `vmp-ida-index-request`；它是 adapter 请求 artifact，不是平行 runtime 状态 |
| 新可执行 adapter | 1 个固定用途、Go-owned、只读 IDA index inspector |
| 本阶段扩展的领域 pack | 仅 `vmp-re`；不同时填充其余 skeleton pack |

## 执行清单

- [x] 用户明确批准本方案，并决定从 `DPC-01` 开始实施。
- [x] 将批准后的 `DPC-01` 提升为唯一 active batch；同步路线图和短 batch pointer。
- [x] 完成 `DPC-01` 的代码、测试、真实入口验收和 Windows 本机 release minimum。
- [x] `DPC-01` 通过后提升并解锁 `DPC-02`。
- [x] 完成 `DPC-02` 的真实 member + Reviewer + completion/correction、两个 cutpoint 恢复和 terminal replay 验收。
- [x] `DPC-02` 通过后提升并解锁 `DPC-03`。
- [x] 完成普通非空目录的预览、确认、原地初始化和 sentinel 不变验收。
- [x] `DPC-03` 通过后提升 `DPC-04`。
- [x] 完成 `vmp-re` IDA index adapter 的授权、contained process、证据登记、member 消费和 Reviewer 复核验收。
- [x] 四批完成后重新进行真实可用性复评；当前快照见 `docs/current-usability-assessment-2026-08-11.md`。
- [x] 在最终工作树上完成四旅程整体验收、最终源码真实 acceptance 与只读终审；Windows 本机 release minimum、commit/push 和 post-push 验证作为本次收尾门禁顺序执行。安装交付、第二个成熟 pack 和跨平台专项另行立项。

## 验证标准

四批全部完成后，以下自然语言旅程必须成立：

1. 用户在新会话说“看看这个 case 现在到哪了”，系统自动进入薄入口，只读返回一条人话动作，不要求用户输入 `/rekit`、lane ID、actor、SHA、session ID 或内部命令。
2. 用户说“在这个新位置开始一个 case，目标是……”，一次用户操作内完成真实 member、独立真实 Reviewer，并到达 `completed`；Reviewer 拒绝时停在 `waiting-for-correction`，不自行编造纠偏。
3. 用户指向已有普通非空目录时，系统不写入就先展示将新增/修改的 case 管理文件；只有明确确认后才原地初始化，原有 sentinel 文件内容和哈希保持不变。
4. 用户说“用现有 IDA 索引查这些字面词”，系统先形成可见授权请求；只有 canonical `authorized-gate` 存在时才启动 contained adapter，固定输入保持不变，结果进入 evidence/receipt/observation，随后 member 可引用结果并由 Reviewer 核验。
5. 重放已完成的 start/continue/correction/adapter 请求不重复启动 Claude、不重复执行 adapter、不产生冲突状态。
6. 任何 blocker、route drift、intervention、Reviewer rejection、目录身份异常、gate 不足、输入替换或 output 越界都 fail-closed，并返回稳定的人话 action code。
7. 每批 focused tests、必要 live acceptance 和完整 Windows release minimum 全部通过；测试失败时批次不得标记 completed。

每批共同 release minimum：

```text
go run ./cmd/rekit -- -Command release-check -Format json
go run ./cmd/rekit -- -Command status
go run ./cmd/rekit -- -Command packs
go run ./cmd/rekit -- -Command doctor
go test ./...
go vet ./...
git diff --check
```

## 风险与注意事项

- **最大风险是再造一套状态机。** 产品层只能读取 fresh status、消费 exact typed current request、调用现有 host/adapter、再读取 fresh status；不得根据文件存在、`FinalState` 或错误字符串猜下一步。
- **自然语言入口不是授权。** 自动加载 skill 不能自动执行写入、sync/promote、heavy action 或 adapter；是否需要确认仍由 canonical runtime/gate 决定。
- **人话输出不是 durable truth。** 中文文案可以改善，但不能写入 `.rekit`、参与 plan hash、反向决定路由或成为 replay identity。
- **已有目录第一版只支持原地接入。** 当前普通 member 只读取 case root，尚无安全的 arbitrary external source binding；拒绝原地接入时就停止，不隐式复制到 sibling case。
- **IDA adapter 第一版只读已有导出。** 不安装或启动 IDA，不联网，不读 IDB，不做 rename/comment/patch/debug/dump，也不执行 catalog 中的任意外部程序。
- **确认、预授权和执行是三件事。** 新 lane 继续默认 `manual-gate`；一次口头确认不能直接变成 `authorized-gate`。`DPC-04` 必须先通过现有 `gate` owner 的 hash-bound profile preview/Apply 显式配置 exact scope、预算和期限，再由 canonical gate 计算 `preauthorized` decision；adapter 仍不得自行写 profile、授权 gate 或 acknowledgement。
- **不把稳定性问题藏在 UX 后面。** typed failure 仍原样保留在机器结果中；人话 action 只做摘要，不能把 failed/rejected 说成 completed。
- **方案批准不等于 commit/push 授权。** 实施、commit、push 和外部发布按用户当时授权执行。

---

## 共同架构边界

### 1. 唯一事实来源不变

以下现有 durable state 继续拥有全部事实：

- mission intent；
- board、lane owner 和 executor generation；
- member execution；
- Reviewer packet/result lineage；
- intervention/reconcile；
- mission completion；
- pending/authorized gate；
- adapter dispatch/receipt/observation；
- evidence ledger。

本阶段禁止新增 `product-state.json`、`journeyPhase`、`workflowStatus` 或任何平行“当前产品阶段”。用户动作永远从 **本次 fresh typed result/status** 派生，不能持久化为第二份真相。

### 2. 固定依赖方向

```text
Claude Code natural language
        |
        v
thin canonical /rekit skill
  - intent normalization
  - read-only status projection
  - explicit confirmation handoff
        |
        +-----------------------+
        |                       |
        v                       v
rekit-host -daily       rekit-adapter-host
        |                       |
        v                       v
sessionhost                 adapterhost
        |                       |
        +-----------+-----------+
                    |
                    v
existing public deterministic runtime
status/current request/onboarding/workstream/member/reviewer/gate
                    |
                    v
existing .rekit durable truth
```

依赖规则：

- skill 只能调用 public binary，不能复制 Go runtime 逻辑。
- `cmd/rekit-host` 只做参数和 JSON 输出，流程归 `internal/rekit/sessionhost`。
- `cmd/rekit-adapter-host` 只做参数、父/子模式和 JSON 输出，adapter 生命周期归 `internal/rekit/adapterhost`。
- `sessionhost` 继续通过 `public_route.go` 的小 adapter 消费 canonical public runtime；不建立 command bus 或大而全 service interface。
- `adapterhost` 继续依赖 gate、adapter execution 和 lane mutation 的现有边界；deterministic packages 不得反向 import `sessionhost` 或 `adapterhost`。
- 不新建名为 `product`、`orchestrator`、`workflowengine`、`router` 的 package。

### 3. Canonical 日常路由不变

```text
durable state
→ status
→ missionControlRunbook.currentDriverRequest
→ run-current-step / run-current-loop / run-driver-step
→ refreshed status
```

产品层每一段只允许：

1. 读取 fresh status；
2. 判断 exact typed request 是否属于当前 host 已拥有的能力；
3. 调用现有 host segment 或 exact public plan/Apply；
4. 再读取 fresh status；
5. 遇到 typed blocker/input requirement 就停止。

不得：

- 扫描文件树重建“当前步骤”；
- 解析错误文本决定 retry 或 route；
- 为 start/continue/reviewer/gate 再排一套优先级；
- 在 skill 中复制 lane/reviewer/gate 状态转换；
- 将多个现有 owner 拼成一个无恢复边界的长事务。

### 4. 非 durable 人话动作

在 `internal/rekit/sessionhost/daily_view.go` 增加小型投影，挂在现有 `DailyResult` 上：

```go
type DailyUserAction struct {
    Code          string        `json:"code"`
    Message       string        `json:"message"`
    RequiresInput bool          `json:"requiresInput,omitempty"`
    Choices       []DailyChoice `json:"choices,omitempty"`
}

type DailyChoice struct {
    ID    string `json:"id"`
    Label string `json:"label"`
}
```

稳定 code 初始集合：

- `completed`
- `ready-to-continue`
- `waiting-for-correction`
- `directory-adoption-required`
- `confirmation-required`
- `ready-for-evidence-review`
- `blocked`
- `failed`

约束：

- `Code` 和 choice `ID` 是机器稳定边界；`Message`/`Label` 是可迭代展示层。
- view 只接受 fresh typed status/result，不直接读零散文件，不执行命令，不写状态。
- view 中不暴露 actor、SHA、session ID、generation、内部路径或内部命令，除非用户进入维护/debug 请求。
- 单元测试断言 code、`RequiresInput` 和 choice ID；不 snapshot 整段中文或整个巨大 JSON。
- typed failure 继续保留在现有结果字段；view 不能吞掉或改写失败类别。

### 5. 模块化不是抽象数量

本方案只在职责已经不同且可独立测试时拆文件：

```text
internal/rekit/sessionhost/daily.go
  RunDaily owner；保持 public API 和高层顺序

internal/rekit/sessionhost/daily_flow.go
  bounded host segment re-entry；不拥有状态，不实现 route priority

internal/rekit/sessionhost/daily_target.go
  read-only target admission classification；不决定 mission 下一步

internal/rekit/sessionhost/daily_view.go
  typed result → human action 的纯投影

internal/rekit/adapterhost/host.go
  immutable dispatch、owner、lease、input/output 共性不变量

internal/rekit/adapterhost/vmp_ida_index.go
  唯一 IDA index 输入解析、literal 过滤和有界 packet 输出

internal/rekit/adapterhost/authorized_run.go
  复用现有 gate/dispatch/contained child/receipt/observation 的产品生命周期

internal/rekit/autonomy/profile_provision.go
  现有 autonomy.json v1 的 hash-bound preview/Apply/revoke；不参与 gate decision
```

不为了“模块化”增加 interface、factory、registry、event bus、generic workflow node 或 mega DTO。一个文件只有一个明确 owner；跨层只传现有 typed contract 或上述小 view。

---

## 四个闭环总览

| 批次 | 用户可感知结果 | 主要 owner | 依赖 | 明确不做 |
|---|---|---|---|---|
| `DPC-01` | 不必记 `/rekit`；状态和下一步是人话 | skill + `daily_view` | 现有 status/daily result | 不自动授权，不改 durable schema |
| `DPC-02` | 一句目标自动走到 Reviewer/completion 或等纠偏 | `sessionhost` | `DPC-01` action view | 不取消 fresh segment，不自动纠偏 |
| `DPC-03` | 普通已有目录先预览确认，再原地接入 | skill + `daily_target` + existing `init` | `DPC-01` confirmation view | 不做 sibling/external source mount |
| `DPC-04` | 已授权后真实查询 IDA index，并进入证据闭环 | `adapterhost` + `vmp-re` | 前三批的入口/继续体验 | 不做通用 plugin/IDA 控制器 |

实施顺序固定为 `DPC-01 → DPC-02 → DPC-03 → DPC-04`。代码事实若使顺序失效，先更新本提案/批准后的路线和理由，不能静默跳批。

---

## DPC-01：薄自然语言入口与人话控制面

### 用户断点与完成后体验

**之前**：用户必须明确输入 `/rekit`，skill 约 32 KiB；加载后带入大量维护细节。状态结果暴露内部术语，主 Agent 还要自己猜“下一步该调用哪个命令”。

**之后**：用户说“开始分析这个目录”“继续上次任务”“复核说得不对，按这个意见重做”“现在进展怎样”，Claude 能自动进入一个小而无副作用的入口。查询只读；写入、纠偏、sync/promote、gate 都在既有边界要求输入或确认。普通结果只给“已完成 / 正在继续 / 等你补一句纠偏 / 需要确认接入 / 被什么阻塞”。

### 文件与职责

修改：

- `.claude/skills/rekit/SKILL.md`
  - 删除 `disable-model-invocation: true`；
  - 压缩到约 8 KiB，保留自然语言意图表、只读/写入边界、canonical runtime 指针和四类常用调用；
  - 保留 `caseshim` 当前 required canonical phrases，除非同批有意更新对应验证。
- `rekit/templates/case-shim/SKILL.md`
  - 允许自动进入；
  - 继续只读取 case binding 并跳回 canonical skill，不复制 runtime。
- `.claude/skills/rekit/references/maintenance.md`
  - 仅承接 canonical skill 中确实没有其它 owner 的维护命令细节；
  - 已由 `docs/agent-team-usage.md`、pack authoring 或其它 canonical doc 拥有的内容只链接，不复制。
- `internal/rekit/sessionhost/daily_view.go`
  - 实现纯 `DailyUserAction` 投影。
- `internal/rekit/sessionhost/daily.go`
  - 仅在 `DailyResult` 增加可选 `Action` 并在统一出口刷新，不将人话判断散落进流程。
- `internal/rekit/caseshim/*_test.go`、`internal/rekit/defaultdocs/*_test.go`、`internal/rekit/sessionhost/daily_test.go`
  - 更新入口和 view focused tests。

不修改：

- `.rekit` schema；
- mission/workstream/reviewer/gate route；
- public `rekit` command inventory；
- `rekit.ps1` runtime；
- machine `readFirst[]`。

### 意图路由边界

skill 只做一层意图归一化：

| 用户意图 | 产品动作 |
|---|---|
| 看状态、问进展、问下一步 | 只运行 public `status`，不启动 Claude、不写状态 |
| 开始或继续普通任务 | 调用 `rekit-host -daily`，消费其 typed action |
| 提供 Reviewer 纠偏 | 调用已有 daily correction，不自己拼 intervention 字段 |
| 接入普通目录 | 进入 `DPC-03` 的 preview/confirm；未实现前明确返回 unsupported，不走捷径 |
| 请求领域只读检查 | 进入 `DPC-04` fixed request/profile/gate/adapter；没有 exact profile 与 `authorized-gate` 时不执行 child |
| sync/promote/heavy action | 沿用现有 review-first / authorized-gate；自动加载 skill 不算确认 |
| 意图确实歧义 | 只问一个影响下一步的澄清问题 |

skill 不根据状态字符串自行选择 lane；它把 fresh status/current request 交给现有 host。多个 legacy lane 无唯一 current request 时，用 `DailyUserAction.Choices` 展示人话差异，让用户选择；choice ID 必须来自 canonical typed lane identity，展示层不发明别名映射。

### 失败边界

- 自动 skill 加载失败：用户显式 `/rekit` 仍可进入，canonical runtime 不受影响。
- status 失败：返回 `failed` 和简短恢复动作，不回退到扫描 `.rekit` 文件猜状态。
- 多 lane 无唯一选择：`RequiresInput=true`，不静默挑选“看起来最合理”的 lane。
- 任何命令需要确认：返回 `confirmation-required`，不能因用户先前说过“继续”而泛化成其它副作用授权。

### 验收

Focused：

- canonical skill 和 shim 都可被模型调用，shim 仍为 thin redirect；
- canonical skill 体积目标和 required phrases 通过；
- status 意图零写入、零 Claude launch；
- action code/choices 的纯映射覆盖 completed、correction、blocked、multi-choice、failed；
- `DailyResult` 旧字段保持兼容。

真实自然语言：

1. 在一个临时 case 外的新 Claude Code 会话中，只说“看看这个 case 现在到哪了”，不输入 `/rekit`。
2. 证明 skill 自动进入并只运行 status。
3. 输出不要求用户填写 lane ID、SHA、actor、session ID 或底层命令。
4. 对多 lane fixture 返回 choices，不擅自推进。

完成门槛：focused + 上述真实入口验收 + release minimum 全绿，且 skill 加载不执行任何写入。

---

## DPC-02：一次用户操作完成 member → Reviewer → completion/correction

### 用户断点与完成后体验

**之前**：`rekit-host -daily -goal ...` 在真实 member 交稿后固定停在 `member-intake-ready`；用户或主 Agent 必须知道还要第二次不带 goal 调用，才会启动 Reviewer 和收尾。

**之后**：一句目标在一个用户操作中经过多个**有明确边界的 fresh segment**：member 返回后重新读取 canonical status，再启动 Reviewer；通过则 completion，拒绝则返回 `waiting-for-correction`。用户不需要知道第二段命令，但 runtime 仍可在任意 segment 间恢复和重放。

### 文件与职责

修改：

- `internal/rekit/sessionhost/daily.go`
  - `RunDaily` 继续拥有 onboarding、goal/correction 分支和最终 result；
  - 删除“goal 完成 member 后无条件 return”的产品断点；
  - correction 继续复用现有 `runDailyCorrection`，不重写 lineage/reconcile。
- `internal/rekit/sessionhost/daily_flow.go`
  - 新增私有 bounded flow helper；
  - 只串联现有 `Run` host segment、`runPublicStatus` 和 `finishDailyCompletion`；
  - 不解析错误文本，不实现第二套 current-step switch。
- `internal/rekit/sessionhost/public_route.go`
  - 只有现有 `publicStatus` 缺少判断 typed ownership 所需字段时才补最小字段；不得引入整份 status mega DTO。
- `internal/rekit/sessionhost/daily_test.go`
  - 覆盖 segment boundary、typed stop、replay 和 correction。
- `internal/rekit/sessionhost/live_acceptance*.go` / tests
  - 将旧“两次 daily 调用”验收更新为“一次 goal 到终态”；保留可验证的 member/Reviewer 独立 session 证据。

### 有界流程

Goal 路径最多包含两个 top-level host segment：

```text
ensure onboarding / mission start
→ segment A: real member lifecycle（沿用当前 StopAfterMemberIntake cut）
→ durable intake-ready
→ update existing generation/result binding
→ fresh public status
→ typed blocker/intervention? stop
→ exact current request 属于现有 reviewer host owner? continue
→ segment B: independent real Reviewer lifecycle
→ fresh public status
→ existing completion path OR waiting-for-correction
```

约束：

- segment A/B 分别保留现有 session attempt/replacement 上限；新增 helper 不再套无界循环。
- segment 间必须重新读取 public status，不能拿上一个 `FinalState` 推断下一步。
- 只有 exact typed current request 属于现有 Claude host owner 且 `blocked=false` 时才进入下一 segment；gate/adapter/人工输入等其它 request 返回 `ready-to-continue` 或对应 input action。
- `Run` 内已有 currentness、route drift、intervention、strict intake 和 replay 检查全部保留。
- Reviewer rejection 原样停住；不得让 member 或主 Agent自动编写 correction。
- completion 只调用现有 `finishDailyCompletion`，不另写 lane/mission terminal state。

### 失败边界

- member 失败或额度/进程错误：保留 typed diagnosis，返回 `failed`；不为“跑到底”吞错或自动无限 replacement。
- member intake 后发现 route drift/intervention：停止，不启动 Reviewer。
- Reviewer 拒绝：返回 `waiting-for-correction`，附简短问题；不自动 reconcile。
- completion 尚不满足：返回 fresh `ready-to-continue`/`blocked`，不伪报 completed。
- terminal replay：零新 Claude launch，返回同一 terminal action。
- member 已 intake-ready、Reviewer 尚未启动的恢复：跳过 segment A，经 fresh status 进入 segment B，不能再次停回 `member-intake-ready`。
- Reviewer 已提交、completion 尚未 Apply 的恢复：零新 Reviewer launch，只执行 canonical remaining step/completion。

### 验收

Focused：

- goal 一次调用按顺序产生 member 和 Reviewer 两个独立 host run；
- `RunDaily` 现有 onboarding replay + member-ready early return 必须改为恢复到 segment boundary；
- fresh status 在两个 segment 之间被读取；
- blocker、intervention、route drift、Reviewer reject 都阻止后续 segment；
- accepted Reviewer 才进入现有 completion；
- 同一 terminal goal/correction replay 不新增 session；
- existing daily correction 的 rejection lineage、replacement generation 和 terminal replay 测试保持通过。

真实 Claude：

1. fresh 临时 case，一句普通中文 goal；一次 `rekit-host -daily` 结果到 `completed/lane-closed`，真实 session roles 正好包含 member 和独立 Reviewer。
2. Reviewer reject fixture/受控目标停在 `waiting-for-correction`；给一句纠偏后启动 replacement member + Reviewer 并完成。
3. 在“member intake 已提交、Reviewer 未启动”cutpoint 中断；重放同一 goal 时 member launch 为 0、Reviewer launch 为 1，最终完成。
4. 在“Reviewer 结果已提交、completion 未 Apply”cutpoint 中断；重放时 Reviewer launch 为 0，只完成 canonical remaining step。
5. 对两个终态请求各 replay 一次，`sessionLaunches=0`。
6. `manualResultWrites=0`，不手工伪造 member/Reviewer 输出。

完成门槛：成功、reject/correction、两个 segment cutpoint 恢复、terminal replay 全部通过，再跑 release minimum。

---

## DPC-03：普通已有目录安全接入

### 用户断点与完成后体验

**之前**：missing path 可自动 onboarding，合法 attached case 可继续；但已有普通非空目录会以“requires a current attached case”一类内部错误退出。用户不知道该 `attach`、`sync` 还是 `init`。

**之后**：产品先只读识别目标。普通目录返回“是否在当前目录新增 case 管理文件”的完整预览；明确确认后复用 canonical `init -Apply`，再回到原有 daily/onboarding。用户拒绝时零写入停止。

第一版只有两个选择：

- `initialize-in-place`：在当前目录建立 case；
- `cancel`：不做任何写入。

不提供“旁边新建但读取原目录”的伪选项，因为当前普通 member 的 Claude launch 只可靠读取 case root，尚无经过身份绑定和 currentness 校验的 arbitrary external source mount。

### 文件与职责

修改：

- `internal/rekit/sessionhost/daily_target.go`
  - 只读分类 target admission；
  - 使用 canonical path、`Lstat`/reparse 边界、instance 和 mission intent inspection；
  - 返回私有 target kind，不写状态。
- `internal/rekit/sessionhost/daily.go`
  - onboarding 前消费分类；ordinary directory 返回 typed action，不把内部 onboarding error 直接抛给普通用户。
- `internal/rekit/sync/sync.go`
  - 为 `InitPlan` 增加稳定 `ExpectedPlanSHA256` 和 exact Apply binding；
  - plan hash 使用专用 canonical semantic projection：包含 case/repo/pack/project identity，以及每个 write 的 relative path、kind、action、source content SHA-256、target exists/kind/content SHA-256；排除 `BackupRoot`、`NextSteps`、绝对展示路径和文案等时间/presentation 字段；
  - Apply 获取 case mutation lease 后重建 fresh plan 并校验 hash，再进行第一笔写入；
  - 提供 ordinary-directory adoption action policy：只允许新增、unchanged 和 skip-existing，不允许修改任何 pre-existing regular file。
- `internal/rekit/cli/cli.go`
  - 现有 `init` 的 preview 输出 exact hash-bound Apply 参数；不增加新 command。
- `.claude/skills/rekit/SKILL.md`
  - 收到 `directory-adoption-required` 后运行 canonical `init -WhatIf -Format json`；
  - 展示 write-set 和 blocked actions；只在本次明确确认后运行 `init -Apply`，然后重新调用 daily。
- `internal/rekit/sessionhost/daily_test.go`
  - target classification 和零写入测试。
- 现有 `sync` / `syncreview` / `onboarding` focused tests
  - 补 hash-bound init、ordinary non-empty directory 的 action policy 和原文件保留测试；不新建 adoption transaction。

私有 classification 只表达 admission，不表达 mission 状态：

- `missing`
- `ordinary-directory`
- `attached-case`
- `mission-case`
- `invalid`

存在 partial `.rekit`、错误 binding、symlink/reparse、非目录或绑定到另一 kit/pack 的目标必须归 `invalid`/repair-needed，不能当 ordinary directory 覆盖。

### 接入流程

```text
natural-language start
→ read-only target classification
→ ordinary-directory
→ DailyUserAction(directory-adoption-required, choices)
→ canonical init -WhatIf JSON
→ deterministic adoption policy 检查 action 白名单
→ 展示 case root、pack、write-set、blocked actions、ExpectedPlanSHA256
→ 用户明确选择 initialize-in-place
→ canonical init -Apply + exact ExpectedPlanSHA256
→ Apply 在 mutation lease 内重建并核对同一计划
→ doctor/attachment check
→ existing daily onboarding + DPC-02 flow
```

这里增强并复用现有 `init` review plan，而不是自己拼 `attach + sync + onboard + start` 长事务。skill 只负责展示和传递本次确认；plan hash、action policy、lease 内重建和所有实际 case 文件写入都由 canonical Go runtime 拥有。

Ordinary-directory adoption 允许的 pre-existing target action 只有 `unchanged`、`skip-existing-local-file`、`skip-existing-support-file`；`create-*` 只能用于原先不存在的路径。preview 出现 `overwrite-with-backup`、`overwrite-local-template-file-with-force`、`append-managed-block`、`replace-managed-block`、已有 binding refresh 或其它会改变旧字节的动作时，产品必须返回 `blocked`，不能将 backup 当作允许覆盖的理由。

### 失败边界

- 用户取消/未明确确认：零写入。
- preview 与 Apply 在 lease 内重建的 `ExpectedPlanSHA256` 不同：零写入拒绝并要求重新 preview/确认。
- 目录中已有不同 managed file、已有会被 append/replace 的 `CLAUDE.local.md`、partial case 或错误 binding：fail-closed，返回 repair/manual review；不覆盖。
- init 成功但 doctor/attachment 不通过：返回 failed，不启动 Claude。
- init 后 daily route 出现 blocker：保留已合法初始化的 case，返回 blocker；不做不安全的“自动回滚整个目录”。
- 任何普通原文件变化都视为失败；不能把“case 初始化成功”掩盖为通过。

### 验收

Focused：

- missing、ordinary、attached、mission、invalid 五类稳定分类；
- classification 和 `init -WhatIf` 零写入；
- ordinary directory 不直接进入 onboarding；
- partial `.rekit`、symlink/reparse、wrong binding 拒绝；
- `init -Apply` 要求 valid `ExpectedPlanSHA256`，并在 mutation lease 内重建 exact semantic plan；
- 同一未变化目录相隔超过一秒重复 preview，`ExpectedPlanSHA256` 必须相同，证明 `BackupRoot`/文案未污染 identity；
- final preview 后创建 managed-file collision，plan hash 必须变化且 Apply 在第一笔写入前拒绝；
- 已有不同 managed file、已有 `CLAUDE.local.md` 的 append/replace action 都阻止 ordinary-directory adoption；
- 合法已有非空目录只新增计划内 case 文件，所有 pre-existing regular file 字节不变。

真实目录：

1. 在仓库外创建临时非空目录，写入多个 sentinel（含嵌套目录和二进制小文件），记录路径、大小和 SHA-256。
2. 用自然语言开始 case；确认前比较完整目录快照，必须零写入。
3. 选择 `initialize-in-place` 后执行 canonical init，sentinel 路径、大小和 SHA-256 全部不变。
4. doctor/attachment 通过，随后一句 goal 走完 DPC-02 的真实 member + Reviewer。
5. 另建已有冲突 managed file、已有 `CLAUDE.local.md`、partial `.rekit` 和 reparse fixture，证明均在写入前 fail-closed。
6. 对未变化目录跨秒重复 preview，plan hash 相同；再在 final preview 后制造 managed-file collision，plan hash 变化且 exact Apply 必须零写入失败。
7. 清理所有本轮临时目录。

完成门槛：零写入 preview、sentinel 保留、invalid fail-closed、真实 daily 全部通过，再跑 release minimum。

**完成状态（2026-08-10）**：已完成。五类 admission、stable choice、manifest/source/target 绑定 plan hash、mutation lease 内 fresh rebuild、Windows create-only publication/exact rollback、late drift/rebound/cleanup 对抗测试和非 Windows 零写入 capability failure 均通过；真实普通目录 `initialize-in-place` 保留二进制 sentinel，doctor 通过。完整 `go test ./...` 因单次工具 10 分钟上限改为覆盖同一 package 全集的三组 fresh tests，全部通过；`go vet ./...`、公开 release/status/packs/doctor、`git diff --check` 和两轮只读复审通过。

---

## DPC-04：`vmp-re` IDA index 只读 adapter

### 用户断点与完成后体验

**之前**：`vmp-re` 有 IDA bridge recipe 和候选说明，但日常产品没有一条可直接选择、受限执行、自动回写证据的 concrete adapter。完整 dispatch/receipt/observation 链主要存在于 `_template` live acceptance。

**之后**：用户把已有 IDA 文本索引放进固定 case-local 目录并提出 literal 查询。Go runtime 先把 query 与 exact input hash 物化为内容寻址 request，再展示一个**限时、exact scope** 的 autonomy profile 预览。用户确认该 profile 后，canonical gate 才能形成 `authorized-gate`。Go-owned parent 在 child 启动前再次验证 current profile 哈希等于 gate authorization snapshot 且尚未过期，然后启动 contained child。child 只读 request 中绑定的固定文件并输出有界 packet；parent 写 receipt、validation 和 execution observation，随后把 generated profile 恢复为默认 `manual-gate`。Mission Commander 审核 packet 后才 acknowledgement/resume，随后 member 可引用结果，Reviewer 校验引用。

这里的用户确认授权的是 exact autonomy profile 变更，不是把 `manual-confirmation-required` gate event 原地“点成通过”。现有 runtime 只有 strict validated `preauthorized` profile 才会生成 `authorized-gate`，因此 profile provision/revoke 是本闭环的确定性前置能力。

### 固定输入合同

Case-local 目录：

```text
tooling/ida-agent-bridge/
  query.json
  export/
    function_index.tsv   # 必需
    strings.tsv          # 可选
    imports.tsv          # 可选
    xrefs.tsv            # 可选
```

`query.json` v1 只支持 literal terms，不做 DSL：

```json
{
  "schemaVersion": 1,
  "terms": ["literal-a", "literal-b"],
  "maxRowsPerIndex": 50
}
```

固定限制：

- `terms` 1–16 个，每个单行、1–128 UTF-8 字符；case-insensitive literal substring，不支持 regex/glob/expression。
- `maxRowsPerIndex` 默认 50，范围 1–200。
- 每个 TSV 最多 1 MiB；单行最多 64 KiB；只接受 UTF-8 regular file。
- `function_index.tsv` 必需；其它缺失仅形成 warning。
- 输入目录和文件全部拒绝 symlink/reparse、越界 path 和运行中替换。
- 输出 packet 最多 256 KiB；达到限制时明确 `truncated=true`，不能静默丢数据。
- v1 不读取 pseudocode/disassembly sidecar，不打开 `.i64/.idb`。

在任何 gate/dispatch 前，runtime 生成：

```text
tooling/ida-agent-bridge/requests/<request-sha256>.json
```

request 是 canonical、exclusive、内容寻址的 case-local tooling artifact，不是 `.rekit` 状态。它包含 query、四个固定输入各自的 case-relative path / exists / SHA-256 / bytes、aggregate input SHA-256 和 limits。文件名必须等于 canonical request bytes 的 SHA-256；同路径只允许 exact replay，不允许覆盖。gate target 和 autonomy `targetScope` 绑定该 exact request path，而不是只绑定可变的导出目录。

parent 与 child 都重新验证：request bytes hash 等于文件名、gate target 等于 request path、当前 input bytes 等于 request snapshot。任何 source 在 request 后发生变化，都要求生成新 request 并重新走 profile/gate；不能用旧 dispatch 处理新字节。terminal receipt 仍是旧 request 的历史事实，不因用户后来修改 source 而被改写；新的自然语言查询必须基于 fresh snapshot 得到新的 request identity。

输出 `ida-index-packet.json` 至 dispatch-owned output root，至少包含：

- adapter/schema version；
- query SHA-256；
- 每个实际输入的 case-relative path、SHA-256、bytes；
- selected rows 的来源文件、1-based line、原始有界 row；
- warnings、errors、truncated、nextActions；
- adapter report 所需 artifact/evidence refs。

### 文件与职责

修改：

- `packs/vmp-re/manifest.yml`
  - 使用现有 `heavyToolGates` schema增加 `inspect` action；
  - `sideEffects: inspect,filesystem-write`、低风险、`requiresConfirmation: true`；
  - 不增加 manifest 字段。
- `packs/vmp-re/tooling/catalog.yml`
  - 增加唯一 `status: mainline` 的 `vmp-ida-index-inspector`；
  - `entry` 固定为 Go-owned `rekit-adapter-host`，明确 `gateActions: inspect`；
  - 现有 external `ida-agent-bridge` 若保留，只能是 non-executable candidate/reference，不能被动态执行。
- `packs/vmp-re/tooling/recipes/ida-agent-bridge-readonly.md`
  - 同步上述固定 input/query/output limits；不扩成通用 IDA 控制说明。
- `internal/rekit/autonomy/profile_provision.go`
  - 复用现有 `autonomy.Profile` schema v1，实现 preview / hash-bound Apply / revoke-to-default-manual；
  - 只允许从 exact default `manual-gate` 进入生成的 `preauthorized` profile，绑定 `inspect`、内容寻址 request path、最小正预算、固定 output paths/stop conditions、`grantedBy/At` 和最长 15 分钟 expiry；
  - Apply 获取 open lane mutation lease、重建并校验 expected current profile hash 与 plan hash，然后原子替换 `autonomy.json`；非默认/自定义 profile fail-closed，不覆盖用户配置。
- `internal/rekit/gate/gate.go`、`internal/rekit/cli/cli.go`
  - 在现有 `gate` command 下暴露 autonomy profile preview/Apply/revoke 模式；不增加 public command；
  - profile Apply 不生成 gate decision，必须重新跑 canonical gate preview/Apply；只有 `DecisionPreauthorized` 才写 `authorized-gate`。
- `internal/rekit/adapterhost/vmp_ida_request.go`
  - 从 raw `query.json` 与固定 TSV 生成 hash-bound request preview/exact Apply；用 exclusive anchored write 发布 request artifact。
- `internal/rekit/adapterhost/host.go`
  - 保留 immutable dispatch、current owner、lease、input immutability、owned output 和 report 共性；
  - 仅用显式 switch 选择两个已知 adapter ID：现有 `_template` inspector 与新 `vmp-ida-index-inspector`。
- `internal/rekit/adapterhost/vmp_ida_index.go`
  - 完成固定文件校验、literal matching、limit 和 packet；不依赖 dynamic registry。
- `internal/rekit/adapterhost/authorized_run.go`
  - 从 live acceptance 提取“运行**已有 authorized gate**”的产品生命周期；
  - dispatch preview/apply → contained child → receipt preview/apply → report validation → execution observation；
  - 不创建/授权 gate，不自动 acknowledgement。
- `cmd/rekit-adapter-host/main.go`
  - 只增加运行已有 authorized gate 的父模式参数及 JSON 输出；child 仍消费 immutable dispatch。
- `internal/rekit/autonomy/*_test.go`、`internal/rekit/gate/*_test.go`、`internal/rekit/adapterhost/*_test.go`、manifest/catalog/doctor focused tests
  - 覆盖 profile provision/revoke、request snapshot、parser、path、limit、gate selection、parent/child/replay。

不新增 generic plugin registry，不从 catalog `entry` 任意启动程序。catalog 是选择与说明合同；可执行实现仍由编译进二进制的显式 adapter ID 决定。

### 授权和执行流程

```text
natural-language inspect request
→ validate fixed case-local input
→ request WhatIf（query + exact input hashes，零写入）
→ autonomy profile WhatIf（exact lane/action/planned request/budget/output/expiry，零写入）
→ DailyUserAction(confirmation-required)
→ user confirms exact request plan hash + profile plan hash
→ exact request Apply（exclusive content-addressed artifact）
→ autonomy profile Apply
→ fresh canonical gate WhatIf / Apply
→ assert durable authorized-gate + DecisionPreauthorized
→ adapterhost RunAuthorizedGate
    → exact terminal receipt/observation exists? return replay with zero launch
    → otherwise revalidate current profile hash/expiry against gate snapshot
    → revalidate request hash and current input snapshot
    → immutable dispatch preview/apply
    → revalidate profile/request/input before child launch
    → launch contained child for exact adapter ID
    → child validates owner/gate/input and writes owned output/report
    → parent verifies input unchanged and output/report
    → receipt preview/apply
    → execution observation
→ hash-bound revoke generated profile to default manual-gate
→ DailyUserAction(ready-for-evidence-review)
→ Mission Commander reviews bounded packet
→ existing acknowledgement/resume
→ DPC-02 member/Reviewer consume evidence
```

授权必须绑定 exact lane、`inspect` action、内容寻址 request path、最小预算、固定 output paths/stop conditions 和短 expiry；adapter ID 继续由 gate-compatible catalog selection + immutable dispatch 绑定。generated profile 只允许从 default manual profile provision；child 启动时 current profile 必须仍与 gate 中的 profile hash 一致且未过期，execution observation 落盘后立即 hash-bound 恢复为 default manual。若 provision 后中断，resume 只能为同一 request 完成 gate/execution/revoke 或先 revoke；不得把临时 profile用于其它 target。

`query.json` 或任一 export hash变化后会产生不同 request identity；旧 gate/dispatch/receipt 不能覆盖新输入。由于 gate binding 已包含 exact request target，而 request 文件名又绑定 canonical bytes，DPC-04 不需要扩 adapter dispatch/receipt durable schema。

### 失败边界

- `manual-gate` 只能得到 `manual-confirmation-required`；没有 exact profile confirmation / `authorized-gate` 时不启动 child。
- current profile 不是 exact default manual，或 generated profile 的 current hash/expiry/scope 漂移：fail-closed，不覆盖自定义 profile。
- profile provision 后、gate/child 前中断：resume 只允许为同一 request 继续或先 revoke；profile 到期后必须重新确认。
- child 前 current profile 缺失、被修改、已过期或 hash 与 gate authorization 不同：不启动 child，fail-closed。
- execution observation 后 revoke 失败：不重复执行 child，返回 blocked 并只允许 exact revoke 恢复；evidence review/continue 在恢复 manual profile 前不推进。
- gate target/action/adapter/lane、request filename/hash 或 current input snapshot 不匹配：fail-closed。
- 输入过大、非 UTF-8、symlink/reparse、路径越界、request 后/运行中变化：失败且不发布成功 receipt。
- child 超时/非零退出/输出越界/report 不一致：记录 typed failed observation；不伪造 packet。
- terminal replay：先于 current profile 校验识别 exact receipt/observation，复用既有终态并零新 child launch；profile 已恢复为 manual 不影响历史 replay。
- packet 生成成功不等于 evidence 已认可：必须由 Mission Commander/Reviewer 审查，产品不自动 acknowledgement。
- adapter 绝不启动 IDA、网络、debug、dump、patch、rename/comment 或任意 catalog executable。

### 验收

Focused：

- manifest/catalog schema valid，只有 concrete Go-owned adapter 可执行；
- manual gate 不能直接产生 authorized gate；profile preview 零写入，Apply 要求 exact current/profile plan hash；
- only-default-manual → generated preauthorized → authorized gate → execution observation → default manual 的顺序成立，自定义 profile 不被覆盖；
- child 启动前必须核对 current profile hash/expiry 与 gate snapshot；
- request/profile/gate/execution/revoke cutpoint 可恢复；profile 前失败最多留下可复用的 immutable request；execution 前 revoke/漂移时 child launch 为 0，execution 后 revoke 失败时 child 不重复启动；
- profile 恢复为 manual 后 terminal replay 仍能先命中 exact receipt/observation，并零启动返回；
- request artifact filename/hash、input aggregate identity和 exclusive replay成立；dispatch 后/child 前替换 query/TSV 必须拒绝；
- literal query 正确返回带来源行和 input hash 的 bounded rows；
- missing optional、truncation、超限、invalid UTF-8、bad query 都有稳定结果；
- symlink/reparse、路径逃逸、输入替换、wrong owner/generation/gate 拒绝；
- contained child 只能写 owned output；
- dispatch/receipt/observation identity 匹配；terminal replay 零启动；
- `_template` 现有 inspector 回归不变。

真实 adapter + Claude：

1. 临时 `vmp-re` case 写入无敏感、可公开的最小 TSV fixture 和 literal query。
2. manual gate preview 和未确认 profile 都必须零 child launch。
3. 确认 exact profile plan；fresh gate 形成 `authorized-gate` 后，验证 child 启动时 current profile hash/expiry 与 gate snapshot 一致。
4. 启动真实 contained child；request 与输入 pre/post SHA-256 相同；observation 后 profile 恢复为 default manual。
5. 在 dispatch Apply 后、child launch 前替换 `query.json` 或一个 TSV，child 必须零成功执行并要求生成新 request。
6. packet/report/receipt/validation/observation 均可由当前 runtime 重新验证，selected rows 可回到 request snapshot 指定的原 TSV 对应行。
7. Mission Commander 审核并 acknowledgement；继续 daily，真实 member 引用 packet evidence refs，独立 Reviewer 核验后完成。
8. profile 已恢复为 manual 后 replay 同一 terminal request，仍零 child/Claude 重复启动；修改 source 后的新查询生成不同 request，不重用旧执行。
9. 清理临时 case。

完成门槛：focused path-security + contained child live + evidence consumer member/Reviewer + release minimum 全绿。

**完成状态（2026-08-11）**：已完成。最终显式 `vmp-re` live acceptance 返回 `passed=true`，真实启动 3 个 member 与 3 个独立 Reviewer session；第一代无 IDA evidence 的 member 被真实拒绝，correction 后 fixed adapter 完成内容寻址 request、generated preauthorized profile、canonical `authorized-gate`、immutable dispatch、contained compiled-in child、committed output、packet/report/receipt/observation、profile revoke、独立 trusted Claude evidence review、`tool-review` acknowledgement、replacement member exact 13-field binding 与 Reviewer exact lineage。terminal adapter replay 零 child，terminal mission replay 零 Claude，attached ordinary-directory 的两个 cutpoint、completion recovery、sentinel hash 与 cleanup 同轮通过；无手写 LLM result、无 authority/confirmed、无 IDA 启动或 catalog entry 执行。

`NoNetwork=true` 在该验收中只证明 fixed Go child 没有网络代码路径，不是 AppContainer/WFP 级 OS socket 隔离；后者若成为威胁模型要求，需单独架构升级。

---

## 每批共同完成门槛

每个 `DPC-*` 都必须独立具备以下证据，不能等四批一起补：

1. **用户 before/after**：用一句普通话说明本批解决了哪个可感知断点。
2. **单一 owner**：每个新增 helper 的数据来源、写入 owner 和失败边界明确；没有双写/双路由。
3. **focused tests**：覆盖成功、typed stop、replay 和至少一个对抗性输入。
4. **真实验收**：声称 Claude 成功必须启动真实 Claude；声称 adapter 成功必须启动真实 contained child。
5. **零伪造结果**：不手写 member output、ReviewerResult、adapter packet 或 receipt 冒充 live success。
6. **本机 release minimum**：全部通过；超时转后台但最终 exit 0 才算通过。
7. **文档按需路由**：只更新用户可见行为和当前批次指针，不把实现日志复制到多个 active docs。
8. **工作树可解释**：不混入真实 case、绝对临时路径、trace/dump/capture、客户信息或运行产物。
9. **停止条件**：任何关键真实链失败、架构预算需要突破、runtime schema 需要迁移或公共入口需要删除时，保持 in-progress 并升级，不用临时补丁绕过。

## 防止后续 AI 维护“牵一发动全身”的规则

- 新需求先判断属于 **意图展示、host segment、deterministic runtime、adapter implementation** 哪一层；只能在其 owner 层改，不横跨四层复制条件。
- `DailyUserAction` 只从 typed result 派生。新增文案不应要求修改 mission/gate schema；新增 runtime 状态也不能靠新增文案 code 代替。
- `daily_flow.go` 不允许出现大量 `switch FinalState`。一旦需要根据多个状态字符串继续，应回到 canonical current request owner 修复。
- `daily_target.go` 只判断“能否安全进入 onboarding”，不判断“mission 下一步”。
- `vmp_ida_index.go` 只处理固定 IDA index contract；process/gate/receipt 仍在 adapterhost 共性层。
- 同类 adapter 达到第二个之前不抽象 plugin interface；第二个出现时也先比较真实重复，再决定是否抽象。
- 不为测试增加生产后门或“跳过 Reviewer/授权/哈希校验”的 flag。
- 每批尽量保持 diff 局部：入口批不顺手改 adapter，adapter 批不重写 daily；相邻 cleanup 只有因本批产生 orphan 或阻塞验证时才做。
- 发生 bug 时优先从 `DailyUserAction.Code`、host segment、current request、dispatch ID 四个稳定边界定位，避免从中文文本或完整日志猜测。

## 方案完成后的下一步（不在本方案内）

四批完成并复评后，再按真实试用数据决定：

1. 预编译 Windows binary / installer 与统一命令体验；
2. 收紧 soak retry 的结构化分类并做当前版本可复核 release；
3. 从 `web-security` 或 `generic-binary-re` 选择第二个深 pack；
4. Linux/macOS product path 专项。

这些项目不应提前混入 `DPC-01`～`DPC-04`，否则会稀释当前“把已有能力包成顺手日常产品”的目标。
