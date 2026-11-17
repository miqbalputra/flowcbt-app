# PRD — FlowCBT

**Computer Based Test (CBT) Ujian Online Multi-Sekolah, Berbayar (SaaS)**

| Item | Detail |
|---|---|
| Nama Produk | FlowCBT |
| Versi Dokumen | 1.3 (final untuk implementasi v1; + skema DB, struktur proyek, rencana sprint) |
| Tanggal | 15 November 2026 |
| Stack | Backend: Golang · Database: PostgreSQL · Frontend: React |
| Model Bisnis | SaaS multi-tenant, langganan per ujian atau per bulan |
| Platform | Web responsif (desktop & mobile), PWA (installable, offline-capable) |

---

## Daftar Isi

1. Ringkasan Produk
2. Persona, Peran & Matriks Hak Akses
3. Model Bisnis, Langganan & Retensi Data
4. Ruang Lingkup & Prioritas Fitur
5. Kebutuhan Fungsional (M1–M13)
6. Kebutuhan Non-Fungsional
7. Arsitektur Teknis & Infrastruktur (VPS + Coolify)
8. Model Data
9. Rancangan API
10. Alur Pengguna & Kriteria Penerimaan
11. Spesifikasi UI/UX & Peta Halaman
12. Keamanan & Integritas Ujian
13. Metrik Keberhasilan
14. Rencana Rilis & Strategi Pengujian
15. Risiko & Mitigasi
16. Asumsi & Ketergantungan
17. Keputusan Produk (Final v1)
18. Lampiran

---

## 1. Ringkasan Produk

FlowCBT adalah platform ujian berbasis komputer (CBT) yang dapat digunakan oleh sekolah/lembaga dari jenjang apa pun (PAUD sampai perguruan tinggi, bimbel, pesantren, PKBM, kursus). Guru/admin mendaftar sendiri, membuat soal dengan format AKM, menjadwalkan ujian, dan memantau pelaksanaan secara real-time. Siswa mengerjakan ujian lewat browser di HP atau komputer, **tetap bisa mengerjakan saat koneksi terputus** dan jawaban akan **tersinkronisasi otomatis** saat online kembali.

### 1.1 Masalah yang Diselesaikan
1. Ujian berbasis kertas mahal, lambat dikoreksi, dan rekap nilai manual.
2. Aplikasi CBT yang ada sering rumit, bergantung pada jaringan stabil, dan tidak ramah mobile.
3. Sekolah kecil tidak sanggup membeli/mengelola server CBT sendiri.
4. Riwayat nilai siswa tersebar dan sulit dilacak antar-ujian.

### 1.2 Tujuan (Goals)
- **G1** — Guru dapat membuat dan mempublikasikan ujian lengkap dalam < 10 menit.
- **G2** — Siswa dapat menyelesaikan ujian tanpa kehilangan jawaban meskipun koneksi putus (zero answer loss).
- **G3** — Pemantauan real-time dan rekap nilai otomatis, tanpa koreksi manual untuk soal objektif.
- **G4** — Model berlangganan yang mudah dibeli siapa saja (self-service) dengan isolasi data antar sekolah.
- **G5** — UI 100% nyaman di mobile dan desktop, mendukung dark/light mode.

### 1.3 Non-Goals (v1)
- Proctoring AI lanjutan (pengenalan identitas wajah, deteksi suara/objek). v1 hanya snapshot kamera + deteksi wajah dasar (lihat FR-5.8).
- White-label (logo/domain sendiri per sekolah) — **tidak dibutuhkan**.
- Bank soal bersama / marketplace antar guru — **tidak dibutuhkan**.
- Aplikasi native Android/iOS (cukup PWA).
- Video conference / kelas online.
- Koreksi uraian otomatis berbasis AI (roadmap; v1 koreksi manual oleh guru).

---

## 2. Persona, Peran & Matriks Hak Akses

| Peran | Deskripsi | Hak Akses Utama |
|---|---|---|
| **Super Admin (Platform)** | Tim FlowCBT | Kelola tenant, paket, harga, pembayaran, monitoring platform |
| **Admin Sekolah (Owner)** | Pendaftar pertama/penanggung jawab langganan | Semua fitur tenant, kelola guru, billing |
| **Guru** | Pembuat soal & pengawas | Kelola soal, ujian, siswa pada kelas yang ditugaskan, laporan |
| **Siswa** | Peserta ujian | Mengerjakan ujian yang ditugaskan, lihat riwayat & nilai (jika diizinkan) |

> **Catatan:** Guru yang mendaftar mandiri otomatis menjadi Admin Sekolah dari tenant barunya (tenant personal/lembaga).

### 2.1 Matriks Hak Akses

| Kemampuan | Super Admin | Admin Sekolah | Guru | Siswa |
|---|:-:|:-:|:-:|:-:|
| Kelola tenant, paket, harga | ✅ | — | — | — |
| Beli/kelola langganan & invoice | — | ✅ | — | — |
| Undang/kelola guru | — | ✅ | — | — |
| Kelola kelas & siswa (import/export) | — | ✅ | ✅ (kelas yang ditugaskan) | — |
| Buat/ubah soal & ujian | — | ✅ | ✅ (milik sendiri; bisa dibagikan ke guru lain di tenant) | — |
| Pantau ujian real-time & aksi pengawas | — | ✅ | ✅ | — |
| Nilai uraian & putuskan laporan curang | — | ✅ | ✅ | — |
| Lihat laporan & track record siswa | — | ✅ | ✅ (kelas yang ditugaskan) | Hanya miliknya (sesuai izin ujian) |
| Mengerjakan ujian | — | — | — | ✅ |
| Export semua data tenant | — | ✅ | — | — |

---

## 3. Model Bisnis, Langganan & Retensi Data

### 3.1 Paket & Harga

**Paket Gratis (sekaligus Trial)**

| Paket | Jumlah Siswa | Harga | Kuota Ujian | Masa Berlaku |
|---|---|---|---|---|
| Gratis / Trial | 1–10 | Rp0 | 1x ujian | 14 hari sejak daftar |

**Paket Per Ujian (bayar sekali, bundel kuota ujian)**

| Paket | Jumlah Siswa | Harga | Kuota Ujian |
|---|---|---|---|
| Per Ujian — Kecil | 11–40 | Rp10.000 | 3x ujian |
| Per Ujian — Menengah | 41–70 | Rp20.000 | 7x ujian |
| Per Ujian — Besar | 71–100 | Rp30.000 | 12x ujian |

**Paket Bulanan (ujian tanpa batas)**

| Paket | Jumlah Siswa | Harga |
|---|---|---|
| Bulanan — Menengah | 1–50 | Rp50.000 / bulan |
| Bulanan — Besar | 51–100 | Rp70.000 / bulan |

Semua harga dan kuota disimpan sebagai **konfigurasi** (tabel `plans`), bisa diubah Super Admin tanpa deploy ulang. Rentang jumlah siswa 1–100 tercakup penuh tanpa celah antar-tier.

**Ketentuan paket:**
- Batas jumlah siswa = jumlah siswa aktif terdaftar di tenant (termasuk peserta tamu yang sudah dibuat).
- "1x ujian" = 1 Ujian (exam session) yang dipublikasikan, berlaku untuk semua peserta dalam batas tier.
- Paket per ujian dibeli sesuai tier jumlah siswa saat pembelian.

### 3.2 Aturan Bisnis
- **Fitur lengkap di semua paket**, termasuk anti-cheat/proctoring. Pembeda hanya jumlah siswa dan kuota ujian.
- Ujian hanya bisa **dipublikasi** bila ada kuota aktif (kredit ujian tersisa, langganan bulanan aktif, atau trial). Kredit terpakai saat publish; membatalkan publish sebelum ada yang mengerjakan mengembalikan kredit.
- Ujian yang **sedang berlangsung tidak boleh terputus** akibat kuota/langganan habis (grace sampai ujian ditutup).
- **Retensi data 7 hari:** tenant masuk mode **baca-saja** selama **7 hari** ketika hak akses berakhir dan tidak diperpanjang. Hitungan 7 hari dimulai pada waktu yang **lebih akhir** antara (a) berakhirnya hak akses (trial habis, langganan bulanan berakhir, atau kredit ujian habis) dan (b) `close_at` ujian terakhir, sehingga hasil ujian selalu tersedia minimal 7 hari setelah ujian terakhir ditutup. Setelah itu data tenant (siswa, soal, hasil, snapshot kamera) **dihapus permanen**.
  - Pengingat email saat berakhir, H+3, dan H+6 beserta tombol **Export Semua Data** (siswa, nilai, soal) yang tersedia selama masa baca-saja.
  - Perpanjangan dalam 7 hari memulihkan akses penuh tanpa kehilangan data.
  - Sebelum dihapus, job penghapusan memverifikasi bahwa tidak ada transaksi/perpanjangan yang tertunda.
- Pembayaran lewat **Midtrans dan Mayar** (pelanggan memilih saat checkout). Invoice otomatis via email.
- **Tier mengikuti jumlah siswa.** Menambah siswa yang melewati batas tier ditolak dengan ajakan upgrade.
- **Upgrade** bulanan: bayar selisih harga dengan prorata sisa hari. **Downgrade** berlaku periode berikutnya dan hanya bila jumlah siswa ≤ batas tier tujuan. Kredit per ujian tier lebih rendah tidak bisa dipakai bila siswa melebihi tier tersebut; tenant membayar selisih harga paket.
- **Kredit per ujian tidak kedaluwarsa.** Bila langganan bulanan aktif dan kredit juga ada, sistem memakai langganan bulanan lebih dulu sehingga kredit tidak terpotong.
- **Di atas 100 siswa:** belum ada paket di v1; tampilkan "Hubungi kami"/daftar tunggu dan blokir penambahan siswa ke-101.
- **Paket gratis sekali per tenant.** Anti-penyalahgunaan: satu email terverifikasi, deteksi duplikasi nama lembaga/nomor HP.
- **Penyimpanan media** (gambar/audio soal) per tenant: default 1 GB (konfigurasi). Jumlah guru tidak dibatasi.

### 3.3 Kebijakan Retensi Data (Ringkasan)

| Data | Selama akses aktif | Setelah akses berakhir |
|---|---|---|
| Siswa, kelas, soal, ujian, hasil, track record | Disimpan | Baca-saja 7 hari → hapus permanen |
| Snapshot kamera | 30 hari setelah ujian selesai | Ikut retensi tenant bila lebih pendek |
| Event pelanggaran & log pengerjaan | Disimpan bersama hasil | Ikut hasil |
| Audit log | 12 bulan | Ikut retensi tenant |
| Invoice & transaksi pembayaran | Disimpan (kebutuhan akuntansi/pajak) | Disimpan, hanya data penagihan tanpa data siswa |
| Backup database | 14 hari bergulir | Data tenant yang dihapus ikut hilang dari backup setelah siklus bergulir selesai |

### 3.4 Catatan Unit Economics
- Biaya payment gateway untuk transaksi kecil (mis. Rp10.000) relatif besar terhadap nominalnya; tinjau margin per paket setelah beta dengan angka biaya aktual dari Midtrans dan Mayar.
- Paket bulanan baru lebih hemat daripada paket per ujian bagi sekolah yang rutin ujian banyak; pantau komposisi pembelian untuk menyesuaikan harga lewat konfigurasi.
- Biaya variabel utama: penyimpanan snapshot kamera/media, bandwidth, dan email; pantau per tenant.

---

## 4. Ruang Lingkup & Prioritas Fitur

### 4.1 Ringkasan Modul

| # | Modul | Pengguna |
|---|---|---|
| M1 | Autentikasi, Registrasi & Multi-Tenant | Semua |
| M2 | Dashboard Guru/Admin | Guru, Admin |
| M3 | Manajemen Siswa & Kelas | Guru, Admin |
| M4 | Bank Soal & Pembuat Soal (AKM) | Guru, Admin |
| M5 | Pengaturan & Penjadwalan Ujian | Guru, Admin |
| M6 | Antarmuka Ujian Siswa (CBT Client) | Siswa |
| M7 | Mode Offline & Sinkronisasi | Siswa |
| M8 | Penilaian | Sistem, Guru |
| M9 | Laporan Real-Time & Historis | Guru, Admin |
| M10 | Billing & Langganan | Admin, Super Admin |
| M11 | Tema (Dark/Light) & UX Responsif | Semua |
| M12 | Super Admin Console | Super Admin |
| M13 | Notifikasi & Email | Semua |

### 4.2 Prioritas Fitur

| Prioritas | Cakupan |
|---|---|
| **P0 — MVP (beta tertutup)** | Registrasi/login, multi-tenant; siswa & kelas + import/export; editor soal 5 tipe AKM + draft/publish + import/export template; pengaturan ujian & token; UI siswa (butir soal, daftar soal, selesai); mode offline + sinkronisasi; penilaian otomatis + koreksi uraian; laporan real-time & historis dasar + track record siswa; dark/light mode; deteksi keluar aplikasi dengan penalti; billing minimal (Midtrans; paket gratis, per ujian, bulanan) |
| **P1 — Rilis publik** | Login siswa tanpa akun + peserta tamu; proctoring kamera + laporan kecurangan; Mayar; analisis butir soal; export PDF; retensi 7 hari otomatis + Export Semua Data; notifikasi email lengkap; Super Admin console penuh |
| **P2 — Setelah rilis** | Auto-renew, kupon, paket tahunan; i18n Inggris/Arab (RTL); proctoring AI lanjutan; penilaian uraian berbantuan AI; paket di atas 100 siswa |

---

## 5. Kebutuhan Fungsional

### M1 — Autentikasi, Registrasi & Multi-Tenant

**FR-1.1 Registrasi Admin/Guru (Form).** Form: nama lengkap, email, nomor HP (opsional), password, nama sekolah/lembaga, jenjang (PAUD/TK, SD/MI, SMP/MTs, SMA/SMK/MA, Perguruan Tinggi, Kursus/Bimbel, Lainnya). Verifikasi email wajib sebelum akses penuh.

**FR-1.2 Login Admin/Guru.** Email + password; opsi *Login with Google*; lupa/reset password via email; rate limiting dan lockout sementara setelah percobaan gagal.

**FR-1.3 Login Siswa (mendukung dua metode sekaligus).** Guru memilih per ujian: *akun saja*, *tanpa akun saja*, atau *keduanya* (default).
1. **Dengan akun:** username/NIS + password (dibuat otomatis saat import, dapat direset guru).
2. **Tanpa akun:** siswa mengisi **data diri** (nama lengkap, jenis kelamin, kelas, NIS bila diwajibkan guru) + **kode ujian + token**.
   - Sistem mencocokkan ke data siswa terdaftar (NIS, atau nama + kelas ternormalisasi). Cocok → attempt tertaut ke siswa tersebut dan masuk track record-nya.
   - Tidak cocok → dibuat **peserta tamu** (`is_guest = true`) dengan ID unik. Guru dapat **menggabungkan (merge)** peserta tamu ke siswa terdaftar agar track record menyatu.
   - Satu identitas hanya boleh punya satu sesi aktif per ujian; sesi tamu diikat ke perangkat (cookie httpOnly) agar bisa dilanjutkan, termasuk setelah offline.
   - Rate limiting pada kode/token untuk mencegah tebak-tebakan.

**FR-1.4 Multi-Tenant.** Setiap sekolah = 1 tenant. Seluruh data (siswa, soal, ujian, hasil) terisolasi per `tenant_id` (enforced di query layer dan Postgres Row-Level Security).

**FR-1.5 Manajemen Guru.** Admin dapat mengundang guru via email, mengatur peran (Admin/Guru), dan membatasi akses guru pada kelas/mapel tertentu.

**FR-1.6 Sesi & Keamanan.** JWT access token (singkat) + refresh token (rotasi); satu siswa hanya boleh punya **satu sesi aktif** per ujian (login ganda ditolak/menggantikan sesi lama sesuai pengaturan).

---

### M2 — Dashboard Guru/Admin

**FR-2.1** Ringkasan: ujian berjalan saat ini, ujian terjadwal, jumlah siswa, rata-rata nilai ujian terakhir, status langganan & sisa kuota.
**FR-2.2** Pintasan cepat: Buat Soal, Buat Ujian, Import Siswa, Lihat Laporan.
**FR-2.3** Navigasi utama: *Siswa · Soal · Ujian · Laporan · Langganan · Pengaturan*.

---

### M3 — Manajemen Siswa & Kelas

**FR-3.1 CRUD Kelas.** Nama kelas (mis. "X IPA 1"), tahun ajaran, wali kelas (opsional). Kelas dapat diarsipkan.

**FR-3.2 CRUD Siswa.** Field wajib: **Nama, Jenis Kelamin (L/P), Kelas**. Field opsional: NIS/NISN, email, nomor HP wali, foto. Siswa memiliki **ID unik** (UUID internal + kode siswa) yang menjadi kunci track record sepanjang waktu.

**FR-3.3 Import Siswa.**
- Unduh template (.xlsx/.csv) dengan kolom: `Nama`, `Jenis Kelamin`, `Kelas` (+ opsional `NIS`).
- Upload → **preview & validasi** (baris error ditandai, mis. gender tidak valid, kelas kosong, duplikat) → konfirmasi.
- Kelas yang belum ada dapat dibuat otomatis (opsi).
- Pembuatan username/password awal otomatis; dapat diekspor untuk dibagikan (kartu login siswa PDF).

**FR-3.3b Batas Kuota Siswa.** Import/penambahan siswa dan pembuatan peserta tamu memeriksa batas tier tenant. Jika melewati batas, proses ditolak (import menampilkan sisa kuota) disertai tombol Upgrade.

**FR-3.4 Export Siswa.** Export daftar siswa ke .xlsx/.csv (filter kelas).

**FR-3.5 Pindah/Naik Kelas.** Aksi massal ubah kelas; **riwayat nilai tetap melekat pada siswa** meskipun kelas berubah (snapshot kelas disimpan pada tiap hasil ujian).

**FR-3.6 Rekapitulasi Nilai per Ujian.** Tabel nilai seluruh siswa untuk satu ujian (nama, kelas, nilai, status, durasi, waktu submit); filter kelas; export .xlsx/.csv/PDF.

**FR-3.7 Tracking Siswa Tertentu.** Halaman profil siswa berisi:
- Daftar seluruh ujian yang pernah diikuti: mata pelajaran, tanggal, percobaan ke-, nilai, durasi, status.
- Grafik tren nilai (per mapel / keseluruhan).
- Detail per percobaan: jawaban tiap soal, benar/salah, skor per soal, waktu pengerjaan, riwayat event (mulai, putus koneksi, sinkron, submit).
- Export profil/track record ke PDF/.xlsx.

---

### M4 — Bank Soal & Pembuat Soal (Format AKM)

**FR-4.1 Paket Soal (Question Set).**
- Field: **Nama mata pelajaran**, judul paket, deskripsi, jenjang/kelas (tag), **kode ujian** (unik per tenant, bisa digenerate otomatis), status (**Draft/Published**).
- Dapat diduplikasi, diarsipkan, dan dicari.

**FR-4.2 Target Peserta ("Untuk siapa").** Penugasan ujian dapat berupa:
1. **Satu kelas tertentu**
2. **Beberapa kelas sekaligus**
3. **Siswa tertentu** (pilih manual / multi-select / import daftar)
Kombinasi diperbolehkan (mis. kelas A + 3 siswa remedial).

**FR-4.3 Tipe Soal (AKM).**

| Tipe | Deskripsi | Penilaian Otomatis |
|---|---|---|
| **Pilihan Ganda** | Pilih satu jawaban benar | Ya (benar/salah) |
| **Pilihan Ganda Kompleks** | Pilih lebih dari satu jawaban benar | Ya (semua-atau-tidak, atau parsial — dipilih guru) |
| **Menjodohkan** | Pasangkan item kiri dengan item kanan (drag-drop/tarik garis di desktop; tap-pilih pasangan di mobile) | Ya (semua benar, atau parsial per pasangan) |
| **Isian Singkat** | Jawaban angka/kata/nama singkat | Ya (kunci jawaban multi-variasi, case-insensitive, normalisasi spasi/angka) |
| **Uraian** | Jawaban kalimat panjang | Manual oleh guru (rubrik skor) |

**FR-4.4 Editor Soal.**
- Rich text: bold, italic, daftar, tabel sederhana, **rumus matematika (LaTeX/KaTeX)**, gambar, dan audio (opsional, untuk listening).
- Teks stimulus/wacana yang dapat dipakai bersama beberapa soal (khas AKM literasi).
- Bobot/poin per soal; opsi **acak urutan soal** dan **acak urutan pilihan**.
- Pembahasan/penjelasan jawaban (opsional, ditampilkan ke siswa jika diizinkan).
- Pratinjau tampilan siswa (desktop & mobile) sebelum publish.
- Drag & drop untuk mengurutkan soal; duplikasi & hapus soal.
- **Autosave** draft setiap perubahan.

**FR-4.5 Import Template Soal.** Unduh template .xlsx per tipe soal → isi → upload → preview & validasi → simpan sebagai draft. Gambar via URL atau upload zip (opsional v1.1).

**FR-4.6 Export Template Soal yang Sudah Diisi.** Export paket soal ke .xlsx (format sama dengan template import) dan opsional ke PDF/Word (naskah soal + kunci jawaban).

**FR-4.7 Simpan Draft & Publish.**
- **Draft:** bisa diubah bebas, tidak terlihat siswa.
- **Publish:** validasi kelengkapan (setiap soal punya kunci jawaban, poin, dsb.), cek kuota langganan, lalu soal terkunci sebagian (perubahan setelah ada yang mengerjakan dibatasi/versioning, lihat FR-4.8).
- Unpublish diperbolehkan selama belum ada pengerjaan.

**FR-4.8 Versioning Soal.** Setiap publish membuat versi baru (`question_set_version`). Tiap percobaan siswa terikat ke versi tertentu agar riwayat nilai konsisten walau soal direvisi.

**FR-4.9 Token.** Token ujian (mis. 6 karakter alfanumerik) digenerate otomatis; dapat di-regenerate, diaktifkan/dinonaktifkan, dan opsional **token per sesi/ruang**. Siswa wajib memasukkan token untuk memulai.

---

### M5 — Pengaturan & Penjadwalan Ujian

Satu paket soal dapat dijadwalkan menjadi satu atau lebih **Ujian (Exam Session)**.

**FR-5.1 Jadwal Buka–Tutup.** `open_at` dan `close_at` (zona waktu tenant; default WIB, dapat diubah). Di luar rentang tersebut siswa tidak bisa memulai.

**FR-5.2 Durasi Pengerjaan.** Batas waktu (menit) sejak siswa menekan *Mulai* — tampil sebagai **timer hitung mundur** pada layar siswa. Waktu pengerjaan tidak boleh melewati `close_at` (opsi: *hard close* memotong waktu, atau *grace* sampai durasi habis).

**FR-5.3 Jumlah Percobaan.** Batas maksimal pengerjaan (1, 2, 3, … atau tak terbatas). Aturan nilai akhir bila lebih dari satu percobaan: **nilai tertinggi / terakhir / rata-rata**.

**FR-5.4 Tampilan Nilai.** Opsi: **nilai langsung muncul** setelah selesai, atau **disembunyikan** sampai guru merilis. Opsi tambahan: tampilkan/ sembunyikan pembahasan & kunci jawaban.

**FR-5.5 Pengaturan Tambahan.**
- Acak soal / acak opsi.
- Navigasi bebas atau satu arah (tidak bisa kembali).
- Wajib menjawab semua soal sebelum selesai (opsional).
- Anti-cheat & proctoring (deteksi keluar aplikasi, penalti nilai, kamera) — lihat **FR-5.8**.
- Tampilkan nilai KKM/ambang kelulusan.

**FR-5.6 Penugasan Peserta.** Sesuai FR-4.2; daftar peserta ujian dapat dilihat dan diubah selama ujian belum selesai.

**FR-5.7 Aksi Pengawas (Guru) saat Ujian.** Tambah waktu individu/semua, reset/izinkan ulang percobaan siswa tertentu, paksa selesai (force submit), keluarkan sesi aktif (reset login), jeda/tutup ujian darurat.

**FR-5.8 Proctoring & Anti-Cheat.** Dikonfigurasi per ujian oleh guru.

*A. Deteksi keluar/pindah aplikasi.*
- Dideteksi: tab/aplikasi berpindah (`visibilitychange`), jendela kehilangan fokus, keluar dari mode layar penuh, layar terkunci/aplikasi masuk background.
- Konfigurasi: aktif/nonaktif; **toleransi** (detik, default 3) agar gangguan singkat tidak langsung dihitung; **kebijakan** yang bisa digabung:
  1. **Peringatan** ke siswa ("Pelanggaran ke-n, sisa kesempatan m").
  2. **Pengurangan nilai** per pelanggaran (X poin atau X%, dengan batas penalti maksimum).
  3. **Dilaporkan curang** otomatis (`cheating_status = flagged`) setelah N pelanggaran.
  4. **Auto-submit** setelah N pelanggaran (opsional).
- Aturan ditampilkan di halaman tata tertib dan siswa wajib menyetujui sebelum mulai.
- Pelanggaran dicatat **meski offline** (event lokal → sinkron); penalti dihitung **di server** dari event yang tersinkron.
- Nilai ditampilkan transparan: *nilai asli, penalti, nilai akhir*. Guru dapat meninjau dan **membatalkan penalti/laporan** dengan alasan (masuk audit log).

*B. Proctoring kamera (opsional per ujian).*
- Layar persetujuan sebelum mulai (penggunaan kamera, tujuan, lama simpan). Untuk siswa di bawah umur, sekolah bertanggung jawab atas persetujuan wali.
- Mekanisme: **snapshot foto berkala acak** (default tiap 30–90 detik) dan snapshot saat terjadi pelanggaran, resolusi rendah (JPEG kecil). **Tidak ada rekaman video/audio.**
- Deteksi wajah ringan di sisi klien, hanya untuk **menandai** (tidak ada wajah, lebih dari satu wajah, kamera dimatikan). Tanda ini tidak mengurangi nilai otomatis kecuali guru mengaktifkannya; keputusan akhir ada di guru.
- Izin kamera ditolak → kebijakan guru: *tidak bisa mulai* atau *boleh mulai dan ditandai*.
- Snapshot disimpan terenkripsi di object storage, hanya bisa dilihat guru/admin tenant (akses tercatat di audit log), dihapus otomatis 30 hari setelah ujian selesai atau lebih cepat mengikuti retensi tenant (7 hari).
- Offline: snapshot diantrekan lokal dengan batas jumlah/ukuran, dikirim setelah jawaban tersinkron (jawaban selalu prioritas).
- Guru dapat menonaktifkan kamera untuk ujian di perangkat/jaringan terbatas.

*Nilai default (dapat diubah guru per ujian; disimpan sebagai konfigurasi):*

| Parameter | Default |
|---|---|
| Toleransi keluar aplikasi | 3 detik |
| Pengurangan nilai per pelanggaran | 5 poin (skala 100) |
| Batas maksimum total penalti | 30 poin |
| Ambang "Dilaporkan Curang" | 3 pelanggaran |
| Auto-submit | Nonaktif (opsional, mis. pada 5 pelanggaran) |
| Kamera | Nonaktif; bila diaktifkan guru, mode default **wajib** |
| Interval snapshot | Acak sekitar 60 detik (rentang 30–90 detik) |
| Penandaan wajah tidak terdeteksi/lebih dari satu | Hanya penanda, tanpa penalti otomatis |

*C. Laporan Kecurangan.* Halaman per ujian: siswa yang ditandai, jumlah dan linimasa pelanggaran, galeri snapshot, dan keputusan guru (**Valid / Batalkan penalti / Diskualifikasi** — nilai 0 atau attempt dibatalkan). Dapat diexport.

> **Catatan:** semua deteksi berbasis browser bersifat *deterrent*, bukan jaminan mutlak (mis. layar terbagi atau perangkat kedua tidak selalu terdeteksi).

---

### M6 — Antarmuka Ujian Siswa (CBT Client)

**FR-6.1 Dashboard Siswa.** Daftar ujian: *Sedang Dibuka*, *Akan Datang*, *Selesai* (dengan nilai bila diizinkan). Info per ujian: mapel, jadwal, durasi, sisa percobaan. Tombol **Mulai/Lanjutkan**.

**FR-6.2 Alur Mulai Ujian.** Pilih ujian → masukkan **token** → halaman petunjuk/tata tertib → konfirmasi *Mulai*. Server mencatat `started_at` dan menetapkan `deadline` (otoritas waktu ada di server).

**FR-6.3 Butir Soal (Halaman Soal).**
- Menampilkan satu soal per layar (opsi: scroll semua) dengan stimulus, gambar, dan rumus.
- Header: nama ujian, **timer**, indikator status koneksi (Online / Offline / Menyinkronkan), tombol tema.
- Tombol **Sebelumnya / Berikutnya**, **Ragu-ragu** (tandai), dan **Daftar Soal**.
- Input jawaban sesuai tipe soal (lihat FR-4.3), dengan target sentuh besar di mobile.
- Jawaban tersimpan otomatis setiap perubahan (lokal dulu, lalu sinkron).

**FR-6.4 Daftar Soal (Navigasi).**
- Grid nomor soal dengan status warna + ikon (tidak hanya warna, demi aksesibilitas): **belum dijawab, sudah dijawab, ragu-ragu, sedang dibuka**.
- Desktop: panel samping; mobile: bottom sheet/drawer.
- Klik nomor untuk lompat ke soal.

**FR-6.5 Selesai Mengerjakan.**
- Tombol **Selesai** → halaman **konfirmasi/review**: ringkasan jumlah terjawab, belum dijawab, dan ragu-ragu; peringatan jika masih ada yang kosong.
- Konfirmasi final → submit. Jika offline, status *"Menunggu sinkronisasi"* dan submit akan dikirim otomatis saat online.
- Setelah submit: layar selesai; tampilkan nilai langsung atau pesan "Nilai akan diumumkan guru" sesuai pengaturan.
- **Auto-submit** saat waktu habis.

**FR-6.6 Riwayat Siswa.** Siswa dapat melihat riwayat ujian dan nilai miliknya (sesuai izin pengaturan ujian).

---

### M7 — Mode Offline & Sinkronisasi

Persyaratan inti: **koneksi terputus → tetap bisa mengerjakan → otomatis sinkron saat online**.

**FR-7.1 PWA.** Aplikasi siswa berupa PWA (Service Worker + manifest), dapat di-install ke home screen, dan aset statis ter-cache sehingga halaman ujian tetap terbuka saat offline.

**FR-7.2 Pra-muat Soal.** Saat siswa menekan *Mulai* (dalam kondisi online), seluruh soal + media (gambar/audio) untuk percobaan itu diunduh dan disimpan di **IndexedDB**. Progress unduhan ditampilkan; ujian baru dimulai setelah paket soal lengkap tersimpan. Soal dikirim **tanpa kunci jawaban** (penilaian dilakukan di server).

**FR-7.3 Penyimpanan Jawaban Lokal.** Setiap perubahan jawaban ditulis ke IndexedDB sebagai **event/antrean** dengan: `client_event_id` (UUID), `attempt_id`, `question_id`, `answer_payload`, `client_timestamp`, `sequence_number`.

**FR-7.4 Sinkronisasi Otomatis.**
- Saat online, antrean dikirim **batch** ke server (`POST /attempts/{id}/answers:sync`) secara periodik (mis. tiap 5–10 dtk) dan segera saat koneksi pulih (event `online`, Background Sync bila didukung).
- Server **idempoten** berdasarkan `client_event_id` (kirim ulang aman, tanpa duplikasi).
- Resolusi konflik per soal: *last-write-wins* berdasarkan `sequence_number` (monoton per attempt), bukan jam perangkat.
- Retry dengan exponential backoff; antrean tidak dihapus sampai server mengkonfirmasi (ACK).

**FR-7.5 Timer saat Offline.** Timer berjalan di klien memakai jam monotonic (`performance.now`) yang dikalibrasi terhadap waktu server saat terakhir online. `deadline` ditetapkan server saat mulai. Saat kembali online, server memvalidasi: jawaban yang dibuat **sebelum deadline** (berdasar elapsed time klien yang dilaporkan + toleransi) diterima; di luar itu ditolak/ditandai.

**FR-7.6 Submit saat Offline.** Tombol *Selesai* menandai attempt "selesai lokal" dan mengunci input. Status: *"Jawaban tersimpan di perangkat — menunggu internet"*. Setelah sinkron sukses, status berubah menjadi *"Terkirim"*. Siswa **dilarang menutup/menghapus data browser** selama menunggu (peringatan jelas); jika tab ditutup, data tetap ada di IndexedDB dan dilanjutkan saat dibuka kembali.

**FR-7.7 Indikator Status.** Banner/ikon persisten: 🟢 Online & tersinkron, 🟡 Menyinkronkan (n jawaban menunggu), 🔴 Offline (jawaban tersimpan lokal). Pesan non-teknis, mudah dipahami siswa.

**FR-7.8 Pemulihan.** Bila perangkat mati/browser crash, siswa dapat login kembali dan **melanjutkan** attempt dari data lokal + data server (gabungan, server sebagai sumber akhir untuk yang sudah ter-ACK).

**FR-7.9 Keamanan Data Offline.** Paket soal di IndexedDB tidak berisi kunci jawaban; dibersihkan setelah attempt tersinkron penuh. Opsi enkripsi ringan (Web Crypto) untuk payload soal.

> **Batasan:** Login pertama, pengambilan paket soal, dan token wajib online. Setelah soal ter-cache, sisa pengerjaan dapat sepenuhnya offline.

---

### M8 — Penilaian

**FR-8.1 Penilaian Otomatis (di server).**
- PG: benar = poin penuh.
- PG Kompleks: mode *all-or-nothing* atau *parsial* (poin proporsional, dengan penalti jawaban salah opsional) — dipilih guru per soal.
- Menjodohkan: *all-or-nothing* atau parsial per pasangan.
- Isian Singkat: cocok dengan salah satu kunci (normalisasi: trim, lowercase, hapus spasi ganda; angka dengan toleransi format desimal `,`/`.`).
- Uraian: status *Menunggu penilaian*; guru memberi skor 0–maks poin + komentar.

**FR-8.1b Rumus Penilaian Parsial.**
- *PG Kompleks (parsial):* skor = poin × max(0, (jumlah benar dipilih − jumlah salah dipilih) ÷ jumlah kunci benar).
- *Menjodohkan (parsial):* skor = poin × (pasangan benar ÷ total pasangan).
- *PG, Isian Singkat, dan mode all-or-nothing:* poin penuh jika benar, 0 jika salah.
- Skor soal dibulatkan 2 desimal; nilai akhir dibulatkan sesuai pengaturan tenant (default 2 desimal).

**FR-8.2 Skor Akhir.** `nilai = (total poin diperoleh / total poin maksimum) × 100` (skala dapat dikonfigurasi). Untuk uraian yang belum dinilai, nilai berstatus **Sementara** sampai dinilai. Penalti pelanggaran (FR-5.8) diterapkan setelah skor dihitung.

**FR-8.3 Koreksi Manual.** Antarmuka guru untuk menilai uraian (per soal atau per siswa) dengan rubrik; guru juga dapat **override** skor soal otomatis (mis. isian singkat yang benar tapi beda penulisan) disertai alasan (audit log).

**FR-8.4 Re-grade.** Bila kunci jawaban dikoreksi, guru dapat menjalankan penilaian ulang untuk seluruh attempt pada versi soal tersebut.

---

### M9 — Laporan Ujian

#### 9A. Laporan Real-Time
**FR-9.1** Daftar **ujian yang sedang berjalan** (mapel, kelas, sisa waktu penutupan).
**FR-9.2** Untuk tiap ujian berjalan, tampilkan:
- Jumlah dan persentase: **Sedang mengerjakan · Belum mengerjakan · Sudah selesai** (dengan **grafik** donut/bar).
- Tabel/daftar siswa per status dengan **nama dan kelas**, progres soal terjawab (mis. 18/40), status koneksi (online/offline), dan waktu aktivitas terakhir.
- Filter per kelas dan pencarian nama.
**FR-9.3** Pembaruan real-time via **WebSocket/SSE** (fallback polling 10 dtk).
**FR-9.4** Aksi pengawas dari halaman ini (lihat FR-5.7).
**FR-9.5** Peringatan dan indikator pada daftar siswa: offline > N menit, jumlah pelanggaran keluar aplikasi, status *Dilaporkan Curang*, tanda kamera (tautan ke FR-5.8C).

#### 9B. Laporan Ujian Terdahulu
**FR-9.6** Daftar seluruh ujian yang telah selesai dengan **pilihan paket soal**.
**FR-9.7** **Pencarian & filter**: nama mata pelajaran, judul, kode ujian, kelas, rentang tanggal, status, guru pembuat; pencarian teks bebas.
**FR-9.8** Detail laporan per ujian: rata-rata, nilai tertinggi/terendah, median, tingkat kelulusan (vs KKM), distribusi nilai (histogram), durasi rata-rata.
**FR-9.9** **Analisis butir soal**: persentase benar per soal, tingkat kesukaran, daya pembeda (opsional), distraktor yang paling banyak dipilih.
**FR-9.10** **Track record per siswa** (terhubung FR-3.7): setiap siswa memiliki rekam jejak seluruh ujian sejak awal dengan **identitas unik per siswa**; dapat dibuka dari laporan mana pun.
**FR-9.11** Export laporan: .xlsx, .csv, PDF.

---

### M10 — Billing & Langganan

**FR-10.1** Halaman paket & harga; pembelian self-service (checkout via payment gateway).
**FR-10.2** Kredit ujian (paket per ujian) dan langganan bulanan; tampilkan status, sisa kuota, tanggal berakhir.
**FR-10.3** Riwayat transaksi, invoice PDF, email pengingat sebelum berakhir (H-7, H-3, H-1).
**FR-10.4** Integrasi **Midtrans dan Mayar** lewat antarmuka `PaymentProvider` (create checkout, handle webhook, cek status). Webhook per provider memverifikasi signature, idempoten, dan mengaktifkan paket otomatis. Ada rekonsiliasi harian terhadap status di provider.
**FR-10.5** Perpanjangan manual / auto-renew (opsional v1.1), upgrade/downgrade paket.
**FR-10.6** Kupon/diskon (Super Admin) — opsional v1.1.

---

### M11 — Tema & UX Responsif

**FR-11.1 Dark/Light Mode.** Toggle tersedia di semua halaman (guru & siswa); default mengikuti `prefers-color-scheme`; pilihan tersimpan (localStorage + profil). Seluruh komponen memenuhi kontras WCAG AA di kedua tema.
**FR-11.2 Responsif.** Pendekatan mobile-first dengan breakpoint (≤640, 641–1024, ≥1025 px). Tabel pada mobile berubah menjadi kartu/daftar; modal menjadi bottom sheet; target sentuh ≥ 44×44 px.
**FR-11.3 Aksesibilitas.** Navigasi keyboard, label ARIA, fokus terlihat, ukuran font dapat diperbesar, status tidak hanya bergantung warna.
**FR-11.4 Bahasa.** Bahasa Indonesia (default), arsitektur i18n siap untuk Inggris & Arab (RTL — roadmap).
**FR-11.5 Performa.** Bundle awal siswa ringan (code-splitting), berjalan baik di HP spesifikasi rendah dan jaringan 3G.

---

### M12 — Super Admin Console
**FR-12.1** Daftar tenant, status langganan, penggunaan kuota; aktif/nonaktifkan tenant.
**FR-12.2** Konfigurasi paket, harga, batas kuota, kupon.
**FR-12.3** Monitoring transaksi dan metrik platform (ujian aktif, peserta konkuren, error rate).
**FR-12.4** Impersonasi terbatas untuk dukungan (dengan audit log & persetujuan).

---

### M13 — Notifikasi & Email

**FR-13.1** Email transaksional: verifikasi email, reset password, undangan guru, invoice/bukti bayar, pengingat langganan (H-7, H-3, H-1), pemberitahuan akses berakhir dan penghapusan data (saat berakhir, H+3, H+6).
**FR-13.2** Notifikasi dalam aplikasi (guru): ujian akan dibuka/ditutup, uraian menunggu penilaian, siswa dilaporkan curang, kuota hampir habis (siswa/ujian).
**FR-13.3** Notifikasi siswa: pengingat ujian akan dibuka (opsional), nilai dirilis.
**FR-13.4** Email dikirim lewat job queue dengan retry; template Bahasa Indonesia; tautan berumur terbatas.

---

## 6. Kebutuhan Non-Fungsional

| Kategori | Persyaratan |
|---|---|
| **Performa** | Respons API p95 < 300 ms (non-upload). Halaman soal siswa muncul < 2 dtk (jaringan 4G). |
| **Skalabilitas** | Target awal pada 1 VPS: ±1.000 siswa konkuren (validasi dengan uji beban; tenant maks. 100 siswa). API stateless sehingga bisa dipindah ke VPS lebih besar/ditambah instance. Lonjakan serentak saat *Mulai* dan *Submit* diatasi dengan rate limit, antrean, dan cache paket soal. |
| **Ketersediaan** | Uptime ≥ 99,5% (single VPS). Backup Postgres harian + WAL ke object storage di luar VPS; restore diuji berkala. Risiko single point of failure diterima di fase awal. |
| **Keandalan Data** | Tidak ada kehilangan jawaban yang telah ter-ACK; jawaban lokal bertahan sampai sinkron. RPO ≤ 5 menit, RTO ≤ 1 jam. |
| **Keamanan** | HTTPS/TLS wajib; hash password (argon2id/bcrypt); JWT rotasi; RBAC; RLS multi-tenant; rate limiting; proteksi CSRF/XSS/SQLi; validasi & sanitasi rich text; audit log aksi sensitif. |
| **Privasi** | Patuh UU PDP Indonesia; data siswa (termasuk anak) minimal & terenkripsi saat transit/diam; kebijakan retensi & hapus data atas permintaan tenant. |
| **Kompatibilitas** | Chrome, Edge, Firefox, Safari (iOS 15+), Android Chrome 90+, WebView. |
| **Observability** | Logging terstruktur, metrics (Prometheus), tracing, alert (error rate, antrean sync, latensi). |
| **Maintainability** | Test coverage inti ≥ 70%; CI/CD; migrasi database versioned. |

---

## 7. Arsitektur Teknis & Infrastruktur (VPS + Coolify)

### 7.1 Gambaran Umum

```
┌───────────────────────────┐        ┌──────────────────────────┐
│  React PWA (Siswa)        │        │  React SPA (Guru/Admin)  │
│  Service Worker+IndexedDB │        │                          │
└────────────┬──────────────┘        └────────────┬─────────────┘
             │ HTTPS (REST/JSON) + WebSocket/SSE  │
             ▼                                    ▼
        ┌──────────────────────────────────────────────┐
        │  Reverse Proxy / CDN (TLS, cache aset)       │
        └────────────────────┬─────────────────────────┘
                             ▼
        ┌──────────────────────────────────────────────┐
        │  Golang API (stateless, N instance)          │
        │  auth · tenant · exam · attempt · grading    │
        │  report · billing · import/export · realtime │
        └──────┬────────────┬────────────┬─────────────┘
               ▼            ▼            ▼
        ┌──────────┐  ┌───────────┐  ┌───────────────┐
        │PostgreSQL│  │Redis      │  │Object Storage │
        │(primary +│  │cache,     │  │(S3/MinIO):    │
        │ replica) │  │pub/sub,   │  │gambar, audio, │
        │          │  │rate limit │  │file import/   │
        │          │  │           │  │export         │
        └──────────┘  └───────────┘  └───────────────┘
               ▲
        ┌──────┴────────┐
        │ Worker (Go)   │  import/export besar, re-grade, email,
        │ job queue     │  rekap laporan, webhook billing
        └───────────────┘
```

### 7.2 Stack & Pustaka (Rekomendasi)

| Lapisan | Pilihan |
|---|---|
| Backend | Go 1.22+, router `chi`/`gin`, `sqlc` atau `pgx`, `golang-migrate`, `go-playground/validator` |
| Database | PostgreSQL 16 (RLS, JSONB, partisi tabel jawaban/event) |
| Cache/Realtime | Redis (cache paket soal, pub/sub realtime, rate limit) |
| Job Queue | `asynq` (Redis) atau `river` (Postgres) |
| Frontend | React 18 + TypeScript, Vite, React Router, TanStack Query, Zustand, Tailwind CSS + shadcn/ui, `react-hook-form` + `zod` |
| Offline | Workbox (service worker), `idb`/Dexie (IndexedDB) |
| Editor Soal | TipTap (rich text) + KaTeX |
| Import/Export | `excelize` (Go) untuk .xlsx; PDF via `chromedp`/`gofpdf` |
| Pembayaran | Midtrans dan Mayar (abstraksi `PaymentProvider`) |
| Email | SMTP/SES/Resend |
| Deploy | **VPS Ubuntu + Coolify** (Docker; layanan API, worker, frontend, PostgreSQL, Redis, MinIO/S3 dikelola di Coolify); deploy otomatis dari Git (webhook), SSL otomatis via Coolify/Traefik; CI GitHub Actions untuk test |

### 7.3 Desain Kunci

1. **Otoritas waktu di server.** Server menetapkan `started_at` dan `deadline`; klien hanya menampilkan.
2. **Soal tanpa kunci ke klien.** Penilaian selalu di server.
3. **Event-sourced answers.** Jawaban disimpan sebagai event append-only + proyeksi jawaban terakhir per soal → audit & rekonstruksi mudah.
4. **Idempotensi.** Semua endpoint sinkron menerima `client_event_id`.
5. **Caching paket soal.** Paket soal versi terbit di-cache (Redis/CDN privat) agar lonjakan *Mulai* serentak tidak membebani DB.
6. **Isolasi tenant.** `tenant_id` pada semua tabel + RLS + indeks komposit.

### 7.4 Infrastruktur & Deployment (VPS Ubuntu + Coolify)

**Topologi awal (satu VPS, dikelola Coolify):**

| Layanan | Keterangan |
|---|---|
| `flowcbt-api` | Go API (stateless), health check `/healthz` |
| `flowcbt-worker` | Go worker (import/export, email, re-grade, retensi, rekonsiliasi pembayaran) |
| `flowcbt-web` | React (build statis, disajikan Nginx/Caddy), mencakup PWA siswa dan SPA guru |
| `postgres` | PostgreSQL 16 dengan volume persisten |
| `redis` | Cache, pub/sub realtime, antrean job, rate limit |
| `minio` atau S3-compatible eksternal | Media soal, snapshot kamera, file import/export |

**Praktik deployment:**
- Dua lingkungan: **staging** dan **production** (resource berbeda; staging boleh di VPS terpisah/kecil).
- Deploy otomatis dari Git (branch `main` → staging, tag rilis → production) lewat Coolify; migrasi database dijalankan sebagai langkah terpisah dan harus *backward-compatible*.
- **Deploy freeze** saat ada ujian berjalan (cek endpoint internal jumlah ujian aktif sebelum rilis produksi).
- Domain & SSL otomatis lewat proxy Coolify; subdomain terpisah untuk `app` (guru), `ujian` (siswa), dan `api`.
- Semua rahasia (JWT, kunci Midtrans/Mayar, SMTP, S3) sebagai environment variable terenkripsi di Coolify; tidak ada di repo.

**Keandalan & keamanan server:**
- Backup Postgres terjadwal (harian + arsip WAL bila diperlukan) ke **object storage di luar VPS**; restore diuji tiap bulan.
- Backup volume media bila memakai MinIO lokal.
- Firewall hanya membuka 80/443 (dan SSH dengan key saja), fail2ban, pembaruan keamanan otomatis, akun non-root.
- Pooling koneksi Postgres (pgxpool, atau PgBouncer bila perlu); batas koneksi disesuaikan.
- Monitoring: uptime check eksternal, metrics (CPU/RAM/disk, latensi API, antrean sync, jumlah siswa konkuren), log terstruktur terpusat, alert ke WhatsApp/Telegram/email.
- Estimasi awal spesifikasi VPS: 4 vCPU, 8 GB RAM, SSD NVMe; **validasi dengan uji beban** sebelum rilis dan tingkatkan sesuai hasil.

**Jalur skala berikutnya:** pindah Postgres ke VPS/managed terpisah → tambah instance API di server kedua → read replica untuk laporan.

---

## 8. Model Data (Skema Tingkat Tinggi)

```
tenants(id, name, level, timezone, status, created_at)
users(id, tenant_id, name, email, password_hash, role[owner|teacher], status, ...)
subscriptions(id, tenant_id, plan_id, type[monthly|per_exam], starts_at, ends_at, status)
plans(id, name, type, price, participant_quota, limits_json)
exam_credits(id, tenant_id, exam_id?, quantity, used, expires_at)
payments(id, tenant_id, provider, provider_ref, amount, status, paid_at, invoice_url)

classes(id, tenant_id, name, academic_year, homeroom_teacher_id?, archived)
students(id, tenant_id, student_code[unik], nis?, name, gender[L|P], class_id,
         username, password_hash, status, created_at)
student_class_history(id, student_id, class_id, from_date, to_date)

question_sets(id, tenant_id, subject_name, title, description, exam_code, level_tag,
              status[draft|published|archived], current_version, created_by)
question_set_versions(id, question_set_id, version, published_at, snapshot_json?)
questions(id, set_version_id, order_no, type[mcq|mcq_complex|matching|short_answer|essay],
          stimulus_id?, body, options_json, answer_key_json, points, scoring_mode,
          explanation, shuffle_options)
stimuli(id, tenant_id, content, media)

exams(id, tenant_id, question_set_version_id, name, token, token_active,
      open_at, close_at, duration_minutes, max_attempts, score_policy[highest|last|avg],
      show_score_immediately, show_review, shuffle_questions, shuffle_options,
      free_navigation, require_all_answered, tab_switch_policy, passing_score, status)
exam_assignments(id, exam_id, target_type[class|student], class_id?, student_id?)
exam_participants(exam_id, student_id, extra_time_seconds, attempts_override)

attempts(id, exam_id, student_id, attempt_no, class_snapshot, penalty_points, cheating_status[none|warned|flagged|disqualified],
         status[in_progress|submitted|graded|expired],
         started_at, deadline_at, submitted_at, score, max_score, final_score_pending, device_info)
attempt_answers(id, attempt_id, question_id, answer_json, flagged, is_correct?, score,
                graded_by[auto|manual], seq, updated_at)
attempt_events(id, attempt_id, client_event_id[unik], type[answer|flag|nav|submit|focus_lost|online|offline|sync],
               payload_json, client_ts, server_ts, seq)   -- partisi per bulan

attempt_violations(id, attempt_id, type[tab_hidden|blur|fullscreen_exit|camera_off|no_face|multi_face],
                   started_at, duration_ms, penalty_applied, client_event_id)
proctor_snapshots(id, attempt_id, object_key, taken_at, flags_json, expires_at)
students.is_guest, students.merged_into_id   -- peserta tamu & penggabungan
audit_logs(id, tenant_id, actor_id, action, entity, entity_id, meta_json, created_at)
```

**Indeks penting:** `(tenant_id, …)` pada semua tabel; `attempts(exam_id, status)`; `attempts(student_id, started_at)`; `attempt_events(attempt_id, seq)`; unik `attempt_events(client_event_id)`; unik `students(tenant_id, student_code)`; unik `question_sets(tenant_id, exam_code)`.

---

## 9. Rancangan API (Ringkas)

**Konvensi:**
- Base `/api/v1`, format JSON, auth `Bearer <JWT>`; dokumentasi OpenAPI dihasilkan dari kode/spec dan menjadi kontrak frontend.
- Error standar `{code, message, details}` dengan kode stabil (mis. `QUOTA_EXCEEDED`, `ATTEMPT_EXPIRED`, `TOKEN_INVALID`); HTTP status sesuai semantik.
- Pagination berbasis cursor/limit untuk daftar besar; filter dan sort lewat query string.
- Header `Idempotency-Key` pada operasi pembayaran dan pembuatan resource; endpoint sinkron jawaban idempoten via `client_event_id`.
- Rate limit per IP dan per akun (lebih ketat pada login, token, dan guest login); respons `429` dengan `Retry-After`.
- Semua query ter-scope `tenant_id` dari token, bukan dari input klien.


**Auth & Tenant**
- `POST /auth/register` · `POST /auth/login` · `POST /auth/refresh` · `POST /auth/logout`
- `POST /auth/verify-email` · `POST /auth/forgot-password` · `POST /auth/reset-password`
- `POST /student/auth/login` · `POST /student/auth/guest` (data diri + kode ujian + token)

**Siswa & Kelas**
- `GET/POST /classes` · `PUT/DELETE /classes/{id}`
- `GET/POST /students` · `GET/PUT/DELETE /students/{id}`
- `GET /students/import/template` · `POST /students/import/preview` · `POST /students/import/commit`
- `GET /students/export`
- `GET /students/{id}/track-record`

**Soal**
- `GET/POST /question-sets` · `GET/PUT/DELETE /question-sets/{id}`
- `POST /question-sets/{id}/questions` · `PUT/DELETE /questions/{id}` · `POST /questions/reorder`
- `POST /question-sets/{id}/publish` · `POST /question-sets/{id}/unpublish`
- `GET /question-sets/import/template` · `POST /question-sets/import` · `GET /question-sets/{id}/export`
- `POST /media/upload`

**Ujian**
- `GET/POST /exams` · `GET/PUT/DELETE /exams/{id}`
- `POST /exams/{id}/token/regenerate`
- `PUT /exams/{id}/assignments`
- `POST /exams/{id}/participants/{studentId}/extend-time` · `.../reset-attempt` · `.../force-submit`

**Pengerjaan (Siswa)**
- `GET /me/exams`
- `POST /exams/{id}/start` (body: token) → `attempt_id`, `deadline_at`, paket soal
- `GET /attempts/{id}` (pemulihan)
- `POST /attempts/{id}/answers:sync` (batch event, idempoten)
- `POST /attempts/{id}/submit`
- `POST /attempts/{id}/violations` · `POST /attempts/{id}/snapshots`
- `GET /exams/{id}/cheating-report` · `PUT /attempts/{id}/cheating-decision`
- `GET /attempts/{id}/result`

**Penilaian & Laporan**
- `GET /exams/{id}/attempts` · `GET /attempts/{id}/grading` · `PUT /attempts/{id}/answers/{qid}/score`
- `POST /exams/{id}/regrade`
- `GET /reports/live` · `GET /reports/live/{examId}` · `WS /reports/live/stream`
- `GET /reports/history?q=&subject=&class=&from=&to=` · `GET /reports/exams/{id}` · `GET /reports/exams/{id}/item-analysis`
- `GET /reports/exams/{id}/export?format=xlsx|csv|pdf`

**Billing**
- `GET /plans` · `POST /billing/checkout` · `GET /billing/subscription` · `GET /billing/invoices`
- `POST /webhooks/payment/midtrans` · `POST /webhooks/payment/mayar` (publik, verifikasi signature)
- `GET /billing/export-all` (export semua data tenant)

---

## 10. Alur Pengguna & Kriteria Penerimaan

### 10.1 Guru: Dari Daftar Sampai Ujian Berjalan
1. Daftar via form → verifikasi email → login.
2. (Opsional) Import siswa lewat template (Nama, Jenis Kelamin, Kelas) → preview → konfirmasi.
3. Buat paket soal: isi nama mapel → pilih target (kelas/siswa) → tambah soal AKM → simpan draft.
4. Beli paket (per ujian / bulanan) bila belum aktif.
5. Publish → atur jadwal buka–tutup, durasi, jumlah percobaan, tampilan nilai → token & kode ujian dihasilkan.
6. Bagikan kode + token ke siswa.
7. Pantau **Laporan Real-Time** saat ujian berlangsung.
8. Nilai uraian (jika ada) → rilis nilai → lihat **Laporan Terdahulu** dan export.

### 10.2 Siswa: Mengerjakan Ujian
1. Login → dashboard ujian → pilih ujian.
2. Masukkan token → baca tata tertib → *Mulai* (soal diunduh).
3. Mengerjakan: jawab, tandai ragu-ragu, buka **Daftar Soal**; jika internet putus, tetap lanjut (indikator 🔴), jawaban tersimpan lokal.
4. Internet pulih → sinkron otomatis (indikator 🟢).
5. Klik **Selesai** → review ringkasan → konfirmasi → terkirim.
6. Lihat nilai (jika diizinkan) / riwayat ujian.

### 10.3 Skenario Offline (Edge Case)
| Skenario | Perilaku |
|---|---|
| Putus saat menjawab | Lanjut mengerjakan; antrean lokal; banner merah |
| Putus lalu waktu habis | Auto-submit lokal; sinkron saat online; server validasi terhadap deadline + toleransi |
| Tab ditutup saat offline | Data tetap di IndexedDB; lanjut saat dibuka |
| Ganti perangkat saat offline | Tidak didukung; jawaban belum tersinkron tidak ikut. Siswa diberi peringatan |
| Sinkron ganda / retry | Idempoten via `client_event_id` |
| Jam perangkat dimanipulasi | Dihitung dari elapsed monotonic + validasi server |
| Browser dibersihkan sebelum sinkron | Jawaban lokal hilang; guru bisa reset attempt (peringatan jelas sebelum ujian) |

### 10.4 Kriteria Penerimaan Fitur Kritis

| # | Fitur | Kriteria Penerimaan |
|---|---|---|
| AC-1 | Offline answering | **Given** soal sudah ter-cache, **when** koneksi diputus dan siswa menjawab 10 soal, **then** semua jawaban tersimpan di perangkat dan tampil kembali setelah reload tab. |
| AC-2 | Auto-sync | **Given** ada 10 jawaban dalam antrean lokal, **when** koneksi pulih, **then** semua terkirim otomatis dalam ≤ 15 detik, tanpa duplikasi, dan indikator berubah ke "Tersinkron". |
| AC-3 | Idempotensi | **Given** batch yang sama dikirim dua kali, **then** server menyimpan sekali dan membalas ACK keduanya. |
| AC-4 | Submit offline | **Given** siswa menekan Selesai saat offline, **then** input terkunci, status "menunggu sinkronisasi", dan saat online attempt menjadi *submitted* dengan jawaban lengkap. |
| AC-5 | Deadline | **Given** waktu pengerjaan habis, **then** attempt otomatis ter-submit; jawaban yang dibuat setelah deadline + toleransi ditolak dan ditandai di log. |
| AC-6 | Batas percobaan | **Given** percobaan maksimum tercapai, **then** tombol Mulai dinonaktifkan dan server menolak start baru (`409`). |
| AC-7 | Publish gating | **Given** tenant tanpa kuota aktif, **when** guru menekan Publish, **then** ditolak dengan pesan dan tautan pembelian paket. |
| AC-8 | Penalti | **Given** penalti aktif dan siswa keluar aplikasi lebih dari toleransi, **then** pelanggaran tercatat, peringatan tampil, dan nilai akhir berkurang sesuai konfigurasi (nilai asli, penalti, nilai akhir terlihat terpisah). |
| AC-9 | Isolasi tenant | **Given** pengguna tenant A, **when** mengakses ID resource tenant B, **then** respons `404`; ada tes otomatis untuk semua endpoint. |
| AC-10 | Import siswa | **Given** file berisi 500 baris dengan 5 baris salah, **then** preview menampilkan 495 valid dan 5 error beserta nomor baris dan alasannya; hanya baris valid yang disimpan setelah konfirmasi. |
| AC-11 | Retensi | **Given** akses berakhir 7 hari lalu tanpa perpanjangan, **then** job menghapus data tenant dan mencatat audit; perpanjangan sebelum hari ke-7 membatalkan penghapusan. |
| AC-12 | Mobile | **Given** layar 360×640, **then** seluruh alur siswa (login → mulai → jawab 5 tipe soal → daftar soal → selesai) dapat dilakukan tanpa scroll horizontal dan dengan target sentuh ≥ 44 px. |

---

## 11. Spesifikasi UI/UX & Peta Halaman

- **Prinsip:** sederhana, minim klik, jelas, bahasa sehari-hari, konsisten.
- **Guru (desktop-first, tetap nyaman di mobile):** sidebar navigasi (menjadi bottom nav/drawer di mobile), tabel dengan filter & pencarian (menjadi kartu di mobile), wizard pembuatan ujian bertahap (Info → Soal → Peserta → Jadwal → Publish).
- **Siswa (mobile-first):** satu kolom, tombol besar, timer sticky, navigasi soal via bottom sheet, mode layar penuh, minim distraksi.
- **Tema:** token warna (CSS variables) untuk light/dark; aksen konsisten; animasi ringan (hormati `prefers-reduced-motion`).
- **Umpan Balik:** skeleton loading, toast, status sinkron jelas, pesan error yang menjelaskan solusi.
- **Menjodohkan di mobile:** pola tap-pilih-kiri lalu tap-pilih-kanan (alternatif drag-and-drop) dengan garis/penanda warna.

### 11.1 Peta Halaman (Sitemap)

**Guru/Admin (`app.`)**
`/login` · `/register` · `/dashboard` · `/classes` · `/students` · `/students/:id` (track record) · `/question-sets` · `/question-sets/:id/edit` · `/exams` · `/exams/new` (wizard) · `/exams/:id` (detail & aksi pengawas) · `/exams/:id/cheating` · `/reports/live` · `/reports/history` · `/reports/exams/:id` · `/billing` · `/settings` (profil, guru, tema)

**Siswa (`ujian.`)**
`/login` · `/guest` (tanpa akun) · `/exams` · `/exams/:id/start` (token & tata tertib) · `/attempt/:id` (butir soal) · `/attempt/:id/review` (konfirmasi selesai) · `/attempt/:id/done` · `/history`

**Super Admin**
`/admin/tenants` · `/admin/plans` · `/admin/payments` · `/admin/metrics`

---

## 12. Keamanan & Integritas Ujian

- Token ujian + kode ujian; token dapat dirotasi; rate limiting login/token (anti brute-force).
- Satu sesi aktif per siswa; deteksi login ganda.
- Acak urutan soal & opsi per siswa.
- Anti-cheat sesuai FR-5.8: deteksi keluar aplikasi dengan penalti/laporan curang, serta proctoring kamera opsional.
- Copy-paste dan klik kanan dinonaktifkan di area soal (deterrent).
- Soal tidak menyertakan kunci jawaban di sisi klien; penilaian dan penalti dihitung di server.
- Audit log: perubahan soal, kunci, nilai, reset attempt, pembatalan penalti, akses snapshot.
- Enkripsi in transit (TLS) dan at rest (backup, object storage); sanitasi HTML rich text (allowlist) untuk mencegah XSS.
- Data foto kamera diperlakukan sebagai data pribadi: persetujuan, retensi pendek, akses terbatas.

---

## 13. Metrik Keberhasilan (KPI)

| Metrik | Target (6 bulan pasca-peluncuran) |
|---|---|
| Tenant terdaftar | ≥ 100 |
| Konversi trial → berbayar | ≥ 15% |
| Kehilangan jawaban (ter-ACK atau lokal) | 0 kasus |
| Keberhasilan sinkron offline | ≥ 99,5% |
| Waktu pembuatan ujian pertama | < 10 menit (median) |
| Crash/error rate sisi siswa | < 0,5% sesi |
| Kepuasan pengguna (CSAT) | ≥ 4,3 / 5 |
| Uptime jam ujian | ≥ 99,5% |

---

## 14. Rencana Rilis & Strategi Pengujian

| Fase | Durasi (perkiraan) | Cakupan |
|---|---|---|
| **Fase 0 — Fondasi** | 2–3 minggu | Setup repo, CI/CD, desain UI/token tema, skema DB, auth & multi-tenant |
| **Fase 1 — Core Soal & Siswa** | 4–5 minggu | Siswa/kelas + import/export, editor soal 5 tipe AKM, draft/publish, import/export template soal |
| **Fase 2 — Ujian & Siswa Client** | 5–6 minggu | Pengaturan ujian, token, UI siswa (butir soal, daftar soal, selesai), penilaian otomatis, dark/light mode |
| **Fase 3 — Offline & Sinkronisasi** | 3–4 minggu | PWA, IndexedDB, antrean sync, timer offline, pemulihan, uji jaringan buruk |
| **Fase 3b — Anti-Cheat & Proctoring** | 3–4 minggu | Deteksi keluar aplikasi + penalti, snapshot kamera, deteksi wajah ringan, laporan kecurangan, login siswa tanpa akun |
| **Fase 4 — Laporan** | 3–4 minggu | Laporan real-time, laporan historis, track record siswa, analisis butir, export |
| **Fase 5 — Billing & Beta** | 3–4 minggu | Paket & kuota, integrasi Midtrans + Mayar, retensi 7 hari & export data, invoice, Super Admin console, setup Coolify/backup, uji beban, beta tertutup |
| **Fase 6 — Peluncuran** | 2 minggu | Hardening, dokumentasi, onboarding, rilis publik |

**Kriteria MVP (P0, beta tertutup):** Fase 0–3 + deteksi keluar aplikasi dengan penalti (bagian dari Fase 3b) + laporan dasar + billing (paket gratis, per ujian, bulanan) dengan Midtrans aktif. Login siswa tanpa akun, kamera, dan Mayar masuk P1 (rilis publik); lihat Bagian 4.2.

### 14.1 Strategi Pengujian & Kualitas

| Jenis | Cakupan |
|---|---|
| Unit test (Go) | Mesin penilaian (semua tipe soal, rumus parsial, normalisasi isian), penalti, kuota/tier, retensi |
| Integration test | API + PostgreSQL asli (testcontainers), RLS/isolasi tenant, webhook Midtrans/Mayar idempoten |
| E2E (Playwright) | Alur guru dan siswa di viewport desktop dan mobile |
| E2E offline | Emulasi jaringan putus/lambat/flapping saat ujian, reload tab, tutup-buka browser, submit offline |
| Uji beban (k6) | Skenario ±1.000 siswa: mulai serentak, sinkron jawaban, submit serentak; ukur p95, error rate, antrean |
| Keamanan | Cek OWASP Top 10, tes otomatis isolasi tenant, review dependency, uji brute-force token |
| Aksesibilitas | Pemeriksaan otomatis (axe) + uji keyboard; kontras dark/light |
| Perangkat nyata | Android low-end, iPhone Safari, Chrome desktop, layar kecil 360 px |

**Definition of Done:** fitur memiliki kriteria penerimaan terpenuhi, test otomatis lulus, tampil benar di dark/light dan mobile/desktop, tercatat di audit log (bila sensitif), dan terdokumentasi di OpenAPI.

---

## 15. Risiko & Mitigasi

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Lonjakan konkuren saat mulai/submit | API lambat/gagal | Cache paket soal, rate limit adaptif, antrean, uji beban sebelum rilis |
| Data lokal hilang (browser dibersihkan) | Jawaban hilang | Autosync agresif saat online, peringatan, fitur reset attempt oleh guru |
| Kecurangan | Kredibilitas rendah | Acak soal/opsi, logging pelanggaran, token, satu sesi; roadmap proctoring |
| Dukungan Safari/iOS terbatas (Background Sync) | Sinkron tertunda | Sinkron saat app aktif + event `online` + polling; jangan bergantung hanya pada Background Sync |
| Kebocoran antar-tenant | Pelanggaran data | RLS, tes otomatis isolasi tenant, audit |
| Kompleksitas editor rumus/gambar | Jadwal molor | Gunakan TipTap + KaTeX; batasi fitur media di MVP |
| Sengketa pembayaran/webhook | Pendapatan salah | Webhook idempoten, rekonsiliasi harian |
| Data anak & foto kamera (UU PDP) | Risiko hukum/reputasi | Kamera opsional per ujian, persetujuan eksplisit, snapshot kecil tanpa video/audio, retensi pendek, akses tercatat |
| Penghapusan data 7 hari menghilangkan track record | Sekolah kehilangan riwayat | Pengingat berulang, tombol Export Semua Data, pemulihan bila diperpanjang dalam 7 hari |
| Single VPS (SPOF) | Downtime saat jam ujian | Backup di luar VPS, monitoring, rencana restore terdokumentasi, siap upgrade/migrasi |
| Penalti keluar aplikasi tidak adil (notifikasi, telepon masuk) | Keluhan siswa/guru | Toleransi detik, penalti dapat dibatalkan guru, batas penalti maksimum |

---

## 16. Asumsi & Ketergantungan

- Pengguna memiliki browser modern; siswa minimal memiliki satu kali koneksi saat memulai ujian.
- Akun Midtrans dan Mayar disetujui dan aktif (verifikasi bisnis); VPS Ubuntu dengan Coolify sudah terpasang.
- Layanan email & object storage tersedia.
- Domain, SSL, dan infrastruktur cloud dipersiapkan sebelum beta.

---

## 17. Keputusan Produk (Final v1)

### 17.1 Keputusan

| # | Topik | Keputusan |
|---|---|---|
| 1 | Harga & kuota | Gratis 1–10 siswa (1x ujian, 14 hari); per ujian: 11–40 Rp10.000/3x, 41–70 Rp20.000/7x, 71–100 Rp30.000/12x; bulanan: 1–50 Rp50.000, 51–100 Rp70.000 (Bagian 3.1) |
| 2 | Trial | Sama dengan paket gratis: 1–10 siswa, 1x ujian, 14 hari, fitur lengkap |
| 3 | Login siswa | Dua metode: username/password dan tanpa akun (data diri + kode + token) |
| 4 | Nilai uraian | Koreksi manual oleh guru |
| 5 | Proctoring | Kamera + anti-cheat; keluar/pindah aplikasi → pengurangan nilai atau dilaporkan curang (FR-5.8) |
| 6 | Retensi data | 7 hari setelah akses berakhir tanpa perpanjangan, lalu dihapus (aturan titik mulai: Bagian 3.2) |
| 7 | White-label | Tidak ada |
| 8 | Bank soal bersama | Tidak ada |
| 9 | Payment gateway | Midtrans dan Mayar |
| 10 | Hosting | VPS Ubuntu, deploy via Coolify (Bagian 7.4) |
| 11 | Di atas 100 siswa | Belum ada paket di v1 ("Hubungi kami") |
| 12 | Kredit per ujian | Tidak kedaluwarsa |
| 13 | Perpindahan tier | Upgrade bayar selisih; downgrade periode berikutnya (Bagian 3.2) |
| 14 | Nilai default anti-cheat/kamera | Lihat tabel default di FR-5.8 (semuanya dapat diubah guru) |

### 17.2 Hal yang Ditinjau Setelah Beta

1. **Margin per paket** setelah biaya gateway, penyimpanan, dan bandwidth dihitung dengan angka nyata.
2. **Retensi 7 hari:** apakah cukup bagi sekolah, mengingat riwayat nilai ikut terhapus; evaluasi jumlah keluhan/permintaan pemulihan.
3. **Default penalti dan kamera:** apakah terlalu ketat/longgar berdasarkan data pelanggaran dan keluhan.
4. **Kapasitas VPS:** hasil uji beban dan data konkurensi nyata untuk menentukan kapan memisahkan database/menambah instance.
5. **Kebutuhan paket di atas 100 siswa** berdasarkan jumlah permintaan "Hubungi kami".

---

## 18. Lampiran

### 18.1 Contoh Struktur Template Import Siswa

| Nama | Jenis Kelamin | Kelas | NIS (opsional) |
|---|---|---|---|
| Ahmad Fauzi | L | X IPA 1 | 12345 |
| Siti Aisyah | P | X IPA 1 | 12346 |

### 18.2 Contoh Struktur Template Import Soal (Ringkas)

| tipe | pertanyaan | opsi_A | opsi_B | opsi_C | opsi_D | kunci | poin | pembahasan |
|---|---|---|---|---|---|---|---|---|
| pg | Ibu kota Indonesia? | Jakarta | Bandung | Medan | Surabaya | A | 2 | — |
| pg_kompleks | Pilih bilangan prima | 2 | 4 | 5 | 9 | A,C | 3 | — |
| menjodohkan | Pasangkan negara–ibu kota | (pasangan kiri\|kanan; dipisah `;`) | | | | Indonesia\|Jakarta;Jepang\|Tokyo | 4 | — |
| isian | 7 + 8 = … | | | | | 15 | 2 | — |
| uraian | Jelaskan proses fotosintesis | | | | | (rubrik) | 10 | — |

### 18.3 Status Attempt (State Machine)

```
not_started ──start──▶ in_progress ──submit(online)──────────▶ submitted ──▶ graded
                           │  │                                   ▲            │
                           │  └─submit(offline)─▶ pending_sync ───┘            └─▶ awaiting_manual_grading ──▶ graded
                           │
                           ├─ waktu habis ─▶ auto-submit ─▶ submitted
                           └─ pelanggaran ≥ ambang (opsi) ─▶ auto-submit; cheating_status = flagged
                                                          └─ keputusan guru: valid | penalti dibatalkan | disqualified
```

- `pending_sync`: sudah selesai di perangkat, menunggu terkirim ke server.
- `awaiting_manual_grading`: ada soal uraian yang belum dinilai; nilai berstatus **Sementara**.
- `disqualified`: attempt dibatalkan oleh guru; nilai 0 dan tidak dihitung pada rata-rata.

### 18.4 Glosarium
- **CBT** — Computer Based Test.
- **AKM** — Asesmen Kompetensi Minimum.
- **Tenant** — satu sekolah/lembaga pelanggan dengan data terisolasi.
- **Attempt** — satu percobaan pengerjaan ujian oleh seorang siswa.
- **PWA** — Progressive Web App.
- **RLS** — Row-Level Security (PostgreSQL).
- **Idempoten** — permintaan yang diulang menghasilkan hasil sama tanpa efek ganda.

### 18.5 Riwayat Revisi

| Versi | Perubahan |
|---|---|
| 1.0 | Draft awal: seluruh modul, arsitektur, model data, API, roadmap |
| 1.1 | Keputusan produk: harga awal, login dua metode, proctoring, retensi 7 hari, Midtrans + Mayar, VPS + Coolify |
| 1.2 | Harga final (tier 1–100 siswa tanpa celah), aturan tier/upgrade, titik mulai retensi, nilai default anti-cheat & kamera, matriks hak akses, prioritas P0–P2, modul notifikasi, infrastruktur Coolify, konvensi API, kriteria penerimaan, peta halaman, strategi pengujian, state machine attempt |
| 1.3 | Lampiran implementasi: skema database lengkap (migrasi PostgreSQL 000001–000010, teruji), struktur proyek Go + React, rincian 32 sprint per fase (18.6–18.8) |

### 18.6 Skema Database Lengkap (Migrasi PostgreSQL 16)

Skema ini menerjemahkan Bagian 8 menjadi migrasi `golang-migrate` (pasangan `NNNNNN_nama.up.sql` / `.down.sql`) di `backend/db/migrations/`. Seluruh migrasi sudah diuji pada PostgreSQL 16: urutan *up* berjalan bersih, isolasi RLS terverifikasi, fungsi purge berjalan, idempotensi event terverifikasi, dan siklus *down → up* bersih.

| # | Migrasi | Isi |
|---|---|---|
| 1 | `000001_init` | Ekstensi `citext`, `pg_trgm`; fungsi `set_updated_at()`, `current_tenant_id()` |
| 2 | `000002_tenancy_auth` | `tenants`, `users`, `one_time_tokens` |
| 3 | `000003_billing` | `plans`, `payments`, `payment_webhook_events`, `subscriptions`, `exam_credits`, `invoices` |
| 4 | `000004_students_classes` | `classes`, `teacher_class_assignments`, `students`, `student_class_history`, `refresh_tokens` |
| 5 | `000005_question_bank` | `media_assets`, `question_sets`, `question_set_shares`, `question_set_versions`, `stimuli`, `questions` |
| 6 | `000006_exams` | `exams`, `exam_rooms`, `exam_tokens`, `exam_assignments`, `exam_participants` |
| 7 | `000007_attempts` | `attempts`, `attempt_sessions`, `attempt_answers`, `attempt_events` (16 partisi), `attempt_violations`, `proctor_snapshots` |
| 8 | `000008_ops_retention` | `audit_logs`, `notifications`, `notice_log`, `file_jobs`, `platform_settings`; fungsi `tenant_retention_start`, `purge_tenant_data`, `tenant_object_keys`; view `tenants_due_for_purge`, `tenant_usage` |
| 9 | `000009_rls_grants_triggers` | Role `flowcbt_app` / `flowcbt_system`, RLS semua tabel ber-`tenant_id`, grant, trigger `updated_at` |
| 10 | `000010_seed_plans` | Seed 6 paket sesuai Bagian 3.1 + `platform_settings` awal |

**Keputusan desain & penyimpangan dari Bagian 8** (perlu disetujui sebelum Sprint 2):

1. **`tenants.code` (kode sekolah)** ditambahkan karena `username` siswa hanya unik per tenant; login siswa dengan akun memerlukan kode sekolah untuk menentukan tenant.
2. **Token ujian di tabel `exam_tokens` dengan keunikan global**, bukan kolom `exams.token`. Kode ujian hanya unik per tenant, sehingga login tanpa akun mencari tenant dari token. Mendukung token per ruang/sesi (FR-4.9) dan regenerasi (token lama dinonaktifkan).
3. **`attempt_events` dipartisi HASH(`attempt_id`), bukan per bulan.** `UNIQUE (client_event_id)` lintas partisi tidak mungkin tanpa kolom partisi di indeks unik; keunikan `(attempt_id, client_event_id)` menjamin idempotensi (AC-3) langsung di database. Penghapusan retensi tetap lewat `ON DELETE CASCADE` dari `attempts`.
4. **Versi soal:** satu versi `draft` (dapat diubah) + versi `published` (beku). Mengubah soal setelah publish = *clone* ke draft baru (FR-4.8). `questions.set_version_id` mengikuti Bagian 8.
5. **`subscriptions.type` = `trial | monthly`**; pembelian per ujian menjadi baris `exam_credits` (tanpa `expires_at`, sesuai Keputusan #12). Sumber kuota yang dipakai tiap ujian dicatat di `exams.quota_source` / `quota_ref_id` agar kredit dapat dikembalikan saat unpublish.
6. **Dua role database.** `flowcbt_app` tunduk RLS (`SET LOCAL app.tenant_id` per transaksi). `flowcbt_system` (BYPASSRLS) hanya untuk login lintas tenant, webhook, worker retensi, dan Super Admin. `audit_logs` dan `attempt_events` append-only bagi role aplikasi. Pembuatan role BYPASSRLS memerlukan superuser (kondisi default Postgres di Coolify); ops mengatur kata sandi login role.
7. **Retensi:** `purge_tenant_data()` menghapus data siswa/soal/hasil/snapshot dan menyisakan baris `tenants` (status `purged`), `payments`, `invoices`, `subscriptions`, `exam_credits`. Worker wajib menghapus object storage (`tenant_object_keys()`) **sebelum** memanggil fungsi ini. Kolom `name_normalized`, `phone_normalized`, `owner_email_hash` tetap tersimpan untuk deteksi paket gratis ganda.
8. **Status attempt** mengikuti state machine 18.3, ditambah `expired` dan `cancelled`; `cheating_status` diperluas (`valid`, `penalty_cancelled`) untuk menyimpan keputusan guru.

#### Kode SQL lengkap (arah *up*)

Berkas *down* tiap migrasi ada di repositori (`backend/db/migrations/*.down.sql`).

**`000001_init.up.sql`**

```sql
-- 000001: ekstensi & fungsi dasar
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.updated_at = now(); RETURN NEW; END $$;

-- Tenant aktif per transaksi: SET LOCAL app.tenant_id = '<uuid>' (diisi middleware Go dari JWT)
CREATE OR REPLACE FUNCTION current_tenant_id() RETURNS uuid LANGUAGE sql STABLE AS
$$ SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid $$;
```

**`000002_tenancy_auth.up.sql`**

```sql
-- 000002: tenant, user (admin/guru/super admin), token sekali pakai
CREATE TABLE tenants (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code                  text NOT NULL UNIQUE,                 -- kode sekolah (dipakai login siswa untuk menentukan tenant)
  name                  text NOT NULL,
  level                 text NOT NULL CHECK (level IN ('paud_tk','sd_mi','smp_mts','sma_smk_ma','perguruan_tinggi','kursus_bimbel','lainnya')),
  timezone              text NOT NULL DEFAULT 'Asia/Jakarta',
  phone                 text,
  status                text NOT NULL DEFAULT 'active' CHECK (status IN ('active','read_only','suspended','purged')),
  score_decimals        smallint NOT NULL DEFAULT 2 CHECK (score_decimals BETWEEN 0 AND 4),
  storage_quota_bytes   bigint NOT NULL DEFAULT 1073741824,   -- 1 GB (FR 3.2), bisa diubah Super Admin
  storage_used_bytes    bigint NOT NULL DEFAULT 0,
  -- siklus akses & retensi 7 hari (PRD 3.2)
  free_plan_used        boolean NOT NULL DEFAULT false,       -- paket gratis sekali per tenant
  access_ended_at       timestamptz,                          -- waktu hak akses berakhir (trial/bulanan/kredit habis)
  read_only_since       timestamptz,
  purge_after           timestamptz,                          -- = greatest(access_ended_at, close_at ujian terakhir) + 7 hari
  purged_at             timestamptz,
  -- anti-penyalahgunaan paket gratis (tetap disimpan setelah purge, tanpa data siswa)
  name_normalized       text,
  phone_normalized      text,
  owner_email_hash      text,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tenants_status_purge_idx ON tenants (status, purge_after);
CREATE INDEX tenants_dup_name_idx  ON tenants (name_normalized);
CREATE INDEX tenants_dup_phone_idx ON tenants (phone_normalized) WHERE phone_normalized IS NOT NULL;

CREATE TABLE users (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           uuid REFERENCES tenants(id) ON DELETE CASCADE,   -- NULL hanya untuk super_admin
  role                text NOT NULL CHECK (role IN ('super_admin','owner','teacher')),
  name                text NOT NULL,
  email               citext NOT NULL UNIQUE,
  phone               text,
  password_hash       text,                                            -- NULL bila hanya login Google
  google_sub          text UNIQUE,
  email_verified_at   timestamptz,
  status              text NOT NULL DEFAULT 'active' CHECK (status IN ('invited','active','disabled')),
  failed_login_count  integer NOT NULL DEFAULT 0,
  locked_until        timestamptz,
  last_login_at       timestamptz,
  theme_pref          text CHECK (theme_pref IN ('light','dark','system')),
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  CHECK ((role = 'super_admin') = (tenant_id IS NULL))
);
CREATE INDEX users_tenant_idx ON users (tenant_id, role);

-- Token sekali pakai: verifikasi email, reset password, undangan guru
CREATE TABLE one_time_tokens (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid REFERENCES tenants(id) ON DELETE CASCADE,
  user_id     uuid REFERENCES users(id) ON DELETE CASCADE,
  email       citext,
  purpose     text NOT NULL CHECK (purpose IN ('verify_email','reset_password','teacher_invite')),
  token_hash  text NOT NULL UNIQUE,
  payload     jsonb NOT NULL DEFAULT '{}',          -- mis. {"role":"teacher","class_ids":[...]}
  expires_at  timestamptz NOT NULL,
  used_at     timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX one_time_tokens_user_idx ON one_time_tokens (user_id, purpose);
```

**`000003_billing.up.sql`**

```sql
-- 000003: paket, langganan, kredit ujian, pembayaran, invoice
CREATE TABLE plans (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code           text NOT NULL UNIQUE,
  name           text NOT NULL,
  type           text NOT NULL CHECK (type IN ('free','per_exam','monthly')),
  price_idr      integer NOT NULL CHECK (price_idr >= 0),
  student_min    integer NOT NULL CHECK (student_min >= 1),
  student_max    integer NOT NULL,
  exam_quota     integer,                 -- free & per_exam; NULL = tanpa batas (monthly)
  duration_days  integer,                 -- free = 14, monthly = 30
  limits_json    jsonb NOT NULL DEFAULT '{}',
  is_active      boolean NOT NULL DEFAULT true,
  sort_order     integer NOT NULL DEFAULT 0,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (student_max >= student_min)
);

CREATE TABLE payments (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid REFERENCES tenants(id) ON DELETE SET NULL,   -- data penagihan disimpan walau tenant dihapus
  tenant_name_snap text NOT NULL,
  plan_id          uuid NOT NULL REFERENCES plans(id),
  purpose          text NOT NULL CHECK (purpose IN ('per_exam','monthly','upgrade')),
  provider         text NOT NULL CHECK (provider IN ('midtrans','mayar')),
  order_id         text NOT NULL UNIQUE,
  provider_ref     text,
  idempotency_key  text,
  amount_idr       integer NOT NULL CHECK (amount_idr >= 0),
  status           text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','failed','expired','refunded')),
  checkout_url     text,
  paid_at          timestamptz,
  meta             jsonb NOT NULL DEFAULT '{}',   -- mis. prorata upgrade, subscription_id target
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX payments_provider_ref_uq ON payments (provider, provider_ref) WHERE provider_ref IS NOT NULL;
CREATE UNIQUE INDEX payments_idem_uq ON payments (tenant_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX payments_tenant_idx ON payments (tenant_id, created_at DESC);
CREATE INDEX payments_pending_idx ON payments (status, created_at) WHERE status = 'pending';  -- rekonsiliasi harian

-- Log webhook mentah; unik per event => idempoten. Tanpa tenant_id (hanya diakses role sistem)
CREATE TABLE payment_webhook_events (
  id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  provider       text NOT NULL,
  event_id       text NOT NULL,
  signature_ok   boolean NOT NULL,
  payload        jsonb NOT NULL,
  received_at    timestamptz NOT NULL DEFAULT now(),
  processed_at   timestamptz,
  error          text,
  UNIQUE (provider, event_id)
);

-- Langganan: trial (paket gratis) & bulanan. Pembelian per ujian => exam_credits.
CREATE TABLE subscriptions (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id          uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  plan_id            uuid NOT NULL REFERENCES plans(id),
  type               text NOT NULL CHECK (type IN ('trial','monthly')),
  status             text NOT NULL CHECK (status IN ('pending','active','scheduled','expired','cancelled')),
  student_limit      integer NOT NULL,                  -- snapshot batas tier saat dibeli
  exam_quota         integer,                           -- trial: 1
  exams_used         integer NOT NULL DEFAULT 0,
  starts_at          timestamptz NOT NULL,
  ends_at            timestamptz NOT NULL,
  payment_id         uuid REFERENCES payments(id),
  replaces_id        uuid REFERENCES subscriptions(id), -- upgrade/downgrade (downgrade = status 'scheduled')
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX subscriptions_one_active_uq ON subscriptions (tenant_id) WHERE status = 'active';
CREATE INDEX subscriptions_tenant_idx ON subscriptions (tenant_id, ends_at DESC);
CREATE INDEX subscriptions_expiry_idx ON subscriptions (ends_at) WHERE status = 'active';

-- Kredit per ujian: tidak kedaluwarsa (Keputusan #12)
CREATE TABLE exam_credits (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  plan_id        uuid NOT NULL REFERENCES plans(id),
  payment_id     uuid REFERENCES payments(id),
  quantity       integer NOT NULL CHECK (quantity > 0),
  used           integer NOT NULL DEFAULT 0,
  student_limit  integer NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  CHECK (used >= 0 AND used <= quantity)
);
CREATE INDEX exam_credits_tenant_idx ON exam_credits (tenant_id) WHERE used < quantity;

CREATE TABLE invoices (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid REFERENCES tenants(id) ON DELETE SET NULL,
  payment_id       uuid NOT NULL UNIQUE REFERENCES payments(id),
  invoice_no       text NOT NULL UNIQUE,
  billing_name     text NOT NULL,
  billing_email    citext,
  amount_idr       integer NOT NULL,
  pdf_object_key   text,
  issued_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX invoices_tenant_idx ON invoices (tenant_id, issued_at DESC);
```

**`000004_students_classes.up.sql`**

```sql
-- 000004: kelas, siswa (termasuk peserta tamu), riwayat kelas, sesi login
CREATE TABLE classes (
  id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id            uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name                 text NOT NULL,
  academic_year        text NOT NULL,
  homeroom_teacher_id  uuid REFERENCES users(id) ON DELETE SET NULL,
  archived_at          timestamptz,
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, name, academic_year)
);

-- Pembatasan akses guru pada kelas/mapel tertentu (FR-1.5)
CREATE TABLE teacher_class_assignments (
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  class_id      uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  subject_name  text NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, class_id, subject_name)
);
CREATE INDEX tca_class_idx ON teacher_class_assignments (class_id);

CREATE TABLE students (
  id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id          uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  student_code       text NOT NULL,                    -- identitas unik untuk track record
  nis                text,
  nisn               text,
  name               text NOT NULL,
  name_normalized    text NOT NULL,                    -- lower, trim, spasi ganda dihapus (pencocokan login tanpa akun)
  gender             text NOT NULL CHECK (gender IN ('L','P')),
  class_id           uuid REFERENCES classes(id) ON DELETE SET NULL,
  username           text,
  password_hash      text,
  email              citext,
  guardian_phone     text,
  photo_key          text,
  status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive','archived')),
  is_guest           boolean NOT NULL DEFAULT false,
  merged_into_id     uuid REFERENCES students(id),     -- hasil merge peserta tamu (FR-1.3)
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, student_code),
  CHECK (merged_into_id IS NULL OR is_guest)
);
CREATE UNIQUE INDEX students_username_uq ON students (tenant_id, username) WHERE username IS NOT NULL;
CREATE UNIQUE INDEX students_nis_uq ON students (tenant_id, nis) WHERE nis IS NOT NULL AND NOT is_guest;
CREATE INDEX students_class_idx ON students (tenant_id, class_id) WHERE status = 'active';
CREATE INDEX students_match_idx ON students (tenant_id, name_normalized, class_id);
CREATE INDEX students_name_trgm ON students USING gin (name gin_trgm_ops);
CREATE INDEX students_merged_idx ON students (merged_into_id) WHERE merged_into_id IS NOT NULL;

CREATE TABLE student_class_history (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  student_id  uuid NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  class_id    uuid NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  from_date   date NOT NULL,
  to_date     date,
  CHECK (to_date IS NULL OR to_date >= from_date)
);
CREATE INDEX sch_student_idx ON student_class_history (student_id, from_date DESC);

-- Refresh token (rotasi) untuk user & siswa (FR-1.6)
CREATE TABLE refresh_tokens (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid REFERENCES tenants(id) ON DELETE CASCADE,
  subject_type  text NOT NULL CHECK (subject_type IN ('user','student')),
  user_id       uuid REFERENCES users(id) ON DELETE CASCADE,
  student_id    uuid REFERENCES students(id) ON DELETE CASCADE,
  family_id     uuid NOT NULL,                       -- deteksi reuse token lama => cabut satu family
  token_hash    text NOT NULL UNIQUE,
  device_hash   text,                                -- sesi tamu diikat ke perangkat (cookie httpOnly)
  ip            inet,
  user_agent    text,
  expires_at    timestamptz NOT NULL,
  revoked_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now(),
  CHECK ((subject_type = 'user' AND user_id IS NOT NULL) OR (subject_type = 'student' AND student_id IS NOT NULL))
);
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX refresh_tokens_student_idx ON refresh_tokens (student_id) WHERE student_id IS NOT NULL;
```

**`000005_question_bank.up.sql`**

```sql
-- 000005: media, paket soal, versi, stimulus, soal
CREATE TABLE media_assets (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('image','audio','import','export')),
  object_key   text NOT NULL UNIQUE,
  mime         text NOT NULL,
  size_bytes   bigint NOT NULL CHECK (size_bytes >= 0),
  created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX media_assets_tenant_idx ON media_assets (tenant_id, kind);

CREATE TABLE question_sets (
  id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id             uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  subject_name          text NOT NULL,
  title                 text NOT NULL,
  description           text,
  exam_code             text NOT NULL,
  level_tags            text[] NOT NULL DEFAULT '{}',
  status                text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','archived')),
  draft_version_id      uuid,                      -- FK ditambah di bawah (sirkular)
  published_version_id  uuid,
  shared_with_tenant    boolean NOT NULL DEFAULT false,   -- dibagikan ke semua guru di tenant (matriks akses)
  created_by            uuid NOT NULL REFERENCES users(id),
  archived_at           timestamptz,
  created_at            timestamptz NOT NULL DEFAULT now(),
  updated_at            timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, exam_code)
);
CREATE INDEX question_sets_list_idx ON question_sets (tenant_id, status, updated_at DESC);
CREATE INDEX question_sets_title_trgm ON question_sets USING gin ((title || ' ' || subject_name) gin_trgm_ops);

CREATE TABLE question_set_shares (
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  question_set_id  uuid NOT NULL REFERENCES question_sets(id) ON DELETE CASCADE,
  user_id          uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (question_set_id, user_id)
);

-- Versi: satu 'draft' (dapat diubah) + n 'published' (beku). Publish membekukan draft;
-- mengubah soal setelah publish = clone ke draft baru (FR-4.8).
CREATE TABLE question_set_versions (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  question_set_id  uuid NOT NULL REFERENCES question_sets(id) ON DELETE CASCADE,
  version_no       integer NOT NULL,
  status           text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  question_count   integer NOT NULL DEFAULT 0,
  max_points       numeric(8,2) NOT NULL DEFAULT 0,
  published_at     timestamptz,
  published_by     uuid REFERENCES users(id),
  created_at       timestamptz NOT NULL DEFAULT now(),
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (question_set_id, version_no)
);
CREATE UNIQUE INDEX qsv_one_draft_uq ON question_set_versions (question_set_id) WHERE status = 'draft';

ALTER TABLE question_sets
  ADD CONSTRAINT qs_draft_fk     FOREIGN KEY (draft_version_id)     REFERENCES question_set_versions(id) DEFERRABLE INITIALLY DEFERRED,
  ADD CONSTRAINT qs_published_fk FOREIGN KEY (published_version_id) REFERENCES question_set_versions(id) DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE stimuli (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id       uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  set_version_id  uuid NOT NULL REFERENCES question_set_versions(id) ON DELETE CASCADE,
  order_no        integer NOT NULL DEFAULT 0,
  title           text,
  content_html    text NOT NULL,              -- sudah disanitasi (allowlist) di server
  media_ids       uuid[] NOT NULL DEFAULT '{}'
);
CREATE INDEX stimuli_version_idx ON stimuli (set_version_id, order_no);

CREATE TABLE questions (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  set_version_id      uuid NOT NULL REFERENCES question_set_versions(id) ON DELETE CASCADE,
  order_no            integer NOT NULL,
  type                text NOT NULL CHECK (type IN ('mcq','mcq_complex','matching','short_answer','essay')),
  stimulus_id         uuid REFERENCES stimuli(id) ON DELETE SET NULL,
  body_html           text NOT NULL,
  options_json        jsonb NOT NULL DEFAULT '[]',   -- mcq: [{id,html}] ; matching: {left:[..],right:[..]}
  answer_key_json     jsonb,                         -- TIDAK pernah dikirim ke klien siswa
  rubric_json         jsonb,                         -- uraian
  points              numeric(6,2) NOT NULL CHECK (points >= 0),
  scoring_mode        text NOT NULL DEFAULT 'all_or_nothing' CHECK (scoring_mode IN ('all_or_nothing','partial')),
  partial_wrong_penalty boolean NOT NULL DEFAULT true,   -- FR-8.1b (benar - salah)
  shuffle_options     boolean NOT NULL DEFAULT true,
  explanation_html    text,
  media_ids           uuid[] NOT NULL DEFAULT '{}',
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT questions_order_uq UNIQUE (set_version_id, order_no) DEFERRABLE INITIALLY DEFERRED
);
CREATE INDEX questions_stimulus_idx ON questions (stimulus_id) WHERE stimulus_id IS NOT NULL;
```

**`000006_exams.up.sql`**

```sql
-- 000006: ujian, token, ruang, penugasan, peserta
CREATE TABLE exams (
  id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id               uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  question_set_id         uuid NOT NULL REFERENCES question_sets(id),
  set_version_id          uuid REFERENCES question_set_versions(id),  -- terisi saat publish
  name                    text NOT NULL,
  status                  text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','paused','closed','cancelled')),
  -- jadwal & aturan (FR-5.1 .. 5.5)
  open_at                 timestamptz,
  close_at                timestamptz,
  duration_minutes        integer CHECK (duration_minutes > 0),
  close_policy            text NOT NULL DEFAULT 'grace' CHECK (close_policy IN ('hard','grace')),
  login_mode              text NOT NULL DEFAULT 'both' CHECK (login_mode IN ('account','guest','both')),
  max_attempts            integer CHECK (max_attempts IS NULL OR max_attempts > 0),   -- NULL = tak terbatas
  score_policy            text NOT NULL DEFAULT 'highest' CHECK (score_policy IN ('highest','last','avg')),
  passing_score           numeric(6,2),
  show_score_immediately  boolean NOT NULL DEFAULT true,
  show_review             boolean NOT NULL DEFAULT false,
  show_explanations       boolean NOT NULL DEFAULT false,
  scores_released_at      timestamptz,
  shuffle_questions       boolean NOT NULL DEFAULT true,
  shuffle_options         boolean NOT NULL DEFAULT true,
  free_navigation         boolean NOT NULL DEFAULT true,
  require_all_answered    boolean NOT NULL DEFAULT false,
  single_session_policy   text NOT NULL DEFAULT 'replace_old' CHECK (single_session_policy IN ('reject_new','replace_old')),
  -- anti-cheat (FR-5.8A) — nilai default = tabel default PRD
  anticheat_enabled       boolean NOT NULL DEFAULT true,
  leave_tolerance_seconds integer NOT NULL DEFAULT 3 CHECK (leave_tolerance_seconds >= 0),
  penalty_mode            text NOT NULL DEFAULT 'points' CHECK (penalty_mode IN ('none','points','percent')),
  penalty_value           numeric(6,2) NOT NULL DEFAULT 5,
  penalty_max             numeric(6,2) NOT NULL DEFAULT 30,
  flag_threshold          integer DEFAULT 3 CHECK (flag_threshold IS NULL OR flag_threshold > 0),
  auto_submit_threshold   integer CHECK (auto_submit_threshold IS NULL OR auto_submit_threshold > 0),
  -- kamera (FR-5.8B)
  camera_enabled          boolean NOT NULL DEFAULT false,
  camera_deny_policy      text NOT NULL DEFAULT 'block' CHECK (camera_deny_policy IN ('block','allow_flag')),
  snapshot_interval_min_s integer NOT NULL DEFAULT 30,
  snapshot_interval_max_s integer NOT NULL DEFAULT 90,
  face_flag_penalty       boolean NOT NULL DEFAULT false,
  -- sumber kuota saat publish (FR 3.2)
  quota_source            text CHECK (quota_source IN ('trial','monthly','credit')),
  quota_ref_id            uuid,                         -- subscriptions.id atau exam_credits.id
  quota_refunded_at       timestamptz,
  published_at            timestamptz,
  created_by              uuid NOT NULL REFERENCES users(id),
  created_at              timestamptz NOT NULL DEFAULT now(),
  updated_at              timestamptz NOT NULL DEFAULT now(),
  CHECK (close_at IS NULL OR open_at IS NULL OR close_at > open_at),
  CHECK (snapshot_interval_max_s >= snapshot_interval_min_s),
  CHECK (status = 'draft' OR (open_at IS NOT NULL AND close_at IS NOT NULL AND duration_minutes IS NOT NULL AND set_version_id IS NOT NULL))
);
CREATE INDEX exams_tenant_status_idx ON exams (tenant_id, status, open_at);
CREATE INDEX exams_window_idx ON exams (tenant_id, close_at);       -- retensi: close_at ujian terakhir
CREATE INDEX exams_set_idx ON exams (question_set_id);

-- Ruang/sesi opsional dengan token sendiri (FR-4.9)
CREATE TABLE exam_rooms (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id    uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  name       text NOT NULL,
  UNIQUE (exam_id, name)
);

-- Token unik GLOBAL: login tanpa akun cukup kode ujian + token (tenant ditemukan dari token)
CREATE TABLE exam_tokens (
  token       text PRIMARY KEY CHECK (token ~ '^[A-Z0-9]{6,12}$'),
  tenant_id   uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id     uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  room_id     uuid REFERENCES exam_rooms(id) ON DELETE CASCADE,
  is_active   boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX exam_tokens_exam_idx ON exam_tokens (exam_id) WHERE is_active;

-- Penugasan: kelas / siswa (kombinasi diperbolehkan) — FR-4.2
CREATE TABLE exam_assignments (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id      uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  target_type  text NOT NULL CHECK (target_type IN ('class','student')),
  class_id     uuid REFERENCES classes(id) ON DELETE CASCADE,
  student_id   uuid REFERENCES students(id) ON DELETE CASCADE,
  CHECK ((target_type = 'class' AND class_id IS NOT NULL AND student_id IS NULL)
      OR (target_type = 'student' AND student_id IS NOT NULL AND class_id IS NULL))
);
CREATE UNIQUE INDEX exam_assign_class_uq ON exam_assignments (exam_id, class_id) WHERE class_id IS NOT NULL;
CREATE UNIQUE INDEX exam_assign_student_uq ON exam_assignments (exam_id, student_id) WHERE student_id IS NOT NULL;

-- Peserta hasil resolusi penugasan (kelas -> siswa) + penyesuaian per siswa
CREATE TABLE exam_participants (
  tenant_id            uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id              uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  student_id           uuid NOT NULL REFERENCES students(id) ON DELETE CASCADE,
  room_id              uuid REFERENCES exam_rooms(id) ON DELETE SET NULL,
  source               text NOT NULL DEFAULT 'assignment' CHECK (source IN ('assignment','manual','guest')),
  extra_time_seconds   integer NOT NULL DEFAULT 0,
  attempts_override    integer CHECK (attempts_override IS NULL OR attempts_override > 0),
  PRIMARY KEY (exam_id, student_id)
);
CREATE INDEX exam_participants_student_idx ON exam_participants (student_id);
```

**`000007_attempts.up.sql`**

```sql
-- 000007: pengerjaan (attempt), jawaban, event sinkron, pelanggaran, snapshot kamera
CREATE TABLE attempts (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id           uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  exam_id             uuid NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
  set_version_id      uuid NOT NULL REFERENCES question_set_versions(id),
  student_id          uuid NOT NULL REFERENCES students(id),
  attempt_no          integer NOT NULL CHECK (attempt_no > 0),
  class_snapshot      text,                                  -- nama kelas saat mengerjakan (FR-3.5)
  -- state machine (PRD 18.3); 'not_started' = belum ada baris
  status              text NOT NULL DEFAULT 'in_progress'
                      CHECK (status IN ('in_progress','pending_sync','submitted','awaiting_manual_grading','graded','expired','cancelled')),
  submit_reason       text CHECK (submit_reason IN ('manual','timeout','violation','force','offline_sync')),
  started_at          timestamptz NOT NULL DEFAULT now(),
  deadline_at         timestamptz NOT NULL,                  -- otoritas waktu di server
  submitted_at        timestamptz,
  -- nilai (FR-8.2): final = raw - penalti
  raw_score           numeric(8,2),
  max_score           numeric(8,2),
  penalty_points      numeric(8,2) NOT NULL DEFAULT 0,
  final_score         numeric(8,2),
  score_provisional   boolean NOT NULL DEFAULT false,        -- ada uraian belum dinilai
  passed              boolean,
  -- anti-cheat (FR-5.8C)
  cheating_status     text NOT NULL DEFAULT 'none'
                      CHECK (cheating_status IN ('none','warned','flagged','valid','penalty_cancelled','disqualified')),
  cheating_decided_by uuid REFERENCES users(id),
  cheating_decided_at timestamptz,
  cheating_reason     text,
  violation_count     integer NOT NULL DEFAULT 0,
  -- pengerjaan & pemantauan
  shuffle_seed        bigint NOT NULL,                       -- urutan soal/opsi deterministik per attempt
  extra_time_seconds  integer NOT NULL DEFAULT 0,
  answered_count      integer NOT NULL DEFAULT 0,
  last_seq            bigint NOT NULL DEFAULT 0,
  last_seen_at        timestamptz,
  device_info         jsonb NOT NULL DEFAULT '{}',
  consent_camera_at   timestamptz,
  consent_rules_at    timestamptz,
  created_at          timestamptz NOT NULL DEFAULT now(),
  updated_at          timestamptz NOT NULL DEFAULT now(),
  UNIQUE (exam_id, student_id, attempt_no)
);
CREATE INDEX attempts_exam_status_idx ON attempts (exam_id, status);
CREATE INDEX attempts_student_idx ON attempts (student_id, started_at DESC);
CREATE INDEX attempts_tenant_exam_idx ON attempts (tenant_id, exam_id);
CREATE INDEX attempts_grading_idx ON attempts (exam_id) WHERE status = 'awaiting_manual_grading';
CREATE INDEX attempts_flagged_idx ON attempts (exam_id) WHERE cheating_status IN ('flagged','warned');
CREATE INDEX attempts_deadline_idx ON attempts (deadline_at) WHERE status = 'in_progress';   -- worker auto-submit

-- Satu sesi aktif per siswa per ujian (FR-1.6)
CREATE TABLE attempt_sessions (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  attempt_id   uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  device_hash  text NOT NULL,
  started_at   timestamptz NOT NULL DEFAULT now(),
  revoked_at   timestamptz,
  revoked_by   text CHECK (revoked_by IN ('new_login','teacher','submit'))
);
CREATE UNIQUE INDEX attempt_sessions_one_active_uq ON attempt_sessions (attempt_id) WHERE revoked_at IS NULL;

-- Proyeksi jawaban terakhir per soal (hasil reduksi attempt_events, last-write-wins by seq)
CREATE TABLE attempt_answers (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  attempt_id       uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  question_id      uuid NOT NULL REFERENCES questions(id),
  answer_json      jsonb,
  flagged          boolean NOT NULL DEFAULT false,           -- ragu-ragu
  is_correct       boolean,
  auto_score       numeric(6,2),
  score            numeric(6,2),                             -- skor akhir soal (auto atau override/manual)
  graded_by        text CHECK (graded_by IN ('auto','manual')),
  grader_id        uuid REFERENCES users(id),
  grader_comment   text,
  override_reason  text,
  time_spent_ms    integer NOT NULL DEFAULT 0,
  seq              bigint NOT NULL DEFAULT 0,
  updated_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (attempt_id, question_id)
);
CREATE INDEX attempt_answers_question_idx ON attempt_answers (question_id);   -- analisis butir soal

-- Event append-only; idempoten via (attempt_id, client_event_id).
-- CATATAN: PRD menyebut partisi per bulan, tetapi UNIQUE lintas-partisi mewajibkan kolom partisi
-- ikut di indeks unik. Dipilih HASH(attempt_id) agar idempotensi dijamin DB; retensi tetap via cascade attempts.
CREATE TABLE attempt_events (
  id               bigint GENERATED ALWAYS AS IDENTITY,
  tenant_id        uuid NOT NULL,
  attempt_id       uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  client_event_id  uuid NOT NULL,
  type             text NOT NULL CHECK (type IN ('answer','flag','nav','submit','focus_lost','online','offline','sync','violation')),
  payload          jsonb NOT NULL DEFAULT '{}',
  client_ts        timestamptz,
  elapsed_ms       bigint,                                   -- jam monotonic klien sejak mulai (FR-7.5)
  server_ts        timestamptz NOT NULL DEFAULT now(),
  seq              bigint NOT NULL,
  accepted         boolean NOT NULL DEFAULT true,            -- false = lewat deadline + toleransi (AC-5)
  reject_reason    text,
  PRIMARY KEY (attempt_id, id),
  UNIQUE (attempt_id, client_event_id)
) PARTITION BY HASH (attempt_id);

DO $$ BEGIN
  FOR i IN 0..15 LOOP
    EXECUTE format('CREATE TABLE attempt_events_p%s PARTITION OF attempt_events FOR VALUES WITH (MODULUS 16, REMAINDER %s)', i, i);
  END LOOP;
END $$;
CREATE INDEX attempt_events_seq_idx ON attempt_events (attempt_id, seq);
CREATE INDEX attempt_events_rejected_idx ON attempt_events (attempt_id) WHERE NOT accepted;

CREATE TABLE attempt_violations (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id        uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  attempt_id       uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  client_event_id  uuid NOT NULL,
  type             text NOT NULL CHECK (type IN ('tab_hidden','blur','fullscreen_exit','app_background','camera_off','no_face','multi_face')),
  started_at       timestamptz NOT NULL,
  duration_ms      integer NOT NULL DEFAULT 0,
  counted          boolean NOT NULL DEFAULT false,           -- melewati toleransi
  penalty_applied  numeric(6,2) NOT NULL DEFAULT 0,
  waived_by        uuid REFERENCES users(id),
  waived_reason    text,
  created_at       timestamptz NOT NULL DEFAULT now(),
  UNIQUE (attempt_id, client_event_id)
);
CREATE INDEX attempt_violations_attempt_idx ON attempt_violations (attempt_id, started_at);

CREATE TABLE proctor_snapshots (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id     uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  attempt_id    uuid NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  object_key    text NOT NULL UNIQUE,
  trigger       text NOT NULL DEFAULT 'periodic' CHECK (trigger IN ('periodic','violation')),
  taken_at      timestamptz NOT NULL,
  size_bytes    integer NOT NULL DEFAULT 0,
  flags_json    jsonb NOT NULL DEFAULT '{}',                 -- {no_face:true, multi_face:true, camera_off:true}
  expires_at    timestamptz NOT NULL,                        -- 30 hari setelah ujian selesai / retensi tenant
  deleted_at    timestamptz,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX proctor_snapshots_attempt_idx ON proctor_snapshots (attempt_id, taken_at);
CREATE INDEX proctor_snapshots_expiry_idx ON proctor_snapshots (expires_at) WHERE deleted_at IS NULL;
```

**`000008_ops_retention.up.sql`**

```sql
-- 000008: audit, notifikasi, job file, pengaturan platform, fungsi retensi
CREATE TABLE audit_logs (
  id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id   uuid REFERENCES tenants(id) ON DELETE CASCADE,
  actor_type  text NOT NULL CHECK (actor_type IN ('user','student','system','super_admin')),
  actor_id    uuid,
  action      text NOT NULL,                   -- mis. question.update, attempt.reset, penalty.waive, snapshot.view
  entity      text NOT NULL,
  entity_id   uuid,
  meta        jsonb NOT NULL DEFAULT '{}',
  ip          inet,
  created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_tenant_idx ON audit_logs (tenant_id, created_at DESC);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity, entity_id);
CREATE INDEX audit_logs_created_idx ON audit_logs (created_at);          -- purge > 12 bulan

CREATE TABLE notifications (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  recipient_type text NOT NULL CHECK (recipient_type IN ('user','student')),
  user_id        uuid REFERENCES users(id) ON DELETE CASCADE,
  student_id     uuid REFERENCES students(id) ON DELETE CASCADE,
  type           text NOT NULL,                 -- exam_opening, essay_pending, cheating_flagged, quota_low, score_released
  title          text NOT NULL,
  body           text,
  data           jsonb NOT NULL DEFAULT '{}',
  read_at        timestamptz,
  created_at     timestamptz NOT NULL DEFAULT now(),
  CHECK ((recipient_type = 'user' AND user_id IS NOT NULL) OR (recipient_type = 'student' AND student_id IS NOT NULL))
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX notifications_student_idx ON notifications (student_id, created_at DESC) WHERE student_id IS NOT NULL;

-- Dedupe email pengingat (langganan H-7/H-3/H-1, retensi saat berakhir/H+3/H+6)
CREATE TABLE notice_log (
  tenant_id  uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  kind       text NOT NULL,
  ref        text NOT NULL DEFAULT '',
  sent_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, kind, ref)
);

-- Import/export besar yang diproses worker (preview -> commit, export semua data)
CREATE TABLE file_jobs (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  kind         text NOT NULL CHECK (kind IN ('student_import','question_import','tenant_export','report_export','login_cards')),
  status       text NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','running','awaiting_commit','done','failed','expired')),
  input_key    text,
  output_key   text,
  summary      jsonb NOT NULL DEFAULT '{}',      -- {valid:495, errors:[{row:12,reason:"..."}]} (AC-10)
  error        text,
  created_by   uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz,
  expires_at   timestamptz
);
CREATE INDEX file_jobs_tenant_idx ON file_jobs (tenant_id, created_at DESC);

CREATE TABLE platform_settings (
  key         text PRIMARY KEY,
  value       jsonb NOT NULL,
  updated_by  uuid REFERENCES users(id),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

-- ---------- Retensi 7 hari (PRD 3.2) ----------
-- Titik mulai = waktu LEBIH AKHIR antara berakhirnya hak akses dan close_at ujian terakhir.
CREATE OR REPLACE FUNCTION tenant_retention_start(p_tenant uuid) RETURNS timestamptz
LANGUAGE sql STABLE AS $$
  SELECT GREATEST(t.access_ended_at, (SELECT max(e.close_at) FROM exams e WHERE e.tenant_id = t.id AND e.status <> 'draft'))
  FROM tenants t WHERE t.id = p_tenant
$$;

-- Tenant yang siap dihapus: read_only, purge_after lewat, tidak ada pembayaran pending
CREATE OR REPLACE VIEW tenants_due_for_purge AS
SELECT t.id, t.purge_after
FROM tenants t
WHERE t.status = 'read_only' AND t.purge_after <= now()
  AND NOT EXISTS (SELECT 1 FROM payments p WHERE p.tenant_id = t.id AND p.status = 'pending')
  AND NOT EXISTS (SELECT 1 FROM subscriptions s WHERE s.tenant_id = t.id AND s.status = 'active' AND s.ends_at > now());

-- Daftar object key yang HARUS dihapus worker dari storage SEBELUM purge_tenant_data()
CREATE OR REPLACE FUNCTION tenant_object_keys(p_tenant uuid) RETURNS SETOF text LANGUAGE sql STABLE AS $$
  SELECT object_key FROM media_assets WHERE tenant_id = p_tenant
  UNION ALL SELECT object_key FROM proctor_snapshots WHERE tenant_id = p_tenant AND deleted_at IS NULL
  UNION ALL SELECT input_key  FROM file_jobs WHERE tenant_id = p_tenant AND input_key IS NOT NULL
  UNION ALL SELECT output_key FROM file_jobs WHERE tenant_id = p_tenant AND output_key IS NOT NULL
$$;

-- Hapus permanen data tenant; baris tenants + payments/invoices/subscriptions/exam_credits dipertahankan
-- (kebutuhan akuntansi & deteksi paket gratis ganda). Dijalankan oleh role sistem.
CREATE OR REPLACE FUNCTION purge_tenant_data(p_tenant uuid) RETURNS void LANGUAGE plpgsql AS $$
BEGIN
  DELETE FROM attempts          WHERE tenant_id = p_tenant;   -- cascade: answers, events, violations, snapshots, sessions
  DELETE FROM exams             WHERE tenant_id = p_tenant;   -- cascade: tokens, rooms, assignments, participants
  DELETE FROM question_sets     WHERE tenant_id = p_tenant;   -- cascade: versions, stimuli, questions, shares
  DELETE FROM refresh_tokens    WHERE tenant_id = p_tenant;
  DELETE FROM students          WHERE tenant_id = p_tenant;
  DELETE FROM classes           WHERE tenant_id = p_tenant;
  DELETE FROM media_assets      WHERE tenant_id = p_tenant;
  DELETE FROM notifications     WHERE tenant_id = p_tenant;
  DELETE FROM file_jobs         WHERE tenant_id = p_tenant;
  DELETE FROM audit_logs        WHERE tenant_id = p_tenant;
  DELETE FROM one_time_tokens   WHERE tenant_id = p_tenant;
  DELETE FROM users             WHERE tenant_id = p_tenant;
  UPDATE tenants SET status = 'purged', purged_at = now(), storage_used_bytes = 0 WHERE id = p_tenant;
END $$;

-- Pemakaian tenant untuk pengecekan batas tier siswa (FR-3.3b) & Super Admin console
CREATE OR REPLACE VIEW tenant_usage AS
SELECT t.id AS tenant_id,
       (SELECT count(*) FROM students s WHERE s.tenant_id = t.id AND s.status = 'active' AND s.merged_into_id IS NULL) AS active_students,
       t.storage_used_bytes, t.storage_quota_bytes
FROM tenants t;
```

**`000009_rls_grants_triggers.up.sql`**

```sql
-- 000009: role DB, Row-Level Security multi-tenant, trigger updated_at
-- Role login dibuat ops: ALTER ROLE flowcbt_app LOGIN PASSWORD '...'; ALTER ROLE flowcbt_system LOGIN PASSWORD '...';
--  * flowcbt_app    : dipakai API; tunduk RLS (SET LOCAL app.tenant_id per transaksi)
--  * flowcbt_system : BYPASSRLS; dipakai login (cari user/siswa lintas tenant), webhook, worker retensi, Super Admin
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'flowcbt_app')    THEN CREATE ROLE flowcbt_app NOLOGIN; END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'flowcbt_system') THEN CREATE ROLE flowcbt_system NOLOGIN BYPASSRLS; END IF;
END $$;

-- trigger updated_at untuk semua tabel yang punya kolom updated_at
DO $$ DECLARE t text; BEGIN
  FOR t IN SELECT c.table_name FROM information_schema.columns c
           JOIN information_schema.tables tb ON tb.table_schema = c.table_schema AND tb.table_name = c.table_name
           WHERE c.table_schema = 'public' AND c.column_name = 'updated_at' AND tb.table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('CREATE TRIGGER trg_%s_updated BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION set_updated_at()', t, t);
  END LOOP;
END $$;

-- RLS: semua tabel ber-kolom tenant_id
DO $$ DECLARE t text; BEGIN
  FOR t IN SELECT c.table_name FROM information_schema.columns c
           JOIN information_schema.tables tb ON tb.table_schema = c.table_schema AND tb.table_name = c.table_name
           WHERE c.table_schema = 'public' AND c.column_name = 'tenant_id' AND tb.table_type = 'BASE TABLE'
  LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id())', t);
  END LOOP;
END $$;

ALTER TABLE tenants ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_self ON tenants USING (id = current_tenant_id()) WITH CHECK (id = current_tenant_id());

-- Hak akses role aplikasi
GRANT USAGE ON SCHEMA public TO flowcbt_app, flowcbt_system;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO flowcbt_app, flowcbt_system;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO flowcbt_app, flowcbt_system;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO flowcbt_app, flowcbt_system;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO flowcbt_app, flowcbt_system;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO flowcbt_app, flowcbt_system;

-- Tabel lintas-tenant: aplikasi hanya membaca paket; webhook hanya role sistem
REVOKE INSERT, UPDATE, DELETE ON plans FROM flowcbt_app;
REVOKE ALL ON payment_webhook_events, platform_settings FROM flowcbt_app;
-- Append-only: log audit & event jawaban tidak boleh diubah/dihapus oleh API
REVOKE UPDATE, DELETE ON audit_logs FROM flowcbt_app;
REVOKE UPDATE, DELETE ON attempt_events FROM flowcbt_app;
-- Fungsi purge hanya role sistem
REVOKE EXECUTE ON FUNCTION purge_tenant_data(uuid), tenant_object_keys(uuid) FROM flowcbt_app, PUBLIC;
GRANT  EXECUTE ON FUNCTION purge_tenant_data(uuid), tenant_object_keys(uuid) TO flowcbt_system;
```

**`000010_seed_plans.up.sql`**

```sql
-- 000010: seed paket (PRD 3.1) & pengaturan awal. Semua dapat diubah Super Admin tanpa deploy.
INSERT INTO plans (code, name, type, price_idr, student_min, student_max, exam_quota, duration_days, sort_order) VALUES
  ('free_trial',   'Gratis / Trial',        'free',     0,     1,  10,   1,  14, 10),
  ('per_exam_s',   'Per Ujian — Kecil',     'per_exam', 10000, 11, 40,   3,  NULL, 20),
  ('per_exam_m',   'Per Ujian — Menengah',  'per_exam', 20000, 41, 70,   7,  NULL, 21),
  ('per_exam_l',   'Per Ujian — Besar',     'per_exam', 30000, 71, 100,  12, NULL, 22),
  ('monthly_m',    'Bulanan — Menengah',    'monthly',  50000, 1,  50,   NULL, 30, 30),
  ('monthly_l',    'Bulanan — Besar',       'monthly',  70000, 51, 100,  NULL, 30, 31)
ON CONFLICT (code) DO NOTHING;

INSERT INTO platform_settings (key, value) VALUES
  ('retention.readonly_days',        '7'),
  ('retention.snapshot_days',        '30'),
  ('retention.audit_months',         '12'),
  ('limits.max_students_per_tenant', '100'),
  ('storage.default_quota_bytes',    '1073741824')
ON CONFLICT (key) DO NOTHING;
```

### 18.7 Struktur Proyek (Go + React)

Monorepo dengan dua bagian: `backend/` (Go, satu modul, tiga biner) dan `web/` (pnpm workspace + Turborepo, dua aplikasi React). Pemetaan ke layanan Coolify (Bagian 7.4): `flowcbt-api` ← `cmd/api`, `flowcbt-worker` ← `cmd/worker`, `flowcbt-web` ← build statis kedua aplikasi web.

```
flowcbt/
├─ backend/
│  ├─ cmd/
│  │  ├─ api/main.go              # HTTP API + WebSocket/SSE, /healthz, /internal/active-exams (deploy freeze)
│  │  ├─ worker/main.go           # asynq: import/export, email, auto-submit, re-grade, retensi, rekonsiliasi
│  │  └─ migrate/main.go          # golang-migrate (dijalankan terpisah dari deploy API)
│  ├─ internal/
│  │  ├─ platform/                # config, logger, db (pgxpool + WithTenantTx / WithSystemTx),
│  │  │                           # redis, storage (S3/MinIO), mailer, metrics, clock, idgen
│  │  ├─ httpx/                   # router chi, middleware (requestid, recover, auth, tenant, rbac,
│  │  │                           # ratelimit, idempotency), error {code,message,details}, pagination, validator
│  │  ├─ auth/                    # register, login, refresh rotasi, Google, reset, login siswa & tamu, JWT, argon2id
│  │  ├─ tenant/  user/           # tenant, undangan guru, peran, batas kelas/mapel guru (M1, FR-1.5)
│  │  ├─ classroom/  student/     # kelas, siswa, import/export, naik kelas, merge tamu, track record (M3)
│  │  ├─ questionbank/            # paket soal, versi, soal 5 tipe, stimulus, media, import/export, publish (M4)
│  │  ├─ exam/                    # ujian, token, ruang, penugasan→peserta, aksi pengawas (M5)
│  │  ├─ attempt/                 # start, paket soal tanpa kunci, answers:sync, submit, pemulihan, deadline (M6/M7)
│  │  ├─ grading/                 # mesin penilaian MURNI (tanpa DB): semua tipe, parsial, normalisasi, penalti (M8)
│  │  ├─ proctor/                 # pelanggaran, penalti, keputusan guru, snapshot kamera
│  │  ├─ report/                  # live, historis, analisis butir, track record, export (M9)
│  │  ├─ realtime/                # hub SSE/WS + Redis pub/sub
│  │  ├─ billing/                 # kuota resolver, subscription/credit, PaymentProvider
│  │  │  └─ provider/{midtrans,mayar}/
│  │  ├─ retention/               # job baca-saja, pengingat, export semua data, purge
│  │  ├─ notify/                  # in-app + email (template Bahasa Indonesia)
│  │  ├─ fileio/                  # excelize (xlsx), chromedp (PDF), file_jobs
│  │  ├─ audit/                   # penulis audit_logs
│  │  └─ admin/                   # Super Admin (M12)
│  ├─ db/
│  │  ├─ migrations/              # 000001..000010 (*.up.sql / *.down.sql)
│  │  ├─ queries/                 # *.sql untuk sqlc
│  │  └─ sqlc.yaml
│  ├─ api/openapi.yaml            # kontrak API (sumber untuk api-client frontend)
│  ├─ test/{integration,loadtest} # testcontainers (RLS, webhook) · k6 (1.000 siswa)
│  ├─ Dockerfile.api  Dockerfile.worker  Makefile  go.mod
│  └─ .golangci.yml               # depguard: pool sistem hanya boleh diimpor auth, billing/webhook, retention, admin
│
├─ web/                           # pnpm workspace + Turborepo
│  ├─ apps/
│  │  ├─ teacher/                 # app.<domain> — guru/admin + rute /admin (Super Admin)
│  │  │  └─ src/
│  │  │     ├─ app/               # router, providers (Query, Theme, Auth), layout sidebar/bottom-nav
│  │  │     ├─ features/
│  │  │     │  ├─ auth/  dashboard/  classes/  students/
│  │  │     │  ├─ question-sets/   # daftar + editor TipTap/KaTeX + import/export + preview
│  │  │     │  ├─ exams/           # wizard (Info→Soal→Peserta→Jadwal→Publish), detail & aksi pengawas
│  │  │     │  ├─ live-report/  history-report/  cheating/  grading/
│  │  │     │  ├─ billing/  settings/  admin/
│  │  │     └─ lib/
│  │  └─ student/                 # ujian.<domain> — PWA siswa (mobile-first)
│  │     └─ src/
│  │        ├─ app/  sw.ts        # Workbox (precache aset, runtime cache media)
│  │        ├─ features/
│  │        │  ├─ auth/           # login akun, login tanpa akun (guest)
│  │        │  ├─ exams/          # dashboard, token & tata tertib
│  │        │  ├─ attempt/        # player/, question-types/{Mcq,McqComplex,Matching,ShortAnswer,Essay},
│  │        │  │                  # navigator/ (grid + bottom sheet), review/, done/
│  │        │  ├─ offline/        # db.ts (Dexie), outbox.ts, sync-engine.ts, clock.ts (monotonic),
│  │        │  │                  # recovery.ts, media-cache.ts, connection-status.ts
│  │        │  ├─ proctor/        # leave-detector.ts, camera.ts, face-check.ts, snapshot-queue.ts
│  │        │  └─ history/
│  │        └─ lib/
│  ├─ packages/
│  │  ├─ ui/                      # shadcn/ui, token tema light/dark, BottomSheet, Toast, ConnectionBadge
│  │  ├─ api-client/              # klien + hook TanStack Query, di-generate dari openapi.yaml
│  │  ├─ shared/                  # skema zod, tipe soal, format tanggal/zona waktu, i18n id
│  │  └─ config/                  # tsconfig, eslint, preset Tailwind
│  ├─ e2e/                        # Playwright: teacher/, student/, offline/ (jaringan putus/lambat/flapping)
│  └─ Dockerfile.web              # Nginx/Caddy: routing per host (app.* / ujian.*), SPA fallback, cache header
│
├─ deploy/                        # docker-compose.dev.yml (pg16, redis, minio, mailpit), catatan Coolify, runbook
├─ .github/workflows/             # ci.yml (lint, unit, integration, e2e) · release.yml (tag → Coolify)
└─ docs/                          # ADR, panduan guru/siswa
```

**Konvensi backend**
- Alur berlapis per modul: `handler` (HTTP, validasi, DTO) → `service` (aturan bisnis, transaksi) → `repo` (pembungkus sqlc). Modul berkomunikasi lewat interface kecil, bukan impor repo modul lain.
- Semua query tenant berjalan di `db.WithTenantTx(ctx, tenantID, fn)` yang menjalankan `SET LOCAL app.tenant_id`; `tenantID` diambil dari JWT, tidak pernah dari input klien (Bagian 9). `WithSystemTx` hanya untuk paket yang diizinkan `depguard`.
- `grading` tidak mengakses database: fungsi murni (`Score(question, answer) → ScoreResult`) agar mudah diuji tabel-driven (rumus FR-8.1b, normalisasi isian, penalti FR-5.8).
- Setiap endpoint sinkron/pembayaran menerima `Idempotency-Key` atau `client_event_id`; kode error stabil (`QUOTA_EXCEEDED`, `ATTEMPT_EXPIRED`, `TOKEN_INVALID`).
- Migrasi wajib *backward-compatible* (deploy freeze saat ada ujian berjalan).

**Konvensi frontend**
- Aplikasi siswa dipisah dari aplikasi guru agar bundel awal siswa kecil (FR-11.5); editor TipTap/KaTeX hanya dimuat di aplikasi guru.
- Seluruh akses jaringan lewat `api-client`; tulis jawaban siswa **selalu ke IndexedDB dulu**, UI tidak pernah menunggu server (FR-6.3, FR-7.3).
- Status penting tidak hanya mengandalkan warna (ikon + teks), semua komponen diuji pada tema terang/gelap dan viewport 360 px.

**Pemetaan modul PRD → kode**

| Modul PRD | Backend | Frontend |
|---|---|---|
| M1 Auth & Tenant | `auth`, `tenant`, `user` | `teacher/features/auth`, `student/features/auth` |
| M2 Dashboard | `report` | `teacher/features/dashboard` |
| M3 Siswa & Kelas | `classroom`, `student`, `fileio` | `teacher/features/classes, students` |
| M4 Bank Soal | `questionbank`, `fileio` | `teacher/features/question-sets` |
| M5 Ujian | `exam`, `proctor` | `teacher/features/exams, cheating` |
| M6 Klien Ujian | `attempt` | `student/features/exams, attempt` |
| M7 Offline | `attempt` (sync) | `student/features/offline`, `sw.ts` |
| M8 Penilaian | `grading` | `teacher/features/grading` |
| M9 Laporan | `report`, `realtime` | `live-report`, `history-report` |
| M10 Billing | `billing`, `retention` | `teacher/features/billing` |
| M11 Tema/UX | — | `packages/ui`, `packages/config` |
| M12 Super Admin | `admin` | `teacher/features/admin` |
| M13 Notifikasi | `notify` | `packages/ui` (toast), `notifications` |

### 18.8 Rincian Sprint per Fase

**Asumsi perencanaan:** 1 sprint = 1 minggu; durasi tiap fase memakai **batas atas** Bagian 14 (total **32 sprint ≈ 7,5 bulan**). Tim acuan: 1–2 developer Go, 1–2 developer React, 1 QA/DevOps paruh waktu. Bila tim lebih kecil, jumlah sprint bertambah, bukan cakupan berkurang. Label: **[BE]** backend, **[FE]** frontend, **[QA/Ops]** pengujian & infrastruktur. Prioritas mengikuti Bagian 4.2: **P0** wajib untuk beta tertutup, **P1** untuk rilis publik.

Setiap sprint berakhir dengan demo di staging dan memenuhi *Definition of Done* Bagian 14.1.

#### Fase 0 — Fondasi (S1–S3)

| Sprint | Sasaran | Pekerjaan | Keluaran / Kriteria selesai |
|---|---|---|---|
| **S1** | Repo & pipeline berdiri | [QA/Ops] monorepo, GitHub Actions (lint, test, build), `docker-compose.dev`, Coolify staging, deploy dari `main`. [BE] kerangka Go (config, logger, chi, `/healthz`, format error, `cmd/migrate`), `openapi.yaml` awal. [FE] Vite + TS + Tailwind + shadcn, token tema terang/gelap, shell layout guru & siswa | Push ke `main` otomatis ter-deploy ke staging; `/healthz` hijau; toggle tema berfungsi |
| **S2** | Skema & isolasi tenant | [BE] terapkan migrasi 000001–000004 dan 000009, setup sqlc, `WithTenantTx`, tes integrasi isolasi tenant (testcontainers), registrasi + verifikasi email, login, refresh rotasi, lockout, rate limit Redis. [FE] halaman `/login`, `/register`, setup `api-client` dari OpenAPI + TanStack Query | Registrasi → verifikasi → login end-to-end; tes RLS otomatis lulus (dasar AC-9) |
| **S3** | Multi-tenant lengkap | [BE] JWT claims, RBAC middleware, undangan guru (`one_time_tokens`), Login with Google, reset password, mailer + worker asynq, penulis `audit_logs`. [FE] kerangka `/dashboard`, `/settings` (profil, guru, tema) | Admin dapat mengundang guru; peran ditegakkan; audit log tercatat. **Milestone M1: fondasi selesai** |

#### Fase 1 — Core Soal & Siswa (S4–S8)

| Sprint | Sasaran | Pekerjaan | Keluaran / Kriteria selesai |
|---|---|---|---|
| **S4** | Kelas & siswa | [BE] CRUD kelas & siswa, generator `student_code`, `student_class_history`, cek batas tier (FR-3.3b). [FE] `/classes`, `/students` (tabel → kartu di mobile), form siswa | CRUD berfungsi di desktop & mobile; penambahan melewati tier ditolak dengan ajakan upgrade |
| **S5** | Import/export siswa | [BE] migrasi 000008 (`file_jobs`), template xlsx, preview/validasi/commit, kelas otomatis, kredensial awal, export xlsx/csv, kartu login siswa PDF, naik/pindah kelas massal. [FE] alur import 3 langkah, aksi massal | **AC-10** lulus (500 baris, 5 error → 495 valid + nomor baris & alasan) |
| **S6** | Bank soal (backend) | [BE] migrasi 000005, `question_sets`, versi draft, 5 tipe soal, stimulus, upload media ke MinIO + kuota penyimpanan, sanitasi HTML allowlist, reorder. [FE] `/question-sets` (daftar, cari, form info paket) | Paket soal & soal 5 tipe tersimpan; XSS pada rich text diuji dan ditolak |
| **S7** | Editor soal | [FE] TipTap + KaTeX, editor per tipe (opsi, kunci, menjodohkan, isian multi-variasi, rubrik uraian), stimulus bersama, autosave, drag-drop urutan, pratinjau desktop & mobile. [BE] endpoint autosave ringan | Guru membuat satu paket 5 tipe soal ≤ 10 menit (G1 awal) |
| **S8** | Publish & template soal | [BE] publish (validasi kelengkapan, pembekuan versi, clone-ke-draft), unpublish, duplikat/arsip, import/export template xlsx, audit perubahan kunci. [FE] alur publish, import/export template. Pengecekan kuota diberi *stub* (penuh di S27) | **Milestone M2:** guru dapat daftar → import siswa → buat & publish soal |

#### Fase 2 — Ujian & Siswa Client (S9–S14)

| Sprint | Sasaran | Pekerjaan | Keluaran / Kriteria selesai |
|---|---|---|---|
| **S9** | Ujian (backend) & wizard awal | [BE] migrasi 000006, CRUD `exams`, `exam_tokens` & ruang, resolver penugasan kelas/siswa → `exam_participants`, validasi jadwal (FR-5.1–5.5). [FE] wizard langkah Info & Soal | Ujian dapat dibuat dari paket soal terbit |
| **S10** | Wizard & aksi pengawas | [FE] langkah Peserta, Jadwal, Publish; detail ujian `/exams/:id`; regenerasi token. [BE] endpoint tambah waktu, reset attempt, force submit, jeda/tutup darurat (FR-5.7) + audit | Ujian terjadwal & token terbit; aksi pengawas teruji API |
| **S11** | Login siswa & mulai ujian | [BE] login siswa (akun), satu sesi aktif, `GET /me/exams`, `POST /exams/{id}/start` (token, `deadline_at`, `shuffle_seed`, paket soal **tanpa kunci**, cache Redis), migrasi 000007. [FE] app siswa mobile-first: login, dashboard ujian, halaman token & tata tertib | Siswa mulai ujian; deadline ditetapkan server; **AC-6** (batas percobaan, `409`) |
| **S12** | Butir soal | [FE] halaman soal 5 tipe (menjodohkan tap-pilih di mobile), header timer sticky, Sebelumnya/Berikutnya/Ragu-ragu, target sentuh ≥ 44 px. [BE] `answers:sync` versi dasar (online) | Siswa mengerjakan 5 tipe soal di layar 360 px |
| **S13** | Daftar soal, selesai, penilaian | [FE] grid daftar soal (ikon + warna), bottom sheet, review & konfirmasi, layar selesai. [BE] submit, auto-submit waktu habis (worker pemindai deadline), **mesin `grading`** (semua tipe, FR-8.1b, normalisasi isian), `GET /attempts/{id}/result` | Tes tabel-driven mesin penilaian lulus; skor akhir benar sesuai rumus |
| **S14** | Koreksi & penyelesaian fase | [FE/BE] koreksi uraian per soal/siswa dengan rubrik, override skor + alasan (audit), re-grade, nilai *Sementara*; mode terang/gelap final, audit aksesibilitas (axe). [QA] E2E Playwright guru→siswa (desktop & mobile) | **Milestone M3:** ujian online end-to-end (tanpa offline); **AC-12** awal |

#### Fase 3 — Offline & Sinkronisasi (S15–S18)

| Sprint | Sasaran | Pekerjaan | Keluaran / Kriteria selesai |
|---|---|---|---|
| **S15** | PWA & penyimpanan lokal | [FE] Workbox (precache aset), manifest installable, skema Dexie (attempt, soal, outbox, media), pra-muat paket soal + media dengan progress, ujian baru mulai setelah paket lengkap | Halaman ujian terbuka saat offline; paket soal tersimpan di IndexedDB |
| **S16** | Antrean & sinkronisasi | [FE] outbox event (`client_event_id`, `sequence_number`), sync engine (batch 5–10 dtk, event `online`, backoff, hapus antrean hanya setelah ACK). [BE] `answers:sync` idempoten, reducer *last-write-wins* by `seq` → `attempt_answers` | **AC-1, AC-2, AC-3** lulus |
| **S17** | Timer, submit & pemulihan | [FE] timer monotonic terkalibrasi server, submit offline (`pending_sync`), indikator 🟢🟡🔴, peringatan jangan hapus data browser, pemulihan tab/browser. [BE] validasi deadline + toleransi (`accepted=false` + alasan), `GET /attempts/{id}` untuk pemulihan | **AC-4, AC-5** lulus; skenario 10.3 terpenuhi |
| **S18** | Uji jaringan buruk | [QA] E2E offline: putus/lambat/*flapping*, reload, tutup-buka browser, submit offline; Safari iOS (sinkron saat aktif + `online` + polling, tanpa bergantung Background Sync); Android low-end; k6 skenario sinkron awal. [FE] pembersihan IndexedDB setelah sinkron penuh; enkripsi Web Crypto (opsional) | **Milestone M4:** nol kehilangan jawaban pada seluruh skenario uji |

#### Fase 3b — Anti-Cheat & Proctoring (S19–S22)

| Sprint | Prioritas | Sasaran | Pekerjaan | Keluaran / Kriteria selesai |
|---|---|---|---|---|
| **S19** | **P0** | Deteksi keluar aplikasi | [FE] `visibilitychange`, blur, keluar layar penuh, background, toleransi detik; event pelanggaran aman saat offline; persetujuan tata tertib. [BE] `POST /attempts/{id}/violations`, penalti dihitung **di server** (poin/persen, batas maksimum), peringatan; konfigurasi anti-cheat di wizard | Pelanggaran tercatat meski offline; peringatan "ke-n, sisa m" tampil |
| **S20** | **P0** | Penalti & laporan curang | [BE] ambang *flagged*, auto-submit opsional, nilai asli/penalti/akhir terpisah, keputusan guru (Valid / Batalkan penalti / Diskualifikasi) + audit. [FE] halaman `/exams/:id/cheating` (linimasa), nonaktifkan copy-paste/klik kanan di area soal | **AC-8** lulus. **Milestone M5:** seluruh fitur P0 siswa selesai |
| **S21** | P1 | Login tanpa akun | [BE] `student/auth/guest` (data diri + kode + token), pencocokan NIS / nama+kelas ternormalisasi, peserta tamu, cookie perangkat httpOnly, merge tamu→siswa, rate limit kode/token, cek tier. [FE] halaman `/guest`, UI merge di guru | Siswa tanpa akun tertaut ke track record; uji brute-force token lulus |
| **S22** | P1 | Proctoring kamera | [FE] layar persetujuan, `getUserMedia`, snapshot acak 30–90 dtk + saat pelanggaran (JPEG kecil), deteksi wajah ringan (penanda), kebijakan izin ditolak, antrean offline berbatas. [BE] upload ke MinIO terenkripsi, akses tercatat audit, job hapus snapshot 30 hari. [FE guru] galeri snapshot | Snapshot tersinkron setelah jawaban; hanya guru/admin tenant yang bisa melihat |

#### Fase 4 — Laporan (S23–S26)

| Sprint | Prioritas | Sasaran | Pekerjaan | Keluaran / Kriteria selesai |
|---|---|---|---|---|
| **S23** | **P0** | Laporan real-time | [BE] agregat status (sedang/belum/selesai), progres, `last_seen_at`, SSE/WebSocket via Redis pub/sub + fallback polling 10 dtk. [FE] `/reports/live`: donut/bar, tabel per status, filter kelas, pencarian, aksi pengawas, indikator offline/pelanggaran/curang | Dashboard diperbarui real-time; aksi pengawas berfungsi dari halaman ini |
| **S24** | **P0** | Laporan historis | [BE] daftar + filter + pencarian (`pg_trgm`), statistik (rata-rata, median, min/maks, kelulusan, histogram, durasi), kebijakan nilai akhir multi-percobaan. [FE] `/reports/history`, `/reports/exams/:id`, rekap nilai per ujian | Laporan sesuai rumus; filter & pencarian < 300 ms (p95) |
| **S25** | **P0** | Track record & rilis nilai | [BE/FE] `/students/:id` (riwayat, grafik tren, detail attempt, riwayat event), `/history` siswa, rilis nilai (`scores_released_at`) + notifikasi in-app | Track record melekat ke siswa walau pindah kelas (snapshot kelas) |
| **S26** | P1 | Analisis & export | [BE] analisis butir (persen benar, kesukaran, daya pembeda, distraktor), export xlsx/csv/PDF (chromedp) lewat worker; optimasi indeks/cache laporan. [FE] tampilan analisis & tombol export | Export tuntas untuk 100 siswa < 30 dtk |

#### Fase 5 — Billing & Beta (S27–S30)

| Sprint | Prioritas | Sasaran | Pekerjaan | Keluaran / Kriteria selesai |
|---|---|---|---|---|
| **S27** | **P0** | Kuota & langganan | [BE] migrasi 000003 & 000010 diaktifkan penuh, resolver kuota (urutan: bulanan → trial → kredit), publish gating (**AC-7**), kredit kembali saat unpublish, *grace* ujian berjalan, batas tier siswa, trial otomatis saat registrasi + anti-penyalahgunaan. [FE] banner kuota & halaman paket | Publish ditolak tanpa kuota dengan tautan beli; ujian berjalan tidak terputus |
| **S28** | **P0** | Midtrans & invoice | [BE] antarmuka `PaymentProvider`, adapter Midtrans (checkout, verifikasi signature, webhook idempoten via `payment_webhook_events`), aktivasi paket, invoice PDF + email, upgrade prorata & downgrade terjadwal, rekonsiliasi harian. [FE] `/billing` (checkout, riwayat, invoice) | Pembayaran sandbox mengaktifkan paket otomatis; webhook ganda tidak menggandakan kredit |
| **S29** | P0 → P1 | Retensi & Mayar | [BE] worker retensi: status baca-saja, `purge_after`, email saat berakhir/H+3/H+6 + pengingat langganan H-7/H-3/H-1, **Export Semua Data**, hapus object storage → `purge_tenant_data()`, pemulihan bila perpanjang (**AC-11**). **[P1]** adapter Mayar. [FE] mode baca-saja + tombol export | Penghapusan hanya terjadi bila tidak ada transaksi tertunda; audit tercatat |
| **S30** | P0 / P1 | Operasional & beta tertutup | [QA/Ops] Coolify produksi, backup Postgres + WAL ke luar VPS, uji restore, monitoring (Prometheus, alert WA/Telegram), *deploy freeze*, **uji beban k6 ±1.000 siswa**. [BE/FE] Super Admin console (**P0** minimal: tenant, paket; **P1** penuh: pembayaran, metrik, impersonasi + audit) | **Milestone M6: beta tertutup dimulai** (kriteria MVP Bagian 14 terpenuhi) |

#### Fase 6 — Peluncuran (S31–S32)

| Sprint | Sasaran | Pekerjaan | Keluaran / Kriteria selesai |
|---|---|---|---|
| **S31** | Hardening | OWASP Top 10, audit dependensi, uji brute-force token, tuning performa dari data beta, axe + uji keyboard, matriks perangkat nyata (Android low-end, iPhone Safari, Chrome desktop, 360 px), perbaikan bug beta | Semua AC-1…AC-12 hijau di staging; p95 API < 300 ms |
| **S32** | Rilis publik | Dokumentasi (OpenAPI final, panduan guru & siswa), onboarding (empty state, tur singkat), halaman harga, runbook & restore drill, tag rilis produksi, dasbor KPI Bagian 13 | **Milestone M7:** rilis publik (P1 lengkap) |

**Ringkasan milestone:** M1 (S3) fondasi · M2 (S8) guru dapat membuat & publish soal · M3 (S14) ujian online end-to-end · M4 (S18) offline tanpa kehilangan jawaban · M5 (S20) seluruh fitur P0 siswa · M6 (S30) beta tertutup · M7 (S32) rilis publik.

**Risiko jadwal & penyangga:** (1) Editor soal (S7) dan sinkronisasi offline (S16–S18) paling berisiko molor; sisakan penyangga ±1 sprint tiap fase tersebut. (2) Bila beta perlu dipercepat, geser P1 (S21–S22, S26, Mayar di S29) setelah beta tanpa mengubah urutan P0. (3) Akun Midtrans/Mayar harus sudah terverifikasi sebelum S28 (Bagian 16).
