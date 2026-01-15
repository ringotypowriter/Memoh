import {
  getAllContainers as dbGetAllContainers,
  getAutoStartContainers,
  getContainerByUserId,
  createContainerRecord,
  updateContainerStatus,
  deleteContainerRecord,
  type ContainerInfo,
} from '@memoh/db'
import { createContainer, useContainer, containerExists, type ContainerConfig } from '@memoh/container'
import { getContainerPaths, ensureDirectoryExists } from './utils'

/**
 * 获取所有容器
 */
export const getAllContainers = async (): Promise<ContainerInfo[]> => {
  return await dbGetAllContainers()
}

/**
 * 为用户创建容器
 */
export const createUserContainer = async (
  userId: string,
  image: string = 'docker.io/library/node:20-alpine',
  namespace: string = 'default'
): Promise<ContainerInfo> => {
  // 检查用户是否已有容器
  const existing = await getContainerByUserId(userId)
  if (existing) {
    throw new Error('User already has a container')
  }

  const containerName = `user-${userId.slice(0, 8)}-container`
  
  // 检查 containerd 中是否已存在同名容器
  try {
    const exists = await containerExists(containerName, { namespace })
    if (exists) {
      console.log(`⚠️  Container ${containerName} already exists in containerd, syncing to database...` )
      
      // 获取容器信息并同步到数据库
      const ops = useContainer(containerName, { namespace })
      const info = await ops.info()
      
      const paths = getContainerPaths(userId)
      const dbRecord = await createContainerRecord({
        userId,
        containerId: info.id,
        containerName: info.name,
        image: info.image,
        namespace,
        autoStart: true,
        hostPath: paths.hostPath,
        containerPath: paths.containerPath,
      })
      
      return dbRecord
    }
  } catch (error) {
    console.error('Error checking container existence:', error)
  }
  
  // 获取挂载路径
  const paths = getContainerPaths(userId)
  
  // 确保宿主机目录存在
  ensureDirectoryExists(paths.hostPath)

  // 创建容器配置
  const config: ContainerConfig = {
    name: containerName,
    image,
    command: ['sh', '-c', 'while true; do sleep 3600; done'], // 保持容器运行
    namespace,
    labels: {
      userId,
      managedBy: 'memoh-api',
    },
    mounts: [
      {
        type: 'bind',
        source: paths.hostPath,
        target: paths.containerPath,
        readonly: false,
      },
    ],
  }

  // 在 containerd 中创建容器
  const containerInfo = await createContainer(config, { 
    namespace,
    ctrCommand: process.env.CTR_COMMAND || 'ctr',
  })

  // 在数据库中记录
  const dbRecord = await createContainerRecord({
    userId,
    containerId: containerInfo.id,
    containerName: containerInfo.name,
    image: containerInfo.image,
    namespace,
    autoStart: true,
    hostPath: paths.hostPath,
    containerPath: paths.containerPath,
  })

  console.log(`✅ Created container with mount: ${paths.hostPath} -> ${paths.containerPath}`)

  return dbRecord
}

/**
 * 启动用户容器
 */
export const startUserContainer = async (userId: string): Promise<void> => {
  const container = await getContainerByUserId(userId)
  if (!container) {
    throw new Error('Container not found for user')
  }

  const ops = useContainer(container.containerName, { namespace: container.namespace })
  await ops.start()

  // 更新数据库状态
  await updateContainerStatus(container.containerId, 'running')
}

/**
 * 停止用户容器
 */
export const stopUserContainer = async (userId: string, timeout: number = 10): Promise<void> => {
  const container = await getContainerByUserId(userId)
  if (!container) {
    throw new Error('Container not found for user')
  }

  const ops = useContainer(container.containerName, { namespace: container.namespace })
  await ops.stop(timeout)

  // 更新数据库状态
  await updateContainerStatus(container.containerId, 'stopped')
}

/**
 * 重启用户容器
 */
export const restartUserContainer = async (userId: string): Promise<void> => {
  const container = await getContainerByUserId(userId)
  if (!container) {
    throw new Error('Container not found for user')
  }

  const ops = useContainer(container.containerName, { namespace: container.namespace })
  await ops.restart()

  // 更新数据库状态
  await updateContainerStatus(container.containerId, 'running')
}

/**
 * 暂停用户容器
 */
export const pauseUserContainer = async (userId: string): Promise<void> => {
  const container = await getContainerByUserId(userId)
  if (!container) {
    throw new Error('Container not found for user')
  }

  const ops = useContainer(container.containerName, { namespace: container.namespace })
  await ops.pause()

  // 更新数据库状态
  await updateContainerStatus(container.containerId, 'paused')
}

/**
 * 恢复用户容器
 */
export const resumeUserContainer = async (userId: string): Promise<void> => {
  const container = await getContainerByUserId(userId)
  if (!container) {
    throw new Error('Container not found for user')
  }

  const ops = useContainer(container.containerName, { namespace: container.namespace })
  await ops.resume()

  // 更新数据库状态
  await updateContainerStatus(container.containerId, 'running')
}

/**
 * 删除用户容器
 */
export const deleteUserContainer = async (userId: string, force: boolean = false): Promise<void> => {
  const container = await getContainerByUserId(userId)
  if (!container) {
    throw new Error('Container not found for user')
  }

  const ops = useContainer(container.containerName, { namespace: container.namespace })
  await ops.remove(force)

  // 从数据库删除记录
  await deleteContainerRecord(container.id)
}

/**
 * 获取用户容器信息
 */
export const getUserContainerInfo = async (userId: string): Promise<ContainerInfo | undefined> => {
  return await getContainerByUserId(userId)
}

/**
 * 启动所有自动启动的容器
 */
export const startAllAutoStartContainers = async (): Promise<{ success: number; failed: number }> => {
  const containers = await getAutoStartContainers()
  let success = 0
  let failed = 0

  console.log(`🚀 Starting ${containers.length} auto-start containers...`)

  for (const container of containers) {
    try {
      const ops = useContainer(container.containerName, { namespace: container.namespace })
      
      // 获取当前状态
      const info = await ops.info()
      
      // 只有非运行状态才启动
      if (info.status !== 'running') {
        await ops.start()
        await updateContainerStatus(container.containerId, 'running')
        console.log(`✅ Started container: ${container.containerName}`)
        success++
      } else {
        console.log(`⏭️  Container already running: ${container.containerName}`)
        success++
      }
    } catch (error) {
      console.error(`❌ Failed to start container ${container.containerName}:`, error)
      failed++
      // 更新状态为 unknown
      await updateContainerStatus(container.containerId, 'unknown')
    }
  }

  console.log(`✨ Container startup complete: ${success} succeeded, ${failed} failed`)
  
  return { success, failed }
}

/**
 * 暂停所有运行中的容器
 */
export const pauseAllContainers = async (): Promise<{ success: number; failed: number }> => {
  const containers = await dbGetAllContainers()
  let success = 0
  let failed = 0

  console.log(`⏸️  Pausing ${containers.length} containers...`)

  for (const container of containers) {
    try {
      const ops = useContainer(container.containerName, { namespace: container.namespace })
      
      // 获取当前状态
      const info = await ops.info()
      
      // 只暂停运行中的容器
      if (info.status === 'running') {
        await ops.pause()
        await updateContainerStatus(container.containerId, 'paused')
        console.log(`✅ Paused container: ${container.containerName}`)
        success++
      } else {
        console.log(`⏭️  Container not running, skipped: ${container.containerName}`)
        success++
      }
    } catch (error) {
      console.error(`❌ Failed to pause container ${container.containerName}:`, error)
      failed++
    }
  }

  console.log(`✨ Container pause complete: ${success} succeeded, ${failed} failed`)
  
  return { success, failed }
}

/**
 * 停止所有运行中的容器
 */
export const stopAllContainers = async (timeout: number = 10): Promise<{ success: number; failed: number }> => {
  const containers = await dbGetAllContainers()
  let success = 0
  let failed = 0

  console.log(`⏹️  Stopping ${containers.length} containers...`)

  for (const container of containers) {
    try {
      const ops = useContainer(container.containerName, { namespace: container.namespace })
      
      // 获取当前状态
      const info = await ops.info()
      
      // 只停止运行中的容器
      if (info.status === 'running') {
        await ops.stop(timeout)
        await updateContainerStatus(container.containerId, 'stopped')
        console.log(`✅ Stopped container: ${container.containerName}`)
        success++
      } else {
        console.log(`⏭️  Container not running, skipped: ${container.containerName}`)
        success++
      }
    } catch (error) {
      console.error(`❌ Failed to stop container ${container.containerName}:`, error)
      failed++
    }
  }

  console.log(`✨ Container stop complete: ${success} succeeded, ${failed} failed`)
  
  return { success, failed }
}

/**
 * 确保用户有容器（没有则创建）
 */
export const ensureUserContainer = async (
  userId: string,
  image?: string,
  namespace?: string
): Promise<ContainerInfo> => {
  const existing = await getContainerByUserId(userId)
  
  if (existing) {
    return existing
  }

  // 创建新容器
  return await createUserContainer(userId, image, namespace)
}

/**
 * 同步所有容器状态
 */
export const syncAllContainerStatus = async (): Promise<void> => {
  const containers = await dbGetAllContainers()
  
  console.log(`🔄 Syncing ${containers.length} container statuses...`)

  for (const container of containers) {
    try {
      const ops = useContainer(container.containerName, { namespace: container.namespace })
      const info = await ops.info()
      
      if (info.status !== container.status) {
        await updateContainerStatus(container.containerId, info.status)
        console.log(`✅ Updated container ${container.containerName}: ${container.status} -> ${info.status}`)
      }
    } catch (error) {
      console.error(`❌ Failed to sync container ${container.containerName}:`, error)
      await updateContainerStatus(container.containerId, 'unknown')
    }
  }
  
  console.log('✨ Container status sync complete')
}

