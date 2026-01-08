package vectordb

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// Ensure MilvusStore implements Eino interfaces
var _ indexer.Indexer = (*MilvusStore)(nil)
var _ retriever.Retriever = (*MilvusStore)(nil)

// Store implements eino.Indexer
// 这让 Eino 的 Graph 可以直接把处理好的 Document 扔给 Milvus
func (s *MilvusStore) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) {
	if len(docs) == 0 {
		return []string{}, nil
	}

	upsertItems := make([]UpsertItem, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if doc.ID == "" {
			return nil, fmt.Errorf("document missing ID")
		}

		vec64 := doc.DenseVector()
		if len(vec64) == 0 {
			return nil, fmt.Errorf("document %s missing dense vector", doc.ID)
		}
		if len(vec64) != s.vectorDim {
			return nil, fmt.Errorf("document %s vector dim mismatch, got=%d want=%d", doc.ID, len(vec64), s.vectorDim)
		}
		vec32 := make([]float32, len(vec64))
		for i := range vec64 {
			vec32[i] = float32(vec64[i])
		}

		upsertItems = append(upsertItems, UpsertItem{
			ID:     doc.ID,
			Vector: vec32,
			Content: doc.Content,
		})
	}

	return s.Upsert(ctx, upsertItems)
}

// Retrieve implements eino.Retriever
// 这让 Eino 可以直接调用 Milvus 进行搜索
func (s *MilvusStore) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	// 1. 获取查询向量
	// 注意：Retriever 接口通常输入是 query string。
	// 但 Milvus 需要向量搜索。
	// 在 Eino 中，Retriever 前面通常也有一个 Embedder。
	// 这里我们需要从 opts 或者上下文中获取 query 的向量，或者这个 Retriever 应该被定义为 "VectorRetriever"

	// Eino 的设计中，Retrieve 接口输入是 string query。
	// 如果是 dense vector search，通常 Retrieve 内部会调用 embedder，或者 Eino 提供了这种组合。
	// 简单起见，我们假设 opts 里传了 vector，或者我们先留空，等待 Pipeline 组装时再完善。

	return nil, fmt.Errorf("not implemented: raw query string search requires internal embedding")
}
