package main

import (
	"flag"
	"fmt"
	"gitlab.com/gunter-go/resample"
	"io"
	"log"
	"os"
)

const wavHeader = 44

var (
	format = flag.String("format", "", "PCM format")
	ch     = flag.Int("ch", 0, "Number of channels")
	ir     = flag.Int("ir", 0, "Input sample rate")
	or     = flag.Int("or", 0, "Output sample rate")
	q      = flag.String("q", "kaiser_fast", "Output quality")
)

func main() {
	flag.Parse()
	validateArgs()

	inputPath := flag.Arg(0)
	outputPath := flag.Arg(1)

	input, output, err := getInOutFiles(inputPath, outputPath, *or)
	if err != nil {
		log.Fatalln(err)
	}
	defer input.Close()
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

func validateArgs() {
	if flag.NArg() < 2 {
		log.Fatalln("No input or output files given")
	}
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
