// Package questionbank: paket soal, versi, dan soal 5 tipe AKM (PRD M4).
// Alur: draft bebas diedit → publish membekukan versi (FR-4.8) → perubahan
// setelah publish = clone ke draft baru (keputusan #4, lampiran 18.6).
package questionbank

import (
	"text/template"
)

// Tipe soal (FR-4.3).
const (
	TypeMCQ         = "mcq"
	TypeMCQComplex  = "mcq_complex"
	TypeMatching    = "matching"
	TypeShortAnswer = "short_answer"
	TypeEssay       = "essay"
)

var allTypes = map[string]bool{
	TypeMCQ: true, TypeMCQComplex: true, TypeMatching: true,
	TypeShortAnswer: true, TypeEssay: true,
}

// QuestionSet: baris tabel question_sets.
type QuestionSet struct {
	ID                 string   `json:"id"`
	SubjectName        string   `json:"subject_name"`
	Title              string   `json:"title"`
	Description        string   `json:"description,omitempty"`
	ExamCode           string   `json:"exam_code"`
	LevelTags          []string `json:"level_tags"`
	Status             string   `json:"status"` // draft | published | archived
	DraftVersionID     string   `json:"draft_version_id,omitempty"`
	PublishedVersionID string   `json:"published_version_id,omitempty"`
	SharedWithTenant   bool     `json:"shared_with_tenant"`
	CreatedBy          string   `json:"created_by"`
	OwnerID            string   `json:"owner_id,omitempty"`
	ArchivedAt         *string  `json:"archived_at,omitempty"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
}

// QuestionSetVersion: baris tabel question_set_versions.
type QuestionSetVersion struct {
	ID            string  `json:"id"`
	QuestionSetID string  `json:"question_set_id"`
	VersionNo     int     `json:"version_no"`
	Status        string  `json:"status"` // draft | published
	QuestionCount int     `json:"question_count"`
	MaxPoints     float64 `json:"max_points"`
	PublishedAt   *string `json:"published_at,omitempty"`
	PublishedBy   *string `json:"published_by,omitempty"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// Question: baris tabel questions (5 tipe; kunci TIDAK dikirim ke klien siswa).
type Question struct {
	ID                  string   `json:"id"`
	TenantID            string   `json:"-"`
	SetVersionID        string   `json:"-"`
	StimulusID          *string  `json:"stimulus_id,omitempty"`
	OrderNo             int      `json:"order_no"`
	Type                string   `json:"type"`
	Body                string   `json:"body"`              // rich text tersanitasi
	Options             []Option `json:"options,omitempty"` // mcq/mcq_complex/matching
	AnswerKey           any      `json:"-"`                 // jsonb; TIDAK pernah ke siswa
	Rubric              any      `json:"rubric,omitempty"`  // uraian
	Points              float64  `json:"points"`
	ScoringMode         string   `json:"scoring_mode"` // all_or_nothing | partial
	PartialWrongPenalty bool     `json:"partial_wrong_penalty"`
	ShuffleOptions      bool     `json:"shuffle_options"`
	ExplanationHTML     string   `json:"explanation_html,omitempty"`
	MediaIDs            []string `json:"media_ids,omitempty"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
}

type Option struct {
	ID      string `json:"id"`
	HTML    string `json:"html"`
	Correct bool   `json:"correct"`
}

// Stimulus: wacana yang dapat dipakai bersama beberapa soal (khas AKM literasi).
type Stimulus struct {
	ID           string   `json:"id"`
	TenantID     string   `json:"-"`
	SetVersionID string   `json:"-"`
	OrderNo      int      `json:"order_no"`
	Title        string   `json:"title,omitempty"`
	ContentHTML  string   `json:"content_html"`
	MediaIDs     []string `json:"media_ids,omitempty"`
}

// KaTeXTemplate: placeholder render rumus matematika (FR-4.4) di editor FE.
// FE memuat KaTeX; di server TIDAK ada render — hanya sanitasi.
var KaTeXTemplate = template.Must(template.New("katex").Parse(`{{.}}`))

// IsSupportedType memvalidasi tipe soal.
func IsSupportedType(t string) bool { return allTypes[t] }
