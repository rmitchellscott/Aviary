package security

import (
	"net"
	"os"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		blockPrivate bool
		blockedDomains string
		wantErr     bool
		errType     error
	}{
		{
			name:    "valid https URL",
			url:     "https://example.com/file.pdf",
			wantErr: false,
		},
		{
			name:    "valid http URL",
			url:     "http://example.com/file.pdf",
			wantErr: false,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
			errType: ErrEmptyURL,
		},
		{
			name:    "invalid scheme",
			url:     "ftp://example.com/file.pdf",
			wantErr: true,
			errType: ErrInvalidScheme,
		},
		{
			name:    "missing hostname",
			url:     "https:///file.pdf",
			wantErr: true,
			errType: ErrInvalidURL,
		},
		{
			name:         "localhost blocked when private IPs blocked",
			url:          "http://localhost/file.pdf",
			blockPrivate: true,
			wantErr:      true,
			errType:      ErrPrivateIP,
		},
		{
			name:         "localhost allowed when private IPs not blocked",
			url:          "http://localhost/file.pdf",
			blockPrivate: false,
			wantErr:      false,
		},
		{
			name:         "127.0.0.1 blocked when private IPs blocked",
			url:          "http://127.0.0.1/file.pdf",
			blockPrivate: true,
			wantErr:      true,
			errType:      ErrPrivateIP,
		},
		{
			name:         "RFC1918 10.x blocked",
			url:          "http://10.0.0.1/file.pdf",
			blockPrivate: true,
			wantErr:      true,
			errType:      ErrPrivateIP,
		},
		{
			name:         "RFC1918 172.16.x blocked",
			url:          "http://172.16.0.1/file.pdf",
			blockPrivate: true,
			wantErr:      true,
			errType:      ErrPrivateIP,
		},
		{
			name:         "RFC1918 192.168.x blocked",
			url:          "http://192.168.1.1/file.pdf",
			blockPrivate: true,
			wantErr:      true,
			errType:      ErrPrivateIP,
		},
		{
			name:         "Link-local 169.254.x blocked",
			url:          "http://169.254.1.1/file.pdf",
			blockPrivate: true,
			wantErr:      true,
			errType:      ErrPrivateIP,
		},
		{
			name:           "blocked domain",
			url:            "https://evil.com/file.pdf",
			blockedDomains: "evil.com,bad.org",
			wantErr:        true,
			errType:        ErrBlockedDomain,
		},
		{
			name:           "blocked subdomain",
			url:            "https://sub.evil.com/file.pdf",
			blockedDomains: "evil.com",
			wantErr:        true,
			errType:        ErrBlockedDomain,
		},
		{
			name:           "allowed domain",
			url:            "https://good.com/file.pdf",
			blockedDomains: "evil.com,bad.org",
			wantErr:        false,
		},
		{
			name:         "IPv6 loopback blocked",
			url:          "http://[::1]/file.pdf",
			blockPrivate: true,
			wantErr:      true,
			errType:      ErrPrivateIP,
		},
		{
			name:         "IPv6 link-local blocked",
			url:          "http://[fe80::1]/file.pdf",
			blockPrivate: true,
			wantErr:      true,
			errType:      ErrPrivateIP,
		},
		{
			name:         "IPv6 ULA blocked",
			url:          "http://[fd00::1]/file.pdf",
			blockPrivate: true,
			wantErr:      true,
			errType:      ErrPrivateIP,
		},
		{
			name:         "public IP allowed",
			url:          "http://8.8.8.8/file.pdf",
			blockPrivate: true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.blockPrivate {
				os.Setenv("BLOCK_PRIVATE_IPS", "true")
			} else {
				os.Unsetenv("BLOCK_PRIVATE_IPS")
			}
			
			if tt.blockedDomains != "" {
				os.Setenv("BLOCKED_DOMAINS", tt.blockedDomains)
			} else {
				os.Unsetenv("BLOCKED_DOMAINS")
			}

			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			
			if tt.wantErr && tt.errType != nil {
				switch tt.errType {
				case ErrEmptyURL:
					if err != ErrEmptyURL {
						t.Errorf("ValidateURL() expected ErrEmptyURL, got %v", err)
					}
				case ErrInvalidScheme:
					if err != ErrInvalidScheme {
						t.Errorf("ValidateURL() expected ErrInvalidScheme, got %v", err)
					}
				}
			}
			
			os.Unsetenv("BLOCK_PRIVATE_IPS")
			os.Unsetenv("BLOCKED_DOMAINS")
		})
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		wantPrivate bool
	}{
		{"loopback IPv4", "127.0.0.1", true},
		{"loopback IPv4 range", "127.0.0.2", true},
		{"RFC1918 10.x", "10.0.0.1", true},
		{"RFC1918 172.16.x", "172.16.0.1", true},
		{"RFC1918 172.31.x", "172.31.255.255", true},
		{"RFC1918 192.168.x", "192.168.1.1", true},
		{"link-local", "169.254.1.1", true},
		{"carrier-grade NAT", "100.64.0.1", true},
		{"documentation 1", "198.18.0.1", true},
		{"documentation 2", "198.51.100.1", true},
		{"documentation 3", "203.0.113.1", true},
		{"public Google DNS", "8.8.8.8", false},
		{"public Cloudflare DNS", "1.1.1.1", false},
		{"IPv6 loopback", "::1", true},
		{"IPv6 ULA fc", "fc00::1", true},
		{"IPv6 ULA fd", "fd00::1", true},
		{"IPv6 link-local", "fe80::1", true},
		{"IPv6 public", "2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("Failed to parse IP: %s", tt.ip)
			}
			
			got := isPrivateIP(ip)
			if got != tt.wantPrivate {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.wantPrivate)
			}
		})
	}
}