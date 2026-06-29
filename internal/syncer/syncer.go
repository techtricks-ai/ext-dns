package syncer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"docker-external-dns/internal/models"
	"docker-external-dns/internal/providers"
	"docker-external-dns/internal/sources"
)

type Syncer struct {
	source        sources.Source
	provider      providers.Provider
	interval      time.Duration
	domainFilters []string
	identifier     string
	registryType   models.RecordType
	proxySupported bool
}

func NewSyncer(source sources.Source, provider providers.Provider, interval time.Duration, domainFilters []string, identifier string, registryType models.RecordType, proxySupported bool) *Syncer {
	return &Syncer{
		source:         source,
		provider:       provider,
		interval:       interval,
		domainFilters:  domainFilters,
		identifier:     identifier,
		registryType:   registryType,
		proxySupported: proxySupported,
	}
}

func (s *Syncer) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	slog.Info("Syncer started", "interval", s.interval)

	// Run once immediately
	s.sync(ctx)

	for {
		select {
		case <-ticker.C:
			s.sync(ctx)
		case <-ctx.Done():
			slog.Info("Syncer stopping")
			return
		}
	}
}

func (s *Syncer) sync(ctx context.Context) {
	slog.Debug("Starting sync cycle")

	desired, err := s.source.GetRecords(ctx)
	if err != nil {
		slog.Error("Failed to get desired records from source", "error", err)
		return
	}
	
	// Filter out unsupported record types
	supportedTypes := s.provider.SupportedRecordTypes()
	filteredDesired := make([]*models.Record, 0)
	for _, rec := range desired {
		supported := false
		for _, st := range supportedTypes {
			if rec.Type == st {
				supported = true
				break
			}
		}
		if supported {
			filteredDesired = append(filteredDesired, rec)
		} else {
			slog.Debug("Ignoring record type not supported by provider", "type", rec.Type, "key", rec.Key())
		}
	}
	desired = s.applyDomainFilter(filteredDesired)
	
	// If the provider doesn't support proxying (e.g. Pi-hole), force it to false
	// so that it doesn't cause false-positive update loops
	if !s.proxySupported {
		for _, r := range desired {
			r.Proxy = false
		}
	}

	// Inject ownership records into desired
	uniqueDomains := make(map[string]bool)
	for _, r := range desired {
		uniqueDomains[r.Name] = true
	}
	for domain := range uniqueDomains {
		regName := "ext-dns-" + domain
		target := s.identifier
		if s.registryType == models.TypeTXT {
			target = fmt.Sprintf(`"%s"`, s.identifier)
		}
		desired = append(desired, &models.Record{
			Type:   s.registryType,
			Name:   regName,
			Target: target,
		})
	}

	actual, err := s.provider.GetRecords(ctx)
	if err != nil {
		slog.Error("Failed to get actual records from provider", "error", err)
		return
	}
	actual = s.applyDomainFilter(actual)

	// Filter actual using Registry pattern
	ownedDomains := make(map[string]bool)
	for _, r := range actual {
		if r.Type == s.registryType && strings.HasPrefix(r.Name, "ext-dns-") {
			target := strings.Trim(r.Target, `"`)
			if target == s.identifier {
				domain := strings.TrimPrefix(r.Name, "ext-dns-")
				ownedDomains[domain] = true
			}
		}
	}

	var managedActual []*models.Record
	for _, r := range actual {
		domain := r.Name
		if strings.HasPrefix(domain, "ext-dns-") {
			domain = strings.TrimPrefix(domain, "ext-dns-")
		}
		if ownedDomains[domain] {
			managedActual = append(managedActual, r)
		}
	}
	actual = managedActual

	desiredMap := make(map[string]*models.Record)
	for _, r := range desired {
		desiredMap[r.Key()] = r
	}

	actualMap := make(map[string]*models.Record)
	for _, r := range actual {
		actualMap[r.Key()] = r
	}

	// Compute differences
	var toCreate []*models.Record
	var toDelete []*models.Record
	type updateOp struct {
		oldRecord *models.Record
		newRecord *models.Record
	}
	var toUpdate []updateOp

	for key, desiredRecord := range desiredMap {
		if actualRecord, exists := actualMap[key]; exists {
			if !desiredRecord.HasSameValue(actualRecord) {
				slog.Debug("Record difference detected", "key", key, "desired", fmt.Sprintf("%+v", desiredRecord), "actual", fmt.Sprintf("%+v", actualRecord))
				// ID needs to be copied so the provider knows what to update
				desiredRecord.ID = actualRecord.ID
				toUpdate = append(toUpdate, updateOp{oldRecord: actualRecord, newRecord: desiredRecord})
			}
		} else {
			toCreate = append(toCreate, desiredRecord)
		}
	}

	for key, actualRecord := range actualMap {
		if _, exists := desiredMap[key]; !exists {
			toDelete = append(toDelete, actualRecord)
		}
	}

	// Apply changes
	for _, record := range toCreate {
		if err := s.provider.CreateRecord(ctx, record); err != nil {
			slog.Error("Failed to create record", "key", record.Key(), "error", err)
		} else {
			slog.Info("Created record", "key", record.Key(), "target", record.Target)
		}
	}

	for _, op := range toUpdate {
		if err := s.provider.UpdateRecord(ctx, op.oldRecord, op.newRecord); err != nil {
			slog.Error("Failed to update record", "key", op.newRecord.Key(), "error", err)
		} else {
			slog.Info("Updated record", "key", op.newRecord.Key(), "target", op.newRecord.Target)
		}
	}

	for _, record := range toDelete {
		if err := s.provider.DeleteRecord(ctx, record); err != nil {
			slog.Error("Failed to delete record", "key", record.Key(), "error", err)
		} else {
			slog.Info("Deleted record", "key", record.Key())
		}
	}

	slog.Debug("Finished sync cycle",
		"created", len(toCreate),
		"updated", len(toUpdate),
		"deleted", len(toDelete),
	)
}

func (s *Syncer) applyDomainFilter(records []*models.Record) []*models.Record {
	if len(s.domainFilters) == 0 {
		return records
	}

	var filtered []*models.Record
	for _, r := range records {
		match := false
		name := "." + r.Name
		for _, filter := range s.domainFilters {
			f := "." + filter
			if strings.HasSuffix(name, f) {
				match = true
				break
			}
		}
		if match {
			filtered = append(filtered, r)
		} else {
			slog.Debug("Record filtered out by domain filter", "name", r.Name)
		}
	}
	return filtered
}
