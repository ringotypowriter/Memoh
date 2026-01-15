import { existsSync, mkdirSync } from 'fs'
import { join } from 'path'

/**
 * Container data directory configuration
 */
const CONTAINER_BASE_DIR = process.env.CONTAINER_DATA_DIR || '/var/lib/memoh/containers'

/**
 * Get host path for user container
 * @param userId - User ID
 * @returns Host path for the container
 */
export function getUserContainerHostPath(userId: string): string {
  return join(CONTAINER_BASE_DIR, userId)
}

/**
 * Ensure directory exists, create if not
 * @param path - Directory path
 */
export function ensureDirectoryExists(path: string): void {
  if (!existsSync(path)) {
    mkdirSync(path, { recursive: true, mode: 0o755 })
    console.log(`📁 Created directory: ${path}`)
  }
}

/**
 * Initialize container base directory
 */
export function initializeContainerBaseDirectory(): void {
  ensureDirectoryExists(CONTAINER_BASE_DIR)
  console.log(`✅ Container base directory initialized: ${CONTAINER_BASE_DIR}`)
}

/**
 * Get container paths for a user
 * @param userId - User ID
 * @returns Object with host and container paths
 */
export function getContainerPaths(userId: string) {
  return {
    hostPath: getUserContainerHostPath(userId),
    containerPath: '/data',
  }
}

