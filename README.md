# go-audio-converter

[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/formeo/go-audio-converter)](https://goreportcard.com/report/github.com/formeo/go-audio-converter)

🎵 **Pure Go audio converter — no ffmpeg, no CGO, no dependencies.**

Convert audio files between WAV, FLAC, OGG, and MP3 formats using a single static binary that works everywhere.

## Why?

- **No ffmpeg** — no installation, no PATH issues, no version conflicts
- **No CGO** — cross-compile for any platform with `go build`
- **Single binary** — download, run, done
- **Tiny Docker images** — ~15MB vs 100MB+ with ffmpeg
- **WASM ready** — runs in the browser
- **Embeddable** — use as a library in your Go projects

## Installation

### Binary

```bash
# Go install
go install github.com/formeo/go-audio-converter/cmd/audioconv@latest

# Or download from releases
curl -LO https://github.com/formeo/go-audio-converter/releases/latest/download/audioconv-linux-amd64.tar.gz
tar -xzf audioconv-linux-amd64.tar.gz
sudo mv audioconv /usr/local/bin/
```

### Docker

```bash
docker pull formeo/audioconv:latest
```

### From source

```bash
git clone https://github.com/formeo/go-audio-converter.git
cd go-audio-converter
make build
./bin/audioconv --help
```

## Usage

### CLI

```bash
# Basic conversion
audioconv input.wav output.mp3
audioconv input.flac output.mp3
audioconv input.ogg output.mp3
audioconv input.mp3 output.wav

# With options
audioconv input.wav output.mp3 --bitrate 320
audioconv input.flac output.mp3 --normalize --trim-silence

# Directory conversion
audioconv ./music --output ./converted --format mp3

# Get file info
audioconv info audio.wav

# List supported formats
audioconv formats
```

### HTTP API

```bash
# Start server
audioconv serve --port 8080

# Convert file
curl -X POST -F "file=@input.wav" http://localhost:8080/api/convert/mp3 -o output.mp3

# With options
curl -X POST -F "file=@input.flac" \
  "http://localhost:8080/api/convert?format=mp3&normalize=true" \
  -o output.mp3

# Get file info
curl -X POST -F "file=@audio.wav" http://localhost:8080/api/info
```

### As Library

```go
package main

import (
    "os"
    "github.com/formeo/go-audio-converter/pkg/converter"
)

func main() {
    // Basic usage
    conv := converter.New()
    err := conv.ConvertFile("input.wav", "output.mp3")
    if err != nil {
        panic(err)
    }
    
    // With options
    opts := converter.Options{
        Bitrate:     320,
        Normalize:   true,
        TrimSilence: true,
    }
    conv = converter.NewWithOptions(opts)
    
    // Convert streams
    in, _ := os.Open("input.flac")
    out, _ := os.Create("output.mp3")
    err = conv.Convert(in, out, converter.FormatFLAC, converter.FormatMP3)
    
    // Batch conversion
    result, err := conv.ConvertDir("./music", "./converted", converter.FormatMP3)
    fmt.Printf("Converted: %d, Failed: %d\n", len(result.Converted), len(result.Failed))
}
```

### Docker

```bash
# Convert single file
docker run --rm -v $(pwd):/data formeo/audioconv input.wav output.mp3

# Directory conversion
docker run --rm -v $(pwd):/data formeo/audioconv ./music -o ./converted -f mp3

# HTTP server
docker run --rm -p 8080:8080 formeo/audioconv serve --host 0.0.0.0
```

## Supported Formats

| Input  | Output | Notes |
|--------|--------|-------|
| WAV    | MP3    | ✅ Full support |
| WAV    | WAV    | ✅ Resample/rechannelize |
| FLAC   | MP3    | ✅ Full support |
| FLAC   | WAV    | ✅ Full support |
| OGG    | MP3    | ✅ Vorbis only |
| OGG    | WAV    | ✅ Vorbis only |
| MP3    | WAV    | ✅ Full support |

## Options

### CLI Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-o, --output` | Output file or directory | auto |
| `-f, --format` | Output format (mp3, wav) | mp3 |
| `-b, --bitrate` | MP3 bitrate (128-320) | 192 |
| `-s, --sample-rate` | Output sample rate | keep |
| `-c, --channels` | Channels (1=mono, 2=stereo) | keep |
| `-n, --normalize` | Normalize audio levels | false |
| `-t, --trim-silence` | Trim silence from edges | false |
| `--overwrite` | Overwrite existing files | false |
| `-v, --verbose` | Verbose output | false |

### Library Options

```go
type Options struct {
    Bitrate     int  // MP3 bitrate: 128, 192, 256, 320
    Quality     int  // Quality: 0 (best) to 9 (worst)
    Channels    int  // 1 = mono, 2 = stereo, 0 = keep
    SampleRate  int  // Hz: 44100, 48000, etc., 0 = keep
    Normalize   bool // Normalize audio levels
    TrimSilence bool // Remove silence from edges
    Overwrite   bool // Overwrite existing files
}
```

## Limitations

- **MP3 quality**: Uses Shine encoder (not LAME). Files are ~10-15% larger with slightly lower quality. Perfect for preprocessing, not for audiophile use.
- **No ID3 tags**: Metadata is not preserved during conversion.
- **Sample rate conversion**: Uses linear interpolation. For critical applications, use dedicated resampling tools.
- **No streaming**: Entire file is loaded into memory. For very large files (>1GB), process in chunks.

## Benchmarks

```
BenchmarkWAVtoMP3_1sec-8      100    15ms/op    1.2MB/op
BenchmarkFLACtoMP3_1sec-8      50    28ms/op    2.1MB/op
BenchmarkOGGtoMP3_1sec-8       50    25ms/op    1.8MB/op
```

Tested on Apple M1, 16GB RAM.

## Use Cases

- 🎵 **Music library conversion** — bulk convert your collection
- 🤖 **ML preprocessing** — prepare audio for speech recognition, music analysis
- 🐳 **Minimal Docker images** — no ffmpeg = smaller images
- 🌐 **Browser apps** — WASM support for client-side conversion
- 📦 **Embedded systems** — single static binary, no dependencies
- ☁️ **Serverless** — fast cold starts, small deployment size

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      audioconv CLI                          │
├─────────────────────────────────────────────────────────────┤
│                    HTTP API Server                          │
├─────────────────────────────────────────────────────────────┤
│                  pkg/converter                              │
│  ┌──────────────┬──────────────┬──────────────────────────┐ │
│  │   Decoders   │   Encoders   │      Processing          │ │
│  │  WAV, FLAC   │   MP3, WAV   │  Resample, Normalize     │ │
│  │  OGG, MP3    │              │  Trim, Channels          │ │
│  └──────────────┴──────────────┴──────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│                   External Libraries                         │
│  shine-mp3 | go-audio | go-mp3 | oggvorbis | flac          │
└─────────────────────────────────────────────────────────────┘
```

## Dependencies

All pure Go, no CGO:

| Library | Purpose |
|---------|---------|
| [braheezy/shine-mp3](https://github.com/braheezy/shine-mp3) | MP3 encoding |
| [hajimehoshi/go-mp3](https://github.com/hajimehoshi/go-mp3) | MP3 decoding |
| [go-audio/wav](https://github.com/go-audio/wav) | WAV reading |
| [mewkiz/flac](https://github.com/mewkiz/flac) | FLAC decoding |
| [jfreymuth/oggvorbis](https://github.com/jfreymuth/oggvorbis) | OGG decoding |
| [spf13/cobra](https://github.com/spf13/cobra) | CLI framework |

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing`)
5. Open a Pull Request

### Development

```bash
# Setup
git clone https://github.com/formeo/go-audio-converter.git
cd go-audio-converter
make deps

# Build
make build

# Test
make test

# Lint
make lint

# All checks
make check
```

## License

MIT License. See [LICENSE](LICENSE) for details.

## Related Projects

- [music_recognition](https://github.com/formeo/music_recognition) — Bulk music identification via Shazam
- [audiobook-cleaner](https://github.com/formeo/audiobook-cleaner) — AI-powered audiobook noise removal

## Author

**Roman Gordienko** ([@formeo](https://github.com/formeo))

---

If this project helped you, consider giving it a ⭐!
