package collect

import "testing"

func TestParseWPTime(t *testing.T) {
	cases := []struct {
		name     string
		primary  string
		fallback string
	}{
		{"RFC3339", "2024-05-01T10:11:12Z", ""},
		{"RFC3339NoZone", "2024-05-01T10:11:12", ""},
		{"Fallback", "", "2024-05-01T10:11:12Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseWPTime(tc.primary, tc.fallback)
			if err != nil {
				t.Fatalf("parseWPTime() error = %v", err)
			}
			if got.IsZero() {
				t.Fatalf("parseWPTime() returned zero time")
			}
		})
	}

	if _, err := parseWPTime("", ""); err == nil {
		t.Fatalf("expected error when both inputs empty")
	}
}
