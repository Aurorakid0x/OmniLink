package vectordb

import (
	"context"
	"fmt"

	"OmniLink/internal/modules/ai/domain/repository"
)

// MilvusVectorStore 是 domain 层 repository.VectorStore 的 Milvus 实现（通过适配 MilvusStore）。
//
// 分层关系：
// - milvus_store.go：Milvus SDK 底层封装（UpsertItem/SearchHit），不依赖 domain。
// - milvus_vector_store.go：实现 domain 接口 repository.VectorStore，把 domain 类型映射到 milvus_store.go。
//
// 这样 application/pipeline 只依赖 repository.VectorStore，底层可替换（Milvus/pgvector/ES 向量等）。

type MilvusVectorStore struct {
	store *MilvusStore
}

var _ repository.VectorStore = (*MilvusVectorStore)(nil)

func NewMilvusVectorStore(store *MilvusStore) (*MilvusVectorStore, error) {
	if store == nil {
		return nil, fmt.Errorf("milvus store is nil")
	}
	return &MilvusVectorStore{store: store}, nil
}

func (s *MilvusVectorStore) Upsert(ctx context.Context, items []repository.VectorUpsertItem) ([]string, error) {
	if len(items) == 0 {
		return []string{}, nil
	}
	upserts := make([]UpsertItem, 0, len(items))
	for _, it := range items {
		upserts = append(upserts, UpsertItem{
			ID:           it.ID,
			Vector:       it.Vector,
			TenantUserID: it.TenantUserID,
			KBID:         it.KBID,
			SourceType:   it.SourceType,
			SourceKey:    it.SourceKey,
			ChunkID:      it.ChunkID,
			Content:      it.Content,
			MetadataJSON: it.MetadataJSON,
		})
	}
	return s.store.Upsert(ctx, upserts)
}

func (s *MilvusVectorStore) DeleteByIDs(ctx context.Context, ids []string) error {
	return s.store.DeleteByIDs(ctx, ids)
}

func (s *MilvusVectorStore) Search(ctx context.Context, vector []float32, topK int, expr string) ([]repository.VectorSearchHit, error) {
	hits, err := s.store.Search(ctx, vector, topK, expr)
	if err != nil {
		return nil, err
	}
	out := make([]repository.VectorSearchHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, repository.VectorSearchHit{
			ID:           h.ID,
			Score:        h.Score,
			ChunkID:      h.ChunkID,
			TenantUserID: h.TenantUserID,
			KBID:         h.KBID,
			SourceType:   h.SourceType,
			SourceKey:    h.SourceKey,
			Content:      h.Content,
			MetadataJSON: h.MetadataJSON,
		})
	}
	return out, nil
}