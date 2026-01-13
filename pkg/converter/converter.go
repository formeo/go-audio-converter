package converter

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	shinemp3 "github.com/braheezy/shine-mp3/pkg/mp3"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	gomp3 "github.com/hajimehoshi/go-mp3"
)

// Format represents audio format
type Format string

const (
	FormatWAV     Format = "wav"
	FormatMP3     Format = "mp3"
	FormatUnknown Format = ""
)

// PCMData represents decoded audio
type PCMData struct {
	Samples    []int16
	SampleRate int
	Channels   int
}

// Converter handles audio conversion
type Converter struct {
	Bitrate int
}

// New creates a new converter
func New() *Converter {
	return &Converter{Bitrate: 192}
}

// ConvertFile converts audio file
func (c *Converter) ConvertFile(inputPath, outputPath string) error {
	inputFmt := DetectFormat(inputPath)
	outputFmt := DetectFormat(outputPath)

	if inputFmt == FormatUnknown || outputFmt == FormatUnknown {
		return fmt.Errorf("unsupported format")
	}

	// Read input
	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer inFile.Close()

	// Decode to PCM
	var pcm *PCMData
	switch inputFmt {
	case FormatWAV:
		pcm, err = decodeWAV(inFile)
	case FormatMP3:
		pcm, err = decodeMP3(inFile)
	default:
		return fmt.Errorf("unsupported input format: %s", inputFmt)
	}
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	// Create output
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	// Encode
	switch outputFmt {
	case FormatWAV:
		return encodeWAV(outFile, pcm)
	case FormatMP3:
		return encodeMP3(outFile, pcm)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFmt)
	}
}

// DetectFormat detects audio format from file extension
func DetectFormat(path string) Format {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "wav", "wave":
		return FormatWAV
	case "mp3":
		return FormatMP3
	default:
		return FormatUnknown
	}
}

// decodeWAV decodes WAV to PCM
func decodeWAV(r io.Reader) (*PCMData, error) {
	// Read all data (WAV decoder needs ReadSeeker)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	rs := bytes.NewReader(data)

	decoder := wav.NewDecoder(rs)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}

	if err := decoder.FwdToPCM(); err != nil {
		return nil, err
	}

	sampleRate := int(decoder.SampleRate)
	channels := int(decoder.NumChans)
	bitDepth := int(decoder.BitDepth)

	// Read all samples
	buf := &audio.IntBuffer{
		Data:   make([]int, 0),
		Format: &audio.Format{SampleRate: sampleRate, NumChannels: channels},
	}

	tmpBuf := &audio.IntBuffer{
		Data:   make([]int, 4096),
		Format: buf.Format,
	}

	for {
		n, err := decoder.PCMBuffer(tmpBuf)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			break
		}
		buf.Data = append(buf.Data, tmpBuf.Data[:n]...)
	}

	// Convert to int16
	samples := make([]int16, len(buf.Data))
	var maxVal float64 = 32768
	if bitDepth == 24 {
		maxVal = 8388608
	} else if bitDepth == 32 {
		maxVal = 2147483648
	}

	for i, s := range buf.Data {
		normalized := float64(s) / maxVal * 32767
		if normalized > 32767 {
			normalized = 32767
		} else if normalized < -32768 {
			normalized = -32768
		}
		samples[i] = int16(normalized)
	}

	return &PCMData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   channels,
	}, nil
}

// decodeMP3 decodes MP3 to PCM
func decodeMP3(r io.Reader) (*PCMData, error) {
	decoder, err := gomp3.NewDecoder(r)
	if err != nil {
		return nil, err
	}

	sampleRate := decoder.SampleRate()

	// Read all decoded data
	data, err := io.ReadAll(decoder)
	if err != nil {
		return nil, err
	}

	// Convert bytes to int16 (little-endian stereo)
	samples := make([]int16, len(data)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(data[i*2]) | int16(data[i*2+1])<<8
	}

	return &PCMData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   2, // go-mp3 always outputs stereo
	}, nil
}

// encodeWAV encodes PCM to WAV
func encodeWAV(w io.Writer, pcm *PCMData) error {
	dataSize := len(pcm.Samples) * 2
	fileSize := 36 + dataSize
	byteRate := pcm.SampleRate * pcm.Channels * 2
	blockAlign := pcm.Channels * 2

	// RIFF header
	w.Write([]byte("RIFF"))
	binary.Write(w, binary.LittleEndian, uint32(fileSize))
	w.Write([]byte("WAVE"))

	// fmt chunk
	w.Write([]byte("fmt "))
	binary.Write(w, binary.LittleEndian, uint32(16))
	binary.Write(w, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(w, binary.LittleEndian, uint16(pcm.Channels))
	binary.Write(w, binary.LittleEndian, uint32(pcm.SampleRate))
	binary.Write(w, binary.LittleEndian, uint32(byteRate))
	binary.Write(w, binary.LittleEndian, uint16(blockAlign))
	binary.Write(w, binary.LittleEndian, uint16(16)) // bits per sample

	// data chunk
	w.Write([]byte("data"))
	binary.Write(w, binary.LittleEndian, uint32(dataSize))

	for _, s := range pcm.Samples {
		binary.Write(w, binary.LittleEndian, s)
	}

	return nil
}

// encodeMP3 encodes PCM to MP3 using shine
func encodeMP3(w io.Writer, pcm *PCMData) error {
	if len(pcm.Samples) == 0 {
		return fmt.Errorf("no samples to encode")
	}

	// shine-mp3 encoder
	encoder := shinemp3.NewEncoder(pcm.SampleRate, pcm.Channels)

	// Encode and write
	if err := encoder.Write(w, pcm.Samples); err != nil {
		return fmt.Errorf("encode mp3: %w", err)
	}

	return nil
}