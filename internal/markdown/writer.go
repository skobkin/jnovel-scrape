package markdown

import (
	"fmt"
	"io"
	"time"

	"github.com/skobkin/jnovels-parser/internal/model"
	"github.com/skobkin/jnovels-parser/internal/util"
)

// WriteTable writes the Markdown output to the provided writer.
func WriteTable(w io.Writer, cutoff time.Time, posts model.Posts) error {
	if _, err := fmt.Fprintf(w, "Generated from jnovels.com (cutoff: %s)\n\n", cutoff.Format("2006-01-02")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Title | Volume | Type | Date | Link |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "|---|---:|---|---|---|"); err != nil {
		return err
	}

	for _, post := range posts {
		title := util.EscapePipes(post.Title)
		volume := util.FormatVolumeWithExtra(post.Volume, post.VolumeExtra)
		date := post.FormatDate()
		link := fmt.Sprintf("[link](%s)", post.Link)
		line := fmt.Sprintf("| %s | %s | %s | %s | %s |", title, volume, post.Type, date, link)
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
