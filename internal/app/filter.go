package app

import (
	"git.skobk.in/skobkin/jnovel-scrape/internal/model"
	"git.skobk.in/skobkin/jnovel-scrape/internal/util"
)

// FilterStats captures how many posts were removed per filter type.
type FilterStats struct {
	TypeDropped   int
	TitleDropped  int
	VolumeDropped int
}

func filterPosts(posts model.Posts, cfg Config) (model.Posts, FilterStats) {
	if len(posts) == 0 {
		return posts, FilterStats{}
	}
	var (
		filtered model.Posts
		stats    FilterStats
	)

	for _, post := range posts {
		if len(cfg.TypeFilters) > 0 {
			if _, ok := cfg.TypeFilters[post.Type]; !ok {
				stats.TypeDropped++
				continue
			}
		}

		if cfg.TitleFilter != "" && !util.ContainsFold(post.Title, cfg.TitleFilter) {
			stats.TitleDropped++
			continue
		}

		if cfg.VolumeFilter != nil {
			if !post.VolumeEqual(*cfg.VolumeFilter) {
				stats.VolumeDropped++
				continue
			}
		}

		filtered = append(filtered, post)
	}

	filtered.Sort()
	return filtered, stats
}
