package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const maxRedirects = 10

var ErrTooManyRedirects = errors.New("too many redirects")

// NewHTTPClient returns a client that validates every address it connects to.
// Callers set Timeout themselves. Validation at dial time covers each redirect
// hop and the address the connection actually reaches, which a check against
// the requested URL alone does not.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport:     newGuardedTransport(),
		CheckRedirect: checkRedirect,
	}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return ErrTooManyRedirects
	}
	return ValidateURL(req.URL.String())
}

func newGuardedTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialGuarded(ctx, dialer, network, addr)
	}

	return transport
}

// dialGuarded resolves the host itself and connects to a validated address, so
// the connection cannot land somewhere a second resolution would have returned.
func dialGuarded(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIPResolutionFailed, err)
	}

	lastErr := error(fmt.Errorf("%w: no IPs found for hostname", ErrIPResolutionFailed))
	for _, resolved := range addrs {
		if err := checkIPAllowed(resolved.IP); err != nil {
			lastErr = err
			continue
		}

		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(resolved.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}

	return nil, lastErr
}
