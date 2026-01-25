package repository

import "context"

// VectorStore 是 domain 层定义的“向量库能力抽象”。
//
// 设计约束：
// 1) application / domain 只能依赖本接口，不应直接依赖 Milvus SDK 或 Eino。
// 2) infrastructure 通过适配器实现本接口（例如 MilvusVectorStore），从而做到可替换（Milvus/pgvector/ES 向量等）。
//
// 字段约定：VectorUpsertItem/VectorSearchHit 中的 TenantUserID/KBID/SourceType/SourceKey/ChunkID 用于多租户隔离与可追溯。
// 上层在写入与检索时应始终携带这些维度，避免越权与数据混淆。

// VectorUpsertItem 向量写入所需的标准字段（用于多租户隔离与可追溯）
type VectorUpsertItem struct {
	ID           string
	Vector       []float32
	TenantUserID string
	KBID         int64
	SourceType   string
	SourceKey    string
	ChunkID      int64
	Content      string
	MetadataJSON string
}

type VectorSearchHit struct {
	ID           string
	Score        float32
	ChunkID      int64
	TenantUserID string
	KBID         int64
	SourceType   string
	SourceKey    string
	Content      string
	MetadataJSON string
}

// VectorStore 向量数据库接口（Upsert/Delete/Search）
type VectorStore interface {
	Upsert(ctx context.Context, items []VectorUpsertItem) ([]string, error)
	DeleteByIDs(ctx context.Context, ids []string) error
	// Search 按向量搜索 (Deprecated: use Retrieve)
	Search(ctx context.Context, vector []float32, topK int, expr string) ([]VectorSearchHit, error)
	// Retrieve 按文本搜索 (Eino Native)
	Retrieve(ctx context.Context, query string, topK int, expr string) ([]VectorSearchHit, error)
}
