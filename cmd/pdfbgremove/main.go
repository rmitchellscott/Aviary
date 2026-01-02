package main

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"github.com/rmitchellscott/aviary/internal/security"
)

type imageInfo struct {
	ObjNr  int
	Width  int
	Height int
	Area   int
	Size   int64
	PageNr int
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: pdfbgremove <input.pdf> [output.pdf]")
		os.Exit(1)
	}

	inputPath := os.Args[1]
	outputPath := inputPath[:len(inputPath)-4] + "_cleaned.pdf"
	if len(os.Args) >= 3 {
		outputPath = os.Args[2]
	}

	removed, err := removeBackgroundImages(inputPath, outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed %d background image(s)\n", removed)
	fmt.Printf("Output saved to: %s\n", outputPath)
}

func removeBackgroundImages(inputPath, outputPath string) (int, error) {
	secureInput, err := security.NewSecurePathFromExisting(inputPath)
	if err != nil {
		return 0, fmt.Errorf("invalid input path: %w", err)
	}
	secureOutput, err := security.NewSecurePathFromExisting(outputPath)
	if err != nil {
		return 0, fmt.Errorf("invalid output path: %w", err)
	}

	inFile, err := security.SafeOpen(secureInput)
	if err != nil {
		return 0, fmt.Errorf("failed to open input PDF: %w", err)
	}
	defer inFile.Close()

	conf := model.NewDefaultConfiguration()
	ctx, err := api.ReadContext(inFile, conf)
	if err != nil {
		return 0, fmt.Errorf("failed to read PDF context: %w", err)
	}

	if err := api.OptimizeContext(ctx); err != nil {
		return 0, fmt.Errorf("failed to optimize PDF context: %w", err)
	}

	if err := ctx.EnsurePageCount(); err != nil {
		return 0, fmt.Errorf("failed to ensure page count: %w", err)
	}

	inFile.Seek(0, io.SeekStart)
	allImages, err := api.Images(inFile, nil, conf)
	if err != nil {
		return 0, fmt.Errorf("failed to get images from PDF: %w", err)
	}

	pageCount := len(allImages)
	fmt.Printf("Processing %d pages...\n", pageCount)

	removedCount := 0

	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		var pageImages []imageInfo
		for _, pageMap := range allImages {
			for objNr, img := range pageMap {
				if img.PageNr == pageNum {
					pageImages = append(pageImages, imageInfo{
						ObjNr:  objNr,
						Width:  img.Width,
						Height: img.Height,
						Area:   img.Width * img.Height,
						Size:   img.Size,
						PageNr: pageNum,
					})
				}
			}
		}

		if len(pageImages) < 2 {
			continue
		}

		sort.Slice(pageImages, func(i, j int) bool {
			return pageImages[i].Size < pageImages[j].Size
		})

		smallest := pageImages[0]
		fmt.Printf("Page %d: removing background (%dx%d, %d bytes)\n", pageNum, smallest.Width, smallest.Height, smallest.Size)

		if err := removeImageFromPage(ctx, pageNum, smallest.ObjNr); err != nil {
			fmt.Printf("  Warning: %v\n", err)
			continue
		}
		removedCount++
	}

	if removedCount == 0 {
		fmt.Println("No background images to remove, copying file as-is")
		inFile.Seek(0, io.SeekStart)
		outFile, err := security.SafeCreate(secureOutput)
		if err != nil {
			return 0, fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()
		if _, err := io.Copy(outFile, inFile); err != nil {
			return 0, fmt.Errorf("failed to copy file: %w", err)
		}
		return 0, nil
	}

	outFile, err := security.SafeCreate(secureOutput)
	if err != nil {
		return 0, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if err := api.WriteContext(ctx, outFile); err != nil {
		return 0, fmt.Errorf("failed to write modified PDF: %w", err)
	}

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
