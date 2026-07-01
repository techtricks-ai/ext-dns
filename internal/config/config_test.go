package config

import (
	"os"
	"reflect"
	"testing"
)

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	originalVal, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset env %s: %v", key, err)
	}
	if exists {
		t.Cleanup(func() {
			os.Setenv(key, originalVal)
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single item without spaces",
			input:    "item1",
			expected: []string{"item1"},
		},
		{
			name:     "single item with spaces",
			input:    "  item1  ",
			expected: []string{"item1"},
		},
		{
			name:     "multiple items",
			input:    "item1,item2,item3",
			expected: []string{"item1", "item2", "item3"},
		},
		{
			name:     "multiple items with spaces and empty spots",
			input:    " item1 , , item2, ",
			expected: []string{"item1", "item2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrim(tt.input)
			// For empty slice, allow nil or empty slice comparison
			if len(result) == 0 && len(tt.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected splitAndTrim(%q) to be %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	// Unset environment variables to ensure fallbacks are triggered
	unsetEnv(t, "DNS_PROVIDER")
	unsetEnv(t, "DNS_SOURCE")
	unsetEnv(t, "INTERVAL_SECONDS")
	unsetEnv(t, "IDENTIFIER")
	unsetEnv(t, "LOG_LEVEL")
	unsetEnv(t, "DOMAIN_FILTER")
	unsetEnv(t, "CF_API_TOKEN")
	unsetEnv(t, "PIHOLE_URL")
	unsetEnv(t, "PIHOLE_API_TOKEN")
	unsetEnv(t, "PIHOLE_API_VERSION")
	unsetEnv(t, "PIHOLE_PASSWORD")
	unsetEnv(t, "PIHOLE_SKIP_VERIFY")
	unsetEnv(t, "DOCKER_HOST")
	unsetEnv(t, "TRAEFIK_INSTANCES")

	cfg := LoadConfig()

	if cfg.Provider != "cloudflare" {
		t.Errorf("expected default provider to be cloudflare, got %q", cfg.Provider)
	}
	if cfg.Source != "docker" {
		t.Errorf("expected default source to be docker, got %q", cfg.Source)
	}
	if cfg.IntervalSeconds != 60 {
		t.Errorf("expected default interval seconds to be 60, got %d", cfg.IntervalSeconds)
	}
	if cfg.Identifier != "docker-external-dns" {
		t.Errorf("expected default identifier to be docker-external-dns, got %q", cfg.Identifier)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level to be info, got %q", cfg.LogLevel)
	}
	if len(cfg.DockerHosts) != 1 || cfg.DockerHosts[0] != "unix:///var/run/docker.sock" {
		t.Errorf("expected default Docker host to be unix:///var/run/docker.sock, got %v", cfg.DockerHosts)
	}
}

func TestLoadConfig_CustomEnvironment(t *testing.T) {
	t.Setenv("DNS_PROVIDER", "pihole")
	t.Setenv("DNS_SOURCE", "traefik")
	t.Setenv("INTERVAL_SECONDS", "30")
	t.Setenv("IDENTIFIER", "my-custom-identifier")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("DOMAIN_FILTER", "example.com, test.com")
	t.Setenv("PIHOLE_URL", "http://192.168.1.5")
	t.Setenv("PIHOLE_API_VERSION", "5")
	t.Setenv("PIHOLE_API_TOKEN", "my-secret-token")
	t.Setenv("PIHOLE_SKIP_VERIFY", "true")
	t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock, tcp://192.168.1.10:2375")

	cfg := LoadConfig()

	if cfg.Provider != "pihole" {
		t.Errorf("expected provider to be pihole, got %q", cfg.Provider)
	}
	if cfg.Source != "traefik" {
		t.Errorf("expected source to be traefik, got %q", cfg.Source)
	}
	if cfg.IntervalSeconds != 30 {
		t.Errorf("expected interval seconds to be 30, got %d", cfg.IntervalSeconds)
	}
	if cfg.Identifier != "my-custom-identifier" {
		t.Errorf("expected identifier to be my-custom-identifier, got %q", cfg.Identifier)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log level to be debug, got %q", cfg.LogLevel)
	}
	expectedDomains := []string{"example.com", "test.com"}
	if !reflect.DeepEqual(cfg.DomainFilters, expectedDomains) {
		t.Errorf("expected domain filters to be %v, got %v", expectedDomains, cfg.DomainFilters)
	}
	if cfg.PiholeURL != "http://192.168.1.5" {
		t.Errorf("expected pihole url to be http://192.168.1.5, got %q", cfg.PiholeURL)
	}
	if cfg.PiholeApiVersion != "5" {
		t.Errorf("expected pihole api version to be 5, got %q", cfg.PiholeApiVersion)
	}
	if cfg.PiholeApiToken != "my-secret-token" {
		t.Errorf("expected pihole api token to be my-secret-token, got %q", cfg.PiholeApiToken)
	}
	if !cfg.PiholeSkipVerify {
		t.Errorf("expected pihole skip verify to be true, got %v", cfg.PiholeSkipVerify)
	}
	expectedDockerHosts := []string{"unix:///var/run/docker.sock", "tcp://192.168.1.10:2375"}
	if !reflect.DeepEqual(cfg.DockerHosts, expectedDockerHosts) {
		t.Errorf("expected docker hosts to be %v, got %v", expectedDockerHosts, cfg.DockerHosts)
	}
}

func TestLoadConfig_TraefikInstances(t *testing.T) {
	t.Setenv("TRAEFIK_INSTANCES", `[{"url":"http://traefik1:8080","target":"192.168.1.100","skip_verify":true},{"url":"http://traefik2:8080","target":"192.168.1.101","username":"admin","password":"pwd"}]`)

	cfg := LoadConfig()

	if len(cfg.TraefikConfigs) != 2 {
		t.Fatalf("expected 2 Traefik configs, got %d", len(cfg.TraefikConfigs))
	}

	inst1 := cfg.TraefikConfigs[0]
	if inst1.ApiURL != "http://traefik1:8080" || inst1.TargetIP != "192.168.1.100" || !inst1.SkipVerify {
		t.Errorf("unexpected configuration for instance 1: %+v", inst1)
	}

	inst2 := cfg.TraefikConfigs[1]
	if inst2.ApiURL != "http://traefik2:8080" || inst2.TargetIP != "192.168.1.101" || inst2.Username != "admin" || inst2.Password != "pwd" || inst2.SkipVerify {
		t.Errorf("unexpected configuration for instance 2: %+v", inst2)
	}
}
