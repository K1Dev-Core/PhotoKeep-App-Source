<script setup lang="ts">
import { computed } from 'vue'
import { HardDriveDownload } from '@lucide/vue'
import type { BackupProgress } from '../types/photo'

const props = defineProps<{ progress: BackupProgress; elapsedMs: number; errorMessage: string; mode: 'backup' | 'refresh' }>()
const emit = defineEmits<{ close: [] }>()
const radius = 72
const circumference = 2 * Math.PI * radius
const percent = computed(() => props.progress.stage === 'done' ? 100 : props.progress.total ? Math.round(props.progress.completed / props.progress.total * 100) : 0)
const offset = computed(() => circumference * (1 - percent.value / 100))
const seconds = computed(() => (props.elapsedMs / 1000).toFixed(props.elapsedMs < 1000 ? 2 : 1))
const finished = computed(() => props.progress.stage === 'done' || props.progress.stage === 'error')
const stageText = computed(() => {
  if (props.progress.stage === 'scanning') return 'กำลังตรวจหารูปภาพ'
  if (props.progress.stage === 'copying') return 'กำลังสำรองรูปภาพ'
  if (props.progress.stage === 'error') return 'สำรองไม่สำเร็จ'
  if (props.progress.failed) return 'สำรองเสร็จบางส่วน'
  if (props.mode === 'refresh') return props.progress.added ? 'อัปเดตคลังแล้ว' : 'ไม่มีรูปใหม่'
  return 'สำรองเสร็จแล้ว'
})
const description = computed(() => {
  if (props.progress.stage === 'scanning') return 'กำลังนับรูปภาพก่อนเริ่มสำรอง'
  if (props.progress.stage === 'error') return 'ตรวจสอบข้อความด้านล่างแล้วลองอีกครั้ง'
  if (props.progress.stage === 'done' && props.progress.failed) return 'บางไฟล์ไม่สำเร็จ ตรวจสอบรายละเอียดด้านล่าง'
  if (props.progress.stage === 'done' && props.mode === 'refresh' && !props.progress.added) return 'คลังรูปเป็นปัจจุบันแล้ว'
  return 'รูปต้นฉบับจะยังอยู่ในโฟลเดอร์เดิม'
})
const fileText = computed(() => {
  if (props.progress.stage === 'scanning') return props.progress.currentFile || 'กำลังค้นหาไฟล์ในโฟลเดอร์ต้นทาง'
  if (props.progress.stage === 'error' || props.progress.failed) return props.errorMessage || 'ตรวจสอบโฟลเดอร์ต้นทางและปลายทาง'
  if (props.progress.stage === 'done') return 'ประมวลผลครบทุกไฟล์แล้ว'
  return props.progress.currentFile || 'กำลังเตรียมไฟล์'
})
</script>

<template>
  <div class="backup-overlay">
    <section class="backup-dialog" :class="{ 'backup-dialog-error': progress.stage === 'error' || progress.failed }" role="dialog" aria-modal="true" aria-labelledby="backup-progress-title" aria-describedby="backup-progress-description">
      <div class="backup-dialog-header">
        <div class="backup-dialog-icon"><HardDriveDownload :size="21" :stroke-width="1.8" /></div>
        <div><span class="backup-dialog-kicker">PHOTOKEEP BACKUP</span><h2 id="backup-progress-title" tabindex="-1">{{ stageText }}</h2></div>
      </div>
      <p id="backup-progress-description" class="backup-dialog-description">{{ description }}</p>

      <div class="backup-ring-wrap" role="status" :aria-label="progress.stage === 'scanning' ? `พบ ${progress.total} ไฟล์` : `เสร็จ ${progress.completed} จาก ${progress.total} ไฟล์`">
        <svg class="backup-ring" viewBox="0 0 176 176" aria-hidden="true">
          <circle class="backup-ring-track" cx="88" cy="88" :r="radius" />
          <circle v-if="progress.stage === 'scanning'" class="backup-ring-scanning" cx="88" cy="88" :r="radius" :stroke-dasharray="`100 ${circumference}`" />
          <circle v-else class="backup-ring-fill" :class="{ 'backup-ring-error': progress.stage === 'error' || progress.failed }" cx="88" cy="88" :r="radius" :stroke-dasharray="circumference" :stroke-dashoffset="offset" />
        </svg>
        <div class="backup-ring-center"><strong>{{ progress.stage === 'scanning' ? progress.total : `${percent}%` }}</strong><span>{{ progress.stage === 'scanning' ? 'ไฟล์ที่พบ' : `${progress.completed} / ${progress.total} ไฟล์` }}</span></div>
      </div>

      <div class="backup-stats" aria-live="polite">
        <div><strong>{{ progress.added }}</strong><span>สำเร็จ</span></div>
        <div><strong>{{ progress.skipped }}</strong><span>ข้าม</span></div>
        <div><strong>{{ progress.failed }}</strong><span>ผิดพลาด</span></div>
      </div>
      <div class="backup-current" :class="{ 'backup-current-error': progress.stage === 'error' || progress.failed }"><span>{{ progress.stage === 'scanning' ? 'กำลังตรวจ' : finished ? 'สถานะล่าสุด' : 'ไฟล์ปัจจุบัน' }}</span><strong :title="fileText">{{ fileText }}</strong></div>
      <div class="backup-dialog-footer"><span>เวลาที่ใช้</span><strong>{{ seconds }} วินาที</strong></div>
      <button v-if="finished" id="backup-finish-button" class="primary backup-finish-button" type="button" @click="emit('close')">ตกลง</button>
    </section>
  </div>
</template>
