// Command migrate menjalankan golang-migrate terpisah dari deploy API (PRD 7.4).
// Pemakaian: go run ./cmd/migrate -direction up [-steps N]
package main

import (
	"errors"
	"flag"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/flowcbt/backend/db/migrations"
)

func main() {
	dir := flag.String("direction", "up", "arah migrasi: up | down")
	steps := flag.Int("steps", -1, "jumlah langkah (-1 = sampai ujung)")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL wajib diisi")
	}

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		log.Fatalf("gagal memuat migrasi: %v", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, "postgres://"+trimScheme(dsn))
	if err != nil {
		log.Fatalf("gagal inisialisasi migrate: %v", err)
	}
	defer func() { _, _ = m.Close() }()

	switch *dir {
	case "up":
		if *steps >= 0 {
			if err := m.Steps(*steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				log.Fatalf("migrate up: %v", err)
			}
		} else {
			if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				log.Fatalf("migrate up: %v", err)
			}
		}
	case "down":
		if *steps >= 0 {
			if err := m.Steps(-*steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				log.Fatalf("migrate down: %v", err)
			}
		} else {
			if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
				log.Fatalf("migrate down: %v", err)
			}
		}
	default:
		log.Fatalf("direction tidak dikenal: %s (gunakan up|down)", *dir)
	}
	log.Printf("migrasi %s selesai", *dir)
}

// trimScheme mengganti prefix database URL agar sesuai driver golang-migrate.
func trimScheme(dsn string) string {
	const pgPrefix = "postgres://"
	if len(dsn) >= len(pgPrefix) && dsn[:len(pgPrefix)] == pgPrefix {
		return dsn[len(pgPrefix):]
	}
	return dsn
}
