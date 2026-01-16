package transcribe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ValidModels is the list of valid whisper model sizes
var ValidModels = map[string]bool{
	"tiny": true, "base": true, "small": true, "medium": true, "large": true,
}

// Default timeouts for external commands
const (
	FFmpegTimeout  = 30 * time.Minute  // Audio extraction timeout
	WhisperTimeout = 60 * time.Minute  // Transcription timeout (can be slow for large files)
	MaxVideoSize   = 10 * 1024 * 1024 * 1024 // 10GB max video size
)

// ModelPath returns the expected path for a whisper model
func ModelPath(modelSize string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "whisper", fmt.Sprintf("ggml-%s.bin", modelSize))
}

// EnsureModel checks if the model exists and provides download instructions if not
func EnsureModel(modelSize string) error {
	modelPath := ModelPath(modelSize)
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("whisper model not found at %s\n\nDownload it with:\n  mkdir -p ~/.cache/whisper\n  curl -L -o %s https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-%s.bin",
			modelPath, modelPath, modelSize)
	}
	return nil
}

// findWhisperCLI finds the whisper CLI tool
func findWhisperCLI() (string, error) {
	// Try common names for whisper.cpp CLI
	names := []string{"whisper-cpp", "whisper", "main"}
	for _, name := range names {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}

	// Check common installation paths
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, "whisper.cpp", "main"),
		filepath.Join(home, "whisper.cpp", "build", "bin", "main"),
		"/usr/local/bin/whisper-cpp",
		"/opt/homebrew/bin/whisper-cpp",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("whisper.cpp CLI not found\n\nInstall whisper.cpp:\n  brew install whisper-cpp\n\nOr build from source:\n  git clone https://github.com/ggerganov/whisper.cpp\n  cd whisper.cpp && make")
}

// extractAudio extracts audio from video file using ffmpeg
func extractAudio(ctx context.Context, videoPath string) (string, func(), error) {
	// Create unique temp file for audio
	audioFile, err := os.CreateTemp("", "video-journal-audio-*.wav")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp audio file: %w", err)
	}
	audioPath := audioFile.Name()
	audioFile.Close() // Close so ffmpeg can write to it

	cleanup := func() {
		os.Remove(audioPath)
	}

	// Use ffmpeg to extract audio as 16kHz mono WAV (required by whisper)
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y",
		"-i", videoPath,
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		audioPath,
	)
	cmd.Stderr = nil // Suppress ffmpeg output

	if err := cmd.Run(); err != nil {
		cleanup()
		if ctx.Err() == context.DeadlineExceeded {
			return "", nil, fmt.Errorf("ffmpeg audio extraction timed out after %v", FFmpegTimeout)
		}
		return "", nil, fmt.Errorf("ffmpeg audio extraction failed: %w\nMake sure ffmpeg is installed", err)
	}

	return audioPath, cleanup, nil
}

// TranscribeVideo transcribes a video file using whisper.cpp CLI
func TranscribeVideo(videoPath string, modelSize string) (string, error) {
	// Check video file exists and validate size
	info, err := os.Stat(videoPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("video file not found: %s", videoPath)
	}
	if err != nil {
		return "", fmt.Errorf("cannot access video file: %w", err)
	}
	if info.Size() > MaxVideoSize {
		return "", fmt.Errorf("video file too large: %d bytes (max: %d bytes)", info.Size(), MaxVideoSize)
	}

	// Ensure model is available
	if err := EnsureModel(modelSize); err != nil {
		return "", err
	}

	// Find whisper CLI
	whisperCLI, err := findWhisperCLI()
	if err != nil {
		return "", err
	}

	// Create context with timeout for ffmpeg
	ffmpegCtx, ffmpegCancel := context.WithTimeout(context.Background(), FFmpegTimeout)
	defer ffmpegCancel()

	fmt.Println("Extracting audio from video...")
	audioPath, audioCleanup, err := extractAudio(ffmpegCtx, videoPath)
	if err != nil {
		return "", err
	}
	defer audioCleanup()

	// Create unique temp file prefix for whisper output
	outputFile, err := os.CreateTemp("", "video-journal-transcript-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp output file: %w", err)
	}
	outputBase := outputFile.Name()
	outputFile.Close()
	os.Remove(outputBase) // Remove the placeholder, whisper will create files with this prefix

	// Cleanup function for all whisper output files
	cleanupWhisperOutputs := func() {
		// Whisper creates files like: outputBase.txt, outputBase.vtt, outputBase.srt, etc.
		extensions := []string{".txt", ".vtt", ".srt", ".json", ".csv", ".lrc"}
		for _, ext := range extensions {
			os.Remove(outputBase + ext)
		}
	}
	defer cleanupWhisperOutputs()

	fmt.Println("Transcribing audio with whisper.cpp...")
	modelPath := ModelPath(modelSize)

	// Create context with timeout for whisper
	whisperCtx, whisperCancel := context.WithTimeout(context.Background(), WhisperTimeout)
	defer whisperCancel()

	// Run whisper.cpp CLI
	cmd := exec.CommandContext(whisperCtx, whisperCLI,
		"-m", modelPath,
		"-f", audioPath,
		"-otxt",
		"-of", outputBase,
		"--no-timestamps",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if whisperCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("whisper transcription timed out after %v", WhisperTimeout)
		}
		return "", fmt.Errorf("whisper transcription failed: %w\nOutput: %s", err, string(output))
	}

	// Read the transcript file
	transcriptPath := outputBase + ".txt"
	transcript, err := os.ReadFile(transcriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read transcript: %w", err)
	}

	result := strings.TrimSpace(string(transcript))
	if result == "" {
		return "", fmt.Errorf("no speech detected in video")
	}

	return result, nil
}
