package pipeline

import (
	"OmniLink/internal/modules/ai/application/dto/respond"
	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/util"
	"OmniLink/pkg/zlog"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"go.uber.org/zap"
)

// retrieveState RAG 召回 Pipeline 的中间状态（在节点间传递）
type retrieveState struct {
	Req           *RetrieveRequest             // 原始请求
	KBID          int64                        // 知识库 ID
	QueryVec      []float32                    // Query 向量
	FilterExpr    string                       // Milvus 过滤表达式
	Hits          []repository.VectorSearchHit // 向量库原始命中
	FilteredHits  []repository.VectorSearchHit // 过滤后的命中
	Chunks        []respond.RAGChunkHit        // 最终返回的 chunks
	Start         time.Time                    // 开始时间
	EmbeddingMs   int64                        // 向量化耗时
	SearchMs      int64                        // 检索耗时
	PostProcessMs int64                        // 后处理耗时
	Err           error                        // 错误（如果有）
}

// buildGraph 构建 RAG 召回 Pipeline 的 Eino Graph
//
// 节点顺序：Validate → EmbedQuery → SearchVector → PostProcess → BuildResult
func (p *RetrievePipeline) buildGraph(ctx context.Context) (compose.Runnable[*RetrieveRequest, *RetrieveResult], error) {
	const (
		Validate     = "Validate"
		EmbedQuery   = "EmbedQuery"
		SearchVector = "SearchVector"
		PostProcess  = "PostProcess"
		BuildResult  = "BuildResult"
	)
	g := compose.NewGraph[*RetrieveRequest, *RetrieveResult]()
	// 添加节点
	_ = g.AddLambdaNode(Validate, compose.InvokableLambdaWithOption(p.validateNode), compose.WithNodeName(Validate))
	_ = g.AddLambdaNode(EmbedQuery, compose.InvokableLambdaWithOption(p.embedQueryNode), compose.WithNodeName(EmbedQuery))
	_ = g.AddLambdaNode(SearchVector, compose.InvokableLambdaWithOption(p.searchVectorNode), compose.WithNodeName(SearchVector))
	_ = g.AddLambdaNode(PostProcess, compose.InvokableLambdaWithOption(p.postProcessNode), compose.WithNodeName(PostProcess))
	_ = g.AddLambdaNode(BuildResult, compose.InvokableLambdaWithOption(p.buildResultNode), compose.WithNodeName(BuildResult))
	// 添加边（定义节点顺序）
	_ = g.AddEdge(compose.START, Validate)
	_ = g.AddEdge(Validate, EmbedQuery)
	_ = g.AddEdge(EmbedQuery, SearchVector)
	_ = g.AddEdge(SearchVector, PostProcess)
	_ = g.AddEdge(PostProcess, BuildResult)
	_ = g.AddEdge(BuildResult, compose.END)
	// 编译为 Runnable
	return g.Compile(ctx, compose.WithGraphName("RAGRetrievePipeline"), compose.WithNodeTriggerMode(compose.AllPredecessor))
}

// validateNode 节点 1：校验请求参数并构造过滤表达式
func (p *RetrievePipeline) validateNode(ctx context.Context, req *RetrieveRequest, _ ...any) (*retrieveState, error) {
	st := &retrieveState{
		Req:   req,
		Start: time.Now(),
	}
	if req == nil {
		st.Err = fmt.Errorf("retrieve request is nil")
		return st, nil
	}
	// 1. 校验必填参数
	tenant := strings.TrimSpace(req.TenantUserID)
	req.TenantUserID = tenant
	if tenant == "" {
		st.Err = fmt.Errorf("missing tenant_user_id")
		return st, nil
	}
	question := strings.TrimSpace(req.Question)
	req.Question = question
	if question == "" {
		st.Err = fmt.Errorf("missing question")
		return st, nil
	}
	// 2. 规范化参数
	req.TopK = normalizeTopK(req.TopK)
	kbType := strings.TrimSpace(req.KBType)
	if kbType == "" {
		kbType = "global"
	}
	req.KBType = kbType
	// 3. 获取 KB ID（确保 KB 存在）
	now := time.Now()
	kb := &rag.AIKnowledgeBase{
		OwnerType: "user",
		OwnerId:   tenant,
		KBType:    kbType,
		Name:      kbType,
		Status:    rag.CommonStatusEnabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	kbID, err := p.repo.EnsureKnowledgeBase(ctx, kb)
	if err != nil {
		st.Err = err
		return st, nil
	}
	st.KBID = kbID
	// 4. 构造过滤表达式（必须包含 tenant_user_id 和 kb_id，防止越权）
	st.FilterExpr = buildFilterExpr(tenant, kbID, req.SourceTypes, req.SourceKeys)
	return st, nil
}

// embedQueryNode 节点 2：将用户问题向量化
func (p *RetrievePipeline) embedQueryNode(ctx context.Context, st *retrieveState, _ ...any) (*retrieveState, error) {
	if st == nil {
		return &retrieveState{Err: fmt.Errorf("nil state"), Start: time.Now()}, nil
	}
	if st.Err != nil {
		return st, nil
	}
	if st.Req == nil {
		st.Err = fmt.Errorf("nil request")
		return st, nil
	}
	embStart := time.Now()
	// 调用 Embedder 对 question 进行向量化
	vecs, err := p.embedder.EmbedStrings(ctx, []string{st.Req.Question})
	if err != nil {
		st.Err = err
		return st, nil
	}
	if len(vecs) == 0 {
		st.Err = fmt.Errorf("embedding result is empty")
		return st, nil
	}
	vec64 := vecs[0]
	if len(vec64) != p.vectorDim {
		st.Err = fmt.Errorf("embedding dim mismatch: got=%d want=%d", len(vec64), p.vectorDim)
		return st, nil
	}
	// 转换为 float32（Milvus 需要 float32）
	vec32 := make([]float32, len(vec64))
	for i := range vec64 {
		vec32[i] = float32(vec64[i])
	}
	st.QueryVec = vec32
	st.EmbeddingMs = time.Since(embStart).Milliseconds()
	return st, nil
}

// searchVectorNode 节点 3：执行向量检索
func (p *RetrievePipeline) searchVectorNode(ctx context.Context, st *retrieveState, _ ...any) (*retrieveState, error) {
	if st == nil {
		return &retrieveState{Err: fmt.Errorf("nil state"), Start: time.Now()}, nil
	}
	if st.Err != nil {
		return st, nil
	}
	if st.Req == nil {
		st.Err = fmt.Errorf("nil request")
		return st, nil
	}
	if len(st.QueryVec) == 0 {
		st.Err = fmt.Errorf("query vector is empty")
		return st, nil
	}
	searchStart := time.Now()
	// 调用 VectorStore.Search 执行向量检索
	hits, err := p.vs.Search(ctx, st.QueryVec, st.Req.TopK, st.FilterExpr)
	if err != nil {
		st.Err = err
		return st, nil
	}
	st.Hits = hits
	st.SearchMs = time.Since(searchStart).Milliseconds()
	return st, nil
}

// postProcessNode 节点 4：后处理（去重、过滤、排序、截断）
func (p *RetrievePipeline) postProcessNode(ctx context.Context, st *retrieveState, _ ...any) (*retrieveState, error) {
	_ = ctx
	if st == nil {
		return &retrieveState{Err: fmt.Errorf("nil state"), Start: time.Now()}, nil
	}
	if st.Err != nil {
		return st, nil
	}
	if st.Req == nil {
		st.Err = fmt.Errorf("nil request")
		return st, nil
	}
	ppStart := time.Now()
	hits := st.Hits
	if len(hits) == 0 {
		st.FilteredHits = []repository.VectorSearchHit{}
		st.PostProcessMs = time.Since(ppStart).Milliseconds()
		return st, nil
	}
	// 1. Score 阈值过滤
	if st.Req.ScoreThreshold > 0 {
		filtered := make([]repository.VectorSearchHit, 0, len(hits))
		for _, h := range hits {
			if h.Score >= st.Req.ScoreThreshold {
				filtered = append(filtered, h)
			}
		}
		hits = filtered
	}
	// 2. 去重策略：如果 dedup_by_same_source=true，对相同 (source_type, source_key) 只保留得分最高的
	if st.Req.DedupBySameSource && len(hits) > 0 {
		dedupMap := make(map[string]repository.VectorSearchHit) // key: "source_type|source_key"
		for _, h := range hits {
			key := fmt.Sprintf("%s|%s", h.SourceType, h.SourceKey)
			existing, ok := dedupMap[key]
			if !ok || h.Score > existing.Score {
				dedupMap[key] = h
			}
		}
		hits = make([]repository.VectorSearchHit, 0, len(dedupMap))
		for _, h := range dedupMap {
			hits = append(hits, h)
		}
	}
	// 3. 排序：按 score 降序
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].Score > hits[j].Score
	})
	// 4. 限制：按 max_chunks 截断
	if st.Req.MaxChunks > 0 && len(hits) > st.Req.MaxChunks {
		hits = hits[:st.Req.MaxChunks]
	}
	// 5. 限制：按 max_content_chars 控制总字符数
	if st.Req.MaxContentChars > 0 {
		totalChars := 0
		finalHits := make([]repository.VectorSearchHit, 0, len(hits))
		for _, h := range hits {
			contentLen := len([]rune(h.Content))
			if totalChars+contentLen > st.Req.MaxContentChars {
				break
			}
			finalHits = append(finalHits, h)
			totalChars += contentLen
		}
		hits = finalHits
	}
	st.FilteredHits = hits
	st.PostProcessMs = time.Since(ppStart).Milliseconds()
	return st, nil
}

// buildResultNode 节点 5：组装最终响应结构
func (p *RetrievePipeline) buildResultNode(ctx context.Context, st *retrieveState, _ ...any) (*RetrieveResult, error) {
	_ = ctx
	if st == nil {
		return nil, fmt.Errorf("nil state")
	}
	req := st.Req
	res := &RetrieveResult{}
	if req != nil {
		res.Question = req.Question
	}
	// 生成唯一的 query_id（用于日志回放）
	res.QueryID = fmt.Sprintf("q_%s_%d", util.GenerateID("Q"), time.Now().UnixNano())
	res.TotalHits = len(st.Hits)
	res.ReturnedCount = len(st.FilteredHits)
	res.EmbeddingMs = st.EmbeddingMs
	res.SearchMs = st.SearchMs
	res.PostProcessMs = st.PostProcessMs
	res.DurationMs = time.Since(st.Start).Milliseconds()
	// 将 VectorSearchHit 转换为 RAGChunkHit
	chunks := make([]respond.RAGChunkHit, 0, len(st.FilteredHits))
	chunkIDs := make([]string, 0, len(st.FilteredHits))
	for _, h := range st.FilteredHits {
		chunks = append(chunks, respond.RAGChunkHit{
			ChunkID:    h.ChunkID,
			SourceType: h.SourceType,
			SourceKey:  h.SourceKey,
			Score:      h.Score,
			Content:    h.Content,
			Metadata:   h.MetadataJSON,
		})
		chunkIDs = append(chunkIDs, fmt.Sprintf("%d", h.ChunkID))
	}
	res.Chunks = chunks
	// 兜底策略：如果未命中任何结果，设置提示信息
	if res.ReturnedCount == 0 {
		res.IsEmpty = true
		res.Message = "未命中知识库，建议运行回填或检查时间范围"
	}
	// 日志记录（用于观测与调试）
	tenantUserID := ""
	topK := 0
	scoreThreshold := float32(0)
	if req != nil {
		tenantUserID = req.TenantUserID
		topK = req.TopK
		scoreThreshold = req.ScoreThreshold
	}
	zlog.Info(
		"ai retrieve done",
		zap.String("query_id", res.QueryID),
		zap.String("tenant_user_id", tenantUserID),
		zap.String("question", res.Question),
		zap.Int("top_k", topK),
		zap.Float32("score_threshold", scoreThreshold),
		zap.String("filter_expr", st.FilterExpr),
		zap.Int("total_hits", res.TotalHits),
		zap.Int("returned_count", res.ReturnedCount),
		zap.String("chunk_ids", strings.Join(chunkIDs, ",")),
		zap.Int64("embedding_ms", res.EmbeddingMs),
		zap.Int64("search_ms", res.SearchMs),
		zap.Int64("post_process_ms", res.PostProcessMs),
		zap.Int64("duration_ms", res.DurationMs),
		zap.Bool("is_empty", res.IsEmpty),
	)
	return res, st.Err
}
