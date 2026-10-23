<script setup lang="ts">
import { FolderCog, Images, ScrollText, Settings2 } from '@lucide/vue'
import type { View } from '../../types/photo'

defineProps<{ view: View; configured: boolean; folderSettingsBusy: boolean }>()
const emit = defineEmits<{ navigate: [view: View]; openFolderSettings: [] }>()
</script>

<template>
  <aside class="app-sidebar">
    <button class="brand" type="button" aria-label="PhotoKeep — รูปภาพ" @click="emit('navigate', 'gallery')">
      <span class="brand-symbol" aria-hidden="true"><Images :size="22" :stroke-width="1.9" /></span>
      <span class="brand-copy"><strong>PhotoKeep</strong><small>คลังรูปภาพ</small></span>
    </button>
    <nav class="side-nav" aria-label="เมนูหลัก">
      <span class="nav-label">คลังภาพ</span>
      <button type="button" :class="{ active: view === 'gallery' }" :aria-current="view === 'gallery' ? 'page' : undefined" @click="emit('navigate', 'gallery')"><Images :size="18" :stroke-width="1.8" /><span>รูปภาพ</span></button>
      <button type="button" :class="{ active: view === 'logs' }" :aria-current="view === 'logs' ? 'page' : undefined" @click="emit('navigate', 'logs')"><ScrollText :size="18" :stroke-width="1.8" /><span>บันทึกระบบ</span></button>
      <button type="button" :disabled="folderSettingsBusy" :title="folderSettingsBusy ? 'รอให้งานปัจจุบันเสร็จก่อน' : 'จัดการโฟลเดอร์ต้นทางและปลายทาง'" @click="emit('openFolderSettings')"><FolderCog :size="18" :stroke-width="1.8" /><span>โฟลเดอร์</span></button>
    </nav>
    <div class="sidebar-footer">
      <span class="sidebar-status" :class="{ ready: configured }" aria-hidden="true"></span>
      <span class="sidebar-status-copy">{{ configured ? 'พร้อมสำรอง' : 'รอตั้งค่า' }}</span>
      <button type="button" :class="{ active: view === 'settings' }" :aria-current="view === 'settings' ? 'page' : undefined" @click="emit('navigate', 'settings')"><Settings2 :size="18" :stroke-width="1.8" /><span>ตั้งค่า</span></button>
    </div>
  </aside>
</template>
