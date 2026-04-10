package main

import (
	"fmt"
	"os"

	"github.com/rmitchellscott/aviary/internal/pdfprocessor"
)

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

	removed, err := pdfprocessor.RemoveBackgroundImages(inputPath, outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Removed %d background image(s)\n", removed)
	fmt.Printf("Output saved to: %s\n", outputPath)
}
