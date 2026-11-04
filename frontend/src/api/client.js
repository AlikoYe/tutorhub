// Fetch wrapper for the TutorHub API (docs/api.yaml).
import { tg } from '@/telegram'

export class ApiError extends Error {
  constructor(status, code, message) {
    super(message)
    this.status = status
    this.code = code
  }
}

function authHeader() {
  if (tg?.initData) return `tma ${tg.initData}`
  // Outside Telegram (local dev only): the API accepts "dev <id>" when APP_ENV=dev.
  const devId = localStorage.getItem('devTelegramId') || import.meta.env.VITE_DEV_TELEGRAM_ID
  return devId ? `dev ${devId}` : ''
}

export async function request(method, path, { body, query, form } = {}) {
  let url = `/api${path}`
  if (query) {
    const params = new URLSearchParams()
    for (const [k, v] of Object.entries(query)) {
      if (v === undefined || v === null || v === '') continue
      if (Array.isArray(v)) v.forEach((x) => params.append(k, x))
      else params.append(k, v)
    }
    const qs = params.toString()
    if (qs) url += `?${qs}`
  }
  const headers = {}
  const auth = authHeader()
  if (auth) headers.Authorization = auth
  let payload
  if (form) {
    payload = form
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }

  let res
  try {
    res = await fetch(url, { method, headers, body: payload })
  } catch {
    throw new ApiError(0, 'network', 'Нет соединения с сервером')
  }
  if (res.status === 204) return null
  const data = await res.json().catch(() => null)
  if (!res.ok) {
    throw new ApiError(res.status, data?.error || 'internal', data?.message || `HTTP ${res.status}`)
  }
  return data
}

// fetchBlob downloads a protected file (e.g. an admin-only document) as a Blob.
export async function fetchBlob(path) {
  const headers = {}
  const auth = authHeader()
  if (auth) headers.Authorization = auth
  const res = await fetch(`/api${path}`, { headers })
  if (!res.ok) {
    const data = await res.json().catch(() => null)
    throw new ApiError(res.status, data?.error || 'internal', data?.message || `HTTP ${res.status}`)
  }
  return res.blob()
}

export const api = {
  get: (path, query) => request('GET', path, { query }),
  post: (path, body) => request('POST', path, { body }),
  put: (path, body) => request('PUT', path, { body }),
  patch: (path, body) => request('PATCH', path, { body }),
  del: (path) => request('DELETE', path),
  upload: (path, file) => {
    const form = new FormData()
    form.append('file', file)
    return request('POST', path, { form })
  },
}
