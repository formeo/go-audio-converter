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
	"github.com/formeo/go-audio-converter/pkg/flacenc"
	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	gomp3 "github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
	"github.com/mewkiz/flac"
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

// PCMData represents decoded audio
type PCMData struct {
	Samples    []int16
	SampleRate int
	Channels   int
}

// Converter handles audio conversion
type Converter struct {
	Bitrate    int
	OGGQuality float32 // -0.1 to 1.0, default 0.4 (~128kbps)
}

// New creates a new converter
func New() *Converter {
	return &Converter{
		Bitrate:    192,
		OGGQuality: 0.4,
	}
}

// ConvertFile converts audio file
func (c *Converter) ConvertFile(inputPath, outputPath string) error {
	inputFmt := DetectFormat(inputPath)
	outputFmt := DetectFormat(outputPath)

	if inputFmt == FormatUnknown || outputFmt == FormatUnknown {
		return fmt.Errorf("unsupported format")
	}

	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer inFile.Close()

	var pcm *PCMData
	switch inputFmt {
	case FormatWAV:
		pcm, err = decodeWAV(inFile)
	case FormatMP3:
		pcm, err = decodeMP3(inFile)
	case FormatFLAC:
		pcm, err = decodeFLAC(inFile)
	case FormatOGG:
		pcm, err = decodeOGG(inFile)
	default:
		return fmt.Errorf("unsupported input format: %s", inputFmt)
	}
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	switch outputFmt {
	case FormatWAV:
		return encodeWAV(outFile, pcm)
	case FormatMP3:
		return encodeMP3(outFile, pcm)
	case FormatFLAC:
		return encodeFLAC(outFile, pcm)
	case FormatOGG:
		return c.encodeOGG(outFile, pcm)
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
	case "flac":
		return FormatFLAC
	case "ogg", "oga", "ogv":
		return FormatOGG
	default:
		return FormatUnknown
	}
}

// decodeWAV decodes WAV to PCM
func decodeWAV(r io.Reader) (*PCMData, error) {
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

	data, err := io.ReadAll(decoder)
	if err != nil {
		return nil, err
	}

	samples := make([]int16, len(data)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(data[i*2]) | int16(data[i*2+1])<<8
	}

	return &PCMData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   2,
	}, nil
}

// decodeFLAC decodes FLAC to PCM
func decodeFLAC(r io.Reader) (*PCMData, error) {
	stream, err := flac.New(r)
	if err != nil {
		return nil, fmt.Errorf("open flac: %w", err)
	}
	defer stream.Close()

	sampleRate := int(stream.Info.SampleRate)
	channels := int(stream.Info.NChannels)
	bitsPerSample := int(stream.Info.BitsPerSample)

	var samples []int16

	for {
		frame, err := stream.ParseNext()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse frame: %w", err)
		}

		nSamples := int(frame.Subframes[0].NSamples)

		for i := 0; i < nSamples; i++ {
			for ch := 0; ch < channels; ch++ {
				sample := frame.Subframes[ch].Samples[i]

				var normalized int16
				switch bitsPerSample {
				case 8:
					normalized = int16(sample << 8)
				case 16:
					normalized = int16(sample)
				case 24:
					normalized = int16(sample >> 8)
				case 32:
					normalized = int16(sample >> 16)
				default:
					normalized = int16(sample)
				}
				samples = append(samples, normalized)
			}
		}
	}

	return &PCMData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   channels,
	}, nil
}

// decodeOGG decodes OGG/Vorbis to PCM
func decodeOGG(r io.Reader) (*PCMData, error) {
	reader, err := oggvorbis.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("open ogg: %w", err)
	}

	sampleRate := reader.SampleRate()
	channels := reader.Channels()

	floatBuf := make([]float32, 8192)
	var samples []int16

	for {
		n, err := reader.Read(floatBuf)
		if n > 0 {
			for i := 0; i < n; i++ {
				sample := floatBuf[i]
				if sample > 1.0 {
					sample = 1.0
				} else if sample < -1.0 {
					sample = -1.0
				}
				samples = append(samples, int16(sample*32767))
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read ogg: %w", err)
		}
	}

	return &PCMData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   channels,
	}, nil
}

// encodeWAV encodes PCM to WAV
func encodeWAV(w io.Writer, pcm *PCMData) error {
	dataSize := len(pcm.Samples) * 2
	fileSize := 36 + dataSize
	byteRate := pcm.SampleRate * pcm.Channels * 2
	blockAlign := pcm.Channels * 2

	w.Write([]byte("RIFF"))
	binary.Write(w, binary.LittleEndian, uint32(fileSize))
	w.Write([]byte("WAVE"))

	w.Write([]byte("fmt "))
	binary.Write(w, binary.LittleEndian, uint32(16))
	binary.Write(w, binary.LittleEndian, uint16(1))
	binary.Write(w, binary.LittleEndian, uint16(pcm.Channels))
	binary.Write(w, binary.LittleEndian, uint32(pcm.SampleRate))
	binary.Write(w, binary.LittleEndian, uint32(byteRate))
	binary.Write(w, binary.LittleEndian, uint16(blockAlign))
	binary.Write(w, binary.LittleEndian, uint16(16))

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

	encoder := shinemp3.NewEncoder(pcm.SampleRate, pcm.Channels)

	if err := encoder.Write(w, pcm.Samples); err != nil {
		return fmt.Errorf("encode mp3: %w", err)
	}

	return nil
}

// encodeFLAC encodes PCM to FLAC
func encodeFLAC(w io.Writer, pcm *PCMData) error {
	enc := flacenc.NewEncoder(pcm.SampleRate, pcm.Channels, 16)

	samples32 := make([]int32, len(pcm.Samples))
	for i, s := range pcm.Samples {
		samples32[i] = int32(s)
	}

	return enc.Encode(w, samples32)
}

// encodeOGG encodes PCM to OGG/Vorbis
// Note: Pure Go Vorbis encoding doesn't exist. Use CGO with libvorbis (github.com/xlab/vorbis-go)
func (c *Converter) encodeOGG(w io.Writer, pcm *PCMData) error {
	return fmt.Errorf("OGG/Vorbis encoding not yet implemented - no pure Go encoder exists, consider CGO with libvorbis")
}
