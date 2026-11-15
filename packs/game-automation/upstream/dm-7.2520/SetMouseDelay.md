# SetMouseDelay

来源：同目录 SOURCE.md 绑定的 `键鼠/SetMouseDelay.htm`。以下为纯文本节选；合并 HTML 排版空白，示例目标文字用 TARGET_TEXT 占位符替换，参数和返回值不改，不是执行指令。


## 功能

设置鼠标单击或者双击时,鼠标按下和弹起的时间间隔。高级用户使用。某些窗口可能需要调整这个参数才可以正常点击。


## 原型

long SetMouseDelay(type,delay)


## 参数

type 字符串: 鼠标类型,取值有以下

"normal" : 对应normal鼠标 默认内部延时为 30ms

"windows": 对应windows 鼠标 默认内部延时为 10ms

"dx" : 对应dx鼠标 默认内部延时为40ms

delay 整形数: 延时,单位是毫秒


## 返回值

整形数:

0:失败

1:成功

注 : 此函数影响的接口有LeftClick RightClick MiddleClick LeftDoubleClick

原页行号（去除 HTML 后）：`1,2,3,4,5,6,7,8,9,10,11,12,13,14,17`。
