package app

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/skobkin/jnovels-parser/internal/collect"
	"github.com/skobkin/jnovels-parser/internal/httpx"
	"github.com/skobkin/jnovels-parser/internal/markdown"
	"github.com/skobkin/jnovels-parser/internal/model"
)

// Run executes the scraper using the provided configuration.
func Run(ctx context.Context, cfg Config, logger *Logger) error {
	if logger == nil {
		logger = NewLogger(os.Stderr)
	}

	client := httpx.NewClient(cfg.ReqInterval, cfg.LimitWait)

	options := collect.Options{
		BaseURL:     collect.DefaultBaseURL,
		MaxPages:    cfg.MaxPages,
		Concurrency: cfg.Concurrency,
		UserAgent:   cfg.UserAgent,
		Logger:      logger,
		Client:      client,
		ReqInterval: cfg.ReqInterval,
	}

	destination := "stdout"
	if cfg.OutputPath != "" {
		destination = cfg.OutputPath
	}
	logger.Infof("Starting crawl: cutoff=%s mode=%s out=%s", cfg.Cutoff.Format("2006-01-02"), cfg.Mode, destination)

	var (
		posts    model.Posts
		warnings []string
		err      error
	)

	switch cfg.Mode {
	case ModeAPI:
		posts, warnings, err = collect.FetchAPI(ctx, cfg.Cutoff, options)
		if err != nil {
			logger.Errorf("API mode failed: %v", err)
			return err
		}
		logger.Infof("API mode retrieved %d posts before filtering", len(posts))
	case ModeHTML:
		posts, warnings, err = collect.FetchHTML(ctx, cfg.Cutoff, options)
		if err != nil {
			logger.Errorf("HTML mode failed: %v", err)
			return err
		}
		logger.Infof("HTML mode retrieved %d posts before filtering", len(posts))
	case ModeAuto:
		posts, warnings, err = collect.FetchAPI(ctx, cfg.Cutoff, options)
		if err != nil {
			logger.Warnf("API mode failed (%v); switching to HTML fallback", err)
			posts, warnings, err = collect.FetchHTML(ctx, cfg.Cutoff, options)
			if err != nil {
				logger.Errorf("HTML fallback failed: %v", err)
				return err
			}
			logger.Infof("HTML fallback succeeded with %d posts before filtering", len(posts))
		} else {
			logger.Infof("API mode succeeded with %d posts before filtering", len(posts))
		}
	default:
		return fmt.Errorf("unsupported mode %q", cfg.Mode)
	}

	for _, warn := range warnings {
		logger.Warnf("%s", warn)
	}

	posts, removed := dedupePosts(posts)
	if removed > 0 {
		logger.Infof("Removed %d duplicate posts (by link)", removed)
	}

	filtered, stats := filterPosts(posts, cfg)
	logger.Infof("Filter stats: type=%d title=%d volume=%d", stats.TypeDropped, stats.TitleDropped, stats.VolumeDropped)
	filtered = applyGrouping(filtered, cfg.GroupMode, cfg.GroupSort)
	logger.Infof("Kept %d posts after filters", len(filtered))

	return writeOutput(cfg, filtered, logger)
}

func writeOutput(cfg Config, posts model.Posts, logger *Logger) error {
	var (
		writer io.Writer
		file   *os.File
		err    error
	)
	if cfg.OutputPath == "" {
		writer = os.Stdout
	} else {
		file, err = os.Create(cfg.OutputPath)
		if err != nil {
			return fmt.Errorf("open output path: %w", err)
		}
		defer file.Close()
		writer = file
	}

	if err := markdown.WriteTable(writer, cfg.Cutoff, posts); err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}

	if cfg.OutputPath == "" {
		logger.Infof("Wrote Markdown to stdout (%d rows)", len(posts))
	} else {
		logger.Infof("Wrote Markdown to %s (%d rows)", cfg.OutputPath, len(posts))
	}
	return nil
}

func dedupePosts(posts model.Posts) (model.Posts, int) {
	seen := make(map[string]struct{}, len(posts))
	var result model.Posts
	removed := 0
	for _, post := range posts {
		if _, ok := seen[post.Link]; ok {
			removed++
			continue
		}
		seen[post.Link] = struct{}{}
		result = append(result, post)
	}
	return result, removed
}

func applyGrouping(posts model.Posts, mode GroupMode, sortOrder GroupSort) model.Posts {
	if len(posts) == 0 {
		return posts
	}
	if mode != GroupTitle {
		return posts
	}
	type group struct {
		title string
		key   string
		list  model.Posts
	}
	groups := make(map[string]*group)
	for _, post := range posts {
		key := strings.ToLower(post.Title)
		g, ok := groups[key]
		if !ok {
			g = &group{title: post.Title, key: key}
			groups[key] = g
		}
		g.list = append(g.list, post)
	}
	ordered := make([]*group, 0, len(groups))
	for _, g := range groups {
		ordered = append(ordered, g)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		ii := strings.ToLower(ordered[i].title)
		jj := strings.ToLower(ordered[j].title)
		if ii == jj {
			return ordered[i].title < ordered[j].title
		}
		return ii < jj
	})
	result := make(model.Posts, 0, len(posts))
	for _, g := range ordered {
		sortGroupPosts(g.list, sortOrder)
		result = append(result, g.list...)
	}
	return result
}

func sortGroupPosts(posts model.Posts, order GroupSort) {
	sort.SliceStable(posts, func(i, j int) bool {
		return compareWithinGroup(posts[i], posts[j], order)
	})
}

func compareWithinGroup(a, b model.Post, order GroupSort) bool {
	av, hasA := volumeSortValue(a)
	bv, hasB := volumeSortValue(b)
	if hasA && hasB {
		if av != bv {
			if order == GroupSortDesc {
				return av > bv
			}
			return av < bv
		} else {
			if res, ok := compareVolumeExtra(a.VolumeExtra, b.VolumeExtra, order); ok {
				return res
			}
		}
	} else if hasA != hasB {
		// Items with a volume number always sort before those without.
		return hasA
	} else {
		if res, ok := compareVolumeExtra(a.VolumeExtra, b.VolumeExtra, order); ok {
			return res
		}
	}
	if !a.Date.Equal(b.Date) {
		return a.Date.After(b.Date)
	}
	if a.Type != b.Type {
		return string(a.Type) < string(b.Type)
	}
	return a.Link < b.Link
}

func volumeSortValue(post model.Post) (float64, bool) {
	if post.Volume == nil {
		return math.NaN(), false
	}
	return *post.Volume, true
}

func compareVolumeExtra(extraA, extraB string, order GroupSort) (bool, bool) {
	extraA = strings.TrimSpace(extraA)
	extraB = strings.TrimSpace(extraB)
	if extraA == "" && extraB == "" {
		return false, false
	}
	if extraA == "" && extraB != "" {
		// empty extras sort after non-empty regardless of order
		return false, true
	}
	if extraA != "" && extraB == "" {
		return true, true
	}
	aVal, aNum := parseVolumeExtraNumber(extraA)
	bVal, bNum := parseVolumeExtraNumber(extraB)
	if aNum && bNum && aVal != bVal {
		if order == GroupSortDesc {
			return aVal > bVal, true
		}
		return aVal < bVal, true
	}
	// fall back to lexical comparison
	if extraA == extraB {
		return false, false
	}
	if order == GroupSortDesc {
		return extraA > extraB, true
	}
	return extraA < extraB, true
}

func parseVolumeExtraNumber(extra string) (float64, bool) {
	fields := strings.Fields(strings.ToLower(extra))
	if len(fields) < 2 {
		return 0, false
	}
	value := fields[1]
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		return num, true
	}
	if romanValue, ok := romanToInt(value); ok {
		return float64(romanValue), true
	}
	return 0, false
}

func romanToInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	values := map[rune]int{
		'i': 1,
		'v': 5,
		'x': 10,
		'l': 50,
		'c': 100,
		'd': 500,
		'm': 1000,
	}
	total := 0
	prev := 0
	for _, r := range strings.ToLower(s) {
		val, ok := values[r]
		if !ok {
			return 0, false
		}
		if val > prev {
			total += val - 2*prev
		} else {
			total += val
		}
		prev = val
	}
	return total, true
}
