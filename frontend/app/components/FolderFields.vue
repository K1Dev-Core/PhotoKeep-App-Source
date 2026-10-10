<script setup lang="ts">
import { FolderOpen, HardDriveDownload, Image as ImageIcon } from '@lucide/vue'

withDefaults(defineProps<{ idPrefix?: string; compact?: boolean }>(), { idPrefix: 'settings', compact: false })
const source = defineModel<string>('source', { required: true })
const dest = defineModel<string>('dest', { required: true })
const emit = defineEmits<{ pick: [which: 'source' | 'dest'] }>()
</script>

<template>
  <div class="settings-card" :class="{ compact }">
    <div class="setting-row">
      <div class="setting-icon"><ImageIcon :size="20" :stroke-width="1.8" /></div>
      <div class="setting-content">
        <label :for="`${idPrefix}-source`">รูปต้นทาง</label>
        <p>แฟลชไดรฟ์หรือโฟลเดอร์รูปภาพ</p>
        <div class="path-control"><input :id="`${idPrefix}-source`" v-model="source" placeholder="เลือกโฟลเดอร์รูปภาพ" /><button class="icon-button" type="button" aria-label="เลือกโฟลเดอร์ต้นทาง" title="เลือกโฟลเดอร์" @click="emit('pick', 'source')"><FolderOpen :size="19" :stroke-width="1.8" /></button></div>
      </div>
    </div>
    <div class="setting-row">
      <div class="setting-icon"><HardDriveDownload :size="20" :stroke-width="1.8" /></div>
      <div class="setting-content">
        <label :for="`${idPrefix}-dest`">เก็บสำเนาที่</label>
        <p>รูปต้นฉบับจะยังอยู่ที่เดิม</p>
        <div class="path-control"><input :id="`${idPrefix}-dest`" v-model="dest" placeholder="เลือกโฟลเดอร์ปลายทาง" /><button class="icon-button" type="button" aria-label="เลือกโฟลเดอร์ปลายทาง" title="เลือกโฟลเดอร์" @click="emit('pick', 'dest')"><FolderOpen :size="19" :stroke-width="1.8" /></button></div>
      </div>
    </div>
  </div>
</template>
