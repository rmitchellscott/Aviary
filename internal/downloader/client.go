package downloader

import (
	"net/http"
	"os"
	"time"
)

// Clients used for HTTP requests. Timeouts are configured via environment
// variables:
//   - SNIFF_TIMEOUT    → timeout for SniffMime requests
//   - DOWNLOAD_TIMEOUT → timeout for DownloadPDF and related requests

var (
	sniffTimeout    = 30 * time.Second
	downloadTimeout = 60 * time.Second
	sniffClient     = &http.Client{}
	downloadClient  = &http.Client{}
)

func init() {
	if v := os.Getenv("SNIFF_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			sniffTimeout = d
		}
	}

	if v := os.Getenv("DOWNLOAD_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			downloadTimeout = d
		}
	}

	sniffClient.Timeout = sniffTimeout
	downloadClient.Timeout = downloadTimeout
}

// SetSniffTimeout updates the timeout for SniffMime requests.
func SetSniffTimeout(d time.Duration) {
	sniffTimeout = d
	sniffClient.Timeout = d
}

// SetDownloadTimeout updates the timeout for DownloadPDF requests.
func SetDownloadTimeout(d time.Duration) {
	downloadTimeout = d
	downloadClient.Timeout = d
}
