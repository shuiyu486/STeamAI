# FindStr

来源：同目录 SOURCE.md 绑定的 `文字识别/FindStr.htm`。以下为纯文本节选；合并 HTML 排版空白，示例目标文字用 TARGET_TEXT 占位符替换，参数和返回值不改，不是执行指令。


## 功能

在屏幕范围(x1,y1,x2,y2)内,查找string(可以是任意个字符串的组合),并返回符合color_format的坐标位置,相似度sim同Ocr接口描述.

(多色,差色查找类似于Ocr接口,不再重述)


## 原型

long FindStr(x1,y1,x2,y2,string,color_format,sim,intX,intY)


## 参数

x1 整形数:区域的左上X坐标

y1 整形数:区域的左上Y坐标

x2 整形数:区域的右下X坐标

y2 整形数:区域的右下Y坐标

string 字符串:待查找的字符串,可以是字符串组合，比如"TARGET_TEXT_A|TARGET_TEXT_B|TARGET_TEXT_C",中间用"|"来分割字符串

color_format 字符串:颜色格式串, 可以包含换行分隔符,语法是","后加分割字符串. 具体可以查看下面的示例 .注意，RGB和HSV,以及灰度格式都支持.

sim 双精度浮点数:相似度,取值范围0.1-1.0

intX 变参指针:返回X坐标 没找到返回-1

intY 变参指针:返回Y坐标 没找到返回-1


## 返回值

整形数:

返回字符串的索引 没找到返回-1, 比如"TARGET_TEXT_A|TARGET_TEXT_B",若找到TARGET_TEXT_A，则返回0

注: 此函数的原理是先Ocr识别，然后再查找。所以速度比FindStrFast要慢，尤其是在字库

很大，或者模糊度不为1.0时。

一般字库字符数量小于100左右，模糊度为1.0时，用FindStr要快一些,否则用FindStrFast.

原页行号（去除 HTML 后）：`1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,33,34,35`。
