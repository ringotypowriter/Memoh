import { createGateway as createAiGateway } from 'ai'
import { createOpenAI } from '@ai-sdk/openai'
import { createAnthropic } from '@ai-sdk/anthropic'
import { createGoogleGenerativeAI } from '@ai-sdk/google'
import { BaseModel, ModelClientType } from '@memohome/shared'

export const createGateway = (model: BaseModel) => {
  const clients = {
    [ModelClientType.OPENAI]: createOpenAI,
    [ModelClientType.ANTHROPIC]: createAnthropic,
    [ModelClientType.GOOGLE]: createGoogleGenerativeAI,
  }
  return (clients[model.clientType] ?? createAiGateway)({
    apiKey: model.apiKey,
    baseURL: model.baseUrl,
  })(model.modelId)
}