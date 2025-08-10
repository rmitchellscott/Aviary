package security

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/rmitchellscott/aviary/internal/config"
)

var (
	ErrInvalidURL        = errors.New("invalid URL format")
	ErrInvalidScheme     = errors.New("URL scheme must be http or https")
	ErrPrivateIP         = errors.New("URL points to private/local IP address")
	ErrBlockedDomain     = errors.New("domain is in blocklist")
	ErrEmptyURL          = errors.New("URL cannot be empty")
	ErrIPResolutionFailed = errors.New("failed to resolve domain")
)

func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return ErrEmptyURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return ErrInvalidScheme
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return fmt.Errorf("%w: missing hostname", ErrInvalidURL)
	}

	blockedDomains := config.Get("BLOCKED_DOMAINS", "")
	if blockedDomains != "" {
		domains := strings.Split(blockedDomains, ",")
		for _, domain := range domains {
			domain = strings.TrimSpace(domain)
			if domain != "" && (hostname == domain || strings.HasSuffix(hostname, "."+domain)) {
				return fmt.Errorf("%w: %s", ErrBlockedDomain, hostname)
			}
		}
	}

	if config.Get("BLOCK_PRIVATE_IPS", "") == "true" {
		if err := checkPrivateIP(hostname); err != nil {
			return err
		}
	}

	return nil
}

func checkPrivateIP(hostname string) error {
	ip := net.ParseIP(hostname)
	if ip == nil {
		ips, err := net.LookupIP(hostname)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrIPResolutionFailed, err)
		}
		if len(ips) == 0 {
			return fmt.Errorf("%w: no IPs found for hostname", ErrIPResolutionFailed)
		}
		for _, resolvedIP := range ips {
			if isPrivateIP(resolvedIP) {
				return fmt.Errorf("%w: %s resolves to %s", ErrPrivateIP, hostname, resolvedIP.String())
			}
		}
	} else {
		if isPrivateIP(ip) {
			return fmt.Errorf("%w: %s", ErrPrivateIP, ip.String())
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}

	if ip.IsPrivate() {
		return true
	}

	if ip.IsLinkLocalUnicast() {
		return true
	}

	if ip.IsLinkLocalMulticast() {
		return true
	}

	if ip.IsUnspecified() {
		return true
	}

	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}

		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}

		if ip4[0] == 198 && (ip4[1] == 18 || ip4[1] == 19) {
			return true
		}

		if ip4[0] == 198 && ip4[1] == 51 && ip4[2] == 100 {
			return true
		}

		if ip4[0] == 203 && ip4[1] == 0 && ip4[2] == 113 {
			return true
		}
	}

	if strings.HasPrefix(ip.String(), "fc") || strings.HasPrefix(ip.String(), "fd") {
		return true
	}

	if strings.HasPrefix(ip.String(), "fe80:") {
		return true
	}

	return false
}