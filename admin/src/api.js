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

  const res = await fetch(apiUrl(path), { method, headers, body: payload })
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
  settings: {
    get: () => api('/api/admin/settings'),
    put: (body) => api('/api/admin/settings', { method: 'PUT', body }),
  },
  nav: {
    get: () => api('/api/admin/nav'),
    put: (body) => api('/api/admin/nav', { method: 'PUT', body }),
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
  media: {
    list: (params = {}) => {
      const q = new URLSearchParams()
      if (params.kind) q.set('kind', params.kind)
      if (params.q) q.set('q', params.q)
      const s = q.toString()
      return api(`/api/admin/media${s ? `?${s}` : ''}`)
    },
    upload: (formData) =>
      api('/api/admin/media', { method: 'POST', body: formData, formData: true }),
    patch: (id, body) => api(`/api/admin/media/${id}`, { method: 'PATCH', body }),
    remove: (id) => api(`/api/admin/media/${id}`, { method: 'DELETE' }),
  },
  generate: (body = {}) => api('/api/admin/generate', { method: 'POST', body }),
  preview: (pageId) => api(`/api/admin/preview/${pageId}`, { method: 'POST' }),
  publish: (body = {}) => api('/api/admin/publish', { method: 'POST', body }),
  publishHistory: () => api('/api/admin/publish/history'),
  /** Import blocks/media from static front/. Pass force to replace existing blocks. */
  importFront: (force = false) =>
    api(`/api/admin/import-front${force ? '?force=1' : ''}`, { method: 'POST' }),
}
