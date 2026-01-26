# Provider 重构总结

## 重构目标

将 Memory 服务和 Chat 服务统一使用 `internal/chat` 中的 Provider 接口，实现代码复用和架构统一。

## 主要变更

### 1. 扩展 Chat Provider 接口

**文件**: `internal/chat/types.go`

- 扩展 `Request` 结构，添加了：
  - `Temperature *float32` - 温度参数
  - `ResponseFormat *ResponseFormat` - 响应格式（支持 JSON 模式）
  - `MaxTokens *int` - 最大 token 数

- 新增 `ResponseFormat` 结构：
  ```go
  type ResponseFormat struct {
      Type string // "json_object" 或 "text"
  }
  ```

### 2. 创建 Memory 的 LLM 接口

**文件**: `internal/memory/types.go`

- 定义了 `LLM` 接口：
  ```go
  type LLM interface {
      Extract(ctx context.Context, req ExtractRequest) (ExtractResponse, error)
      Decide(ctx context.Context, req DecideRequest) (DecideResponse, error)
  }
  ```

- 这个接口被以下两个实现：
  - `LLMClient` (旧实现，向后兼容)
  - `ProviderLLMClient` (新实现，使用 chat.Provider)

### 3. 创建 Provider-based LLM 客户端

**文件**: `internal/memory/llm_provider_client.go` (新文件)

- 实现了 `ProviderLLMClient` 结构
- 使用 `chat.Provider` 来执行 LLM 调用
- 支持 JSON 模式输出，确保结构化响应
- 重用了 `internal/memory` 包中的辅助函数

关键代码：
```go
type ProviderLLMClient struct {
    provider chat.Provider
    model    string
}

func NewProviderLLMClient(provider chat.Provider, model string) *ProviderLLMClient
func (c *ProviderLLMClient) Extract(ctx context.Context, req ExtractRequest) (ExtractResponse, error)
func (c *ProviderLLMClient) Decide(ctx context.Context, req DecideRequest) (DecideResponse, error)
```

### 4. 更新 Memory 服务

**文件**: `internal/memory/service.go`

- 将 `llm *LLMClient` 改为 `llm LLM`
- 现在接受任何实现 `LLM` 接口的类型
- 保持了所有现有功能

### 5. 更新主程序

**文件**: `cmd/agent/main.go`

#### 添加的函数：

1. **selectMemoryModel** - 选择用于 memory 操作的模型
   - 优先级：memory 模型 → chat 模型 → 任何 chat 类型模型
   
2. **fetchProviderByID** - 根据 ID 获取 provider 配置

3. **createChatProvider** - 根据配置创建 provider 实例
   - 支持 OpenAI、Anthropic、Google、Ollama

#### 初始化流程更新：

```go
// 1. 初始化 chat resolver（用于 chat 和 memory）
chatResolver := chat.NewResolver(modelsService, queries, 30*time.Second)

// 2. 尝试为 memory 创建 provider-based 客户端
memoryModel, memoryProvider, err := selectMemoryModel(ctx, modelsService, queries, &cfg)
if err != nil {
    // 回退到旧的 LLMClient
    llmClient = memory.NewLLMClient(cfg.Memory.BaseURL, cfg.Memory.APIKey, ...)
} else {
    // 使用新的 provider-based 客户端
    provider, _ := createChatProvider(memoryProvider, 30*time.Second)
    llmClient = memory.NewProviderLLMClient(provider, memoryModel.ModelID)
}

// 3. 创建 memory 服务
memoryService = memory.NewService(llmClient, embedder, store, resolver, ...)
```

## 架构优势

### 1. 统一的 Provider 管理
- Chat 和 Memory 服务共享相同的 Provider 实现
- 减少代码重复
- 统一的配置和管理

### 2. 灵活的模型选择
- 可以为不同功能配置不同的模型
- 支持 `enable_as` 字段来指定模型用途
- 自动回退机制

### 3. 向后兼容
- 保留了旧的 `LLMClient` 实现
- 如果数据库中没有配置模型，自动回退到配置文件
- 平滑迁移路径

### 4. 类型安全
- 使用 Go 接口而不是运行时类型判断
- 编译时类型检查
- 更好的 IDE 支持

### 5. 易于扩展
- 添加新 Provider 只需实现 `Provider` 接口
- 添加新 LLM 客户端只需实现 `LLM` 接口
- 模块化设计

## 配置说明

### 数据库配置

为 Memory 操作配置模型：

```sql
-- 方式 1: 使用专用的 memory 模型
UPDATE models SET enable_as = 'memory' 
WHERE model_id = 'gpt-4-turbo-preview';

-- 方式 2: 使用 chat 模型（如果没有专用 memory 模型）
UPDATE models SET enable_as = 'chat' 
WHERE model_id = 'gpt-4';
```

### 环境变量（回退配置）

如果数据库中没有配置模型，系统会使用这些配置：

```toml
[memory]
base_url = "https://api.openai.com/v1"
api_key = "sk-..."
model = "gpt-4.1-nano"
timeout_seconds = 10
```

## 测试建议

### 1. Memory 操作测试
```bash
# 测试 Extract（提取事实）
curl -X POST http://localhost:8080/api/memory/add \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "messages": [
      {"role": "user", "content": "My name is Alice and I like pizza"}
    ],
    "user_id": "user-123"
  }'

# 测试 Search（搜索记忆）
curl -X POST http://localhost:8080/api/memory/search \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "query": "What food do I like?",
    "user_id": "user-123"
  }'
```

### 2. Chat 操作测试
```bash
# 测试普通聊天
curl -X POST http://localhost:8080/api/chat \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }'
```

### 3. 验证日志
启动时应该看到：
```
Using memory model: gpt-4-turbo-preview (provider: openai)
```

或者（如果回退）：
```
WARNING: No memory model configured, using fallback LLMClient: ...
```

## 后续工作

### 短期（必须）
- [ ] 实现各个 Provider 的具体逻辑（目前大部分返回 "not yet implemented"）
- [ ] 添加流式响应支持
- [ ] 完善错误处理

### 中期（建议）
- [ ] 添加 Provider 的单元测试
- [ ] 添加 Memory 集成测试
- [ ] 实现 Provider 连接池
- [ ] 添加请求重试机制

### 长期（优化）
- [ ] Provider 性能监控
- [ ] 自动模型选择和负载均衡
- [ ] 模型响应缓存
- [ ] 支持更多 Provider（如 Cohere、HuggingFace）

## 迁移检查清单

- [x] 扩展 Request 结构支持 JSON 模式
- [x] 创建 LLM 接口
- [x] 实现 ProviderLLMClient
- [x] 更新 Memory Service 使用接口
- [x] 更新主程序初始化流程
- [x] 添加模型选择逻辑
- [x] 添加 Provider 创建逻辑
- [x] 保持向后兼容
- [x] 添加架构文档
- [ ] 添加单元测试
- [ ] 添加集成测试
- [ ] 更新部署文档

## 文件清单

### 新增文件
- `internal/memory/llm_provider_client.go` - Provider-based LLM 客户端
- `internal/chat/ARCHITECTURE.md` - 架构文档
- `REFACTORING_SUMMARY.md` - 本文件

### 修改文件
- `internal/chat/types.go` - 扩展 Request 结构
- `internal/memory/types.go` - 添加 LLM 接口
- `internal/memory/service.go` - 使用 LLM 接口
- `cmd/agent/main.go` - 更新初始化流程

### 保留文件（向后兼容）
- `internal/memory/llm_client.go` - 旧的 HTTP 客户端实现

## 注意事项

1. **JSON 模式兼容性**: 不是所有模型都支持 JSON 模式，需要在实现 Provider 时处理
2. **错误处理**: 当前错误处理较简单，生产环境需要更详细的错误信息
3. **超时设置**: 不同操作可能需要不同的超时时间，可以考虑配置化
4. **并发安全**: Provider 实例应该是并发安全的
5. **资源清理**: 确保 Provider 的资源（如 HTTP 连接）正确释放

## 问题反馈

如果遇到问题，请检查：
1. 数据库中是否有配置的 chat 模型
2. Provider 配置是否正确（API key、base URL）
3. 日志中的错误信息
4. 是否正确初始化了 chat resolver

详细架构说明请参考：`internal/chat/ARCHITECTURE.md`

