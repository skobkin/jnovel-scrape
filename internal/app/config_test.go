package app

import (
	"strings"
	"testing"
	"time"

	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
)

func TestParseMode(t *testing.T) {
	cases := []struct {
		input string
		want  Mode
		ok    bool
	}{
		{"auto", ModeAuto, true},
		{"API", ModeAPI, true},
		{"html", ModeHTML, true},
		{"invalid", "", false},
	}

	for _, tc := range cases {
		got, err := parseMode(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("parseMode(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("parseMode(%q) expected error", tc.input)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("parseMode(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseGroupMode(t *testing.T) {
	cases := []struct {
		input string
		want  GroupMode
		ok    bool
	}{
		{"none", GroupNone, true},
		{"TITLE", GroupTitle, true},
		{"invalid", "", false},
	}

	for _, tc := range cases {
		got, err := parseGroupMode(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("parseGroupMode(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("parseGroupMode(%q) expected error", tc.input)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("parseGroupMode(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseGroupSort(t *testing.T) {
	cases := []struct {
		input string
		want  GroupSort
		ok    bool
	}{
		{"asc", GroupSortAsc, true},
		{"DESC", GroupSortDesc, true},
		{"invalid", "", false},
	}
	for _, tc := range cases {
		got, err := parseGroupSort(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("parseGroupSort(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("parseGroupSort(%q) expected error", tc.input)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("parseGroupSort(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseTitleMode(t *testing.T) {
	cases := []struct {
		input string
		want  TitleMode
		ok    bool
	}{
		{"substring", TitleModeSubstring, true},
		{"SUBSTRING", TitleModeSubstring, true},
		{"word", TitleModeWord, true},
		{"WORD", TitleModeWord, true},
		// Empty string is treated as "use the confmap default"
		// rather than rejected, so a user can unset a
		// previously-set JN_TITLE_MODE without causing the run
		// to fail.
		{"", TitleModeSubstring, true},
		{"prefix", "", false},
		{"regex", "", false},
		{"exact", "", false},
	}
	for _, tc := range cases {
		got, err := parseTitleMode(tc.input)
		if tc.ok && err != nil {
			t.Fatalf("parseTitleMode(%q) unexpected error: %v", tc.input, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("parseTitleMode(%q) expected error", tc.input)
		}
		if tc.ok && got != tc.want {
			t.Fatalf("parseTitleMode(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseArgsSuccess(t *testing.T) {
	args := []string{
		"--until", "2025-02-01",
		"--type", "epub,pdf",
		"--title", "dragon",
		"--title-mode", "word",
		"--volume", "3",
		"--out", "result.md",
		"--mode", "api",
		"--group", "title",
		"--group-sort", "desc",
		"--max-pages", "10",
		"--concurrency", "2",
		"--req-interval", "150ms",
		"--limit-wait", "300ms",
	}

	cfg, err := ParseArgs(args, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}

	if cfg.Cutoff.Format("2006-01-02") != "2025-02-01" {
		t.Fatalf("unexpected cutoff: %v", cfg.Cutoff)
	}
	if len(cfg.TypeList) != 2 || cfg.TypeList[0] != model.TypeEPUB || cfg.TypeList[1] != model.TypePDF {
		t.Fatalf("unexpected type list: %+v", cfg.TypeList)
	}
	if len(cfg.TitleFilters) != 1 || cfg.TitleFilters[0] != "dragon" {
		t.Fatalf("unexpected title filters: %v", cfg.TitleFilters)
	}
	if cfg.TitleMode != TitleModeWord {
		t.Fatalf("unexpected title mode: %s", cfg.TitleMode)
	}
	if cfg.VolumeFilter == nil || *cfg.VolumeFilter != 3 {
		t.Fatalf("unexpected volume filter: %v", cfg.VolumeFilter)
	}
	if cfg.OutputPath != "result.md" {
		t.Fatalf("unexpected output path: %q", cfg.OutputPath)
	}
	if cfg.Mode != ModeAPI {
		t.Fatalf("unexpected mode: %s", cfg.Mode)
	}
	if cfg.GroupMode != GroupTitle || cfg.GroupSort != GroupSortDesc {
		t.Fatalf("unexpected grouping: %s / %s", cfg.GroupMode, cfg.GroupSort)
	}
	if cfg.MaxPages != 10 {
		t.Fatalf("unexpected max pages: %d", cfg.MaxPages)
	}
	if cfg.Concurrency != 2 {
		t.Fatalf("unexpected concurrency: %d", cfg.Concurrency)
	}
	if cfg.ReqInterval != 150*time.Millisecond {
		t.Fatalf("unexpected req interval: %s", cfg.ReqInterval)
	}
	if cfg.LimitWait != 300*time.Millisecond {
		t.Fatalf("unexpected limit wait: %s", cfg.LimitWait)
	}
}

// TestParseArgsTitleModeDefault ensures the default is substring
// (full back-compat) when no --title-mode flag or env var is set.
func TestParseArgsTitleModeDefault(t *testing.T) {
	cfg, err := ParseArgs([]string{"--until", "2025-02-01"}, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if cfg.TitleMode != TitleModeSubstring {
		t.Fatalf("default title mode: got %s, want %s", cfg.TitleMode, TitleModeSubstring)
	}
}

// TestParseArgsTitleModeEnv covers the JN_TITLE_MODE env var
// path, including the empty-string unset case which must NOT
// fail (parseTitleMode returns the confmap default in that
// case).
func TestParseArgsTitleModeEnv(t *testing.T) {
	t.Setenv("JN_TITLE_MODE", "word")
	cfg, err := ParseArgs([]string{"--until", "2025-02-01"}, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if cfg.TitleMode != TitleModeWord {
		t.Fatalf("JN_TITLE_MODE=word: got %s, want %s", cfg.TitleMode, TitleModeWord)
	}
}

// TestParseArgsTitleModeInvalid covers the error path: a value
// that is neither substring nor word (and not empty) must be
// rejected at parse time, with a message that names the flag.
func TestParseArgsTitleModeInvalid(t *testing.T) {
	_, err := ParseArgs([]string{"--until", "2025-02-01", "--title-mode", "exact"}, nil)
	if err == nil {
		t.Fatalf("expected error for invalid --title-mode value")
	}
	if !strings.Contains(err.Error(), "--title-mode") {
		t.Fatalf("expected error to mention --title-mode, got: %v", err)
	}
}

// TestParseArgsTitleModeFlagOverridesEnv verifies the layering
// rule: defaults -> env -> flags, so a flag value wins over an
// env value.
func TestParseArgsTitleModeFlagOverridesEnv(t *testing.T) {
	t.Setenv("JN_TITLE_MODE", "word")
	cfg, err := ParseArgs([]string{
		"--until", "2025-02-01",
		"--title-mode", "substring",
	}, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if cfg.TitleMode != TitleModeSubstring {
		t.Fatalf("flag should override env: got %s, want %s", cfg.TitleMode, TitleModeSubstring)
	}
}

func TestParseArgsMissingUntil(t *testing.T) {
	if _, err := ParseArgs([]string{"--type", "epub"}, nil); err == nil {
		t.Fatalf("expected error when --until is missing")
	}
}
