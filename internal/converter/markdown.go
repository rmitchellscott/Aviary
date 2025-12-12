// internal/converter/markdown.go

package converter

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rmitchellscott/aviary/internal/logging"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// MarkdownMetadata contains metadata extracted from Markdown frontmatter
type MarkdownMetadata struct {
	Title       string
	Author      string
	Description string
	Date        string
}

// MarkdownContent represents the converted Markdown content
type MarkdownContent struct {
	HTML     string
	Metadata MarkdownMetadata
}

// ConvertMarkdownToHTML converts a Markdown file to HTML using goldmark.
// It parses YAML frontmatter for metadata and returns clean HTML.
func ConvertMarkdownToHTML(mdPath string) (*MarkdownContent, error) {
	logging.Logf("[MARKDOWN] ConvertMarkdownToHTML: processing %s", mdPath)

	// Read the markdown file
	content, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read markdown file: %w", err)
	}

	return ConvertMarkdownStringToHTML(string(content))
}

// ConvertMarkdownStringToHTML converts a Markdown string to HTML.
// It parses YAML frontmatter for metadata and returns clean HTML.
func ConvertMarkdownStringToHTML(mdContent string) (*MarkdownContent, error) {
	logging.Logf("[MARKDOWN] ConvertMarkdownStringToHTML: processing markdown content")

	// Extract frontmatter if present
	metadata, bodyContent := extractFrontmatter(mdContent)

	// Configure goldmark with common extensions
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,        // GitHub Flavored Markdown
			extension.Table,      // Tables
			extension.Strikethrough,
			extension.Linkify,    // Auto-link URLs
			extension.TaskList,   // Task lists
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // Convert line breaks to <br>
			html.WithXHTML(),     // Use XHTML-style tags
		),
	)

	// Convert markdown to HTML
	var buf bytes.Buffer
	if err := md.Convert([]byte(bodyContent), &buf); err != nil {
		return nil, fmt.Errorf("failed to convert markdown to HTML: %w", err)
	}

	html := buf.String()

	logging.Logf("[MARKDOWN] ConvertMarkdownStringToHTML: successfully converted to HTML")

	return &MarkdownContent{
		HTML:     html,
		Metadata: metadata,
	}, nil
}

// extractFrontmatter parses YAML frontmatter from markdown content.
// It looks for content between --- delimiters at the start of the file.
// Returns (metadata, content without frontmatter).
func extractFrontmatter(mdContent string) (MarkdownMetadata, string) {
	metadata := MarkdownMetadata{}

	// Check if the content starts with ---
	if !strings.HasPrefix(strings.TrimSpace(mdContent), "---") {
		return metadata, mdContent
	}

	// Find the closing ---
	lines := strings.Split(mdContent, "\n")
	if len(lines) < 3 {
		return metadata, mdContent
	}

	// Skip the opening ---
	startIdx := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			startIdx = i
			break
		}
	}

	// Find the closing ---
	endIdx := -1
	for i := startIdx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		// No closing ---, treat as regular content
		return metadata, mdContent
	}

	// Parse the frontmatter lines
	frontmatterLines := lines[startIdx+1 : endIdx]
	for _, line := range frontmatterLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Simple key: value parsing
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		value = strings.Trim(value, `"'`)

		switch key {
		case "title":
			metadata.Title = value
		case "author":
			metadata.Author = value
		case "description":
			metadata.Description = value
		case "date":
			metadata.Date = value
		}
	}

	// Return content without frontmatter
	bodyLines := lines[endIdx+1:]
	bodyContent := strings.Join(bodyLines, "\n")

	logging.Logf("[MARKDOWN] extractFrontmatter: found metadata - title: %q, author: %q", metadata.Title, metadata.Author)

	return metadata, bodyContent
}

// SaveMarkdownAsHTML converts a Markdown file to HTML and saves it to disk.
// Returns the path to the generated HTML file.
func SaveMarkdownAsHTML(mdPath string) (string, error) {
	logging.Logf("[MARKDOWN] SaveMarkdownAsHTML: converting %s", mdPath)

	// Convert markdown to HTML
	result, err := ConvertMarkdownToHTML(mdPath)
	if err != nil {
		return "", err
	}

	// Create output path (same directory, .html extension)
	ext := filepath.Ext(mdPath)
	base := strings.TrimSuffix(filepath.Base(mdPath), ext)
	htmlPath := filepath.Join(filepath.Dir(mdPath), base+".html")

	// Wrap in basic HTML structure
	fullHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.6;
        }
        img {
            max-width: 100%%;
            height: auto;
        }
        code {
            background-color: #f4f4f4;
            padding: 2px 6px;
            border-radius: 3px;
        }
        pre {
            background-color: #f4f4f4;
            padding: 10px;
            border-radius: 5px;
            overflow-x: auto;
        }
        table {
            border-collapse: collapse;
            width: 100%%;
        }
        th, td {
            border: 1px solid #ddd;
            padding: 8px;
            text-align: left;
        }
        th {
            background-color: #f4f4f4;
        }
    </style>
</head>
<body>
%s
</body>
</html>`, result.Metadata.Title, result.HTML)

	// Write to file
	if err := os.WriteFile(htmlPath, []byte(fullHTML), 0644); err != nil {
		return "", fmt.Errorf("failed to write HTML file: %w", err)
	}

	logging.Logf("[MARKDOWN] SaveMarkdownAsHTML: saved to %s", htmlPath)
	return htmlPath, nil
}
