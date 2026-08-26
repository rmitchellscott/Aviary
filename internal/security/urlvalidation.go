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
	ErrLinkLocal         = errors.New("URL points to a link-local address")
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

	return checkHostAddresses(hostname)
}

func blockPrivateIPs() bool {
	return config.Get("BLOCK_PRIVATE_IPS", "") == "true"
}

// checkIPAllowed rejects an address the server must never connect to. Link-local
// space carries the cloud metadata endpoints and is refused unconditionally.
// The wider private ranges are refused only when BLOCK_PRIVATE_IPS is set, so a
// self-hosted deployment can still fetch from its own network.
func checkIPAllowed(ip net.IP) error {
	if isLinkLocal(ip) {
		return fmt.Errorf("%w: %s", ErrLinkLocal, ip.String())
	}

	if blockPrivateIPs() && isPrivateIP(ip) {
		return fmt.Errorf("%w: %s (unset BLOCK_PRIVATE_IPS to allow)", ErrPrivateIP, ip.String())
	}

	return nil
}

// IsBlockedAddress reports whether err comes from an address or domain the
// server refused to connect to, rather than from a network or server fault.
func IsBlockedAddress(err error) bool {
	return errors.Is(err, ErrLinkLocal) ||
		errors.Is(err, ErrPrivateIP) ||
		errors.Is(err, ErrBlockedDomain)
}

// checkHostAddresses validates an IP literal directly. A hostname is resolved
// here only when BLOCK_PRIVATE_IPS is set; otherwise the guarded dialer applies
// checkIPAllowed to whatever address the connection actually reaches, which
// avoids a DNS lookup on every validation and closes the rebinding gap.
func checkHostAddresses(hostname string) error {
	if ip := net.ParseIP(hostname); ip != nil {
		return checkIPAllowed(ip)
	}

	if !blockPrivateIPs() {
		return nil
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrIPResolutionFailed, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: no IPs found for hostname", ErrIPResolutionFailed)
	}
	for _, resolvedIP := range ips {
		if err := checkIPAllowed(resolvedIP); err != nil {
			return fmt.Errorf("%s resolves to %w", hostname, err)
		}
	}

	return nil
}

func isLinkLocal(ip net.IP) bool {
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 169 && ip4[1] == 254
	}

	return false
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