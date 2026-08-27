const API_BASE = (import.meta.env.VITE_API_URL || '').replace(/\/$/, '')
const TOKEN_KEY = 'sheyanova_token'

export function apiUrl(path) {
  if (!path) return API_BASE || ''
  if (/^https?:\/\//i.test(path)) return path
  return `${API_BASE}${path.startsWith('/') ? path : `/${path}`}`
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token) {
  if (token) localStorage.setItem(TOKEN_KEY, token)
  else localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  constructor(message, { status = 0, body = null } = {}) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.body = body
  }
}

/**
 * Thin fetch helper with Bearer auth.
 * Returns parsed JSON when present; throws ApiError on HTTP/API failure.
 */
export async function api(path, options = {}) {
  const {
    method = 'GET',
    body,
    headers: extraHeaders = {},
    auth = true,
    formData = false,
  } = options

  const headers = { ...extraHeaders }
  if (auth) {
    const token = getToken()
    if (token) headers.Authorization = `Bearer ${token}`
  }

  let payload = body
  if (body != null && !formData && !(body instanceof FormData)) {
    headers['Content-Type'] = headers['Content-Type'] || 'application/json'
    payload = typeof body === 'string' ? body : JSON.stringify(body)
  }

  const res = await fetch(apiUrl(path), { method, headers, body: payload, credentials: 'include' })
  const text = await res.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { raw: text }
    }
  }

  if (!res.ok || data?.ok === false) {
    const msg =
      data?.error ||
      data?.message ||
      (res.status === 404 ? `Not found: ${path}` : `Request failed (${res.status})`)
    throw new ApiError(msg, { status: res.status, body: data })
  }

  return data
}

export const admin = {
  me: () => api('/api/admin/me'),
  session: {
    create: () => api('/api/admin/session', { method: 'POST' }),
    destroy: () => api('/api/admin/session', { method: 'DELETE', auth: false }),
  },
  settings: {
    get: () => api('/api/admin/settings'),
    put: (body) => api('/api/admin/settings', { method: 'PUT', body }),
  },
  nav: {
    get: () => api('/api/admin/nav'),
    put: (body) => api('/api/admin/nav', { method: 'PUT', body }),
    save: (nav) => api('/api/admin/nav', { method: 'PUT', body: { nav } }),
  },
  pages: {
    list: () => api('/api/admin/pages'),
    create: (body) => api('/api/admin/pages', { method: 'POST', body }),
    get: (id) => api(`/api/admin/pages/${id}`),
    patch: (id, body) => api(`/api/admin/pages/${id}`, { method: 'PATCH', body }),
    remove: (id) => api(`/api/admin/pages/${id}`, { method: 'DELETE' }),
    getBlocks: (id) => api(`/api/admin/pages/${id}/blocks`),
    putBlocks: (id, blocks) =>
      api(`/api/admin/pages/${id}/blocks`, { method: 'PUT', body: { blocks } }),
  },
  templates: {
    list: () => api('/api/admin/templates'),
    get: (id) => api(`/api/admin/templates/${id}`),
    create: (body) => api('/api/admin/templates', { method: 'POST', body }),
    patch: (id, body) => api(`/api/admin/templates/${id}`, { method: 'PATCH', body }),
    put: (id, body) => api(`/api/admin/templates/${id}`, { method: 'PUT', body }),
  },
  stats: () => api('/api/admin/stats'),
  media: {
    list: (params = {}) => {
      const q = new URLSearchParams()
      if (params.kind) q.set('kind', params.kind)
      if (params.q) q.set('q', params.q)
      if (params.ids != null && params.ids !== '') {
        const ids = Array.isArray(params.ids) ? params.ids.filter(Boolean).join(',') : String(params.ids)
        if (ids) q.set('ids', ids)
      }
      if (params.limit != null && params.limit !== '') q.set('limit', String(params.limit))
      const s = q.toString()
      return api(`/api/admin/media${s ? `?${s}` : ''}`)
    },
    get: (id) => api(`/api/admin/media/${id}`),
    upload: (formData) =>
      api('/api/admin/media', { method: 'POST', body: formData, formData: true }),
    patch: (id, body) => api(`/api/admin/media/${id}`, { method: 'PATCH', body }),
    remove: (id) => api(`/api/admin/media/${id}`, { method: 'DELETE' }),
  },
  generate: (body = {}) => api('/api/admin/generate', { method: 'POST', body }),
  preview: (pageId) => api(`/api/admin/preview/${pageId}`, { method: 'POST' }),
  publish: (body = {}) => api('/api/admin/publish', { method: 'POST', body }),
  publishJob: (id) => api(`/api/admin/publish/jobs/${id}`),
  publishHistory: () => api('/api/admin/publish/history'),
  /** Poll publish job until terminal status (ok|error|stub). */
  async waitPublishJob(jobId, { intervalMs = 1000, timeoutMs = 10 * 60 * 1000, onUpdate } = {}) {
    const started = Date.now()
    let last = null
    while (Date.now() - started < timeoutMs) {
      last = await admin.publishJob(jobId)
      if (typeof onUpdate === 'function') onUpdate(last)
      const st = String(last.status || '').toLowerCase()
      if (st && st !== 'queued' && st !== 'running') return last
      await new Promise((r) => setTimeout(r, intervalMs))
    }
    throw new ApiError('Publish timed out while waiting for job', { status: 408, body: last })
  },
  /** Import blocks/media from static front/. Pass force to replace existing blocks. */
  importFront: (force = false) =>
    api(`/api/admin/import-front${force ? '?force=1' : ''}`, { method: 'POST' }),
}
