<script setup lang="ts">
import type { ApprovalDecision, ApprovalProjection, ProviderProfile, TurnResponse } from '../api/kernelApi'
import type { TimelineRow } from '../timelineView'
import WorkspaceHeader from './WorkspaceHeader.vue'
import WorkspaceTimeline from './WorkspaceTimeline.vue'
import TaskComposer from './TaskComposer.vue'

defineProps<{
  title: string
  kindLabel: string
  workspaceRoot: string
  modelLabel: string
  inspectorOpen: boolean
  messageText: string
  lastTurn: TurnResponse | null
  rows: TimelineRow[]
  selectedFileName: string
  selectedFileIsDirectory: boolean
  error: string
  approvals: ApprovalProjection[]
  retryText: string
  interruptAvailable: boolean
  interrupting: boolean
  profiles: ProviderProfile[]
  selectedModelProfile: string
  modelSelectionDisabled: boolean
}>()

const emit = defineEmits<{
  'update:messageText': [value: string]
  sendMessage: []
  decideApproval: [approvalId: string, decision: ApprovalDecision]
  pickMaterialArchive: []
  pickMaterialDirectory: []
  loadDetail: [detailRef: string]
  retry: []
  interrupt: []
  selectModel: [profileID: string]
  toggleInspector: []
}>()

function forwardApproval(approvalID: string, decision: ApprovalDecision) {
  emit('decideApproval', approvalID, decision)
}
</script>

<template>
  <section class="agent-workspace">
    <WorkspaceHeader :title="title" :kind-label="kindLabel" :workspace-root="workspaceRoot" :model-label="modelLabel" :inspector-open="inspectorOpen" @toggle-inspector="$emit('toggleInspector')" />
    <div class="agent-workspace-canvas">
      <WorkspaceTimeline v-if="rows.length || approvals.length" :rows="rows" :approvals="approvals" @decide-approval="forwardApproval" @load-detail="$emit('loadDetail', $event)" />
      <section v-else class="workspace-empty-state">
        <p class="workspace-kicker">新的任务</p>
        <h2>从一个目标开始。</h2>
        <p>描述结果、提供材料，或让 Genesis 先探索当前工作区。</p>
      </section>
    </div>
    <TaskComposer
      :message-text="messageText"
      :last-turn="lastTurn"
      :selected-file-name="selectedFileName"
      :selected-file-is-directory="selectedFileIsDirectory"
      :error="error"
      :profiles="profiles"
      :selected-model-profile="selectedModelProfile"
      :model-selection-disabled="modelSelectionDisabled"
      :interrupt-available="interruptAvailable"
      :interrupting="interrupting"
      :retry-text="retryText"
      @update:message-text="$emit('update:messageText', $event)"
      @send-message="$emit('sendMessage')"
      @select-model="$emit('selectModel', $event)"
      @pick-material-archive="$emit('pickMaterialArchive')"
      @pick-material-directory="$emit('pickMaterialDirectory')"
      @retry="$emit('retry')"
      @interrupt="$emit('interrupt')"
    />
  </section>
</template>
