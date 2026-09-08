# STeamAI 专业实操与联合研究路线

## 读取指南

- 路线 ID：`steamai-research-execution-v1`。
- 当前状态：`A/B/C 实现完成；实际 IDA/IDAPython 定点导出与 14 条真实 loopback/client 路径已验，研究对照按超时停止规则闭合为 incomplete/inconclusive；可见成员验收已证明真人纠偏、命名可见会话与原生消息身份链，两个独立短 gate 又分别闭合 stale-task HOLD 零覆盖，以及联合 E/F/R 的 needs-evidence→单次补证→append-only round 2；最终 accepted 只覆盖明示来源映射范围内的记录级联合解释`。
- 本文保留本路线已验事实和未完成结论，不续写新实验日志；当前路线见 `docs/research-validity-roadmap.md`，完整路由在 `docs/context-routing.md`，短投影在 `docs/batch-plan.md`。
- 旧 `research-capability` 的方法/主辅与32轮平局事实不改写；verified-learning 正式证据与待验项仍归 `docs/verified-learning-roadmap.md`，不预填 go。

## 目标与用户画面

1. A：通过实际工具取得一段能改变判断的新证据，不只说明缺什么。Binary 是已打开的授权 IDA 数据库副本定点导出，Web 是原生 curl 显式单次 loopback GET。
2. B：说明少量处理/状态步骤为什么相连，并以尚未查看的分支检验解释。单点索引不强制增加格式。
3. C：同一 case 联合客户端/本地模块与 API 证据，核对版本、对象、编码、请求、服务结果和客户端解释，不把两个局部 accepted 合并成整体 accepted。
4. 独立成员、原生 session/消息、任务单写和用户纠偏不变；一主最多一辅，learning 仍只回主包。

## 批次与边界

| 批次 | 内容 | 状态 |
|---|---|---|
| RE-01 | 工具与模型前置、三类切片和新保留题冻结 | 9题及当前high配置已冻结，真实身份/Read范围/resume通过；IDA 9.3、Python/IDAPython 与官方 MCP 入口已核验 |
| RE-02 | 唯一 IDA 定点 exporter、schema/直接测试与 curl recipe | 实现与27项 Python tests 通过；真实 curl 11场景及 IDA/IDAPython 定点导出通过 |
| RE-03 | Binary/Web 跨步骤连接与预测验证 | 方法与相关内容合同已实现；原研究效果对照已因盲评超时闭合为不完整，不能在解盲后补跑 |
| RE-04 | 单源 client/API 联合方法与真实双包 case | 方法及3条真实合成客户端/API路径已验；可见成员联合 gate 完成初轮 E1/E2/F、Reviewer `needs-evidence`、唯一 E3 补证、同一 F/index 更新与 append-only round 2 `accepted`；接受范围不含客户端采用或内部机制 |
| RE-05 | 默认回归、真实工具/成员旅程、9题独立对照 | 36次研究调用完成，Q02盲评超时后按冻结规则闭合为 incomplete/inconclusive；真人纠偏、命名可见会话、原生消息链及独立 stale HOLD 零覆盖 gate 已验；不再重复整趟旅程 |
| VL-CLOSE | 旧证据离线核对、已知报告 Reviewer 校准（report-copy）/原计划最终单主候选、锁文件恢复 | 旧原件/锁恢复通过；新20运输 arms 经 byte-exact 复核后仅执行10次首次 Reviewer，2个有效、8个协议无效，closure 为 complete/inconclusive；无 go、candidate、preview 或 Apply |

### 唯一定点脚本

`packs/binary-re/tooling/scripts/export_function_evidence.py` 显式运行于已授权的 IDA Windows x64 稳定数据库副本，仅读取指定函数/最多两个调用点和有界引用；不安装或启动宿主，不分析/执行目标，不写 IDB，不默认调用反编译器。范围、条目、输出及合作式截止必须检查；读取失败或漂移不发布成功结果。宿主后台行为、autosave 与阻塞 API 不由 Python 脚本假装强隔离。

脚本是 pack 工具，不是 steamai façade、adapter host 或通用 runner；原生 Fresh 已可固定 tracked regular 资产，不新增快照格式。`vnext/**` 仍仅 Markdown，catalog 不执行，learningTargets 不含脚本/schema。

### 证据与联合判断

综合 F 直接绑定全部支撑连接的 E，R 审查连接与整体 claim，局部 F 只导航；不假设存在 F→F 的自动传递 currentness。事后解释不等于预测，反复读同源导出不等于独立复现。pack revision/digest 不证明被研究客户端/API 的版本对应。缺连接时保留局部结论并给 joint unknown，不扩权限取得确定答案。

C 首批为字段构造/编码→单次 GET→权限结果→客户端解析，受控 fixture 预置状态。不因多步研究引入 POST/body、批量请求、自动 retry 或多步 schema；客户端执行必须单独在具体动作范围内，IDA 只读许可不授权运行。

## 验证

默认不启动模型、IDA、HTTP 或可见会话：

```text
python -m unittest discover -s tests/pack_tooling -p "test_*.py"
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

真实层分别证明：实际 IDA 定点导出；真实 loopback GET 与业务对照；客户端/API 联合链；原生可见成员、定向补证与整体复审。fake IDA、Go fixture、文本断言、同源重复导出不替代对应实机层。人工纠偏只能由实际用户输入证明，不以自动 console 输入或成员消息冒充。

效果对照使用9道未用于调优的静态题（Binary/Web/joint各正向、反例、unknown），当前基线与最终版共18个独立case/arms，每arm最多两轮，另9次盲评。运行前冻结题目、标准、current配置解析后的模型、预算与真实pinned内容；隔离session/auto memory，不改全局配置。任务不额外提示预测步骤，实际行为与结论分别检查。全unknown不算改善，正确限定缺证不扣成错误；负向、平局、失败和费用全部保留，不重跑刷强。

批准的名义总预算为40美元以内，计划分项38.60美元；CLI budget不是账单硬上限，不完整的interactive费用须明确。身份不符、硬边界失败或达到预算就停止，不切模型、不降标准。静态比较不证明实际工具效率或普遍能力；V3/V4资格仍遵循各自证据。

## 当前证据与未完成条件

- 2026-09-06 实施基线为 `96e33591`，开始时工作区干净。原生工具版本已核对：curl 8.13.0、Go 1.26.3 windows/amd64、Python 3.10.5、Claude Code 2.1.236；随后首个模型身份探针通过，实际 assistant 为 `gpt-6-astra`，2.322秒、CLI报告费用 `$0.006235`，无工具调用。
- 2026-09-07 用户提供合法 IDA 9.3 入口后，已并存安装 Python 3.12.10 与 uv 0.12.10，保留 Python 3.10.5；`idapyswitch` dry-run/实际绑定核验通过，GUI 日志确认 Python 3.12.10 / IDAPython 9.3.0。官方 `mrexodia/ida-pro-mcp` 包已安装，Claude Code 以 RE 目录限定的 stdio `steamai-ida-local` MCP 连接，RE 外会话不加载；MCP 环境按插件锁文件使用隔离 CPython 3.11.16，`idapro` 初始化返回 IDA library `9.3.251224`。当前会话又通过 MCP 完成合成 PE 的 `open → lookup → close(save=false)`，不是只注册名称。安装和验收未读取或运行同目录无关第三方脚本，也未改 STeamAI 产品路径。
- 实际 IDA live 使用仓库外新建的合成 Go Windows x64 PE（input SHA `80dcf74c…`），先由官方 MCP 的 headless idalib 完成 auto-analysis、Hex-Rays/cache warmup、`idb_list` 与不保存关闭；再在 IDA 9.3 GUI 的真实 IDAPython 会话对稳定 `.i64` 副本运行 production `export_function_evidence.py`。目标 `main.transform` 范围 `0x140082260–0x140082298`，成功输出19条指令、39 entries、4695 bytes，artifact SHA `8a1e1171…`；记录的 PE/x64/metapc、input SHA、image base、analysis ready、debugger inactive 均通过，磁盘 `.i64` 前后 SHA 均为 `cb0d85da…`。GUI batch 脚本完成并写出成功 receipt 后，IDA 进程未自行退出，外层300秒等待到期才定点终止该测试 PID；无残留进程，不能把自动退出记为通过。该证据证明本 exporter 的当前 IDA/IDAPython 实机路径可用，不证明目标执行、动态行为、其它架构或任意数据库兼容。
- 新资产的 production Fresh 回归实际暴露并修复空 `head-blob` 解析问题：stage-0 新文件合法没有 HEAD blob，旧解析先 TrimSpace 删除分隔空格，导致 Apply 后 current 深验失败。现在先解析分隔再规范化值，保留合法缺省 HEAD anchor；其它空必需字段与畸形记录仍拒绝。新脚本/schema 主辅两个方向的 committed/staged-new 固定、漂移拒绝与旧 literal current 零改写 focused tests 通过（35.420 秒），没有提前提交 fixture 掩盖新资产路径。
- Windows 真实共享锁的生产文件恢复子路径通过：active/previous executable、published/backup source 四个场景，确为 Win32 handle 限制而非注入错误；解除本次 handle 后旧 bytes 恢复。未调用会读取 HKCU 的完整 ActivateUpdate，因此不代表完整安装、Registry 或 Release 验收；显式 gate 与范围见 `vnext/acceptance.md`。
- B/C 三篇方法与共享路由已写入，8 项直接相关的模板/原生成员/方法合同 focused tests 通过，三篇均小于16 KiB。唯一 IDA exporter 的26项 Python tests 通过（0.044 秒），新增 ResearchExecution 合同通过（0.012 秒）；fake IDA/内容测试不证明实际 IDA 或研究行为。
- 真实原生 curl 11场景与合成客户端3场景共14次 loopback GET 全部符合冻结判据：身份、动态字段、合法路径回归、对象/版本错配、禁止跟随 redirect、已知/未知长度 body 超限、送达后 timeout、输出 collision 均保留。chunked 超限 exit63、部分 body 为262144 bytes；1秒测试 deadline 的 timeout 为1.016秒/exit28，服务端记录确认送达，无重试；collision 保留原件并识别数字后缀，不接受为 exact target 成功。3条客户端运行观察为200/403/409与loaded/denied/version-mismatch；不提供 IDA 静态连接证明。40个引用文件 SHA/bytes 复核一致，receipt SHA `f7d9a54560d37a821a3887e407c833b0e10aca19b58213f6c53e77547d576c28`，本轮服务已停止。此层不证明 LLM 联合研究、独立 Reviewer 或用户纠偏。
- 独立9题及标准在方法实施之外冻结，freeze SHA `dd02e5e8b6ef9b14cbbc4dc11b1631c9050155d4a0f759f70898b4e405ba7d7c`；每 arm R1 从3张索引选1张，R2只见所选卡。评分按实际所见材料，题作者了解批准主题但未看新实现，不属于外部基准；未先试题调难度。首个配置冻结为 `gpt-6-astra[1m]`、CLI2.1.236、effort xhigh，并由真实探针的 assistant 消息核验。网络中断恢复后只读核对发现当前 effort 已为 high，模型环境选择未变；旧身份文件和探针保留，后续调用须按当前配置另行冻结/核验，不混用不同 runtime 的证据。
- 独立审查发现并修复 IDA recipe 加载污染：默认 SourceFileLoader 会在 pinned scripts 目录写入 bytecode，导致 current 路径集合漂移；现直接 `runpy.run_path` 加载 exact `.py`，不更改全局配置或放宽 current。新增测试执行 recipe 的实际加载片段及生产 main，旧加载方式红测、新方式绿测，27项 Python tests 通过（0.070秒）；仅 mock IDA，不代表真实宿主。Fresh 与 learningbatch 两个完整包回归分别通过（106.977秒、111.554秒）。
- 仅含交付源码的隔离副本完成全量回归：272个stage-0文件，明确列入11个新增交付文件，排除临时验收源码及bytecode；main的index未变。27项 Python tests（0.072秒）、`go test -count=1 -p=2 -timeout=30m ./...` 全部包、`go vet ./...` 与暂存差异检查通过，均未启动模型/IDA/HTTP/可见会话。最后的证据状态文档更新不改变方法与生产代码，交付前仍核对差异。
- 当前 high runtime 的独立身份探针通过（13.374秒、$0.006240）；真实 Read 范围/两轮 resume 通过（22.190秒＋11.861秒、$0.052475）：case内读取成功、已存在的外部无害文件明确因 dontAsk 权限拒绝，第二轮同session仅读新材料并回忆前轮随机值。后续 Windows 消息验收改用并冻结较保守的 Claude Code 2.1.250（exe SHA `63403e00…`）；`--name` 基础启动门槛为2.1.76，完整 Windows 原生跨会话消息门槛为2.1.248。CLI 自动更新已关闭、插件更新保留；这不改写此前 2.1.236 产生的历史证据。前置专项已知费用共$0.064950；不含开发会话或未知interactive费用。
- 可见联合旅程的失败按 one-shot 原样保留，不覆盖或伪装通过。Attempts 01/02 暴露主题/trust 页折行识别问题；后续归一化匹配与显式 Windows `VK_DOWN` 修复后，Claude Code 2.1.250 上的 production `__open-member` 已把 Reviewer、Verifier、Owner 启动为三个独立可见、带 `--name <member>` 的普通会话并全部 READY。Attempt 05 由用户亲自在 Owner 窗口输入 exact 纠偏，Owner 仅在“目标”开头加入指定预测前缀，回复 `HUMAN_CORRECTION_APPLIED`；Owner 文件 SHA 固定为 `02fbe923…`，case 仍 current。Attempt 08 又以同配置域 `claude agents --json` 的 exact cwd→name、`ListAgents` 当前可达 name、`SendMessage success:true` 和接收方 incoming origin 四层证据，证明 Owner→Reviewer（`msg_id=80f904a8…`）与 Owner→Verifier（`msg_id=5fb6dfa0…`）两条纠偏通知真实送达；离线复核确认发送与接收的 msg_id/name/body 一一相同。一次性 Commander→Owner stale 消息也得到 `success:true`（`msg_id=2002f093…`）且 Owner transcript 有同 id/body 的 incoming record，但控制器因发送者把 executable 写成带引号的等价命令而提前拒绝并清理成员，Owner 来不及产出 `HOLD_STALE_TASK`。因此该消息只能证明送达，不能证明 compare-before-update HOLD。Attempt 08 result SHA 为 `6db47a90…`；sender/Owner/Reviewer/Verifier 冻结 transcript SHA 分别为 `154f0e24…`、`3708e0f0…`、`4d59204e…`、`bb6938b8…`。case 零研究产物改动、Owner bytes 保留且 production InspectCurrent 通过。这里的“窗口可见”仅来自 Win32 `isWindowVisible=true`；用户没有另行确认实际看见 Attempt 08 的三个窗口。
- 删除未启动且过重的 Attempt 09 后，剩余行为拆成两个仓库外短 gate 实际运行。`stale-hold-gate-01` 以 production launcher 启动 Owner；sender 依次执行 exact `claude agents --json`、`ListAgents`、`SendMessage`，`msg_id=bb679337…` 的正文在 Owner incoming 中逐字匹配，Owner 返回 `HOLD_STALE_TASK`，纠偏后任务 SHA 仍为 `02fbe923…`，case 文件零变化，前后 InspectCurrent 均通过；result SHA `e26c0b3a…`、9081 bytes。`joint-research-gate-01` 复用同一合成 current case 的三个 production 命名可见成员：Owner 在首次读取三个 client stdout 前固定 URL 预测，Reviewer round 1 实际为 `needs-evidence`；仅据该真实缺口启动唯一 Verifier，Owner 在读取新服务端材料前再固定条件预测，Verifier 只写 E-VIS-003，Owner 更新同一 F 与 index，Reviewer 将 round 2 追加到同一 R。round 1 的 `66590051…`/10173 bytes 是最终 R 的 exact 前缀，追加12786 bytes；round 2 实际为 `accepted`，只接受 `sourceMapping` 明示范围内的“客户端自报告→服务端请求/解码对象记录→正文代表”连接，客户端接收/采用、真实认证/归属、部署身份、映射生成追踪和内部机制仍 unknown。最终 F/R SHA 分别为 `67d0c96c…`/`048155e4…`；14条 SendMessage 均有 `success:true` 及唯一 exact msg_id/body incoming，角色写路径合规，case current，三个 gate session 已定点关闭且配置域剩余0；Verifier 回传使用请求的 exact incoming endpoint，未重复 cwd→name discovery，因此不把返回方向写成再次完成双来源寻址。final result SHA `e8309545…`、11852 bytes。
- 18个production Fresh对照case各两轮，共36次研究调用全部completed，1164.43秒，CLI报告研究费用$1.966009。两版本同用对应pack README条件路由，没有单给新版预测提示。匿名盲评仅第一题完成（49.78秒、$0.106692），机械裁决为tie；第二题唯一一次Reviewer在180秒timeout，无stdout/actual-model/费用记录，不能把超时归因为已证实的模型质量问题。随后按冻结规则记录 `paid-stopped-after-review-Q02-timeout`，将其余7题闭合为 `not-comparable` 并生成解盲结论；现在补跑会发生在解盲之后，违反 no-rerun 与“先冻结完整评分、后解盲”，故该 suite 永久保持 incomplete/inconclusive，不再续跑。独立复核确认首题12:12 tie与原材料、观察前预测及范围限定一致，无hard failure；36次研究记录与首题Reviewer共97次Read全部与相应文件bytes一致、无offset/limit、完整行数，无截断。Q02只有零字节stdout与deadline错误，原因未知，不能仅据共有的unrecognized_model警告归因为网络/CLI/模型拒绝。
- 已知专项费用至少$4.454090（两runtime探针、scope两轮、36研究轮、第一题盲评、新学习20运输arms、10次首次Reviewer、两次exact permission probe、Attempt 08一次性stale sender，以及两个短 gate 的已知 print sender `$0.421828`）；第二题timeout、交互式可见成员和onboarding费用未知，不能当0或完整账单。没有重跑已执行题、换模型或降标准。全部临时repo验收代码按exact bytes留在仓库外，未发布 Release 或执行真实 learning Apply。
- 实际 IDA、独立 stale HOLD 零覆盖、联合 E/F/R、条件性单 Verifier 补证和 append-only 整体复审均已闭合；原研究效果对照仍按冻结停止规则闭合为 `incomplete/inconclusive`，不能在解盲后补跑剩余题，也不能用一次有界 joint `accepted` 改写为“整体研究能力已证明提升”。过重且未启动的 Attempt 09 仓库外脚手架已删除；`vnext/acceptance.md` 现明确按行为边界拆短 gate，不要求一个外部控制器同步驱动全部窗口，且 Reviewer round 2 decision 必须由证据决定、不预填 accepted。旧学习本轮可完成的首次 Reviewer 与 structural closure 已完成但结果为 `inconclusive`，详见 `docs/verified-learning-roadmap.md`；新的研究增益或 learning `go` 证据只能使用另行冻结的新 suite。
