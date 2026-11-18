package exam

import "testing"

func ptr[T any](v T) *T { return &v }

func TestValidateSchedule(t *testing.T) {
	t.Parallel()

	// Lengkap & sah.
	valid := ExamInput{
		QuestionSetID: "abc", Name: "Ujian Harian", DurationMin: 60,
		OpenAt: "2026-01-10T07:00:00+07:00", CloseAt: "2026-01-10T09:00:00+07:00",
		MaxAttempts: ptr(2), PassingScore: ptr(70.0),
		ClosePolicy: "grace", LoginMode: "both", ScorePolicy: "highest",
	}
	if err := valid.ValidateSchedule(); err != nil {
		t.Fatalf("input sah ditolak: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*ExamInput)
	}{
		{"nama kosong", func(i *ExamInput) { i.Name = " " }},
		{"paket kosong", func(i *ExamInput) { i.QuestionSetID = "" }},
		{"durasi nol", func(i *ExamInput) { i.DurationMin = 0 }},
		{"percobaan 0", func(i *ExamInput) { i.MaxAttempts = ptr(0) }},
		{"KKM 150", func(i *ExamInput) { i.PassingScore = ptr(150.0) }},
		{"close_policy asing", func(i *ExamInput) { i.ClosePolicy = "soft" }},
		{"login_mode asing", func(i *ExamInput) { i.LoginMode = "magic" }},
		{"score_policy asing", func(i *ExamInput) { i.ScorePolicy = "min" }},
		{"tutup sebelum buka", func(i *ExamInput) {
			i.OpenAt, i.CloseAt = i.CloseAt, i.OpenAt
		}},
		{"durasi melampaui jendela", func(i *ExamInput) { i.DurationMin = 200 }},
		{"hanya open_at", func(i *ExamInput) { i.CloseAt = "" }},
		{"format waktu buruk", func(i *ExamInput) { i.OpenAt = "besok pagi" }},
	}
	for _, c := range cases {
		in := valid
		c.mutate(&in)
		if err := in.ValidateSchedule(); err == nil {
			t.Errorf("%s: seharusnya ditolak", c.name)
		}
	}
}
