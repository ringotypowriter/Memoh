import type { MemoryUnit } from '@memohome/memory'
import { BaseModel } from '@memohome/shared'
import { ModelMessage } from 'ai'

export interface AgentParams {
  model: BaseModel

  /**
   * Unit: minutes
   */
  maxContextLoadTime: number

  locale?: Intl.LocalesArgument

  /**
   * Preferred language of the assistant.
   * @default 'Same as user input'
   */
  language?: string

  onReadMemory?: (from: Date, to: Date) => Promise<MemoryUnit[]>

  onSearchMemory?: (query: string) => Promise<MemoryUnit[]>

  onFinish?: (messages: ModelMessage[]) => Promise<void>

  onError?: (error: Error) => Promise<void>
}