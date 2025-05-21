package downloader

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// DownloadPDF fetches the PDF and saves locally.
// tmp==true → /tmp, else under PDF_DIR/prefix (if supplied) or PDF_DIR.
func DownloadPDF(url string, tmp bool, prefix string) (string, error) {
	var destDir string
	if tmp {
		destDir = "/tmp"
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
	name := filepath.Base(url)
	path := filepath.Join(destDir, name)

	r, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return "", errors.New("failed to download PDF: status " + r.Status)
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, r.Body); err != nil {
		return "", err
	}
	return path, nil
}
