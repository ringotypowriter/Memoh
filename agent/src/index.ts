import { Elysia } from 'elysia'
import { chatModule } from './modules/chat'
import { corsMiddleware } from './middlewares/cors'
import { errorMiddleware } from './middlewares/error'
import { loadConfig } from './config'

const config = loadConfig('../config.toml')

export const getBraveConfig = () => {
  return {
    apiKey: config.brave.api_key ?? '',
    baseUrl: config.brave.base_url ?? 'https://api.search.brave.com/res/v1/',
  }
}

export const getBaseUrl = () => {
  const rawAddr =
    typeof config.agent_gateway.server_addr === 'string'
      ? config.agent_gateway.server_addr.trim()
      : typeof config.server.addr === 'string'
        ? config.server.addr.trim()
        : ''

  if (!rawAddr) {
    return 'http://127.0.0.1'
  }

  if (rawAddr.startsWith('http://') || rawAddr.startsWith('https://')) {
    return rawAddr.replace(/\/+$/, '')
  }

  if (rawAddr.startsWith(':')) {
    return `http://127.0.0.1${rawAddr}`
  }

  return `http://${rawAddr}`
}

export type AuthFetcher = (
  url: string,
  options?: RequestInit,
) => Promise<Response>;
export const createAuthFetcher = (bearer: string | undefined): AuthFetcher => {
  return async (url: string, options?: RequestInit) => {
    const requestOptions = options ?? {}
    const headers = new Headers(requestOptions.headers || {})
    if (bearer) {
      headers.set('Authorization', `Bearer ${bearer}`)
    }

    const requestUrl = new URL(
      url,
      `${getBaseUrl().replace(/\/+$/, '')}/`,
    ).toString()

    return await fetch(requestUrl, {
      ...requestOptions,
      headers,
    })
  }
}

const app = new Elysia()
  .use(corsMiddleware)
  .use(errorMiddleware)
  .get('/health', () => ({
    status: 'ok',
  }))
  .use(chatModule)
  .listen({
    port: config.agent_gateway.port ?? 8081,
    hostname: config.agent_gateway.host ?? '127.0.0.1',
    idleTimeout: 255, // max allowed by Bun, to accommodate long-running tool calls
  })

console.log(
  `Agent Gateway is running at ${app.server?.hostname}:${app.server?.port}`,
)
