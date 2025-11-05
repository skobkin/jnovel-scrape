package app

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/skobkin/jnovels-parser/internal/model"
)

const (
	defaultReqInterval = 600 * time.Millisecond
	defaultLimitWait   = 60 * time.Second
	defaultConcurrency = 4
	defaultMaxPages    = 2000
	defaultUserAgent   = "jnovels-scrape/1.0 (+https://example.com/contact)"
)

// Mode selects how the collector fetches content.
type Mode string

const (
	ModeAuto Mode = "auto"
	ModeAPI  Mode = "api"
	ModeHTML Mode = "html"
)

// GroupMode defines how posts are grouped before output.
type GroupMode string

const (
	GroupNone  GroupMode = "none"
	GroupTitle GroupMode = "title"
)

// GroupSort controls ordering within groups.
type GroupSort string

const (
	GroupSortAsc  GroupSort = "asc"
	GroupSortDesc GroupSort = "desc"
)

// Config represents the fully-parsed CLI configuration.
type Config struct {
	Cutoff       time.Time
	TypeFilters  map[model.PostType]struct{}
	TypeList     []model.PostType
	TitleFilter  string
	VolumeFilter *float64
	OutputPath   string
	MaxPages     int
	Concurrency  int
	ReqInterval  time.Duration
	LimitWait    time.Duration
	UserAgent    string
	Mode         Mode
	GroupMode    GroupMode
	GroupSort    GroupSort
}

// ParseArgs parses CLI flags into a Config.
func ParseArgs(args []string, output io.Writer) (Config, error) {
	var cfg Config

	fs := flag.NewFlagSet("jnovels-scrape", flag.ContinueOnError)
	if output != nil {
		fs.SetOutput(output)
	}

	var (
		until          string
		typeList       string
		titleFilter    string
		reqIntervalStr string
		limitWaitStr   string
		volumeStr      string
		maxPages       int
		concurrency    int
		outPath        string
		modeStr        string
		groupModeStr   string
		groupSortStr   string
	)

	fs.StringVar(&until, "until", "", "Cutoff date (YYYY-MM-DD). Required.")
	fs.StringVar(&typeList, "type", "", "Comma separated content types (epub,pdf,manga,unknown).")
	fs.StringVar(&typeList, "t", "", "Alias for --type.")
	fs.StringVar(&titleFilter, "title", "", "Case-insensitive title substring filter.")
	fs.StringVar(&titleFilter, "name", "", "Alias for --title.")
	fs.StringVar(&titleFilter, "n", "", "Alias for --title.")
	fs.StringVar(&volumeStr, "volume", "", "Filter by volume number (integer or decimal).")
	fs.StringVar(&volumeStr, "v", "", "Alias for --volume.")
	fs.StringVar(&outPath, "out", "", "Output path for Markdown (default stdout).")

	modeStr = string(ModeAuto)
	fs.StringVar(&modeStr, "mode", modeStr, "Fetch mode: auto, api, html.")

	reqIntervalStr = defaultReqInterval.String()
	limitWaitStr = defaultLimitWait.String()
	fs.StringVar(&reqIntervalStr, "req-interval", reqIntervalStr, "Delay between HTTP requests (time.ParseDuration).")
	fs.StringVar(&limitWaitStr, "limit-wait", limitWaitStr, "Delay when server rate limits without Retry-After.")

	maxPages = defaultMaxPages
	concurrency = defaultConcurrency
	fs.IntVar(&maxPages, "max-pages", maxPages, "Maximum number of pages to traverse (API or HTML).")
	fs.IntVar(&concurrency, "concurrency", concurrency, "Number of concurrent fetches for detail pages/taxonomies.")

	groupModeStr = string(GroupNone)
	groupSortStr = string(GroupSortAsc)
	fs.StringVar(&groupModeStr, "group", groupModeStr, "Grouping strategy (none,title).")
	fs.StringVar(&groupSortStr, "group-sort", groupSortStr, "Sort order within groups (asc,desc).")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if until == "" {
		return cfg, fmt.Errorf("--until is required")
	}
	cutoff, err := time.Parse("2006-01-02", until)
	if err != nil {
		return cfg, fmt.Errorf("invalid --until value: %w", err)
	}
	cfg.Cutoff = time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC)

	cfg.TitleFilter = strings.TrimSpace(titleFilter)
	cfg.OutputPath = strings.TrimSpace(outPath)

	reqInterval, err := time.ParseDuration(reqIntervalStr)
	if err != nil || reqInterval <= 0 {
		return cfg, fmt.Errorf("invalid --req-interval: %s", reqIntervalStr)
	}
	cfg.ReqInterval = reqInterval

	limitWait, err := time.ParseDuration(limitWaitStr)
	if err != nil || limitWait <= 0 {
		return cfg, fmt.Errorf("invalid --limit-wait: %s", limitWaitStr)
	}
	cfg.LimitWait = limitWait

	if maxPages <= 0 {
		return cfg, fmt.Errorf("--max-pages must be positive")
	}
	cfg.MaxPages = maxPages
	if concurrency <= 0 {
		return cfg, fmt.Errorf("--concurrency must be positive")
	}
	cfg.Concurrency = concurrency

	if typeList != "" {
		types, err := parseTypeList(typeList)
		if err != nil {
			return cfg, err
		}
		cfg.TypeFilters = make(map[model.PostType]struct{}, len(types))
		for _, t := range types {
			cfg.TypeFilters[t] = struct{}{}
		}
		cfg.TypeList = types
	} else {
		cfg.TypeFilters = make(map[model.PostType]struct{})
	}

	if strings.TrimSpace(volumeStr) != "" {
		value, err := strconv.ParseFloat(strings.TrimSpace(volumeStr), 64)
		if err != nil {
			return cfg, fmt.Errorf("invalid --volume value: %w", err)
		}
		cfg.VolumeFilter = &value
	}

	cfg.UserAgent = defaultUserAgent
	mode, err := parseMode(modeStr)
	if err != nil {
		return cfg, err
	}
	cfg.Mode = mode

	groupMode, err := parseGroupMode(groupModeStr)
	if err != nil {
		return cfg, err
	}
	cfg.GroupMode = groupMode

	groupSort, err := parseGroupSort(groupSortStr)
	if err != nil {
		return cfg, err
	}
	cfg.GroupSort = groupSort
	return cfg, nil
}

func parseTypeList(raw string) ([]model.PostType, error) {
	items := strings.Split(raw, ",")
	seen := make(map[model.PostType]struct{})
	var result []model.PostType
	for _, item := range items {
		token := strings.ToLower(strings.TrimSpace(item))
		if token == "" {
			continue
		}
		postType, ok := map[string]model.PostType{
			"epub":    model.TypeEPUB,
			"pdf":     model.TypePDF,
			"manga":   model.TypeManga,
			"unknown": model.TypeUnknown,
		}[token]
		if !ok {
			return nil, fmt.Errorf("invalid type %q (allowed: epub,pdf,manga,unknown)", token)
		}
		if _, exists := seen[postType]; exists {
			continue
		}
		seen[postType] = struct{}{}
		result = append(result, postType)
	}
	return result, nil
}

func parseMode(raw string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(ModeAuto):
		return ModeAuto, nil
	case string(ModeAPI):
		return ModeAPI, nil
	case string(ModeHTML):
		return ModeHTML, nil
	default:
		return "", fmt.Errorf("invalid --mode %q (expected auto, api, html)", raw)
	}
}

func parseGroupMode(raw string) (GroupMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(GroupNone):
		return GroupNone, nil
	case string(GroupTitle):
		return GroupTitle, nil
	default:
		return "", fmt.Errorf("invalid --group %q (expected none, title)", raw)
	}
}

func parseGroupSort(raw string) (GroupSort, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(GroupSortAsc):
		return GroupSortAsc, nil
	case string(GroupSortDesc):
		return GroupSortDesc, nil
	default:
		return "", fmt.Errorf("invalid --group-sort %q (expected asc, desc)", raw)
	}
}
