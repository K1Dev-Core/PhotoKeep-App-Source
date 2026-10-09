<script setup lang="ts">
import { Image as ImageIcon, X } from '@lucide/vue'
import type { Photo } from '../types/photo'

defineProps<{ photo: Photo; fullImage: string; previewLoading: boolean; previewFailed: boolean; busy: boolean; saving: boolean }>()
const description = defineModel<string>('description', { required: true })
const tags = defineModel<string>('tags', { required: true })
const emit = defineEmits<{ close: []; save: [] }>()
const formatDate = (value: string) => value ? new Intl.DateTimeFormat('th-TH', { dateStyle: 'medium' }).format(new Date(value)) : '—'
const sizeText = (size: number) => size < 1048576 ? `${Math.round(size / 1024)} KB` : `${(size / 1048576).toFixed(1)} MB`
</script>

<template>
  <div class="modal-scrim" @click.self="emit('close')">
    <section class="detail" role="dialog" aria-modal="true" :aria-label="photo.filename">
      <header class="detail-header"><div><span>รายละเอียดรูปภาพ</span><h2 :title="photo.filename">{{ photo.filename }}</h2></div><button class="icon-button" aria-label="ปิดรายละเอียด" @click="emit('close')"><X :size="21" /></button></header>
      <div class="detail-layout"><div class="detail-preview"><div v-if="previewLoading" class="preview-status" role="status"><span class="loading-spinner" aria-hidden="true"></span><span>กำลังโหลดภาพ…</span></div><img v-else-if="fullImage" :src="fullImage" :alt="photo.description || photo.filename" /><div v-else class="preview-status"><ImageIcon :size="28" :stroke-width="1.3" /><span>{{ previewFailed ? 'เปิดภาพไม่ได้' : 'ไม่มีภาพตัวอย่าง' }}</span></div></div><div class="detail-fields"><div class="photo-facts"><span>{{ photo.width }} × {{ photo.height }}</span><span>{{ sizeText(photo.size) }}</span><span>{{ formatDate(photo.takenAt) }}</span></div><label for="description">คำอธิบาย</label><textarea id="description" v-model="description" rows="4" maxlength="500" placeholder="เพิ่มคำอธิบายภาพ"></textarea><label for="tags">แท็ก <small>คั่นด้วย ,</small></label><input id="tags" v-model="tags" placeholder="เช่น ทะเล, ครอบครัว" /><button class="primary save-button" :disabled="busy" @click="emit('save')"><span v-if="saving" class="loading-spinner light" aria-hidden="true"></span>{{ saving ? 'กำลังบันทึก…' : 'บันทึกรายละเอียด' }}</button></div></div>
    </section>
  </div>
</template>
