import { createAgent as createAgentService } from '@memoh/agent'
import { createMemory, filterByTimestamp, MemoryUnit } from '@memoh/memory'
import { ChatModel, EmbeddingModel, Platform, Schedule } from '@memoh/shared'
import { useContainer } from '@memoh/container'
import { createSchedule, deleteSchedule, getActiveSchedules } from '../schedule/service'
import { getActivePlatforms, sendMessageToPlatform } from '../platform/service'
import { getActiveMCPConnections } from '../mcp/service'
import { getUserContainerInfo } from '../container/service'

// Type for messages passed to onFinish callback
type MessageType = Record<string, unknown>

export interface CreateAgentStreamParams {
  userId: string
  chatModel: ChatModel
  embeddingModel: EmbeddingModel
  summaryModel: ChatModel
  maxContextLoadTime?: number
  language?: string
  platform?: string
  onFinish?: (messages: MessageType[]) => Promise<void>
}

export async function createAgent(params: CreateAgentStreamParams) {
  const {
    userId,
    chatModel,
    embeddingModel,
    summaryModel,
    maxContextLoadTime,
    language,
    platform,
    onFinish,
  } = params

  // Create memory instance
  const memoryInstance = createMemory({
    summaryModel,
    embeddingModel,
  })

  const platforms = await getActivePlatforms()
  const mcpConnections = await getActiveMCPConnections(userId)
  const containerInfo = await getUserContainerInfo(userId)
  if (!containerInfo) {
    throw new Error('Container not found')
  }

  // Ensure container is running before creating agent
  const container = useContainer(containerInfo.containerName, {
    namespace: containerInfo.namespace,
  })
  
  // Check and start container if not running
  const info = await container.info()
  if (info.status !== 'running') {
    console.log(`🚀 Starting container ${containerInfo.containerName} for agent...`)
    await container.start()
    // Wait a bit for container to be fully ready
    await new Promise(resolve => setTimeout(resolve, 2000))
  }
  // Create agent
  const agent = createAgentService({
    model: chatModel,
    maxContextLoadTime,
    language: language || 'Same as user input',
    platforms: platforms as Platform[],
    currentPlatform: platform,
    mcpConnections,
    onSendMessage: async (platform: string, options) => {
      await sendMessageToPlatform(platform, {
        message: options.message,
        userId,
      })
    },
    onReadMemory: async (from: Date, to: Date) => {
      return await filterByTimestamp(from, to, userId)
    },
    onSearchMemory: async (query: string) => {
      const results = await memoryInstance.searchMemory(query, userId)
      return results
    },
    onFinish: async (messages: MessageType[]) => {
      // Save conversation to memory
      const memoryUnit: MemoryUnit = {
        messages: messages as unknown as MemoryUnit['messages'],
        timestamp: new Date(),
        user: userId,
      }
      await memoryInstance.addMemory(memoryUnit)
      
      // Call custom onFinish handler if provided
      await onFinish?.(messages)
    },
    onGetSchedules: async () => {
      const schedules = await getActiveSchedules(userId)
      return schedules.map(schedule => ({
        id: schedule.id!,
        pattern: schedule.pattern,
        name: schedule.name,
        description: schedule.description,
        command: schedule.command,
        maxCalls: schedule.maxCalls || undefined,
      }))
    },
    onRemoveSchedule: async (id: string) => {
      await deleteSchedule(id, userId)
    },
    onSchedule: async (schedule: Schedule) => {
      await createSchedule(userId, {
        name: schedule.name,
        description: schedule.description,
        command: schedule.command,
        pattern: schedule.pattern,
        maxCalls: schedule.maxCalls || undefined,
      })
    },
    onBuildExecCommand(command) {
      return container.buildExecCommand(command)
    },
    async onExecCommand(command) {
      return await container.exec(command)
    },
  })

  return agent
}

