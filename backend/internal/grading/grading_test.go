// Uji tabel-driven mesin penilaian (FR-8.1, FR-8.1b) — kriteria S13.
package grading

import "testing"

type tc struct {
	name   string
	q      QuestionSpec
	answer any
	want   Result
}

func mcq() QuestionSpec {
	return QuestionSpec{
		ID: "q1", Type: TypeMCQ, Points: 5,
		Options: []OptionSpec{{ID: "a", Correct: false}, {ID: "b", Correct: true}},
	}
}

func mcqComplex() QuestionSpec {
	return QuestionSpec{
		ID: "q2", Type: TypeMCQComplex, Points: 6,
		ScoringMode: "all_or_nothing", PartialWrongPenalty: true,
		Options: []OptionSpec{
			{ID: "a", Correct: true}, {ID: "b", Correct: true}, {ID: "c", Correct: false},
		},
	}
}

func matching() QuestionSpec {
	return QuestionSpec{
		ID: "q3", Type: TypeMatching, Points: 4,
		ScoringMode: "partial",
		MatchPairs:  [][2]string{{"L1", "R1"}, {"L2", "R2"}},
	}
}

func short() QuestionSpec {
	return QuestionSpec{ID: "q4", Type: TypeShortAnswer, Points: 3, Accepted: []string{"96", "sembilan puluh enam"}}
}

func run(t *testing.T, cases []tc) {
	t.Helper()
	for _, c := range cases {
		got := Score(c.q, c.answer)
		if got.NeedsManual != c.want.NeedsManual || got.Score != c.want.Score ||
			!boolEq(got.IsCorrect, c.want.IsCorrect) {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

func boolEq(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func TestMCQ(t *testing.T) {
	t.Parallel()
	run(t, []tc{
		{"benar", mcq(), map[string]any{"selected": "b"}, Result{Score: 5, IsCorrect: boolPtr(true)}},
		{"salah", mcq(), map[string]any{"selected": "a"}, Result{Score: 0, IsCorrect: boolPtr(false)}},
		{"kosong", mcq(), nil, Result{Score: 0, IsCorrect: boolPtr(false)}},
	})
}

func TestMCQComplex(t *testing.T) {
	t.Parallel()
	run(t, []tc{
		// all-or-nothing
		{"AON tepat", mcqComplex(), sel("a", "b"), Result{Score: 6, IsCorrect: boolPtr(true)}},
		{"AON kurang satu", mcqComplex(), sel("a"), Result{Score: 0, IsCorrect: boolPtr(false)}},
		{"AON ada salah", mcqComplex(), sel("a", "b", "c"), Result{Score: 0, IsCorrect: boolPtr(false)}},
	})
	// parsial: (benar − salah) / jumlah kunci (kunci = a,b)
	partial := mcqComplex()
	partial.ScoringMode = "partial"
	run(t, []tc{
		{"parsial dua kunci", partial, sel("a", "b"), Result{Score: 6, IsCorrect: boolPtr(true)}},
		{"parsial satu kunci", partial, sel("a"), Result{Score: 3, IsCorrect: nil}},
		{"parsial benar+salah", partial, sel("a", "b", "c"), Result{Score: 3, IsCorrect: nil}}, // (2-1)/2
		{"parsial minus jadi 0", partial, sel("c", "b"), Result{Score: 0, IsCorrect: boolPtr(false)}},
	})
}

func TestMatching(t *testing.T) {
	t.Parallel()
	run(t, []tc{
		{"parsial penuh", matching(), pairsAns([2]string{"L1", "R1"}, [2]string{"L2", "R2"}),
			Result{Score: 4, IsCorrect: boolPtr(true)}},
		{"parsial setengah", matching(), pairsAns([2]string{"L1", "R1"}),
			Result{Score: 2, IsCorrect: nil}},
		{"parsial salah semua", matching(), pairsAns([2]string{"L1", "R2"}, [2]string{"L2", "R1"}),
			Result{Score: 0, IsCorrect: boolPtr(false)}},
	})
	aon := matching()
	aon.ScoringMode = "all_or_nothing"
	run(t, []tc{
		{"AON tepat", aon, pairsAns([2]string{"L1", "R1"}, [2]string{"L2", "R2"}), Result{Score: 4, IsCorrect: boolPtr(true)}},
		{"AON kurang", aon, pairsAns([2]string{"L1", "R1"}), Result{Score: 0, IsCorrect: boolPtr(false)}},
	})
}

func TestShortAnswer(t *testing.T) {
	t.Parallel()
	run(t, []tc{
		{"tepat persis", short(), map[string]any{"text": "96"}, Result{Score: 3, IsCorrect: boolPtr(true)}},
		{"tepat teks kedua", short(), map[string]any{"text": "Sembilan  Puluh Enam"},
			Result{Score: 3, IsCorrect: boolPtr(true)}},
		{"koma desimal", QuestionSpec{ID: "q5", Type: TypeShortAnswer, Points: 2, Accepted: []string{"12.5"}},
			map[string]any{"text": "12,5"}, Result{Score: 2, IsCorrect: boolPtr(true)}},
		{"spasi & kapital", short(), map[string]any{"text": "  96  "}, Result{Score: 3, IsCorrect: boolPtr(true)}},
		{"salah", short(), map[string]any{"text": "69"}, Result{Score: 0, IsCorrect: boolPtr(false)}},
		{"kosong", short(), map[string]any{"text": ""}, Result{Score: 0, IsCorrect: boolPtr(false)}},
	})
}

func TestEssay(t *testing.T) {
	t.Parallel()
	got := Score(QuestionSpec{ID: "q6", Type: TypeEssay, Points: 10}, map[string]any{"text": "apa pun"})
	if !got.NeedsManual || got.Score != 0 || got.IsCorrect != nil {
		t.Fatalf("uraian harus menunggu penilaian: %+v", got)
	}
}

func sel(ids ...string) any {
	out := make([]any, len(ids))
	for i, s := range ids {
		out[i] = s
	}
	return map[string]any{"selected": out}
}

func pairsAns(ps ...[2]string) any {
	out := make([]any, len(ps))
	for i, p := range ps {
		out[i] = map[string]any{"left": p[0], "right": p[1]}
	}
	return map[string]any{"pairs": out}
}
