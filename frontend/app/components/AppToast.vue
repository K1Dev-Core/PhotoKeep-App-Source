<script setup lang="ts">
import { AlertCircle, Check, X } from '@lucide/vue'

type Toast = { id: number; type: 'success' | 'error'; message: string; duration: number }
defineProps<{ toast: Toast | null }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <div class="toast-viewport" aria-live="polite">
    <Transition name="toast-pop" mode="out-in">
      <section v-if="toast" :key="toast.id" class="app-toast" :class="`app-toast-${toast.type}`" :role="toast.type === 'error' ? 'alert' : 'status'">
        <span class="app-toast-icon" aria-hidden="true"><Check v-if="toast.type === 'success'" :size="18" :stroke-width="2.5" /><AlertCircle v-else :size="18" :stroke-width="2.2" /></span>
        <div class="app-toast-content"><strong>{{ toast.type === 'success' ? 'เรียบร้อย' : 'เกิดข้อผิดพลาด' }}</strong><p>{{ toast.message }}</p></div>
        <button type="button" class="app-toast-close" aria-label="ปิดการแจ้งเตือน" @click="emit('close')"><X :size="16" /></button>
        <span class="app-toast-time" :style="{ animationDuration: `${toast.duration}ms` }" aria-hidden="true"></span>
      </section>
    </Transition>
  </div>
</template>
