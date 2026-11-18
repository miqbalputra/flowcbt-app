// Package grading: mesin penilaian murni (tanpa DB) — PRD 18.7.
// Semua tipe soal (FR-8.1), rumus parsial (FR-8.1b), normalisasi isian.
// Bentuk jawaban dari klien (konvensi S12):
//
//	mcq          {"selected":"optId"}
//	mcq_complex  {"selected":["optId",...]}
//	matching     {"pairs":[{"left":"idL","right":"idR"}]}
//	short_answer {"text":"..."}
//	essay        {"text":"..."}  → selalu Menunggu penilaian
package grading

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

const (
	TypeMCQ         = "mcq"
	TypeMCQComplex  = "mcq_complex"
	TypeMatching    = "matching"
	TypeShortAnswer = "short_answer"
	TypeEssay       = "essay"
)

// OptionSpec: satu opsi pilihan + tanda benar (dari options_json).
type OptionSpec struct {
	ID      string
	Correct bool
}

// QuestionSpec: spesifikasi soal lengkap dengan kunci (dari server, bukan klien).
type QuestionSpec struct {
	ID                  string
	Type                string
	Points              float64
	ScoringMode         string // all_or_nothing | partial
	PartialWrongPenalty bool
	Options             []OptionSpec // mcq/mcq_complex
	MatchPairs          [][2]string  // menjodohkan: pasangan benar [leftID,rightID]
	Accepted            []string     // isian singkat: kunci yang diterima
}

// Result: hasil satu soal.
type Result struct {
	Score       float64 // 0..Points, dibulatkan 2 desimal (FR-8.1b)
	IsCorrect   *bool   // nil saat parsial selangkangan / uraian
	NeedsManual bool    // uraian
}

// Score menilai satu jawaban.
func Score(q QuestionSpec, answer any) Result {
	switch q.Type {
	case TypeMCQ:
		return scoreMCQ(q, answer)
	case TypeMCQComplex:
		return scoreMCQComplex(q, answer)
	case TypeMatching:
		return scoreMatching(q, answer)
	case TypeShortAnswer:
		return scoreShort(q, answer)
	case TypeEssay:
		return Result{Score: 0, IsCorrect: nil, NeedsManual: true}
	}
	return Result{}
}

// ---- pilihan ganda ----

func scoreMCQ(q QuestionSpec, answer any) Result {
	selected := selectedOne(answer)
	correct := ""
	for _, o := range q.Options {
		if o.Correct {
			correct = o.ID
			break
		}
	}
	if selected != "" && selected == correct {
		return Result{Score: round2(q.Points), IsCorrect: boolPtr(true)}
	}
	return Result{Score: 0, IsCorrect: boolPtr(false)}
}

// ---- PG kompleks (FR-8.1b) ----

func scoreMCQComplex(q QuestionSpec, answer any) Result {
	sel := selectedMany(answer)
	trueSet := map[string]bool{}
	totalKeys := 0
	for _, o := range q.Options {
		if o.Correct {
			trueSet[o.ID] = true
			totalKeys++
		}
	}
	pickedCorrect, pickedWrong := 0, 0
	seen := map[string]bool{}
	for _, s := range sel {
		if seen[s] {
			continue
		}
		seen[s] = true
		if trueSet[s] {
			pickedCorrect++
		} else {
			pickedWrong++
		}
	}

	if q.ScoringMode == "partial" && totalKeys > 0 {
		raw := float64(pickedCorrect-pickedWrong) / float64(totalKeys)
		if raw < 0 {
			raw = 0 // max(0, ...) — penalti salah tidak membuat minus
		}
		score := round2(q.Points * raw)
		var ic *bool
		if pickedWrong == 0 && pickedCorrect == totalKeys {
			ic = boolPtr(true)
		} else if score == 0 {
			ic = boolPtr(false)
		} // selain itu nil (parsial)
		return Result{Score: score, IsCorrect: ic}
	}
	// all-or-nothing: pilih persis semua kunci dan tak ada salah.
	ok := pickedWrong == 0 && pickedCorrect == totalKeys
	if ok {
		return Result{Score: round2(q.Points), IsCorrect: boolPtr(true)}
	}
	return Result{Score: 0, IsCorrect: boolPtr(false)}
}

// ---- menjodohkan (FR-8.1b: parsial per pasangan) ----

func scoreMatching(q QuestionSpec, answer any) Result {
	pairs := answerPairs(answer)
	type pk struct{ l, r string }
	keySet := map[pk]bool{}
	for _, p := range q.MatchPairs {
		keySet[pk{p[0], p[1]}] = true
	}
	correct := 0
	for _, p := range pairs {
		if keySet[pk{p.Left, p.Right}] {
			correct++
		}
	}
	total := len(q.MatchPairs)
	if total == 0 {
		return Result{Score: 0, IsCorrect: boolPtr(false)}
	}
	if q.ScoringMode == "partial" {
		score := round2(q.Points * float64(correct) / float64(total))
		var ic *bool
		switch {
		case correct == total:
			ic = boolPtr(true)
		case correct == 0:
			ic = boolPtr(false)
		}
		return Result{Score: score, IsCorrect: ic}
	}
	if correct == total && len(pairs) == total {
		return Result{Score: round2(q.Points), IsCorrect: boolPtr(true)}
	}
	return Result{Score: 0, IsCorrect: boolPtr(false)}
}

// ---- isian singkat (FR-8.1: normalisasi; angka toleran desimal koma/titik) ----

func scoreShort(q QuestionSpec, answer any) Result {
	text := answerText(answer)
	if text == "" {
		return Result{Score: 0, IsCorrect: boolPtr(false)}
	}
	norm := Normalize(text)
	for _, key := range q.Accepted {
		if Normalize(key) == norm {
			return Result{Score: round2(q.Points), IsCorrect: boolPtr(true)}
		}
		// Perbandingan numerik: 12,5 == 12.5 == " 12.50 ".
		if a, err1 := numericOf(norm); err1 == nil {
			if b, err2 := numericOf(Normalize(key)); err2 == nil && a == b {
				return Result{Score: round2(q.Points), IsCorrect: boolPtr(true)}
			}
		}
	}
	return Result{Score: 0, IsCorrect: boolPtr(false)}
}

// Normalize: trim, lowercase, spasi ganda tunggal, koma → titik (angka).
func Normalize(s string) string {
	s = strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
	if numRe.MatchString(s) {
		s = strings.ReplaceAll(s, ",", ".")
	}
	return s
}

var numRe = regexp.MustCompile(`^-?\d+([.,]\d+)?$`)

func numericOf(s string) (float64, error) {
	if !numRe.MatchString(s) {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseFloat(s, 64)
}

// ---- pembantu bentuk jawaban ----

func selectedOne(answer any) string {
	if m, ok := answer.(map[string]any); ok {
		if s, ok := m["selected"].(string); ok {
			return s
		}
	}
	return ""
}

func selectedMany(answer any) []string {
	m, ok := answer.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := m["selected"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

type pairAns struct{ Left, Right string }

func answerPairs(answer any) []pairAns {
	m, ok := answer.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := m["pairs"].([]any)
	if !ok {
		return nil
	}
	out := make([]pairAns, 0, len(raw))
	for _, v := range raw {
		pm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		l, _ := pm["left"].(string)
		r, _ := pm["right"].(string)
		out = append(out, pairAns{l, r})
	}
	return out
}

func answerText(answer any) string {
	if m, ok := answer.(map[string]any); ok {
		if s, ok := m["text"].(string); ok {
			return s
		}
	}
	if s, ok := answer.(string); ok {
		return s
	}
	return ""
}

func boolPtr(b bool) *bool { return &b }

func round2(v float64) float64 { return math.Round(v*100) / 100 }
