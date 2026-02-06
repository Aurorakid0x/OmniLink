package pipeline

import (
	"context"
	"fmt"

	"OmniLink/internal/modules/ai/application/dto/respond"
	"OmniLink/internal/modules/ai/domain/repository"
	userRepository "OmniLink/internal/modules/user/domain/repository"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

type JobExecutionRequest struct {
	TenantUserID string
	AgentID      string
	SessionID    string
	Prompt       string
	TopK         int
}

type JobExecutionResult struct {
	Answer     string
	Citations  []respond.CitationEntry
	TokenStats TokenStats
	Err        error
}

type JobExecutionPipeline struct {
	sessionRepo  repository.AssistantSessionRepository
	messageRepo  repository.AssistantMessageRepository
	agentRepo    repository.AgentRepository
	ragRepo      repository.RAGRepository
	userRepo     userRepository.UserInfoRepository
	retrievePipe *RetrievePipeline
	chatModel    model.BaseChatModel
	chatMeta     ChatModelMeta
	tools        []tool.BaseTool
	r            compose.Runnable[*JobExecutionRequest, *JobExecutionResult]
}

func NewJobExecutionPipeline(
	sessionRepo repository.AssistantSessionRepository,
	messageRepo repository.AssistantMessageRepository,
	agentRepo repository.AgentRepository,
	ragRepo repository.RAGRepository,
	userRepo userRepository.UserInfoRepository,
	retrievePipe *RetrievePipeline,
	chatModel model.BaseChatModel,
	chatMeta ChatModelMeta,
	tools []tool.BaseTool,
) (*JobExecutionPipeline, error) {
	if chatModel == nil {
		return nil, fmt.Errorf("chat model is nil")
	}
	p := &JobExecutionPipeline{
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
	ctx := context.Background()
	r, err := p.buildGraph(ctx)
	if err != nil {
		return nil, err
	}
	p.r = r
	return p, nil
}

func (p *JobExecutionPipeline) SetTools(tools []tool.BaseTool) {
	p.tools = tools
}

func (p *JobExecutionPipeline) Execute(ctx context.Context, req *JobExecutionRequest) (*JobExecutionResult, error) {
	if req == nil {
		return &JobExecutionResult{Err: fmt.Errorf("request is nil")}, nil
	}
	result, err := p.r.Invoke(ctx, req)
	if err != nil {
		return &JobExecutionResult{Err: err}, nil
	}
	return result, nil
}

func (p *JobExecutionPipeline) buildGraph(ctx context.Context) (compose.Runnable[*JobExecutionRequest, *JobExecutionResult], error) {
	const (
		LoadContext = "LoadContext"
		Retrieve    = "Retrieve"
		BuildPrompt = "BuildPrompt"
		ChatModel   = "ChatModel"
		Tools       = "Tools"
		Persist     = "Persist"
	)

	g := compose.NewGraph[*JobExecutionRequest, *JobExecutionResult]()

	_ = g.AddLambdaNode(LoadContext, compose.InvokableLambdaWithOption(p.loadContextNode), compose.WithNodeName(LoadContext))
	_ = g.AddLambdaNode(Retrieve, compose.InvokableLambdaWithOption(p.retrieveNode), compose.WithNodeName(Retrieve))
	_ = g.AddLambdaNode(BuildPrompt, compose.InvokableLambdaWithOption(p.buildPromptNode), compose.WithNodeName(BuildPrompt))
	_ = g.AddLambdaNode(ChatModel, compose.InvokableLambdaWithOption(p.chatModelNode), compose.WithNodeName(ChatModel))
	_ = g.AddLambdaNode(Tools, compose.InvokableLambdaWithOption(p.toolsNode), compose.WithNodeName(Tools))
	_ = g.AddLambdaNode(Persist, compose.InvokableLambdaWithOption(p.persistNode), compose.WithNodeName(Persist))

	_ = g.AddEdge(compose.START, LoadContext)
	_ = g.AddEdge(LoadContext, Retrieve)
	_ = g.AddEdge(Retrieve, BuildPrompt)
	_ = g.AddEdge(BuildPrompt, ChatModel)

	shouldCallTools := func(ctx context.Context, st *jobExecutionState) (string, error) {
		hasToolCalls := st.LastResponse != nil && len(st.LastResponse.ToolCalls) > 0
		reachedMaxIterations := st.IterationCount >= st.MaxIterations
		if hasToolCalls && !reachedMaxIterations {
			return Tools, nil
		}
		return Persist, nil
	}

	branch := compose.NewGraphBranch(shouldCallTools, map[string]bool{
		Tools:   true,
		Persist: true,
	})

	_ = g.AddBranch(ChatModel, branch)
	_ = g.AddEdge(Tools, ChatModel)
	_ = g.AddEdge(Persist, compose.END)

	maxSteps := 24
	return g.Compile(ctx,
		compose.WithGraphName("JobExecutionPipeline"),
		compose.WithNodeTriggerMode(compose.AnyPredecessor),
		compose.WithMaxRunSteps(maxSteps))
}

func (p *JobExecutionPipeline) buildFinalResult(st *jobExecutionState) *JobExecutionResult {
	return &JobExecutionResult{
		Answer:     st.Answer,
		Citations:  st.Citations,
		TokenStats: st.Tokens,
		Err:        st.Err,
	}
}
