<script setup lang="ts">
import { ArrowRight, X } from '@lucide/vue'
import FolderFields from './FolderFields.vue'

defineProps<{ canSave: boolean; saving: boolean }>()
const source = defineModel<string>('source', { required: true })
const dest = defineModel<string>('dest', { required: true })
const emit = defineEmits<{ pick: [which: 'source' | 'dest']; close: []; save: [] }>()
</script>

<template>
  <div class="modal-scrim setup-scrim">
    <section class="setup-dialog" role="dialog" aria-modal="true" aria-labelledby="setup-title">
      <header class="setup-header"><div class="setup-kicker">เริ่มใช้งาน PhotoKeep</div><button class="icon-button" type="button" aria-label="ปิดการตั้งค่าเริ่มต้น" @click="emit('close')"><X :size="20" /></button></header>
      <div class="setup-body"><h2 id="setup-title">เลือกรูปต้นทางและที่เก็บสำเนา</h2><p>ตั้งค่าเพียงครั้งเดียว เปลี่ยนภายหลังได้ที่เมนูตั้งค่า</p><FolderFields id-prefix="setup" v-model:source="source" v-model:dest="dest" compact @pick="emit('pick', $event)" /></div>
      <footer class="setup-actions"><button class="secondary" type="button" :disabled="saving" @click="emit('close')">ไว้ก่อน</button><button class="primary" type="button" :disabled="!canSave" @click="emit('save')"><span v-if="saving" class="loading-spinner light" aria-hidden="true"></span><ArrowRight v-else :size="16" />{{ saving ? 'กำลังบันทึก…' : 'บันทึกและเริ่มใช้' }}</button></footer>
    </section>
  </div>
</template>
