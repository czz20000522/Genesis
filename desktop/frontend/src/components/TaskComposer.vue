<script setup lang="ts">
import { computed } from 'vue'
import { FolderOpened, Paperclip, Promotion } from '@element-plus/icons-vue'
import type { ProviderProfile, TurnResponse } from '../api/kernelApi'
import { isLocalProfile, profileDisplayName } from '../modelSelection'
import { turnErrorLabel } from '../display'

const props = defineProps<{
  messageText: string
  lastTurn: TurnResponse | null
  selectedFileName: string
  selectedFileIsDirectory: boolean
  error: string
  profiles: ProviderProfile[]
  selectedModelProfile: string
  modelSelectionDisabled: boolean
  interruptAvailable: boolean
  interrupting: boolean
  retryText: string
}>()

const emit = defineEmits<{
  'update:messageText': [value: string]
  sendMessage: []
  selectModel: [profileID: string]
  pickMaterialArchive: []
  pickMaterialDirectory: []
  retry: []
  interrupt: []
}>()

const selectedModelLabel = computed(() => profileDisplayName(props.profiles.find((profile) => profile.profile_id === props.selectedModelProfile)))
const turnStatus = computed(() => {
  if (props.lastTurn?.error) return turnErrorLabel([props.lastTurn.error.code, props.lastTurn.error.message].filter(Boolean).join(': '))
  if (props.lastTurn?.pause) return String(props.lastTurn.pause.reason ?? props.lastTurn.pause.wait_reason ?? '回合已暂停')
  return ''
})

function onKeydown(rawEvent: Event | KeyboardEvent) {
  const event = rawEvent as KeyboardEvent
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return
  event.preventDefault()
  emit('sendMessage')
}
</script>

<template>
  <section class="task-composer" aria-label="任务输入">
    <div v-if="error" class="composer-recovery" role="alert">
      <strong>这项操作没有完成</strong>
      <p>{{ error }}</p>
      <el-button text @click="$emit('retry')">{{ retryText ? '重试这次任务' : '重新连接' }}</el-button>
    </div>
    <p v-if="turnStatus" class="composer-turn-status">{{ turnStatus }}</p>
    <div class="task-composer-card">
      <el-input
        :model-value="messageText"
        type="textarea"
        :autosize="{ minRows: 3, maxRows: 8 }"
        placeholder="描述你希望 Genesis 完成的任务…"
        spellcheck="true"
        @keydown="onKeydown"
        @update:model-value="$emit('update:messageText', String($event))"
      />
      <div class="task-composer-footer">
        <div class="composer-utility-actions">
          <el-button text @click="$emit('pickMaterialArchive')"><el-icon><Paperclip /></el-icon>附件</el-button>
          <el-button text @click="$emit('pickMaterialDirectory')"><el-icon><FolderOpened /></el-icon>文件夹</el-button>
        </div>
        <div class="composer-submit-actions">
          <el-select
            class="composer-model-select"
            :model-value="selectedModelProfile"
            :disabled="modelSelectionDisabled || !profiles.length"
            :placeholder="selectedModelLabel || '选择模型'"
            filterable
            @update:model-value="$emit('selectModel', String($event || ''))"
          >
            <el-option v-for="profile in profiles" :key="profile.profile_id" :value="String(profile.profile_id || '')" :label="String(profile.model_id || profile.profile_id || '')" :disabled="!profile.credential_present && !isLocalProfile(profile)" />
          </el-select>
          <el-button v-if="interruptAvailable" plain :loading="interrupting" :disabled="interrupting" @click="$emit('interrupt')">{{ interrupting ? '正在停止…' : '停止' }}</el-button>
          <el-button v-else type="primary" circle :disabled="!selectedModelProfile || modelSelectionDisabled" aria-label="发送任务" @click="$emit('sendMessage')"><el-icon><Promotion /></el-icon></el-button>
        </div>
      </div>
    </div>
    <p v-if="selectedFileName" class="composer-selection">已选择{{ selectedFileIsDirectory ? '文件夹' : '附件' }}：{{ selectedFileName }}</p>
  </section>
</template>
