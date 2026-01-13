package converter

import (
	"encoding/binary"
	"fmt"
	"io"

	shinemp3 "github.com/braheezy/shine-mp3/pkg/mp3"
)

// encodeToMP3 encodes PCM data to MP3 format
func encodeToMP3(w io.Writer, pcm *PCMData) error {
	if pcm == nil || len(pcm.Samples) == 0 {
		return fmt.Errorf("no PCM data to encode")
	}
	
	// Create shine-mp3 encoder
	encoder := shinemp3.NewEncoder(pcm.SampleRate, pcm.Channels)
	
	// Write samples
	if err := encoder.Write(w, pcm.Samples); err != nil {
		return fmt.Errorf("encode MP3: %w", err)
	}
	
	return nil
}

// encodeToWAV encodes PCM data to WAV format
func encodeToWAV(w io.Writer, pcm *PCMData) error {
	if pcm == nil || len(pcm.Samples) == 0 {
		return fmt.Errorf("no PCM data to encode")
	}
	
	// Calculate sizes
	dataSize := len(pcm.Samples) * 2 // 2 bytes per int16 sample
	fileSize := 36 + dataSize        // Header size + data
	byteRate := pcm.SampleRate * pcm.Channels * 2
	blockAlign := pcm.Channels * 2
	
	// Write RIFF header
	if _, err := w.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(fileSize)); err != nil {
		return err
	}
	if _, err := w.Write([]byte("WAVE")); err != nil {
		return err
	}
	
	// Write fmt subchunk
	if _, err := w.Write([]byte("fmt ")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(16)); err != nil { // Subchunk1Size
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(1)); err != nil { // AudioFormat (PCM)
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(pcm.Channels)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(pcm.SampleRate)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(byteRate)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(16)); err != nil { // BitsPerSample
		return err
	}
	
	// Write data subchunk
	if _, err := w.Write([]byte("data")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(dataSize)); err != nil {
		return err
	}
	
	// Write samples
	for _, sample := range pcm.Samples {
		if err := binary.Write(w, binary.LittleEndian, sample); err != nil {
			return err
		}
	}
	
	return nil
}

// Conversion functions

// wavToMP3 converts WAV to MP3
func (c *Converter) wavToMP3(in io.Reader, out io.Writer) error {
	pcm, err := decodeWAV(in)
	if err != nil {
		return fmt.Errorf("decode WAV: %w", err)
	}
	
	// Apply processing if needed
	pcm = c.processPCM(pcm)
	
	return encodeToMP3(out, pcm)
}

// flacToMP3 converts FLAC to MP3
func (c *Converter) flacToMP3(in io.Reader, out io.Writer) error {
	pcm, err := decodeFLAC(in)
	if err != nil {
		return fmt.Errorf("decode FLAC: %w", err)
	}
	
	pcm = c.processPCM(pcm)
	
	return encodeToMP3(out, pcm)
}

// oggToMP3 converts OGG to MP3
func (c *Converter) oggToMP3(in io.Reader, out io.Writer) error {
	pcm, err := decodeOGG(in)
	if err != nil {
		return fmt.Errorf("decode OGG: %w", err)
	}
	
	pcm = c.processPCM(pcm)
	
	return encodeToMP3(out, pcm)
}

// mp3ToWAV converts MP3 to WAV
func (c *Converter) mp3ToWAV(in io.Reader, out io.Writer) error {
	pcm, err := decodeMP3(in)
	if err != nil {
		return fmt.Errorf("decode MP3: %w", err)
	}
	
	pcm = c.processPCM(pcm)
	
	return encodeToWAV(out, pcm)
}

// flacToWAV converts FLAC to WAV
func (c *Converter) flacToWAV(in io.Reader, out io.Writer) error {
	pcm, err := decodeFLAC(in)
	if err != nil {
		return fmt.Errorf("decode FLAC: %w", err)
	}
	
	pcm = c.processPCM(pcm)
	
	return encodeToWAV(out, pcm)
}

// oggToWAV converts OGG to WAV
func (c *Converter) oggToWAV(in io.Reader, out io.Writer) error {
	pcm, err := decodeOGG(in)
	if err != nil {
		return fmt.Errorf("decode OGG: %w", err)
	}
	
	pcm = c.processPCM(pcm)
	
	return encodeToWAV(out, pcm)
}

// processWAV processes WAV (resample, rechannelize)
func (c *Converter) processWAV(in io.Reader, out io.Writer) error {
	pcm, err := decodeWAV(in)
	if err != nil {
		return fmt.Errorf("decode WAV: %w", err)
	}
	
	pcm = c.processPCM(pcm)
	
	return encodeToWAV(out, pcm)
}

// processPCM applies processing options to PCM data
func (c *Converter) processPCM(pcm *PCMData) *PCMData {
	result := pcm
	
	// Resample if needed
	if c.opts.SampleRate > 0 && c.opts.SampleRate != pcm.SampleRate {
		result = resample(result, c.opts.SampleRate)
	}
	
	// Convert channels if needed
	if c.opts.Channels > 0 && c.opts.Channels != pcm.Channels {
		result = convertChannels(result, c.opts.Channels)
	}
	
	// Normalize if requested
	if c.opts.Normalize {
		result = normalize(result)
	}
	
	// Trim silence if requested
	if c.opts.TrimSilence {
		result = trimSilence(result)
	}
	
	return result
}

// resample changes sample rate (simple linear interpolation)
func resample(pcm *PCMData, targetRate int) *PCMData {
	if pcm.SampleRate == targetRate {
		return pcm
	}
	
	ratio := float64(targetRate) / float64(pcm.SampleRate)
	newLen := int(float64(len(pcm.Samples)) * ratio)
	newSamples := make([]int16, newLen)
	
	for i := 0; i < newLen; i++ {
		srcIdx := float64(i) / ratio
		srcIdxInt := int(srcIdx)
		frac := srcIdx - float64(srcIdxInt)
		
		if srcIdxInt+1 < len(pcm.Samples) {
			// Linear interpolation
			s1 := float64(pcm.Samples[srcIdxInt])
			s2 := float64(pcm.Samples[srcIdxInt+1])
			newSamples[i] = int16(s1 + frac*(s2-s1))
		} else if srcIdxInt < len(pcm.Samples) {
			newSamples[i] = pcm.Samples[srcIdxInt]
		}
	}
	
	return &PCMData{
		Samples:    newSamples,
		SampleRate: targetRate,
		Channels:   pcm.Channels,
		BitDepth:   pcm.BitDepth,
	}
}

// convertChannels converts between mono and stereo
func convertChannels(pcm *PCMData, targetChannels int) *PCMData {
	if pcm.Channels == targetChannels {
		return pcm
	}
	
	var newSamples []int16
	
	if pcm.Channels == 2 && targetChannels == 1 {
		// Stereo to mono: average channels
		newSamples = make([]int16, len(pcm.Samples)/2)
		for i := 0; i < len(newSamples); i++ {
			left := int32(pcm.Samples[i*2])
			right := int32(pcm.Samples[i*2+1])
			newSamples[i] = int16((left + right) / 2)
		}
	} else if pcm.Channels == 1 && targetChannels == 2 {
		// Mono to stereo: duplicate
		newSamples = make([]int16, len(pcm.Samples)*2)
		for i, s := range pcm.Samples {
			newSamples[i*2] = s
			newSamples[i*2+1] = s
		}
	} else {
		// Unsupported conversion, return original
		return pcm
	}
	
	return &PCMData{
		Samples:    newSamples,
		SampleRate: pcm.SampleRate,
		Channels:   targetChannels,
		BitDepth:   pcm.BitDepth,
	}
}

// normalize normalizes audio levels
func normalize(pcm *PCMData) *PCMData {
	if len(pcm.Samples) == 0 {
		return pcm
	}
	
	// Find peak
	var peak int16 = 0
	for _, s := range pcm.Samples {
		if s < 0 {
			s = -s
		}
		if s > peak {
			peak = s
		}
	}
	
	if peak == 0 {
		return pcm
	}
	
	// Calculate gain (target: 90% of max)
	gain := float64(29490) / float64(peak) // 32767 * 0.9
	
	// Apply gain
	newSamples := make([]int16, len(pcm.Samples))
	for i, s := range pcm.Samples {
		newVal := float64(s) * gain
		if newVal > 32767 {
			newVal = 32767
		} else if newVal < -32768 {
			newVal = -32768
		}
		newSamples[i] = int16(newVal)
	}
	
	return &PCMData{
		Samples:    newSamples,
		SampleRate: pcm.SampleRate,
		Channels:   pcm.Channels,
		BitDepth:   pcm.BitDepth,
	}
}

// trimSilence removes silence from start and end
func trimSilence(pcm *PCMData) *PCMData {
	if len(pcm.Samples) == 0 {
		return pcm
	}
	
	threshold := int16(500) // Silence threshold
	samplesPerFrame := pcm.Channels
	
	// Find start (first non-silent frame)
	start := 0
	for i := 0; i < len(pcm.Samples); i += samplesPerFrame {
		silent := true
		for ch := 0; ch < samplesPerFrame && i+ch < len(pcm.Samples); ch++ {
			s := pcm.Samples[i+ch]
			if s < 0 {
				s = -s
			}
			if s > threshold {
				silent = false
				break
			}
		}
		if !silent {
			start = i
			break
		}
	}
	
	// Find end (last non-silent frame)
	end := len(pcm.Samples)
	for i := len(pcm.Samples) - samplesPerFrame; i >= 0; i -= samplesPerFrame {
		silent := true
		for ch := 0; ch < samplesPerFrame && i+ch < len(pcm.Samples); ch++ {
			s := pcm.Samples[i+ch]
			if s < 0 {
				s = -s
			}
			if s > threshold {
				silent = false
				break
			}
		}
		if !silent {
			end = i + samplesPerFrame
			break
		}
	}
	
	if start >= end {
		return pcm
	}
	
	return &PCMData{
		Samples:    pcm.Samples[start:end],
		SampleRate: pcm.SampleRate,
		Channels:   pcm.Channels,
		BitDepth:   pcm.BitDepth,
	}
}
