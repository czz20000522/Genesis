<script setup lang="ts">
defineProps<{
  baseUrl: string
  runtimeToken: string
  readiness: string
  error: string
}>()

defineEmits<{
  'update:baseUrl': [value: string]
  'update:runtimeToken': [value: string]
  checkReady: []
}>()
</script>

<template>
  <header class="topbar">
    <div>
      <p class="eyebrow">Genesis Desktop</p>
      <h1>Session workbench</h1>
      <p class="status">readiness: {{ readiness }}</p>
      <p v-if="error" class="error">{{ error }}</p>
    </div>
    <div class="connection-fields">
      <label>
        Kernel URL
        <input :value="baseUrl" spellcheck="false" @input="$emit('update:baseUrl', ($event.target as HTMLInputElement).value)" />
      </label>
      <label>
        Runtime token
        <input :value="runtimeToken" type="password" spellcheck="false" @input="$emit('update:runtimeToken', ($event.target as HTMLInputElement).value)" />
      </label>
      <button type="button" @click="$emit('checkReady')">Check kernel</button>
    </div>
  </header>
</template>
