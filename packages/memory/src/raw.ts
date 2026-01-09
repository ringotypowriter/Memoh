import { ModelMessage } from 'ai'
import { MemoryUnit } from './memory-unit'

export const rawMessages = (messages: ModelMessage[]) => {
  return messages.map((message) => {
    if (message.role === 'user') {
      if (Array.isArray(message.content)) {
        return `User: ${message.content.filter(c => c.type === 'text').map(c => c.text).join('\n')}`
      }
      return `User: ${message.content}`
    } else if (message.role === 'assistant') {
      let toolCalls = ''
      if (Array.isArray(message.content)) {
        toolCalls = message.content.filter(c => c.type === 'tool-call').map(c => c.toolName).join(', ')
      }
      return `You: ${message.content} \n${toolCalls}`
    } else if (message.role === 'tool') {
      return `Tool Result: ${message.content}`
    } else {
      return null
    }
  })
  .filter((message) => message !== null)
  .join('\n\n')
}

export const rawMemory = (memory: MemoryUnit, locale?: Intl.LocalesArgument) => {
  return `
  ---
  date: ${memory.timestamp.toLocaleDateString(locale)}
  time: ${memory.timestamp.toLocaleTimeString(locale)}
  timezone: ${memory.timestamp.getTimezoneOffset()}
  ---
  ${rawMessages(memory.messages)}
  `.trim()
}
