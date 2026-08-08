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
