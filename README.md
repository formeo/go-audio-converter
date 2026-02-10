![Downloads](https://img.shields.io/github/downloads/formeo/go-audio-converter/total)
![Stars](https://img.shields.io/github/stars/formeo/go-audio-converter)

# go-audio-converter

🎵 **Pure Go audio converter — no FFmpeg, no CGO.**

Convert between WAV, MP3, FLAC, and OGG using a single static binary.

## Why?

- **No FFmpeg** — no installation, no PATH issues
- **No CGO** — cross-compile for any platform
- **Single binary** — download and run
- **Pure Go** — works everywhere including WASM

## Installation

```bash
go install github.com/formeo/go-audio-converter/cmd/audioconv@latest
```

Or build from source:

```bash
git clone https://github.com/formeo/go-audio-converter.git
cd go-audio-converter
go build -o audioconv ./cmd/audioconv
```

## Usage

```bash
# WAV to MP3
audioconv input.wav output.mp3

# MP3 to WAV
audioconv input.mp3 output.wav

# FLAC to MP3
audioconv input.flac output.mp3

# OGG to FLAC
audioconv input.ogg output.flac

# Any format to FLAC (lossless)
audioconv input.wav output.flac
```

## Supported Conversions

| From | To WAV | To MP3 | To FLAC | To OGG |
|------|--------|--------|---------|--------|
| WAV  | ✅     | ✅     | ✅      | ❌     |
| MP3  | ✅     | ✅     | ✅      | ❌     |
| FLAC | ✅     | ✅     | ✅      | ❌     |
| OGG  | ✅     | ✅     | ✅      | ❌     |

**Legend:**
- ✅ Supported (pure Go)
- ❌ No pure Go encoder exists

## FLAC Encoder

This project includes a **custom pure Go FLAC encoder** — the first of its kind!

Features:
- FIXED prediction (orders 0-4)
- Rice coding for residuals
- Automatic prediction order selection
- MD5 checksum for verification
- Full STREAMINFO metadata

Expected compression ratios:
| Content | Compression |
|---------|-------------|
| Silence | 95%+ |
| Sine wave | 50-70% |
| Music | 30-50% |
| White noise | ~0% |

## Limitations

- **OGG encoding**: No pure Go Vorbis encoder exists. Decoding works fine.
- **MP3 encoder**: Uses [shine-mp3](https://github.com/braheezy/shine-mp3) (not LAME). Good quality, but files may be slightly larger.
- **FLAC encoder**: Uses FIXED prediction only (no LPC). Compression is good but not as optimal as libFLAC.
- **Memory**: Entire file loaded into memory.
- **No metadata**: ID3 tags and Vorbis comments are not preserved.

## Roadmap

- [ ] LPC prediction for better FLAC compression
- [ ] OGG Vorbis encoding (CGO option)
- [ ] Batch directory conversion
- [ ] Normalize audio levels
- [ ] Metadata preservation
- [ ] HTTP API server

## Dependencies

All pure Go, no CGO:

| Library | Purpose |
|---------|---------|
| [braheezy/shine-mp3](https://github.com/braheezy/shine-mp3) | MP3 encoding |
| [hajimehoshi/go-mp3](https://github.com/hajimehoshi/go-mp3) | MP3 decoding |
| [go-audio/wav](https://github.com/go-audio/wav) | WAV reading |
| [mewkiz/flac](https://github.com/mewkiz/flac) | FLAC decoding |
| [jfreymuth/oggvorbis](https://github.com/jfreymuth/oggvorbis) | OGG/Vorbis decoding |
| **Built-in** | FLAC encoding, WAV writing |

## Part of audiotools.dev

This converter is part of the [audiotools.dev](https://audiotools.dev) ecosystem:

- **[music_recognition](https://github.com/formeo/music_recognition)** — Bulk music identification via Shazam
- **[Audiobook-Cleaner](https://github.com/formeo/Audiobook-Cleaner)** — AI-powered audiobook noise removal
- **go-audio-converter** — Format conversion without dependencies

## License

MIT

## Author

[@formeo](https://github.com/formeo)
