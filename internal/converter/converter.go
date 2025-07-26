// internal/converter/converter.go

package converter

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rmitchellscott/aviary/internal/config"
)

// defaultRemarkable2Resolution and defaultRemarkable2DPI are Remarkable 2’s
// native screen resolution (in pixels) and approximate DPI.
const (
	defaultRemarkable2Resolution = "1404x1872" // width x height, in pixels
	defaultRemarkable2DPI        = 226         // dots per inch
)

// parseEnvResolution reads PAGE_RESOLUTION (in "WIDTHxHEIGHT" pixels) from the environment.
// If unset or malformed, it falls back to defaultRemarkable2Resolution.
// Returns (widthPx, heightPx, error).
func parseEnvResolution() (widthPx int, heightPx int, err error) {
	raw := config.Get("PAGE_RESOLUTION", "")
	if raw == "" {
		raw = defaultRemarkable2Resolution
	}
	return parseResolutionString(raw)
}

// parseResolutionString parses a resolution string in "WIDTHxHEIGHT" format.
// Returns (widthPx, heightPx, error).
func parseResolutionString(raw string) (widthPx int, heightPx int, err error) {
	if raw == "" {
		raw = defaultRemarkable2Resolution
	}
	parts := strings.Split(raw, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("PAGE_RESOLUTION %q is not in WIDTHxHEIGHT form", raw)
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil || w <= 0 {
		return 0, 0, fmt.Errorf("invalid width %q in PAGE_RESOLUTION: %v", parts[0], err)
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil || h <= 0 {
		return 0, 0, fmt.Errorf("invalid height %q in PAGE_RESOLUTION: %v", parts[1], err)
	}
	return w, h, nil
}

// parseEnvDPI reads PAGE_DPI (dots per inch) from the environment.
// If unset or malformed, it falls back to defaultRemarkable2DPI.
// Returns (dpi, error).
func parseEnvDPI() (float64, error) {
	raw := config.Get("PAGE_DPI", "")
	if raw == "" {
		return defaultRemarkable2DPI, nil
	}
	return parseDPIValue(raw)
}

// parseDPIValue parses a DPI value from a string.
// Returns (dpi, error).
func parseDPIValue(raw string) (float64, error) {
	if raw == "" {
		return defaultRemarkable2DPI, nil
	}
	dpi, err := strconv.ParseFloat(raw, 64)
	if err != nil || dpi <= 0 {
		return 0, fmt.Errorf("PAGE_DPI %q is invalid: %v", raw, err)
	}
	return dpi, nil
}

// ConvertImageToPDF takes an input image path (must end in .jpg, .jpeg, or .png),
// reads PAGE_RESOLUTION (pixels) and PAGE_DPI from the environment (defaulting
// to Remarkable 2’s specs), and invokes ImageMagick’s convert to produce a PDF
// whose page size is exactly the target pixel resolution, with the image
// scaled down if larger, or left at its original size if smaller. It writes
// the output PDF alongside the input (basename + ".pdf") and returns its full path.
func ConvertImageToPDF(imgPath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(imgPath))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return "", fmt.Errorf("ConvertImageToPDF: unsupported extension %q", ext)
	}

	// 1. Parse resolution in pixels:
	resWpx, resHpx, err := parseEnvResolution()
	if err != nil {
		return "", err
	}

	// 2. Parse DPI:
	dpi, err := parseEnvDPI()
	if err != nil {
		return "", err
	}

	// 3. Build output PDF path (same directory, same basename):
	base := strings.TrimSuffix(filepath.Base(imgPath), ext)
	outPDF := filepath.Join(filepath.Dir(imgPath), base+".pdf")

	// --- LOGGING FOR DIAGNOSIS: print computed values ----
	log.Printf("ConvertImageToPDF: input image = %s", imgPath)
	log.Printf("ConvertImageToPDF: target resolution = %dx%d px, DPI = %.2f", resWpx, resHpx, dpi)

	// 4. Use ImageMagick’s "convert" to:
	//    a) Set image density to DPI (so PDF metadata knows dpi)
	//    b) Resize image to fit within resWpx × resHpx (pixels), preserving aspect ratio,
	//       but never upscale smaller images (via the trailing ">").
	//    c) Center it on a white canvas of exactly resWpx × resHpx
	//    d) Output as PDF
	//
	//    convert <input.jpg> \
	//       -density <dpi> \
	//       -resize <resWpx>x<resHpx>\> \
	//       -background white -gravity center -extent <resWpx>x<resHpx> \
	//       -quality 100 \
	//       <outPDF>
	args := []string{
		imgPath,
		"-density", fmt.Sprintf("%.0f", dpi),
		"-resize", fmt.Sprintf("%dx%d>", resWpx, resHpx),
		"-background", "white",
		"-gravity", "center",
		"-extent", fmt.Sprintf("%dx%d", resWpx, resHpx),
		"-quality", "100",
		outPDF,
	}
	log.Printf("ConvertImageToPDF: running ImageMagick convert with args: %v", args)
	cmd := exec.Command("convert", args...)

	// Capture combined stdout+stderr so we can log if conversion fails:
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if outputErr := cmd.Run(); outputErr != nil {
		// Dump the full convert output for diagnosis:
		log.Printf("ConvertImageToPDF: ImageMagick output:\n%s", buf.String())
		return "", fmt.Errorf("imagemagick convert failed (exit: %v): %s", outputErr, buf.String())
	}

	log.Printf("ConvertImageToPDF: successfully created PDF = %s", outPDF)
	return outPDF, nil
}

// ConvertImageToPDFWithSettings takes an input image path and custom resolution/DPI settings,
// converts it to PDF using the specified settings, falling back to environment defaults if empty.
func ConvertImageToPDFWithSettings(imgPath, resolution string, dpi float64) (string, error) {
	ext := strings.ToLower(filepath.Ext(imgPath))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return "", fmt.Errorf("ConvertImageToPDFWithSettings: unsupported extension %q", ext)
	}

	// 1. Parse resolution in pixels (use custom or fall back to environment):
	var resWpx, resHpx int
	var err error
	if resolution != "" {
		resWpx, resHpx, err = parseResolutionString(resolution)
	} else {
		resWpx, resHpx, err = parseEnvResolution()
	}
	if err != nil {
		return "", err
	}

	// 2. Parse DPI (use custom or fall back to environment):
	var finalDPI float64
	if dpi > 0 {
		finalDPI = dpi
	} else {
		finalDPI, err = parseEnvDPI()
		if err != nil {
			return "", err
		}
	}

	// 3. Build output PDF path (same directory, same basename):
	base := strings.TrimSuffix(filepath.Base(imgPath), ext)
	outPDF := filepath.Join(filepath.Dir(imgPath), base+".pdf")

	// --- LOGGING FOR DIAGNOSIS: print computed values ----
	log.Printf("ConvertImageToPDFWithSettings: input image = %s", imgPath)
	log.Printf("ConvertImageToPDFWithSettings: target resolution = %dx%d px, DPI = %.2f", resWpx, resHpx, finalDPI)

	// 4. Use ImageMagick's "convert" with the same logic as ConvertImageToPDF
	args := []string{
		imgPath,
		"-density", fmt.Sprintf("%.2f", finalDPI),
		"-resize", fmt.Sprintf("%dx%d>", resWpx, resHpx),
		"-background", "white",
		"-gravity", "center",
		"-extent", fmt.Sprintf("%dx%d", resWpx, resHpx),
		"-quality", "100",
		outPDF,
	}
	log.Printf("ConvertImageToPDFWithSettings: running ImageMagick convert with args: %v", args)
	cmd := exec.Command("convert", args...)

	// Capture combined stdout+stderr so we can log if conversion fails:
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if outputErr := cmd.Run(); outputErr != nil {
		// Dump the full convert output for diagnosis:
		log.Printf("ConvertImageToPDFWithSettings: ImageMagick output:\n%s", buf.String())
		return "", fmt.Errorf("imagemagick convert failed (exit: %v): %s", outputErr, buf.String())
	}

	log.Printf("ConvertImageToPDFWithSettings: successfully created PDF = %s", outPDF)
	return outPDF, nil
}
