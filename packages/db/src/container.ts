import { pgTable, text, timestamp, uuid, boolean } from 'drizzle-orm/pg-core'
import { users } from './users'

/**
 * 容器表 - 存储用户容器信息
 */
export const containers = pgTable('containers', {
  // 主键ID
  id: uuid('id').primaryKey().defaultRandom(),
  
  // 关联用户ID
  userId: uuid('user_id')
    .notNull()
    .references(() => users.id, { onDelete: 'cascade' }),
  
  // 容器ID（containerd 中的实际容器ID）
  containerId: text('container_id').notNull().unique(),
  
  // 容器名称
  containerName: text('container_name').notNull().unique(),
  
  // 容器镜像
  image: text('image').notNull(),
  
  // 容器状态：created, running, paused, stopped, unknown
  status: text('status').notNull().default('created'),
  
  // 容器命名空间
  namespace: text('namespace').notNull().default('default'),
  
  // 是否自动启动
  autoStart: boolean('auto_start').notNull().default(true),
  
  // 宿主机挂载目录
  hostPath: text('host_path'),
  
  // 容器内挂载目录
  containerPath: text('container_path').notNull().default('/data'),
  
  // 创建时间
  createdAt: timestamp('created_at').notNull().defaultNow(),
  
  // 更新时间
  updatedAt: timestamp('updated_at').notNull().defaultNow(),
  
  // 最后启动时间
  lastStartedAt: timestamp('last_started_at'),
  
  // 最后停止时间
  lastStoppedAt: timestamp('last_stopped_at'),
})

