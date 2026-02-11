import { createMCPClient } from '@ai-sdk/mcp'
export const getMCPTools = async (url: string, headers: Record<string, string> = {}) => {
  const client = await createMCPClient({
    transport: {
      type: 'http',
      url,
      headers,
    }
  })
  const tools = await client.tools()
  return {
    tools,
    close: async () => {
      await client.close()
    }
  }
}
