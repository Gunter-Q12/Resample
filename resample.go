package resample

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/exp/constraints"
	"io"
	"runtime"
	"slices"
	"sync"
)

const routinesPerCore = 4

type number interface {
	constraints.Float | constraints.Integer
}

type Format int

const (
	FormatInt16 Format = iota
	FormatInt32
	FormatInt64
	FormatFloat32
	FormatFloat64
)

//nolint:mnd // map used as a constant
var formatElementSize = map[Format]int{
	FormatInt16:   2,
	FormatInt32:   4,
	FormatInt64:   8,
	FormatFloat32: 4,
	FormatFloat64: 8,
}

// A Resampler is an object that writes resampled data into an io.Writer.
type Resampler struct {
	outBuf      io.Writer
	format      Format
	inRate      int
	outRate     int
	ch          int
	memoization bool
	f           *filter
	elemSize    int
}

// New creates a new Resampler.
//
// Default filter is kaiser fast filter, use WithXFilter options to change it.
// Memoization is enabled by default, use WithNoMemoization function to disable it.
func New(outBuffer io.Writer, format Format, inRate, outRate, ch int,
	options ...Option) (*Resampler, error) {
	if inRate <= 0 || outRate <= 0 || ch <= 0 {
		return nil, errors.New("sampling rates and channel number must be greater than zero")
	}

	resampler := &Resampler{
		outBuf:      outBuffer,
		format:      format,
		inRate:      inRate,
		outRate:     outRate,
		ch:          ch,
		memoization: true,
		elemSize:    formatElementSize[format],
	}

	slices.SortFunc(options, optionCmp)
	for _, option := range options {
		if err := option.apply(resampler); err != nil {
			return nil, err
		}
	}

	if resampler.f == nil {
		if err := WithKaiserFastFilter().apply(resampler); err != nil {
			return nil, err
		}
	}

	return resampler, nil
}

func (r *Resampler) Write(input []byte) (int, error) {
	switch r.format {
	case FormatInt16:
		return write[int16](r, input)
	case FormatInt32:
		return write[int32](r, input)
	case FormatInt64:
		return write[int64](r, input)
	case FormatFloat32:
		return write[float32](r, input)
	case FormatFloat64:
		return write[float64](r, input)
	default:
		panic("unknown format")
	}
}

func (r *Resampler) ReadFrom(reader io.Reader) (int64, error) {
	switch r.format {
	case FormatInt16:
		return readFrom[int16](r, reader)
	case FormatInt32:
		return readFrom[int32](r, reader)
	case FormatInt64:
		return readFrom[int64](r, reader)
	case FormatFloat32:
		return readFrom[float32](r, reader)
	case FormatFloat64:
		return readFrom[float64](r, reader)
	default:
		panic("unknown format")
	}
}

func readFrom[T number](r *Resampler, reader io.Reader) (int64, error) {
	wingSize := r.f.Length(0) * r.elemSize
	middleSize := (runtime.NumCPU()*1024 + r.inRate - 1) / r.inRate * r.inRate
	buffSize := wingSize*3 + middleSize*r.elemSize

	buff := make([]byte, buffSize)

	c := convolver[T]{
		r: r,
	}

	read := 0

	n, err := reader.Read(buff[:middleSize+wingSize])
	read += n
	if err != nil && err != io.EOF {
		return int64(read), err
	}
	if n < middleSize+wingSize {
		_, err = c.resample(buff[:n], 0, n)
		return int64(read), err
	}

	// Special case for the first part
	_, err = c.resample(buff[:middleSize], 0, middleSize)
	if err != nil {
		return int64(read), err
	}
	_ = copy(buff[:wingSize*2], buff[middleSize-wingSize:middleSize+wingSize])

	for {
		n, err = reader.Read(buff[wingSize*2 : wingSize*2+middleSize])
		read += n
		if err != nil && err != io.EOF {
			return int64(read), err
		}
		if n < middleSize {
			_, err = c.resample(buff[0:wingSize*2+n], wingSize, wingSize*2+n)
			return int64(read), err
		}

		_, err = c.resample(buff[0:wingSize*2+middleSize], wingSize, wingSize+middleSize)
		if err != nil {
			return int64(read), err
		}
		_ = copy(buff[:wingSize*2], buff[middleSize:middleSize+wingSize*2])
	}
}

func write[T number](r *Resampler, input []byte) (int, error) {
	c := convolver[T]{
		r: r,
	}
	return c.resample(input, 0, len(input))
}

type convolver[T number] struct {
	r             *Resampler
	frameFunc     frameCalcFunc[T]
	timeIncrement float64

	startSample int
	endSample   int
	processed   int
	samples     []float64
	output      []T
}

// resample is an actual implementation of Write.
func (c *convolver[T]) resample(input []byte, start, end int) (int, error) {
	samples, err := getSamples[T](input, c.r.elemSize)
	if err != nil {
		return 0, fmt.Errorf("resampler: resample: %w", err)
	}

	c.frameFunc = calcFrame[T]
	c.timeIncrement = float64(c.r.inRate) / float64(c.r.outRate)
	c.startSample = start / c.r.elemSize
	c.endSample = end / c.r.elemSize
	c.samples = samples

	inFrames := (c.endSample - c.startSample) / c.r.ch
	outFrames := int(float64(inFrames*c.r.outRate) / float64(c.r.inRate))
	outSamples := outFrames * c.r.ch

	c.output = make([]T, outSamples)

	if c.r.memoization {
		c.frameFunc = calcFrameWithMemoization
	}

	c.convolve()

	err = binary.Write(c.r.outBuf, binary.LittleEndian, c.output)
	if err != nil {
		return 0, fmt.Errorf("resampler: resample: %w", err)
	}

	c.processed += outSamples
	return len(input), nil
}

// getSamples reads input and converts it to a slice of floats.
func getSamples[T number](input []byte, elemSize int) ([]float64, error) {
	samples := make([]T, len(input)/elemSize)
	err := binary.Read(bytes.NewReader(input), binary.LittleEndian, &samples)
	if err != nil {
		return nil, fmt.Errorf("getting samples: %w", err)
	}

	fSamples := make([]float64, len(samples))
	for i, s := range samples {
		fSamples[i] = float64(s)
	}
	return fSamples, nil
}

func (c *convolver[T]) convolve() {
	routines := runtime.NumCPU() * routinesPerCore
	frames := len(c.output) / c.r.ch
	framesPerRoutine := (frames + routines - 1) / routines
	if frames < routines {
		routines = 1
		framesPerRoutine = frames
	}

	allNewSamples := make([]float64, routines*c.r.ch)

	wg := sync.WaitGroup{}
	for i := range routines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startFrame := framesPerRoutine * i
			batchSize := min(framesPerRoutine, frames-startFrame)
			newSamples := allNewSamples[i*c.r.ch : (i+1)*c.r.ch]
			for currFrame := range batchSize {
				outputFrame := startFrame + currFrame
				inputTime := float64(outputFrame) * c.timeIncrement

				c.frameFunc(c.r.f, c.samples, newSamples, inputTime, outputFrame, c)

				startSample := outputFrame * c.r.ch
				for s, sample := range newSamples {
					c.output[startSample+s] = T(sample)
					newSamples[s] = 0
				}
			}
		}()
	}
	wg.Wait()
}

type frameCalcFunc[T number] func(*filter, []float64, []float64, float64, int, *convolver[T])

func calcFrame[T number](f *filter, samples, newSamples []float64, inputTime float64, _ int, c *convolver[T]) {
	ch := len(newSamples)
	batchNum := len(samples) / ch

	inputFrame := int(inputTime) + (c.startSample / c.r.ch)
	offset := inputTime + float64(c.processed/c.r.ch)*c.timeIncrement
	offset = offset - float64(int(offset))

	// computing left wing including the middle element
	iters := min(f.Length(offset), inputFrame+1)
	for i := range iters {
		weight := f.Value(offset, i)
		startSample := (inputFrame - i) * ch
		for s := range newSamples {
			newSamples[s] += weight * samples[startSample+s]
		}
	}

	offset = 1 - offset

	// computing right wing
	iters = min(f.Length(offset), batchNum-1-inputFrame)
	for i := range iters {
		weight := f.Value(offset, i)
		startSample := (inputFrame + i + 1) * ch
		for s := range newSamples {
			newSamples[s] += weight * samples[startSample+s]
		}
	}
}

func calcFrameWithMemoization[T number](
	f *filter, samples, newSamples []float64, inputTime float64, outputFrame int, c *convolver[T],
) {
	ch := len(newSamples)
	batchNum := len(samples) / ch

	offsetsNum := len(f.offsetWins)
	inputFrame := int(inputTime) + (c.startSample / c.r.ch)
	offset := (outputFrame + c.processed) % offsetsNum

	// computing left wing including the middle element
	iters := min(len(f.offsetWins[offset]), inputFrame+1)
	for i, weight := range f.offsetWins[offset][:iters] {
		startSample := (inputFrame - i) * ch
		for s := range newSamples {
			newSamples[s] += weight * samples[startSample+s]
		}
	}

	offset = (offsetsNum - offset) % offsetsNum

	// computing right wing
	start := 0
	if offset == 0 { // avoid counting the first element twice
		start = 1
	}
	iters = min(len(f.offsetWins[offset]), batchNum-1-inputFrame)
	iters = max(start, iters)
	for i, weight := range f.offsetWins[offset][start:iters] {
		startSample := (inputFrame + i + 1) * ch
		for s := range newSamples {
			newSamples[s] += weight * samples[startSample+s]
		}
	}
}
