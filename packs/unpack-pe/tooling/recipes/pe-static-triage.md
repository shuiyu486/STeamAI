# Pe Static Triage

## 适用范围

用于 PE static triage、loader behavior、unpack candidate 与 import recovery evidence 中的窄范围 `pe-static-triage` 任务。只处理 case-local 脱敏引用和已存在的有界输入。

## 输入

- exact scope 与要回答的问题；
- case-local artifact alias；
- 允许读取/写入范围；
- 时间、请求、空间或输出预算；
- 停止条件。

## 步骤

1. 验证输入属于当前明确授权的 case，并记录来源与完整性信息。
2. 优先执行只读、静态或 dry-run 观察；禁止无关扩展。
3. 将关键观察写入 evidence，包含定位、方法、限制和不确定性。
4. 需要 heavy action 时，先向用户展示具体动作、目标、预算、副作用和停止条件；得到针对该动作的确认且工具权限允许后才执行。
5. 输出 finding candidate 或 `needs-evidence`，不自动修改原 artifact 或共享 pack。

## 停止条件

授权、目标或输入身份不清；范围漂移；预算耗尽；出现意外副作用；输出可能含真实对象、凭据、客户信息或不适合进入团队文档的敏感内容。
