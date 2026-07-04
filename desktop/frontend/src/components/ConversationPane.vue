<script setup lang="ts">
import { computed } from 'vue'
import type { ApprovalDecision, ApprovalProjection, TurnResponse } from '../api/kernelApi'
import { assistantMarkdown } from '../assistantMarkdown'
import { approvalSummary } from '../approvalView'
import { readinessLabel, sessionLabel } from '../display'
import type { TimelineRow } from '../timelineView'

const props = defineProps<{
  sessionId: string
  messageText: string
  lastTurn: TurnResponse | null
  rows: TimelineRow[]
  detailEntries: Array<{ detailRef: string; label: string }>
  selectedFileName: string
  readiness: string
  approvals: ApprovalProjection[]
}>()

const emit = defineEmits<{
  'update:messageText': [value: string]
  sendMessage: []
  decideApproval: [approvalId: string, decision: ApprovalDecision]
  pickMaterial: []
  selectMaterial: [event: Event]
  loadDetail: [detailRef: string]
}>()

const approvalRows = computed(() => props.approvals.map((approval) => ({
  approval,
  rows: approvalSummary(approval),
})))

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

function useStarter(text: string) {
  emit('update:messageText', text)
}
</script>

<template>
  <section class="conversation">
    <div class="transcript" aria-live="polite">
      <article v-if="!rows.length" class="empty-chat">
        <div class="empty-mark">G</div>
        <h2>Genesis</h2>
        <p>从一个问题、一个任务或一个代码包开始。</p>
        <div class="prompt-row">
          <button type="button" @click="useStarter('帮我梳理今天最重要的下一步。')">梳理下一步</button>
          <button type="button" @click="useStarter('我会上传一个代码包，请先查看顶层结构。')">查看代码包</button>
          <button type="button" @click="useStarter('根据当前会话，给我一个简洁总结。')">总结会话</button>
          <button type="button" @click="useStarter('帮我把这个想法整理成可执行计划。')">整理计划</button>
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
            <div v-if="row.kind === 'assistant'" class="assistant-markdown" :class="{ 'assistant-markdown--streaming': row.streaming }" v-html="assistantMarkdown(row.text || row.kind)" />
            <pre v-else>{{ row.text || row.kind }}</pre>
          </template>
          <button v-if="row.detailAvailable" type="button" class="secondary-button" @click="$emit('loadDetail', row.detailRef)">详情</button>
        </div>
      </article>
    </div>

    <div class="composer-wrap">
      <div v-for="entry in approvalRows" :key="entry.approval.approval_id" class="approval-prompt" role="status" aria-live="polite">
        <div class="approval-copy">
          <p class="eyebrow">当前会话需要确认</p>
          <strong>{{ entry.rows[1] }}</strong>
          <code v-if="entry.rows[2]">{{ entry.rows[2] }}</code>
          <small v-else>{{ entry.rows[0] }}</small>
        </div>
        <div class="approval-actions">
          <button type="button" class="secondary-button" @click="$emit('decideApproval', String(entry.approval.approval_id || ''), 'denied')">拒绝</button>
          <button type="button" class="send-button" @click="$emit('decideApproval', String(entry.approval.approval_id || ''), 'approved')">允许一次</button>
        </div>
      </div>
      <p v-if="turnStatus" class="turn-status">{{ turnStatus }}</p>
      <div class="composer-card">
        <textarea
          :value="messageText"
          rows="2"
          placeholder="输入消息，或添加附件后直接发送..."
          spellcheck="true"
          @keydown="onKeydown"
          @input="$emit('update:messageText', ($event.target as HTMLTextAreaElement).value)"
        ></textarea>
        <div class="composer-actions">
          <label class="file-action">
            ＋
            <input type="file" accept=".zip,application/zip,application/x-zip-compressed" @click="$emit('pickMaterial')" @change="$emit('selectMaterial', $event)" />
          </label>
          <button type="button" class="send-button" @click="$emit('sendMessage')">发送</button>
        </div>
      </div>
      <div class="composer-meta">
        <span>{{ readinessLabel(readiness) }}</span>
        <span>{{ sessionLabel(sessionId) }}</span>
        <span v-if="selectedFileName">已选择：{{ selectedFileName }}，发送时一并上传</span>
        <span v-if="detailEntries.length">{{ detailEntries.length }} 个详情入口</span>
      </div>
    </div>
  </section>
</template>
