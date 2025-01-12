package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"resample/pkg/resample"
	"strings"
)

const wavHeader = 44

var (
	format = flag.String("format", "f64", "PCM format")
	ch     = flag.Int("ch", 2, "Number of channels")
	ir     = flag.Int("ir", 44100, "Input sample rate")
	or     = flag.Int("or", 0, "Output sample rate")
	q      = flag.String("q", "kaiser_fast", "Output quality")
)

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatalln("No input or output files given")
	}

	inputPath := flag.Arg(0)
	outputPath := flag.Arg(1)

	input, err := getInputData(inputPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer input.Close()

	output, err := os.Create(outputPath)
	if err != nil {
		log.Fatalln(err)
	}
	defer output.Close()

	var res io.Writer
	switch *format {
	case "i16":
		res, err = getResampler[int16](output, *ir, *or, *ch, *q)
	case "i32":
		res, err = getResampler[int32](output, *ir, *or, *ch, *q)
	case "i64":
		res, err = getResampler[int64](output, *ir, *or, *ch, *q)
	case "f32":
		res, err = getResampler[float32](output, *ir, *or, *ch, *q)
	case "f64":
		res, err = getResampler[float64](output, *ir, *or, *ch, *q)
	default:
		err = fmt.Errorf("unsupported format: %s", *format)
	}
	if err != nil {
		os.Remove(outputPath)
		log.Fatalln(err)
	}

	_, err = io.Copy(res, input)
	if err != nil {
		os.Remove(outputPath)
		log.Fatalln(err)
	}
}

func getInputData(path string) (io.ReadCloser, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// Skip WAV file header in order to pass only the PCM data to the Resampler
	if strings.ToLower(filepath.Ext(path)) == ".wav" {
		input.Seek(wavHeader, 0)
	}

	return input, nil
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
