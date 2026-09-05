# STeamAI

STeamAI 是面向安全研究的、人在环的 Claude Code 多会话团队协作与经验学习层。一个真实项目目录对应一个明确授权的安全研究 case：用户主要指挥 Commander，也可以随时观察、暂停和纠偏屏幕上独立可见的正式成员会话。

Canonical repository：[`shuiyu486/STeamAI`](https://github.com/shuiyu486/STeamAI)。当前正式 Release 为 [`v1.0.4`](https://github.com/shuiyu486/STeamAI/releases/tag/v1.0.4)；v1 正式产品路径支持 Windows 10/11 x64。

## 安装

前提：本机已经安装并登录 Claude Code，且有原生 Git for Windows。

1. 从 GitHub Release 下载 `steamai-windows-amd64.exe`、`steamai-release.json` 与 `SHA256SUMS`，先按 `SHA256SUMS` 核验两个文件，再确认 manifest 中的 exe SHA-256 与实际 exe 一致。
2. 将 exe 放到临时目录并运行：

```text
steamai-windows-amd64.exe setup
```

setup 默认把 canonical checkout 放到 `%LOCALAPPDATA%\STeamAI\source`，把原生 `steamai.exe` 安装到 `%LOCALAPPDATA%\STeamAI\bin`，并只为当前用户加入 PATH。也可以绑定已经 clone 的普通 Git checkout：

```text
steamai-windows-amd64.exe setup --source <SOURCE_CLONE>
```

重新打开终端后即可使用。STeamAI 不安装 Claude Code、不管理登录、不启用全局 plugin，也不使用 PowerShell、`.cmd` 或 `.bat` 产品脚本。

## Quickstart

在明确授权的目标项目目录运行：

```text
cd <CASE_ROOT>
steamai
```

`steamai` 会启动当前目录中的 Commander Claude Code，并自动进入 `/steamai`：

- `.steamai-vnext/` 完全不存在：连续完成目标与授权澄清、初始组队、零写入 exact preview、用户确认、Fresh 创建和可见成员窗口启动；
- 完整有效的 `.steamai-vnext/`：继续当前 case，复用仍运行的 active 成员，按需重开未完成成员；
- partial、来源不明或冲突：fail-closed，不自动 repair、迁移、覆盖或删除。

Fresh 来源是 setup 绑定的 canonical checkout 当前 working-tree bytes；stage-0 index 定义 current tracked path/mode，HEAD 只是历史 anchor。因此已经审查、用户确认并应用但尚未 commit 的本机经验，也能供后续新 case 使用。preview 绑定 case facts、source records、目标 pre-state 和全部 writes；只有 `CONFIRM STEAMAI FRESH <identity>` 才能 Apply。

Apply 在同卷 sibling staging 中生成并验证完整 state tree，先 no-replace 发布并重验 project-local skill，最后发布包含 marker 的 `.steamai-vnext/`。case 建立后固定读取自己的 selected pack + 完整 `common/**` snapshot，不随 mutable canonical checkout 漂移。

普通项目只运行 `claude` 时，不会加载或携带 STeamAI。

## 团队模型

- **Commander**：理解目标与授权、按需组队、解决协作冲突、组织审查、集成交付和发起经验回流。
- **正式成员**：每名成员拥有 `.steamai-vnext/members/<name>/CLAUDE.md`，身份与当前任务属于该目录，不属于 session ID。Commander 通过原生 launcher 打开屏幕上独立可见的普通 Claude Code 窗口。
- **Reviewer**：只读 artifact/evidence/finding/spec/run/candidate/patch，只写 `reviews/` 与任务指定的 exact evaluation attestation，不执行 heavy action或运行 arms。
- active team 默认最多 3 名执行成员 + 1 名 Reviewer；每个问题一名 owner、最多一名 verifier。
- Claude Code 原生 session 是工作记忆，`ListAgents` / `SendMessage` 是协作通道，原生 logs/attach/resume/respawn 只作观察与恢复。STeamAI 不自建 task/session/message registry、队列或 supervisor。
- 用户在成员窗口里的直接输入优先；跨会话消息不能冒充用户纠偏、改派正式任务或扩大 case 授权。
- 同一 case 同时只允许一个 Commander；重复启动会拒绝第二个。

## 研究产物

```text
.steamai-vnext/
  CLAUDE.md
  pack-snapshot/
  contracts/
  members/<member>/CLAUDE.md
  artifacts/index.md
  evidence/E-*.md
  findings/F-*.md
  reviews/R-*.md
  reviews/R-L-*.md
  reviews/R-LB-*.md
  learnings/candidates/L-*.md
  learnings/patches/LB-*.patch
  evaluations/specs/*.md
  evaluations/specs/<suite-spec>.json
  evaluations/runs/<run-id>/manifest.json
  evaluations/runs/<suite-manifest>.json
  evaluations/attestations/*.md
  evaluations/outcomes/*.md
```

重要 finding 可按需升级为 proof-carrying replay；accepted review 只证明 V0。V1/V2/V3/V4 分别表示 mechanical、replay-backed、calibrated comparative 与 multiple field-observed 的已证明范围，不是自动状态机。

artifact index 记录 case-relative path、SHA-256、bytes、来源和授权范围；evidence 绑定 artifact；finding 引用 evidence；append-only review round 绑定 finding/evidence exact SHA。临时思考、聊天和长工具输出留在原生 session，不持久化为控制面状态。

## 经验批次回流

在现有 Commander 窗口说“整理并回流本 case 的经验”。需要比较 canonical checkout 时，用 Claude Code 原生 `/add-dir <CANONICAL_CHECKOUT>` 为同一 session 增加目录访问；用户不需要手工编辑 pack。

流程是：

1. 从 current accepted evidence chain 提炼任意多条 immutable candidate；每条只提出 selected pack 中一个 destination，并声明 `mechanical→V1`、`analysis-method→V2` 或 `behavioral→V3` 的最低 maturity。
2. Reviewer 逐条只做 eligibility，检查证据、通用性、反例、重复/冲突、脱敏与目标资格。
3. 将同主题 eligible candidates 组成一个或多个 batch；一个 batch 可修改同一 pack 中多个现有 Markdown targets。
4. Reviewer 独立绑定完整未截断 patch、candidate/review SHA、canonical HEAD、target pre/postimage；behavioral batch 还必须绑定经 native prepare→逐 slot run→finalize 闭合所有预注册独立 control patches 的 current `go` calibration、解盲前 exact blind decision、`pass`/`improved`/`V3` promotion、matched runtime/contract run bundle，以及该 run 对应的 exact `reveal.json` path/SHA，且被评估的是最终完整 patch，才可给出 accepted batch review。
5. 原生 helper 生成零写入 exact preview；只有 `CONFIRM STEAMAI LEARNING BATCH <identity>` 才会 Apply。
6. Apply 只改变 canonical working-tree targets；HEAD、index、当前 case snapshot 不变；失败只恢复本 batch targets；不自动 `git add`、commit 或 push。

未确认 candidate 留在来源 case，不扫描或汇总其它 case。verified-learning 引入前创建的 case 仍可作为 current case 继续研究，但不支持新版 learning preview/apply；helper 会在解析旧 artifact 前明确拒绝，不迁移或猜测旧字段。后续 field outcome 只由对应 case 用户逐份 opt-in，negative/inconclusive 也永久保留；单个 case 不足以 V4，也不建立 Hub、inbox 或经验数据库。已确认并应用的 working-tree bytes 可立即供本机后续 Fresh 使用；Git history 最终自然汇聚用户另行授权 commit/push 的通用经验。

## 更新与卸载

普通 `steamai` 不联网。显式更新到 GitHub 上的最新正式 release：

```text
steamai update
```

update 从 latest release manifest 取得 exact version/revision。请从 canonical checkout 之外的目录运行该命令；Windows 会阻止替换被当前终端占用的 source 目录。切换前要求 canonical checkout 无 staged、unstaged、untracked 或 ignored 本机内容，拒绝 release 未包含的任何本地 branch/stash commit，校验 exe SHA 与 `--version`，并验证 exact tag/revision 和 canonical identity。最终切换前再次绑定 HEAD、working tree 与本地 refs；任一漂移都停止。source 发生替换后，旧 checkout 以 sibling backup 保留并输出路径，不自动递归删除。它不自动 merge/rebase/stash/reset/restore/clean。需要保留的本地经验先做 local commit；若该 commit 尚未进入 release，则 update 会保守停止，push 仍不是本机消费经验的前提。

保守卸载：

```text
steamai uninstall
```

只删除已安装入口、setup 自己添加的 PATH 项和最小安装定位信息；默认保留 canonical Git checkout、未 push commits 和所有 case。当前 exe 先原子重命名，再由同字节的窄职责临时原生 helper 等待卸载命令退出后删除；helper 不执行其它产品职责，普通用户不依赖管理员权限或重启。Windows 锁语义会让该 helper 自身留在原安装目录，命令会显示其精确路径，进程退出后可手工删除。

v1 不支持 active case 跨电脑迁移、case import/export、云同步或 session 迁移器。新电脑只从 Release/Git 获取产品和已经 commit/push 的通用经验，用于新的 case。

## 维护与验收

- 产品入口：`cmd/steamai`、`internal/steamai/**`
- canonical/project-local skill：`.claude/skills/steamai/SKILL.md`
- case/研究模板与合同：`vnext/**`
- pack/common：`packs/<pack>/**`、`common/**`
- 文档路由：`docs/context-routing.md`
- 当前路线：`docs/verified-learning-roadmap.md`
- 已完成 Windows 产品基线：`docs/windows-native-product-roadmap.md`
- 自动与人工验收分层：`vnext/acceptance.md`

维护验证：

```text
go test -count=1 -p=2 -timeout=30m ./...
go vet ./...
git diff --check
```

fake process、synthetic fixture、cross-compile 或 workflow definition 都不能冒充真实 Windows setup/PATH、可见成员窗口、用户纠偏、跨会话消息、evaluator calibration/comparative result、field outcomes、learning apply 或 formal release 验收。Windows native test binary 可证明 suspended→Job→resume 的普通执行路径，但不能替代真实 timeout/process-tree cleanup live gate。默认测试不调用模型。

旧 Go control plane、mega CLI、project-local runtime、PowerShell façade、adapter host、legacy `/rekit` 与旧项目 importer 均已删除。STeamAI 不是自动逆向/漏洞挖掘或渗透引擎；危险动作仍受明确 case 授权、具体用户确认与 Claude Code 工具权限约束，`CLAUDE.md` 只提供上下文，不授予权限。
