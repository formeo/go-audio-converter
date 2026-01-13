# go-audio-converter

🎵 **Pure Go audio converter — no ffmpeg, no CGO.**

Convert between WAV and MP3 using a single static binary.

## Why?

- **No ffmpeg** — no installation, no PATH issues
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
```

## Supported Conversions

| From | To  | Status |
|------|-----|--------|
| WAV  | MP3 | ✅ |
| MP3  | WAV | ✅ |
| WAV  | WAV | ✅ |

## Limitations

- **MP3 encoder**: Uses [shine-mp3](https://github.com/braheezy/shine-mp3) (not LAME). Good quality, but files may be slightly larger.
- **Memory**: Entire file loaded into memory. For very large files, consider chunking.
- **No metadata**: ID3 tags are not preserved.

## Roadmap

- [ ] FLAC support
- [ ] OGG Vorbis support
- [ ] Batch directory conversion
- [ ] Normalize audio levels
- [ ] HTTP API server

## Dependencies

All pure Go, no CGO:

| Library | Purpose |
|---------|---------|
| [braheezy/shine-mp3](https://github.com/braheezy/shine-mp3) | MP3 encoding |
| [hajimehoshi/go-mp3](https://github.com/hajimehoshi/go-mp3) | MP3 decoding |
| [go-audio/wav](https://github.com/go-audio/wav) | WAV reading/writing |

## Related Projects

- [music_recognition](https://github.com/formeo/music_recognition) — Bulk music identification via Shazam
- [Audiobook-Cleaner](https://github.com/formeo/Audiobook-Cleaner) — AI-powered audiobook noise removal

## License

MIT

## Author

[@formeo](https://github.com/formeo)
