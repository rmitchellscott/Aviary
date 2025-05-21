package manager

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func DefaultRmDir() string {
	d := os.Getenv("RM_TARGET_DIR")
	if d == "" {
		d = "/"
	}
	return d
}

func Logf(format string, v ...interface{}) {
	fmt.Printf("[%s] "+format+"\n", append([]interface{}{time.Now().Format(time.RFC3339)}, v...)...)
}
func RenameLocalNoYear(src, prefix string) (string, error) {
	dir := filepath.Dir(src)
	today := time.Now()
	month, day := today.Format("January"), today.Day()

	name := fmt.Sprintf("%s %s %d.pdf", prefix, month, day)
	if prefix == "" {
		name = fmt.Sprintf("%s %d.pdf", month, day)
	}
	dest := filepath.Join(dir, name)
	if err := moveFile(src, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func AppendYearLocal(noYearPath string) (string, error) {
	dir := filepath.Dir(noYearPath)
	today := time.Now()
	year := today.Year()

	base := strings.TrimSuffix(filepath.Base(noYearPath), ".pdf")
	name := fmt.Sprintf("%s %d.pdf", base, year)
	dest := filepath.Join(dir, name)
	if err := moveFile(noYearPath, dest); err != nil {
		return "", err
	}
	return dest, nil
}
func moveFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	// ensure data flushed
	if err := out.Sync(); err != nil {
		return err
	}
	// remove src
	return os.Remove(src)
}

// SimpleUpload calls rmapi put
func SimpleUpload(path, rmDir string) error {
	cmd := exec.Command("rmapi", "put", path, rmDir)
	return cmd.Run()
}

func RenameLocal(path, prefix string) (string, error) {
	dir := filepath.Dir(path)
	today := time.Now()
	month, day, year := today.Format("January"), today.Day(), today.Year()

	// 1) rename to no‐year
	noYear := fmt.Sprintf("%s %s %d.pdf", prefix, month, day)
	if prefix == "" {
		noYear = fmt.Sprintf("%s %d.pdf", month, day)
	}
	noYearPath := filepath.Join(dir, noYear)
	if err := moveFile(path, noYearPath); err != nil {
		return "", err
	}

	// 2) rename to include year
	withYear := strings.TrimSuffix(noYear, ".pdf") + fmt.Sprintf(" %d.pdf", year)
	withYearPath := filepath.Join(dir, withYear)
	if err := moveFile(noYearPath, withYearPath); err != nil {
		return "", err
	}

	return withYearPath, nil
}

// RenameAndUpload implements rename/upload logic
func RenameAndUpload(path, prefix, rmDir string) error {
	// Build target dir under PDF_DIR
	pdfDir := os.Getenv("PDF_DIR")
	if pdfDir == "" {
		pdfDir = "/app/pdfs"
	}
	if prefix != "" {
		pdfDir = filepath.Join(pdfDir, prefix)
		if err := os.MkdirAll(pdfDir, 0755); err != nil {
			return err
		}
	}

	// Date strings
	today := time.Now()
	month, day, year := today.Format("January"), today.Day(), today.Year()

	// Move into target as "<prefix> Month D.pdf"
	noYear := fmt.Sprintf("%s %s %d.pdf", prefix, month, day)
	if prefix == "" {
		noYear = fmt.Sprintf("%s %d.pdf", month, day)
	}
	noYearPath := filepath.Join(pdfDir, noYear)
	if err := moveFile(path, noYearPath); err != nil {
		return err
	}

	// Upload yearless file
	if err := exec.Command("rmapi", "put", noYearPath, rmDir).Run(); err != nil {
		return err
	}

	// Rename local to include year
	withYear := strings.TrimSuffix(noYear, ".pdf") + fmt.Sprintf(" %d.pdf", year)
	withYearPath := filepath.Join(pdfDir, withYear)
	if err := moveFile(noYearPath, withYearPath); err != nil {
		return err
	}
	return nil
}

func CleanupOld(prefix, rmDir string) error {
	today := time.Now()
	cutoff := today.AddDate(0, 0, -7)
	Logf("[cleanup] today=%s, cutoff=%s",
		today.Format("2006-01-02"), cutoff.Format("2006-01-02"))

	// 1) List remote files
	proc := exec.Command("rmapi", "ls", rmDir)
	out, err := proc.Output()
	if err != nil {
		return err
	}

	// 2) Compile regexes
	// match any file entry (we’ll strip .pdf later if present)
	lineRe := regexp.MustCompile(`^\[f\]\s+(.*)$`)

	// date pattern: optional prefix, Month, Day, with or without “.pdf”
	var dateRe *regexp.Regexp
	if prefix != "" {
		// e.g. “DUMMY May 7” or “DUMMY May 7.pdf”
		dateRe = regexp.MustCompile(
			`^` + regexp.QuoteMeta(prefix+` `) + `([A-Za-z]+)\s+(\d+)(?:\.pdf)?$`,
		)
	} else {
		dateRe = regexp.MustCompile(
			`^([A-Za-z]+)\s+(\d+)(?:\.pdf)?$`,
		)
	}

	// 3) Scan lines
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()

		// grab the name field (might end in “.pdf” or not)
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		fname := strings.TrimSpace(m[1])

		// match date portion
		md := dateRe.FindStringSubmatch(fname)
		if md == nil {
			continue
		}
		monthStr, dayStr := md[1], md[2]

		// parse month and day into a time.Time
		t, err := time.Parse("January 2", monthStr+" "+dayStr)
		if err != nil {
			Logf("  ↳ parse error: %v", err)
			continue
		}
		fileDate := time.Date(today.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
		if fileDate.After(today) {
			fileDate = fileDate.AddDate(-1, 0, 0)
		}

		// decide whether to remove
		if fileDate.Before(cutoff) {
			Logf("Removing %s (dated %s < %s)",
				fname, fileDate.Format("2006-01-02"), cutoff.Format("2006-01-02"))
			exec.Command("rmapi", "rm", filepath.Join(rmDir, fname)).Run()
		}
	}

	return nil
}
