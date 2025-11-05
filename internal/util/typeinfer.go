package util

import (
	"strings"

	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
)

var typeKeywords = map[model.PostType][]string{
	model.TypeEPUB:  {"epub"},
	model.TypePDF:   {"pdf"},
	model.TypeManga: {"manga"},
}

// InferType attempts to classify a post using title, categories, and tags.
func InferType(title string, categories, tags []string) model.PostType {
	titleLower := strings.ToLower(title)
	candidates := []string{titleLower}
	candidates = append(candidates, toLowerSlice(categories)...)
	candidates = append(candidates, toLowerSlice(tags)...)

	for postType, tokens := range typeKeywords {
		for _, candidate := range candidates {
			for _, token := range tokens {
				if strings.Contains(candidate, token) {
					return postType
				}
			}
		}
	}

	return model.TypeUnknown
}

func toLowerSlice(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, strings.ToLower(item))
	}
	return out
}
