package compressor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ExecCommand is exec.Command by default, but can be overridden in tests.
var ExecCommand = exec.Command

// CompressPDF invokes Ghostscript with GS_COMPAT and GS_SETTINGS env vars
func CompressPDF(path string) (string, error) {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	out := fmt.Sprintf("%s_compressed%s", base, ext)
	compat := getEnv("GS_COMPAT", "1.4")
	settings := getEnv("GS_SETTINGS", "/ebook")
	args := []string{
		"gs", "-sDEVICE=pdfwrite",
		fmt.Sprintf("-dCompatibilityLevel=%s", compat),
		fmt.Sprintf("-dPDFSETTINGS=%s", settings),
		"-dNOPAUSE", "-dBATCH",
		fmt.Sprintf("-sOutputFile=%s", out),
		path,
	}
	cmd := ExecCommand(args[0], args[1:]...)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
