// Package fileio: baca/tulis file .xlsx/.csv dan kartu login PDF (excelize, fpdf).
// Import siswa: parse → validasi baris per baris (AC-10) → preview → commit.
package fileio

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
)

// StudentRowParsed: satu baris mentah dari file import.
type StudentRowParsed struct {
	Row    int    `json:"row"` // nomor baris di file (1-based, termasuk header)
	Name   string `json:"name"`
	Gender string `json:"gender"` // L | P | "" (belum valid)
	Class  string `json:"class"`
	NIS    string `json:"nis,omitempty"`
}

// Template kolom PRD 18.1.
var studentColumns = []string{"Nama", "Jenis Kelamin", "Kelas", "NIS (opsional)"}

// StudentTemplateXLSX membuat template import siswa dengan header + contoh 2 baris.
func StudentTemplateXLSX() ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	for i, col := range studentColumns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, col)
	}
	f.SetCellStyle(sheet, "A1", "D1", style)
	f.SetColWidth(sheet, "A", "D", 22)

	samples := [][]string{{"Ahmad Fauzi", "L", "X IPA 1", "12345"}, {"Siti Aisyah", "P", "X IPA 1", "12346"}}
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

// MaxImportRows batas baris per import (AC-10 skenario 500; beri ruang).
const MaxImportRows = 2000

// ParseStudentFile membaca xlsx/csv dan mengembalikan baris terstruktur.
// Header WAJIB memuat kolom Nama, Jenis Kelamin, Kelas; NIS opsional.
func ParseStudentFile(filename string, r io.Reader) ([]StudentRowParsed, error) {
	var records [][]string
	if strings.EqualFold(strings.TrimPrefix(filename, "/"), "") || strings.HasSuffix(strings.ToLower(filename), ".csv") {
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

	// Petakan kolom berdasarkan header (abaikan selisih spasi/kapital).
	nameCol, genderCol, classCol, nisCol := -1, -1, -1, -1
	for i, h := range records[0] {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "nama":
			nameCol = i
		case "jenis kelamin", "jk", "gender":
			genderCol = i
		case "kelas":
			classCol = i
		case "nis (opsional)", "nis", "nisn":
			nisCol = i
		}
	}
	if nameCol < 0 || genderCol < 0 || classCol < 0 {
		return nil, fmt.Errorf("header wajib memuat kolom: Nama, Jenis Kelamin, Kelas")
	}

	out := make([]StudentRowParsed, 0, len(records)-1)
	for i, rec := range records[1:] {
		rowNo := i + 2 // 1-based; header di baris 1
		if allEmpty(rec) {
			continue
		}
		if rowNo > MaxImportRows+1 {
			break
		}
		get := func(i int) string {
			if i < 0 || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		out = append(out, StudentRowParsed{
			Row:    rowNo,
			Name:   get(nameCol),
			Gender: NormalizeGender(get(genderCol)),
			Class:  get(classCol),
			NIS:    get(nisCol),
		})
	}
	return out, nil
}

func allEmpty(rec []string) bool {
	for _, v := range rec {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// NormalizeGender menerima variasi umum; mengembalikan L/P/sesuai isi asli.
func NormalizeGender(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "l", "laki-laki", "lelaki", "laki", "pria", "m", "male":
		return "L"
	case "p", "perempuan", "wanita", "f", "female":
		return "P"
	}
	return strings.TrimSpace(v)
}

// StudentsToXLSX menghasilkan file export daftar siswa.
func StudentsToXLSX(rows [][]string) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"
	style, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	for c, h := range []string{"Kode", "Nama", "Jenis Kelamin", "Kelas", "NIS", "Username", "Status"} {
		cell, _ := excelize.CoordinatesToCellName(c+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	f.SetCellStyle(sheet, "A1", "G1", style)
	f.SetColWidth(sheet, "A", "G", 18)
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

// StudentsToCSV menghasilkan export CSV.
func StudentsToCSV(rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	header := []string{"Kode", "Nama", "Jenis Kelamin", "Kelas", "NIS", "Username", "Status"}
	if err := w.Write(header); err != nil {
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

// LoginCardsPDF membuat kartu login siswa (4 kartu per halaman, A4).
func LoginCardsPDF(school string, cards []LoginCard) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 10)
	pdf.AddPage()
	w, h := 85.0, 45.0 // ukuran kartu mm
	mx, my := 20.0, 15.0
	gapX, gapY := 10.0, 8.0
	school = truncate(school, 28)

	for i, c := range cards {
		col, row := i%2, (i/2)%2
		if i > 0 && i%4 == 0 {
			pdf.AddPage()
		}
		x := mx + float64(col)*(w+gapX)
		y := my + float64(row)*(h+gapY)

		pdf.Rect(x, y, w, h, "D")
		pdf.SetXY(x+4, y+2)
		pdf.SetFont("Helvetica", "B", 11)
		pdf.CellFormat(0, 6, "FlowCBT", "", 0, "", false, 0, "")
		pdf.Ln(6)
		pdf.SetX(x + 4)
		pdf.SetFont("Helvetica", "", 8)
		pdf.CellFormat(0, 4, school, "", 1, "", false, 0, "")
		pdf.Ln(3)
		pdf.SetX(x + 4)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(0, 5, truncate(c.Name, 30), "", 1, "", false, 0, "")
		pdf.SetX(x + 4)
		pdf.SetFont("Helvetica", "", 9)
		pdf.CellFormat(0, 5, "Kelas: "+truncate(c.Class, 16), "", 1, "", false, 0, "")
		pdf.SetX(x + 4)
		pdf.CellFormat(0, 5, "Kode: "+c.StudentCode, "", 1, "", false, 0, "")
		pdf.SetX(x + 4)
		pdf.CellFormat(0, 5, "Username: "+c.Username, "", 1, "", false, 0, "")
		pdf.SetX(x + 4)
		pdf.CellFormat(0, 5, "Password: "+c.Password, "", 1, "", false, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type LoginCard struct {
	Name        string
	Class       string
	StudentCode string
	Username    string
	Password    string
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
