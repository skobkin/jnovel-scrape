package util

import "testing"

func TestCleanTitle(t *testing.T) {
	input := "<em>My Novel</em> Volume&nbsp;5 | Special"
	want := "My Novel Volume 5 | Special"
	got := CleanTitle(input)
	if got != want {
		t.Fatalf("CleanTitle(%q) = %q, want %q", input, got, want)
	}
}

func TestEscapePipes(t *testing.T) {
	got := EscapePipes("A | B")
	if got != "A \\| B" {
		t.Fatalf("EscapePipes() = %q", got)
	}
}
