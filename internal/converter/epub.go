// internal/converter/epub.go

package converter

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmaupin/go-epub"
	"github.com/rmitchellscott/aviary/internal/logging"
	"github.com/vincent-petithory/dataurl"
)

// EPUBOptions contains options for EPUB generation
type EPUBOptions struct {
	Title       string
	Author      string
	Description string
	Language    string
	CSSContent  string
}

// defaultEPUBCSS provides readable styling for EPUB content
const defaultEPUBCSS = `
body {
    font-family: Georgia, serif;
    line-height: 1.6;
    margin: 1em;
}
h1, h2, h3, h4, h5, h6 {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    margin-top: 1.5em;
    margin-bottom: 0.5em;
}
img {
    max-width: 100%;
    height: auto;
}
code {
    background-color: #f4f4f4;
    padding: 2px 6px;
    border-radius: 3px;
    font-family: monospace;
}
pre {
    background-color: #f4f4f4;
    padding: 10px;
    border-radius: 5px;
    overflow-x: auto;
}
pre code {
    background-color: transparent;
    padding: 0;
}
table {
    border-collapse: collapse;
    width: 100%;
    margin: 1em 0;
}
th, td {
    border: 1px solid #ddd;
    padding: 8px;
    text-align: left;
}
th {
    background-color: #f4f4f4;
    font-weight: bold;
}
blockquote {
    border-left: 4px solid #ddd;
    padding-left: 1em;
    margin-left: 0;
    font-style: italic;
    color: #666;
}
`

// ConvertHTMLToEPUB creates an EPUB file from HTML content.
// It downloads and embeds images, rewrites image paths, and applies metadata.
func ConvertHTMLToEPUB(htmlContent string, outputPath string, options EPUBOptions) error {
	logging.Logf("[EPUB] ConvertHTMLToEPUB: generating EPUB at %s", outputPath)

	// Create a new EPUB
	e := epub.NewEpub(options.Title)

	// Set metadata
	if options.Author != "" {
		e.SetAuthor(options.Author)
	}
	if options.Description != "" {
		e.SetDescription(options.Description)
	}
	if options.Language != "" {
		e.SetLang(options.Language)
	} else {
		e.SetLang("en")
	}

	// Add CSS
	cssContent := options.CSSContent
	if cssContent == "" {
		cssContent = defaultEPUBCSS
	}
	// Convert CSS content to data URL since AddCSS expects a source (file/URL/data URL), not content
	cssDataURL := dataurl.EncodeBytes([]byte(cssContent))
	cssPath, err := e.AddCSS(cssDataURL, "styles.css")
	if err != nil {
		return fmt.Errorf("failed to add CSS: %w", err)
	}

	// Process images: download and embed them
	processedHTML, err := processImagesForEPUB(htmlContent, e, outputPath)
	if err != nil {
		logging.Logf("[EPUB] Warning: failed to process some images: %v", err)
		// Continue with unprocessed HTML
		processedHTML = htmlContent
	}

	// Add the main content section
	_, err = e.AddSection(processedHTML, options.Title, "", cssPath)
	if err != nil {
		return fmt.Errorf("failed to add section: %w", err)
	}

	// Write the EPUB file
	err = e.Write(outputPath)
	if err != nil {
		return fmt.Errorf("failed to write EPUB: %w", err)
	}

	logging.Logf("[EPUB] ConvertHTMLToEPUB: successfully created EPUB")
	return nil
}

// ConvertHTMLFileToEPUB reads an HTML file and converts it to EPUB.
func ConvertHTMLFileToEPUB(htmlPath string, options EPUBOptions) (string, error) {
	logging.Logf("[EPUB] ConvertHTMLFileToEPUB: processing %s", htmlPath)

	// Read the HTML file
	content, err := os.ReadFile(htmlPath)
	if err != nil {
		return "", fmt.Errorf("failed to read HTML file: %w", err)
	}

	// Extract title from HTML if not provided
	if options.Title == "" {
		options.Title = extractTitleFromHTML(string(content))
		if options.Title == "" {
			// Use filename as fallback
			options.Title = strings.TrimSuffix(filepath.Base(htmlPath), filepath.Ext(htmlPath))
		}
	}

	// Create output path (same directory, .epub extension)
	ext := filepath.Ext(htmlPath)
	base := strings.TrimSuffix(filepath.Base(htmlPath), ext)
	epubPath := filepath.Join(filepath.Dir(htmlPath), base+".epub")

	// Convert to EPUB
	err = ConvertHTMLToEPUB(string(content), epubPath, options)
	if err != nil {
		return "", err
	}

	logging.Logf("[EPUB] ConvertHTMLFileToEPUB: created %s", epubPath)
	return epubPath, nil
}

// processImagesForEPUB downloads images from the HTML and embeds them in the EPUB.
// It rewrites image src attributes to point to the embedded resources.
func processImagesForEPUB(htmlContent string, e *epub.Epub, epubPath string) (string, error) {
	// Create a temporary directory for downloaded images
	tempDir, err := os.MkdirTemp("", "epub-images-*")
	if err != nil {
		return htmlContent, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	processedHTML := htmlContent
	images := extractImageURLs(htmlContent)

	logging.Logf("[EPUB] processImagesForEPUB: found %d images to process", len(images))

	for _, imgURL := range images {
		// Skip data URLs
		if strings.HasPrefix(imgURL, "data:") {
			continue
		}

		// Parse the URL
		parsedURL, err := url.Parse(imgURL)
		if err != nil {
			logging.Logf("[EPUB] Warning: invalid image URL %q: %v", imgURL, err)
			continue
		}

		// Skip if it's a relative URL (can't download)
		if !parsedURL.IsAbs() {
			logging.Logf("[EPUB] Warning: skipping relative image URL %q", imgURL)
			continue
		}

		// Generate a unique filename based on the URL
		hash := md5.Sum([]byte(imgURL))
		ext := filepath.Ext(parsedURL.Path)
		if ext == "" {
			ext = ".jpg" // default extension
		}
		filename := fmt.Sprintf("img_%x%s", hash[:8], ext)
		tempPath := filepath.Join(tempDir, filename)

		// Download the image
		err = DownloadImage(imgURL, tempPath)
		if err != nil {
			logging.Logf("[EPUB] Warning: failed to download image %q: %v", imgURL, err)
			continue
		}

		// Add the image to the EPUB
		// Read the downloaded image file
		imageData, err := os.ReadFile(tempPath)
		if err != nil {
			logging.Logf("[EPUB] Warning: failed to read image file: %v", err)
			continue
		}

		// Convert to data URL since AddImage expects a source (file/URL/data URL), not a path
		imageDataURL := dataurl.EncodeBytes(imageData)
		internalPath, err := e.AddImage(imageDataURL, filename)
		if err != nil {
			logging.Logf("[EPUB] Warning: failed to add image to EPUB: %v", err)
			continue
		}

		// Rewrite the HTML to use the internal path
		processedHTML = strings.ReplaceAll(processedHTML, imgURL, internalPath)
		logging.Logf("[EPUB] Embedded image: %s -> %s", imgURL, internalPath)
	}

	return processedHTML, nil
}

// extractTitleFromHTML attempts to extract the <title> tag from HTML.
func extractTitleFromHTML(html string) string {
	lower := strings.ToLower(html)

	// Find <title> tag
	startTag := "<title>"
	endTag := "</title>"

	startIdx := strings.Index(lower, startTag)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(startTag)

	endIdx := strings.Index(lower[startIdx:], endTag)
	if endIdx == -1 {
		return ""
	}

	// Extract the title content (use original case from html, not lower)
	title := html[startIdx : startIdx+endIdx]
	return strings.TrimSpace(title)
}
