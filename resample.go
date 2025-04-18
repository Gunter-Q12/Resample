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
	data, err := io.ReadAll(reader)
	if err != nil {
		return 0, fmt.Errorf("resampler: reading from: %w", err)
	}
	n, err := r.Write(data)
	return int64(n), err
}

// write is an actual implementation of Write.
func write[T number](r *Resampler, input []byte) (int, error) {
	samples, err := getSamples[T](input, r.elemSize)
	if err != nil {
		return 0, fmt.Errorf("resampler: write: %w", err)
	}

	n := len(samples) / r.ch
	shape := int(float64(n) * float64(r.outRate) / float64(r.inRate))
	result := make([]T, shape*r.ch)
	timeIncrement := float64(r.inRate) / float64(r.outRate)

	frameFunc := calcFrame
	if r.memoization {
		frameFunc = calcFrameWithMemoization
	}
	convolveWithMemoization[T](r.f, samples, result, frameFunc, timeIncrement, r.ch)

	err = binary.Write(r.outBuf, binary.LittleEndian, result)
	if err != nil {
		return 0, fmt.Errorf("resampler: write: %w", err)
	}
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

func convolveWithMemoization[T number](f *filter, samples []float64, y []T,
	frameCalc frameCalcFunc, timeIncrement float64, ch int) {
	newSamples := make([]float64, ch)

	routines := runtime.NumCPU() * routinesPerCore
	frames := len(y) / ch
	framesPerRoutine := frames / routines
	if framesPerRoutine == 0 {
		routines = 1
		framesPerRoutine = frames
	}

	for startFrame := 0; startFrame < frames; startFrame += framesPerRoutine {
		for currFrame := range min(framesPerRoutine, frames-startFrame) {
			outputFrame := startFrame + currFrame
			inputTime := float64(outputFrame) * timeIncrement

			frameCalc(f, samples, newSamples, inputTime, outputFrame, ch)

			startSample := outputFrame * ch
			for s := range newSamples {
				y[startSample+s] = T(newSamples[s])
				newSamples[s] = 0
			}
		}
	}
}

type frameCalcFunc func(*filter, []float64, []float64, float64, int, int)

func calcFrame(f *filter, samples, newSamples []float64, inputTime float64, outputFrame int, ch int) {
	// computing left wing including the middle element
	samplesLen := len(samples) / ch
	sampleID := int(inputTime)
	offset := inputTime - float64(sampleID)
	iters := min(f.Length(offset), sampleID+1)
	for i := range iters {
		weight := f.Value(offset, i)
		batchID := (sampleID - i) * ch
		for s := range newSamples {
			newSamples[s] += weight * samples[batchID+s]
		}
	}

	offset = 1 - offset

	// computing right wing
	iters = min(f.Length(offset), samplesLen-1-sampleID)
	for i := range iters {
		weight := f.Value(offset, i)
		batchID := (sampleID + i + 1) * ch
		for s := range newSamples {
			newSamples[s] += weight * samples[batchID+s]
		}
	}
}

func calcFrameWithMemoization(f *filter, samples, newSamples []float64, inputTime float64, outputFrame int, ch int) {
	inputFrame := int(inputTime)
	samplesLen := len(samples) / ch
	offsetsNum := len(f.offsetWins)
	offset := outputFrame % offsetsNum

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
	iters = min(len(f.offsetWins[offset]), samplesLen-1-inputFrame)
	iters = max(start, iters)
	for i, weight := range f.offsetWins[offset][start:iters] {
		startSample := (inputFrame + i + 1) * ch
		for s := range newSamples {
			newSamples[s] += weight * samples[startSample+s]
		}
	}
}
