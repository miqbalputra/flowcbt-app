// Import/export soal via template xlsx (S8, FR-4.9): preview (validasi AC-10
// dengan nomor baris & alasan) → commit; export xlsx/csv paket aktif.
// Kuota penyimpanan media TIDAK terkait di sini (stub kuota, penuh di S27).
package questionbank

import (
	"context"
	"encoding/json"
	"errors"
	"html"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/flowcbt/backend/internal/audit"
	"github.com/flowcbt/backend/internal/auth"
	"github.com/flowcbt/backend/internal/fileio"
	"github.com/flowcbt/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type QuestionImportError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type QuestionImportPreview struct {
	Total  int                   `json:"total"`
	Valid  []Question            `json:"valid"`
	Errors []QuestionImportError `json:"errors"`
}

func (s *Store) parseQuestionUpload(fh *multipart.FileHeader) ([]fileio.QuestionRowParsed, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, httpx.ErrBadRequest("unggahan tidak dapat dibuka", nil)
	}
	defer f.Close()
	return fileio.ParseQuestionFile(fh.Filename, f)
}

// PreviewQuestionImport memvalidasi file tanpa menyimpan apa pun (guru melihat
// dulu di layar). Validasi tipe, poin, kelengkapan opsi & kunci per tipe.
func (s *Store) PreviewQuestionImport(ctx context.Context, tenantID, setID string,
	fh *multipart.FileHeader) (*QuestionImportPreview, error) {

	// Set harus ada & masih draft — import hanya untuk draft (FR-4.8).
	if _, err := s.GetSet(ctx, tenantID, setID); err != nil {
		return nil, notFoundOrErr(err)
	}
	rows, err := s.parseQuestionUpload(fh)
	if err != nil {
		return nil, httpx.ErrBadRequest(err.Error(), nil)
	}
	preview := &QuestionImportPreview{Valid: []Question{}, Errors: []QuestionImportError{}}
	for _, r := range rows {
		q, reason := rowToQuestion(r)
		if reason != "" {
			preview.Errors = append(preview.Errors, QuestionImportError{Row: r.Row, Reason: reason})
			continue
		}
		preview.Valid = append(preview.Valid, *q)
	}
	preview.Total = len(rows)
	return preview, nil
}

// CommitQuestionImport: satu transaksi untuk seluruh baris valid.
func (s *Store) CommitQuestionImport(ctx context.Context, tenantID, setID string,
	fh *multipart.FileHeader) (int, []QuestionImportError, error) {

	if set, err := s.GetSet(ctx, tenantID, setID); err != nil {
		return 0, nil, notFoundOrErr(err)
	} else if set.Status != "draft" {
		return 0, nil, httpx.ErrBadRequest("import hanya pada paket draft", nil)
	}
	rows, err := s.parseQuestionUpload(fh)
	if err != nil {
		return 0, nil, httpx.ErrBadRequest(err.Error(), nil)
	}
	inserted := 0
	errs := []QuestionImportError{}
	err = s.db.WithTenantTx(ctx, tenantID, func(tx pgx.Tx) error {
		draftID, err := ensureDraftVersionTx(ctx, tx, setID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			q, reason := rowToQuestion(r)
			if reason != "" {
				errs = append(errs, QuestionImportError{Row: r.Row, Reason: reason})
				continue
			}
			if _, err := insertQuestionTx(ctx, tx, draftID, *q); err != nil {
				errs = append(errs, QuestionImportError{Row: r.Row, Reason: "gagal menyimpan: " + err.Error()})
				continue
			}
			inserted++
		}
		return nil
	})
	return inserted, errs, err
}

// rowToQuestion mengubah satu baris mentah menjadi Question tersanitasi atau
// alasan penolakan (AC-10).
func rowToQuestion(r fileio.QuestionRowParsed) (*Question, string) {
	if !IsSupportedType(r.Type) {
		return nil, "tipe soal tidak dikenal: " + r.Type
	}
	if r.Body == "" {
		return nil, "pertanyaan kosong"
	}
	if r.Points < 0 {
		return nil, "poin tidak valid: harus angka >= 0"
	}
	body := "<p>" + html.EscapeString(r.Body) + "</p>"
	expl := ""
	if r.Explanation != "" {
		expl = "<p>" + html.EscapeString(r.Explanation) + "</p>"
	}
	q := &Question{
		Type: r.Type, Body: body, Points: r.Points,
		ScoringMode: "all_or_nothing", ExplanationHTML: expl,
		Options: []Option{}, AnswerKey: nil, Rubric: nil,
	}

	switch r.Type {
	case TypeMCQ, TypeMCQComplex:
		parsed := fileio.OptionsFromCell(r.Options)
		if len(parsed) < 2 {
			return nil, "soal pilihan butuh minimal 2 opsi dipisah \"|\""
		}
		opts := make([]Option, 0, len(parsed))
		for i, o := range parsed {
			opts = append(opts, Option{
				ID:      "imp-" + strconv.Itoa(r.Row) + "-" + strconv.Itoa(i+1),
				HTML:    "<p>" + html.EscapeString(o.Text) + "</p>",
				Correct: o.Correct,
			})
		}
		// Kunci berupa huruf opsi ("A") bila tanda "*" tak dipakai.
		if key := strings.ToUpper(strings.TrimSpace(r.Key)); key != "" {
			if idx := int(key[0] - 'A'); idx >= 0 && idx < len(opts) {
				for i := range opts {
					opts[i].Correct = i == idx
				}
			} else {
				return nil, "kunci \"" + key + "\" bukan huruf opsi yang sah"
			}
		}
		hasCorrect := false
		for _, o := range opts {
			if o.Correct {
				hasCorrect = true
				break
			}
		}
		if !hasCorrect {
			return nil, "kunci jawaban belum ditandai (awalan * pada Opsi atau huruf di Kunci)"
		}
		q.Options = opts
		q.AnswerKey = answerKeyLetters(opts)

	case TypeMatching:
		// Bentuk data: options_json = {left:[{id,text}], right:[{id,text}]},
		// kunci = {pairs:[{left:"idL", right:"idR"}]} (S12 — siswa menjodohkan
		// id pilihan; teks pilihan bukan kunci).
		rawPairs := fileio.SplitPipe(r.Key)
		if len(rawPairs) < 2 {
			return nil, "menjodohkan butuh minimal 2 pasangan di kolom Kunci (kiri=kanan|...)"
		}
		lefts, rights, pairs := []map[string]any{}, []map[string]any{}, []map[string]any{}
		seenLeft, seenRight := map[string]string{}, map[string]string{}
		for i, p := range rawPairs {
			sides := strings.SplitN(p, "=", 2)
			if len(sides) != 2 || strings.TrimSpace(sides[0]) == "" || strings.TrimSpace(sides[1]) == "" {
				return nil, "pasangan menjodohkan harus format kiri=kanan dipisah \"|\""
			}
			lid, rid := "L"+strconv.Itoa(i+1), "R"+strconv.Itoa(i+1)
			if id, ok := seenLeft[sides[0]]; ok {
				lid = id
			} else {
				seenLeft[sides[0]] = lid
				lefts = append(lefts, map[string]any{"id": lid, "text": sides[0]})
			}
			if id, ok := seenRight[sides[1]]; ok {
				rid = id
			} else {
				seenRight[sides[1]] = rid
				rights = append(rights, map[string]any{"id": rid, "text": sides[1]})
			}
			pairs = append(pairs, map[string]any{"left": lid, "right": rid})
		}
		q.MatchingOptions = map[string]any{"left": lefts, "right": rights}
		q.AnswerKey = map[string]any{"pairs": pairs}

	case TypeShortAnswer:
		keys := fileio.SplitPipe(r.Key)
		if len(keys) == 0 {
			return nil, "isian singkat butuh minimal satu kunci di kolom Kunci (dipisah |)"
		}
		q.AnswerKey = map[string]any{"accepted": keys}

	case TypeEssay:
		q.Rubric = map[string]any{"max_points": r.Points}
	}
	return q, ""
}

// answerKeyLetters: indeks opsi benar (untuk konsistensi penilaian MCQ).
func answerKeyLetters(opts []Option) map[string]any {
	correct := []string{}
	for i, o := range opts {
		if o.Correct {
			correct = append(correct, string(rune('A'+i)))
		}
	}
	return map[string]any{"letters": correct}
}

// ExportQuestions: baris export dari versi draft/terbit aktif paket.
func (s *Store) ExportQuestions(ctx context.Context, tenantID, setID string) ([][]string, error) {
	qs, err := s.QuestionsOfSet(ctx, tenantID, setID)
	if err != nil {
		return nil, notFoundOrErr(err)
	}
	rows := make([][]string, 0, len(qs))
	for _, q := range qs {
		rows = append(rows, []string{
			q.Type, StripHTML(q.Body), strconv.FormatFloat(q.Points, 'f', -1, 64),
			optionsCell(q), keyCell(q), StripHTML(q.ExplanationHTML),
		})
	}
	return rows, nil
}

func optionsCell(q Question) string {
	parts := make([]string, 0, len(q.Options))
	for _, o := range q.Options {
		text := StripHTML(o.HTML)
		if o.Correct {
			text = "*" + text
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "|")
}

func keyCell(q Question) string {
	switch q.Type {
	case TypeMCQ, TypeMCQComplex:
		if letters, ok := (q.AnswerKey).(map[string]any)["letters"].([]any); ok && len(letters) > 0 {
			out := make([]string, 0, len(letters))
			for _, l := range letters {
				out = append(out, toStr(l))
			}
			return strings.Join(out, "")
		}
		return optionsCellCorrectLetters(q)
	case TypeMatching:
		if pairs, ok := (q.AnswerKey).(map[string]any)["pairs"].([]any); ok {
			out := []string{}
			for _, p := range pairs {
				if m, ok := p.(map[string]any); ok {
					out = append(out, toStr(m["left"])+"="+toStr(m["right"]))
				}
			}
			return strings.Join(out, "|")
		}
	case TypeShortAnswer:
		if acc, ok := (q.AnswerKey).(map[string]any)["accepted"].([]any); ok {
			out := []string{}
			for _, a := range acc {
				out = append(out, toStr(a))
			}
			return strings.Join(out, "|")
		}
	}
	return ""
}

// optionsCellCorrectLetters: kunci MCQ dari tanda benar opsi (data lama tanpa letters).
func optionsCellCorrectLetters(q Question) string {
	letters := []string{}
	for i, o := range q.Options {
		if o.Correct {
			letters = append(letters, string(rune('A'+i)))
		}
	}
	return strings.Join(letters, "")
}

func toStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

var tagRe = regexp.MustCompile(`<[^>]*>`)

// StripHTML: teks polos dari HTML (para export template).
func StripHTML(src string) string {
	un := html.UnescapeString(tagRe.ReplaceAllString(src, " "))
	return strings.Join(strings.Fields(un), " ")
}

// ---- handler endpoints S8 (dipasang lewat Mount di handler.go) ----

func (h *Handler) questionTemplate(w http.ResponseWriter, r *http.Request) {
	b, err := fileio.QuestionTemplateXLSX()
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="template-soal-flowcbt.xlsx"`)
	_, _ = w.Write(b)
}

func (h *Handler) questionImportPreview(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("unggahan tidak valid", nil))
		return
	}
	fh := r.MultipartForm.File["file"]
	if len(fh) == 0 {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("field file wajib diisi", nil))
		return
	}
	p, err := h.store.PreviewQuestionImport(r.Context(), c.TenantID, chi.URLParam(r, "id"), fh[0])
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, p)
}

func (h *Handler) questionImportCommit(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("unggahan tidak valid", nil))
		return
	}
	fh := r.MultipartForm.File["file"]
	if len(fh) == 0 {
		httpx.ErrorWriter(w, r, httpx.ErrBadRequest("field file wajib diisi", nil))
		return
	}
	setID := chi.URLParam(r, "id")
	n, errs, err := h.store.CommitQuestionImport(r.Context(), c.TenantID, setID, fh[0])
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "question.import", Entity: "question_set", EntityID: setID,
		Meta: map[string]any{"inserted": n, "errors": len(errs)}, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"inserted": n, "errors": errs})
}

func (h *Handler) questionExport(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	rows, err := h.store.ExportQuestions(r.Context(), c.TenantID, chi.URLParam(r, "id"))
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	var b []byte
	if r.URL.Query().Get("format") == "csv" {
		b, err = fileio.QuestionsToCSV(rows)
	} else {
		b, err = fileio.QuestionsToXLSX(rows)
	}
	if err != nil {
		httpx.ErrorWriter(w, r, err)
		return
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	}
	w.Header().Set("Content-Disposition", `attachment; filename="soal-export.xlsx"`)
	_, _ = w.Write(b)
}

func (h *Handler) unpublish(w http.ResponseWriter, r *http.Request) {
	c := auth.FromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.store.UnpublishSet(r.Context(), c.TenantID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			httpx.ErrorWriter(w, r, httpx.AppErr(409, "UNPUBLISH_BLOCKED",
				"tidak dapat menarik publikasi: ujian masih memakai versi ini atau paket tidak sedang terbit", nil))
			return
		}
		httpx.ErrorWriter(w, r, err)
		return
	}
	h.audit.Log(r.Context(), audit.Entry{
		TenantID: c.TenantID, ActorType: "user", ActorID: c.UserID,
		Action: "question_set.unpublish", Entity: "question_set", EntityID: id, IP: r.RemoteAddr,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "publikasi ditarik, paket kembali ke draft"})
}
