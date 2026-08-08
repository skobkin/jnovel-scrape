package app

import (
	"strings"
	"testing"

	"github.com/knadh/koanf/v2"
)

// TestKoanfIsImportable locks in that the koanf dependency is wired up.
// Removing the dependency should make this file fail to compile.
func TestKoanfIsImportable(t *testing.T) {
	// koanf.Koanf contains a map[string]any, so it cannot be compared
	// with ==. A nil check is enough to prove the type is in scope
	// and the zero value is constructable.
	var k *koanf.Koanf
	if k != nil {
		t.Fatalf("expected nil pointer to be nil; got %#v", k)
	}
	k = koanf.New(".")
	if k == nil {
		t.Fatalf("koanf.New returned nil")
	}
}

func TestConfigKeysAreNonEmpty(t *testing.T) {
	keys := configKeys()
	if len(keys) == 0 {
		t.Fatalf("configKeys() returned no keys")
	}
	for k, v := range keys {
		if k == "" {
			t.Fatalf("configKeys has an empty koanf key for env %q", v)
		}
		if v == "" {
			t.Fatalf("configKeys has an empty env-var suffix for key %q", k)
		}
		if strings.ContainsAny(k, " 	\n") {
			t.Fatalf("koanf key %q must not contain whitespace", k)
		}
	}
}
