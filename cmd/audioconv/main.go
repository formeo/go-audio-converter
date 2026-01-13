// audioconv is a command-line audio converter that works without ffmpeg.
// It supports WAV, FLAC, OGG, and MP3 formats using pure Go libraries.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/formeo/go-audio-converter/pkg/converter"
	"github.com/spf13/cobra"
)

var (
	version   = "0.1.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// Flags
var (
	outputPath  string
	outputFmt   string
	bitrate     int
	sampleRate  int
	channels    int
	normalize   bool
	trimSilence bool
	overwrite   bool
	recursive   bool
	verbose     bool
	quiet       bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		if !quiet {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "audioconv [input] [output]",
	Short: "Convert audio files without ffmpeg",
	Long: `audioconv - Pure Go audio converter

Convert audio files between WAV, FLAC, OGG, and MP3 formats
without requiring ffmpeg or any external dependencies.

Examples:
  audioconv input.wav output.mp3
  audioconv input.flac output.mp3 --bitrate 320
  audioconv ./music --output ./converted --format mp3
  audioconv input.wav --normalize --trim-silence output.mp3`,
	Version: version,
	Args:    cobra.MinimumNArgs(1),
	RunE:    runConvert,
}

func init() {
	// Conversion flags
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output file or directory")
	rootCmd.Flags().StringVarP(&outputFmt, "format", "f", "mp3", "Output format (mp3, wav)")
	rootCmd.Flags().IntVarP(&bitrate, "bitrate", "b", 192, "MP3 bitrate (128, 192, 256, 320)")
	rootCmd.Flags().IntVarP(&sampleRate, "sample-rate", "s", 0, "Output sample rate (0 = keep original)")
	rootCmd.Flags().IntVarP(&channels, "channels", "c", 0, "Output channels (1=mono, 2=stereo, 0=keep)")
	
	// Processing flags
	rootCmd.Flags().BoolVarP(&normalize, "normalize", "n", false, "Normalize audio levels")
	rootCmd.Flags().BoolVarP(&trimSilence, "trim-silence", "t", false, "Trim silence from start/end")
	rootCmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing files")
	
	// Directory flags
	rootCmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "Process directories recursively")
	
	// Output flags
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress all output")
	
	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(formatsCmd)
}

func runConvert(cmd *cobra.Command, args []string) error {
	input := args[0]
	
	// Determine output
	var output string
	if len(args) > 1 {
		output = args[1]
	} else if outputPath != "" {
		output = outputPath
	}
	
	// Setup converter options
	opts := converter.Options{
		Bitrate:     bitrate,
		SampleRate:  sampleRate,
		Channels:    channels,
		Normalize:   normalize,
		TrimSilence: trimSilence,
		Overwrite:   overwrite,
	}
	conv := converter.NewWithOptions(opts)
	
	// Check if input is directory
	info, err := os.Stat(input)
	if err != nil {
		return fmt.Errorf("cannot access %s: %w", input, err)
	}
	
	if info.IsDir() {
		return convertDirectory(conv, input, output)
	}
	
	return convertFile(conv, input, output)
}

func convertFile(conv *converter.Converter, input, output string) error {
	// Auto-generate output name if not provided
	if output == "" {
		ext := filepath.Ext(input)
		output = strings.TrimSuffix(input, ext) + "." + outputFmt
	}
	
	if verbose && !quiet {
		fmt.Printf("Converting: %s → %s\n", input, output)
	}
	
	start := time.Now()
	if err := conv.ConvertFile(input, output); err != nil {
		return fmt.Errorf("conversion failed: %w", err)
	}
	
	if !quiet {
		elapsed := time.Since(start)
		outInfo, _ := os.Stat(output)
		size := "unknown"
		if outInfo != nil {
			size = formatSize(outInfo.Size())
		}
		fmt.Printf("✓ %s (%s, %v)\n", filepath.Base(output), size, elapsed.Round(time.Millisecond))
	}
	
	return nil
}

func convertDirectory(conv *converter.Converter, inputDir, outputDir string) error {
	if outputDir == "" {
		outputDir = inputDir + "_converted"
	}
	
	if !quiet {
		fmt.Printf("Converting directory: %s → %s\n", inputDir, outputDir)
	}
	
	start := time.Now()
	result, err := conv.ConvertDir(inputDir, outputDir, converter.Format(outputFmt))
	if err != nil {
		return err
	}
	
	if !quiet {
		fmt.Printf("\nConverted: %d files\n", len(result.Converted))
		if len(result.Failed) > 0 {
			fmt.Printf("Failed:    %d files\n", len(result.Failed))
			if verbose {
				for _, f := range result.Failed {
					fmt.Printf("  ✗ %s: %s\n", f.Path, f.Error)
				}
			}
		}
		fmt.Printf("Time:      %v\n", time.Since(start).Round(time.Millisecond))
	}
	
	return nil
}

// Subcommands

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start HTTP API server",
	Long:  `Start an HTTP server that provides audio conversion as an API.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetString("port")
		host, _ := cmd.Flags().GetString("host")
		return runServer(host, port)
	},
}

func init() {
	serveCmd.Flags().StringP("port", "p", "8080", "Server port")
	serveCmd.Flags().StringP("host", "H", "0.0.0.0", "Server host")
}

var infoCmd = &cobra.Command{
	Use:   "info [file]",
	Short: "Show audio file information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return showFileInfo(args[0])
	},
}

func showFileInfo(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	stat, _ := file.Stat()
	format := converter.DetectFormat(path)
	
	if format == converter.FormatUnknown {
		return fmt.Errorf("unknown audio format")
	}
	
	info, err := converter.GetInfo(file, format)
	if err != nil {
		return err
	}
	
	fmt.Printf("File:        %s\n", path)
	fmt.Printf("Size:        %s\n", formatSize(stat.Size()))
	fmt.Printf("Format:      %s\n", info.Format)
	fmt.Printf("Duration:    %.2f seconds\n", info.Duration)
	fmt.Printf("Sample Rate: %d Hz\n", info.SampleRate)
	fmt.Printf("Channels:    %d\n", info.Channels)
	fmt.Printf("Bit Depth:   %d\n", info.BitDepth)
	
	return nil
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("audioconv %s\n", version)
		fmt.Printf("  Build time: %s\n", buildTime)
		fmt.Printf("  Git commit: %s\n", gitCommit)
		fmt.Printf("  Go version: %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

var formatsCmd = &cobra.Command{
	Use:   "formats",
	Short: "List supported formats",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Supported input formats:")
		for _, f := range converter.SupportedInputFormats() {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println("\nSupported output formats:")
		for _, f := range converter.SupportedOutputFormats() {
			fmt.Printf("  - %s\n", f)
		}
		fmt.Println("\nSupported conversions:")
		inputs := converter.SupportedInputFormats()
		outputs := converter.SupportedOutputFormats()
		for _, in := range inputs {
			for _, out := range outputs {
				if converter.CanConvert(in, out) {
					fmt.Printf("  %s → %s\n", in, out)
				}
			}
		}
	},
}

// Helpers

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
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
