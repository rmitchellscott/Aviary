package manager

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRenameFilenameGeneration(t *testing.T) {
	tests := []struct {
		name     string
		srcKey   string
		prefix   string
		month    string
		day      int
		wantName string
	}{
		{"pdf with prefix", "pdfs/upload.pdf", "Notes", "March", 5, "Notes March 5.pdf"},
		{"pdf no prefix", "pdfs/upload.pdf", "", "March", 5, "March 5.pdf"},
		{"epub with prefix", "pdfs/book.epub", "Reading", "January", 12, "Reading January 12.epub"},
		{"epub no prefix", "pdfs/book.epub", "", "January", 12, "January 12.epub"},
		{"nested key pdf", "users/abc/pdfs/doc.pdf", "Work", "July", 1, "Work July 1.pdf"},
		{"nested key epub", "users/abc/pdfs/book.epub", "Work", "July", 1, "Work July 1.epub"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := filepath.Ext(tt.srcKey)
			var filename string
			if tt.prefix != "" {
				filename = fmt.Sprintf("%s %s %d%s", tt.prefix, tt.month, tt.day, ext)
			} else {
				filename = fmt.Sprintf("%s %d%s", tt.month, tt.day, ext)
			}
			if filename != tt.wantName {
				t.Errorf("got %q, want %q", filename, tt.wantName)
			}
		})
	}
}

func TestAppendYearFilenameGeneration(t *testing.T) {
	tests := []struct {
		name     string
		noYearFn string
		year     int
		wantName string
	}{
		{"pdf with prefix", "Notes March 5.pdf", 2026, "Notes March 5 2026.pdf"},
		{"pdf no prefix", "March 5.pdf", 2026, "March 5 2026.pdf"},
		{"epub with prefix", "Reading January 12.epub", 2026, "Reading January 12 2026.epub"},
		{"epub no prefix", "January 12.epub", 2026, "January 12 2026.epub"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := filepath.Ext(tt.noYearFn)
			base := strings.TrimSuffix(tt.noYearFn, ext)
			result := fmt.Sprintf("%s %d%s", base, tt.year, ext)
			if result != tt.wantName {
				t.Errorf("got %q, want %q", result, tt.wantName)
			}
		})
	}
}

func TestCleanupRegexMatchesExtensions(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		fname   string
		wantMon string
		wantDay string
	}{
		{"pdf with prefix", "Notes", "Notes March 5.pdf", "March", "5"},
		{"epub with prefix", "Notes", "Notes March 5.epub", "March", "5"},
		{"no ext with prefix", "Notes", "Notes March 5", "March", "5"},
		{"pdf no prefix", "", "March 5.pdf", "March", "5"},
		{"epub no prefix", "", "March 5.epub", "March", "5"},
		{"no ext no prefix", "", "March 5", "March", "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dateRe *regexp.Regexp
			if tt.prefix != "" {
				dateRe = regexp.MustCompile(
					`^` + regexp.QuoteMeta(tt.prefix+` `) + `([A-Za-z]+)\s+(\d+)(?:\.\w+)?$`,
				)
			} else {
				dateRe = regexp.MustCompile(
					`^([A-Za-z]+)\s+(\d+)(?:\.\w+)?$`,
				)
			}
			md := dateRe.FindStringSubmatch(tt.fname)
			if md == nil {
				t.Fatalf("regex did not match %q", tt.fname)
			}
			if md[1] != tt.wantMon {
				t.Errorf("month: got %q, want %q", md[1], tt.wantMon)
			}
			if md[2] != tt.wantDay {
				t.Errorf("day: got %q, want %q", md[2], tt.wantDay)
			}
		})
	}
}

func TestCleanupRegexRejectsNonMatches(t *testing.T) {
	dateRe := regexp.MustCompile(`^([A-Za-z]+)\s+(\d+)(?:\.\w+)?$`)
	nonMatches := []string{
		"random file.pdf",
		"March 5 2026.pdf",
		"12 March.pdf",
	}
	for _, s := range nonMatches {
		if dateRe.FindStringSubmatch(s) != nil {
			t.Errorf("regex should not match %q", s)
		}
	}
}
