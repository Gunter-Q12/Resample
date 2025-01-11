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

	var output []T
	switch t := r.filter.(type) {
	case *linearFilter:
		output, err = r.linear(samples)
	case *kaiserFilter:
		output, err = r.kaiserFast(samples)
	default:
		err = fmt.Errorf("custom filters are not supported: %v", t)
	}
	if err != nil {
		return 0, err
	}

	err = binary.Write(r.outBuf, binary.LittleEndian, output)
	if err != nil {
		return 0, fmt.Errorf("resampler write: %w", err)
	}
	return len(input), nil
}

func (r *Resampler[T]) linear(samples []T) ([]T, error) {
	if len(samples) < 2 {
		return nil, errors.New("input should have at least two samples")
	}

	ratio := float64(r.outRate) / float64(r.inRate)
	shape := int(float64(len(samples)) * float64(r.outRate) / float64(r.inRate))

	scale := 1.0
	timeIncrement := 1.0 / ratio

	timeOut := make([]float64, shape)
	for i := range timeOut {
		timeOut[i] = float64(i) * timeIncrement
	}

	y := make([]T, shape)
	r.resample(samples, timeOut, scale, &y)
	return y, nil
}

func (r *Resampler[T]) kaiserFast(samples []T) ([]T, error) {
	ratio := float64(r.outRate) / float64(r.inRate)
	shape := int(float64(len(samples)) * float64(r.outRate) / float64(r.inRate))

	timeIncrement := 1.0 / ratio
	timeOut := make([]float64, shape)
	for i := range timeOut {
		timeOut[i] = float64(i) * timeIncrement
	}

	scale := min(1.0, float64(r.outRate)/float64(r.inRate))

	y := make([]T, shape)
	r.resample(samples, timeOut, scale, &y)
	return y, nil
}

func (r *Resampler[T]) resample(samples []T, timeOut []float64, scale float64, y *[]T) {

	winLen := r.filter.GetLength()
	samplesLen := len(samples)

	for t := range *y {
		var newSample float64

		timeRegister := timeOut[t]

		sampleId := int(timeRegister)
		frac := scale * (timeRegister - float64(sampleId))
		step := float64(r.filter.GetDensity()) * scale
		filterId := int(frac * float64(r.filter.GetDensity()))
		frac -= float64(filterId) * (1 / float64(r.filter.GetDensity()))

		// computing left wing (because of the middle element)
		i := 0
		for sampleId-i >= 0 && int(float64(filterId)+step*float64(i)) < winLen {
			currFilter := int(float64(filterId) + step*float64(i))
			currFrac := frac + step*float64(i) - float64(int(frac+step*float64(i)))

			weight := scale * r.filter.GetValue(float64(currFilter)+currFrac)
			newSample += weight * float64(samples[sampleId-i])
			i += 1
		}

		frac = scale * (1 - (timeRegister - float64(sampleId)))
		filterId = int(frac * float64(r.filter.GetDensity()))
		frac -= float64(filterId) * (1 / float64(r.filter.GetDensity()))

		// computing right wing
		i = 0
		for (sampleId+i+1) < samplesLen && int(float64(filterId)+step*float64(i)) < winLen {
			currFilter := int(float64(filterId) + step*float64(i))
			currFrac := frac + step*float64(i) - float64(int(frac+step*float64(i)))

			weight := scale * r.filter.GetValue(float64(currFilter)+currFrac)
			newSample += weight * float64(samples[sampleId+i+1])
			i += 1
		}
		(*y)[t] = T(newSample) // TODO: proper rounding
	}
}
