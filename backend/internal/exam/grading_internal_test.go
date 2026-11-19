// Uji unit murni S14: format titik & pembulatan nilai (tanpa DB).
package exam

import (
	"math"
	"testing"
)

func TestFormatPoints(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{10, "10"},
		{2.5, "2.5"},
		{1.25, "1.25"},
		{0, "0"},
	}
	for _, c := range cases {
		if got := formatPoints(c.in); got != c.want {
			t.Errorf("formatPoints(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRound2(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{3.14159, 3.14},
		{2.675, 2.68}, // pembulatan desimal setengah naik seperti ekspektasi guru
		{1 / 3.0, 0.33},
	}
	for _, c := range cases {
		if got := round2(c.in); math.Abs(got-c.want) > 0.001 {
			t.Errorf("round2(%v) = %v, want ~%v", c.in, got, c.want)
		}
	}
}

func TestSafePct(t *testing.T) {
	if got := safePct(70, 100); got != 70 {
		t.Errorf("safePct(70,100) = %v, want 70", got)
	}
	if got := safePct(5, 0); got != 0 {
		t.Errorf("safePct(5,0) = %v, want 0 (tanpa panik)", got)
	}
}
