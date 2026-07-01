package models

import (
	"testing"
)

func TestRecord_Key(t *testing.T) {
	tests := []struct {
		name     string
		record   Record
		expected string
	}{
		{
			name: "Type A record key",
			record: Record{
				Type: TypeA,
				Name: "example.com",
			},
			expected: "A-example.com",
		},
		{
			name: "Type CNAME record key",
			record: Record{
				Type: TypeCNAME,
				Name: "sub.example.com",
			},
			expected: "CNAME-sub.example.com",
		},
		{
			name: "Type MX record key",
			record: Record{
				Type: TypeMX,
				Name: "mail.example.com",
			},
			expected: "MX-mail.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.record.Key()
			if result != tt.expected {
				t.Errorf("expected Key() to be %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRecord_HasSameValue(t *testing.T) {
	tests := []struct {
		name     string
		record1  Record
		record2  Record
		expected bool
	}{
		{
			name: "identical records",
			record1: Record{
				Type:     TypeA,
				Name:     "example.com",
				Target:   "192.168.1.1",
				Proxy:    true,
				Priority: 0,
			},
			record2: Record{
				Type:     TypeA,
				Name:     "example.com",
				Target:   "192.168.1.1",
				Proxy:    true,
				Priority: 0,
			},
			expected: true,
		},
		{
			name: "identical except for ID",
			record1: Record{
				ID:       "id-123",
				Type:     TypeA,
				Name:     "example.com",
				Target:   "192.168.1.1",
				Proxy:    true,
				Priority: 0,
			},
			record2: Record{
				ID:       "id-456",
				Type:     TypeA,
				Name:     "example.com",
				Target:   "192.168.1.1",
				Proxy:    true,
				Priority: 0,
			},
			expected: true,
		},
		{
			name: "case insensitivity in Name and Target",
			record1: Record{
				Type:   TypeCNAME,
				Name:   "EXAMPLE.com",
				Target: "Target.Example.Com",
			},
			record2: Record{
				Type:   TypeCNAME,
				Name:   "example.com",
				Target: "target.example.com",
			},
			expected: true,
		},
		{
			name: "trim spacing in Name and Target",
			record1: Record{
				Type:   TypeA,
				Name:   " example.com  ",
				Target: " 192.168.1.1 ",
			},
			record2: Record{
				Type:   TypeA,
				Name:   "example.com",
				Target: "192.168.1.1",
			},
			expected: true,
		},
		{
			name: "different types",
			record1: Record{
				Type:   TypeA,
				Name:   "example.com",
				Target: "192.168.1.1",
			},
			record2: Record{
				Type:   TypeCNAME,
				Name:   "example.com",
				Target: "192.168.1.1",
			},
			expected: false,
		},
		{
			name: "different targets",
			record1: Record{
				Type:   TypeA,
				Name:   "example.com",
				Target: "192.168.1.1",
			},
			record2: Record{
				Type:   TypeA,
				Name:   "example.com",
				Target: "192.168.1.2",
			},
			expected: false,
		},
		{
			name: "different proxy status",
			record1: Record{
				Type:   TypeA,
				Name:   "example.com",
				Target: "192.168.1.1",
				Proxy:  true,
			},
			record2: Record{
				Type:   TypeA,
				Name:   "example.com",
				Target: "192.168.1.1",
				Proxy:  false,
			},
			expected: false,
		},
		{
			name: "different priorities",
			record1: Record{
				Type:     TypeMX,
				Name:     "example.com",
				Target:   "mail.example.com",
				Priority: 10,
			},
			record2: Record{
				Type:     TypeMX,
				Name:     "example.com",
				Target:   "mail.example.com",
				Priority: 20,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.record1.HasSameValue(&tt.record2)
			if result != tt.expected {
				t.Errorf("expected HasSameValue() to be %v, got %v for comparison of %+v and %+v", tt.expected, result, tt.record1, tt.record2)
			}
		})
	}
}
