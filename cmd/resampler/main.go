package main

import (
	"encoding/binary"
	"flag"
	"log"
	"os"
	"path/filepath"
	"resample"
	"strings"
)

const wavHeader = 44

var (
	format = flag.String("format", "i16", "PCM format")
	ch     = flag.Int("ch", 2, "Number of channels")
	ir     = flag.Int("ir", 44100, "Input sample rate")
	or     = flag.Int("or", 0, "Output sample rate")
	q      = flag.Int("q", 0, "Output quality")
)

func main() {
	flag.Parse()
	if *format != "i16" {
		log.Fatalln("the only supported format is i16")
	}
	if *ch != 1 {
		log.Fatalln("the only supported number of channels is 2")
	}
	if *ir <= 0 || *or <= 0 {
		log.Fatalln("Invalid input or output sample rate")
	}
	if flag.NArg() < 2 {
		log.Fatalln("No input or output files given")
	}
	inputFile := flag.Arg(0)
	outputFile := flag.Arg(1)
	var err error

	// Open input file (WAV or RAW PCM)
	input, err := os.Open(inputFile)
	if err != nil {
		log.Fatalln(err)
	}
	defer input.Close()

	inputInfo, err := input.Stat()
	if err != nil {
		log.Fatalf("failed to get file info: %s", err)
	}
	inputSize := inputInfo.Size()

	output, err := os.Create(outputFile)
	if err != nil {
		log.Fatalln(err)
	}
	// Skip WAV file header in order to pass only the PCM data to the Resampler
	if strings.ToLower(filepath.Ext(inputFile)) == ".wav" {
		input.Seek(wavHeader, 0)
		inputSize -= wavHeader
	}

	// Read input and pass it to the Resampler in chunks
	int16Slice := make([]int16, inputSize/2)
	err = binary.Read(input, binary.LittleEndian, &int16Slice)
	if err != nil {
		log.Printf("convert input to int16: %s", err)
	}

	outputData, err := resample.Int16(int16Slice, *ir, *or, *ch, resample.Quality(*q))
	err = binary.Write(output, binary.LittleEndian, outputData)
	output.Close()

	if err != nil {
		os.Remove(outputFile)
		log.Fatalln(err)
	}
}
