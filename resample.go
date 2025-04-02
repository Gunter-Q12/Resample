package resample

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/exp/constraints"
	"io"
)

type Number interface {
	constraints.Float | constraints.Integer
}

type Format int

const (
	FormatInt16 = iota
	FormatInt32
	FormatInt64
	FormatFloat32
	FormatFloat64
)

var formatElementSize = map[Format]int{
	FormatInt16:   2,
	FormatInt32:   4,
	FormatInt64:   8,
	FormatFloat32: 4,
	FormatFloat64: 8,
}

type Resampler[T Number] struct {
	outBuf   io.Writer
	format   Format
	inRate   int
	outRate  int
	ch       int
	f        *filter
	elemSize int
}

func New[T Number](outBuffer io.Writer, format Format, inRate, outRate, ch int,
	options ...Option[T]) (*Resampler[T], error) {
	if inRate <= 0 || outRate <= 0 || ch <= 0 {
		return nil, errors.New("sampling rates and channel number must be greater than zero")
	}

	elemSize := formatElementSize[format]

	resampler := &Resampler[T]{
		outBuf:   outBuffer,
		format:   format,
		inRate:   inRate,
		outRate:  outRate,
		ch:       ch,
		elemSize: elemSize,
	}

	for _, option := range options {
		if err := option(resampler); err != nil {
			return nil, err
		}
	}

	if resampler.f == nil {
		if err := KaiserBestFilter[T]()(resampler); err != nil {
			return nil, err
		}
	}

	return resampler, nil
}

func (r *Resampler[T]) Write(input []byte) (int, error) {
	if r.inRate == r.outRate {
		return r.outBuf.Write(input)
	}

	samples := make([]T, len(input)/r.elemSize)
	err := binary.Read(bytes.NewReader(input), binary.LittleEndian, &samples)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}

	n := len(samples) / r.ch

	if n < 2 {
		return 0, errors.New("input should have at least two samples")
	}

	shape := int(float64(n) * float64(r.outRate) / float64(r.inRate))
	result := make([]T, shape*r.ch)

	timeIncrement := float64(r.inRate) / float64(r.outRate)
	y := make([]T, shape)
	channel := make([]T, n)
	for i := 0; i < r.ch; i++ {
		for j := 0; j < n; j++ {
			channel[j] = samples[j*r.ch+i]
		}
		r.convolveAny(r.f, channel, timeIncrement, &y)
		for j := 0; j < shape; j++ {
			result[j*r.ch+i] = y[j]
		}
	}

	err = binary.Write(r.outBuf, binary.LittleEndian, result)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}
	return len(input), nil
}

func (r *Resampler[T]) convolveAny(f *filter, samples any, timeIncrement float64, y any) {
	switch r.format {
	case FormatInt16:
		convolve[int16](f, samples.([]int16), timeIncrement, y.(*[]int16))
	case FormatInt32:
		convolve[int32](f, samples.([]int32), timeIncrement, y.(*[]int32))
	case FormatInt64:
		convolve[int64](f, samples.([]int64), timeIncrement, y.(*[]int64))
	case FormatFloat32:
		convolve[float32](f, samples.([]float32), timeIncrement, y.(*[]float32))
	case FormatFloat64:
		convolve[float64](f, samples.([]float64), timeIncrement, y.(*[]float64))
	default:
		panic("unknown format")
	}
}

func convolve[T Number](f *filter, samples []T, timeIncrement float64, y *[]T) {
	samplesLen := len(samples)
	for t := range *y {
		var newSample float64

		timeRegister := float64(t) * timeIncrement
		offset := timeRegister - float64(int(timeRegister))
		sampleId := int(timeRegister)

		// computing left wing (because of the middle element)
		iters := min(f.GetLength(offset), sampleId+1)
		for i := range iters {
			weight := f.GetPoint(offset, i)
			newSample += weight * float64(samples[sampleId-i])
		}

		offset = 1 - offset

		// computing right wing
		iters = min(f.GetLength(offset), samplesLen-1-sampleId)
		for i := range iters {
			weight := f.GetPoint(offset, i)
			newSample += weight * float64(samples[sampleId+i+1])
		}
		(*y)[t] = T(newSample) // TODO: proper rounding
	}
}
