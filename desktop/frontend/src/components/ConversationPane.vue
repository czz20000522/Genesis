<script setup lang="ts">
import type { KernelTimeline, TurnResponse } from '../api/kernelApi'

defineProps<{
  messageText: string
  lastTurn: TurnResponse | null
  timeline: KernelTimeline | null
  detailEntries: Array<{ detailRef: string; label: string }>
}>()

defineEmits<{
  'update:messageText': [value: string]
  sendMessage: []
  loadDetail: [detailRef: string]
}>()
</script>

<template>
  <section class="conversation">
    <div class="composer">
      <label>
        Message
        <textarea :value="messageText" rows="4" spellcheck="true" @input="$emit('update:messageText', ($event.target as HTMLTextAreaElement).value)"></textarea>
      </label>
      <button type="button" @click="$emit('sendMessage')">Send turn</button>
    </div>

    <article v-if="lastTurn" class="message-card assistant-card">
      <p class="eyebrow">Turn result</p>
      <dl>
        <dt>Turn</dt>
        <dd><code>{{ lastTurn.turn_id || 'unknown' }}</code></dd>
        <template v-if="lastTurn.final?.text">
          <dt>Final</dt>
          <dd><pre>{{ lastTurn.final.text }}</pre></dd>
        </template>
        <template v-if="lastTurn.pause">
          <dt>Pause</dt>
          <dd>{{ lastTurn.pause.reason || lastTurn.pause.wait_reason || 'paused' }}</dd>
        </template>
        <template v-if="lastTurn.error">
          <dt>Error</dt>
          <dd>{{ lastTurn.error.code }} {{ lastTurn.error.message }}</dd>
        </template>
      </dl>
    </article>

    <article class="message-card">
      <p class="eyebrow">Timeline</p>
      <p class="status">{{ timeline?.items?.length ?? 0 }} projected item(s)</p>
      <div v-if="detailEntries.length" class="detail-list">
        <button v-for="entry in detailEntries" :key="entry.detailRef" type="button" @click="$emit('loadDetail', entry.detailRef)">
          {{ entry.label }}
        </button>
      </div>
    </article>
  </section>
</template>
