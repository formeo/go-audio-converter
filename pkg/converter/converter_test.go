package converter

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// createTestWAV creates a simple WAV file for testing
func createTestWAV(t *testing.T, duration float64, sampleRate, channels int) []byte {
	t.Helper()
	
	numSamples := int(duration * float64(sampleRate) * float64(channels))
	samples := make([]int16, numSamples)
	
	// Generate simple sine wave
	for i := 0; i < numSamples; i++ {
		// 440Hz sine wave
		phase := float64(i) / float64(sampleRate) * 440.0 * 2 * 3.14159
		samples[i] = int16(16000 * sin(phase))
	}
	
	// Create WAV
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

// Simple sine function
func sin(x float64) float64 {
	// Taylor series approximation
	x = x - float64(int(x/(2*3.14159)))*2*3.14159
	if x > 3.14159 {
		x -= 2 * 3.14159
	}
	result := x
	term := x
	for i := 1; i < 10; i++ {
		term *= -x * x / float64((2*i)*(2*i+1))
		result += term
	}
	return result
}

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	
	opts := c.Options()
	if opts.Bitrate != 192 {
		t.Errorf("Default bitrate = %d, want 192", opts.Bitrate)
	}
}

func TestNewWithOptions(t *testing.T) {
	opts := Options{
		Bitrate:    320,
		SampleRate: 48000,
		Channels:   1,
	}
	
	c := NewWithOptions(opts)
	if c == nil {
		t.Fatal("NewWithOptions() returned nil")
	}
	
	got := c.Options()
	if got.Bitrate != 320 {
		t.Errorf("Bitrate = %d, want 320", got.Bitrate)
	}
	if got.SampleRate != 48000 {
		t.Errorf("SampleRate = %d, want 48000", got.SampleRate)
	}
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path string
		want Format
	}{
		{"audio.wav", FormatWAV},
		{"audio.WAVE", FormatWAV},
		{"audio.mp3", FormatMP3},
		{"audio.MP3", FormatMP3},
		{"audio.flac", FormatFLAC},
		{"audio.ogg", FormatOGG},
		{"audio.oga", FormatOGG},
		{"audio.txt", FormatUnknown},
		{"audio", FormatUnknown},
		{"/path/to/audio.wav", FormatWAV},
	}
	
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DetectFormat(tt.path)
			if got != tt.want {
				t.Errorf("DetectFormat(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectFormatFromMIME(t *testing.T) {
	tests := []struct {
		mime string
		want Format
	}{
		{"audio/wav", FormatWAV},
		{"audio/wave", FormatWAV},
		{"audio/x-wav", FormatWAV},
		{"audio/mpeg", FormatMP3},
		{"audio/mp3", FormatMP3},
		{"audio/flac", FormatFLAC},
		{"audio/ogg", FormatOGG},
		{"text/plain", FormatUnknown},
	}
	
	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			got := DetectFormatFromMIME(tt.mime)
			if got != tt.want {
				t.Errorf("DetectFormatFromMIME(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}

func TestCanConvert(t *testing.T) {
	tests := []struct {
		from, to Format
		want     bool
	}{
		{FormatWAV, FormatMP3, true},
		{FormatFLAC, FormatMP3, true},
		{FormatOGG, FormatMP3, true},
		{FormatMP3, FormatWAV, true},
		{FormatWAV, FormatWAV, true},
		{FormatMP3, FormatMP3, false}, // No same-format via this method
		{FormatMP3, FormatFLAC, false},
		{FormatWAV, FormatOGG, false},
	}
	
	for _, tt := range tests {
		name := string(tt.from) + "->" + string(tt.to)
		t.Run(name, func(t *testing.T) {
			got := CanConvert(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("CanConvert(%v, %v) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestSupportedFormats(t *testing.T) {
	inputs := SupportedInputFormats()
	if len(inputs) < 4 {
		t.Errorf("SupportedInputFormats() returned %d formats, want at least 4", len(inputs))
	}
	
	outputs := SupportedOutputFormats()
	if len(outputs) < 2 {
		t.Errorf("SupportedOutputFormats() returned %d formats, want at least 2", len(outputs))
	}
}

func TestConvert_WAVToMP3(t *testing.T) {
	// Create test WAV
	wavData := createTestWAV(t, 0.5, 44100, 2)
	
	c := New()
	var out bytes.Buffer
	
	err := c.Convert(bytes.NewReader(wavData), &out, FormatWAV, FormatMP3)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	
	// Check output is valid MP3 (starts with ID3 or frame sync)
	result := out.Bytes()
	if len(result) == 0 {
		t.Fatal("Output is empty")
	}
	
	// MP3 should start with frame sync (0xFF 0xFB) or ID3 tag
	isMP3 := (result[0] == 0xFF && (result[1]&0xE0) == 0xE0) ||
		(result[0] == 'I' && result[1] == 'D' && result[2] == '3')
	if !isMP3 {
		t.Errorf("Output doesn't look like MP3: starts with %X %X", result[0], result[1])
	}
}

func TestConvertFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	
	// Create test WAV file
	wavData := createTestWAV(t, 0.5, 44100, 2)
	inputPath := filepath.Join(tmpDir, "test.wav")
	outputPath := filepath.Join(tmpDir, "test.mp3")
	
	if err := os.WriteFile(inputPath, wavData, 0644); err != nil {
		t.Fatal(err)
	}
	
	c := New()
	if err := c.ConvertFile(inputPath, outputPath); err != nil {
		t.Fatalf("ConvertFile() error = %v", err)
	}
	
	// Check output exists
	stat, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Output file not created: %v", err)
	}
	
	if stat.Size() == 0 {
		t.Error("Output file is empty")
	}
}

func TestConvertDir(t *testing.T) {
	// Create temp directories
	inputDir := t.TempDir()
	outputDir := t.TempDir()
	
	// Create test files
	wavData := createTestWAV(t, 0.5, 44100, 2)
	
	files := []string{"test1.wav", "subdir/test2.wav"}
	for _, f := range files {
		path := filepath.Join(inputDir, f)
		os.MkdirAll(filepath.Dir(path), 0755)
		if err := os.WriteFile(path, wavData, 0644); err != nil {
			t.Fatal(err)
		}
	}
	
	c := New()
	result, err := c.ConvertDir(inputDir, outputDir, FormatMP3)
	if err != nil {
		t.Fatalf("ConvertDir() error = %v", err)
	}
	
	if len(result.Converted) != 2 {
		t.Errorf("Converted %d files, want 2", len(result.Converted))
	}
	
	if len(result.Failed) != 0 {
		t.Errorf("Failed %d files, want 0", len(result.Failed))
	}
}

func TestNormalize(t *testing.T) {
	pcm := &PCMData{
		Samples:    []int16{1000, -1000, 2000, -2000},
		SampleRate: 44100,
		Channels:   1,
		BitDepth:   16,
	}
	
	result := normalize(pcm)
	
	// Check peak is around 90% of max
	var peak int16
	for _, s := range result.Samples {
		if s < 0 {
			s = -s
		}
		if s > peak {
			peak = s
		}
	}
	
	// Should be close to 29490 (90% of 32767)
	if peak < 25000 || peak > 32000 {
		t.Errorf("Normalized peak = %d, want ~29490", peak)
	}
}

func TestConvertChannels(t *testing.T) {
	// Stereo to mono
	stereo := &PCMData{
		Samples:    []int16{100, 200, 300, 400},
		SampleRate: 44100,
		Channels:   2,
	}
	
	mono := convertChannels(stereo, 1)
	if mono.Channels != 1 {
		t.Errorf("Channels = %d, want 1", mono.Channels)
	}
	if len(mono.Samples) != 2 {
		t.Errorf("Sample count = %d, want 2", len(mono.Samples))
	}
	
	// Mono to stereo
	monoIn := &PCMData{
		Samples:    []int16{100, 200},
		SampleRate: 44100,
		Channels:   1,
	}
	
	stereoOut := convertChannels(monoIn, 2)
	if stereoOut.Channels != 2 {
		t.Errorf("Channels = %d, want 2", stereoOut.Channels)
	}
	if len(stereoOut.Samples) != 4 {
		t.Errorf("Sample count = %d, want 4", len(stereoOut.Samples))
	}
}

func BenchmarkWAVToMP3(b *testing.B) {
	// Create 1 second WAV
	numSamples := 44100 * 2
	samples := make([]int16, numSamples)
	for i := range samples {
		samples[i] = int16(i % 32767)
	}
	
	var buf bytes.Buffer
	pcm := &PCMData{
		Samples:    samples,
		SampleRate: 44100,
		Channels:   2,
		BitDepth:   16,
	}
	encodeToWAV(&buf, pcm)
	wavData := buf.Bytes()
	
	c := New()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		c.Convert(bytes.NewReader(wavData), &out, FormatWAV, FormatMP3)
	}
}
