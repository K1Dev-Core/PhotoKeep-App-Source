<template>
  <section class="loading-state" role="status" aria-live="polite" aria-label="กำลังเปิดคลังภาพ">
    <div class="loading-state-heading">
      <div class="loading-pulse loading-line loading-line-short"></div>
      <div class="loading-pulse loading-line loading-line-title"></div>
      <div class="loading-pulse loading-line loading-line-copy"></div>
    </div>
    <div class="loading-pulse loading-search"></div>
    <div class="loading-grid" aria-hidden="true">
      <div v-for="item in 3" :key="item" class="loading-card">
        <div class="loading-pulse loading-image"></div>
        <div class="loading-pulse loading-line loading-caption"></div>
        <div class="loading-pulse loading-line loading-date"></div>
      </div>
    </div>
    <span class="sr-only">กำลังเปิดคลังภาพ</span>
  </section>
</template>
