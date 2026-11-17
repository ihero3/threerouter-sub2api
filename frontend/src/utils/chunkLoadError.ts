/**
 * Detect "stale chunk" errors.
 *
 * After a deployment replaces the hashed assets, a tab that is still running an
 * older build can request a chunk that no longer exists. Browsers and servers
 * report this in different ways:
 * - 404 on the chunk => "Failed to fetch dynamically imported module"
 * - SPA fallback serving index.html => "text/html is not a valid JavaScript MIME type"
 *   or "Expected a JavaScript module script but the server responded with ..."
 * - webpack-style loaders => "Loading chunk ... failed"
 */
const CHUNK_LOAD_ERROR_PATTERNS = [
  'failed to fetch dynamically imported module',
  'loading chunk',
  'loading css chunk',
  'is not a valid javascript mime type',
  'expected a javascript module script',
  'importing a module script failed',
]

export function isChunkLoadError(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false

  const err = error as { name?: string; message?: string }
  if (err.name === 'ChunkLoadError') return true

  const message = (err.message || '').toLowerCase()
  return CHUNK_LOAD_ERROR_PATTERNS.some((pattern) => message.includes(pattern))
}
