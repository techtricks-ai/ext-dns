package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Provider        string
	Source          string
	IntervalSeconds int
	Identifier      string
	LogLevel        string
	DomainFilters   []string

	// Cloudflare
	CFApiToken string

	// Pihole
	PiholeURL        string
	PiholeApiToken   string
	PiholeApiVersion string
	PiholePassword   string
	PiholeSkipVerify bool

	// Traefik
	TraefikConfigs []TraefikConfig

	// Docker (now supports multiple hosts as a comma-separated list)
	DockerHosts []string
}

type TraefikConfig struct {
	ApiURL     string
	TargetIP   string
	Username   string
	Password   string
	SkipVerify bool
}

type TraefikInstanceJSON struct {
	URL        string `json:"url"`
	Target     string `json:"target"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	SkipVerify bool   `json:"skip_verify"`
}

func LoadConfig() *Config {
	cfg := &Config{
		Provider:        getEnv("DNS_PROVIDER", "cloudflare"),
		Source:          getEnv("DNS_SOURCE", "docker"),
		IntervalSeconds: getEnvAsInt("INTERVAL_SECONDS", 60),
		Identifier:      getEnv("IDENTIFIER", "docker-external-dns"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		DomainFilters:   splitAndTrim(getEnv("DOMAIN_FILTER", "")),

		CFApiToken: getEnv("CF_API_TOKEN", ""),

		PiholeURL:        getEnv("PIHOLE_URL", ""),
		PiholeApiToken:   getEnv("PIHOLE_API_TOKEN", ""),
		PiholeApiVersion: getEnv("PIHOLE_API_VERSION", "6"),
		PiholePassword:   getEnv("PIHOLE_PASSWORD", ""),
		PiholeSkipVerify: strings.ToLower(getEnv("PIHOLE_SKIP_VERIFY", "false")) == "true",

		DockerHosts: splitAndTrim(getEnv("DOCKER_HOST", "unix:///var/run/docker.sock")),
	}

	traefikInstancesJSON := getEnv("TRAEFIK_INSTANCES", "")

	if traefikInstancesJSON != "" {
		var instances []TraefikInstanceJSON
		if err := json.Unmarshal([]byte(traefikInstancesJSON), &instances); err != nil {
			slog.Error("Failed to parse TRAEFIK_INSTANCES JSON", "error", err)
			os.Exit(1)
		}
		for _, inst := range instances {
			cfg.TraefikConfigs = append(cfg.TraefikConfigs, TraefikConfig{
				ApiURL:     inst.URL,
				TargetIP:   inst.Target,
				Username:   inst.Username,
				Password:   inst.Password,
				SkipVerify: inst.SkipVerify,
			})
		}
	} else {
		// Fallback to default
		cfg.TraefikConfigs = append(cfg.TraefikConfigs, TraefikConfig{
			ApiURL:     getEnv("TRAEFIK_API_URL", "http://localhost:8080"),
			TargetIP:   getEnv("TRAEFIK_TARGET_IP", "127.0.0.1"),
			Username:   getEnv("TRAEFIK_USERNAME", ""),
			Password:   getEnv("TRAEFIK_PASSWORD", ""),
			SkipVerify: strings.ToLower(getEnv("TRAEFIK_SKIP_VERIFY", "false")) == "true",
		})
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}

func splitAndTrim(value string) []string {
	if value == "" {
		return []string{}
	}
	var result []string
	parts := strings.Split(value, ",")
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
