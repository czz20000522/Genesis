import type { SessionListItem } from './api/kernelApi'

const sessionCatalogKey = 'genesis.desktop.session_catalog'
const projectCatalogKey = 'genesis.desktop.project_catalog'

export type DesktopSessionKind = 'project' | 'task' | 'chat'

export type DesktopProjectCatalogEntry = {
  projectId: string
  name: string
  root: string
}

export type DesktopSessionCatalogEntry = {
  sessionId: string
  kind: DesktopSessionKind
  projectId?: string
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

export function loadProjectCatalog(storage: Pick<Storage, 'getItem'> | null = safeLocalStorage()): DesktopProjectCatalogEntry[] {
  const raw = storage?.getItem(projectCatalogKey)
  if (!raw) return []
  try {
    const entries = JSON.parse(raw)
    if (!Array.isArray(entries)) return []
    return entries.flatMap(normalizeProjectCatalogEntry)
  } catch {
    return []
  }
}

export function recordProjectCatalogEntry(entry: DesktopProjectCatalogEntry, storage: CatalogStorage | null = safeLocalStorage()) {
  if (!storage) return
  const normalized = normalizeProjectCatalogEntry(entry)[0]
  if (!normalized) return
  const existing = loadProjectCatalog(storage)
  const next = [...existing.filter((item) => item.projectId !== normalized.projectId), normalized]
  storage.setItem(projectCatalogKey, JSON.stringify(next))
}

export function recordSessionCatalogEntry(entry: DesktopSessionCatalogEntry, storage: CatalogStorage | null = safeLocalStorage()) {
  if (!storage) return
  const normalized = normalizeCatalogEntry(entry)[0]
  if (!normalized) return
  const existing = loadSessionCatalog(storage)
  const next = [...existing.filter((item) => item.sessionId !== normalized.sessionId), normalized]
  storage.setItem(sessionCatalogKey, JSON.stringify(next))
}

export function replaceDesktopCatalog(projects: DesktopProjectCatalogEntry[], sessions: DesktopSessionCatalogEntry[], storage: CatalogStorage | null = safeLocalStorage()) {
  if (!storage) return
  const normalizedProjects = projects.flatMap(normalizeProjectCatalogEntry)
  const normalizedSessions = sessions.flatMap(normalizeCatalogEntry)
  storage.setItem(projectCatalogKey, JSON.stringify(normalizedProjects))
  storage.setItem(sessionCatalogKey, JSON.stringify(normalizedSessions))
}

export function latestKnownSessionID(catalog: DesktopSessionCatalogEntry[], sessions: SessionListItem[]) {
  const available = new Set(sessions.map((session) => String(session.session_id || '').trim()).filter(Boolean))
  for (let index = catalog.length - 1; index >= 0; index -= 1) {
    const sessionID = String(catalog[index]?.sessionId || '').trim()
    if (available.has(sessionID)) return sessionID
  }
  return ''
}

function normalizeCatalogEntry(value: unknown): DesktopSessionCatalogEntry[] {
  if (!value || typeof value !== 'object') return []
  const item = value as Partial<DesktopSessionCatalogEntry>
  const sessionId = String(item.sessionId || '').trim()
  const kind = String(item.kind || '').trim()
  if (!sessionId || (kind !== 'project' && kind !== 'task' && kind !== 'chat')) return []
  const projectId = String(item.projectId || '').trim()
  const root = kind === 'chat' ? '' : String(item.root || '').trim()
  if (kind === 'task' && !root) return []
  if (kind === 'project' && !projectId && !root) return []
  return [{
    sessionId,
    kind,
    projectId,
    root,
    name: String(item.name || '').trim(),
  }]
}

function normalizeProjectCatalogEntry(value: unknown): DesktopProjectCatalogEntry[] {
  if (!value || typeof value !== 'object') return []
  const item = value as Partial<DesktopProjectCatalogEntry>
  const projectId = String(item.projectId || '').trim()
  const name = String(item.name || '').trim()
  const root = String(item.root || '').trim()
  if (!projectId || !name || !root) return []
  return [{ projectId, name, root }]
}

function safeLocalStorage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}
