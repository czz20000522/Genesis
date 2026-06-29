<script setup lang="ts">
import { ref } from 'vue'
import { getReady, kernelConfig, saveKernelConfig } from './api/kernelApi'

const config = ref(kernelConfig())
const readiness = ref('unchecked')
const error = ref('')

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
    </section>
  </main>
</template>
