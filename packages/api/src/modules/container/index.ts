import { Elysia } from 'elysia'
import { adminMiddleware } from '../../middlewares'
import {
  createUserContainer,
  startUserContainer,
  stopUserContainer,
  restartUserContainer,
  pauseUserContainer,
  resumeUserContainer,
  deleteUserContainer,
  getUserContainerInfo,
  ensureUserContainer,
  syncAllContainerStatus,
  getAllContainers,
  startAllAutoStartContainers,
  pauseAllContainers,
} from './service'
import {
  CreateContainerSchema,
  ContainerActionSchema,
  EnsureContainerSchema,
} from './model'
import { getUsers } from '../user/service'

/**
 * Container Management Module
 * All routes require admin privileges
 */
export const containerModule = new Elysia({ prefix: '/containers' })
  // Protect all routes with admin middleware
  .use(adminMiddleware)
  .onStart(async () => {
    console.log('\n📦 Initializing containers...')
    
    try {
      // 0. 初始化容器基础目录
      const { initializeContainerBaseDirectory } = await import('./utils')
      initializeContainerBaseDirectory()
      
      // 1. 同步所有容器状态
      await syncAllContainerStatus()
      
      // 2. 检查所有用户是否有容器，没有则创建
      const usersResult = await getUsers({ page: 1, limit: 1000 })
      console.log(`👥 Found ${usersResult.items.length} users`)
      
      for (const user of usersResult.items) {
        try {
          await ensureUserContainer(user.id)
          console.log(`✅ Container ensured for user: ${user.username}`)
        } catch (error) {
          console.error(`❌ Failed to ensure container for user ${user.username}:`, error)
        }
      }
      
      // 3. 启动所有自动启动的容器
      await startAllAutoStartContainers()
      
      console.log('✨ Container initialization complete\n')
    } catch (error) {
      console.error('❌ Container initialization failed:', error)
    }
  })
  .onStop(async () => {
    console.log('\n⏸️  Pausing all containers...')
    
    try {
      await pauseAllContainers()
      console.log('✨ All containers paused\n')
    } catch (error) {
      console.error('❌ Failed to pause containers:', error)
    }
  })
  .get(
    '/',
    async () => {
      const containers = await getAllContainers()
      return {
        success: true,
        data: containers,
      }
    },
    {
      detail: {
        tags: ['Container'],
        summary: 'Get all containers',
        description: 'Retrieve information about all containers in the system',
      },
    }
  )
  .get(
    '/user/:userId',
    async ({ params: { userId } }) => {
      const container = await getUserContainerInfo(userId)
      if (!container) {
        return {
          success: false,
          error: 'Container not found for user',
        }
      }
      return {
        success: true,
        data: container,
      }
    },
    {
      detail: {
        tags: ['Container'],
        summary: 'Get user container',
        description: 'Get container information for a specific user',
      },
    }
  )
  .post(
    '/create',
    async ({ body }) => {
      try {
        const container = await createUserContainer(
          body.userId,
          body.image,
          body.namespace
        )
        return {
          success: true,
          data: container,
        }
      } catch (error) {
        return {
          success: false,
          error: error instanceof Error ? error.message : 'Failed to create container',
        }
      }
    },
    {
      body: CreateContainerSchema,
      detail: {
        tags: ['Container'],
        summary: 'Create user container',
        description: 'Create a new container for a specified user',
      },
    }
  )
  .post(
    '/user/:userId/ensure',
    async ({ params: { userId }, body }) => {
      try {
        const container = await ensureUserContainer(
          userId,
          body?.image,
          body?.namespace
        )
        return {
          success: true,
          data: container,
        }
      } catch (error) {
        return {
          success: false,
          error: error instanceof Error ? error.message : 'Failed to ensure container',
        }
      }
    },
    {
      body: EnsureContainerSchema,
      detail: {
        tags: ['Container'],
        summary: 'Ensure user has container',
        description: 'Check if user has a container, create one if not exists',
      },
    }
  )
  .post(
    '/user/:userId/action',
    async ({ params: { userId }, body }) => {
      try {
        switch (body.action) {
          case 'start':
            await startUserContainer(userId)
            break
          case 'stop':
            await stopUserContainer(userId)
            break
          case 'restart':
            await restartUserContainer(userId)
            break
          case 'pause':
            await pauseUserContainer(userId)
            break
          case 'resume':
            await resumeUserContainer(userId)
            break
          default:
            return {
              success: false,
              error: 'Invalid action',
            }
        }
        
        return {
          success: true,
          message: `Container ${body.action} successful`,
        }
      } catch (error) {
        return {
          success: false,
          error: error instanceof Error ? error.message : `Failed to ${body.action} container`,
        }
      }
    },
    {
      body: ContainerActionSchema,
      detail: {
        tags: ['Container'],
        summary: 'Execute container action',
        description: 'Perform start, stop, restart, pause, or resume actions on a user container',
      },
    }
  )
  .delete(
    '/user/:userId',
    async ({ params: { userId }, query }) => {
      try {
        const force = query.force === 'true'
        await deleteUserContainer(userId, force)
        return {
          success: true,
          message: 'Container deleted successfully',
        }
      } catch (error) {
        return {
          success: false,
          error: error instanceof Error ? error.message : 'Failed to delete container',
        }
      }
    },
    {
      detail: {
        tags: ['Container'],
        summary: 'Delete user container',
        description: 'Delete the container for a specified user',
      },
    }
  )
  .post(
    '/sync',
    async () => {
      try {
        await syncAllContainerStatus()
        return {
          success: true,
          message: 'Container status synced successfully',
        }
      } catch (error) {
        return {
          success: false,
          error: error instanceof Error ? error.message : 'Failed to sync container status',
        }
      }
    },
    {
      detail: {
        tags: ['Container'],
        summary: 'Sync all container statuses',
        description: 'Synchronize all container statuses from containerd to the database',
      },
    }
  )

