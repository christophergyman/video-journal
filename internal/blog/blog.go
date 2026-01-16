package blog

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	ClaudeTimeout      = 10 * time.Minute // Claude CLI timeout
	MaxTranscriptSize  = 500000           // ~500KB max transcript to send to Claude
)

// ConvertToBlog converts a transcript into a blog post using Claude CLI
func ConvertToBlog(transcript string, styleGuidePath string) (string, error) {
	// Validate transcript size
	if len(transcript) > MaxTranscriptSize {
		return "", fmt.Errorf("transcript too large: %d bytes (max: %d bytes)", len(transcript), MaxTranscriptSize)
	}

	// Load style guide
	styleGuide, err := loadStyleGuide(styleGuidePath)
	if err != nil {
		return "", err
	}

	// Build the prompt
	prompt := buildPrompt(transcript, styleGuide)

	fmt.Println("Generating blog post with Claude CLI...")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), ClaudeTimeout)
	defer cancel()

	// Execute claude CLI with the prompt
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("claude CLI timed out after %v", ClaudeTimeout)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude CLI error: %w\nstderr: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("claude CLI error: %w", err)
	}

	// Validate output is non-empty
	result := strings.TrimSpace(string(output))
	if result == "" {
		return "", fmt.Errorf("claude CLI returned empty output")
	}

	return result, nil
}

// LoadStyleGuide loads a style guide from the given path.
// If path is empty, returns the default style guide.
// If path is the default "style_guide.md" and doesn't exist, uses default silently.
// If path is explicitly specified and doesn't exist, returns an error.
func loadStyleGuide(path string) (string, error) {
	if path == "" {
		return getDefaultStyleGuide(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Only silently fallback for the default style_guide.md
			if path == "style_guide.md" {
				return getDefaultStyleGuide(), nil
			}
			return "", fmt.Errorf("style guide not found: %s", path)
		}
		return "", fmt.Errorf("failed to read style guide: %w", err)
	}

	return string(data), nil
}

func getDefaultStyleGuide() string {
	return `Write in a conversational, engaging tone.
Use clear headings to organize the content.
Include practical takeaways where relevant.
Keep paragraphs short and scannable.
Use active voice.`
}

func buildPrompt(transcript string, styleGuide string) string {
	return fmt.Sprintf(`Convert the following video transcript into a well-structured blog post.

## Style Guide
%s

## Instructions
1. Create an engaging title that captures the main topic
2. Write a brief introduction that hooks the reader
3. Organize the main content with clear headings
4. Preserve the key insights and examples from the transcript
5. Add a conclusion with key takeaways
6. Output the blog post in markdown format
7. Do not include phrases like "In this video" - write as if it was always a blog post

## Transcript
%s

## Blog Post (Markdown)`, styleGuide, transcript)
}
