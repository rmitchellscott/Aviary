// internal/converter/reader.go

package converter

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/rmitchellscott/aviary/internal/downloader"
	"github.com/rmitchellscott/aviary/internal/logging"
	"github.com/rmitchellscott/aviary/internal/security"
)

// ArticleContent represents the extracted clean article content
type ArticleContent struct {
	HTML    string   // Clean HTML content
	Title   string   // Article title
	Byline  string   // Author/byline
	Excerpt string   // Short excerpt
	Images  []string // Image URLs found in the article
}

func articleToContent(article readability.Article) (*ArticleContent, error) {
	var htmlBuf bytes.Buffer
	if err := article.RenderHTML(&htmlBuf); err != nil {
		return nil, fmt.Errorf("failed to render article HTML: %w", err)
	}
	htmlContent := htmlBuf.String()

	return &ArticleContent{
		HTML:    htmlContent,
		Title:   article.Title(),
		Byline:  article.Byline(),
		Excerpt: article.Excerpt(),
		Images:  extractImageURLs(htmlContent),
	}, nil
}

// ExtractFromURL fetches a URL and extracts readable article content using go-readability.
// It removes ads, navigation, scripts, and other non-content elements.
func ExtractFromURL(urlStr string) (*ArticleContent, error) {
	logging.Logf("[READER] ExtractFromURL: fetching %s", urlStr)

	// Validate URL for SSRF protection
	if err := security.ValidateURL(urlStr); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	// Parse URL for readability
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", urlStr, err)
	}

	// Fetch the webpage with timeout and realistic User-Agent
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", downloader.PickUA())

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d when fetching URL", resp.StatusCode)
	}

	// Parse with readability
	article, err := readability.FromReader(resp.Body, parsedURL)
	if err != nil {
		return nil, fmt.Errorf("readability extraction failed: %w", err)
	}

	logging.Logf("[READER] ExtractFromURL: successfully extracted article %q", article.Title())

	return articleToContent(article)
}

// ExtractFromHTML reads an HTML file from disk and extracts readable article content.
// It applies the same readability processing to remove clutter.
func ExtractFromHTML(htmlPath string) (*ArticleContent, error) {
	logging.Logf("[READER] ExtractFromHTML: processing %s", htmlPath)

	// Validate path for security
	securePath, err := security.NewSecurePathFromExisting(htmlPath)
	if err != nil {
		return nil, fmt.Errorf("invalid HTML path: %w", err)
	}

	// Read the HTML file
	file, err := security.SafeOpen(securePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open HTML file: %w", err)
	}
	defer file.Close()

	// Create a fake base URL for relative link resolution
	// Use file:// scheme with the file's directory
	baseURL, err := url.Parse("file://" + htmlPath)
	if err != nil {
		// Fallback to a generic base URL
		baseURL, _ = url.Parse("http://localhost/")
	}

	// Parse with readability
	article, err := readability.FromReader(file, baseURL)
	if err != nil {
		return nil, fmt.Errorf("readability extraction failed: %w", err)
	}

	logging.Logf("[READER] ExtractFromHTML: successfully extracted article %q", article.Title())

	return articleToContent(article)
}

// ExtractFromHTMLString processes an HTML string and extracts readable content.
// This is useful when you already have HTML in memory.
func ExtractFromHTMLString(html string, baseURL *url.URL) (*ArticleContent, error) {
	logging.Logf("[READER] ExtractFromHTMLString: processing HTML string")

	if baseURL == nil {
		baseURL, _ = url.Parse("http://localhost/")
	}

	reader := strings.NewReader(html)
	article, err := readability.FromReader(reader, baseURL)
	if err != nil {
		return nil, fmt.Errorf("readability extraction failed: %w", err)
	}

	logging.Logf("[READER] ExtractFromHTMLString: successfully extracted article %q", article.Title())

	return articleToContent(article)
}

// extractImageURLs finds all image URLs in the HTML content.
// This is a simple implementation that looks for <img src="..."> tags.
func extractImageURLs(html string) []string {
	var images []string

	// Simple regex-free approach: find img tags
	lower := strings.ToLower(html)
	pos := 0
	for {
		imgPos := strings.Index(lower[pos:], "<img")
		if imgPos == -1 {
			break
		}
		imgPos += pos

		// Find the closing >
		closePos := strings.Index(lower[imgPos:], ">")
		if closePos == -1 {
			break
		}
		closePos += imgPos

		// Extract the tag content
		tag := html[imgPos:closePos]

		// Find src attribute
		srcPos := strings.Index(strings.ToLower(tag), "src=")
		if srcPos != -1 {
			srcPos += 4 // skip "src="
			quote := tag[srcPos]
			if quote == '"' || quote == '\'' {
				endQuote := strings.Index(tag[srcPos+1:], string(quote))
				if endQuote != -1 {
					src := tag[srcPos+1 : srcPos+1+endQuote]
					if src != "" {
						images = append(images, src)
					}
				}
			}
		}

		pos = closePos + 1
	}

	logging.Logf("[READER] extractImageURLs: found %d images", len(images))
	return images
}

// DownloadImage fetches an image from a URL and saves it to the specified path.
// Returns an error if the download fails.
func DownloadImage(imageURL, outputPath string) error {
	logging.Logf("[READER] DownloadImage: fetching %s", imageURL)

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", downloader.PickUA())

	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d when downloading image", resp.StatusCode)
	}

	// Create output file
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	// Copy the image data
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save image: %w", err)
	}

	logging.Logf("[READER] DownloadImage: saved to %s", outputPath)
	return nil
}
