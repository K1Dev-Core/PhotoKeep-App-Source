<script setup lang="ts">
import { Images, Settings2 } from '@lucide/vue'
import type { View } from '../types/photo'

defineProps<{ view: View; configured: boolean }>()
const emit = defineEmits<{ navigate: [view: View] }>()
</script>

<template>
  <header class="app-header">
    <div class="app-header-inner">
      <button class="brand" type="button" aria-label="ไปหน้ารูปภาพ" @click="emit('navigate', 'gallery')">
        <span class="brand-symbol" aria-hidden="true"><Images :size="22" :stroke-width="1.8" /></span>
        <span class="brand-copy"><strong>PhotoKeep</strong><small>คลังภาพส่วนตัว</small></span>
      </button>
      <div class="header-end">
        <span class="header-status"><span class="status-dot"></span>{{ configured ? 'พร้อมสำรอง' : 'รอตั้งค่า' }}</span>
        <nav class="header-nav" aria-label="เมนูหลัก">
          <button type="button" :class="{ active: view === 'gallery' }" :aria-current="view === 'gallery' ? 'page' : undefined" @click="emit('navigate', 'gallery')"><Images :size="17" :stroke-width="1.8" /><span>รูปภาพ</span></button>
          <button type="button" :class="{ active: view === 'settings' }" :aria-current="view === 'settings' ? 'page' : undefined" @click="emit('navigate', 'settings')"><Settings2 :size="17" :stroke-width="1.8" /><span>ตั้งค่า</span></button>
        </nav>
      </div>
    </div>
  </header>
</template>
