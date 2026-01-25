package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/application/dto/respond"
	"OmniLink/internal/modules/ai/domain/repository"
	userRepository "OmniLink/internal/modules/user/domain/repository"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// AssistantRequest Assistant Pipeline 输入请求
type AssistantRequest struct {
	SessionID    string   // 会话ID（可空，不传则创建新会话）
	TenantUserID string   // 租户用户ID（必填）
	Question     string   // 用户问题（必填）
	TopK         int      // 召回Top-K个chunks（默认5）
	Scope        string   // 检索范围：global/chat_private/chat_group
	SourceKeys   []string // 限制检索的source_key列表（可选）
	AgentID      string   // 绑定的Agent ID（可选）
}

// AssistantResult Assistant Pipeline 输出结果
type AssistantResult struct {
	SessionID  string                  // 会话ID
	Answer     string                  // AI回答
	Citations  []respond.CitationEntry // 引用列表
	QueryID    string                  // 本次查询ID
	Timing     respond.TimingInfo      // 耗时统计
	TokenStats TokenStats              // Token统计
	Err        error                   // 错误（如果有）
}

// TokenStats Token统计
type TokenStats struct {
	PromptTokens int `json:"prompt_tokens"`
	AnswerTokens int `json:"answer_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// AssistantPipeline AI助手Pipeline（基于Eino Graph）
type AssistantPipeline struct {
	sessionRepo  repository.AssistantSessionRepository
	messageRepo  repository.AssistantMessageRepository
	agentRepo    repository.AgentRepository
	ragRepo      repository.RAGRepository
	userRepo     userRepository.UserInfoRepository
	retrievePipe *RetrievePipeline
	chatModel    model.BaseChatModel
	chatMeta     ChatModelMeta
	tools        []tool.BaseTool
	r            compose.Runnable[*AssistantRequest, *AssistantResult]
}

// ChatModelMeta ChatModel元数据
type ChatModelMeta struct {
	Provider string
	Model    string
}

// NewAssistantPipeline 创建Assistant Pipeline
func NewAssistantPipeline(
	sessionRepo repository.AssistantSessionRepository,
	messageRepo repository.AssistantMessageRepository,
	agentRepo repository.AgentRepository,
	ragRepo repository.RAGRepository,
	userRepo userRepository.UserInfoRepository,
	retrievePipe *RetrievePipeline,
	chatModel model.BaseChatModel,
	chatMeta ChatModelMeta,
	tools []tool.BaseTool,
) (*AssistantPipeline, error) {
	if sessionRepo == nil || messageRepo == nil || ragRepo == nil || retrievePipe == nil || chatModel == nil {
		return nil, fmt.Errorf("required dependencies are nil")
	}

	p := &AssistantPipeline{
		sessionRepo:  sessionRepo,
		messageRepo:  messageRepo,
		agentRepo:    agentRepo,
		ragRepo:      ragRepo,
		userRepo:     userRepo,
		retrievePipe: retrievePipe,
		chatModel:    chatModel,
		chatMeta:     chatMeta,
		tools:        tools,
	}

	// 构建Eino Graph
	r, err := p.buildGraph(context.Background())
	if err != nil {
		return nil, err
	}
	p.r = r

	return p, nil
}

func (p *AssistantPipeline) SetTools(tools []tool.BaseTool) {
	p.tools = tools
}

// Execute 执行Assistant Pipeline（非流式）
func (p *AssistantPipeline) Execute(ctx context.Context, req *AssistantRequest) (*AssistantResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if p.r == nil {
		return nil, fmt.Errorf("pipeline runnable is nil")
	}
	return p.r.Invoke(ctx, req)
}

// ExecuteStream 执行Assistant Pipeline（流式）返回StreamReader
func (p *AssistantPipeline) ExecuteStream(ctx context.Context, req *AssistantRequest) (*schema.StreamReader[*schema.Message], *assistantState, error) {
	if req == nil {
		return nil, nil, fmt.Errorf("request is nil")
	}

	// 手动执行前3个节点
	st, err := p.loadMemoryNode(ctx, req)
	if err != nil || st.Err != nil {
		return nil, nil, getError(err, st.Err)
	}

	// Node 2: Retrieve
	st, err = p.retrieveNode(ctx, st)
	if err != nil || st.Err != nil {
		return nil, nil, getError(err, st.Err)
	}

	// Node 3: BuildPrompt
	st, err = p.buildPromptNode(ctx, st)
	if err != nil || st.Err != nil {
		return nil, nil, getError(err, st.Err)
	}

	// Node 4: ChatModel (返回StreamReader)
	promptMsgs := make([]*schema.Message, len(st.PromptMsgs))
	for i := range st.PromptMsgs {
		promptMsgs[i] = &st.PromptMsgs[i]
	}

	streamReader, err := p.chatModel.Stream(ctx, promptMsgs)
	if err != nil {
		return nil, nil, err
	}

	return streamReader, st, nil
}

// PersistStreamResult 持久化流式结果
func (p *AssistantPipeline) PersistStreamResult(ctx context.Context, st *assistantState, fullAnswer string, llmMs int64) (*AssistantResult, error) {
	st.Answer = fullAnswer
	st.LLMMs = llmMs

	// Node 5: Persist
	result, err := p.persistNode(ctx, st)
	if err != nil || (result != nil && result.Err != nil) {
		return nil, getError(err, result.Err)
	}

	return result, nil
}

// buildGraph 构建Eino Graph（5个节点）
func (p *AssistantPipeline) buildGraph(ctx context.Context) (compose.Runnable[*AssistantRequest, *AssistantResult], error) {
	const (
		LoadMemory  = "LoadMemory"
		Retrieve    = "Retrieve"
		BuildPrompt = "BuildPrompt"
		ChatModel   = "ChatModel"
		Persist     = "Persist"
	)

	g := compose.NewGraph[*AssistantRequest, *AssistantResult]()

	_ = g.AddLambdaNode(LoadMemory, compose.InvokableLambdaWithOption(p.loadMemoryNode), compose.WithNodeName(LoadMemory))
	_ = g.AddLambdaNode(Retrieve, compose.InvokableLambdaWithOption(p.retrieveNode), compose.WithNodeName(Retrieve))
	_ = g.AddLambdaNode(BuildPrompt, compose.InvokableLambdaWithOption(p.buildPromptNode), compose.WithNodeName(BuildPrompt))
	_ = g.AddLambdaNode(ChatModel, compose.InvokableLambdaWithOption(p.chatModelNode), compose.WithNodeName(ChatModel))
	_ = g.AddLambdaNode(Persist, compose.InvokableLambdaWithOption(p.persistNode), compose.WithNodeName(Persist))

	_ = g.AddEdge(compose.START, LoadMemory)
	_ = g.AddEdge(LoadMemory, Retrieve)
	_ = g.AddEdge(Retrieve, BuildPrompt)
	_ = g.AddEdge(BuildPrompt, ChatModel)
	_ = g.AddEdge(ChatModel, Persist)
	_ = g.AddEdge(Persist, compose.END)

	return g.Compile(ctx, compose.WithGraphName("AssistantPipeline"), compose.WithNodeTriggerMode(compose.AllPredecessor))
}

func (p *AssistantPipeline) buildFinalResult(st *assistantState) *AssistantResult {
	return &AssistantResult{
		SessionID: st.SessionID,
		Answer:    st.Answer,
		Citations: st.Citations,
		QueryID:   st.QueryID,
		Timing: respond.TimingInfo{
			EmbeddingMs:   st.EmbeddingMs,
			SearchMs:      st.SearchMs,
			PostProcessMs: st.PostProcessMs,
			LLMMs:         st.LLMMs,
			TotalMs:       time.Since(st.Start).Milliseconds(),
		},
		TokenStats: st.Tokens,
		Err:        st.Err,
	}
}

func getError(err1, err2 error) error {
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return fmt.Errorf("unknown error")
}

// normalizeScope 规范化Scope参数
func normalizeScope(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return "global"
	}
	return scope
}
