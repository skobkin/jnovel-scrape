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

// TestFilterPosts_TitleFilterWordMode uses the post-fix behaviour
// for the new --title-mode=word option. Word mode requires every
// needle token to appear as a complete token in the title.
func TestFilterPosts_TitleFilterWordMode(t *testing.T) {
	cfg := Config{
		TitleFilters: []string{"art"},
		TitleMode:    TitleModeWord,
	}
	posts := model.Posts{
		{Title: "Sword Art Online", Date: time.Now()},
		{Title: "Arte", Date: time.Now()},
		{Title: "Departure", Date: time.Now()},
		{Title: "Heart no Kuni no Alice", Date: time.Now()},
		{Title: "Smartphone Isekai", Date: time.Now()},
		{Title: "Party of Heroes", Date: time.Now()},
	}
	filtered, stats := filterPosts(posts, cfg)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 post (only 'Sword Art Online'), got %d: %+v", len(filtered), titles(filtered))
	}
	if filtered[0].Title != "Sword Art Online" {
		t.Fatalf("expected 'Sword Art Online', got %q", filtered[0].Title)
	}
	if stats.TitleDropped != 5 {
		t.Fatalf("expected 5 title drops, got %d", stats.TitleDropped)
	}
}

// TestFilterPosts_TitleFilterWordModeMultiToken covers multi-word
// needles and out-of-order matching. "no game" must match "No
// Game No Life" (both words present, one duplicated) and not
// match a title that only has one of the tokens.
func TestFilterPosts_TitleFilterWordModeMultiToken(t *testing.T) {
	cfg := Config{
		TitleFilters: []string{"no game"},
		TitleMode:    TitleModeWord,
	}
	posts := model.Posts{
		{Title: "No Game No Life", Date: time.Now()},
		{Title: "Game of Thrones", Date: time.Now()},
		{Title: "No Tomorrow", Date: time.Now()},
		{Title: "Sword Art Online", Date: time.Now()},
	}
	filtered, stats := filterPosts(posts, cfg)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 post, got %d: %+v", len(filtered), titles(filtered))
	}
	if filtered[0].Title != "No Game No Life" {
		t.Fatalf("expected 'No Game No Life', got %q", filtered[0].Title)
	}
	if stats.TitleDropped != 3 {
		t.Fatalf("expected 3 title drops, got %d", stats.TitleDropped)
	}
}

// TestFilterPosts_TitleFilterWordModeUnicode verifies word-mode
// folding: tokens fold through the same Unicode + diacritic
// pipeline as substring mode.
func TestFilterPosts_TitleFilterWordModeUnicode(t *testing.T) {
	cfg := Config{
		TitleFilters: []string{"valkyrie"},
		TitleMode:    TitleModeWord,
	}
	posts := model.Posts{
		{Title: "Shūmatsu no Valkyrie", Date: time.Now()},
		{Title: "Valkyrie Drive", Date: time.Now()},
		{Title: "Sword Art Online", Date: time.Now()},
	}
	filtered, stats := filterPosts(posts, cfg)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 posts, got %d: %+v", len(filtered), titles(filtered))
	}
	if stats.TitleDropped != 1 {
		t.Fatalf("expected 1 title drop, got %d", stats.TitleDropped)
	}
}

// TestFilterPosts_TitleFilterSubstringIsDefault is a regression
// guard: if a caller constructs Config manually without setting
// TitleMode, the filter must still use the historical substring
// path. This is what the empty-string fallback in
// parseTitleMode buys us, but the filter is what actually
// dispatches on TitleMode so the guard belongs at this layer.
func TestFilterPosts_TitleFilterSubstringIsDefault(t *testing.T) {
	cfg := Config{TitleFilters: []string{"art"}}
	posts := model.Posts{
		{Title: "Sword Art Online", Date: time.Now()},
		{Title: "Arte", Date: time.Now()},
		{Title: "Departure", Date: time.Now()},
	}
	filtered, stats := filterPosts(posts, cfg)
	if len(filtered) != 3 {
		t.Fatalf("default substring mode should match all three, got %d", len(filtered))
	}
	if stats.TitleDropped != 0 {
		t.Fatalf("default substring mode: expected 0 drops, got %d", stats.TitleDropped)
	}
}

func titles(posts model.Posts) []string {
	out := make([]string, 0, len(posts))
	for _, p := range posts {
		out = append(out, p.Title)
	}

	return out
}
