package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(&buf)

	logger.Infof("info %d", 1)
	logger.Warnf("warn %s", "two")
	logger.Errorf("error %s", "three")

	output := buf.String()
	for _, token := range []string{"INFO info 1", "WARN warn two", "ERROR error three"} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected log output to contain %q, got %q", token, output)
		}
	}
}
