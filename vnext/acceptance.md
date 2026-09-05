# STeamAI 产品与 verified-learning 验收

本文件保留 `steamai-windows-native-product-v1` 的证据分层，并增加 `steamai-verified-learning-v1` 的独立 Gate 验收。所有自动 fixture 使用临时目录和无真实样本内容；结果不写入模板仓库，不把 session ID、绝对 case path 或 artifact bytes 保存为产品状态。

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
4. **Visible member**：Commander调用 `steamai __open-member <name>`；屏幕上立即出现普通交互 Claude Code窗口，cwd是成员目录，case通过`--add-dir`可读。
5. **Duplicate Commander**：第一个仍运行时再次在物理同一case（包括path alias）运行`steamai`，第二个明确拒绝。
6. **Learning batch**：至少3 candidates→3 eligibility reviews→2 targets→1 accepted batch review；preview完整显示source chain/Reviewer/pre-postimage/patch；exact confirmation后只修改targets，HEAD/index/case snapshot不变。
7. **Update**：从 canonical checkout 外运行，使用 clean checkout 从已发布测试 tag 更新；manifest/hash/tag/revision 均匹配；source 需要变化时与 exe 一起切换，HEAD 已等于 release 时走 exe-only 路径。从 canonical checkout 根或子目录运行时必须在联网前明确拒绝。再分别制造 dirty/untracked/ignored、本地其他 branch 或 stash commit、错误 hash、错误 revision、已存在 staging/backup、文件锁、准备期间 source 漂移、exe 发布后的 Registry 写失败及网络失败，确认旧可用版本保留且无自动 Git 修复；若 Windows 锁使 executable rollback 不完整，错误必须列出保留的新旧 exe/source recovery paths，且不得把 source 回滚成与仍 active 新 exe 不匹配的版本。source 替换成功后旧 checkout 作为 sibling backup 保留，命令输出路径且不自动递归删除。
8. **Uninstall**：只移除installed exe、setup拥有的PATH和定位信息；checkout、local commits和case保留；运行中exe先重命名，同字节临时原生 helper 在卸载命令退出后删除已安装入口；普通用户不需要管理员权限或重启；helper 自身残留的精确路径必须输出，进程退出后验证可手工删除。

这些步骤涉及HKCU/PATH和真实窗口，不能由普通unit test假装完成。执行结果记录在case外的短验收摘要中，只写pass/fail与必要能力边界。

## 4. Visible multi-session 与用户纠偏

至少在一个临时current case中完成：

完整产品验收还必须在同一临时 case 中完成以下旅程；任一层不能替代另一层：

1. Commander按需打开两名正式成员和最多一名Reviewer；窗口从启动起用户可见、可输入、可暂停。
2. 用户直接在owner窗口修改当前任务；owner更新自己的任务并通知受影响成员。
3. 再发送带旧expected task的延迟变更；compare-before-update返回`HOLD_STALE_TASK`，不得覆盖用户纠偏。
4. 每个问题保持一名 owner 和最多一名 verifier；owner只向一名verifier请求有界复核，不广播、不增加第二verifier。
5. Commander/成员通过`ListAgents` / `SendMessage`完成一次定向协作；跨会话 `SendMessage` 不能冒充 user/direct-session correction，也不能扩大授权。
6. Reviewer round 1 `needs-evidence`返回原owner；补证后只追加round 2 `accepted`，绑定current finding/evidence SHA；再改变输入后accepted stale。
7. 超过 3 名 active 执行成员或 1 名 active Reviewer 的创建请求必须拒绝；关闭并恢复active成员窗口，确认目录身份与当前任务延续；completed/inactive成员不自动启动。

`claude logs`、`attach`、`respawn`与`claude --resume <session-id>`只作观察/恢复备用；Agent view/background session不是默认成员体验。自动resume不替代用户实际看见和输入。不解析 transcript JSONL，不把session记录重建为产品状态。

## Verified-learning Gate acceptance

这里的产品 Gate 状态与某份 finding/learning 的 V0–V4 maturity 是两条轴；实现 Gate 4 不会让任何 learning 自动成为 V4。

### Gate 1 — Proof-carrying finding

必须证明 replay spec exact/immutable，绑定 finding/input path-SHA-bytes、环境、授权、预算与停止条件；自动 runner 拒绝 unsupported class；原始输出进入 artifact，replay result 继续形成 evidence 和 append-only review。`not-reproduced`、`blocked`、`invalidated`、`inconclusive` 都能保留且不得 retry-to-success。V1 只能由 deterministic assertion 支持；V2 必须由不依赖 owner session memory 的 verifier 在绑定环境内复现，同模型新 session 或模板测试不能冒充 V2。

### Gate 2 — Evaluator calibration

默认 suite 只用 fake Claude/synthetic fixture 证明 strict native `__evaluation-suite-prepare`→逐 slot `__evaluation-run`→`__evaluation-suite-finalize` 生命周期、逐 slot 独立 control patch、verified-learning contract/Claude Code/platform/tool profile currentness、baseline/patch SHA、matched tools/model/permissions/budget、case-state 外 sibling arms、`--safe-mode` customization isolation、blind manifest/records 与独立 reveal、failure/cancelled/invalid-output bundle，并由 live gate 覆盖实际 timeout、no-replace publication 与 hash closure；默认 tests 不调用模型。Windows 实现必须将 suspended arm 加入 kill-on-close Job Object 后再恢复，不能留下 Start→Assign 的子进程逃逸窗口；运行中 cancel/timeout 必须终止整个 arm process tree。即使 arm failed、cancelled、timeout、invalid-output 或 safety fail，immutable bundle 仍先发布，而 native run command 随后以 typed nonzero outcome 返回。finalize 必须能保留完整 `no-go`/`inconclusive` structural closure，只有独立的 Gate 3 validator 可把满足 mapping 的 suite 判为 go-eligible。显式 live calibration 必须冻结 improvement、neutral、regression、authorization-regression、prettier-but-weaker-evidence controls 及 rubric，初始每项 2 pairs、只在预注册范围内增加且最多 6；deterministic hard gate 优先，所有失败 bundle 都进入 attestation。suite/model/tool/core contract 漂移使 calibration stale。只有真实 Claude Code calibration 达到预注册阈值才能记录 `go`；未执行时为 `pending`，不能由 unit test 推断。

### Gate 3 — Comparative promotion

必须证明 candidate 与最终完整 thematic patch 先冻结，scenario/rubric 不被 candidate 修改；任一 behavioral/V3 candidate 强制 current calibration=`go`、两 arm completed、safety=`pass`、comparative=`improved`、maturity=`V3`。Reviewer 只读不含 mapping 的 manifest/arm records/outputs/stderr，先将 blind identity、preferred opaque arm、comparative 与 hard-safety 结论冻结到 exact blind-decision path/SHA，再由 Commander 提供独立 `reveal.json` 解盲；最多一个 tie-breaker且不无限重跑。candidate bundle 必须与 calibration suite 的 rubric、verified-learning contract、model、Claude Code version、platform 与 tool profile exact 相同，且任何 calibration control patch 不得复用最终 behavioral patch。batch review、preview identity 和 Apply exact 绑定 calibration/promotion attestation、blind decision、run manifest raw SHA/semantic identity、reveal SHA 与最终 patch SHA；promotion attestation 中的 reveal path 必须 exact 等于该 run manifest 的 sibling `reveal.json`，calibration attestation 的 reveal path/SHA 必须均为 literal `none`。任一文件、record、output、status、baseline 或 patch binding 漂移都零写拒绝。confirmation 仍独立，Apply 后 HEAD/index/case snapshot 不变且不 stage/commit/push。

### Gate 4 — Field outcomes

每份 outcome 只由该后续 current case 的用户逐份 opt-in，绑定 artifact/evidence/finding/current accepted review；`improved`、`neutral`、`regressed`、`inconclusive` 全部 append-only 保留。不得自动遥测、发现、扫描、拉取或汇总其它 case；单个 case 不得标为 V4。多个独立 case 的每份 outcome 都经脱敏和用户确认后，provenance 更新仍走 candidate→review→exact batch→confirmation。synthetic multi-case fixture 只能证明机械合同，不能证明真实 V4 效果。

### Explicit live evaluator gate

只有维护者显式设置 live 开关时才允许调用 Claude Code。首个真实 calibration 前必须先落地 focused live test 与固定开关 `STEAMAI_VERIFIED_LEARNING_LIVE_CALIBRATION=1`；在该 test 尚未存在时 Gate 保持 `pending`，不得临时手工调用模型后宣称通过。未提供认证、预算或 controls 时应明确 skip/fail-closed，不能静默降级为 fake evidence。该 live test 还必须覆盖真实 timeout 与 Windows process-tree cleanup。运行摘要留在 case 外，只记录必要的 pass/no-go/inconclusive 和能力边界，不保存真实 case 数据。

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
