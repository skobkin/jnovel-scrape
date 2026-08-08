package app

import (
	"strings"
	"testing"
	"time"

	"github.com/knadh/koanf/v2"

	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
)

// TestKoanfIsImportable locks in that the koanf dependency is wired up.
// Removing the dependency should make this file fail to compile.
func TestKoanfIsImportable(t *testing.T) {
	// koanf.Koanf contains a map[string]any, so it cannot be compared
	// with ==. A nil check is enough to prove the type is in scope
	// and the zero value is constructable.
	var k *koanf.Koanf
	if k != nil {
		t.Fatalf("expected nil pointer to be nil; got %#v", k)
	}
	k = koanf.New(".")
	if k == nil {
		t.Fatalf("koanf.New returned nil")
	}
}

func TestConfigKeysAreNonEmpty(t *testing.T) {
	keys := configKeys()
	if len(keys) == 0 {
		t.Fatalf("configKeys() returned no keys")
	}
	for k, v := range keys {
		if k == "" {
			t.Fatalf("configKeys has an empty koanf key for env %q", v)
		}
		if v == "" {
			t.Fatalf("configKeys has an empty env-var suffix for key %q", k)
		}
		if strings.ContainsAny(k, " 	\n") {
			t.Fatalf("koanf key %q must not contain whitespace", k)
		}
	}
}

func TestConfigUnmarshalFromFlatMap(t *testing.T) {
	in := map[string]any{
		"until":        "2025-02-01",
		"type":         "epub,pdf",
		"title":        []string{"dragon", "spice"},
		"volume":       "3",
		"mode":         "api",
		"group":        "title",
		"group-sort":   "desc",
		"req-interval": "150ms",
		"limit-wait":   "300ms",
		"max-pages":    10,
		"concurrency":  2,
		"out":          "result.md",
	}

	cfg, err := unmarshalConfig(in)
	if err != nil {
		t.Fatalf("unmarshalConfig() unexpected error: %v", err)
	}

	if cfg.Cutoff.Format("2006-01-02") != "2025-02-01" {
		t.Fatalf("Cutoff: got %v", cfg.Cutoff)
	}
	if len(cfg.TypeList) != 2 || cfg.TypeList[0] != model.TypeEPUB || cfg.TypeList[1] != model.TypePDF {
		t.Fatalf("TypeList: got %v", cfg.TypeList)
	}
	if _, ok := cfg.TypeFilters[model.TypeEPUB]; !ok {
		t.Fatalf("TypeFilters missing epub: %v", cfg.TypeFilters)
	}
	if len(cfg.TitleFilters) != 2 || cfg.TitleFilters[0] != "dragon" || cfg.TitleFilters[1] != "spice" {
		t.Fatalf("TitleFilters: got %v", cfg.TitleFilters)
	}
	if cfg.VolumeFilter == nil || *cfg.VolumeFilter != 3 {
		t.Fatalf("VolumeFilter: got %v", cfg.VolumeFilter)
	}
	if cfg.OutputPath != "result.md" {
		t.Fatalf("OutputPath: got %q", cfg.OutputPath)
	}
	if cfg.Mode != ModeAPI {
		t.Fatalf("Mode: got %v", cfg.Mode)
	}
	if cfg.GroupMode != GroupTitle || cfg.GroupSort != GroupSortDesc {
		t.Fatalf("Group: got %v / %v", cfg.GroupMode, cfg.GroupSort)
	}
	if cfg.MaxPages != 10 {
		t.Fatalf("MaxPages: got %d", cfg.MaxPages)
	}
	if cfg.Concurrency != 2 {
		t.Fatalf("Concurrency: got %d", cfg.Concurrency)
	}
	if cfg.ReqInterval != 150*time.Millisecond {
		t.Fatalf("ReqInterval: got %v", cfg.ReqInterval)
	}
	if cfg.LimitWait != 300*time.Millisecond {
		t.Fatalf("LimitWait: got %v", cfg.LimitWait)
	}
}

func TestParseArgsThroughKoanf_AllFlags(t *testing.T) {
	args := []string{
		"--until", "2025-02-01",
		"--type", "epub,pdf",
		"--title", "dragon",
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
		t.Fatalf("Cutoff: got %v", cfg.Cutoff)
	}
	if len(cfg.TitleFilters) != 1 || cfg.TitleFilters[0] != "dragon" {
		t.Fatalf("TitleFilters: got %v", cfg.TitleFilters)
	}
	if cfg.VolumeFilter == nil || *cfg.VolumeFilter != 3 {
		t.Fatalf("VolumeFilter: got %v", cfg.VolumeFilter)
	}
	if cfg.Mode != ModeAPI {
		t.Fatalf("Mode: got %v", cfg.Mode)
	}
}

func TestParseArgsThroughKoanf_MissingUntilFails(t *testing.T) {
	if _, err := ParseArgs([]string{"--type", "epub"}, nil); err == nil {
		t.Fatalf("expected error when --until is missing")
	}
}

func TestParseArgsEnvOverridesDefaults(t *testing.T) {
	t.Setenv("JNOVEL_UNTIL", "2025-03-15")
	t.Setenv("JNOVEL_TYPE", "manga")
	t.Setenv("JNOVEL_MODE", "html")
	t.Setenv("JNOVEL_MAX_PAGES", "7")

	cfg, err := ParseArgs(nil, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if cfg.Cutoff.Format("2006-01-02") != "2025-03-15" {
		t.Fatalf("Cutoff: got %v", cfg.Cutoff)
	}
	if len(cfg.TypeList) != 1 || cfg.TypeList[0] != model.TypeManga {
		t.Fatalf("TypeList: got %v", cfg.TypeList)
	}
	if cfg.Mode != ModeHTML {
		t.Fatalf("Mode: got %v", cfg.Mode)
	}
	if cfg.MaxPages != 7 {
		t.Fatalf("MaxPages: got %d", cfg.MaxPages)
	}
}

func TestParseArgsFlagOverridesEnv(t *testing.T) {
	t.Setenv("JNOVEL_UNTIL", "2025-03-15")
	t.Setenv("JNOVEL_MODE", "html")

	cfg, err := ParseArgs([]string{"--mode", "api"}, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	// --until comes from env (no CLI override), so it's preserved.
	if cfg.Cutoff.Format("2006-01-02") != "2025-03-15" {
		t.Fatalf("Cutoff: got %v", cfg.Cutoff)
	}
	// --mode is overridden by the CLI flag, so env yields.
	if cfg.Mode != ModeAPI {
		t.Fatalf("Mode: got %v, want api", cfg.Mode)
	}
}

func TestParseArgsEnvTitleCommaSplit(t *testing.T) {
	t.Setenv("JNOVEL_UNTIL", "2025-03-15")
	t.Setenv("JNOVEL_TITLE", "dragon,spice")

	cfg, err := ParseArgs(nil, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if len(cfg.TitleFilters) != 2 || cfg.TitleFilters[0] != "dragon" || cfg.TitleFilters[1] != "spice" {
		t.Fatalf("TitleFilters: got %v", cfg.TitleFilters)
	}
}
