# PhotoKeep — Final Project

แอปเดสก์ท็อปสำหรับสำรองรูปภาพจากแฟลชไดรฟ์ โทรศัพท์ที่เชื่อมต่อเป็นโฟลเดอร์ หรือโฟลเดอร์รูปทั่วไป พัฒนาต่อจากโปรแกรม Go CLI ของ midterm โดยคงแนวคิด `source` / `dest` / `files` / `settings` และเพิ่ม Wails, Nuxt, คลังรูปภาพ, คำอธิบาย และแท็ก

## ความสามารถตามโจทย์

| เกณฑ์ | ส่วนที่ทำ |
| --- | --- |
| Wails + Nuxt บน Windows/macOS | Wails v2 + Nuxt 4 แบบ static SPA |
| SQLite + GORM | `crossplatform.db` ในโฟลเดอร์ config ของผู้ใช้; AutoMigrate ตาราง `files`, `settings`, `photos` |
| Backup | เลือกต้นทาง/ปลายทาง, สแกนโฟลเดอร์ย่อย, คัดลอกภาพโดยไม่ลบต้นฉบับ, ข้ามภาพเดิมจาก SHA-256, เปลี่ยนชื่อเมื่อปลายทางมีไฟล์ชื่อซ้ำ, บันทึกประวัติ |
| วิเคราะห์ภาพ | ส่งภาพผ่าน AI gateway ที่แยกจากแอป โดยค่าเริ่มต้นใช้ xKiro/Mistral Medium 3.5; บันทึกคำบรรยายไทยและแท็กใน SQLite |
| ค้นหา/แสดงภาพ | แกลเลอรีหลายรูป ค้นจากชื่อไฟล์/คำอธิบาย เลือกแท็กจาก Tag Cloud ดูภาพและรายละเอียด |

รองรับ JPG, PNG, GIF, WebP, BMP, TIFF รูป HEIC/AVIF ต้องแปลงเป็นรูปแบบที่รองรับก่อนนำเข้า คำอธิบายและแท็กจาก AI จะถูกจัดเก็บในฐานข้อมูลเดิมและค้นหาผ่านระบบค้นหาเดิมได้

ระบบค้นหาทำงานในเครื่องผ่าน SQLite FTS5 และตัวตัดคำภาษาไทย จากชื่อไฟล์ คำอธิบาย และแท็กที่บันทึกไว้ ไม่ต้องเรียก AI ตอนค้นหา งานวิเคราะห์เริ่มหลังสำรองภาพครบและสามารถพักหรือทำต่อได้

### โครงสร้าง AI และไฟล์หลัก

- `services/ai-gateway/`: gateway ที่ให้แอปเรียกผ่าน API วิเคราะห์ภาพเส้นเดียว
- `scripts/`: สคริปต์รันและสร้างชุด gateway; [คู่มือย้ายไปรันอีกเครื่อง](docs/remote-ai-gateway-source.md)
- `frontend/dist/`, `frontend/.nuxt/`, `frontend/.output/`, `build/` และ `dist/` เป็นไฟล์สร้างอัตโนมัติหรือแพ็กเกจพร้อมใช้ ไม่ใช่ source ที่แก้โดยตรง

แอปใช้ gateway ที่ `127.0.0.1:8081` โดยค่าเริ่มต้น และเปลี่ยนได้ด้วย `PHOTOKEEP_AI_GATEWAY_URL` คีย์ผู้ให้บริการอยู่ฝั่ง gateway เท่านั้น

การตั้งค่าแอปอยู่ใน `.env` (คัดลอกจาก `.env.example`); คีย์ AI สำหรับ gateway อยู่ใน `.env.gateway` (คัดลอกจาก `scripts/ai-gateway.env.example`) และไม่ควรนำขึ้น Git เอกสารโครงการเพิ่มเติมอยู่ใน `docs/project/`.

## ใช้งาน

1. เปิดแอปครั้งแรก จะมีหน้าต่างให้เลือกโฟลเดอร์ทันที
2. เลือกโฟลเดอร์ต้นทางและปลายทาง กด **บันทึกและเริ่มใช้** (เปลี่ยนภายหลังได้ที่ **ตั้งค่า**)
3. กด **สำรองรูป** จะได้สำเนาใน `<ปลายทาง>/PhotoKeep/<ชื่อต้นทาง>/...` โดยเก็บโครงสร้างโฟลเดอร์ย่อย
4. หน้าแรกแสดงรูปของโฟลเดอร์ที่กำลังใช้อยู่ เลือกแท็บ **ทั้งหมด** เพื่อดูรูปจากโฟลเดอร์เดิม คลิกรูปเพื่อแก้คำอธิบายหรือแท็ก แล้วใช้ช่องค้นหาหรือ Tag Cloud
5. หากเพิ่มรูปในโฟลเดอร์ต้นทางภายหลัง กด **รีเฟรช** เพื่อตรวจและสำรองรูปใหม่เข้าคลัง ระบบจะข้ามรูปที่เคยสำรองแล้ว

การเชื่อมต่อโทรศัพท์ต้องให้ระบบปฏิบัติการแสดงไฟล์เป็นโฟลเดอร์ก่อน เช่น อุปกรณ์ที่ mount เป็นไดรฟ์ แอปไม่เข้าถึงคลังภาพใน iPhone/Android ผ่านโปรโตคอลเฉพาะโดยตรง

หน้าตาแอปใช้ Liquid Glass โทนเขียวธรรมชาติ แผงโปร่งใส แสงนุ่ม และกรอบเหลี่ยมคมที่ยังอ่านตัวอักษรได้ชัด หน้าแรกมีแถบคำสั่งที่แสดงเพียงปุ่มสำรองและรีเฟรช แล้วตามด้วยคลังภาพ หน้าตั้งค่าอยู่กลางจอและมีเมนูด้านบนที่ปรับตามขนาดจอ ส่วนคลังภาพแยกรูปของโฟลเดอร์ปัจจุบันกับรูปทั้งหมด ขณะสำรองจะแสดงหน้าต่างกลางจอพร้อมวงแหวนความคืบหน้าจริง ชื่อไฟล์ จำนวนสำเร็จ/ข้าม/ผิดพลาด และเวลาที่ใช้ เมื่อเสร็จหรือเกิดข้อผิดพลาด หน้าต่างจะค้างจนกดปุ่มตกลงเอง การรีเฟรชแสดงผลผ่าน toast โดยไม่เปิดหน้าต่างความคืบหน้า มีสถานะโหลดสำหรับเปิดคลังภาพ ค้นหา โหลดภาพ และบันทึก พร้อมรองรับการลดภาพเคลื่อนไหวตามการตั้งค่าระบบ

## พัฒนาและ build

ต้องมี Go 1.26+, Node.js 22+, npm และ Wails CLI ตาม [คู่มือ Wails](https://wails.io/docs/gettingstarted/installation) บน Windows ต้องมี WebView2; บน macOS ต้องมี Xcode Command Line Tools

```sh
cd frontend
npm install --legacy-peer-deps
npm run typecheck
npm run generate
cd ..
go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 dev
```

สร้างไฟล์แอป:

```sh
go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 build
```

ผล build อยู่ที่ `build/bin/` โปรเจกต์นี้มีตัวแอป macOS arm64 และ Windows x64 ที่ build แล้วในชุดส่งงาน ตัว Windows ผ่านการ cross compile แต่ยังไม่ได้ทดสอบเปิดบนเครื่อง Windows จริง

## โครงสร้างและความต่อเนื่องจาก midterm

- [แผนผังโค้ดและแนวทางอธิบาย](docs/ARCHITECTURE.md): ดูลำดับการทำงานจาก Nuxt → Wails → Go → SQLite และไฟล์ที่รับผิดชอบแต่ละส่วน
- `main.go`: เปิดแอป Wails และ bind เมธอด Go กับหน้า Nuxt
- `app.go`: ตัวเชื่อม Wails ที่ส่งคำสั่งไป `Service` และเปิดตัวเลือกโฟลเดอร์ของระบบปฏิบัติการ
- `internal/photokeep/models.go`: โครงสร้างข้อมูล GORM และผลลัพธ์ API
- `internal/photokeep/service.go`: เปิด SQLite, AutoMigrate, อ่านสถานะ
- `internal/photokeep/settings.go`: ตรวจสอบและบันทึกโฟลเดอร์ต้นทาง/ปลายทาง
- `internal/photokeep/backup.go`: สแกนรูป คัดลอกอย่างปลอดภัย ตรวจไฟล์ซ้ำ และบันทึกประวัติ
- `internal/photokeep/analysis.go`: วิเคราะห์แนวภาพ สี และความสว่างเพื่อสร้างแท็ก
- `internal/photokeep/photos_search.go`, `search_terms.go`: ค้นหาและจัดอันดับรูป
- `internal/photokeep/photos_metadata.go`: แก้คำบรรยายและแท็ก
- `internal/photokeep/photos_preview.go`: โหลดภาพจริงและสร้างภาพย่อ
- `frontend/app/app.vue`: จุดเริ่มของ Nuxt ที่เรียก Layout และ Page
- `frontend/app/layouts/default.vue`: โครง Layout หลัก
- `frontend/app/pages/index.vue`: หน้าแรกของแอป
- `frontend/app/components/PhotoKeepApp.vue`: คุมสถานะ เรียก Wails binding และประกอบหน้าจอ
- `frontend/app/components/shared/LoadingState.vue`: skeleton ระหว่างเปิดคลังภาพ
- `frontend/app/components/shared/AppNavigation.vue`: แถบนำทางด้านบนที่ปรับตามขนาดจอ
- `frontend/app/components/gallery/GalleryPanel.vue`: แกลเลอรี ค้นหา และ Tag Cloud
- `frontend/app/components/gallery/PhotoDetailDialog.vue`: ดูภาพและแก้คำบรรยาย/แท็ก
- `frontend/app/components/settings/SettingsPanel.vue`: หน้าตั้งค่าโฟลเดอร์
- `frontend/app/components/settings/OpenVSXThemePanel.vue`: ค้นหาและจัดการธีม
- `frontend/app/components/backup/SetupDialog.vue`: หน้าต่างตั้งค่าครั้งแรก
- `frontend/app/components/backup/BackupProgressDialog.vue`: แสดงขั้นตอนและความคืบหน้าสำรอง
- `frontend/app/components/ai/ImageAILogsPanel.vue`: ประวัติงานและข้อผิดพลาดจาก AI
- `frontend/app/components/shared/FolderFields.vue`: ช่องโฟลเดอร์ที่ใช้ร่วมกัน
- `frontend/app/composables/usePhotoKeepTheme.ts`: เลือก ใช้ และบันทึกธีม
- `frontend/app/types/photo.ts`: ชนิดข้อมูลที่คอมโพเนนต์ Nuxt ใช้ร่วมกัน
- `frontend/app/assets/css/main.css`: สไตล์พื้นฐานของคอมโพเนนต์และเลย์เอาต์
- `frontend/app/assets/css/theme.css`: ธีมกระจกฝ้าโทนเขียวและเลย์เอาต์ที่ปรับตามขนาดจอ
- `frontend/scripts/copy-dist.mjs`: คัดลอก static build ไป `frontend/dist` ด้วย Node จึงใช้ได้ทั้ง Windows และ macOS

ไฟล์ฐานข้อมูลเดิมของ midterm มีเฉพาะประวัติชื่อไฟล์ ไม่มีภาพหรือคำอธิบาย จึงไม่ได้แสดงประวัตินั้นในแกลเลอรีใหม่ หากนำ `crossplatform.db` เดิมมาวางในโฟลเดอร์ config ของ PhotoKeep ก่อนเปิดแอป ตารางเดิมจะถูกเก็บไว้และสร้างตาราง `photos` เพิ่มโดย AutoMigrate

## วิเคราะห์ภาพด้วย AI server

แอปสามารถส่งภาพไปยัง AI server ที่กำหนดไว้เพื่อสร้างคำอธิบายและแท็กเมื่อผู้ใช้เริ่มงานเอง ระหว่างนี้การสำรองภาพจะเริ่มจัดทำดัชนีค้นหาภาพในเครื่องโดยอัตโนมัติ

## สาธิต 5 นาที

1. แสดงหน้า UI และตั้งค่าต้นทางเป็น `demo/sample-source`, ปลายทางเป็นโฟลเดอร์ว่างอื่น (1 นาที)
2. สำรองชุดตัวอย่าง 5–10 รูปจากแฟลชไดรฟ์หรือโฟลเดอร์ (1 นาที)
3. เปิดรูป แสดงขนาด/แนว/วันที่ แล้วแก้คำอธิบายและแท็ก (1 นาที)
4. ค้นคำอธิบาย และกด Tag Cloud เพื่อกรองหลายรูป (1 นาที)
5. สาธิตสำรองซ้ำแล้วระบบข้ามรูปเดิม เปิดปลายทางยืนยันต้นฉบับยังอยู่ (1 นาที)
