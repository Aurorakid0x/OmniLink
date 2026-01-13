package chunking

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

// SimpleChunker 将文本切分为固定大小、带重叠的多个片段
type SimpleChunker struct {
	ChunkSize    int
	ChunkOverlap int
	useRecursive bool

	initOnce      sync.Once
	initErr       error
	recursiveImpl document.Transformer
}

// NewSimpleChunker 创建一个切片器，并设置切片大小与重叠长度
func NewSimpleChunker(size, overlap int) *SimpleChunker {
	if size <= 0 {
		size = 500
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= size {
		overlap = size / 2
	}
	return &SimpleChunker{ChunkSize: size, ChunkOverlap: overlap}
}

func NewRecursiveChunker(size, overlap int) *SimpleChunker {
	c := NewSimpleChunker(size, overlap)
	c.useRecursive = true
	return c
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

func (c *SimpleChunker) ChunkDocuments(ctx context.Context, docs []*schema.Document) ([]*schema.Document, error) {
	if len(docs) == 0 {
		return []*schema.Document{}, nil
	}

	if !c.useRecursive {
		out := make([]*schema.Document, 0, len(docs))
		for _, d := range docs {
			if d == nil {
				continue
			}
			parts := c.Chunk(d.Content)
			for i, p := range parts {
				n := &schema.Document{Content: p, MetaData: map[string]any{}}
				for k, v := range d.MetaData {
					n.MetaData[k] = v
				}
				n.MetaData["chunk_index"] = i
				out = append(out, n)
			}
		}
		return out, nil
	}

	c.initOnce.Do(func() {
		impl, err := recursive.NewSplitter(ctx, &recursive.Config{
			ChunkSize:   c.ChunkSize,
			OverlapSize: c.ChunkOverlap,
			Separators:  []string{"\n\n", "\n", "。", "！", "？", "；", "，", " "},
			LenFunc: func(s string) int {
				return len([]rune(s))
			},
			KeepType: recursive.KeepTypeEnd,
		})
		if err != nil {
			c.initErr = err
			return
		}
		c.recursiveImpl = impl
	})
	if c.initErr != nil {
		return nil, c.initErr
	}
	if c.recursiveImpl == nil {
		return nil, fmt.Errorf("recursive splitter not initialized")
	}

	out := make([]*schema.Document, 0, len(docs))
	for _, d := range docs {
		if d == nil {
			continue
		}
		frags, err := c.recursiveImpl.Transform(ctx, []*schema.Document{{Content: d.Content}})
		if err != nil {
			return nil, err
		}
		for i, f := range frags {
			if f == nil {
				continue
			}
			n := &schema.Document{Content: f.Content, MetaData: map[string]any{}}
			for k, v := range d.MetaData {
				n.MetaData[k] = v
			}
			n.MetaData["chunk_index"] = i
			out = append(out, n)
		}
	}
	return out, nil
}
