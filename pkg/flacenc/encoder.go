package flacenc

import (
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
)

// Encoder encodes audio to FLAC format
type Encoder struct {
	SampleRate    int
	Channels      int
	BitsPerSample int
	BlockSize     int // samples per block (typically 4096)

	totalSamples uint64
	minBlockSize uint16
	maxBlockSize uint16
	minFrameSize uint32
	maxFrameSize uint32
	md5sum       [16]byte
	md5hash      []byte
}

// NewEncoder creates a new FLAC encoder
func NewEncoder(sampleRate, channels, bitsPerSample int) *Encoder {
	return &Encoder{
		SampleRate:    sampleRate,
		Channels:      channels,
		BitsPerSample: bitsPerSample,
		BlockSize:     4096,
		minBlockSize:  4096,
		maxBlockSize:  4096,
		minFrameSize:  0xFFFFFF,
		maxFrameSize:  0,
	}
}

// Encode encodes PCM samples to FLAC
func (e *Encoder) Encode(w io.Writer, samples []int32) error {
	// We need to write to a buffer first to calculate MD5 and frame sizes
	var buf bytes.Buffer

	// Calculate total samples per channel
	e.totalSamples = uint64(len(samples) / e.Channels)

	// Compute MD5 of raw samples
	md5h := md5.New()
	for _, s := range samples {
		// Write sample as little-endian based on bit depth
		switch e.BitsPerSample {
		case 8:
			md5h.Write([]byte{byte(s)})
		case 16:
			b := make([]byte, 2)
			binary.LittleEndian.PutUint16(b, uint16(int16(s)))
			md5h.Write(b)
		case 24:
			b := make([]byte, 3)
			b[0] = byte(s)
			b[1] = byte(s >> 8)
			b[2] = byte(s >> 16)
			md5h.Write(b)
		case 32:
			b := make([]byte, 4)
			binary.LittleEndian.PutUint32(b, uint32(s))
			md5h.Write(b)
		}
	}
	copy(e.md5sum[:], md5h.Sum(nil))

	// Encode frames
	samplesPerChannel := len(samples) / e.Channels
	frameNum := uint64(0)

	for offset := 0; offset < samplesPerChannel; offset += e.BlockSize {
		blockSize := e.BlockSize
		if offset+blockSize > samplesPerChannel {
			blockSize = samplesPerChannel - offset
		}

		// Extract block samples (deinterleave)
		block := make([][]int32, e.Channels)
		for ch := 0; ch < e.Channels; ch++ {
			block[ch] = make([]int32, blockSize)
			for i := 0; i < blockSize; i++ {
				block[ch][i] = samples[(offset+i)*e.Channels+ch]
			}
		}

		frameSize, err := e.encodeFrame(&buf, block, frameNum)
		if err != nil {
			return fmt.Errorf("encode frame %d: %w", frameNum, err)
		}

		if uint32(frameSize) < e.minFrameSize {
			e.minFrameSize = uint32(frameSize)
		}
		if uint32(frameSize) > e.maxFrameSize {
			e.maxFrameSize = uint32(frameSize)
		}

		if uint16(blockSize) < e.minBlockSize {
			e.minBlockSize = uint16(blockSize)
		}
		if uint16(blockSize) > e.maxBlockSize {
			e.maxBlockSize = uint16(blockSize)
		}

		frameNum++
	}

	// Now write everything with correct header
	// Magic number
	if _, err := w.Write([]byte("fLaC")); err != nil {
		return err
	}

	// STREAMINFO block
	if err := e.writeStreamInfo(w); err != nil {
		return err
	}

	// Copy encoded frames
	if _, err := io.Copy(w, &buf); err != nil {
		return err
	}

	return nil
}

// writeStreamInfo writes the STREAMINFO metadata block
func (e *Encoder) writeStreamInfo(w io.Writer) error {
	// Block header: 1 bit last-metadata-block flag + 7 bits type + 24 bits length
	// Type 0 = STREAMINFO, length = 34 bytes
	// 0x80 = last block flag set, type 0
	header := []byte{0x80, 0x00, 0x00, 0x22} // 0x22 = 34 in decimal
	if _, err := w.Write(header); err != nil {
		return err
	}

	// STREAMINFO data (34 bytes)
	var buf bytes.Buffer

	// Min block size (16 bits)
	binary.Write(&buf, binary.BigEndian, e.minBlockSize)
	// Max block size (16 bits)
	binary.Write(&buf, binary.BigEndian, e.maxBlockSize)
	// Min frame size (24 bits)
	buf.Write([]byte{byte(e.minFrameSize >> 16), byte(e.minFrameSize >> 8), byte(e.minFrameSize)})
	// Max frame size (24 bits)
	buf.Write([]byte{byte(e.maxFrameSize >> 16), byte(e.maxFrameSize >> 8), byte(e.maxFrameSize)})

	// Sample rate (20 bits) + channels-1 (3 bits) + bits per sample-1 (5 bits) + total samples (36 bits)
	// = 64 bits = 8 bytes
	sr := uint64(e.SampleRate)
	ch := uint64(e.Channels - 1)
	bps := uint64(e.BitsPerSample - 1)
	ts := e.totalSamples

	// Pack: SSSSSSSS SSSSSSSS SSSSCCCC CBBBBBTT TTTTTTTT TTTTTTTT TTTTTTTT TTTTTTTT
	packed := (sr << 44) | (ch << 41) | (bps << 36) | ts
	for i := 7; i >= 0; i-- {
		buf.WriteByte(byte(packed >> (i * 8)))
	}

	// MD5 signature (16 bytes)
	buf.Write(e.md5sum[:])

	_, err := w.Write(buf.Bytes())
	return err
}

// encodeFrame encodes a single frame
func (e *Encoder) encodeFrame(w io.Writer, block [][]int32, frameNum uint64) (int, error) {
	var buf bytes.Buffer
	bw := NewBitWriter(&buf)

	blockSize := len(block[0])

	// Frame header
	bw.ResetCRC8()
	bw.ResetCRC16()

	// Sync code (14 bits: 0x3FFE)
	bw.WriteBits(0x3FFE, 14)
	// Reserved (1 bit)
	bw.WriteBits(0, 1)
	// Blocking strategy (1 bit): 0 = fixed block size
	bw.WriteBits(0, 1)

	// Block size (4 bits) - encode block size
	blockSizeCode := getBlockSizeCode(blockSize)
	bw.WriteBits(uint64(blockSizeCode), 4)

	// Sample rate (4 bits)
	sampleRateCode := getSampleRateCode(e.SampleRate)
	bw.WriteBits(uint64(sampleRateCode), 4)

	// Channel assignment (4 bits)
	channelCode := uint64(e.Channels - 1)
	bw.WriteBits(channelCode, 4)

	// Sample size (3 bits)
	sampleSizeCode := getSampleSizeCode(e.BitsPerSample)
	bw.WriteBits(uint64(sampleSizeCode), 3)

	// Reserved (1 bit)
	bw.WriteBits(0, 1)

	// Frame/sample number (UTF-8 coded)
	bw.WriteUTF8(frameNum)

	// Block size (if code requires it)
	if blockSizeCode == 6 {
		bw.WriteBits(uint64(blockSize-1), 8)
	} else if blockSizeCode == 7 {
		bw.WriteBits(uint64(blockSize-1), 16)
	}

	// Sample rate (if code requires it)
	if sampleRateCode == 12 {
		bw.WriteBits(uint64(e.SampleRate/1000), 8)
	} else if sampleRateCode == 13 {
		bw.WriteBits(uint64(e.SampleRate), 16)
	} else if sampleRateCode == 14 {
		bw.WriteBits(uint64(e.SampleRate/10), 16)
	}

	// CRC-8 of frame header
	bw.Flush()
	crc8 := bw.GetCRC8()
	bw.WriteByte(crc8)

	// Subframes (one per channel)
	for ch := 0; ch < e.Channels; ch++ {
		if err := e.encodeSubframe(bw, block[ch]); err != nil {
			return 0, err
		}
	}

	// Byte-align
	bw.Flush()

	// CRC-16 of entire frame
	crc16 := bw.GetCRC16()
	bw.WriteUint16BE(crc16)

	// Write to output
	n, err := w.Write(buf.Bytes())
	return n, err
}

// encodeSubframe encodes a single subframe using FIXED prediction
func (e *Encoder) encodeSubframe(bw *BitWriter, samples []int32) error {
	// Try different prediction orders and pick the best
	bestOrder := 0
	bestSize := int64(1<<63 - 1)

	for order := 0; order <= 4; order++ {
		residuals := computeFixedResiduals(samples, order)
		size := estimateRiceSize(residuals)
		if size < bestSize {
			bestSize = size
			bestOrder = order
		}
	}

	// Also try verbatim
	verbatimSize := int64(len(samples) * e.BitsPerSample)
	if verbatimSize < bestSize {
		return e.encodeVerbatimSubframe(bw, samples)
	}

	return e.encodeFixedSubframe(bw, samples, bestOrder)
}

// encodeVerbatimSubframe encodes without compression
func (e *Encoder) encodeVerbatimSubframe(bw *BitWriter, samples []int32) error {
	// Subframe header
	// 1 bit zero padding
	bw.WriteBits(0, 1)
	// 6 bits subframe type: 1 = VERBATIM
	bw.WriteBits(1, 6)
	// 1 bit wasted bits flag
	bw.WriteBits(0, 1)

	// Raw samples
	for _, s := range samples {
		bw.WriteBits(uint64(uint32(s)), e.BitsPerSample)
	}

	return nil
}

// encodeFixedSubframe encodes using FIXED prediction
func (e *Encoder) encodeFixedSubframe(bw *BitWriter, samples []int32, order int) error {
	// Subframe header
	// 1 bit zero padding
	bw.WriteBits(0, 1)
	// 6 bits subframe type: 001xxx = FIXED, xxx = order
	bw.WriteBits(uint64(0x08|order), 6)
	// 1 bit wasted bits flag
	bw.WriteBits(0, 1)

	// Warmup samples (unencoded)
	for i := 0; i < order; i++ {
		bw.WriteBits(uint64(uint32(samples[i])), e.BitsPerSample)
	}

	// Compute residuals
	residuals := computeFixedResiduals(samples, order)

	// Encode residuals with Rice coding
	return encodeRicePartition(bw, residuals)
}

// computeFixedResiduals computes residuals for FIXED prediction
func computeFixedResiduals(samples []int32, order int) []int32 {
	n := len(samples)
	residuals := make([]int32, n-order)

	switch order {
	case 0:
		// No prediction: residual = sample
		for i := order; i < n; i++ {
			residuals[i-order] = samples[i]
		}
	case 1:
		// residual = sample[i] - sample[i-1]
		for i := order; i < n; i++ {
			residuals[i-order] = samples[i] - samples[i-1]
		}
	case 2:
		// residual = sample[i] - 2*sample[i-1] + sample[i-2]
		for i := order; i < n; i++ {
			residuals[i-order] = samples[i] - 2*samples[i-1] + samples[i-2]
		}
	case 3:
		// residual = sample[i] - 3*sample[i-1] + 3*sample[i-2] - sample[i-3]
		for i := order; i < n; i++ {
			residuals[i-order] = samples[i] - 3*samples[i-1] + 3*samples[i-2] - samples[i-3]
		}
	case 4:
		// residual = sample[i] - 4*sample[i-1] + 6*sample[i-2] - 4*sample[i-3] + sample[i-4]
		for i := order; i < n; i++ {
			residuals[i-order] = samples[i] - 4*samples[i-1] + 6*samples[i-2] - 4*samples[i-3] + samples[i-4]
		}
	}

	return residuals
}

// estimateRiceSize estimates bits needed for Rice coding
func estimateRiceSize(residuals []int32) int64 {
	if len(residuals) == 0 {
		return 0
	}

	// Find optimal k parameter
	var sum int64
	for _, r := range residuals {
		if r >= 0 {
			sum += int64(r)
		} else {
			sum += int64(-r - 1)
		}
	}

	// Estimate k from average magnitude
	avg := float64(sum) / float64(len(residuals))
	k := 0
	for (1 << k) < int(avg) {
		k++
	}
	if k > 14 {
		k = 14
	}

	// Estimate total bits
	var bits int64
	for _, r := range residuals {
		var uval uint32
		if r >= 0 {
			uval = uint32(r) << 1
		} else {
			uval = (uint32(-r-1) << 1) | 1
		}
		q := uval >> k
		bits += int64(q) + 1 + int64(k)
	}

	return bits
}

// encodeRicePartition encodes residuals using Rice coding
func encodeRicePartition(bw *BitWriter, residuals []int32) error {
	if len(residuals) == 0 {
		// Write minimal partition header
		bw.WriteBits(0, 2) // encoding method
		bw.WriteBits(0, 4) // partition order
		return nil
	}

	// Calculate optimal Rice parameter
	var sum int64
	for _, r := range residuals {
		if r >= 0 {
			sum += int64(r)
		} else {
			sum += int64(-r - 1)
		}
	}

	avg := float64(sum) / float64(len(residuals))
	k := 0
	for (1 << k) < int(avg) {
		k++
	}
	if k > 14 {
		k = 14
	}

	// Residual coding method (2 bits): 0 = Rice coding with 4-bit parameter
	bw.WriteBits(0, 2)
	// Partition order (4 bits): 0 = single partition
	bw.WriteBits(0, 4)
	// Rice parameter (4 bits for method 0, 5 bits for method 1)
	bw.WriteBits(uint64(k), 4)

	// Encode each residual
	for _, r := range residuals {
		if err := bw.WriteSignedRice(r, k); err != nil {
			return err
		}
	}

	return nil
}

// Helper functions for encoding
func getBlockSizeCode(blockSize int) int {
	switch blockSize {
	case 192:
		return 1
	case 576:
		return 2
	case 1152:
		return 3
	case 2304:
		return 4
	case 4608:
		return 5
	case 256:
		return 8
	case 512:
		return 9
	case 1024:
		return 10
	case 2048:
		return 11
	case 4096:
		return 12
	case 8192:
		return 13
	case 16384:
		return 14
	case 32768:
		return 15
	default:
		if blockSize <= 256 {
			return 6 // 8-bit block size - 1
		}
		return 7 // 16-bit block size - 1
	}
}

func getSampleRateCode(sampleRate int) int {
	switch sampleRate {
	case 88200:
		return 1
	case 176400:
		return 2
	case 192000:
		return 3
	case 8000:
		return 4
	case 16000:
		return 5
	case 22050:
		return 6
	case 24000:
		return 7
	case 32000:
		return 8
	case 44100:
		return 9
	case 48000:
		return 10
	case 96000:
		return 11
	default:
		if sampleRate%1000 == 0 && sampleRate/1000 <= 255 {
			return 12 // 8-bit kHz
		} else if sampleRate <= 65535 {
			return 13 // 16-bit Hz
		}
		return 14 // 16-bit tens of Hz
	}
}

func getSampleSizeCode(bitsPerSample int) int {
	switch bitsPerSample {
	case 8:
		return 1
	case 12:
		return 2
	case 16:
		return 4
	case 20:
		return 5
	case 24:
		return 6
	case 32:
		return 7 // Non-standard but some decoders support it
	default:
		return 0 // Get from STREAMINFO
	}
}
