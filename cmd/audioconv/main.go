package main

import (
	"fmt"
	"os"
	"time"

	"github.com/formeo/go-audio-converter/pkg/converter"
)

const version = "0.3.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Handle flags
	if os.Args[1] == "-v" || os.Args[1] == "--version" {
		fmt.Printf("audioconv %s\n", version)
		os.Exit(0)
	}

	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		printUsage()
		os.Exit(0)
	}

	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	input := os.Args[1]
	output := os.Args[2]

	// Validate input exists
	if _, err := os.Stat(input); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: input file not found: %s\n", input)
		os.Exit(1)
	}

	// Detect and show formats
	inputFmt := converter.DetectFormat(input)
	outputFmt := converter.DetectFormat(output)

	if inputFmt == converter.FormatUnknown {
		fmt.Fprintf(os.Stderr, "Error: unsupported input format: %s\n", input)
		fmt.Fprintln(os.Stderr, "Supported: wav, mp3, flac, ogg")
		os.Exit(1)
	}

	if outputFmt == converter.FormatUnknown {
		fmt.Fprintf(os.Stderr, "Error: unsupported output format: %s\n", output)
		fmt.Fprintln(os.Stderr, "Supported: wav, mp3, flac")
		os.Exit(1)
	}

	// Warn about encoding limitations
	if outputFmt == converter.FormatOGG {
		fmt.Fprintln(os.Stderr, "Error: OGG encoding not supported (no pure Go encoder)")
		fmt.Fprintln(os.Stderr, "Tip: convert to WAV, MP3, or FLAC instead")
		os.Exit(1)
	}

	fmt.Printf("Converting: %s (%s) -> %s (%s)\n", input, inputFmt, output, outputFmt)

	conv := converter.New()

	start := time.Now()
	if err := conv.ConvertFile(input, output); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Get output file size
	info, _ := os.Stat(output)
	size := formatSize(info.Size())

	fmt.Printf("Done in %v (%s)\n", time.Since(start).Round(time.Millisecond), size)
}

func printUsage() {
	fmt.Printf("audioconv %s - Pure Go audio converter\n\n", version)
	fmt.Println("Usage: audioconv <input> <output>")
	fmt.Println("")
	fmt.Println("Supported formats:")
	fmt.Println("  Decode: wav, mp3, flac, ogg")
	fmt.Println("  Encode: wav, mp3, flac")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  audioconv input.wav output.mp3")
	fmt.Println("  audioconv input.flac output.wav")
	fmt.Println("  audioconv input.ogg output.flac")
	fmt.Println("  audioconv input.mp3 output.flac")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -h, --help     Show this help")
	fmt.Println("  -v, --version  Show version")
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMG"[exp])
}
