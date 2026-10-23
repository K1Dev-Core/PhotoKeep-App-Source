# โครงสร้าง PhotoKeep

## เส้นทางการทำงาน

```text
Nuxt (frontend/app)
  → Wails bridge (app.go และไฟล์ *_app.go ที่ root)
  → Service (internal/photokeep)
  → SQLite และไฟล์รูปในเครื่อง

Service → AI gateway (services/ai-gateway) → ผู้ให้บริการ AI
```

แอปและ AI gateway เป็นคนละโปรเซส คีย์ผู้ให้บริการอยู่ฝั่ง gateway เท่านั้น ภาพและคำบรรยายที่วิเคราะห์เสร็จถูกบันทึกใน SQLite ของแอป ระบบค้นหาคำจากชื่อไฟล์ คำบรรยาย และแท็กทำงานในเครื่อง

## เปิดไฟล์ไหนเมื่อต้องแก้งาน

| งาน | ไฟล์หลัก | หน้าที่ |
| --- | --- | --- |
| เริ่มแอปและ Wails | `main.go`, `app.go`, `image_ai_app.go`, `openvsx_themes.go` | เปิดหน้าต่างและเปิดเมธอดให้ Nuxt เรียก |
| ฐานข้อมูลและการตั้งค่า | `internal/photokeep/models.go`, `service.go`, `settings.go` | โครงข้อมูล เปิด SQLite และจัดการโฟลเดอร์ |
| สำรองรูป | `internal/photokeep/backup.go` | สแกน คัดลอก/ย้าย ตรวจไฟล์ซ้ำ และรายงานความคืบหน้า |
| ค้นหารูป | `internal/photokeep/photos_search.go`, `search_terms.go` | กรองและเรียงผลด้วยคำค้นและตัวตัดคำภาษาไทย |
| แก้คำบรรยาย/แท็ก | `internal/photokeep/photos_metadata.go` | ตรวจข้อมูลและบันทึกลง SQLite |
| แสดงรูป | `internal/photokeep/photos_preview.go` | อ่านรูปจริง สร้างและแคชภาพย่อ |
| งาน AI | `internal/photokeep/image_ai.go`, `image_ai_jobs.go` | คิวงาน สถานะ และการสื่อสารกับ AI gateway |
| ผู้ให้บริการ AI | `services/ai-gateway/internal/gateway/` | รับรูปจากแอปและเรียกผู้ให้บริการที่ตั้งไว้ |
| จุดเริ่ม gateway | `services/ai-gateway/cmd/` | เปิดเซิร์ฟเวอร์ AI gateway |

ส่วน `frontend/dist`, `frontend/.nuxt`, `build` และ `dist` เป็นผลลัพธ์จากการ build ไม่ใช่จุดแก้ source

## หน้าบ้าน

```text
frontend/app/
├── components/
│   ├── PhotoKeepApp.vue       คุมสถานะและเรียก Wails bridge
│   ├── shared/                เมนู toast ช่องโฟลเดอร์ และสถานะโหลด
│   ├── gallery/               รายการรูปและหน้าต่างรายละเอียดรูป
│   ├── backup/                สถานะสำรองและหน้าต่างเลือกโฟลเดอร์
│   ├── settings/              ตั้งค่าและนำเข้าธีม
│   └── ai/                    ประวัติงานวิเคราะห์และข้อผิดพลาด
├── assets/css/                สไตล์และตัวแปรสีของธีม
├── types/                     ชนิดข้อมูลที่ใช้ร่วมกัน
└── utils/                     การอ่านและตรวจข้อมูลธีม
```

`PhotoKeepApp.vue` ประกอบหน้าจอและเก็บสถานะร่วม คอมโพเนนต์ในโฟลเดอร์ย่อยรับข้อมูลผ่าน props และส่งเหตุการณ์กลับขึ้นมา จึงเริ่มอ่านจาก `PhotoKeepApp.vue` เมื่อแก้การไหลระหว่างหลายหน้า และเริ่มจากคอมโพเนนต์ย่อยเมื่อแก้หน้าจอเฉพาะส่วน

## ตัวอย่าง: สำรองภาพแล้วค้นหา

1. `GalleryPanel.vue` ส่งคำสั่งสำรองให้ `PhotoKeepApp.vue` เรียก Wails bridge
2. `app.go` ส่งต่อไป `Service`; `backup.go` สแกนรูปและบันทึก `Photo` ใน SQLite
3. หน้าบ้านรับความคืบหน้าและแสดงที่ `BackupProgressDialog.vue`
4. เมื่อผู้ใช้ค้นหา `photos_search.go` คัดผู้สมัครจากดัชนี SQLite; `search_terms.go` ตัดคำและจัดอันดับข้อความ
5. หน้ารายการขอภาพย่อจาก `photos_preview.go`; เปิดรายละเอียดใช้ภาพต้นฉบับ

คู่มือแยก AI gateway ไปอีกเครื่องอยู่ที่ [remote-ai-gateway-source.md](remote-ai-gateway-source.md)
