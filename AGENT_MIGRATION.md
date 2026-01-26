# Agent Prompts 初步迁移

## 概述

本文档记录了从 TypeScript agent (`@packages/agent`) 到 Go chat 服务的初步迁移工作。

## 完成的工作

### 1. 删除 Config 中的 Memory 配置

由于现在使用数据库中的模型配置，不再需要配置文件中的 memory 配置。

**变更文件**:
- ✅ `config.toml.example` - 删除 `[memory]` 配置段
- ✅ `internal/config/config.go` - 删除 `MemoryConfig` 结构
- ✅ `cmd/agent/main.go` - 移除对 `cfg.Memory` 的引用

**影响**:
- Memory 服务现在完全依赖数据库中配置的模型
- 如果数据库中没有配置模型，服务会启动失败并提示用户配置
- 更清晰的配置管理，避免配置分散

### 2. 迁移 Agent Prompts 到 Chat 包

从 TypeScript 的 `packages/agent/src/prompts/` 迁移到 Go 的 `internal/chat/prompts.go`。

**迁移的内容**:

#### System Prompt (系统提示词)
- ✅ 基础系统提示词
- ✅ 日期时间格式化
- ✅ 语言设置
- ✅ 平台信息（available-platforms, current-platform）
- ✅ 上下文加载时间配置
- ✅ 响应指南

#### Schedule Prompt (定时任务提示词)
- ✅ 定时任务触发提示
- ✅ 任务信息（名称、描述、ID、最大调用次数、Cron 模式）
- ✅ 命令内容

#### 辅助函数
- ✅ `FormatTime()` - 时间格式化
- ✅ `Quote()` - Markdown 代码格式化
- ✅ `Block()` - 代码块格式化

**暂未迁移（后续工作）**:
- ⏸️ Memory 工具说明
- ⏸️ Schedule 工具说明
- ⏸️ Message 工具说明
- ⏸️ MCP 工具集成
- ⏸️ 工具调用逻辑

## 新的 Prompt 结构

### SystemPrompt

```go
type PromptParams struct {
    Date               time.Time
    Locale             string
    Language           string
    MaxContextLoadTime int      // 上下文加载时间（分钟）
    Platforms          []string // 可用平台列表
    CurrentPlatform    string   // 当前平台
}

func SystemPrompt(params PromptParams) string
```

**示例**:
```go
prompt := chat.SystemPrompt(chat.PromptParams{
    Date:               time.Now(),
    Locale:             "zh-CN",
    Language:           "Chinese",
    MaxContextLoadTime: 24 * 60,
    Platforms:          []string{"telegram", "wechat"},
    CurrentPlatform:    "telegram",
})
```

### SchedulePrompt

```go
type SchedulePromptParams struct {
    Date                time.Time
    Locale              string
    ScheduleName        string
    ScheduleDescription string
    ScheduleID          string
    MaxCalls            *int   // nil 表示无限次
    CronPattern         string
    Command             string
}

func SchedulePrompt(params SchedulePromptParams) string
```

**示例**:
```go
maxCalls := 1
prompt := chat.SchedulePrompt(chat.SchedulePromptParams{
    Date:                time.Now(),
    Locale:              "zh-CN",
    ScheduleName:        "早餐提醒",
    ScheduleDescription: "每天早上 7 点提醒吃早餐",
    ScheduleID:          "schedule-123",
    MaxCalls:            &maxCalls,
    CronPattern:         "0 7 * * *",
    Command:             "提醒用户吃早餐，推荐健康食谱",
})
```

## 使用方式

### Chat Resolver 中的使用

Chat Resolver 会自动为每个请求添加系统提示词：

```go
// 在 resolver.go 中
systemPrompt := SystemPrompt(PromptParams{
    Date:               time.Now(),
    Locale:             "en-US",
    Language:           "Same as user input",
    MaxContextLoadTime: 24 * 60,
    Platforms:          []string{},
    CurrentPlatform:    "api",
})
```

### 自定义 Prompt 参数

未来可以通过以下方式自定义：

1. **从数据库加载平台列表**:
```go
platforms, _ := platformService.GetActivePlatforms(ctx)
platformNames := make([]string, len(platforms))
for i, p := range platforms {
    platformNames[i] = p.Name
}
```

2. **从用户设置加载语言偏好**:
```go
userSettings, _ := settingsService.GetUserSettings(ctx, userID)
language := userSettings.Language
```

3. **从会话上下文获取当前平台**:
```go
currentPlatform := "telegram" // 从请求头或会话中获取
```

## 对比 TypeScript 版本

### TypeScript (原始)
```typescript
export const system = ({ date, locale, language, maxContextLoadTime, platforms, currentPlatform }: SystemParams) => {
  return `
---
${time({ date, locale })}
language: ${language}
available-platforms:
${platforms.map(platform => `  - ${platform.name}`).join('\n')}
current-platform: ${currentPlatform}
---
You are a personal housekeeper assistant...
  `.trim()
}
```

### Go (迁移后)
```go
func SystemPrompt(params PromptParams) string {
    timeStr := FormatTime(params.Date, params.Locale)
    platformsList := buildPlatformsList(params.Platforms)
    
    return fmt.Sprintf(`---
%s
language: %s
available-platforms:
%s
current-platform: %s
---
You are a personal housekeeper assistant...`,
        timeStr, params.Language, platformsList, params.CurrentPlatform)
}
```

## 配置迁移指南

### 旧配置 (config.toml)
```toml
[memory]
base_url = "https://api.openai.com/v1"
api_key = "sk-..."
model = "gpt-4.1-nano"
timeout_seconds = 10
```

### 新配置 (数据库)
```sql
-- 1. 创建 LLM Provider
INSERT INTO llm_providers (name, client_type, base_url, api_key)
VALUES ('OpenAI', 'openai', 'https://api.openai.com/v1', 'sk-...');

-- 2. 创建 Chat 模型（用于 memory）
INSERT INTO models (model_id, name, llm_provider_id, type, enable_as)
VALUES ('gpt-4-turbo', 'GPT-4 Turbo', '<provider-uuid>', 'chat', 'memory');

-- 或使用现有的 chat 模型
UPDATE models SET enable_as = 'memory' WHERE model_id = 'gpt-4-turbo';
```

## 测试建议

### 1. 测试系统提示词
```bash
# 发起聊天请求
curl -X POST http://localhost:8080/api/chat \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "messages": [
      {"role": "user", "content": "你好"}
    ]
  }'
```

检查响应中是否：
- 使用了正确的语言
- AI 理解自己是个人管家助手
- 响应风格友好且有帮助

### 2. 验证配置迁移
```bash
# 启动服务，检查日志
# 应该看到：
# Using memory model: gpt-4-turbo (provider: openai)

# 不应该看到：
# WARNING: No memory model configured, using fallback LLMClient
```

### 3. 测试 Memory 操作
```bash
# 添加记忆
curl -X POST http://localhost:8080/api/memory/add \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "messages": [
      {"role": "user", "content": "我喜欢吃披萨"}
    ],
    "user_id": "user-123"
  }'

# 搜索记忆
curl -X POST http://localhost:8080/api/memory/search \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "query": "我喜欢什么食物?",
    "user_id": "user-123"
  }'
```

## 后续工作

### 短期（工具集成）
- [ ] 添加 Memory 工具说明到系统提示词
- [ ] 添加 Schedule 工具说明到系统提示词
- [ ] 添加 Message 工具说明到系统提示词
- [ ] 实现工具调用功能
- [ ] 添加工具参数验证

### 中期（MCP 集成）
- [ ] MCP 连接管理
- [ ] MCP 工具动态加载
- [ ] MCP stdio/http/sse 传输支持
- [ ] 容器环境中的 MCP 执行

### 长期（功能完善）
- [ ] 多语言本地化支持
- [ ] 用户自定义系统提示词
- [ ] Prompt 模板系统
- [ ] A/B 测试不同 Prompt 版本
- [ ] Prompt 性能监控

## 文件清单

### 删除/清理
- ✅ `config.toml.example` - 删除 `[memory]` 段
- ✅ `internal/config/config.go` - 删除 `MemoryConfig`
- ✅ `cmd/agent/main.go` - 删除 `cfg.Memory` 引用

### 修改
- ✅ `internal/chat/prompts.go` - 完全重写，添加完整的 prompt 系统
- ✅ `internal/chat/resolver.go` - 使用新的 `SystemPrompt` 函数

### 新增
- ✅ `AGENT_MIGRATION.md` - 本文档

## 注意事项

1. **必须配置数据库模型**: 由于删除了配置文件中的回退配置，必须在数据库中配置至少一个 chat 模型

2. **Prompt 参数**: 当前使用硬编码的默认值，未来应该从用户设置或请求上下文中获取

3. **多语言支持**: `FormatTime()` 当前使用标准格式，未来应该使用 i18n 库进行本地化

4. **工具说明**: 当前 prompt 中提到了工具能力，但实际的工具说明还未添加，需要后续实现

5. **向后兼容**: 删除了配置文件中的 memory 配置，如果有旧的部署需要迁移

## 迁移检查清单

- [x] 删除 config.toml 中的 memory 配置
- [x] 删除 Config 结构中的 MemoryConfig
- [x] 更新 main.go 移除 cfg.Memory 引用
- [x] 迁移系统提示词
- [x] 迁移定时任务提示词
- [x] 迁移辅助函数
- [x] 更新 resolver 使用新的 prompt
- [x] 通过 linter 检查
- [x] 编写迁移文档
- [ ] 测试 chat 功能
- [ ] 测试 memory 功能
- [ ] 更新部署文档

## 相关文档

- [Provider 重构总结](./REFACTORING_SUMMARY.md)
- [Chat 架构文档](./internal/chat/ARCHITECTURE.md)
- [TypeScript Agent 源码](./packages/agent/src/)

