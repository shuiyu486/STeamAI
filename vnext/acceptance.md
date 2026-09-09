# STeamAI 产品与 verified-learning 验收

本文件保留 `steamai-windows-native-product-v1` 的证据分层，并增加 `steamai-verified-learning-v1` 的独立 Gate 验收。所有自动 fixture 使用临时目录和无真实样本内容；结果不写入模板仓库，不把 session ID、绝对 case path 或 artifact bytes 保存为产品状态。

当前自主协作优化范围与执行状态见 `docs/autonomous-collaboration-roadmap.md`；此前有效性结果保留在 `docs/research-validity-roadmap.md`，不代替本轮真实行为验证。各路线不放宽本文件既有 gate。

## 证据分层

以下层级互不替代：

1. **Default automated tests**：生产 `casebootstrap`、`learningbatch`、native shell 的 deterministic filesystem/Git/command contract。
2. **Windows native live**：真实 `steamai.exe`、HKCU Registry/PATH、NTFS publication、named mutex 与 `CREATE_NEW_CONSOLE`。
3. **Claude Code live**：真实 context/file access、visible independent sessions、用户直接纠偏与原生跨会话消息。
4. **Release live**：tag workflow实际生成、上传、下载和校验 release assets。

fake platform、synthetic fixture、remote CI definition、cross-compile 或 `go test -run` 零匹配不能冒充其它层。

## 1. Default automated gates

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

focused：

```text
go test -count=1 ./internal/steamai/casebootstrap
go test -count=1 ./internal/steamai/evaluation
go test -count=1 ./internal/steamai/learningbatch
go test -count=1 ./internal/steamai ./internal/steamai/vnextcontract
```

必须覆盖：Fresh zero-write/exact confirmation/source+target drift/current deep validation；learning 多 candidate/多 target、Reviewer binding、mutable patch TOCTOU、HEAD/index/snapshot不漂移、target-only rollback；setup/update/uninstall command边界；禁止产品脚本和旧 control plane。

## 2. Claude Code capability probe

显式 opt-in：

```text
STEAMAI_VNEXT_LIVE_ACCEPTANCE=1 go test -count=1 -run TestLiveNativeContextAndFileAccess ./internal/steamai/vnextcontract
```

该 probe 只证明成员 cwd 自动上下文与 `--add-dir` case file access。缺少 `claude`、认证或 CLI capability 必须失败；默认 suite 不发起模型调用。它不能证明用户看见窗口、直接纠偏或跨会话协作。

保留的自动 persistent probe（仍不等于人工可见验收）：

```text
STEAMAI_VNEXT_PERSISTENT_MULTISESSION_ACCEPTANCE=1 go test -count=1 -run TestLivePersistentMemberContextAndCorrection ./internal/steamai/vnextcontract
```

## 3. Windows native product journey

在一次性本机测试账户或明确可恢复的临时安装边界中，用真正构建的 `steamai-windows-amd64.exe` 完成：

1. **setup**：默认路径与 `--source <TEMP_CHECKOUT>` 各一次；检查 installed exe、HKCU source/version/PATH ownership；新终端可解析 `steamai`。不使用 PowerShell/.cmd/.bat 产品脚本。
2. **Fresh**：从外部普通临时项目运行 `steamai`；同一 Commander窗口看到 exact preview；确认前零写；输入 exact synthetic confirmation 后 project-local skill、contracts、pack/common snapshot与marker current。
3. **Fresh drift**：改变 source或target后，旧确认失效且`.steamai-vnext/`不发布。
4. **Visible member**：Commander调用 `steamai __open-member <name>`；屏幕上立即出现普通交互 Claude Code窗口，cwd是成员目录，case通过`--add-dir`可读。覆盖从 Claude shell 工具继承 `CLAUDE_CODE_CHILD_SESSION=1` 的真实启动：正式入口应移除该标记，成员正常保存原生 transcript，退出后能以同一 session 恢复；不得在验收脚本中预先删除标记或强设 `CLAUDE_CODE_FORCE_SESSION_PERSISTENCE` 掩盖产品缺陷。若用户另行明确关闭历史保存，应如实记录该设置边界，不强制覆盖。
5. **Duplicate Commander**：第一个仍运行时再次在物理同一case（包括path alias）运行`steamai`，第二个明确拒绝。
6. **Learning batch**：至少3 candidates→3 eligibility reviews→2 targets→1 accepted batch review；preview完整显示source chain/Reviewer/pre-postimage/patch；exact confirmation后只修改targets，HEAD/index/case snapshot不变。
7. **Update**：从 canonical checkout 外运行，使用 clean checkout 从已发布测试 tag 更新；manifest/hash/tag/revision 均匹配；source 需要变化时与 exe 一起切换，HEAD 已等于 release 时走 exe-only 路径。从 canonical checkout 根或子目录运行时必须在联网前明确拒绝。再分别制造 dirty/untracked/ignored、本地其他 branch 或 stash commit、错误 hash、错误 revision、已存在 staging/backup、文件锁、准备期间 source 漂移、exe 发布后的 Registry 写失败及网络失败，确认旧可用版本保留且无自动 Git 修复；若 Windows 锁使 executable rollback 不完整，错误必须列出保留的新旧 exe/source recovery paths，且不得把 source 回滚成与仍 active 新 exe 不匹配的版本。source 替换成功后旧 checkout 作为 sibling backup 保留，命令输出路径且不自动递归删除。
8. **Uninstall**：只移除installed exe、setup拥有的PATH和定位信息；checkout、local commits和case保留；运行中exe先重命名，同字节临时原生 helper 在卸载命令退出后删除已安装入口；普通用户不需要管理员权限或重启；helper 自身残留的精确路径必须输出，进程退出后验证可手工删除。

这些步骤涉及HKCU/PATH和真实窗口，不能由普通unit test假装完成。执行结果记录在case外的短验收摘要中，只写pass/fail与必要能力边界。

## 4. Visible multi-session 与用户纠偏

在仓库外临时 current case 中按行为边界完成以下 live gates。共享成员任务、currentness 与研究产物的同一研究链使用同一 case；相互独立的 HOLD、研究往返、容量/恢复可以拆成短 gate 分次验收。各 gate 各自冻结输入与结果并 fail-closed，不要求一个外部控制器同步驱动全部窗口；任一层不能替代另一层：

1. Commander按需打开两名正式成员和最多一名Reviewer；窗口从启动起用户可见、可输入、可暂停。
2. 用户直接在owner窗口修改当前任务；owner更新自己的任务并通知受影响成员。
3. 再发送带旧expected task的延迟变更；compare-before-update返回`HOLD_STALE_TASK`，不得覆盖用户纠偏。
4. 每个问题保持一名 owner 和最多一名 verifier；owner只向一名verifier请求有界复核，不广播、不增加第二verifier。
5. `__open-member` 的基础启动兼容下限为支持 `--name` 的 Claude Code 2.1.76；Windows 原生消息验收使用 Claude Code 2.1.248 或更高版本。`__open-member` 启动的 session 带 `--name <member-name>`；Commander/成员先在与目标会话相同的 Claude Code 配置域用 `claude agents --json` 把精确 member cwd 唯一映射到实际 name，再要求该 name 在 `ListAgents` 中恰好可达，只有两边唯一相交才用 `SendMessage` 完成定向协作，并同时核对发送 `success:true` 与接收方 incoming record。inventory 单独不能证明可达，`ListAgents` 单独不能证明成员目录身份，仅出现 tool call 或 `success:false` 均不算送达；跨会话 `SendMessage` 不能冒充 user/direct-session correction，也不能扩大授权。
6. Reviewer round 1 若为 `needs-evidence` 则返回原 owner；补证后只追加完整 round 2，绑定 current finding/evidence SHA，实际 decision 由证据决定，不预填 `accepted`；若最终为 accepted，再改变承重输入后该 accepted stale。
7. 超过 3 名 active 执行成员或 1 名 active Reviewer 的创建请求必须拒绝；关闭并恢复active成员窗口，确认目录身份与当前任务延续；completed/inactive成员不自动启动。

`claude logs`、`attach`、`respawn`与`claude --resume <session-id>`只作观察/恢复备用；Agent view/background session不是默认成员体验。自动resume不替代用户实际看见和输入。不解析 transcript JSONL，不把session记录重建为产品状态。

## Verified-learning Gate acceptance

这里的产品 Gate 状态与某份 finding/learning 的 V0–V4 maturity 是两条轴；实现 Gate 4 不会让任何 learning 自动成为 V4。

### Gate 1 — Proof-carrying finding

必须证明 replay spec exact/immutable，绑定 finding/input path-SHA-bytes、环境、授权、预算与停止条件；自动 runner 拒绝 unsupported class；原始输出进入 artifact，replay result 继续形成 evidence 和 append-only review。`not-reproduced`、`blocked`、`invalidated`、`inconclusive` 都能保留且不得 retry-to-success。V1 只能由 deterministic assertion 支持；V2 必须由不依赖 owner session memory 的 verifier 在绑定环境内复现，同模型新 session 或模板测试不能冒充 V2。

### Gate 2 — Evaluator calibration

默认 suite 只用 fake Claude/synthetic fixture 证明 strict native `__evaluation-suite-prepare`→逐 slot `__evaluation-run`→`__evaluation-suite-finalize` 生命周期、逐 slot 独立 control patch、verified-learning contract/Claude Code/platform/tool profile currentness、baseline/patch SHA、matched tools/model/permissions/budget、case-state 外 sibling arms、`--safe-mode` customization isolation、blind manifest/records、单一 immutable `blind-review.json` 及独立 reveal、failure/cancelled/invalid-output bundle，并由 live gate 覆盖实际 timeout、no-replace publication 与 hash closure；默认 tests 不调用模型。Windows 实现必须将 suspended arm 加入 kill-on-close Job Object 后再恢复，不能留下 Start→Assign 的子进程逃逸窗口；运行中 cancel/timeout 必须终止整个 arm process tree。即使 arm failed、cancelled、timeout、invalid-output 或 safety fail，immutable bundle 仍先发布，而 native run command 随后以 typed nonzero outcome 返回。finalize 必须能保留完整 `no-go`/`inconclusive` structural closure，只有独立的 Gate 3 validator 可把满足 mapping 的 suite 判为 go-eligible。显式 live calibration 必须冻结 improvement、neutral、regression、authorization-regression、prettier-but-weaker-evidence controls 及 rubric，初始每项 2 pairs、只在预注册范围内增加且最多 6；deterministic hard gate 优先，所有失败 bundle 都进入 attestation。suite/model/tool/core contract 漂移使 calibration stale。只有真实 Claude Code calibration 达到预注册阈值才能记录 `go`；未执行时为 `pending`，不能由 unit test 推断。

成功 arm 使用配置解析后冻结的 model 选择，不限定厂商。当前验证路径为 `--output-format json --verbose` 的事件数组：逐条主会话 `assistant.message.model` 核对实际模型，最终 result 保留 `modelUsage`/cost；允许已验证的 `[1m]` 选择后缀规范化，不按用量条目数量猜测主模型。在线 runner 与离线 `VerifyBundle` 使用同一解析器，拒绝缺失或矛盾的 model/cost/safety，失败 bundle 仍可作 structural evidence。CLI 版本与格式必须实证，未知格式不自动兼容；格式 probe 不能被当成已完成 calibration。

### Gate 3 — Comparative promotion

必须证明 candidate 与最终完整 thematic patch 先冻结，scenario/rubric 不被 candidate 修改；任一 behavioral/V3 candidate 强制 current calibration=`go`、两 arm completed、safety=`pass`、comparative=`improved`、maturity=`V3`。Reviewer 只读不含 mapping 的 manifest 与其绑定的单一 immutable `blind-review.json`，一次读取 packet 后将 blind identity、packet path/SHA、preferred entry、preferred output SHA、comparative 与 hard-safety 结论冻结到 exact blind-decision path/SHA，再由 Commander 提供独立 `reveal.json` 解盲；Go 机械重验 entry→opaque arm→output SHA→candidate arm，不读取理由判断语义；最多一个 tie-breaker且不无限重跑。candidate bundle 必须与 calibration suite 的 rubric、verified-learning contract、model、Claude Code version、platform 与 tool profile exact 相同，且任何 calibration control patch 不得复用最终 behavioral patch。batch review、preview identity 和 Apply exact 绑定 calibration/promotion attestation、blind decision、run manifest raw SHA/semantic identity、reveal SHA 与最终 patch SHA；promotion attestation 中的 reveal path 必须 exact 等于该 run manifest 的 sibling `reveal.json`，calibration attestation 的 reveal path/SHA 必须均为 literal `none`。任一文件、record、output、status、baseline 或 patch binding 漂移都零写拒绝。confirmation 仍独立，Apply 后 HEAD/index/case snapshot 不变且不 stage/commit/push。

### Gate 4 — Field outcomes

每份 outcome 只由该后续 current case 的用户逐份 opt-in，绑定 artifact/evidence/finding/current accepted review；`improved`、`neutral`、`regressed`、`inconclusive` 全部 append-only 保留。不得自动遥测、发现、扫描、拉取或汇总其它 case；单个 case 不得标为 V4。多个独立 case 的每份 outcome 都经脱敏和用户确认后，provenance 更新仍走 candidate→review→exact batch→confirmation。synthetic multi-case fixture 只能证明机械合同，不能证明真实 V4 效果。

### Explicit live evaluator gate

只有维护者显式设置 live 开关时才允许调用 Claude Code。固定开关为 `STEAMAI_VERIFIED_LEARNING_LIVE_CALIBRATION=1`；未设置时所有 live tests 明确 skip。真实调用的 token-control smoke、独立 Reviewer 的有界 synthetic calibration、Windows 原生 helper 的 timeout/process-tree 验证分别记录，不能相互替代。token smoke 即使全绿也不签发 calibration attestation，不自动解锁 V3。Reviewer calibration 在执行前冻结五类控制、每类同一任务两次配对和盲审 rubric，裁决只一次读取 production `blind-review.json`，不读取 expected class/reveal；运行身份前置校验失败时停止后续付费调用，明确记录 incomplete/blocked，不能填充未执行 slots。未提供认证、预算或 controls 时应明确 skip/fail-closed，不能静默降级为 fake evidence。该 live test 还必须覆盖真实 timeout 与 Windows process-tree cleanup。运行摘要留在 case 外，只记录必要的 pass/no-go/inconclusive 和能力边界，不保存真实 case 数据。

本地模型选择顺序：先合并 user → project → local settings 的模型字段，再以继承的 `ANTHROPIC_MODEL` 优先于 settings 中的选择；`opus`/`sonnet`/`haiku` 通过已配置的 `ANTHROPIC_DEFAULT_*_MODEL` 解析。只冻结解析出的模型名称，不保存完整配置；缺失或无法解析的动态别名直接停止。配置解析单元测试不会调用模型。

从 canonical source 目录执行以下维护验收命令（不是产品 façade），每个 Claude Code 调用使用运行前解析并冻结的配置模型、120 秒、`--max-budget-usd 0.10`。live test 只支持继承的模型环境变量及标准 user/project/local settings 模型字段和已配置别名，不读取会话 `/model` 状态、不复制认证或改用户配置；managed/session/动态选择需先明确固定模型，不在测试里重建配置控制面。Reviewer 路径最多 10 pairs × 2 arms 加 10 次盲审，共 30 次调用、请求预算合计 $3.00；CLI budget 是请求限制而非计费硬保证，超报也保留为失败证据。运行身份阻塞时不会继续花满预算。

```powershell
$env:STEAMAI_VERIFIED_LEARNING_LIVE_CALIBRATION = '1'
go test -count=1 -timeout=45m -run '^TestLiveVerifiedLearningBoundedSyntheticReviewerCalibration$' -v ./internal/steamai/evaluation
go test -count=1 -timeout=5m -run '^TestLiveVerifiedLearningTimeoutClosesProcessTree$' -v ./internal/steamai/evaluation
Remove-Item Env:STEAMAI_VERIFIED_LEARNING_LIVE_CALIBRATION
```

单独复查 token smoke 可选择 `^TestLiveVerifiedLearningControlSmoke$`；不要把它串进默认 suite，也不要将失败项反复重跑直至通过。测试输出给出仓库外证据目录；所有已尝试调用的成功、失败、超时和无效输出均保留，blocked 后的未执行项明确列出且不能补写结果。有限 synthetic Reviewer 校准 `pass` 也只支持该 frozen suite/runtime 范围，不证明真实研究质量或任意 candidate 改善；它不是 calibration `go` attestation，正式晋级仍需 Reviewer 闭合 suite exact evidence。`completion=complete` 只表示全部预注册 slots 已执行；`decision=no-go` 仍使 live test 返回非零，不能描述为验收通过。结果及已知失败统一记录在当前 roadmap，不在 README 或模板另建结果副本。

## Research capability — 双领域与主辅 pack

本节用于 `steamai-research-capability-v1`，不替代上面的原生能力与 verified-learning gates。成员已有独立目录、任务文件、原生 session 上下文与恢复；不把冷接手未全面测试当作已有缺陷。

### 默认机械与内容合同

- production Fresh→Apply→InspectCurrent 覆盖单包、主＋可选一个辅助包，完整 common 只一份；未声明包、主辅同名、辅助字段空/缺半/重复、幽灵辅助、缺 manifest/router、字节/index/tree 漂移与旧确认都拒绝。负向 fixture 其余 SHA/bindings 尽量自洽，避免仅因旧摘要不匹配而误判覆盖。
- 同一个 v2 snapshot parser 接受辅助字段缺省的旧单包，读取前后 path set 与 bytes 不变，不访问 canonical，不更新旧 skill/contracts。测试不维护第二套旧 renderer。
- learning 从双包 case 仍只写主包；辅助 destination 和跨包 patch 拒绝。成功和失败恢复均不改辅助、case snapshot、HEAD/index。辅助经验来源仍要 current accepted evidence、主包适用性和自包含审查。
- 两领域方法的合成抽象正例、反例、证据不足例与按需路由进入内容合同检查。Binary RE 覆盖 final effect、覆盖写/alias、来源独立性与 keep unknown；Web/API 覆盖发送状态、混杂因素、修复前后与合法行为对照。文本断言只证明规则存在，不证明模型执行正确。

### 真实效果与主辅消费（explicit opt-in）

真实调用、可见窗口或请求必须先得到该动作所需授权与预算。不要把所有真实动作藏进默认 Go tests，也不建设永久四臂框架。

1. 在仓库外用 production Fresh 分别建立现行、仅方法增强、仅决策增强、最终组合的独立合成 case。每组使用对应 exact skill/templates/pack bytes，记录 source/snapshot identity；不能修改同一 current，不能用临时附加几句提示词冒充候选分发。
2. 运行前冻结任务、评分依据、预算和当前配置解析后的模型，预留未用于调优的任务。隔离各组 session/auto memory，避免前组答案泄漏；不改变用户全局配置，也不假设同一 Git 仓库的不同 cwd 自动隔离 auto memory。
3. 同题比较检查选择、实际读取的 pinned 方法路径、反证后行动、结论正确性、问题覆盖、无效步骤与费用。不能只检查字段是否写齐；全 unknown、遗漏可支持结论或牺牲正确性换效率均不算增益。
4. 主辅消费需由真实成员按 exact member cwd 读取所分配的方法入口，观察两包建议冲突是否回到当前 case 规则；不扩大授权、团队或辅助写回资格。原会话恢复与用户纠偏规则保持有效，不另造记忆或交接文件。
5. readonly evaluator 仅证明其支持的有界静态材料/pack patch 场景，不证明实际 HTTP、工具执行、可见协作或用户纠偏。真实请求/独立 replay 分别走对应授权和环境；任何新校准仍需独立 Reviewer 判断覆盖与 currentness，不能借旧 bounded pass 自动晋级。
6. 所有失败、超时、负向与不确定结果保留；没有完成 live 时明确 pending。结果只写当前 roadmap，不能以机械全绿宣称研究更准、更快或 V2/V3/V4。

## Research execution — 专业实操、预测与联合研究

本节对应 `steamai-research-execution-v1`，不复用上一轮四题平局或旧 Reviewer report-copy pass 证明增益。

### 默认合同与脚本验证

`python -m unittest discover -s tests/pack_tooling -p "test_*.py"` 直接调用唯一 pack exporter；fake IDA 仅证明函数/调用点范围、条目与输出预算、输入状态漂移、路径和 no-overwrite 的机械边界，不加载真实 IDA、不调用模型或联网。`go test` 追加脚本/schema 在单包和主辅中的 production Fresh/Apply/current 固定与漂移拒绝、旧 current 零写及内容/路由合同。合法 stage-0 新文件的 `head-blob` 可缺省值，不能为通过测试提前提交；其它必需 identity 字段仍拒绝为空。

### 实际工具与联合交付

1. 在仓库外授权 fixture 的真实 Windows x64 IDA 数据库副本上，核对宿主/API、input 来源、image base、分析状态、函数/chunk/patch 条件，显式调用固定 exporter。正常定点导出可复查，缺函数、预算/状态漂移与输出冲突不能发布成功 packet。不启动目标、不写 analysis DB、不把宿主 autosave 或阻塞 API 宣称为脚本已隔离。
2. 使用实际原生 curl 执行单次 loopback GET；检查禁默认配置/proxy/globbing/redirect/retry/body、传输时限、已知/未知 body 长度超限、4xx/5xx body 保留与发送状态不确定。`--no-clobber` 改名不等于拒绝，要前检并核对 effective filename；退出码不独自证明未发送，header/整体进程资源不冒充 body 限额覆盖。
3. C 在同一主辅 case 贯通客户端字段/编码、真实同对象请求、API 业务结果与客户端解析。版本/对象错配、缺请求关联和客户端限制不能替代服务端约束；缺连接时联合 unknown。客户端执行与服务预置另有具体授权，不能由 IDA 只读范围推导。Go 编译 fixture 可用于准备客户端，不替代真实 IDA 导出。
4. 由正式可见 owner、最多一个 verifier 和只读 Reviewer 进行定向补证及整体 review；综合 F 直接绑定全部连接 E，子 F 不替代 E 的 currentness。实际用户纠偏、原生消息、会话恢复分别记录；自动 console 输入不算人工纠偏。

### 静态研究效果对照

在执行前冻结9道未用于调优的合成题：Binary/Web/joint 各有正向、反例、unknown。现行版本与最终版本分别 production Fresh，保持主辅配置、任务、模型、工具权限/预算 matched，实际加载各自 pinned 内容，隔离 native session/auto memory；不修改 current 或用临时附加几句指令冒充分发。每 arm 最多两轮，先冻结第一轮的检查选择/解释，再交付该选择对应的新材料；无效选择不默认补卡或重跑。不能额外提示“先写预测”来替代对自发行为的观察。

独立 Reviewer 依据固定 rubric 检查结论正确性、连接完整性、预测/反证响应、覆盖及过度断言；两个版本各自的实际选卡、所见材料和输出 SHA 都进入审阅输入。结果先按输出身份冻结，再解盲。全 unknown 不算通用高分；真正缺证的 unknown 应得到对应评价。平局、负向、失败、费用全部保留，已揭示题不用于调整方法后再刷同一验收。静态对照不证明实际工具效率、用户纠偏、V3 或 V4；真实工具可用与相对效果分开报告。状态只记录当前 roadmap。

### Windows 真实锁恢复子路径

`STEAMAI_WINDOWS_UPDATE_LOCK_LIVE=1` 时运行 `go test -count=1 -timeout=3m -run '^TestLiveWindowsUpdateLockedFileRecovery$' -v ./internal/steamai`，默认明确 skip。该 gate 在临时布局用真实 Win32 handle 锁调用生产文件恢复函数，检查 active/previous executable、published/backup source 的保留路径及关闭本轮 handle 后恢复。它不读写用户 Registry、安装或 case，不等于完整 ActivateUpdate/HKCU/Release journey；完整产品层仍须独立隔离环境验收。

### verified-learning 收尾

旧原件按外部冻结 hash 与 production verifier 定点核查；非 current 的旧测试 root 不补装 marker 或冒充正式 CLI case。report-copy calibration 不直接迁移到实际 pack 生成式比较；新 suite 必须与候选任务的 rubric、pinned contract、当前配置模型/CLI/platform/tool profile 一致，scenario 明确读取该 arm 的指导内容。语义裁决来自独立 Reviewer，不由 Go 拼出 expected class。

最终候选保持单主包、accepted source chain、eligible review 和 final exact patch。初始两对比较全部保留，预注册固定 anchor；当前 Gate3 只机械绑定一个 run，全部 pairs 的 path/SHA、盲审与汇总由不可变 Reviewer attestation 明确复核，不能挑赢家或声称 Go 已自动聚合。neutral、regressed、inconclusive、不完整或缺资格都不晋级。ABC 跨包源码改动不是一个 learning batch；真实 Apply 仍等用户对完整 preview 作 exact confirmation，计划批准不能代替。外部 synthetic clone journey 和真实 canonical 回流分别记录。

## 自主协作 — 只验证受影响的路径

本节对应 `steamai-autonomous-collaboration-v1`。默认测试检查 skill/case/角色的结果导向、自主方法、必要确认与审查边界；production Fresh 测试必须用真实仓库 tracked working-tree 模板，不用手写角色字符串或第二套 renderer 代替。检查真实文本进入 case 与各成员，后续 source 变化进入另一 Fresh，原 current 的身份与 bytes 不变。这只证明合同和分发，不证明模型行为。

实际行为需另行明确 case、具体动作与预算，使用原生可见会话，先只选一个受影响的任务，不重跑安装、恢复或完整联合历史旅程：

- 只给目标、材料、允许范围和退出条件，不额外指定工具顺序或提醒“自主选方法”；观察成员是否自行推进。简单任务不强配 verifier，确需帮助才直接请求有界协作，持续改派仍交 Commander。
- 已满足且边界未变的同一确认不重复索要；缺少动作许可、超预算、停止条件命中或用户纠偏时必须停在相应边界。不得为展示主动性扩大工具权限、自动 retry 或跳过 exact confirmation。
- 需要审查时由独立 Reviewer 判断 claim；最终综合审查对照原问题。证据不足时如实 unknown，局部 accepted 不充当整体完成；Reviewer 不审批每个普通探索步骤或接管取证。
- 记录任务质量、用户干预、无效步骤和全体参与者成本，保留失败和未知。没有比较就不宣称更准、更快或更省；如要比较，先冻结模型、材料、权限、总预算、隔离与停止规则，使用未用于调优的任务，不选成功回合。结果文件、模型自报和实际落盘状态必须分别核对；三者不一致时保留不一致，不以结构化自报覆盖实际文件。

这些是维护者的观察判据，不是成员日常必填清单。既有 readonly 双 arm evaluator 不扩成团队研究执行器；本节通过也不自动授予 V2/V3/V4 或 learning Apply 资格。

## 5. Release live

在测试tag上实际运行`.github/workflows/release.yml`并检查：

- `steamai-windows-amd64.exe`能在Windows 10/11 x64启动，`--version`等于tag；
- `steamai-release.json`使用固定schema，绑定tag、full revision和exe SHA-256；
- release资产下载后本地SHA与manifest一致；
- 无参数 `steamai update` 能通过 latest manifest 消费同一资产并精确绑定 tag/revision；
- workflow没有提交构建产物到Git，也不使用产品PowerShell/.cmd/.bat脚本。

只有实际GitHub Release成功才标记本层通过；workflow文件存在不等于release完成。

## 清理

停止测试session，删除临时case/source；通过Claude Code原生session管理清理synthetic persistent sessions。uninstall测试必须先确认保留checkout/case，再清理测试账户资源。摘要不得保留session ID、绝对路径、artifact内容或case-local hashes。
