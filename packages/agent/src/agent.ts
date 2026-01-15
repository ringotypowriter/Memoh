import { streamText, generateText, ModelMessage, stepCountIs, UserModelMessage, Tool } from 'ai'
import { AgentParams } from './types'
import { system, schedule as schedulePrompt } from './prompts'
import { getMemoryTools, getScheduleTools, getMessageTools } from './tools'
import { createChatGateway } from '@memoh/ai-gateway'
import { MCPConnection, Schedule } from '@memoh/shared'
import { createMCPClient, MCPClient } from '@ai-sdk/mcp'
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js'

export const createAgent = (params: AgentParams) => {
  const messages: ModelMessage[] = []
  const mcpClients: MCPClient[] = []

  const gateway = createChatGateway(params.model)

  const maxContextLoadTime = params.maxContextLoadTime ?? 24 * 60 // 24 hours
  const language = params.language ?? 'Same as user input'
  const platforms = params.platforms ?? []
  const currentPlatform = params.platforms
    ? platforms.find(p => p.name === params.currentPlatform)?.name ?? 'Unknown Platform'
    : 'client'
  const mcpConnections = params.mcpConnections ?? []

  const launchMCPConnections = async () => {
    const launch = async (connection: MCPConnection) => {
      if (connection.type === 'http' || connection.type === 'sse') {
        return await createMCPClient({
          transport: {
            url: connection.url,
            headers: connection.headers,
            type: connection.type,
          }
        })
      } else if (connection.type === 'stdio') {
        // Build exec command for container execution
        const commands = params.onBuildExecCommand?.([connection.command]) ?? [connection.command]
        commands.push(...connection.args)
        const [command, ...args] = commands
        return await createMCPClient({
          transport: new StdioClientTransport({
            command,
            args,
            env: connection.env,
            cwd: connection.cwd,
          }),
        })
      }
    }
    const connections = await Promise.all(mcpConnections.map(launch))
    return connections.filter(connection => connection !== undefined)
  }

  const getTools = async () => {
    const connections = await launchMCPConnections()
    mcpClients.length = 0
    mcpClients.push(...connections)
    const mcpTools = await Promise.all(connections.map(connection => connection.tools())) as Record<string, Tool>[]
    const tools = Object.assign({}, ...mcpTools)
    return {
      ...getMemoryTools({
        searchMemory: params.onSearchMemory ?? (() => Promise.resolve([]))
      }),
      ...getScheduleTools({
        onGetSchedules: params.onGetSchedules ?? (() => Promise.resolve([])),
        onRemoveSchedule: params.onRemoveSchedule ?? (() => Promise.resolve()),
        onSchedule: params.onSchedule ?? (() => Promise.resolve()),
      }),
      ...getMessageTools(
        platforms,
        params.onSendMessage ?? (() => Promise.resolve())
      ),
      ...tools,
    }
  }

  const onComplete = async () => {
    await Promise.all(mcpClients.map(client => client.close()))
  }

  const loadContext = async () => {
    const from = new Date(Date.now() - maxContextLoadTime * 60 * 1000)
    const to = new Date()
    const memory = await params.onReadMemory?.(from, to) ?? []
    const context = memory.flatMap(m => m.messages)
    messages.unshift(...context)
  }

  const getSystemPrompt = () => {
    return system({
      date: new Date(),
      language,
      locale: params.locale,
      maxContextLoadTime,
      platforms,
      currentPlatform,
    })
  }

  const getSchedulePrompt = (schedule: Schedule) => {
    return schedulePrompt({
      schedule,
      locale: params.locale,
      date: new Date(),
    })
  }

  async function askDirectly(input: string) {
    await loadContext()
    const user = {
      role: 'user',
      content: input,
    } as UserModelMessage
    messages.push(user)
    const { response } = await generateText({
      model: gateway,
      system: getSystemPrompt(),
      messages,
      tools: await getTools(),
      onFinish: async () => {
        await onComplete()
      },
    })
    await params.onFinish?.([
      user as ModelMessage,
      ...response.messages,
    ])
  }

  async function* ask(input: string) {
    try {
      await loadContext()
      const user = {
        role: 'user',
        content: input,
      } as UserModelMessage
      messages.push(user)
      const { fullStream, response } = streamText({
        model: gateway,
        system: getSystemPrompt(),
        prepareStep: async () => {
          return {
            system: getSystemPrompt(),
          }
        },
        stopWhen: stepCountIs(10),
        messages,
        tools: await getTools(),
        onFinish: async () => {
          await onComplete()
        },
      })
      for await (const event of fullStream) {
        yield event
      }
      
      // Wait for response and save to memory
      try {
        const newMessages = (await response).messages
        await params.onFinish?.([
          user as ModelMessage,
          ...newMessages,
        ])
      } catch (finishError) {
        console.error('Error in onFinish callback:', finishError)
        // Yield error event but don't throw - let the stream complete
        yield {
          type: 'error' as const,
          error: finishError instanceof Error ? finishError.message : 'Failed to save conversation'
        }
      }
    } catch (error) {
      console.error('Error in agent.ask():', error)
      yield {
        type: 'error' as const,
        error: error instanceof Error ? error.message : 'Unknown error occurred'
      }
    }
  }

  const triggerSchedule = async (schedule: Schedule) => {
    const prompt = getSchedulePrompt(schedule)
    await askDirectly(prompt)
  }

  return {
    ask,
    askDirectly,
    loadContext,
    getSystemPrompt,
    getSchedulePrompt,
    triggerSchedule,
    onComplete,
    launchMCPConnections,
  }
}