package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"OmniLink/internal/modules/ai/domain/repository"

	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// EinoVectorStore 是“Eino <-> domain VectorStore”的适配层。
//
// 它做的事情：
// - 实现 Eino 的 indexer.Indexer（Store），从而允许 Eino Graph/Chain 把 []*schema.Document 交给向量库写入。
// - 内部并不直接操作 Milvus/pgvector 等底层，而是依赖 domain 的 repository.VectorStore，使底层可替换。
//
// MetaData 约定：Store 会从 doc.MetaData 读取以下键，并转成 repository.VectorUpsertItem 的字段：
// - tenant_user_id (string)
// - kb_id (int64/int/float64/string/json.Number)
// - source_type (string)
// - source_key (string)
// - chunk_id (int64/int/float64/string/json.Number)
// - metadata (可选，string 或任意可 JSON 序列化对象；缺省为 "{}")
//
// Retrieve 目前不实现：Eino 的 Retriever 接口入参是 query string，但向量检索需要先 embedding 得到向量。
// 推荐做法是在 pipeline/graph 中先执行 embedding，再直接调用 repository.VectorStore.Search。

type EinoVectorStore struct {
	vs repository.VectorStore
}

var _ indexer.Indexer = (*EinoVectorStore)(nil)
var _ retriever.Retriever = (*EinoVectorStore)(nil)

func NewEinoVectorStore(vs repository.VectorStore) (*EinoVectorStore, error) {
	if vs == nil {
		return nil, fmt.Errorf("vector store is nil")
	}
	return &EinoVectorStore{vs: vs}, nil
}

func (s *EinoVectorStore) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) {
	if len(docs) == 0 {
		return []string{}, nil
	}

	items := make([]repository.VectorUpsertItem, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		it, err := einoDocToVectorUpsertItem(doc)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return s.vs.Upsert(ctx, items)
}

func (s *EinoVectorStore) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	return nil, fmt.Errorf("not implemented: raw query string search requires internal embedding")
}

func einoDocToVectorUpsertItem(doc *schema.Document) (repository.VectorUpsertItem, error) {
	if doc == nil {
		return repository.VectorUpsertItem{}, fmt.Errorf("document is nil")
	}
	if doc.ID == "" {
		return repository.VectorUpsertItem{}, fmt.Errorf("document missing ID")
	}

	vec64 := doc.DenseVector()
	if len(vec64) == 0 {
		return repository.VectorUpsertItem{}, fmt.Errorf("document %s missing dense vector", doc.ID)
	}
	vec32 := make([]float32, len(vec64))
	for i := range vec64 {
		vec32[i] = float32(vec64[i])
	}

	md := doc.MetaData
	tenantUserID, ok := metaString(md, "tenant_user_id")
	if !ok || tenantUserID == "" {
		return repository.VectorUpsertItem{}, fmt.Errorf("document %s missing meta tenant_user_id", doc.ID)
	}
	kbID, err := metaInt64(md, "kb_id")
	if err != nil || kbID == 0 {
		if err != nil {
			return repository.VectorUpsertItem{}, fmt.Errorf("document %s invalid meta kb_id: %w", doc.ID, err)
		}
		return repository.VectorUpsertItem{}, fmt.Errorf("document %s missing meta kb_id", doc.ID)
	}
	sourceType, ok := metaString(md, "source_type")
	if !ok || sourceType == "" {
		return repository.VectorUpsertItem{}, fmt.Errorf("document %s missing meta source_type", doc.ID)
	}
	sourceKey, ok := metaString(md, "source_key")
	if !ok || sourceKey == "" {
		return repository.VectorUpsertItem{}, fmt.Errorf("document %s missing meta source_key", doc.ID)
	}
	chunkID, err := metaInt64(md, "chunk_id")
	if err != nil || chunkID == 0 {
		if err != nil {
			return repository.VectorUpsertItem{}, fmt.Errorf("document %s invalid meta chunk_id: %w", doc.ID, err)
		}
		return repository.VectorUpsertItem{}, fmt.Errorf("document %s missing meta chunk_id", doc.ID)
	}

	metadataJSON, err := metaJSON(md, "metadata")
	if err != nil {
		return repository.VectorUpsertItem{}, fmt.Errorf("document %s invalid meta metadata: %w", doc.ID, err)
	}

	return repository.VectorUpsertItem{
		ID:           doc.ID,
		Vector:       vec32,
		TenantUserID: tenantUserID,
		KBID:         kbID,
		SourceType:   sourceType,
		SourceKey:    sourceKey,
		ChunkID:      chunkID,
		Content:      doc.Content,
		MetadataJSON: metadataJSON,
	}, nil
}

func metaString(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	if ok {
		return s, true
	}
	return fmt.Sprintf("%v", v), true
}

func metaInt64(m map[string]any, key string) (int64, error) {
	if m == nil {
		return 0, fmt.Errorf("missing meta")
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0, fmt.Errorf("missing meta")
	}
	switch t := v.(type) {
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case int32:
		return int64(t), nil
	case float64:
		return int64(t), nil
	case float32:
		return int64(t), nil
	case json.Number:
		return t.Int64()
	case string:
		if t == "" {
			return 0, fmt.Errorf("empty")
		}
		return strconv.ParseInt(t, 10, 64)
	default:
		return 0, fmt.Errorf("unsupported type %T", v)
	}
}

func metaJSON(m map[string]any, key string) (string, error) {
	if m == nil {
		return "{}", nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return "{}", nil
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return "{}", nil
		}
		return s, nil
	}
	bs, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	if len(bs) == 0 {
		return "{}", nil
	}
	return string(bs), nil
}
