package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/internal/modules/ai/infrastructure/chunking"
	"OmniLink/internal/modules/ai/infrastructure/transform"
	chatEntity "OmniLink/internal/modules/chat/domain/entity"
	"OmniLink/pkg/zlog"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/compose"
	"go.uber.org/zap"
)

type IngestRequest struct {
	TenantUserID string
	SessionUUID  string
	SessionType  int
	SessionName  string
	SourceType   string
	SourceKey    string
	Messages     []chatEntity.Message
}

type IngestResult struct {
	TenantUserID string `json:"tenant_user_id"`
	SourceType   string `json:"source_type"`
	SourceKey    string `json:"source_key"`
	KBID         int64  `json:"kb_id"`
	SourceID     int64  `json:"source_id"`
	Messages     int    `json:"messages"`
	Segments     int    `json:"segments"`
	Chunks       int    `json:"chunks"`
	VectorsOK    int    `json:"vectors_ok"`
	VectorsSkip  int    `json:"vectors_skip"`
	VectorsFail  int    `json:"vectors_fail"`
	DurationMs   int64  `json:"duration_ms"`
}

type IngestPipeline struct {
	repo       repository.RAGRepository
	vs         repository.VectorStore
	embedder   embedding.Embedder
	merger     *transform.ChatTurnMerger
	chunker    *chunking.SimpleChunker
	collection string
	vectorDim  int
	r          compose.Runnable[*IngestRequest, *IngestResult]
}

func NewIngestPipeline(repo repository.RAGRepository, vs repository.VectorStore, embedder embedding.Embedder, merger *transform.ChatTurnMerger, chunker *chunking.SimpleChunker, collection string, vectorDim int) (*IngestPipeline, error) {
	p := &IngestPipeline{repo: repo, vs: vs, embedder: embedder, merger: merger, chunker: chunker, collection: collection, vectorDim: vectorDim}
	g := compose.NewGraph[*IngestRequest, *IngestResult]()
	_ = g.AddLambdaNode("Ingest", compose.InvokableLambdaWithOption(p.ingestNode), compose.WithNodeName("RAGIngest"))
	_ = g.AddEdge(compose.START, "Ingest")
	_ = g.AddEdge("Ingest", compose.END)
	r, err := g.Compile(context.Background(), compose.WithGraphName("RAGIngestPipeline"), compose.WithNodeTriggerMode(compose.AllPredecessor))
	if err != nil {
		return nil, err
	}
	p.r = r
	return p, nil
}

func (p *IngestPipeline) Ingest(ctx context.Context, req IngestRequest) (*IngestResult, error) {
	return p.r.Invoke(ctx, &req)
}

func (p *IngestPipeline) ingestNode(ctx context.Context, req *IngestRequest, _ ...any) (*IngestResult, error) {
	start := time.Now()
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	tenant := strings.TrimSpace(req.TenantUserID)
	if tenant == "" {
		return nil, fmt.Errorf("missing tenant_user_id")
	}
	if strings.TrimSpace(req.SourceType) == "" || strings.TrimSpace(req.SourceKey) == "" {
		return nil, fmt.Errorf("missing source_type/source_key")
	}

	now := time.Now()
	kb := &rag.AIKnowledgeBase{OwnerType: "user", OwnerId: tenant, KBType: "global", Name: "global", Status: rag.CommonStatusEnabled, CreatedAt: now, UpdatedAt: now}
	kbID, err := p.repo.EnsureKnowledgeBase(ctx, kb)
	if err != nil {
		return nil, err
	}
	src := &rag.AIKnowledgeSource{KBId: kbID, SourceType: req.SourceType, SourceKey: req.SourceKey, TenantUserId: tenant, Version: 1, Status: rag.CommonStatusEnabled, CreatedAt: now, UpdatedAt: now}
	sourceID, err := p.repo.EnsureKnowledgeSource(ctx, src)
	if err != nil {
		return nil, err
	}

	segments := p.merger.Merge(req.Messages)
	chunks := make([]string, 0, 8)
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		parts := p.chunker.Chunk(seg)
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				chunks = append(chunks, part)
			}
		}
	}

	res := &IngestResult{TenantUserID: tenant, SourceType: req.SourceType, SourceKey: req.SourceKey, KBID: kbID, SourceID: sourceID, Messages: len(req.Messages), Segments: len(segments), Chunks: len(chunks)}
	if len(chunks) == 0 {
		res.DurationMs = time.Since(start).Milliseconds()
		return res, nil
	}

	type upsertItem struct {
		VectorID   string
		ChunkID    int64
		Content    string
		MetaJSON   string
		TextForEmb string
	}
	items := make([]upsertItem, 0, len(chunks))

	for i, content := range chunks {
		chash := sha256Hex(content)
		ckey := "ck_" + sha256Hex(fmt.Sprintf("%s|%s|%s|%d|%s", tenant, req.SourceType, req.SourceKey, i, chash))
		vid := "v_" + sha256Hex(fmt.Sprintf("%s|%s|%s|%s|%d", tenant, req.SourceType, req.SourceKey, ckey, p.vectorDim))[:48]

		existingChunk, err := p.repo.GetChunkByChunkKey(ctx, ckey)
		if err != nil {
			res.VectorsFail++
			continue
		}
		if existingChunk != nil {
			vr, err := p.repo.GetVectorRecordByChunkID(ctx, existingChunk.Id)
			if err != nil {
				res.VectorsFail++
				continue
			}
			if vr != nil && vr.EmbedStatus == rag.VectorEmbedStatusSucceeded {
				res.VectorsSkip++
				continue
			}
			if vr == nil {
				err = p.repo.CreateVectorRecord(ctx, &rag.AIVectorRecord{ChunkId: existingChunk.Id, VectorStore: "milvus", Collection: p.collection, VectorId: vid, EmbeddingProvider: "mock", EmbeddingModel: "mock", Dim: p.vectorDim, EmbedStatus: rag.VectorEmbedStatusPending, CreatedAt: now, UpdatedAt: now})
				if err != nil {
					res.VectorsFail++
					continue
				}
			}
			items = append(items, upsertItem{VectorID: vid, ChunkID: existingChunk.Id, Content: truncate4096(content), MetaJSON: safeMeta(req, i), TextForEmb: content})
			continue
		}

		chunk := &rag.AIKnowledgeChunk{KBId: kbID, SourceId: sourceID, ChunkKey: ckey, ChunkIndex: i, Content: content, ContentHash: chash, MetadataJson: safeMeta(req, i), Status: rag.CommonStatusEnabled, CreatedAt: now, UpdatedAt: now}
		record := &rag.AIVectorRecord{VectorStore: "milvus", Collection: p.collection, VectorId: vid, EmbeddingProvider: "mock", EmbeddingModel: "mock", Dim: p.vectorDim, EmbedStatus: rag.VectorEmbedStatusPending, CreatedAt: now, UpdatedAt: now}
		if err := p.repo.CreateChunkAndVectorRecord(ctx, chunk, record); err != nil {
			res.VectorsFail++
			continue
		}
		items = append(items, upsertItem{VectorID: vid, ChunkID: chunk.Id, Content: truncate4096(content), MetaJSON: chunk.MetadataJson, TextForEmb: content})
	}

	if len(items) == 0 {
		res.DurationMs = time.Since(start).Milliseconds()
		return res, nil
	}

	texts := make([]string, 0, len(items))
	for _, it := range items {
		texts = append(texts, it.TextForEmb)
	}
	vecs, err := p.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		for _, it := range items {
			_ = p.repo.UpdateVectorStatus(ctx, it.VectorID, rag.VectorEmbedStatusFailed, err.Error())
		}
		res.VectorsFail += len(items)
		res.DurationMs = time.Since(start).Milliseconds()
		return res, err
	}

	upserts := make([]repository.VectorUpsertItem, 0, len(items))
	for i := range items {
		vec64 := vecs[i]
		if len(vec64) != p.vectorDim {
			_ = p.repo.UpdateVectorStatus(ctx, items[i].VectorID, rag.VectorEmbedStatusFailed, fmt.Sprintf("vector dim mismatch got=%d want=%d", len(vec64), p.vectorDim))
			res.VectorsFail++
			continue
		}
		vec32 := make([]float32, len(vec64))
		for j := range vec64 {
			vec32[j] = float32(vec64[j])
		}
		upserts = append(upserts, repository.VectorUpsertItem{ID: items[i].VectorID, Vector: vec32, TenantUserID: tenant, KBID: kbID, SourceType: req.SourceType, SourceKey: req.SourceKey, ChunkID: items[i].ChunkID, Content: items[i].Content, MetadataJSON: items[i].MetaJSON})
	}

	if len(upserts) > 0 {
		_, err = p.vs.Upsert(ctx, upserts)
		if err != nil {
			for _, it := range upserts {
				_ = p.repo.UpdateVectorStatus(ctx, it.ID, rag.VectorEmbedStatusFailed, err.Error())
			}
			res.VectorsFail += len(upserts)
			res.DurationMs = time.Since(start).Milliseconds()
			return res, err
		}
		for _, it := range upserts {
			_ = p.repo.UpdateVectorStatus(ctx, it.ID, rag.VectorEmbedStatusSucceeded, "")
			res.VectorsOK++
		}
	}

	zlog.Info("ai ingest done", zap.String("tenant_user_id", tenant), zap.String("source_type", req.SourceType), zap.String("source_key", req.SourceKey), zap.Int("chunks", res.Chunks), zap.Int("ok", res.VectorsOK), zap.Int("skip", res.VectorsSkip), zap.Int("fail", res.VectorsFail), zap.Int64("ms", time.Since(start).Milliseconds()))
	res.DurationMs = time.Since(start).Milliseconds()
	return res, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func truncate4096(s string) string {
	r := []rune(s)
	if len(r) <= 4096 {
		return s
	}
	return string(r[:4096])
}

func safeMeta(req *IngestRequest, chunkIndex int) string {
	m := map[string]any{"session_uuid": req.SessionUUID, "session_type": req.SessionType, "session_name": req.SessionName, "chunk_index": chunkIndex}
	bs, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	if len(bs) == 0 {
		return "{}"
	}
	return string(bs)
}
