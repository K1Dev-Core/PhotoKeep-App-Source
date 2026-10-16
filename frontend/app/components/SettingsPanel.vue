<script setup lang="ts">
import { ArrowRight, Trash2 } from '@lucide/vue'
import FolderFields from './FolderFields.vue'

defineProps<{ canSave: boolean; saving: boolean; clearArmed: boolean; clearing: boolean }>()
const source = defineModel<string>('source', { required: true })
const dest = defineModel<string>('dest', { required: true })
const emit = defineEmits<{ pick: [which: 'source' | 'dest']; save: []; clear: [] }>()
</script>

<template>
  <section class="settings-view">
    <div class="page-heading"><div><div class="eyebrow">การตั้งค่า</div><h1>โฟลเดอร์สำรอง</h1><p>เลือกต้นทางและปลายทางสำหรับรูปที่จะสำรอง</p></div></div>
    <FolderFields v-model:source="source" v-model:dest="dest" @pick="emit('pick', $event)" />
    <p class="settings-note">เปลี่ยนโฟลเดอร์แล้ว รูปที่เคยสำรองยังดูได้ในแท็บ “ทั้งหมด”</p>
    <div class="settings-actions"><button class="primary" :disabled="!canSave" @click="emit('save')"><span v-if="saving" class="loading-spinner light" aria-hidden="true"></span><ArrowRight v-else :size="16" />{{ saving ? 'กำลังบันทึก…' : 'บันทึกการตั้งค่า' }}</button></div>
    <div class="danger-zone">
      <h2>โซนอันตราย</h2>
      <p>ลบรูปทั้งหมดออกจากฐานข้อมูล (ไฟล์ต้นฉบับไม่ถูกลบ)</p>
      <button class="danger" :class="{ armed: clearArmed }" :disabled="clearing" @click="emit('clear')"><span v-if="clearing" class="loading-spinner light" aria-hidden="true"></span><Trash2 v-else :size="16" />{{ clearing ? 'กำลังล้าง…' : clearArmed ? 'กดอีกครั้งเพื่อยืนยัน' : 'ล้างข้อมูลทั้งหมด' }}</button>
    </div>
  </section>
</template>

<style scoped>
.danger-zone {
  margin-top: 24px;
  padding: 16px;
  border: 1.5px dashed #dc2626;
  border-radius: 12px;
}
.danger-zone h2 {
  font-size: 15px;
  color: #dc2626;
  margin-bottom: 4px;
}
.danger-zone p {
  font-size: 13px;
  color: #5f7062;
  margin-bottom: 12px;
}
.danger-zone .danger {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: #fff;
  color: #dc2626;
  border: 1.5px solid #dc2626;
  border-radius: 10px;
  padding: 9px 16px;
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
}
.danger-zone .danger.armed {
  background: #dc2626;
  color: #fff;
}
.danger-zone .danger:disabled {
  opacity: 0.6;
  cursor: wait;
}
</style>
