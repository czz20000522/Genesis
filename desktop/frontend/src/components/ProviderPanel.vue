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
}>()

defineEmits<{
  close: []
  'update:selectedProfile': [value: string]
  'update:credential': [value: string]
  setupDeepSeekFlash: []
  rotateCredential: []
  verify: []
  apply: []
  toggleLocalModel: []
}>()
</script>

<template>
  <section class="provider-panel" aria-label="模型提供商配置">
    <div class="provider-panel-head">
      <div>
        <p class="eyebrow">全局默认协调模型</p>
        <strong>选择后续对话的默认模型</strong>
      </div>
      <el-button plain @click="$emit('close')">关闭</el-button>
    </div>

    <div v-if="!profiles.length" class="provider-controls provider-first-run">
      <el-alert title="配置 DeepSeek Flash 后，先验证连接，再由你决定是否应用为默认协调模型。" type="info" :closable="false" show-icon />
      <label>
        DeepSeek API Key（保存到本机受保护存储）
        <el-input :model-value="credential" type="password" show-password autocomplete="off" placeholder="sk-..." @update:model-value="$emit('update:credential', String($event))" />
      </label>
      <div class="button-row">
        <el-button type="primary" :loading="busy" :disabled="busy || !credential.trim()" @click="$emit('setupDeepSeekFlash')">保存并验证</el-button>
      </div>
      <el-alert v-if="notice" class="provider-notice" :title="notice" type="info" :closable="false" show-icon />
    </div>
    <div v-else class="provider-controls">
      <label>
        模型
        <el-select :model-value="selectedProfile" filterable placeholder="选择模型" @update:model-value="$emit('update:selectedProfile', String($event))">
          <el-option v-for="profile in profiles" :key="profile.profile_id" :value="String(profile.profile_id || '')" :label="`${profile.model_id} · ${profile.profile_id}`">
            <span>{{ profile.model_id }}</span>
            <small>{{ profile.gateway_route }}</small>
          </el-option>
        </el-select>
      </label>
      <el-alert title="应用后会重启本机 Genesis 服务；已持久化的会话记录不会改变，之后的新回合会使用此模型。" type="warning" :closable="false" show-icon />
      <div v-if="selectedProfileIsLocal" class="local-model-control">
        <div>
          <strong>本地模型</strong>
          <p>{{ localModelLabel }}</p>
        </div>
        <el-button plain :loading="localModelStarting" :disabled="busy || localModelStarting" @click="$emit('toggleLocalModel')">{{ localModelStarting ? '正在启动…' : localModelRunning ? '停止本地模型' : '启动本地模型' }}</el-button>
      </div>
      <div class="button-row">
        <el-button plain :loading="busy" :disabled="busy || !selectedProfile" @click="$emit('verify')">验证模型</el-button>
        <el-button type="primary" :loading="busy" :disabled="busy || !selectedProfile" @click="$emit('apply')">应用并重启服务</el-button>
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
