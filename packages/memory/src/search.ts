import { embed } from 'ai'
import { filterByEmbedding } from './filter'
import { EmbedParams } from './types'
import { createOpenAI } from '@ai-sdk/openai'

// eslint-disable-next-line @typescript-eslint/no-empty-object-type
export interface MemorySearchParams extends EmbedParams { }

export interface MemorySearchInput {
  user: string
  query: string
  maxResults?: number
}

export const createMemorySearch = (params: MemorySearchParams) =>
  async ({ user, query, maxResults = 10 }: MemorySearchInput) => {
    const { embedding } = await embed({
      model: createOpenAI({
        apiKey: params.apiKey,
        baseURL: params.baseURL,
      }).embedding(params.model),
      value: query,
    })
    return await filterByEmbedding(embedding, user, maxResults)
  }