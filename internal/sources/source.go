package sources

import (
	"context"
	"docker-external-dns/internal/models"
)

// Source represents an interface for fetching desired DNS records
type Source interface {
	// Initialize performs any necessary setup
	Initialize(ctx context.Context) error

	// GetRecords returns the desired state of DNS records
	GetRecords(ctx context.Context) ([]*models.Record, error)
}
