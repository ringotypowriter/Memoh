import { describe, expect, test } from 'bun:test'
import { sseChunked } from '../utils/sse'

function parseChunkedSSE(payload: string): string {
  const lines = payload.split('\n')
  const dataLines = lines.filter(line => line.startsWith('data:'))
  return dataLines.map(line => line.slice('data:'.length)).join('')
}

describe('sseChunked', () => {
  test('reconstructs original payload losslessly', () => {
    const input = JSON.stringify({
      type: 'tool_call_end',
      toolName: 'big_tool',
      toolCallId: 'call-1',
      // include whitespace and unicode so trimming/surrogate splitting bugs show up
      result: '  leading spaces\tand tabs\nand unicode 😀😃😄  ',
      blob: 'x'.repeat(200_000),
    })

    const chunked = sseChunked(input, 1024).toSSE()
    const reconstructed = parseChunkedSSE(chunked)

    expect(reconstructed).toBe(input)
  })

  test('chunkSize=1 does not produce invalid UTF-8 (surrogate pairs)', () => {
    const input = `😀${'x'.repeat(1000)}😃`
    const payload = sseChunked(input, 1).toSSE()

    // Simulate the UTF-8 encode/decode step that happens over the network.
    const encoded = new TextEncoder().encode(payload)
    const decoded = new TextDecoder().decode(encoded)
    expect(decoded).toBe(payload)

    const reconstructed = parseChunkedSSE(decoded)
    expect(reconstructed).toBe(input)
  })

  test('does not inject an extra space after data:', () => {
    const input = ' abc'
    const chunked = sseChunked(input, 2).toSSE()
    expect(chunked.split('\n')[0]).toBe('data: a')
  })
})
