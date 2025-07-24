package compressor

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rmitchellscott/aviary/internal/config"
)

// ExecCommand is exec.Command by default, but can be overridden in tests.
var ExecCommand = exec.Command

// CompressPDF invokes Ghostscript with GS_COMPAT and GS_SETTINGS env vars.
// It is a thin wrapper over CompressPDFWithProgress without progress updates.
func CompressPDF(path string) (string, error) {
	return CompressPDFWithProgress(path, nil)
}

// CompressPDFWithProgress runs Ghostscript and reports progress via the callback.
// progress is called with the current page and total pages processed.
func CompressPDFWithProgress(path string, progress func(page, total int)) (string, error) {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	out := fmt.Sprintf("%s_compressed%s", base, ext)
	compat := config.Get("GS_COMPAT", "1.7")
	settings := config.Get("GS_SETTINGS", "/ebook")
	args := []string{
		"gs", "-sDEVICE=pdfwrite",
		fmt.Sprintf("-dCompatibilityLevel=%s", compat),
		fmt.Sprintf("-dPDFSETTINGS=%s", settings),
		"-dNOPAUSE", "-dBATCH",
		fmt.Sprintf("-sOutputFile=%s", out),
		path,
	}
	cmd := ExecCommand(args[0], args[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return "", err
	}

	// Parse progress lines like "Processing pages 1 through N." and "Page X"
	done := 0
	total := 0
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "Processing pages ") {
				fmt.Sscanf(line, "Processing pages 1 through %d.", &total)
			} else if strings.HasPrefix(line, "Page ") {
				fmt.Sscanf(line, "Page %d", &done)
				if progress != nil && total > 0 {
					progress(done, total)
				}
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		return "", err
	}

	if progress != nil && total > 0 {
		progress(total, total)
	}
	return out, nil
}
