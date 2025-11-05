package markdown

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
)

func TestWriteTable(t *testing.T) {
	var buf bytes.Buffer
	cutoff := time.Date(2025, time.February, 2, 0, 0, 0, 0, time.UTC)
	v := 4.0
	posts := model.Posts{
		{
			Title:       "Mage | Academy",
			Volume:      &v,
			VolumeExtra: "Act 1",
			Type:        model.TypeEPUB,
			Date:        cutoff,
			Link:        "https://example.com/mage-academy",
		},
	}

	if err := WriteTable(&buf, cutoff, posts); err != nil {
		t.Fatalf("WriteTable error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Generated from jnovels.com (cutoff: 2025-02-02)") {
		t.Fatalf("header missing cutoff: %s", output)
	}
	if !strings.Contains(output, "| Title | Volume | Type | Date | Link |") {
		t.Fatalf("missing table header: %s", output)
	}
	expectedRow := "| Mage \\| Academy | 4 Act 1 | EPUB | 2025-02-02 | [link](https://example.com/mage-academy) |"
	if !strings.Contains(output, expectedRow) {
		t.Fatalf("unexpected table row, want substring %s\noutput:\n%s", expectedRow, output)
	}
}
