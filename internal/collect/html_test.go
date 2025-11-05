package collect

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"git.skobk.in/skobkin/jnovel-scrape/internal/httpx"
	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
)

func TestFetchHTMLSuccess(t *testing.T) {
	var archiveRequests int32
	var detailRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/" || r.URL.Path == "":
			atomic.AddInt32(&archiveRequests, 1)
			fmt.Fprint(w, `
				<html><body>
					<article>
						<h2 class="entry-title"><a href="/hero-volume-2-epub/">Hero Volume 2 EPUB</a></h2>
					</article>
					<article>
						<h2 class="entry-title"><a href="/mystery-epub-volume-3/">Mystery EPUB</a></h2>
					</article>
				</body></html>
			`)
		case r.URL.Path == "/page/2/":
			atomic.AddInt32(&archiveRequests, 1)
			fmt.Fprint(w, "<html><body></body></html>")
		case r.URL.Path == "/hero-volume-2-epub/":
			atomic.AddInt32(&detailRequests, 1)
			fmt.Fprintf(w, `
				<html>
					<body>
						<time datetime="2025-10-15T00:00:00Z"></time>
						<a rel="category">Light Novels</a>
						<a rel="tag">EPUB</a>
					</body>
				</html>
			`)
		case r.URL.Path == "/mystery-epub-volume-3/":
			atomic.AddInt32(&detailRequests, 1)
			fmt.Fprintf(w, `
				<html>
					<body>
						<p>Published on October 10, 2025</p>
						<a rel="tag">EPUB</a>
					</body>
				</html>
			`)
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
	opt := Options{
		BaseURL:     server.URL,
		MaxPages:    2,
		Concurrency: 1,
		Client:      client,
		Logger:      noopLogger{},
	}

	posts, warnings, err := FetchHTML(context.Background(), cutoff, opt)
	if err != nil {
		t.Fatalf("FetchHTML() error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning (missing volume), got %d: %v", len(warnings), warnings)
	}
	if posts[0].Title != "Hero" || posts[0].Type != model.TypeEPUB {
		t.Fatalf("unexpected first post: %+v", posts[0])
	}
	if posts[1].Volume == nil || *posts[1].Volume != 3 {
		t.Fatalf("expected slug-derived volume 3, got %v", posts[1].Volume)
	}
	if !strings.Contains(warnings[0], "missing volume") {
		t.Fatalf("unexpected warning text: %v", warnings)
	}
	if atomic.LoadInt32(&archiveRequests) != 2 {
		t.Fatalf("expected two archive requests, got %d", archiveRequests)
	}
	if atomic.LoadInt32(&detailRequests) != 2 {
		t.Fatalf("expected two detail requests, got %d", detailRequests)
	}
}
