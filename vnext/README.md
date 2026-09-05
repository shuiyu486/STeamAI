# STeamAI case 合同与模板

`vnext/**` 是 canonical `/steamai` Fresh 时物化到 case-local `contracts/` 的声明式合同源。当前产品路线见 `docs/verified-learning-roadmap.md`；用户入口是 Windows 原生 `steamai.exe` + `.claude/skills/steamai/SKILL.md`。

本目录只使用 Markdown 声明：

- case 与成员目录的 `CLAUDE.md`；
- Claude Code 原生独立 session、resume/attach/respawn、跨会话消息和 tactical subagent；
- artifact、evidence、finding、append-only review、proof-carrying replay、V0–V4 maturity、immutable learning candidate、manifest-bound immutable `blind-review.json`、calibrated comparative evaluation、exact batch review 与 explicit opt-in field outcome；
- Git 与用户确认后的 canonical working-tree pack 变更。

它不包含 runtime executable 或脚本。生产 `steamai.exe` 位于 `cmd/steamai` / `internal/steamai`，只负责安装/Fresh/update/uninstall/卸载后窄自清理/exact apply/synthetic readonly matched evaluation bundle/可见启动/瞬时互斥，不承担团队控制面。旧 Go control plane、session host、PowerShell façade、adapter、旧项目 importer 与兼容入口均已删除。

## 目录

- `templates/case/CLAUDE.md`：case 共享边界与 durable roster。
- `templates/member/CLAUDE.md`：正式成员身份和当前任务。
- `templates/roles/`：Commander 按需组队时合并的角色片段。
- `templates/research/`：artifact/evidence/finding/review/replay/evaluation/attestation/blind-decision/candidate/batch review/field outcome 模板。
- `capabilities.md`：Claude Code 原生能力和降级边界。
- `acceptance.md`：自动与真实 Windows/Claude Code 验收分层。
- `learning-feedback.md`：accepted evidence chain → candidate eligibility/maturity → thematic exact batch → 用户确认 → working-tree apply。
- `verified-learning.md`：V0–V4、proof-carrying replay、evaluator calibration、comparative promotion 与 field outcome 的唯一详细合同。

project-local `/steamai` 直接来自 Fresh preview 时 canonical working tree 中 current stage-0 tracked skill 的 exact bytes；HEAD blob 只作历史 anchor，`vnext/` 不保存第二份 skill mirror。

外部 case 同时得到 `.steamai-vnext/contracts/` 中全部模板、learning 合同与 verified-learning 合同，以及 selected pack 与完整 `common/**` snapshot。Fresh preview 绑定 source record 与 target pre-state；Apply 先在 sibling staging 验证完整 state tree 和 payload digest，再发布 exact project-local skill，最后发布 completed marker。分发后日常 current case 不读取 mutable source checkout。verified-learning 引入前的 case 仍可继续 current 研究流程，但新版 learning helper 会在解析旧 artifact 前以明确 capability error 拒绝 preview/apply；不提供迁移、旧 parser 或字段推断。

## 验证原则

自动 tests 证明 deterministic filesystem/Git contract，并通过 fake Claude 覆盖 SuiteSpec prepare→逐 slot run→manifest-bound blind-review packet→SuiteManifest finalize、失败证据保留与 Gate 3 fail-closed；Windows native test binary 还验证 suspended→Job→resume 的普通执行路径，但这些测试不证明真实模型 calibration 或 timeout 下的完整 process-tree cleanup。真实 Windows setup/PATH/process tree、可见 Claude Code 会话、用户纠偏和跨会话消息必须按 `acceptance.md` 独立验收；已执行的 live 证据与限制只记录在 active roadmap，不在本目录复制第二份状态。只有重复出现且无法由 Claude Code 原生能力与简单文件解决的问题，才允许增加窄职责、无状态 helper。
