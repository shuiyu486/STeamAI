package sourceartifact

import (
	"bytes"
	"os"
)

// SemanticText 消除仓库文本的换行表示差异。
func SemanticText(data []byte) []byte {
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
}

// CanonicalText 生成 Windows 新 case 写入使用的文本表示。
func CanonicalText(data []byte) []byte {
	return bytes.ReplaceAll(SemanticText(data), []byte("\n"), []byte("\r\n"))
}

// ReadCanonical 读取仓库文本并转换为 canonical case artifact bytes。
func ReadCanonical(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return CanonicalText(data), nil
}
