# โครงสร้าง PhotoKeep สำหรับอธิบายงาน

จัดตามแผนผังใน `Wails.zip`: Nuxt เป็นหน้าจอ, Wails binding เป็นสะพาน, Go เป็นลอจิกและการเข้าถึงระบบปฏิบัติการ, GORM ดูแล SQLite. ใน Go ใช้ `struct` และแยกไฟล์ตามหน้าที่ แทนการสร้าง class แบบภาษาเชิงวัตถุ

```text
ผู้ใช้กดปุ่มใน Nuxt
  → PhotoKeepApp.vue เรียก window.go.main.App.<เมธอด>()
  → Wails ส่งคำสั่งไปยัง App ใน Go
  → App ส่งต่อให้ Service ใน internal/photokeep
  → Service ทำงานกับโฟลเดอร์/รูปภาพและ SQLite ผ่าน GORM
  → ส่งผลกลับเป็น Promise ให้ Nuxt แสดงบนหน้าจอ
```

## แผนผังโฟลเดอร์

```text
flashback/
├── main.go                    เปิดหน้าต่าง Wails และ bind App
├── app.go                     ตัวเชื่อม Wails และเลือกโฟลเดอร์ของ OS
├── app_test.go                ทดสอบการไหลหลักด้วยไฟล์รูปจริงชั่วคราว
├── internal/photokeep/        ลอจิก Go ที่ไม่ขึ้นกับหน้าต่าง Wails
│   ├── models.go              FileRecord, Setting, Photo และผลลัพธ์ API
│   ├── service.go             เปิด SQLite, AutoMigrate, อ่านสถานะ
│   ├── settings.go            ตรวจสอบ source/dest และบันทึกการตั้งค่า
│   ├── backup.go              สำรองภาพและตรวจไฟล์ซ้ำ
│   ├── analysis.go            วิเคราะห์แนวภาพ/โทนสี/ความสว่าง
│   ├── photos.go              ค้นหา, Tag Cloud, แก้ข้อมูล, preview
│   └── analysis_test.go       ทดสอบแท็กโทนสี
├── frontend/
│   ├── app/app.vue            Nuxt root
│   ├── app/layouts/           layout ของ Nuxt
│   ├── app/pages/             หน้าแรก
│   ├── app/components/
│   │   ├── PhotoKeepApp.vue   สถานะกลางและการเรียก Go
│   │   ├── LoadingState.vue   สถานะระหว่างเปิดคลังภาพ
│   │   ├── AppNavigation.vue  เมนู
│   │   ├── GalleryPanel.vue   คลังภาพและค้นหา
│   │   ├── SettingsPanel.vue  ตั้งค่า
│   │   ├── SetupDialog.vue    ตั้งค่าครั้งแรก
│   │   ├── PhotoDetailDialog.vue รายละเอียดรูป
│   │   └── FolderFields.vue   ช่องเลือกโฟลเดอร์ที่ใช้ซ้ำ
│   └── app/types/photo.ts     ชนิดข้อมูลของหน้าบ้าน
└── docs/reference/            โค้ด midterm เดิมเพื่อเทียบการต่อยอด
```

## อธิบายแต่ละส่วนสั้น ๆ

| คำถาม | เปิดไฟล์ | คำตอบ |
| --- | --- | --- |
| แอปเริ่มตรงไหน | `main.go` | สร้าง `App`, โหลด Nuxt ที่ build แล้ว และให้ Wails bind เมธอด Go |
| ฐานข้อมูลคืออะไร | `internal/photokeep/models.go`, `service.go` | เก็บตารางเดิม `files`, `settings` และตารางใหม่ `photos`; `NewService` เปิด SQLite และ AutoMigrate ผ่าน GORM |
| ตั้งค่าโฟลเดอร์อย่างไร | `SetupDialog.vue`, `settings.go` | ผู้ใช้เลือก source/dest; Go ตรวจว่า source มีจริงและโฟลเดอร์ไม่ซ้อนกันก่อนบันทึก |
| สำรองอย่างไร | `backup.go` | เดินไฟล์รูปใน source, คัดลอกไป dest โดยไม่ลบต้นฉบับ, ใช้ SHA-256 ข้ามไฟล์เดิม, เก็บข้อมูลภาพใน DB |
| แท็กมาจากไหน | `analysis.go` | สร้างแท็กชนิดไฟล์/ปี/แนวภาพ และตรวจตัวอย่างพิกเซลในเครื่องเพื่อหาโทนสี/ความสว่าง |
| ค้นหาอย่างไร | `GalleryPanel.vue`, `photos.go` | ช่องค้นหาจับชื่อไฟล์/คำอธิบาย; Tag Cloud ส่งแท็กให้ Go กรองรูปจาก SQLite |
| แก้คำอธิบาย/แท็กอย่างไร | `PhotoDetailDialog.vue`, `photos.go` | แก้ใน dialog แล้วเรียก `UpdatePhoto` ให้ GORM บันทึก |

## ตัวอย่างเล่าเส้นทางคำสั่ง Backup

1. ผู้ใช้กดปุ่มสำรองบน `PhotoKeepApp.vue`; หน้านี้เรียก `Backup()` ผ่าน Wails binding
2. `backup.go` อ่านค่าต้นทาง/ปลายทางที่ `settings.go` บันทึกใน SQLite
3. `backup.go` เดินไฟล์รูป, คัดลอก, คำนวณ SHA-256 และเรียก `analysis.go` สร้างแท็ก พร้อมส่งสถานะทุกไฟล์ผ่าน callback
4. `app.go` ส่งสถานะผ่าน Wails event `backup:progress` ให้ `BackupProgressDialog.vue` แสดงจำนวนไฟล์ ชื่อไฟล์ และเวลาที่ใช้ทันที
5. GORM บันทึก `Photo` และ `FileRecord` ใน SQLite
6. ผลจำนวนรูปที่เพิ่ม/ข้าม/ผิดพลาดกลับไป `PhotoKeepApp.vue`; จากนั้นค้นหาใหม่และ `GalleryPanel.vue` แสดงรูป

โค้ด midterm อยู่ใน `docs/reference/midterm-main.go.txt` เพื่อชี้ให้เห็นส่วนที่นำมาต่อยอด โดยแอป Final เน้นการสำรองรูปตามโจทย์ ไม่ได้ย้ายคำสั่ง CLI เดิมทุกคำสั่งขึ้น UI
