package pihole

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"docker-external-dns/internal/models"
)

type piholeClientV5 struct {
	url    string
	token  string
	client *http.Client
}

func newPiholeClientV5(url, token string, skipVerify bool) *piholeClientV5 {
	client := &http.Client{}
	if skipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	return &piholeClientV5{
		url:    url,
		token:  token,
		client: client,
	}
}

func (p *piholeClientV5) Initialize(ctx context.Context) error {
	if p.url == "" || p.token == "" {
		return fmt.Errorf("pihole URL and token must be provided for v5")
	}
	return nil
}

type apiResponseV5 struct {
	Data [][]string `json:"data"`
}

func (p *piholeClientV5) GetRecords(ctx context.Context) ([]*models.Record, error) {
	var allRecords []*models.Record

	aReqURL := fmt.Sprintf("%s/admin/api.php?customdns&action=get&auth=%s", p.url, url.QueryEscape(p.token))
	if aResp, err := p.doGet(ctx, aReqURL); err == nil {
		for _, row := range aResp.Data {
			if len(row) == 2 {
				allRecords = append(allRecords, &models.Record{
					Type:   models.TypeA,
					Name:   row[0],
					Target: row[1],
				})
			}
		}
	} else {
		return nil, fmt.Errorf("failed to get A records: %w", err)
	}

	cReqURL := fmt.Sprintf("%s/admin/api.php?customcname&action=get&auth=%s", p.url, url.QueryEscape(p.token))
	if cResp, err := p.doGet(ctx, cReqURL); err == nil {
		for _, row := range cResp.Data {
			if len(row) == 2 {
				allRecords = append(allRecords, &models.Record{
					Type:   models.TypeCNAME,
					Name:   row[0],
					Target: row[1],
				})
			}
		}
	} else {
		return nil, fmt.Errorf("failed to get CNAME records: %w", err)
	}

	return allRecords, nil
}

func (p *piholeClientV5) doGet(ctx context.Context, reqURL string) (*apiResponseV5, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var res apiResponseV5
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (p *piholeClientV5) CreateRecord(ctx context.Context, record *models.Record) error {
	return p.modifyRecord(ctx, "add", record)
}

func (p *piholeClientV5) UpdateRecord(ctx context.Context, oldRecord *models.Record, newRecord *models.Record) error {
	// Pi-hole doesn't support atomic updates by ID, so we must delete the old and create the new.
	if err := p.DeleteRecord(ctx, oldRecord); err != nil {
		slog.Warn("Failed to delete old pi-hole record during update", "error", err)
	}
	return p.CreateRecord(ctx, newRecord)
}

func (p *piholeClientV5) DeleteRecord(ctx context.Context, record *models.Record) error {
	return p.modifyRecord(ctx, "delete", record)
}

func (p *piholeClientV5) modifyRecord(ctx context.Context, action string, record *models.Record) error {
	var endpoint string
	var query string

	switch record.Type {
	case models.TypeA:
		endpoint = "customdns"
		query = fmt.Sprintf("domain=%s&ip=%s", url.QueryEscape(record.Name), url.QueryEscape(record.Target))
	case models.TypeCNAME:
		endpoint = "customcname"
		query = fmt.Sprintf("domain=%s&target=%s", url.QueryEscape(record.Name), url.QueryEscape(record.Target))
	default:
		slog.Warn("Pi-hole v5 provider only supports A and CNAME records", "type", record.Type)
		return nil
	}

	reqURL := fmt.Sprintf("%s/admin/api.php?%s&action=%s&%s&auth=%s", p.url, endpoint, action, query, url.QueryEscape(p.token))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}
