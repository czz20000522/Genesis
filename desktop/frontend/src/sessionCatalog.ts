const sessionCatalogKey = 'genesis.desktop.session_catalog'

export type DesktopSessionKind = 'project' | 'task' | 'chat'

export type DesktopSessionCatalogEntry = {
  sessionId: string
  kind: DesktopSessionKind
  root?: string
  name?: string
}

type CatalogStorage = Pick<Storage, 'getItem' | 'setItem'>

export function loadSessionCatalog(storage: Pick<Storage, 'getItem'> | null = safeLocalStorage()): DesktopSessionCatalogEntry[] {
  const raw = storage?.getItem(sessionCatalogKey)
  if (!raw) return []
  try {
    const entries = JSON.parse(raw)
    if (!Array.isArray(entries)) return []
    return entries.flatMap(normalizeCatalogEntry)
  } catch {
    return []
  }
}

export function recordSessionCatalogEntry(entry: DesktopSessionCatalogEntry, storage: CatalogStorage | null = safeLocalStorage()) {
  if (!storage) return
  const normalized = normalizeCatalogEntry(entry)[0]
  if (!normalized) return
  const existing = loadSessionCatalog(storage)
  const next = [...existing.filter((item) => item.sessionId !== normalized.sessionId), normalized]
  storage.setItem(sessionCatalogKey, JSON.stringify(next))
}

function normalizeCatalogEntry(value: unknown): DesktopSessionCatalogEntry[] {
  if (!value || typeof value !== 'object') return []
  const item = value as Partial<DesktopSessionCatalogEntry>
  const sessionId = String(item.sessionId || '').trim()
  const kind = String(item.kind || '').trim()
  if (!sessionId || (kind !== 'project' && kind !== 'task' && kind !== 'chat')) return []
  const root = kind === 'chat' ? '' : String(item.root || '').trim()
  if (kind !== 'chat' && !root) return []
  return [{
    sessionId,
    kind,
    root,
    name: String(item.name || '').trim(),
  }]
}

function safeLocalStorage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}
