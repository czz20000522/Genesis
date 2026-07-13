<script setup lang="ts">
import { FolderOpened, MoreFilled } from '@element-plus/icons-vue'

defineProps<{
  title: string
  kindLabel: string
  workspaceRoot: string
  modelLabel: string
  readinessLabel: string
  inspectorOpen: boolean
}>()

defineEmits<{
  toggleInspector: []
}>()
</script>

<template>
  <header class="workspace-header">
    <div class="workspace-header-copy">
      <p class="workspace-kicker">{{ kindLabel }}</p>
      <h1>{{ title }}</h1>
      <div class="workspace-facts">
        <span v-if="workspaceRoot"><el-icon><FolderOpened /></el-icon>{{ workspaceRoot }}</span>
        <span v-if="modelLabel">{{ modelLabel }}</span>
        <span>{{ readinessLabel }}</span>
      </div>
    </div>
    <el-button text circle class="workspace-detail-toggle" :aria-label="inspectorOpen ? '收起详情' : '打开详情'" @click="$emit('toggleInspector')">
      <el-icon><MoreFilled /></el-icon>
    </el-button>
  </header>
</template>
