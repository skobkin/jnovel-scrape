package util

import (
	"html"
	"regexp"
	"strings"
)

var tagPattern = regexp.MustCompile(`<[^>]+>`)
var whitespacePattern = regexp.MustCompile(`\s+`)

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
