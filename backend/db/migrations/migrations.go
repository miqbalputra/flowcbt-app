// Package migrations menanam (embed) seluruh file migrasi SQL agar binary
// migrate self-contained (tidak membaca folder saat runtime).
package migrations

import "embed"

// FS berisi pasangan file NNNNNN_*.up.sql / NNNNNN_*.down.sql.
//
//go:embed *.sql
var FS embed.FS
