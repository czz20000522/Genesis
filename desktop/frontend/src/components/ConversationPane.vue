<script setup lang="ts">
import { computed } from 'vue'
import type { ApprovalDecision, ApprovalProjection, TurnResponse } from '../api/kernelApi'
import { approvalSummary } from '../approvalView'
import type { TimelineRow } from '../timelineView'
import AssistantMessage from './AssistantMessage.vue'

const props = defineProps<{
  sessionId: string
  messageText: string
  lastTurn: TurnResponse | null
  rows: TimelineRow[]
  detailEntries: Array<{ detailRef: string; label: string }>
  selectedFileName: string
  selectedFileIsDirectory: boolean
  error: string
  readiness: string
  approvals: ApprovalProjection[]
  retryText: string
  interruptAvailable: boolean
  interrupting: boolean
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

function onKeydown(rawEvent: Event | KeyboardEvent) {
  const event = rawEvent as KeyboardEvent
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
        <p class="empty-chat-kicker">新的会话</p>
        <h2>从一个问题开始。</h2>
        <p>也可以交给 Genesis 一个任务、一个项目目录或一份文件。</p>
        <div class="prompt-row">
          <el-button plain @click="useStarter('帮我梳理今天最重要的下一步。')">梳理下一步</el-button>
          <el-button plain @click="useStarter('我会上传一个代码包，请先查看顶层结构。')">查看代码包</el-button>
          <el-button plain @click="useStarter('根据当前会话，给我一个简洁总结。')">总结会话</el-button>
          <el-button plain @click="useStarter('帮我把这个想法整理成可执行计划。')">整理计划</el-button>
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
            <p v-if="row.meta && row.kind !== 'reasoning'" class="eyebrow">{{ row.meta }}</p>
            <details v-if="row.kind === 'reasoning'" class="reasoning-message">
              <summary>{{ row.meta }}</summary>
              <AssistantMessage :text="row.text || row.kind" :streaming="row.streaming" />
            </details>
            <AssistantMessage v-else-if="row.kind === 'assistant'" :text="row.text || row.kind" :streaming="row.streaming" />
            <pre v-else>{{ row.text || row.kind }}</pre>
          </template>
          <el-button v-if="row.detailAvailable" text size="small" @click="$emit('loadDetail', row.detailRef)">查看详情</el-button>
        </div>
      </article>
    </div>

    <div class="composer-wrap">
      <el-alert v-if="error" class="send-failure" title="未能完成操作" :description="error" type="error" show-icon :closable="false">
        <template v-if="retryText" #default><el-button size="small" plain @click="$emit('retry')">重试</el-button></template>
      </el-alert>
      <div v-for="entry in approvalRows" :key="entry.approval.approval_id" class="approval-prompt" role="status" aria-live="polite">
        <div class="approval-copy">
          <p class="eyebrow">当前会话需要确认</p>
          <strong>{{ entry.rows[1] }}</strong>
          <code v-if="entry.rows[2]">{{ entry.rows[2] }}</code>
          <small v-else>{{ entry.rows[0] }}</small>
        </div>
        <div class="approval-actions">
          <el-button plain @click="$emit('decideApproval', String(entry.approval.approval_id || ''), 'denied')">拒绝</el-button>
          <el-button type="primary" @click="$emit('decideApproval', String(entry.approval.approval_id || ''), 'approved')">允许一次</el-button>
        </div>
      </div>
      <p v-if="turnStatus" class="turn-status">{{ turnStatus }}</p>
      <div class="composer-card">
        <el-input
          :model-value="messageText"
          type="textarea"
          :autosize="{ minRows: 2, maxRows: 7 }"
          placeholder="输入消息，或添加附件后直接发送..."
          spellcheck="true"
          @keydown="onKeydown"
          @update:model-value="$emit('update:messageText', String($event))"
        />
        <div class="composer-actions">
          <el-button class="file-action" plain @click="$emit('pickMaterialArchive')">添加压缩包</el-button>
          <el-button class="file-action" plain @click="$emit('pickMaterialDirectory')">添加文件夹</el-button>
          <el-button v-if="interruptAvailable" plain :loading="interrupting" :disabled="interrupting" @click="$emit('interrupt')">{{ interrupting ? '正在停止…' : '停止生成' }}</el-button>
          <el-button v-else class="send-button" type="primary" @click="$emit('sendMessage')">发送</el-button>
        </div>
      </div>
      <div class="composer-meta">
        <span v-if="selectedFileName && !selectedFileIsDirectory">已选择：{{ selectedFileName }}，发送时一并上传</span>
        <span v-else-if="selectedFileName">已选择文件夹：{{ selectedFileName }}。发送时仅归档源文件，并自动跳过凭据、版本控制、依赖和构建目录。</span>
      </div>
    </div>
  </section>
</template>
