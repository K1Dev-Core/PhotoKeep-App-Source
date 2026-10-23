<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, Check, Clock3, RefreshCw, ScrollText } from '@lucide/vue'
import type { ImageAIAnalysisJob, ImageAIErrorLog } from '../../types/photo'

const props = defineProps<{ logs: ImageAIErrorLog[]; jobs: ImageAIAnalysisJob[]; loading: boolean; retrying: boolean }>()
const emit = defineEmits<{ refresh: []; retry: [photoIds: number[]] }>()
const failedCaptionIds = computed(() => [...new Set(props.logs.filter(log => log.kind === 'คำอธิบายภาพ').map(log => log.photoId))])
const formatDate = (value: string) => value ? new Intl.DateTimeFormat('th-TH', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '—'
function formatDuration(seconds: number) {
  const total = Math.max(0, Math.floor(seconds || 0))
  return `${Math.floor(total / 60)} นาที ${total % 60} วินาที`
}
function jobStatus(status: string) {
  if (status === 'completed') return 'เสร็จแล้ว'
  if (status === 'paused') return 'หยุดชั่วคราว'
  if (status === 'interrupted') return 'หยุดเมื่อปิดแอป'
  if (status === 'failed') return 'ผิดพลาด'
  return 'กำลังทำงาน'
}
</script>

<template>
  <section class="ai-logs-page">
    <header class="logs-heading">
      <div><span class="logs-eyebrow">PHOTOKEEP ACTIVITY</span><h1>บันทึกระบบ</h1><p>ดูประวัติคิววิเคราะห์และข้อผิดพลาดของภาพ</p></div>
      <button class="quiet-button logs-refresh" type="button" :disabled="loading" @click="emit('refresh')"><RefreshCw :size="16" :class="{ 'logs-spinning': loading }" />รีเฟรช</button>
    </header>

    <section class="logs-section" aria-labelledby="analysis-history-title">
      <div class="logs-section-heading"><div><h2 id="analysis-history-title">ประวัติวิเคราะห์</h2><span>{{ jobs.length }} รายการล่าสุด</span></div></div>
      <div v-if="jobs.length" class="job-log-list">
        <article v-for="job in jobs" :key="job.id" class="job-log-row">
          <span class="job-log-icon" :class="`job-${job.status}`"><Check v-if="job.status === 'completed'" :size="16" /><AlertTriangle v-else-if="job.failed || job.status === 'failed'" :size="16" /><Clock3 v-else :size="16" /></span>
          <div class="job-log-main"><strong>วิเคราะห์ {{ job.total.toLocaleString('th-TH') }} รูป</strong><small>{{ formatDate(job.createdAt) }}<span v-if="job.error"> · {{ job.error.split('\n')[0] }}</span></small></div>
          <div class="job-log-result"><strong>{{ job.completed }}/{{ job.total }}</strong><small>{{ jobStatus(job.status) }}<span v-if="job.durationSeconds > 0"> · ใช้เวลา {{ formatDuration(job.durationSeconds) }}</span><span v-if="job.failed"> · ผิดพลาด {{ job.failed }}</span></small></div>
        </article>
      </div>
      <div v-else class="logs-empty"><ScrollText :size="22" /><span>{{ loading ? 'กำลังโหลดบันทึก…' : 'ยังไม่มีประวัติการวิเคราะห์' }}</span></div>
    </section>

    <section class="logs-section error-logs-section" aria-labelledby="analysis-errors-title">
      <div class="logs-section-heading"><div><h2 id="analysis-errors-title">ข้อผิดพลาด</h2><span>{{ logs.length }} รายการ</span></div><button v-if="failedCaptionIds.length" class="quiet-button logs-retry" type="button" :disabled="retrying || loading" @click="emit('retry', failedCaptionIds)"><RefreshCw :size="14" :class="{ 'logs-spinning': retrying }" />{{ retrying ? 'กำลังเข้าคิว…' : `ลองใหม่ ${failedCaptionIds.length} รูป` }}</button></div>
      <div v-if="logs.length" class="error-log-list">
        <article v-for="log in logs" :key="`${log.photoId}-${log.kind}`" class="error-log-row">
          <AlertTriangle :size="17" />
          <div><strong :title="log.filename">{{ log.filename }}</strong><small class="error-log-kind">{{ log.kind || 'วิเคราะห์ภาพ' }}</small><p>{{ log.error }}</p><small>{{ formatDate(log.loggedAt) }}</small></div>
        </article>
      </div>
      <div v-else class="logs-empty"><Check :size="22" /><span>{{ loading ? 'กำลังโหลดบันทึก…' : 'ไม่มีข้อผิดพลาดของภาพ' }}</span></div>
    </section>
  </section>
</template>

<style scoped>
.ai-logs-page { width: min(100%, 920px); margin: 0 auto; color: #252923; }
.logs-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 18px; padding: 6px 0 23px; border-bottom: 1px solid #e7e9e5; }
.logs-eyebrow { color: #78837a; font-size: 10px; font-weight: 700; letter-spacing: .08em; }
.logs-heading h1 { margin: 6px 0 4px; font-size: 23px; font-weight: 680; }
.logs-heading p { margin: 0; color: #737a73; font-size: 13px; }
.logs-refresh { display: inline-flex; align-items: center; gap: 7px; min-height: 34px; padding: 0 10px; border: 1px solid #d9ded9; background: #fff; font-size: 12px; }
.logs-section { margin-top: 24px; border: 1px solid #e1e5e0; background: #fff; }
.logs-section-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 14px 16px; border-bottom: 1px solid #e8ebe7; }
.logs-retry { display: inline-flex; align-items: center; gap: 6px; min-height: 30px; padding: 0 9px; border: 1px solid #d9ded9; background: #fff; color: #42654b; font-size: 11px; white-space: nowrap; }
.logs-section-heading > div { display: flex; align-items: baseline; gap: 9px; }
.logs-section-heading h2 { margin: 0; font-size: 14px; font-weight: 680; }
.logs-section-heading span { color: #7b837b; font-size: 11px; }
.job-log-row { display: flex; align-items: center; gap: 11px; min-height: 64px; padding: 10px 16px; border-bottom: 1px solid #eff1ee; }
.job-log-row:last-child, .error-log-row:last-child { border-bottom: 0; }
.job-log-icon { display: grid; width: 30px; height: 30px; flex: none; place-items: center; border: 1px solid #d9e3da; color: #557b5e; }
.job-log-icon.job-failed { border-color: #ead9d6; background: #fbf4f3; color: #a3473d; }
.job-log-icon.job-paused, .job-log-icon.job-interrupted { color: #777b72; }
.job-log-main, .job-log-result { display: grid; gap: 3px; min-width: 0; }
.job-log-main { flex: 1; }
.job-log-main strong, .error-log-row strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; font-weight: 640; }
.job-log-main small, .job-log-result small { overflow: hidden; color: #788078; text-overflow: ellipsis; white-space: nowrap; font-size: 10px; }
.job-log-result { text-align: right; }
.job-log-result strong { font-size: 12px; }
.error-log-row { display: flex; align-items: flex-start; gap: 11px; padding: 13px 16px; border-bottom: 1px solid #eff1ee; color: #a3473d; }
.error-log-row > svg { flex: none; margin-top: 1px; }
.error-log-row > div { min-width: 0; }
.error-log-kind { display: inline-block; margin-top: 4px; color: #788078; font-size: 10px; }
.error-log-row p { margin: 5px 0; color: #555c55; font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }
.error-log-row small { color: #888e87; font-size: 10px; }
.logs-empty { display: flex; align-items: center; justify-content: center; gap: 9px; min-height: 104px; color: #778078; font-size: 12px; }
.logs-spinning { animation: logs-spin .9s linear infinite; }
@keyframes logs-spin { to { transform: rotate(360deg); } }
@media (max-width: 560px) { .logs-heading h1 { font-size: 20px; } .job-log-row, .error-log-row { padding-right: 11px; padding-left: 11px; } }
</style>
