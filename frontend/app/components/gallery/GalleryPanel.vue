<script setup lang="ts">
import {
    ArrowRight,
    Check,
    HardDriveDownload,
    Image as ImageIcon,
    Images,
    Plus,
    RefreshCw,
    Search,
    Sparkles,
    X,
} from "@lucide/vue";
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { Photo } from "../../types/photo";

const props = defineProps<{
    configured: boolean;
    photos: Photo[];
    resultTotal: number;
    startOffset: number;
    loadingMore: boolean;
    loadingPrevious: boolean;
    thumbnails: Record<number, string>;
    thumbnailFailures: Record<number, boolean>;
    searching: boolean;
    busy: boolean;
    backupActive: boolean;
    keepOriginal: boolean;
    backupMode: "backup" | "refresh" | null;
    selectionMode: boolean;
    selectedPhotoIds: number[];
    analysisBusy: boolean;
    analysisControlBusy: boolean;
    aiConfigured: boolean;
    aiProgress: {
        total: number;
        completed: number;
        pending: number;
        analyzing: number;
        failed: number;
        elapsedSeconds: number;
        paused?: boolean;
        error?: string;
    };
}>();
const query = defineModel<string>("query", { required: true });
const loadMoreSentinel = ref<HTMLElement | null>(null);
const loadPreviousSentinel = ref<HTMLElement | null>(null);
const galleryScroll = ref<HTMLElement | null>(null);
const galleryGrid = ref<HTMLElement | null>(null);
const galleryRail = ref<HTMLElement | null>(null);
const cardWidth = ref(190);
const railHeight = ref(600);
const scrollRatio = ref(0);
const dragRatio = ref(0);
const draggingRail = ref(false);
const loadedThumbnailIds = ref(new Set<number>());
const failedThumbnailIds = ref(new Set<number>());
const hoveredPhoto = ref<Photo | null>(null);
const hoverPoint = ref({ x: 0, y: 0 });
const loadingCardSpans = [12, 9, 14, 10, 8, 13, 11, 15];
let loadMoreObserver: IntersectionObserver | null = null;
let loadPreviousObserver: IntersectionObserver | null = null;
let thumbnailObserver: IntersectionObserver | null = null;
const photoCardElements = new Map<number, HTMLElement>();
const photoCardRefs = new Map<number, (element: unknown) => void>();
let gridWidthObserver: ResizeObserver | null = null;
let railHeightObserver: ResizeObserver | null = null;
let scrollFrame = 0;
const emit = defineEmits<{
    openSetup: [];
    backup: [mode: "backup" | "refresh"];
    openPhoto: [photo: Photo];
    toggleSelection: [photo: Photo];
    selectionMode: [];
    selectVisible: [photos: Photo[]];
    loadMore: [];
    loadPrevious: [];
    seekPhotos: [offset: number];
    requestThumbnail: [photoId: number, distance: number];
    analyzeSelected: [];
    stopAnalysis: [];
    continueAnalysis: [];
}>();
const dateFormatter = new Intl.DateTimeFormat("th-TH", { dateStyle: "medium" });
const formattedDates = new Map<string, string>();
function formatDate(value: string) {
    if (!value) return "—";
    let formatted = formattedDates.get(value);
    if (!formatted) {
        formatted = dateFormatter.format(new Date(value));
        formattedDates.set(value, formatted);
    }
    return formatted;
}
function formatElapsed(seconds: number) {
    const safeSeconds = Math.max(0, Math.floor(seconds || 0));
    const minutes = Math.floor(safeSeconds / 60);
    const remainingSeconds = safeSeconds % 60;
    return `${minutes} นาที ${remainingSeconds} วินาที`;
}
function isSelected(id: number) {
    return props.selectedPhotoIds.includes(id);
}
function thumbnailFailed(id: number) {
    return !!props.thumbnailFailures[id] || failedThumbnailIds.value.has(id);
}
function markThumbnailLoaded(id: number) {
    loadedThumbnailIds.value.add(id);
    if (!galleryBackdrop.value && props.thumbnails[id])
        galleryBackdrop.value = `url(${props.thumbnails[id]})`;
}
function markThumbnailFailed(id: number) {
    failedThumbnailIds.value.add(id);
}
function showPhotoTooltip(photo: Photo, event: MouseEvent) {
    hoveredPhoto.value = photo;
    hoverPoint.value = { x: event.clientX, y: event.clientY };
}
function movePhotoTooltip(event: MouseEvent) {
    hoverPoint.value = { x: event.clientX, y: event.clientY };
}
function focusPhotoTooltip(photo: Photo, event: FocusEvent) {
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    hoveredPhoto.value = photo;
    hoverPoint.value = {
        x: rect.left + rect.width / 2,
        y: rect.top + Math.min(rect.height / 2, 180),
    };
}
function hidePhotoTooltip() {
    hoveredPhoto.value = null;
}
function openPhoto(photo: Photo) {
    hidePhotoTooltip();
    if (props.selectionMode) emit("toggleSelection", photo);
    else emit("openPhoto", photo);
}
function previewTags(photo: Photo) {
    return photo.tags
        .split(",")
        .map((tag) => tag.trim())
        .filter(Boolean);
}
const tooltipStyle = computed(() => {
    if (typeof window === "undefined") return {};
    const width = Math.min(260, window.innerWidth - 24);
    const height = 132;
    const left =
        hoverPoint.value.x + 16 + width > window.innerWidth - 8
            ? hoverPoint.value.x - width - 16
            : hoverPoint.value.x + 16;
    const top =
        hoverPoint.value.y + 16 + height > window.innerHeight - 8
            ? Math.max(8, hoverPoint.value.y - height - 16)
            : hoverPoint.value.y + 16;
    return { left: `${Math.max(8, left)}px`, top: `${Math.max(8, top)}px` };
});
const galleryBackdrop = ref("");
const galleryMarkers = computed(() => {
    const total = Math.max(0, props.resultTotal);
    const count = Math.min(
        48,
        Math.ceil(total / 25),
        Math.max(2, Math.floor(railHeight.value / 14)),
    );
    if (count < 2) return [];
    return Array.from({ length: count }, (_, index) => {
        const offset = Math.round((index * (total - 1)) / (count - 1));
        return {
            index,
            offset,
            top: (index / (count - 1)) * 100,
            major: index % 5 === 0 || index === count - 1,
            label: `รูปที่ ${(offset + 1).toLocaleString("th-TH")}`,
        };
    });
});
const activeScrollRatio = computed(() =>
    draggingRail.value ? dragRatio.value : scrollRatio.value,
);
const scrollThumbStyle = computed(() => {
    const height = 3;
    return {
        top: `${Math.min(100 - height, activeScrollRatio.value * 100)}%`,
        height: `${height}%`,
    };
});
function updateScrollRatio() {
    if (hoveredPhoto.value) hoveredPhoto.value = null;
    if (scrollFrame) return;
    scrollFrame = requestAnimationFrame(() => {
        scrollFrame = 0;
        measureScrollRatio();
    });
}
function measureScrollRatio() {
    const root = galleryScroll.value;
    if (!root || !props.resultTotal) {
        scrollRatio.value = 0;
        return;
    }
    const maxScroll = Math.max(0, root.scrollHeight - root.clientHeight);
    const progress = maxScroll ? root.scrollTop / maxScroll : 0;
    const visibleIndex = Math.round(
        progress * Math.max(0, props.photos.length - 1),
    );
    scrollRatio.value = Math.min(
        1,
        Math.max(
            0,
            (props.startOffset + visibleIndex) /
                Math.max(1, props.resultTotal - 1),
        ),
    );
}
function seekToRatio(ratio: number) {
    emit(
        "seekPhotos",
        Math.max(
            0,
            Math.min(
                props.resultTotal - 1,
                Math.floor(ratio * props.resultTotal),
            ),
        ),
    );
}
function railPointerRatio(event: PointerEvent) {
    const rect = galleryRail.value?.getBoundingClientRect();
    return rect
        ? Math.min(1, Math.max(0, (event.clientY - rect.top) / rect.height))
        : 0;
}
function startRailDrag(event: PointerEvent) {
    if (props.resultTotal <= 0 || !galleryRail.value) return;
    draggingRail.value = true;
    dragRatio.value = railPointerRatio(event);
    galleryRail.value.setPointerCapture(event.pointerId);
}
function moveRailDrag(event: PointerEvent) {
    if (draggingRail.value) dragRatio.value = railPointerRatio(event);
}
function endRailDrag() {
    if (!draggingRail.value) return;
    draggingRail.value = false;
    seekToRatio(dragRatio.value);
}
function cancelRailDrag() {
    draggingRail.value = false;
}
function observeLoadMore() {
    if (!loadMoreObserver || !loadMoreSentinel.value) return;
    loadMoreObserver.unobserve(loadMoreSentinel.value);
    loadMoreObserver.observe(loadMoreSentinel.value);
}
function observeLoadPrevious() {
    if (!loadPreviousObserver || !loadPreviousSentinel.value) return;
    loadPreviousObserver.unobserve(loadPreviousSentinel.value);
    loadPreviousObserver.observe(loadPreviousSentinel.value);
}
function setPhotoCardElement(id: number, element: unknown) {
    const previous = photoCardElements.get(id);
    if (previous && previous !== element) {
        thumbnailObserver?.unobserve(previous);
        photoCardElements.delete(id);
    }
    if (
        element instanceof HTMLElement &&
        photoCardElements.get(id) !== element
    ) {
        photoCardElements.set(id, element);
        thumbnailObserver?.observe(element);
    }
}
function cardRef(id: number) {
    let callback = photoCardRefs.get(id);
    if (!callback) {
        callback = (element) => setPhotoCardElement(id, element);
        photoCardRefs.set(id, callback);
    }
    return callback;
}
onMounted(async () => {
    await nextTick();
    gridWidthObserver = new ResizeObserver(() => {
        const grid = galleryGrid.value;
        if (!grid) return;
        const width = Number.parseFloat(
            getComputedStyle(grid).gridTemplateColumns.split(" ")[0] || "",
        );
        if (Number.isFinite(width) && width > 0) cardWidth.value = width;
    });
    if (galleryGrid.value) gridWidthObserver.observe(galleryGrid.value);
    railHeightObserver = new ResizeObserver((entries) => {
        if (entries[0]?.contentRect.height)
            railHeight.value = entries[0].contentRect.height;
    });
    if (galleryRail.value) railHeightObserver.observe(galleryRail.value);
    const scrollRoot = galleryScroll.value;
    thumbnailObserver = new IntersectionObserver(
        (entries) => {
            const center = scrollRoot
                ? scrollRoot.getBoundingClientRect().top +
                  scrollRoot.clientHeight / 2
                : 0;
            entries
                .filter((entry) => entry.isIntersecting)
                .sort(
                    (a, b) =>
                        Math.abs(b.boundingClientRect.top - center) -
                        Math.abs(a.boundingClientRect.top - center),
                )
                .forEach((entry) => {
                    const card = entry.target as HTMLElement;
                    const photoId = Number(card.dataset.photoId);
                    if (
                        Number.isFinite(photoId) &&
                        !props.thumbnails[photoId] &&
                        !thumbnailFailed(photoId)
                    ) {
                        emit(
                            "requestThumbnail",
                            photoId,
                            Math.abs(entry.boundingClientRect.top - center),
                        );
                    }
                    thumbnailObserver?.unobserve(card);
                });
        },
        { root: scrollRoot, rootMargin: "1200px 0px", threshold: 0 },
    );
    photoCardElements.forEach((card) => thumbnailObserver?.observe(card));
    loadMoreObserver = new IntersectionObserver(
        ([entry]) => {
            if (
                entry?.isIntersecting &&
                !props.loadingMore &&
                !props.searching &&
                props.startOffset + props.photos.length < props.resultTotal
            )
                emit("loadMore");
        },
        { root: scrollRoot, rootMargin: "1800px 0px", threshold: 0 },
    );
    loadPreviousObserver = new IntersectionObserver(
        ([entry]) => {
            if (
                entry?.isIntersecting &&
                !props.loadingPrevious &&
                !props.searching &&
                props.startOffset > 0
            )
                emit("loadPrevious");
        },
        { root: scrollRoot, rootMargin: "1200px 0px", threshold: 0 },
    );
    observeLoadMore();
    observeLoadPrevious();
});
watch(galleryGrid, (grid, previous) => {
    if (previous) gridWidthObserver?.unobserve(previous);
    if (grid) gridWidthObserver?.observe(grid);
});
watch(galleryRail, (rail, previous) => {
    if (previous) railHeightObserver?.unobserve(previous);
    if (rail) railHeightObserver?.observe(rail);
});
watch(
    () => [
        props.photos.length,
        props.resultTotal,
        props.startOffset,
        props.loadingMore,
        props.loadingPrevious,
        props.searching,
    ],
    async () => {
        await nextTick();
        observeLoadMore();
        observeLoadPrevious();
        updateScrollRatio();
    },
);
onBeforeUnmount(() => {
    if (scrollFrame) cancelAnimationFrame(scrollFrame);
    loadMoreObserver?.disconnect();
    loadPreviousObserver?.disconnect();
    thumbnailObserver?.disconnect();
    gridWidthObserver?.disconnect();
    railHeightObserver?.disconnect();
    photoCardElements.clear();
    photoCardRefs.clear();
});
function imageRatio(photo: Photo) {
    const ratio =
        photo.width > 0 && photo.height > 0 ? photo.width / photo.height : 1;
    return Math.max(0.35, Math.min(2.5, ratio)).toFixed(3);
}
function estimatedSpan(photo: Photo) {
    const ratio = Number(imageRatio(photo));
    const captionHeight = photo.aiStatus === "completed" ? 43 : 69;
    return Math.max(
        1,
        Math.ceil((cardWidth.value / ratio + captionHeight + 12) / 18),
    );
}
</script>

<template>
    <section class="gallery-view">
        <div v-if="!configured" class="start-panel">
            <span class="start-icon"
                ><Images :size="27" :stroke-width="1.8"
            /></span>
            <h1>เริ่มสำรองรูปภาพ</h1>
            <p>เลือกโฟลเดอร์รูปภาพและที่เก็บสำเนา แล้วเริ่มใช้งานได้ทันที</p>
            <button class="primary" type="button" @click="emit('openSetup')">
                เลือกโฟลเดอร์ <ArrowRight :size="17" />
            </button>
        </div>
        <template v-else>
            <div class="gallery-controls">
                <div class="gallery-topbar">
                    <div class="search-field global-search">
                        <Search :size="19" :stroke-width="1.8" />
                        <input
                            v-model="query"
                            aria-label="ค้นหารูปภาพ"
                            placeholder="ค้นหาชื่อ คำอธิบาย หรือแท็ก"
                        />
                        <button
                            v-if="query"
                            class="clear-button"
                            type="button"
                            aria-label="ล้างคำค้นหา"
                            @click="query = ''"
                        >
                            <X :size="17" />
                        </button>
                    </div>
                    <button
                        class="toolbar-icon-button refresh-button"
                        type="button"
                        :disabled="busy"
                        :aria-label="
                            backupMode === 'refresh'
                                ? 'กำลังรีเฟรชคลังรูปภาพ'
                                : 'รีเฟรชคลังรูปภาพ'
                        "
                        title="รีเฟรชคลังรูปภาพ"
                        @click="emit('backup', 'refresh')"
                    >
                        <span
                            v-if="backupMode === 'refresh'"
                            class="loading-spinner"
                            aria-hidden="true"
                        ></span
                        ><RefreshCw v-else :size="19" :stroke-width="1.9" />
                    </button>
                    <button
                        class="primary backup-button"
                        type="button"
                        :disabled="busy"
                        @click="emit('backup', 'backup')"
                    >
                        <span
                            v-if="backupMode === 'backup'"
                            class="loading-spinner light"
                            aria-hidden="true"
                        ></span
                        ><Plus v-else :size="19" :stroke-width="2" />{{
                            backupMode === "backup"
                                ? keepOriginal
                                    ? "กำลังสำรอง…"
                                    : "กำลังย้าย…"
                                : keepOriginal
                                  ? "สำรองรูป"
                                  : "ย้ายรูป"
                        }}
                    </button>
                </div>
                <div class="gallery-heading-row">
                    <div class="gallery-title"><h1>รูปภาพ</h1></div>
                    <button
                        v-if="!selectionMode"
                        class="quiet-button analyze-entry"
                        type="button"
                        :disabled="busy || !photos.length"
                        title="เลือกภาพเพื่อวิเคราะห์ซ้ำ"
                        @click="emit('selectionMode')"
                    >
                        <Sparkles :size="16" />วิเคราะห์ซ้ำ
                    </button>
                    <div v-else class="library-heading-actions">
                        <span class="library-count"
                            >เลือก {{ selectedPhotoIds.length }} รูป</span
                        >
                        <button
                            class="quiet-button compact-action"
                            type="button"
                            :disabled="analysisBusy || !photos.length"
                            @click="emit('selectVisible', photos)"
                        >
                            เลือกทั้งหมด
                        </button>
                        <button
                            class="primary compact-action"
                            type="button"
                            :disabled="analysisBusy || !selectedPhotoIds.length"
                            @click="emit('analyzeSelected')"
                        >
                            <span
                                v-if="analysisBusy"
                                class="loading-spinner light"
                                aria-hidden="true"
                            ></span
                            ><Sparkles v-else :size="16" />วิเคราะห์
                        </button>
                        <button
                            class="quiet-button compact-action"
                            type="button"
                            :disabled="analysisBusy"
                            @click="emit('selectionMode')"
                        >
                            ยกเลิก
                        </button>
                    </div>
                </div>

                <div
                    v-if="
                        aiConfigured &&
                        !backupActive &&
                        aiProgress.total &&
                        (aiProgress.analyzing > 0 || aiProgress.pending > 0)
                    "
                    class="image-ai-queue-status ai-queue-status"
                    role="status"
                    aria-live="polite"
                >
                    <span
                        v-if="aiProgress.analyzing"
                        class="loading-spinner"
                        aria-hidden="true"
                    ></span>
                    <Check
                        v-else-if="!aiProgress.pending && !aiProgress.failed"
                        :size="17"
                    />
                    <Sparkles v-else :size="17" />
                    <div class="ai-queue-copy">
                        <strong>{{
                            aiProgress.paused
                                ? "หยุดวิเคราะห์ชั่วคราว"
                                : aiProgress.analyzing
                                  ? "กำลังวิเคราะห์ภาพ"
                                  : aiProgress.pending
                                    ? "รอวิเคราะห์ภาพ"
                                    : aiProgress.failed
                                      ? "วิเคราะห์เสร็จบางส่วน"
                                      : "วิเคราะห์ภาพครบแล้ว"
                        }}</strong
                        ><small
                            >{{ aiProgress.completed }} /
                            {{ aiProgress.total }} รูป · กำลังทำ
                            {{ aiProgress.analyzing }} · รอคิว
                            {{ aiProgress.pending
                            }}<span v-if="aiProgress.failed">
                                · ไม่สำเร็จ {{ aiProgress.failed }}</span
                            ><span v-if="aiProgress.elapsedSeconds > 0">
                                · เวลาที่ใช้
                                {{
                                    formatElapsed(aiProgress.elapsedSeconds)
                                }}</span
                            ></small
                        ><small
                            v-if="aiProgress.error"
                            class="ai-queue-error"
                            :title="aiProgress.error"
                            >{{ aiProgress.error }}</small
                        ><progress
                            :value="aiProgress.completed + aiProgress.failed"
                            :max="aiProgress.total"
                        ></progress>
                    </div>
                    <button
                        v-if="aiProgress.analyzing && !aiProgress.paused"
                        class="ai-queue-control"
                        type="button"
                        :disabled="analysisControlBusy"
                        @click="emit('stopAnalysis')"
                    >
                        {{
                            analysisControlBusy ? "กำลังหยุด…" : "หยุดวิเคราะห์"
                        }}
                    </button>
                    <button
                        v-else-if="aiProgress.paused && aiProgress.pending"
                        class="ai-queue-control"
                        type="button"
                        :disabled="analysisControlBusy"
                        @click="emit('continueAnalysis')"
                    >
                        {{ analysisControlBusy ? "กำลังเริ่ม…" : "ทำต่อ" }}
                    </button>
                </div>
            </div>

            <div
                class="gallery-scroll-region"
                :class="{ 'has-position-rail': galleryMarkers.length > 0 }"
            >
                <div
                    ref="galleryScroll"
                    class="gallery-scroll-area"
                    @scroll.passive="updateScrollRatio"
                >
                    <div
                        v-if="startOffset > 0"
                        ref="loadPreviousSentinel"
                        class="load-previous-sentinel"
                        role="status"
                        aria-live="polite"
                    >
                        <span
                            v-if="loadingPrevious"
                            class="loading-spinner"
                            aria-hidden="true"
                        ></span
                        ><span v-if="loadingPrevious"
                            >กำลังโหลดรูปก่อนหน้า…</span
                        >
                    </div>
                    <div class="results-heading">
                        <span>{{ query ? "ผลการค้นหา" : "ทั้งหมด" }}</span>
                        <span
                            v-if="searching"
                            class="search-status"
                            role="status"
                            ><span
                                class="loading-spinner"
                                aria-hidden="true"
                            ></span
                            >กำลังค้นหา…</span
                        >
                        <span v-else
                            >{{ resultTotal.toLocaleString("th-TH") }} รูป</span
                        >
                    </div>
                    <template v-if="photos.length">
                        <div
                            ref="galleryGrid"
                            class="photo-grid masonry-grid"
                            :style="
                                galleryBackdrop
                                    ? { '--gallery-backdrop': galleryBackdrop }
                                    : undefined
                            "
                            :aria-busy="searching"
                        >
                            <button
                                v-for="(photo, index) in photos"
                                :key="photo.id"
                                v-memo="[
                                    photo,
                                    index,
                                    thumbnails[photo.id],
                                    loadedThumbnailIds.has(photo.id),
                                    thumbnailFailed(photo.id),
                                    selectionMode,
                                    isSelected(photo.id),
                                    hoveredPhoto?.id === photo.id,
                                    cardWidth,
                                ]"
                                :ref="cardRef(photo.id)"
                                :data-photo-id="photo.id"
                                :data-photo-index="index"
                                class="photo-card"
                                :class="{
                                    selectable: selectionMode,
                                    selected:
                                        selectionMode && isSelected(photo.id),
                                }"
                                :style="{
                                    '--photo-ratio': imageRatio(photo),
                                    '--masonry-span': estimatedSpan(photo),
                                }"
                                :aria-label="
                                    selectionMode
                                        ? `${isSelected(photo.id) ? 'ยกเลิกเลือก' : 'เลือก'} ${photo.filename}`
                                        : `เปิด ${photo.filename}`
                                "
                                :aria-describedby="
                                    hoveredPhoto?.id === photo.id
                                        ? 'photo-hover-tooltip'
                                        : undefined
                                "
                                :aria-pressed="
                                    selectionMode
                                        ? isSelected(photo.id)
                                        : undefined
                                "
                                @mouseenter="showPhotoTooltip(photo, $event)"
                                @mousemove="movePhotoTooltip"
                                @mouseleave="hidePhotoTooltip"
                                @focus="focusPhotoTooltip(photo, $event)"
                                @blur="hidePhotoTooltip"
                                @click="openPhoto(photo)"
                            >
                                <span
                                    v-if="selectionMode"
                                    class="photo-check"
                                    aria-hidden="true"
                                    ><Transition name="selection-check"
                                        ><Check
                                            v-if="isSelected(photo.id)"
                                            :size="14" /></Transition
                                ></span>
                                <div class="thumb">
                                    <div
                                        v-if="
                                            !thumbnailFailed(photo.id) &&
                                            (!thumbnails[photo.id] ||
                                                !loadedThumbnailIds.has(
                                                    photo.id,
                                                ))
                                        "
                                        class="loading-pulse thumb-loading"
                                        aria-hidden="true"
                                    ></div>
                                    <img
                                        v-if="
                                            thumbnails[photo.id] &&
                                            !thumbnailFailed(photo.id)
                                        "
                                        :class="{
                                            'thumb-image-loaded':
                                                loadedThumbnailIds.has(
                                                    photo.id,
                                                ),
                                        }"
                                        :src="thumbnails[photo.id]"
                                        :alt="
                                            photo.description || photo.filename
                                        "
                                        decoding="async"
                                        @load="markThumbnailLoaded(photo.id)"
                                        @error="markThumbnailFailed(photo.id)"
                                    />
                                    <ImageIcon
                                        v-if="thumbnailFailed(photo.id)"
                                        :size="27"
                                        :stroke-width="1.4"
                                    />
                                </div>
                                <div class="photo-caption">
                                    <strong>{{ photo.filename }}</strong
                                    ><span>{{
                                        formatDate(photo.takenAt)
                                    }}</span>
                                </div>
                                <span
                                    v-if="photo.aiStatus !== 'completed'"
                                    class="photo-ai-status"
                                    :class="`status-${photo.aiStatus || 'pending'}`"
                                    :title="photo.aiError || undefined"
                                    ><span
                                        v-if="photo.aiStatus === 'analyzing'"
                                        class="loading-spinner"
                                        aria-hidden="true"
                                    ></span
                                    ><Sparkles
                                        v-else-if="photo.aiStatus !== 'failed'"
                                        :size="12"
                                    /><span>{{
                                        photo.aiStatus === "analyzing"
                                            ? "กำลังวิเคราะห์"
                                            : photo.aiStatus === "failed"
                                              ? "วิเคราะห์ไม่สำเร็จ"
                                              : "รอวิเคราะห์"
                                    }}</span></span
                                >
                            </button>
                            <div
                                v-for="(span, index) in loadingMore
                                    ? loadingCardSpans
                                    : []"
                                :key="`loading-${photos.length}-${index}`"
                                class="photo-loading-card"
                                :style="{
                                    '--masonry-span': span,
                                    '--photo-enter-delay': `${index * 12}ms`,
                                }"
                                aria-hidden="true"
                            >
                                <span class="loading-pulse"></span>
                            </div>
                        </div>
                        <div
                            v-if="startOffset + photos.length < resultTotal"
                            ref="loadMoreSentinel"
                            class="load-more-sentinel"
                            role="status"
                            aria-live="polite"
                        >
                            <span
                                v-if="loadingMore"
                                class="loading-spinner"
                                aria-hidden="true"
                            ></span>
                            <span v-if="loadingMore"
                                >กำลังโหลดรูปเพิ่มเติม…</span
                            >
                        </div>
                    </template>
                    <div
                        v-else-if="searching"
                        class="loading-grid search-loading"
                        role="status"
                        aria-label="กำลังค้นหารูปภาพ"
                    >
                        <div v-for="item in 5" :key="item" class="loading-card">
                            <div class="loading-pulse loading-image"></div>
                            <div
                                class="loading-pulse loading-line loading-caption"
                            ></div>
                        </div>
                        <span class="sr-only">กำลังค้นหารูปภาพ</span>
                    </div>
                    <div v-else class="empty">
                        <ImageIcon :size="29" :stroke-width="1.5" />
                        <h3>{{ query ? "ไม่พบรูปภาพ" : "ยังไม่มีรูปภาพ" }}</h3>
                        <p>
                            {{
                                query
                                    ? "ลองเปลี่ยนคำค้นหา"
                                    : `กด “${keepOriginal ? "สำรองรูป" : "ย้ายรูป"}” เพื่อเพิ่มรูปภาพ`
                            }}
                        </p>
                    </div>
                </div>
                <div
                    v-if="galleryMarkers.length"
                    ref="galleryRail"
                    class="gallery-position-rail"
                    role="group"
                    aria-label="ข้ามไปยังช่วงรูปภาพ"
                    @pointerdown="startRailDrag"
                    @pointermove="moveRailDrag"
                    @pointerup="endRailDrag"
                    @pointercancel="cancelRailDrag"
                >
                    <span
                        class="gallery-position-track"
                        aria-hidden="true"
                    ></span>
                    <button
                        v-for="marker in galleryMarkers"
                        :key="marker.index"
                        class="gallery-position-mark"
                        :class="{ major: marker.major }"
                        type="button"
                        :style="{ top: `${marker.top}%` }"
                        :data-rank="(marker.offset + 1).toLocaleString('th-TH')"
                        :aria-label="`ไปที่${marker.label}`"
                        @pointerdown.stop
                        @click.stop="emit('seekPhotos', marker.offset)"
                    >
                        <span></span>
                    </button>
                    <span
                        class="gallery-position-thumb"
                        :class="{ dragging: draggingRail }"
                        :style="scrollThumbStyle"
                        aria-hidden="true"
                    ></span>
                    <span
                        class="gallery-position-endpoint gallery-position-start"
                        >1</span
                    >
                    <span
                        class="gallery-position-endpoint gallery-position-end"
                        >{{ resultTotal.toLocaleString("th-TH") }}</span
                    >
                </div>
            </div>
        </template>
        <Teleport to="body">
            <Transition name="photo-tooltip">
                <aside
                    v-if="hoveredPhoto"
                    id="photo-hover-tooltip"
                    class="photo-hover-tooltip"
                    role="tooltip"
                    :style="tooltipStyle"
                >
                    <strong class="tooltip-filename">{{
                        hoveredPhoto.filename
                    }}</strong>
                    <span class="tooltip-label">คำอธิบาย</span>
                    <p class="tooltip-description">
                        {{ hoveredPhoto.description || "ยังไม่มีคำอธิบาย" }}
                    </p>
                    <div class="tooltip-tags-row">
                        <span class="tooltip-label">แท็ก</span>
                        <div
                            v-if="previewTags(hoveredPhoto).length"
                            class="tooltip-tags"
                        >
                            <span
                                v-for="tag in previewTags(hoveredPhoto).slice(
                                    0,
                                    3,
                                )"
                                :key="tag"
                                class="tooltip-tag"
                                >{{ tag }}</span
                            ><span
                                v-if="previewTags(hoveredPhoto).length > 3"
                                class="tooltip-tag-more"
                                >+{{
                                    previewTags(hoveredPhoto).length - 3
                                }}</span
                            >
                        </div>
                        <span v-else class="tooltip-no-tags">ไม่มีแท็ก</span>
                    </div>
                </aside>
            </Transition>
        </Teleport>
    </section>
</template>

<style scoped>
.photo-card.selectable {
    position: relative;
    isolation: isolate;
    transition:
        transform 250ms cubic-bezier(0.2, 0.75, 0.25, 1),
        filter 250ms ease;
}

.photo-hover-tooltip {
    position: fixed;
    z-index: 9999;
    width: min(260px, calc(100vw - 24px));
    padding: 10px 11px;
    border: 1px solid #d9e2d9;
    border-top: 2px solid #62846a;
    background: #fff;
    color: #29332b;
    box-shadow: 0 8px 24px rgb(24 37 27 / 16%);
    pointer-events: none;
}
.tooltip-filename {
    display: block;
    overflow: hidden;
    margin-bottom: 7px;
    color: #253029;
    font-size: 11px;
    font-weight: 680;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.tooltip-label {
    color: #778279;
    font-size: 9px;
    font-weight: 650;
}
.tooltip-description {
    display: -webkit-box;
    overflow: hidden;
    margin: 2px 0 8px;
    color: #505a52;
    font-size: 10px;
    line-height: 1.45;
    overflow-wrap: anywhere;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
}
.tooltip-tags-row {
    display: flex;
    align-items: flex-start;
    gap: 8px;
}
.tooltip-tags-row > .tooltip-label {
    flex: none;
    padding-top: 3px;
}
.tooltip-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    min-width: 0;
    max-height: 42px;
    overflow: hidden;
}
.tooltip-tag,
.tooltip-tag-more {
    max-width: 112px;
    overflow: hidden;
    padding: 2px 5px;
    border: 1px solid #dce6dc;
    background: #f2f6f2;
    color: #49664e;
    font-size: 9px;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.tooltip-tag-more {
    flex: none;
    background: #fff;
    color: #68736a;
}
.tooltip-no-tags {
    color: #899189;
    font-size: 10px;
}
.photo-tooltip-enter-active,
.photo-tooltip-leave-active {
    transition:
        opacity 100ms ease,
        transform 100ms ease;
}
.photo-tooltip-enter-from,
.photo-tooltip-leave-to {
    opacity: 0;
    transform: translateY(3px);
}

.photo-card.selectable.selected {
    z-index: 1;
    outline: none !important;
    animation: selection-glow-pulse 2.8s ease-in-out infinite;
}

.photo-card.selectable::before {
    position: absolute;
    z-index: 2;
    inset: 0;
    padding: 1.5px;
    background: conic-gradient(
        from var(--selection-angle, 0deg),
        transparent 0 62%,
        #6d9978 72%,
        #9ab9a2 82%,
        #d1dfd3 90%,
        transparent 100%
    );
    content: "";
    opacity: 0;
    pointer-events: none;
    transition: opacity 220ms ease;
    -webkit-mask:
        linear-gradient(#fff 0 0) content-box,
        linear-gradient(#fff 0 0);
    -webkit-mask-composite: xor;
    mask:
        linear-gradient(#fff 0 0) content-box,
        linear-gradient(#fff 0 0);
    mask-composite: exclude;
}

.photo-card.selectable.selected::before {
    opacity: 1;
    animation: selection-edge-travel 3.2s linear infinite;
}

.photo-card.selectable.selected::after {
    position: absolute;
    z-index: 1;
    inset: 0;
    background: linear-gradient(
        112deg,
        transparent 24%,
        rgb(255 255 255 / 22%) 48%,
        transparent 70%
    );
    content: "";
    opacity: 0;
    pointer-events: none;
    animation: selection-shimmer 700ms ease-out 1;
}

.photo-card.selectable.selected .thumb img {
    animation: selection-image-in 250ms cubic-bezier(0.2, 0.75, 0.25, 1) both;
}

.photo-card.selectable.selected .thumb {
    outline: none !important;
}

.photo-card.selectable .photo-check {
    position: absolute;
    z-index: 3;
    top: 10px;
    right: 10px;
    left: auto;
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    border: 1px solid rgb(52 95 67 / 25%);
    border-radius: 50% !important;
    background: rgb(255 255 255 / 88%);
    color: #fff;
    opacity: 0.86;
    transform: scale(0.9);
    transition:
        opacity 180ms ease,
        transform 220ms cubic-bezier(0.2, 0.8, 0.2, 1),
        background-color 180ms ease,
        border-color 180ms ease;
}

.photo-card.selectable.selected .photo-check {
    border-color: #47785a;
    background: linear-gradient(145deg, #5b8968, #47785a);
    box-shadow: 0 2px 10px rgb(71 120 90 / 28%);
    opacity: 1;
    transform: scale(1);
    animation: selection-check-pop 250ms cubic-bezier(0.18, 0.9, 0.28, 1.35)
        both;
}

.selection-check-enter-active,
.selection-check-leave-active {
    transition:
        opacity 160ms ease,
        transform 180ms cubic-bezier(0.2, 0.8, 0.2, 1);
}

.selection-check-enter-from,
.selection-check-leave-to {
    opacity: 0;
    transform: scale(0.65);
}

@keyframes selection-edge-travel {
    to {
        --selection-angle: 360deg;
    }
}

@keyframes selection-image-in {
    from {
        transform: scale(0.97);
    }
    to {
        transform: scale(1);
    }
}

@keyframes selection-glow-pulse {
    0%,
    100% {
        filter: drop-shadow(0 0 3px rgb(71 120 90 / 14%));
    }
    50% {
        filter: drop-shadow(0 0 6px rgb(71 120 90 / 24%));
    }
}

@keyframes selection-check-pop {
    from {
        opacity: 0;
        transform: scale(0.7);
    }
    to {
        opacity: 1;
        transform: scale(1);
    }
}

@keyframes selection-shimmer {
    0% {
        opacity: 0;
        transform: translateX(-25%);
    }
    35% {
        opacity: 0.65;
    }
    100% {
        opacity: 0;
        transform: translateX(25%);
    }
}

@property --selection-angle {
    syntax: "<angle>";
    inherits: false;
    initial-value: 0deg;
}

@media (prefers-reduced-motion: reduce) {
    .photo-card.selectable,
    .photo-card.selectable::before,
    .photo-card.selectable::after,
    .photo-card.selectable .thumb img,
    .photo-card.selectable .photo-check {
        animation: none !important;
        transition: none !important;
    }
}
</style>
