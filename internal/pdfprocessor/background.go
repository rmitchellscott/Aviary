// Package pdfprocessor provides PDF manipulation utilities
package pdfprocessor

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"github.com/rmitchellscott/aviary/internal/logging"
)

// imageInfo holds metadata about an embedded image for sorting
type imageInfo struct {
	ObjNr  int
	Width  int
	Height int
	Area   int
	PageNr int
}

// RemoveBackgroundImages processes a PDF and removes the smallest image
// from pages that have 2 or more embedded images. The smallest image is
// assumed to be a background or watermark.
// Returns the number of images removed, or an error.
func RemoveBackgroundImages(inputPath, outputPath string) (int, error) {
	// Open the input file
	inFile, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open input PDF: %w", err)
	}
	defer inFile.Close()

	// Read PDF context
	conf := model.NewDefaultConfiguration()
	ctx, err := api.ReadContext(inFile, conf)
	if err != nil {
		return 0, fmt.Errorf("failed to read PDF context: %w", err)
	}

	// Optimize to build the xref table properly
	if err := api.OptimizeContext(ctx); err != nil {
		return 0, fmt.Errorf("failed to optimize PDF context: %w", err)
	}

	// Initialize page tree for PageDict access
	if err := ctx.EnsurePageCount(); err != nil {
		return 0, fmt.Errorf("failed to ensure page count: %w", err)
	}

	// Reopen file for Images API (needs fresh ReadSeeker)
	inFile.Seek(0, io.SeekStart)

	// Get all images for all pages
	allImages, err := api.Images(inFile, nil, conf)
	if err != nil {
		return 0, fmt.Errorf("failed to get images from PDF: %w", err)
	}

	pageCount := len(allImages)
	removedCount := 0

	// Process each page
	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		// Find images for this page
		var pageImages []imageInfo
		for _, pageMap := range allImages {
			for objNr, img := range pageMap {
				if img.PageNr == pageNum {
					pageImages = append(pageImages, imageInfo{
						ObjNr:  objNr,
						Width:  img.Width,
						Height: img.Height,
						Area:   img.Width * img.Height,
						PageNr: pageNum,
					})
				}
			}
		}

		// Only process pages with 2+ images
		if len(pageImages) < 2 {
			continue
		}

		// Sort by area to find smallest
		sort.Slice(pageImages, func(i, j int) bool {
			return pageImages[i].Area < pageImages[j].Area
		})

		// Remove smallest image (assumed to be background)
		smallest := pageImages[0]
		logging.Logf("[PDFPROCESSOR] Page %d: removing background image (%dx%d)", pageNum, smallest.Width, smallest.Height)

		if err := removeImageFromPage(ctx, pageNum, smallest.ObjNr); err != nil {
			logging.Logf("[PDFPROCESSOR] Warning: failed to remove image from page %d: %v", pageNum, err)
			continue
		}
		removedCount++
	}

	if removedCount == 0 {
		logging.Logf("[PDFPROCESSOR] No background images to remove, copying file as-is")
		inFile.Seek(0, io.SeekStart)
		outFile, err := os.Create(outputPath)
		if err != nil {
			return 0, fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()
		if _, err := io.Copy(outFile, inFile); err != nil {
			return 0, fmt.Errorf("failed to copy file: %w", err)
		}
		return 0, nil
	}

	// Write the modified PDF
	outFile, err := os.Create(outputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if err := api.WriteContext(ctx, outFile); err != nil {
		return 0, fmt.Errorf("failed to write modified PDF: %w", err)
	}

	logging.Logf("[PDFPROCESSOR] Successfully removed %d background image(s)", removedCount)
	return removedCount, nil
}

func removeImageFromPage(ctx *model.Context, pageNr int, objNr int) error {
	pageDict, _, inheritedAttrs, err := ctx.PageDict(pageNr, true)
	if err != nil {
		return fmt.Errorf("failed to get page dict: %w", err)
	}

	var resDict types.Dict
	if inheritedAttrs != nil && inheritedAttrs.Resources != nil {
		resDict = inheritedAttrs.Resources
	} else {
		resDict = pageDict.DictEntry("Resources")
	}

	if resDict == nil {
		return fmt.Errorf("page %d has no Resources dictionary", pageNr)
	}

	xobjEntry, found := resDict.Find("XObject")
	if !found {
		return fmt.Errorf("page %d Resources has no XObject entry", pageNr)
	}

	var xobjDict types.Dict
	switch v := xobjEntry.(type) {
	case types.Dict:
		xobjDict = v
	case types.IndirectRef:
		deref, err := ctx.Dereference(v)
		if err != nil {
			return fmt.Errorf("failed to dereference XObject dict: %w", err)
		}
		var ok bool
		xobjDict, ok = deref.(types.Dict)
		if !ok {
			return fmt.Errorf("XObject is not a dictionary")
		}
	default:
		return fmt.Errorf("unexpected XObject type: %T", xobjEntry)
	}

	var keyToRemove string
	for key, val := range xobjDict {
		if indRef, ok := val.(types.IndirectRef); ok {
			if int(indRef.ObjectNumber) == objNr {
				keyToRemove = key
				break
			}
		}
	}

	if keyToRemove == "" {
		return fmt.Errorf("could not find XObject key for objNr %d", objNr)
	}

	xobjDict.Delete(keyToRemove)
	ctx.FreeObject(objNr)

	return nil
}
