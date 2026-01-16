package blog

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
)

const defaultModel = "claude-sonnet-4-20250514"

// ConvertToBlog converts a transcript into a blog post using Claude
func ConvertToBlog(transcript string, styleGuidePath string) (string, error) {
	// Check for API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY environment variable is required")
	}

	// Load style guide
	styleGuide, err := loadStyleGuide(styleGuidePath)
	if err != nil {
		return "", err
	}

	// Build the prompt
	prompt := buildPrompt(transcript, styleGuide)

	// Create Anthropic client
	client := anthropic.NewClient()

	fmt.Println("Generating blog post with Claude...")

	// Make API request
	message, err := client.Messages.New(context.Background(), anthropic.MessageNewParams{
		Model:     anthropic.Model(defaultModel),
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("Claude API error: %w", err)
	}

	// Extract text from response
	if len(message.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}

	var result string
	for _, block := range message.Content {
		if block.Type == "text" {
			result += block.Text
		}
	}

	return result, nil
}

func loadStyleGuide(path string) (string, error) {
	if path == "" {
		return getDefaultStyleGuide(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Style guide not found at %s, using default\n", path)
			return getDefaultStyleGuide(), nil
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
