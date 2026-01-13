// Package converter provides audio format conversion without external dependencies.
// It supports WAV, FLAC, OGG, and MP3 formats using pure Go libraries.
package converter

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Format represents audio format
type Format string

const (
	FormatWAV     Format = "wav"
	FormatMP3     Format = "mp3"
	FormatFLAC    Format = "flac"
	FormatOGG     Format = "ogg"
	FormatUnknown Format = ""
)

// Common errors
var (
	ErrUnsupportedFormat    = errors.New("unsupported audio format")
	ErrUnsupportedConversion = errors.New("unsupported conversion pair")
	ErrInvalidInput         = errors.New("invalid input file")
	ErrEncodingFailed       = errors.New("encoding failed")
	ErrDecodingFailed       = errors.New("decoding failed")
)

// Options configures the conversion process
type Options struct {
	// MP3 encoding options
	Bitrate    int // Target bitrate: 128, 192, 256, 320 kbps
	Quality    int // Quality level: 0 (best) to 9 (worst)
	
	// Audio options
	Channels   int // Number of channels: 1 (mono), 2 (stereo)
	SampleRate int // Sample rate in Hz: 44100, 48000, etc.
	
	// Processing options
	Normalize    bool // Normalize audio levels
	TrimSilence  bool // Remove silence from start/end
	
	// Output options
	Overwrite    bool // Overwrite existing files
	PreserveMeta bool // Try to preserve metadata (limited support)
}

// DefaultOptions returns sensible default conversion options
func DefaultOptions() Options {
	return Options{
		Bitrate:      192,
		Quality:      5,
		Channels:     2,
		SampleRate:   44100,
		Normalize:    false,
		TrimSilence:  false,
		Overwrite:    false,
		PreserveMeta: false,
	}
}

// Converter handles audio format conversion
type Converter struct {
	opts Options
	mu   sync.Mutex
}

// New creates a new Converter with default options
func New() *Converter {
	return &Converter{opts: DefaultOptions()}
}

// NewWithOptions creates a new Converter with custom options
func NewWithOptions(opts Options) *Converter {
	return &Converter{opts: opts}
}

// Options returns current converter options
func (c *Converter) Options() Options {
	return c.opts
}

// SetOptions updates converter options
func (c *Converter) SetOptions(opts Options) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.opts = opts
}

// ConvertFile converts an audio file from one format to another
func (c *Converter) ConvertFile(inputPath, outputPath string) error {
	// Detect formats from file extensions
	inputFmt := DetectFormat(inputPath)
	outputFmt := DetectFormat(outputPath)
	
	if inputFmt == FormatUnknown {
		return fmt.Errorf("%w: cannot detect input format for %s", ErrUnsupportedFormat, inputPath)
	}
	if outputFmt == FormatUnknown {
		return fmt.Errorf("%w: cannot detect output format for %s", ErrUnsupportedFormat, outputPath)
	}
	
	// Check if output exists
	if !c.opts.Overwrite {
		if _, err := os.Stat(outputPath); err == nil {
			return fmt.Errorf("output file already exists: %s (use Overwrite option)", outputPath)
		}
	}
	
	// Create output directory if needed
	outDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	
	// Open input file
	in, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input file: %w", err)
	}
	defer in.Close()
	
	// Create output file
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer out.Close()
	
	// Perform conversion
	if err := c.Convert(in, out, inputFmt, outputFmt); err != nil {
		// Clean up failed output
		os.Remove(outputPath)
		return err
	}
	
	return nil
}

// Convert converts audio data from reader to writer
func (c *Converter) Convert(in io.Reader, out io.Writer, inputFmt, outputFmt Format) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Same format - just copy
	if inputFmt == outputFmt {
		_, err := io.Copy(out, in)
		return err
	}
	
	// Get converter function
	switch {
	case inputFmt == FormatWAV && outputFmt == FormatMP3:
		return c.wavToMP3(in, out)
	case inputFmt == FormatFLAC && outputFmt == FormatMP3:
		return c.flacToMP3(in, out)
	case inputFmt == FormatOGG && outputFmt == FormatMP3:
		return c.oggToMP3(in, out)
	case inputFmt == FormatMP3 && outputFmt == FormatWAV:
		return c.mp3ToWAV(in, out)
	case inputFmt == FormatFLAC && outputFmt == FormatWAV:
		return c.flacToWAV(in, out)
	case inputFmt == FormatOGG && outputFmt == FormatWAV:
		return c.oggToWAV(in, out)
	case inputFmt == FormatWAV && outputFmt == FormatWAV:
		// Resample/rechannelize
		return c.processWAV(in, out)
	default:
		return fmt.Errorf("%w: %s to %s", ErrUnsupportedConversion, inputFmt, outputFmt)
	}
}

// ConvertDir converts all audio files in a directory
func (c *Converter) ConvertDir(inputDir, outputDir string, outputFmt Format) (*BatchResult, error) {
	result := &BatchResult{
		Converted: make([]string, 0),
		Failed:    make([]FailedFile, 0),
	}
	
	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if info.IsDir() {
			return nil
		}
		
		// Check if it's a supported audio file
		inputFmt := DetectFormat(path)
		if inputFmt == FormatUnknown {
			return nil // Skip unsupported files
		}
		
		// Skip if same format
		if inputFmt == outputFmt {
			return nil
		}
		
		// Build output path
		relPath, err := filepath.Rel(inputDir, path)
		if err != nil {
			return err
		}
		
		ext := filepath.Ext(relPath)
		outPath := filepath.Join(outputDir, strings.TrimSuffix(relPath, ext)+"."+string(outputFmt))
		
		// Convert file
		if err := c.ConvertFile(path, outPath); err != nil {
			result.Failed = append(result.Failed, FailedFile{
				Path:  path,
				Error: err.Error(),
			})
		} else {
			result.Converted = append(result.Converted, outPath)
		}
		
		return nil
	})
	
	return result, err
}

// BatchResult contains results of batch conversion
type BatchResult struct {
	Converted []string     `json:"converted"`
	Failed    []FailedFile `json:"failed"`
}

// FailedFile represents a file that failed to convert
type FailedFile struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// DetectFormat detects audio format from file path
func DetectFormat(path string) Format {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "wav", "wave":
		return FormatWAV
	case "mp3":
		return FormatMP3
	case "flac":
		return FormatFLAC
	case "ogg", "oga", "ogv":
		return FormatOGG
	default:
		return FormatUnknown
	}
}

// DetectFormatFromMIME detects format from MIME type
func DetectFormatFromMIME(mime string) Format {
	mime = strings.ToLower(strings.Split(mime, ";")[0])
	switch mime {
	case "audio/wav", "audio/wave", "audio/x-wav":
		return FormatWAV
	case "audio/mpeg", "audio/mp3":
		return FormatMP3
	case "audio/flac", "audio/x-flac":
		return FormatFLAC
	case "audio/ogg", "audio/vorbis", "application/ogg":
		return FormatOGG
	default:
		return FormatUnknown
	}
}

// SupportedInputFormats returns list of supported input formats
func SupportedInputFormats() []Format {
	return []Format{FormatWAV, FormatMP3, FormatFLAC, FormatOGG}
}

// SupportedOutputFormats returns list of supported output formats
func SupportedOutputFormats() []Format {
	return []Format{FormatWAV, FormatMP3}
}

// CanConvert checks if conversion between formats is supported
func CanConvert(from, to Format) bool {
	conversions := map[Format][]Format{
		FormatWAV:  {FormatMP3, FormatWAV},
		FormatFLAC: {FormatMP3, FormatWAV},
		FormatOGG:  {FormatMP3, FormatWAV},
		FormatMP3:  {FormatWAV},
	}
	
	targets, ok := conversions[from]
	if !ok {
		return false
	}
	
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}
