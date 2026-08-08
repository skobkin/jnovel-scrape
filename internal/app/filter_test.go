package app

import (
	"testing"
	"time"

	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
)

func TestFilterPosts_VolumeFilterExcludesBlanks(t *testing.T) {
	volumeVal := 11.0
	cfg := Config{
		VolumeFilter: &volumeVal,
	}

	posts := model.Posts{
		{
			Title:  "Series Volume 11 PDF",
			Volume: &volumeVal,
			Type:   model.TypePDF,
			Date:   time.Now(),
			Link:   "https://example.com/a",
		},
		{
			Title: "Bundle Pack",
			Type:  model.TypeEPUB,
			Date:  time.Now(),
			Link:  "https://example.com/b",
		},
	}

	filtered, stats := filterPosts(posts, cfg)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 post after filtering, got %d", len(filtered))
	}
	if stats.VolumeDropped != 1 {
		t.Fatalf("expected 1 volume drop, got %d", stats.VolumeDropped)
	}
}

func TestFilterPosts_TypeFilter(t *testing.T) {
	cfg := Config{
		TypeFilters: map[model.PostType]struct{}{
			model.TypeEPUB: {},
		},
	}

	posts := model.Posts{
		{
			Title: "Item 1",
			Type:  model.TypeUnknown,
			Date:  time.Now(),
			Link:  "https://example.com/1",
		},
		{
			Title: "Item 2",
			Type:  model.TypeEPUB,
			Date:  time.Now(),
			Link:  "https://example.com/2",
		},
	}

	filtered, stats := filterPosts(posts, cfg)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 post, got %d", len(filtered))
	}
	if stats.TypeDropped != 1 {
		t.Fatalf("expected 1 type drop, got %d", stats.TypeDropped)
	}
	if filtered[0].Type != model.TypeEPUB {
		t.Fatalf("expected EPUB entry, got %v", filtered[0].Type)
	}
}

func TestFilterPosts_MatchesAnyTitleFilterCaseInsensitively(t *testing.T) {
	cfg := Config{TitleFilters: []string{"sword art online", "OVERLORD"}}
	posts := model.Posts{
		{Title: "Sword Art Online Volume 1", Date: time.Now()},
		{Title: "overlord Volume 2", Date: time.Now()},
		{Title: "Re:Zero Volume 3", Date: time.Now()},
	}

	filtered, stats := filterPosts(posts, cfg)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(filtered))
	}
	if stats.TitleDropped != 1 {
		t.Fatalf("expected 1 title drop, got %d", stats.TitleDropped)
	}
}

// TestFilterPosts_TitleFilterIsUnicodeAndDiacriticInsensitive exercises
// the post-koanf filter against titles and needles with combining
// marks, diacritics, and decomposed Unicode sequences. Each pair must
// produce the documented folded form so the user can search using the
// romanisation they prefer.
func TestFilterPosts_TitleFilterIsUnicodeAndDiacriticInsensitive(t *testing.T) {
	cfg := Config{TitleFilters: []string{"shumatsu", "chunibyo", "cafe"}}
	posts := model.Posts{
		{Title: "Shūmatsu no Valkyrie Volume 1", Date: time.Now()},
		{Title: "Chûnibyô demo Koi ga Shitai! Volume 2", Date: time.Now()},
		{Title: "Café Stéreo Volume 3", Date: time.Now()},
		{Title: "Decomposed Cafe\u0301 Volume 4", Date: time.Now()},
		{Title: "Unrelated Title Volume 5", Date: time.Now()},
	}

	filtered, stats := filterPosts(posts, cfg)
	if len(filtered) != 4 {
		t.Fatalf("expected 4 posts, got %d: %+v", len(filtered), filtered)
	}
	if stats.TitleDropped != 1 {
		t.Fatalf("expected 1 title drop, got %d", stats.TitleDropped)
	}
}

// TestFilterPosts_TitleFilterMatchesAcrossMultipleTitleFilters verifies
// that more than one unicode title filter is honoured in a single
// pass, with each title matching exactly one needle.
func TestFilterPosts_TitleFilterMatchesAcrossMultipleTitleFilters(t *testing.T) {
	cfg := Config{TitleFilters: []string{"ōsō", "alice"}}
	posts := model.Posts{
		{Title: "Sōsō no Pet na Kanojo", Date: time.Now()},
		{Title: "Alice in Borderland", Date: time.Now()},
		{Title: "Spice and Wolf", Date: time.Now()},
	}

	filtered, stats := filterPosts(posts, cfg)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(filtered))
	}
	if stats.TitleDropped != 1 {
		t.Fatalf("expected 1 title drop, got %d", stats.TitleDropped)
	}
}

// TestFilterPosts_TitleFilterNormalisesNeedleWhitespaceAndNbsp covers
// commit 2's needle normalisation. The haystack is pre-normalised
// by CleanTitle; the needle is taken as the user typed it. The
// filter must accept needles with extra spaces, tabs, and NBSP
// without silently dropping the post.
func TestFilterPosts_TitleFilterNormalisesNeedleWhitespaceAndNbsp(t *testing.T) {
	cases := []struct {
		name   string
		needle string
	}{
		{"multi-space", "sword  art  online"},
		{"leading and trailing spaces", "   sword art online   "},
		{"nbsp", "sword\u00a0art\u00a0online"},
		{"mixed nbsp and space", "sword\u00a0 art   online"},
		{"tabs", "sword	art	online"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{TitleFilters: []string{tc.needle}}
			posts := model.Posts{
				{Title: "Sword Art Online Volume 1", Date: time.Now()},
				{Title: "Overlord Volume 2", Date: time.Now()},
			}
			filtered, stats := filterPosts(posts, cfg)
			if len(filtered) != 1 {
				t.Fatalf("needle %q: expected 1 post, got %d", tc.needle, len(filtered))
			}
			if stats.TitleDropped != 1 {
				t.Fatalf("needle %q: expected 1 title drop, got %d", tc.needle, stats.TitleDropped)
			}
		})
	}
}
