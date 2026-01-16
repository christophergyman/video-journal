package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chezu/video-journal/internal/blog"
	"github.com/chezu/video-journal/internal/transcribe"
)

func main() {
	// Define flags
	modelFlag := flag.String("model", "base", "Whisper model size (tiny/base/small/medium/large)")
	styleFlag := flag.String("style", "style_guide.md", "Path to style guide file")
	outputFlag := flag.String("output", "", "Output file path (default: auto-generated from video name)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: video-journal [flags] <video-path>\n\n")
		fmt.Fprintf(os.Stderr, "Convert a video file into a blog post using AI.\n\n")
		fmt.Fprintf(os.Stderr, "Prerequisites:\n")
		fmt.Fprintf(os.Stderr, "  - claude CLI must be installed and authenticated\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  video-journal --model base my-video.mp4\n")
	}

	flag.Parse()

	// Check for video path argument
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}

	videoPath := args[0]

	// Validate model size
	validModels := map[string]bool{
		"tiny": true, "base": true, "small": true, "medium": true, "large": true,
	}
	if !validModels[*modelFlag] {
		fmt.Fprintf(os.Stderr, "Error: invalid model size '%s'. Use: tiny, base, small, medium, or large\n", *modelFlag)
		os.Exit(1)
	}

	// Determine output path
	outputPath := *outputFlag
	if outputPath == "" {
		baseName := filepath.Base(videoPath)
		ext := filepath.Ext(baseName)
		nameWithoutExt := strings.TrimSuffix(baseName, ext)
		outputPath = nameWithoutExt + ".md"
	}

	// Run the pipeline
	if err := run(videoPath, *modelFlag, *styleFlag, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(videoPath, modelSize, stylePath, outputPath string) error {
	fmt.Printf("Processing video: %s\n", videoPath)
	fmt.Printf("Using whisper model: %s\n", modelSize)

	// Step 1: Transcribe video
	fmt.Println("\n[1/3] Transcribing video...")
	transcript, err := transcribe.TranscribeVideo(videoPath, modelSize)
	if err != nil {
		return fmt.Errorf("transcription failed: %w", err)
	}
	fmt.Printf("Transcription complete (%d characters)\n", len(transcript))

	// Step 2: Convert to blog post
	fmt.Println("\n[2/3] Converting to blog post...")
	blogPost, err := blog.ConvertToBlog(transcript, stylePath)
	if err != nil {
		return fmt.Errorf("blog conversion failed: %w", err)
	}

	// Step 3: Write output file
	fmt.Println("\n[3/3] Writing output file...")
	if err := os.WriteFile(outputPath, []byte(blogPost), 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Printf("\nBlog post saved to: %s\n", outputPath)
	return nil
}
