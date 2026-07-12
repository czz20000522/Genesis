<script setup lang="ts">
import { computed } from 'vue'
import type { AgentInvocationChildConversation, AgentInvocationProjection, CloseBehavior, DesktopUpdate, KernelTimelineDetail, TaskGraphProjection } from '../api/kernelApi'
import { operationErrorLabel, readinessLabel } from '../display'

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
  taskGraphs: TaskGraphProjection[]
  debugExportReady: boolean
  updateToken: string
  update: DesktopUpdate | null
  closeBehavior: CloseBehavior
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
  refreshTaskGraphs: []
  selectWorker: [invocationId: string]
  'update:updateToken': [value: string]
  saveUpdateToken: []
  checkUpdate: []
  installUpdate: []
  'update:closeBehavior': [value: CloseBehavior]
  close: []
}>()

const detailItem = computed(() => props.detail?.item ?? {})
const workerFailure = computed(() => {
  const error = props.workerConversation?.error
  if (!error?.code && !error?.message) return ''
  return operationErrorLabel([error.code, error.message].filter(Boolean).join(': '), '完成工作代理任务')
})

function detailField(name: string) {
  return String(detailItem.value[name] ?? '').trim()
}

function workerRole(worker: AgentInvocationProjection) {
  return String(worker.agent_profile_ref ?? '').replace(/^agent_profile:/, '') || 'worker'
}

function workerStatus(worker: AgentInvocationProjection) {
  return String(worker.status ?? 'admitted')
}

function taskSummary(graph: TaskGraphProjection) {
  const nodes = graph.nodes ?? []
  const done = nodes.filter((node) => node.status === 'completed').length
  const blocked = nodes.filter((node) => node.status === 'blocked').length
  return `${done}/${nodes.length} 已完成${blocked ? ` · ${blocked} 项受阻` : ''}`
}
</script>

<template>
  <aside class="inspector">
    <div class="inspector-head">
      <div>
        <p class="eyebrow">设置与诊断</p>
        <strong>{{ readinessLabel(readiness) }}</strong>
      </div>
      <el-button plain @click="$emit('close')">关闭</el-button>
    </div>

    <section class="panel">
	  <p class="eyebrow">应用行为</p>
	  <label>关闭窗口时
		<el-select :model-value="closeBehavior" @update:model-value="$emit('update:closeBehavior', String($event) as CloseBehavior)">
		  <el-option label="直接关闭 Genesis" value="exit" />
		  <el-option label="最小化到托盘" value="minimize_to_tray" />
		</el-select>
	  </label>
	</section>

    <section class="panel">
      <p class="eyebrow">连接设置</p>
      <label>
        本地服务地址
        <el-input :model-value="baseUrl" spellcheck="false" @update:model-value="$emit('update:baseUrl', String($event))" />
      </label>
      <label>
        访问令牌
        <el-input :model-value="runtimeToken" type="password" show-password spellcheck="false" @update:model-value="$emit('update:runtimeToken', String($event))" />
      </label>
      <el-button plain @click="$emit('checkReady')">检查连接</el-button>
    </section>

    <section class="panel">
      <div class="panel-head">
        <p class="eyebrow">项目任务图</p>
        <el-button plain size="small" @click="$emit('refreshTaskGraphs')">刷新</el-button>
      </div>
      <p v-if="!taskGraphs.length" class="status">当前会话尚未记录项目任务图。</p>
      <div v-for="graph in taskGraphs" :key="graph.graph_id" class="worker-row">
        <span>任务图 · {{ String(graph.graph_id || '').slice(-8) || 'unknown' }}</span>
        <small>{{ taskSummary(graph) }}</small>
      </div>
    </section>

    <section class="panel">
      <div class="panel-head">
        <p class="eyebrow">工作代理</p>
        <el-button plain size="small" @click="$emit('refreshWorkers')">刷新</el-button>
      </div>
      <p v-if="!workerInvocations.length" class="status">当前会话尚未创建工作代理。</p>
      <el-button v-for="worker in workerInvocations" :key="worker.invocation_id" plain class="worker-row" @click="$emit('selectWorker', String(worker.invocation_id || ''))">
        <span>{{ workerRole(worker) }}</span>
        <small>{{ workerStatus(worker) }}</small>
      </el-button>
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
        <template v-if="workerFailure">
          <dt>失败</dt>
          <dd>{{ workerFailure }}</dd>
        </template>
      </dl>
    </section>

    <section class="panel">
      <p class="eyebrow">会话诊断</p>
      <label>
        详情编号
        <el-input :model-value="selectedDetailRef" spellcheck="false" @update:model-value="$emit('update:selectedDetailRef', String($event))" />
      </label>
      <div class="button-row">
        <el-button plain @click="$emit('loadDetail')">加载详情</el-button>
        <el-button plain @click="$emit('enableDebug')">记录诊断</el-button>
        <el-button plain @click="$emit('exportDebug')">导出诊断</el-button>
        <el-button plain :disabled="!debugExportReady" @click="$emit('downloadDebug')">下载</el-button>
        <el-button plain @click="$emit('compactContext')">整理上下文</el-button>
      </div>
    </section>

    <section class="panel">
      <p class="eyebrow">应用更新</p>
      <label>
        GitHub 只读令牌
        <el-input :model-value="updateToken" type="password" show-password autocomplete="off" spellcheck="false" @update:model-value="$emit('update:updateToken', String($event))" />
      </label>
      <div class="button-row">
        <el-button plain :disabled="!updateToken.trim()" @click="$emit('saveUpdateToken')">保存令牌</el-button>
        <el-button plain @click="$emit('checkUpdate')">检查更新</el-button>
      </div>
      <p v-if="update" class="status">{{ update.available ? `发现 ${update.latest_version}` : `已是最新版本 ${update.current_version}` }}</p>
      <el-button v-if="update?.available" type="primary" @click="$emit('installUpdate')">下载并安装 {{ update.latest_version }}</el-button>
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
