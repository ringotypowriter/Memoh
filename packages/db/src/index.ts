import { drizzle } from 'drizzle-orm/node-postgres'
import { config } from 'dotenv'

config({ path: '../../' })

export const db = drizzle(process.env.DATABASE_URL!)

// Export helpers
export * from './user-helpers'
export * from './container-helpers'
