package questionbank

import (
	"strings"
	"testing"

	"github.com/flowcbt/backend/internal/fileio"
)

func TestRowToQuestion(t *testing.T) {
	t.Parallel()

	// MCQ dengan tanda * pada opsi.
	q, errReason := rowToQuestion(fileio.QuestionRowParsed{
		Row: 2, Type: "mcq", Body: "Ibu kota?", Points: 5,
		Options: "*Jakarta|Bandung", Explanation: "UUD 1945",
	})
	if errReason != "" {
		t.Fatalf("mcq valid ditolak: %s", errReason)
	}
	if len(q.Options) != 2 || !q.Options[0].Correct || q.Options[1].Correct {
		t.Fatalf("opsi/kunci salah: %+v", q.Options)
	}
	if q.Body != "<p>Ibu kota?</p>" {
		t.Fatalf("body tidak dibungkus paragraf: %q", q.Body)
	}
	if q.ExplanationHTML != "<p>UUD 1945</p>" {
		t.Fatalf("penjelasan salah: %q", q.ExplanationHTML)
	}

	// MCQ dengan kunci huruf.
	q, errReason = rowToQuestion(fileio.QuestionRowParsed{
		Row: 3, Type: "mcq", Body: "2+2?", Points: 5,
		Options: "3|4|5", Key: "B",
	})
	if errReason != "" || !q.Options[1].Correct || q.Options[0].Correct {
		t.Fatalf("kunci huruf tidak diproses: %s %v", errReason, q.Options)
	}

	// MCQ tanpa kunci → ditolak.
	if _, errReason = rowToQuestion(fileio.QuestionRowParsed{
		Row: 4, Type: "mcq", Body: "x", Points: 1, Options: "a|b",
	}); errReason == "" {
		t.Fatal("mcq tanpa kunci seharusnya ditolak")
	}

	// Isian singkat multi-kunci.
	q, errReason = rowToQuestion(fileio.QuestionRowParsed{
		Row: 5, Type: "short_answer", Body: "12x8", Points: 4, Key: "96|sembilan puluh enam",
	})
	if errReason != "" {
		t.Fatalf("isian valid ditolak: %s", errReason)
	}
	acc, ok := q.AnswerKey.(map[string]any)["accepted"].([]string)
	if !ok || len(acc) != 2 {
		t.Fatalf("kunci isian salah: %#v", q.AnswerKey)
	}

	// Menjodohkan.
	q, errReason = rowToQuestion(fileio.QuestionRowParsed{
		Row: 6, Type: "matching", Body: "Jodohkan", Points: 4, Key: "Jakarta=Indonesia|Kuala Lumpur=Malaysia",
	})
	if errReason != "" {
		t.Fatalf("matching valid ditolak: %s", errReason)
	}
	if pairs, ok := q.AnswerKey.(map[string]any)["pairs"].([]map[string]any); !ok || len(pairs) != 2 {
		t.Fatalf("pasangan salah: %#v", q.AnswerKey)
	}

	// Uraian: rubrik dari poin.
	q, errReason = rowToQuestion(fileio.QuestionRowParsed{
		Row: 7, Type: "essay", Body: "Jelaskan!", Points: 10,
	})
	if errReason != "" {
		t.Fatalf("uraian valid ditolak: %s", errReason)
	}
	if mp, ok := q.Rubric.(map[string]any)["max_points"].(float64); !ok || mp != 10 {
		t.Fatalf("rubrik salah: %#v", q.Rubric)
	}

	// Tidak valid: poin negatif, tipe asing, baris kosong.
	if _, errReason = rowToQuestion(fileio.QuestionRowParsed{Row: 8, Type: "mcq", Body: "x", Points: -1}); errReason == "" {
		t.Fatal("poin negatif seharusnya ditolak")
	}
	if _, errReason = rowToQuestion(fileio.QuestionRowParsed{Row: 9, Type: "listening", Points: 1}); errReason == "" {
		t.Fatal("tipe asing seharusnya ditolak")
	}
}

func TestParseQuestionFileCSV(t *testing.T) {
	t.Parallel()
	csvData := "Tipe,Pertanyaan,Poin,Opsi,Kunci,Penjelasan\n" +
		"mcq,Ibu kota?,5,*Jakarta|Bandung,,UUD\n" +
		"short_answer,12x8,4,,96|sembilan puluh enam,\n" +
		",,,,,\n" // baris kosong diabaikan
	rows, err := fileio.ParseQuestionFile("soal.csv", strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("parse gagal: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(rows))
	}
	if rows[0].Type != "mcq" || rows[0].Points != 5 || rows[1].Key == "" {
		t.Fatalf("isi kolom salah: %+v", rows)
	}
}
