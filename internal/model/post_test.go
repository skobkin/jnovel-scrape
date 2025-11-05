package model

import (
	"testing"
	"time"
)

func TestAllTypesIncludesUnknown(t *testing.T) {
	types := AllTypes()
	if len(types) != len(KnownTypes)+1 {
		t.Fatalf("unexpected length: %d", len(types))
	}
	if types[len(types)-1] != TypeUnknown {
		t.Fatalf("expected last type to be UNKNOWN, got %s", types[len(types)-1])
	}
}

func TestNormalizeType(t *testing.T) {
	cases := map[string]PostType{
		"EPUB":    TypeEPUB,
		"pdf":     TypePDF,
		" Manga ": TypeManga,
		"UNK":     TypeUnknown,
	}
	for input, want := range cases {
		if got := NormalizeType(input); got != want {
			t.Fatalf("NormalizeType(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestPostVolumeHelpers(t *testing.T) {
	vol := 3.0
	post := Post{Volume: &vol}
	if !post.HasVolume() {
		t.Fatalf("expected HasVolume true")
	}
	if !post.VolumeEqual(3) {
		t.Fatalf("expected VolumeEqual true")
	}
	if post.VolumeEqual(2) {
		t.Fatalf("did not expect volume equal 2")
	}
}

func TestFormatDateAndSort(t *testing.T) {
	base := time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)
	posts := Posts{
		{Title: "B", Date: base.Add(-time.Hour)},
		{Title: "A", Date: base.Add(-time.Hour)},
		{Title: "Latest", Date: base},
	}
	posts.Sort()

	if posts[0].Title != "Latest" {
		t.Fatalf("expected most recent post first, got %s", posts[0].Title)
	}
	if posts[1].Title != "A" || posts[2].Title != "B" {
		t.Fatalf("expected alphabetical order for equal dates, got %+v", posts[1:])
	}
	if got := posts[0].FormatDate(); got != "2025-01-01" {
		t.Fatalf("unexpected date formatting: %s", got)
	}
}
