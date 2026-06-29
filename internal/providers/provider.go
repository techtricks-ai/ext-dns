package providers

import (
	"context"
	"docker-external-dns/internal/models"
)

// Provider represents an interface for manipulating external DNS records
type Provider interface {
	// Initialize performs any necessary setup (e.g. authentication)
	Initialize(ctx context.Context) error

	// SupportedRecordTypes returns a list of DNS record types supported by this provider
	SupportedRecordTypes() []models.RecordType

	// GetRecords fetches the current state of DNS records managed by this application
	GetRecords(ctx context.Context) ([]*models.Record, error)

	// CreateRecord creates a new DNS record
	CreateRecord(ctx context.Context, record *models.Record) error

	// UpdateRecord updates an existing DNS record
	UpdateRecord(ctx context.Context, oldRecord *models.Record, newRecord *models.Record) error

	// DeleteRecord deletes a DNS record
	DeleteRecord(ctx context.Context, record *models.Record) error
}
