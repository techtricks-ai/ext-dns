package models

import (
	"fmt"
	"strings"
)

type RecordType string

const (
	TypeA     RecordType = "A"
	TypeCNAME RecordType = "CNAME"
	TypeMX    RecordType = "MX"
	TypeNS    RecordType = "NS"
	TypeTXT   RecordType = "TXT"
)

// Record represents a normalized DNS record
type Record struct {
	ID       string     // The ID assigned by the provider (empty if it's from a Source)
	Type     RecordType // A, CNAME, MX, NS
	Name     string     // The FQDN of the record
	Target   string     // The IP or target FQDN
	Proxy    bool       // Cloudflare specific proxy flag
	Priority uint16     // MX record priority
}

// Key returns a unique string for the record identity (Type + Name)
func (r *Record) Key() string {
	return fmt.Sprintf("%s-%s", r.Type, r.Name)
}

// HasSameValue compares if the values of another record match this one (ignores ID)
func (r *Record) HasSameValue(other *Record) bool {
	return r.Type == other.Type &&
		strings.EqualFold(strings.TrimSpace(r.Name), strings.TrimSpace(other.Name)) &&
		strings.EqualFold(strings.TrimSpace(r.Target), strings.TrimSpace(other.Target)) &&
		r.Proxy == other.Proxy &&
		r.Priority == other.Priority
}
