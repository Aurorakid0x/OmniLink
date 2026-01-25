package vectordb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"OmniLink/internal/modules/ai/domain/repository"

	"github.com/cloudwego/eino-ext/components/retriever/milvus2"
	"github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"

	// V2 SDK for Eino
	"github.com/milvus-io/milvus/client/v2/milvusclient"

	// V1 SDK for Stable Upsert/Delete
	v1client "github.com/milvus-io/milvus-sdk-go/v2/client"
	v1entity "github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// MilvusImpl Unified VectorStore implementation.
// Internally manages both V1 and V2 clients to provide best stability and new features.
type MilvusImpl struct {
	// v1Cli for stable Upsert/Delete operations
	v1Cli v1client.Client

	// v2Cli for Eino Retriever (V2 required)
	v2Cli *milvusclient.Client

	// einoRetriever for advanced retrieval (Text -> Embedding -> Search)
	einoRetriever retriever.Retriever

	collection  string
	vectorDim   int
	vectorField string
}

// Interfaces check
var _ repository.VectorStore = (*MilvusImpl)(nil)
var _ indexer.Indexer = (*MilvusImpl)(nil)

type MilvusConfig struct {
	Address  string
	Username string
	Password string
	DBName   string
}

// NewMilvusImpl creates a new MilvusImpl.
// It requires an existing V1 client (shared) and creates a private V2 client.
func NewMilvusImpl(
	ctx context.Context,
	v1Cli v1client.Client, // Injected from initial.MilvusClient
	conf MilvusConfig,
	embedder embedding.Embedder,
	collection string,
	vectorDim int,
) (*MilvusImpl, error) {
	if v1Cli == nil {
		return nil, fmt.Errorf("milvus v1 client is nil")
	}
	if embedder == nil {
		return nil, fmt.Errorf("embedder is nil")
	}

	// 1. Create Private V2 Client for Eino
	v2Cli, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address:  conf.Address,
		Username: conf.Username,
		Password: conf.Password,
		DBName:   conf.DBName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create milvus v2 client: %w", err)
	}

	// 2. Initialize Eino Retriever
	rConfig := &milvus2.RetrieverConfig{
		Client:       v2Cli,
		Collection:   collection,
		TopK:         5,
		Embedding:    embedder,
		OutputFields: []string{"id", "content", "metadata", "tenant_user_id", "kb_id", "source_type", "source_key", "chunk_id"},
		SearchMode:   search_mode.NewApproximate(milvus2.COSINE),
	}

	r, err := milvus2.NewRetriever(ctx, rConfig)
	if err != nil {
		v2Cli.Close(ctx)
		return nil, fmt.Errorf("failed to create eino retriever: %w", err)
	}

	return &MilvusImpl{
		v1Cli:         v1Cli,
		v2Cli:         v2Cli,
		einoRetriever: r,
		collection:    collection,
		vectorDim:     vectorDim,
		vectorField:   "vector",
	}, nil
}

// Close closes the internal V2 client. V1 client is managed externally.
func (s *MilvusImpl) Close(ctx context.Context) error {
	if s.v2Cli != nil {
		return s.v2Cli.Close(ctx)
	}
	return nil
}

// Retrieve implements repository.VectorStore using Eino
func (s *MilvusImpl) Retrieve(ctx context.Context, query string, topK int, expr string) ([]repository.VectorSearchHit, error) {
	opts := []retriever.Option{
		retriever.WithTopK(topK),
	}
	if expr != "" {
		opts = append(opts, milvus2.WithFilter(expr))
	}

	docs, err := s.einoRetriever.Retrieve(ctx, query, opts...)
	if err != nil {
		return nil, err
	}

	hits := make([]repository.VectorSearchHit, 0, len(docs))
	for _, doc := range docs {
		score := float32(doc.Score())

		hit := repository.VectorSearchHit{
			ID:      doc.ID,
			Content: doc.Content,
			Score:   score,
		}

		if val, ok := doc.MetaData["tenant_user_id"].(string); ok {
			hit.TenantUserID = val
		}
		hit.KBID = toInt64(doc.MetaData["kb_id"])
		hit.ChunkID = toInt64(doc.MetaData["chunk_id"])
		if val, ok := doc.MetaData["source_type"].(string); ok {
			hit.SourceType = val
		}
		if val, ok := doc.MetaData["source_key"].(string); ok {
			hit.SourceKey = val
		}

		if val, ok := doc.MetaData["metadata"]; ok {
			if str, ok := val.(string); ok {
				hit.MetadataJSON = str
			} else if bs, err := json.Marshal(val); err == nil {
				hit.MetadataJSON = string(bs)
			}
		}

		hits = append(hits, hit)
	}
	return hits, nil
}

// Upsert implements repository.VectorStore using V1 SDK
func (s *MilvusImpl) Upsert(ctx context.Context, items []repository.VectorUpsertItem) ([]string, error) {
	if len(items) == 0 {
		return []string{}, nil
	}

	// Prepare Columns using V1 Entity
	ids := make([]string, 0, len(items))
	vectors := make([][]float32, 0, len(items))
	tenantUserIDs := make([]string, 0, len(items))
	kbIDs := make([]int64, 0, len(items))
	sourceTypes := make([]string, 0, len(items))
	sourceKeys := make([]string, 0, len(items))
	chunkIDs := make([]int64, 0, len(items))
	contents := make([]string, 0, len(items))
	metas := make([]string, 0, len(items))

	for _, it := range items {
		if it.ID == "" {
			return nil, fmt.Errorf("upsert item missing ID")
		}
		if len(it.Vector) != s.vectorDim {
			return nil, fmt.Errorf("vector dim mismatch for id=%s", it.ID)
		}

		ids = append(ids, it.ID)
		vectors = append(vectors, it.Vector)
		tenantUserIDs = append(tenantUserIDs, it.TenantUserID)
		kbIDs = append(kbIDs, it.KBID)
		sourceTypes = append(sourceTypes, it.SourceType)
		sourceKeys = append(sourceKeys, it.SourceKey)
		chunkIDs = append(chunkIDs, it.ChunkID)
		contents = append(contents, it.Content)

		m := it.MetadataJSON
		if m == "" {
			m = "{}"
		}
		metas = append(metas, m)
	}

	_, err := s.v1Cli.Upsert(
		ctx,
		s.collection,
		"", // partition
		v1entity.NewColumnVarChar("id", ids),
		v1entity.NewColumnFloatVector(s.vectorField, s.vectorDim, vectors),
		v1entity.NewColumnVarChar("tenant_user_id", tenantUserIDs),
		v1entity.NewColumnInt64("kb_id", kbIDs),
		v1entity.NewColumnVarChar("source_type", sourceTypes),
		v1entity.NewColumnVarChar("source_key", sourceKeys),
		v1entity.NewColumnInt64("chunk_id", chunkIDs),
		v1entity.NewColumnVarChar("content", contents),
		v1entity.NewColumnJSONBytes("metadata", stringSliceToJSONBytes(metas)),
	)

	if err != nil {
		return nil, err
	}
	return ids, nil
}

// DeleteByIDs implements repository.VectorStore using V1 SDK
func (s *MilvusImpl) DeleteByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	expr := fmt.Sprintf(`id in ["%s"]`, strings.Join(ids, `","`))
	return s.v1Cli.Delete(ctx, s.collection, "", expr)
}

// Search implements repository.VectorStore using V1 SDK (Deprecated, but supported)
func (s *MilvusImpl) Search(ctx context.Context, vector []float32, topK int, expr string) ([]repository.VectorSearchHit, error) {
	if len(vector) != s.vectorDim {
		return nil, fmt.Errorf("vector dim mismatch")
	}

	sp, _ := v1entity.NewIndexAUTOINDEXSearchParam(1)

	res, err := s.v1Cli.Search(
		ctx,
		s.collection,
		nil,
		expr,
		[]string{"id", "content", "metadata", "tenant_user_id", "kb_id", "source_type", "source_key", "chunk_id"},
		[]v1entity.Vector{v1entity.FloatVector(vector)},
		s.vectorField,
		v1entity.COSINE,
		topK,
		sp,
	)
	if err != nil {
		return nil, err
	}

	hits := make([]repository.VectorSearchHit, 0)
	if len(res) > 0 {
		sr := res[0]
		if sr.Err != nil {
			return nil, sr.Err
		}

		idCol := sr.IDs
		// Helper to get column
		getCol := func(name string) v1entity.Column {
			for _, c := range sr.Fields {
				if c.Name() == name {
					return c
				}
			}
			return nil
		}

		chunkIDCol := getCol("chunk_id")
		tenantCol := getCol("tenant_user_id")
		kbIDCol := getCol("kb_id")
		sourceTypeCol := getCol("source_type")
		sourceKeyCol := getCol("source_key")
		contentCol := getCol("content")
		metaCol := getCol("metadata")

		for i := 0; i < sr.ResultCount; i++ {
			id, _ := idCol.GetAsString(i)
			score := sr.Scores[i]

			hit := repository.VectorSearchHit{
				ID:    id,
				Score: score,
			}

			if chunkIDCol != nil {
				v, _ := chunkIDCol.GetAsInt64(i)
				hit.ChunkID = v
			}
			if tenantCol != nil {
				v, _ := tenantCol.GetAsString(i)
				hit.TenantUserID = v
			}
			if kbIDCol != nil {
				v, _ := kbIDCol.GetAsInt64(i)
				hit.KBID = v
			}
			if sourceTypeCol != nil {
				v, _ := sourceTypeCol.GetAsString(i)
				hit.SourceType = v
			}
			if sourceKeyCol != nil {
				v, _ := sourceKeyCol.GetAsString(i)
				hit.SourceKey = v
			}
			if contentCol != nil {
				v, _ := contentCol.GetAsString(i)
				hit.Content = v
			}
			if metaCol != nil {
				v, _ := metaCol.Get(i)
				if bs, ok := v.([]byte); ok {
					hit.MetadataJSON = string(bs)
				}
			}

			hits = append(hits, hit)
		}
	}
	return hits, nil
}

// Store implements indexer.Indexer using internal Upsert
func (s *MilvusImpl) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) {
	if len(docs) == 0 {
		return []string{}, nil
	}

	items := make([]repository.VectorUpsertItem, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}

		vec64 := doc.DenseVector()
		if len(vec64) == 0 {
			return nil, fmt.Errorf("document %s missing dense vector", doc.ID)
		}
		vec32 := make([]float32, len(vec64))
		for i := range vec64 {
			vec32[i] = float32(vec64[i])
		}

		md := doc.MetaData
		tenantUserID, _ := metaString(md, "tenant_user_id")
		kbID := toInt64(md["kb_id"])
		sourceType, _ := metaString(md, "source_type")
		sourceKey, _ := metaString(md, "source_key")
		chunkID := toInt64(md["chunk_id"])

		metadataJSON := "{}"
		if val, ok := md["metadata"]; ok {
			if str, ok := val.(string); ok {
				metadataJSON = str
			} else if bs, err := json.Marshal(val); err == nil {
				metadataJSON = string(bs)
			}
		}

		items = append(items, repository.VectorUpsertItem{
			ID:           doc.ID,
			Vector:       vec32,
			TenantUserID: tenantUserID,
			KBID:         kbID,
			SourceType:   sourceType,
			SourceKey:    sourceKey,
			ChunkID:      chunkID,
			Content:      doc.Content,
			MetadataJSON: metadataJSON,
		})
	}

	return s.Upsert(ctx, items)
}

// Helpers

func stringSliceToJSONBytes(values []string) [][]byte {
	out := make([][]byte, 0, len(values))
	for _, v := range values {
		out = append(out, []byte(v))
	}
	return out
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case json.Number:
		i, _ := t.Int64()
		return i
	default:
		return 0
	}
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
