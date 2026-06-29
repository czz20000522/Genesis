export type SessionDraftSnapshot = {
  messageText?: string
  timelineRowCount?: number
  selectedFileName?: string
  hasMaterial?: boolean
  hasDebugExport?: boolean
  hasCompaction?: boolean
  hasLastTurn?: boolean
}

export function isBlankSessionDraft(snapshot: SessionDraftSnapshot) {
  return !String(snapshot.messageText ?? '').trim()
    && Number(snapshot.timelineRowCount ?? 0) === 0
    && !String(snapshot.selectedFileName ?? '').trim()
    && !snapshot.hasMaterial
    && !snapshot.hasDebugExport
    && !snapshot.hasCompaction
    && !snapshot.hasLastTurn
}
