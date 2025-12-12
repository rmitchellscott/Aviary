// internal/converter/htmlpdf.go

package converter

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rmitchellscott/aviary/internal/config"
	"github.com/rmitchellscott/aviary/internal/logging"
)

// PDFOptions contains options for PDF generation
type PDFOptions struct {
	Title       string
	PageSize    string  // e.g., "A4", "Letter", or custom like "1404x1872"
	MarginTop   string  // e.g., "10mm"
	MarginBottom string
	MarginLeft   string
	MarginRight  string
	DPI          uint    // Dots per inch for rendering
}

// defaultPDFCSS provides readable styling for PDF content optimized for reMarkable
const defaultPDFCSS = `
<style>
body {
    font-family: Georgia, serif;
    line-height: 1.6;
    margin: 0;
    padding: 20px;
    color: #000;
}
h1, h2, h3, h4, h5, h6 {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    margin-top: 1.5em;
    margin-bottom: 0.5em;
    color: #000;
    page-break-after: avoid;
}
h1 {
    font-size: 2em;
}
h2 {
    font-size: 1.5em;
}
h3 {
    font-size: 1.3em;
}
p {
    margin: 0.8em 0;
}
img {
    max-width: 100%;
    height: auto;
    display: block;
    margin: 1em 0;
}
code {
    background-color: #f4f4f4;
    padding: 2px 6px;
    border-radius: 3px;
    font-family: monospace;
    font-size: 0.9em;
}
pre {
    background-color: #f4f4f4;
    padding: 10px;
    border-radius: 5px;
    overflow-x: auto;
    page-break-inside: avoid;
}
pre code {
    background-color: transparent;
    padding: 0;
}
table {
    border-collapse: collapse;
    width: 100%;
    margin: 1em 0;
    page-break-inside: avoid;
}
th, td {
    border: 1px solid #000;
    padding: 8px;
    text-align: left;
}
th {
    background-color: #e0e0e0;
    font-weight: bold;
}
blockquote {
    border-left: 4px solid #000;
    padding-left: 1em;
    margin-left: 0;
    font-style: italic;
    color: #333;
}
a {
    color: #000;
    text-decoration: underline;
}
</style>
`

// ConvertHTMLToPDF creates a PDF file from HTML content using EPUB → PDF pipeline.
// It first generates an EPUB (which handles images and styling well), then converts to PDF using mutool.
func ConvertHTMLToPDF(htmlContent string, outputPath string, options PDFOptions) error {
	logging.Logf("[HTMLPDF] ConvertHTMLToPDF: generating PDF at %s", outputPath)

	// Create a temporary directory for processing
	tempDir, err := os.MkdirTemp("", "htmlpdf-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Step 1: Generate EPUB from HTML
	logging.Logf("[HTMLPDF] Step 1: Converting HTML to EPUB")
	tempEPUBPath := filepath.Join(tempDir, "temp.epub")

	epubOptions := EPUBOptions{
		Title:    options.Title,
		Language: "en",
	}

	if err := ConvertHTMLToEPUB(htmlContent, tempEPUBPath, epubOptions); err != nil {
		return fmt.Errorf("failed to generate EPUB: %w", err)
	}

	// Step 2: Convert EPUB to PDF using mutool
	logging.Logf("[HTMLPDF] Step 2: Converting EPUB to PDF using mutool")

	// Calculate page dimensions from PageSize or default to A4
	width, height := getPageDimensions(options.PageSize, options.DPI)

	// Build mutool command
	// Note: -W and -H set page dimensions for EPUB layout (numeric values, no "pt" suffix)
	args := []string{
		"convert",
		"-W", strings.TrimSuffix(width, "pt"),
		"-H", strings.TrimSuffix(height, "pt"),
		"-F", "pdf",
		"-o", outputPath,
		tempEPUBPath,
	}

	logging.Logf("[HTMLPDF] Running mutool with args: %v", args)

	// Execute mutool
	cmd := exec.Command("mutool", args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		logging.Logf("[HTMLPDF] mutool output:\n%s", buf.String())
		return fmt.Errorf("mutool conversion failed: %w: %s", err, buf.String())
	}

	logging.Logf("[HTMLPDF] ConvertHTMLToPDF: successfully created PDF")
	return nil
}

// getPageDimensions returns width and height in points based on PageSize option
func getPageDimensions(pageSize string, dpi uint) (string, string) {
	// Handle custom resolution format (e.g., "1404x1872" or "954x1696")
	if strings.Contains(pageSize, "x") {
		parts := strings.Split(pageSize, "x")
		if len(parts) == 2 {
			widthPx, err1 := strconv.ParseFloat(parts[0], 64)
			heightPx, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 == nil && err2 == nil {
				// Convert pixels to points: points = pixels × (72 / DPI)
				if dpi == 0 {
					dpi = 226 // default reMarkable 2 DPI
				}
				widthPt := widthPx * (72.0 / float64(dpi))
				heightPt := heightPx * (72.0 / float64(dpi))
				return fmt.Sprintf("%.0fpt", widthPt), fmt.Sprintf("%.0fpt", heightPt)
			}
		}
	}

	// Standard page sizes in points (1 inch = 72 points)
	switch strings.ToUpper(pageSize) {
	case "A4":
		return "595pt", "842pt" // A4: 210mm x 297mm
	case "LETTER":
		return "612pt", "792pt" // Letter: 8.5" x 11"
	case "LEGAL":
		return "612pt", "1008pt" // Legal: 8.5" x 14"
	default:
		return "595pt", "842pt" // Default to A4
	}
}

// ConvertHTMLFileToPDF reads an HTML file and converts it to PDF.
func ConvertHTMLFileToPDF(htmlPath string, options PDFOptions) (string, error) {
	logging.Logf("[HTMLPDF] ConvertHTMLFileToPDF: processing %s", htmlPath)

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

	// Create output path (same directory, .pdf extension)
	ext := filepath.Ext(htmlPath)
	base := strings.TrimSuffix(filepath.Base(htmlPath), ext)
	pdfPath := filepath.Join(filepath.Dir(htmlPath), base+".pdf")

	// Convert to PDF
	err = ConvertHTMLToPDF(string(content), pdfPath, options)
	if err != nil {
		return "", err
	}

	logging.Logf("[HTMLPDF] ConvertHTMLFileToPDF: created %s", pdfPath)
	return pdfPath, nil
}

// processImagesForPDF downloads images from the HTML and saves them locally.
// It rewrites image src attributes to point to the local files.
func processImagesForPDF(htmlContent string, tempDir string) (string, error) {
	processedHTML := htmlContent
	images := extractImageURLs(htmlContent)

	logging.Logf("[HTMLPDF] processImagesForPDF: found %d images to process", len(images))

	for _, imgURL := range images {
		// Skip data URLs (already embedded)
		if strings.HasPrefix(imgURL, "data:") {
			continue
		}

		// Parse the URL
		parsedURL, err := url.Parse(imgURL)
		if err != nil {
			logging.Logf("[HTMLPDF] Warning: invalid image URL %q: %v", imgURL, err)
			continue
		}

		// Skip if it's a relative URL or already a file path
		if !parsedURL.IsAbs() || parsedURL.Scheme == "file" {
			continue
		}

		// Generate a unique filename based on the URL
		hash := md5.Sum([]byte(imgURL))
		ext := filepath.Ext(parsedURL.Path)
		if ext == "" {
			ext = ".jpg" // default extension
		}
		filename := fmt.Sprintf("img_%x%s", hash[:8], ext)
		localPath := filepath.Join(tempDir, filename)

		// Download the image
		err = DownloadImage(imgURL, localPath)
		if err != nil {
			logging.Logf("[HTMLPDF] Warning: failed to download image %q: %v", imgURL, err)
			continue
		}

		// Rewrite the HTML to use the local file path
		// Use file:// protocol for wkhtmltopdf
		fileURL := "file://" + localPath
		processedHTML = strings.ReplaceAll(processedHTML, imgURL, fileURL)
		logging.Logf("[HTMLPDF] Downloaded image: %s -> %s", imgURL, localPath)
	}

	return processedHTML, nil
}

// wrapHTMLForPDF wraps HTML content with proper structure and CSS for PDF generation.
func wrapHTMLForPDF(htmlContent, title string) string {
	// Check if the HTML is already a complete document
	lowerHTML := strings.ToLower(htmlContent)
	if strings.Contains(lowerHTML, "<!doctype") || strings.Contains(lowerHTML, "<html") {
		// Already a complete document, just inject CSS if needed
		if !strings.Contains(lowerHTML, "<style") {
			// Insert CSS before </head> or at the beginning of <body>
			if strings.Contains(lowerHTML, "</head>") {
				return strings.Replace(htmlContent, "</head>", defaultPDFCSS+"\n</head>", 1)
			} else if strings.Contains(lowerHTML, "<body>") {
				return strings.Replace(htmlContent, "<body>", "<body>\n"+defaultPDFCSS, 1)
			}
		}
		return htmlContent
	}

	// Wrap in complete HTML structure
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>%s</title>
    %s
</head>
<body>
%s
</body>
</html>`, title, defaultPDFCSS, htmlContent)
}

// GetPDFOptionsFromConfig creates PDFOptions from environment configuration.
func GetPDFOptionsFromConfig() PDFOptions {
	pageSize := config.Get("PAGE_RESOLUTION", "")
	if pageSize == "" {
		pageSize = defaultRemarkable2Resolution
	}

	dpi, err := parseEnvDPI()
	if err != nil {
		dpi = defaultRemarkable2DPI
	}

	return PDFOptions{
		PageSize:     pageSize,
		MarginTop:    "10mm",
		MarginBottom: "10mm",
		MarginLeft:   "10mm",
		MarginRight:  "10mm",
		DPI:          uint(dpi),
	}
}

// GetPDFOptionsForUser creates PDFOptions using user settings with environment fallback.
func GetPDFOptionsForUser(pageResolution string, pageDPI float64) PDFOptions {
	pageSize := config.Get("PAGE_RESOLUTION", "")
	if pageSize == "" {
		pageSize = defaultRemarkable2Resolution
	}
	dpi := float64(defaultRemarkable2DPI)

	if pageResolution != "" {
		pageSize = pageResolution
	}

	if pageDPI > 0 {
		dpi = pageDPI
	} else {
		if envDPI, err := parseEnvDPI(); err == nil {
			dpi = envDPI
		}
	}

	return PDFOptions{
		PageSize:     pageSize,
		MarginTop:    "10mm",
		MarginBottom: "10mm",
		MarginLeft:   "10mm",
		MarginRight:  "10mm",
		DPI:          uint(dpi),
	}
}
