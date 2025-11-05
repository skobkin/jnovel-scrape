package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
	"git.skobk.in/skobkin/jnovel-scrape/internal/util"
)

// FetchAPI crawls posts using the WordPress REST API.
func FetchAPI(ctx context.Context, cutoff time.Time, opt Options) (model.Posts, []string, error) {
	if opt.Client == nil {
		return nil, nil, fmt.Errorf("http client is required")
	}
	if opt.BaseURL == "" {
		opt.BaseURL = DefaultBaseURL
	}
	if opt.MaxPages <= 0 {
		opt.MaxPages = 2000
	}

	logger := opt.Logger
	if logger == nil {
		logger = noopLogger{}
	}

	logger.Infof("API mode: cutoff=%s maxPages=%d", cutoff.Format("2006-01-02"), opt.MaxPages)

	postsEndpoint, err := url.JoinPath(opt.BaseURL, "/wp-json/wp/v2/posts")
	if err != nil {
		return nil, nil, fmt.Errorf("build posts endpoint: %w", err)
	}

	var (
		rawPosts   []apiPost
		warnings   []string
		totalPages int
		stopPaging bool
	)
	categoryIDs := make(map[int]struct{})
	tagIDs := make(map[int]struct{})

	for page := 1; page <= opt.MaxPages; page++ {
		reqURL, err := url.Parse(postsEndpoint)
		if err != nil {
			return nil, nil, fmt.Errorf("parse posts endpoint: %w", err)
		}
		query := reqURL.Query()
		query.Set("per_page", "100")
		query.Set("page", strconv.Itoa(page))
		query.Set("order", "desc")
		query.Set("orderby", "date")
		query.Set("after", cutoff.Format(time.RFC3339))
		reqURL.RawQuery = query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
		if err != nil {
			return nil, nil, err
		}
		setStandardHeaders(req, opt.UserAgent)

		resp, err := opt.Client.Do(ctx, req)
		if err != nil {
			return nil, nil, err
		}
		if resp.StatusCode >= 400 {
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, nil, fmt.Errorf("posts request failed: %s (%s)", resp.Status, string(payload))
		}

		if page == 1 {
			if headerVal := resp.Header.Get("X-WP-TotalPages"); headerVal != "" {
				if total, err := strconv.Atoi(headerVal); err == nil {
					totalPages = total
				}
			}
			if totalPages == 0 {
				totalPages = opt.MaxPages
			}
		}

		var apiPosts []apiPost
		if err := decodeJSON(resp.Body, &apiPosts); err != nil {
			resp.Body.Close()
			return nil, nil, err
		}
		resp.Body.Close()

		logger.Infof("API page=%d returned %d posts", page, len(apiPosts))

		if len(apiPosts) == 0 {
			break
		}

		for _, ap := range apiPosts {
			rawPosts = append(rawPosts, ap)
			for _, id := range ap.Categories {
				categoryIDs[id] = struct{}{}
			}
			for _, id := range ap.Tags {
				tagIDs[id] = struct{}{}
			}
			if !stopPaging {
				if parsed, err := parseWPTime(ap.Date, ap.DateGMT); err == nil && parsed.Before(cutoff) {
					stopPaging = true
				}
			}
		}

		if page >= totalPages || stopPaging {
			break
		}
	}

	if len(rawPosts) == 0 {
		return nil, warnings, nil
	}

	if stopPaging {
		logger.Infof("API pagination stopped after encountering posts older than cutoff")
	}

	categoryList := sortedKeys(categoryIDs)
	if len(categoryList) > 0 {
		logger.Infof("API taxonomy lookup: categories=%d", len(categoryList))
	}

	categoryMap, err := fetchSelectedTaxonomy(ctx, opt, "categories", categoryList)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch categories: %w", err)
	}

	var allPosts model.Posts
	for _, ap := range rawPosts {
		categoryNames := lookupNames(categoryMap, ap.Categories)

		post, warn, skip := transformAPIPost(ap, cutoff, categoryNames, nil)
		if warn != "" {
			warnings = append(warnings, warn)
		}
		if skip {
			continue
		}
		if post != nil {
			allPosts = append(allPosts, *post)
		}
	}

	allPosts.Sort()
	return allPosts, warnings, nil
}

func fetchSelectedTaxonomy(ctx context.Context, opt Options, taxonomy string, ids []int) (map[int]string, error) {
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	endpoint, err := url.JoinPath(opt.BaseURL, "/wp-json/wp/v2/", taxonomy)
	if err != nil {
		return nil, err
	}
	result := make(map[int]string, len(ids))

	batches := chunkInts(ids, 100)
	for _, batch := range batches {
		reqURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		query := reqURL.Query()
		query.Set("per_page", strconv.Itoa(len(batch)))
		query.Set("include", joinInts(batch))
		query.Set("_fields", "id,name")
		reqURL.RawQuery = query.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
		if err != nil {
			return nil, err
		}
		setStandardHeaders(req, opt.UserAgent)

		resp, err := opt.Client.Do(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode >= 400 {
			payload, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("%s request failed: %s (%s)", taxonomy, resp.Status, string(payload))
		}

		var items []taxonomyItem
		if err := decodeJSON(resp.Body, &items); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		for _, item := range items {
			result[item.ID] = item.Name
		}
	}
	return result, nil
}

func setStandardHeaders(req *http.Request, userAgent string) {
	if userAgent == "" {
		userAgent = "jnovels-scrape/1.0 (+https://example.com/contact)"
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
}

func decodeJSON(body io.ReadCloser, target interface{}) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(target); err != nil && err != io.EOF {
		return err
	}
	return nil
}

type taxonomyItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type apiPost struct {
	ID         int64    `json:"id"`
	Date       string   `json:"date"`
	DateGMT    string   `json:"date_gmt"`
	Link       string   `json:"link"`
	Title      rendered `json:"title"`
	Categories []int    `json:"categories"`
	Tags       []int    `json:"tags"`
}

type rendered struct {
	Text string `json:"rendered"`
}

func transformAPIPost(src apiPost, cutoff time.Time, categoryNames []string, tagNames []string) (*model.Post, string, bool) {
	postDate, err := parseWPTime(src.Date, src.DateGMT)
	if err != nil {
		return nil, fmt.Sprintf("%s failed to parse date: %v → skipped", src.Link, err), true
	}
	if postDate.Before(cutoff) {
		return &model.Post{Date: postDate}, "", true
	}
	rawTitle := util.CleanTitle(src.Title.Text)
	if rawTitle == "" {
		return nil, fmt.Sprintf("post id=%d missing title → skipped", src.ID), true
	}
	postType := util.InferType(rawTitle, categoryNames, tagNames)
	title, volume, volumeExtra := util.ExtractTitleAndVolume(rawTitle)
	if volume == nil {
		volume, _ = util.ParseVolume(rawTitle)
	}

	if src.Link == "" {
		return nil, fmt.Sprintf("post id=%d missing link → skipped", src.ID), true
	}

	post := model.Post{
		Title:       title,
		Volume:      volume,
		VolumeExtra: volumeExtra,
		Type:        postType,
		Date:        postDate.UTC(),
		Link:        src.Link,
		SourceID:    src.ID,
		Categories:  categoryNames,
		Tags:        tagNames,
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

	if post.Volume == nil {
		return &post, fmt.Sprintf("%s missing volume (no regex match) → kept with blank volume", post.Link), false
	}
	if post.Type == model.TypeUnknown {
		return &post, fmt.Sprintf("%s type unresolved → UNKNOWN", post.Link), false
	}
	return &post, "", false
}

func lookupNames(table map[int]string, ids []int) []string {
	if len(ids) == 0 || len(table) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := table[id]; ok {
			out = append(out, name)
		}
	}
	return out
}

func parseWPTime(primary, fallback string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, candidate := range []string{primary, fallback} {
		if candidate == "" {
			continue
		}
		for _, layout := range formats {
			if parsed, err := time.Parse(layout, candidate); err == nil {
				return parsed, nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format: primary=%q fallback=%q", primary, fallback)
}

func sortedKeys(set map[int]struct{}) []int {
	if len(set) == 0 {
		return nil
	}
	out := make([]int, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}

func chunkInts(ids []int, size int) [][]int {
	if size <= 0 {
		size = 100
	}
	var batches [][]int
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		chunk := make([]int, end-start)
		copy(chunk, ids[start:end])
		batches = append(batches, chunk)
	}
	return batches
}

func joinInts(ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

type taxonomyResolver struct {
	taxonomy string
	opt      Options

	mu      sync.Mutex
	cache   map[int]string
	fetched map[int]struct{}
}

func newTaxonomyResolver(taxonomy string, opt Options) *taxonomyResolver {
	return &taxonomyResolver{
		taxonomy: taxonomy,
		opt:      opt,
		cache:    make(map[int]string),
		fetched:  make(map[int]struct{}),
	}
}

func (r *taxonomyResolver) Resolve(ctx context.Context, ids []int) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	unique := uniqueInts(ids)
	var missing []int

	r.mu.Lock()
	for _, id := range unique {
		if _, ok := r.cache[id]; !ok {
			missing = append(missing, id)
		}
	}
	r.mu.Unlock()

	if len(missing) > 0 {
		for _, chunk := range chunkInts(missing, 100) {
			data, err := fetchTaxonomyChunk(ctx, r.opt, r.taxonomy, chunk)
			if err != nil {
				return nil, err
			}
			r.mu.Lock()
			for id, name := range data {
				r.cache[id] = name
				r.fetched[id] = struct{}{}
			}
			for _, id := range chunk {
				if _, ok := data[id]; !ok {
					r.cache[id] = ""
					r.fetched[id] = struct{}{}
				}
			}
			r.mu.Unlock()
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := r.cache[id]; ok && name != "" {
			result = append(result, name)
		}
	}
	return result, nil
}

func (r *taxonomyResolver) ResolvedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.fetched)
}

func fetchTaxonomyChunk(ctx context.Context, opt Options, taxonomy string, ids []int) (map[int]string, error) {
	if len(ids) == 0 {
		return map[int]string{}, nil
	}
	endpoint, err := url.JoinPath(opt.BaseURL, "/wp-json/wp/v2/", taxonomy)
	if err != nil {
		return nil, err
	}
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	query := reqURL.Query()
	limit := len(ids)
	if limit > 100 {
		limit = 100
	}
	query.Set("per_page", strconv.Itoa(limit))
	query.Set("include", joinInts(ids))
	query.Set("_fields", "id,name")
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	setStandardHeaders(req, opt.UserAgent)

	resp, err := opt.Client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		payload, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("%s request failed: %s (%s)", taxonomy, resp.Status, string(payload))
	}

	var items []taxonomyItem
	if err := decodeJSON(resp.Body, &items); err != nil {
		resp.Body.Close()
		return nil, err
	}
	resp.Body.Close()

	result := make(map[int]string, len(items))
	for _, item := range items {
		result[item.ID] = item.Name
	}
	return result, nil
}

func uniqueInts(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
