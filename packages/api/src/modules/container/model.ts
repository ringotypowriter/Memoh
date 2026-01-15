import { z } from 'zod'

/**
 * 创建容器请求模型
 */
export const CreateContainerSchema = z.object({
  userId: z.string(),
  image: z.string().optional().default('docker.io/library/alpine:latest'),
  namespace: z.string().optional().default('default'),
  autoStart: z.boolean().optional().default(true),
})

export type CreateContainerInput = z.infer<typeof CreateContainerSchema>

/**
 * 更新容器请求模型
 */
export const UpdateContainerSchema = z.object({
  autoStart: z.boolean().optional(),
})

export type UpdateContainerInput = z.infer<typeof UpdateContainerSchema>

/**
 * 容器操作请求模型
 */
export const ContainerActionSchema = z.object({
  action: z.enum(['start', 'stop', 'restart', 'pause', 'resume']),
})

export type ContainerActionInput = z.infer<typeof ContainerActionSchema>

/**
 * 确保容器请求模型
 */
export const EnsureContainerSchema = z.object({
  image: z.string().optional(),
  namespace: z.string().optional(),
})

export type EnsureContainerInput = z.infer<typeof EnsureContainerSchema>

/**
 * 容器响应模型
 */
export const ContainerResponseSchema = z.object({
  id: z.string().uuid(),
  userId: z.string().uuid(),
  containerId: z.string(),
  containerName: z.string(),
  image: z.string(),
  status: z.string(),
  namespace: z.string(),
  autoStart: z.boolean(),
  createdAt: z.date(),
  updatedAt: z.date(),
  lastStartedAt: z.date().nullable().optional(),
  lastStoppedAt: z.date().nullable().optional(),
})

export type ContainerResponse = z.infer<typeof ContainerResponseSchema>

