package pihole

import (
	"context"
	"fmt"
	"log/slog"

	"docker-external-dns/internal/models"
)

type piholeAPI interface {
	Initialize(ctx context.Context) error
	GetRecords(ctx context.Context) ([]*models.Record, error)
	CreateRecord(ctx context.Context, record *models.Record) error
	UpdateRecord(ctx context.Context, oldRecord *models.Record, newRecord *models.Record) error
	DeleteRecord(ctx context.Context, record *models.Record) error
}

type PiholeProvider struct {
	client piholeAPI
}

func NewPiholeProvider(url, token, apiVersion, password string, skipVerify bool) *PiholeProvider {
	var client piholeAPI
	if apiVersion == "6" {
		slog.Info("Initializing Pi-hole provider with v6 API")
		client = newPiholeClientV6(url, password, skipVerify)
	} else {
		slog.Info("Initializing Pi-hole provider with v5 API")
		client = newPiholeClientV5(url, token, skipVerify)
	}

	return &PiholeProvider{
		client: client,
	}
}

func (p *PiholeProvider) SupportedRecordTypes() []models.RecordType {
	return []models.RecordType{models.TypeA, models.TypeCNAME}
}

func (p *PiholeProvider) Initialize(ctx context.Context) error {
	if p.client == nil {
		return fmt.Errorf("pihole client is nil")
	}
	return p.client.Initialize(ctx)
}

func (p *PiholeProvider) GetRecords(ctx context.Context) ([]*models.Record, error) {
	return p.client.GetRecords(ctx)
}

func (p *PiholeProvider) CreateRecord(ctx context.Context, record *models.Record) error {
	return p.client.CreateRecord(ctx, record)
}

func (p *PiholeProvider) UpdateRecord(ctx context.Context, oldRecord *models.Record, newRecord *models.Record) error {
	return p.client.UpdateRecord(ctx, oldRecord, newRecord)
}

func (p *PiholeProvider) DeleteRecord(ctx context.Context, record *models.Record) error {
	return p.client.DeleteRecord(ctx, record)
}
