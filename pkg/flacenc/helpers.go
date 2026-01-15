package flacenc

import (
	"io"
)

// EncodeFromInt16 is a convenience function to encode int16 samples
func EncodeFromInt16(w io.Writer, samples []int16, sampleRate, channels int) error {
	enc := NewEncoder(sampleRate, channels, 16)

	// Convert int16 to int32
	samples32 := make([]int32, len(samples))
	for i, s := range samples {
		samples32[i] = int32(s)
	}

	return enc.Encode(w, samples32)
}

// EncodeFromInt32 encodes int32 samples (for 24-bit audio stored in int32)
func EncodeFromInt32(w io.Writer, samples []int32, sampleRate, channels, bitsPerSample int) error {
	enc := NewEncoder(sampleRate, channels, bitsPerSample)
	return enc.Encode(w, samples)
}
