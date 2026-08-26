package converter

import (
	"strings"
	"testing"
)

func TestSourceHeaderHTMLEscapesURL(t *testing.T) {
	tests := []struct {
		name      string
		sourceURL string
		unwanted  string
	}{
		{
			name:      "quote closing the href attribute",
			sourceURL: `https://example.com/x"><script>alert(1)</script>`,
			unwanted:  "<script>",
		},
		{
			name:      "quote opening an attribute on the anchor",
			sourceURL: `https://example.com/x" onmouseover="alert(1)`,
			unwanted:  `onmouseover="`,
		},
		{
			name:      "angle brackets in the link text",
			sourceURL: `https://example.com/<img src=x onerror=alert(1)>`,
			unwanted:  "<img",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sourceHeaderHTML(tt.sourceURL)
			if strings.Contains(got, tt.unwanted) {
				t.Errorf("sourceHeaderHTML(%q) leaked %q:\n%s", tt.sourceURL, tt.unwanted, got)
			}
			if strings.Count(got, `<a href="`) != 1 {
				t.Errorf("expected exactly one anchor, got:\n%s", got)
			}
		})
	}
}

func TestSourceHeaderHTMLKeepsOrdinaryURL(t *testing.T) {
	url := "https://example.com/article?id=1&page=2"
	got := sourceHeaderHTML(url)

	if !strings.Contains(got, "https://example.com/article?id=1&amp;page=2") {
		t.Errorf("expected the URL preserved with escaping, got:\n%s", got)
	}
}
