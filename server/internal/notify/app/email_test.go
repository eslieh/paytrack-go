package app

import "testing"

func TestFormatting(t *testing.T) {
	for in, want := range map[int64]string{1: "KES 1", 999: "KES 999", 1500: "KES 1,500", 250000: "KES 250,000", 1234567: "KES 1,234,567"} {
		if got := FormatKES(in); got != want {
			t.Errorf("FormatKES(%d) = %q, want %q", in, got, want)
		}
	}
	if got := MaskPhone("254712345678"); got != "+254 7•• ••• 678" {
		t.Errorf("MaskPhone = %q", got)
	}
}
