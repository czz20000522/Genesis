<script setup lang="ts">
import { approvalSummary } from '../approvalView'
import type { ApprovalDecision, ApprovalProjection } from '../api/kernelApi'

defineProps<{
  approvals: ApprovalProjection[]
  approvalReason: string
}>()

defineEmits<{
  'update:approvalReason': [value: string]
  loadApprovals: []
  decideApproval: [approvalId: string, decision: ApprovalDecision]
}>()
</script>

<template>
  <aside class="dock">
    <section class="panel">
      <p class="eyebrow">Approvals</p>
      <button type="button" @click="$emit('loadApprovals')">Load pending approvals</button>
      <label>
        Decision reason
        <input :value="approvalReason" spellcheck="true" @input="$emit('update:approvalReason', ($event.target as HTMLInputElement).value)" />
      </label>
      <article v-for="approval in approvals" :key="approval.approval_id" class="approval-row">
        <dl>
          <dt>Status</dt>
          <dd>{{ approvalSummary(approval)[0] }}</dd>
          <dt>Tool</dt>
          <dd>{{ approvalSummary(approval)[1] }}</dd>
          <dt>Effect</dt>
          <dd><code>{{ approvalSummary(approval)[2] }}</code></dd>
        </dl>
        <div class="detail-list">
          <button type="button" @click="$emit('decideApproval', String(approval.approval_id), 'approved')">Approve</button>
          <button type="button" class="danger" @click="$emit('decideApproval', String(approval.approval_id), 'denied')">Deny</button>
        </div>
      </article>
    </section>
  </aside>
</template>
