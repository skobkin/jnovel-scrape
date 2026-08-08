package app

import (
	"strings"
	"testing"
	"time"

	"github.com/knadh/koanf/v2"
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
	// Build a flat map keyed exactly as koanf would expose it after
	// defaults+env+flag providers are loaded. Both providers write
	// strings for scalar fields and []string for slice fields.
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
	if len(cfg.TitleFilters) != 2 || cfg.TitleFilters[0] != "dragon" || cfg.TitleFilters[1] != "spice" {
		t.Fatalf("TitleFilters: got %v", cfg.TitleFilters)
	}
	if cfg.Mode != ModeAPI {
		t.Fatalf("Mode: got %v", cfg.Mode)
	}
	if cfg.MaxPages != 10 {
		t.Fatalf("MaxPages: got %d", cfg.MaxPages)
	}
	if cfg.ReqInterval != 150*time.Millisecond {
		t.Fatalf("ReqInterval: got %v", cfg.ReqInterval)
	}
	// Cutoff is still zero here — date parsing is in parseRawConfig (Task 4).
	_ = cfg.Cutoff
}
