import type { TimelineRow } from './timelineView'

export type WorkspaceActivityRow = TimelineRow & {
  presentation: 'brief' | 'thinking' | 'work' | 'decision' | 'output'
  label: string
}

export function workspaceActivity(rows: TimelineRow[]): WorkspaceActivityRow[] {
  return rows.map((row) => ({
    ...row,
    presentation: presentationFor(row),
    label: labelFor(row),
  }))
}

function presentationFor(row: TimelineRow): WorkspaceActivityRow['presentation'] {
  if (row.kind === 'user') return 'brief'
  if (row.kind === 'reasoning') return 'thinking'
  if (row.kind === 'processing') return 'work'
  if (row.kind === 'action') return 'decision'
  return 'output'
}

function labelFor(row: TimelineRow): string {
  if (row.kind === 'reasoning') return 'Thinking'
  if (row.kind === 'action') return 'Needs your decision'
  if (row.kind === 'processing') return row.terminalOutcome === 'succeeded' ? '已完成' : (row.text || '正在处理')
  if (row.kind === 'assistant') return 'Result'
  return 'Task'
}
