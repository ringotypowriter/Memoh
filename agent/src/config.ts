import { readFileSync } from 'fs'
import { parse } from 'toml'

type AgentGatewayConfig = {
  'agent_gateway': {
    host?: string
    server_addr?: string
    port?: number
  },
  'server': {
    addr?: string
  },
  'brave': {
    api_key?: string
    base_url?: string
  }
}

export const loadConfig = (path: string = './config.toml'): AgentGatewayConfig => {
  const config = parse(readFileSync(path, 'utf-8'))
  return config satisfies AgentGatewayConfig
}