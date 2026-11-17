// Command worker menjalankan job queue asynq: import/export, email, auto-submit,
// re-grade, retensi 7 hari, rekonsiliasi pembayaran. Diisi bertahap per sprint.
package main

import (
	"log"
)

func main() {
	log.Println("flowcbt-worker: belum ada handler terdaftar (diisi pada S3+: mailer, auto-submit, retensi)")
	select {}
}
