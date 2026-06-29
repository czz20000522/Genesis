<script setup lang="ts">
import { computed } from 'vue'
import type { KernelTimelineDetail } from '../api/kernelApi'

const props = defineProps<{
  baseUrl: string
  runtimeToken: string
  readiness: string
  detail: KernelTimelineDetail | null
  selectedDetailRef: string
  materialSummary: string[]
  debugSummaryRows: string[]
  compactionSummaryRows: string[]
  debugExportReady: boolean
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
  close: []
}>()

const detailItem = computed(() => props.detail?.item ?? {})

function detailField(name: string) {
  return String(detailItem.value[name] ?? '').trim()
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
        内核地址
        <input :value="baseUrl" spellcheck="false" @input="$emit('update:baseUrl', ($event.target as HTMLInputElement).value)" />
      </label>
      <label>
        运行令牌
        <input :value="runtimeToken" type="password" spellcheck="false" @input="$emit('update:runtimeToken', ($event.target as HTMLInputElement).value)" />
      </label>
      <button type="button" class="secondary-button" @click="$emit('checkReady')">检查连接</button>
    </section>

    <section class="panel">
      <p class="eyebrow">会话诊断</p>
      <label>
        详情引用
        <input :value="selectedDetailRef" spellcheck="false" @input="$emit('update:selectedDetailRef', ($event.target as HTMLInputElement).value)" />
      </label>
      <div class="button-row">
        <button type="button" class="secondary-button" @click="$emit('loadDetail')">加载详情</button>
        <button type="button" class="secondary-button" @click="$emit('enableDebug')">开启调试</button>
        <button type="button" class="secondary-button" @click="$emit('exportDebug')">导出调试</button>
        <button type="button" class="secondary-button" :disabled="!debugExportReady" @click="$emit('downloadDebug')">下载</button>
        <button type="button" class="secondary-button" @click="$emit('compactContext')">压缩上下文</button>
      </div>
    </section>

    <section v-if="detail" class="detail-panel">
      <p class="eyebrow">时间线详情</p>
      <h2>{{ detailField('kind') || '详情' }}</h2>
      <dl>
        <template v-if="detailField('tool')">
          <dt>工具</dt>
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
        <dt>准入</dt>
        <dd>{{ materialSummary[0] }}</dd>
        <dt>来源/拒绝</dt>
        <dd><code>{{ materialSummary[1] }}</code></dd>
        <dt>可用操作</dt>
        <dd>{{ materialSummary[2] }}</dd>
      </dl>
    </section>

    <section v-if="compactionSummaryRows.length" class="detail-panel">
      <p class="eyebrow">上下文压缩</p>
      <dl>
        <dt>准入</dt>
        <dd>{{ compactionSummaryRows[0] }}</dd>
        <dt>原因</dt>
        <dd>{{ compactionSummaryRows[1] || 'none' }}</dd>
      </dl>
    </section>

    <section v-if="debugSummaryRows.length" class="detail-panel">
      <p class="eyebrow">会话调试</p>
      <dl>
        <dt>就绪</dt>
        <dd>{{ debugSummaryRows[0] }}</dd>
        <dt>步骤</dt>
        <dd>{{ debugSummaryRows[1] }}</dd>
        <dt>输入类型</dt>
        <dd>{{ debugSummaryRows[2] }}</dd>
        <dt>模型</dt>
        <dd>{{ debugSummaryRows[3] }}</dd>
      </dl>
    </section>
  </aside>
</template>
