<script setup lang="ts">
import { computed } from 'vue'
import type { AgentInvocationChildConversation, AgentInvocationProjection, DesktopUpdate, KernelTimelineDetail } from '../api/kernelApi'

const props = defineProps<{
  baseUrl: string
  runtimeToken: string
  readiness: string
  detail: KernelTimelineDetail | null
  selectedDetailRef: string
  materialSummary: string[]
  debugSummaryRows: string[]
  compactionSummaryRows: string[]
  workerInvocations: AgentInvocationProjection[]
  workerConversation: AgentInvocationChildConversation | null
  debugExportReady: boolean
  updateToken: string
  update: DesktopUpdate | null
}>()

defineEmits<{
  'update:baseUrl': [value: string]
  'update:runtimeToken': [value: string]
  'update:selectedDetailRef': [value: string]
  checkReady: []
  loadDetail: []
  enableDebug: []
  exportDebug: []
  downloadDebug: []
  compactContext: []
  refreshWorkers: []
  selectWorker: [invocationId: string]
  'update:updateToken': [value: string]
  saveUpdateToken: []
  checkUpdate: []
  installUpdate: []
  close: []
}>()

const detailItem = computed(() => props.detail?.item ?? {})

function detailField(name: string) {
  return String(detailItem.value[name] ?? '').trim()
}

function workerRole(worker: AgentInvocationProjection) {
  return String(worker.agent_profile_ref ?? '').replace(/^agent_profile:/, '') || 'worker'
}

function workerStatus(worker: AgentInvocationProjection) {
  return String(worker.status ?? 'admitted')
}
</script>

<template>
  <aside class="inspector">
    <div class="inspector-head">
      <div>
        <p class="eyebrow">设置与诊断</p>
        <strong>{{ readiness }}</strong>
      </div>
      <button type="button" class="secondary-button" @click="$emit('close')">关闭</button>
    </div>

    <section class="panel">
      <p class="eyebrow">连接设置</p>
      <label>
        本地服务地址
        <input :value="baseUrl" spellcheck="false" @input="$emit('update:baseUrl', ($event.target as HTMLInputElement).value)" />
      </label>
      <label>
        访问令牌
        <input :value="runtimeToken" type="password" spellcheck="false" @input="$emit('update:runtimeToken', ($event.target as HTMLInputElement).value)" />
      </label>
      <button type="button" class="secondary-button" @click="$emit('checkReady')">检查连接</button>
    </section>

    <section class="panel">
      <div class="panel-head">
        <p class="eyebrow">工作代理</p>
        <button type="button" class="secondary-button" @click="$emit('refreshWorkers')">刷新</button>
      </div>
      <p v-if="!workerInvocations.length" class="status">当前会话尚未创建工作代理。</p>
      <button v-for="worker in workerInvocations" :key="worker.invocation_id" type="button" class="worker-row" @click="$emit('selectWorker', String(worker.invocation_id || ''))">
        <span>{{ workerRole(worker) }}</span>
        <small>{{ workerStatus(worker) }}</small>
      </button>
    </section>

    <section v-if="workerConversation" class="detail-panel">
      <p class="eyebrow">工作代理对话</p>
      <h2>{{ workerConversation.role_id || 'worker' }} · {{ workerConversation.status || 'unknown' }}</h2>
      <dl>
        <template v-if="workerConversation.model">
          <dt>模型</dt>
          <dd>{{ workerConversation.model }}</dd>
        </template>
        <template v-if="workerConversation.final?.text">
          <dt>终态</dt>
          <dd><pre>{{ workerConversation.final.text }}</pre></dd>
        </template>
        <template v-if="workerConversation.error?.code || workerConversation.error?.message">
          <dt>失败</dt>
          <dd>{{ workerConversation.error?.code || 'worker_failed' }}{{ workerConversation.error?.message ? `: ${workerConversation.error.message}` : '' }}</dd>
        </template>
      </dl>
    </section>

    <section class="panel">
      <p class="eyebrow">会话诊断</p>
      <label>
        详情编号
        <input :value="selectedDetailRef" spellcheck="false" @input="$emit('update:selectedDetailRef', ($event.target as HTMLInputElement).value)" />
      </label>
      <div class="button-row">
        <button type="button" class="secondary-button" @click="$emit('loadDetail')">加载详情</button>
        <button type="button" class="secondary-button" @click="$emit('enableDebug')">记录诊断</button>
        <button type="button" class="secondary-button" @click="$emit('exportDebug')">导出诊断</button>
        <button type="button" class="secondary-button" :disabled="!debugExportReady" @click="$emit('downloadDebug')">下载</button>
        <button type="button" class="secondary-button" @click="$emit('compactContext')">整理上下文</button>
      </div>
    </section>

    <section class="panel">
      <p class="eyebrow">应用更新</p>
      <label>
        GitHub 只读令牌
        <input :value="updateToken" type="password" autocomplete="off" spellcheck="false" @input="$emit('update:updateToken', ($event.target as HTMLInputElement).value)" />
      </label>
      <div class="button-row">
        <button type="button" class="secondary-button" :disabled="!updateToken.trim()" @click="$emit('saveUpdateToken')">保存令牌</button>
        <button type="button" class="secondary-button" @click="$emit('checkUpdate')">检查更新</button>
      </div>
      <p v-if="update" class="status">{{ update.available ? `发现 ${update.latest_version}` : `已是最新版本 ${update.current_version}` }}</p>
      <button v-if="update?.available" type="button" @click="$emit('installUpdate')">下载并安装 {{ update.latest_version }}</button>
      <a v-if="update?.release_url" :href="update.release_url" target="_blank" rel="noreferrer">查看发行说明</a>
    </section>

    <section v-if="detail" class="detail-panel">
      <p class="eyebrow">处理详情</p>
      <h2>{{ detailField('kind') || '详情' }}</h2>
      <dl>
        <template v-if="detailField('tool')">
          <dt>类型</dt>
          <dd>{{ detailField('tool') }}</dd>
        </template>
        <template v-if="detailField('command_preview')">
          <dt>命令</dt>
          <dd><code>{{ detailField('command_preview') }}</code></dd>
        </template>
        <template v-if="detailField('duration_ms')">
          <dt>耗时</dt>
          <dd>{{ detailField('duration_ms') }} ms</dd>
        </template>
        <template v-if="detailField('output_truncation')">
          <dt>截断</dt>
          <dd>{{ detailField('output_truncation') }}</dd>
        </template>
        <template v-if="detailField('visible_output') || detailField('output_preview')">
          <dt>输出</dt>
          <dd><pre>{{ detailField('visible_output') || detailField('output_preview') }}</pre></dd>
        </template>
      </dl>
    </section>

    <section v-if="materialSummary.length" class="detail-panel">
      <p class="eyebrow">资料导入</p>
      <dl>
        <dt>结果</dt>
        <dd>{{ materialSummary[0] }}</dd>
        <dt>资料引用</dt>
        <dd><code>{{ materialSummary[1] }}</code></dd>
        <dt>可用能力</dt>
        <dd>{{ materialSummary[2] }}</dd>
      </dl>
    </section>

    <section v-if="compactionSummaryRows.length" class="detail-panel">
      <p class="eyebrow">上下文整理</p>
      <dl>
        <dt>结果</dt>
        <dd>{{ compactionSummaryRows[0] }}</dd>
        <dt>原因</dt>
        <dd>{{ compactionSummaryRows[1] || '无' }}</dd>
      </dl>
    </section>

    <section v-if="debugSummaryRows.length" class="detail-panel">
      <p class="eyebrow">诊断记录</p>
      <dl>
        <dt>状态</dt>
        <dd>{{ debugSummaryRows[0] }}</dd>
        <dt>记录数</dt>
        <dd>{{ debugSummaryRows[1] }}</dd>
        <dt>上下文</dt>
        <dd>{{ debugSummaryRows[2] }}</dd>
        <dt>模型</dt>
        <dd>{{ debugSummaryRows[3] }}</dd>
      </dl>
    </section>
  </aside>
</template>
