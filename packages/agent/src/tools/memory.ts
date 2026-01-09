import { MemoryUnit } from '@memohome/memory'
import { tool } from 'ai'
import { z } from 'zod'

export interface GetMemoryToolParams {
  searchMemory: (query: string) => Promise<MemoryUnit[]>
  onLoadMemory: (memory: MemoryUnit[]) => Promise<void>
}

export const getMemoryTools = ({ searchMemory, onLoadMemory }: GetMemoryToolParams) => {
  const searchMemoryTool = tool({
    description: 'Search chat history in the memory',
    inputSchema: z.object({
      query: z.string().describe('The query to search the memory'),
    }),
    execute: async ({ query }) => {
      const memory = await searchMemory(query)
      onLoadMemory(memory)
      return {
        success: true,
        message: `${memory.length} memories has load into your context`,
      }
    },
  })

  return {
    'search-memory': searchMemoryTool,
  }
}