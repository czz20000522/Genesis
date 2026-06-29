export type TimelineDetailEntry = {
  detailRef: string
  label: string
}

type TimelineItemLike = {
  item_id?: unknown
  kind?: unknown
  detail_ref?: unknown
  detail_available?: unknown
  children?: unknown
}

export function timelineDetailEntries(items: Array<Record<string, unknown>> | undefined): TimelineDetailEntry[] {
  const out: TimelineDetailEntry[] = []
  for (const item of items ?? []) collectDetailEntries(item, out)
  return out
}

function collectDetailEntries(item: TimelineItemLike, out: TimelineDetailEntry[]) {
  const kind = String(item.kind ?? 'item')
  const itemID = String(item.item_id ?? '').trim()
  const detailRef = String(item.detail_ref ?? itemID).trim()
  if (item.detail_available === true && detailRef) {
    out.push({ detailRef, label: `${kind}: ${detailRef}` })
  }
  if (Array.isArray(item.children)) {
    for (const child of item.children) {
      if (child && typeof child === 'object') collectDetailEntries(child as TimelineItemLike, out)
    }
  }
}
