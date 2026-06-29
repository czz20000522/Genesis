<script setup lang="ts">
import { computed } from 'vue'
import { approvalSummary } from '../approvalView'
import type { ApprovalDecision, ApprovalProjection, TurnResponse } from '../api/kernelApi'
import type { TimelineRow } from '../timelineView'

const props = defineProps<{
  sessionId: string
  messageText: string
  lastTurn: TurnResponse | null
  rows: TimelineRow[]
  detailEntries: Array<{ detailRef: string; label: string }>
  approvals: ApprovalProjection[]
  approvalReason: string
  selectedFileName: string
  readiness: string
}>()

const emit = defineEmits<{
  'update:messageText': [value: string]
  'update:approvalReason': [value: string]
  sendMessage: []
  selectMaterial: [event: Event]
  uploadMaterial: []
  loadDetail: [detailRef: string]
  loadApprovals: []
  decideApproval: [approvalId: string, decision: ApprovalDecision]
}>()

const turnStatus = computed(() => {
  if (props.lastTurn?.error) return [props.lastTurn.error.code, props.lastTurn.error.message].filter(Boolean).join(': ')
  if (props.lastTurn?.pause) return String(props.lastTurn.pause.reason ?? props.lastTurn.pause.wait_reason ?? '回合已暂停')
  if (props.lastTurn?.turn_id) return `回合 ${props.lastTurn.turn_id} 已提交`
  return ''
})

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return
  event.preventDefault()
  emit('sendMessage')
}
</script>

<template>
  <section class="conversation">
    <div class="transcript" aria-live="polite">
      <article v-if="!rows.length && !approvals.length" class="empty-chat">
        <div class="empty-mark">G</div>
        <h2>Genesis</h2>
        <div class="prompt-row">
          <span>总结这个会话</span>
          <span>分析一个代码包</span>
          <span>规划下一步</span>
        </div>
      </article>

      <article v-for="row in rows" :key="row.id" class="chat-row" :class="`chat-row-${row.kind}`">
        <div class="chat-bubble">
          <div v-if="row.kind === 'processing'" class="processing-line">
            <span class="pulse-dot" />
            <span>{{ row.text }}</span>
            <span v-if="row.meta" class="row-meta">{{ row.meta }}</span>
          </div>
          <template v-else>
            <p v-if="row.meta" class="eyebrow">{{ row.meta }}</p>
            <pre>{{ row.text || row.kind }}</pre>
          </template>
          <button v-if="row.detailAvailable" type="button" class="secondary-button" @click="$emit('loadDetail', row.detailRef)">详情</button>
        </div>
      </article>

      <article v-for="approval in approvals" :key="approval.approval_id" class="chat-row chat-row-action">
        <div class="chat-bubble approval-card">
          <p class="eyebrow">需要审批</p>
          <dl>
            <dt>状态</dt>
            <dd>{{ approvalSummary(approval)[0] }}</dd>
            <dt>工具</dt>
            <dd>{{ approvalSummary(approval)[1] }}</dd>
            <dt>动作</dt>
            <dd><code>{{ approvalSummary(approval)[2] }}</code></dd>
          </dl>
          <label>
            审批说明
            <input :value="approvalReason" spellcheck="true" @input="$emit('update:approvalReason', ($event.target as HTMLInputElement).value)" />
          </label>
          <div class="button-row">
            <button type="button" @click="$emit('decideApproval', String(approval.approval_id), 'approved')">批准</button>
            <button type="button" class="danger" @click="$emit('decideApproval', String(approval.approval_id), 'denied')">拒绝</button>
          </div>
        </div>
      </article>
    </div>

    <div class="composer-wrap">
      <p v-if="turnStatus" class="turn-status">{{ turnStatus }}</p>
      <div class="composer-card">
        <textarea
          :value="messageText"
          rows="2"
          placeholder="给 Genesis 发送消息..."
          spellcheck="true"
          @keydown="onKeydown"
          @input="$emit('update:messageText', ($event.target as HTMLTextAreaElement).value)"
        ></textarea>
        <div class="composer-actions">
          <label class="file-action">
            附件
            <input type="file" accept=".zip,application/zip,application/x-zip-compressed" @change="$emit('selectMaterial', $event)" />
          </label>
          <button type="button" class="secondary-button" :disabled="!selectedFileName" @click="$emit('uploadMaterial')">上传</button>
          <button type="button" class="secondary-button" @click="$emit('loadApprovals')">审批</button>
          <button type="button" class="send-button" @click="$emit('sendMessage')">发送</button>
        </div>
      </div>
      <div class="composer-meta">
        <span>{{ readiness }}</span>
        <span>{{ sessionId }}</span>
        <span v-if="selectedFileName">{{ selectedFileName }}</span>
        <span v-if="detailEntries.length">{{ detailEntries.length }} 个详情入口</span>
      </div>
    </div>
  </section>
</template>
