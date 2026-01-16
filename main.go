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

// validVideoExtensions lists supported video file extensions
var validVideoExtensions = map[string]bool{
	".mp4": true, ".mov": true, ".avi": true, ".mkv": true,
	".webm": true, ".m4v": true, ".wmv": true, ".flv": true,
}

func main() {
	// Define flags
	modelFlag := flag.String("model", "base", "Whisper model size (tiny/base/small/medium/large)")
	styleFlag := flag.String("style", "style_guide.md", "Path to style guide file")
	outputFlag := flag.String("output", "", "Output file path (default: auto-generated from video name)")
	forceFlag := flag.Bool("force", false, "Overwrite output file if it exists")
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

	// Validate video file extension
	ext := strings.ToLower(filepath.Ext(videoPath))
	if !validVideoExtensions[ext] {
		fmt.Fprintf(os.Stderr, "Error: unsupported video format '%s'. Supported formats: mp4, mov, avi, mkv, webm, m4v, wmv, flv\n", ext)
		os.Exit(1)
	}

	// Validate model size using shared constant
	if !transcribe.ValidModels[*modelFlag] {
		fmt.Fprintf(os.Stderr, "Error: invalid model size '%s'. Use: tiny, base, small, medium, or large\n", *modelFlag)
		os.Exit(1)
	}

	// Determine output path
	outputPath := *outputFlag
	if outputPath == "" {
		baseName := filepath.Base(videoPath)
		vidExt := filepath.Ext(baseName)
		nameWithoutExt := strings.TrimSuffix(baseName, vidExt)
		outputPath = nameWithoutExt + ".md"
	}

	// Validate output path (prevent path traversal)
	if err := validateOutputPath(outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check for overwrite
	if !*forceFlag {
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Fprintf(os.Stderr, "Error: output file already exists: %s\nUse --force to overwrite\n", outputPath)
			os.Exit(1)
		}
	}

	// Run the pipeline
	if err := run(videoPath, *modelFlag, *styleFlag, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// validateOutputPath checks for path traversal and ensures the output directory exists
func validateOutputPath(outputPath string) error {
	// Get absolute path
	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Check if the output path is within the current directory or a subdirectory
	// For security, we only allow relative paths within the current directory
	relPath, err := filepath.Rel(cwd, absPath)
	if err != nil {
		return fmt.Errorf("invalid output path: %w", err)
	}

	// Reject paths that traverse outside current directory
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("output path must be within current directory (no path traversal): %s", outputPath)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(absPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		return fmt.Errorf("output directory does not exist: %s", parentDir)
	}

	return nil
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

	// Validate blog content before writing
	blogPost = strings.TrimSpace(blogPost)
	if blogPost == "" {
		return fmt.Errorf("generated blog post is empty")
	}

	// Step 3: Write output file
	fmt.Println("\n[3/3] Writing output file...")
	if err := os.WriteFile(outputPath, []byte(blogPost+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Printf("\nBlog post saved to: %s\n", outputPath)
	return nil
}
