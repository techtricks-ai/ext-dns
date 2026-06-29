package pihole

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"docker-external-dns/internal/models"
)

type piholeClientV6 struct {
	url      string
	password string
	session  string
	client   *http.Client
}

func newPiholeClientV6(url, password string, skipVerify bool) *piholeClientV6 {
	client := &http.Client{}
	if skipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}
	return &piholeClientV6{
		url:      url,
		password: password,
		client:   client,
	}
}

type authResponseV6 struct {
	Session struct {
		Valid bool   `json:"valid"`
		SID   string `json:"sid"`
	} `json:"session"`
}

type recordsResponseV6 struct {
	Config struct {
		DNS struct {
			Hosts        []string `json:"hosts"`
			CnameRecords []string `json:"cnameRecords"`
		} `json:"dns"`
	} `json:"config"`
}

func (p *piholeClientV6) Initialize(ctx context.Context) error {
	if p.url == "" || p.password == "" {
		return fmt.Errorf("pihole URL and password must be provided for v6")
	}
	return p.authenticate(ctx)
}

func (p *piholeClientV6) authenticate(ctx context.Context) error {
	authURL := fmt.Sprintf("%s/api/auth", p.url)
	payload := map[string]string{"password": p.password}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to authenticate to pihole v6, status: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var authRes authResponseV6
	if err := json.NewDecoder(resp.Body).Decode(&authRes); err != nil {
		return err
	}

	if authRes.Session.SID == "" {
		return fmt.Errorf("no SID returned from pihole auth")
	}

	p.session = authRes.Session.SID
	slog.Debug("Successfully authenticated with Pi-hole v6")
	return nil
}

func (p *piholeClientV6) doRequest(ctx context.Context, method, endpoint string, retryAuth bool) ([]byte, error) {
	reqURL := fmt.Sprintf("%s%s", p.url, endpoint)
	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if p.session != "" {
		req.Header.Set("X-FTL-SID", p.session)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized && retryAuth {
		slog.Warn("Pi-hole v6 session expired, re-authenticating")
		if err := p.authenticate(ctx); err == nil {
			return p.doRequest(ctx, method, endpoint, false)
		}
	}

	if resp.StatusCode >= 400 {
		return bodyBytes, fmt.Errorf("API error: %s", string(bodyBytes))
	}

	return bodyBytes, nil
}

func (p *piholeClientV6) GetRecords(ctx context.Context) ([]*models.Record, error) {
	body, err := p.doRequest(ctx, http.MethodGet, "/api/config/dns", true)
	if err != nil {
		return nil, fmt.Errorf("failed to get v6 records: %w", err)
	}

	var res recordsResponseV6
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	var records []*models.Record

	// hosts array format: "ip domain"
	for _, host := range res.Config.DNS.Hosts {
		parts := strings.Fields(host)
		if len(parts) >= 2 {
			records = append(records, &models.Record{
				Type:   models.TypeA,
				Name:   parts[1],
				Target: parts[0],
			})
		}
	}

	// cnameRecords array format: "domain,target"
	for _, cname := range res.Config.DNS.CnameRecords {
		parts := strings.Split(cname, ",")
		if len(parts) >= 2 {
			records = append(records, &models.Record{
				Type:   models.TypeCNAME,
				Name:   parts[0],
				Target: parts[1],
			})
		}
	}

	return records, nil
}

func (p *piholeClientV6) CreateRecord(ctx context.Context, record *models.Record) error {
	return p.applyRecord(ctx, http.MethodPut, record)
}

func (p *piholeClientV6) UpdateRecord(ctx context.Context, oldRecord *models.Record, newRecord *models.Record) error {
	if err := p.DeleteRecord(ctx, oldRecord); err != nil {
		slog.Warn("Failed to delete old pi-hole v6 record during update", "error", err)
	}
	return p.CreateRecord(ctx, newRecord)
}

func (p *piholeClientV6) DeleteRecord(ctx context.Context, record *models.Record) error {
	return p.applyRecord(ctx, http.MethodDelete, record)
}

func (p *piholeClientV6) applyRecord(ctx context.Context, method string, record *models.Record) error {
	var endpoint string

	switch record.Type {
	case models.TypeA:
		// Pihole v6 host endpoint format: /api/config/dns/hosts/{ip} {domain}
		pathParam := fmt.Sprintf("%s %s", record.Target, record.Name)
		endpoint = fmt.Sprintf("/api/config/dns/hosts/%s", pathParam)
	case models.TypeCNAME:
		// Pihole v6 cname endpoint format: /api/config/dns/cnameRecords/{domain},{target}
		pathParam := fmt.Sprintf("%s,%s", record.Name, record.Target)
		endpoint = fmt.Sprintf("/api/config/dns/cnameRecords/%s", pathParam)
	default:
		slog.Warn("Pi-hole v6 provider only supports A and CNAME", "type", record.Type)
		return nil
	}

	// The spaces and commas in the path might need exact URL encoding as expected by pihole
	_, err := p.doRequest(ctx, method, endpoint, true)
	if err != nil {
		// Pi-hole might return 400 if item already exists or doesn't exist, ignore these softly if deleting or adding
		if strings.Contains(err.Error(), "Item already present") || strings.Contains(err.Error(), "does not exist") {
			return nil
		}
		return err
	}
	return nil
}
