package app

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	koanfbasicflag "github.com/knadh/koanf/providers/basicflag"
	koanfconfmap "github.com/knadh/koanf/providers/confmap"
	koanfenv "github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"

	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
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
	// ModeAuto lets the scraper prefer the API and fall back to HTML.
	ModeAuto Mode = "auto"
	// ModeAPI forces the WordPress REST API only.
	ModeAPI Mode = "api"
	// ModeHTML forces HTML scraping only.
	ModeHTML Mode = "html"
)

// GroupMode defines how posts are grouped before output.
type GroupMode string

const (
	// GroupNone keeps the default flat ordering.
	GroupNone GroupMode = "none"
	// GroupTitle groups posts by case-folded title.
	GroupTitle GroupMode = "title"
)

// GroupSort controls ordering within groups.
type GroupSort string

const (
	// GroupSortAsc sorts items within a group ascending.
	GroupSortAsc GroupSort = "asc"
	// GroupSortDesc sorts items within a group descending.
	GroupSortDesc GroupSort = "desc"
)

// Config represents the fully-parsed CLI configuration.
//
// Field names are part of the package's public API and are referenced by
// the rest of internal/app (filter.go, run.go) and by every test. Do not
// rename; add a `koanf:"<key>"` tag to make the field unmarshallable
// from a flat koanf instance. Use `koanf:"-"` to skip a field.
type Config struct {
	Cutoff       time.Time                   `koanf:"-"`
	TypeFilters  map[model.PostType]struct{} `koanf:"-"`
	TypeList     []model.PostType            `koanf:"type"`
	TitleFilters []string                    `koanf:"title"`
	VolumeFilter *float64                    `koanf:"volume"`
	OutputPath   string                      `koanf:"out"`
	MaxPages     int                         `koanf:"max-pages"`
	Concurrency  int                         `koanf:"concurrency"`
	ReqInterval  time.Duration               `koanf:"req-interval"`
	LimitWait    time.Duration               `koanf:"limit-wait"`
	UserAgent    string                      `koanf:"-"`
	Mode         Mode                        `koanf:"mode"`
	GroupMode    GroupMode                   `koanf:"group"`
	GroupSort    GroupSort                   `koanf:"group-sort"`
}

// ParseArgs parses CLI flags into a Config.
//
// It is preserved as the public entry point for callers in
// cmd/jnovels-scrape. Internally it delegates to loadConfig, which
// builds a koanf.Koanf instance from defaults → env → CLI flags and
// unmarshals it into a Config.
func ParseArgs(args []string, output io.Writer) (Config, error) {
	fs := flag.NewFlagSet("jnovels-scrape", flag.ContinueOnError)
	if output != nil {
		fs.SetOutput(output)
	}

	return loadConfig(fs, args)
}

// loadConfig binds CLI flags to the FlagSet, then layers
// defaults → env → flags into a single koanf.Koanf instance and
// unmarshals it into a Config. Later layers override earlier ones,
// matching the order described in the issue body.
//
// The CLI provider is basicflag (Go stdlib flag), so --help keeps
// the standard Go help format. Aliases (-t, -n, -v, --name) are
// handled by binding multiple flag names to the same underlying
// *string or *[]string via stdlib flag (which already supports this).
// The basicflag callback (below) remaps flag names to canonical
// koanf keys.
func loadConfig(fs *flag.FlagSet, args []string) (Config, error) {
	keys := configKeys()

	// 1. Defaults via confmap. All defaults are stringified so the
	// confmap provider can write them into the flat koanf namespace;
	// strongly-typed fields (Mode, GroupMode, etc.) and the numeric /
	// duration fields round-trip through koanf.Unmarshal.
	defaults := map[string]any{
		keys["mode"]:         string(ModeAuto),
		keys["group"]:        string(GroupNone),
		keys["group-sort"]:   string(GroupSortAsc),
		keys["req-interval"]: defaultReqInterval.String(),
		keys["limit-wait"]:   defaultLimitWait.String(),
		keys["max-pages"]:    strconv.Itoa(defaultMaxPages),
		keys["concurrency"]:  strconv.Itoa(defaultConcurrency),
	}

	// 2. Bind CLI flags. Aliases share a single *string variable;
	// stdlib flag.StringVar already supports this.
	fs.String("until", "", "Cutoff date (YYYY-MM-DD). Required.")

	typePtr := fs.String("type", "", "Comma separated content types (epub,pdf,manga,unknown).")
	fs.String("t", *typePtr, "Alias for --type.")

	titlePtr := stringListFlag(fs, "title", "Case-insensitive title substring filter; may be repeated or comma-separated.")
	stringListFlagAlias(fs, titlePtr, "name", "Alias for --title; may be repeated or comma-separated.")
	stringListFlagAlias(fs, titlePtr, "n", "Alias for --title; may be repeated or comma-separated.")

	volumePtr := fs.String("volume", "", "Filter by volume number (integer or decimal).")
	fs.String("v", *volumePtr, "Alias for --volume.")

	fs.String("out", "", "Output path for Markdown (default stdout).")
	fs.String("mode", defaults[keys["mode"]].(string), "Fetch mode: auto, api, html.")
	fs.String("group", defaults[keys["group"]].(string), "Grouping strategy (none,title).")
	fs.String("group-sort", defaults[keys["group-sort"]].(string), "Sort order within groups (asc,desc).")
	fs.String("req-interval", defaults[keys["req-interval"]].(string), "Delay between HTTP requests (time.ParseDuration).")
	fs.String("limit-wait", defaults[keys["limit-wait"]].(string), "Delay when server rate limits without Retry-After.")
	fs.String("max-pages", defaults[keys["max-pages"]].(string), "Maximum number of pages to traverse (API or HTML).")
	fs.String("concurrency", defaults[keys["concurrency"]].(string), "Number of concurrent fetches for detail pages/taxonomies.")

	// 3. Parse the CLI args. ContinueOnError is set on the FlagSet
	// already by ParseArgs; flag.Parse returns an error which we
	// forward.
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	// 4. Build the koanf instance and load each layer in order.
	k := koanf.New(".")
	if err := k.Load(koanfconfmap.Provider(defaults, "."), nil); err != nil {
		return Config{}, fmt.Errorf("load defaults: %w", err)
	}
	if err := k.Load(koanfenv.Provider(envPrefix, ".", func(suffix string) string {
		return envKey(suffix, keys)
	}), nil); err != nil {
		return Config{}, fmt.Errorf("load env: %w", err)
	}
	if err := k.Load(koanfbasicflag.ProviderWithValue(fs, ".", func(name, value string) (string, any) {
		return remapFlagName(name, keys), value
	}, k), nil); err != nil {
		return Config{}, fmt.Errorf("load flags: %w", err)
	}

	// 5. Unmarshal into the strongly-typed Config and post-process.
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal: %w", err)
	}

	cfg, err := parseRawConfig(k, cfg)
	if err != nil {
		return Config{}, err
	}

	cfg.UserAgent = defaultUserAgent

	return cfg, nil
}

// envKey maps an env-var name (e.g. "JN_TYPE") to a koanf key
// (e.g. "type") using the keys registry. The env provider in
// v1.1.0 passes the full name (including the prefix) to the
// callback, so we strip the prefix before lookup.
func envKey(fullName string, keys map[string]string) string {
	suffix := strings.TrimPrefix(fullName, envPrefix)
	for k, v := range keys {
		if v == suffix {
			return k
		}
	}

	return strings.ToLower(suffix)
}

// remapFlagName maps a CLI flag name to its canonical koanf key.
// Aliases (-t, -n, -v, --name) all collapse to the same key as
// their canonical name. Unknown flag names pass through unchanged.
func remapFlagName(name string, _ map[string]string) string {
	alias := map[string]string{
		"t":    "type",
		"n":    "title",
		"name": "title",
		"v":    "volume",
	}
	if canonical, ok := alias[name]; ok {
		return canonical
	}

	return name
}

// stringListFlag is a tiny helper that registers a flag that
// accumulates repeated values into a []string. It returns a
// *stringListValue so the caller can read the slice after parsing.
//
// The returned flag.Value's String() method returns a comma-joined
// representation of the accumulated values. This is what basicflag
// sees when it calls f.Value.String(); the comma form is then split
// again in parseRawConfig so that --title "dragon,spice" and
// --title "dragon" --title "spice" both work uniformly.
func stringListFlag(fs *flag.FlagSet, name, usage string) *stringListValue {
	v := &stringListValue{}
	fs.Var(v, name, usage)

	return v
}

// stringListFlagAlias registers an additional flag name that
// appends to the same underlying *stringListValue. Used for
// --name/-n aliases of --title.
func stringListFlagAlias(fs *flag.FlagSet, ptr *stringListValue, name, usage string) {
	fs.Var(ptr, name, usage)
}

// stringListValue implements flag.Value. It accumulates values set
// via Set() into a slice and exposes them via Values(). String()
// returns a comma-joined representation so the basicflag provider
// can read a single string out via f.Value.String() during Read().
type stringListValue struct {
	values []string
}

func (s *stringListValue) Set(value string) error {
	s.values = append(s.values, value)

	return nil
}

func (s *stringListValue) String() string {
	return strings.Join(s.values, ",")
}

func (s *stringListValue) Values() []string {
	return s.values
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

// configKeys returns the canonical (koanf key → env-var suffix) mapping for
// every configuration field that can be set from defaults, env, or CLI flags.
// Keeping the mapping in one place guarantees that the three sources stay in
// sync: changing a key here propagates to all three providers and to the
// struct tags used by koanf.Unmarshal.
func configKeys() map[string]string {
	return map[string]string{
		"until":        "UNTIL",
		"type":         "TYPE",
		"title":        "TITLE",
		"volume":       "VOLUME",
		"mode":         "MODE",
		"group":        "GROUP",
		"group-sort":   "GROUP_SORT",
		"req-interval": "REQ_INTERVAL",
		"limit-wait":   "LIMIT_WAIT",
		"max-pages":    "MAX_PAGES",
		"concurrency":  "CONCURRENCY",
		"out":          "OUT",
	}
}

// envPrefix is prepended to every env-var name. Fixed for now; if a
// configurable prefix is ever needed, expose it as a build flag.
const envPrefix = "JN_"

// unmarshalConfig fills a Config from a flat map. It is the one place
// where the layered koanf instance is converted into a Config.
//
// The map's value types follow koanf's conventions: scalars arrive as
// strings, slices arrive as []string. Parsing of those raw values into
// the strongly-typed Config fields (cutoff date, durations, enum-typed
// modes, etc.) happens in parseRawConfig, which this function calls
// after unmarshalling.
func unmarshalConfig(raw map[string]any) (Config, error) {
	k := koanf.New(".")
	if err := k.Load(koanfconfmap.Provider(raw, "."), nil); err != nil {
		return Config{}, fmt.Errorf("load raw config: %w", err)
	}
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return parseRawConfig(k, cfg)
}

// parseRawConfig post-processes the unmarshalled Config. It parses raw
// string values (cutoff dates, durations, enum-typed modes, comma-
// separated type lists, optional volume filter) and validates that
// required fields are present and that numeric ranges are positive.
//
// Reading from the koanf instance via k.String() (not the unmarshalled
// struct) is intentional: it gives us the raw string the user supplied,
// which is what we want to surface in error messages like
// "invalid --req-interval: %s". The unmarshalled struct is used for
// the values that unmarshalling already handled correctly (numeric
// fields, durations, enum-typed fields that don't need extra
// validation).
//
// Behaviour parity with the pre-koanf ParseArgs:
//   - --until is required.
//   - --volume is optional; an empty string leaves VolumeFilter as nil.
//   - --type is optional; an empty string leaves TypeList and
//     TypeFilters as empty.
//   - --title accepts a slice (from basicflag/stringListFlag) and
//     trims/dedups each element. The pre-koanf code also split
//     single values on commas; that responsibility now lives here
//     too, so `--title "dragon,spice"` and
//     `--title "dragon" --title "spice"` both work.
//   - --req-interval and --limit-wait must be valid durations > 0.
//   - --max-pages and --concurrency must be positive.
//   - --mode, --group, --group-sort accept the same set of values.
func parseRawConfig(k *koanf.Koanf, cfg Config) (Config, error) {
	// --until
	until := k.String("until")
	if until == "" {
		return cfg, fmt.Errorf("--until is required")
	}
	cutoff, err := time.Parse("2006-01-02", until)
	if err != nil {
		return cfg, fmt.Errorf("invalid --until value: %w", err)
	}
	cfg.Cutoff = time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC)

	// --type
	if typeRaw := k.String("type"); typeRaw != "" {
		types, err := parseTypeList(typeRaw)
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

	// --title: each element from the slice may itself be comma-
	// separated. The pre-koanf code did strings.Join + strings.Split,
	// which had the same effect. We iterate, split, trim, and skip
	// empties so `--title " dragon , spice "` produces ["dragon", "spice"].
	var titles []string
	for _, raw := range cfg.TitleFilters {
		for _, part := range strings.Split(raw, ",") {
			if t := strings.TrimSpace(part); t != "" {
				titles = append(titles, t)
			}
		}
	}
	cfg.TitleFilters = titles

	// --volume
	//
	// Reset cfg.VolumeFilter to nil first: mapstructure zero-initialises
	// the *float64 field during koanf.Unmarshal even when the key is
	// absent. A non-nil VolumeFilter pointing at 0.0 silently filters
	// out every post that lacks a parsed volume — which is the vast
	// majority of posts — making the CLI look like it returns nothing
	// when in fact everything is being dropped by a phantom volume=0
	// filter. We always start from nil and only set it when the user
	// actually passed --volume / JN_VOLUME.
	cfg.VolumeFilter = nil
	if v := strings.TrimSpace(k.String("volume")); v != "" {
		value, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return cfg, fmt.Errorf("invalid --volume value: %w", err)
		}
		cfg.VolumeFilter = &value
	}

	// --req-interval / --limit-wait
	if cfg.ReqInterval <= 0 {
		return cfg, fmt.Errorf("invalid --req-interval: %s", k.String("req-interval"))
	}
	if cfg.LimitWait <= 0 {
		return cfg, fmt.Errorf("invalid --limit-wait: %s", k.String("limit-wait"))
	}

	// --max-pages / --concurrency
	if cfg.MaxPages <= 0 {
		return cfg, fmt.Errorf("--max-pages must be positive")
	}
	if cfg.Concurrency <= 0 {
		return cfg, fmt.Errorf("--concurrency must be positive")
	}

	// --mode / --group / --group-sort
	mode, err := parseMode(k.String("mode"))
	if err != nil {
		return cfg, err
	}
	cfg.Mode = mode

	groupMode, err := parseGroupMode(k.String("group"))
	if err != nil {
		return cfg, err
	}
	cfg.GroupMode = groupMode

	groupSort, err := parseGroupSort(k.String("group-sort"))
	if err != nil {
		return cfg, err
	}
	cfg.GroupSort = groupSort

	return cfg, nil
}
