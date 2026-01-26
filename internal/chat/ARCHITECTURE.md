# Chat Provider 架构文档

## 概述

本文档描述了 Memoh 项目中统一的 Chat Provider 架构，该架构被 chat 服务和 memory 服务共同使用。

## 架构设计

### 核心接口

#### Provider 接口

所有 LLM 提供商都实现 `chat.Provider` 接口：

```go
type Provider interface {
    Chat(ctx context.Context, req Request) (Result, error)
    StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, <-chan error)
}
```

#### Request 结构

Provider 请求支持多种配置选项：

```go
type Request struct {
    Messages       []Message
    Model          string
    Provider       string
    Temperature    *float32          // 可选：温度参数
    ResponseFormat *ResponseFormat   // 可选：响应格式（JSON 模式）
    MaxTokens      *int              // 可选：最大 token 数
}

type ResponseFormat struct {
    Type string // "json_object" 或 "text"
}
```

### 支持的提供商

1. **OpenAI** (`openai` / `openai-compat`)
   - 标准 OpenAI API
   - 兼容 OpenAI 格式的自定义端点

2. **Anthropic** (`anthropic`)
   - Claude 系列模型

3. **Google** (`google`)
   - Gemini 系列模型

4. **Ollama** (`ollama`)
   - 本地部署的开源模型

## 使用场景

### 1. Chat 服务

Chat 服务通过 `chat.Resolver` 使用 Provider：

```go
chatResolver := chat.NewResolver(modelsService, queries, 30*time.Second)
response, err := chatResolver.Chat(ctx, ChatRequest{
    Messages: messages,
    Model:    "gpt-4",
})
```

### 2. Memory 服务

Memory 服务通过 `memory.ProviderLLMClient` 使用 Provider：

```go
// 创建 provider
provider, err := chat.NewOpenAIProvider(apiKey, baseURL, timeout)

// 创建 memory LLM 客户端
llmClient := memory.NewProviderLLMClient(provider, modelID)

// 使用 memory 服务
memoryService := memory.NewService(llmClient, embedder, store, resolver, ...)
```

Memory 服务需要两个核心功能：
- **Extract**: 从对话中提取事实信息
- **Decide**: 决定如何更新记忆（添加/更新/删除）

这两个操作都使用 JSON 模式来确保结构化输出。

## 配置示例

### 数据库配置

Provider 配置存储在 `llm_providers` 表：

```sql
CREATE TABLE llm_providers (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  client_type TEXT NOT NULL, -- 'openai', 'anthropic', 'google', 'ollama'
  base_url TEXT NOT NULL,
  api_key TEXT NOT NULL,
  metadata JSONB
);
```

模型配置存储在 `models` 表：

```sql
CREATE TABLE models (
  id UUID PRIMARY KEY,
  model_id TEXT NOT NULL,
  name TEXT,
  llm_provider_id UUID REFERENCES llm_providers(id),
  type TEXT NOT NULL, -- 'chat' or 'embedding'
  enable_as TEXT, -- 'chat', 'memory', 'embedding'
  ...
);
```

### 启动时初始化

在 `cmd/agent/main.go` 中：

```go
// 1. 初始化 chat resolver（用于 chat 和 memory）
chatResolver := chat.NewResolver(modelsService, queries, 30*time.Second)

// 2. 为 memory 选择模型和创建 provider
memoryModel, memoryProvider, err := selectMemoryModel(ctx, modelsService, queries, cfg)
provider, err := createChatProvider(memoryProvider, 30*time.Second)

// 3. 创建 memory LLM 客户端
llmClient := memory.NewProviderLLMClient(provider, memoryModel.ModelID)

// 4. 创建 memory 服务
memoryService := memory.NewService(llmClient, embedder, store, resolver, ...)
```

## 模型选择策略

### Memory 模型选择优先级

1. `enable_as = 'memory'` 的模型（专用 memory 模型）
2. `enable_as = 'chat'` 的模型（通用 chat 模型）
3. 任何可用的 chat 类型模型
4. 回退到配置文件中的 LLMClient（向后兼容）

### Chat 模型选择优先级

1. 请求中指定的模型
2. `enable_as = 'chat'` 的模型
3. 任何可用的 chat 类型模型

## 优势

1. **统一架构**: Chat 和 Memory 使用相同的 Provider 接口
2. **灵活配置**: 支持多个提供商和模型
3. **向后兼容**: 保留旧的 LLMClient 作为回退选项
4. **类型安全**: 使用 Go 接口确保类型安全
5. **易于扩展**: 添加新的提供商只需实现 Provider 接口

## 扩展新提供商

要添加新的 LLM 提供商：

1. 在 `internal/chat/` 创建新文件（如 `newprovider.go`）
2. 实现 `Provider` 接口
3. 在 `resolver.go` 的 `createProvider()` 中添加新的 case
4. 在数据库的 `llm_providers_client_type_check` 约束中添加新类型

示例：

```go
// newprovider.go
type NewProvider struct {
    apiKey  string
    timeout time.Duration
}

func NewNewProvider(apiKey string, timeout time.Duration) (*NewProvider, error) {
    return &NewProvider{apiKey: apiKey, timeout: timeout}, nil
}

func (p *NewProvider) Chat(ctx context.Context, req Request) (Result, error) {
    // 实现 chat 逻辑
}

func (p *NewProvider) StreamChat(ctx context.Context, req Request) (<-chan StreamChunk, <-chan error) {
    // 实现流式 chat 逻辑
}
```

## 迁移指南

从旧的 TypeScript 后端迁移到 Go：

1. ✅ 创建 Provider 接口和实现
2. ✅ 实现 Chat Resolver
3. ✅ 创建 Memory 的 Provider 适配器
4. ✅ 更新主程序使用统一 Provider
5. 🚧 实现各个 Provider 的具体逻辑（OpenAI, Anthropic, Google, Ollama）
6. 🚧 添加流式响应支持
7. 🚧 添加完整的错误处理和重试机制

## 注意事项

1. **JSON 模式**: Memory 操作需要 `ResponseFormat.Type = "json_object"` 来确保结构化输出
2. **温度参数**: Memory 操作使用 `Temperature = 0` 确保确定性输出
3. **超时设置**: 不同操作可能需要不同的超时时间
4. **错误处理**: Provider 应该返回清晰的错误信息，包括 API 错误详情

