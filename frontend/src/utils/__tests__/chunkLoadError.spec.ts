import { describe, expect, it } from 'vitest'
import { isChunkLoadError } from '@/utils/chunkLoadError'

describe('isChunkLoadError', () => {
  it('detects dynamic import failures caused by a 404 chunk', () => {
    expect(
      isChunkLoadError(new Error('Failed to fetch dynamically imported module: /assets/DashboardView-x.js')),
    ).toBe(true)
  })

  it('detects the text/html MIME error produced by SPA fallback for missing assets', () => {
    expect(isChunkLoadError(new Error("'text/html' is not a valid JavaScript MIME type"))).toBe(true)
    expect(
      isChunkLoadError(
        new Error(
          "Failed to load module script: Expected a JavaScript module script but the server responded with a MIME type of 'text/html'.",
        ),
      ),
    ).toBe(true)
  })

  it('detects Safari module import failures and webpack chunk errors', () => {
    expect(isChunkLoadError(new TypeError('Importing a module script failed.'))).toBe(true)
    expect(isChunkLoadError(new Error('Loading CSS chunk 42 failed.'))).toBe(true)
    const chunkError = new Error('boom')
    chunkError.name = 'ChunkLoadError'
    expect(isChunkLoadError(chunkError)).toBe(true)
  })

  it('ignores unrelated errors', () => {
    expect(isChunkLoadError(new Error('Invalid credentials'))).toBe(false)
    expect(isChunkLoadError({ status: 401, message: 'Unauthorized' })).toBe(false)
    expect(isChunkLoadError(null)).toBe(false)
    expect(isChunkLoadError('text/html')).toBe(false)
  })
})
