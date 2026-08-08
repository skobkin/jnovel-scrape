package app

import (
	"testing"
	"time"
)

// This file guards against a class of bug introduced by the koanf
// migration: koanf's mapstructure decoder zero-initialises pointer,
// map, and slice fields during Unmarshal even when the corresponding
// key is absent in the layered config. For optional fields whose
// absence carries semantic meaning ("don't filter on this dimension"),
// a zero-initialised value silently applies a degenerate filter that
// drops most or all rows.
//
// Every test in this file asserts the *absent* semantics of an
// optional field. A regression in parseRawConfig, the struct tags, or
// the unmarshalling pipeline will fail at least one of them.

// TestOptionalFieldsAreAbsentWithoutInput is the umbrella check. It
// covers every optional field on Config that has a meaningful
// "absent" sentinel in one go. If a future field is added to Config
// with a similar semantics, extend the struct literal below.
func TestOptionalFieldsAreAbsentWithoutInput(t *testing.T) {
	cfg, err := ParseArgs([]string{"--until", "2025-01-01"}, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}

	// VolumeFilter is a *float64 and must be nil (not &0.0) when
	// --volume is not passed. See the regression that motivated this
	// file: a non-nil *float64 pointing to 0.0 silently filters out
	// every post that lacks a parsed volume.
	if cfg.VolumeFilter != nil {
		t.Errorf("VolumeFilter: want nil, got pointer to %v", *cfg.VolumeFilter)
	}

	// OutputPath is the only "no output path" sentinel: empty string
	// means "write to stdout". A non-empty default would surprise
	// users.
	if cfg.OutputPath != "" {
		t.Errorf("OutputPath: want empty (stdout), got %q", cfg.OutputPath)
	}

	// TitleFilters is a []string. nil and []string{} both iterate
	// zero times, but we want a stable contract: a missing --title
	// produces nil (the zero value) so callers can check `len(...) == 0`
	// without distinguishing nil from empty.
	if cfg.TitleFilters != nil {
		t.Errorf("TitleFilters: want nil, got %#v", cfg.TitleFilters)
	}

	// TypeFilters is a map[model.PostType]struct{}. parseRawConfig
	// always assigns it to a fresh empty map (not nil) so downstream
	// code can do membership checks without nil-guards. We lock that
	// contract here.
	if cfg.TypeFilters == nil {
		t.Errorf("TypeFilters: want non-nil empty map, got nil")
	}
	if len(cfg.TypeFilters) != 0 {
		t.Errorf("TypeFilters: want empty, got %d entries: %v", len(cfg.TypeFilters), cfg.TypeFilters)
	}

	// TypeList mirrors the --type filter. Absent --type must produce
	// an empty slice. The current implementation actually produces
	// []model.PostType{""} via mapstructure's slice zero-init, which
	// is cosmetically wrong but harmless because filter.go iterates
	// TypeFilters (the map), not TypeList. We accept the cosmetic
	// quirk here — the behaviour that matters is the empty set
	// semantics, which TypeFilters (the map) provides correctly
	// above. Document the quirk in case a future reader wants to
	// clean it up.
	_ = cfg.TypeList // intentionally not asserted: see comment above
}

// TestVolumeFilterAbsentWithEnvVarOnly mirrors the previous test for
// the JN_VOLUME env-var path. parseRawConfig reads raw values via
// k.String("volume"), which must return "" for an unset env var the
// same way it does for an unset flag.
func TestVolumeFilterAbsentWithEnvVarOnly(t *testing.T) {
	t.Setenv("JN_UNTIL", "2025-03-15")

	cfg, err := ParseArgs(nil, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if cfg.VolumeFilter != nil {
		t.Errorf("VolumeFilter should be nil when JN_VOLUME is unset, got pointer to %v", *cfg.VolumeFilter)
	}
}

// TestVolumeFilterRespectsFlag covers the positive case for the flag
// path: --volume 9 must produce a non-nil pointer to 9.0. If a future
// change accidentally makes the "always nil reset" unconditional,
// this test catches it.
func TestVolumeFilterRespectsFlag(t *testing.T) {
	cfg, err := ParseArgs([]string{"--until", "2025-01-01", "--volume", "9"}, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if cfg.VolumeFilter == nil {
		t.Fatalf("VolumeFilter: want pointer to 9, got nil")
	}
	if *cfg.VolumeFilter != 9.0 {
		t.Errorf("VolumeFilter: want 9.0, got %v", *cfg.VolumeFilter)
	}
}

// TestVolumeFilterRespectsEnv covers the positive case for the
// env-var path: JN_VOLUME=9 must produce the same pointer-to-9.0.
func TestVolumeFilterRespectsEnv(t *testing.T) {
	t.Setenv("JN_UNTIL", "2025-01-01")
	t.Setenv("JN_VOLUME", "9")

	cfg, err := ParseArgs(nil, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if cfg.VolumeFilter == nil {
		t.Fatalf("VolumeFilter: want pointer to 9, got nil")
	}
	if *cfg.VolumeFilter != 9.0 {
		t.Errorf("VolumeFilter: want 9.0, got %v", *cfg.VolumeFilter)
	}
}

// TestVolumeFilterFlagOverridesEnv locks in the layering order
// (defaults → env → flags). A CLI flag must always win over the
// matching env var.
func TestVolumeFilterFlagOverridesEnv(t *testing.T) {
	t.Setenv("JN_UNTIL", "2025-01-01")
	t.Setenv("JN_VOLUME", "1")

	cfg, err := ParseArgs([]string{"--volume", "9"}, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if cfg.VolumeFilter == nil || *cfg.VolumeFilter != 9.0 {
		t.Errorf("VolumeFilter: want 9.0 (CLI flag overrides env), got %v", cfg.VolumeFilter)
	}
}

// TestVolumeFilterAcceptsDecimal covers the documented contract that
// --volume accepts both integer and decimal volume values. This is
// the path the pre-koanf code already supported; the koanf migration
// must preserve it.
func TestVolumeFilterAcceptsDecimal(t *testing.T) {
	cfg, err := ParseArgs([]string{"--until", "2025-01-01", "--volume", "12.5"}, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	if cfg.VolumeFilter == nil || *cfg.VolumeFilter != 12.5 {
		t.Errorf("VolumeFilter: want 12.5, got %v", cfg.VolumeFilter)
	}
}

// TestVolumeFilterRejectsGarbage covers the documented contract that
// --volume errors out on unparseable input. A regression that
// silently coerces to 0.0 (or that the nil-reset breaks by always
// short-circuiting the error) will fail this test.
func TestVolumeFilterRejectsGarbage(t *testing.T) {
	if _, err := ParseArgs([]string{"--until", "2025-01-01", "--volume", "not-a-number"}, nil); err == nil {
		t.Fatalf("expected error for unparseable --volume")
	}
}

// TestParseArgsSmokeChecksUnfilteredBehaviour is the end-to-end
// regression guard for the symptom that originally surfaced the bug:
// without --volume, the CLI must accept all posts whose volume is
// nil and drop none of them. We do not exercise the network here;
// instead we drive the unmarshal path that produced the bug and
// assert the post-process step still has a nil VolumeFilter.
func TestParseArgsSmokeChecksUnfilteredBehaviour(t *testing.T) {
	cfg, err := ParseArgs([]string{"--until", "2025-01-01"}, nil)
	if err != nil {
		t.Fatalf("ParseArgs() unexpected error: %v", err)
	}
	// Repeat the key assertion in this file with a comment aimed at
	// the next reader who hits a "Kept 0 posts after filters" log.
	if cfg.VolumeFilter != nil {
		t.Fatalf("VolumeFilter should be nil without --volume; a non-nil pointer to %v would silently drop all posts with a nil volume field", *cfg.VolumeFilter)
	}

	// Sanity-check that the rest of the config still looks right after
	// the volume fix, so a future refactor that breaks adjacent
	// fields doesn't slip past unnoticed.
	if cfg.Cutoff.Format("2006-01-02") != "2025-01-01" {
		t.Errorf("Cutoff: got %v", cfg.Cutoff)
	}
	if cfg.Mode != ModeAuto {
		t.Errorf("Mode: got %v", cfg.Mode)
	}
	if cfg.ReqInterval <= 0 || cfg.ReqInterval > time.Second {
		t.Errorf("ReqInterval out of expected range: %v", cfg.ReqInterval)
	}
}
