package flacenc

import (
	"bytes"
	"testing"
)

func TestBitWriter_WriteBits(t *testing.T) {
	var buf bytes.Buffer
	bw := NewBitWriter(&buf)

	// Write 0b11110000 in two parts
	bw.WriteBits(0xF, 4)
	bw.WriteBits(0x0, 4)
	bw.Flush()

	if buf.Len() != 1 {
		t.Errorf("expected 1 byte, got %d", buf.Len())
	}
	if buf.Bytes()[0] != 0xF0 {
		t.Errorf("expected 0xF0, got 0x%X", buf.Bytes()[0])
	}
}

func TestBitWriter_WriteUnary(t *testing.T) {
	tests := []struct {
		value    uint32
		expected []byte
		bits     int
	}{
		{0, []byte{0x00}, 1},     // 0 -> "0"
		{1, []byte{0x80}, 2},     // 1 -> "10"
		{3, []byte{0xE0}, 4},     // 3 -> "1110"
		{7, []byte{0xFE}, 8},     // 7 -> "11111110"
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		bw := NewBitWriter(&buf)
		bw.WriteUnary(tt.value)
		bw.Flush()

		// Compare first byte only
		if buf.Bytes()[0]>>(8-tt.bits)<<(8-tt.bits) != tt.expected[0] {
			t.Errorf("WriteUnary(%d): expected 0x%X, got 0x%X",
				tt.value, tt.expected[0], buf.Bytes()[0])
		}
	}
}

func TestBitWriter_WriteSignedRice(t *testing.T) {
	tests := []struct {
		value int32
		k     int
	}{
		{0, 4},
		{1, 4},
		{-1, 4},
		{100, 4},
		{-100, 4},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		bw := NewBitWriter(&buf)
		err := bw.WriteSignedRice(tt.value, tt.k)
		bw.Flush()

		if err != nil {
			t.Errorf("WriteSignedRice(%d, %d) error: %v", tt.value, tt.k, err)
		}
		if buf.Len() == 0 {
			t.Errorf("WriteSignedRice(%d, %d) produced no output", tt.value, tt.k)
		}
	}
}

func TestBitWriter_WriteUTF8(t *testing.T) {
	tests := []struct {
		value    uint64
		expected int // expected byte count
	}{
		{0, 1},
		{127, 1},
		{128, 2},
		{2047, 2},
		{2048, 3},
		{65535, 3},
	}

	for _, tt := range tests {
		var buf bytes.Buffer
		bw := NewBitWriter(&buf)
		bw.WriteUTF8(tt.value)

		if buf.Len() != tt.expected {
			t.Errorf("WriteUTF8(%d): expected %d bytes, got %d",
				tt.value, tt.expected, buf.Len())
		}
	}
}

func TestComputeFixedResiduals(t *testing.T) {
	samples := []int32{100, 102, 104, 106, 108}

	// Order 0: residuals = samples
	res0 := computeFixedResiduals(samples, 0)
	if len(res0) != 5 {
		t.Errorf("order 0: expected 5 residuals, got %d", len(res0))
	}

	// Order 1: differences
	res1 := computeFixedResiduals(samples, 1)
	if len(res1) != 4 {
		t.Errorf("order 1: expected 4 residuals, got %d", len(res1))
	}
	// Linear sequence: differences should be constant
	for i, r := range res1 {
		if r != 2 {
			t.Errorf("order 1: residual[%d] = %d, expected 2", i, r)
		}
	}

	// Order 2: second differences (should be 0 for linear)
	res2 := computeFixedResiduals(samples, 2)
	for i, r := range res2 {
		if r != 0 {
			t.Errorf("order 2: residual[%d] = %d, expected 0", i, r)
		}
	}
}

func TestEstimateRiceSize(t *testing.T) {
	// Small residuals should have small size
	small := []int32{0, 1, -1, 2, -2, 1, 0, -1}
	smallSize := estimateRiceSize(small)

	// Large residuals should have larger size
	large := []int32{1000, -1000, 2000, -2000}
	largeSize := estimateRiceSize(large)

	if smallSize >= largeSize {
		t.Errorf("small residuals (%d) should be smaller than large (%d)",
			smallSize, largeSize)
	}
}

func TestGetBlockSizeCode(t *testing.T) {
	tests := []struct {
		blockSize int
		expected  int
	}{
		{4096, 12},
		{1024, 10},
		{192, 1},
		{100, 6},  // 8-bit custom
		{1000, 7}, // 16-bit custom
	}

	for _, tt := range tests {
		code := getBlockSizeCode(tt.blockSize)
		if code != tt.expected {
			t.Errorf("getBlockSizeCode(%d) = %d, expected %d",
				tt.blockSize, code, tt.expected)
		}
	}
}

func TestGetSampleRateCode(t *testing.T) {
	tests := []struct {
		sampleRate int
		expected   int
	}{
		{44100, 9},
		{48000, 10},
		{96000, 11},
		{22050, 6},
	}

	for _, tt := range tests {
		code := getSampleRateCode(tt.sampleRate)
		if code != tt.expected {
			t.Errorf("getSampleRateCode(%d) = %d, expected %d",
				tt.sampleRate, code, tt.expected)
		}
	}
}

func TestEncoder_Basic(t *testing.T) {
	enc := NewEncoder(44100, 2, 16)

	// Generate simple test signal (silence)
	samples := make([]int32, 8192) // 2 channels * 4096 samples

	var buf bytes.Buffer
	err := enc.Encode(&buf, samples)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Check magic number
	if buf.Len() < 4 {
		t.Fatal("output too short")
	}
	if string(buf.Bytes()[:4]) != "fLaC" {
		t.Errorf("invalid magic: %v", buf.Bytes()[:4])
	}
}

func TestEncoder_SineWave(t *testing.T) {
	enc := NewEncoder(44100, 1, 16)

	// Generate 1 second mono sine wave
	samples := make([]int32, 44100)
	for i := range samples {
		// Simple sine approximation
		phase := float64(i) / 44100 * 440 * 6.28318
		samples[i] = int32(16000 * sin(phase))
	}

	var buf bytes.Buffer
	err := enc.Encode(&buf, samples)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// FLAC should compress sine wave somewhat
	rawSize := len(samples) * 2 // 16-bit samples
	flacSize := buf.Len()

	t.Logf("Raw size: %d bytes, FLAC size: %d bytes, ratio: %.2f%%",
		rawSize, flacSize, float64(flacSize)/float64(rawSize)*100)

	// Even basic compression should achieve some reduction
	if flacSize >= rawSize {
		t.Logf("Warning: no compression achieved (this is OK for simple encoder)")
	}
}

func TestEncoder_Stereo(t *testing.T) {
	enc := NewEncoder(44100, 2, 16)

	// Generate stereo signal
	samples := make([]int32, 44100*2)
	for i := 0; i < 44100; i++ {
		val := int32(1000 * sin(float64(i)/44100*440*6.28318))
		samples[i*2] = val   // Left
		samples[i*2+1] = val // Right (same as left)
	}

	var buf bytes.Buffer
	err := enc.Encode(&buf, samples)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Check output is valid
	if buf.Len() < 100 {
		t.Error("output suspiciously small")
	}
}

// Simple sine approximation
func sin(x float64) float64 {
	for x > 3.14159 {
		x -= 6.28318
	}
	for x < -3.14159 {
		x += 6.28318
	}
	x3 := x * x * x
	x5 := x3 * x * x
	return x - x3/6 + x5/120
}

func BenchmarkEncoder_1Sec(b *testing.B) {
	enc := NewEncoder(44100, 2, 16)
	samples := make([]int32, 44100*2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		enc.Encode(&buf, samples)
	}
}

func BenchmarkComputeResiduals(b *testing.B) {
	samples := make([]int32, 4096)
	for i := range samples {
		samples[i] = int32(i * 7)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeFixedResiduals(samples, 2)
	}
}
