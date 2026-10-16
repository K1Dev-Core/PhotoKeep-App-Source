<script setup lang="ts">
import { ArrowRight, HardDriveDownload, Image as ImageIcon, Images, RefreshCw, Search, Tag, X } from '@lucide/vue'
import type { Photo, TagItem } from '../types/photo'

const props = defineProps<{ configured: boolean; photos: Photo[]; total: number; tags: TagItem[]; thumbnails: Record<number, string>; thumbnailFailures: Record<number, boolean>; searching: boolean; busy: boolean; backupMode: 'backup' | 'refresh' | null }>()
const query = defineModel<string>('query', { required: true })
const activeTag = defineModel<string>('activeTag', { required: true })
const emit = defineEmits<{ openSetup: []; backup: [mode: 'backup' | 'refresh']; openPhoto: [photo: Photo] }>()
const formatDate = (value: string) => value ? new Intl.DateTimeFormat('th-TH', { dateStyle: 'medium' }).format(new Date(value)) : '—'
function chooseTag(tag: string) { activeTag.value = activeTag.value === tag ? '' : tag }
</script>

<template>
  <section class="gallery-view">
    <div v-if="!configured" class="start-panel">
      <div class="start-icon"><Images :size="31" :stroke-width="1.6" /></div>
      <h1>เริ่มสำรองรูปภาพ</h1>
      <p>เลือกโฟลเดอร์รูปภาพและที่เก็บสำเนา<br />แล้วเริ่มใช้งานได้ทันที</p>
      <button class="primary" type="button" @click="emit('openSetup')">เลือกโฟลเดอร์ <ArrowRight :size="17" /></button>
    </div>
    <template v-else>
      <div class="dashboard-top">
        <section class="featured-panel" aria-label="คำสั่งสำรองรูปภาพ">
          <div class="featured-actions">
            <button class="primary backup-button" type="button" :disabled="busy" @click="emit('backup', 'backup')"><span v-if="backupMode === 'backup'" class="loading-spinner light" aria-hidden="true"></span><HardDriveDownload v-else :size="18" :stroke-width="1.9" />{{ backupMode === 'backup' ? 'กำลังสำรอง…' : 'สำรองรูป' }}</button>
            <button class="secondary refresh-button" type="button" :disabled="busy" @click="emit('backup', 'refresh')"><span v-if="backupMode === 'refresh'" class="loading-spinner" aria-hidden="true"></span><RefreshCw v-else :size="19" :stroke-width="1.9" />{{ backupMode === 'refresh' ? 'กำลังรีเฟรช…' : 'รีเฟรช' }}</button>
          </div>
        </section>
      </div>

      <section class="library-section" aria-labelledby="library-title">
        <div class="library-heading">
          <div><span class="section-label">คลังภาพ</span><h2 id="library-title">รูปภาพของคุณ</h2></div>
          <span class="library-count">{{ total }} รูป</span>
        </div>
        <div class="library-tools">
          <div class="search-field">
            <Search :size="18" :stroke-width="1.8" />
            <input v-model="query" aria-label="ค้นหารูปภาพ" placeholder="ค้นหาชื่อหรือคำอธิบายภาพ" />
            <button v-if="query" class="clear-button" type="button" aria-label="ล้างคำค้นหา" @click="query = ''"><X :size="17" /></button>
          </div>
          <div v-if="tags.length" class="tag-cloud" aria-label="กรองตามแท็ก">
            <Tag :size="16" :stroke-width="1.8" />
            <button type="button" :class="{ active: !activeTag }" @click="activeTag = ''">ทุกแท็ก</button>
            <button v-for="tag in tags" :key="tag.name" type="button" :class="{ active: activeTag === tag.name }" @click="chooseTag(tag.name)">{{ tag.name }}</button>
          </div>
        </div>
        <div class="results-heading">
          <span>{{ query || activeTag ? 'ผลการค้นหา' : 'รูปทั้งหมด' }}</span>
          <span v-if="searching" class="search-status" role="status"><span class="loading-spinner" aria-hidden="true"></span>กำลังค้นหา…</span>
          <span v-else>{{ photos.length }} รูป</span>
        </div>
        <div v-if="photos.length" class="photo-grid" :aria-busy="searching">
          <button v-for="photo in photos" :key="photo.id" class="photo-card" :aria-label="`เปิด ${photo.filename}`" @click="emit('openPhoto', photo)">
            <div class="thumb">
              <img v-if="thumbnails[photo.id]" :src="thumbnails[photo.id]" :alt="photo.description || photo.filename" />
              <ImageIcon v-else-if="thumbnailFailures[photo.id]" :size="26" :stroke-width="1.4" />
              <div v-else class="loading-pulse thumb-loading" aria-hidden="true"></div>
            </div>
            <div class="photo-caption"><strong :title="photo.filename">{{ photo.filename }}</strong><span>{{ formatDate(photo.takenAt) }}</span></div>
          </button>
        </div>
        <div v-else-if="searching" class="loading-grid search-loading" role="status" aria-label="กำลังค้นหารูปภาพ">
          <div v-for="item in 3" :key="item" class="loading-card"><div class="loading-pulse loading-image"></div><div class="loading-pulse loading-line loading-caption"></div></div>
          <span class="sr-only">กำลังค้นหารูปภาพ</span>
        </div>
        <div v-else class="empty">
          <ImageIcon :size="29" :stroke-width="1.5" />
          <h3>{{ query || activeTag ? 'ไม่พบรูปภาพ' : 'ยังไม่มีรูปภาพ' }}</h3>
          <p>{{ query || activeTag ? 'ลองเปลี่ยนคำค้นหาหรือแท็ก' : 'กด “สำรองรูป” เพื่อเพิ่มรูปภาพ' }}</p>
        </div>
      </section>
    </template>
  </section>
</template>
