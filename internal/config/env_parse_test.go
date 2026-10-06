package config

import (
	"strings"
	"testing"
)

// A value the parser does not understand must stop startup. "True" and "1" used to read as false, so
// AUTH_ENABLED=True switched authentication off on a server whose config file had enabled it.
func TestAnUnparsableEnvironmentValueIsRefused(t *testing.T) {
	for _, c := range []struct{ key, value string }{
		{"AUTH_ENABLED", "yes"},
		{"AUTH_REQUIRE_AUTH", "on"},
		{"SERVER_PORT", "eighty-eighty"},
		{"OAUTH_TIMEOUT", "30"},
		{"CACHE_TTL", "5"},
	} {
		t.Run(c.key, func(t *testing.T) {
			cfg := &Config{}
			t.Setenv(c.key, c.value)
			err := cfg.loadFromEnv()
			if err == nil {
				t.Fatalf("%s=%q was accepted; want an error", c.key, c.value)
			}
			if !strings.HasPrefix(err.Error(), c.key+":") {
				t.Errorf("error does not name the variable: %v", err)
			}
		})
	}
}

// The spellings strconv.ParseBool and time.ParseDuration do understand keep working, and mean what they say.
func TestTheBooleanSpellingsGoUnderstandAreHonoured(t *testing.T) {
	for _, c := range []struct {
		value string
		want  bool
	}{{"true", true}, {"True", true}, {"TRUE", true}, {"1", true}, {"t", true},
		{"false", false}, {"False", false}, {"0", false}, {"f", false}} {
		t.Run(c.value, func(t *testing.T) {
			cfg := &Config{}
			t.Setenv("AUTH_ENABLED", c.value)
			if err := cfg.loadFromEnv(); err != nil {
				t.Fatalf("AUTH_ENABLED=%q: %v", c.value, err)
			}
			if cfg.Auth.Enabled != c.want {
				t.Errorf("AUTH_ENABLED=%q gave %v; want %v", c.value, cfg.Auth.Enabled, c.want)
			}
		})
	}
}
