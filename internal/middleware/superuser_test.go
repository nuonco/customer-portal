package middleware

import (
	"testing"
)

func TestIsSuperuserEmail(t *testing.T) {
	tests := []struct {
		email  string
		domain string
		want   bool
	}{
		{"alice@nuon.co", "nuon.co", true},
		{"ALICE@NUON.CO", "nuon.co", true},
		{"alice@nuon.co", "Nuon.Co", true},
		{"alice@example.com", "nuon.co", false},
		{"alice@notnuon.co", "nuon.co", false},
		{"", "nuon.co", false},
		{"alice@nuon.co", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		got := IsSuperuserEmail(tt.email, tt.domain)
		if got != tt.want {
			t.Errorf("IsSuperuserEmail(%q, %q) = %v, want %v", tt.email, tt.domain, got, tt.want)
		}
	}
}
