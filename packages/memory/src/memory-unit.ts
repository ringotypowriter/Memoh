import { ModelMessage } from 'ai'

export interface MemoryUnit {
  messages: ModelMessage[]
  timestamp: Date
  user: string
  raw: string
}