# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

video-journal is a CLI tool that converts video files into blog posts. It uses a two-stage pipeline: transcription via whisper.cpp, then blog generation via Claude CLI.

## Build and Run Commands

```bash
# Build
go build -o video-journal

# Run
./video-journal <video-path>
./video-journal --model base --style style_guide.md my-video.mp4

# Clean dependencies
go mod tidy
```

## Architecture

The pipeline has two main stages:

1. **Transcription** (`internal/transcribe/`) - Extracts audio from video using ffmpeg, then transcribes using whisper.cpp CLI
2. **Blog Generation** (`internal/blog/`) - Sends transcript to Claude CLI with a style guide prompt, returns markdown blog post

Entry point is `main.go` which orchestrates the pipeline: transcribe → convert to blog → write output file.

## External Dependencies

- **ffmpeg** - Audio extraction from video (must be installed)
- **whisper.cpp** - Speech-to-text transcription (must be installed, model downloaded to `~/.cache/whisper/`)
- **claude CLI** - Blog post generation (must be installed and authenticated)
