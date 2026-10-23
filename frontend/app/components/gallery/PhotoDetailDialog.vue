<script setup lang="ts">
import { Image as ImageIcon, Plus, Tag as TagIcon, X } from "@lucide/vue";
import { computed, ref } from "vue";
import type { Photo } from "../../types/photo";

defineProps<{
    photo: Photo;
    fullImage: string;
    previewLoading: boolean;
    previewFailed: boolean;
    busy: boolean;
    saving: boolean;
}>();
const description = defineModel<string>("description", { required: true });
const tags = defineModel<string>("tags", { required: true });
const emit = defineEmits<{ close: []; save: [] }>();
const newTag = ref("");
const tagItems = computed(() => [
    ...new Set(
        tags.value
            .split(",")
            .map((tag) => tag.trim())
            .filter(Boolean),
    ),
]);
const formatDate = (value: string) =>
    value
        ? new Intl.DateTimeFormat("th-TH", { dateStyle: "medium" }).format(
              new Date(value),
          )
        : "—";
const sizeText = (size: number) =>
    size < 1048576
        ? `${Math.round(size / 1024)} KB`
        : `${(size / 1048576).toFixed(1)} MB`;
function addTag() {
    const additions = newTag.value
        .split(",")
        .map((tag) => tag.trim().replace(/^#/, "").trim())
        .filter(Boolean);
    if (!additions.length) return;
    const next = [...tagItems.value];
    for (const value of additions) {
        if (
            !next.some(
                (tag) => tag.toLocaleLowerCase() === value.toLocaleLowerCase(),
            )
        )
            next.push(value);
    }
    tags.value = next.join(",");
    newTag.value = "";
}
function removeTag(value: string) {
    tags.value = tagItems.value.filter((tag) => tag !== value).join(",");
}
function handleTagKeydown(event: KeyboardEvent) {
    if (event.key === "Enter" || event.key === ",") {
        event.preventDefault();
        addTag();
    }
    if (event.key === "Backspace" && !newTag.value && tagItems.value.length)
        removeTag(tagItems.value[tagItems.value.length - 1]!);
}
</script>

<template>
    <div class="modal-scrim photo-detail-scrim" @click.self="emit('close')">
        <section
            class="detail"
            role="dialog"
            aria-modal="true"
            :aria-label="photo.filename"
        >
            <header class="detail-header">
                <div>
                    <span>รายละเอียดรูปภาพ</span>
                    <h2 :title="photo.filename">{{ photo.filename }}</h2>
                </div>
                <button
                    class="icon-button"
                    aria-label="ปิดรายละเอียด"
                    @click="emit('close')"
                >
                    <X :size="21" />
                </button>
            </header>
            <div class="detail-layout">
                <div class="detail-preview">
                    <img
                        v-if="fullImage"
                        :src="fullImage"
                        :alt="photo.description || photo.filename"
                        decoding="async"
                    />
                    <div
                        v-if="previewLoading"
                        class="preview-status"
                        :class="{ 'preview-loading-overlay': fullImage }"
                        role="status"
                    >
                        <span class="loading-spinner" aria-hidden="true"></span
                        ><span>{{
                            fullImage
                                ? "กำลังโหลดภาพความละเอียดเต็ม…"
                                : "กำลังโหลดภาพ…"
                        }}</span>
                    </div>
                    <div v-else-if="!fullImage" class="preview-status">
                        <ImageIcon :size="28" :stroke-width="1.3" /><span>{{
                            previewFailed ? "เปิดภาพไม่ได้" : "ไม่มีภาพตัวอย่าง"
                        }}</span>
                    </div>
                </div>
                <div class="detail-fields">
                    <div class="photo-facts">
                        <span
                            ><small>ขนาดภาพ</small>{{ photo.width }} ×
                            {{ photo.height }} px</span
                        ><span
                            ><small>ขนาดไฟล์</small
                            >{{ sizeText(photo.size) }}</span
                        ><span
                            ><small>วันที่</small
                            >{{ formatDate(photo.takenAt) }}</span
                        >
                    </div>
                    <label for="description">คำอธิบาย</label
                    ><textarea
                        id="description"
                        v-model="description"
                        rows="4"
                        maxlength="500"
                        placeholder="เพิ่มคำอธิบายภาพ"
                    ></textarea
                    ><label for="tags"
                        >แท็ก <small>กด Enter เพื่อเพิ่ม</small></label
                    >
                    <div class="tag-editor">
                        <div
                            v-if="tagItems.length"
                            class="tag-list"
                            aria-label="แท็กของภาพ"
                        >
                            <span
                                v-for="tag in tagItems"
                                :key="tag"
                                class="photo-tag"
                                ><TagIcon
                                    :size="12"
                                    aria-hidden="true" /><span>{{ tag }}</span
                                ><button
                                    type="button"
                                    :aria-label="`ลบแท็ก ${tag}`"
                                    @click="removeTag(tag)"
                                >
                                    <X :size="12" /></button
                            ></span>
                        </div>
                        <div v-else class="tag-empty">ยังไม่มีแท็ก</div>
                        <div class="tag-input-row">
                            <input
                                id="tags"
                                v-model="newTag"
                                aria-label="เพิ่มแท็ก"
                                placeholder="พิมพ์แท็กแล้วกด Enter"
                                @keydown="handleTagKeydown"
                            /><button
                                type="button"
                                class="tag-add-button"
                                :disabled="!newTag.trim()"
                                aria-label="เพิ่มแท็ก"
                                @click="addTag"
                            >
                                <Plus :size="16" />เพิ่ม
                            </button>
                        </div>
                    </div>
                    <button
                        class="primary save-button"
                        :disabled="busy"
                        @click="emit('save')"
                    >
                        <span
                            v-if="saving"
                            class="loading-spinner light"
                            aria-hidden="true"
                        ></span
                        >{{ saving ? "กำลังบันทึก…" : "บันทึกรายละเอียด" }}
                    </button>
                </div>
            </div>
        </section>
    </div>
</template>

<style scoped>
.photo-facts > span {
    display: grid;
    gap: 3px;
}
.photo-facts small {
    color: var(--muted);
    font-size: 10px;
}
.tag-editor {
    display: grid;
    gap: 8px;
}
.tag-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    min-height: 25px;
}
.photo-tag {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    max-width: 100%;
    min-height: 27px;
    padding: 3px 6px 3px 8px;
    border: 1px solid var(--border);
    background: var(--green-soft);
    color: var(--ink);
    font-size: 11px;
}
.photo-tag > span {
    overflow: hidden;
    text-overflow: ellipsis;
}
.photo-tag button {
    display: grid;
    width: 18px;
    height: 18px;
    place-items: center;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--muted);
    cursor: pointer;
}
.photo-tag button:hover {
    background: var(--surface);
    color: var(--ink);
}
.tag-input-row {
    display: flex;
}
.tag-input-row input {
    flex: 1;
    min-width: 0;
}
.tag-add-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    min-width: 65px;
    border: 1px solid var(--border);
    border-left: 0;
    background: var(--green-soft);
    color: var(--ink);
    font-size: 12px;
    cursor: pointer;
}
.tag-add-button:disabled {
    opacity: 0.5;
    cursor: default;
}
.tag-empty {
    align-self: center;
    color: var(--muted);
    font-size: 11px;
}
</style>
