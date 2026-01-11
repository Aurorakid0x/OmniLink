package chunking

import "math"

// SimpleChunker 将文本切分为固定大小、带重叠的多个片段
type SimpleChunker struct {
	ChunkSize    int
	ChunkOverlap int
}

// NewSimpleChunker 创建一个切片器，并设置切片大小与重叠长度
func NewSimpleChunker(size, overlap int) *SimpleChunker {
	if size <= 0 {
		size = 500 // 默认切片大小
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= size {
		overlap = size / 2 // 避免步长为 0 导致死循环
	}
	return &SimpleChunker{
		ChunkSize:    size,
		ChunkOverlap: overlap,
	}
}

// Chunk 基于 rune（字符）数量切分文本，确保中文等多字节字符不会被截断
func (c *SimpleChunker) Chunk(text string) []string {
	if text == "" {
		return []string{}
	}

	// 转为 rune 切片，正确处理多字节字符（例如中文）
	runes := []rune(text)
	totalLen := len(runes)

	if totalLen <= c.ChunkSize {
		return []string{text}
	}

	var chunks []string
	step := c.ChunkSize - c.ChunkOverlap

	// 理论上构造函数已保证 step > 0；这里兜底，避免出现无法推进的情况
	if step <= 0 {
		step = 1
	}

	for i := 0; i < totalLen; i += step {
		end := int(math.Min(float64(i+c.ChunkSize), float64(totalLen)))

		// 提取切片
		chunkRunes := runes[i:end]
		chunks = append(chunks, string(chunkRunes))

		// 已到末尾则结束
		if end == totalLen {
			break
		}
	}

	return chunks
}