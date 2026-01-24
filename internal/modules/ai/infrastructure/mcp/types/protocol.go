package types

// ServerInfo MCP Server 基本信息
type ServerInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// ClientInfo MCP Client 基本信息
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult 初始化结果
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
}

// Content MCP 内容类型
type Content struct {
	Type string `json:"type"` // text, image, resource
	Text string `json:"text,omitempty"`
}

// CallToolResult 工具调用结果
type CallToolResult struct {
	Content  []Content              `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	IsError  bool                   `json:"isError,omitempty"`
}
