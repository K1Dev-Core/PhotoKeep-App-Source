<script setup lang="ts">
import { ArrowRight, X } from "@lucide/vue";
import FolderFields from "../shared/FolderFields.vue";

withDefaults(
    defineProps<{
        canSave: boolean;
        saving: boolean;
        mode?: "setup" | "folder";
    }>(),
    { mode: "setup" },
);
const source = defineModel<string>("source", { required: true });
const dest = defineModel<string>("dest", { required: true });
const keepOriginal = defineModel<boolean>("keepOriginal", { required: true });
const emit = defineEmits<{
    pick: [which: "source" | "dest"];
    close: [];
    save: [];
}>();
</script>

<template>
    <div class="modal-scrim setup-scrim">
        <section
            class="setup-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="setup-title"
        >
            <header class="setup-header">
                <div class="setup-kicker">
                    {{
                        mode === "folder"
                            ? "โฟลเดอร์สำรอง"
                            : "เริ่มใช้งาน PhotoKeep"
                    }}
                </div>
                <button
                    class="icon-button"
                    type="button"
                    :aria-label="
                        mode === 'folder'
                            ? 'ปิดการตั้งค่าโฟลเดอร์'
                            : 'ปิดการตั้งค่าเริ่มต้น'
                    "
                    @click="emit('close')"
                >
                    <X :size="20" />
                </button>
            </header>
            <div class="setup-body">
                <h2 id="setup-title">เลือกรูปต้นทางและปลายทาง</h2>
                <p>
                    {{
                        mode === "folder"
                            ? "เลือกโฟลเดอร์และวิธีจัดการรูปต้นฉบับสำหรับครั้งถัดไป"
                            : "ตั้งค่าเพียงครั้งเดียว เปลี่ยนภายหลังได้ที่เมนูโฟลเดอร์"
                    }}
                </p>
                <FolderFields
                    id-prefix="setup"
                    v-model:source="source"
                    v-model:dest="dest"
                    v-model:keep-original="keepOriginal"
                    compact
                    @pick="emit('pick', $event)"
                />
            </div>
            <footer class="setup-actions">
                <button
                    class="secondary"
                    type="button"
                    :disabled="saving"
                    @click="emit('close')"
                >
                    {{ mode === "folder" ? "ยกเลิก" : "ไว้ก่อน" }}</button
                ><button
                    class="primary"
                    type="button"
                    :disabled="!canSave"
                    @click="emit('save')"
                >
                    <span
                        v-if="saving"
                        class="loading-spinner light"
                        aria-hidden="true"
                    ></span
                    ><ArrowRight v-else :size="16" />{{
                        saving
                            ? "กำลังบันทึก…"
                            : mode === "folder"
                              ? "บันทึกโฟลเดอร์"
                              : "บันทึกและเริ่มใช้"
                    }}
                </button>
            </footer>
        </section>
    </div>
</template>
