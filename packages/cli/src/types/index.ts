// API response type definitions

export interface ApiResponse<T = unknown> {
  success?: boolean
  data?: T
  error?: string
}

export interface User {
  id: string
  username: string
  role: string
  createdAt: string
}

export interface Model {
  id: string
  name: string
  modelId: string
  baseUrl: string
  apiKey?: string
  clientType: string
  type?: 'chat' | 'embedding'
  dimensions?: number
  createdAt: string
  updatedAt?: string
}

export interface Memory {
  content: string
  timestamp: string
  similarity?: number
}

export interface Message {
  role: 'user' | 'assistant'
  content: string
  timestamp: string
}

export interface MessageListResponse {
  messages: Message[]
  pagination: {
    page: number
    limit: number
    total: number
    totalPages: number
  }
}

export interface Settings {
  userId: string
  language?: string
  maxContextLoadTime?: number
  defaultChatModel?: string
  defaultSummaryModel?: string
  defaultEmbeddingModel?: string
  createdAt: string
  updatedAt: string
}

export interface Schedule {
  id: string
  title: string
  description?: string
  cronExpression: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface Platform {
  id: string
  name: string
  endpoint: string
  config: Record<string, unknown>
  active: boolean
  createdAt: string
  updatedAt: string
}

