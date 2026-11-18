// Import siswa: preview → commit (PRD FR-3.3; AC-10: 495 valid + 5 error
// dengan nomor baris & alasan). Kuota tier dicek per FR-3.3b.
package student

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flowcbt/backend/internal/audit"
	"github.com/flowcbt/backend/internal/auth"
	"github.com/flowcbt/backend/internal/fileio"
	"github.com/flowcbt/backend/internal/httpx"

	"github.com/jackc/pgx/v5"
)

type ImportError struct {
	Row    int    `json:"row"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason"`
}

// ImportPreview: hasil validasi siap ditampilkan pada layar konfirmasi guru.
type ImportPreview struct {
	Total     int           `json:"total"`
	Valid     []ValidRow    `json:"valid"`
	Errors    []ImportError `json:"errors"`
	Limit     int           `json:"limit"`
	Remaining int           `json:"remaining"`
}

type ValidRow struct {
	Row    int    `json:"row"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
	Class  string `json:"class"`
	NIS    string `json:"nis,omitempty"`
}

func (s *Store) parseUpload(fh *multipart.FileHeader) ([]fileio.StudentRowParsed, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, httpx.ErrBadRequest("unggahan tidak dapat dibuka", nil)
	}
	defer f.Close()
	return fileio.ParseStudentFile(fh.Filename, f)
}

// EffectiveLimits batas tier efektif & jumlah siswa aktif (transaksi tenant).
func (s *Store) EffectiveLimits(ctx context.Context, tenantID string) (limit, active int, err error) {
	err = s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		limit, active, err = limits(ctx, tx)
		return err
	})
	return limit, active, err
}

// PreviewImport memvalidasi file tanpa menyimpan apa pun.
func (s *Store) PreviewImport(ctx context.Context, tenantID string, fh *multipart.FileHeader) (*ImportPreview, error) {
	rows, err := s.parseUpload(fh)
	if err != nil {
		return nil, httpx.ErrBadRequest(err.Error(), nil)
	}
	limit, active, err := s.EffectiveLimits(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	remaining := limit - active

	preview := &ImportPreview{
		Total:     len(rows),
		Valid:     []ValidRow{},
		Errors:    []ImportError{},
		Limit:     limit,
		Remaining: remaining,
	}
	seen := map[string]bool{} // duplikat dalam file: NIS dan nama+kelas
	for _, r := range rows {
		if err := validate(r.Name, r.Gender); err != nil {
			preview.Errors = append(preview.Errors, ImportError{Row: r.Row, Name: r.Name, Reason: err.Error()})
			continue
		}
		if strings.TrimSpace(r.Class) == "" {
			preview.Errors = append(preview.Errors, ImportError{Row: r.Row, Name: r.Name, Reason: "kelas wajib diisi"})
			continue
		}
		if r.NIS != "" {
			if seen["n:"+r.NIS] {
				preview.Errors = append(preview.Errors, ImportError{Row: r.Row, Name: r.Name, Reason: "NIS duplikat dalam file"})
				continue
			}
			seen["n:"+r.NIS] = true
		}
		nk := "nk:" + normalizeName(r.Name) + "|" + normalizeName(r.Class)
		if seen[nk] {
			preview.Errors = append(preview.Errors, ImportError{Row: r.Row, Name: r.Name, Reason: "duplikat nama+kelas dalam file"})
			continue
		}
		seen[nk] = true

		preview.Valid = append(preview.Valid, ValidRow{
			Row: r.Row, Name: r.Name, Gender: r.Gender, Class: r.Class, NIS: r.NIS,
		})
	}
	// Melebihi sisa kuota: baris kelebihan ditandai, sisanya tetap valid (FR-3.3b).
	if over := len(preview.Valid) - remaining; over > 0 {
		for _, v := range preview.Valid[len(preview.Valid)-over:] {
			preview.Errors = append(preview.Errors, ImportError{Row: v.Row, Name: v.Name,
				Reason: fmt.Sprintf("melebihi batas siswa: sisa kuota %d dari %d", remaining, limit)})
		}
		preview.Valid = preview.Valid[:len(preview.Valid)-over]
	}
	return preview, nil
}

type commitResult struct {
	Imported       int           `json:"imported"`
	Credentials    []Credentials `json:"credentials"`
	ClassesCreated []string      `json:"classes_created,omitempty"`
}

// CommitImport menyimpan seluruh baris valid dalam SATU transaksi tenant.
func (s *Store) CommitImport(ctx context.Context, tenantID string, fh *multipart.FileHeader, autoCreate bool) (*commitResult, error) {
	preview, err := s.PreviewImport(ctx, tenantID, fh)
	if err != nil {
		return nil, err
	}
	if len(preview.Errors) > 0 {
		b, _ := json.Marshal(preview.Errors)
		return nil, httpx.AppErr(422, "IMPORT_VALIDATION_FAILED",
			"file masih berisi baris error — perbaiki lalu unggah ulang", json.RawMessage(b))
	}
	if len(preview.Valid) == 0 {
		return nil, httpx.ErrBadRequest("tidak ada baris valid untuk disimpan", nil)
	}

	var result *commitResult
	err = s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		res := &commitResult{Credentials: []Credentials{}}

		// Resolusi kelas: cari (tanpa peduli tahun ajaran) → buat bila diizinkan.
		classIDs := map[string]string{}
		for _, v := range preview.Valid {
			if _, ok := classIDs[v.Class]; ok {
				continue
			}
			var id string
			err := tx.QueryRow(ctx,
				`SELECT id::text FROM classes WHERE name = $1 AND archived_at IS NULL LIMIT 1`,
				v.Class).Scan(&id)
			if errors.Is(err, pgx.ErrNoRows) {
				if !autoCreate {
					return httpx.ErrBadRequest(
						`kelas "`+v.Class+`" belum ada. Aktifkan buat-kelas-otomatis untuk membuatnya`, nil)
				}
				if err := tx.QueryRow(ctx,
					`INSERT INTO classes (name, academic_year) VALUES ($1, $2) RETURNING id::text`,
					v.Class, currentAY()).Scan(&id); err != nil {
					return err
				}
				res.ClassesCreated = append(res.ClassesCreated, v.Class)
			} else if err != nil {
				return err
			}
			classIDs[v.Class] = id
		}

		for _, v := range preview.Valid {
			code, err := uniqueCode(ctx, tx)
			if err != nil {
				return err
			}
			username, err := uniqueUsername(ctx, tx, v.Name)
			if err != nil {
				return err
			}
			password, err := randomToken(8)
			if err != nil {
				return err
			}
			hash, err := auth.HashPassword(password)
			if err != nil {
				return err
			}
			var nis any
			if v.NIS != "" {
				nis = v.NIS
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO students (student_code, nis, name, name_normalized, gender, class_id, username, password_hash)
				VALUES ($1, $2, $3, $4, $5, $6::uuid, $7, $8)`,
				code, nis, v.Name, normalizeName(v.Name), v.Gender, classIDs[v.Class], username, hash); err != nil {
				return err
			}
			res.Imported++
			res.Credentials = append(res.Credentials, Credentials{
				Username: username, Password: password, StudentCode: code,
			})
		}
		result = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// currentAY: tahun ajaran berjalan (Juli memulai tahun ajaran baru).
func currentAY() string {
	now := time.Now()
	y := now.Year()
	if now.Month() >= time.July {
		return strconv.Itoa(y) + "/" + strconv.Itoa(y+1)
	}
	return strconv.Itoa(y-1) + "/" + strconv.Itoa(y)
}

// ---- Handler HTTP (dipasang di Mount) ----

func (h *Handler) importTemplate(w http.ResponseWriter, r *http.Request) {
	b, err := fileio.StudentTemplateXLSX()
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="template-import-siswa.xlsx"`)
	_, _ = w.Write(b)
}

func (h *Handler) importPreview(w http.ResponseWriter, r *http.Request) {
	h.importHandle(w, r, false)
}

func (h *Handler) importCommit(w http.ResponseWriter, r *http.Request) {
	h.importHandle(w, r, true)
}

// importHandle mengurus multipart sekali untuk preview maupun commit.
func (h *Handler) importHandle(w http.ResponseWriter, r *http.Request, commit bool) {
	if err := r.ParseMultipartForm(10 << 20); err != nil { // maks 10 MB
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("unggahan tidak valid (maks 10 MB)", nil))
		return
	}
	if len(r.MultipartForm.File["file"]) != 1 {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("field file wajib diisi", nil))
		return
	}
	fh := r.MultipartForm.File["file"][0]
	c := auth.FromContext(r.Context())

	if !commit {
		preview, err := h.store.PreviewImport(r.Context(), c.TenantID, fh)
		if err != nil {
			httpx.ErrorWriter(w, r, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, preview)
		return
	}

	autoCreate := r.FormValue("auto_create_class") == "1" || r.FormValue("auto_create_class") == "true"
	res, err := h.store.CommitImport(r.Context(), c.TenantID, fh, autoCreate)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "student.import", Entity: "tenant", EntityID: c.TenantID,
		Meta: map[string]any{"imported": res.Imported, "classes_created": res.ClassesCreated},
		IP:   r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusCreated, res)
}

// ExportList: kumpulan siswa untuk export (xlsx/csv).
func (s *Store) ExportList(ctx context.Context, tenantID, classID string, limit int) ([]Student, error) {
	if limit <= 0 || limit > 5000 {
		limit = 5000
	}
	return s.List(ctx, tenantID, classID, "", limit, 0)
}

func studentExportRows(list []Student) [][]string {
	rows := make([][]string, len(list))
	for i, s := range list {
		className := ""
		if s.ClassName != nil {
			className = *s.ClassName
		}
		nis := ""
		if s.NIS != nil {
			nis = *s.NIS
		}
		username := ""
		if s.Username != nil {
			username = *s.Username
		}
		rows[i] = []string{s.StudentCode, s.Name, s.Gender, className, nis, username, s.Status}
	}
	return rows
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "xlsx"
	}
	list, err := h.store.ExportList(r.Context(), c.TenantID, r.URL.Query().Get("class_id"), 5000)
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="siswa.csv"`)
		b, err := fileio.StudentsToCSV(studentExportRows(list))
		if err != nil {
			httpx.ErrorWriter(w, r, err)
			return
		}
		_, _ = w.Write(b)
	default:
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="siswa.xlsx"`)
		b, err := fileio.StudentsToXLSX(studentExportRows(list))
		if err != nil {
			httpx.ErrorWriter(w, r, err)
			return
		}
		_, _ = w.Write(b)
	}
}
