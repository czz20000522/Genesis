<script setup lang="ts">
import type { ProviderProfile } from '../api/kernelApi'

defineProps<{
  profiles: ProviderProfile[]
  roleBindings: Record<string, string>
  selectedRole: string
  selectedProfile: string
  credential: string
  busy: boolean
  notice: string
}>()

defineEmits<{
  close: []
  'update:selectedRole': [value: string]
  'update:selectedProfile': [value: string]
  'update:credential': [value: string]
  rotateCredential: []
  verify: []
  apply: []
}>()
</script>

<template>
  <section class="provider-panel" aria-label="模型提供商配置">
    <div class="provider-panel-head">
      <div>
        <p class="eyebrow">模型与提供商</p>
        <strong>已配置的模型</strong>
      </div>
      <button type="button" class="secondary-button" @click="$emit('close')">关闭</button>
    </div>

    <p v-if="!profiles.length" class="provider-empty">未发现已配置的模型。</p>
    <div v-else class="provider-controls">
      <label>
        角色
        <input :value="selectedRole" spellcheck="false" @input="$emit('update:selectedRole', ($event.target as HTMLInputElement).value)" />
      </label>
      <label>
        Profile
        <select :value="selectedProfile" @change="$emit('update:selectedProfile', ($event.target as HTMLSelectElement).value)">
          <option v-for="profile in profiles" :key="profile.profile_id" :value="profile.profile_id">
            {{ profile.model_id }} · {{ profile.profile_id }}
          </option>
        </select>
      </label>
      <label>
        API Key（仅本次提交）
        <input :value="credential" type="password" autocomplete="off" spellcheck="false" @input="$emit('update:credential', ($event.target as HTMLInputElement).value)" />
      </label>
      <div class="button-row">
        <button type="button" class="secondary-button" :disabled="busy || !credential.trim()" @click="$emit('rotateCredential')">保存密钥</button>
        <button type="button" class="secondary-button" :disabled="busy || !selectedProfile" @click="$emit('verify')">验证模型</button>
        <button type="button" :disabled="busy || !selectedProfile" @click="$emit('apply')">应用并重启</button>
      </div>
      <p v-if="notice" class="provider-notice">{{ notice }}</p>
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
