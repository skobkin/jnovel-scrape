package app

import "testing"

func TestParseMode(t *testing.T) {
	cases := []struct {
		input string
		want  Mode
		ok    bool
	}{
		{"auto", ModeAuto, true},
		{"API", ModeAPI, true},
		{"html", ModeHTML, true},
		{"invalid", "", false},
	}

	for _, tc := range cases {
		got, err := parseMode(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("parseMode(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("parseMode(%q) expected error", tc.input)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("parseMode(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseGroupMode(t *testing.T) {
	cases := []struct {
		input string
		want  GroupMode
		ok    bool
	}{
		{"none", GroupNone, true},
		{"TITLE", GroupTitle, true},
		{"invalid", "", false},
	}

	for _, tc := range cases {
		got, err := parseGroupMode(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("parseGroupMode(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("parseGroupMode(%q) expected error", tc.input)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("parseGroupMode(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseGroupSort(t *testing.T) {
	cases := []struct {
		input string
		want  GroupSort
		ok    bool
	}{
		{"asc", GroupSortAsc, true},
		{"DESC", GroupSortDesc, true},
		{"invalid", "", false},
	}

	for _, tc := range cases {
		got, err := parseGroupSort(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("parseGroupSort(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("parseGroupSort(%q) expected error", tc.input)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("parseGroupSort(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
