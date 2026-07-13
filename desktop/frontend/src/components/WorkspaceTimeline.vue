<script setup lang="ts">
import { computed } from 'vue'
import { CircleCheckFilled, CircleCloseFilled, MoreFilled } from '@element-plus/icons-vue'
import type { ApprovalDecision, ApprovalProjection } from '../api/kernelApi'
import type { TimelineRow } from '../timelineView'
import { workspaceActivity } from '../workspaceActivity'
import { approvalSummary } from '../approvalView'
import AssistantMessage from './AssistantMessage.vue'

const props = defineProps<{
  rows: TimelineRow[]
  approvals: ApprovalProjection[]
}>()

defineEmits<{
  decideApproval: [approvalId: string, decision: ApprovalDecision]
  loadDetail: [detailRef: string]
}>()

const activityRows = computed(() => workspaceActivity(props.rows))
const approvalRows = computed(() => props.approvals.map((approval) => ({ approval, rows: approvalSummary(approval) })))
</script>

<template>
  <section class="workspace-timeline" aria-live="polite">
    <article v-for="row in activityRows" :key="row.id" class="activity-row" :class="`activity-row--${row.presentation}`">
      <template v-if="row.presentation === 'output'">
        <p class="activity-label">{{ row.label }}</p>
        <div class="activity-output"><AssistantMessage :text="row.text || '已生成结果。'" :streaming="row.streaming" /></div>
      </template>
      <template v-else-if="row.presentation === 'thinking'">
        <details class="activity-thinking">
          <summary><span class="activity-marker" />{{ row.label }}<span v-if="row.meta" class="activity-meta">{{ row.meta }}</span></summary>
          <AssistantMessage :text="row.text || '思考内容不可用。'" :streaming="row.streaming" />
        </details>
      </template>
      <template v-else-if="row.presentation === 'brief'">
        <p class="activity-label">{{ row.label }}</p>
        <p class="activity-brief">{{ row.text }}</p>
      </template>
      <template v-else>
        <div class="activity-line">
          <span class="activity-marker" :class="{ 'activity-marker--done': row.terminalOutcome === 'succeeded' }" />
          <strong>{{ row.label }}</strong>
          <span v-if="row.meta" class="activity-meta">{{ row.meta }}</span>
          <el-button v-if="row.detailAvailable" text circle size="small" :aria-label="`${row.label}详情`" @click="$emit('loadDetail', row.detailRef)"><el-icon><MoreFilled /></el-icon></el-button>
        </div>
        <p v-if="row.text && row.text !== row.label" class="activity-copy">{{ row.text }}</p>
      </template>
      <el-button v-if="row.detailAvailable && row.presentation === 'output'" text size="small" @click="$emit('loadDetail', row.detailRef)">查看详情</el-button>
    </article>

    <article v-for="entry in approvalRows" :key="entry.approval.approval_id" class="activity-approval" role="status">
      <div>
        <p class="activity-label">Needs your decision</p>
        <strong>{{ entry.rows[1] }}</strong>
        <small v-if="entry.rows[2]">{{ entry.rows[2] }}</small>
      </div>
      <div class="activity-approval-actions">
        <el-button plain @click="$emit('decideApproval', String(entry.approval.approval_id || ''), 'denied')"><el-icon><CircleCloseFilled /></el-icon>拒绝</el-button>
        <el-button type="primary" @click="$emit('decideApproval', String(entry.approval.approval_id || ''), 'approved')"><el-icon><CircleCheckFilled /></el-icon>允许一次</el-button>
      </div>
    </article>
  </section>
</template>
