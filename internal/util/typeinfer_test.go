package util

import (
	"testing"

	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
)

func TestInferType(t *testing.T) {
	type args struct {
		title      string
		categories []string
		tags       []string
	}
	cases := []struct {
		name string
		args args
		want model.PostType
	}{
		{
			name: "TitlePDF",
			args: args{title: "Awesome Novel Volume 10 PDF"},
			want: model.TypePDF,
		},
		{
			name: "CategoryEPUB",
			args: args{title: "Awesome Novel Volume 10", categories: []string{"Light Novels EPUB"}},
			want: model.TypeEPUB,
		},
		{
			name: "TagManga",
			args: args{title: "Chapter Release", tags: []string{"Manga Updates"}},
			want: model.TypeManga,
		},
		{
			name: "Unknown",
			args: args{title: "Special Pack"},
			want: model.TypeUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := InferType(tc.args.title, tc.args.categories, tc.args.tags)
			if got != tc.want {
				t.Fatalf("InferType() = %v, want %v", got, tc.want)
			}
		})
	}
}
