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
  return sessionId === currentSessionId ? '正在使用' : '本地会话'
}

export function connectionErrorLabel(error: string) {
  return String(error || '').trim() ? '连接失败，请检查本地服务' : ''
}
