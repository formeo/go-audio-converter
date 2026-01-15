package converter

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected Format
	}{
		{"test.wav", FormatWAV},
		{"test.WAV", FormatWAV},
		{"test.wave", FormatWAV},
		{"path/to/file.wav", FormatWAV},
		{"test.mp3", FormatMP3},
		{"test.MP3", FormatMP3},
		{"test.flac", FormatFLAC},
		{"test.FLAC", FormatFLAC},
		{"test.ogg", FormatOGG},
		{"test.oga", FormatOGG},
		{"test.ogv", FormatOGG},
		{"test.txt", FormatUnknown},
		{"test.aac", FormatUnknown},
		{"test", FormatUnknown},
		{"", FormatUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := DetectFormat(tt.path)
			if result != tt.expected {
				t.Errorf("DetectFormat(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.Bitrate != 192 {
		t.Errorf("Bitrate = %d, want 192", c.Bitrate)
	}
	if c.OGGQuality != 0.4 {
		t.Errorf("OGGQuality = %f, want 0.4", c.OGGQuality)
	}
}

// generateTestWAV creates a valid WAV file with a sine wave
func generateTestWAV(sampleRate, channels, durationMs int) []byte {
	numSamples := sampleRate * durationMs / 1000 * channels
	samples := make([]int16, numSamples)

	// Generate simple sine wave (440 Hz)
	for i := 0; i < numSamples/channels; i++ {
		// Simple approximation of sine
		phase := float64(i) / float64(sampleRate) * 440 * 6.28318
		val := int16(16000 * sinApprox(phase))
		for ch := 0; ch < channels; ch++ {
			samples[i*channels+ch] = val
		}
	}

	var buf bytes.Buffer

	dataSize := numSamples * 2
	fileSize := 36 + dataSize
	byteRate := sampleRate * channels * 2
	blockAlign := channels * 2

	buf.Write([]byte("RIFF"))
	binary.Write(&buf, binary.LittleEndian, uint32(fileSize))
	buf.Write([]byte("WAVE"))

	buf.Write([]byte("fmt "))
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(channels))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(&buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(&buf, binary.LittleEndian, uint16(16))

	buf.Write([]byte("data"))
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))

	for _, s := range samples {
		binary.Write(&buf, binary.LittleEndian, s)
	}

	return buf.Bytes()
}

// Simple sine approximation using Taylor series
func sinApprox(x float64) float64 {
	// Normalize to [-π, π]
	for x > 3.14159 {
		x -= 6.28318
	}
	for x < -3.14159 {
		x += 6.28318
	}
	// Taylor series: sin(x) ≈ x - x³/6 + x⁵/120
	x3 := x * x * x
	x5 := x3 * x * x
	return x - x3/6 + x5/120
}

func TestDecodeWAV(t *testing.T) {
	wavData := generateTestWAV(44100, 2, 100) // 100ms stereo
	reader := bytes.NewReader(wavData)

	pcm, err := decodeWAV(reader)
	if err != nil {
		t.Fatalf("decodeWAV() error: %v", err)
	}

	if pcm.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want 44100", pcm.SampleRate)
	}
	if pcm.Channels != 2 {
		t.Errorf("Channels = %d, want 2", pcm.Channels)
	}
	if len(pcm.Samples) == 0 {
		t.Error("Samples is empty")
	}

	expectedSamples := 44100 * 100 / 1000 * 2 // 100ms stereo
	if len(pcm.Samples) != expectedSamples {
		t.Errorf("len(Samples) = %d, want %d", len(pcm.Samples), expectedSamples)
	}
}

func TestDecodeWAV_Mono(t *testing.T) {
	wavData := generateTestWAV(22050, 1, 50) // 50ms mono
	reader := bytes.NewReader(wavData)

	pcm, err := decodeWAV(reader)
	if err != nil {
		t.Fatalf("decodeWAV() error: %v", err)
	}

	if pcm.SampleRate != 22050 {
		t.Errorf("SampleRate = %d, want 22050", pcm.SampleRate)
	}
	if pcm.Channels != 1 {
		t.Errorf("Channels = %d, want 1", pcm.Channels)
	}
}

func TestDecodeWAV_Invalid(t *testing.T) {
	invalidData := []byte("not a wav file")
	reader := bytes.NewReader(invalidData)

	_, err := decodeWAV(reader)
	if err == nil {
		t.Error("decodeWAV() should return error for invalid data")
	}
}

func TestEncodeWAV(t *testing.T) {
	pcm := &PCMData{
		Samples:    []int16{0, 1000, 2000, 3000, -1000, -2000},
		SampleRate: 44100,
		Channels:   2,
	}

	var buf bytes.Buffer
	err := encodeWAV(&buf, pcm)
	if err != nil {
		t.Fatalf("encodeWAV() error: %v", err)
	}

	// Verify WAV header
	data := buf.Bytes()
	if string(data[0:4]) != "RIFF" {
		t.Error("Missing RIFF header")
	}
	if string(data[8:12]) != "WAVE" {
		t.Error("Missing WAVE marker")
	}
	if string(data[12:16]) != "fmt " {
		t.Error("Missing fmt chunk")
	}
}

func TestWAVRoundtrip(t *testing.T) {
	original := &PCMData{
		Samples:    make([]int16, 4410), // 50ms at 44100Hz stereo
		SampleRate: 44100,
		Channels:   2,
	}

	// Fill with test pattern
	for i := range original.Samples {
		original.Samples[i] = int16(i % 32767)
	}

	// Encode
	var buf bytes.Buffer
	if err := encodeWAV(&buf, original); err != nil {
		t.Fatalf("encodeWAV() error: %v", err)
	}

	// Decode
	decoded, err := decodeWAV(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decodeWAV() error: %v", err)
	}

	// Verify
	if decoded.SampleRate != original.SampleRate {
		t.Errorf("SampleRate mismatch: %d vs %d", decoded.SampleRate, original.SampleRate)
	}
	if decoded.Channels != original.Channels {
		t.Errorf("Channels mismatch: %d vs %d", decoded.Channels, original.Channels)
	}
	if len(decoded.Samples) != len(original.Samples) {
		t.Errorf("Samples length mismatch: %d vs %d", len(decoded.Samples), len(original.Samples))
	}

	// Check sample values
	for i := 0; i < min(len(original.Samples), len(decoded.Samples)); i++ {
		if original.Samples[i] != decoded.Samples[i] {
			t.Errorf("Sample[%d] mismatch: %d vs %d", i, original.Samples[i], decoded.Samples[i])
			break
		}
	}
}

func TestEncodeWAV_EmptySamples(t *testing.T) {
	pcm := &PCMData{
		Samples:    []int16{},
		SampleRate: 44100,
		Channels:   2,
	}

	var buf bytes.Buffer
	err := encodeWAV(&buf, pcm)
	if err != nil {
		t.Fatalf("encodeWAV() should handle empty samples: %v", err)
	}
}

func TestEncodeMP3_EmptySamples(t *testing.T) {
	pcm := &PCMData{
		Samples:    []int16{},
		SampleRate: 44100,
		Channels:   2,
	}

	var buf bytes.Buffer
	err := encodeMP3(&buf, pcm)
	if err == nil {
		t.Error("encodeMP3() should return error for empty samples")
	}
}

func TestEncodeOGG_NotImplemented(t *testing.T) {
	c := New()
	pcm := &PCMData{
		Samples:    []int16{1, 2, 3, 4},
		SampleRate: 44100,
		Channels:   2,
	}

	var buf bytes.Buffer
	err := c.encodeOGG(&buf, pcm)
	if err == nil {
		t.Error("encodeOGG() should return not implemented error")
	}
}

func TestEncodeFLAC_NotImplemented(t *testing.T) {
	pcm := &PCMData{
		Samples:    []int16{1, 2, 3},
		SampleRate: 44100,
		Channels:   2,
	}

	var buf bytes.Buffer
	err := encodeFLAC(&buf, pcm)
	if err == nil {
		t.Error("encodeFLAC() should return not implemented error")
	}
}

func TestConvertFile_UnsupportedFormat(t *testing.T) {
	c := New()

	err := c.ConvertFile("input.txt", "output.wav")
	if err == nil {
		t.Error("ConvertFile() should return error for unsupported input format")
	}

	err = c.ConvertFile("input.wav", "output.txt")
	if err == nil {
		t.Error("ConvertFile() should return error for unsupported output format")
	}
}

func TestConvertFile_FileNotFound(t *testing.T) {
	c := New()

	err := c.ConvertFile("nonexistent.wav", "output.mp3")
	if err == nil {
		t.Error("ConvertFile() should return error for non-existent file")
	}
}

func TestConvertFile_WAVtoWAV(t *testing.T) {
	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.wav")
	outputPath := filepath.Join(tmpDir, "output.wav")

	// Create test WAV
	wavData := generateTestWAV(44100, 2, 100)
	if err := os.WriteFile(inputPath, wavData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	c := New()
	err := c.ConvertFile(inputPath, outputPath)
	if err != nil {
		t.Fatalf("ConvertFile() error: %v", err)
	}

	// Verify output exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output file was not created")
	}

	// Verify output is valid WAV
	outputData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}
	if string(outputData[0:4]) != "RIFF" {
		t.Error("Output is not a valid WAV file")
	}
}

func TestPCMData(t *testing.T) {
	pcm := &PCMData{
		Samples:    []int16{1, 2, 3, 4},
		SampleRate: 48000,
		Channels:   2,
	}

	if len(pcm.Samples) != 4 {
		t.Errorf("Samples length = %d, want 4", len(pcm.Samples))
	}
	if pcm.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", pcm.SampleRate)
	}
	if pcm.Channels != 2 {
		t.Errorf("Channels = %d, want 2", pcm.Channels)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Benchmarks
func BenchmarkDecodeWAV(b *testing.B) {
	wavData := generateTestWAV(44100, 2, 1000) // 1 second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(wavData)
		_, err := decodeWAV(reader)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeWAV(b *testing.B) {
	pcm := &PCMData{
		Samples:    make([]int16, 44100*2), // 1 second stereo
		SampleRate: 44100,
		Channels:   2,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := encodeWAV(&buf, pcm); err != nil {
			b.Fatal(err)
		}
	}
}
