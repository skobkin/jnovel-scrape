package app

// Version is the build-time version string injected by the release pipeline.
// It is set via -ldflags at build time, e.g.:
//
//	-X git.skobk.in/skobkin/jnovel-scrape/internal/app.Version=v1.2.3
//
// When built from a development checkout without ldflags, Version falls back
// to "dev".
var Version = "dev"
