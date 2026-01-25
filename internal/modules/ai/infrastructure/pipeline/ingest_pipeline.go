package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/internal/modules/ai/infrastructure/chunking"
	"OmniLink/internal/modules/ai/infrastructure/transform"
	chatEntity "OmniLink/internal/modules/chat/domain/entity"
	"OmniLink/pkg/zlog"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
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
	Documents    []string
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
	repo        repository.RAGRepository
	vs          repository.VectorStore
	einoIndexer indexer.Indexer
	embedder    embedding.Embedder

	embeddingProvider string
	embeddingModel    string

	merger     *transform.ChatTurnMerger
	chunker    *chunking.SimpleChunker
	collection string
	vectorDim  int
	r          compose.Runnable[*IngestRequest, *IngestResult]
}

func NewIngestPipeline(repo repository.RAGRepository, vs repository.VectorStore, embedder embedding.Embedder, embeddingProvider, embeddingModel string, merger *transform.ChatTurnMerger, chunker *chunking.SimpleChunker, collection string, vectorDim int) (*IngestPipeline, error) {
	var einoIndexer indexer.Indexer
	if idx, ok := vs.(indexer.Indexer); ok {
		einoIndexer = idx
	} else {
		return nil, fmt.Errorf("vector store must implement indexer.Indexer")
	}
	p := &IngestPipeline{repo: repo, vs: vs, einoIndexer: einoIndexer, embedder: embedder, embeddingProvider: strings.TrimSpace(embeddingProvider), embeddingModel: strings.TrimSpace(embeddingModel), merger: merger, chunker: chunker, collection: collection, vectorDim: vectorDim}
	r, err := p.buildGraph(context.Background())
	if err != nil {
		return nil, err
	}
	p.r = r
	return p, nil
}

func (p *IngestPipeline) Ingest(ctx context.Context, req IngestRequest) (*IngestResult, error) {
	return p.r.Invoke(ctx, &req)
}

func (p *IngestPipeline) PurgeSource(ctx context.Context, tenantUserID, sourceType, sourceKey string, disableSource bool) error {
	if p == nil || p.repo == nil || p.vs == nil {
		return fmt.Errorf("pipeline repo/vs is nil")
	}
	tenant := strings.TrimSpace(tenantUserID)
	sourceType = strings.TrimSpace(sourceType)
	sourceKey = strings.TrimSpace(sourceKey)
	if tenant == "" || sourceType == "" || sourceKey == "" {
		return fmt.Errorf("missing tenant/source")
	}

	now := time.Now()
	kb := &rag.AIKnowledgeBase{OwnerType: "user", OwnerId: tenant, KBType: "global", Name: "global", Status: rag.CommonStatusEnabled, CreatedAt: now, UpdatedAt: now}
	kbID, err := p.repo.EnsureKnowledgeBase(ctx, kb)
	if err != nil {
		return err
	}

	src, err := p.repo.GetKnowledgeSource(ctx, kbID, tenant, sourceType, sourceKey)
	if err != nil {
		return err
	}
	if src == nil || src.Id <= 0 {
		return nil
	}

	ids, err := p.repo.ListVectorIDsBySourceID(ctx, src.Id)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		if err := p.vs.DeleteByIDs(ctx, ids); err != nil {
			return err
		}
	}
	if err := p.repo.DeleteChunksAndVectorRecordsBySourceID(ctx, src.Id); err != nil {
		return err
	}
	if disableSource {
		if err := p.repo.UpdateKnowledgeSourceStatus(ctx, src.Id, rag.CommonStatusDisabled); err != nil {
			return err
		}
	}
	return nil
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

	type chunkItem struct {
		unitKey string
		subIdx  int
		msg     *chatEntity.Message
		content string
	}

	chunks := make([]chunkItem, 0, 64)

	if len(req.Documents) > 0 {
		for di, d := range req.Documents {
			d = strings.TrimSpace(d)
			if d == "" {
				continue
			}
			parts := p.chunker.Chunk(d)
			for si, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				chunks = append(chunks, chunkItem{unitKey: fmt.Sprintf("doc_%d", di), subIdx: si, content: part})
			}
		}
	} else if req.SourceType == "chat_private" || req.SourceType == "chat_group" {
		msgs := make([]chatEntity.Message, 0, len(req.Messages))
		for _, m := range req.Messages {
			if m.Type != 0 {
				continue
			}
			m.Content = strings.TrimSpace(m.Content)
			if m.Content == "" {
				continue
			}
			msgs = append(msgs, m)
		}
		sort.Slice(msgs, func(i, j int) bool {
			if msgs[i].CreatedAt.Equal(msgs[j].CreatedAt) {
				return msgs[i].Uuid < msgs[j].Uuid
			}
			return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
		})
		for _, m := range msgs {
			mm := m
			text := fmt.Sprintf("%s(%s): %s", strings.TrimSpace(mm.SendName), mm.CreatedAt.Format("15:04:05"), mm.Content)
			parts := p.chunker.Chunk(text)
			for si, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				chunks = append(chunks, chunkItem{unitKey: strings.TrimSpace(mm.Uuid), subIdx: si, msg: &mm, content: part})
			}
		}
	} else {
		segments := p.merger.Merge(req.Messages)
		for gi, seg := range segments {
			seg = strings.TrimSpace(seg)
			if seg == "" {
				continue
			}
			parts := p.chunker.Chunk(seg)
			for si, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				chunks = append(chunks, chunkItem{unitKey: fmt.Sprintf("seg_%d", gi), subIdx: si, content: part})
			}
		}
	}

	res := &IngestResult{TenantUserID: tenant, SourceType: req.SourceType, SourceKey: req.SourceKey, KBID: kbID, SourceID: sourceID, Messages: len(req.Messages), Segments: 0, Chunks: len(chunks)}
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

	for i := range chunks {
		content := chunks[i].content
		chash := sha256Hex(content)
		unitKey := strings.TrimSpace(chunks[i].unitKey)
		if unitKey == "" {
			unitKey = fmt.Sprintf("idx_%d", i)
		}
		ckey := "ck_" + sha256Hex(fmt.Sprintf("%s|%s|%s|%s|%d|%s", tenant, req.SourceType, req.SourceKey, unitKey, chunks[i].subIdx, chash))
		vid := "v_" + sha256Hex(fmt.Sprintf("%s|%s|%s|%s|%d", tenant, req.SourceType, req.SourceKey, ckey, p.vectorDim))[:48]

		metaJSON := safeMeta(req, i)
		if chunks[i].msg != nil {
			metaJSON = safeChatMsgMeta(req, i, *chunks[i].msg)
		}

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
			items = append(items, upsertItem{VectorID: vid, ChunkID: existingChunk.Id, Content: truncate4096(content), MetaJSON: metaJSON, TextForEmb: content})
			continue
		}

		chunk := &rag.AIKnowledgeChunk{KBId: kbID, SourceId: sourceID, ChunkKey: ckey, ChunkIndex: i, Content: content, ContentHash: chash, MetadataJson: metaJSON, Status: rag.CommonStatusEnabled, CreatedAt: now, UpdatedAt: now}
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

func safeChatMsgMeta(req *IngestRequest, chunkIndex int, msg chatEntity.Message) string {
	m := map[string]any{
		"session_uuid": req.SessionUUID,
		"session_type": req.SessionType,
		"session_name": req.SessionName,
		"chunk_index":  chunkIndex,
		"message_uuid": strings.TrimSpace(msg.Uuid),
		"message_time": msg.CreatedAt.Format(time.RFC3339),
		"send_id":      strings.TrimSpace(msg.SendId),
		"send_name":    strings.TrimSpace(msg.SendName),
		"receive_id":   strings.TrimSpace(msg.ReceiveId),
	}
	bs, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	if len(bs) == 0 {
		return "{}"
	}
	return string(bs)
}
