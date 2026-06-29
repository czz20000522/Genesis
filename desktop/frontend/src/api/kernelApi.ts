const baseUrlKey = 'genesis.desktop.kernel_base_url'
const runtimeTokenKey = 'genesis.desktop.runtime_token'

export type KernelConfig = {
  baseUrl: string
  runtimeToken: string
}

export type KernelTimeline = {
  session_id?: string
  readiness?: string
  items?: Array<Record<string, unknown>>
}

export type KernelTimelineDetail = {
  detail_ref?: string
  readiness?: string
  item?: Record<string, unknown>
}

export type MaterialIntakeProjection = {
  admission_result?: string
  refusal_reason_class?: string
  source_snapshot_ref?: string
  available_operations?: string[]
  diagnostics?: Array<Record<string, unknown>>
}

export type TurnResponse = {
  session_id?: string
  turn_id?: string
  final?: {
    text?: string
    model?: string
  }
  pause?: Record<string, unknown>
  error?: {
    code?: string
    message?: string
  }
}

export type ApprovalProjection = {
  approval_id?: string
  status?: string
  effect?: {
    tool?: string
    command_preview?: string
    cwd?: string
  }
  blocked_reason?: string
}

export type ApprovalListResponse = {
  items?: ApprovalProjection[]
}

export type ApprovalDecision = 'approved' | 'denied'

export type SessionDebugExport = Record<string, unknown>

export function kernelConfig(storage: Pick<Storage, 'getItem'> | null = safeLocalStorage()): KernelConfig {
  return {
    baseUrl: String(storage?.getItem(baseUrlKey) ?? 'http://127.0.0.1:8765').trim(),
    runtimeToken: String(storage?.getItem(runtimeTokenKey) ?? '').trim(),
  }
}

export function saveKernelConfig(config: KernelConfig, storage: Pick<Storage, 'setItem'> | null = safeLocalStorage()) {
  storage?.setItem(baseUrlKey, normalizedBaseUrl(config.baseUrl))
  storage?.setItem(runtimeTokenKey, String(config.runtimeToken || '').trim())
}

export async function getReady(config = kernelConfig()) {
  return requestKernel(config, '/ready')
}

export async function getCapabilities(config = kernelConfig()) {
  return requestKernel(config, '/capabilities')
}

export async function getTimeline(config: KernelConfig, sessionId: string) {
  return requestKernel<KernelTimeline>(config, `/sessions/${encodeURIComponent(sessionId)}/timeline`)
}

export async function getTimelineDetail(config: KernelConfig, sessionId: string, detailRef: string) {
  return requestKernel<KernelTimelineDetail>(
    config,
    `/sessions/${encodeURIComponent(sessionId)}/timeline/details/${encodeURIComponent(detailRef)}`,
  )
}

export async function uploadMaterial(config: KernelConfig, sessionId: string, file: File) {
  const form = new FormData()
  form.set('session_id', sessionId)
  form.set('purpose', 'source_analysis')
  form.set('file', file)
  return requestKernel<MaterialIntakeProjection>(config, '/materials/upload', {
    method: 'POST',
    body: form,
  })
}

export async function submitTurn(config: KernelConfig, sessionId: string, text: string, idempotencyKey: string) {
  return requestKernel<TurnResponse>(config, '/turn', {
    method: 'POST',
    body: JSON.stringify({
      session_id: sessionId,
      idempotency_key: idempotencyKey,
      input_items: [{ type: 'text', text }],
    }),
  })
}

export async function listApprovals(config: KernelConfig, status = 'pending') {
  return requestKernel<ApprovalListResponse>(config, `/approvals?status=${encodeURIComponent(status)}`)
}

export async function decideApproval(config: KernelConfig, approvalId: string, decision: ApprovalDecision, reason: string) {
  return requestKernel<ApprovalProjection>(config, `/approvals/${encodeURIComponent(approvalId)}/decision`, {
    method: 'POST',
    body: JSON.stringify({
      decision,
      decision_authority: 'desktop:operator',
      decision_reason: String(reason || decision).trim(),
      decision_evidence_ref: 'approval:desktop-operator',
    }),
  })
}

export async function enableSessionDebug(config: KernelConfig, sessionId: string) {
  return requestKernel<SessionDebugExport>(config, `/sessions/${encodeURIComponent(sessionId)}/debug/enable`, {
    method: 'POST',
    body: JSON.stringify({}),
  })
}

export async function getSessionDebug(config: KernelConfig, sessionId: string) {
  return requestKernel<SessionDebugExport>(config, `/sessions/${encodeURIComponent(sessionId)}/debug`)
}

export async function requestKernel<T = Record<string, unknown>>(config: KernelConfig, path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(kernelUrl(config.baseUrl, path), {
    ...init,
    headers: mergeHeaders(kernelHeaders(config.runtimeToken, init.body), init.headers),
  })
  if (!response.ok) {
    throw new Error(await responseMessage(response))
  }
  return (await response.json()) as T
}

export function kernelUrl(baseUrl: string, path: string) {
  const base = normalizedBaseUrl(baseUrl)
  return `${base}/${path.replace(/^\/+/, '')}`
}

function normalizedBaseUrl(baseUrl: string) {
  return String(baseUrl || 'http://127.0.0.1:8765').trim().replace(/\/+$/, '')
}

function kernelHeaders(token: string, body?: BodyInit | null) {
  const headers = new Headers()
  const trimmed = String(token || '').trim()
  if (trimmed) headers.set('Authorization', `Bearer ${trimmed}`)
  if (typeof body === 'string') headers.set('Content-Type', 'application/json')
  return headers
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
