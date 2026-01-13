package converter

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/hajimehoshi/go-mp3"
	"github.com/jfreymuth/oggvorbis"
	"github.com/mewkiz/flac"
)

// PCMData represents decoded audio data
type PCMData struct {
	Samples    []int16 // Interleaved samples
	SampleRate int
	Channels   int
	BitDepth   int
}

// decodeWAV decodes WAV file to PCM data
func decodeWAV(r io.Reader) (*PCMData, error) {
	// WAV decoder needs ReadSeeker, buffer if needed
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		// Read all data into buffer
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("read WAV data: %w", err)
		}
		rs = bytes.NewReader(data)
	}
	
	decoder := wav.NewDecoder(rs)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}
	
	// Read format info
	if err := decoder.FwdToPCM(); err != nil {
		return nil, fmt.Errorf("read WAV format: %w", err)
	}
	
	sampleRate := int(decoder.SampleRate)
	channels := int(decoder.NumChans)
	bitDepth := int(decoder.BitDepth)
	
	// Read all samples
	buf := &audio.IntBuffer{
		Data:   make([]int, 0),
		Format: &audio.Format{SampleRate: sampleRate, NumChannels: channels},
	}
	
	// Read in chunks
	chunkSize := 4096
	tmpBuf := &audio.IntBuffer{
		Data:   make([]int, chunkSize),
		Format: buf.Format,
	}
	
	for {
		n, err := decoder.PCMBuffer(tmpBuf)
		if err != nil {
			return nil, fmt.Errorf("decode WAV PCM: %w", err)
		}
		if n == 0 {
			break
		}
		buf.Data = append(buf.Data, tmpBuf.Data[:n]...)
	}
	
	// Convert to int16
	samples := make([]int16, len(buf.Data))
	
	// Normalize based on bit depth
	var maxVal float64
	switch bitDepth {
	case 8:
		maxVal = 128
	case 16:
		maxVal = 32768
	case 24:
		maxVal = 8388608
	case 32:
		maxVal = 2147483648
	default:
		maxVal = 32768
	}
	
	for i, s := range buf.Data {
		// Normalize to int16 range
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
		BitDepth:   16, // Normalized to 16-bit
	}, nil
}

// decodeFLAC decodes FLAC file to PCM data
func decodeFLAC(r io.Reader) (*PCMData, error) {
	// FLAC decoder needs ReadSeeker
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("read FLAC data: %w", err)
		}
		rs = bytes.NewReader(data)
	}
	
	stream, err := flac.New(rs)
	if err != nil {
		return nil, fmt.Errorf("open FLAC stream: %w", err)
	}
	defer stream.Close()
	
	info := stream.Info
	sampleRate := int(info.SampleRate)
	channels := int(info.NChannels)
	bitDepth := int(info.BitsPerSample)
	
	// Calculate total samples
	totalSamples := int(info.NSamples) * channels
	samples := make([]int16, 0, totalSamples)
	
	// Normalization factor
	var maxVal float64
	switch bitDepth {
	case 8:
		maxVal = 128
	case 16:
		maxVal = 32768
	case 24:
		maxVal = 8388608
	case 32:
		maxVal = 2147483648
	default:
		maxVal = 32768
	}
	
	// Read all frames
	for {
		frame, err := stream.ParseNext()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse FLAC frame: %w", err)
		}
		
		// Interleave channels
		nSamples := len(frame.Subframes[0].Samples)
		for i := 0; i < nSamples; i++ {
			for ch := 0; ch < channels; ch++ {
				s := frame.Subframes[ch].Samples[i]
				// Normalize to int16
				normalized := float64(s) / maxVal * 32767
				if normalized > 32767 {
					normalized = 32767
				} else if normalized < -32768 {
					normalized = -32768
				}
				samples = append(samples, int16(normalized))
			}
		}
	}
	
	return &PCMData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   16,
	}, nil
}

// decodeOGG decodes OGG Vorbis file to PCM data
func decodeOGG(r io.Reader) (*PCMData, error) {
	// OGG decoder needs ReadSeeker
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("read OGG data: %w", err)
		}
		rs = bytes.NewReader(data)
	}
	
	decoder, err := oggvorbis.NewReader(rs)
	if err != nil {
		return nil, fmt.Errorf("create OGG decoder: %w", err)
	}
	
	sampleRate := decoder.SampleRate()
	channels := decoder.Channels()
	
	// Read all samples (float32)
	floatSamples := make([]float32, 0)
	buf := make([]float32, 4096)
	
	for {
		n, err := decoder.Read(buf)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("decode OGG: %w", err)
		}
		if n == 0 {
			break
		}
		floatSamples = append(floatSamples, buf[:n]...)
	}
	
	// Convert float32 to int16
	samples := make([]int16, len(floatSamples))
	for i, f := range floatSamples {
		// Float samples are in range [-1.0, 1.0]
		s := f * 32767
		if s > 32767 {
			s = 32767
		} else if s < -32768 {
			s = -32768
		}
		samples[i] = int16(s)
	}
	
	return &PCMData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   channels,
		BitDepth:   16,
	}, nil
}

// decodeMP3 decodes MP3 file to PCM data
func decodeMP3(r io.Reader) (*PCMData, error) {
	decoder, err := mp3.NewDecoder(r)
	if err != nil {
		return nil, fmt.Errorf("create MP3 decoder: %w", err)
	}
	
	sampleRate := decoder.SampleRate()
	
	// MP3 decoder outputs stereo int16 samples
	// Read all data
	data, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("decode MP3: %w", err)
	}
	
	// Convert bytes to int16 samples (little-endian)
	samples := make([]int16, len(data)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(data[i*2]) | int16(data[i*2+1])<<8
	}
	
	return &PCMData{
		Samples:    samples,
		SampleRate: sampleRate,
		Channels:   2, // go-mp3 always outputs stereo
		BitDepth:   16,
	}, nil
}

// AudioInfo contains information about an audio file
type AudioInfo struct {
	Format     Format  `json:"format"`
	Duration   float64 `json:"duration_seconds"`
	SampleRate int     `json:"sample_rate"`
	Channels   int     `json:"channels"`
	BitDepth   int     `json:"bit_depth"`
	Bitrate    int     `json:"bitrate,omitempty"` // For MP3
}

// GetInfo returns information about an audio file
func GetInfo(r io.Reader, format Format) (*AudioInfo, error) {
	switch format {
	case FormatWAV:
		return getWAVInfo(r)
	case FormatFLAC:
		return getFLACInfo(r)
	case FormatOGG:
		return getOGGInfo(r)
	case FormatMP3:
		return getMP3Info(r)
	default:
		return nil, ErrUnsupportedFormat
	}
}

func getWAVInfo(r io.Reader) (*AudioInfo, error) {
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		rs = bytes.NewReader(data)
	}
	
	decoder := wav.NewDecoder(rs)
	if !decoder.IsValidFile() {
		return nil, fmt.Errorf("invalid WAV file")
	}
	
	dur, err := decoder.Duration()
	if err != nil {
		dur = 0
	}
	
	return &AudioInfo{
		Format:     FormatWAV,
		Duration:   dur.Seconds(),
		SampleRate: int(decoder.SampleRate),
		Channels:   int(decoder.NumChans),
		BitDepth:   int(decoder.BitDepth),
	}, nil
}

func getFLACInfo(r io.Reader) (*AudioInfo, error) {
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		rs = bytes.NewReader(data)
	}
	
	stream, err := flac.New(rs)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	
	info := stream.Info
	duration := float64(info.NSamples) / float64(info.SampleRate)
	
	return &AudioInfo{
		Format:     FormatFLAC,
		Duration:   duration,
		SampleRate: int(info.SampleRate),
		Channels:   int(info.NChannels),
		BitDepth:   int(info.BitsPerSample),
	}, nil
}

func getOGGInfo(r io.Reader) (*AudioInfo, error) {
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		rs = bytes.NewReader(data)
	}
	
	decoder, err := oggvorbis.NewReader(rs)
	if err != nil {
		return nil, err
	}
	
	return &AudioInfo{
		Format:     FormatOGG,
		Duration:   decoder.Length().Seconds(),
		SampleRate: decoder.SampleRate(),
		Channels:   decoder.Channels(),
		BitDepth:   16, // Vorbis uses float internally
	}, nil
}

func getMP3Info(r io.Reader) (*AudioInfo, error) {
	decoder, err := mp3.NewDecoder(r)
	if err != nil {
		return nil, err
	}
	
	// Calculate duration from length
	length := decoder.Length()
	sampleRate := decoder.SampleRate()
	duration := float64(length) / float64(sampleRate) / 4 // 4 bytes per sample (stereo int16)
	
	return &AudioInfo{
		Format:     FormatMP3,
		Duration:   duration,
		SampleRate: sampleRate,
		Channels:   2,
		BitDepth:   16,
	}, nil
}
