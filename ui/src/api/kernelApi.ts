const runtimeTokenKey = 'genesis.kernel.runtime_token'

export type KernelTimeline = {
  status?: string
  readiness?: string
  items?: Array<Record<string, unknown>>
}

export function runtimeTokenFromStorage(storage: Pick<Storage, 'getItem'> | null = safeLocalStorage()) {
  return String(storage?.getItem(runtimeTokenKey) ?? '').trim()
}

export function saveRuntimeToken(token: string, storage: Pick<Storage, 'setItem'> | null = safeLocalStorage()) {
  storage?.setItem(runtimeTokenKey, String(token || '').trim())
}

export function runtimeHeaders(token = runtimeTokenFromStorage(), body?: BodyInit | null) {
  const headers = new Headers()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  if (typeof body === 'string') headers.set('Content-Type', 'application/json')
  return headers
}

export async function requestJson<T>(path: string, init: RequestInit = {}): Promise<T> {
  const body = init.body ?? null
  const response = await fetch(path, {
    ...init,
    headers: mergeHeaders(runtimeHeaders(undefined, body), init.headers),
  })
  if (!response.ok) {
    throw new Error(await responseMessage(response))
  }
  return (await response.json()) as T
}

export function getReady() {
  return requestJson<Record<string, unknown>>('/ready')
}

export function getCapabilities() {
  return requestJson<Record<string, unknown>>('/capabilities')
}

export function submitTurn(sessionId: string, text: string) {
  return requestJson<Record<string, unknown>>('/turn', {
    method: 'POST',
    body: JSON.stringify({
      session_id: sessionId,
      input_items: [{ type: 'text', text }],
    }),
  })
}

export function getTimeline(sessionId: string) {
  return requestJson<KernelTimeline>(`/sessions/${encodeURIComponent(sessionId)}/timeline`)
}

export function uploadMaterial(sessionId: string, file: File) {
  const form = new FormData()
  form.set('session_id', sessionId)
  form.set('purpose', 'webui.material_upload')
  form.set('file', file)
  return requestJson<Record<string, unknown>>('/materials/upload', {
    method: 'POST',
    body: form,
  })
}

export function enableSessionDebug(sessionId: string) {
  return requestJson<Record<string, unknown>>(`/sessions/${encodeURIComponent(sessionId)}/debug/enable`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export function exportSessionDebug(sessionId: string) {
  return requestJson<Record<string, unknown>>(`/sessions/${encodeURIComponent(sessionId)}/debug`)
}

export function compactSessionContext(sessionId: string) {
  return requestJson<Record<string, unknown>>(`/sessions/${encodeURIComponent(sessionId)}/context/compact`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

function mergeHeaders(base: Headers, extra: HeadersInit | undefined) {
  const merged = new Headers(base)
  if (!extra) return merged
  new Headers(extra).forEach((value, key) => merged.set(key, value))
  return merged
}

async function responseMessage(response: Response) {
  try {
    const payload = await response.json() as { error?: { code?: string; message?: string } }
    const code = String(payload.error?.code ?? '').trim()
    const message = String(payload.error?.message ?? '').trim()
    return [code, message].filter(Boolean).join(': ') || `HTTP ${response.status}`
  } catch {
    return `HTTP ${response.status}`
  }
}

function safeLocalStorage() {
  try {
    return typeof window === 'undefined' ? null : window.localStorage
  } catch {
    return null
  }
}
