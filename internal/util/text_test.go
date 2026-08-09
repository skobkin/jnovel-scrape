package util

import "testing"

func TestCleanTitle(t *testing.T) {
	input := "<em>My Novel</em> Volume&nbsp;5 | Special"
	want := "My Novel Volume 5 | Special"
	got := CleanTitle(input)
	if got != want {
		t.Fatalf("CleanTitle(%q) = %q, want %q", input, got, want)
	}
}

func TestEscapePipes(t *testing.T) {
	got := EscapePipes("A | B")
	if got != "A \\| B" {
		t.Fatalf("EscapePipes() = %q", got)
	}
}

func TestFoldForSearch(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"ascii lower", "Hello", "hello"},
		{"ascii upper", "HELLO", "hello"},
		{"mixed case", "HeLLo", "hello"},
		{"macron u", "Shūmatsu", "shumatsu"},
		{"macron o", "Sōsō", "soso"},
		{"circumflex", "Chûnibyô", "chunibyo"},
		{"acute accent", "Café", "cafe"},
		{"german umlaut", "Mädchen", "madchen"},
		{"already ascii", "Shuumatsu", "shuumatsu"},
		{"empty", "", ""},
		{"combining mark only fold", "a\u0308", "a"},
		{"greek with diacritics", "Αθήνα", "αθηνα"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FoldForSearch(tc.in)
			if got != tc.want {
				t.Fatalf("FoldForSearch(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFoldedContains(t *testing.T) {
	// Note on canonicalisation: Unicode case folding never expands a
	// letter. U+016B ū folds to U+0075 u (one character), not to
	// "uu" — the latter is a romanisation choice, not a fold. The
	// practical effect for the user is that a macroned title
	// ("Shūmatsu") matches an un-macroned query ("shumatsu"), and an
	// un-macroned title ("Shuumatsu") matches a macroned query
	// ("shūmatsu" → "shumatsu") only when the macroned form is
	// present in the haystack, not when the title uses two un-macroned
	// u's. A romanisation-aliases feature would be a separate
	// concern; for now the user gets a clear, conservative semantic.
	cases := []struct {
		name     string
		haystack string
		needle   string
		want     bool
	}{
		{"empty needle matches", "Anything Goes", "", true},
		{"ascii case insensitive", "Sword Art Online", "sword", true},
		{"ascii case insensitive upper needle", "Sword Art Online", "SWORD", true},
		{"ascii case insensitive upper haystack", "SWORD ART ONLINE", "sword", true},
		{"ascii exact substring", "Sword Art Online", "art", true},
		{"ascii no match", "Sword Art Online", "xyz", false},
		{"macroned haystack un-macroned needle", "Shūmatsu no Valkyrie", "shumatsu", true},
		{"un-macroned haystack macroned needle matching its own fold", "Shūmatsu no Valkyrie", "shūmatsu", true},
		{"romanisation difference does not match", "Shuumatsu no Valkyrie", "shūmatsu", false},
		{"both macroned same form", "Shūmatsu no Valkyrie", "Shūmatsu", true},
		{"soso with macrons", "Sōsō no Pet na Kanojo", "soso", true},
		{"chunibyo with circumflex", "Chûnibyô demo Koi ga Shitai!", "chunibyo", true},
		{"cafe with accent", "Café Stéreo", "cafe", true},
		{"cafe with decomposed e + combining acute", "Cafe\u0301", "cafe", true},
		{"madchen with umlaut", "Mädchen im Moor", "madchen", true},
		{"greek with diacritics", "Αθήνα", "αθηνα", true},
		{"subtle false positive guarded by fold", "Star", "tsar", false},
		{"alpha-num", "Sword Art Online 21", "21", true},
		{"invalid utf-8 falls back gracefully", "\xffSword", "sword", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FoldedContains(tc.haystack, tc.needle)
			if got != tc.want {
				t.Fatalf("FoldedContains(%q, %q) = %v, want %v", tc.haystack, tc.needle, got, tc.want)
			}
		})
	}
}

// FoldedContains must keep every behaviour of the older ASCII-only
// ContainsFold. This guard prevents a future regression where
// Unicode folding accidentally drops a match that the legacy path
// would have kept.
func TestFoldedContains_AsciiSupersetOfContainsFold(t *testing.T) {
	pairs := []struct {
		h, n string
	}{
		{"Sword Art Online", "sword"},
		{"Sword Art Online", "SWORD"},
		{"SWORD ART ONLINE", "sword"},
		{"Overlord", "lord"},
		{"Mushoku Tensei", "TENSEI"},
		{"Re:Zero", "re:zero"},
		{"A", "a"},
		{"Hello World", "world"},
	}
	for _, p := range pairs {
		old := ContainsFold(p.h, p.n)
		newer := FoldedContains(p.h, p.n)
		if old != newer {
			t.Fatalf("divergence on (%q, %q): ContainsFold=%v FoldedContains=%v", p.h, p.n, old, newer)
		}
	}
}

func TestNormalizeForSearch(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"ascii no change", "Sword Art Online", "Sword Art Online"},
		{"collapse spaces", "Sword  Art   Online", "Sword Art Online"},
		{"trim leading and trailing", "   Sword Art Online   ", "Sword Art Online"},
		{"tab treated as whitespace", "Sword	Art	Online", "Sword Art Online"},
		{"newline treated as whitespace", "Sword\nArt\nOnline", "Sword Art Online"},
		{"nbsp folded to space", "Sword\u00a0Art\u00a0Online", "Sword Art Online"},
		{"mixed nbsp and space", "Sword\u00a0 Art   Online", "Sword Art Online"},
		{"only whitespace", "   	\u00a0  ", ""},
		{"ideographic space folded", "Sword\u3000Art\u3000Online", "Sword Art Online"},
		{"preserves internal punctuation", "Re:Zero - Starting Life", "Re:Zero - Starting Life"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeForSearch(tc.in)
			if got != tc.want {
				t.Fatalf("NormalizeForSearch(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// FoldedContains is whitespace- and NBSP-insensitive on the needle
// side. Post titles are pre-normalised by CleanTitle so the
// haystack normalisation is a no-op in practice, but it is
// applied for defensive symmetry.
func TestFoldedContains_WhitespaceAndNbspInsensitive(t *testing.T) {
	cases := []struct {
		name     string
		haystack string
		needle   string
		want     bool
	}{
		{"multi-space needle matches", "Sword Art Online", "sword  art  online", true},
		{"tab in needle matches", "Sword Art Online", "sword	art", true},
		{"nbsp in needle matches", "Sword Art Online", "sword\u00a0art", true},
		{"leading whitespace in needle", "Sword Art Online", "   sword art", true},
		{"trailing whitespace in needle", "Sword Art Online", "sword art   ", true},
		{"needle trims to empty substring of haystack", "Sword Art Online", "   	\u00a0  sword", true},
		{"needle contains only whitespace", "Sword Art Online", "   	  ", true},
		{"needle whitespace does not invent new characters", "Sword Art Online", "sword\u00a0art\u00a0online", true},
		{"untrimmed haystack also normalised", "  Sword\u00a0Art\u00a0Online  ", "sword art", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FoldedContains(tc.haystack, tc.needle)
			if got != tc.want {
				t.Fatalf("FoldedContains(%q, %q) = %v, want %v", tc.haystack, tc.needle, got, tc.want)
			}
		})
	}
}

func TestFoldedWordContains(t *testing.T) {
	cases := []struct {
		name     string
		haystack string
		needle   string
		want     bool
	}{
		{"empty needle matches", "Anything Goes", "", true},
		{"whitespace-only needle matches", "Anything Goes", "   	\u00a0  ", true},
		{"single word present", "Sword Art Online", "sword", true},
		{"single word case insensitive", "Sword Art Online", "ART", true},
		{"all words present in order", "Sword Art Online", "sword art", true},
		{"all words present out of order", "Sword Art Online", "art sword", true},
		{"missing word", "Sword Art Online", "sword life", false},
		{"word boundary prevents substring match", "Sword Art Online", "art", true},
		{"substring only is rejected", "Arte", "art", false},
		{"hyphenated word counts as one token", "Re:Zero - Starting Life in Another World", "re:zero", true},
		{"unicode word boundary", "Shūmatsu no Valkyrie", "valkyrie", true},
		{"unicode word boundary ascii needle", "Shūmatsu no Valkyrie", "shumatsu", true},
		{"unicode word missing", "Shūmatsu no Valkyrie", "shuumatsu", false},
		{"nbsp inside needle is treated as whitespace", "Sword Art Online", "sword\u00a0art", true},
		{"empty haystack no match", "", "sword", false},
		{"needle single token matches diacritic", "Café Stéreo", "cafe", true},
		{"duplicate needle tokens are collapsed (set semantics)", "No Game No Life", "no no no", true},
		{"duplicate needle token missing once still succeeds by set membership", "No Game Life", "no no", true},
		// --- Word-boundary punctuation regression coverage. ---
		// These cases were reported in a post-release bug report:
		// titles ending in ":" or "?" were tokenising as
		// "tensei:" and "dungeon?" rather than "tensei" and
		// "dungeon", so a whole-token needle never matched.
		{"trailing colon peeled from word", "Mushoku Tensei: Jobless Reincarnation", "tensei", true},
		{"trailing question mark peeled from word", "Is It Wrong to Try to Pick Up Girls in a Dungeon?", "dungeon", true},
		{"trailing colon does not split the prefix", "Mushoku Tensei: Jobless Reincarnation", "mushoku tensei", true},
		{"trailing question mark does not split the prefix", "Is It Wrong to Try to Pick Up Girls in a Dungeon?", "is it wrong", true},
		// A title with multiple colons and exclamations parses
		// each word correctly even when the same title has
		// glued punctuation in the middle of the description.
		{"long title with multiple colons", "7th Time Loop: The Villainess Enjoys a Carefree Life Married to Her Worst Enemy! Short Story Collection", "villainess", true},
		{"em-dash separator produces standalone token", "GATE – Thus the JSDF Fought There", "gate", true},
		{"em-dash separator also matches the next word", "GATE – Thus the JSDF Fought There", "jsdf", true},
		// Apostrophes are part of a word; "archdemon's" must
		// stay as one token rather than splitting into
		// "archdemon" + "s" or "archdemons".
		{"apostrophe inside word stays inside", "An Archdemon's Dilemma: How to Love Your Elf Bride", "archdemon's", true},
		{"leading asterisk peeled from word", "*Sword Art Online*", "sword", true},
		{"trailing asterisk peeled from word", "*Sword Art Online*", "online", true},
		{"bracketed prefix is dropped", "[Light Novel] BOFURI", "bofuri", true},
		{"quoted title drops quotes", "\"Ascendance of a Bookworm\"", "ascendance", true},
		{"comma between words stripped", "Goodbye, Horrible Fiancé, Hello, Fun Magic School Life!", "hello", true},
		// Whole-token semantic for a title with internal
		// punctuation: typing just "re" must NOT match the
		// single token "re:zero"; the user has to type the
		// colon themselves. This is consistent with the
		// "word boundary" intent of the mode.
		{"internal colon is not a word separator", "Re:Zero - Starting Life in Another World", "re:zero", true},
		{"internal colon stays inside the token", "Re:Zero - Starting Life in Another World", "re", false},
		{"internal colon does not split on space either", "Re:Zero - Starting Life in Another World", "re zero", false},
		// All-punctuation title produces no tokens, so a
		// non-empty needle cannot match.
		{"all-punctuation haystack", "???", "anything", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FoldedWordContains(tc.haystack, tc.needle)
			if got != tc.want {
				t.Fatalf("FoldedWordContains(%q, %q) = %v, want %v", tc.haystack, tc.needle, got, tc.want)
			}
		})
	}
}

// TestTokenizeFolded probes the helper directly. FoldedWordContains
// covers the user-visible behaviour; this table pins down the
// tokeniser's exact output so future changes cannot silently
// shift the boundaries.
func TestTokenizeFolded(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", []string{}},
		{"only whitespace", "   	  ", []string{}},
		{"ascii words", "hello world", []string{"hello", "world"}},
		{"trailing colon peeled", "tensei:", []string{"tensei"}},
		{"trailing question mark peeled", "dungeon?", []string{"dungeon"}},
		{"trailing exclamation peeled", "wow!", []string{"wow"}},
		{"trailing period peeled", "ends.", []string{"ends"}},
		{"trailing semicolon peeled", "shh;", []string{"shh"}},
		{"trailing comma peeled", "yes,", []string{"yes"}},
		{"trailing em-dash peeled", "gate –", []string{"gate"}},
		{"trailing en-dash peeled", "gate –", []string{"gate"}},
		{"trailing horizontal ellipsis peeled", "wait…", []string{"wait"}},
		{"trailing hyphen peeled", "pre-", []string{"pre"}},
		{"trailing asterisk peeled", "*sword*", []string{"sword"}},
		{"trailing right bracket peeled", "[light]", []string{"light"}},
		{"trailing double quote peeled", "\"bookworm\"", []string{"bookworm"}},
		{"leading asterisk peeled", "*sword", []string{"sword"}},
		{"leading bracket peeled", "[light]", []string{"light"}},
		{"internal colon preserved", "re:zero", []string{"re:zero"}},
		{"internal apostrophe preserved", "archdemon's", []string{"archdemon's"}},
		{"right single quote preserved", "alice\u2019s", []string{"alice\u2019s"}},
		{"digits preserved", "volume 21", []string{"volume", "21"}},
		{"only edge punctuation drops the field", "?!", []string{}},
		{"multiple words with mixed punctuation", "Hello, World! How's it going?",
			[]string{"Hello", "World", "How's", "it", "going"}},
		{"nbsp separates fields", "hello\u00a0world", []string{"hello", "world"}},
		{"ideographic space separates fields", "hello\u3000world", []string{"hello", "world"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenizeFolded(tc.in)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !slicesEqual(got, tc.want) {
				t.Fatalf("tokenizeFolded(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// TestFoldedWordContains_RejectsSubstringNoise demonstrates the
// noise that the word mode was added to suppress. Searching for
// "art" in substring mode matches every title containing the
// letters a-r-t; word mode rejects anything where "art" is not
// a complete token.
func TestFoldedWordContains_RejectsSubstringNoise(t *testing.T) {
	titles := []string{
		"Sword Art Online",
		"Arte",
		"Departure",
		"Heart no Kuni no Alice",
		"Smartphone Isekai",
		"Party of Heroes",
	}
	for _, title := range titles {
		t.Run(title, func(t *testing.T) {
			// "art" as a whole token only matches the first title.
			if got, want := FoldedWordContains(title, "art"), title == "Sword Art Online"; got != want {
				t.Fatalf("FoldedWordContains(%q, %q) = %v, want %v", title, "art", got, want)
			}
		})
	}
}
