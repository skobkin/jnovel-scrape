package app

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
)

func TestDedupePosts(t *testing.T) {
	ts := time.Now()
	posts := model.Posts{
		{Title: "A", Link: "https://example.com/a", Date: ts},
		{Title: "B", Link: "https://example.com/b", Date: ts.Add(-time.Hour)},
		{Title: "A-dup", Link: "https://example.com/a", Date: ts.Add(-2 * time.Hour)},
	}

	deduped, removed := dedupePosts(posts)
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if len(deduped) != 2 {
		t.Fatalf("expected 2 posts after dedupe, got %d", len(deduped))
	}
	if deduped[0].Title != "A" || deduped[1].Title != "B" {
		t.Fatalf("dedupe preserved wrong order: %+v", deduped)
	}
}

func TestApplyGroupingTitleAsc(t *testing.T) {
	ts := time.Now()
	v1 := 1.0
	v2 := 2.0
	posts := model.Posts{
		{Title: "B Title", Volume: &v2, Date: ts.Add(-4 * time.Hour), Link: "https://example.com/b2"},
		{Title: "A Title", Volume: &v2, Date: ts.Add(-2 * time.Hour), Link: "https://example.com/a2"},
		{Title: "A Title", Volume: &v1, Date: ts.Add(-time.Hour), Link: "https://example.com/a1"},
		{Title: "A Title", Date: ts, Link: "https://example.com/a0"},
	}

	grouped := applyGrouping(posts, GroupTitle, GroupSortAsc)
	if len(grouped) != len(posts) {
		t.Fatalf("unexpected grouped length: %d", len(grouped))
	}
	if grouped[0].Title != "A Title" || grouped[1].Title != "A Title" || grouped[2].Title != "A Title" {
		t.Fatalf("expected A Title group first, got %+v", grouped[:3])
	}
	if grouped[0].Volume == nil || *grouped[0].Volume != v1 {
		t.Fatalf("expected first volume=1, got %v", grouped[0].Volume)
	}
	if grouped[1].Volume == nil || *grouped[1].Volume != v2 {
		t.Fatalf("expected second volume=2, got %v", grouped[1].Volume)
	}
	if grouped[2].Volume != nil {
		t.Fatalf("expected third volume nil, got %v", grouped[2].Volume)
	}
	if grouped[3].Title != "B Title" {
		t.Fatalf("expected B Title last, got %s", grouped[3].Title)
	}
}

func TestApplyGroupingTitleDesc(t *testing.T) {
	ts := time.Now()
	v1 := 1.0
	v2 := 2.0
	posts := model.Posts{
		{Title: "A Title", Volume: &v1, Date: ts.Add(-time.Hour), Link: "https://example.com/a1"},
		{Title: "A Title", Volume: &v2, Date: ts.Add(-2 * time.Hour), Link: "https://example.com/a2"},
		{Title: "B Title", Date: ts, Link: "https://example.com/b0"},
	}

	grouped := applyGrouping(posts, GroupTitle, GroupSortDesc)
	if len(grouped) != len(posts) {
		t.Fatalf("unexpected grouped length: %d", len(grouped))
	}
	if grouped[0].Title != "A Title" || grouped[1].Title != "A Title" {
		t.Fatalf("expected A Title group first, got %+v", grouped[:2])
	}
	if grouped[0].Volume == nil || *grouped[0].Volume != v2 {
		t.Fatalf("expected first volume=2 desc, got %v", grouped[0].Volume)
	}
	if grouped[1].Volume == nil || *grouped[1].Volume != v1 {
		t.Fatalf("expected second volume=1 desc, got %v", grouped[1].Volume)
	}
	if grouped[2].Title != "B Title" {
		t.Fatalf("expected B Title last, got %s", grouped[2].Title)
	}
}

func TestApplyGroupingWithExtra(t *testing.T) {
	ts := time.Now()
	v10 := 10.0
	posts := model.Posts{
		{Title: "Series", Volume: &v10, VolumeExtra: "Act 2", Date: ts.Add(-2 * time.Hour), Link: "https://example.com/s2"},
		{Title: "Series", Volume: &v10, VolumeExtra: "Act 1", Date: ts.Add(-time.Hour), Link: "https://example.com/s1"},
	}

	groupedAsc := applyGrouping(posts, GroupTitle, GroupSortAsc)
	if groupedAsc[0].VolumeExtra != "Act 1" {
		t.Fatalf("expected Act 1 first, got %s", groupedAsc[0].VolumeExtra)
	}

	groupedDesc := applyGrouping(posts, GroupTitle, GroupSortDesc)
	if groupedDesc[0].VolumeExtra != "Act 2" {
		t.Fatalf("expected Act 2 first in desc, got %s", groupedDesc[0].VolumeExtra)
	}
}

func TestWriteOutputToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "result.md")

	cutoff := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	vol := 2.0
	posts := model.Posts{
		{
			Title:  "Sample Story",
			Volume: &vol,
			Type:   model.TypeEPUB,
			Date:   cutoff,
			Link:   "https://example.com/sample-story",
		},
	}

	logger := NewLogger(io.Discard)
	cfg := Config{Cutoff: cutoff, OutputPath: outPath}

	if err := writeOutput(cfg, posts, logger); err != nil {
		t.Fatalf("writeOutput error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	content := string(data)
	if !bytes.Contains(data, []byte("Generated from jnovels.com (cutoff: 2025-01-01)")) {
		t.Fatalf("output missing header: %q", content)
	}
	if !strings.Contains(content, "| Sample Story | 2 | EPUB | 2025-01-01 | [link](https://example.com/sample-story) |") {
		t.Fatalf("unexpected table row: %q", content)
	}
}
