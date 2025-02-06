package main

import (
	"flag"
	"fmt"
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
)

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

	var res io.Writer
	switch *format {
	case "i16":
		res, err = getResampler[int16](out, *ir, *or, *ch, *q)
	case "i32":
		res, err = getResampler[int32](out, *ir, *or, *ch, *q)
	case "i64":
		res, err = getResampler[int64](out, *ir, *or, *ch, *q)
	case "f32":
		res, err = getResampler[float32](out, *ir, *or, *ch, *q)
	case "f64":
		res, err = getResampler[float64](out, *ir, *or, *ch, *q)
	default:
		err = fmt.Errorf("unsupported format: %s", *format)
	}
	if err != nil {
		os.Remove(outputPath)
		log.Fatalln(err)
	}

	_, err = io.Copy(res, in)
	if err != nil {
		os.Remove(outputPath)
		log.Fatalln(err)
	}
}

func validateArgs() {
	if *ch <= 0 {
		log.Fatalln("Specif correct number of channels")
	}
	if *ir <= 0 {
		log.Fatalln("Specif correct input sample rate")
	}
	if *or <= 0 {
		log.Fatalln("Specif correct ouput sample rate")
	}
}

func getResampler[T resample.Number](output io.Writer, inRate, outRate int, ch int, q string) (*resample.Resampler[T], error) {
	var filter resample.Option[T]
	switch q {
	case "linear":
		filter = resample.LinearFilter[T]()
	case "kaiser_fast":
		filter = resample.KaiserFastFilter[T]()
	case "kaiser_best":
		filter = resample.KaiserBestFilter[T]()
	default:
		return nil, fmt.Errorf("unknown filter type: %s", q)
	}
	return resample.New[T](output, inRate, outRate, ch, filter)
}
