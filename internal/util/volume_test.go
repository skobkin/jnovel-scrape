package util

import "testing"

func TestParseVolume(t *testing.T) {
	cases := []struct {
		title   string
		want    float64
		wantHit bool
	}{
		{"Legendary Hero Volume 12 Pdf", 12, true},
		{"Series Volume 04 EPUB", 4, true},
		{"Special Volume 20.5 release", 20.5, true},
		{"Best Of Collection", 0, false},
		{"Volume XX", 0, false},
	}

	for _, tc := range cases {
		got, ok := ParseVolume(tc.title)
		if tc.wantHit && (!ok || got == nil) {
			t.Fatalf("ParseVolume(%q) expected hit", tc.title)
		}
		if !tc.wantHit && ok {
			t.Fatalf("ParseVolume(%q) expected miss", tc.title)
		}
		if tc.wantHit && *got != tc.want {
			t.Fatalf("ParseVolume(%q) = %v, want %v", tc.title, *got, tc.want)
		}
	}
}

func TestFormatVolume(t *testing.T) {
	input := []struct {
		value float64
		want  string
	}{
		{12, "12"},
		{20.5, "20.5"},
		{3.75, "3.75"},
	}
	for _, tc := range input {
		v := tc.value
		got := FormatVolume(&v)
		if got != tc.want {
			t.Fatalf("FormatVolume(%v) = %q, want %q", tc.value, got, tc.want)
		}
	}

	if got := FormatVolume(nil); got != "" {
		t.Fatalf("FormatVolume(nil) = %q, want empty", got)
	}
}

func TestFormatVolumeWithExtra(t *testing.T) {
	base := floatPtr(10)
	if got := FormatVolumeWithExtra(base, "Act 1"); got != "10 Act 1" {
		t.Fatalf("FormatVolumeWithExtra = %q", got)
	}
	if got := FormatVolumeWithExtra(nil, "Part II"); got != "Part II" {
		t.Fatalf("FormatVolumeWithExtra missing base = %q", got)
	}
	if got := FormatVolumeWithExtra(base, ""); got != "10" {
		t.Fatalf("FormatVolumeWithExtra no extra = %q", got)
	}
}

func TestExtractTitleAndVolume(t *testing.T) {
	cases := []struct {
		title      string
		wantTitle  string
		wantVolume *float64
		wantExtra  string
	}{
		{
			title:      "I Want to Be a Saint, But I Can Only Use Attack Magic! Volume 2 Epub",
			wantTitle:  "I Want to Be a Saint, But I Can Only Use Attack Magic!",
			wantVolume: floatPtr(2),
			wantExtra:  "",
		},
		{
			title:      "Demon Lord, Retry! Volume 10",
			wantTitle:  "Demon Lord, Retry!",
			wantVolume: floatPtr(10),
			wantExtra:  "",
		},
		{
			title:      "The Creative Gene Light Novel Epub",
			wantTitle:  "The Creative Gene",
			wantVolume: nil,
			wantExtra:  "",
		},
		{
			title:      "Volume 2 Side Stories EPUB",
			wantTitle:  "Side Stories",
			wantVolume: floatPtr(2),
			wantExtra:  "",
		},
		{
			title:      "The Trials and Tribulations of My Next Life as a Noblewoman Volume 3 Part 1",
			wantTitle:  "The Trials and Tribulations of My Next Life as a Noblewoman",
			wantVolume: floatPtr(3),
			wantExtra:  "Part 1",
		},
		{
			title:      "Agents of the Four Seasons Volume 4: Dance of Summer, Part II",
			wantTitle:  "Agents of the Four Seasons: Dance of Summer, Part II",
			wantVolume: floatPtr(4),
			wantExtra:  "",
		},
		{
			title:      "Agents of the Four Seasons Volume 5: The Archer of Dawn",
			wantTitle:  "Agents of the Four Seasons: The Archer of Dawn",
			wantVolume: floatPtr(5),
			wantExtra:  "",
		},
		{
			title:      "Classroom of the Elite Year 2 Vol. 12",
			wantTitle:  "Classroom of the Elite Year 2",
			wantVolume: floatPtr(12),
			wantExtra:  "",
		},
		{
			title:      "Peerless Vol. 4",
			wantTitle:  "Peerless",
			wantVolume: floatPtr(4),
			wantExtra:  "",
		},
		{
			title:      "The Misfit of Demon King Academy Volume 10 Act 1",
			wantTitle:  "The Misfit of Demon King Academy",
			wantVolume: floatPtr(10),
			wantExtra:  "Act 1",
		},
		{
			title:      "lets get to villainessin stratagems of a former commoner 2 epub",
			wantTitle:  "lets get to villainessin stratagems of a former commoner",
			wantVolume: floatPtr(2),
			wantExtra:  "",
		},
	}

	for _, tc := range cases {
		gotTitle, gotVolume, gotExtra := ExtractTitleAndVolume(tc.title)
		if gotTitle != tc.wantTitle {
			t.Fatalf("ExtractTitleAndVolume(%q) title = %q, want %q", tc.title, gotTitle, tc.wantTitle)
		}
		if tc.wantVolume == nil {
			if gotVolume != nil {
				t.Fatalf("ExtractTitleAndVolume(%q) volume = %v, want nil", tc.title, *gotVolume)
			}
		} else {
			if gotVolume == nil || *gotVolume != *tc.wantVolume {
				t.Fatalf("ExtractTitleAndVolume(%q) volume = %v, want %v", tc.title, gotVolumeVal(gotVolume), *tc.wantVolume)
			}
		}
		if gotExtra != tc.wantExtra {
			t.Fatalf("ExtractTitleAndVolume(%q) extra = %q, want %q", tc.title, gotExtra, tc.wantExtra)
		}
	}
}

func floatPtr(v float64) *float64 {
	return &v
}

func gotVolumeVal(v *float64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func TestExtractVolumeFromLink(t *testing.T) {
	cases := []struct {
		link      string
		wantVol   *float64
		wantExtra string
		ok        bool
	}{
		{
			link:      "https://jnovels.com/lets-get-to-villainessin-stratagems-of-a-former-commoner-2-epub/",
			wantVol:   floatPtr(2),
			wantExtra: "",
			ok:        true,
		},
		{
			link:      "https://jnovels.com/the-water-magician-arc-1-light-novel-epub/",
			wantVol:   nil,
			wantExtra: "",
			ok:        false,
		},
		{
			link:      "https://jnovels.com/the-misfit-of-demon-king-academy-volume-10-act-1-epub/",
			wantVol:   floatPtr(10),
			wantExtra: "Act 1",
			ok:        true,
		},
		{
			link:      "https://jnovels.com/to-sir-without-love-im-divorcing-you-i-part-1-light-novel-epub/",
			wantVol:   floatPtr(1),
			wantExtra: "Part 1",
			ok:        true,
		},
	}

	for _, tc := range cases {
		vol, extra, ok := ExtractVolumeFromLink(tc.link)
		if ok != tc.ok {
			t.Fatalf("ExtractVolumeFromLink(%q) ok = %v, want %v", tc.link, ok, tc.ok)
		}
		if tc.wantVol == nil {
			if vol != nil {
				t.Fatalf("ExtractVolumeFromLink(%q) volume = %v, want nil", tc.link, *vol)
			}
		} else {
			if vol == nil || *vol != *tc.wantVol {
				t.Fatalf("ExtractVolumeFromLink(%q) volume = %v, want %v", tc.link, gotVolumeVal(vol), *tc.wantVol)
			}
		}
		if extra != tc.wantExtra {
			t.Fatalf("ExtractVolumeFromLink(%q) extra = %q, want %q", tc.link, extra, tc.wantExtra)
		}
	}
}
