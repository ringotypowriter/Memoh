import { db } from './index'
import { containers } from './container'
import { eq } from 'drizzle-orm'

/**
 * 容器信息类型
 */
export type ContainerInfo = {
  id: string
  userId: string
  containerId: string
  containerName: string
  image: string
  status: string
  namespace: string
  autoStart: boolean
  hostPath: string | null
  containerPath: string
  createdAt: Date
  updatedAt: Date
  lastStartedAt: Date | null
  lastStoppedAt: Date | null
}

/**
 * 创建容器输入类型
 */
export type CreateContainerInput = {
  userId: string
  containerId: string
  containerName: string
  image: string
  namespace?: string
  autoStart?: boolean
  hostPath?: string
  containerPath?: string
}

/**
 * 更新容器输入类型
 */
export type UpdateContainerInput = {
  status?: string
  autoStart?: boolean
  lastStartedAt?: Date
  lastStoppedAt?: Date
}

/**
 * 获取所有容器
 */
export const getAllContainers = async (): Promise<ContainerInfo[]> => {
  const containerList = await db.select().from(containers)
  return containerList
}

/**
 * 获取所有自动启动的容器
 */
export const getAutoStartContainers = async (): Promise<ContainerInfo[]> => {
  const containerList = await db
    .select()
    .from(containers)
    .where(eq(containers.autoStart, true))
  return containerList
}

/**
 * 根据用户ID获取容器
 */
export const getContainerByUserId = async (userId: string): Promise<ContainerInfo | undefined> => {
  const [container] = await db
    .select()
    .from(containers)
    .where(eq(containers.userId, userId))
  
  return container
}

/**
 * 根据容器名称获取容器
 */
export const getContainerByName = async (containerName: string): Promise<ContainerInfo | undefined> => {
  const [container] = await db
    .select()
    .from(containers)
    .where(eq(containers.containerName, containerName))
  
  return container
}

/**
 * 根据容器ID获取容器
 */
export const getContainerById = async (id: string): Promise<ContainerInfo | undefined> => {
  const [container] = await db
    .select()
    .from(containers)
    .where(eq(containers.id, id))
  
  return container
}

/**
 * 创建容器记录
 */
export const createContainerRecord = async (data: CreateContainerInput): Promise<ContainerInfo> => {
  // 检查用户是否已有容器
  const existing = await getContainerByUserId(data.userId)
  if (existing) {
    throw new Error('User already has a container')
  }

  const [newContainer] = await db
    .insert(containers)
    .values({
      userId: data.userId,
      containerId: data.containerId,
      containerName: data.containerName,
      image: data.image,
      namespace: data.namespace || 'default',
      autoStart: data.autoStart ?? true,
      hostPath: data.hostPath || null,
      containerPath: data.containerPath || '/data',
      status: 'created',
    })
    .returning()

  return newContainer
}

/**
 * 更新容器状态
 */
export const updateContainerStatus = async (
  containerId: string,
  status: string
): Promise<ContainerInfo | null> => {
  const [updated] = await db
    .update(containers)
    .set({
      status,
      updatedAt: new Date(),
      ...(status === 'running' ? { lastStartedAt: new Date() } : {}),
      ...(status === 'stopped' || status === 'paused' ? { lastStoppedAt: new Date() } : {}),
    })
    .where(eq(containers.containerId, containerId))
    .returning()

  return updated || null
}

/**
 * 更新容器信息
 */
export const updateContainer = async (
  id: string,
  data: UpdateContainerInput
): Promise<ContainerInfo | null> => {
  const [updated] = await db
    .update(containers)
    .set({
      ...data,
      updatedAt: new Date(),
    })
    .where(eq(containers.id, id))
    .returning()

  return updated || null
}

/**
 * 删除容器记录
 */
export const deleteContainerRecord = async (id: string): Promise<ContainerInfo | null> => {
  const [deleted] = await db
    .delete(containers)
    .where(eq(containers.id, id))
    .returning()

  return deleted || null
}

/**
 * 根据用户ID删除容器记录
 */
export const deleteContainerByUserId = async (userId: string): Promise<ContainerInfo | null> => {
  const [deleted] = await db
    .delete(containers)
    .where(eq(containers.userId, userId))
    .returning()

  return deleted || null
}

/**
 * 检查用户是否有容器
 */
export const userHasContainer = async (userId: string): Promise<boolean> => {
  const container = await getContainerByUserId(userId)
  return !!container
}

