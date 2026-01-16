package transcribe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
func extractAudio(videoPath string) (string, error) {
	// Create temp file for audio
	tempDir := os.TempDir()
	audioPath := filepath.Join(tempDir, "video-journal-audio.wav")

	// Use ffmpeg to extract audio as 16kHz mono WAV (required by whisper)
	cmd := exec.Command("ffmpeg", "-y",
		"-i", videoPath,
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		audioPath,
	)
	cmd.Stderr = nil // Suppress ffmpeg output

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg audio extraction failed: %w\nMake sure ffmpeg is installed", err)
	}

	return audioPath, nil
}

// TranscribeVideo transcribes a video file using whisper.cpp CLI
func TranscribeVideo(videoPath string, modelSize string) (string, error) {
	// Check video file exists
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return "", fmt.Errorf("video file not found: %s", videoPath)
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

	fmt.Println("Extracting audio from video...")
	audioPath, err := extractAudio(videoPath)
	if err != nil {
		return "", err
	}
	defer os.Remove(audioPath)

	// Create temp file for output
	tempDir := os.TempDir()
	outputBase := filepath.Join(tempDir, "video-journal-transcript")

	fmt.Println("Transcribing audio with whisper.cpp...")
	modelPath := ModelPath(modelSize)

	// Run whisper.cpp CLI
	cmd := exec.Command(whisperCLI,
		"-m", modelPath,
		"-f", audioPath,
		"-otxt",
		"-of", outputBase,
		"--no-timestamps",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("whisper transcription failed: %w\nOutput: %s", err, string(output))
	}

	// Read the transcript file
	transcriptPath := outputBase + ".txt"
	defer os.Remove(transcriptPath)

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
