package main

import (
	"flag"
	"gitlab.com/gunter-go/resample"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const wavHeaderSize = 44

var (
	format = flag.String("format", "", "PCM format: i16, i32, i64, f32, f64")
	ch     = flag.Int("ch", 0, "Number of channels")
	ir     = flag.Int("ir", 0, "Input sample rate in Hz")
	or     = flag.Int("or", 0, "Output sample rate in Hz")
	q      = flag.String("q", "kaiser_fast",
		"Output quality: linear, kaiser_fast, kaiser_best")
	ml = flag.Int("ml", 50*1024*1024, "Memory limit in bytes. 0 disable memoization. -1 means no limit")
)

var flagToFormat = map[string]resample.Format{
	"i16": resample.FormatInt16,
	"i32": resample.FormatInt32,
	"i64": resample.FormatInt64,
	"f32": resample.FormatFloat32,
	"f64": resample.FormatFloat64,
}

var flagToFilter = map[string]resample.Option{
	"linear":         resample.WithLinearFilter(),
	"kaiser_fastest": resample.WithKaiserFastestFilter(),
	"kaiser_fast":    resample.WithKaiserFastFilter(),
	"kaiser_best":    resample.WithKaiserBestFilter(),
}

func main() {
	flag.Parse()

	if flag.NArg() < 2 {
		log.Fatalln("No input or output files given")
	}
	inputPath := flag.Arg(0)
	outputPath := flag.Arg(1)

	in, err := os.Open(inputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer in.Close()

	if strings.ToLower(filepath.Ext(inputPath)) == ".wav" {
		*ir, *ch, *format, err = readHeader(in)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("parameters are overwritten with .WAV file header: -ir %d -ch %d -f %s",
			*ir, *ch, *format)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	if strings.ToLower(filepath.Ext(outputPath)) == ".wav" {
		out.Seek(wavHeaderSize, io.SeekStart)
		defer func(f *os.File, rate, ch int, format string) {
			f.Seek(0, io.SeekStart)
			writeHeader(f, rate, ch, format)
		}(out, *or, *ch, *format)
	}

	validateArgs()

	res, err := resample.New(out, flagToFormat[*format], *ir, *or, *ch, flagToFilter[*q], resample.WithMemoryLimit(*ml))
	if err != nil {
		os.Remove(outputPath)
		log.Fatalf("Error while creating a resampler: %s", err)
	}

	_, err = io.Copy(res, in)
	if err != nil {
		os.Remove(outputPath)
		log.Fatalf("Error while resampling: %s", err)
	}
}

func validateArgs() {
	if *ch <= 0 {
		log.Fatalf("Incorrect number of channels: %d. Must be > 0", *ch)
	}
	if *ir <= 0 {
		log.Fatalf("Incorrect input rate: %d. Must be > 0", *ir)
	}
	if *or <= 0 {
		log.Fatalf("Incorrect output rate: %d. Must be > 0", *or)
	}

	if _, ok := flagToFormat[*format]; !ok {
		log.Fatalf("Incorrect format:: %s", *format)
	}

	if _, ok := flagToFilter[*q]; !ok {
		log.Fatalf("Incorrect quality: %s", *q)
	}
}
