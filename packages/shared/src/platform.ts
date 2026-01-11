export interface Platform {
  id: string
  name: string
  endpoint: string
  config: Record<string, unknown>
  active: boolean
}