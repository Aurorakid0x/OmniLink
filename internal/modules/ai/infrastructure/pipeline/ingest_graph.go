package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/pkg/zlog"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type ingestState struct {
	Req *IngestRequest

	KBID     int64
	SourceID int64

	Docs []*schema.Document

	VectorIDsAttempted       []string
	VectorIDsUpsertAttempted []string
	UpsertedIDs              []string
	VectorIDErrors           map[string]string

	Segments    int
	Chunks      int
	VectorsSkip int

	Start time.Time
	Err   error
}

func (p *IngestPipeline) buildGraph(ctx context.Context) (compose.Runnable[*IngestRequest, *IngestResult], error) {
	const (
		Prepare      = "Prepare"
		MergeTurns   = "MergeTurns"
		Chunk        = "Chunk"
		Embed        = "Embed"
		Upsert       = "Upsert"
		StatusUpdate = "StatusUpdate"
	)

	g := compose.NewGraph[*IngestRequest, *IngestResult]()

	_ = g.AddLambdaNode(Prepare, compose.InvokableLambdaWithOption(p.prepareNode), compose.WithNodeName(Prepare))
	_ = g.AddLambdaNode(MergeTurns, compose.InvokableLambdaWithOption(p.mergeTurnsNode), compose.WithNodeName(MergeTurns))
	_ = g.AddLambdaNode(Chunk, compose.InvokableLambdaWithOption(p.chunkNode), compose.WithNodeName(Chunk))
	_ = g.AddLambdaNode(Embed, compose.InvokableLambdaWithOption(p.embedNode), compose.WithNodeName(Embed))
	_ = g.AddLambdaNode(Upsert, compose.InvokableLambdaWithOption(p.upsertNode), compose.WithNodeName(Upsert))
	_ = g.AddLambdaNode(StatusUpdate, compose.InvokableLambdaWithOption(p.statusUpdateNode), compose.WithNodeName(StatusUpdate))

	_ = g.AddEdge(compose.START, Prepare)
	_ = g.AddEdge(Prepare, MergeTurns)
	_ = g.AddEdge(MergeTurns, Chunk)
	_ = g.AddEdge(Chunk, Embed)
	_ = g.AddEdge(Embed, Upsert)
	_ = g.AddEdge(Upsert, StatusUpdate)
	_ = g.AddEdge(StatusUpdate, compose.END)

	return g.Compile(ctx, compose.WithGraphName("RAGIngestPipeline"), compose.WithNodeTriggerMode(compose.AllPredecessor))
}

func (p *IngestPipeline) prepareNode(ctx context.Context, req *IngestRequest, _ ...any) (*ingestState, error) {
	st := &ingestState{
		Req:            req,
		VectorIDErrors: map[string]string{},
		Start:          time.Now(),
	}
	if req == nil {
		st.Err = fmt.Errorf("nil request")
		return st, nil
	}

	tenant := strings.TrimSpace(req.TenantUserID)
	req.TenantUserID = tenant
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.SourceKey = strings.TrimSpace(req.SourceKey)

	if tenant == "" {
		st.Err = fmt.Errorf("missing tenant_user_id")
		return st, nil
	}
	if req.SourceType == "" || req.SourceKey == "" {
		st.Err = fmt.Errorf("missing source_type/source_key")
		return st, nil
	}
	if p.repo == nil {
		st.Err = fmt.Errorf("rag repository is nil")
		return st, nil
	}
	if p.einoIndexer == nil {
		st.Err = fmt.Errorf("eino indexer is nil")
		return st, nil
	}
	if p.embedder == nil {
		st.Err = fmt.Errorf("embedder is nil")
		return st, nil
	}

	now := time.Now()
	kb := &rag.AIKnowledgeBase{
		OwnerType: "user",
		OwnerId:   tenant,
		KBType:    "global",
		Name:      "global",
		Status:    rag.CommonStatusEnabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	kbID, err := p.repo.EnsureKnowledgeBase(ctx, kb)
	if err != nil {
		st.Err = err
		return st, nil
	}
	src := &rag.AIKnowledgeSource{
		KBId:         kbID,
		SourceType:   req.SourceType,
		SourceKey:    req.SourceKey,
		TenantUserId: tenant,
		Version:      1,
		Status:       rag.CommonStatusEnabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	sourceID, err := p.repo.EnsureKnowledgeSource(ctx, src)
	if err != nil {
		st.Err = err
		return st, nil
	}

	st.KBID = kbID
	st.SourceID = sourceID
	return st, nil
}

func (p *IngestPipeline) mergeTurnsNode(ctx context.Context, st *ingestState, _ ...any) (*ingestState, error) {
	_ = ctx
	if st == nil {
		return &ingestState{Err: fmt.Errorf("nil state"), Start: time.Now()}, nil
	}
	if st.Err != nil {
		return st, nil
	}
	if st.Req == nil {
		st.Err = fmt.Errorf("nil request")
		return st, nil
	}

	segments := p.merger.Merge(st.Req.Messages)
	docs := make([]*schema.Document, 0, len(segments))
	segIndex := 0
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		md := map[string]any{
			"tenant_user_id": st.Req.TenantUserID,
			"kb_id":          st.KBID,
			"source_type":    st.Req.SourceType,
			"source_key":     st.Req.SourceKey,
			"session_uuid":   st.Req.SessionUUID,
			"session_type":   st.Req.SessionType,
			"session_name":   st.Req.SessionName,
			"segment_index":  segIndex,
		}
		docs = append(docs, &schema.Document{Content: seg, MetaData: md})
		segIndex++
	}
	st.Docs = docs
	st.Segments = len(docs)
	return st, nil
}

func (p *IngestPipeline) chunkNode(ctx context.Context, st *ingestState, _ ...any) (*ingestState, error) {
	if st == nil {
		return &ingestState{Err: fmt.Errorf("nil state"), Start: time.Now()}, nil
	}
	if st.Err != nil {
		return st, nil
	}
	if st.Req == nil {
		st.Err = fmt.Errorf("nil request")
		return st, nil
	}

	segDocs := st.Docs
	if len(segDocs) == 0 {
		st.Docs = []*schema.Document{}
		return st, nil
	}

	chunkDocs, err := stChunkDocuments(ctx, p, segDocs)
	if err != nil {
		st.Err = err
		return st, nil
	}

	out := make([]*schema.Document, 0, len(chunkDocs))
	attempted := make([]string, 0, len(chunkDocs))
	now := time.Now()

	for _, d := range chunkDocs {
		if d == nil {
			continue
		}
		content := strings.TrimSpace(d.Content)
		if content == "" {
			continue
		}
		st.Chunks++

		segIndex, _ := metaInt(d.MetaData, "segment_index")
		chunkIndex, _ := metaInt(d.MetaData, "chunk_index")
		chash := sha256Hex(content)

		ckey := "ck_" + sha256Hex(fmt.Sprintf("%s|%s|%s|%d|%d|%s", st.Req.TenantUserID, st.Req.SourceType, st.Req.SourceKey, segIndex, chunkIndex, chash))
		defaultVID := "v_" + sha256Hex(fmt.Sprintf("%s|%s|%s|%s|%d", st.Req.TenantUserID, st.Req.SourceType, st.Req.SourceKey, ckey, p.vectorDim))[:48]

		metaJSON := buildMetadataJSON(st.Req, segIndex, chunkIndex)
		existingChunk, err := p.repo.GetChunkByChunkKey(ctx, ckey)
		if err != nil {
			st.Err = err
			return st, nil
		}

		if existingChunk != nil {
			vr, err := p.repo.GetVectorRecordByChunkID(ctx, existingChunk.Id)
			if err != nil {
				st.Err = err
				return st, nil
			}
			if vr != nil && vr.EmbedStatus == rag.VectorEmbedStatusSucceeded {
				st.VectorsSkip++
				continue
			}

			vectorID := defaultVID
			if vr != nil && strings.TrimSpace(vr.VectorId) != "" {
				vectorID = strings.TrimSpace(vr.VectorId)
			}
			if vr == nil {
				err = p.repo.CreateVectorRecord(ctx, &rag.AIVectorRecord{
					ChunkId:           existingChunk.Id,
					VectorStore:       "milvus",
					Collection:        p.collection,
					VectorId:          vectorID,
					EmbeddingProvider: "mock",
					EmbeddingModel:    "mock",
					Dim:               p.vectorDim,
					EmbedStatus:       rag.VectorEmbedStatusPending,
					CreatedAt:         now,
					UpdatedAt:         now,
				})
				if err != nil {
					st.Err = err
					return st, nil
				}
			}

			d.ID = vectorID
			d.Content = truncate4096(content)
			if d.MetaData == nil {
				d.MetaData = map[string]any{}
			}
			d.MetaData["tenant_user_id"] = st.Req.TenantUserID
			d.MetaData["kb_id"] = st.KBID
			d.MetaData["source_type"] = st.Req.SourceType
			d.MetaData["source_key"] = st.Req.SourceKey
			d.MetaData["chunk_id"] = existingChunk.Id
			d.MetaData["metadata"] = metaJSON

			out = append(out, d)
			attempted = append(attempted, vectorID)
			continue
		}

		chunk := &rag.AIKnowledgeChunk{
			KBId:         st.KBID,
			SourceId:     st.SourceID,
			ChunkKey:     ckey,
			ChunkIndex:   st.Chunks - 1,
			Content:      content,
			ContentHash:  chash,
			MetadataJson: metaJSON,
			Status:       rag.CommonStatusEnabled,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		record := &rag.AIVectorRecord{
			VectorStore:       "milvus",
			Collection:        p.collection,
			VectorId:          defaultVID,
			EmbeddingProvider: "mock",
			EmbeddingModel:    "mock",
			Dim:               p.vectorDim,
			EmbedStatus:       rag.VectorEmbedStatusPending,
			CreatedAt:         now,
			UpdatedAt:         now,
		}
		if err := p.repo.CreateChunkAndVectorRecord(ctx, chunk, record); err != nil {
			st.Err = err
			return st, nil
		}

		d.ID = defaultVID
		d.Content = truncate4096(content)
		if d.MetaData == nil {
			d.MetaData = map[string]any{}
		}
		d.MetaData["tenant_user_id"] = st.Req.TenantUserID
		d.MetaData["kb_id"] = st.KBID
		d.MetaData["source_type"] = st.Req.SourceType
		d.MetaData["source_key"] = st.Req.SourceKey
		d.MetaData["chunk_id"] = chunk.Id
		d.MetaData["metadata"] = metaJSON

		out = append(out, d)
		attempted = append(attempted, defaultVID)
	}

	st.Docs = out
	st.VectorIDsAttempted = attempted
	return st, nil
}

func (p *IngestPipeline) embedNode(ctx context.Context, st *ingestState, _ ...any) (*ingestState, error) {
	if st == nil {
		return &ingestState{Err: fmt.Errorf("nil state"), Start: time.Now()}, nil
	}
	if st.Err != nil {
		return st, nil
	}
	if len(st.Docs) == 0 {
		return st, nil
	}

	texts := make([]string, 0, len(st.Docs))
	for _, d := range st.Docs {
		if d != nil {
			texts = append(texts, d.Content)
		}
	}

	vecs, err := p.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		st.Err = err
		return st, nil
	}

	next := make([]*schema.Document, 0, len(st.Docs))
	for i, d := range st.Docs {
		if d == nil {
			continue
		}
		if i >= len(vecs) {
			st.VectorIDErrors[d.ID] = "embedding result missing"
			continue
		}
		if len(vecs[i]) != p.vectorDim {
			st.VectorIDErrors[d.ID] = fmt.Sprintf("vector dim mismatch got=%d want=%d", len(vecs[i]), p.vectorDim)
			continue
		}
		d.WithDenseVector(vecs[i])
		next = append(next, d)
	}
	st.Docs = next
	return st, nil
}

func (p *IngestPipeline) upsertNode(ctx context.Context, st *ingestState, _ ...any) (*ingestState, error) {
	if st == nil {
		return &ingestState{Err: fmt.Errorf("nil state"), Start: time.Now()}, nil
	}
	if st.Err != nil {
		return st, nil
	}
	if len(st.Docs) == 0 {
		return st, nil
	}

	ids := make([]string, 0, len(st.Docs))
	for _, d := range st.Docs {
		if d != nil && d.ID != "" {
			ids = append(ids, d.ID)
		}
	}
	st.VectorIDsUpsertAttempted = ids

	outIDs, err := p.einoIndexer.Store(ctx, st.Docs)
	if err != nil {
		st.Err = err
		return st, nil
	}
	st.UpsertedIDs = outIDs
	return st, nil
}

func (p *IngestPipeline) statusUpdateNode(ctx context.Context, st *ingestState, _ ...any) (*IngestResult, error) {
	if st == nil {
		return nil, fmt.Errorf("nil state")
	}

	req := st.Req
	res := &IngestResult{}
	if req != nil {
		res.TenantUserID = req.TenantUserID
		res.SourceType = req.SourceType
		res.SourceKey = req.SourceKey
		res.Messages = len(req.Messages)
	}
	res.KBID = st.KBID
	res.SourceID = st.SourceID
	res.Segments = st.Segments
	res.Chunks = st.Chunks
	res.VectorsSkip = st.VectorsSkip

	failed := map[string]struct{}{}
	for id, msg := range st.VectorIDErrors {
		if strings.TrimSpace(id) == "" {
			continue
		}
		_ = p.repo.UpdateVectorStatus(ctx, id, rag.VectorEmbedStatusFailed, msg)
		failed[id] = struct{}{}
		res.VectorsFail++
	}

	if st.Err != nil {
		errMsg := st.Err.Error()
		for _, id := range st.VectorIDsAttempted {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := failed[id]; ok {
				continue
			}
			_ = p.repo.UpdateVectorStatus(ctx, id, rag.VectorEmbedStatusFailed, errMsg)
			failed[id] = struct{}{}
			res.VectorsFail++
		}
	} else {
		for _, id := range st.UpsertedIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			_ = p.repo.UpdateVectorStatus(ctx, id, rag.VectorEmbedStatusSucceeded, "")
			res.VectorsOK++
		}
	}

	res.DurationMs = time.Since(st.Start).Milliseconds()
	zlog.Info(
		"ai ingest done",
		zap.String("tenant_user_id", res.TenantUserID),
		zap.String("source_type", res.SourceType),
		zap.String("source_key", res.SourceKey),
		zap.String("session_uuid", safeString(req, func(r *IngestRequest) string { return r.SessionUUID })),
		zap.Int("chunks", res.Chunks),
		zap.Int("ok", res.VectorsOK),
		zap.Int("skip", res.VectorsSkip),
		zap.Int("fail", res.VectorsFail),
		zap.Int64("ms", res.DurationMs),
	)

	return res, st.Err
}

func stChunkDocuments(ctx context.Context, p *IngestPipeline, docs []*schema.Document) ([]*schema.Document, error) {
	if p == nil || p.chunker == nil {
		return nil, fmt.Errorf("chunker is nil")
	}
	return p.chunker.ChunkDocuments(ctx, docs)
}

func buildMetadataJSON(req *IngestRequest, segmentIndex, chunkIndex int) string {
	if req == nil {
		return "{}"
	}
	m := map[string]any{
		"session_uuid":  req.SessionUUID,
		"session_type":  req.SessionType,
		"session_name":  req.SessionName,
		"segment_index": segmentIndex,
		"chunk_index":   chunkIndex,
	}
	bs, err := json.Marshal(m)
	if err != nil || len(bs) == 0 {
		return "{}"
	}
	return string(bs)
}

func metaInt(m map[string]any, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case float32:
		return int(t), true
	default:
		return 0, false
	}
}

func safeString(req *IngestRequest, f func(*IngestRequest) string) string {
	if req == nil {
		return ""
	}
	return f(req)
}
