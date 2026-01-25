package pipeline

import (
	"OmniLink/internal/modules/ai/application/dto/respond"
	"OmniLink/internal/modules/ai/domain/repository"
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
)

// RetrieveRequest RAG 召回 Pipeline 的输入请求
type RetrieveRequest struct {
	TenantUserID string // 租户用户 ID（必填，从 JWT 提取）
	Question     string // 用户问题（必填）
	TopK         int    // 返回 Top-K 个 chunks（默认 5，范围 1-50）
	KBType       string // 知识库类型（默认 global）
	// 可选过滤条件
	SourceTypes []string // 过滤数据源类型（如 chat_private, chat_group）
	SourceKeys  []string // 过滤数据源键（如特定的 session_uuid）
	// 召回质量控制参数
	ScoreThreshold    float32 // 相似度得分阈值（低于此值的结果会被过滤）
	MaxChunks         int     // 最大返回 chunks 数量（硬限制）
	MaxContentChars   int     // 最大返回内容字符数（避免超过 LLM 窗口）
	DedupBySameSource bool    // 是否对同一个 source 去重（只保留得分最高的）
}

// RetrieveResult RAG 召回 Pipeline 的输出结果
type RetrieveResult struct {
	QueryID       string                       // 本次查询唯一 ID（便于追踪回放）
	Question      string                       // 原始用户问题
	Hits          []repository.VectorSearchHit // 向量库原始命中结果
	Chunks        []respond.RAGChunkHit        // 最终返回的 chunks 列表
	TotalHits     int                          // 向量库实际返回的结果数（过滤前）
	ReturnedCount int                          // 最终返回的 chunk 数量（过滤后）
	DurationMs    int64                        // 召回总耗时（毫秒）
	EmbeddingMs   int64                        // 向量化耗时（毫秒）
	SearchMs      int64                        // 向量检索耗时（毫秒）
	PostProcessMs int64                        // 后处理耗时（毫秒）
	IsEmpty       bool                         // 是否未命中任何结果（兜底标识）
	Message       string                       // 提示信息（如"未命中知识库，建议回填"）
}

// RetrievePipeline RAG 召回 Pipeline（基于 Eino compose.Graph）
//
// 设计原则：
// 1. 与 IngestPipeline 保持一致的架构风格（使用 Eino Graph + Lambda 节点）
// 2. 只依赖 domain 层接口（VectorStore, Embedder, RAGRepository），不直接依赖 Milvus SDK
// 3. 权限隔离内建：过滤表达式必须包含 tenant_user_id
// 4. 观测优先：记录 query_id、各阶段耗时、命中 chunk_id 列表
type RetrievePipeline struct {
	repo repository.RAGRepository                            // RAG 仓储（用于获取 KB ID）
	vs   repository.VectorStore                              // 向量存储（用于检索）
	r    compose.Runnable[*RetrieveRequest, *RetrieveResult] // Eino Runnable
}

// NewRetrievePipeline 创建 RAG 召回 Pipeline
//
// 参数：
//   - repo: RAG 仓储接口
//   - vs: 向量存储接口
func NewRetrievePipeline(
	repo repository.RAGRepository,
	vs repository.VectorStore,
) (*RetrievePipeline, error) {
	if repo == nil {
		return nil, fmt.Errorf("rag repository is nil")
	}
	if vs == nil {
		return nil, fmt.Errorf("vector store is nil")
	}
	p := &RetrievePipeline{
		repo: repo,
		vs:   vs,
	}
	// 构建 Eino Graph
	r, err := p.buildGraph(context.Background())
	if err != nil {
		return nil, err
	}
	p.r = r
	return p, nil
}

// Retrieve 执行 RAG 召回（封装 Eino Runnable.Invoke）
func (p *RetrievePipeline) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResult, error) {
	if req == nil {
		return nil, fmt.Errorf("retrieve request is nil")
	}
	if p.r == nil {
		return nil, fmt.Errorf("pipeline runnable is nil")
	}
	return p.r.Invoke(ctx, req)
}

// normalizeTopK 规范化 TopK 参数（默认 5，范围 1-50）
func normalizeTopK(topK int) int {
	if topK <= 0 {
		return 5
	}
	if topK > 50 {
		return 50
	}
	return topK
}

// buildFilterExpr 构造 Milvus 过滤表达式（必须包含 tenant_user_id 和 kb_id）
//
// 示例输出：
//
//	tenant_user_id == "U123" AND kb_id == 1
//	tenant_user_id == "U123" AND kb_id == 1 AND source_type in ["chat_private","chat_group"]
func buildFilterExpr(tenantUserID string, kbID int64, sourceTypes, sourceKeys []string) string {
	// 基础过滤：tenant_user_id + kb_id（必须包含，防止越权）
	expr := fmt.Sprintf(`tenant_user_id == "%s" && kb_id == %d`, tenantUserID, kbID)
	// 可选过滤：source_type
	if len(sourceTypes) > 0 {
		validTypes := make([]string, 0, len(sourceTypes))
		for _, st := range sourceTypes {
			st = strings.TrimSpace(st)
			if st != "" {
				validTypes = append(validTypes, fmt.Sprintf(`"%s"`, st))
			}
		}
		if len(validTypes) > 0 {
			expr += fmt.Sprintf(` && source_type in [%s]`, strings.Join(validTypes, ","))
		}
	}
	// 可选过滤：source_key
	if len(sourceKeys) > 0 {
		validKeys := make([]string, 0, len(sourceKeys))
		for _, sk := range sourceKeys {
			sk = strings.TrimSpace(sk)
			if sk != "" {
				validKeys = append(validKeys, fmt.Sprintf(`"%s"`, sk))
			}
		}
		if len(validKeys) > 0 {
			expr += fmt.Sprintf(` && source_key in [%s]`, strings.Join(validKeys, ","))
		}
	}
	return expr
}
