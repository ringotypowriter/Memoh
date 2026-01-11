import { boolean, jsonb, pgTable, text, timestamp, uuid } from 'drizzle-orm/pg-core'

export const platform = pgTable('platform', {
  id: uuid('id').primaryKey().defaultRandom(),
  name: text('name').notNull(),
  endpoint: text('endpoint').notNull(),
  config: jsonb('config').notNull(),
  active: boolean('active').notNull().default(true),
  createdAt: timestamp('created_at').notNull().defaultNow(),
  updatedAt: timestamp('updated_at').notNull().defaultNow(),
})