package vectordb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type UpsertItem struct {
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

type SearchHit struct {
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

type MilvusStore struct {
	cli         mclient.Client
	collection  string
	vectorField string
	metricType  entity.MetricType
	vectorDim   int
	searchParam entity.SearchParam
}

func NewMilvusStore(cli mclient.Client, collection string, vectorField string, vectorDim int, metricType entity.MetricType) (*MilvusStore, error) {
	if cli == nil {
		return nil, errors.New("milvus client is nil")
	}
	if strings.TrimSpace(collection) == "" {
		return nil, errors.New("collection is empty")
	}
	if strings.TrimSpace(vectorField) == "" {
		return nil, errors.New("vectorField is empty")
	}
	if vectorDim <= 0 {
		return nil, fmt.Errorf("invalid vectorDim: %d", vectorDim)
	}
	sp, err := entity.NewIndexAUTOINDEXSearchParam(1)
	if err != nil {
		return nil, err
	}
	return &MilvusStore{cli: cli, collection: collection, vectorField: vectorField, metricType: metricType, vectorDim: vectorDim, searchParam: sp}, nil
}

func (s *MilvusStore) Upsert(ctx context.Context, items []UpsertItem) ([]string, error) {
	if len(items) == 0 {
		return []string{}, nil
	}
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
			return nil, errors.New("upsert item missing ID")
		}
		if len(it.Vector) != s.vectorDim {
			return nil, fmt.Errorf("vector dim mismatch for id=%s, got=%d want=%d", it.ID, len(it.Vector), s.vectorDim)
		}
		ids = append(ids, it.ID)
		vectors = append(vectors, it.Vector)
		tenantUserIDs = append(tenantUserIDs, it.TenantUserID)
		kbIDs = append(kbIDs, it.KBID)
		sourceTypes = append(sourceTypes, it.SourceType)
		sourceKeys = append(sourceKeys, it.SourceKey)
		chunkIDs = append(chunkIDs, it.ChunkID)
		contents = append(contents, it.Content)
		metas = append(metas, it.MetadataJSON)
	}

	_, err := s.cli.Upsert(
		ctx,
		s.collection,
		"",
		entity.NewColumnVarChar("id", ids),
		entity.NewColumnFloatVector(s.vectorField, s.vectorDim, vectors),
		entity.NewColumnVarChar("tenant_user_id", tenantUserIDs),
		entity.NewColumnInt64("kb_id", kbIDs),
		entity.NewColumnVarChar("source_type", sourceTypes),
		entity.NewColumnVarChar("source_key", sourceKeys),
		entity.NewColumnInt64("chunk_id", chunkIDs),
		entity.NewColumnVarChar("content", contents),
		entity.NewColumnJSONBytes("metadata", stringSliceToJSONBytes(metas)),
	)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *MilvusStore) DeleteByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	expr := fmt.Sprintf(`id in ["%s"]`, strings.Join(ids, `","`))
	return s.cli.Delete(ctx, s.collection, "", expr)
}

func (s *MilvusStore) Search(ctx context.Context, vector []float32, topK int, expr string) ([]SearchHit, error) {
	if len(vector) != s.vectorDim {
		return nil, fmt.Errorf("vector dim mismatch, got=%d want=%d", len(vector), s.vectorDim)
	}
	if topK <= 0 {
		topK = 5
	}
	res, err := s.cli.Search(
		ctx,
		s.collection,
		[]string{},
		expr,
		[]string{"chunk_id", "tenant_user_id", "kb_id", "source_type", "source_key", "content", "metadata"},
		[]entity.Vector{entity.FloatVector(vector)},
		s.vectorField,
		s.metricType,
		topK,
		s.searchParam,
	)
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return []SearchHit{}, nil
	}
	return parseSearchResult(res[0])
}

func parseSearchResult(sr mclient.SearchResult) ([]SearchHit, error) {
	if sr.Err != nil {
		return nil, sr.Err
	}
	hits := make([]SearchHit, 0, sr.ResultCount)

	idCol := sr.IDs
	chunkIDCol := columnByName(sr.Fields, "chunk_id")
	tenantCol := columnByName(sr.Fields, "tenant_user_id")
	kbIDCol := columnByName(sr.Fields, "kb_id")
	sourceTypeCol := columnByName(sr.Fields, "source_type")
	sourceKeyCol := columnByName(sr.Fields, "source_key")
	contentCol := columnByName(sr.Fields, "content")
	metaCol := columnByName(sr.Fields, "metadata")

	for i := 0; i < sr.ResultCount; i++ {
		id, _ := idCol.GetAsString(i)
		score := float32(0)
		if i < len(sr.Scores) {
			score = sr.Scores[i]
		}

		h := SearchHit{ID: id, Score: score}
		if chunkIDCol != nil {
			v, _ := chunkIDCol.GetAsInt64(i)
			h.ChunkID = v
		}
		if tenantCol != nil {
			v, _ := tenantCol.GetAsString(i)
			h.TenantUserID = v
		}
		if kbIDCol != nil {
			v, _ := kbIDCol.GetAsInt64(i)
			h.KBID = v
		}
		if sourceTypeCol != nil {
			v, _ := sourceTypeCol.GetAsString(i)
			h.SourceType = v
		}
		if sourceKeyCol != nil {
			v, _ := sourceKeyCol.GetAsString(i)
			h.SourceKey = v
		}
		if contentCol != nil {
			v, _ := contentCol.GetAsString(i)
			h.Content = v
		}
		if metaCol != nil {
			v, _ := metaCol.Get(i)
			if bs, ok := v.([]byte); ok {
				h.MetadataJSON = string(bs)
			}
		}
		hits = append(hits, h)
	}

	return hits, nil
}

func columnByName(cols mclient.ResultSet, name string) entity.Column {
	for _, c := range cols {
		if c != nil && c.Name() == name {
			return c
		}
	}
	return nil
}

func stringSliceToJSONBytes(values []string) [][]byte {
	out := make([][]byte, 0, len(values))
	for _, v := range values {
		out = append(out, []byte(v))
	}
	return out
}
