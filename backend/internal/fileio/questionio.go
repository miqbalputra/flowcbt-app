// Template & parser import/export soal (S8, FR-4.x) — pola sama dengan
// import siswa: preview → commit dengan nomor baris & alasan (AC-10).
//
// Konvensi kolom template soal:
//
//	Tipe       : mcq | mcq_complex | matching | short_answer | essay
//	Pertanyaan : teks soal (plain; server menyimpan sebagai HTML paragraf)
//	Poin       : angka >= 0
//	Opsi       : dipisah "|" ; awalan "*" menandai jawaban benar
//	             (kosong untuk isian/uraian)
//	Kunci      : mcq/mcq_complex → huruf opsi "A"/"B"/… (alternatif tanda *)
//	             short_answer → daftar kunci dipisah "|"
//	             matching → "kiri=kanan|kiri2=kanan2"
//	             essay → kosong (rubrik dari Poin)
//	Penjelasan : teks opsional
package fileio

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

var questionColumns = []string{"Tipe", "Pertanyaan", "Poin", "Opsi", "Kunci", "Penjelasan"}

// QuestionTemplateXLSX: template import soal + contoh 4 baris (per tipe).
func QuestionTemplateXLSX() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	for i, col := range questionColumns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, col)
	}
	f.SetCellStyle(sheet, "A1", "F1", style)
	f.SetColWidth(sheet, "A", "F", 26)

	samples := [][]string{
		{"mcq", "Ibu kota Indonesia adalah ...", "5", "*Jakarta|Bandung|Surabaya", "A", "Pasal 7 UUD 1945."},
		{"mcq_complex", "Pilih semua bilangan prima.", "6", "*3|*5|9|*11", "", "Bilangan prima hanya habis dibagi 1 dan dirinya."},
		{"short_answer", "Hasil 12 x 8 = ...", "4", "", "96|sembilan puluh enam", ""},
		{"essay", "Jelaskan pros fotosintesis!", "10", "", "", "Rubrik: ada klorofil (3), reaksi (4), glukosa+O2 (3)."},
	}
	for r, row := range samples {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue(sheet, cell, v)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// QuestionRowParsed: satu baris mentah dari file import soal.
type QuestionRowParsed struct {
	Row         int     `json:"row"`
	Type        string  `json:"type"`
	Body        string  `json:"body"`
	Points      float64 `json:"points"`
	Options     string  `json:"options"` // teks mentah, dipisah "|"
	Key         string  `json:"key"`     // teks mentah sesuai tipe
	Explanation string  `json:"explanation,omitempty"`
}

// ParseQuestionFile membaca xlsx/csv soal; header memetakan kolom bebas urutan.
func ParseQuestionFile(filename string, r io.Reader) ([]QuestionRowParsed, error) {
	var records [][]string
	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		cr := csv.NewReader(r)
		cr.FieldsPerRecord = -1
		cr.TrimLeadingSpace = true
		var err error
		records, err = cr.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("CSV tidak valid: %w", err)
		}
	} else {
		f, err := excelize.OpenReader(r)
		if err != nil {
			return nil, fmt.Errorf("file Excel tidak valid: %w", err)
		}
		defer f.Close()
		records, err = f.GetRows(f.GetSheetName(0))
		if err != nil {
			return nil, fmt.Errorf("gagal membaca sheet: %w", err)
		}
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("file kosong: butuh header dan minimal satu baris data")
	}

	col := map[string]int{}
	for i, h := range records[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"tipe", "pertanyaan", "poin"}
	for _, c := range required {
		if _, ok := col[c]; !ok {
			return nil, fmt.Errorf("header wajib memuat kolom: %s", strings.Join(questionColumns, ", "))
		}
	}

	get := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	out := make([]QuestionRowParsed, 0, len(records)-1)
	for i, rec := range records[1:] {
		rowNo := i + 2
		if allEmpty(rec) {
			continue
		}
		if rowNo > MaxImportRows+1 {
			break
		}
		points := 0.0
		if p := strings.ReplaceAll(get(rec, "poin"), ",", "."); p != "" {
			if _, err := fmt.Sscanf(p, "%f", &points); err != nil {
				points = -1 // tandai invalid; divalidasi pemanggil
			}
		}
		out = append(out, QuestionRowParsed{
			Row:         rowNo,
			Type:        strings.ToLower(get(rec, "tipe")),
			Body:        get(rec, "pertanyaan"),
			Points:      points,
			Options:     get(rec, "opsi"),
			Key:         get(rec, "kunci"),
			Explanation: get(rec, "penjelasan"),
		})
	}
	return out, nil
}

// SplitPipe memecah sel Opsi/Kunci pada "|" sambil membuang spasi pinggir.
func SplitPipe(v string) []string {
	parts := strings.Split(v, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ParsedOption: satu opsi hasil sel Opsi; awalan "*" berarti jawaban benar.
type ParsedOption struct {
	Text    string
	Correct bool
}

// OptionsFromCell mengubah sel Opsi menjadi daftar opsi.
func OptionsFromCell(v string) []ParsedOption {
	out := []ParsedOption{}
	for _, p := range SplitPipe(v) {
		correct := strings.HasPrefix(p, "*")
		out = append(out, ParsedOption{Text: strings.TrimPrefix(p, "*"), Correct: correct})
	}
	return out
}

// QuestionsToXLSX: export daftar soal ke xlsx dengan kolom template.
func QuestionsToXLSX(rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	for c, h := range questionColumns {
		cell, _ := excelize.CoordinatesToCellName(c+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	f.SetCellStyle(sheet, "A1", "F1", style)
	f.SetColWidth(sheet, "A", "F", 26)
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			f.SetCellValue(sheet, cell, v)
		}
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// QuestionsToCSV: export daftar soal ke CSV.
func QuestionsToCSV(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(questionColumns); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
