package traefik

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"

	"docker-external-dns/internal/config"
	"docker-external-dns/internal/models"
)

type endpoint struct {
	apiURL     string
	targetIP   string
	username   string
	password   string
	skipVerify bool
	client     *http.Client
}

type TraefikSource struct {
	endpoints []endpoint
}

func NewTraefikSource(configs []config.TraefikConfig) *TraefikSource {
	var endpoints []endpoint
	for _, cfg := range configs {
		client := &http.Client{}
		if cfg.SkipVerify {
			client.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
		endpoints = append(endpoints, endpoint{
			apiURL:     cfg.ApiURL,
			targetIP:   cfg.TargetIP,
			username:   cfg.Username,
			password:   cfg.Password,
			skipVerify: cfg.SkipVerify,
			client:     client,
		})
	}
	return &TraefikSource{endpoints: endpoints}
}

func (s *TraefikSource) Initialize(ctx context.Context) error {
	if len(s.endpoints) == 0 {
		return fmt.Errorf("no traefik endpoints configured")
	}
	for i, ep := range s.endpoints {
		if ep.apiURL == "" || ep.targetIP == "" {
			return fmt.Errorf("traefik api URL and target IP must be provided for endpoint index %d", i)
		}
	}
	slog.Info("Traefik source initialized successfully", "endpoints_configured", len(s.endpoints))
	return nil
}

type Router struct {
	Rule   string `json:"rule"`
	Status string `json:"status"`
}

// Regex to extract domains from rules like Host(`example.com`) or HostSNI(`example.com`)
var hostRegex = regexp.MustCompile(`Host(?:SNI)?\(\x60([^\x60]+)\x60\)`)

func (s *TraefikSource) GetRecords(ctx context.Context) ([]*models.Record, error) {
	var allRecords []*models.Record
	seen := make(map[string]bool)

	for _, ep := range s.endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep.apiURL+"/api/http/routers", nil)
		if err != nil {
			slog.Warn("Failed to create request for traefik endpoint", "url", ep.apiURL, "error", err)
			continue
		}

		if ep.username != "" && ep.password != "" {
			req.SetBasicAuth(ep.username, ep.password)
		}

		resp, err := ep.client.Do(req)
		if err != nil {
			slog.Warn("Failed to execute request for traefik endpoint", "url", ep.apiURL, "error", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			slog.Warn("Unexpected status code from traefik endpoint", "url", ep.apiURL, "status", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		var routers []Router
		if err := json.NewDecoder(resp.Body).Decode(&routers); err != nil {
			slog.Warn("Failed to decode response from traefik endpoint", "url", ep.apiURL, "error", err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		var endpointRecordsCount int
		for _, router := range routers {
			if router.Status != "enabled" {
				continue
			}

			matches := hostRegex.FindAllStringSubmatch(router.Rule, -1)
			for _, match := range matches {
				if len(match) > 1 {
					domain := match[1]
					// We use seen map so if multiple endpoints report the same domain, we only sync the first one we find
					if !seen[domain] {
						seen[domain] = true
						allRecords = append(allRecords, &models.Record{
							Type:   models.TypeA,
							Name:   domain,
							Target: ep.targetIP,
							Proxy:  false,
						})
						endpointRecordsCount++
					}
				}
			}
		}
		slog.Debug("Fetched records from Traefik endpoint", "url", ep.apiURL, "count", endpointRecordsCount)
	}

	return allRecords, nil
}
