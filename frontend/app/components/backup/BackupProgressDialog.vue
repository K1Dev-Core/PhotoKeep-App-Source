<script setup lang="ts">
import { computed } from "vue";
import { Check, FolderOpen, HardDriveDownload, X } from "@lucide/vue";
import type { BackupProgress } from "../../types/photo";

const props = defineProps<{
    progress: BackupProgress;
    elapsedMs: number;
    errorMessage: string;
    mode: "backup" | "refresh";
    sourcePath: string;
    destinationPath: string;
    keepOriginal: boolean;
    folderSettingsBusy: boolean;
}>();
const emit = defineEmits<{ close: []; openBackupFolder: [] }>();
const percent = computed(() =>
    props.progress.stage === "done"
        ? 100
        : props.progress.total
          ? Math.round((props.progress.completed / props.progress.total) * 100)
          : 0,
);
const seconds = computed(() =>
    (props.elapsedMs / 1000).toFixed(props.elapsedMs < 1000 ? 2 : 1),
);
const finished = computed(
    () => props.progress.stage === "done" || props.progress.stage === "error",
);
const stageText = computed(() => {
    if (props.progress.stage === "scanning") return "กำลังตรวจหารูปภาพ";
    if (props.progress.stage === "copying")
        return props.keepOriginal ? "กำลังสำรองรูปภาพ" : "กำลังย้ายรูปภาพ";
    if (props.progress.stage === "error") return "สำรองไม่สำเร็จ";
    if (props.progress.failed) return "สำรองเสร็จบางส่วน";
    if (props.mode === "refresh")
        return props.progress.added ? "อัปเดตคลังแล้ว" : "ไม่มีรูปใหม่";
    return props.keepOriginal ? "สำรองเสร็จแล้ว" : "ย้ายรูปเสร็จแล้ว";
});
const fileText = computed(() => {
    if (props.progress.stage === "scanning")
        return props.progress.currentFile || "กำลังค้นหาไฟล์ในโฟลเดอร์ต้นทาง";
    if (props.progress.stage === "error" || props.progress.failed)
        return props.errorMessage || "ตรวจสอบโฟลเดอร์ต้นทางและปลายทาง";
    if (props.progress.stage === "done") return "ประมวลผลครบทุกไฟล์แล้ว";
    return props.progress.currentFile || "กำลังเตรียมไฟล์";
});
</script>

<template>
    <div class="backup-overlay backup-progress-floating">
        <section
            class="backup-dialog backup-strip"
            :class="{
                'backup-dialog-error':
                    progress.stage === 'error' || progress.failed,
            }"
            role="status"
            aria-live="polite"
            aria-label="สถานะการสำรองรูปภาพ"
        >
            <div class="backup-strip-main">
                <span class="backup-strip-icon" aria-hidden="true"
                    ><HardDriveDownload :size="19" :stroke-width="1.8"
                /></span>
                <div class="backup-strip-copy">
                    <strong>{{ stageText }}</strong>
                    <small :title="fileText">{{ fileText }}</small>
                </div>
                <div class="backup-strip-count">
                    <strong>{{
                        progress.stage === "scanning"
                            ? progress.total.toLocaleString("th-TH")
                            : `${progress.completed.toLocaleString("th-TH")} / ${progress.total.toLocaleString("th-TH")}`
                    }}</strong
                    ><span>{{
                        progress.stage === "scanning"
                            ? "รูปที่พบ"
                            : `${percent}%`
                    }}</span>
                </div>
                <button
                    class="backup-strip-folder"
                    type="button"
                    :disabled="folderSettingsBusy || !destinationPath"
                    aria-label="เปิดโฟลเดอร์ที่สำรองรูปภาพ"
                    :title="
                        folderSettingsBusy
                            ? 'รอให้งานปัจจุบันเสร็จก่อน'
                            : 'เปิดโฟลเดอร์สำรองใน Finder'
                    "
                    @click="emit('openBackupFolder')"
                >
                    <FolderOpen :size="17" :stroke-width="1.8" />
                </button>
                <button
                    v-if="finished"
                    class="backup-strip-close"
                    type="button"
                    aria-label="ปิดสถานะการสำรอง"
                    @click="emit('close')"
                >
                    <X :size="17" />
                </button>
            </div>
            <progress
                class="backup-inline-progress"
                :value="progress.stage === 'scanning' ? undefined : percent"
                max="100"
                :aria-label="
                    progress.stage === 'scanning'
                        ? 'กำลังตรวจหารูปภาพ'
                        : `สำรองแล้ว ${percent}%`
                "
            ></progress>
            <div class="backup-strip-bottom">
                <ol class="backup-strip-steps" aria-label="ขั้นตอนการสำรอง">
                    <li
                        :class="{
                            active: progress.stage === 'scanning',
                            complete: progress.stage !== 'scanning',
                        }"
                    >
                        <Check
                            v-if="progress.stage !== 'scanning'"
                            :size="12"
                        /><span v-else>1</span>ตรวจหารูป
                    </li>
                    <li
                        :class="{
                            active: progress.stage === 'copying',
                            complete: progress.stage === 'done',
                        }"
                    >
                        <Check
                            v-if="progress.stage === 'done'"
                            :size="12"
                        /><span v-else>2</span
                        >{{ keepOriginal ? "คัดลอกไฟล์" : "ย้ายไฟล์" }}
                    </li>
                    <li :class="{ active: progress.stage === 'done' }">
                        <Check
                            v-if="progress.stage === 'done'"
                            :size="12"
                        /><span v-else>3</span>เสร็จสิ้น
                    </li>
                </ol>
                <span class="backup-strip-meta"
                    ><template v-if="progress.skipped"
                        >ข้าม {{ progress.skipped }} · </template
                    ><template v-if="progress.failed"
                        >ผิดพลาด {{ progress.failed }} · </template
                    >{{ seconds }} วินาที</span
                >
            </div>
        </section>
    </div>
</template>
