package downloader

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/rmitchellscott/aviary/internal/security"
)

// SniffMime fetches the first few bytes of urlStr and returns the detected MIME type.
// It uses the Content-Type header as a fallback if detection fails.
func SniffMime(urlStr string) (string, error) {
	if err := security.ValidateURL(urlStr); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", pickUA())
	req.Header.Set("Range", "bytes=0-4095")

	resp, err := sniffClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		ct := resp.Header.Get("Content-Type")
		if ct != "" {
			return strings.Split(ct, ";")[0], nil
		}
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	limited := io.LimitReader(resp.Body, 4096)
	mt, err := mimetype.DetectReader(limited)
	if err == nil && mt != nil {
		return mt.String(), nil
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" {
		return strings.Split(ct, ";")[0], nil
	}

	if err != nil {
		return "", err
	}
	return "", nil
}
