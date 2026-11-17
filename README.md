# FlowCBT

CBT (Computer Based Test) ujian online multi-sekolah, berbayar (SaaS). Implementasi PRD v1.3.

## Struktur

```
flowcbt/
├─ backend/     Go API + worker + migrasi PostgreSQL 16
├─ web/         pnpm workspace — aplikasi React guru & siswa (PWA)
├─ deploy/      docker-compose.dev, catatan Coolify
└─ docs/        ADR, panduan
```

## Menjalankan (dev)

```bash
# Backend
cd backend
go run ./cmd/api          # API + /healthz

# Migrasi database (butuh PostgreSQL 16, DATABASE_URL)
go run ./cmd/migrate -direction up

# Frontend
cd web
pnpm install
pnpm dev                  # teacher & student app (Vite)
pnpm build
```

## Roadmap

Fase 0–6 mengikuti PRD Bagian 14 (32 sprint). Status:

- [ ] Fase 0 — Fondasi (auth, multi-tenant)
- [ ] Fase 1 — Core soal & siswa
- [ ] Fase 2 — Ujian & klien siswa
- [ ] Fase 3 — Offline & sinkronisasi
- [ ] Fase 3b — Anti-cheat & proctoring
- [ ] Fase 4 — Laporan
- [ ] Fase 5 — Billing & beta
- [ ] Fase 6 — Peluncuran
