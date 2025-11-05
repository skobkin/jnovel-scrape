package util

import (
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var volumePattern = regexp.MustCompile(`(?i)\bvol(?:\.|ume)?\s*0*([0-9]+(?:\.[0-9]+)?)\b`)
var trailingTypePattern = regexp.MustCompile(`(?i)(?:[\s\-\x{2013}\x{2014}:|()]*)(?:light\s+novel\s+)?(epub|pdf|manga)\s*$`)
var trailingLightNovelPattern = regexp.MustCompile(`(?i)[\s\-\x{2013}\x{2014}:|()]*light\s+novel\s*$`)
var punctuationSpaceReplacer = strings.NewReplacer(" :", ":", " ,", ",", " .", ".", " !", "!", " ?", "?")
var volumeExtraPattern = regexp.MustCompile(`(?i)^([\s:,\-\x{2013}\x{2014}]*)((act|part|episode|section)\s+(?:[0-9]+|[ivxlcdm]+)(?:\s*(?:[0-9]+|[ivxlcdm]+|[a-z]+))?)`)
var trailingExtraPattern = regexp.MustCompile(`(?i)(?:[,\s]+)(act|part|episode|section)\s+([0-9]+|[ivxlcdm]+)$`)

var skipVolumePreceders = map[string]struct{}{
	"level":    {},
	"levels":   {},
	"arc":      {},
	"season":   {},
	"seasons":  {},
	"chapter":  {},
	"chapters": {},
	"stage":    {},
	"stages":   {},
	"week":     {},
	"weeks":    {},
	"day":      {},
	"days":     {},
	"night":    {},
	"nights":   {},
	"episode":  {},
	"episodes": {},
	"lesson":   {},
	"lessons":  {},
	"mission":  {},
	"missions": {},
	"route":    {},
	"routes":   {},
	"story":    {},
	"stories":  {},
	"book":     {},
	"books":    {},
	"volume":   {},
	"vol":      {},
}

// ParseVolume searches for a volume token and returns a float pointer when present.
func ParseVolume(text string) (*float64, bool) {
	if text == "" {
		return nil, false
	}
	match := volumePattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return nil, false
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, false
	}
	return &value, true
}

// FormatVolume renders a volume for Markdown output.
func FormatVolume(v *float64) string {
	if v == nil {
		return ""
	}
	val := *v
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return ""
	}
	if math.Mod(val, 1.0) == 0 {
		return strconv.FormatInt(int64(val), 10)
	}
	// Trim trailing zeros for decimal volumes.
	s := strconv.FormatFloat(val, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// ExtractTitleAndVolume removes trailing volume/type tokens, returning the cleaned title and parsed volume.
func ExtractTitleAndVolume(title string) (string, *float64, string) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "", nil, ""
	}

	matches := volumePattern.FindAllStringSubmatchIndex(trimmed, -1)
	var volumePtr *float64
	var volumeExtra string
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		numberStart := last[2]
		numberEnd := last[3]
		valueStr := trimmed[numberStart:numberEnd]
		if value, err := strconv.ParseFloat(valueStr, 64); err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) {
			vol := value
			volumePtr = &vol
		}

		suffixCandidate := trimmed[last[1]:]
		if suff, consumed := extractVolumeExtra(suffixCandidate); suff != "" {
			volumeExtra = suff
			// adjust matches to skip consumed tail in final rebuild
			matches[len(matches)-1][1] += consumed
		}

		var builder strings.Builder
		prev := 0
		for _, match := range matches {
			builder.WriteString(trimmed[prev:match[0]])
			prev = match[1]
		}
		builder.WriteString(trimmed[prev:])
		trimmed = builder.String()
	}

	originalTrimmed := trimmed
	var extraCandidate string
	var extraFound bool
	if volumePtr == nil && volumeExtra == "" {
		trimmedCandidate, extra, found := detachTrailingExtra(trimmed)
		if found {
			extraCandidate = extra
			extraFound = true
			trimmed = trimmedCandidate
		}
	}

	if volumePtr == nil {
		if newTrim, volPtr, ok := extractStandaloneVolume(trimmed); ok {
			trimmed = newTrim
			volumePtr = volPtr
			if volumeExtra == "" && extraCandidate != "" {
				volumeExtra = extraCandidate
			}
		} else if extraFound {
			// revert removal of extra if no volume detected
			trimmed = originalTrimmed
		}
	}

	if volumeExtra == "" && extraCandidate != "" && volumePtr != nil {
		volumeExtra = extraCandidate
	}

	trimmed = removeTrailingTypeTokens(trimmed)
	trimmed = trimTrailingDelimiters(trimmed)
	trimmed = collapseSpaces(trimmed)
	trimmed = punctuationSpaceReplacer.Replace(trimmed)
	if volumePtr == nil {
		if newTrim, slugVol, ok := extractStandaloneVolume(trimmed); ok {
			trimmed = newTrim
			volumePtr = slugVol
			if volumeExtra == "" && extraCandidate != "" {
				volumeExtra = extraCandidate
			}
		}
	}

	if trimmed == "" {
		trimmed = strings.TrimSpace(title)
	}

	return trimmed, volumePtr, volumeExtra
}

func removeTrailingTypeTokens(s string) string {
	trimmed := strings.TrimSpace(s)
	for {
		loc := trailingTypePattern.FindStringIndex(trimmed)
		if loc == nil || loc[1] != len(trimmed) {
			break
		}
		trimmed = strings.TrimSpace(trimmed[:loc[0]])
	}
	for {
		loc := trailingLightNovelPattern.FindStringIndex(trimmed)
		if loc == nil || loc[1] != len(trimmed) {
			break
		}
		trimmed = strings.TrimSpace(trimmed[:loc[0]])
	}
	return trimmed
}

func trimTrailingDelimiters(s string) string {
	trimmed := strings.TrimRightFunc(strings.TrimSpace(s), func(r rune) bool {
		switch r {
		case '-', '\u2013', '\u2014', ':', '|', '/', '\\':
			return true
		case '(', ')', '[', ']', '{', '}':
			return true
		}
		return unicode.IsSpace(r)
	})
	return strings.TrimSpace(trimmed)
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func extractVolumeExtra(after string) (string, int) {
	if after == "" {
		return "", 0
	}
	loc := volumeExtraPattern.FindStringSubmatchIndex(after)
	if loc == nil {
		return "", 0
	}
	descriptor := after[loc[4]:loc[5]]
	consumed := loc[1]
	descriptor = normalizeVolumeExtra(descriptor)
	return descriptor, consumed
}

func normalizeVolumeExtra(extra string) string {
	extra = strings.TrimSpace(removeTrailingTypeTokens(extra))
	words := strings.Fields(extra)
	if len(words) == 0 {
		return ""
	}
	for i, word := range words {
		if isRomanNumeral(word) {
			words[i] = strings.ToUpper(word)
		} else {
			lower := strings.ToLower(word)
			if len(lower) > 0 {
				words[i] = strings.ToUpper(lower[:1]) + lower[1:]
			}
		}
	}
	return strings.Join(words, " ")
}

func isRomanNumeral(s string) bool {
	if s == "" {
		return false
	}
	s = strings.ToUpper(s)
	for _, r := range s {
		switch r {
		case 'I', 'V', 'X', 'L', 'C', 'D', 'M':
		default:
			return false
		}
	}
	return true
}

func detachTrailingExtra(s string) (string, string, bool) {
	trimmed := strings.TrimSpace(s)
	loc := trailingExtraPattern.FindStringSubmatchIndex(trimmed)
	if loc == nil || loc[1] != len(trimmed) {
		return s, "", false
	}
	extra := trimmed[loc[2]:loc[3]] + " " + trimmed[loc[4]:loc[5]]
	prefix := strings.TrimSpace(trimmed[:loc[0]])
	return prefix, normalizeVolumeExtra(extra), true
}

func extractStandaloneVolume(s string) (string, *float64, bool) {
	trimmed := strings.TrimSpace(s)
	prefix, token := splitLastAlphaNum(trimmed)
	if token == "" {
		return s, nil, false
	}
	tokenLower := strings.ToLower(token)
	var value float64
	var ok bool
	if num, err := strconv.ParseFloat(tokenLower, 64); err == nil {
		value = num
		ok = true
	} else if val, convOK := romanToInt(tokenLower); convOK {
		value = float64(val)
		ok = true
	}
	if !ok {
		return s, nil, false
	}

	_, prevToken := splitLastAlphaNum(prefix)
	prevToken = strings.Trim(strings.ToLower(prevToken), "'\"")
	if isSkipVolumeWord(prevToken) {
		return s, nil, false
	}

	vol := value
	return strings.TrimSpace(prefix), &vol, true
}

func splitLastAlphaNum(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	runes := []rune(s)
	end := len(runes)
	for end > 0 && !isAlphaNumeric(runes[end-1]) {
		end--
	}
	if end == 0 {
		return strings.TrimSpace(s), ""
	}
	start := end
	for start > 0 && isAlphaNumeric(runes[start-1]) {
		start--
	}
	token := string(runes[start:end])
	prefix := strings.TrimSpace(string(runes[:start]))
	return prefix, token
}

func isAlphaNumeric(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isSkipVolumeWord(word string) bool {
	if word == "" {
		return false
	}
	_, ok := skipVolumePreceders[word]
	return ok
}

func romanToInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	values := map[rune]int{
		'i': 1,
		'v': 5,
		'x': 10,
		'l': 50,
		'c': 100,
		'd': 500,
		'm': 1000,
	}
	total := 0
	prev := 0
	for _, r := range strings.ToLower(s) {
		val, ok := values[r]
		if !ok {
			return 0, false
		}
		if val > prev {
			total += val - 2*prev
		} else {
			total += val
		}
		prev = val
	}
	return total, true
}

func isPunctuation(r rune) bool {
	switch r {
	case '-', '\u2013', '\u2014', ':', '|', '/', '\\', ',', '.', '!', '?':
		return true
	case '(', ')', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}

func lastNumericToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	runes := []rune(s)
	end := len(runes)
	for end > 0 && unicode.IsSpace(runes[end-1]) {
		end--
	}
	if end == 0 {
		return ""
	}
	start := end
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	token := strings.TrimSpace(string(runes[start:end]))
	if token == "" {
		return ""
	}
	prevEnd := start
	for prevEnd > 0 && unicode.IsSpace(runes[prevEnd-1]) {
		prevEnd--
	}
	prevStart := prevEnd
	for prevStart > 0 && isAlphaNumeric(runes[prevStart-1]) {
		prevStart--
	}
	prevToken := strings.ToLower(strings.Trim(string(runes[prevStart:prevEnd]), "'\""))
	if isSkipVolumeWord(prevToken) {
		return ""
	}
	if _, err := strconv.ParseFloat(token, 64); err == nil {
		return token
	}
	if _, ok := romanToInt(token); ok {
		return token
	}
	return ""
}

// FormatVolumeWithExtra combines base volume and suffix for display.
func FormatVolumeWithExtra(number *float64, extra string) string {
	base := FormatVolume(number)
	extra = strings.TrimSpace(extra)
	switch {
	case base == "" && extra == "":
		return ""
	case base == "":
		return extra
	case extra == "":
		return base
	default:
		return base + " " + extra
	}
}

// ExtractVolumeFromLink attempts to parse volume metadata from a post URL slug.
func ExtractVolumeFromLink(link string) (*float64, string, bool) {
	if link == "" {
		return nil, "", false
	}
	path := extractPathFromURL(link)
	if path == "" {
		return nil, "", false
	}
	if decoded, err := url.PathUnescape(path); err == nil && decoded != "" {
		path = decoded
	}
	slug := strings.Trim(path, "/")
	if slug == "" {
		return nil, "", false
	}
	parts := strings.Split(slug, "/")
	last := parts[len(parts)-1]
	cleaned := strings.ReplaceAll(last, "-", " ")
	cleaned = strings.ReplaceAll(cleaned, "_", " ")

	original := cleaned
	_, vol, extra := ExtractTitleAndVolume(cleaned)
	if vol == nil {
		if newClean, slugVol, ok := extractStandaloneVolume(cleaned); ok {
			vol = slugVol
			cleaned = newClean
		}
	}
	if extra == "" {
		if _, slugExtra, ok := detachTrailingExtra(original); ok {
			extra = slugExtra
		}
	}

	if vol == nil || extra == "" {
		slugBase := removeTrailingTypeTokens(original)
		slugBase = trimTrailingDelimiters(slugBase)
		slugBase = collapseSpaces(slugBase)
		if extra == "" {
			if _, slugExtra, ok := detachTrailingExtra(slugBase); ok {
				extra = slugExtra
				slugBase = strings.TrimSpace(strings.TrimSuffix(slugBase, " "+strings.ToLower(slugExtra)))
			}
		}
		if vol == nil {
			if newBase, slugVol, ok := extractStandaloneVolume(slugBase); ok {
				vol = slugVol
				slugBase = newBase
			} else {
				if token := lastNumericToken(slugBase); token != "" {
					if num, err := strconv.ParseFloat(token, 64); err == nil {
						vol = &num
					}
				}
			}
		}
	}

	if vol == nil && extra == "" {
		return nil, "", false
	}
	return vol, extra, true
}

func extractPathFromURL(link string) string {
	u, err := url.Parse(link)
	if err == nil {
		if u.Path != "" {
			return u.Path
		}
		if u.Opaque != "" {
			return u.Opaque
		}
	}
	// Manual fallback
	link = strings.TrimSpace(link)
	lower := strings.ToLower(link)
	idx := strings.Index(lower, "://")
	if idx == -1 {
		idx = 0
	} else {
		idx += 3
	}
	pos := strings.Index(link[idx:], "/")
	if pos == -1 {
		return ""
	}
	return link[idx+pos:]
}
