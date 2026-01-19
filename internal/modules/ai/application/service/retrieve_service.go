package service

import (
	"OmniLink/internal/modules/ai/application/dto/request"
	"OmniLink/internal/modules/ai/application/dto/respond"
	"OmniLink/internal/modules/ai/infrastructure/pipeline"
	"context"
	"fmt"
	"strings"
)

// RetrieveService RAG 召回服务接口
type RetrieveService interface {
	// Query 执行 RAG 召回查询
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: RAG 查询请求（包含 question, topK 等参数）
	//   - tenantUserID: 租户用户 ID（从 JWT 提取，必填）
	//
	// 返回：
	//   - RAG 查询响应（包含召回的 chunks、耗时统计等）
	//   - 错误（如果有）
	Query(ctx context.Context, req request.RAGQueryRequest, tenantUserID string) (*respond.RAGQueryRespond, error)
}
type retrieveServiceImpl struct {
	pipeline *pipeline.RetrievePipeline
}

// NewRetrieveService 创建 RAG 召回服务
func NewRetrieveService(pipeline *pipeline.RetrievePipeline) RetrieveService {
	return &retrieveServiceImpl{pipeline: pipeline}
}

// Query 执行 RAG 召回查询
func (s *retrieveServiceImpl) Query(ctx context.Context, req request.RAGQueryRequest, tenantUserID string) (*respond.RAGQueryRespond, error) {
	if s.pipeline == nil {
		return nil, fmt.Errorf("retrieve pipeline is nil")
	}
	// 1. 参数校验与规范化
	tenant := strings.TrimSpace(tenantUserID)
	if tenant == "" {
		return nil, fmt.Errorf("tenant_user_id is required")
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}
	// 2. 构造 Pipeline 请求
	pipelineReq := &pipeline.RetrieveRequest{
		TenantUserID:      tenant,
		Question:          question,
		TopK:              req.TopK,
		KBType:            req.KBType,
		SourceTypes:       req.SourceTypes,
		SourceKeys:        req.SourceKeys,
		ScoreThreshold:    req.ScoreThreshold,
		MaxChunks:         req.MaxChunks,
		MaxContentChars:   req.MaxContentChars,
		DedupBySameSource: req.DedupBySameSource,
	}
	// 3. 调用 Pipeline 执行召回
	result, err := s.pipeline.Retrieve(ctx, pipelineReq)
	if err != nil {
		return nil, err
	}
	// 4. 将 Pipeline 结果转换为 DTO 响应
	resp := &respond.RAGQueryRespond{
		QueryID:       result.QueryID,
		Question:      result.Question,
		Chunks:        result.Chunks,
		TotalHits:     result.TotalHits,
		ReturnedCount: result.ReturnedCount,
		DurationMs:    result.DurationMs,
		EmbeddingMs:   result.EmbeddingMs,
		SearchMs:      result.SearchMs,
		PostProcessMs: result.PostProcessMs,
		IsEmpty:       result.IsEmpty,
		Message:       result.Message,
	}
	return resp, nil
}
