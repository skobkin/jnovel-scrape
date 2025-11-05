package collect

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"git.skobk.in/skobkin/jnovel-scrape/internal/httpx"
	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
	"git.skobk.in/skobkin/jnovel-scrape/internal/util"
)

const fallbackUserAgent = "jnovels-scrape/1.0 (+https://example.com/contact)"

var (
	articlePattern   = regexp.MustCompile(`(?is)<article\b.*?</article>`)
	headingPattern   = regexp.MustCompile(`(?is)<h[12][^>]*class="[^">]*entry-title[^">]*"[^>]*>.*?<a[^>]*href="([^\"]+)"[^>]*>(.*?)</a>`)
	timePattern      = regexp.MustCompile(`(?is)<time[^>]*datetime="([^\"]+)"`)
	metaTimePattern  = regexp.MustCompile(`(?is)<meta[^>]*property="article:published_time"[^>]*content="([^\"]+)"`)
	anchorRelPattern = regexp.MustCompile(`(?is)<a[^>]*rel="([^\"]+)"[^>]*>(.*?)</a>`)
	dateTextPattern  = regexp.MustCompile(`(?i)(January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},\s+\d{4}`)
)

// FetchHTML crawls the website using HTML as a fallback.
func FetchHTML(ctx context.Context, cutoff time.Time, opt Options) (model.Posts, []string, error) {
	if opt.Client == nil {
		return nil, nil, fmt.Errorf("http client is required")
	}
	if opt.BaseURL == "" {
		opt.BaseURL = DefaultBaseURL
	}
	if opt.MaxPages <= 0 {
		opt.MaxPages = 2000
	}
	if opt.Concurrency <= 0 {
		opt.Concurrency = 4
	}

	logger := opt.Logger
	if logger == nil {
		logger = noopLogger{}
	}

	var (
		allPosts model.Posts
		warnings []string
	)

	for page := 1; page <= opt.MaxPages; page++ {
		pageURL := archiveURL(opt.BaseURL, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return nil, nil, err
		}
		setHTMLHeaders(req, opt.UserAgent)

		resp, err := opt.Client.Do(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		if resp.StatusCode >= 400 {
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, nil, fmt.Errorf("archive request failed: %s (%s)", resp.Status, string(payload))
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, nil, fmt.Errorf("read archive: %w", err)
		}

		candidates := extractArchiveCandidates(string(body), opt.BaseURL)
		logger.Infof("HTML page=%d candidates=%d", page, len(candidates))
		if len(candidates) == 0 {
			break
		}

		pagePosts, pageWarnings := enrichCandidates(ctx, opt, candidates)
		warnings = append(warnings, pageWarnings...)

		var kept model.Posts
		for _, post := range pagePosts {
			if post.Date.Before(cutoff) {
				warnings = append(warnings, fmt.Sprintf("%s skipped (date %s before cutoff)", post.Link, post.FormatDate()))
				continue
			}
			allPosts = append(allPosts, post)
			kept = append(kept, post)
		}

		if len(kept) == 0 {
			break
		}
	}

	allPosts.Sort()
	return allPosts, warnings, nil
}

type archiveCandidate struct {
	Title string
	Link  string
}

func archiveURL(base string, page int) string {
	trimmed := strings.TrimRight(base, "/")
	if page <= 1 {
		return trimmed
	}
	return fmt.Sprintf("%s/page/%d/", trimmed, page)
}

func setHTMLHeaders(req *http.Request, userAgent string) {
	if userAgent == "" {
		userAgent = fallbackUserAgent
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
}

func extractArchiveCandidates(pageHTML, base string) []archiveCandidate {
	blocks := articlePattern.FindAllString(pageHTML, -1)
	candidates := make([]archiveCandidate, 0, len(blocks))
	for _, block := range blocks {
		match := headingPattern.FindStringSubmatch(block)
		if len(match) < 3 {
			continue
		}
		href := strings.TrimSpace(match[1])
		title := util.CleanTitle(match[2])
		if href == "" || title == "" {
			continue
		}
		candidates = append(candidates, archiveCandidate{
			Title: title,
			Link:  resolveLink(base, href),
		})
	}
	return candidates
}

type detailResult struct {
	post     *model.Post
	warnings []string
}

func enrichCandidates(ctx context.Context, opt Options, candidates []archiveCandidate) ([]model.Post, []string) {
	jobCh := make(chan archiveCandidate)
	resultCh := make(chan detailResult)
	var wg sync.WaitGroup

	workerCount := opt.Concurrency
	if workerCount <= 0 {
		workerCount = 4
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for candidate := range jobCh {
				select {
				case <-ctx.Done():
					return
				default:
				}
				resultCh <- fetchDetail(ctx, opt.Client, opt.UserAgent, candidate)
			}
		}()
	}

	go func() {
		for _, candidate := range candidates {
			select {
			case jobCh <- candidate:
			case <-ctx.Done():
				close(jobCh)
				return
			}
		}
		close(jobCh)
	}()

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var (
		collected []model.Post
		warnings  []string
	)

	for result := range resultCh {
		warnings = append(warnings, result.warnings...)
		if result.post != nil {
			collected = append(collected, *result.post)
		}
	}
	return collected, warnings
}

func fetchDetail(ctx context.Context, client *httpx.Client, userAgent string, candidate archiveCandidate) detailResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate.Link, nil)
	if err != nil {
		return detailResult{warnings: []string{fmt.Sprintf("%s build request: %v → skipped", candidate.Link, err)}}
	}
	setHTMLHeaders(req, userAgent)

	resp, err := client.Do(ctx, req)
	if err != nil {
		return detailResult{warnings: []string{fmt.Sprintf("%s request failed: %v → skipped", candidate.Link, err)}}
	}
	if resp.StatusCode >= 400 {
		payload, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return detailResult{warnings: []string{fmt.Sprintf("%s unexpected status %s (%s) → skipped", candidate.Link, resp.Status, string(payload))}}
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return detailResult{warnings: []string{fmt.Sprintf("%s read error: %v → skipped", candidate.Link, err)}}
	}

	html := string(body)
	published, err := extractPublishedDate(html)
	if err != nil {
		return detailResult{warnings: []string{fmt.Sprintf("%s missing date (%v) → skipped", candidate.Link, err)}}
	}

	categories, tags := extractTaxonomy(html)
	rawTitle := candidate.Title
	postType := util.InferType(rawTitle, categories, tags)
	title, volume, volumeExtra := util.ExtractTitleAndVolume(rawTitle)
	if volume == nil {
		volume, _ = util.ParseVolume(rawTitle)
	}

	post := model.Post{
		Title:       title,
		Volume:      volume,
		VolumeExtra: volumeExtra,
		Type:        postType,
		Date:        published.UTC(),
		Link:        candidate.Link,
		Categories:  categories,
		Tags:        tags,
	}

	if post.Volume == nil {
		if slugVol, slugExtra, ok := util.ExtractVolumeFromLink(post.Link); ok {
			post.Volume = slugVol
			if post.VolumeExtra == "" {
				post.VolumeExtra = slugExtra
			}
		}
	}
	post.VolumeExtra = strings.TrimSpace(post.VolumeExtra)

	warnings := make([]string, 0, 2)
	if volume == nil {
		warnings = append(warnings, fmt.Sprintf("%s missing volume (no regex match) → kept with blank volume", candidate.Link))
	}
	if postType == model.TypeUnknown {
		warnings = append(warnings, fmt.Sprintf("%s type unresolved → UNKNOWN", candidate.Link))
	}

	return detailResult{post: &post, warnings: warnings}
}

func extractPublishedDate(content string) (time.Time, error) {
	if match := timePattern.FindStringSubmatch(content); len(match) > 1 {
		if parsed, err := parseWPTime(match[1], ""); err == nil {
			return parsed, nil
		}
	}
	if match := metaTimePattern.FindStringSubmatch(content); len(match) > 1 {
		if parsed, err := parseWPTime(match[1], ""); err == nil {
			return parsed, nil
		}
	}

	stripped := util.StripTags(content)
	if match := dateTextPattern.FindString(stripped); match != "" {
		if parsed, err := time.Parse("January 2, 2006", strings.TrimSpace(match)); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("no publish date found")
}

func extractTaxonomy(content string) (categories, tags []string) {
	catSet := map[string]struct{}{}
	tagSet := map[string]struct{}{}
	matches := anchorRelPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		rel := strings.ToLower(match[1])
		label := util.CleanTitle(match[2])
		if label == "" {
			continue
		}
		switch {
		case strings.Contains(rel, "category"):
			catSet[label] = struct{}{}
		case strings.Contains(rel, "tag"):
			tagSet[label] = struct{}{}
		}
	}
	categories = setToSortedSlice(catSet)
	tags = setToSortedSlice(tagSet)
	return
}

func resolveLink(baseURL, href string) string {
	if href == "" {
		return href
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	return base.ResolveReference(parsed).String()
}

func setToSortedSlice(items map[string]struct{}) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for k := range items {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
