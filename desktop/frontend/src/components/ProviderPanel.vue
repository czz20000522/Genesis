<script setup lang="ts">
import type { ProviderProfile } from '../api/kernelApi'

defineProps<{
  profiles: ProviderProfile[]
  selectedProfile: string
  credential: string
  busy: boolean
  notice: string
  selectedProfileIsLocal: boolean
  localModelLabel: string
  localModelStarting: boolean
  localModelRunning: boolean
	localModelExternallyServing: boolean
	templateId: string
	baseUrl: string
	modelId: string
}>()

defineEmits<{
  close: []
  'update:selectedProfile': [value: string]
  'update:credential': [value: string]
	'update:templateId': [value: string]
	'update:baseUrl': [value: string]
	'update:modelId': [value: string]
	importProvider: []
  setupDeepSeekFlash: []
  rotateCredential: []
  verify: []
  toggleLocalModel: []
}>()
</script>

<template>
  <section class="provider-panel" aria-label="模型提供商配置">
    <div class="provider-panel-head">
      <div>
		<p class="eyebrow">模型与凭据</p>
		<strong>导入后，在各会话输入框旁选择要使用的模型</strong>
      </div>
      <el-button plain @click="$emit('close')">关闭</el-button>
    </div>

    <div class="provider-controls provider-first-run">
		<p class="eyebrow">导入模型提供商</p>
		<div class="provider-template-grid">
			<el-button v-for="item in [['deepseek', 'DeepSeek'], ['openai', 'OpenAI'], ['opencode-go', 'OpenCode Go'], ['local-llama-cpp', '本地 llama.cpp'], ['openai-compatible', 'OpenAI 兼容']]" :key="item[0]" plain :type="templateId === item[0] ? 'primary' : 'default'" @click="$emit('update:templateId', item[0])">{{ item[1] }}</el-button>
		</div>
      <label>
        API Key（保存到本机受保护存储）
        <el-input :model-value="credential" type="password" show-password autocomplete="off" placeholder="sk-..." @update:model-value="$emit('update:credential', String($event))" />
      </label>
		<el-collapse v-if="templateId === 'openai-compatible'">
			<el-collapse-item title="高级选项" name="advanced">
				<label>服务地址<el-input :model-value="baseUrl" placeholder="https://your-api.example/v1" @update:model-value="$emit('update:baseUrl', String($event))" /></label>
				<label>模型 ID<el-input :model-value="modelId" placeholder="provider-model" @update:model-value="$emit('update:modelId', String($event))" /></label>
			</el-collapse-item>
		</el-collapse>
      <div class="button-row">
			<el-button type="primary" :loading="busy" :disabled="busy || (templateId !== 'local-llama-cpp' && !credential.trim())" @click="$emit('importProvider')">{{ templateId === 'local-llama-cpp' ? '导入本地模型' : '导入并获取模型' }}</el-button>
      </div>
		<el-alert v-if="templateId === 'local-llama-cpp'" title="读取已保存的本地模型设置创建 profile；不会启动模型。随后可在会话中选择并显式启动。" type="info" :closable="false" show-icon />
		<el-alert v-if="notice && !profiles.length" class="provider-notice" :title="notice" type="info" :closable="false" show-icon />
    </div>
    <div v-if="profiles.length" class="provider-controls">
      <label>
        管理模型
        <el-select :model-value="selectedProfile" filterable placeholder="选择模型" @update:model-value="$emit('update:selectedProfile', String($event))">
          <el-option v-for="profile in profiles" :key="profile.profile_id" :value="String(profile.profile_id || '')" :label="`${profile.model_id} · ${profile.profile_id}`">
            <span>{{ profile.model_id }}</span>
            <small>{{ profile.gateway_route }}</small>
          </el-option>
        </el-select>
      </label>
		<el-alert title="此处只选择要验证或更新凭据的模型；当前会话的模型请在输入框旁选择。" type="info" :closable="false" show-icon />
      <div v-if="selectedProfileIsLocal" class="local-model-control">
        <div>
          <strong>本地模型</strong>
          <p>{{ localModelLabel }}</p>
        </div>
        <el-button plain :loading="localModelStarting" :disabled="busy || localModelStarting || localModelExternallyServing" @click="$emit('toggleLocalModel')">{{ localModelStarting ? '正在启动…' : localModelExternallyServing ? '外部服务正在运行' : localModelRunning ? '停止本地模型' : '启动本地模型' }}</el-button>
      </div>
      <div class="button-row">
        <el-button plain :loading="busy" :disabled="busy || !selectedProfile" @click="$emit('verify')">验证模型</el-button>
      </div>
      <el-alert v-if="notice" class="provider-notice" :title="notice" type="info" :closable="false" show-icon />
      <el-collapse class="provider-credential">
        <el-collapse-item title="更改此模型的 API Key" name="credential">
        <label>
          API Key（保存到本机受保护存储）
          <el-input :model-value="credential" type="password" show-password autocomplete="off" @update:model-value="$emit('update:credential', String($event))" />
        </label>
        <el-button plain :loading="busy" :disabled="busy || !credential.trim()" @click="$emit('rotateCredential')">保存密钥</el-button>
        </el-collapse-item>
      </el-collapse>
    </div>

    <ul v-if="profiles.length" class="provider-list">
      <li v-for="profile in profiles" :key="profile.profile_id" :class="{ 'provider-list-active': profile.profile_id === selectedProfile }">
        <strong>{{ profile.model_id }}</strong>
        <span>{{ profile.gateway_route }} · {{ profile.protocol }}</span>
        <small>{{ profile.provider_adapter_id || 'default adapter' }} · {{ profile.credential_present ? '凭据已配置' : '无需或尚未配置凭据' }}</small>
      </li>
    </ul>
  </section>
</template>
