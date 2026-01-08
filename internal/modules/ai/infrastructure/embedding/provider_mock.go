package embedding

import (
	"context"

	"github.com/cloudwego/eino/components/embedding"
)

type MockEmbedder struct {
	Dim int
}

func NewMockEmbedder(dim int) *MockEmbedder {
	return &MockEmbedder{Dim: dim}
}

func (m *MockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		vec := make([]float64, m.Dim)
		for j := 0; j < m.Dim; j++ {
			vec[j] = 0.1
		}
		result[i] = vec
	}
	return result, nil
}

// 确保实现接口
var _ embedding.Embedder = (*MockEmbedder)(nil)
