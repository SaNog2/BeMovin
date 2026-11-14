package web

import "testing"

func TestFormatTime(t *testing.T) {
	for in, want := range map[int]string{
		45: "45 s", 60: "1 min", 90: "1:30", 900: "15 min",
		3600: "1 h", 5400: "1 h 30 min", 3725: "1:02:05",
	} {
		if got := formatTime(in); got != want {
			t.Errorf("formatTime(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatDistance(t *testing.T) {
	for in, want := range map[int]string{
		400: "400 m", 1000: "1 km", 1500: "1.5 km", 2400: "2.4 km", 10000: "10 km", 21097: "21.1 km",
	} {
		if got := formatDistance(in); got != want {
			t.Errorf("formatDistance(%d) = %q, want %q", in, got, want)
		}
	}
}
