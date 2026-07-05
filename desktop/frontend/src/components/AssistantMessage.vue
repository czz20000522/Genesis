<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { assistantMarkdown } from '../assistantMarkdown'

const props = defineProps<{
  text: string
  streaming?: boolean
}>()

const root = ref<HTMLElement | null>(null)
let renderSerial = 0
let mermaidModule: Promise<typeof import('mermaid')> | null = null

watch(() => props.text, () => {
  void renderMermaidBlocks()
}, { flush: 'post' })

onMounted(() => {
  root.value?.addEventListener('click', onClick)
  void renderMermaidBlocks()
})

onUnmounted(() => {
  root.value?.removeEventListener('click', onClick)
})

async function renderMermaidBlocks() {
  await nextTick()
  for (const card of Array.from(root.value?.querySelectorAll<HTMLElement>('.mermaid-card') ?? [])) {
    if (card.dataset.rendered === 'true') continue
    const code = mermaidCode(card)
    const target = card.querySelector<HTMLElement>('[data-mermaid-panel="diagram"]')
    if (!code || !target) continue
    try {
      const mermaid = await loadMermaid()
      const id = `genesis-mermaid-${Date.now()}-${renderSerial++}`
      const { svg } = await mermaid.default.render(id, code)
      target.innerHTML = svg
      card.dataset.rendered = 'true'
    } catch (error) {
      target.textContent = error instanceof Error ? error.message : String(error)
      card.dataset.rendered = 'error'
    }
  }
}

function loadMermaid() {
  mermaidModule ??= import('mermaid').then((module) => {
    module.default.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      flowchart: { htmlLabels: true },
    })
    return module
  })
  return mermaidModule
}

async function onClick(event: MouseEvent) {
  const button = (event.target as HTMLElement).closest<HTMLButtonElement>('[data-mermaid-action]')
  if (!button) return
  const card = button.closest<HTMLElement>('.mermaid-card')
  if (!card) return
  const action = String(button.dataset.mermaidAction || '')
  if (action === 'show-diagram' || action === 'show-code') {
    showPanel(card, action === 'show-code' ? 'code' : 'diagram')
  } else if (action === 'copy-code') {
    await navigator.clipboard?.writeText(mermaidCode(card))
  } else if (action === 'copy-svg') {
    await copySvg(card)
  } else if (action === 'download-svg') {
    downloadSvg(card)
  }
}

function showPanel(card: HTMLElement, panel: 'diagram' | 'code') {
  card.querySelector<HTMLElement>('[data-mermaid-panel="diagram"]')?.toggleAttribute('hidden', panel !== 'diagram')
  card.querySelector<HTMLElement>('[data-mermaid-panel="code"]')?.toggleAttribute('hidden', panel !== 'code')
  for (const tab of Array.from(card.querySelectorAll<HTMLElement>('.mermaid-tab'))) {
    tab.classList.toggle('mermaid-tab-active', tab.dataset.mermaidAction === `show-${panel}`)
  }
}

function mermaidCode(card: HTMLElement) {
  return decodeURIComponent(String(card.dataset.mermaidCode || ''))
}

async function copySvg(card: HTMLElement) {
  const svg = card.querySelector('[data-mermaid-panel="diagram"] svg')?.outerHTML ?? ''
  if (!svg) return
  const ClipboardItemCtor = (globalThis as Record<string, unknown>).ClipboardItem as typeof ClipboardItem | undefined
  if (navigator.clipboard?.write && ClipboardItemCtor) {
    await navigator.clipboard.write([new ClipboardItemCtor({ 'image/svg+xml': new Blob([svg], { type: 'image/svg+xml' }) })])
    return
  }
  await navigator.clipboard?.writeText(svg)
}

function downloadSvg(card: HTMLElement) {
  const svg = card.querySelector('[data-mermaid-panel="diagram"] svg')?.outerHTML ?? ''
  if (!svg) return
  const url = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }))
  const link = document.createElement('a')
  link.href = url
  link.download = 'genesis-diagram.svg'
  link.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <div
    ref="root"
    class="assistant-markdown"
    :class="{ 'assistant-markdown--streaming': streaming }"
    v-html="assistantMarkdown(text)"
  />
</template>
