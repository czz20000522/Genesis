<script setup lang="ts">
import { computed, ref } from 'vue'
import { compactSessionContext, decideApproval, enableSessionDebug, getReady, getSessionDebug, getTimeline, getTimelineDetail, kernelConfig, listApprovals, saveKernelConfig, submitTurn, uploadMaterial, type ApprovalProjection, type ContextCompactionResponse, type KernelTimeline, type KernelTimelineDetail, type MaterialIntakeProjection, type SessionDebugExport, type TurnResponse } from './api/kernelApi'
import { approvalSummary } from './approvalView'
import { compactionSummary } from './compactionView'
import { debugExportText, debugSummary } from './debugExport'
import { materialIntakeSummary } from './materialIntake'
import { timelineDetailEntries } from './timelineDetail'

const config = ref(kernelConfig())
const readiness = ref('unchecked')
const error = ref('')
const sessionId = ref('')
const messageText = ref('')
const lastTurn = ref<TurnResponse | null>(null)
const selectedDetailRef = ref('')
const timeline = ref<KernelTimeline | null>(null)
const detail = ref<KernelTimelineDetail | null>(null)
const selectedFile = ref<File | null>(null)
const material = ref<MaterialIntakeProjection | null>(null)
const approvals = ref<ApprovalProjection[]>([])
const approvalReason = ref('')
const debugExport = ref<SessionDebugExport | null>(null)
const compaction = ref<ContextCompactionResponse | null>(null)
const detailEntries = computed(() => timelineDetailEntries(timeline.value?.items))
const detailItem = computed(() => detail.value?.item ?? {})
const materialSummary = computed(() => material.value ? materialIntakeSummary(material.value) : [])
const debugSummaryRows = computed(() => debugExport.value ? debugSummary(debugExport.value) : [])
const compactionSummaryRows = computed(() => compaction.value ? compactionSummary(compaction.value) : [])

async function checkReady() {
  error.value = ''
  saveKernelConfig(config.value)
  try {
    const payload = await getReady(config.value)
    readiness.value = String(payload.readiness ?? payload.status ?? 'unknown')
  } catch (err) {
    readiness.value = 'not_ready'
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadTimeline() {
  error.value = ''
  saveKernelConfig(config.value)
  detail.value = null
  try {
    timeline.value = await getTimeline(config.value, sessionId.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function sendMessage() {
  error.value = ''
  saveKernelConfig(config.value)
  const text = messageText.value.trim()
  const session = sessionId.value.trim()
  if (!session) {
    error.value = 'session id is required'
    return
  }
  if (!text) {
    error.value = 'message is required'
    return
  }
  try {
    lastTurn.value = await submitTurn(config.value, session, text, newDesktopIdempotencyKey())
    messageText.value = ''
    timeline.value = await getTimeline(config.value, session)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadDetail(detailRef = selectedDetailRef.value) {
  error.value = ''
  saveKernelConfig(config.value)
  selectedDetailRef.value = detailRef
  try {
    detail.value = await getTimelineDetail(config.value, sessionId.value, detailRef)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function detailField(name: string) {
  return String(detailItem.value[name] ?? '').trim()
}

function selectMaterial(event: Event) {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0] ?? null
}

async function uploadSelectedMaterial() {
  error.value = ''
  saveKernelConfig(config.value)
  if (!selectedFile.value) {
    error.value = 'select a material file first'
    return
  }
  try {
    material.value = await uploadMaterial(config.value, sessionId.value, selectedFile.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function loadApprovals() {
  error.value = ''
  saveKernelConfig(config.value)
  try {
    approvals.value = (await listApprovals(config.value)).items ?? []
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function submitApprovalDecision(approvalId: string, decision: 'approved' | 'denied') {
  error.value = ''
  saveKernelConfig(config.value)
  try {
    await decideApproval(config.value, approvalId, decision, approvalReason.value || decision)
    approvalReason.value = ''
    await loadApprovals()
    if (sessionId.value.trim()) {
      timeline.value = await getTimeline(config.value, sessionId.value)
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function enableDebug() {
  error.value = ''
  saveKernelConfig(config.value)
  if (!sessionId.value.trim()) {
    error.value = 'session id is required'
    return
  }
  try {
    debugExport.value = await enableSessionDebug(config.value, sessionId.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

async function exportDebug() {
  error.value = ''
  saveKernelConfig(config.value)
  if (!sessionId.value.trim()) {
    error.value = 'session id is required'
    return
  }
  try {
    debugExport.value = await getSessionDebug(config.value, sessionId.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function downloadDebugExport() {
  if (!debugExport.value) return
  const url = URL.createObjectURL(new Blob([debugExportText(debugExport.value)], { type: 'application/json' }))
  const link = document.createElement('a')
  link.href = url
  link.download = `${sessionId.value.trim() || 'session'}-debug.json`
  link.click()
  URL.revokeObjectURL(url)
}

async function compactContext() {
  error.value = ''
  saveKernelConfig(config.value)
  if (!sessionId.value.trim()) {
    error.value = 'session id is required'
    return
  }
  try {
    compaction.value = await compactSessionContext(config.value, sessionId.value)
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
}

function newDesktopIdempotencyKey() {
  const randomUUID = globalThis.crypto?.randomUUID?.()
  if (randomUUID) return `desktop-turn-${randomUUID}`
  return `desktop-turn-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}
</script>

<template>
  <main>
    <section class="shell">
      <header>
        <p class="eyebrow">Genesis Desktop</p>
        <h1>Local kernel shell</h1>
      </header>

      <label>
        Kernel URL
        <input v-model="config.baseUrl" spellcheck="false" />
      </label>

      <label>
        Runtime token
        <input v-model="config.runtimeToken" type="password" spellcheck="false" />
      </label>

      <button type="button" @click="checkReady">Check kernel</button>

      <p class="status">readiness: {{ readiness }}</p>
      <p v-if="error" class="error">{{ error }}</p>

      <div class="divider"></div>

      <label>
        Session ID
        <input v-model="sessionId" spellcheck="false" />
      </label>

      <label>
        Message
        <textarea v-model="messageText" rows="4" spellcheck="true"></textarea>
      </label>

      <button type="button" @click="sendMessage">Send turn</button>

      <aside v-if="lastTurn" class="detail-panel">
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
      </aside>

      <label>
        Material zip
        <input type="file" accept=".zip,application/zip,application/x-zip-compressed" @change="selectMaterial" />
      </label>

      <button type="button" @click="uploadSelectedMaterial">Upload material</button>

      <aside v-if="material" class="detail-panel">
        <p class="eyebrow">Material intake</p>
        <dl>
          <dt>Admission</dt>
          <dd>{{ materialSummary[0] }}</dd>
          <dt>Source/refusal</dt>
          <dd><code>{{ materialSummary[1] }}</code></dd>
          <dt>Operations</dt>
          <dd>{{ materialSummary[2] }}</dd>
        </dl>
      </aside>

      <button type="button" @click="loadTimeline">Load timeline</button>

      <div class="divider"></div>

      <button type="button" @click="loadApprovals">Load pending approvals</button>

      <div class="detail-list">
        <button type="button" @click="enableDebug">Enable debug</button>
        <button type="button" @click="exportDebug">Export debug</button>
        <button type="button" :disabled="!debugExport" @click="downloadDebugExport">Download debug JSON</button>
        <button type="button" @click="compactContext">Compact context</button>
      </div>

      <aside v-if="compaction" class="detail-panel">
        <p class="eyebrow">Context compaction</p>
        <dl>
          <dt>Admission</dt>
          <dd>{{ compactionSummaryRows[0] }}</dd>
          <dt>Reason</dt>
          <dd>{{ compactionSummaryRows[1] || 'none' }}</dd>
        </dl>
      </aside>

      <aside v-if="debugExport" class="detail-panel">
        <p class="eyebrow">Session debug</p>
        <dl>
          <dt>Readiness</dt>
          <dd>{{ debugSummaryRows[0] }}</dd>
          <dt>Steps</dt>
          <dd>{{ debugSummaryRows[1] }}</dd>
          <dt>Input kinds</dt>
          <dd>{{ debugSummaryRows[2] }}</dd>
          <dt>Models</dt>
          <dd>{{ debugSummaryRows[3] }}</dd>
        </dl>
      </aside>

      <label>
        Decision reason
        <input v-model="approvalReason" spellcheck="true" />
      </label>

      <aside v-if="approvals.length" class="detail-panel">
        <p class="eyebrow">Pending approvals</p>
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
            <button type="button" @click="submitApprovalDecision(String(approval.approval_id), 'approved')">Approve</button>
            <button type="button" class="danger" @click="submitApprovalDecision(String(approval.approval_id), 'denied')">Deny</button>
          </div>
        </article>
      </aside>

      <div v-if="detailEntries.length" class="detail-list">
        <button v-for="entry in detailEntries" :key="entry.detailRef" type="button" @click="loadDetail(entry.detailRef)">
          {{ entry.label }}
        </button>
      </div>

      <label>
        Detail ref
        <input v-model="selectedDetailRef" spellcheck="false" />
      </label>

      <button type="button" @click="loadDetail()">Load detail</button>

      <aside v-if="detail" class="detail-panel">
        <p class="eyebrow">Timeline detail</p>
        <h2>{{ detailField('kind') || 'detail' }}</h2>
        <dl>
          <template v-if="detailField('tool')">
            <dt>Tool</dt>
            <dd>{{ detailField('tool') }}</dd>
          </template>
          <template v-if="detailField('command_preview')">
            <dt>Command</dt>
            <dd><code>{{ detailField('command_preview') }}</code></dd>
          </template>
          <template v-if="detailField('duration_ms')">
            <dt>Duration</dt>
            <dd>{{ detailField('duration_ms') }} ms</dd>
          </template>
          <template v-if="detailField('output_truncation')">
            <dt>Truncation</dt>
            <dd>{{ detailField('output_truncation') }}</dd>
          </template>
          <template v-if="detailField('visible_output') || detailField('output_preview')">
            <dt>Output</dt>
            <dd><pre>{{ detailField('visible_output') || detailField('output_preview') }}</pre></dd>
          </template>
        </dl>
      </aside>
    </section>
  </main>
</template>
