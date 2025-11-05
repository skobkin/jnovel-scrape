package app

import (
	"testing"
	"time"

	"github.com/skobkin/jnovels-parser/internal/model"
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
