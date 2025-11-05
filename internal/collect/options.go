package collect

import (
	"time"

	"github.com/skobkin/jnovels-parser/internal/httpx"
)

// Logger is the minimal logging interface expected by collectors.
type Logger interface {
	Infof(format string, args ...any)
}

type noopLogger struct{}

func (noopLogger) Infof(string, ...any) {}

// Options controls collector behavior shared by API and HTML modes.
type Options struct {
	BaseURL     string
	MaxPages    int
	Concurrency int
	UserAgent   string
	Logger      Logger
	Client      *httpx.Client
	ReqInterval time.Duration
}

// DefaultBaseURL for jnovels.
const DefaultBaseURL = "https://jnovels.com"
