import { z } from 'zod'

const PlatformSchema = z.object({
  name: z.string().min(1, 'Platform name is required'),
  endpoint: z.string().min(1, 'Endpoint is required'),
  config: z.record(z.string(), z.unknown()),
  active: z.boolean().optional().default(true),
})

export type PlatformInput = z.infer<typeof PlatformSchema>

export const CreatePlatformModel = {
  body: PlatformSchema,
}

export const UpdatePlatformModel = {
  params: z.object({
    id: z.string(),
  }),
  body: PlatformSchema,
}

export const GetPlatformByIdModel = {
  params: z.object({
    id: z.string(),
  }),
}

export const DeletePlatformModel = {
  params: z.object({
    id: z.string(),
  }),
}

export const UpdatePlatformConfigModel = {
  params: z.object({
    id: z.string(),
  }),
  body: z.object({
    config: z.record(z.string(), z.unknown()),
  }),
}

export const SetPlatformActiveModel = {
  params: z.object({
    id: z.string(),
  }),
}

