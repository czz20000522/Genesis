export function readinessLabel(readiness: string) {
  const value = String(readiness || '').trim().toLowerCase()
  if (value === 'ready' || value === 'serving-ready' || value === 'ok') return '已连接'
  if (value === 'not_ready' || value === 'failed' || value === 'error') return '连接失败'
  if (value === 'checking') return '检查中'
  return '未连接'
}

export function sessionLabel(sessionId: string) {
  return String(sessionId || '').trim() ? '当前会话' : '未选择会话'
}

export function sessionStatus(sessionId: string, currentSessionId: string) {
  return sessionId === currentSessionId ? '正在使用' : '未打开'
}

export function connectionErrorLabel(error: string) {
  return String(error || '').trim() ? '连接失败，请检查本地服务' : ''
}

export function turnErrorLabel(error: unknown) {
  const detail = String(error instanceof Error ? error.message : error || '').trim().toLowerCase()
  if (!detail) return '未能完成这次对话，请重试。'
  if (detail.includes('provider_profile_missing') || detail.includes('model profile missing')) return '请先选择一个模型，然后再发送消息。'
  if (detail.includes('llama.cpp') || detail.includes('connection refused') || detail.includes('winerror 10061')) return '本地模型尚未启动。请在“模型”中启动它，或改用云端模型。'
  if (detail.includes('credential') || detail.includes('api key') || detail.includes('unauthorized') || detail.includes('forbidden') || detail.includes('401') || detail.includes('403')) return '模型凭据不可用。请在“模型”中检查 API Key。'
  if (detail.includes('failed to fetch') || detail.includes('network') || detail.includes('connection reset')) return '无法连接 Genesis 本地服务，请稍后重试。'
  if (detail.includes('provider') || detail.includes('model')) return '模型服务暂时无法完成此请求。请稍后重试或切换模型。'
  return '未能完成这次对话，请重试。'
}
