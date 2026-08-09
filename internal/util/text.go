package util

import (
	"html"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var tagPattern = regexp.MustCompile(`<[^>]+>`)
var whitespacePattern = regexp.MustCompile(`\s+`)

// searchFold is a Unicode-aware case+diacritic folder shared by every
// search-time comparison in this package. It folds both sides to a
// canonical form so that, for example, "Shūmatsu" matches "Shuumatsu"
// and "Sōsō" matches "Soso".
//
//	searchFold  = Unicode case-fold  (cases.Fold)
//	            + NFD decompose
//	            + drop combining marks (Mn)
//
// NFD splits "ū" into "u" + U+0304 (combining macron); the runes
// transformer then strips the Mn category, leaving the ASCII base
// letter. The order matters: case-fold first, then decompose, so
// that mixed-case + macroned input ("Shūmatsu") and all-ASCII input
// ("Shuumatsu") collapse to the same bytes.
var searchFold = transform.Chain(
	cases.Fold(),
	norm.NFD,
	runes.Remove(runes.In(unicode.Mn)),
)

// FoldForSearch returns a canonical, case-folded, diacritic-stripped
// form of s suitable for case-insensitive substring and word
// comparisons. The result is intended only for matching; do not
// display it to users. The input is not whitespace-normalised —
// use NormalizeForSearch when the needle or haystack may contain
// runs of whitespace or non-breaking spaces.
func FoldForSearch(s string) string {
	out, _, err := transform.String(searchFold, s)
	if err != nil {
		// transform.String only fails if the input contains an
		// invalid UTF-8 byte; fall back to the original string so
		// search still works (without folding) on bad data rather
		// than returning an empty match.
		return s
	}

	return out
}

// CleanTitle removes HTML tags, unescapes entities, and collapses whitespace.
func CleanTitle(input string) string {
	if input == "" {
		return ""
	}
	stripped := tagPattern.ReplaceAllString(input, "")
	unescaped := html.UnescapeString(stripped)
	unescaped = strings.ReplaceAll(unescaped, "\u00a0", " ")
	collapsed := whitespacePattern.ReplaceAllString(unescaped, " ")

	return strings.TrimSpace(collapsed)
}

// StripTags removes HTML tags without unescaping entities or trimming.
func StripTags(input string) string {
	if input == "" {
		return ""
	}

	return tagPattern.ReplaceAllString(input, "")
}

// EscapePipes escapes Markdown table separators within text.
func EscapePipes(s string) string {
	return strings.ReplaceAll(s, "|", `\|`)
}

// ContainsFold performs case-insensitive substring search.
func ContainsFold(haystack, needle string) bool {
	if needle == "" {
		return true
	}

	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// FoldedContains reports whether needle appears in haystack under
// Unicode case-folding and diacritic stripping. Both sides are
// transformed by FoldForSearch so that matches are insensitive to
// case (ASCII or otherwise) and to the presence of combining
// marks: "Shūmatsu" matches "shuumatsu", "Sōsō" matches "soso",
// and "Café" matches "cafe". An empty needle always matches.
//
// Both sides are also passed through NormalizeForSearch so that
// the comparison is robust to non-breaking spaces and runs of
// whitespace in either operand; in practice the haystack comes
// pre-cleaned by CleanTitle and the needle is what benefits.
func FoldedContains(haystack, needle string) bool {
	if needle == "" {
		return true
	}

	return strings.Contains(
		FoldForSearch(NormalizeForSearch(haystack)),
		FoldForSearch(NormalizeForSearch(needle)),
	)
}

// NormalizeForSearch prepares a string for case-insensitive
// substring matching: every Unicode whitespace rune (including
// non-breaking space U+00A0) becomes a regular ASCII space,
// runs of whitespace collapse to a single space, and the result
// is trimmed. Unlike CleanTitle it does NOT strip HTML tags or
// unescape entities — the needle side comes from the CLI and
// the haystack side is already CleanTitle'd by the time it
// reaches a comparison.
func NormalizeForSearch(s string) string {
	if s == "" {
		return s
	}

	// We can't reuse the package-level whitespacePattern because
	// Go's regexp \s class is ASCII-only — it does not match
	// non-breaking space or any other Unicode whitespace. Use
	// strings.Map so every Unicode-space rune is folded to a
	// single ASCII space; the subsequent Fields call then
	// collapses runs.
	mapped := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}

		return r
	}, s)

	return strings.TrimSpace(strings.Join(strings.Fields(mapped), " "))
}

// FoldedWordContains reports whether every token of the needle
// appears as a complete token in the haystack, after Unicode
// case-folding and diacritic stripping. A "token" is a maximal
// run of non-whitespace runes in the normalised string with any
// edge punctuation trimmed off (so "tensei:" and "tensei" match
// as the same token, but "Re:Zero" stays intact). An empty
// needle matches anything (mirroring FoldedContains).
//
// In word mode the needle is a query like "no game no life" and
// the haystack is a full post title; the post matches when every
// word of the query appears as a discrete word in the title,
// anywhere and in any order. This avoids the substring noise
// that makes --title "art" match "Arte", "Kakuriyo no Yadomeshi:
// Art", and any other title containing the letters "art".
//
// Token matching is set-based: a needle of "no no" matches any
// title containing the word "no" at least once. Duplicate tokens
// in the needle are not enforced as duplicates in the haystack.
// This is intentional: a user searching "no game no life" is
// expressing a phrase, not a multiset requirement, and set
// semantics avoids surprises on the common queries.
//
// Matching is case- and diacritic-insensitive through the same
// fold as FoldedContains, and whitespace-insensitive through the
// same NormalizeForSearch.
func FoldedWordContains(haystack, needle string) bool {
	normalisedNeedle := NormalizeForSearch(needle)
	if normalisedNeedle == "" {
		return true
	}

	normalisedHaystack := NormalizeForSearch(haystack)
	if normalisedHaystack == "" {
		return false
	}

	needleTokens := tokenizeFolded(FoldForSearch(normalisedNeedle))
	if len(needleTokens) == 0 {
		return true
	}

	// Build the folded haystack once and split into tokens. We
	// compare by membership in a set rather than scanning slices
	// for every needle token, which is O(h + n) instead of
	// O(h * n) for large titles.
	haystackTokens := tokenizeFolded(FoldForSearch(normalisedHaystack))
	tokenSet := make(map[string]struct{}, len(haystackTokens))
	for _, t := range haystackTokens {
		tokenSet[t] = struct{}{}
	}

	for _, t := range needleTokens {
		if _, ok := tokenSet[t]; !ok {
			return false
		}
	}

	return true
}

// tokenizeFolded splits s on Unicode whitespace and trims any
// leading or trailing edge punctuation from each field. The
// function is named "Folded" because callers are expected to pass
// an already-folded string (it does not apply FoldForSearch
// itself; that would fold every rune twice when used inside a
// pipeline that already folded once).
//
// "Edge punctuation" is the set of runes for which
// unicode.IsPunct or unicode.IsSymbol returns true, with one
// carve-out:
//   - '\” (ASCII apostrophe and right-single-quote U+2019)
//     stays inside tokens so "Alice's Adventures" stays a single
//     token rather than splitting into "Alice" + "s" +
//     "Adventures". (The leading- or trailing-apostrophe case
//     is rare in titles, and stripping a stray apostrophe at a
//     field boundary is fine because the user did not intend it
//     to be part of the word.)
//
// The colon has no special case: a trailing colon in
// "Mushoku Tensei:" is peeled off (it is at the field
// boundary), while the internal colon in "Re:Zero" stays
// because neither edge of the field is a colon. This matches
// how users parse titles in their head: "Mushoku Tensei:" is
// two tokens [mushoku, tensei], "Is It Wrong... Dungeon?" is
// the right number of tokens ending with [dungeon], "Re:Zero"
// is one token, and "*Sword Art Online*" still parses to
// [sword, art, online].
func tokenizeFolded(s string) []string {
	out := make([]string, 0, 8)
	field := make([]rune, 0, len(s))
	flush := func() {
		if len(field) == 0 {
			return
		}
		start, end := 0, len(field)
		for start < end && isEdgePunct(field[start]) {
			start++
		}
		for end > start && isEdgePunct(field[end-1]) {
			end--
		}
		if start < end {
			out = append(out, string(field[start:end]))
		}
		field = field[:0]
	}
	for _, r := range s {
		if unicode.IsSpace(r) {
			flush()

			continue
		}
		field = append(field, r)
	}
	flush()

	return out
}

// isEdgePunct reports whether r should be peeled off when it
// appears at the start or end of a token. The apostrophe
// carve-out is documented on tokenizeFolded. Colon is NOT
// carved out — it is stripped from a field boundary (so
// "tensei:" -> "tensei") but stays inside a field (so
// "Re:Zero" stays intact).
func isEdgePunct(r rune) bool {
	if r == '\'' {
		return false
	}

	return unicode.IsPunct(r) || unicode.IsSymbol(r)
}
