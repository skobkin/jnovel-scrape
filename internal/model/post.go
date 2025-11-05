package model

import (
	"sort"
	"strings"
	"time"
)

// PostType enumerates normalized content types.
type PostType string

const (
	TypeEPUB    PostType = "EPUB"
	TypePDF     PostType = "PDF"
	TypeManga   PostType = "MANGA"
	TypeUnknown PostType = "UNKNOWN"
)

// KnownTypes contains the ordered set of valid types (excluding UNKNOWN).
var KnownTypes = []PostType{TypeEPUB, TypePDF, TypeManga}

// AllTypes returns known types including UNKNOWN.
func AllTypes() []PostType {
	out := append([]PostType{}, KnownTypes...)
	out = append(out, TypeUnknown)
	return out
}

// NormalizeType coerces a loose string token into a PostType.
func NormalizeType(token string) PostType {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "epub":
		return TypeEPUB
	case "pdf":
		return TypePDF
	case "manga":
		return TypeManga
	case "unknown":
		return TypeUnknown
	default:
		return TypeUnknown
	}
}

// Post holds the normalized metadata for a jnovels post.
type Post struct {
	Title       string
	Volume      *float64
	VolumeExtra string
	Type        PostType
	Date        time.Time
	Link        string
	SourceID    int64
	Categories  []string
	Tags        []string
}

// HasVolume returns true when the post has a parsed volume number.
func (p Post) HasVolume() bool {
	return p.Volume != nil
}

// VolumeEqual reports whether the post's volume equals the input value.
func (p Post) VolumeEqual(value float64) bool {
	if p.Volume == nil {
		return false
	}
	return *p.Volume == value
}

// FormatDate returns the canonical YYYY-MM-DD string.
func (p Post) FormatDate() string {
	return p.Date.Format("2006-01-02")
}

// Posts represents a slice of Post values.
type Posts []Post

// Sort sorts posts by Date desc, then Title asc.
func (ps Posts) Sort() {
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].Date.Equal(ps[j].Date) {
			return strings.ToLower(ps[i].Title) < strings.ToLower(ps[j].Title)
		}
		return ps[i].Date.After(ps[j].Date)
	})
}
