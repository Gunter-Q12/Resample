package resample

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/exp/constraints"
	"io"
	"reflect"
)

type Number interface {
	constraints.Float | constraints.Integer
}

type Resampler[T Number] struct {
	outBuf   io.Writer
	inRate   int
	outRate  int
	ch       int
	filter   Filter
	elemSize int
}

func New[T Number](outBuffer io.Writer, inRate, outRate, ch int,
	options ...option[T]) (*Resampler[T], error) {
	if inRate <= 0 || outRate <= 0 {
		return nil, errors.New("sampling rates must be greater than zero")
	}

	var t T
	elemSize := int(reflect.TypeOf(t).Size())

	resampler := &Resampler[T]{
		outBuf:   outBuffer,
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

	if len(samples) < 2 {
		return 0, errors.New("input should have at least two samples")
	}

	timeIncrement := float64(r.inRate) / float64(r.outRate)
	shape := int(float64(len(samples)) * float64(r.outRate) / float64(r.inRate))
	timeOut := make([]float64, shape)
	for i := range timeOut {
		timeOut[i] = float64(i) * timeIncrement
	}

	y := make([]T, shape)

	r.convolve(samples, timeOut, &y)

	err = binary.Write(r.outBuf, binary.LittleEndian, y)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}
	return len(input), nil
}

func (r *Resampler[T]) convolve(samples []T, timeOut []float64, y *[]T) {
	samplesLen := len(samples)
	for t := range *y {
		var newSample float64

		timeRegister := timeOut[t]
		offset := timeRegister - float64(int(timeRegister))
		sampleId := int(timeRegister)

		// computing left wing (because of the middle element)
		i := 0
		weight, err := r.filter.GetPoint(offset, i)
		for sampleId-i >= 0 && err == nil {
			newSample += weight * float64(samples[sampleId-i])
			i += 1
			weight, err = r.filter.GetPoint(offset, i)
		}

		offset = 1 - offset

		// computing right wing
		i = 0
		weight, err = r.filter.GetPoint(offset, i)
		for (sampleId+i+1) < samplesLen && err == nil {
			newSample += weight * float64(samples[sampleId+i+1])
			i += 1
			weight, err = r.filter.GetPoint(offset, i)
		}
		(*y)[t] = T(newSample) // TODO: proper rounding
	}
}
