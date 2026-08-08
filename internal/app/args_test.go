package app

import (
	"testing"
)

func TestParseArgsMultipleTitleFilters(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "repeated flag",
			args: []string{"--until", "2025-02-01", "--title", "dragon", "--title", "spice"},
			want: []string{"dragon", "spice"},
		},
		{
			name: "comma separated",
			args: []string{"--until", "2025-02-01", "--title", "dragon,spice and wolf"},
			want: []string{"dragon", "spice and wolf"},
		},
		{
			name: "name alias repeats",
			args: []string{"--until", "2025-02-01", "--name", "dragon", "--name", "spice"},
			want: []string{"dragon", "spice"},
		},
		{
			name: "trims whitespace",
			args: []string{"--until", "2025-02-01", "--title", " dragon , spice "},
			want: []string{"dragon", "spice"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseArgs(tc.args, nil)
			if err != nil {
				t.Fatalf("ParseArgs() unexpected error: %v", err)
			}
			if len(cfg.TitleFilters) != len(tc.want) {
				t.Fatalf("expected %d title filters, got %d (%v)", len(tc.want), len(cfg.TitleFilters), cfg.TitleFilters)
			}
			for i, want := range tc.want {
				if cfg.TitleFilters[i] != want {
					t.Fatalf("title filter %d: got %q, want %q", i, cfg.TitleFilters[i], want)
				}
			}
		})
	}
}
