import type { ApprovalProjection } from './api/kernelApi'

export function approvalSummary(approval: ApprovalProjection) {
  return [
    approvalStatusLabel(approval.status),
    operationTypeLabel(approval.effect?.tool),
    String(approval.effect?.command_preview ?? approval.blocked_reason ?? '').trim() || '等待确认',
  ]
}

function approvalStatusLabel(status: unknown) {
  const value = String(status ?? '').trim()
  if (value === 'pending') return '等待确认'
  if (value === 'approved') return '已允许'
  if (value === 'denied') return '已拒绝'
  return '需要确认'
}

function operationTypeLabel(tool: unknown) {
  const value = String(tool ?? '').trim()
  if (value === 'shell_exec') return '运行命令'
  if (value === 'job_cancel') return '取消任务'
  if (value === 'job_status') return '查看任务'
  if (value === 'source_tree') return '查看资料目录'
  if (value === 'source_read') return '读取资料'
  return '系统操作'
}
