# Providers 模块文档

## 概述

本文档描述了独立的 Providers 模块，用于管理 LLM Provider 配置。

## 架构设计

### 模块结构

```
internal/
├── providers/          # 独立的 provider 模块
│   ├── types.go       # 类型定义
│   └── service.go     # 业务逻辑
└── handlers/
    └── providers.go   # API 处理器
```

### 分层设计

```
┌─────────────────────┐
│   API Layer         │
│  (handlers)         │
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│  Service Layer      │
│  (providers pkg)    │
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│  Data Layer         │
│  (sqlc queries)     │
└─────────────────────┘
```

## API 端点

### Providers API (`/providers`)

所有端点都需要 JWT 认证。

#### 1. 创建 Provider

```http
POST /providers
Content-Type: application/json
Authorization: Bearer <token>

{
  "name": "OpenAI Official",
  "client_type": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "metadata": {
    "description": "Official OpenAI API"
  }
}
```

**响应** (201 Created):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "OpenAI Official",
  "client_type": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-12345***",
  "metadata": {
    "description": "Official OpenAI API"
  },
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

#### 2. 列出所有 Providers

```http
GET /providers
Authorization: Bearer <token>
```

**可选查询参数**:
- `client_type` - 按客户端类型过滤 (openai, anthropic, google, ollama)

**响应** (200 OK):
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "OpenAI Official",
    "client_type": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-12345***",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
]
```

#### 3. 获取单个 Provider

```http
GET /providers/{id}
Authorization: Bearer <token>
```

或者按名称获取：

```http
GET /providers/name/{name}
Authorization: Bearer <token>
```

**响应** (200 OK): 同创建响应

#### 4. 更新 Provider

```http
PUT /providers/{id}
Content-Type: application/json
Authorization: Bearer <token>

{
  "name": "OpenAI Updated",
  "api_key": "sk-newkey..."
}
```

**注意**: 所有字段都是可选的，只更新提供的字段。

**响应** (200 OK): 返回更新后的 provider

#### 5. 删除 Provider

```http
DELETE /providers/{id}
Authorization: Bearer <token>
```

**响应** (204 No Content)

#### 6. 统计 Provider 数量

```http
GET /providers/count
Authorization: Bearer <token>
```

**可选查询参数**:
- `client_type` - 按客户端类型过滤

**响应** (200 OK):
```json
{
  "count": 5
}
```

## 支持的 Client Types

| Client Type | 描述 | 需要 API Key |
|------------|------|-------------|
| `openai` | OpenAI 官方 API | ✅ |
| `openai-compat` | OpenAI 兼容的 API | ✅ |
| `anthropic` | Anthropic Claude API | ✅ |
| `google` | Google Gemini API | ✅ |
| `ollama` | 本地 Ollama | ❌ |

## 数据模型

### Provider 结构

```go
type CreateRequest struct {
    Name       string                 `json:"name"`        // 必填
    ClientType ClientType             `json:"client_type"` // 必填
    BaseURL    string                 `json:"base_url"`    // 必填
    APIKey     string                 `json:"api_key"`     // 可选
    Metadata   map[string]interface{} `json:"metadata"`    // 可选
}

type GetResponse struct {
    ID         string                 `json:"id"`
    Name       string                 `json:"name"`
    ClientType string                 `json:"client_type"`
    BaseURL    string                 `json:"base_url"`
    APIKey     string                 `json:"api_key"`      // 已脱敏
    Metadata   map[string]interface{} `json:"metadata"`
    CreatedAt  time.Time              `json:"created_at"`
    UpdatedAt  time.Time              `json:"updated_at"`
}
```

## 安全特性

### 1. API Key 脱敏

在响应中，API Key 会被自动脱敏：
- 只显示前 8 个字符
- 其余部分用 `*` 替换
- 例如: `sk-12345678***`

### 2. 认证保护

所有 API 端点都需要 JWT 认证：
```http
Authorization: Bearer <your-jwt-token>
```

### 3. 输入验证

- 自动验证 UUID 格式
- 验证 client_type 是否支持
- 验证必填字段

## 使用示例

### 1. 配置 OpenAI Provider

```bash
curl -X POST http://localhost:8080/providers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "OpenAI GPT-4",
    "client_type": "openai",
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-..."
  }'
```

### 2. 配置自定义 OpenAI 兼容服务

```bash
curl -X POST http://localhost:8080/providers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Azure OpenAI",
    "client_type": "openai-compat",
    "base_url": "https://your-resource.openai.azure.com/v1",
    "api_key": "your-azure-key",
    "metadata": {
      "deployment": "gpt-4",
      "region": "eastus"
    }
  }'
```

### 3. 配置本地 Ollama

```bash
curl -X POST http://localhost:8080/providers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Local Ollama",
    "client_type": "ollama",
    "base_url": "http://localhost:11434"
  }'
```

### 4. 列出所有 OpenAI Providers

```bash
curl http://localhost:8080/providers?client_type=openai \
  -H "Authorization: Bearer $TOKEN"
```

### 5. 更新 Provider API Key

```bash
curl -X PUT http://localhost:8080/providers/{id} \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "api_key": "sk-new-key..."
  }'
```

## 与 Models 的关系

Provider 和 Model 是一对多的关系：

```
┌─────────────┐
│  Provider   │
│  (OpenAI)   │
└──────┬──────┘
       │
       ├─── Model (gpt-4)
       ├─── Model (gpt-3.5-turbo)
       └─── Model (text-embedding-ada-002)
```

### 创建 Model 时引用 Provider

```bash
# 1. 创建 Provider
PROVIDER_ID=$(curl -X POST http://localhost:8080/providers \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"OpenAI","client_type":"openai",...}' \
  | jq -r '.id')

# 2. 创建 Model 并引用 Provider
curl -X POST http://localhost:8080/models \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "model_id": "gpt-4",
    "name": "GPT-4",
    "llm_provider_id": "'$PROVIDER_ID'",
    "type": "chat"
  }'
```

## 代码集成

### 在代码中使用 Provider Service

```go
import "github.com/memohai/memoh/internal/providers"

// 创建 service
providersService := providers.NewService(queries)

// 创建 provider
provider, err := providersService.Create(ctx, providers.CreateRequest{
    Name:       "OpenAI",
    ClientType: providers.ClientTypeOpenAI,
    BaseURL:    "https://api.openai.com/v1",
    APIKey:     "sk-...",
})

// 列出所有 providers
allProviders, err := providersService.List(ctx)

// 按类型过滤
openaiProviders, err := providersService.ListByClientType(ctx, providers.ClientTypeOpenAI)

// 获取单个 provider
provider, err := providersService.Get(ctx, "provider-uuid")

// 更新 provider
updated, err := providersService.Update(ctx, "provider-uuid", providers.UpdateRequest{
    APIKey: stringPtr("new-key"),
})

// 删除 provider
err := providersService.Delete(ctx, "provider-uuid")
```

## 错误处理

### 常见错误

| 状态码 | 错误 | 原因 |
|-------|------|------|
| 400 | Bad Request | 缺少必填字段或格式错误 |
| 404 | Not Found | Provider ID 不存在 |
| 409 | Conflict | Provider 名称已存在 |
| 500 | Internal Server Error | 服务器错误 |

### 错误响应格式

```json
{
  "message": "invalid UUID: invalid UUID length: 5"
}
```

## 数据库 Schema

Providers 存储在 `llm_providers` 表：

```sql
CREATE TABLE llm_providers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  client_type TEXT NOT NULL,
  base_url TEXT NOT NULL,
  api_key TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT llm_providers_name_unique UNIQUE (name),
  CONSTRAINT llm_providers_client_type_check 
    CHECK (client_type IN ('openai', 'openai-compat', 'anthropic', 'google', 'ollama'))
);
```

## 最佳实践

### 1. 命名规范

- 使用描述性名称: `OpenAI GPT-4`, `Anthropic Claude 3`
- 包含环境信息: `OpenAI Production`, `Ollama Local`
- 避免特殊字符和空格

### 2. API Key 管理

- 定期轮换 API Keys
- 使用环境变量存储敏感信息
- 不要在日志中输出完整的 API Key

### 3. Metadata 使用

使用 metadata 字段存储额外信息：

```json
{
  "metadata": {
    "environment": "production",
    "rate_limit": 10000,
    "contact": "admin@example.com",
    "notes": "Primary provider for production"
  }
}
```

### 4. Provider 测试

创建 provider 后，建议测试连接：

```go
// 未来可以实现的测试端点
POST /providers/test
{
  "client_type": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key": "sk-...",
  "model": "gpt-3.5-turbo"
}
```

## 迁移指南

### 从配置文件迁移到数据库

**旧方式** (config.toml):
```toml
[memory]
base_url = "https://api.openai.com/v1"
api_key = "sk-..."
model = "gpt-4"
```

**新方式** (API):
```bash
# 1. 创建 provider
curl -X POST http://localhost:8080/providers \
  -d '{"name":"OpenAI","client_type":"openai","base_url":"https://api.openai.com/v1","api_key":"sk-..."}'

# 2. 创建 model
curl -X POST http://localhost:8080/models \
  -d '{"model_id":"gpt-4","llm_provider_id":"<provider-id>","type":"chat","enable_as":"memory"}'
```

## 相关文档

- [Provider 重构总结](./REFACTORING_SUMMARY.md)
- [Agent 迁移文档](./AGENT_MIGRATION.md)
- [Chat 架构文档](./internal/chat/ARCHITECTURE.md)
- [Models API 文档](#) (待创建)

## TODO

- [ ] 实现 provider 连接测试端点
- [ ] 添加 provider 使用统计
- [ ] 实现 provider 健康检查
- [ ] 添加 provider 费用跟踪
- [ ] 支持 provider 负载均衡
- [ ] 实现 provider 故障转移

