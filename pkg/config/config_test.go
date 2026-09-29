package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tyemirov/RSVP/internal/testsupport"
	"github.com/tyemirov/RSVP/pkg/config"
	"gopkg.in/yaml.v3"
)

func TestLoadCanonicalConfiguration(t *testing.T) {
	valid := testsupport.EnvironmentConfig()
	payload, err := yaml.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name, extra string
		failure     bool
	}{
		{"current", "", false}, {"obsolete parser", "natural_language_parser_endpoint: https://example.test\n", true},
		{"multiple documents", "---\nserver: {}\n", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yml")
			if err := os.WriteFile(path, append(payload, []byte(scenario.extra)...), 0666); err != nil {
				t.Fatal(err)
			}
			_, err := config.Load(path)
			if (err != nil) != scenario.failure {
				t.Fatalf("configuration outcome: %v", err)
			}
		})
	}
	t.Run("missing private reference", func(t *testing.T) {
		t.Setenv("RSVP_TEST_PRIVATE_VALUE", "")
		source := strings.Replace(string(payload), "secret: fixture-secret", "secret: ${RSVP_TEST_PRIVATE_VALUE}", 1)
		path := filepath.Join(t.TempDir(), "config.yml")
		if err := os.WriteFile(path, []byte(source), 0666); err != nil {
			t.Fatal(err)
		}
		_, err := config.Load(path)
		if err == nil || !strings.Contains(err.Error(), "RSVP_TEST_PRIVATE_VALUE") {
			t.Fatalf("missing-reference error: %v", err)
		}
	})
}
