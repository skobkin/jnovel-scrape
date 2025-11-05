package collect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"git.skobk.in/skobkin/jnovel-scrape/internal/httpx"
	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
)

func TestParseWPTime(t *testing.T) {
	cases := []struct {
		name     string
		primary  string
		fallback string
	}{
		{"RFC3339", "2024-05-01T10:11:12Z", ""},
		{"RFC3339NoZone", "2024-05-01T10:11:12", ""},
		{"Fallback", "", "2024-05-01T10:11:12Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseWPTime(tc.primary, tc.fallback)
			if err != nil {
				t.Fatalf("parseWPTime() error = %v", err)
			}
			if got.IsZero() {
				t.Fatalf("parseWPTime() returned zero time")
			}
		})
	}

	if _, err := parseWPTime("", ""); err == nil {
		t.Fatalf("expected error when both inputs empty")
	}
}

func TestFetchAPISuccess(t *testing.T) {
	var postRequests int32
	var categoryRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wp-json/wp/v2/posts":
			if atomic.AddInt32(&postRequests, 1) == 1 {
				w.Header().Set("X-WP-TotalPages", "1")
			}
			page := r.URL.Query().Get("page")
			if page != "1" {
				json.NewEncoder(w).Encode([]apiPost{})
				return
			}

			posts := []apiPost{
				{
					ID:         101,
					Date:       "2025-10-15T00:00:00",
					DateGMT:    "2025-10-15T00:00:00",
					Link:       "https://example.com/hero-volume-2-epub/",
					Title:      rendered{Text: "Hero Volume 2 EPUB"},
					Categories: []int{11},
				},
				{
					ID:         102,
					Date:       "2025-10-10T00:00:00",
					DateGMT:    "2025-10-10T00:00:00",
					Link:       "https://example.com/mystery-epub-volume-3/",
					Title:      rendered{Text: "Mystery EPUB"},
					Categories: []int{12},
				},
			}
			json.NewEncoder(w).Encode(posts)
		case "/wp-json/wp/v2/categories":
			atomic.AddInt32(&categoryRequests, 1)
			items := []taxonomyItem{
				{ID: 11, Name: "Light Novels"},
				{ID: 12, Name: "Downloads"},
			}
			json.NewEncoder(w).Encode(items)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := httpx.NewClient(1*time.Millisecond, 5*time.Millisecond,
		httpx.WithHTTPClient(server.Client()),
		httpx.WithJitterFactor(0),
	)

	cutoff := time.Date(2025, time.October, 1, 0, 0, 0, 0, time.UTC)
	opt := Options{BaseURL: server.URL, Client: client, Logger: noopLogger{}}

	posts, warnings, err := FetchAPI(context.Background(), cutoff, opt)
	if err != nil {
		t.Fatalf("FetchAPI() error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %d: %v", len(warnings), warnings)
	}
	if posts[0].Title != "Hero" {
		t.Fatalf("unexpected first title: %s", posts[0].Title)
	}
	if posts[0].Volume == nil || *posts[0].Volume != 2 {
		t.Fatalf("expected volume 2, got %v", posts[0].Volume)
	}
	if posts[1].Volume == nil || *posts[1].Volume != 3 {
		t.Fatalf("expected slug-derived volume 3, got %v", posts[1].Volume)
	}
	if posts[0].Type != model.TypeEPUB {
		t.Fatalf("expected first post type EPUB, got %s", posts[0].Type)
	}
	if posts[0].Categories == nil || posts[0].Categories[0] != "Light Novels" {
		t.Fatalf("unexpected categories: %+v", posts[0].Categories)
	}
	if atomic.LoadInt32(&categoryRequests) != 1 {
		t.Fatalf("expected single taxonomy request, got %d", categoryRequests)
	}
}

func TestTaxonomyResolverCaching(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		items := []taxonomyItem{
			{ID: 1, Name: "One"},
			{ID: 2, Name: "Two"},
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer server.Close()

	client := httpx.NewClient(1*time.Millisecond, 5*time.Millisecond,
		httpx.WithHTTPClient(server.Client()),
		httpx.WithJitterFactor(0),
	)

	opt := Options{BaseURL: server.URL, Client: client}
	resolver := newTaxonomyResolver("categories", opt)

	ctx := context.Background()
	names, err := resolver.Resolve(ctx, []int{1, 2, 2, 3})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 resolved names (excluding missing), got %d", len(names))
	}
	if resolver.ResolvedCount() != 3 {
		t.Fatalf("expected resolver to track 3 ids, got %d", resolver.ResolvedCount())
	}

	names2, err := resolver.Resolve(ctx, []int{2, 3})
	if err != nil {
		t.Fatalf("Resolve second call error: %v", err)
	}
	if len(names2) != 1 || names2[0] != "Two" {
		t.Fatalf("unexpected second call names: %+v", names2)
	}
	if atomic.LoadInt32(&requests) != 1 {
		t.Fatalf("expected cached responses, got %d requests", requests)
	}
}
