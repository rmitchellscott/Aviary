package downloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rmitchellscott/aviary/internal/manager"
)

const (
	defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) " +
		"Chrome/113.0.0.0 Safari/537.36"

	uaListURL = "https://raw.githubusercontent.com/jnrbsn/user-agents/main/user-agents.json"
)

var (
	userAgents []string
	rng        *rand.Rand
)

func init() {
	// Create a local RNG rather than seeding the global one
	rng = rand.New(rand.NewSource(time.Now().UnixNano()))

	// Try to fetch the UA list
	resp, err := downloadClient.Get(uaListURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to fetch UA list: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "warning: UA list fetch returned %s\n", resp.Status)
		return
	}

	if err := json.NewDecoder(resp.Body).Decode(&userAgents); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to parse UA list JSON: %v\n", err)
	}
}

func pickUA() string {
	if len(userAgents) == 0 {
		return defaultUA
	}
	return userAgents[rng.Intn(len(userAgents))]
}

type progressReader struct {
	r     io.Reader
	total int64
	done  int64
	cb    func(done, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	if n > 0 {
		pr.done += int64(n)
		if pr.cb != nil {
			pr.cb(pr.done, pr.total)
		}
	}
	return n, err
}

// DownloadPDF fetches the PDF and saves locally.
// tmp==true → os.TempDir(), else under $PDF_DIR/prefix or $PDF_DIR.
// progress is an optional callback receiving bytes downloaded and total bytes.
func DownloadPDF(urlStr string, tmp bool, prefix string, progress func(done, total int64)) (string, error) {
	// Sanitize prefix before using it in any paths
	var err error
	prefix, err = manager.SanitizePrefix(prefix)
	if err != nil {
		return "", err
	}

	// 1) Determine destination directory
	var destDir string
	if tmp {
		destDir = os.TempDir()
	} else {
		d := os.Getenv("PDF_DIR")
		if d == "" {
			d = "/app/pdfs"
		}
		if prefix != "" {
			d = filepath.Join(d, prefix)
		}
		if err := os.MkdirAll(d, 0755); err != nil {
			return "", err
		}
		destDir = d
	}

	// 2) Build request with a random UA
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", pickUA())

	resp, err := downloadClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", errors.New("failed to download PDF: status " + resp.Status)
	}

	// 3) Choose a filename, preferring the final URL
	name := filepath.Base(resp.Request.URL.Path)
	if strings.TrimSpace(name) == "" {
		name = filepath.Base(urlStr)
	}
	if filepath.Ext(name) == "" {
		ct := resp.Header.Get("Content-Type")
		switch {
		case strings.HasPrefix(ct, "application/pdf"):
			name += ".pdf"
		case strings.HasPrefix(ct, "image/png"):
			name += ".png"
		case strings.HasPrefix(ct, "image/jpeg"), strings.HasPrefix(ct, "image/jpg"):
			name += ".jpg"
		case strings.HasPrefix(ct, "application/epub+zip"):
			name += ".epub"
		}
	}

	// 4) Write to disk
	outPath := filepath.Join(destDir, name)
	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	pr := &progressReader{r: resp.Body, total: resp.ContentLength, cb: progress}
	if progress != nil && resp.ContentLength > 0 {
		progress(0, resp.ContentLength)
	}
	if _, err := io.Copy(f, pr); err != nil {
		return "", err
	}

	return outPath, nil
}
