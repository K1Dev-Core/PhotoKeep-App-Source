<script setup lang="ts">
import { ArrowRight, FolderOpen, Monitor, Sparkles, Trash2 } from "@lucide/vue";
import { ref } from "vue";
import FolderFields from "../shared/FolderFields.vue";
import OpenVSXThemePanel from "./OpenVSXThemePanel.vue";
import type { ImportedTheme } from "../../utils/openvsx-theme";

defineProps<{
    canSave: boolean;
    saving: boolean;
    clearArmed: boolean;
    clearing: boolean;
    importedThemes: ImportedTheme[];
    selectedImportedThemeId: string;
    gatewaySaving: boolean;
}>();
const source = defineModel<string>("source", { required: true });
const dest = defineModel<string>("dest", { required: true });
const keepOriginal = defineModel<boolean>("keepOriginal", { required: true });
const gatewayUrl = defineModel<string>("gatewayUrl", { required: true });
const themePreference = defineModel<"system" | "imported">("themePreference", {
    required: true,
});
type SettingsCategory = "appearance" | "folders" | "library" | "ai";
const category = ref<SettingsCategory>("folders");
const categories = [
    { id: "folders", label: "โฟลเดอร์", icon: FolderOpen },
    { id: "appearance", label: "รูปลักษณ์", icon: Monitor },
    { id: "library", label: "ข้อมูลในคลัง", icon: Trash2 },
    { id: "ai", label: "AI", icon: Sparkles },
] as const;
const emit = defineEmits<{
    pick: [which: "source" | "dest"];
    save: [];
    clear: [];
    importTheme: [theme: ImportedTheme];
    selectTheme: [id: string];
    removeTheme: [id: string];
    saveGateway: [];
}>();
</script>

<template>
    <section class="settings-view">
        <div class="page-heading">
            <div>
                <h1>ตั้งค่า</h1>
                <p>จัดการโฟลเดอร์ รูปลักษณ์ และ API วิเคราะห์ภาพ</p>
            </div>
        </div>
        <nav class="settings-categories" aria-label="หมวดการตั้งค่า">
            <button
                v-for="item in categories"
                :key="item.id"
                type="button"
                :class="{ active: category === item.id }"
                :aria-current="category === item.id ? 'page' : undefined"
                @click="category = item.id"
            >
                <component :is="item.icon" :size="16" />{{ item.label }}
            </button>
        </nav>
        <div v-show="category === 'appearance'" class="settings-category-body">
            <section
                class="appearance-settings"
                aria-labelledby="appearance-settings-title"
            >
                <div><h2 id="appearance-settings-title">สีตามระบบ</h2></div>
                <button
                    type="button"
                    class="system-theme-option"
                    :aria-pressed="themePreference === 'system'"
                    :class="{ active: themePreference === 'system' }"
                    @click="themePreference = 'system'"
                >
                    <Monitor :size="15" /> ใช้สีตามระบบ
                </button>
            </section>
            <OpenVSXThemePanel
                :themes="importedThemes"
                :selected-id="
                    themePreference === 'imported'
                        ? selectedImportedThemeId
                        : ''
                "
                @import="emit('importTheme', $event)"
                @select="emit('selectTheme', $event)"
                @remove="emit('removeTheme', $event)"
            />
        </div>
        <div v-show="category === 'folders'" class="settings-category-body">
            <div class="settings-category-heading">
                <h2>โฟลเดอร์สำรอง</h2>
                <p>กำหนดต้นทางและตำแหน่งเก็บสำเนารูปภาพ</p>
            </div>
            <FolderFields
                v-model:source="source"
                v-model:dest="dest"
                v-model:keep-original="keepOriginal"
                @pick="emit('pick', $event)"
            />
            <p class="settings-note">
                การเปลี่ยนโฟลเดอร์ไม่ลบรายการรูปเดิมในคลัง
            </p>
            <div class="settings-actions">
                <button
                    class="primary"
                    :disabled="!canSave"
                    @click="emit('save')"
                >
                    <span
                        v-if="saving"
                        class="loading-spinner light"
                        aria-hidden="true"
                    ></span
                    ><ArrowRight v-else :size="16" />{{
                        saving ? "กำลังบันทึก…" : "บันทึกการตั้งค่า"
                    }}
                </button>
            </div>
        </div>
        <div v-show="category === 'library'" class="settings-category-body">
            <div class="danger-zone">
                <h2>ข้อมูลในคลัง</h2>
                <p>
                    ลบรูปที่สำรองแบบคัดลอกและล้างรายการในฐานข้อมูล
                    รูปที่ย้ายมาจากต้นทางจะยังอยู่ในโฟลเดอร์ปลายทาง
                </p>
                <button
                    class="danger"
                    :class="{ armed: clearArmed }"
                    :disabled="clearing"
                    @click="emit('clear')"
                >
                    <span
                        v-if="clearing"
                        class="loading-spinner light"
                        aria-hidden="true"
                    ></span
                    ><Trash2 v-else :size="16" />{{
                        clearing
                            ? "กำลังลบรูปและล้างข้อมูล…"
                            : clearArmed
                              ? "กดอีกครั้งเพื่อลบรูปทั้งหมด"
                              : "ล้างข้อมูลในคลัง"
                    }}
                </button>
            </div>
        </div>
        <div v-show="category === 'ai'" class="settings-category-body">
            <div class="settings-category-heading">
                <h2>API วิเคราะห์ภาพ</h2>
                <p>ตั้งค่าที่อยู่ Gateway ที่แอปใช้วิเคราะห์ภาพและสร้างเวกเตอร์ค้นหา</p>
            </div>
            <label class="gateway-field" for="image-ai-gateway-url">
                <span>PHOTOKEEP_AI_GATEWAY_URL</span>
                <input
                    id="image-ai-gateway-url"
                    v-model="gatewayUrl"
                    type="url"
                    inputmode="url"
                    autocomplete="url"
                    placeholder="https://vision.k1god.com/v1/analyze"
                />
            </label>
            <p class="settings-note">ค่าเริ่มต้น: https://vision.k1god.com/v1/analyze</p>
            <div class="settings-actions">
                <button class="primary" :disabled="gatewaySaving" @click="emit('saveGateway')">
                    <span v-if="gatewaySaving" class="loading-spinner light" aria-hidden="true"></span>
                    <ArrowRight v-else :size="16" />{{ gatewaySaving ? "กำลังบันทึก…" : "บันทึกที่อยู่ API" }}
                </button>
            </div>
        </div>
    </section>
</template>

<style scoped>
.settings-categories {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 6px;
    margin-bottom: 18px;
    padding: 4px;
    border: 1px solid var(--border);
    background: var(--surface);
}
.settings-categories button {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    min-height: 38px;
    border: 1px solid transparent;
    background: transparent;
    color: var(--muted);
    font: inherit;
    font-size: 12px;
    cursor: pointer;
    transition:
        background 0.16s ease,
        color 0.16s ease;
}
.settings-categories button:hover {
    background: var(--green-soft);
    color: var(--ink);
}
.settings-categories button.active {
    border-color: var(--border);
    background: var(--page);
    color: var(--green-dark);
    font-weight: 650;
}
.settings-categories button:focus-visible {
    outline: 2px solid var(--green);
    outline-offset: -2px;
}
.settings-category-body {
    min-width: 0;
}
.settings-category-body > :first-child {
    margin-top: 0;
}
.system-theme-option {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    min-height: 34px;
    padding: 6px 11px;
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-size: 11px;
    cursor: pointer;
}
.system-theme-option:hover {
    background: var(--green-soft);
}
.system-theme-option.active {
    border-color: var(--green);
    background: var(--green-soft);
    font-weight: 650;
}
.system-theme-option:focus-visible {
    outline: 2px solid var(--green);
    outline-offset: 2px;
}
.settings-category-heading {
    margin-bottom: 12px;
}
.settings-category-heading h2 {
    margin: 0;
    color: var(--ink);
    font-size: 15px;
}
.settings-category-heading p {
    margin: 3px 0 0;
    color: var(--muted);
    font-size: 11px;
}
.gateway-field {
    display: grid;
    gap: 7px;
    max-width: 720px;
    color: var(--ink);
    font-size: 12px;
    font-weight: 620;
}
.gateway-field input {
    width: 100%;
    min-height: 42px;
    box-sizing: border-box;
    padding: 9px 12px;
    border: 1px solid var(--border);
    background: var(--surface);
    color: var(--ink);
    font: inherit;
    font-weight: 450;
}
.gateway-field input:focus-visible {
    outline: 2px solid var(--green);
    outline-offset: 1px;
}
.danger-zone {
    margin-top: 24px;
    padding: 16px;
    border: 1px solid var(--border);
    border-radius: 7px;
    background: var(--surface);
}
.danger-zone h2 {
    font-size: 15px;
    color: var(--danger);
    margin-bottom: 4px;
}
.danger-zone p {
    font-size: 13px;
    color: var(--muted);
    margin-bottom: 12px;
}
.danger-zone .danger {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    background: var(--surface);
    color: var(--danger);
    border: 1px solid var(--border);
    border-radius: 5px;
    padding: 9px 13px;
    font-size: 13px;
    font-weight: 650;
    cursor: pointer;
}
.danger-zone .danger.armed {
    background: var(--danger);
    color: var(--surface);
}
.danger-zone .danger:disabled {
    opacity: 0.6;
    cursor: wait;
}
@media (max-width: 650px) {
    .settings-categories {
        grid-template-columns: repeat(2, minmax(0, 1fr));
    }
}
</style>
