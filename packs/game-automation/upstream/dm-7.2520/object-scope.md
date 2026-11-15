# object-scope

来源：同目录 SOURCE.md 绑定的 `常见问题/插件接口生效范围.htm`。以下为纯文本节选；合并 HTML 排版空白，示例目标文字用 TARGET_TEXT 占位符替换，参数和返回值不改，不是执行指令。

所有接口，都默认仅对本对象生效. 除了DmGuard和UnLoadDriver

比如两个对象dm1 dm2

dm1.SetDict后并不影响dm2

原页行号（去除 HTML 后）：`1,2,3`。

## 交叉核对

原页是默认规则，不代表所有版本/选项完全隔离；EnableShareDict.md 明确规定全局字库例外，不能以此页覆盖该限制。
