import { createClient, requireAuth } from './client'
import type { Platform, ApiResponse } from '../types'

export interface CreatePlatformParams {
  name: string
  endpoint: string
  config: Record<string, unknown>
  active?: boolean
}

export interface PlatformListItem {
  id: string
  name: string
  endpoint: string
  config: Record<string, unknown>
  active: boolean
  createdAt: string
  updatedAt: string
}

/**
 * List all platforms
 */
export async function listPlatforms(): Promise<PlatformListItem[]> {
  requireAuth()
  const client = createClient()
  
  const response = await client.platform.get()

  if (response.error) {
    throw new Error(response.error.value)
  }

  const data = response.data as { success?: boolean; items?: PlatformListItem[] } | null
  if (data?.success && data?.items) {
    return data.items
  }
  
  throw new Error('Failed to fetch platform list')
}

/**
 * Create platform configuration
 */
export async function createPlatform(params: CreatePlatformParams): Promise<Platform> {
  requireAuth()
  const client = createClient()

  const payload: Record<string, unknown> = {
    name: params.name,
    endpoint: params.endpoint,
    config: params.config,
    active: params.active ?? true,
  }

  const response = await client.platform.post(payload)

  if (response.error) {
    throw new Error(response.error.value)
  }

  const data = response.data as ApiResponse<Platform> | null
  if (data?.success && data?.data) {
    return data.data
  }
  
  throw new Error('Failed to create platform configuration')
}

/**
 * Get platform by ID
 */
export async function getPlatform(id: string): Promise<Platform> {
  requireAuth()
  const client = createClient()

  const response = await client.platform({ id }).get()

  if (response.error) {
    throw new Error(response.error.value)
  }

  const data = response.data as ApiResponse<Platform> | null
  if (data?.success && data?.data) {
    return data.data
  }
  
  throw new Error('Failed to fetch platform configuration')
}

/**
 * Update platform
 */
export async function updatePlatform(id: string, params: Partial<CreatePlatformParams>): Promise<Platform> {
  requireAuth()
  const client = createClient()

  const response = await client.platform({ id }).put(params)

  if (response.error) {
    throw new Error(response.error.value)
  }

  const data = response.data as ApiResponse<Platform> | null
  if (data?.success && data?.data) {
    return data.data
  }
  
  throw new Error('Failed to update platform configuration')
}

/**
 * Update platform config
 */
export async function updatePlatformConfig(id: string, config: Record<string, unknown>): Promise<Platform> {
  requireAuth()
  const client = createClient()

  const response = await client.platform({ id }).config.put({ config })

  if (response.error) {
    throw new Error(response.error.value)
  }

  const data = response.data as ApiResponse<Platform> | null
  if (data?.success && data?.data) {
    return data.data
  }
  
  throw new Error('Failed to update platform config')
}

/**
 * Delete platform
 */
export async function deletePlatform(id: string): Promise<void> {
  requireAuth()
  const client = createClient()

  const response = await client.platform({ id }).delete()

  if (response.error) {
    throw new Error(response.error.value)
  }
}

/**
 * Activate platform
 */
export async function activatePlatform(id: string): Promise<Platform> {
  requireAuth()
  const client = createClient()

  const response = await client.platform({ id }).active.post()

  if (response.error) {
    throw new Error(response.error.value)
  }

  const data = response.data as ApiResponse<Platform> | null
  if (data?.success && data?.data) {
    return data.data
  }
  
  throw new Error('Failed to activate platform')
}

/**
 * Inactivate platform
 */
export async function inactivatePlatform(id: string): Promise<Platform> {
  requireAuth()
  const client = createClient()

  const response = await client.platform({ id }).inactive.post()

  if (response.error) {
    throw new Error(response.error.value)
  }

  const data = response.data as ApiResponse<Platform> | null
  if (data?.success && data?.data) {
    return data.data
  }
  
  throw new Error('Failed to inactivate platform')
}

