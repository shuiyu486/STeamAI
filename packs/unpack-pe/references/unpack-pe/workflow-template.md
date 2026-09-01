# Authorized PE unpacking research workflow

1. **确认边界**：记录目标、授权范围、禁止事项、预算和停止条件。
2. **索引输入**：只为 case-local artifact 建立 alias、相对路径、SHA-256、bytes 与来源说明；不复制真实 artifact 到 pack。
3. **静态优先**：先执行无副作用的最小观察，生成可复查 evidence。
4. **形成 finding**：声明结论、置信度、evidence 引用、限制和尚未证明部分。
5. **有界验证**：需要第二视角时指定一名 verifier；需要 heavy action 时先向用户展示具体动作、目标、预算、副作用和止损条件。
6. **独立审查**：重要 finding 或交付由 Reviewer 给出 `accepted`、`needs-evidence`、`disputed` 或 `superseded`。
7. **交付与学习**：只交付可追溯结论；只从 accepted finding/review 提炼脱敏 learning candidate。

领域重点：PE static triage、loader behavior、unpack candidate 与 import recovery evidence。任何新证据超出原授权或暴露新的外部副作用时立即停止并升级给 Commander。
