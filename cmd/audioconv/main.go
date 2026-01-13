package main

import (
	"fmt"
	"os"
	"time"
    
	"github.com/formeo/go-audio-converter/pkg/converter"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: audioconv <input> <output>")
		fmt.Println("")
		fmt.Println("Supported conversions:")
		fmt.Println("  WAV -> WAV (copy/resample)")
		fmt.Println("  MP3 -> WAV (decode)")
		fmt.Println("  WAV -> MP3 (coming soon)")
		os.Exit(1)
	}

	input := os.Args[1]
	output := os.Args[2]

	fmt.Printf("Converting: %s -> %s\n", input, output)

	conv := converter.New()


	start := time.Now()
	if err := conv.ConvertFile(input, output); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Done in %v\n", time.Since(start).Round(time.Millisecond))
}
