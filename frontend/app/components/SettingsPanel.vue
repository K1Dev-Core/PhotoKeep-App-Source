<script setup lang="ts">
import { ArrowRight } from '@lucide/vue'
import FolderFields from './FolderFields.vue'

defineProps<{ canSave: boolean; saving: boolean }>()
const source = defineModel<string>('source', { required: true })
const dest = defineModel<string>('dest', { required: true })
const emit = defineEmits<{ pick: [which: 'source' | 'dest']; save: [] }>()
</script>

<template>
  <section class="settings-view">
    <div class="page-heading"><div><div class="eyebrow">การตั้งค่า</div><h1>โฟลเดอร์สำรอง</h1><p>เลือกต้นทางและปลายทางสำหรับรูปที่จะสำรอง</p></div></div>
    <FolderFields v-model:source="source" v-model:dest="dest" @pick="emit('pick', $event)" />
    <p class="settings-note">เปลี่ยนโฟลเดอร์แล้ว รูปที่เคยสำรองยังดูได้ในแท็บ “ทั้งหมด”</p>
    <div class="settings-actions"><button class="primary" :disabled="!canSave" @click="emit('save')"><span v-if="saving" class="loading-spinner light" aria-hidden="true"></span><ArrowRight v-else :size="16" />{{ saving ? 'กำลังบันทึก…' : 'บันทึกการตั้งค่า' }}</button></div>
  </section>
</template>
