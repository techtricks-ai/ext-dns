package cloudflare

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	cf "github.com/cloudflare/cloudflare-go"
	"docker-external-dns/internal/models"
)

type CloudflareProvider struct {
	api        *cf.API
	token      string
	identifier string
	zones      []cf.Zone
}

func NewCloudflareProvider(token, identifier string) *CloudflareProvider {
	return &CloudflareProvider{
		token:      token,
		identifier: identifier,
	}
}

func (p *CloudflareProvider) SupportedRecordTypes() []models.RecordType {
	return []models.RecordType{models.TypeA, models.TypeCNAME, models.TypeTXT, models.TypeMX, models.TypeNS}
}

func (p *CloudflareProvider) Initialize(ctx context.Context) error {
	api, err := cf.NewWithAPIToken(p.token)
	if err != nil {
		return fmt.Errorf("failed to create cloudflare client: %w", err)
	}
	p.api = api

	// Fetch all zones once during initialization
	// In a dynamic setup, you might want to refresh this
	zones, err := p.api.ListZones(ctx)
	if err != nil {
		return fmt.Errorf("failed to list cloudflare zones: %w", err)
	}
	p.zones = zones
	slog.Info("Cloudflare initialized", "zones", len(p.zones))

	return nil
}

func (p *CloudflareProvider) getZoneForName(name string) (cf.Zone, bool) {
	for _, z := range p.zones {
		if strings.HasSuffix(name, z.Name) {
			return z, true
		}
	}
	return cf.Zone{}, false
}

func (p *CloudflareProvider) GetRecords(ctx context.Context) ([]*models.Record, error) {
	var allRecords []*models.Record

	for _, z := range p.zones {
		rc := cf.ResourceContainer{
			Level:      cf.ZoneRouteLevel,
			Identifier: z.ID,
		}

		params := cf.ListDNSRecordsParams{
			ResultInfo: cf.ResultInfo{
				Page:    1,
				PerPage: 500,
			},
		}

		for {
			records, resultInfo, err := p.api.ListDNSRecords(ctx, &rc, params)
			if err != nil {
				slog.Error("Failed to list DNS records", "zone", z.Name, "error", err)
				break
			}

			for _, r := range records {
				rec := &models.Record{
					ID:   r.ID,
					Type: models.RecordType(r.Type),
					Name: r.Name,
				}

				switch r.Type {
				case "A", "CNAME", "NS", "TXT":
					rec.Target = r.Content
				case "MX":
					rec.Target = r.Content
					if r.Priority != nil {
						rec.Priority = *r.Priority
					}
				}

				if r.Proxied != nil {
					rec.Proxy = *r.Proxied
				}

				allRecords = append(allRecords, rec)
			}

			if resultInfo.Page >= resultInfo.TotalPages {
				break
			}
			params.ResultInfo.Page++
		}
	}

	return allRecords, nil
}

func (p *CloudflareProvider) CreateRecord(ctx context.Context, record *models.Record) error {
	z, ok := p.getZoneForName(record.Name)
	if !ok {
		return fmt.Errorf("no zone found for name %s", record.Name)
	}

	rc := cf.ResourceContainer{Level: cf.ZoneRouteLevel, Identifier: z.ID}
	
	params := cf.CreateDNSRecordParams{
		Type:    string(record.Type),
		Name:    record.Name,
		Content: record.Target,
		Proxied: cf.BoolPtr(record.Proxy),
	}

	if record.Type == models.TypeMX {
		params.Priority = cf.Uint16Ptr(record.Priority)
	}

	_, err := p.api.CreateDNSRecord(ctx, &rc, params)
	return err
}

func (p *CloudflareProvider) UpdateRecord(ctx context.Context, oldRecord *models.Record, newRecord *models.Record) error {
	if newRecord.ID == "" {
		return fmt.Errorf("cannot update record without ID")
	}

	z, ok := p.getZoneForName(newRecord.Name)
	if !ok {
		return fmt.Errorf("no zone found for name %s", newRecord.Name)
	}

	rc := cf.ResourceContainer{Level: cf.ZoneRouteLevel, Identifier: z.ID}
	
	params := cf.UpdateDNSRecordParams{
		ID:      newRecord.ID,
		Type:    string(newRecord.Type),
		Name:    newRecord.Name,
		Content: newRecord.Target,
		Proxied: cf.BoolPtr(newRecord.Proxy),
	}

	if newRecord.Type == models.TypeMX {
		params.Priority = cf.Uint16Ptr(newRecord.Priority)
	}

	_, err := p.api.UpdateDNSRecord(ctx, &rc, params)
	return err
}

func (p *CloudflareProvider) DeleteRecord(ctx context.Context, record *models.Record) error {
	if record.ID == "" {
		return fmt.Errorf("cannot delete record without ID")
	}

	z, ok := p.getZoneForName(record.Name)
	if !ok {
		return fmt.Errorf("no zone found for name %s", record.Name)
	}

	rc := cf.ResourceContainer{Level: cf.ZoneRouteLevel, Identifier: z.ID}
	return p.api.DeleteDNSRecord(ctx, &rc, record.ID)
}
