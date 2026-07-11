export type TimelineRow = {
  id: string
  kind: 'user' | 'assistant' | 'reasoning' | 'processing' | 'action'
  text: string
  meta: string
  detailRef: string
  detailAvailable: boolean
  turnId: string
  terminalOutcome: string
  streaming?: boolean
}

type TimelineItemLike = {
  item_id?: unknown
  turn_id?: unknown
  kind?: unknown
  text?: unknown
  label?: unknown
  tool?: unknown
  tool_count?: unknown
  detail_ref?: unknown
  detail_available?: unknown
  children?: unknown
  terminal_outcome?: unknown
}

export function timelineRows(items: Array<Record<string, unknown>> | undefined): TimelineRow[] {
  const rows: TimelineRow[] = []
  for (const item of items ?? []) collectRows(item, rows)
  return rows
}

function collectRows(item: TimelineItemLike, rows: TimelineRow[]) {
  const kind = String(item.kind ?? '')
  if (kind === 'user_message') rows.push(row('user', item))
  if (kind === 'assistant_reasoning') rows.push(row('reasoning', item))
  if (kind === 'assistant_message') rows.push(row('assistant', item))
  if (kind === 'processing_group') rows.push(row('processing', item))
  if (kind === 'user_action_request') rows.push(row('action', item))
  if (Array.isArray(item.children)) {
    for (const child of item.children) {
      if (child && typeof child === 'object') collectRows(child as TimelineItemLike, rows)
    }
  }
}

function row(kind: TimelineRow['kind'], item: TimelineItemLike): TimelineRow {
  const detailRef = item.detail_available === true ? String(item.detail_ref ?? '').trim() : ''
  return {
    id: String(item.item_id ?? item.turn_id ?? `${kind}-${String(item.text ?? item.label ?? '').slice(0, 40)}`).trim(),
    kind,
    text: String(item.text ?? item.label ?? '').trim(),
    meta: rowMeta(kind, item),
    detailRef,
    detailAvailable: detailRef !== '',
    turnId: String(item.turn_id ?? '').trim(),
    terminalOutcome: String(item.terminal_outcome ?? '').trim(),
  }
}

function rowMeta(kind: TimelineRow['kind'], item: TimelineItemLike) {
  if (kind === 'processing') {
    const count = Number(item.tool_count ?? 0)
    return count > 0 ? `${count} 项操作` : ''
  }
  if (kind === 'action') return String(item.tool ?? '').trim() ? '需要确认' : ''
  if (kind === 'reasoning') return '已思考'
  return ''
}
