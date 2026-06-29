package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"docker-external-dns/internal/models"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type DockerSource struct {
	clients    []*client.Client
	hosts      []string
	identifier string
}

func NewDockerSource(hosts []string, identifier string) *DockerSource {
	return &DockerSource{
		hosts:      hosts,
		identifier: identifier,
	}
}

func (s *DockerSource) Initialize(ctx context.Context) error {
	tlsVerify := os.Getenv("DOCKER_TLS_VERIFY") != ""
	certPath := os.Getenv("DOCKER_CERT_PATH")

	for _, host := range s.hosts {
		opts := []client.Opt{
			client.WithHost(host),
			client.WithAPIVersionNegotiation(),
		}

		// Only apply TLS configuration to remote TCP hosts
		if strings.HasPrefix(host, "tcp://") && tlsVerify && certPath != "" {
			ca := certPath + "/ca.pem"
			cert := certPath + "/cert.pem"
			key := certPath + "/key.pem"
			opts = append(opts, client.WithTLSClientConfig(ca, cert, key))
		}

		cli, err := client.NewClientWithOpts(opts...)
		if err != nil {
			slog.Warn("Failed to create docker client, skipping host", "host", host, "error", err)
			continue
		}
		s.clients = append(s.clients, cli)
	}

	if len(s.clients) == 0 {
		return fmt.Errorf("failed to connect to any of the provided docker hosts")
	}

	slog.Info("Docker source initialized successfully", "hosts_connected", len(s.clients))
	return nil
}

type labelEntry struct {
	Type     models.RecordType `json:"type"`
	Name     string            `json:"name"`
	Address  string            `json:"address"`
	Target   string            `json:"target"`
	Server   string            `json:"server"`
	Proxy    bool              `json:"proxy"`
	Priority uint16            `json:"priority"`
}

func (s *DockerSource) GetRecords(ctx context.Context) ([]*models.Record, error) {
	f := filters.NewArgs()
	f.Add("label", s.identifier)

	var allRecords []*models.Record

	for i, cli := range s.clients {
		containers, err := cli.ContainerList(ctx, types.ContainerListOptions{
			All:     true,
			Filters: f,
		})
		if err != nil {
			slog.Warn("Failed to list containers from host", "host", s.hosts[i], "error", err)
			continue
		}

		for _, c := range containers {
			labelValue := c.Labels[s.identifier]
			var entries []labelEntry
			if err := json.Unmarshal([]byte(labelValue), &entries); err != nil {
				slog.Warn("Failed to parse docker label as JSON", "container", c.ID, "error", err)
				continue
			}

			for _, e := range entries {
				var target string
				switch e.Type {
				case models.TypeA:
					target = e.Address
				case models.TypeCNAME:
					target = e.Target
				case models.TypeMX, models.TypeNS:
					target = e.Server
				default:
					slog.Warn("Unsupported record type in docker label", "container", c.ID, "type", e.Type)
					continue
				}

				allRecords = append(allRecords, &models.Record{
					Type:     e.Type,
					Name:     e.Name,
					Target:   target,
					Proxy:    e.Proxy,
					Priority: e.Priority,
				})
			}
		}
	}

	return allRecords, nil
}
